package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- NormalizeURL ----------

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no-op", "https://api.smith.langchain.com", "https://api.smith.langchain.com"},
		{"strips /api/v1", "https://myhost.com/api/v1", "https://myhost.com"},
		{"strips /api/v1/", "https://myhost.com/api/v1/", "https://myhost.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeURL(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------- PlatformPath ----------

func TestDerivePlatformPrefix(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"bare host is multi-origin", "http://localhost:1980", "/v1/platform"},
		{"trailing slash", "http://localhost:1980/", "/v1/platform"},
		{"api/v1 suffix is single-origin", "https://host/api/v1", "/api/v1/platform"},
		{"api/v1 with trailing slash", "https://host/api/v1/", "/api/v1/platform"},
		{"api suffix is single-origin", "https://host/api", "/api/v1/platform"},
		{"saas default", "https://api.smith.langchain.com", "/v1/platform"},
		{"empty", "", "/v1/platform"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := derivePlatformPrefix(tt.endpoint); got != tt.want {
				t.Errorf("derivePlatformPrefix(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestClientPlatformPaths(t *testing.T) {
	const appID = "6f1c9b0e-6b3e-4a0e-9a4a-2c1d3e4f5a6b"
	tests := []struct {
		name           string
		endpoint       string
		wantCollection string
		wantSingleApp  string
	}{
		{"multi-origin", "http://localhost:1980", "/v1/platform/custom-apps", "/v1/platform/custom-apps/" + appID},
		{"single-origin", "https://host/api/v1", "/api/v1/platform/custom-apps", "/api/v1/platform/custom-apps/" + appID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("k", tt.endpoint)
			if got := c.CustomAppsPath(); got != tt.wantCollection {
				t.Errorf("CustomAppsPath() = %q, want %q", got, tt.wantCollection)
			}
			if got := c.CustomAppPath(appID); got != tt.wantSingleApp {
				t.Errorf("CustomAppPath() = %q, want %q", got, tt.wantSingleApp)
			}
		})
	}
}

func TestPlatformPath_EscapesElements(t *testing.T) {
	c := New("k", "http://localhost:1980")
	if got := c.CustomAppPath("a b/../c"); got != "/v1/platform/custom-apps/a%20b%2F..%2Fc" {
		t.Errorf("expected the app ID escaped, got %q", got)
	}
}

// ---------- New ----------

func TestNew_CreatesClient(t *testing.T) {
	c := New("test-key", "http://localhost:1234")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.SDK == nil {
		t.Fatal("expected non-nil SDK")
	}
	if c.apiKey != "test-key" {
		t.Errorf("expected apiKey=test-key, got %q", c.apiKey)
	}
	if c.apiURL != "http://localhost:1234" {
		t.Errorf("expected apiURL=http://localhost:1234, got %q", c.apiURL)
	}
	if c.sessionCache == nil {
		t.Fatal("expected non-nil sessionCache")
	}
}

func TestNew_TrimsTrailingSlash(t *testing.T) {
	c := New("key", "http://example.com/")
	if c.apiURL != "http://example.com" {
		t.Errorf("expected trailing slash trimmed, got %q", c.apiURL)
	}
}

func TestNew_EmptyURL(t *testing.T) {
	c := New("key", "")
	if c.apiURL != "" {
		t.Errorf("expected empty apiURL, got %q", c.apiURL)
	}
}

func TestNewWithOptions_CreatesOAuthClient(t *testing.T) {
	c := NewWithOptions(Options{
		OAuthAccessToken: "test-access-token",
		APIURL:           "http://localhost:1234",
		WorkspaceID:      "ws-123",
	})
	if c == nil || c.SDK == nil {
		t.Fatal("expected non-nil client and SDK")
	}
	if c.oauthAccessToken != "test-access-token" {
		t.Fatalf("unexpected OAuth access token: %q", c.oauthAccessToken)
	}
	if c.APIKey() != "" {
		t.Fatalf("expected empty API key, got %q", c.APIKey())
	}
}

func TestNewWithOptions_ProfileNameDelegatesAuthToSDK(t *testing.T) {
	var gotAPIKey string
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/info" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAPIKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "test"})
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_API_KEY", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "default",
  "profiles": {
    "default": {
      "api_key": "default-api-key"
    },
    "prod": {
      "oauth": {
        "access_token": "prod-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	c := NewWithOptions(Options{
		ProfileName: "prod",
		APIURL:      ts.URL,
	})
	if _, err := c.SDK.Info.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer prod-access-token" {
		t.Fatalf("expected explicit profile bearer auth, got %q", gotAuth)
	}
	if gotAPIKey != "" {
		t.Fatalf("expected explicit OAuth profile to override default API key, got %q", gotAPIKey)
	}
}

func TestNewWithOptions_APIKeyProfileRoutesThroughSDK(t *testing.T) {
	var gotAPIKey string
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "test"})
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_API_KEY", "")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "prod": {
      "api_key": "prod-api-key"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	// An api-key profile carries both ProfileName and its own APIKey (as
	// resolveClientOptions produces). It routes through WithProfile: the profile's
	// key is sent, no bearer.
	c := NewWithOptions(Options{
		APIKey:      "prod-api-key",
		ProfileName: "prod",
		APIURL:      ts.URL,
	})
	if _, err := c.SDK.Info.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "prod-api-key" {
		t.Fatalf("expected profile api key, got %q", gotAPIKey)
	}
	if gotAuth != "" {
		t.Fatalf("expected no bearer for an api-key profile, got %q", gotAuth)
	}
}

