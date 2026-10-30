package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

const (
	linearWebhook  = "https://agent.example/channels/linear/events"
	linearRedirect = "https://api.smith.example/v1/agent-auth/oauth/callback"
)

func testLinearChannel(t *testing.T, sessions bool) *linearChannel {
	manifest := linearManifest(t, sessions)
	delete(manifest, "requiredPermissions")
	delete(manifest, "requiredEnv")
	channel, err := prepareLinear(linearRequirements(manifest), true, "triage-agent", ".", nil)
	require.NoError(t, err)
	return channel
}

func linearCredentials() linearAppCredentials {
	return linearAppCredentials{clientID: "client-1", clientSecret: "secret-1", webhookSecret: "lin_wh_signing"}
}

func linearSession(status, redirect string) hostapi.OAuthAuthorizationSession {
	return hostapi.OAuthAuthorizationSession{
		ID:              "session-1",
		Status:          status,
		VerificationURL: "https://linear.app/oauth/authorize?actor=app&redirect_uri=" + strings.NewReplacer(":", "%3A", "/", "%2F").Replace(redirect),
	}
}

// fakeLinearEnv records what setup did; answers come from the queues.
type fakeLinearEnv struct {
	isInteractive    bool
	connections      []*linearAppConnection
	envCredentials   *linearAppCredentials
	envWebhookSecret string
	workspace        string
	install          hostapi.OAuthAuthorizationSession
	statuses         []string
	retry            []bool
	failCreate       bool
	// forbidConnectionDelete refuses deleting a workspace connection, as for
	// anyone but a workspace admin.
	forbidConnectionDelete bool
	appSource              linearAppSource
	askedAppSource         bool
	existingSettings       string
	// redirectAnswers answer "added the redirect URI?"; Yes once they run
	// out.
	redirectAnswers []bool
	// settingsAnswers answer "set the app's settings?"; Yes once they run
	// out.
	settingsAnswers []bool
	calls           []string
}

func newFakeLinearEnv() *fakeLinearEnv {
	return &fakeLinearEnv{
		isInteractive: true,
		workspace:     "acme",
		install:       linearSession("pending", linearRedirect),
		statuses:      []string{"completed"},
		appSource:     linearAppNew,
	}
}

func (f *fakeLinearEnv) called(prefix string) []string {
	var matched []string
	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			matched = append(matched, call)
		}
	}
	return matched
}

func (f *fakeLinearEnv) record(call string) {
	f.calls = append(f.calls, call)
}

func popBool(queue *[]bool, otherwise bool) bool {
	if len(*queue) == 0 {
		return otherwise
	}
	next := (*queue)[0]
	*queue = (*queue)[1:]
	return next
}

func (f *fakeLinearEnv) interactive() bool { return f.isInteractive }

func (f *fakeLinearEnv) retryOrSkip(context.Context, error) (bool, error) {
	f.record("retry_or_skip")
	return popBool(&f.retry, false), nil
}

func (f *fakeLinearEnv) success(message string) error {
	f.record("success " + message)
	return nil
}

func (f *fakeLinearEnv) info(message string) error {
	f.record("info " + message)
	return nil
}

func (f *fakeLinearEnv) warning(message string) error {
	f.record("warning " + message)
	return nil
}

func (f *fakeLinearEnv) appConnection(context.Context) (*linearAppConnection, error) {
	if len(f.connections) == 0 {
		return nil, nil
	}
	next := f.connections[0]
	f.connections = f.connections[1:]
	return next, nil
}

func (f *fakeLinearEnv) credentialsFromEnv() (*linearAppCredentials, error) {
	return f.envCredentials, nil
}

func (f *fakeLinearEnv) webhookSecretFromEnv() (string, error) {
	return f.envWebhookSecret, nil
}

func (f *fakeLinearEnv) workspaceKey(context.Context) (string, error) {
	return f.workspace, nil
}

func (f *fakeLinearEnv) configuredWorkspaceKey() string {
	return f.workspace
}

func (f *fakeLinearEnv) chooseAppSource(context.Context) (linearAppSource, error) {
	f.askedAppSource = true
	return f.appSource, nil
}

func (f *fakeLinearEnv) showExistingAppSettings(settings string) error {
	f.existingSettings = settings
	return nil
}

func (f *fakeLinearEnv) openCreatePage(url string) error {
	f.record("open_create_page " + url)
	return nil
}

func (f *fakeLinearEnv) askCredentials(context.Context) (linearAppCredentials, error) {
	f.record("ask_credentials")
	return linearCredentials(), nil
}

func (f *fakeLinearEnv) askWebhookSecret(context.Context) (string, error) {
	f.record("ask_webhook_secret")
	return "lin_wh_signing", nil
}

