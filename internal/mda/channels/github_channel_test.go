package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

const (
	githubDeployment = "0f7d5c42-f907-4b4f-b8ae-ff8801592265"
	githubSession    = "99999999-8888-7777-6666-555555555555"
	githubAppRowID   = "33333333-4444-5555-6666-777777777777"
	githubConnection = "11111111-2222-3333-4444-555555555555"
	githubWebhook    = "https://agent.example/channels/github/events"
	otherWebhook     = "https://ci.example/hooks/github"
	githubProviderID = "22222222-3333-4444-5555-666666666666"
)

func setupChannel() *githubChannel {
	return &githubChannel{
		name:        "github",
		eventsPath:  "/channels/github/events",
		handled:     []string{"issues.opened"},
		events:      []string{"issues"},
		permissions: map[string]permissionLevel{"issues": permissionWrite, "metadata": permissionRead},
		appName:     "acme-triage",
	}
}

func githubStatus(t *testing.T, installed bool) *hostapi.GitHubChannelStatus {
	installations := `[]`
	if installed {
		installations = `[{
			"id": 9, "account": { "login": "acme", "type": "Organization" },
			"repository_selection": "selected", "repositories": ["acme/api"],
			"permissions": { "issues": "write", "metadata": "read" }
		}]`
	}
	return channelStatus(t, jsonObject(t, `{
		"app": {
			"id": 42, "slug": "acme-triage",
			"owner": { "login": "acme", "type": "Organization" },
			"permissions": { "issues": "write", "metadata": "read" },
			"events": ["issues"],
			"webhook_url": "`+githubWebhook+`"
		},
		"installations": `+installations+`
	}`))
}

func synced(status *hostapi.GitHubChannelStatus) hostapi.GitHubSyncOutcome {
	return hostapi.GitHubSyncOutcome{Kind: hostapi.GitHubSyncOutcomeSynced, Status: status}
}

func unavailable(reason string) hostapi.GitHubSyncOutcome {
	return hostapi.GitHubSyncOutcome{Kind: hostapi.GitHubSyncOutcomeUnavailable, Reason: reason}
}

func githubSessionWith(status string) hostapi.OAuthAuthorizationSession {
	return hostapi.OAuthAuthorizationSession{
		ID:              githubSession,
		Status:          status,
		VerificationURL: "https://aa.example/v1/agent-auth/github-app/start?state=s",
	}
}

func failedSession(code string) hostapi.OAuthAuthorizationSession {
	session := githubSessionWith("failed")
	session.ErrorCode = code
	return session
}

// agentAuthApp is the agent's App in Agent Auth. A brought App's webhook
// starts out at whatever served it before; Agent Auth points an App it
// creates here.
func agentAuthApp(t *testing.T, state hostapi.GitHubAppState) hostapi.AgentAuthApp {
	source, created, webhook := "manifest", true, githubWebhook
	switch state {
	case hostapi.GitHubAppStatePending:
		created = false
	case hostapi.GitHubAppStateExternal:
		source, webhook = "existing", otherWebhook
	}
	value := map[string]any{
		"id": githubAppRowID, "provider": "github", "source": source, "slug": githubSlugs.connection(githubDeployment),
		"manifest": map[string]any{"webhook_url": webhook},
	}
	if created {
		value["app_id"] = "42"
		value["connection_id"] = githubConnection
		value["app_metadata"] = map[string]any{"slug": "acme-triage", "html_url": "https://github.com/apps/acme-triage"}
	}
	data, err := json.Marshal(value)
	require.NoError(t, err)
	var app hostapi.AgentAuthApp
	require.NoError(t, json.Unmarshal(data, &app))
	return app
}

func appConnection(t *testing.T, state hostapi.GitHubAppState, installed bool) *githubAppConnection {
	return &githubAppConnection{
		app:                 agentAuthApp(t, state),
		providerID:          githubProviderID,
		state:               state,
		scopes:              []string{"issues:write", "metadata:read"},
		installed:           installed,
		webhookSecretStored: true,
		botIDStored:         true,
		keptWebhooks:        map[string]bool{},
	}
}

// brought is an App the user brought, before deploy stored its webhook
// secret and bot ID.
func brought(t *testing.T, installed bool) *githubAppConnection {
	connection := appConnection(t, hostapi.GitHubAppStateExternal, installed)
	connection.webhookSecretStored = false
	connection.botIDStored = false
	return connection
}

// debugStart writes a session start as the mda CLI's tests print it.
func debugStart(start hostapi.GitHubAppSessionStart) string {
	switch start.Kind {
	case hostapi.GitHubAppSessionStartCreate:
		if start.Organization == "" {
			return "Create { organization: None }"
		}
		return fmt.Sprintf("Create { organization: Some(%q) }", start.Organization)
	case hostapi.GitHubAppSessionStartInstall:
		return "Install"
	default:
		return fmt.Sprintf("Installation(%q)", start.InstallationID)
	}
}

func debugOwner(owner appOwner) string {
	if owner.organization == "" {
		return "Personal"
	}
	return fmt.Sprintf("Organization(%q)", owner.organization)
}

type fakeGitHubEnv struct {
	t             *testing.T
	isInteractive bool
	connection    *githubAppConnection
	// afterFailure is what appConnection reports once a setup step has
	// failed.
	afterFailure *githubAppConnection
	source       *githubAppSource
	// envSource is what the CLI-only env vars choose, so nobody is asked.
	envSource   *githubAppSource
	startErrors []error
	started     *hostapi.OAuthAuthorizationSession
	polls       []hostapi.OAuthAuthorizationSession
	// interruptPolls is Ctrl-C while waiting for the session.
	interruptPolls bool
	syncs          []hostapi.GitHubSyncOutcome
	ticks          []waitTick
	retries        []bool
	scopeErrors    []error
	// secret is what webhookSecret returns; the fake never logs the value.
	secret     string
	ownerError error
	// replaceWebhook is the answer to replacing a webhook that points
	// elsewhere.
	replaceWebhook bool
	// cancelWebhookReplace is Escape or Ctrl-C at the webhook prompt.
	cancelWebhookReplace bool
	// webhookElsewhere is `MDA_GITHUB_WEBHOOK_ELSEWHERE`.
	webhookElsewhere string
	// installationEnv is `MDA_GITHUB_APP_INSTALLATION_ID`.
	installationEnv string
	log             []string
}

func interactiveGitHubEnv(t *testing.T) *fakeGitHubEnv {
	return &fakeGitHubEnv{t: t, isInteractive: true, secret: "whsec"}
}

func sourceOf(source githubAppSource) *githubAppSource {
	return &source
}

func sessionOf(session hostapi.OAuthAuthorizationSession) *hostapi.OAuthAuthorizationSession {
	return &session
}

func (f *fakeGitHubEnv) record(line string) {
	f.log = append(f.log, line)
}

func (f *fakeGitHubEnv) interactive() bool { return f.isInteractive }

func (f *fakeGitHubEnv) retryOrSkip(_ context.Context, err error) (bool, error) {
	f.record("retry? " + err.Error())
	if f.afterFailure != nil {
		f.connection, f.afterFailure = f.afterFailure, nil
	}
	require.NotEmpty(f.t, f.retries, "unexpected retry prompt")
	retry := f.retries[0]
	f.retries = f.retries[1:]
	return retry, nil
}

