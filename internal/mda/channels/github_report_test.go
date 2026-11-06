package channels

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

func reportChannel() *githubChannel {
	return &githubChannel{
		name:        "github",
		eventsPath:  "/channels/github/events",
		handled:     []string{"issue_comment.created", "issues.opened"},
		events:      []string{"issue_comment", "issues"},
		permissions: map[string]permissionLevel{"issues": permissionWrite, "metadata": permissionRead},
		appName:     "acme-triage",
		eventPermissions: []eventPermission{
			{name: "issues", level: permissionWrite, events: []string{"issue_comment.created", "issues.opened"}},
		},
	}
}

func channelStatus(t *testing.T, value map[string]any) *hostapi.GitHubChannelStatus {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	var status hostapi.GitHubChannelStatus
	require.NoError(t, json.Unmarshal(data, &status))
	return &status
}

func healthy(t *testing.T) map[string]any {
	return jsonObject(t, `{
		"app": {
			"id": 1, "slug": "acme-triage",
			"owner": { "login": "acme", "type": "Organization" },
			"permissions": { "issues": "write", "metadata": "read" },
			"events": ["issue_comment", "issues"]
		},
		"installations": [{
			"id": 9, "account": { "login": "acme", "type": "Organization" },
			"repository_selection": "selected", "repositories": ["acme/api", "acme/web"],
			"permissions": { "issues": "write", "metadata": "read" }
		}]
	}`)
}

func appOf(value map[string]any) map[string]any {
	return value["app"].(map[string]any)
}

func installationOf(value map[string]any) map[string]any {
	return value["installations"].([]any)[0].(map[string]any)
}

func TestNarrowingIsADroppedPermissionOrALowerLevel(t *testing.T) {
	current := []string{"contents:write", "issues:read"}
	assert.False(t, narrows(current, current))
	assert.False(t, narrows(current, []string{"contents:write", "issues:write"}))
	assert.False(t, narrows(current, []string{"contents:write", "issues:read", "pull_requests:read"}))
	assert.True(t, narrows(current, []string{"issues:read"}))
	assert.True(t, narrows(current, []string{"contents:read", "issues:read"}))
	// A token without scopes has all of the App's permissions.
	assert.True(t, narrows(nil, current))
	assert.False(t, narrows(nil, nil))
	assert.True(t, narrows(current, nil))
}

func TestGrantedScopesKeepWhatTheInstallationCovers(t *testing.T) {
	required := map[string]permissionLevel{"issues": permissionWrite, "metadata": permissionRead, "pull_requests": permissionWrite}
	granted := map[string]string{"issues": "write", "metadata": "read", "pull_requests": "read"}

	assert.Equal(t, []string{"issues:write", "metadata:read"}, grantedScopes(required, granted))
}

func TestHealthyAppReportsReady(t *testing.T) {
	report := buildGitHubReport(reportChannel(), channelStatus(t, healthy(t)))

	assert.Equal(t, githubReport{
		ready: "GitHub channel ready · App acme-triage (acme) · installed on acme/api, acme/web",
	}, report)
}

func TestMissingPermissionsAndEventsNameTheAppOwnerAndLink(t *testing.T) {
	value := healthy(t)
	appOf(value)["permissions"] = map[string]any{"issues": "read", "metadata": "read"}
	appOf(value)["events"] = []any{"issues"}

	report := buildGitHubReport(reportChannel(), channelStatus(t, value))

	assert.Equal(t, []string{strings.Join([]string{
		"GitHub App acme-triage's permissions do not match this channel.",
		"",
		"GitHub has no API to change an App's permissions, so the App owner makes the changes below at",
		"https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
		"",
		"Add (required):",
		"  issues         read → write  to reply to issue_comment.created, issues.opened",
		"  issue_comment  event         to receive issue_comment.created",
	}, "\n")}, report.warnings)
	assert.NotEmpty(t, report.ready)
}

