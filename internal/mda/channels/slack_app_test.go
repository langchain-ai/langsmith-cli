package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

type fakeChoice struct {
	isInteractive    bool
	isSelfHosted     bool
	envCredentials   *hostapi.SlackCustomerApp
	slackManager     *hostapi.SlackManager
	answer           *slackAppSource
	offered          []slackAppOption
	createPage       string
	existingSettings string
	askedCredentials bool
	askedManager     bool
	notes            []string
	t                *testing.T
}

func slackCredentials() hostapi.SlackCustomerApp {
	return hostapi.SlackCustomerApp{BotToken: "xoxb-test", SigningSecret: "signing-test"}
}

func slackTestManifest(t *testing.T) map[string]any {
	return jsonObject(t, `{
		"display_information": {"name": "Support"},
		"oauth_config": {"scopes": {"bot": ["chat:write", "channels:history"]}},
		"settings": {
			"event_subscriptions": {
				"request_url": "https://triggers.example/v1/triggers/webhooks/template",
				"bot_events": ["app_mention", "message.im"]
			},
			"interactivity": {
				"is_enabled": true,
				"request_url": "https://triggers.example/v1/triggers/webhooks/slack-custom-app/interactivity"
			}
		}
	}`)
}

func (f *fakeChoice) interactive() bool { return f.isInteractive }

func (f *fakeChoice) selfHosted() bool { return f.isSelfHosted }

func (f *fakeChoice) credentialsFromEnv() (*hostapi.SlackCustomerApp, error) {
	return f.envCredentials, nil
}

func (f *fakeChoice) manager(context.Context) *hostapi.SlackManager {
	f.askedManager = true
	return f.slackManager
}

func (f *fakeChoice) choose(_ context.Context, options []slackAppOption) (slackAppSource, error) {
	f.offered = options
	if f.answer == nil {
		return 0, errors.New("no answer")
	}
	return *f.answer, nil
}

func (f *fakeChoice) info(message string) error {
	f.notes = append(f.notes, message)
	return nil
}

func (f *fakeChoice) manifest(context.Context) (map[string]any, error) {
	return slackTestManifest(f.t), nil
}

func (f *fakeChoice) showCreatePage(link *url.URL) error {
	f.createPage = link.String()
	return nil
}

func (f *fakeChoice) showExistingSettings(settings string) error {
	f.existingSettings = settings
	return nil
}

func (f *fakeChoice) askCredentials(context.Context) (hostapi.SlackCustomerApp, error) {
	f.askedCredentials = true
	return slackCredentials(), nil
}

func sources(options []slackAppOption) []slackAppSource {
	var listed []slackAppSource
	for _, option := range options {
		listed = append(listed, option.source)
	}
	return listed
}

func answer(source slackAppSource) *slackAppSource {
	return &source
}

func TestEnvCredentialsNeedNoQuestion(t *testing.T) {
	credentials := slackCredentials()
	env := &fakeChoice{t: t, isInteractive: true, envCredentials: &credentials}

	app, err := chooseSlackApp(t.Context(), env)

	require.NoError(t, err)
	require.NotNil(t, app.customer)
	assert.Equal(t, "xoxb-test", app.customer.BotToken)
	assert.Empty(t, env.offered)
	assert.False(t, env.askedManager)
}

func TestUnattendedHostedDeployCreatesAManagedAppInAnAuthorizedWorkspace(t *testing.T) {
	for _, manager := range []*hostapi.SlackManager{{Kind: hostapi.SlackManagerAuthorized, TeamName: "Acme"}, nil} {
		env := &fakeChoice{t: t, slackManager: manager}

		app, err := chooseSlackApp(t.Context(), env)

		require.NoError(t, err)
		assert.Nil(t, app.customer)
		assert.True(t, env.askedManager)
		// Not silently: it says so, and how to use your own app.
		require.Len(t, env.notes, 1)
		assert.Contains(t, env.notes[0], SlackBotTokenEnv)
	}
}