func (f *fakeGitHubEnv) success(message string) error {
	f.record("success: " + message)
	return nil
}

func (f *fakeGitHubEnv) info(message string) error {
	f.record("info: " + message)
	return nil
}

func (f *fakeGitHubEnv) warning(message string) error {
	f.record("warning: " + message)
	return nil
}

func (f *fakeGitHubEnv) appConnection(context.Context) (*githubAppConnection, error) {
	if f.connection == nil {
		return nil, nil
	}
	copied := *f.connection
	return &copied, nil
}

func (f *fakeGitHubEnv) chooseAppSource(context.Context) (githubAppSource, error) {
	f.record("choose")
	require.NotNil(f.t, f.source, "app source")
	return *f.source, nil
}

func (f *fakeGitHubEnv) appSourceFromEnv() (githubAppSource, bool) {
	if f.envSource == nil {
		return 0, false
	}
	return *f.envSource, true
}

func (f *fakeGitHubEnv) createApp(_ context.Context, channel *githubChannel, owner appOwner) (githubAppConnection, hostapi.OAuthAuthorizationSession, error) {
	f.record(fmt.Sprintf("create %s for %s", channel.appName, debugOwner(owner)))
	f.connection = appConnection(f.t, hostapi.GitHubAppStatePending, false)
	session := githubSessionWith("pending")
	if f.started != nil {
		session = *f.started
	}
	return *appConnection(f.t, hostapi.GitHubAppStatePending, false), session, nil
}

func (f *fakeGitHubEnv) storeExistingApp(context.Context) (githubAppConnection, error) {
	f.record("existing")
	return *brought(f.t, false), nil
}

func (f *fakeGitHubEnv) chooseOwner(context.Context) (appOwner, error) {
	f.record("owner")
	if err := f.ownerError; err != nil {
		f.ownerError = nil
		return appOwner{}, err
	}
	return appOwner{organization: "acme"}, nil
}

func (f *fakeGitHubEnv) askInstallationID(context.Context) (string, error) {
	f.record("installation")
	return "777", nil
}

func (f *fakeGitHubEnv) installationFromEnv() string {
	return f.installationEnv
}

func (f *fakeGitHubEnv) chooseInstallation(_ context.Context, installations []hostapi.GitHubInstallation) (string, error) {
	var ids []string
	for _, installation := range installations {
		ids = append(ids, fmt.Sprint(installation.ID))
	}
	f.record("choose installation " + strings.Join(ids, ","))
	return ids[len(ids)-1], nil
}

func (f *fakeGitHubEnv) webhookSecret(context.Context) (string, bool, error) {
	f.record("webhook secret")
	return f.secret, f.secret != "", nil
}

func (f *fakeGitHubEnv) storeAppValue(_ context.Context, suffix, value string) error {
	require.NotEmpty(f.t, value)
	f.record("store " + suffix)
	return nil
}

func (f *fakeGitHubEnv) startSession(_ context.Context, connection *githubAppConnection, start hostapi.GitHubAppSessionStart) (hostapi.OAuthAuthorizationSession, error) {
	f.record(fmt.Sprintf("start %s %s", connection.app.ID, debugStart(start)))
	if len(f.startErrors) > 0 {
		err := f.startErrors[0]
		f.startErrors = f.startErrors[1:]
		return hostapi.OAuthAuthorizationSession{}, err
	}
	if f.started != nil {
		return *f.started, nil
	}
	// Recording an installation completes at once, like Agent Auth's.
	if start.Kind == hostapi.GitHubAppSessionStartInstallation {
		return githubSessionWith("completed"), nil
	}
	return githubSessionWith("pending"), nil
}

func (f *fakeGitHubEnv) openSetupPage(url string, creating bool) error {
	f.record(fmt.Sprintf("open %s creating=%t", url, creating))
	return nil
}

func (f *fakeGitHubEnv) nextSession(_ context.Context, sessionID string) (hostapi.OAuthAuthorizationSession, error) {
	f.record("poll " + sessionID)
	if f.interruptPolls {
		return hostapi.OAuthAuthorizationSession{}, context.Canceled
	}
	require.NotEmpty(f.t, f.polls, "unexpected poll")
	next := f.polls[0]
	f.polls = f.polls[1:]
	return next, nil
}

func (f *fakeGitHubEnv) sessionStatus(_ context.Context, sessionID string) (string, error) {
	f.record("status " + sessionID)
	require.NotEmpty(f.t, f.polls, "unexpected status")
	next := f.polls[0]
	f.polls = f.polls[1:]
	return next.Status, nil
}

func (f *fakeGitHubEnv) updateTokenScopes(_ context.Context, providerID string, scopes []string) error {
	f.record(fmt.Sprintf("scopes %s %s", providerID, strings.Join(scopes, ",")))
	if len(f.scopeErrors) > 0 {
		err := f.scopeErrors[0]
		f.scopeErrors = f.scopeErrors[1:]
		return err
	}
	return nil
}

func (f *fakeGitHubEnv) sync(context.Context) hostapi.GitHubSyncOutcome {
	f.record("sync")
	require.NotEmpty(f.t, f.syncs, "unexpected sync")
	next := f.syncs[0]
	f.syncs = f.syncs[1:]
	return next
}

func (f *fakeGitHubEnv) webhookURL() string {
	return githubWebhook
}

func (f *fakeGitHubEnv) confirmWebhookReplace(_ context.Context, current string) (bool, error) {
	f.record(fmt.Sprintf("replace webhook %s?", current))
	if f.cancelWebhookReplace {
		return false, fmt.Errorf("failed to confirm the GitHub App webhook: %w", errs.ErrCancelled)
	}
	return f.replaceWebhook, nil
}

func (f *fakeGitHubEnv) pointWebhook(context.Context) error {
	f.record("point webhook")
	return nil
}

func (f *fakeGitHubEnv) webhookElsewhereOverride() string {
	return f.webhookElsewhere
}

func (f *fakeGitHubEnv) forgetKeptWebhooks(context.Context) error {
	f.record("forget kept webhooks")
	return nil
}

func (f *fakeGitHubEnv) openInstallPage(url string) error {
	f.record("open install " + url)
	return nil
}

func (f *fakeGitHubEnv) wait(context.Context, time.Duration) waitTick {
	if len(f.ticks) == 0 {
		return waitElapsed
	}
	tick := f.ticks[0]
	f.ticks = f.ticks[1:]
	return tick
}

const (
	githubReady = "success: GitHub channel ready · App acme-triage (acme) · installed on acme/api"
	githubSetUp = "success: The GitHub App is set up for this agent."
	// githubToken is the token summary once the agent's token has what the
	// channel needs.
	githubToken = "info: The agent's GitHub token has:\n  issues  write  from app.permissions"
)

func TestAnInstalledAppSyncsAndReportsWithoutPrompting(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{"sync", githubReady, githubToken}, env.log)
}

func TestWithoutATerminalSetupPrintsTheLinkAndWaits(t *testing.T) {
	// The fake's owner stands in for MDA_GITHUB_APP_OWNER.
	env := &fakeGitHubEnv{
		t:         t,
		envSource: sourceOf(githubAppNew),
		polls:     []hostapi.OAuthAuthorizationSession{githubSessionWith("completed")},
		syncs:     []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))},
	}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"owner",
		`create acme-triage for Organization("acme")`,
		"open https://aa.example/v1/agent-auth/github-app/start?state=s creating=true",
	}, env.log[:3])
	assert.Contains(t, env.log, githubSetUp)
}

