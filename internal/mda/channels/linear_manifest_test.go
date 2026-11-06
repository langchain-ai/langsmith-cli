package channels

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
)

// linearProviderConfigJSON is the `providerConfig` `channels.linear(...)`
// emits with sessions on or off and no handlers of its own.
func linearProviderConfigJSON(t *testing.T, sessions bool) map[string]any {
	if !sessions {
		return jsonObject(t, `{
			"events": [],
			"sessions": false,
			"webhookResourceTypes": ["OAuthAuthorization"],
			"scopes": ["read", "write"],
			"allowedScopes": ["read", "write", "customer:read", "customer:write", "initiative:read", "initiative:write"]
		}`)
	}
	return jsonObject(t, `{
		"events": ["AgentSessionEvent.created", "AgentSessionEvent.prompted"],
		"sessions": true,
		"webhookResourceTypes": ["AgentSessionEvent", "OAuthAuthorization", "PermissionChange"],
		"scopes": ["read", "write", "app:mentionable", "app:assignable"],
		"allowedScopes": [
			"read", "write", "app:mentionable", "app:assignable",
			"customer:read", "customer:write", "initiative:read", "initiative:write"
		]
	}`)
}

func linearManifest(t *testing.T, sessions bool) map[string]any {
	manifest := jsonObject(t, `{
		"name": "linear",
		"provider": "linear",
		"autoReply": true,
		"eventsPath": "/channels/linear/events",
		"requiredPermissions": [],
		"requiredEnv": []
	}`)
	manifest["providerConfig"] = linearProviderConfigJSON(t, sessions)
	return manifest
}

func linearRequirements(manifest map[string]any) introspect.ChannelRequirements {
	return introspect.ChannelRequirements{"linear": manifest}
}

func TestUsesTheLinearAppSetupTheRuntimeEmits(t *testing.T) {
	channel, err := prepareLinear(linearRequirements(linearManifest(t, true)), true, "triage-agent", ".", nil)
	require.NoError(t, err)
	assert.True(t, channel.sessions)
	assert.Equal(t, []string{"AgentSessionEvent.created", "AgentSessionEvent.prompted"}, channel.handled)
	assert.Equal(t, []string{"AgentSessionEvent", "OAuthAuthorization", "PermissionChange"}, channel.resourceTypes)
	assert.Equal(t, []string{"read", "write", "app:mentionable", "app:assignable"}, channel.scopes)
	assert.Contains(t, channel.allowedScopes, "initiative:write")
	assert.Equal(t, "triage-agent", channel.appName)
	assert.Equal(t, defaultAppDescription, channel.appDescription)
	assert.Empty(t, channel.appIcon)

	branded := linearManifest(t, false)
	branded["providerConfig"].(map[string]any)["app"] = map[string]any{"name": "Triage Bot", "description": "Triages new issues."}
	channel, err = prepareLinear(linearRequirements(branded), true, "a", ".", nil)
	require.NoError(t, err)
	assert.False(t, channel.sessions)
	assert.Equal(t, "Triage Bot", channel.appName)
	assert.Equal(t, "Triages new issues.", channel.appDescription)

	// Linear refuses app names that are too short or mention Linear.
	for _, assistantID := range []string{"a", "my-linear-agent"} {
		channel, err = prepareLinear(linearRequirements(linearManifest(t, false)), true, assistantID, ".", nil)
		require.NoError(t, err)
		assert.Equal(t, "Deep Agent", channel.appName, assistantID)
	}
}

