package channels

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestEveryChannelsFailureIsReported(t *testing.T) {
	require.NoError(t, combineFailures(nil))

	one := combineFailures([]channelFailure{{channel: "Slack", err: errors.New("no app")}})
	require.EqualError(t, one, "no app")

	var failures []channelFailure
	failures = collectFailure(failures, "Slack", errors.New("no app"))
	failures = collectFailure(failures, "GitHub", nil)
	failures = collectFailure(failures, "Linear", errors.New("no secret"))
	require.EqualError(t, combineFailures(failures),
		"The deployment is live, but these channels did not finish setting up:\nSlack: no app\nLinear: no secret")
}

func TestOnlyDirectHTTPChannelsListTheirEventsURL(t *testing.T) {
	assert.True(t, ListsEventsURL(map[string]any{"provider": "slack", "eventsPath": "/channels/slack/events"}))
	assert.False(t, ListsEventsURL(map[string]any{"provider": "slack", "transport": "trigger_server"}))
	assert.False(t, ListsEventsURL(map[string]any{"provider": "github"}))
	assert.False(t, ListsEventsURL(map[string]any{"provider": "linear"}))
}

func allChannels(t *testing.T) introspect.ChannelRequirements {
	requirements := slackRequirements(t)
	requirements["github"] = githubManifest(t, "github")
	requirements["linear"] = linearManifest(t, true)
	return requirements
}

func TestPrepareFindsEveryChannelThatNeedsSetup(t *testing.T) {
	setup, err := Prepare(allChannels(t), true, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)
	assert.True(t, setup.NeedsLiveURL())
	assert.Equal(t, "slack", setup.trigger.name)
	assert.Equal(t, "github", setup.github.name)
	assert.Equal(t, "linear", setup.linear.name)

	none, err := Prepare(introspect.ChannelRequirements{}, false, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)
	assert.False(t, none.NeedsLiveURL())

	_, err = Prepare(allChannels(t), false, ".", nil, project.TypeScript, "triage-agent")
	require.EqualError(t, err, "An agent with a Slack channel cannot be deployed with --no-wait.")
}

func TestPreflightLooksUpTheDeploymentOnlyWhenAChannelNeedsIt(t *testing.T) {
	none, err := Prepare(introspect.ChannelRequirements{}, true, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)
	require.NoError(t, none.Preflight(t.Context(), nil, func(context.Context) (map[string]any, error) {
		t.Fatal("the deployment was looked up for no channel")
		return nil, nil
	}, map[string]string{}))

	setup, err := Prepare(allChannels(t), true, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)
	lookupFailed := errors.New("deployment lookup failed")
	err = setup.Preflight(t.Context(), nil, func(context.Context) (map[string]any, error) {
		return nil, lookupFailed
	}, map[string]string{})
	require.ErrorIs(t, err, lookupFailed)
}

func TestUnattendedPreflightChecksEachChannelForTheExistingDeployment(t *testing.T) {
	unattended(t)
	// Trigger Server has the Slack channel; nothing else is configured, so
	// GitHub, which needs an owner for its App, stops the deploy.
	trigger := serveSequence(t,
		reply{http.StatusOK, `{"channels":[{"provider":"slack","channel_name":"slack"}]}`},
		reply{http.StatusOK, `[]`})
	setup, err := Prepare(allChannels(t), true, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)

	err = setup.Preflight(t.Context(), oauthClient(t, trigger.URL), func(context.Context) (map[string]any, error) {
		return map[string]any{"id": "dep-1"}, nil
	}, map[string]string{})

	require.EqualError(t, err, githubOwnerRequired)
	assert.Equal(t, []string{
		"GET /v1/channels?deployment_id=dep-1",
		"GET /v1/agent-auth/apps?owner_id=dep-1",
	}, trigger.lines())
}

func TestEveryChannelHasItsTurnWhenSetupFails(t *testing.T) {
	unattended(t)
	setup, err := Prepare(allChannels(t), true, ".", nil, project.TypeScript, "triage-agent")
	require.NoError(t, err)

	err = setup.SetUp(t.Context(), SetupTarget{
		Client:         oauthClient(t, "http://unused.invalid"),
		AgentServerURL: "http://agent.example",
		DeploymentID:   "dep-1",
		AssistantID:    "triage-agent",
		ProjectRoot:    ".",
		EnvVars:        map[string]string{},
	})

	const invalid = "Agent Server URL must be absolute HTTPS without credentials, query, or fragment"
	require.EqualError(t, err, "The deployment is live, but these channels did not finish setting up:\n"+
		"Slack: "+invalid+"\nGitHub: "+invalid+"\nLinear: "+invalid)
}
