package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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
	if !containsStr(err.Error(), "403") {
		t.Errorf("expected error to contain 403, got %q", err.Error())
	}
}

func TestRawGet_NilResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
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
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "test" {
			t.Errorf("expected body name=test, got %v", body["name"])
		}
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
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
		w.Write([]byte("{}"))
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
		w.Write([]byte("{}"))
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
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("secret-key", ts.URL)
	c.RawGet(context.Background(), "/test", nil)
}

func TestRawRequest_SetsWorkspaceHeader(t *testing.T) {
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-123")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-tenant-id"); got != "ws-123" {
			t.Errorf("expected x-tenant-id=ws-123, got %q", got)
		}
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	c.RawGet(context.Background(), "/test", nil)
}

func TestRawRequest_NoWorkspaceHeaderWhenUnset(t *testing.T) {
	t.Setenv("LANGSMITH_WORKSPACE_ID", "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-tenant-id"); got != "" {
			t.Errorf("expected empty x-tenant-id, got %q", got)
		}
		w.WriteHeader(200)
		w.Write([]byte("{}"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	c.RawGet(context.Background(), "/test", nil)
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
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	c := New("key", ts.URL)
	var result map[string]any
	err := c.RawGet(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !containsStr(err.Error(), "decoding") {
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
	if !containsStr(err.Error(), "400") {
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
	if !containsStr(err.Error(), "500") {
		t.Errorf("expected 500 in error, got %q", err.Error())
	}
}

// helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
