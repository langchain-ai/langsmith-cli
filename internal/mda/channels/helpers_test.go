package channels

// Helpers for tests that talk to a stub LangSmith or Linear through real
// clients.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// isolateEnv keeps the developer's LangSmith config and environment out of
// the SDK, which reads both.
func isolateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	for _, key := range []string{
		"LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_TENANT_ID",
		"LANGSMITH_WORKSPACE_ID", "LANGSMITH_PROFILE", "LANGCHAIN_CUSTOM_HEADERS",
		AgentAuthURLEnv, agentAuthAPIKeyEnv, agentAuthTenantEnv, OAuthCallbackEnv,
		SlackBotTokenEnv, SlackSigningSecretEnv,
		GitHubWebhookSecretEnv, GitHubAppIDEnv, GitHubPrivateKeyFileEnv, GitHubInstallationIDEnv,
		GitHubWebhookElsewhereEnv, GitHubAppOwnerEnv,
		LinearClientIDEnv, LinearClientSecretEnv, LinearWebhookSecretEnv, LinearWorkspaceEnv,
	} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}

// oauthClient is a LangSmith client for a stub platform API at baseURL,
// signed in with an OAuth profile, the way most developers use the CLI.
func oauthClient(t *testing.T, baseURL string) *hostapi.Client {
	t.Helper()
	isolateEnv(t)
	data, err := json.Marshal(map[string]any{"profiles": map[string]any{
		"oauth-profile": map[string]any{"oauth": map[string]any{"access_token": "oauth-token"}},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(os.Getenv("LANGSMITH_CONFIG_FILE"), data, 0o600))
	ls := client.NewWithOptions(client.Options{
		ProfileName: "oauth-profile",
		APIURL:      "http://unused.invalid",
		WorkspaceID: "tenant",
	})
	c, err := hostapi.New(ls, "http://unused.invalid", baseURL)
	require.NoError(t, err)
	return c
}

// unattended makes the default terminal one nobody answers on.
func unattended(t *testing.T) *strings.Builder {
	t.Helper()
	output := &strings.Builder{}
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(""), output, ui.WithInteractive(false), ui.WithColors(false))))
	return output
}

// reply is one stub response.
type reply struct {
	status int
	body   string
}

// stub is a server answering requests with replies in order.
type stub struct {
	URL string

	t        *testing.T
	mu       sync.Mutex
	replies  []reply
	requests []recorded
}

// recorded is a request a stub received.
type recorded struct {
	method string
	// uri is the path and query.
	uri    string
	header http.Header
	body   string
}

// line is the request line without the protocol, e.g. `GET /path?query`.
func (r recorded) line() string {
	return r.method + " " + r.uri
}

// serveSequence serves replies in order, one per request, and records each
// request. A request past the last reply fails the test.
func serveSequence(t *testing.T, replies ...reply) *stub {
	t.Helper()
	s := &stub{t: t, replies: replies}
	server := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(server.Close)
	s.URL = server.URL
	return s
}

func (s *stub) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Errorf("reading request body: %v", err)
	}
	s.mu.Lock()
	s.requests = append(s.requests, recorded{method: r.Method, uri: r.URL.RequestURI(), header: r.Header.Clone(), body: string(body)})
	index := len(s.requests) - 1
	s.mu.Unlock()
	if index >= len(s.replies) {
		s.t.Errorf("unexpected request %d: %s %s", index+1, r.Method, r.URL)
		w.WriteHeader(http.StatusTeapot)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.replies[index].status)
	if _, err := io.WriteString(w, s.replies[index].body); err != nil {
		s.t.Errorf("writing response: %v", err)
	}
}

// received are the requests received so far.
func (s *stub) received() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.requests...)
}

// lines are the request lines received so far.
func (s *stub) lines() []string {
	var lines []string
	for _, request := range s.received() {
		lines = append(lines, request.line())
	}
	return lines
}

// jsonValue decodes a JSON literal for a test.
func jsonValue(t *testing.T, text string) any {
	t.Helper()
	var value any
	require.NoError(t, json.Unmarshal([]byte(text), &value))
	return value
}

// jsonObject decodes a JSON object literal for a test.
func jsonObject(t *testing.T, text string) map[string]any {
	t.Helper()
	object, ok := jsonValue(t, text).(map[string]any)
	require.True(t, ok, "%s is not an object", text)
	return object
}
