package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
