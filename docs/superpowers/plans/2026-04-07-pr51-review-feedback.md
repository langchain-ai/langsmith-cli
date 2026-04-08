# PR #51 Review Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address all review feedback on PR #51 (feat: add langsmith api command) from ericdong-langchain and QuentinBrosse.

**Architecture:** Extract shared resolve/cache utilities into dedicated packages (`internal/cmdutil`, `internal/cache`) to break circular dependencies. Refactor `client.go` to share HTTP logic between `RawDo` and `rawRequest`. Fix multiple small issues (os.Exit, header handling, trailing slashes, timeouts, protocol version, API key leakage).

**Tech Stack:** Go, cobra, net/http, httptest

---

### Task 1: Create `internal/cache` package

Extract cache utilities from `spec.go` into a reusable package per QuentinBrosse's comment on spec.go:341.

**Files:**
- Create: `internal/cache/cache.go`
- Create: `internal/cache/cache_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/cache/cache_test.go
package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Fatal("expected non-empty default cache dir")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
}

func TestPathForKey(t *testing.T) {
	p1 := PathForKey("/tmp/cache", "openapi", "https://api.smith.langchain.com")
	p2 := PathForKey("/tmp/cache", "openapi", "https://myhost.com")
	if p1 == p2 {
		t.Error("expected different paths for different keys")
	}
	if filepath.Dir(p1) != "/tmp/cache" {
		t.Errorf("expected dir /tmp/cache, got %s", filepath.Dir(p1))
	}
}

func TestReadIfFresh_Missing(t *testing.T) {
	_, err := ReadIfFresh("/nonexistent/path", time.Hour)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadIfFresh_Expired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	_, err := ReadIfFresh(path, time.Hour)
	if err == nil {
		t.Fatal("expected error for expired cache")
	}
}

func TestReadIfFresh_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadIfFresh(path, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("unexpected data: %s", data)
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.json")
	if err := Write(path, []byte(`{"written":true}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != `{"written":true}` {
		t.Errorf("unexpected content: %s", data)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cache/ -v`
Expected: compilation error — package doesn't exist yet.

- [ ] **Step 3: Write implementation**

```go
// internal/cache/cache.go
package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultDir returns ~/.langsmith/cache.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "langsmith-cache")
	}
	return filepath.Join(home, ".langsmith", "cache")
}

// PathForKey returns a cache file path using a SHA256 hash of the key.
func PathForKey(dir, prefix, key string) string {
	h := sha256.Sum256([]byte(key))
	name := fmt.Sprintf("%s-%x.json", prefix, h[:8])
	return filepath.Join(dir, name)
}

// ReadIfFresh reads a cached file if it exists and is within TTL.
func ReadIfFresh(path string, ttl time.Duration) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, fmt.Errorf("cache expired")
	}
	return os.ReadFile(path)
}

// Write writes data to a cache file, creating parent directories as needed.
func Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cache/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/cache.go internal/cache/cache_test.go
git commit -m "refactor: extract cache utilities into internal/cache package"
```

---

### Task 2: Create `internal/cmdutil` package

Extract shared resolve functions per ericdong's comment on resolve.go:14, QuentinBrosse's comments on resolve.go:14 and api.go:50. Unify with GetAPIKey/GetAPIURL/GetFormat and provide shared GetClient.

**Files:**
- Create: `internal/cmdutil/resolve.go`
- Create: `internal/cmdutil/resolve_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/cmdutil/resolve_test.go
package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().String("api-url", "", "")
	root.PersistentFlags().String("format", "json", "")
	return root
}

func TestResolveAPIKey_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-key", "from-flag")
	if got := ResolveAPIKey(cmd); got != "from-flag" {
		t.Errorf("expected from-flag, got %q", got)
	}
}

func TestResolveAPIKey_Env(t *testing.T) {
	cmd := newTestCmd()
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	if got := ResolveAPIKey(cmd); got != "from-env" {
		t.Errorf("expected from-env, got %q", got)
	}
}

func TestResolveAPIKey_Empty(t *testing.T) {
	cmd := newTestCmd()
	if got := ResolveAPIKey(cmd); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveAPIURL_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-url", "http://custom.example.com")
	if got := ResolveAPIURL(cmd); got != "http://custom.example.com" {
		t.Errorf("expected http://custom.example.com, got %q", got)
	}
}