func (f *fakeLinearEnv) createApp(_ context.Context, channel *linearChannel, credentials linearAppCredentials) (linearAppConnection, error) {
	f.record(fmt.Sprintf("create_app %s %s %s", credentials.clientID, credentials.webhookSecret, strings.Join(channel.scopes, ",")))
	if f.failCreate {
		return linearAppConnection{}, errors.New("Agent Auth is down")
	}
	return *owned("app-1", "conn-1", hostapi.AgentGrantMissing), nil
}

func (f *fakeLinearEnv) storeWebhookSecret(_ context.Context, secret string) error {
	f.record("store_webhook_secret " + secret)
	return nil
}

func (f *fakeLinearEnv) startInstall(_ context.Context, connectionID string) (hostapi.OAuthAuthorizationSession, error) {
	f.record("start_install " + connectionID)
	return f.install, nil
}

func (f *fakeLinearEnv) confirmRedirectURI(_ context.Context, actual string) (bool, error) {
	f.record("confirm_redirect_uri " + actual)
	return popBool(&f.redirectAnswers, true), nil
}

func (f *fakeLinearEnv) confirmAppSettings(context.Context) (bool, error) {
	f.record("confirm_app_settings")
	return popBool(&f.settingsAnswers, true), nil
}

func (f *fakeLinearEnv) openInstallPage(string) error {
	f.record("open_install_page")
	return nil
}

func (f *fakeLinearEnv) nextInstallStatus(context.Context, string) (string, error) {
	if len(f.statuses) == 0 {
		return "failed", nil
	}
	next := f.statuses[0]
	f.statuses = f.statuses[1:]
	return next, nil
}

func (f *fakeLinearEnv) deleteApp(_ context.Context, appID string) error {
	f.record("delete_app " + appID)
	return nil
}

func (f *fakeLinearEnv) deleteConnection(_ context.Context, connectionID string) error {
	f.record("delete_connection " + connectionID)
	if f.forbidConnectionDelete {
		return fmt.Errorf("failed to delete the connection: %w", &hostapi.APIError{Status: http.StatusForbidden, Detail: "missing permission workspaces:manage"})
	}
	return nil
}

func (f *fakeLinearEnv) deploymentID() string { return "dep-1" }

func (f *fakeLinearEnv) webhookURL() string { return linearWebhook }

func (f *fakeLinearEnv) redirectURI() string { return linearRedirect }

// owned is the deployment's Agent Auth app.
func owned(id, connection string, grant hostapi.AgentGrantKind) *linearAppConnection {
	return &linearAppConnection{connectionID: connection, store: linearAppStore{appID: id}, grant: hostapi.AgentGrant{Kind: grant}}
}

// legacy is the workspace connections an earlier mda set up.
func legacy(id, secret string, grant hostapi.AgentGrant) *linearAppConnection {
	return &linearAppConnection{connectionID: id, store: linearAppStore{webhookSecretID: secret}, grant: grant}
}

func active() hostapi.AgentGrant {
	return hostapi.AgentGrant{Kind: hostapi.AgentGrantActive}
}

func manifestOf(t *testing.T, link string) map[string]any {
	u, err := url.Parse(link)
	require.NoError(t, err)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(u.Query().Get("manifest")), &manifest))
	return manifest
}

func TestCreatesAndInstallsANewAppFromAFilledInPage(t *testing.T) {
	env := newFakeLinearEnv()
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))

	opened := env.called("open_create_page")
	require.Len(t, opened, 1)
	link := strings.TrimPrefix(opened[0], "open_create_page ")
	assert.True(t, strings.HasPrefix(link, "https://linear.app/acme/settings/api/applications/new?manifest="), link)
	manifest := manifestOf(t, link)
	assert.Equal(t, "triage-agent", manifest["oauth"].(map[string]any)["client_name"])
	assert.True(t, strings.HasPrefix(manifest["display"].(map[string]any)["description"].(string), "A Managed Deep Agent"))
	assert.NotContains(t, manifest["display"], "iconUrl")
	assert.Equal(t, []any{linearRedirect}, manifest["oauth"].(map[string]any)["redirect_uris"])
	assert.Equal(t, linearWebhook, manifest["webhook"].(map[string]any)["url"])
	assert.Equal(t, []any{"AgentSessionEvent", "OAuthAuthorization", "PermissionChange"}, manifest["webhook"].(map[string]any)["resourceTypes"])
	// One app holds the credentials and the signing secret.
	assert.Equal(t, []string{
		"ask_credentials",
		"create_app client-1 lin_wh_signing read,write,app:mentionable,app:assignable",
		"start_install conn-1",
		"open_install_page",
	}, env.calls[1:5])
	assert.Equal(t, "success Linear channel ready · the Linear app sends its events to "+linearWebhook, env.calls[5])
	assert.Empty(t, env.called("store_webhook_secret"))
	assert.Empty(t, env.called("delete_"))
	assert.True(t, env.askedAppSource)
	// Said once, on the deploy that installs the app.
	assert.Len(t, env.called("info Linear has no API to change an app"), 1)
}