func TestNewWithOptions_ProfileNameTakesPrecedenceOverExplicitAPIKey(t *testing.T) {
	var gotAPIKey string
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "test"})
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_API_KEY", "")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "prod": {
      "oauth": {
        "access_token": "prod-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	// resolveClientOptions never produces this combo, but NewWithOptions is
	// exported: when a caller passes an explicit APIKey alongside a ProfileName,
	// the profile wins (routes through WithProfile) and the explicit key is not
	// sent for SDK calls.
	c := NewWithOptions(Options{
		APIKey:      "explicit-key",
		ProfileName: "prod",
		APIURL:      ts.URL,
	})
	if _, err := c.SDK.Info.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer prod-access-token" {
		t.Fatalf("expected profile OAuth bearer to win, got %q", gotAuth)
	}
	if gotAPIKey != "" {
		t.Fatalf("expected explicit API key not sent for SDK calls, got %q", gotAPIKey)
	}
}

// ---------- RawGet ----------

func TestRawGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/test/path" {
			t.Errorf("expected /test/path, got %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "my-key" {
			t.Errorf("expected x-api-key=my-key, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %q", r.Header.Get("Content-Type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	c := New("my-key", ts.URL)
	var result map[string]string
	err := c.RawGet(context.Background(), "/test/path", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestRawGet_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	var result map[string]any
	err := c.RawGet(context.Background(), "/fail", &result)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error to contain 403, got %q", err.Error())
	}
}

func TestRawGet_HTTPErrorStatus(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusConflict, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(status), status)
			}))
			defer ts.Close()

			err := New("key", ts.URL).RawGet(context.Background(), "/fail", nil)
			if got := IsNotFound(err); got != (status == http.StatusNotFound) {
				t.Errorf("IsNotFound(%v) = %t", err, got)
			}
			if got := IsConflict(err); got != (status == http.StatusConflict) {
				t.Errorf("IsConflict(%v) = %t", err, got)
			}
		})
	}
}