func TestEnvCredentialsSkipTheAppPromptInATerminalToo(t *testing.T) {
	for source, step := range map[githubAppSource]string{githubAppNew: "owner", githubAppExisting: "existing"} {
		env := interactiveGitHubEnv(t)
		env.envSource = sourceOf(source)
		env.started = sessionOf(githubSessionWith("completed"))
		env.replaceWebhook = true
		env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true)), synced(githubStatus(t, true))}

		require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

		assert.NotContains(t, env.log, "choose")
		assert.Equal(t, step, env.log[0], "%v", env.log)
	}
}

func TestWithoutATerminalAMissingOwnerFailsBeforeAnythingIsCreated(t *testing.T) {
	env := &fakeGitHubEnv{t: t, ownerError: fmt.Errorf("%s", githubOwnerRequired)}

	err := runGitHubSetup(t.Context(), env, setupChannel())

	require.EqualError(t, err, githubOwnerRequired)
	// No question, no retry prompt and no pending connection left behind.
	assert.Equal(t, []string{"owner"}, env.log)
}

func TestTheOwnerSettingNamesAnOrganizationOrThePersonalAccount(t *testing.T) {
	owner, err := parseOwner(" acme ")
	require.NoError(t, err)
	assert.Equal(t, appOwner{organization: "acme"}, owner)
	owner, err = parseOwner("@me")
	require.NoError(t, err)
	assert.Equal(t, appOwner{}, owner)
	for _, invalid := range []string{"", "-acme", "acme/web", "@you"} {
		_, err := parseOwner(invalid)
		require.ErrorContains(t, err, "MDA_GITHUB_APP_OWNER must be a GitHub organization name or @me.", invalid)
	}
}

func TestANewAppIsCreatedAndInstalledThroughOneSession(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.source = sourceOf(githubAppNew)
	env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("pending"), githubSessionWith("completed")}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"choose",
		"owner",
		`create acme-triage for Organization("acme")`,
		"open https://aa.example/v1/agent-auth/github-app/start?state=s creating=true",
		"poll " + githubSession,
		"poll " + githubSession,
		githubSetUp,
		"sync",
		githubReady,
		githubToken,
	}, env.log)
}

func TestAnAbandonedSetupResumesOnItsConnection(t *testing.T) {
	for _, tc := range []struct {
		state    hostapi.GitHubAppState
		start    string
		creating bool
	}{
		{hostapi.GitHubAppStatePending, `Create { organization: Some("acme") }`, true},
		{hostapi.GitHubAppStateCreated, "Install", false},
	} {
		env := interactiveGitHubEnv(t)
		env.connection = appConnection(t, tc.state, false)
		env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("completed")}
		env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}

		require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

		assert.NotContains(t, env.log, "choose", tc.start)
		assert.Contains(t, env.log, fmt.Sprintf("start %s %s", githubAppRowID, tc.start))
		assert.True(t, slices.ContainsFunc(env.log, func(line string) bool {
			return strings.HasSuffix(line, fmt.Sprintf("creating=%t", tc.creating))
		}), "%v", env.log)
	}
}

func TestAnExistingAppCompletesWithTheInstallationGitHubReports(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.source = sourceOf(githubAppExisting)
	env.started = sessionOf(githubSessionWith("completed"))
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true)), synced(githubStatus(t, true))}
	env.replaceWebhook = true

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"choose",
		"existing",
		"sync",
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		githubSetUp,
	}, env.log[:5])
	// The status reported no bot ID, so deploy says why it could not store
	// one.
	assert.True(t, slices.ContainsFunc(env.log, func(line string) bool {
		return strings.HasPrefix(line, "warning: Could not store the GitHub App's bot ID")
	}), "%v", env.log)
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.Contains(line, ".env") }), "%v", env.log)
}

func TestABroughtAppGetsItsBotIDFromTheReport(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = brought(t, true)
	env.connection.webhookSecretStored = true
	reported := githubStatus(t, true)
	reported.App.BotID = 4242
	env.syncs = []hostapi.GitHubSyncOutcome{synced(reported)}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{"sync", githubReady, "store " + botIDSuffix, githubToken}, env.log)
}

func TestABroughtAppWithoutAStoredWebhookSecretGetsOne(t *testing.T) {
	// Set up before deploy stored brought Apps' secrets.
	env := interactiveGitHubEnv(t)
	env.connection = brought(t, true)
	env.connection.botIDStored = true
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, []string{"webhook secret", "store " + webhookSecretSuffix, githubToken}, env.log[len(env.log)-3:])

	// Nobody to ask: the deploy still succeeds and says what to do.
	unattendedEnv := &fakeGitHubEnv{t: t, connection: brought(t, true), syncs: []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}}
	unattendedEnv.connection.botIDStored = true
	require.NoError(t, runGitHubSetup(t.Context(), unattendedEnv, setupChannel()))
	assert.False(t, slices.ContainsFunc(unattendedEnv.log, func(line string) bool { return strings.HasPrefix(line, "store") }))
	assert.Contains(t, unattendedEnv.log, "warning: "+missingWebhookSecret)
}

func TestAFailedOrExpiredSessionOffersRetry(t *testing.T) {
	for _, status := range []string{"failed", "expired"} {
		env := interactiveGitHubEnv(t)
		env.source = sourceOf(githubAppNew)
		env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith(status)}
		env.retries = []bool{false}

		require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

		assert.True(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "retry? ") }), status)
		assert.Equal(t, "warning: "+githubSkippedSetup, env.log[len(env.log)-1], status)
	}
}

func TestAppsAgentAuthCouldNotStoreAreNamedForDeletion(t *testing.T) {
	// GitHub created Apps that Agent Auth could not store, on this attempt
	// and an earlier one: the pending app lists them, oldest first.
	pending := appConnection(t, hostapi.GitHubAppStatePending, false)
	for _, slug := range []string{"acme-triage", "acme-triage-2"} {
		pending.app.StrandedApps = append(pending.app.StrandedApps, hostapi.AppMetadata{Slug: slug, HTMLURL: "https://github.com/apps/" + slug})
	}
	env := interactiveGitHubEnv(t)
	env.connection = pending
	env.polls = []hostapi.OAuthAuthorizationSession{failedSession("app_store_failed")}
	env.retries = []bool{false}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	index := slices.IndexFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "retry? ") })
	require.GreaterOrEqual(t, index, 0, "retry prompt")
	retry := env.log[index]
	assert.Contains(t, retry, "Agent Auth could not store it. Delete it on GitHub")
	assert.True(t, strings.HasSuffix(retry, "\nhttps://github.com/apps/acme-triage\nhttps://github.com/apps/acme-triage-2"), retry)
}