func TestResolveAPIURL_Env(t *testing.T) {
	cmd := newTestCmd()
	t.Setenv("LANGSMITH_ENDPOINT", "http://env.example.com")
	if got := ResolveAPIURL(cmd); got != "http://env.example.com" {
		t.Errorf("expected http://env.example.com, got %q", got)
	}
}

func TestResolveAPIURL_Default(t *testing.T) {
	cmd := newTestCmd()
	if got := ResolveAPIURL(cmd); got != "https://api.smith.langchain.com" {
		t.Errorf("expected default, got %q", got)
	}
}

func TestResolveAPIURL_NormalizesTrailingAPIV1(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-url", "https://myhost.com/api/v1")
	if got := ResolveAPIURL(cmd); got != "https://myhost.com" {
		t.Errorf("expected normalized URL, got %q", got)
	}
}

func TestResolveFormat_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("format", "pretty")
	if got := ResolveFormat(cmd); got != "pretty" {
		t.Errorf("expected pretty, got %q", got)
	}
}

func TestResolveFormat_Default(t *testing.T) {
	cmd := newTestCmd()
	if got := ResolveFormat(cmd); got != "json" {
		t.Errorf("expected json, got %q", got)
	}
}

func TestGetClient_Success(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-key", "test-key")
	_ = cmd.PersistentFlags().Set("api-url", "http://localhost:1234")
	c, err := GetClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey() != "test-key" {
		t.Errorf("expected api key test-key, got %q", c.APIKey())
	}
	if c.APIURL() != "http://localhost:1234" {
		t.Errorf("expected api url http://localhost:1234, got %q", c.APIURL())
	}
}

func TestGetClient_MissingKey(t *testing.T) {
	cmd := newTestCmd()
	_, err := GetClient(cmd)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmdutil/ -v`
Expected: compilation error — package doesn't exist yet.

- [ ] **Step 3: Write implementation**

```go
// internal/cmdutil/resolve.go
package cmdutil

import (
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/spf13/cobra"
)

// ResolveAPIKey reads the API key from cobra's flag tree → env.
func ResolveAPIKey(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-key"); v != "" {
		return v
	}
	return os.Getenv("LANGSMITH_API_KEY")
}

// ResolveAPIURL reads the API URL from cobra's flag tree → env → default.
func ResolveAPIURL(cmd *cobra.Command) string {
	if v, _ := cmd.Flags().GetString("api-url"); v != "" {
		return client.NormalizeURL(v)
	}
	if v := os.Getenv("LANGSMITH_ENDPOINT"); v != "" {
		return client.NormalizeURL(v)
	}
	return "https://api.smith.langchain.com"
}

// ResolveFormat reads the output format from cobra's flag tree.
func ResolveFormat(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("format")
	if v == "" {
		return "json"
	}
	return v
}

