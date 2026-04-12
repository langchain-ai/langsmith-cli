# `langsmith api` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `langsmith api` command with OpenAPI spec browsing (`ls`, `info`) and authenticated HTTP requests (`METHOD path`).

**Architecture:** New sub-package `internal/cmd/api/` containing three subcommands. OpenAPI spec is fetched from `{api_url}/openapi.json` and cached locally with 24h TTL. Request-making uses a new `RawDo` method on the client that returns raw bytes. The api package reads persistent flags from the root cobra command for api-key, api-url, and format — no coupling to the parent cmd package.

**Tech Stack:** Go stdlib only (net/http, encoding/json, crypto/sha256, os). Cobra for CLI. No new dependencies.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/client/client.go` | Modify | Add `RawDo` method returning raw status, headers, body |
| `internal/client/client_test.go` | Modify | Tests for `RawDo` |
| `internal/cmd/api/resolve.go` | Create | Shared helpers: flag resolution, path resolution, error output |
| `internal/cmd/api/resolve_test.go` | Create | Tests for path resolution and flag helpers |
| `internal/cmd/api/spec.go` | Create | OpenAPI spec fetching, caching, parsing |
| `internal/cmd/api/spec_test.go` | Create | Tests for spec fetch/cache/parse |
| `internal/cmd/api/ls.go` | Create | `langsmith api ls` subcommand |
| `internal/cmd/api/ls_test.go` | Create | Tests for ls |
| `internal/cmd/api/info.go` | Create | `langsmith api info` subcommand |
| `internal/cmd/api/info_test.go` | Create | Tests for info |
| `internal/cmd/api/request.go` | Create | `langsmith api METHOD path` request execution |
| `internal/cmd/api/request_test.go` | Create | Tests for request |
| `internal/cmd/api/api.go` | Create | Parent command wiring `ls`, `info`, and request dispatch |
| `internal/cmd/api/api_test.go` | Create | Tests for parent command dispatch |
| `internal/cmd/root.go` | Modify | Register `api.NewCmd()` |
| `internal/cmd/root_test.go` | Modify | Add `"api"` to expected subcommands |

---

### Task 1: Add `RawDo` to client

**Files:**
- Modify: `internal/client/client.go:82-145`
- Modify: `internal/client/client_test.go`

Add a low-level HTTP method that returns raw status, headers, and body bytes without unmarshaling or treating 4xx as errors.

- [ ] **Step 1: Write failing tests for RawDo**

Append to `internal/client/client_test.go`:

```go
func TestRawDo_ReturnsStatusAndBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sessions" {
			t.Errorf("expected /api/v1/sessions, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "my-key" {
			t.Errorf("expected x-api-key=my-key, got %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("X-Request-Id", "req-abc")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer ts.Close()

	c := New("my-key", ts.URL)
	status, hdr, body, err := c.RawDo(context.Background(), "PATCH", "/api/v1/sessions", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if hdr.Get("X-Request-Id") != "req-abc" {
		t.Errorf("expected X-Request-Id=req-abc, got %q", hdr.Get("X-Request-Id"))
	}
	if string(body) != `{"id":"123"}` {
		t.Errorf("expected body {\"id\":\"123\"}, got %q", string(body))
	}
}

func TestRawDo_WithBodyReader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		w.WriteHeader(201)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	status, _, body, err := c.RawDo(context.Background(), "POST", "/create", strings.NewReader(`{"name":"test"}`), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 201 {
		t.Errorf("expected 201, got %d", status)
	}
	if string(body) != `{"name":"test"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestRawDo_ExtraHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "hello" {
			t.Errorf("expected X-Custom=hello, got %q", r.Header.Get("X-Custom"))
		}
		if r.Header.Get("x-api-key") != "key" {
			t.Errorf("expected x-api-key=key, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	extra := http.Header{"X-Custom": []string{"hello"}}
	_, _, _, err := c.RawDo(context.Background(), "GET", "/test", nil, extra)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawDo_Returns4xxWithoutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":"invalid"}`))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	status, _, body, err := c.RawDo(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 422 {
		t.Errorf("expected 422, got %d", status)
	}
	if string(body) != `{"detail":"invalid"}` {
		t.Errorf("unexpected body: %s", body)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/client/ -run "TestRawDo" -v`
Expected: compilation error — `c.RawDo` undefined.

- [ ] **Step 3: Implement RawDo**

Add to `internal/client/client.go` after the `RawDelete` method (after line 97):

```go
// RawDo performs an arbitrary HTTP request and returns the raw response.
// Unlike RawGet/RawPost/RawDelete, it does not unmarshal the response and
// does not treat 4xx/5xx as errors — callers decide how to handle status codes.
// body may be nil. extraHeaders are merged on top of the default auth headers.
func (c *Client) RawDo(ctx context.Context, method, path string, body io.Reader, extraHeaders http.Header) (statusCode int, respHeaders http.Header, respBody []byte, err error) {
	url := c.apiURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if wsID := os.Getenv("LANGSMITH_WORKSPACE_ID"); wsID != "" {
		req.Header.Set("x-tenant-id", wsID)
	}
	for k, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Set(k, v)
		}
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("reading response: %w", err)
	}

	return resp.StatusCode, resp.Header, respBody, nil
}
```

Also add `APIKey` and `APIURL` accessor methods so the api sub-package can use the client for requests without re-resolving config:

```go
// APIKey returns the client's API key.
func (c *Client) APIKey() string { return c.apiKey }

// APIURL returns the client's normalized API URL.
func (c *Client) APIURL() string { return c.apiURL }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/client/ -run "TestRawDo" -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/client.go internal/client/client_test.go
git commit -m "feat(client): add RawDo method for raw HTTP requests"
```

---

### Task 2: Shared helpers — resolve.go

**Files:**
- Create: `internal/cmd/api/resolve.go`
- Create: `internal/cmd/api/resolve_test.go`

Shared helpers used by all three subcommands: resolving API key/URL/format from cobra persistent flags + env vars, resolving endpoint paths, and printing errors.

- [ ] **Step 1: Write failing tests for resolveEndpoint**

Create `internal/cmd/api/resolve_test.go`:

```go
package api

import (
	"testing"
)

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{"absolute path", "https://api.smith.langchain.com", "/api/v1/sessions", "https://api.smith.langchain.com/api/v1/sessions"},
		{"shorthand", "https://api.smith.langchain.com", "sessions", "https://api.smith.langchain.com/api/v1/sessions"},
		{"shorthand with subpath", "https://api.smith.langchain.com", "runs/query", "https://api.smith.langchain.com/api/v1/runs/query"},
		{"shorthand with query params", "https://api.smith.langchain.com", "sessions?limit=5", "https://api.smith.langchain.com/api/v1/sessions?limit=5"},
		{"full URL https", "https://api.smith.langchain.com", "https://other.host/foo", "https://other.host/foo"},
		{"full URL http", "https://api.smith.langchain.com", "http://other.host/foo", "http://other.host/foo"},
		{"self-hosted base", "https://myhost.com", "sessions", "https://myhost.com/api/v1/sessions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(tt.baseURL, tt.path)
			if got != tt.want {
				t.Errorf("resolveEndpoint(%q, %q) = %q, want %q", tt.baseURL, tt.path, got, tt.want)
			}
		})
	}
}

