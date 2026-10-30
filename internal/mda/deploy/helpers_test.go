package deploy

// Helpers for tests that talk to a stub LangSmith through real clients.

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
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
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
	} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}

// cliClient is a LangSmith CLI client authenticated with an API key. Its own
// API URL is unroutable: hostapi addresses every request absolutely.
func cliClient(t *testing.T) *client.Client {
	t.Helper()
	isolateEnv(t)
	return client.NewWithOptions(client.Options{APIKey: "key", APIURL: "http://unused.invalid"})
}

// hostClient is a client whose host backend and platform API are both baseURL.
func hostClient(t *testing.T, baseURL string) *hostapi.Client {
	t.Helper()
	c, err := hostapi.New(cliClient(t), baseURL, baseURL)
	require.NoError(t, err)
	return c
}

// unreachableClient is a client for a LangSmith that nothing listens on, for
// tests that must not make a request.
func unreachableClient(t *testing.T) *hostapi.Client {
	t.Helper()
	return hostClient(t, "http://127.0.0.1:1")
}

// contextHubClient is a Context Hub client for baseURL.
func contextHubClient(t *testing.T, baseURL string) *hostapi.ContextHubClient {
	t.Helper()
	c, err := hostapi.NewContextHubClient(cliClient(t), baseURL)
	require.NoError(t, err)
	return c
}

// unattended makes the default terminal one nobody answers on, and returns
// what is drawn on it.
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

// okReply is a 200 reply with body.
func okReply(body string) reply {
	return reply{status: http.StatusOK, body: body}
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

// stub is a server answering requests with replies in order.
type stub struct {
	URL string

	t        *testing.T
	mu       sync.Mutex
	replies  []reply
	requests []recorded
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

// writeFile writes contents to rel under root, creating directories.
func writeFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

// testProject is a project rooted at root whose agent is agentFile.
func testProject(root, agentFile string, language project.Language) *project.Project {
	return &project.Project{
		Root:      root,
		Language:  language,
		AgentFile: filepath.FromSlash(agentFile),
		Files:     []string{filepath.FromSlash(agentFile)},
	}
}

func ptr[T any](value T) *T {
	return &value
}
