package connection

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// signInMessages are what `mda connections create` says about its sign-in.
var signInMessages = OAuthWaitMessages{
	Failed:     "Agent account sign-in failed; try again.",
	Expired:    "Agent account sign-in expired; run the command again.",
	Unexpected: "Agent account sign-in returned an unexpected status",
}

func TestPollOAuthSessionWaitsUntilCompleted(t *testing.T) {
	statuses := []string{"pending", "completed"}

	err := PollOAuthSession("pending", func() (string, error) {
		next := statuses[0]
		statuses = statuses[1:]
		return next, nil
	}, signInMessages)

	require.NoError(t, err)
	assert.Empty(t, statuses)
}

func TestPollOAuthSessionReportsFailedSession(t *testing.T) {
	err := PollOAuthSession("pending", func() (string, error) { return "failed", nil }, signInMessages)

	require.EqualError(t, err, "Agent account sign-in failed; try again.")
}

func TestPollOAuthSessionReportsExpiredSession(t *testing.T) {
	err := PollOAuthSession("expired", func() (string, error) {
		t.Fatal("an expired session must not be polled")
		return "", nil
	}, signInMessages)

	require.EqualError(t, err, "Agent account sign-in expired; run the command again.")
}

func TestPollOAuthSessionNamesAnUnexpectedStatus(t *testing.T) {
	err := PollOAuthSession("weird", nil, signInMessages)

	require.EqualError(t, err, "Agent account sign-in returned an unexpected status: weird.")
}

// stub is a stub Agent Auth that records each request and answers from its
// queue.
type stub struct {
	mu        sync.Mutex
	requests  []*http.Request
	bodies    []string
	responses []stubResponse
}

type stubResponse struct {
	status int
	body   string
}

