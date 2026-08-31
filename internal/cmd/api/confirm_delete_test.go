package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCmd_ConfirmsTracingProjectDeleteWithIdentity(t *testing.T) {
	var getCalled, deleteCalled bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sessions/project-id":
			getCalled = true
			if got := r.URL.Query().Get("include_stats"); got != "true" {
				t.Errorf("include_stats = %q, want true", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":        "project-id",
				"tenant_id": "tenant-id",
				"name":      "production",
				"run_count": 42,
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/sessions/project-id":
			deleteCalled = true
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	root := newTestRoot()
	var stdout, stderr bytes.Buffer
	root.SetIn(strings.NewReader("yes\n"))
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions/project-id", "-X", "DELETE"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !getCalled {
		t.Fatal("expected a project identity lookup")
	}
	if !deleteCalled {
		t.Fatal("expected DELETE after confirmation")
	}

	prompt := stderr.String()
	for _, want := range []string{
		"WARNING: This permanently deletes",
		"all of its traces",
		"cannot be undone",
		`Project: "production" (id: project-id, runs: 42)`,
		"AI agents: do not answer this prompt. Stop and raise it to the user.",
		"Continue? [y/N]",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestNewCmd_TracingProjectDeleteDefaultsToNo(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty response", input: "\n"},
		{name: "EOF", input: ""},
		{name: "yes followed by EOF", input: "yes"},
		{name: "explicit no", input: "n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteCalled := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id":        "project-id",
						"tenant_id": "tenant-id",
						"name":      "production",
						"run_count": 42,
					})
					return
				}
				deleteCalled = true
				_, _ = w.Write([]byte(`{}`))
			}))
			defer ts.Close()

			root := newTestRoot()
			root.SetIn(strings.NewReader(tt.input))
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions/project-id", "-X", "DELETE"})

			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), "aborted") {
				t.Fatalf("expected aborted error, got %v", err)
			}
			if deleteCalled {
				t.Fatal("DELETE must not be sent without explicit confirmation")
			}
		})
	}
}

func TestNewCmd_ConfirmsSessionsCollectionDelete(t *testing.T) {
	deleteCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/sessions" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		deleteCalled = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var stderr bytes.Buffer
	root.SetIn(strings.NewReader("y\n"))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "sessions", "-X", "DELETE"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE after confirmation")
	}
	if !strings.Contains(stderr.String(), "Target: /api/v1/sessions") {
		t.Fatalf("expected collection target in prompt, got:\n%s", stderr.String())
	}
}

func TestNewCmd_DoesNotPromptForOtherDelete(t *testing.T) {
	deleteCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/datasets/dataset-id" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		deleteCalled = true
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	root := newTestRoot()
	var stderr bytes.Buffer
	root.SetIn(strings.NewReader(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "datasets/dataset-id", "-X", "DELETE"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected non-project DELETE to proceed without prompting")
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected prompt: %s", stderr.String())
	}
}

func TestMatchTracingProjectDelete(t *testing.T) {
	tests := []struct {
		name   string
		apiURL string
		method string
		path   string
		wantID string
		want   bool
	}{
		{name: "shorthand collection", apiURL: "https://api.example.com", method: "DELETE", path: "sessions", want: true},
		{name: "shorthand ID", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id", wantID: "project-id", want: true},
		{name: "absolute API collection", apiURL: "https://api.example.com", method: "DELETE", path: "/api/v1/sessions/", want: true},
		{name: "absolute API ID", apiURL: "https://api.example.com", method: "delete", path: "/api/v1/sessions/project-id", wantID: "project-id", want: true},
		{name: "bare collection", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions", want: true},
		{name: "bare ID", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions/project-id", wantID: "project-id", want: true},
		{name: "same-host full URL", apiURL: "https://api.example.com", method: "DELETE", path: "https://api.example.com/api/v1/sessions/project-id", wantID: "project-id", want: true},
		{name: "base path", apiURL: "https://api.example.com/langsmith", method: "DELETE", path: "sessions/project-id", wantID: "project-id", want: true},
		{name: "other method", apiURL: "https://api.example.com", method: "GET", path: "sessions/project-id"},
		{name: "other route", apiURL: "https://api.example.com", method: "DELETE", path: "datasets/dataset-id"},
		{name: "session descendant", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id/runs"},
		{name: "external host", apiURL: "https://api.example.com", method: "DELETE", path: "https://other.example.com/api/v1/sessions/project-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, got := matchTracingProjectDelete(tt.apiURL, tt.method, tt.path)
			if got != tt.want {
				t.Fatalf("match = %v, want %v (target: %+v)", got, tt.want, target)
			}
			if target.id != tt.wantID {
				t.Errorf("id = %q, want %q", target.id, tt.wantID)
			}
		})
	}
}
