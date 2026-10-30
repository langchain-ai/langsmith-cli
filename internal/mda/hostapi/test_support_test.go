package hostapi

// Helpers for tests that talk to a stub LangSmith through a real
// internal/client.Client.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/client"
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
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

// apiKeyCLIClient is a CLI client authenticated with apiKey and scoped to
// workspace ("" for none). Its own API URL is unroutable: hostapi addresses
// every request absolutely.
func apiKeyCLIClient(t *testing.T, apiKey, workspace string) *client.Client {
	t.Helper()
	isolateEnv(t)
	return client.NewWithOptions(client.Options{
		APIKey:      apiKey,
		APIURL:      "http://unused.invalid",
		WorkspaceID: workspace,
	})
}

// oauthCLIClient is a CLI client signed in with an OAuth profile holding
// accessToken, scoped to workspace.
func oauthCLIClient(t *testing.T, accessToken, workspace string) *client.Client {
	t.Helper()
	isolateEnv(t)
	config := map[string]any{"profiles": map[string]any{
		"oauth-profile": map[string]any{"oauth": map[string]any{"access_token": accessToken}},
	}}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("LANGSMITH_CONFIG_FILE"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return client.NewWithOptions(client.Options{
		ProfileName: "oauth-profile",
		APIURL:      "http://unused.invalid",
		WorkspaceID: workspace,
	})
}

// newClient is a Client over ls for hostBackendURL and platformAPIURL.
func newClient(t *testing.T, ls *client.Client, hostBackendURL, platformAPIURL string) *Client {
	t.Helper()
	c, err := New(ls, hostBackendURL, platformAPIURL)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// testClient is a client for a stub platform API at platformAPIURL,
// authenticated with the API key "key" in the workspace "tenant".
//
// The host backend is unroutable, so a test that reaches it by mistake fails
// instead of hitting the stub server.
func testClient(t *testing.T, platformAPIURL string) *Client {
	t.Helper()
	return newClient(t, apiKeyCLIClient(t, "key", "tenant"), "http://unused.invalid", platformAPIURL)
}

// recorded is a request a stub received.
type recorded struct {
	Method string
	// URI is the request URI: path and query.
	URI    string
	Header http.Header
	Body   []byte
}

// Line is the request line without the protocol, e.g. `GET /path?query`.
func (r recorded) Line() string {
	return r.Method + " " + r.URI
}

// JSON decodes the body.
func (r recorded) JSON(t *testing.T) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(r.Body, &value); err != nil {
		t.Fatalf("request body %q is not JSON: %v", r.Body, err)
	}
	return value
}

// Query is the decoded query.
func (r recorded) Query(t *testing.T) map[string][]string {
	t.Helper()
	_, query, _ := strings.Cut(r.URI, "?")
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// reply is one stub response.
type reply struct {
	status int
	body   string
	header map[string]string
}

// stub is a server answering requests with replies in order.
type stub struct {
	URL string

	mu       sync.Mutex
	replies  []reply
	received []recorded
	t        *testing.T
}

// serveSequence serves replies in order, one per request, and records each
// request. A request past the last reply fails the test.
func serveSequence(t *testing.T, replies ...reply) *stub {
	t.Helper()
	s := &stub{replies: replies, t: t}
	server := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(server.Close)
	s.URL = server.URL
	return s
}

// serveOnce serves one JSON response.
func serveOnce(t *testing.T, status int, body string) *stub {
	t.Helper()
	return serveSequence(t, reply{status: status, body: body})
}

func (s *stub) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Errorf("reading request body: %v", err)
	}
	s.mu.Lock()
	s.received = append(s.received, recorded{Method: r.Method, URI: r.URL.RequestURI(), Header: r.Header.Clone(), Body: body})
	index := len(s.received) - 1
	s.mu.Unlock()
	if index >= len(s.replies) {
		s.t.Errorf("unexpected request %d: %s %s", index+1, r.Method, r.URL)
		w.WriteHeader(http.StatusTeapot)
		return
	}
	answer := s.replies[index]
	w.Header().Set("Content-Type", "application/json")
	for name, value := range answer.header {
		w.Header().Set(name, value)
	}
	w.WriteHeader(answer.status)
	_, _ = io.WriteString(w, answer.body)
}

// Requests are the requests received so far.
func (s *stub) Requests() []recorded {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recorded(nil), s.received...)
}

// Only is the one request the stub received.
func (s *stub) Only(t *testing.T) recorded {
	t.Helper()
	requests := s.Requests()
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1: %v", len(requests), requests)
	}
	return requests[0]
}

// mustJSON is value as JSON text.
func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// jsonEqual reports whether got and want are the same JSON value.
func jsonEqual(t *testing.T, got any, want string) bool {
	t.Helper()
	var expected any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	return mustJSON(t, got) == mustJSON(t, expected)
}

// captureUI sends the default terminal's output to the returned buffer for
// the rest of the test.
func captureUI(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(""), &out)))
	return &out
}