func TestFailureMessagesForEachErrorCode(t *testing.T) {
	assert.True(t, strings.HasPrefix(setupFailureMessage("app_creation_denied", nil), "GitHub did not create"))
	assert.Contains(t, setupFailureMessage("conversion_failed", nil), "one-time code")
	assert.True(t, strings.HasPrefix(setupFailureMessage("connection_failed", nil), "Agent Auth could not look up the GitHub App"))
	assert.Equal(t, "The App was created on GitHub, but Agent Auth could not store it. Delete it on GitHub before trying again.",
		setupFailureMessage("app_store_failed", nil))
	assert.True(t, strings.HasSuffix(setupFailureMessage("app_store_failed", []string{"https://github.com/apps/acme-triage"}),
		"before trying again.\nhttps://github.com/apps/acme-triage"))
	assert.Equal(t, "GitHub App setup failed (new_code).", setupFailureMessage("new_code", nil))
	assert.True(t, strings.HasPrefix(setupFailureMessage("", nil), "GitHub App setup failed."))
}

func TestARetryStartsANewSessionOnTheAppItCreated(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.source = sourceOf(githubAppNew)
	env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("expired"), githubSessionWith("completed")}
	env.retries = []bool{true}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	created := 0
	for _, line := range env.log {
		if strings.HasPrefix(line, "create ") {
			created++
		}
	}
	assert.Equal(t, 1, created, "%v", env.log)
	assert.Contains(t, env.log, fmt.Sprintf(`start %s Create { organization: Some("acme") }`, githubAppRowID))
	assert.Equal(t, []string{githubReady, githubToken}, env.log[len(env.log)-2:])
}

func broughtAndStarted(t *testing.T) *fakeGitHubEnv {
	env := interactiveGitHubEnv(t)
	env.connection = brought(t, false)
	env.connection.webhookSecretStored = true
	env.started = sessionOf(githubSessionWith("completed"))
	return env
}

// uninstalledBroughtApp is a brought App that GitHub reports with no
// installation.
func uninstalledBroughtApp(t *testing.T) *fakeGitHubEnv {
	env := interactiveGitHubEnv(t)
	env.connection = brought(t, false)
	env.connection.webhookSecretStored = true
	return env
}

func TestAnUninstalledBroughtAppInstallsThroughAnInstallSession(t *testing.T) {
	env := uninstalledBroughtApp(t)
	env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("completed")}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, false)), synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"sync",
		fmt.Sprintf("start %s Install", githubAppRowID),
		"open install https://aa.example/v1/agent-auth/github-app/start?state=s",
		"status " + githubSession,
	}, env.log[:4])
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.Contains(line, "Installation(") }))
}

func TestAnInstallGitHubReportsFirstIsRecordedWithoutTheSession(t *testing.T) {
	// An App whose setup URL does not lead back to Agent Auth never completes
	// the install session.
	env := uninstalledBroughtApp(t)
	env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("pending")}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, false)), synced(githubStatus(t, true)), synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"sync",
		fmt.Sprintf("start %s Install", githubAppRowID),
		"open install https://aa.example/v1/agent-auth/github-app/start?state=s",
		"status " + githubSession,
		"sync",
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
	}, env.log[:6])
}

func TestCtrlCWhileABroughtAppInstallsSkipsTheChannel(t *testing.T) {
	env := uninstalledBroughtApp(t)
	env.ticks = []waitTick{waitInterrupted}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, false))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, "warning: "+githubSkippedSetup, env.log[len(env.log)-1])
}

func twoInstallations(t *testing.T, otherLogin string) *hostapi.GitHubChannelStatus {
	several := githubStatus(t, true)
	other := several.Installations[0]
	other.ID = 10
	if otherLogin != "" {
		other.Account.Login = otherLogin
	}
	several.Installations = append(several.Installations, other)
	return several
}

func TestAnExistingAppInSeveralAccountsLetsTheUserPickOne(t *testing.T) {
	env := broughtAndStarted(t)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(twoInstallations(t, "octo")), synced(twoInstallations(t, "octo"))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"sync",
		"choose installation 9,10",
		fmt.Sprintf(`start %s Installation("10")`, githubAppRowID),
	}, env.log[:3])
}

func TestAnExistingAppInSeveralAccountsTakesTheInstallationFromTheEnv(t *testing.T) {
	env := broughtAndStarted(t)
	require.True(t, env.isInteractive)
	env.installationEnv = "10"
	env.syncs = []hostapi.GitHubSyncOutcome{synced(twoInstallations(t, ""))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	// In a terminal too, MDA_GITHUB_APP_INSTALLATION_ID picks it: nobody is
	// asked.
	assert.Equal(t, fmt.Sprintf(`start %s Installation("10")`, githubAppRowID), env.log[0])
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "choose installation") }), "%v", env.log)
}

func TestAnExistingAppAsksForItsInstallationOnlyWhenGitHubCannotBeAsked(t *testing.T) {
	env := broughtAndStarted(t)
	for range syncAttempts {
		env.syncs = append(env.syncs, unavailable("timed out"))
	}
	env.syncs = append(env.syncs, synced(githubStatus(t, true)))

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	asked := slices.Index(env.log, "installation")
	require.GreaterOrEqual(t, asked, 0)
	syncs := 0
	for _, line := range env.log[:asked] {
		if line == "sync" {
			syncs++
		}
	}
	assert.Equal(t, syncAttempts, syncs)
	assert.Equal(t, fmt.Sprintf(`start %s Installation("777")`, githubAppRowID), env.log[asked+1])
}

func TestCtrlCWhileWaitingSkipsWithoutAsking(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, false)
	env.interruptPolls = true

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, "warning: "+githubSkippedSetup, env.log[len(env.log)-1])
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "retry?") }))
}

func TestUninstalledAppWaitsForInstallationWhenAgentAuthReports(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, false))}
	env.ticks = []waitTick{waitInterrupted}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"sync",
		"open install https://github.com/apps/acme-triage/installations/new",
		"warning: Skipped waiting for the GitHub App to be installed.",
		"warning: GitHub App acme-triage is not installed yet. Install it on your repositories at https://github.com/apps/acme-triage/installations/new",
	}, env.log)
}

func TestTheManifestComesFromTheChannel(t *testing.T) {
	manifest := githubAppManifest(setupChannel(), "https://agent.example", "https://agent.example/channels/github/events")
	assert.Equal(t, "acme-triage", manifest.Name)
	assert.Equal(t, "https://agent.example", manifest.URL)
	assert.Equal(t, []string{"issues"}, manifest.Events)
	assert.Equal(t, map[string]string{"issues": "write", "metadata": "read"}, manifest.Permissions)
	assert.Equal(t, "mda/github-app/"+githubDeployment, githubSlugs.connection(githubDeployment))
	assert.Equal(t, []string{"issues:write", "metadata:read"}, permissionScopes(setupChannel()))
}

func TestGitHubIDParsing(t *testing.T) {
	id, err := parseGitHubID(" 123 ", githubAppIDHint)
	require.NoError(t, err)
	assert.Equal(t, uint64(123), id)
	for _, invalid := range []string{"", "0", "-1", "12a", "+5"} {
		_, err := parseGitHubID(invalid, githubAppIDHint)
		require.EqualError(t, err, githubAppIDHint, invalid)
	}
	// Each prompt says which ID it wants.
	_, err = parseGitHubID("abc", githubInstallationIDHint)
	require.EqualError(t, err, githubInstallationIDHint)
}