func TestFillsInTheConfiguredNameAndDescription(t *testing.T) {
	manifest := linearManifest(t, true)
	manifest["providerConfig"].(map[string]any)["app"] = map[string]any{"name": "Triage Bot", "description": "Triages new issues."}
	channel, err := prepareLinear(linearRequirements(manifest), true, "triage-agent", ".", nil)
	require.NoError(t, err)

	link, err := createAppURL("acme", channel, linearWebhook, linearRedirect)

	require.NoError(t, err)
	created := manifestOf(t, link)
	assert.Equal(t, "Triage Bot", created["oauth"].(map[string]any)["client_name"])
	assert.Equal(t, map[string]any{"description": "Triages new issues."}, created["display"])
}

func TestPointsTheIconAtTheOneTheDeploymentServes(t *testing.T) {
	channel := testLinearChannel(t, true)
	channel.appIcon = "assets/icon.png"

	link, err := createAppURL("acme", channel, linearWebhook, linearRedirect)

	require.NoError(t, err)
	assert.Equal(t, "https://agent.example/channels/linear/icon", manifestOf(t, link)["display"].(map[string]any)["iconUrl"])
}

func TestUsesCredentialsFromTheEnvWithoutAPage(t *testing.T) {
	credentials := linearCredentials()
	env := newFakeLinearEnv()
	env.isInteractive = false
	env.envCredentials = &credentials
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.Empty(t, env.called("open_create_page"))
	assert.Empty(t, env.called("ask_credentials"))
	assert.Len(t, env.called("create_app"), 1)
	assert.Empty(t, env.called("store_webhook_secret"))

	// Without a terminal and without them, setup says what to set.
	unattendedEnv := newFakeLinearEnv()
	unattendedEnv.isInteractive = false
	unattendedEnv.workspace = ""
	err := runLinearSetup(t.Context(), unattendedEnv, testLinearChannel(t, true))
	require.ErrorContains(t, err, LinearClientIDEnv)
	require.ErrorContains(t, err, "settings/api/applications/new")
	// Without the workspace, the link opens wherever the person last was.
	require.ErrorContains(t, err, LinearWorkspaceEnv)

	scoped := newFakeLinearEnv()
	scoped.isInteractive = false
	err = runLinearSetup(t.Context(), scoped, testLinearChannel(t, true))
	require.ErrorContains(t, err, "linear.app/acme/settings/api")
	assert.NotContains(t, err.Error(), LinearWorkspaceEnv)
}

func TestListsAnEnvAppsSettingsAndWaitsForThemInATerminal(t *testing.T) {
	credentials := linearCredentials()
	env := newFakeLinearEnv()
	env.envCredentials = &credentials
	env.settingsAnswers = []bool{false}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))

	warnings := env.called("warning Point your Linear app")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Redirect URIs: add "+linearRedirect)
	assert.Contains(t, warnings[0], "URL "+linearWebhook)
	assert.Contains(t, warnings[0], "Linear can't tell deploy what the app has set")
	assert.NotContains(t, warnings[0], "prompts below")
	// "No" asks again; the install page opens only after "Yes".
	assert.Len(t, env.called("confirm_app_settings"), 2)
	assert.Len(t, env.called("info Linear refuses the install"), 1)
	confirmed := lastIndex(env.calls, "confirm_app_settings")
	open := slices.Index(env.calls, "open_install_page")
	assert.Less(t, confirmed, open)

	// Without a terminal the settings come before the install link, unasked.
	unattendedEnv := newFakeLinearEnv()
	unattendedEnv.isInteractive = false
	unattendedEnv.envCredentials = &credentials
	require.NoError(t, runLinearSetup(t.Context(), unattendedEnv, testLinearChannel(t, true)))
	assert.Empty(t, unattendedEnv.called("confirm_app_settings"))
	listed := slices.IndexFunc(unattendedEnv.calls, func(call string) bool { return strings.HasPrefix(call, "warning Point your Linear app") })
	assert.GreaterOrEqual(t, listed, 0)
	assert.Less(t, listed, slices.Index(unattendedEnv.calls, "open_install_page"))

	// A new app from deploy's page is filled in, so nothing is listed.
	created := newFakeLinearEnv()
	require.NoError(t, runLinearSetup(t.Context(), created, testLinearChannel(t, true)))
	assert.Empty(t, created.called("warning Point your Linear app"))
	assert.Empty(t, created.called("confirm_app_settings"))
}

