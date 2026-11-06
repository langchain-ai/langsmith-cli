package channels

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/deployment"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

func TestWebhookURLRequiresHTTPS(t *testing.T) {
	url, err := webhookURL("https://agent.example/", "/channels/github/events")
	require.NoError(t, err)
	assert.Equal(t, "https://agent.example/channels/github/events", url)
	for _, invalid := range []string{
		"http://agent.example",
		"https://user:pass@agent.example",
		"https://agent.example?x=1",
		"not a url",
	} {
		_, err := webhookURL(invalid, "/channels/github/events")
		assert.Error(t, err, invalid)
	}
}

func TestAgentAuthOverridesComeFromDotenvAndNeverBecomeSecrets(t *testing.T) {
	isolateEnv(t)
	envVars := map[string]string{
		AgentAuthURLEnv:    " http://localhost:1980 ",
		agentAuthAPIKeyEnv: "lsv2_local",
		agentAuthTenantEnv: "",
		OAuthCallbackEnv:   "http://localhost:21980/api/v1/agent-auth/oauth/callback",
		"OPENAI_API_KEY":   "sk-test",
	}
	value, ok := overrideValue(AgentAuthURLEnv, envVars)
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:1980", value)
	_, ok = overrideValue(agentAuthTenantEnv, envVars)
	assert.False(t, ok)

	var names []string
	for _, secret := range deployment.SecretsFromEnv(envVars) {
		names = append(names, secret.Name)
	}
	assert.Equal(t, []string{"OPENAI_API_KEY"}, names)
}

func TestAgentAuthOverrideSendsItsOwnCredentialToItsOwnLangSmith(t *testing.T) {
	isolateEnv(t)
	local := serveSequence(t, reply{http.StatusOK, `{"items":[],"next_cursor":null}`})
	deploy, err := hostapi.New(client.NewWithOptions(client.Options{APIKey: "lsv2_deploy", APIURL: "http://unused.invalid"}),
		"http://unused.invalid", "http://unused.invalid")
	require.NoError(t, err)

	agentAuth, err := agentAuthClient(deploy, lookupIn(map[string]string{
		AgentAuthURLEnv:    local.URL,
		agentAuthAPIKeyEnv: "lsv2_local",
		agentAuthTenantEnv: "local-tenant",
	}))
	require.NoError(t, err)
	_, err = agentAuth.ListConnectionsRaw(t.Context())
	require.NoError(t, err)

	request := local.received()[0]
	assert.Equal(t, "lsv2_local", request.header.Get("X-Api-Key"))
	assert.Equal(t, "local-tenant", request.header.Get("X-Tenant-Id"))

	// Without the override, Agent Auth calls go out on the deploy client.
	same, err := agentAuthClient(deploy, lookupIn(map[string]string{}))
	require.NoError(t, err)
	assert.Same(t, deploy, same)
}

func TestAgentAuthOverrideWithoutAnAPIKeyIsRefused(t *testing.T) {
	// No credential the CLI holds goes to that other LangSmith: not the
	// deploy's OAuth token, nor the shell's key.
	local := serveSequence(t)
	deploy := oauthClient(t, "http://unused.invalid")
	t.Setenv("LANGSMITH_API_KEY", "lsv2_shell")

	_, err := agentAuthClient(deploy, lookupIn(map[string]string{AgentAuthURLEnv: local.URL}))

	require.EqualError(t, err, "MDA_AGENT_AUTH_URL is set, so MDA_AGENT_AUTH_API_KEY must be set too")
	assert.Empty(t, local.received())
}

func lookupIn(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok && value != ""
	}
}

func TestAConfiguredOAuthCallbackIsChecked(t *testing.T) {
	configured := "http://localhost:21980/api/v1/agent-auth/oauth/callback"
	callback, err := configuredOAuthCallback(configured)
	require.NoError(t, err)
	assert.Equal(t, configured, callback)
	callback, err = configuredOAuthCallback("")
	require.NoError(t, err)
	assert.Empty(t, callback)
	for _, invalid := range []string{"not a url", "ftp://localhost/callback"} {
		_, err := configuredOAuthCallback(invalid)
		require.EqualError(t, err, "MDA_AGENT_AUTH_OAUTH_CALLBACK_URL must be an http(s) URL", invalid)
	}
}