func TestPrivateKeyFileReading(t *testing.T) {
	const pem = "-----BEGIN RSA PRIVATE KEY-----\nMIIabc\n-----END RSA PRIVATE KEY-----\n"
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	require.NoError(t, os.WriteFile(key, []byte(pem), 0o600))
	parsed, err := readPrivateKey(key)
	require.NoError(t, err)
	assert.Equal(t, pem, parsed.Expose())

	notKey := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(notKey, []byte("hello"), 0o600))
	for _, path := range []string{notKey, filepath.Join(dir, "missing.pem"), dir} {
		_, err := readPrivateKey(path)
		require.Error(t, err, path)
	}
}

func TestGitHubOwnerFromRemoteURLs(t *testing.T) {
	for remote, owner := range map[string]string{
		"git@github.com:acme/agent.git":           "acme",
		"https://github.com/acme/agent":           "acme",
		"ssh://git@github.com/acme-inc/agent.git": "acme-inc",
		"https://gitlab.com/acme/agent":           "",
		"git@github.com:-bad/agent.git":           "",
	} {
		parsed, ok := parseGitHubOwner(remote)
		assert.Equal(t, owner, parsed, remote)
		assert.Equal(t, owner != "", ok, remote)
	}
}

func TestAChannelWithoutHandlersSkipsAppSetup(t *testing.T) {
	env := interactiveGitHubEnv(t)
	receiptOnly := setupChannel()
	receiptOnly.handled = nil
	receiptOnly.events = nil

	require.NoError(t, runGitHubSetup(t.Context(), env, receiptOnly))

	assert.Equal(t, []string{"info: " + githubNoHandlers}, env.log)
}

func TestSyncRetriesWhileAgentAuthIsUnavailable(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.syncs = []hostapi.GitHubSyncOutcome{unavailable("timeout"), synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{"sync", "info: Retrying the GitHub App check…", "sync", githubReady, githubToken}, env.log)
}

func TestARetryPicksUpAnAppGitHubCreatedMeanwhile(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStatePending, false)
	env.polls = []hostapi.OAuthAuthorizationSession{githubSessionWith("expired"), githubSessionWith("completed")}
	env.retries = []bool{true}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}
	// Agent Auth filled the provider before the first session expired.
	env.afterFailure = appConnection(t, hostapi.GitHubAppStateCreated, false)

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	var starts []string
	for _, line := range env.log {
		if strings.HasPrefix(line, "start ") {
			starts = append(starts, line)
		}
	}
	require.Len(t, starts, 2, "%v", env.log)
	assert.True(t, strings.HasSuffix(starts[1], " Install"), "%v", env.log)
}

// drifted is an installed App whose token was minted for fewer permissions
// than the channel now needs.
func drifted(t *testing.T) *fakeGitHubEnv {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.connection.scopes = []string{"issues:read", "metadata:read"}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}
	return env
}

func TestAGrantedPermissionChangeRenewsTheToken(t *testing.T) {
	env := drifted(t)
	env.started = sessionOf(githubSessionWith("completed"))

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		"sync",
		githubReady,
		fmt.Sprintf("scopes %s issues:write,metadata:read", githubProviderID),
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		"info: The agent's GitHub token now has:\n  issues  write  from app.permissions",
	}, env.log)
}

func TestAnUngrantedPermissionChangeKeepsTheToken(t *testing.T) {
	env := drifted(t)
	ungranted := githubStatus(t, true)
	ungranted.Installations[0].Permissions["issues"] = "read"
	env.syncs = []hostapi.GitHubSyncOutcome{synced(ungranted)}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "scopes") }), "%v", env.log)
	assert.Equal(t, strings.Join([]string{
		"info: The agent's GitHub token has:",
		"  issues  read   from app.permissions",
		"",
		"Waiting for GitHub:",
		"  issues  write  from app.permissions",
		"",
		"1. An owner of acme accepts them at https://github.com/organizations/acme/settings/installations/9",
		"2. Run `langsmith deploy` again.",
	}, "\n"), env.log[len(env.log)-1])
}

func TestAFailedTokenRenewalOnlyWarns(t *testing.T) {
	// The scopes could not change, so the token is as it was.
	env := drifted(t)
	env.scopeErrors = []error{fmt.Errorf("Agent Auth unavailable")}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, "warning: Could not update the agent's GitHub token (Agent Auth unavailable), so it keeps its current permissions. Run `langsmith deploy` again to retry.",
		env.log[len(env.log)-1])
}

func TestAFailedTokenRenewalRestoresTheOldScopes(t *testing.T) {
	// Changing the scopes dropped the token: the old scopes go back and the
	// token is renewed with them, so the next deploy retries the change.
	env := drifted(t)
	env.startErrors = []error{fmt.Errorf("session failed")}
	env.started = sessionOf(githubSessionWith("completed"))

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		fmt.Sprintf("scopes %s issues:write,metadata:read", githubProviderID),
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		fmt.Sprintf("scopes %s issues:read,metadata:read", githubProviderID),
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		"warning: Could not update the agent's GitHub token (session failed), so it keeps its current permissions. Run `langsmith deploy` again to retry.",
	}, env.log[2:])

	// Restoring failed too: the warning says the agent has no token.
	env = drifted(t)
	env.started = sessionOf(githubSessionWith("pending"))
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, "warning: Could not update the agent's GitHub token (Agent Auth did not renew the token), and the agent has no GitHub token until it is renewed. Run `langsmith deploy` again to set it up.",
		env.log[len(env.log)-1])
}

// narrowed is an installed App whose token was minted for more permissions
// than the channel now needs: contents:write is dropped.
func narrowed(t *testing.T) *fakeGitHubEnv {
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.connection.scopes = []string{"contents:write", "issues:write", "metadata:read"}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}
	return env
}

func TestANarrowingDoesNotWaitForUngrantedPermissions(t *testing.T) {
	// The channel drops contents:write and adds pull_requests:write, which
	// the installation has not granted: the token narrows to what it has.
	wider := setupChannel()
	wider.permissions["pull_requests"] = permissionWrite
	env := narrowed(t)
	env.started = sessionOf(githubSessionWith("completed"))

	require.NoError(t, runGitHubSetup(t.Context(), env, wider))

	var token []string
	for _, line := range env.log {
		if strings.HasPrefix(line, "scopes ") || strings.HasPrefix(line, "start ") || strings.Contains(line, "agent's GitHub token") {
			token = append(token, line)
		}
	}
	assert.Equal(t, []string{
		fmt.Sprintf("scopes %s issues:write,metadata:read", githubProviderID),
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		strings.Join([]string{
			"info: The agent's GitHub token now has:",
			"  issues         write  from app.permissions",
			"",
			"Waiting for GitHub:",
			"  pull_requests  write  from app.permissions",
			"",
			"1. The App owner grants pull_requests (write) at https://github.com/organizations/acme/settings/apps/acme-triage/permissions",
			"2. An owner of acme accepts them at https://github.com/organizations/acme/settings/installations/9",
			"3. Run `langsmith deploy` again.",
		}, "\n"),
	}, token)
}