func lastIndex(calls []string, call string) int {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i] == call {
			return i
		}
	}
	return -1
}

func TestTellsAWrongClientIDOrSecretFromAnAnswerThatSaysNothing(t *testing.T) {
	invalid := func(code string) string { return fmt.Sprintf(`{"error":%q,"error_description":"x"}`, code) }
	require.ErrorContains(t, credentialsVerdict(400, invalid("invalid_secret")), LinearClientSecretEnv)
	require.ErrorContains(t, credentialsVerdict(400, invalid("invalid_client")), LinearClientIDEnv)
	// Right credentials: Linear only refuses the made-up code.
	require.NoError(t, credentialsVerdict(400, invalid("invalid_grant")))
	// Anything else can't tell, so deploy goes on.
	require.NoError(t, credentialsVerdict(500, invalid("invalid_client")))
	require.NoError(t, credentialsVerdict(400, "not json"))
	require.NoError(t, credentialsVerdict(200, "{}"))
}

func TestAsksLinearWhetherTheClientIDAndSecretBelongTogether(t *testing.T) {
	linear := serveSequence(t,
		reply{http.StatusBadRequest, `{"error":"invalid_client","error_description":"x"}`},
		reply{http.StatusBadRequest, `{"error":"invalid_grant"}`})
	original := linearTokenURL
	linearTokenURL = linear.URL + "/oauth/token"
	t.Cleanup(func() { linearTokenURL = original })

	err := checkCredentials(t.Context(), linearCredentials())
	require.EqualError(t, err, "Linear knows no app with MDA_LINEAR_CLIENT_ID and MDA_LINEAR_CLIENT_SECRET. Copy both from the app's settings page and run `langsmith deploy` again.")
	require.NoError(t, checkCredentials(t.Context(), linearCredentials()))

	request := linear.received()[0]
	assert.Equal(t, "POST /oauth/token", request.line())
	assert.Equal(t, "application/x-www-form-urlencoded", request.header.Get("Content-Type"))
	form, err := url.ParseQuery(request.body)
	require.NoError(t, err)
	assert.Equal(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"mda-deploy-credentials-check"},
		"client_id":     {"client-1"},
		"client_secret": {"secret-1"},
	}, form)
	// Linear never sees a LangSmith credential.
	assert.Empty(t, request.header.Get("Authorization"))
	assert.Empty(t, request.header.Get("X-Api-Key"))

	// An unreachable Linear says nothing, so deploy goes on.
	linearTokenURL = "http://127.0.0.1:1/oauth/token"
	require.NoError(t, checkCredentials(t.Context(), linearCredentials()))
}

func TestSaysWhenAgentAuthRedirectsElsewhere(t *testing.T) {
	tunnel := "https://tunnel.trycloudflare.com/v1/agent-auth/oauth/callback"
	env := newFakeLinearEnv()
	env.install = linearSession("pending", tunnel)
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	// Asked about before the install page opens, not after.
	confirm := slices.Index(env.calls, "confirm_redirect_uri "+tunnel)
	assert.GreaterOrEqual(t, confirm, 0, "%v", env.calls)
	assert.Less(t, confirm, slices.Index(env.calls, "open_install_page"))

	// A redirect URI the new app already lists needs nothing.
	listed := newFakeLinearEnv()
	require.NoError(t, runLinearSetup(t.Context(), listed, testLinearChannel(t, true)))
	assert.Empty(t, listed.called("confirm_redirect_uri"))
}

func TestANoToTheRedirectURIAsksAgainBeforeTheInstallPageOpens(t *testing.T) {
	tunnel := "https://tunnel.trycloudflare.com/v1/agent-auth/oauth/callback"
	env := newFakeLinearEnv()
	env.install = linearSession("pending", tunnel)
	env.redirectAnswers = []bool{false, false}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.Len(t, env.called("confirm_redirect_uri"), 3)
	assert.Len(t, env.called("info Linear refuses the install"), 2)
	assert.Less(t, lastIndex(env.calls, "confirm_redirect_uri "+tunnel), slices.Index(env.calls, "open_install_page"))

	// Without a terminal nobody is asked: deploy prints the URI and goes on.
	credentials := linearCredentials()
	unattendedEnv := newFakeLinearEnv()
	unattendedEnv.isInteractive = false
	unattendedEnv.envCredentials = &credentials
	unattendedEnv.install = linearSession("pending", tunnel)
	require.NoError(t, runLinearSetup(t.Context(), unattendedEnv, testLinearChannel(t, true)))
	assert.Empty(t, unattendedEnv.called("confirm_redirect_uri"))
	assert.Len(t, unattendedEnv.called("warning Agent Auth expects Linear to redirect"), 1)
}

