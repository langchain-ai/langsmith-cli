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
	cmd := NewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--api-key", "key", "BOGUS", "sessions"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid method")
	}
	if !strings.Contains(err.Error(), "unknown subcommand or HTTP method") {
		t.Errorf("expected helpful error, got %q", err.Error())
	}
}