func TestAFailedNarrowingNeverRestoresTheBroaderToken(t *testing.T) {
	env := narrowed(t)
	env.startErrors = []error{fmt.Errorf("session failed")}
	env.started = sessionOf(githubSessionWith("completed"))

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		fmt.Sprintf("scopes %s issues:write,metadata:read", githubProviderID),
		fmt.Sprintf(`start %s Installation("9")`, githubAppRowID),
		"warning: Could not update the agent's GitHub token (session failed), and the agent has no GitHub token until it is renewed. Run `langsmith deploy` again to set it up.",
	}, env.log[2:])

	// Agent Auth refused the narrower scopes, so the old token is still
	// there, and the warning says it is broader.
	env = narrowed(t)
	env.scopeErrors = []error{fmt.Errorf("Agent Auth unavailable")}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, "warning: Could not update the agent's GitHub token (Agent Auth unavailable), so it still has its previous, broader permissions. Run `langsmith deploy` again to retry.",
		env.log[len(env.log)-1])
}

func TestANarrowingDropsTheTokenItCannotRenew(t *testing.T) {
	// With two installations there is no one installation to mint for, so
	// the broader token goes and none replaces it.
	env := narrowed(t)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(twoInstallations(t, ""))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{
		fmt.Sprintf("scopes %s issues:write,metadata:read", githubProviderID),
		"warning: The GitHub App is installed more than once, so the agent's GitHub token cannot be renewed with fewer permissions, and it may not keep its broader ones, so the agent has no GitHub token for now. Run `langsmith deploy` again once that is fixed.",
	}, env.log[2:])

	// The installation grants none of the narrower permissions either.
	env = narrowed(t)
	bare := githubStatus(t, true)
	bare.Installations[0].Permissions = map[string]string{}
	env.syncs = []hostapi.GitHubSyncOutcome{synced(bare)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "start ") }), "%v", env.log)
	assert.Contains(t, env.log[len(env.log)-1], "the agent has no GitHub token")
}

func TestSeveralInstallationsKeepTheToken(t *testing.T) {
	env := drifted(t)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(twoInstallations(t, ""))}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "scopes") }), "%v", env.log)
	assert.Contains(t, env.log[len(env.log)-1], "installed more than once")
}

const testKey = "-----BEGIN RSA PRIVATE KEY-----\nMIIB\n-----END RSA PRIVATE KEY-----"

func existingEnv(t *testing.T) *existingAppEnv {
	key, err := hostapi.ParsePrivateKeyPem(testKey)
	require.NoError(t, err)
	return &existingAppEnv{appID: "42", privateKey: key, webhookSecret: "whsec"}
}

func TestUnattendedSetupNeedsAnOwnerOrAnExistingAppOnlyForANewApp(t *testing.T) {
	none := githubAppEnv{}
	owner := githubAppEnv{owner: &appOwner{}}
	existing := githubAppEnv{existing: existingEnv(t)}
	installation := githubAppEnv{installationID: "777"}
	problem := unattendedSetupProblem

	// No deployment or connection yet: a new App needs an owner, or the env
	// brings an existing one.
	assert.Equal(t, githubOwnerRequired, problem(nil, none))
	assert.Empty(t, problem(nil, owner))
	assert.Empty(t, problem(nil, existing))
	pending := appConnection(t, hostapi.GitHubAppStatePending, false)
	assert.Equal(t, githubOwnerRequired, problem(pending, none))
	assert.Empty(t, problem(pending, owner))
	// A created App only needs its install link.
	assert.Empty(t, problem(appConnection(t, hostapi.GitHubAppStateCreated, false), none))
	// A brought App needs its installation, from a prompt or the env.
	external := appConnection(t, hostapi.GitHubAppStateExternal, false)
	assert.Equal(t, existingAppNeedsTerminal, problem(external, owner))
	assert.Empty(t, problem(external, installation))
	for _, state := range []hostapi.GitHubAppState{hostapi.GitHubAppStatePending, hostapi.GitHubAppStateCreated, hostapi.GitHubAppStateExternal} {
		assert.Empty(t, problem(appConnection(t, state, true), none))
	}
}

func TestTheAppEnvVarsAreCheckedAsAGroupAndNeverEchoValues(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	key := filepath.Join(dir, "app.pem")
	require.NoError(t, os.WriteFile(key, []byte(testKey), 0o600))
	notAKey := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(notAKey, []byte("hello"), 0o600))

	empty, err := readGitHubAppEnv(map[string]string{})
	require.NoError(t, err)
	assert.Nil(t, empty.existing)
	broughtEnv, err := readGitHubAppEnv(map[string]string{
		GitHubAppIDEnv:          "42",
		GitHubPrivateKeyFileEnv: key,
		GitHubWebhookSecretEnv:  "whsec-value",
		GitHubInstallationIDEnv: "777",
	})
	require.NoError(t, err)
	require.NotNil(t, broughtEnv.existing)
	assert.Equal(t, "42", broughtEnv.existing.appID)
	assert.Equal(t, "whsec-value", broughtEnv.existing.webhookSecret)
	assert.Equal(t, "777", broughtEnv.installationID)
	assert.NotContains(t, fmt.Sprintf("%v %+v %#v", *broughtEnv.existing, *broughtEnv.existing, broughtEnv.existing), "whsec-value")

	for _, tc := range []struct {
		vars  map[string]string
		names string
	}{
		{map[string]string{GitHubAppIDEnv: "42"}, GitHubPrivateKeyFileEnv},
		{map[string]string{GitHubAppIDEnv: "42", GitHubPrivateKeyFileEnv: key}, GitHubWebhookSecretEnv},
		{map[string]string{GitHubAppIDEnv: "app", GitHubPrivateKeyFileEnv: key, GitHubWebhookSecretEnv: "whsec-value"}, GitHubAppIDEnv},
		{map[string]string{GitHubAppIDEnv: "42", GitHubPrivateKeyFileEnv: notAKey, GitHubWebhookSecretEnv: "whsec-value"}, GitHubPrivateKeyFileEnv},
		{map[string]string{GitHubInstallationIDEnv: "x9"}, GitHubInstallationIDEnv},
		{map[string]string{GitHubAppOwnerEnv: "acme", GitHubAppIDEnv: "42", GitHubPrivateKeyFileEnv: key, GitHubWebhookSecretEnv: "whsec-value"}, GitHubAppOwnerEnv},
	} {
		_, err := readGitHubAppEnv(tc.vars)
		require.ErrorContains(t, err, tc.names)
		assert.NotContains(t, err.Error(), "whsec-value")
		assert.NotContains(t, err.Error(), "hello")
	}
}

func TestTheCLIWebhookSecretIsNeverUploadedButTheRuntimeOverrideIs(t *testing.T) {
	envVars := map[string]string{
		GitHubWebhookSecretEnv:  "cli",
		GitHubAppIDEnv:          "42",
		GitHubPrivateKeyFileEnv: "app.pem",
		GitHubInstallationIDEnv: "7",
		"GH_WEBHOOK_SECRET":     "runtime",
	}
	var names []string
	for _, secret := range deployment.SecretsFromEnv(envVars) {
		names = append(names, secret.Name)
	}
	assert.Equal(t, []string{"GH_WEBHOOK_SECRET"}, names)
}

// webhookAt is a brought App, installed, whose webhook the report says
// points at url.
func webhookAt(t *testing.T, url string) (*fakeGitHubEnv, *hostapi.GitHubChannelStatus) {
	env := interactiveGitHubEnv(t)
	env.connection = brought(t, true)
	env.connection.webhookSecretStored = true
	env.connection.botIDStored = true
	reported := githubStatus(t, true)
	reported.App.WebhookURL = url
	return env, reported
}