func TestRemovesAnUnfinishedInstallAndOffersToTryAgain(t *testing.T) {
	env := newFakeLinearEnv()
	env.statuses = []string{"failed", "completed"}
	env.retry = []bool{true}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	// The failed attempt's app goes, with its connection and secret.
	assert.Equal(t, []string{"delete_app app-1"}, env.called("delete_"))
	assert.Len(t, env.called("create_app"), 2)
	assert.Len(t, env.called("retry_or_skip"), 1)
	assert.Len(t, env.called("start_install"), 2)
	assert.Len(t, env.called("success"), 1)

	// Skipping continues the deploy without the channel.
	skip := newFakeLinearEnv()
	skip.failCreate = true
	require.NoError(t, runLinearSetup(t.Context(), skip, testLinearChannel(t, true)))
	assert.Contains(t, skip.called("warning")[0], "Continuing without the Linear channel")
}

func TestFinishesWhatAnEarlierSetupLeft(t *testing.T) {
	// An earlier mda's install, missing the signing secret.
	env := newFakeLinearEnv()
	env.connections = []*linearAppConnection{legacy("conn-1", "", active())}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.Equal(t, []string{"ask_webhook_secret", "store_webhook_secret lin_wh_signing"}, env.calls[:2])
	assert.Empty(t, env.called("start_install"))

	// An install that never finished starts over with new credentials.
	stale := newFakeLinearEnv()
	stale.connections = []*linearAppConnection{owned("app-0", "conn-0", hostapi.AgentGrantMissing)}
	require.NoError(t, runLinearSetup(t.Context(), stale, testLinearChannel(t, true)))
	assert.Equal(t, []string{"delete_app app-0"}, stale.called("delete_"))
	assert.Len(t, stale.called("ask_credentials"), 1)
	assert.Len(t, stale.called("create_app"), 1)

	// So does an earlier mda's, whose connections go one by one.
	stale = newFakeLinearEnv()
	stale.connections = []*linearAppConnection{legacy("conn-0", "secret-0", hostapi.AgentGrant{Kind: hostapi.AgentGrantMissing})}
	require.NoError(t, runLinearSetup(t.Context(), stale, testLinearChannel(t, true)))
	assert.Equal(t, []string{"delete_connection secret-0", "delete_connection conn-0"}, stale.called("delete_"))
	assert.Len(t, stale.called("create_app"), 1)

	// A ready app needs nothing, and setup never asks Linear about it.
	ready := newFakeLinearEnv()
	ready.connections = []*linearAppConnection{owned("app-1", "conn-1", hostapi.AgentGrantActive)}
	require.NoError(t, runLinearSetup(t.Context(), ready, testLinearChannel(t, true)))
	assert.True(t, strings.HasPrefix(ready.calls[0], "success Linear channel ready · "), ready.calls[0])
	assert.Empty(t, ready.called("start_install"))
	assert.Empty(t, ready.called("delete_"))

	// An earlier mda's ready connections are kept as they are.
	kept := newFakeLinearEnv()
	kept.connections = []*linearAppConnection{legacy("conn-1", "secret-1", active())}
	require.NoError(t, runLinearSetup(t.Context(), kept, testLinearChannel(t, true)))
	assert.Equal(t, []string{"success Linear channel ready · the Linear app sends its events to " + linearWebhook}, kept.calls)
}

func TestSaysWhereTheAppChangesOnceWhenItIsInstalled(t *testing.T) {
	env := newFakeLinearEnv()
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	notes := env.called("info Linear has no API to change an app")
	require.Len(t, notes, 1, "%v", env.calls)
	assert.Contains(t, notes[0], "app name, description or icon")
	assert.Contains(t, notes[0], "https://linear.app/acme/settings/api")

	// Without a workspace key, it names where the page is.
	unscoped := newFakeLinearEnv()
	unscoped.workspace = ""
	require.NoError(t, runLinearSetup(t.Context(), unscoped, testLinearChannel(t, true)))
	assert.Contains(t, unscoped.called("info Linear has no API to change an app")[0], "Settings → Administration → API")

	// A redeploy that keeps the app only says it is ready.
	kept := newFakeLinearEnv()
	kept.connections = []*linearAppConnection{owned("app-1", "conn-1", hostapi.AgentGrantActive)}
	require.NoError(t, runLinearSetup(t.Context(), kept, testLinearChannel(t, true)))
	assert.Equal(t, []string{"success Linear channel ready · the Linear app sends its events to " + linearWebhook}, kept.calls)
}