func TestIsHTTPMethod(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
		if !isHTTPMethod(m) {
			t.Errorf("expected %q to be recognized as HTTP method", m)
		}
	}
	for _, m := range []string{"get", "ls", "info", "FOO", ""} {
		if isHTTPMethod(m) {
			t.Errorf("expected %q to NOT be recognized as HTTP method", m)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestResolve|TestIsHTTP" -v`
Expected: compilation error — package and functions don't exist.

- [ ] **Step 3: Implement resolve.go**

Create `internal/cmd/api/resolve.go`:

```go
package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// resolveAPIKey resolves the API key from cobra persistent flags → env → empty.
func resolveAPIKey(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-key"); v != "" {
		return v
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

// resolveAPIURL resolves the API URL from cobra persistent flags → env → default.
func resolveAPIURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-url"); v != "" {
		return client.NormalizeURL(v)
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return client.NormalizeURL(v)
	}
	return "https://api.smith.langchain.com"
}

// resolveFormat resolves the output format from cobra persistent flags.
func resolveFormat(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("format")
	if v == "" {
		return "json"
	}
	return v
}

// mustClient creates a client or exits with an error.
func mustClient(cmd *cobra.Command) *client.Client {
	apiKey := resolveAPIKey(cmd)
	if apiKey == "" {
		exitError("LANGSMITH_API_KEY not set")
	}
	return client.New(apiKey, resolveAPIURL(cmd))
}

// resolveEndpoint resolves an endpoint argument to a full URL.
//   - Full URL (http:// or https://) → returned as-is.
//   - Absolute path (starts with /) → baseURL + path.
//   - Shorthand (e.g. "sessions") → baseURL + /api/v1/ + path.
func resolveEndpoint(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return baseURL + path
	}
	return baseURL + "/api/v1/" + path
}

// isHTTPMethod returns true if s is an uppercase HTTP method name.
func isHTTPMethod(s string) bool {
	switch s {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

// exitError prints a JSON error to stderr and exits.
func exitError(msg string) {
	fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", msg)
	os.Exit(1)
}

// exitErrorf prints a formatted JSON error to stderr and exits.
func exitErrorf(format string, args ...any) {
	exitError(fmt.Sprintf(format, args...))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestResolve|TestIsHTTP" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/api/resolve.go internal/cmd/api/resolve_test.go
git commit -m "feat(api): add shared helpers for path resolution and flag reading"
```

---

### Task 3: OpenAPI spec fetching and caching — spec.go

**Files:**
- Create: `internal/cmd/api/spec.go`
- Create: `internal/cmd/api/spec_test.go`

Fetches the OpenAPI spec from `{api_url}/openapi.json`, caches it to disk with a 24h TTL.

- [ ] **Step 1: Write failing tests for spec caching**

Create `internal/cmd/api/spec_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSpecCachePath(t *testing.T) {
	p1 := specCachePath("/tmp/test-cache", "https://api.smith.langchain.com")
	p2 := specCachePath("/tmp/test-cache", "https://myhost.com")
	if p1 == p2 {
		t.Error("expected different cache paths for different API URLs")
	}
	if filepath.Dir(p1) != "/tmp/test-cache" {
		t.Errorf("expected cache dir /tmp/test-cache, got %s", filepath.Dir(p1))
	}
}

func TestFetchSpec_FromServer(t *testing.T) {
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary": "List sessions",
					"tags":    []any{"tracer-sessions"},
				},
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi.json" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(spec)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	result, err := loadSpec(ts.URL, cacheDir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OpenAPI != "3.1.0" {
		t.Errorf("expected openapi 3.1.0, got %q", result.OpenAPI)
	}
	paths := result.Paths
	if len(paths) != 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}
}

func TestFetchSpec_UsesCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	// First call fetches from server
	_, err := loadSpec(ts.URL, cacheDir, false)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 server call, got %d", callCount)
	}

	// Second call should use cache
	_, err = loadSpec(ts.URL, cacheDir, false)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected cache hit (still 1 call), got %d calls", callCount)
	}
}

func TestFetchSpec_RefreshBypassesCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	_, _ = loadSpec(ts.URL, cacheDir, false)
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Force refresh
	_, _ = loadSpec(ts.URL, cacheDir, true)
	if callCount != 2 {
		t.Errorf("expected 2 calls after refresh, got %d", callCount)
	}
}

func TestFetchSpec_ExpiredCache(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"openapi": "3.1.0",
			"paths":   map[string]any{},
		})
	}))
	defer ts.Close()

	cacheDir := t.TempDir()

	// Fetch and then manually backdate the cache file
	_, _ = loadSpec(ts.URL, cacheDir, false)
	cachePath := specCachePath(cacheDir, ts.URL)
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(cachePath, old, old)

	// Should re-fetch because cache is expired
	_, _ = loadSpec(ts.URL, cacheDir, false)
	if callCount != 2 {
		t.Errorf("expected 2 calls after TTL expiry, got %d", callCount)
	}
}

func TestParseEndpoints(t *testing.T) {
	raw := json.RawMessage(`{
		"/api/v1/sessions": {
			"get": {"summary": "List sessions", "tags": ["tracer-sessions"]},
			"post": {"summary": "Create session", "tags": ["tracer-sessions"]}
		},
		"/api/v1/datasets": {
			"get": {"summary": "List datasets", "tags": ["datasets"]}
		}
	}`)
	var paths map[string]map[string]json.RawMessage
	json.Unmarshal(raw, &paths)

	spec := &OpenAPISpec{Paths: paths}
	endpoints := spec.Endpoints()
	if len(endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(endpoints))
	}

	// Check they're sorted by path then method
	if endpoints[0].Path != "/api/v1/datasets" {
		t.Errorf("expected first endpoint /api/v1/datasets, got %s", endpoints[0].Path)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestSpec|TestFetch|TestParse" -v`
Expected: compilation error — types and functions don't exist.

- [ ] **Step 3: Implement spec.go**

Create `internal/cmd/api/spec.go`:

```go
package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const specCacheTTL = 24 * time.Hour

// OpenAPISpec holds the parsed parts of the OpenAPI spec we care about.
type OpenAPISpec struct {
	OpenAPI    string                                  `json:"openapi"`
	Info       json.RawMessage                         `json:"info"`
	Paths      map[string]map[string]json.RawMessage   `json:"paths"`
	Components json.RawMessage                         `json:"components"`
}

// Endpoint is a single method+path from the spec.
type Endpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
	Tag     string `json:"tag"`
}

// EndpointDetail has full info for a single endpoint.
type EndpointDetail struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary"`
	Tag         string      `json:"tag"`
	Description string      `json:"description,omitempty"`
	Parameters  []Parameter `json:"parameters"`
	RequestBody any         `json:"request_body"`
	Response    any         `json:"response_schema"`
}

// Parameter describes a single API parameter.
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// Endpoints returns a sorted list of all endpoints in the spec.
func (s *OpenAPISpec) Endpoints() []Endpoint {
	var endpoints []Endpoint
	for path, methods := range s.Paths {
		for method, raw := range methods {
			m := strings.ToUpper(method)
			if !isHTTPMethod(m) {
				continue // skip "parameters", "summary", etc.
			}
			var detail struct {
				Summary string   `json:"summary"`
				Tags    []string `json:"tags"`
			}
			json.Unmarshal(raw, &detail)
			tag := ""
			if len(detail.Tags) > 0 {
				tag = detail.Tags[0]
			}
			endpoints = append(endpoints, Endpoint{
				Method:  m,
				Path:    path,
				Summary: detail.Summary,
				Tag:     tag,
			})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
	return endpoints
}

// LookupEndpoint finds an endpoint by method and path, returning full detail.
// The path argument can be shorthand ("sessions") or absolute ("/api/v1/sessions").
func (s *OpenAPISpec) LookupEndpoint(method, path string) (*EndpointDetail, error) {
	// Normalize: if shorthand, prefix /api/v1/
	normalized := path
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/api/v1/" + normalized
	}
	method = strings.ToUpper(method)

	methods, ok := s.Paths[normalized]
	if !ok {
		return nil, fmt.Errorf("endpoint not found: %s %s", method, normalized)
	}
	raw, ok := methods[strings.ToLower(method)]
	if !ok {
		return nil, fmt.Errorf("endpoint not found: %s %s", method, normalized)
	}

	var parsed struct {
		Summary     string            `json:"summary"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Parameters  []json.RawMessage `json:"parameters"`
		RequestBody json.RawMessage   `json:"requestBody"`
		Responses   json.RawMessage   `json:"responses"`
	}
	json.Unmarshal(raw, &parsed)

	tag := ""
	if len(parsed.Tags) > 0 {
		tag = parsed.Tags[0]
	}

	// Parse parameters
	var params []Parameter
	for _, pRaw := range parsed.Parameters {
		var p struct {
			Name        string `json:"name"`
			In          string `json:"in"`
			Required    bool   `json:"required"`
			Description string `json:"description"`
			Schema      struct {
				Type string `json:"type"`
			} `json:"schema"`
		}
		json.Unmarshal(pRaw, &p)
		params = append(params, Parameter{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required,
			Type:        p.Schema.Type,
			Description: p.Description,
		})
	}

	// Parse request body — resolve $ref one level deep
	var reqBody any
	if parsed.RequestBody != nil {
		reqBody = s.resolveRequestBody(parsed.RequestBody)
	}

	// Parse response — take first 2xx response
	var respSchema any
	if parsed.Responses != nil {
		respSchema = s.resolveResponse(parsed.Responses)
	}

	return &EndpointDetail{
		Method:      method,
		Path:        normalized,
		Summary:     parsed.Summary,
		Tag:         tag,
		Description: parsed.Description,
		Parameters:  params,
		RequestBody: reqBody,
		Response:    respSchema,
	}, nil
}

// resolveRequestBody extracts and resolves the request body schema.
func (s *OpenAPISpec) resolveRequestBody(raw json.RawMessage) any {
	var body struct {
		Required bool `json:"required"`
		Content  map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil
	}
	for contentType, ct := range body.Content {
		schema := s.resolveRef(ct.Schema)
		return map[string]any{
			"content_type": contentType,
			"required":     body.Required,
			"schema":       schema,
		}
	}
	return nil
}

// resolveResponse extracts the first 2xx response schema.
func (s *OpenAPISpec) resolveResponse(raw json.RawMessage) any {
	var responses map[string]struct {
		Content map[string]struct {
			Schema json.RawMessage `json:"schema"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &responses); err != nil {
		return nil
	}
	// Look for 200, 201, etc.
	for _, code := range []string{"200", "201", "202", "204"} {
		resp, ok := responses[code]
		if !ok {
			continue
		}
		for _, ct := range resp.Content {
			return s.resolveRef(ct.Schema)
		}
	}
	return nil
}

// resolveRef resolves a JSON schema, inlining one level of $ref from components.
func (s *OpenAPISpec) resolveRef(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		var result any
		json.Unmarshal(raw, &result)
		return result
	}

	// Check for $ref
	if refRaw, ok := obj["$ref"]; ok {
		var ref string
		json.Unmarshal(refRaw, &ref)
		resolved := s.resolveComponentRef(ref)
		if resolved != nil {
			return resolved
		}
	}

	// Otherwise return as generic map, resolving allOf if present
	var result any
	json.Unmarshal(raw, &result)
	return result
}

// resolveComponentRef resolves a $ref like "#/components/schemas/Foo" one level deep.
func (s *OpenAPISpec) resolveComponentRef(ref string) any {
	if s.Components == nil {
		return nil
	}
	// Parse "#/components/schemas/Foo"
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	if len(parts) < 3 || parts[0] != "components" || parts[1] != "schemas" {
		return map[string]any{"$ref": ref}
	}
	schemaName := parts[2]

	var components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	}
	if err := json.Unmarshal(s.Components, &components); err != nil {
		return map[string]any{"$ref": ref}
	}
	schemaRaw, ok := components.Schemas[schemaName]
	if !ok {
		return map[string]any{"$ref": ref}
	}

	// Parse one level — don't recurse into nested $refs
	var schema any
	json.Unmarshal(schemaRaw, &schema)
	return schema
}

