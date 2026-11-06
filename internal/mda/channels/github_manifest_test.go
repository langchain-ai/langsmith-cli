package channels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
)

// githubManifest is what `channels.github(...)` emits for issue, comment and
// workflow handlers with `app: { permissions: { contents: "read" } }`.
func githubManifest(t *testing.T, name string) map[string]any {
	manifest := jsonObject(t, `{
		"provider": "github",
		"autoReply": false,
		"requiredPermissions": [],
		"requiredEnv": [],
		"providerConfig": {
			"events": ["issue_comment.created", "issues.opened", "workflow_run.completed"],
			"webhookEvents": ["issue_comment", "issues", "workflow_run"],
			"permissions": [
				{ "name": "actions", "level": "read", "events": ["workflow_run.completed"] },
				{ "name": "contents", "level": "read", "events": [] },
				{ "name": "issues", "level": "write", "events": ["issue_comment.created", "issues.opened"] },
				{ "name": "metadata", "level": "read", "events": [] }
			]
		}
	}`)
	manifest["name"] = name
	manifest["eventsPath"] = "/channels/" + name + "/events"
	return manifest
}

func TestGitHubPages(t *testing.T) {
	assert.Equal(t, "https://github.com/apps/acme-triage/installations/new", githubInstallURL("acme-triage"))
	assert.Equal(t, "https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
		githubAppPermissionsURL("acme", true, "acme-triage"))
	assert.Equal(t, "https://github.com/settings/apps/acme-triage/permissions",
		githubAppPermissionsURL("alice", false, "acme-triage"))
}

func TestReturnsNoneWithoutAGitHubChannel(t *testing.T) {
	slack := jsonObject(t, `{ "name": "slack", "provider": "slack", "transport": "trigger_server" }`)

	channel, err := prepareGitHub(introspect.ChannelRequirements{"slack": slack}, false, "agent")

	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestUsesTheGitHubAppSetupTheRuntimeEmits(t *testing.T) {
	slack := jsonObject(t, `{ "name": "slack", "provider": "slack", "transport": "trigger_server" }`)
	github := githubManifest(t, "github")
	github["providerConfig"].(map[string]any)["repositories"] = []any{"acme/api", "acme/web"}

	channel, err := prepareGitHub(introspect.ChannelRequirements{"github": github, "slack": slack}, true,
		"a-very-long-assistant-name-that-exceeds-the-limit")

	require.NoError(t, err)
	assert.Equal(t, &githubChannel{
		name:       "github",
		eventsPath: "/channels/github/events",
		handled:    []string{"issue_comment.created", "issues.opened", "workflow_run.completed"},
		events:     []string{"issue_comment", "issues", "workflow_run"},
		permissions: map[string]permissionLevel{
			"actions":  permissionRead,
			"contents": permissionRead,
			"issues":   permissionWrite,
			"metadata": permissionRead,
		},
		appName:      "a-very-long-assistant-name-that-ex",
		repositories: []string{"acme/api", "acme/web"},
		eventPermissions: []eventPermission{
			{name: "actions", level: permissionRead, events: []string{"workflow_run.completed"}},
			{name: "issues", level: permissionWrite, events: []string{"issue_comment.created", "issues.opened"}},
		},
	}, channel)
}

func TestRejectsTwoGitHubChannels(t *testing.T) {
	_, err := prepareGitHub(introspect.ChannelRequirements{
		"github": githubManifest(t, "github"),
		"gh":     githubManifest(t, "gh"),
	}, true, "agent")

	require.EqualError(t, err, "An agent can have only one GitHub channel (found: gh, github).")
}

func TestRejectsNoWait(t *testing.T) {
	_, err := prepareGitHub(introspect.ChannelRequirements{"github": githubManifest(t, "github")}, false, "agent")

	require.EqualError(t, err, "An agent with a GitHub channel cannot be deployed with --no-wait.")
}

func TestRejectsInvalidGitHubConfiguration(t *testing.T) {
	for _, tc := range []struct {
		label    string
		mutate   func(map[string]any)
		expected string
	}{
		{"unknown field", func(v map[string]any) { v["extra"] = 1 }, "configuration is invalid"},
		{"unknown provider config", func(v map[string]any) { v["providerConfig"].(map[string]any)["app"] = map[string]any{} }, "configuration is invalid"},
		{"permission level", func(v map[string]any) {
			v["providerConfig"].(map[string]any)["permissions"].([]any)[0].(map[string]any)["level"] = "admin"
		}, "configuration is invalid"},
		{"missing events", func(v map[string]any) { delete(v["providerConfig"].(map[string]any), "events") }, "missing field `events`"},
		{"null webhook events", func(v map[string]any) { v["providerConfig"].(map[string]any)["webhookEvents"] = nil }, "configuration is invalid"},
		{"transport", func(v map[string]any) { v["transport"] = "trigger_server" }, "must not set a transport"},
		{"auto reply", func(v map[string]any) { v["autoReply"] = true }, "do not reply automatically"},
		{"env", func(v map[string]any) { v["requiredEnv"] = []any{"X"} }, "requiredEnv"},
		{"events path", func(v map[string]any) { v["eventsPath"] = "https://x/events" }, "absolute path"},
		{"name mismatch", func(v map[string]any) { v["name"] = "other" }, "must match its manifest key"},
	} {
		value := githubManifest(t, "github")
		tc.mutate(value)

		_, err := prepareGitHub(introspect.ChannelRequirements{"github": value}, true, "agent")

		require.ErrorContains(t, err, tc.expected, tc.label)
	}
}