// GetClient creates a LangSmith client from cobra flags, returning an error
// if the API key is not set.
func GetClient(cmd *cobra.Command) (*client.Client, error) {
	apiKey := ResolveAPIKey(cmd)
	if apiKey == "" {
		return nil, fmt.Errorf("LANGSMITH_API_KEY not set")
	}
	return client.New(apiKey, ResolveAPIURL(cmd)), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmdutil/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmdutil/resolve.go internal/cmdutil/resolve_test.go
git commit -m "refactor: extract shared resolve functions into internal/cmdutil package"
```

---

### Task 3: Refactor `client.go` — share logic between `RawDo` and `rawRequest`

Per QuentinBrosse's comment on client.go:103. Also adds `proto` to `RawDo` return for protocol version fix (ericdong request.go:57).

**Files:**
- Modify: `internal/client/client.go:99-187`

- [ ] **Step 1: Run existing tests as baseline**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/client/ ./internal/cmd/api/ -v -count=1`
Expected: all PASS.

- [ ] **Step 2: Refactor client.go**

Replace `RawDo` and `rawRequest` methods with a shared `doHTTP` helper. Add `proto string` to `RawDo` return signature.

In `internal/client/client.go`, replace the `RawDo` method (lines 99-133) and `rawRequest` method (lines 141-187) with:

```go
// RawDo performs an arbitrary HTTP request and returns the raw response.
// Unlike RawGet/RawPost/RawDelete, it does not unmarshal the response and
// does not treat 4xx/5xx as errors — callers decide how to handle status codes.
// body may be nil. extraHeaders are merged on top of the default auth headers.
func (c *Client) RawDo(ctx context.Context, method, path string, body io.Reader, extraHeaders http.Header) (statusCode int, proto string, respHeaders http.Header, respBody []byte, err error) {
	resp, err := c.doHTTP(ctx, method, path, body, extraHeaders)
	if err != nil {
		return 0, "", nil, nil, err
	}
	return resp.statusCode, resp.proto, resp.headers, resp.body, nil
}

type httpResponse struct {
	statusCode int
	proto      string
	headers    http.Header
	body       []byte
}

func (c *Client) doHTTP(ctx context.Context, method, path string, body io.Reader, extraHeaders http.Header) (*httpResponse, error) {
	url := c.apiURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if wsID := os.Getenv("LANGSMITH_WORKSPACE_ID"); wsID != "" {
		req.Header.Set("x-tenant-id", wsID)
	}
	for k, vals := range extraHeaders {
		req.Header[k] = vals
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &httpResponse{
		statusCode: resp.StatusCode,
		proto:      resp.Proto,
		headers:    resp.Header,
		body:       respBody,
	}, nil
}

func (c *Client) rawRequest(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doHTTP(ctx, method, path, bodyReader, nil)
	if err != nil {
		return err
	}

	if resp.statusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.statusCode, string(resp.body))
	}

	if result != nil {
		if err := json.Unmarshal(resp.body, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 3: Update `request.go` to match new `RawDo` signature (5 return values)**

In `internal/cmd/api/request.go`, update the `RawDo` call at line 50:

Change:
```go
statusCode, respHeaders, respBody, err := c.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
```
To:
```go
statusCode, proto, respHeaders, respBody, err := c.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
```

And update the `--include` status line at line 57:

Change:
```go
fmt.Fprintf(w, "HTTP/1.1 %d %s\n", statusCode, http.StatusText(statusCode))
```
To:
```go
fmt.Fprintf(w, "%s %d %s\n", proto, statusCode, http.StatusText(statusCode))
```

- [ ] **Step 4: Run tests to verify everything still passes**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/client/ ./internal/cmd/api/ -v -count=1`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/client/client.go internal/cmd/api/request.go
git commit -m "refactor: share HTTP logic between RawDo and rawRequest in client.go

Extracts doHTTP helper to eliminate duplicate request setup code.
Adds proto to RawDo return for accurate protocol version display."
```

---

### Task 4: Refactor `api` package to use `cmdutil` and `cache`

Remove duplicate resolve functions (ericdong resolve.go:14, QuentinBrosse resolve.go:14 + api.go:50). Remove test overrides in ls.go/info.go in favor of cmdutil-based resolution. Use cache package in spec.go.

**Files:**
- Modify: `internal/cmd/api/resolve.go` — remove `resolveAPIKey`, `resolveAPIURL`, `resolveFormat`; keep `resolveEndpoint`, `isHTTPMethod`
- Modify: `internal/cmd/api/api.go` — use `cmdutil.GetClient`
- Modify: `internal/cmd/api/ls.go` — use `cmdutil.ResolveAPIURL`, `cmdutil.ResolveFormat`, `cache.DefaultDir`; remove test overrides
- Modify: `internal/cmd/api/info.go` — use `cmdutil.ResolveAPIURL`, `cmdutil.ResolveFormat`, `cache.DefaultDir`; remove test overrides
- Modify: `internal/cmd/api/spec.go` — use `cache` package
- Modify: `internal/cmd/api/ls_test.go` — update test setup
- Modify: `internal/cmd/api/info_test.go` — update test setup
- Modify: `internal/cmd/api/spec_test.go` — update for cache package usage
- Modify: `internal/cmd/api/resolve_test.go` — remove tests for deleted functions (they're now in cmdutil)

- [ ] **Step 1: Run existing tests as baseline**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/cmd/api/ -v -count=1`
Expected: all PASS.

- [ ] **Step 2: Update resolve.go — remove duplicate functions**

Replace the entire file `internal/cmd/api/resolve.go` with:

```go
package api

import (
	"strings"
)

// resolveEndpoint resolves an endpoint argument to a full URL.
//   - Full URL (http:// or https://) → returned as-is.
//   - Absolute path (starts with /) → baseURL + path.
//   - Shorthand (e.g. "sessions") → baseURL + /api/v1/ + path.
func resolveEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
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
```

Note: `resolveEndpoint` now trims trailing slash from baseURL (ericdong resolve.go:54 nit).

- [ ] **Step 3: Update api.go — use cmdutil.GetClient, return error instead of os.Exit**

Replace `internal/cmd/api/api.go` with:

```go
package api

import (
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
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
  langsmith api ls                              List all endpoints
  langsmith api ls --tag datasets               Filter by tag
  langsmith api ls --search create              Search endpoints
  langsmith api info GET sessions               Show endpoint details

Make requests:
  langsmith api GET sessions?limit=5
  langsmith api POST runs/query --body '{"session_id":"abc"}'
  langsmith api DELETE sessions/abc-123
  langsmith api POST datasets --body @body.json
  echo '{"name":"x"}' | langsmith api POST sessions --body @-
  langsmith api GET sessions --include`,
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

			c, err := cmdutil.GetClient(cmd)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			statusCode, err := runRequest(c, method, path, body, headers, include, w)
			if err != nil {
				return err
			}
			if statusCode >= 400 {
				return fmt.Errorf("HTTP %d", statusCode)
			}
			return nil
		},
	}

	// Flags for request mode
	cmd.Flags().StringVar(&body, "body", "", `Request body (JSON string, @file, or @- for stdin)`)
	cmd.Flags().StringArrayVarP(&headers, "header", "H", nil, "Additional headers (Key:Value, repeatable)")
	cmd.Flags().BoolVarP(&include, "include", "i", false, "Include HTTP response headers in output")

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newInfoCmd())

	return cmd
}
```

Key changes:
- Uses `cmdutil.GetClient(cmd)` instead of manual resolveAPIKey/resolveAPIURL + client.New
- Returns `fmt.Errorf("HTTP %d", statusCode)` instead of `os.Exit(1)` (ericdong api.go:62)
- Removes quotes from `--search "create"` example (QuentinBrosse ls.go:36 — applied here since the example is in api.go's help text)
- `runRequest` now takes `*client.Client` instead of separate apiURL/apiKey args

- [ ] **Step 4: Update request.go — accept client, use Add for headers, skip auth for external hosts**

Replace `internal/cmd/api/request.go` with:

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
func runRequest(c *client.Client, method, path, body string, headers []string, include bool, w io.Writer) (int, error) {
	apiURL := c.APIURL()
	fullURL := resolveEndpoint(apiURL, path)

	// RawDo prepends apiURL, so compute the relative path.
	// For full URLs with a different host, pass the full URL as the path
	// to a client constructed with an empty base URL and no API key
	// (don't leak credentials to external hosts).
	reqClient := c
	relPath := fullURL
	if strings.HasPrefix(fullURL, apiURL) {
		relPath = strings.TrimPrefix(fullURL, apiURL)
	} else if strings.HasPrefix(fullURL, "http://") || strings.HasPrefix(fullURL, "https://") {
		reqClient = client.New("", "")
	}

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
		extraHeaders.Add(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	statusCode, proto, respHeaders, respBody, err := reqClient.RawDo(context.Background(), method, relPath, bodyReader, extraHeaders)
	if err != nil {
		return 0, err
	}

	// Print response headers if --include
	if include {
		fmt.Fprintf(w, "%s %d %s\n", proto, statusCode, http.StatusText(statusCode))
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
		if _, err := w.Write(respBody); err != nil {
			return statusCode, fmt.Errorf("writing response: %w", err)
		}
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

Key changes:
- Takes `*client.Client` instead of `apiURL, apiKey` (enables cmdutil.GetClient unification)
- Uses `extraHeaders.Add()` instead of `.Set()` (ericdong request.go:47)
- Uses `proto` from RawDo instead of hardcoded `HTTP/1.1` (ericdong request.go:57)
- External host requests use `client.New("", "")` — no API key leaked (QuentinBrosse request.go:32)

- [ ] **Step 5: Update ls.go — use cmdutil and cache, remove test overrides**

Replace `internal/cmd/api/ls.go` with:

```go
package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
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
  langsmith api ls --search create
  langsmith api ls --tag run --search query
  langsmith api ls --refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL := cmdutil.ResolveAPIURL(cmd)
			cacheDir := cache.DefaultDir()
			format := cmdutil.ResolveFormat(cmd)

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