// loadSpec loads the OpenAPI spec, using cache if available and not expired.
func loadSpec(apiURL, cacheDir string, forceRefresh bool) (*OpenAPISpec, error) {
	cachePath := specCachePath(cacheDir, apiURL)

	if !forceRefresh {
		if spec, err := loadCachedSpec(cachePath); err == nil {
			return spec, nil
		}
	}

	// Fetch from server
	specURL := apiURL + "/openapi.json"
	resp, err := http.Get(specURL)
	if err != nil {
		return nil, fmt.Errorf("fetching OpenAPI spec from %s: %w", specURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching OpenAPI spec: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OpenAPI spec: %w", err)
	}

	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}

	// Write cache
	os.MkdirAll(filepath.Dir(cachePath), 0755)
	os.WriteFile(cachePath, data, 0644)

	return &spec, nil
}

// loadCachedSpec reads a cached spec if it exists and is within TTL.
func loadCachedSpec(cachePath string) (*OpenAPISpec, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > specCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// specCachePath returns the cache file path for a given API URL.
func specCachePath(cacheDir, apiURL string) string {
	h := sha256.Sum256([]byte(apiURL))
	name := fmt.Sprintf("openapi-%x.json", h[:8])
	return filepath.Join(cacheDir, name)
}

// defaultCacheDir returns ~/.langsmith/cache.
func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "langsmith-cache")
	}
	return filepath.Join(home, ".langsmith", "cache")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestSpec|TestFetch|TestParse" -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/api/spec.go internal/cmd/api/spec_test.go