func TestUnacceptedPermissionsAskTheInstallationOwner(t *testing.T) {
	value := healthy(t)
	appOf(value)["permissions"] = map[string]any{"issues": "write", "metadata": "read", "checks": "read"}

	report := buildGitHubReport(reportChannel(), channelStatus(t, value))

	assert.Equal(t, []string{
		strings.Join([]string{
			"GitHub App acme-triage's permissions do not match this channel.",
			"",
			"GitHub has no API to change an App's permissions, so the App owner makes the changes below at",
			"https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
			"",
			"Remove or lower (optional: deploy already keeps these out of the agent's token, but the App still holds them):",
			"  checks  read  unused",
		}, "\n"),
		"An owner of acme must accept the App's updated permissions (checks: read) at https://github.com/organizations/acme/settings/installations/9",
	}, report.warnings)
	assert.Empty(t, report.notes)
}

func TestPermissionsAboveWhatTheChannelNeedsAreAWarning(t *testing.T) {
	value := healthy(t)
	appOf(value)["permissions"] = map[string]any{"contents": "write", "issues": "write", "metadata": "read", "pull_requests": "admin"}
	narrowed := reportChannel()
	narrowed.permissions["contents"] = permissionRead
	narrowed.permissions["pull_requests"] = permissionWrite

	report := buildGitHubReport(narrowed, channelStatus(t, value))

	assert.Equal(t, strings.Join([]string{
		"GitHub App acme-triage's permissions do not match this channel.",
		"",
		"GitHub has no API to change an App's permissions, so the App owner makes the changes below at",
		"https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
		"",
		"Remove or lower (optional: deploy already keeps these out of the agent's token, but the App still holds them):",
		"  contents       write → read   the channel needs only read",
		"  pull_requests  admin → write  the channel needs only write",
	}, "\n"), report.warnings[0])
}

func TestTokenSummarySaysWhyAndWhatIsStillWaiting(t *testing.T) {
	explained := reportChannel()
	explained.permissions["contents"] = permissionRead
	explained.permissions["projects"] = permissionWrite
	explained.permissions["pull_requests"] = permissionWrite
	explained.eventPermissions = append(explained.eventPermissions, eventPermission{
		name: "pull_requests", level: permissionWrite, events: []string{"pull_request_review.submitted"},
	})
	reported := channelStatus(t, healthy(t))
	installation := reported.Installations[0]

	// The App lacks projects, so its owner adds it before the installation
	// can accept it.
	assert.Equal(t, strings.Join([]string{
		"The agent's GitHub token now has:",
		"  contents       read   from app.permissions",
		"  issues         write  to reply to issue_comment.created, issues.opened",
		"  pull_requests  write  to reply to pull_request_review.submitted",
		"",
		"Waiting for GitHub:",
		"  projects       write  from app.permissions",
		"",
		"1. The App owner grants projects (write) at https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
		"2. An owner of acme accepts them at https://github.com/organizations/acme/settings/installations/9",
		"3. Run `langsmith deploy` again.",
	}, "\n"), tokenSummary(explained, reported, installation,
		[]string{"contents:read", "issues:write", "metadata:read", "pull_requests:write"}, true))

	// The App is not subscribed to every event either: step 1 says so.
	unsubscribedValue := healthy(t)
	appOf(unsubscribedValue)["events"] = []any{}
	unsubscribed := channelStatus(t, unsubscribedValue)
	summary := tokenSummary(explained, unsubscribed, unsubscribed.Installations[0], []string{"issues:write", "metadata:read"}, false)
	assert.Contains(t, summary, "\n1. The App owner grants contents (read), projects (write) and pull_requests (write) and subscribes to the issue_comment and issues events at ")

	// The App has issues: write; only the installation has to accept it. A
	// lower level than the channel needs is still waiting.
	acceptOnly := []string{
		"",
		"Waiting for GitHub:",
		"  issues  write  to reply to issue_comment.created, issues.opened",
		"",
		"1. An owner of acme accepts them at https://github.com/organizations/acme/settings/installations/9",
		"2. Run `langsmith deploy` again.",
	}
	assert.Equal(t, strings.Join(append([]string{
		"The agent's GitHub token has:",
		"  issues  read   to reply to issue_comment.created, issues.opened",
	}, acceptOnly...), "\n"), tokenSummary(reportChannel(), reported, installation, []string{"issues:read", "metadata:read"}, false))

	assert.Equal(t, strings.Join(append([]string{
		"The agent's GitHub token has no permissions beyond reading repository metadata.",
	}, acceptOnly...), "\n"), tokenSummary(reportChannel(), reported, installation, []string{"metadata:read"}, false))
}

