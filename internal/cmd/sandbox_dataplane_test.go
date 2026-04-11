package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newBytesReader(b []byte) io.Reader {
	return bytes.NewReader(b)
}

func TestDataplaneURL_TrailingSlash(t *testing.T) {
	tests := []struct {
		base     string
		path     string
		expected string
	}{
		{"https://example.com/sandbox/", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com/sandbox", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com/sandbox///", "/execute", "https://example.com/sandbox/execute"},
		{"https://example.com", "/upload?path=/root/.ssh", "https://example.com/upload?path=/root/.ssh"},
	}
	for _, tc := range tests {
		got := dataplaneURL(tc.base, tc.path)
		if got != tc.expected {
			t.Errorf("dataplaneURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.expected)
		}
	}
}

func TestDataplaneWSURL(t *testing.T) {
	tests := []struct {
		base     string
		path     string
		expected string
	}{
		{"https://example.com/sandbox", "/execute/ws", "wss://example.com/sandbox/execute/ws"},
		{"http://example.com/sandbox", "/execute/ws", "ws://example.com/sandbox/execute/ws"},
		{"https://example.com/sandbox/", "/execute/ws", "wss://example.com/sandbox/execute/ws"},
		{"http://localhost:8080/sb-123/", "/tunnel", "ws://localhost:8080/sb-123/tunnel"},
	}
	for _, tc := range tests {
		got := dataplaneWSURL(tc.base, tc.path)
		if got != tc.expected {
			t.Errorf("dataplaneWSURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.expected)
		}
	}
}

func TestDataplanePost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/execute" {
			t.Errorf("expected /execute, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stdout":    "hello\n",
			"stderr":    "",
			"exit_code": 0,
		})
	}))
	defer srv.Close()

	var result execResult
	if err := dataplanePost(srv.URL, "/execute", map[string]interface{}{"command": []string{"echo", "hello"}}, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", result.ExitCode)
	}
}

func TestDataplanePost_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer srv.Close()

	var result execResult
	err := dataplanePost(srv.URL, "/execute", map[string]interface{}{"command": []string{"echo"}}, &result)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if got := err.Error(); got != `HTTP 500: {"error": "internal server error"}` {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestDataplanePost_TrailingSlashURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/execute" {
			t.Errorf("expected /execute, got %s (double slash?)", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// URL with trailing slash should not produce //execute
	if err := dataplanePost(srv.URL+"/", "/execute", nil, &json.RawMessage{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDataplanePostRaw_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("expected text/plain content type, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := []byte("file content")
	err := dataplanePostRaw(srv.URL, "/upload?path=/root/test", "text/plain", newBytesReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDataplanePostRaw_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("forbidden"))
	}))
	defer srv.Close()

	err := dataplanePostRaw(srv.URL, "/upload", "text/plain", newBytesReader([]byte("data")))
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}

func TestDataplanePost_NilResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	// Should not error when result is nil (fire-and-forget)
	if err := dataplanePost(srv.URL, "/action", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
