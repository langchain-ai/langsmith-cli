package channels

// Compare what the GitHub channel needs with what the GitHub App has, and
// word the result for the deploy output. Pure, so every message is
// unit-tested.

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// listedRepositories is how many repositories are listed by name before the
// summary switches to a count.
const listedRepositories = 5

type githubReport struct {
	// ready is shown as success when the App is installed somewhere; "" when
	// it is not.
	ready string
	// warnings are things someone must fix on GitHub. Each names who acts and
	// where.
	warnings []string
	// notes are harmless observations.
	notes []string
}

func buildGitHubReport(channel *githubChannel, status *hostapi.GitHubChannelStatus) githubReport {
	app := status.App
	ownerIsOrg := app.Owner.IsOrganization()
	permissionsURL := githubAppPermissionsURL(app.Owner.Login, ownerIsOrg, app.Slug)
	var report githubReport

	for _, name := range sortedKeys(channel.permissions) {
		if slices.Contains(knownPermissions, name) {
			continue
		}
		hint := ""
		if similar := similarPermissions(name); len(similar) > 0 {
			hint = fmt.Sprintf(" Did you mean %s?", orList(similar))
		}
		report.warnings = append(report.warnings, fmt.Sprintf(
			"app.permissions names %s, which GitHub does not list as an App permission.%s", name, hint))
	}

	if changes, ok := appChanges(channel, status, permissionsURL); ok {
		report.warnings = append(report.warnings, changes)
	}

	for _, request := range status.InstallationRequests {
		requester := ""
		if request.Requester != nil {
			requester = " requested by " + request.Requester.Login
		}
		report.warnings = append(report.warnings, fmt.Sprintf(
			"Waiting for an owner of %s to approve the installation%s.", request.Account.Login, requester))
	}

	if len(status.Installations) == 0 {
		report.warnings = append(report.warnings, fmt.Sprintf(
			"GitHub App %s is not installed yet. Install it on your repositories at %s", app.Slug, githubInstallURL(app.Slug)))
		return report
	}

	var installedOn []string
	for _, installation := range status.Installations {
		settings := installationSettingsURL(installation)
		if installation.SuspendedAt != "" {
			report.warnings = append(report.warnings, fmt.Sprintf(
				"The installation on %s is suspended, so it receives no events. Unsuspend it at %s", installation.Account.Login, settings))
			continue
		}
		if repositories := missingRepositories(channel, installation); len(repositories) > 0 {
			report.warnings = append(report.warnings, fmt.Sprintf(
				"The installation on %s does not include %s, which this channel is limited to. Add them at %s",
				installation.Account.Login, strings.Join(repositories, ", "), settings))
		}
		if pending := missingPermissions(parseLevels(app.Permissions), installation.Permissions); len(pending) > 0 {
			report.warnings = append(report.warnings, fmt.Sprintf(
				"An owner of %s must accept the App's updated permissions (%s) at %s",
				installation.Account.Login, strings.Join(pending, ", "), settings))
		}
		installedOn = append(installedOn, installationScope(installation))
	}
	if len(installedOn) > 0 {
		report.ready = fmt.Sprintf("GitHub channel ready · App %s (%s) · installed on %s",
			app.Slug, app.Owner.Login, strings.Join(installedOn, "; "))
	}
	return report
}

// coversLevel reports whether an App's granted level covers needed. An
// unknown level (`admin`) covers what the channel needs.
func coversLevel(granted string, needed permissionLevel) bool {
	level, ok := parsePermissionLevel(granted)
	return !ok || level >= needed
}

// exceedsLevel reports whether an App's granted level is more than needed;
// an unknown level (`admin`) is.
func exceedsLevel(granted string, needed permissionLevel) bool {
	level, ok := parsePermissionLevel(granted)
	return !ok || level > needed
}