git commit -m "feat(api): add OpenAPI spec fetching and caching"
```

---

### Task 4: `langsmith api ls` subcommand

**Files:**
- Create: `internal/cmd/api/ls.go`
- Create: `internal/cmd/api/ls_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/api/ls_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestSpecServer(t *testing.T) *httptest.Server {
	t.Helper()
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get":  map[string]any{"summary": "List sessions", "tags": []any{"tracer-sessions"}},
				"post": map[string]any{"summary": "Create session", "tags": []any{"tracer-sessions"}},
			},
			"/api/v1/datasets": map[string]any{
				"get":  map[string]any{"summary": "List datasets", "tags": []any{"datasets"}},
				"post": map[string]any{"summary": "Create dataset", "tags": []any{"datasets"}},
			},
			"/api/v1/runs/query": map[string]any{
				"post": map[string]any{"summary": "Query runs", "tags": []any{"run"}},
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			json.NewEncoder(w).Encode(spec)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestLsCmd_JSON(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	cmd := newLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--refresh"})

	// Override internals for test
	lsAPIURL = ts.URL
	lsCacheDir = t.TempDir()
	lsFormat = "json"
	defer func() { lsAPIURL = ""; lsCacheDir = ""; lsFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	if err := json.Unmarshal(out.Bytes(), &endpoints); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out.String())
	}
	if len(endpoints) != 5 {
		t.Errorf("expected 5 endpoints, got %d", len(endpoints))
	}
}