func serve(t *testing.T, responses ...stubResponse) (*stub, *httptest.Server) {
	t.Helper()
	s := &stub{responses: responses}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests = append(s.requests, r)
		s.bodies = append(s.bodies, string(body))
		if len(s.responses) == 0 {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		next := s.responses[0]
		s.responses = s.responses[1:]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(next.status)
		if _, err := io.WriteString(w, next.body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return s, server
}

// oauthClient is an Agent Auth client signed in with an OAuth profile, the
// way most developers use the CLI.
func oauthClient(t *testing.T, baseURL string) *hostapi.Client {
	t.Helper()
	config := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", config)
	for _, key := range []string{
		"LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_TENANT_ID",
		"LANGSMITH_WORKSPACE_ID", "LANGSMITH_PROFILE", "LANGCHAIN_CUSTOM_HEADERS",
	} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	data, err := json.Marshal(map[string]any{"profiles": map[string]any{
		"oauth-profile": map[string]any{"oauth": map[string]any{"access_token": "oauth-token"}},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(config, data, 0o600))
	ls := client.NewWithOptions(client.Options{
		ProfileName: "oauth-profile",
		APIURL:      "http://unused.invalid",
		WorkspaceID: "tenant",
	})
	c, err := hostapi.New(ls, "http://unused.invalid", baseURL)
	require.NoError(t, err)
	return c
}

func TestWaitForOAuthSessionLongPollsAgentAuth(t *testing.T) {
	stub, server := serve(t, stubResponse{http.StatusOK, `{"id":"99999999-8888-7777-6666-555555555555","status":"completed","verification_url":"https://aa.example/start"}`})

	status, err := WaitForOAuthStatus(t.Context(), oauthClient(t, server.URL), "99999999-8888-7777-6666-555555555555")

	require.NoError(t, err)
	assert.Equal(t, "completed", status)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, "/v1/agent-auth/oauth-authorization-sessions/99999999-8888-7777-6666-555555555555", stub.requests[0].URL.Path)
	assert.Equal(t, "25", stub.requests[0].URL.Query().Get("wait_seconds"))
	assert.Equal(t, "Bearer oauth-token", stub.requests[0].Header.Get("Authorization"))
	assert.Empty(t, stub.requests[0].Header.Get("X-Api-Key"))
}

func TestWaitForOAuthSessionStopsWhenCtrlCIsRequested(t *testing.T) {
	stub, server := serve(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := WaitForOAuthSession(ctx, oauthClient(t, server.URL), "99999999-8888-7777-6666-555555555555")

	assert.True(t, errs.IsCancellation(err), "%v", err)
	assert.Empty(t, stub.requests)
}

func TestWaitForOAuthSessionSaysWhichCheckFailed(t *testing.T) {
	_, server := serve(t, stubResponse{http.StatusInternalServerError, `{"detail":"boom"}`})

	_, err := WaitForOAuthSession(t.Context(), oauthClient(t, server.URL), "99999999-8888-7777-6666-555555555555")

	require.ErrorContains(t, err, "failed to check the OAuth authorization session: ")
	require.ErrorContains(t, err, "boom")
}

func TestCreateDiscoveredMcpConnectionAsksAgentAuthToDiscover(t *testing.T) {
	stub, server := serve(t, stubResponse{http.StatusOK, `{"connection_id":"c-1","slug":"linear","kind":"oauth2"}`})

	created, err := CreateDiscoveredMcpConnection(t.Context(), oauthClient(t, server.URL), "linear", "https://mcp.linear.app/mcp")

	require.NoError(t, err)
	assert.Equal(t, "c-1", created.ConnectionID)
	assert.Equal(t, "user", created.OwnerPolicy)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, "POST /v1/agent-auth/connections", stub.requests[0].Method+" "+stub.requests[0].URL.Path)
	assert.JSONEq(t, `{
		"slug": "linear",
		"display_name": "linear",
		"credential": {
			"kind": "oauth2",
			"provider": {"display_name": "linear", "mcp_server_url": "https://mcp.linear.app/mcp"}
		}
	}`, stub.bodies[0])
}

// failedDiscovery is how Agent Auth answers a discovery it could not finish.
func failedDiscovery(t *testing.T) *hostapi.Client {
	t.Helper()
	_, server := serve(t, stubResponse{http.StatusBadRequest, `{"detail":"MCP server does not support dynamic client registration"}`})
	return oauthClient(t, server.URL)
}

func TestFailedDiscoveryOfAServerThatAnswersNamesTheFallback(t *testing.T) {
	agentAuth := failedDiscovery(t)
	var probed *http.Request
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probed = r
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(mcp.Close)

	_, err := CreateDiscoveredMcpConnection(t.Context(), agentAuth, "foobar", mcp.URL+"/mcp")

	require.EqualError(t, err, "'"+mcp.URL+"/mcp' does not support CIMD or OAuth Dynamic Client Registration, so "+
		"langsmith cannot register a client automatically. Create the connection with "+
		"`mda connections create foobar --oauth <service>` when the provider is in "+
		"`mda connections catalog`, or pass --authorize-url, --token-url, and --client-id.")
	assert.Equal(t, errs.Remote, errs.CategoryOf(err))
	// The probe carries no LangSmith credential.
	require.NotNil(t, probed)
	assert.Equal(t, "/mcp", probed.URL.Path)
	assert.Empty(t, probed.Header.Get("Authorization"))
	assert.Empty(t, probed.Header.Get("X-Api-Key"))
}

func TestFailedDiscoveryOfAMissingPathPointsAtThePath(t *testing.T) {
	agentAuth := failedDiscovery(t)
	mcp := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(mcp.Close)

	_, err := CreateDiscoveredMcpConnection(t.Context(), agentAuth, "foobar", mcp.URL+"/mpc")

	assert.Equal(t, errs.UserInput, errs.CategoryOf(err))
	require.ErrorContains(t, err, "answered 404")
}

func TestFailedDiscoveryOfAnUnreachableServerIsNotMissingDcr(t *testing.T) {
	agentAuth := failedDiscovery(t)
	mcp := httptest.NewServer(http.NotFoundHandler())
	unreachable := mcp.URL + "/mcp"
	mcp.Close()

	_, err := CreateDiscoveredMcpConnection(t.Context(), agentAuth, "foobar", unreachable)

	assert.Equal(t, errs.Network, errs.CategoryOf(err))
	require.ErrorContains(t, err, "did not answer")
}

func TestAReachableServerWithoutDcrNamesTheURLAndTheFallback(t *testing.T) {
	message := mcpDiscoveryError(mcpProbeResponded, "foobar", "https://mcp.acme.com/mcp").Error()

	assert.Contains(t, message, "https://mcp.acme.com/mcp")
	assert.Contains(t, message, "does not support CIMD or OAuth Dynamic Client Registration")
	assert.Contains(t, message, "--oauth")
	assert.Contains(t, message, "--client-id")
}

// A typo'd host is the common case, and it must not read as a provider that
// refused to register a client.
func TestAnUnreachableServerIsNotReportedAsMissingDcr(t *testing.T) {
	err := mcpDiscoveryError(mcpProbeUnreachable, "my-2linear", "https://mscp.linear.app/mcp")

	assert.Equal(t, errs.Network, errs.CategoryOf(err))
	assert.Contains(t, err.Error(), "did not answer")
	assert.Contains(t, err.Error(), "typo")
	assert.NotContains(t, err.Error(), "does not support CIMD or OAuth Dynamic Client Registration")
}

func TestAServerAnswering404PointsAtThePath(t *testing.T) {
	err := mcpDiscoveryError(mcpProbeNotFound, "foobar", "https://mcp.acme.com/mpc")

	assert.Equal(t, errs.UserInput, errs.CategoryOf(err))
	assert.Contains(t, err.Error(), "answered 404")
	assert.Contains(t, err.Error(), "path")
}

func TestProbeStatuses(t *testing.T) {
	assert.Equal(t, mcpProbeUnreachable, classifyMcpProbe(0))
	assert.Equal(t, mcpProbeNotFound, classifyMcpProbe(http.StatusNotFound))
	assert.Equal(t, mcpProbeResponded, classifyMcpProbe(http.StatusUnauthorized))
}

func TestMcpDcrConflictStillNamesTheExistingSlug(t *testing.T) {
	err := explainMcpDcr(t.Context(), &hostapi.APIError{Status: http.StatusConflict, Detail: "already exists"},
		"foobar", "https://mcp.acme.com/mcp")

	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), "`mda connections delete foobar`")
	assert.NotContains(t, err.Error(), "Dynamic Client Registration")
}

func TestAnOrgScopedKeyIsAskedForAWorkspace(t *testing.T) {
	err := explain(&hostapi.APIError{Status: http.StatusForbidden, Detail: "ORG_SCOPED_KEY_REQUIRES_WORKSPACE"}, "foobar")

	assert.Equal(t, errs.Permission, errs.CategoryOf(err))
	require.EqualError(t, err, "This API key is not scoped to a workspace, so LangSmith cannot tell "+
		"which one to use. Pass --workspace-id, or set LANGSMITH_WORKSPACE_ID.")

	other := errors.New("boom")
	assert.Same(t, other, explain(other, "foobar"))
}

func TestAgentAuthCimdFailureIsTreatedAsUnsupportedRegistration(t *testing.T) {
	assert.True(t, dcrUnsupportedDetail("authorization server supports neither CIMD nor dynamic client registration"))
	assert.True(t, dcrUnsupportedDetail("DCR is not supported"))
	assert.False(t, dcrUnsupportedDetail("upstream timed out"))
	assert.True(t, mcpDcrUnsupported(&hostapi.APIError{Status: http.StatusUnprocessableEntity, Detail: "x"}))
	assert.False(t, mcpDcrUnsupported(&hostapi.APIError{Status: http.StatusBadGateway, Detail: "upstream timed out"}))
}