Key changes:
- Uses `cmdutil.ResolveAPIURL(cmd)`, `cmdutil.ResolveFormat(cmd)`, `cache.DefaultDir()`
- Removes test override vars (`lsAPIURL`, `lsCacheDir`, `lsFormat`)
- Removes quotes from `--search "create"` example (QuentinBrosse ls.go:36)

- [ ] **Step 6: Update info.go — use cmdutil and cache, remove test overrides**

Replace `internal/cmd/api/info.go` with:

```go
package api

import (
	"encoding/json"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/spf13/cobra"
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

			apiURL := cmdutil.ResolveAPIURL(cmd)
			cacheDir := cache.DefaultDir()
			format := cmdutil.ResolveFormat(cmd)

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

Key changes:
- Uses `cmdutil.ResolveAPIURL(cmd)`, `cmdutil.ResolveFormat(cmd)`, `cache.DefaultDir()`
- Removes test override vars (`infoAPIURL`, `infoCacheDir`, `infoFormat`)

- [ ] **Step 7: Update spec.go — use cache package, add HTTP timeout**

In `internal/cmd/api/spec.go`:

Replace the imports to add `cache` and `time`, remove `crypto/sha256` and `path/filepath`:

```go
import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/cache"
)
```

Replace the `loadSpec` function (lines 271-311) with:

```go
func loadSpec(apiURL, cacheDir string, forceRefresh bool) (*OpenAPISpec, error) {
	cachePath := cache.PathForKey(cacheDir, "openapi", apiURL)

	if !forceRefresh {
		if data, err := cache.ReadIfFresh(cachePath, specCacheTTL); err == nil {
			var spec OpenAPISpec
			if err := json.Unmarshal(data, &spec); err == nil {
				return &spec, nil
			}
		}
	}

	// Fetch from server
	specURL := apiURL + "/openapi.json"
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Get(specURL)
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

	// Write cache (best-effort)
	_ = cache.Write(cachePath, data)

	return &spec, nil
}
```

Remove these functions from spec.go (they've been moved to the cache package):
- `loadCachedSpec` (lines 314-331)
- `specCachePath` (lines 333-338)
- `defaultCacheDir` (lines 340-347)

Keep `specCacheTTL` constant at the top.

Remove `"crypto/sha256"`, `"os"`, and `"path/filepath"` from the import block (no longer needed).

- [ ] **Step 8: Update tests — ls_test.go, info_test.go, spec_test.go, resolve_test.go, request_test.go**

For `ls_test.go` and `info_test.go`: tests previously set package-level override vars like `lsAPIURL`. Now they should use `newTestRoot()` and pass `--api-url` flag. However, since `ls` and `info` subcommands use `cmdutil.ResolveAPIURL(cmd)` which reads from cobra flags, we need a different testing approach.

The simplest approach: tests should execute the full command tree via `newTestRoot()` with `--api-url` pointing to the test server. Read the existing tests to understand current patterns and adapt.

For `spec_test.go`: `specCachePath` and `loadCachedSpec` no longer exist. Tests that used them should use `cache.PathForKey` and `cache.ReadIfFresh` instead. The `TestSpecCachePath` test moves to `cache_test.go` (already covered in Task 1).

For `resolve_test.go`: remove `resolveAPIKey/resolveAPIURL/resolveFormat` tests (moved to cmdutil). Keep `resolveEndpoint` and `isHTTPMethod` tests. Add a test for trailing slash trimming.

For `request_test.go`: update `runRequest` calls to pass `*client.Client` instead of `apiURL, apiKey`. Add test for multi-value header with `Add`.

This step requires careful reading and updating of each test file. Key changes per file:

**resolve_test.go** — Add trailing slash test case, remove nothing since resolveAPIKey tests don't exist here:

Add this test case to `TestResolveEndpoint`:
```go
{"trailing slash on base", "https://api.smith.langchain.com/", "sessions", "https://api.smith.langchain.com/api/v1/sessions"},
```

**request_test.go** — Update all `runRequest` calls and add multi-value header test:

All `runRequest(tsURL, "key", ...)` calls become `runRequest(client.New("key", tsURL), ...)`. Import `"github.com/langchain-ai/langsmith-cli/internal/client"`.

Add this test:
```go
func TestRunRequest_MultiValueHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vals := r.Header.Values("X-Multi")
		if len(vals) != 2 || vals[0] != "one" || vals[1] != "two" {
			t.Errorf("expected X-Multi=[one, two], got %v", vals)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", ts.URL)
	_, err := runRequest(c, "GET", "sessions", "", []string{"X-Multi:one", "X-Multi:two"}, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

The `TestRunRequest_FullURLDifferentHost` test should verify that the API key is NOT sent to external hosts:
```go
func TestRunRequest_FullURLDifferentHost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom/endpoint" {
			t.Errorf("expected /custom/endpoint, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("expected no x-api-key for external host, got %q", r.Header.Get("x-api-key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"full_url":true}`))
	}))
	defer ts.Close()

	var out bytes.Buffer
	c := client.New("key", "https://different.host")
	code, err := runRequest(c, "GET", ts.URL+"/custom/endpoint", "", nil, false, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 200 {
		t.Errorf("expected 200, got %d", code)
	}
}
```

**ls_test.go** — Change from test override vars to `newTestRoot()` command execution. Each test should set up a test spec server, then execute `langsmith api ls --api-url <server-url>` through `newTestRoot()`.

**info_test.go** — Same pattern as ls_test.go: use `newTestRoot()` with `--api-url`.

**spec_test.go** — Remove `TestSpecCachePath` (covered by cache_test.go). Update `loadSpec` tests to use `cache.PathForKey` where they referenced `specCachePath`. Remove references to `loadCachedSpec`.

- [ ] **Step 9: Run all tests**

Run: `cd /Users/mukil/langchain/langsmith-cli && go test ./internal/... -v -count=1`
Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/cmd/api/ internal/cmd/root.go
git commit -m "refactor: unify resolve/cache across cmd and api packages

- api package now uses cmdutil.ResolveAPIKey/URL/Format and cmdutil.GetClient
- api package now uses cache package for spec caching
- Removes duplicate resolve functions from api/resolve.go
- Removes test override vars from ls.go and info.go
- Fixes trailing slash handling in resolveEndpoint
- Uses Add instead of Set for multi-value headers
- Uses actual protocol version instead of hardcoded HTTP/1.1
- Returns error instead of os.Exit(1) for HTTP 4xx/5xx
- Adds timeout to OpenAPI spec fetch
- Does not send API key to external hosts
- Removes quotes from help text examples"
```