func TestLsCmd_FilterByTag(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	cmd := newLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--tag", "datasets", "--refresh"})

	lsAPIURL = ts.URL
	lsCacheDir = t.TempDir()
	lsFormat = "json"
	defer func() { lsAPIURL = ""; lsCacheDir = ""; lsFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	json.Unmarshal(out.Bytes(), &endpoints)
	if len(endpoints) != 2 {
		t.Errorf("expected 2 dataset endpoints, got %d", len(endpoints))
	}
	for _, e := range endpoints {
		if e.Tag != "datasets" {
			t.Errorf("expected tag=datasets, got %q", e.Tag)
		}
	}
}

func TestLsCmd_Search(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	cmd := newLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--search", "query", "--refresh"})

	lsAPIURL = ts.URL
	lsCacheDir = t.TempDir()
	lsFormat = "json"
	defer func() { lsAPIURL = ""; lsCacheDir = ""; lsFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var endpoints []Endpoint
	json.Unmarshal(out.Bytes(), &endpoints)
	if len(endpoints) != 1 {
		t.Errorf("expected 1 match for 'query', got %d", len(endpoints))
	}
}

func TestLsCmd_Pretty(t *testing.T) {
	ts := newTestSpecServer(t)
	defer ts.Close()

	cmd := newLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--refresh"})

	lsAPIURL = ts.URL
	lsCacheDir = t.TempDir()
	lsFormat = "pretty"
	defer func() { lsAPIURL = ""; lsCacheDir = ""; lsFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "METHOD") {
		t.Errorf("expected table header with METHOD, got %q", output)
	}
	if !strings.Contains(output, "/api/v1/sessions") {
		t.Errorf("expected endpoint path in output, got %q", output)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestLsCmd" -v`
Expected: compilation error — `newLsCmd` doesn't exist.

- [ ] **Step 3: Implement ls.go**

Create `internal/cmd/api/ls.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// Test overrides — empty means "use real values from cobra flags".
var (
	lsAPIURL   string
	lsCacheDir string
	lsFormat   string
)

func newLsCmd() *cobra.Command {
	var (
		tag     string
		search  string
		refresh bool
	)

	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available API endpoints from the OpenAPI spec",
		Long: `List all available LangSmith API endpoints.

The endpoint list is fetched from the OpenAPI spec and cached locally for 24 hours.

Examples:
  langsmith api ls
  langsmith api ls --tag datasets
  langsmith api ls --search "create"
  langsmith api ls --tag run --search query
  langsmith api ls --refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := lsAPIURL
			if apiURL == "" {
				apiURL = resolveAPIURL(cmd)
			}
			cacheDir := lsCacheDir
			if cacheDir == "" {
				cacheDir = defaultCacheDir()
			}
			format := lsFormat
			if format == "" {
				format = resolveFormat(cmd)
			}

			spec, err := loadSpec(apiURL, cacheDir, refresh)
			if err != nil {
				return err
			}

			endpoints := spec.Endpoints()

			// Apply filters
			if tag != "" || search != "" {
				var filtered []Endpoint
				for _, e := range endpoints {
					if tag != "" && e.Tag != tag {
						continue
					}
					if search != "" {
						q := strings.ToLower(search)
						if !strings.Contains(strings.ToLower(e.Path), q) &&
							!strings.Contains(strings.ToLower(e.Summary), q) &&
							!strings.Contains(strings.ToLower(e.Tag), q) {
							continue
						}
					}
					filtered = append(filtered, e)
				}
				endpoints = filtered
			}

			w := cmd.OutOrStdout()

			if format == "pretty" {
				table := tablewriter.NewWriter(w)
				table.SetHeader([]string{"Method", "Path", "Tag", "Summary"})
				table.SetBorder(false)
				table.SetColumnSeparator("  ")
				table.SetHeaderLine(true)
				table.SetAutoWrapText(false)
				for _, e := range endpoints {
					table.Append([]string{e.Method, e.Path, e.Tag, e.Summary})
				}
				table.Render()
				fmt.Fprintf(w, "(%d endpoints)\n", len(endpoints))
			} else {
				data, _ := json.MarshalIndent(endpoints, "", "  ")
				fmt.Fprintln(w, string(data))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Filter by tag")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search path, summary, or tag (case-insensitive)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")

	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestLsCmd" -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/api/ls.go internal/cmd/api/ls_test.go
git commit -m "feat(api): add ls subcommand for listing API endpoints"
```

---

### Task 5: `langsmith api info` subcommand

**Files:**
- Create: `internal/cmd/api/info.go`
- Create: `internal/cmd/api/info_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/api/info_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newDetailedSpecServer(t *testing.T) *httptest.Server {
	t.Helper()
	spec := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api/v1/sessions": map[string]any{
				"get": map[string]any{
					"summary": "List sessions",
					"tags":    []any{"tracer-sessions"},
					"parameters": []any{
						map[string]any{
							"name": "limit", "in": "query", "required": false,
							"schema": map[string]any{"type": "integer"},
							"description": "Max results",
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"type": "array"},
								},
							},
						},
					},
				},
			},
			"/api/v1/runs/query": map[string]any{
				"post": map[string]any{
					"summary": "Query runs",
					"tags":    []any{"run"},
					"requestBody": map[string]any{
						"required": true,
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"session_id": map[string]any{"type": "string"},
									},
									"required": []any{"session_id"},
								},
							},
						},
					},
					"responses": map[string]any{
						"200": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"type": "object"},
								},
							},
						},
					},
				},
			},
		},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			json.NewEncoder(w).Encode(spec)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestInfoCmd_JSON(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	cmd := newInfoCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"GET", "/api/v1/sessions"})

	infoAPIURL = ts.URL
	infoCacheDir = t.TempDir()
	infoFormat = "json"
	defer func() { infoAPIURL = ""; infoCacheDir = ""; infoFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	if err := json.Unmarshal(out.Bytes(), &detail); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if detail.Method != "GET" {
		t.Errorf("expected method GET, got %q", detail.Method)
	}
	if detail.Path != "/api/v1/sessions" {
		t.Errorf("expected path /api/v1/sessions, got %q", detail.Path)
	}
	if len(detail.Parameters) != 1 {
		t.Errorf("expected 1 parameter, got %d", len(detail.Parameters))
	}
	if detail.Parameters[0].Name != "limit" {
		t.Errorf("expected param name 'limit', got %q", detail.Parameters[0].Name)
	}
}

func TestInfoCmd_Shorthand(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	cmd := newInfoCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"GET", "sessions"})

	infoAPIURL = ts.URL
	infoCacheDir = t.TempDir()
	infoFormat = "json"
	defer func() { infoAPIURL = ""; infoCacheDir = ""; infoFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	json.Unmarshal(out.Bytes(), &detail)
	if detail.Path != "/api/v1/sessions" {
		t.Errorf("expected resolved path /api/v1/sessions, got %q", detail.Path)
	}
}

func TestInfoCmd_WithRequestBody(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	cmd := newInfoCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"POST", "runs/query"})

	infoAPIURL = ts.URL
	infoCacheDir = t.TempDir()
	infoFormat = "json"
	defer func() { infoAPIURL = ""; infoCacheDir = ""; infoFormat = "" }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var detail EndpointDetail
	json.Unmarshal(out.Bytes(), &detail)
	if detail.RequestBody == nil {
		t.Fatal("expected request_body to be non-nil")
	}
}

