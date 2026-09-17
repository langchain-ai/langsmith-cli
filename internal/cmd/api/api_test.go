package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
	for _, name := range []string{"body", "field", "header", "include", "input", "method", "raw-field"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found on api command", name)
		}
	}
}

func TestNewCmd_AutoPOSTWithFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		if data["name"] != "x" {
			t.Errorf("expected name=x, got %v", data["name"])
		}
		if data["limit"] != float64(10) {
			t.Errorf("expected limit=10, got %v", data["limit"])
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions", "-f", "name=x", "-F", "limit=10"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCmd_GETWithFieldsUsesQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("name") != "x" {
			t.Errorf("expected query name=x, got %q", r.URL.RawQuery)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions", "-X", "GET", "-f", "name=x"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewCmd_DefaultGETRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"openapi": "3.1.0", "paths": map[string]any{}})
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

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Errorf("expected JSON response, got %q", out.String())
	}
}

func TestNewCmd_POSTWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"openapi": "3.1.0", "paths": map[string]any{}})
			return
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions", "-X", "POST", "--body", `{"name":"x"}`})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "created") {
		t.Errorf("expected JSON response, got %q", out.String())
	}
}

func TestNewCmd_NoArgsShowsHelp(t *testing.T) {
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	// Should not error — shows help
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "Browse") {
		t.Errorf("expected help output, got %q", out.String())
	}
}

func TestNewCmd_InvalidMethod(t *testing.T) {
	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "key", "sessions", "-X", "BOGUS"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if !strings.Contains(err.Error(), "invalid HTTP method") {
		t.Errorf("expected helpful error, got %q", err.Error())
	}
}

// The SDK consumes an error response body to build its error type and only
// re-exposes it when it parsed as JSON, so a non-JSON error page would reach
// the user empty. The buffering middleware is what prevents that.
func TestNewCmd_PrintsNonJSONErrorBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("<!doctype html><title>429</title>429 Too Many Requests"))
	}))
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions"})
	_ = root.Execute() // a 4xx is reported, not swallowed; the body is what matters here

	if !strings.Contains(out.String(), "429 Too Many Requests") {
		t.Fatalf("error body missing from output:\n%s", out.String())
	}
}

func TestNewCmd_RetriesRetryableStatus(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"detail":"slow down"}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got < 2 {
		t.Fatalf("server saw %d attempt(s); a 429 should have been retried", got)
	}
	if !strings.Contains(out.String(), `"ok"`) {
		t.Fatalf("retry result missing from output:\n%s", out.String())
	}
}