// Authenticated raw requests go through the SDK; they must still return the
// raw response for any status and make a single attempt.
func TestRawDo_ReturnsNonJSONErrorResponseWithoutRetrying(t *testing.T) {
	var attempts int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("X-API-Key") != "key" || r.Header.Get("X-Extra") != "yes" {
			t.Errorf("expected API key and extra header, got %v", r.Header)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("<html>slow down</html>"))
	}))
	defer ts.Close()

	status, _, headers, body, err := New("key", ts.URL).RawDo(context.Background(), http.MethodGet, "/api/v1/limited", nil, http.Header{"X-Extra": {"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusTooManyRequests || string(body) != "<html>slow down</html>" || headers.Get("Content-Type") != "text/html" {
		t.Fatalf("unexpected response: status=%d body=%q headers=%v", status, body, headers)
	}
	if attempts != 1 {
		t.Fatalf("expected one attempt, got %d", attempts)
	}
}

func TestRawGet_JSONHTTPErrorIncludesMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "org_scoped_key_requires_workspace",
			"message": "This API key requires a workspace ID.",
		})
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawGet(context.Background(), "/fail", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "org_scoped_key_requires_workspace") {
		t.Fatalf("expected error code, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "workspace ID") {
		t.Fatalf("expected workspace guidance, got %q", err.Error())
	}
}

func TestRawGet_NilResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawGet(context.Background(), "/path", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- RawPost ----------

func TestRawPost_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if body["name"] != "test" {
			t.Errorf("expected body name=test, got %v", body["name"])
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	var result map[string]string
	err := c.RawPost(context.Background(), "/create", map[string]any{"name": "test"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["id"] != "123" {
		t.Errorf("expected id=123, got %q", result["id"])
	}
}

func TestRawPost_NilBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawPost(context.Background(), "/path", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- RawDelete ----------

func TestRawDelete_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/items/abc" {
			t.Errorf("expected /items/abc, got %s", r.URL.Path)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawDelete(context.Background(), "/items/abc", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawDelete_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", 404)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawDelete(context.Background(), "/missing", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

// ---------- Headers ----------

func TestRawRequest_SetsAPIKeyHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret-key" {
			t.Errorf("expected x-api-key=secret-key, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("secret-key", ts.URL)
	_ = c.RawGet(context.Background(), "/test", nil)
}

func TestRawRequest_SetsWorkspaceHeader(t *testing.T) {
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-123")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-tenant-id"); got != "ws-123" {
			t.Errorf("expected x-tenant-id=ws-123, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	_ = c.RawGet(context.Background(), "/test", nil)
}

func TestRawRequest_NoWorkspaceHeaderWhenUnset(t *testing.T) {
	t.Setenv("LANGSMITH_WORKSPACE_ID", "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-tenant-id"); got != "" {
			t.Errorf("expected empty x-tenant-id, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	_ = c.RawGet(context.Background(), "/test", nil)
}

func TestRawRequest_SetsOAuthAuthorizationHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("expected bearer auth header, got %q", got)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Errorf("expected empty x-api-key, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := NewWithOptions(Options{OAuthAccessToken: "test-access-token", APIURL: ts.URL})
	_ = c.RawGet(context.Background(), "/test", nil)
}

// ---------- Error cases ----------

func TestRawGet_InvalidURL(t *testing.T) {
	c := New("key", "http://127.0.0.1:1") // unlikely to be listening
	err := c.RawGet(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRawGet_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("not json"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	var result map[string]any
	err := c.RawGet(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "decoding") {
		t.Errorf("expected decoding error, got %q", err.Error())
	}
}

// ---------- Various HTTP status codes ----------

func TestRawRequest_400Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", 400)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawGet(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected 400 in error, got %q", err.Error())
	}
}

func TestRawRequest_500Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", 500)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	err := c.RawPost(context.Background(), "/test", map[string]string{"a": "b"}, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in error, got %q", err.Error())
	}
}

// ---------- RawDo ----------

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
	status, _, hdr, body, err := c.RawDo(context.Background(), "PATCH", "/api/v1/sessions", nil, nil)
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
	status, _, _, body, err := c.RawDo(context.Background(), "POST", "/create", strings.NewReader(`{"name":"test"}`), nil)
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
	_, _, _, _, err := c.RawDo(context.Background(), "GET", "/test", nil, extra)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRawDo_RejectsCrossOriginPathsBeforeSendingCredentials(t *testing.T) {
	attackerRequests := 0
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerRequests++
		if r.Header.Get("x-api-key") != "" {
			t.Error("credentials reached the cross-origin server")
		}
	}))
	defer attacker.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected API request: %s", r.URL)
	}))
	defer api.Close()

	c := New("secret", api.URL)
	for _, path := range []string{attacker.URL + "/steal", "//example.com/steal", "ftp://example.com/steal"} {
		if _, _, _, _, err := c.RawDo(context.Background(), http.MethodGet, path, nil, nil); err == nil {
			t.Errorf("RawDo accepted cross-origin path %q", path)
		}
	}
	if attackerRequests != 0 {
		t.Fatalf("cross-origin server received %d requests", attackerRequests)
	}
}