func TestInfoCmd_NotFound(t *testing.T) {
	ts := newDetailedSpecServer(t)
	defer ts.Close()

	cmd := newInfoCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"GET", "nonexistent"})

	infoAPIURL = ts.URL
	infoCacheDir = t.TempDir()
	infoFormat = "json"
	defer func() { infoAPIURL = ""; infoCacheDir = ""; infoFormat = "" }()

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent endpoint")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestInfoCmd" -v`
Expected: compilation error — `newInfoCmd` doesn't exist.

- [ ] **Step 3: Implement info.go**

Create `internal/cmd/api/info.go`:

```go
package api

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Test overrides.
var (
	infoAPIURL   string
	infoCacheDir string
	infoFormat   string
)

func newInfoCmd() *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "info METHOD PATH",
		Short: "Show details for a specific API endpoint",
		Long: `Show full details for a specific API endpoint including parameters,
request body schema, and response schema.

Examples:
  langsmith api info GET /api/v1/sessions
  langsmith api info GET sessions
  langsmith api info POST runs/query`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := args[0]
			path := args[1]

			apiURL := infoAPIURL
			if apiURL == "" {
				apiURL = resolveAPIURL(cmd)
			}
			cacheDir := infoCacheDir
			if cacheDir == "" {
				cacheDir = defaultCacheDir()
			}
			format := infoFormat
			if format == "" {
				format = resolveFormat(cmd)
			}

			spec, err := loadSpec(apiURL, cacheDir, refresh)
			if err != nil {
				return err
			}

			detail, err := spec.LookupEndpoint(method, path)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			if format == "pretty" {
				fmt.Fprintf(w, "%s %s\n", detail.Method, detail.Path)
				fmt.Fprintf(w, "Tag: %s\n", detail.Tag)
				fmt.Fprintf(w, "Summary: %s\n", detail.Summary)
				if detail.Description != "" {
					fmt.Fprintf(w, "Description: %s\n", detail.Description)
				}
				if len(detail.Parameters) > 0 {
					fmt.Fprintf(w, "\nParameters:\n")
					for _, p := range detail.Parameters {
						req := ""
						if p.Required {
							req = " (required)"
						}
						fmt.Fprintf(w, "  %-20s %-10s %s%s\n", p.Name, p.Type, p.Description, req)
					}
				}
				if detail.RequestBody != nil {
					fmt.Fprintf(w, "\nRequest Body:\n")
					b, _ := json.MarshalIndent(detail.RequestBody, "  ", "  ")
					fmt.Fprintf(w, "  %s\n", b)
				}
				if detail.Response != nil {
					fmt.Fprintf(w, "\nResponse Schema:\n")
					b, _ := json.MarshalIndent(detail.Response, "  ", "  ")
					fmt.Fprintf(w, "  %s\n", b)
				}
			} else {
				data, _ := json.MarshalIndent(detail, "", "  ")
				fmt.Fprintln(w, string(data))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch of the OpenAPI spec")

	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestInfoCmd" -v`
Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/api/info.go internal/cmd/api/info_test.go
git commit -m "feat(api): add info subcommand for endpoint details"
```

---

### Task 6: `langsmith api METHOD path` — request execution

**Files:**
- Create: `internal/cmd/api/request.go`
- Create: `internal/cmd/api/request_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/api/request_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRunRequest_GET(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sessions" {
			t.Errorf("expected /api/v1/sessions, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"id":"s1"}]`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	code, err := runRequest(ts.URL, "test-key", "GET", "sessions", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected status 200, got %d", code)
	}
	if !strings.Contains(out.String(), `"id"`) {
		t.Errorf("expected JSON output, got %q", out.String())
	}
}

func TestRunRequest_POSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		json.Unmarshal(body, &data)
		if data["name"] != "test" {
			t.Errorf("expected name=test, got %v", data["name"])
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"new"}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	code, err := runRequest(ts.URL, "key", "POST", "sessions", `{"name":"test"}`, nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 201 {
		t.Errorf("expected 201, got %d", code)
	}
}

func TestRunRequest_ExtraHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "val" {
			t.Errorf("expected X-Custom=val, got %q", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	_, err := runRequest(ts.URL, "key", "GET", "sessions", "", []string{"X-Custom:val"}, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRequest_Include(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	_, err := runRequest(ts.URL, "key", "GET", "sessions", "", nil, true, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "200") {
		t.Errorf("expected status line, got %q", out.String())
	}
	if !strings.Contains(out.String(), "X-Request-Id") {
		t.Errorf("expected header in output, got %q", out.String())
	}
}

func TestRunRequest_4xxPrintsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"not found"}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	code, err := runRequest(ts.URL, "key", "GET", "sessions", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 404 {
		t.Errorf("expected 404, got %d", code)
	}
	if !strings.Contains(out.String(), "not found") {
		t.Errorf("expected error body in output, got %q", out.String())
	}
}

func TestRunRequest_BodyFromFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(200)
		w.Write(body)
	}))
	defer ts.Close()

	// Write temp file
	f, _ := os.CreateTemp(t.TempDir(), "body-*.json")
	f.WriteString(`{"from":"file"}`)
	f.Close()

	var out bytes.Buffer
	code, err := runRequest(ts.URL, "key", "POST", "sessions", "@"+f.Name(), nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
	if !strings.Contains(out.String(), "from") {
		t.Errorf("expected file body echoed, got %q", out.String())
	}
}