// appChanges is one list of what the App owner must add on GitHub and what
// they can remove; false when the App matches the channel.
func appChanges(channel *githubChannel, status *hostapi.GitHubChannelStatus, permissionsURL string) (string, bool) {
	app := status.App
	var add []row
	for _, name := range sortedKeys(channel.permissions) {
		needed := channel.permissions[name]
		level := needed.String()
		if granted, ok := app.Permissions[name]; ok {
			if coversLevel(granted, needed) {
				continue
			}
			level = granted + " → " + needed.String()
		}
		add = append(add, row{name: name, level: level, reason: why(channel, name)})
	}
	for _, event := range channel.events {
		if slices.Contains(app.Events, event) {
			continue
		}
		var keys []string
		for _, key := range channel.handled {
			if prefix, _, _ := strings.Cut(key, "."); prefix == event {
				keys = append(keys, key)
			}
		}
		add = append(add, row{name: event, level: "event", reason: "to receive " + strings.Join(keys, ", ")})
	}
	var remove []row
	for _, name := range sortedKeys(app.Permissions) {
		granted := app.Permissions[name]
		needed, ok := channel.permissions[name]
		switch {
		case !ok:
			remove = append(remove, row{name: name, level: granted, reason: "unused"})
		case exceedsLevel(granted, needed):
			remove = append(remove, row{
				name:   name,
				level:  granted + " → " + needed.String(),
				reason: "the channel needs only " + needed.String(),
			})
		}
	}
	if len(add) == 0 && len(remove) == 0 {
		return "", false
	}

	nameWidth, levelWidth := widths(append(slices.Clone(add), remove...))
	lines := []string{
		fmt.Sprintf("GitHub App %s's permissions do not match this channel.", app.Slug),
		"",
		"GitHub has no API to change an App's permissions, so the App owner makes the changes below at",
		permissionsURL,
	}
	if len(add) > 0 {
		lines = append(lines, "", "Add (required):")
		lines = append(lines, tableLines(add, nameWidth, levelWidth)...)
	}
	if len(remove) > 0 {
		lines = append(lines, "", "Remove or lower (optional: deploy already keeps these out of the agent's token, but the App still holds them):")
		lines = append(lines, tableLines(remove, nameWidth, levelWidth)...)
	}
	return strings.Join(lines, "\n"), true
}

// tokenSummary is the agent's token as deploy leaves it: what it has, from
// its `permission:level` scopes, each with why the channel needs it, then
// what the channel needs that it does not have yet and who grants it where.
// `metadata`, which every App has, is left out.
func tokenSummary(channel *githubChannel, status *hostapi.GitHubChannelStatus, installation hostapi.GitHubInstallation, has []string, renewed bool) string {
	held := map[string]permissionLevel{}
	for _, scope := range has {
		name, level, ok := strings.Cut(scope, ":")
		if !ok {
			continue
		}
		if parsed, ok := parsePermissionLevel(level); ok {
			held[name] = parsed
		}
	}
	makeRow := func(name string, level permissionLevel) row {
		return row{name: name, level: level.String(), reason: why(channel, name)}
	}
	var current []row
	for _, name := range sortedKeys(held) {
		if name != "metadata" {
			current = append(current, makeRow(name, held[name]))
		}
	}
	var waiting []row
	for _, name := range sortedKeys(channel.permissions) {
		needed := channel.permissions[name]
		if level, ok := held[name]; name != "metadata" && (!ok || level < needed) {
			waiting = append(waiting, makeRow(name, needed))
		}
	}

	nameWidth, levelWidth := widths(append(slices.Clone(current), waiting...))
	hasWord := "has"
	if renewed {
		hasWord = "now has"
	}
	var lines []string
	if len(current) == 0 {
		lines = []string{fmt.Sprintf("The agent's GitHub token %s no permissions beyond reading repository metadata.", hasWord)}
	} else {
		lines = append([]string{fmt.Sprintf("The agent's GitHub token %s:", hasWord)}, tableLines(current, nameWidth, levelWidth)...)
	}
	if len(waiting) == 0 {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "", "Waiting for GitHub:")
	lines = append(lines, tableLines(waiting, nameWidth, levelWidth)...)

	// The App must have a permission before an installation can grant it.
	app := status.App
	var appLacks []string
	for _, waited := range waiting {
		granted, ok := app.Permissions[waited.name]
		needed, known := channel.permissions[waited.name]
		if ok && (!known || coversLevel(granted, needed)) {
			continue
		}
		appLacks = append(appLacks, fmt.Sprintf("%s (%s)", waited.name, waited.level))
	}
	var steps []string
	if len(appLacks) > 0 {
		var unsubscribed []string
		for _, event := range channel.events {
			if !slices.Contains(app.Events, event) {
				unsubscribed = append(unsubscribed, event)
			}
		}
		subscribe := ""
		switch len(unsubscribed) {
		case 0:
		case 1:
			subscribe = fmt.Sprintf(" and subscribes to the %s event", unsubscribed[0])
		default:
			subscribe = fmt.Sprintf(" and subscribes to the %s events", andList(unsubscribed))
		}
		steps = append(steps, fmt.Sprintf("The App owner grants %s%s at %s",
			andList(appLacks), subscribe, githubAppPermissionsURL(app.Owner.Login, app.Owner.IsOrganization(), app.Slug)))
	}
	steps = append(steps,
		fmt.Sprintf("An owner of %s accepts them at %s", installation.Account.Login, installationSettingsURL(installation)),
		"Run `langsmith deploy` again.")
	lines = append(lines, "")
	for index, step := range steps {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, step))
	}
	return strings.Join(lines, "\n")
}