---

### Task 5: Reply to PR comments

Post reply comments on all open review threads.

**Files:** None (GitHub API calls only)

- [ ] **Step 1: Reply to all open review comments**

Use `gh api` to reply to each open comment thread. Reference the specific commits that address each piece of feedback.

Comment replies:
1. **ericdong on spec.go:282** (timeout) — "Fixed — `loadSpec` now uses `&http.Client{Timeout: 30 * time.Second}` instead of `http.Get`."
2. **ericdong on resolve.go:14** (shared package) — "Done — created `internal/cmdutil` package with shared `ResolveAPIKey`, `ResolveAPIURL`, `ResolveFormat`, and `GetClient`. Both `cmd` and `cmd/api` now import from cmdutil."
3. **ericdong on api.go:62** (os.Exit) — "Fixed — now returns `fmt.Errorf(\"HTTP %d\", statusCode)` and lets cobra/main handle the exit."
4. **ericdong on request.go:57** (hardcoded protocol) — "Fixed — `RawDo` now returns the actual `resp.Proto` and `--include` uses it."
5. **ericdong on request.go:47** (Add vs Set) — "Fixed — using `extraHeaders.Add()` now."
6. **ericdong on resolve.go:54** (trailing slash) — "Fixed — `resolveEndpoint` now trims trailing slash from baseURL."
7. **QuentinBrosse on client.go:103** (share logic) — "Done — extracted `doHTTP` helper that both `RawDo` and `rawRequest` use."
8. **QuentinBrosse on api.go:50** (unify with MustGetClient) — "Done — `cmdutil.GetClient(cmd)` is now the shared client constructor. The `api` command uses it directly."
9. **QuentinBrosse on resolve.go:14** (unify with GetAPIKey) — "Done — unified in `internal/cmdutil`."
10. **QuentinBrosse on spec.go:341** (extract cache) — "Done — created `internal/cache` package with `DefaultDir`, `PathForKey`, `ReadIfFresh`, and `Write`."
11. **QuentinBrosse on ls.go:36** (remove quotes) — "Fixed."
12. **QuentinBrosse on request.go:47** (add test) — "Added `TestRunRequest_MultiValueHeaders` test."
13. **QuentinBrosse on request.go:32** (API key to external hosts) — "Fixed — external host requests now use `client.New(\"\", \"\")` so no API key is sent."

- [ ] **Step 2: Verify all comments are replied to**

Run: `gh api repos/langchain-ai/langsmith-cli/pulls/51/comments --jq '[.[] | select(.user.login != "devin-ai-integration[bot]" and .user.login != "langchain-infra") | {id: .id, author: .user.login, file: .path, line: .line}]'`

Confirm all human review comments have replies.