func TestResolveBody_InlineJSON(t *testing.T) {
	r, err := resolveBody(`{"key":"val"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := io.ReadAll(r)
	if string(data) != `{"key":"val"}` {
		t.Errorf("unexpected body: %s", data)
	}
}

func TestResolveBody_Empty(t *testing.T) {
	r, err := resolveBody("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r != nil {
		t.Error("expected nil reader for empty body")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestRunRequest|TestResolveBody" -v`
Expected: compilation error — `runRequest`, `resolveBody` don't exist.

- [ ] **Step 3: Implement request.go**

Create `internal/cmd/api/request.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// runRequest executes an HTTP request and writes the response to w.
// Returns the HTTP status code and any transport-level error.
func runRequest(apiURL, apiKey, method, path, body string, headers []string, include bool, w io.Writer) (int, error) {
	c := client.New(apiKey, apiURL)

	fullURL := resolveEndpoint(apiURL, path)
	// Compute relative path for RawDo (which prepends apiURL)
	relPath := strings.TrimPrefix(fullURL, apiURL)

	// Resolve body
	bodyReader, err := resolveBody(body)
	if err != nil {
		return 0, err
	}

	// Parse extra headers
	extraHeaders := make(http.Header)
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return 0, fmt.Errorf("invalid header format %q (expected Key:Value)", h)
		}
		extraHeaders.Set(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	ctx := contextBackground()
	statusCode, respHeaders, respBody, err := c.RawDo(ctx, method, relPath, bodyReader, extraHeaders)
	if err != nil {
		return 0, err
	}

	// Print response headers if --include
	if include {
		fmt.Fprintf(w, "HTTP/1.1 %d %s\n", statusCode, http.StatusText(statusCode))
		for k, vals := range respHeaders {
			for _, v := range vals {
				fmt.Fprintf(w, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintln(w)
	}

	// Pretty-print JSON if possible, otherwise print raw
	var prettyBuf bytes.Buffer
	if json.Indent(&prettyBuf, respBody, "", "  ") == nil {
		fmt.Fprintln(w, prettyBuf.String())
	} else {
		w.Write(respBody)
		fmt.Fprintln(w)
	}

	return statusCode, nil
}

// resolveBody resolves a --body value to an io.Reader.
//   - Empty string → nil (no body).
//   - "@-" → stdin.
//   - "@path" → file contents.
//   - Otherwise → treated as inline JSON string.
func resolveBody(body string) (io.Reader, error) {
	if body == "" {
		return nil, nil
	}
	if body == "@-" {
		return os.Stdin, nil
	}
	if strings.HasPrefix(body, "@") {
		filePath := body[1:]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading body file %q: %w", filePath, err)
		}
		return bytes.NewReader(data), nil
	}
	return strings.NewReader(body), nil
}

// contextBackground returns a background context. Extracted for testability.
var contextBackground = defaultContextBackground

func defaultContextBackground() contextType {
	return contextType{}
}

// We avoid importing context at the top and instead use it inline to keep the
// contextBackground variable simple for testing. This is actually cleaner
// with a real import:
```

Wait — that context abstraction is over-engineered. Let me simplify. Replace `request.go` with:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
)

// runRequest executes an HTTP request and writes the response to w.
// Returns the HTTP status code and any transport-level error.
func runRequest(apiURL, apiKey, method, path, body string, headers []string, include bool, w io.Writer) (int, error) {
	c := client.New(apiKey, apiURL)

	fullURL := resolveEndpoint(apiURL, path)
	// Compute relative path for RawDo (which prepends apiURL)
	relPath := strings.TrimPrefix(fullURL, apiURL)

	// Resolve body
	bodyReader, err := resolveBody(body)
	if err != nil {
		return 0, err
	}

	// Parse extra headers
	extraHeaders := make(http.Header)
	for _, h := range headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return 0, fmt.Errorf("invalid header format %q (expected Key:Value)", h)
		}
		extraHeaders.Set(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	statusCode, respHeaders, respBody, err := c.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
	if err != nil {
		return 0, err
	}

	// Print response headers if --include
	if include {
		fmt.Fprintf(w, "HTTP/1.1 %d %s\n", statusCode, http.StatusText(statusCode))
		for k, vals := range respHeaders {
			for _, v := range vals {
				fmt.Fprintf(w, "%s: %s\n", k, v)
			}
		}
		fmt.Fprintln(w)
	}

	// Pretty-print JSON if possible, otherwise print raw
	var prettyBuf bytes.Buffer
	if json.Indent(&prettyBuf, respBody, "", "  ") == nil {
		fmt.Fprintln(w, prettyBuf.String())
	} else {
		w.Write(respBody)
		fmt.Fprintln(w)
	}

	return statusCode, nil
}

// resolveBody resolves a --body value to an io.Reader.
//   - Empty string → nil (no body).
//   - "@-" → stdin.
//   - "@path" → file contents.
//   - Otherwise → treated as inline JSON string.
func resolveBody(body string) (io.Reader, error) {
	if body == "" {
		return nil, nil
	}
	if body == "@-" {
		return os.Stdin, nil
	}
	if strings.HasPrefix(body, "@") {
		filePath := body[1:]
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading body file %q: %w", filePath, err)
		}
		return bytes.NewReader(data), nil
	}
	return strings.NewReader(body), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestRunRequest|TestResolveBody" -v`
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/api/request.go internal/cmd/api/request_test.go
git commit -m "feat(api): add request execution for langsmith api METHOD path"
```

---

### Task 7: Parent command — api.go — wiring it all together

**Files:**
- Create: `internal/cmd/api/api.go`
- Create: `internal/cmd/api/api_test.go`

The parent `api` command registers `ls` and `info` as subcommands. When the first arg is an HTTP method (GET, POST, etc.), it handles the request itself rather than delegating to a subcommand.

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/api/api_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCmd_HasSubcommands(t *testing.T) {
	cmd := NewCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	if !names["ls"] {
		t.Error("missing subcommand 'ls'")
	}
	if !names["info"] {
		t.Error("missing subcommand 'info'")
	}
}

func TestNewCmd_UseField(t *testing.T) {
	cmd := NewCmd()
	if cmd.Use != "api" {
		t.Errorf("expected Use='api', got %q", cmd.Use)
	}
}

func TestNewCmd_RequestFlags(t *testing.T) {
	cmd := NewCmd()
	for _, name := range []string{"body", "header", "include"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found on api command", name)
		}
	}
}

func TestNewCmd_GETRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			json.NewEncoder(w).Encode(map[string]any{"openapi": "3.1.0", "paths": map[string]any{}})
			return
		}
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sessions" {
			t.Errorf("expected /api/v1/sessions, got %s", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--api-key", "test-key", "--api-url", ts.URL, "GET", "sessions"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Errorf("expected JSON response, got %q", out.String())
	}
}

func TestNewCmd_POSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			json.NewEncoder(w).Encode(map[string]any{"openapi": "3.1.0", "paths": map[string]any{}})
			return
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--api-key", "test-key", "--api-url", ts.URL, "POST", "sessions", "--body", `{"name":"x"}`})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "created") {
		t.Errorf("expected JSON response, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestNewCmd" -v`
Expected: compilation error — `NewCmd` doesn't exist.

- [ ] **Step 3: Implement api.go**

Create `internal/cmd/api/api.go`:

```go
package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// NewCmd creates the top-level `langsmith api` command.
func NewCmd() *cobra.Command {
	var (
		body    string
		headers []string
		include bool
	)

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Browse API endpoints and make authenticated requests",
		Long: `Browse LangSmith API endpoints and make authenticated HTTP requests.

Browse endpoints:
  langsmith api ls                          List all endpoints
  langsmith api ls --tag datasets           Filter by tag
  langsmith api ls --search "create"        Search endpoints
  langsmith api info GET sessions           Show endpoint details

Make requests:
  langsmith api GET sessions?limit=5
  langsmith api POST runs/query --body '{"session_id":"abc"}'
  langsmith api DELETE sessions/abc-123
  langsmith api POST datasets --body @body.json
  echo '{"name":"x"}' | langsmith api POST sessions --body @-
  langsmith api GET sessions --include`,
		// DisableFlagParsing would break subcommands, so we use Args + RunE
		// to detect HTTP method as first arg.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}

			method := strings.ToUpper(args[0])
			if !isHTTPMethod(method) {
				return fmt.Errorf("unknown subcommand or HTTP method: %q\nRun 'langsmith api --help' for usage", args[0])
			}

			path := args[1]

			apiKey := resolveAPIKey(cmd)
			if apiKey == "" {
				return fmt.Errorf("LANGSMITH_API_KEY not set")
			}
			apiURL := resolveAPIURL(cmd)

			w := cmd.OutOrStdout()
			statusCode, err := runRequest(apiURL, apiKey, method, path, body, headers, include, w)
			if err != nil {
				return err
			}
			if statusCode >= 400 {
				os.Exit(1)
			}
			return nil
		},
	}

	// Flags for request mode (ignored by ls/info subcommands)
	cmd.Flags().StringVar(&body, "body", "", `Request body (JSON string, @file, or @- for stdin)`)
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Additional headers (Key:Value, repeatable)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Include HTTP response headers in output")

	// Need to declare persistent flags that mirror root's for resolveAPIKey/resolveAPIURL
	// to work when tests call the api command directly without the root parent.
	cmd.PersistentFlags().String("api-key", "", "LangSmith API key [env: LANGSMITH_API_KEY]")
	cmd.PersistentFlags().String("api-url", "", "LangSmith API URL [env: LANGSMITH_ENDPOINT]")
	cmd.PersistentFlags().String("format", "json", "Output format: json or pretty")

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newInfoCmd())

	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -run "TestNewCmd" -v`
Expected: all tests PASS.

- [ ] **Step 5: Run all api package tests**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -v`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/api/api.go internal/cmd/api/api_test.go
git commit -m "feat(api): add parent command wiring ls, info, and request dispatch"
```

---

### Task 8: Register in root and update root test

**Files:**
- Modify: `internal/cmd/root.go:3-8` (add import) and line 67 (add command)
- Modify: `internal/cmd/root_test.go:12` (add "api" to expected list)

- [ ] **Step 1: Update root_test.go to expect "api"**

In `internal/cmd/root_test.go` line 12, change:

```go
expected := []string{"project", "trace", "run", "thread", "dataset", "example", "evaluator", "experiment", "self-update"}
```

to:

```go
expected := []string{"project", "trace", "run", "thread", "dataset", "example", "evaluator", "experiment", "self-update", "api"}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/ -run "TestRootCmd_HasAllSubcommands" -v`
Expected: FAIL — missing subcommand "api".

- [ ] **Step 3: Register api command in root.go**

In `internal/cmd/root.go`, add the import:

```go
import (
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmd/api"
	"github.com/spf13/cobra"
)
```

Then after `rootCmd.AddCommand(newUpdateCmd(rawVersion))` (line 67), add:

```go
rootCmd.AddCommand(api.NewCmd())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/ -run "TestRootCmd_HasAllSubcommands" -v`
Expected: PASS.

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./...`
Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/root.go internal/cmd/root_test.go
git commit -m "feat: register api command in root"
```

---

### Task 9: Smoke test

**Files:** None (manual verification).

- [ ] **Step 1: Verify help output**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api --help`
Expected: help text showing browse + request examples, ls/info subcommands, --body/--header/--include flags.

- [ ] **Step 2: Verify ls help**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api ls --help`
Expected: help text with --tag, --search, --refresh flags.

- [ ] **Step 3: Test ls against real API**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api ls --tag datasets | head -20`
Expected: JSON array of dataset endpoints.

- [ ] **Step 4: Test ls with pretty format**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api ls --tag datasets --format pretty`
Expected: table with METHOD, PATH, TAG, SUMMARY columns.

- [ ] **Step 5: Test info**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api info GET sessions`
Expected: JSON with method, path, summary, parameters, response_schema.

- [ ] **Step 6: Test GET request**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api GET sessions?limit=1`
Expected: JSON response with session data.

- [ ] **Step 7: Test --include**

Run: `cd /Users/mukil/langchain/langsmith-cli && go run ./cmd/langsmith api GET sessions?limit=1 --include`
Expected: HTTP headers printed before JSON body.