// knownPermissions are GitHub App permission names, from the
// `app-permissions` schema of GitHub's REST API. GitHub adds permissions over
// time, so a name missing here is only a warning.
var knownPermissions = []string{
	"actions",
	"administration",
	"attestations",
	"checks",
	"codespaces",
	"contents",
	"dependabot_secrets",
	"deployments",
	"email_addresses",
	"environments",
	"followers",
	"git_ssh_keys",
	"gpg_keys",
	"interaction_limits",
	"issues",
	"members",
	"merge_queues",
	"metadata",
	"organization_administration",
	"organization_announcement_banners",
	"organization_copilot_seat_management",
	"organization_custom_org_roles",
	"organization_custom_properties",
	"organization_custom_roles",
	"organization_events",
	"organization_hooks",
	"organization_packages",
	"organization_personal_access_token_requests",
	"organization_personal_access_tokens",
	"organization_plan",
	"organization_projects",
	"organization_secrets",
	"organization_self_hosted_runners",
	"organization_user_blocking",
	"packages",
	"pages",
	"profile",
	"pull_requests",
	"repository_custom_properties",
	"repository_hooks",
	"repository_projects",
	"secret_scanning_alerts",
	"secrets",
	"security_events",
	"single_file",
	"starring",
	"statuses",
	"team_discussions",
	"vulnerability_alerts",
	"workflows",
}

// similarPermissions are known permissions a mistyped or shortened name
// likely means: those that end in `_{name}` (`projects` is
// `repository_projects`), or are at most two edits away (`pull_request`).
func similarPermissions(name string) []string {
	suffix := "_" + name
	var similar []string
	for _, known := range knownPermissions {
		if strings.HasSuffix(known, suffix) || editDistance(known, name) <= 2 {
			similar = append(similar, known)
		}
	}
	return similar
}

// editDistance is the Levenshtein distance between two ASCII names.
func editDistance(a, b string) int {
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := range len(a) {
		current := []int{i + 1}
		for j := range len(b) {
			substitute := previous[j]
			if a[i] != b[j] {
				substitute++
			}
			current = append(current, min(substitute, previous[j+1]+1, current[j]+1))
		}
		previous = current
	}
	return previous[len(b)]
}

// andList is `a`, `a and b`, `a, b and c`.
func andList(items []string) string {
	return joined(items, "and")
}

// orList is `a`, `a or b`, `a, b or c`.
func orList(items []string) string {
	return joined(items, "or")
}

func joined(items []string, last string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return fmt.Sprintf("%s %s %s", strings.Join(items[:len(items)-1], ", "), last, items[len(items)-1])
	}
}

// why is why the channel needs a permission: the handled events that need
// its level, or `app.permissions`.
func why(channel *githubChannel, name string) string {
	for _, permission := range channel.eventPermissions {
		if permission.name != name {
			continue
		}
		if permission.level == permissionWrite {
			return "to reply to " + strings.Join(permission.events, ", ")
		}
		return "to receive " + strings.Join(permission.events, ", ")
	}
	return "from app.permissions"
}

// row is a permission line in the deploy output: name, level, and why.
type row struct {
	name   string
	level  string
	reason string
}

// widths are the column widths that align every row, in characters.
func widths(rows []row) (nameWidth, levelWidth int) {
	for _, r := range rows {
		nameWidth = max(nameWidth, utf8.RuneCountInString(r.name))
		levelWidth = max(levelWidth, utf8.RuneCountInString(r.level))
	}
	return nameWidth, levelWidth
}