func TestUnattendedHostedDeployThatNeedsAuthorizationFailsWithTheEnvHint(t *testing.T) {
	for _, kind := range []hostapi.SlackManagerKind{hostapi.SlackManagerNotAuthorized, hostapi.SlackManagerUnavailable} {
		env := &fakeChoice{t: t, slackManager: &hostapi.SlackManager{Kind: kind}}

		_, err := chooseSlackApp(t.Context(), env)

		require.Error(t, err)
		assert.Contains(t, err.Error(), SlackBotTokenEnv)
		assert.Contains(t, err.Error(), "in the shell or .env")
		assert.Contains(t, err.Error(), "run `langsmith deploy` again")
	}
	problem, ok := slackUnattendedProblem(false, &hostapi.SlackManager{Kind: hostapi.SlackManagerNotAuthorized})
	assert.True(t, ok)
	assert.Contains(t, problem, "in a terminal once")
	problem, ok = slackUnattendedProblem(false, &hostapi.SlackManager{Kind: hostapi.SlackManagerUnavailable})
	assert.True(t, ok)
	assert.NotContains(t, problem, "in a terminal once")
}

func TestUnattendedSelfHostedDeployNeedsTheEnvCredentials(t *testing.T) {
	env := &fakeChoice{t: t, isSelfHosted: true}

	_, err := chooseSlackApp(t.Context(), env)

	require.Error(t, err)
	assert.Contains(t, err.Error(), SlackBotTokenEnv)
	assert.Contains(t, err.Error(), SlackSigningSecretEnv)
}

func TestHostedWithAnAuthorizedWorkspaceOffersIt(t *testing.T) {
	env := &fakeChoice{
		t:             t,
		isInteractive: true,
		slackManager:  &hostapi.SlackManager{Kind: hostapi.SlackManagerAuthorized, TeamName: "Acme"},
		answer:        answer(slackAppManaged),
	}

	app, err := chooseSlackApp(t.Context(), env)

	require.NoError(t, err)
	assert.Nil(t, app.customer)
	assert.Equal(t, []slackAppSource{slackAppManaged, slackAppExisting}, sources(env.offered))
	assert.Equal(t, "Create a new Slack app in Acme", env.offered[0].label)
	assert.False(t, env.askedCredentials)
}

func TestHostedWithoutAuthorizationOffersAllThree(t *testing.T) {
	for _, manager := range []*hostapi.SlackManager{{Kind: hostapi.SlackManagerNotAuthorized}, nil} {
		options := slackAppOptions(false, manager)

		assert.Equal(t, []slackAppSource{slackAppManaged, slackAppManifest, slackAppExisting}, sources(options))
		assert.Equal(t, "Let LangSmith create and manage a new Slack app", options[0].label)
	}
}

func TestWithoutAppManagementOnlyTheUsersOwnAppIsOffered(t *testing.T) {
	for _, offer := range []struct {
		selfHosted bool
		manager    *hostapi.SlackManager
	}{{true, nil}, {false, &hostapi.SlackManager{Kind: hostapi.SlackManagerUnavailable}}} {
		options := slackAppOptions(offer.selfHosted, offer.manager)

		assert.Equal(t, []slackAppSource{slackAppManifest, slackAppExisting}, sources(options))
	}
}

func TestSelfHostedNeverAsksAboutAppManagement(t *testing.T) {
	env := &fakeChoice{t: t, isInteractive: true, isSelfHosted: true, answer: answer(slackAppExisting)}

	_, err := chooseSlackApp(t.Context(), env)

	require.NoError(t, err)
	assert.False(t, env.askedManager)
}

func TestANewAppOpensSlackWithTheManifestThenAsksItsCredentials(t *testing.T) {
	env := &fakeChoice{t: t, isInteractive: true, isSelfHosted: true, answer: answer(slackAppManifest)}

	app, err := chooseSlackApp(t.Context(), env)

	require.NoError(t, err)
	assert.NotNil(t, app.customer)
	link, err := url.Parse(env.createPage)
	require.NoError(t, err)
	assert.Equal(t, "https://api.slack.com/apps", link.Scheme+"://"+link.Host+link.Path)
	assert.True(t, strings.HasPrefix(link.RawQuery, "new_app=1&manifest_json="), link.RawQuery)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(link.Query().Get("manifest_json")), &manifest))
	assert.Equal(t, slackTestManifest(t), manifest)
	assert.True(t, env.askedCredentials)
}