func TestAsksForConsentAgainWhenTheGrantLapsed(t *testing.T) {
	// The app and its secret stay: only the admin's approval is needed.
	env := newFakeLinearEnv()
	env.connections = []*linearAppConnection{owned("app-0", "conn-0", hostapi.AgentGrantAuthorizationRequired)}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.True(t, strings.HasPrefix(env.calls[0], "warning Linear stopped renewing"), env.calls[0])
	assert.Equal(t, []string{"start_install conn-0", "open_install_page"}, env.calls[1:3])
	assert.Empty(t, env.called("delete_"))
	assert.Empty(t, env.called("ask_credentials"))
	assert.Empty(t, env.called("open_create_page"))
	assert.Empty(t, env.called("create_app"))
	assert.Len(t, env.called("success"), 1)

	// A missing signing secret of an earlier mda's install is stored along
	// the way.
	unsigned := newFakeLinearEnv()
	unsigned.connections = []*linearAppConnection{legacy("conn-0", "", hostapi.AgentGrant{Kind: hostapi.AgentGrantAuthorizationRequired})}
	require.NoError(t, runLinearSetup(t.Context(), unsigned, testLinearChannel(t, true)))
	assert.Equal(t, []string{"store_webhook_secret lin_wh_signing"}, unsigned.called("store_webhook_secret"))
	assert.Equal(t, []string{"start_install conn-0"}, unsigned.called("start_install"))

	// Without a terminal, deploy prints the link and waits for it.
	unattendedEnv := newFakeLinearEnv()
	unattendedEnv.isInteractive = false
	unattendedEnv.connections = []*linearAppConnection{owned("app-0", "conn-0", hostapi.AgentGrantAuthorizationRequired)}
	require.NoError(t, runLinearSetup(t.Context(), unattendedEnv, testLinearChannel(t, true)))
	assert.Len(t, unattendedEnv.called("open_install_page"), 1)
	assert.Len(t, unattendedEnv.called("success"), 1)

	// A consent that fails keeps the app, to approve again next time.
	refused := newFakeLinearEnv()
	refused.statuses = []string{"failed"}
	refused.connections = []*linearAppConnection{owned("app-0", "conn-0", hostapi.AgentGrantAuthorizationRequired)}
	require.NoError(t, runLinearSetup(t.Context(), refused, testLinearChannel(t, true)))
	assert.Empty(t, refused.called("delete_"))
	assert.Len(t, refused.called("retry_or_skip"), 1)
	warnings := refused.called("warning")
	assert.Contains(t, warnings[len(warnings)-1], "Continuing without the Linear channel")
}

func TestAsksForConsentAgainWhenTheGrantWasRevoked(t *testing.T) {
	// A revoke keeps the app and its secret, like a lapsed grant: it does not
	// delete the connection or ask for a new app.
	env := newFakeLinearEnv()
	env.connections = []*linearAppConnection{owned("app-0", "conn-0", hostapi.AgentGrantRevoked)}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.True(t, strings.HasPrefix(env.calls[0], "warning This deployment's access to the Linear app was revoked"), "%v", env.calls)
	assert.Equal(t, []string{"start_install conn-0", "open_install_page"}, env.calls[1:3])
	assert.Empty(t, env.called("delete_"))
	assert.Empty(t, env.called("ask_credentials"))
	assert.Empty(t, env.called("create_app"))
	assert.Len(t, env.called("success"), 1)
}

func TestWarnsAboutAGrantStatusItDoesNotKnowAndKeepsTheApp(t *testing.T) {
	suspended := hostapi.AgentGrant{Kind: hostapi.AgentGrantUnknown, Status: "suspended"}
	env := newFakeLinearEnv()
	env.connections = []*linearAppConnection{{connectionID: "conn-1", store: linearAppStore{appID: "app-1"}, grant: suspended}}
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	warnings := env.called("warning")
	require.Len(t, warnings, 1, "%v", env.calls)
	assert.Contains(t, warnings[0], "`suspended`")
	// Neither healthy without a word nor a reason to fail or re-consent.
	assert.Empty(t, env.called("start_install"))
	assert.Empty(t, env.called("delete_"))
	assert.Empty(t, env.called("create_app"))
	assert.Len(t, env.called("success"), 1)

	// A missing signing secret is still stored, also without a terminal.
	unsigned := newFakeLinearEnv()
	unsigned.isInteractive = false
	unsigned.envWebhookSecret = "lin_wh_from_env"
	unsigned.connections = []*linearAppConnection{legacy("conn-1", "", suspended)}
	require.NoError(t, runLinearSetup(t.Context(), unsigned, testLinearChannel(t, true)))
	assert.Equal(t, []string{"store_webhook_secret lin_wh_from_env"}, unsigned.called("store_webhook_secret"))
}

