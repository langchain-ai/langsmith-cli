package channels

// Read the GitHub channel manifest the runtime emits into
// `channel-manifests.json`.
//
// `channels.github(...)` works out what its App needs, from the events it
// handles and its `app` options, and puts it in `providerConfig`: the handled
// events, the webhook events, the permissions and the repositories. The CLI
// uses it as given. A new App is named after the agent, and the first deploy
// asks where to create it.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
)

const githubProvider = "github"

// githubAppNameMaxChars is GitHub's limit on App names.
const githubAppNameMaxChars = 34

// permissionLevel is a GitHub App permission's level; write covers read.
type permissionLevel int

const (
	permissionRead permissionLevel = iota
	permissionWrite
)

// parsePermissionLevel reads `read` or `write`.
func parsePermissionLevel(value string) (permissionLevel, bool) {
	switch value {
	case "read":
		return permissionRead, true
	case "write":
		return permissionWrite, true
	default:
		return 0, false
	}
}

func (l permissionLevel) String() string {
	if l == permissionWrite {
		return "write"
	}
	return "read"
}

// UnmarshalText reads `read` or `write`.
func (l *permissionLevel) UnmarshalText(text []byte) error {
	level, ok := parsePermissionLevel(string(text))
	if !ok {
		return fmt.Errorf("unknown variant `%s`, expected `read` or `write`", text)
	}
	*l = level
	return nil
}

// githubChannel is a validated GitHub channel, ready for App setup.
type githubChannel struct {
	// name is the channel name (file stem), also the manifest key.
	name string
	// eventsPath is the events route on the Agent Server, e.g.
	// `/channels/github/events`.
	eventsPath string
	// handled are the handled event keys, e.g. `issues.opened`. Empty for a
	// receipt-only channel.
	handled []string
	// events are the App webhook events, sorted and deduplicated.
	events []string
	// permissions are the App permissions, always including `metadata: read`.
	permissions map[string]permissionLevel
	// appName is the name of a new App: the agent's, cut to GitHub's limit.
	appName string
	// repositories are those the channel is limited to, as `owner/name`,
	// sorted. All share one owner. Empty means every repository of the
	// installation.
	repositories []string
	// eventPermissions are the permissions whose level the handled events
	// set, by name; `metadata`, which every App has, is left out.
	eventPermissions []eventPermission
}

// eventPermission is a token permission at the level handled events need:
// read to receive them, write to also reply where they happened.
type eventPermission struct {
	name  string
	level permissionLevel
	// events are the handled event keys that need this level, sorted.
	events []string
}

type githubProviderConfig struct {
	Events        []string        `json:"events" strict:"required"`
	WebhookEvents []string        `json:"webhookEvents" strict:"required"`
	Permissions   []appPermission `json:"permissions" strict:"required"`
	Repositories  []string        `json:"repositories"`
}

// UnmarshalJSON decodes strictly.
func (c *githubProviderConfig) UnmarshalJSON(data []byte) error {
	type plain githubProviderConfig
	return decodeStrict(data, (*plain)(c), true)
}

// appPermission is a permission the App asks for, with the handled events
// that need it; no events when the level comes from `app.permissions` or is
// `metadata`.
type appPermission struct {
	Name   string          `json:"name" strict:"required"`
	Level  permissionLevel `json:"level" strict:"required"`
	Events []string        `json:"events" strict:"required"`
}

// UnmarshalJSON decodes strictly.
func (p *appPermission) UnmarshalJSON(data []byte) error {
	type plain appPermission
	return decodeStrict(data, (*plain)(p), true)
}

// prepareGitHub finds the GitHub channel, if the project declares one.
//
// wait is false for `langsmith deploy --no-wait`: App setup needs the live
// Agent Server URL, so that combination is rejected before anything is
// uploaded.
func prepareGitHub(requirements introspect.ChannelRequirements, wait bool, assistantID string) (*githubChannel, error) {
	manifest, err := findAppChannelManifest[githubProviderConfig](requirements, githubProvider, "GitHub")
	if err != nil || manifest == nil {
		return nil, err
	}
	if manifest.AutoReply {
		return nil, errors.New("GitHub channels do not reply automatically; give the agent a GitHub MCP server to act on GitHub.")
	}

	if !wait {
		return nil, errors.New("An agent with a GitHub channel cannot be deployed with --no-wait.")
	}

	config := manifest.ProviderConfig
	var eventPermissions []eventPermission
	permissions := map[string]permissionLevel{}
	for _, permission := range config.Permissions {
		if len(permission.Events) > 0 {
			eventPermissions = append(eventPermissions, eventPermission{
				name:   permission.Name,
				level:  permission.Level,
				events: permission.Events,
			})
		}
		permissions[permission.Name] = permission.Level
	}
	return &githubChannel{
		name:             manifest.Name,
		eventsPath:       manifest.EventsPath,
		handled:          config.Events,
		events:           config.WebhookEvents,
		permissions:      permissions,
		appName:          defaultGitHubAppName(assistantID),
		repositories:     config.Repositories,
		eventPermissions: eventPermissions,
	}, nil
}

func defaultGitHubAppName(assistantID string) string {
	runes := []rune(assistantID)
	if len(runes) > githubAppNameMaxChars {
		runes = runes[:githubAppNameMaxChars]
	}
	return strings.TrimRight(string(runes), "-_ ")
}