// tableLines are rows as indented, aligned columns.
func tableLines(rows []row, nameWidth, levelWidth int) []string {
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		line := "  " + pad(r.name, nameWidth) + "  " + pad(r.level, levelWidth) + "  " + r.reason
		lines = append(lines, strings.TrimRight(line, " \t\n"))
	}
	return lines
}

// pad pads text with spaces to width characters, as Rust's `{:width$}`.
func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(text)))
}

// missingRepositories are the channel's repositories an installation does
// not cover; empty when it covers all of them. An installation on another
// account covers none. When the listing was truncated, an unlisted
// repository counts as covered; GitHub refuses a token for it if it is not.
func missingRepositories(channel *githubChannel, installation hostapi.GitHubInstallation) []string {
	var missing []string
	for _, repository := range channel.repositories {
		owner, _, hasOwner := strings.Cut(repository, "/")
		onAccount := hasOwner && strings.EqualFold(owner, installation.Account.Login)
		listed := slices.ContainsFunc(installation.Repositories, func(listed string) bool {
			return strings.EqualFold(listed, repository)
		})
		if !onAccount || (installation.RepositorySelection != "all" && !installation.RepositoriesTruncated && !listed) {
			missing = append(missing, repository)
		}
	}
	return missing
}

// grants reports whether granted covers every required permission at its
// level.
func grants(required map[string]permissionLevel, granted map[string]string) bool {
	return len(missingPermissions(required, granted)) == 0
}

// grantedScopes are the required permissions the granted set covers, as
// `permission:level` scopes.
func grantedScopes(required map[string]permissionLevel, granted map[string]string) []string {
	scopes := []string{}
	for _, name := range sortedKeys(required) {
		level := required[name]
		if value, ok := parsePermissionLevel(granted[name]); ok && value >= level {
			scopes = append(scopes, name+":"+level.String())
		}
	}
	return scopes
}

// narrows reports whether going from the `permission:level` scopes from to
// to drops a permission or lowers its level. An empty from means no limit:
// the token then has every permission of the App's installation. An empty to
// still counts as narrowing, so a change never widens a token to no limit.
func narrows(from, to []string) bool {
	if len(from) == 0 {
		return len(to) > 0
	}
	// A level that does not parse is lower than any that does, as Rust's
	// `None < Some(_)`.
	level := func(scope string) (string, int) {
		name, value, _ := strings.Cut(scope, ":")
		parsed, ok := parsePermissionLevel(value)
		if !ok {
			return name, -1
		}
		return name, int(parsed)
	}
	target := map[string]int{}
	for _, scope := range to {
		name, value := level(scope)
		target[name] = value
	}
	for _, scope := range from {
		name, value := level(scope)
		next, ok := target[name]
		if !ok || next < value {
			return true
		}
	}
	return false
}

// missingPermissions are the required permissions the granted set lacks or
// grants at a lower level.
func missingPermissions(required map[string]permissionLevel, granted map[string]string) []string {
	var missing []string
	for _, name := range sortedKeys(required) {
		level := required[name]
		if value, ok := parsePermissionLevel(granted[name]); !ok || value < level {
			missing = append(missing, name+": "+level.String())
		}
	}
	return missing
}

// parseLevels are the App's own permissions as levels; unknown values
// (`admin`) are skipped.
func parseLevels(permissions map[string]string) map[string]permissionLevel {
	levels := map[string]permissionLevel{}
	for name, value := range permissions {
		if level, ok := parsePermissionLevel(value); ok {
			levels[name] = level
		}
	}
	return levels
}

func installationScope(installation hostapi.GitHubInstallation) string {
	if installation.RepositorySelection == "all" {
		return "all repositories of " + installation.Account.Login
	}
	repositories := installation.Repositories
	if len(repositories) == 0 {
		return "no repositories of " + installation.Account.Login
	}
	listed := repositories[:min(len(repositories), listedRepositories)]
	if len(repositories) > listedRepositories || installation.RepositoriesTruncated {
		return strings.Join(listed, ", ") + " and more"
	}
	return strings.Join(listed, ", ")
}

func installationSettingsURL(installation hostapi.GitHubInstallation) string {
	if installation.SettingsURL != "" {
		return installation.SettingsURL
	}
	if installation.Account.IsOrganization() {
		return fmt.Sprintf("https://github.com/organizations/%s/settings/installations/%d", installation.Account.Login, installation.ID)
	}
	return fmt.Sprintf("https://github.com/settings/installations/%d", installation.ID)
}