func TestServesAnIconFileInsideChannels(t *testing.T) {
	// Like the Slack channel's icon, a path relative to channels/.
	project := t.TempDir()
	channels := filepath.Join(project, "channels")
	require.NoError(t, os.MkdirAll(filepath.Join(channels, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(channels, "assets", "icon.png"), []byte("\x89PNG\r\n\x1a\nrest"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(project, "top.png"), []byte("\x89PNG\r\n\x1a\nrest"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(channels, "fake.png"), []byte("<svg/>"), 0o644))
	big := make([]byte, linearIconMaxBytes+1)
	for i := range big {
		big[i] = 0xff
	}
	require.NoError(t, os.WriteFile(filepath.Join(channels, "big.jpg"), big, 0o644))
	// On disk, but in a folder the build leaves out.
	require.NoError(t, os.MkdirAll(filepath.Join(channels, "build"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(channels, "build", "icon.png"), []byte("\x89PNG\r\n\x1a\nrest"), 0o644))
	// What the project loader carries into the build: regular files outside
	// ignored folders.
	built := []string{
		filepath.Join("channels", "assets", "icon.png"),
		filepath.Join("channels", "fake.png"),
		filepath.Join("channels", "big.jpg"),
		"top.png",
	}
	withIcon := func(icon string) (*linearChannel, error) {
		iconed := linearManifest(t, false)
		iconed["providerConfig"].(map[string]any)["app"] = map[string]any{"icon": icon}
		return prepareLinear(linearRequirements(iconed), true, "a", project, built)
	}

	channel, err := withIcon(" assets/icon.png ")
	require.NoError(t, err)
	assert.Equal(t, "assets/icon.png", channel.appIcon)
	for icon, expected := range map[string]string{
		"missing.png": "Linear channel app.icon missing.png does not exist.",
		// At the project's root, not under channels/.
		"top.png":           "Linear channel app.icon top.png does not exist.",
		"fake.png":          "Linear channel app.icon fake.png is not the image its extension names.",
		"big.jpg":           "Linear channel app.icon must be at most 1 MB",
		"../icon.png":       "Linear channel app.icon must be a path relative to channels/",
		"/etc/icon.png":     "Linear channel app.icon must be a path relative to channels/",
		"./assets/icon.png": "Linear channel app.icon must be a path relative to channels/",
		"assets//icon.png":  "Linear channel app.icon must be a path relative to channels/",
		`assets\icon.png`:   "Linear channel app.icon must be a path relative to channels/",
		"icon.svg":          "Linear channel app.icon must be a PNG or JPEG image, such as assets/icon.png.",
	} {
		_, err := withIcon(icon)
		require.EqualError(t, err, expected, icon)
	}
	// The deployment serves the icon from the build, so one the build doesn't
	// carry fails here rather than as a broken link in Linear.
	_, err = withIcon("build/icon.png")
	require.ErrorContains(t, err, "isn't part of the build")
}

func TestRejectsWhatLinearSetupCannotHonor(t *testing.T) {
	_, err := prepareLinear(linearRequirements(linearManifest(t, true)), false, "a", ".", nil)
	require.EqualError(t, err, "An agent with a Linear channel cannot be deployed with --no-wait.")

	unknown := linearManifest(t, true)
	// Like Slack's and GitHub's, the app takes no icon URL.
	unknown["providerConfig"].(map[string]any)["app"] = map[string]any{"iconUrl": "https://acme.dev/icon.png"}
	_, err = prepareLinear(linearRequirements(unknown), true, "a", ".", nil)
	require.ErrorContains(t, err, "Linear channel configuration is invalid.: ")

	transport := linearManifest(t, true)
	transport["transport"] = "trigger_server"
	_, err = prepareLinear(linearRequirements(transport), true, "a", ".", nil)
	require.EqualError(t, err, "Linear channels receive events directly and must not set a transport.")

	two := introspect.ChannelRequirements{"a": linearManifest(t, true), "b": linearManifest(t, true)}
	_, err = prepareLinear(two, true, "a", ".", nil)
	require.EqualError(t, err, "An agent can have only one Linear channel (found: a, b).")

	none, err := prepareLinear(introspect.ChannelRequirements{}, true, "a", ".", nil)
	require.NoError(t, err)
	assert.Nil(t, none)
}