func TestSetsUpNothingWithoutHandledEvents(t *testing.T) {
	env := newFakeLinearEnv()
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, false)))
	assert.Equal(t, []string{"info " + linearNoEvents}, env.calls)
}

func TestReadsTheWorkspaceKeyFromAURLOrTheKey(t *testing.T) {
	key, err := workspaceKey("Acme")
	require.NoError(t, err)
	assert.Equal(t, "acme", key)
	key, err = workspaceKey("https://linear.app/acme-inc/team/ENG/active")
	require.NoError(t, err)
	assert.Equal(t, "acme-inc", key)
	for _, invalid := range []string{"", "acme/../x", "https://linear.app/", "a b"} {
		_, err := workspaceKey(invalid)
		require.EqualError(t, err, "Enter the workspace's URL key, as in https://linear.app/<key>/.", invalid)
	}
	link, err := createAppURL("", testLinearChannel(t, true), linearWebhook, linearRedirect)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(link, "https://linear.app/settings/api/applications/new?manifest="), link)
}

func TestStoresAMissingSigningSecretFromTheEnvAlone(t *testing.T) {
	installed := func() []*linearAppConnection { return []*linearAppConnection{legacy("conn-1", "", active())} }
	// Without a terminal, the signing secret alone repairs the install.
	env := newFakeLinearEnv()
	env.isInteractive = false
	env.connections = installed()
	env.envWebhookSecret = "lin_wh_from_env"
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))
	assert.Equal(t, []string{"store_webhook_secret lin_wh_from_env"}, env.called("store_webhook_secret"))
	assert.Empty(t, env.called("ask_"))
	assert.Empty(t, env.called("start_install"))

	// Without it, deploy names just that variable.
	missing := newFakeLinearEnv()
	missing.isInteractive = false
	missing.connections = installed()
	err := runLinearSetup(t.Context(), missing, testLinearChannel(t, true))
	require.EqualError(t, err, linearMissingSecret)
	assert.NotContains(t, err.Error(), LinearClientIDEnv)
}

func TestReadsTheSigningSecretAloneFromTheEnv(t *testing.T) {
	isolateEnv(t)
	secret, err := linearWebhookSecretEnv(map[string]string{LinearWebhookSecretEnv: "lin_wh_abc"})
	require.NoError(t, err)
	assert.Equal(t, "lin_wh_abc", secret)
	secret, err = linearWebhookSecretEnv(map[string]string{})
	require.NoError(t, err)
	assert.Empty(t, secret)
	// Checked like a pasted one, and never echoed.
	_, err = linearWebhookSecretEnv(map[string]string{LinearWebhookSecretEnv: "not-a-secret"})
	require.ErrorContains(t, err, LinearWebhookSecretEnv)
	assert.NotContains(t, err.Error(), "not-a-secret")
	// The other two are not needed for it, while a new app still needs all
	// three.
	_, err = linearCredentialsEnv(map[string]string{LinearWebhookSecretEnv: "lin_wh_abc"})
	require.EqualError(t, err, "Set all of MDA_LINEAR_CLIENT_ID, MDA_LINEAR_CLIENT_SECRET and MDA_LINEAR_WEBHOOK_SECRET, or none of them.")
	// In a terminal, the signing secret alone repairs an installed app.
	require.NoError(t, interactiveCredentialsEnv(map[string]string{LinearWebhookSecretEnv: "lin_wh_abc"}))
	require.NoError(t, interactiveCredentialsEnv(map[string]string{}))
	require.Error(t, interactiveCredentialsEnv(map[string]string{LinearClientIDEnv: "id"}))
}

func TestUsesAnExistingAppPointedAtTheDeployment(t *testing.T) {
	env := newFakeLinearEnv()
	env.appSource = linearAppExisting
	require.NoError(t, runLinearSetup(t.Context(), env, testLinearChannel(t, true)))

	assert.Empty(t, env.called("open_create_page"))
	settings := env.existingSettings
	assert.Contains(t, settings, "https://linear.app/acme/settings/api")
	assert.Contains(t, settings, "Redirect URIs: add "+linearRedirect)
	assert.Contains(t, settings, "URL "+linearWebhook)
	assert.Contains(t, settings, "events: AgentSessionEvent, OAuthAuthorization, PermissionChange")
	assert.Equal(t, []string{
		"ask_credentials",
		"create_app client-1 lin_wh_signing read,write,app:mentionable,app:assignable",
		"start_install conn-1",
		"open_install_page",
	}, env.calls[:4])
}