func TestABroughtAppsWebhookMovesOnlyWithConsent(t *testing.T) {
	env, elsewhere := webhookAt(t, otherWebhook)
	env.replaceWebhook = true
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, []string{"sync", fmt.Sprintf("replace webhook %s?", otherWebhook), "point webhook", githubReady, githubToken}, env.log)

	// Declined: deploy remembers to keep it there, and the report still runs.
	env, elsewhere = webhookAt(t, otherWebhook)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.NotContains(t, env.log, "point webhook")
	assert.Contains(t, env.log, "store "+webhookElsewhereSuffix+webhookDigest(otherWebhook))
	assert.True(t, slices.ContainsFunc(env.log, func(line string) bool {
		return strings.HasPrefix(line, "info:") && strings.Contains(line, "keeps sending its events to "+otherWebhook)
	}))
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "warning:") }))

	// Escape or Ctrl-C at the prompt keeps it there for now, without
	// remembering, rather than failing the live deploy.
	env, elsewhere = webhookAt(t, otherWebhook)
	env.cancelWebhookReplace = true
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.NotContains(t, env.log, "point webhook")
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "store") }))
	assert.True(t, slices.ContainsFunc(env.log, func(line string) bool {
		return strings.HasPrefix(line, "warning:") && strings.Contains(line, "keeps sending its events to "+otherWebhook)
	}))

	// Already pointing here: nothing to do. Unset: nobody to take it from.
	env, here := webhookAt(t, githubWebhook)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(here)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, []string{"sync", githubReady, githubToken}, env.log)
	env, unset := webhookAt(t, "")
	env.syncs = []hostapi.GitHubSyncOutcome{synced(unset)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Equal(t, []string{"sync", "point webhook", githubReady, githubToken}, env.log)
}

func TestAnAppAgentAuthPointsHereFollowsWithoutAsking(t *testing.T) {
	// Someone moved the webhook of an App Agent Auth created for this
	// deployment; deploy points it back, also without a terminal.
	env := &fakeGitHubEnv{t: t, connection: appConnection(t, hostapi.GitHubAppStateCreated, true)}
	moved := githubStatus(t, true)
	moved.App.WebhookURL = otherWebhook
	env.syncs = []hostapi.GitHubSyncOutcome{synced(moved)}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	assert.Equal(t, []string{"sync", "point webhook", githubReady, githubToken}, env.log)
}

func TestWithoutATerminalABroughtAppsOtherWebhookFailsTheDeploy(t *testing.T) {
	env, elsewhere := webhookAt(t, otherWebhook)
	env.isInteractive = false
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}

	err := runGitHubSetup(t.Context(), env, setupChannel())

	require.EqualError(t, err, webhookElsewhere("acme-triage", otherWebhook, githubWebhook))
	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "replace webhook") }))
}

// keptAt is [webhookAt] for an App an earlier deploy kept at kept.
func keptAt(t *testing.T, url, kept string) (*fakeGitHubEnv, *hostapi.GitHubChannelStatus) {
	env, reported := webhookAt(t, url)
	env.connection.keptWebhooks = map[string]bool{webhookDigest(kept): true}
	return env, reported
}

func TestAKeptWebhookIsNotAskedAboutAgain(t *testing.T) {
	// With a terminal and without: no question, no failure, one line saying
	// where to forward the events.
	for _, interactive := range []bool{true, false} {
		env, elsewhere := keptAt(t, otherWebhook, otherWebhook)
		env.isInteractive = interactive
		env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
		require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
		assert.False(t, slices.ContainsFunc(env.log, func(line string) bool {
			return strings.HasPrefix(line, "replace webhook") || strings.HasPrefix(line, "store") || line == "point webhook"
		}))
		assert.True(t, slices.ContainsFunc(env.log, func(line string) bool {
			return strings.HasPrefix(line, "info:") && strings.Contains(line, "Forward the ones this agent handles to "+githubWebhook)
		}))
	}

	// The webhook moved to yet another receiver: that one wasn't agreed to.
	env, elsewhere := keptAt(t, "https://new.example/hook", otherWebhook)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Contains(t, env.log, "replace webhook https://new.example/hook?")
}

func TestAKeptWebhookIsForgottenOnceItPointsHere(t *testing.T) {
	// Someone pointed it here on GitHub.
	env, here := keptAt(t, githubWebhook, otherWebhook)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(here)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.Contains(t, env.log, "forget kept webhooks")

	// Deploy moves it here with consent.
	env, elsewhere := keptAt(t, "https://new.example/hook", otherWebhook)
	env.replaceWebhook = true
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	point := slices.Index(env.log, "point webhook")
	forget := slices.Index(env.log, "forget kept webhooks")
	assert.GreaterOrEqual(t, point, 0, "%v", env.log)
	assert.Greater(t, forget, point, "%v", env.log)

	// Nothing kept: nothing to forget.
	env, here = webhookAt(t, githubWebhook)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(here)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	assert.NotContains(t, env.log, "forget kept webhooks")
}

func TestTheEnvKeepsAWebhookWithoutATerminal(t *testing.T) {
	env, elsewhere := webhookAt(t, otherWebhook)
	env.isInteractive = false
	env.webhookElsewhere = " " + otherWebhook + " "
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))
	// Remembered, so later deploys need no env.
	assert.Contains(t, env.log, "store "+webhookElsewhereSuffix+webhookDigest(otherWebhook))
	assert.NotContains(t, env.log, "point webhook")

	// It names one URL; another one still fails without a terminal.
	env, elsewhere = webhookAt(t, "https://new.example/hook")
	env.isInteractive = false
	env.webhookElsewhere = otherWebhook
	env.syncs = []hostapi.GitHubSyncOutcome{synced(elsewhere)}
	require.EqualError(t, runGitHubSetup(t.Context(), env, setupChannel()),
		webhookElsewhere("acme-triage", "https://new.example/hook", githubWebhook))
}

func TestAWebhookDigestIsShortAndStable(t *testing.T) {
	digest := webhookDigest(otherWebhook)
	assert.Len(t, digest, webhookDigestLength)
	assert.Regexp(t, "^[0-9a-f]+$", digest)
	assert.Equal(t, digest, webhookDigest(" "+otherWebhook+"\n"))
	assert.NotEqual(t, digest, webhookDigest(githubWebhook))
}

func TestARefusedFirstTokenSaysHowToFixIt(t *testing.T) {
	env := interactiveGitHubEnv(t)
	env.source = sourceOf(githubAppExisting)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}
	env.startErrors = []error{&hostapi.APIError{
		Status: http.StatusBadGateway,
		Detail: "Request only permissions the GitHub App installation grants: " + permissionsRejected,
	}}
	env.retries = []bool{false}

	require.NoError(t, runGitHubSetup(t.Context(), env, setupChannel()))

	index := slices.IndexFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "retry? ") })
	require.GreaterOrEqual(t, index, 0, "retry prompt")
	prompt := env.log[index]
	assert.True(t, strings.HasPrefix(prompt, "retry? GitHub refused the agent's token. Grant what is missing"), prompt)
	assert.Contains(t, prompt, "remove the handlers or app.permissions")

	// Other failures, including other GitHub errors, keep their own message.
	require.EqualError(t, explainRefusedToken(fmt.Errorf("Agent Auth unavailable")), "Agent Auth unavailable")
	upstream := explainRefusedToken(&hostapi.APIError{Status: http.StatusBadGateway, Detail: "OAuth provider request failed"})
	assert.NotContains(t, upstream.Error(), "GitHub refused")
}