func TestAnExistingAppIsToldWhereToPointThenAskedItsCredentials(t *testing.T) {
	env := &fakeChoice{t: t, isInteractive: true, answer: answer(slackAppExisting)}

	app, err := chooseSlackApp(t.Context(), env)

	require.NoError(t, err)
	assert.NotNil(t, app.customer)
	settings := env.existingSettings
	assert.Contains(t, settings, "Request URL to\n       https://triggers.example/v1/triggers/webhooks/template\n")
	assert.Contains(t, settings, "bot events, add\n       app_mention, message.im\n")
	assert.Contains(t, settings, "Request URL to\n       https://triggers.example/v1/triggers/webhooks/slack-custom-app/interactivity\n")
	assert.Contains(t, settings, "Bot Token Scopes, add\n       chat:write, channels:history\n")
	assert.Contains(t, settings, "  1. Socket Mode")
	assert.NotContains(t, settings, "App Home")
	assert.NotContains(t, settings, "Agents & AI Apps")
	assert.True(t, env.askedCredentials)
}

func TestExistingAppSettingsCoverDMsAndAgentViewWhenTheManifestAsks(t *testing.T) {
	manifest := slackTestManifest(t)
	manifest["features"] = jsonObject(t, `{
		"app_home": {"messages_tab_enabled": true},
		"agent_view": {"agent_description": "Support"}
	}`)

	settings, err := slackExistingAppSettings(manifest)

	require.NoError(t, err)
	assert.Contains(t, settings, "  4. App Home: turn on the Messages Tab")
	assert.Contains(t, settings, "  5. Agents & AI Apps: turn it on")
	assert.Contains(t, settings, "  6. OAuth & Permissions")
}

func TestSettingsListsWrapOnWholeItems(t *testing.T) {
	scopes := make([]string, 8)
	for i := range scopes {
		scopes[i] = "channels:history"
	}

	wrapped := wrapList(scopes, "  ")

	lines := strings.Split(wrapped, "\n")
	require.Len(t, lines, 2)
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "  channels:history"), line)
		assert.LessOrEqual(t, len(line), 2+settingsListWidth, line)
	}
	assert.True(t, strings.HasSuffix(lines[1], "channels:history"), lines[1])
}

func TestExistingAppSettingsNeedACompleteManifest(t *testing.T) {
	incomplete := slackTestManifest(t)
	delete(incomplete["settings"].(map[string]any), "interactivity")

	_, err := slackExistingAppSettings(incomplete)

	require.EqualError(t, err, "Trigger Server returned an incomplete Slack app manifest.")
}

func TestEnvCredentialsComeInPairsAndNeverEchoValues(t *testing.T) {
	isolateEnv(t)

	none, err := slackCredentialsEnv(map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, none)

	_, err = slackCredentialsEnv(map[string]string{SlackBotTokenEnv: "xoxb-secret-token"})
	require.EqualError(t, err, "Set both MDA_SLACK_BOT_TOKEN and MDA_SLACK_SIGNING_SECRET, or neither.")

	_, err = slackCredentialsEnv(map[string]string{SlackBotTokenEnv: "xoxp-user-token", SlackSigningSecretEnv: "signing"})
	require.EqualError(t, err, "MDA_SLACK_BOT_TOKEN is not valid: a bot token starts with xoxb-")

	both, err := slackCredentialsEnv(map[string]string{SlackBotTokenEnv: " xoxb-token ", SlackSigningSecretEnv: "signing"})
	require.NoError(t, err)
	assert.Equal(t, "xoxb-token", both.BotToken)
	assert.Equal(t, "signing", both.SigningSecret)
}

func TestCredentialsNeverPrint(t *testing.T) {
	credentials := slackCredentials()
	for _, printed := range []string{fmt.Sprint(credentials), fmt.Sprintf("%v %+v %#v", credentials, credentials, &credentials)} {
		assert.NotContains(t, printed, "xoxb-test")
		assert.NotContains(t, printed, "signing-test")
	}
}