func TestEnvCredentialsOrNoTerminalNeverAskWhereTheAppComesFrom(t *testing.T) {
	credentials := linearCredentials()
	withEnv := newFakeLinearEnv()
	withEnv.envCredentials = &credentials
	require.NoError(t, runLinearSetup(t.Context(), withEnv, testLinearChannel(t, true)))
	assert.False(t, withEnv.askedAppSource)

	unattendedEnv := newFakeLinearEnv()
	unattendedEnv.isInteractive = false
	require.Error(t, runLinearSetup(t.Context(), unattendedEnv, testLinearChannel(t, true)))
	assert.False(t, unattendedEnv.askedAppSource)
}

func TestSaysWhoCanDeleteAnEarlierMdasUnfinishedSetup(t *testing.T) {
	// Only a workspace admin can delete an earlier mda's connections, so
	// anyone else learns who can, rather than a bare 403.
	credentials := linearCredentials()
	env := newFakeLinearEnv()
	env.isInteractive = false
	env.envCredentials = &credentials
	env.forbidConnectionDelete = true
	env.connections = []*linearAppConnection{legacy("conn-0", "secret-0", hostapi.AgentGrant{Kind: hostapi.AgentGrantMissing})}

	err := runLinearSetup(t.Context(), env, testLinearChannel(t, true))

	require.EqualError(t, err, legacyConnectionsMessage("dep-1"))
	assert.Contains(t, err.Error(), "LangSmith workspace admin")
	assert.Contains(t, err.Error(), "mda connections delete mda/linear-app/dep-1/webhook-secret")
	assert.Contains(t, err.Error(), "mda connections delete mda/linear-app/dep-1`")
	assert.NotContains(t, err.Error(), "HTTP 403")
	// Nothing new is created over the old connections.
	assert.Empty(t, env.called("create_app"))
}

const linearConnectionID = "11111111-2222-3333-4444-555555555555"

func TestLooksUpTheDeploymentsLinearApp(t *testing.T) {
	agentAuth := serveSequence(t,
		reply{http.StatusOK, `[
			{ "id": "app-gh", "provider": "github", "slug": "mda/github-app/dep-1", "connection_id": "conn-gh" },
			{ "id": "app-1", "provider": "linear", "source": "existing", "slug": "mda/linear-app/dep-1", "connection_id": "` + linearConnectionID + `" }
		]`},
		// The app's connection, with no grant yet.
		reply{http.StatusOK, `{ "connection_id": "` + linearConnectionID + `" }`})

	found, err := lookupLinearAppConnection(t.Context(), oauthClient(t, agentAuth.URL), "dep-1")

	require.NoError(t, err)
	assert.Equal(t, &linearAppConnection{
		connectionID: linearConnectionID,
		store:        linearAppStore{appID: "app-1"},
		grant:        hostapi.AgentGrant{Kind: hostapi.AgentGrantMissing},
	}, found)
	requests := agentAuth.received()
	assert.Equal(t, "GET /v1/agent-auth/apps?owner_id=dep-1", requests[0].line())
	assert.Contains(t, requests[1].uri, "/"+linearConnectionID+"?")
	assert.Equal(t, "Bearer oauth-token", requests[1].header.Get("Authorization"))
}

func TestFindsAnEarlierMdasConnectionsWithoutAnApp(t *testing.T) {
	agentAuth := serveSequence(t,
		reply{http.StatusOK, `[]`},
		reply{http.StatusOK, `{"items": [
			{ "connection_id": "conn-0", "slug": "mda/linear-app/dep-1", "kind": "oauth" },
			{ "connection_id": "secret-0", "slug": "mda/linear-app/dep-1/webhook-secret", "kind": "secret" },
			{ "connection_id": "other", "slug": "mda/linear-app/dep-10", "kind": "oauth" }
		]}`},
		reply{http.StatusOK, `{ "connection_id": "conn-0" }`})

	found, err := lookupLinearAppConnection(t.Context(), oauthClient(t, agentAuth.URL), "dep-1")

	require.NoError(t, err)
	assert.Equal(t, "conn-0", found.connectionID)
	assert.Equal(t, linearAppStore{webhookSecretID: "secret-0"}, found.store)

	// Neither: no app yet.
	none := serveSequence(t,
		// A LangSmith without Agent Auth apps.
		reply{http.StatusNotFound, `{}`},
		reply{http.StatusOK, `{"items": []}`})
	found, err = lookupLinearAppConnection(t.Context(), oauthClient(t, none.URL), "dep-1")
	require.NoError(t, err)
	assert.Nil(t, found)
}