func TestRepositoriesTheInstallationLacksAreReportedAndLeaveTheToken(t *testing.T) {
	// Tokens reach what the installation does, so repositories never change
	// the token; deploy only says which ones to add.
	limited := setupChannel()
	limited.repositories = []string{"acme/api", "acme/web"}
	env := interactiveGitHubEnv(t)
	env.connection = appConnection(t, hostapi.GitHubAppStateCreated, true)
	env.syncs = []hostapi.GitHubSyncOutcome{synced(githubStatus(t, true))}

	require.NoError(t, runGitHubSetup(t.Context(), env, limited))

	assert.False(t, slices.ContainsFunc(env.log, func(line string) bool { return strings.HasPrefix(line, "scopes") }), "%v", env.log)
	assert.Contains(t, env.log, "warning: The installation on acme does not include acme/web, which this channel is limited to. Add them at https://github.com/organizations/acme/settings/installations/9")
}

func githubAppProviders() string {
	return `{
		"items": [
			{ "id": "p-1", "slug": "mda-github-app-dep-1", "client_id": "42", "grant_type": "github_app" },
			{ "id": "p-2", "slug": "mda-github-app-dep-2", "client_id": "43", "grant_type": "github_app" }
		],
		"next_cursor": null
	}`
}

func oauthConnection(slug, providerID string) string {
	return fmt.Sprintf(`{"id": "c-%s", "slug": %q, "credential": {"kind": "oauth2", "oauth_provider_id": %q}}`, slug, slug, providerID)
}

func connectionPage(items ...string) string {
	return `{"items": [` + strings.Join(items, ",") + `], "next_cursor": null}`
}

func TestAppHolderFindsTheDeploymentThatOwnsTheApp(t *testing.T) {
	agentAuth := serveSequence(t,
		reply{http.StatusOK, githubAppProviders()},
		reply{http.StatusOK, connectionPage(oauthConnection("mda/github-app/dep-2", "p-2"), oauthConnection("mda/github-app/dep-1", "p-1"))},
		reply{http.StatusOK, `[{"id": "app-1", "provider": "github", "source": "manifest", "slug": "mda/github-app/dep-1", "app_id": "42"}]`})

	deploymentID, app, found, err := appHolder(t.Context(), oauthClient(t, agentAuth.URL), "42")

	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "dep-1", deploymentID)
	assert.Equal(t, "app-1", app.ID)
	requests := agentAuth.received()
	require.Len(t, requests, 3)
	assert.Equal(t, "GET /v1/agent-auth/apps?owner_id=dep-1", requests[2].line())
	assert.Equal(t, "Bearer oauth-token", requests[2].header.Get("Authorization"))
}

func TestAppHolderNeedsTheDeploymentToOwnAnAppWithTheAppID(t *testing.T) {
	agentAuth := serveSequence(t,
		reply{http.StatusOK, githubAppProviders()},
		reply{http.StatusOK, connectionPage(oauthConnection("mda/github-app/dep-1", "p-1"))},
		reply{http.StatusOK, `[{"id": "app-1", "provider": "github", "source": "existing", "slug": "mda/github-app/dep-1", "app_id": "7"}]`})

	_, _, found, err := appHolder(t.Context(), oauthClient(t, agentAuth.URL), "42")

	require.NoError(t, err)
	assert.False(t, found)
}

func TestAppHolderIgnoresConnectionsDeployDidNotCreate(t *testing.T) {
	agentAuth := serveSequence(t,
		reply{http.StatusOK, githubAppProviders()},
		reply{http.StatusOK, connectionPage(oauthConnection("someone-else/github", "p-1"), oauthConnection("mda/github-app/dep-1/bot-id", "p-1"))})

	_, _, found, err := appHolder(t.Context(), oauthClient(t, agentAuth.URL), "42")

	require.NoError(t, err)
	assert.False(t, found)
	assert.Len(t, agentAuth.received(), 2)
}

func TestAppHolderStopsWhenNoProviderMintsForTheApp(t *testing.T) {
	agentAuth := serveSequence(t, reply{http.StatusOK, githubAppProviders()})

	_, _, found, err := appHolder(t.Context(), oauthClient(t, agentAuth.URL), "99")

	require.NoError(t, err)
	assert.False(t, found)
	assert.Len(t, agentAuth.received(), 1)
}

func TestAppTakenReadsTheConflictStatus(t *testing.T) {
	conflict := fmt.Errorf("failed to store the GitHub App: %w", &hostapi.APIError{
		Status: http.StatusConflict, Detail: "the GitHub App already belongs to another Agent Auth app",
	})
	invalid := &hostapi.APIError{Status: http.StatusUnprocessableEntity, Detail: "invalid private key"}

	assert.True(t, appTaken(conflict))
	assert.False(t, appTaken(invalid))
}

func TestUnattendedPreflightFailsForABroughtAppsOtherWebhook(t *testing.T) {
	unattended(t)
	app := `{"id": "` + githubAppRowID + `", "provider": "github", "source": "existing", "slug": "mda/github-app/dep-1",
		"app_id": "42", "connection_id": "` + githubConnection + `", "manifest": {"webhook_url": "` + otherWebhook + `"}}`
	agentAuth := serveSequence(t,
		reply{http.StatusOK, `[` + app + `]`},
		reply{http.StatusOK, connectionPage(
			`{"connection_id": "`+githubConnection+`", "slug": "mda/github-app/dep-1", "kind": "oauth2", "oauth_provider_id": "`+githubProviderID+`"}`,
			`{"connection_id": "c-secret", "slug": "mda/github-app/dep-1/webhook-secret", "kind": "secret"}`)},
		reply{http.StatusOK, `{"id": "` + githubProviderID + `", "allowed_scopes": ["metadata:read", "issues:write"]}`},
		reply{http.StatusOK, `{"connection_id": "` + githubConnection + `", "credential": {"credential_id": "cred-1"}}`},
		reply{http.StatusOK, `{"github": {"app": {"id": 42, "slug": "acme-triage", "owner": {"login": "acme", "type": "Organization"},
			"webhook_url": "` + otherWebhook + `"}, "installations": []}}`})

	err := preflightGitHub(t.Context(), oauthClient(t, agentAuth.URL), setupChannel(), "dep-1", "https://agent.example", map[string]string{})

	require.EqualError(t, err, webhookElsewhere("acme-triage", otherWebhook, githubWebhook))
	for _, request := range agentAuth.received() {
		assert.Equal(t, "Bearer oauth-token", request.header.Get("Authorization"), request.line())
	}
}

func TestPreflightWithoutADeploymentNeedsAnOwnerUnattended(t *testing.T) {
	unattended(t)
	isolateEnv(t)

	err := preflightGitHub(t.Context(), nil, setupChannel(), "", "", map[string]string{})
	require.EqualError(t, err, githubOwnerRequired)

	require.NoError(t, preflightGitHub(t.Context(), nil, setupChannel(), "", "", map[string]string{GitHubAppOwnerEnv: "@me"}))
}