func TestUnknownPermissionNamesGetAHint(t *testing.T) {
	misnamed := reportChannel()
	for _, name := range []string{"projects", "pull_request", "zzz"} {
		misnamed.permissions[name] = permissionWrite
	}

	report := buildGitHubReport(misnamed, channelStatus(t, healthy(t)))

	assert.Equal(t, []string{
		"app.permissions names projects, which GitHub does not list as an App permission. Did you mean organization_projects or repository_projects?",
		"app.permissions names pull_request, which GitHub does not list as an App permission. Did you mean pull_requests?",
		"app.permissions names zzz, which GitHub does not list as an App permission.",
	}, report.warnings[:3])
	assert.Empty(t, buildGitHubReport(reportChannel(), channelStatus(t, healthy(t))).warnings)
}

func TestNotInstalledLinksTheInstallPage(t *testing.T) {
	value := healthy(t)
	value["installations"] = []any{}
	value["installation_requests"] = jsonValue(t, `[{ "account": { "login": "acme" }, "requester": { "login": "alice" } }]`)

	report := buildGitHubReport(reportChannel(), channelStatus(t, value))

	assert.Empty(t, report.ready)
	assert.Equal(t, []string{
		"Waiting for an owner of acme to approve the installation requested by alice.",
		"GitHub App acme-triage is not installed yet. Install it on your repositories at https://github.com/apps/acme-triage/installations/new",
	}, report.warnings)
}

func TestRepositoriesOutsideTheInstallationAreReported(t *testing.T) {
	limited := reportChannel()
	limited.repositories = []string{"acme/api", "acme/docs"}
	assert.Equal(t, []string{
		"The installation on acme does not include acme/docs, which this channel is limited to. Add them at https://github.com/organizations/acme/settings/installations/9",
	}, buildGitHubReport(limited, channelStatus(t, healthy(t))).warnings)

	value := healthy(t)
	installationOf(value)["repository_selection"] = "all"
	assert.Empty(t, buildGitHubReport(limited, channelStatus(t, value)).warnings)

	value = healthy(t)
	installationOf(value)["repositories_truncated"] = true
	assert.Empty(t, buildGitHubReport(limited, channelStatus(t, value)).warnings)

	limited.repositories = []string{"other/api"}
	assert.Contains(t, buildGitHubReport(limited, channelStatus(t, healthy(t))).warnings[0], "other/api")
}

func TestSuspendedInstallationsAreNotReady(t *testing.T) {
	value := healthy(t)
	installationOf(value)["suspended_at"] = "2026-10-01T00:00:00Z"

	report := buildGitHubReport(reportChannel(), channelStatus(t, value))

	assert.Empty(t, report.ready)
	assert.Equal(t, []string{
		"The installation on acme is suspended, so it receives no events. Unsuspend it at https://github.com/organizations/acme/settings/installations/9",
	}, report.warnings)
}

func TestInstallationScopeWording(t *testing.T) {
	value := healthy(t)
	installationOf(value)["repository_selection"] = "all"
	assert.Equal(t, "GitHub channel ready · App acme-triage (acme) · installed on all repositories of acme",
		buildGitHubReport(reportChannel(), channelStatus(t, value)).ready)

	value = healthy(t)
	installationOf(value)["repositories"] = []any{"a/1", "a/2", "a/3", "a/4", "a/5", "a/6"}
	assert.True(t, strings.HasSuffix(buildGitHubReport(reportChannel(), channelStatus(t, value)).ready,
		"installed on a/1, a/2, a/3, a/4, a/5 and more"))

	value = healthy(t)
	appOf(value)["owner"] = map[string]any{"login": "alice", "type": "User"}
	installationOf(value)["account"] = map[string]any{"login": "alice", "type": "User"}
	installationOf(value)["suspended_at"] = "x"
	assert.True(t, strings.HasSuffix(buildGitHubReport(reportChannel(), channelStatus(t, value)).warnings[0],
		"https://github.com/settings/installations/9"))
}