func TestOnlyUnpublishedClientMetadataFallsBackToTheDefaultCallback(t *testing.T) {
	none := map[string]string{}

	// Published: used as is.
	published := serveSequence(t, reply{http.StatusOK, `{"redirect_uris":["https://api.smith.example/v1/agent-auth/oauth/callback"]}`})
	callback, err := oauthCallbackURL(t.Context(), oauthClient(t, published.URL), none)
	require.NoError(t, err)
	assert.Equal(t, "https://api.smith.example/v1/agent-auth/oauth/callback", callback)
	assert.Equal(t, []string{"GET /v1/agent-auth/oauth/client-metadata.json"}, published.lines())
	assert.Equal(t, "Bearer oauth-token", published.received()[0].header.Get("Authorization"))

	// Not published (an older Agent Auth): the callback on its API.
	unpublished := serveSequence(t, reply{http.StatusNotFound, `{}`})
	agentAuth := oauthClient(t, unpublished.URL)
	callback, err = oauthCallbackURL(t.Context(), agentAuth, none)
	require.NoError(t, err)
	fallback, err := agentAuth.OAuthRedirectURI()
	require.NoError(t, err)
	assert.Equal(t, fallback, callback)

	// Any other failure says nothing about the callback, so it fails.
	for _, status := range []int{http.StatusInternalServerError, http.StatusUnauthorized} {
		failing := serveSequence(t, reply{status, `{}`})
		_, err := oauthCallbackURL(t.Context(), oauthClient(t, failing.URL), none)
		require.ErrorContains(t, err, "failed to read Agent Auth's OAuth callback from its client metadata: ", "HTTP %d", status)
	}

	// The override wins without asking Agent Auth.
	callback, err = oauthCallbackURL(t.Context(), oauthClient(t, "http://unused.invalid"),
		map[string]string{OAuthCallbackEnv: "http://localhost:21980/api/v1/agent-auth/oauth/callback"})
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:21980/api/v1/agent-auth/oauth/callback", callback)
}

func TestEnvNoticesNameEveryVariableTheyUsed(t *testing.T) {
	assert.Equal(t, "A", joinNames([]string{"A"}))
	assert.Equal(t, "A and B", joinNames([]string{"A", "B"}))
	assert.Equal(t, "A, B and C", joinNames([]string{"A", "B", "C"}))
	assert.Equal(t,
		"Using the Slack app from X and Y. LangSmith keeps its credentials, so you can remove X and Y afterwards.",
		envAppNotice("Slack app", "LangSmith", []string{"X", "Y"}))
}

func TestSlugsNameTheDeploymentsAppAndItsSecrets(t *testing.T) {
	slugs := appSlugs{prefix: "mda/linear-app/"}
	assert.Equal(t, "mda/linear-app/d-1", slugs.connection("d-1"))
	assert.Equal(t, "mda/linear-app/d-1/webhook-secret", slugs.secret("d-1", webhookSecretSuffix))
	deploymentID, ok := slugs.deploymentOf("mda/linear-app/d-1")
	assert.True(t, ok)
	assert.Equal(t, "d-1", deploymentID)
	for _, other := range []string{
		"mda/linear-app/",
		"mda/linear-app/d-1/webhook-secret",
		"mda/github-app/d-1",
	} {
		_, ok := slugs.deploymentOf(other)
		assert.False(t, ok, other)
	}
}

// fakeUI records what setup showed and answers the retry prompt from a
// queue.
type fakeUI struct {
	isInteractive bool
	retries       []bool
	log           []string
}

func (u *fakeUI) interactive() bool {
	return u.isInteractive
}

func (u *fakeUI) retryOrSkip(_ context.Context, err error) (bool, error) {
	u.log = append(u.log, "retry? "+err.Error())
	retry := u.retries[0]
	u.retries = u.retries[1:]
	return retry, nil
}

func (u *fakeUI) success(message string) error {
	u.log = append(u.log, "success "+message)
	return nil
}

func (u *fakeUI) info(message string) error {
	u.log = append(u.log, "info "+message)
	return nil
}

func (u *fakeUI) warning(message string) error {
	u.log = append(u.log, "warning "+message)
	return nil
}

func TestAFailedStepIsRetriedFromWhatIsFoundThenOrSkipped(t *testing.T) {
	retried := &fakeUI{isInteractive: true, retries: []bool{true}}
	done, ok, err := retryOrSkip(t.Context(), retried, 0, "skipped",
		func(attempt int) (int, error) {
			if attempt == 0 {
				return 0, errors.New("first")
			}
			return attempt, nil
		},
		func() (int, error) { return 1, nil })
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, done)
	assert.Equal(t, []string{"retry? first"}, retried.log)

	skipped := &fakeUI{isInteractive: true, retries: []bool{false}}
	_, ok, err = retryOrSkip(t.Context(), skipped, struct{}{}, "skipped",
		func(struct{}) (struct{}, error) { return struct{}{}, errors.New("no") },
		func() (struct{}, error) { return struct{}{}, nil })
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"retry? no", "warning skipped"}, skipped.log)

	// Ctrl-C skips without asking.
	cancelled := &fakeUI{isInteractive: true}
	_, ok, err = retryOrSkip(t.Context(), cancelled, struct{}{}, "skipped",
		func(struct{}) (struct{}, error) { return struct{}{}, context.Canceled },
		func() (struct{}, error) { return struct{}{}, nil })
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, []string{"warning skipped"}, cancelled.log)

	// Without a terminal, the error ends the deploy.
	unattended := &fakeUI{}
	_, _, err = retryOrSkip(t.Context(), unattended, struct{}{}, "skipped",
		func(struct{}) (struct{}, error) { return struct{}{}, errors.New("no") },
		func() (struct{}, error) { return struct{}{}, nil })
	require.EqualError(t, err, "no")
	assert.Empty(t, unattended.log)
}

func TestAWaitEndsAtCtrlC(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.Equal(t, waitInterrupted, wait(ctx, time.Hour))
	assert.Equal(t, waitElapsed, wait(t.Context(), time.Millisecond))
}