func TestRawDo_Returns4xxWithoutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":"invalid"}`))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	status, _, _, body, err := c.RawDo(context.Background(), "GET", "/test", nil, nil)
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

func TestDoAbsolute_SendsCredentialsToAnotherHost(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the API URL was called: %s %s", r.Method, r.URL)
	}))
	defer api.Close()
	var got *http.Request
	var gotBody string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"no active plan"}`))
	}))
	defer other.Close()

	c := NewWithOptions(Options{APIKey: "my-key", APIURL: api.URL, WorkspaceID: "ws-1"})
	resp, err := c.DoAbsolute(t.Context(), http.MethodPost, other.URL+"/v2/deployments?limit=1", strings.NewReader(`{"a":1}`), http.Header{"X-Auth-Scheme": {"langsmith"}})
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusForbidden || string(resp.Body) != `{"detail":"no active plan"}` {
		t.Fatalf("response = %d %q, want 403 with its body", resp.StatusCode, resp.Body)
	}
	if got.URL.String() != "/v2/deployments?limit=1" {
		t.Errorf("request URL = %q, want /v2/deployments?limit=1", got.URL)
	}
	for name, want := range map[string]string{"X-Api-Key": "my-key", "X-Tenant-Id": "ws-1", "X-Auth-Scheme": "langsmith", "Content-Type": "application/json", "Authorization": ""} {
		if v := got.Header.Get(name); v != want {
			t.Errorf("%s = %q, want %q", name, v, want)
		}
	}
	if gotBody != `{"a":1}` {
		t.Errorf("body = %q, want {\"a\":1}", gotBody)
	}
}

func TestDoAbsolute_SendsTheOAuthProfileToken(t *testing.T) {
	var gotAuth, gotAPIKey string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-Api-Key")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer other.Close()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_API_KEY", "")
	if err := os.WriteFile(path, []byte(`{"profiles": {"prod": {"oauth": {"access_token": "prod-access-token"}}}}`), 0600); err != nil {
		t.Fatal(err)
	}

	c := NewWithOptions(Options{ProfileName: "prod", APIURL: "https://api.smith.langchain.com"})
	resp, err := c.DoAbsolute(t.Context(), http.MethodGet, other.URL+"/runs/crons", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK || gotAuth != "Bearer prod-access-token" || gotAPIKey != "" {
		t.Fatalf("status %d, Authorization %q, X-Api-Key %q; want 200 with the profile's bearer token only", resp.StatusCode, gotAuth, gotAPIKey)
	}
}

func TestDoAbsolute_RejectsRelativeURLs(t *testing.T) {
	c := New("my-key", "http://127.0.0.1:1")
	_, err := c.DoAbsolute(t.Context(), http.MethodGet, "/v2/deployments", nil, nil)
	if err == nil || err.Error() != `request URL "/v2/deployments" must be an absolute http(s) URL` {
		t.Fatalf("err = %v, want the absolute URL error", err)
	}
}

// isolateAmbientAuth gives the SDK a current profile signed in with OAuth
// to workspace "profile-ws", and returns nothing else it could pick up.
func isolateAmbientAuth(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	for _, key := range []string{"LANGSMITH_PROFILE", "LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_WORKSPACE_ID", "LANGSMITH_TENANT_ID", "LANGCHAIN_CUSTOM_HEADERS"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	config := `{"current_profile": "default", "profiles": {"default": {"workspace_id": "profile-ws", "oauth": {"access_token": "profile-token", "expires_at": "2099-01-01T00:00:00Z"}}}}`
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
}

// recordingServer records the headers of the last request it received.
func recordingServer(t *testing.T, status int) (*httptest.Server, *http.Header) {
	t.Helper()
	got := &http.Header{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server, got
}

func TestDoAbsolute_RefusesAClientWithoutItsOwnCredential(t *testing.T) {
	isolateAmbientAuth(t)
	t.Setenv("LANGSMITH_API_KEY", "shell-key")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a request was sent with ambient credentials: %s", r.Header)
	}))
	defer other.Close()

	c := NewWithOptions(Options{APIURL: other.URL})
	_, err := c.DoAbsolute(t.Context(), http.MethodGet, other.URL+"/v1/agent-auth/connections", nil, nil)

	want := "refusing to send a request to " + other.URL + ": the client has no LangSmith credential to send"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestDoAbsolute_RefusesPlainHTTPToAnotherMachine(t *testing.T) {
	c := NewWithOptions(Options{APIKey: "my-key", APIURL: "https://api.smith.langchain.com"})

	_, err := c.DoAbsolute(t.Context(), http.MethodGet, "http://deployment.example.com/runs/crons/search?x=1", nil, nil)

	want := "refusing to send LangSmith credentials over plain HTTP to http://deployment.example.com; use https"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestDoAbsolute_RefusesARedirectToAnotherOrigin(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the redirect was followed with %s", r.Header)
	}))
	defer elsewhere.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/steal", http.StatusFound)
	}))
	defer redirecting.Close()

	c := NewWithOptions(Options{APIKey: "my-key", APIURL: redirecting.URL, WorkspaceID: "ws-1"})
	_, err := c.DoAbsolute(t.Context(), http.MethodGet, redirecting.URL+"/runs/crons", nil, nil)

	want := "refusing to follow a redirect from " + redirecting.URL + " to another origin, " + elsewhere.URL
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %v, want it to contain %q", err, want)
	}
}

func TestDoAbsolute_SendsCustomHeadersOnlyToTheAPIOrigin(t *testing.T) {
	isolateAmbientAuth(t)
	t.Setenv("LANGCHAIN_CUSTOM_HEADERS", "X-Proxy-Token: proxy-secret")
	api, gotAPI := recordingServer(t, http.StatusOK)
	other, gotOther := recordingServer(t, http.StatusOK)

	c := NewWithOptions(Options{APIKey: "my-key", APIURL: api.URL})
	for _, target := range []string{api.URL, other.URL} {
		if _, err := c.DoAbsolute(t.Context(), http.MethodGet, target+"/info", nil, nil); err != nil {
			t.Fatal(err)
		}
	}

	if v := gotAPI.Get("X-Proxy-Token"); v != "proxy-secret" {
		t.Errorf("API X-Proxy-Token = %q, want proxy-secret", v)
	}
	if v := gotOther.Get("X-Proxy-Token"); v != "" {
		t.Errorf("other host X-Proxy-Token = %q, want none", v)
	}
	if v := gotOther.Get("X-Api-Key"); v != "my-key" {
		t.Errorf("other host X-Api-Key = %q, want my-key", v)
	}
}

func TestIsolatedOptionsIgnoreTheCurrentProfile(t *testing.T) {
	isolateAmbientAuth(t)
	t.Setenv("LANGSMITH_WORKSPACE_ID", "shell-ws")
	var hint strings.Builder
	defer func(previous *authHint) { processAuthHint = previous }(processAuthHint)
	processAuthHint = &authHint{w: &hint}
	server, got := recordingServer(t, http.StatusUnauthorized)

	c := NewWithOptions(Options{APIKey: "project-key", APIURL: server.URL, Isolated: true})
	_, err := c.SDK.Info.List(t.Context())

	if err == nil {
		t.Fatal("want the 401 as an error")
	}
	for name, want := range map[string]string{"X-Api-Key": "project-key", "Authorization": "", "X-Tenant-Id": ""} {
		if v := got.Get(name); v != want {
			t.Errorf("%s = %q, want %q", name, v, want)
		}
	}
	if c.WorkspaceID() != "" {
		t.Errorf("WorkspaceID() = %q, want none", c.WorkspaceID())
	}
	if hint.String() != "" {
		t.Errorf("auth hint = %q, want none", hint.String())
	}
}

func TestIsolatedOptionsWithoutACredentialSendNone(t *testing.T) {
	isolateAmbientAuth(t)
	server, got := recordingServer(t, http.StatusOK)

	c := NewWithOptions(Options{APIURL: server.URL, Isolated: true})
	if _, err := c.SDK.Info.List(t.Context()); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"X-Api-Key", "Authorization", "X-Tenant-Id"} {
		if v := got.Get(name); v != "" {
			t.Errorf("%s = %q, want none", name, v)
		}
	}
}

func TestIsNotFound_And_IsConflict_SeeThroughWrappedErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   func(error) bool
		other  func(error) bool
	}{
		{http.StatusNotFound, IsNotFound, IsConflict},
		{http.StatusConflict, IsConflict, IsNotFound},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(tc.status), tc.status)
			}))
			defer ts.Close()

			err := New("key", ts.URL).RawGet(context.Background(), "/x", nil)
			wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", err))
			if !tc.want(wrapped) {
				t.Errorf("detector did not see through wrapping: %v", wrapped)
			}
			if tc.other(wrapped) {
				t.Errorf("wrong detector matched: %v", wrapped)
			}
		})
	}
}

func TestRawDo_PreservesQueryString(t *testing.T) {
	var sawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.RawQuery
		w.WriteHeader(200)
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	_, _, _, _, err := c.RawDo(context.Background(), "GET", "/runs?status=pending&limit=5", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sawQuery != "status=pending&limit=5" {
		t.Errorf("expected query preserved, got %q", sawQuery)
	}
}

// ---------- Accessors ----------

func TestAPIKey(t *testing.T) {
	c := New("secret", "http://localhost")
	if c.APIKey() != "secret" {
		t.Errorf("expected secret, got %q", c.APIKey())
	}
}

func TestAPIURL(t *testing.T) {
	c := New("key", "http://localhost:1234")
	if c.APIURL() != "http://localhost:1234" {
		t.Errorf("expected http://localhost:1234, got %q", c.APIURL())
	}
}

// ---------- Run-query backend selection ----------

func TestSupportsTraceFeedbackWindow(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"", true},
		{"0.18.15", true},
		{"0.18.16rc1", true},
		{"v0.18.15", true},
		{"0.19.0", true},
		{"1.0.0", true},
		{"0.18.14", false},
		{"0.18", false},
		{"0.17.40", false},
		{"0.16.0", false},
	}
	for _, tc := range cases {
		if got := supportsTraceFeedbackWindow(tc.version); got != tc.want {
			t.Errorf("supportsTraceFeedbackWindow(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestUseV2API(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		// Cloud reports a non-release version.
		{"dev", true},
		{"", true},
		{"latest", true},
		// Self-hosted release semvers.
		{"0.16", true},
		{"0.16.0", true},
		{"0.16.18", true},
		{"0.16.18rc1", true},
		{"v0.16.0", true},
		{"0.17.3", true},
		{"1.0.0", true},
		{"1.2.3", true},
		{"0.15.9", false},
		{"0.15", false},
		{"0.10.7", false},
		{"0.0.1", false},
	}
	for _, tc := range cases {
		if got := useV2API(tc.version); got != tc.want {
			t.Errorf("useV2API(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}

func TestNewWithOptions_APIKeyRefProfileSendsResolvedKey(t *testing.T) {
	var gotAPIKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "test"})
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_API_KEY", "")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "prod": {
      "api_key_env": "PROD_LANGSMITH_KEY_UNSET"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	// The CLI resolves api_key_env/api_key_cmd itself; the SDK must send that
	// key rather than clearing it or resolving the reference again.
	c := NewWithOptions(Options{
		APIKey:      "resolved-key",
		ProfileName: "prod",
		APIURL:      ts.URL,
	})
	if _, err := c.SDK.Info.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotAPIKey != "resolved-key" {
		t.Fatalf("expected the CLI-resolved key, got %q", gotAPIKey)
	}
}
