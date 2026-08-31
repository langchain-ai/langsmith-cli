package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCmd_BlocksRawTracingProjectDelete(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "collection", path: "sessions"},
		{name: "project ID", path: "sessions/project-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCalled := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCalled = true
				http.Error(w, "request must be blocked", http.StatusInternalServerError)
			}))
			defer ts.Close()

			root := newTestRoot()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, tt.path, "-X", "DELETE"})

			err := root.Execute()
			if err == nil {
				t.Fatal("expected raw tracing project DELETE to be blocked")
			}
			if !strings.Contains(err.Error(), "langsmith project delete --project-id PROJECT_ID") {
				t.Fatalf("expected project delete guidance, got %v", err)
			}
			if requestCalled {
				t.Fatal("blocked request reached the server")
			}
		})
	}
}

func TestNewCmd_DoesNotBlockOtherDelete(t *testing.T) {
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
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"api", "--api-key", "test-key", "--api-url", ts.URL, "datasets/dataset-id", "-X", "DELETE"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected non-project DELETE to proceed")
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestIsRawTracingProjectDelete(t *testing.T) {
	tests := []struct {
		name   string
		apiURL string
		method string
		path   string
		want   bool
	}{
		{name: "shorthand collection", apiURL: "https://api.example.com", method: "DELETE", path: "sessions", want: true},
		{name: "shorthand ID", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id", want: true},
		{name: "absolute API collection", apiURL: "https://api.example.com", method: "DELETE", path: "/api/v1/sessions/", want: true},
		{name: "absolute API ID", apiURL: "https://api.example.com", method: "delete", path: "/api/v1/sessions/project-id", want: true},
		{name: "bare collection", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions", want: true},
		{name: "bare ID", apiURL: "https://api.example.com", method: "DELETE", path: "/sessions/project-id", want: true},
		{name: "same-host full URL", apiURL: "https://api.example.com", method: "DELETE", path: "https://api.example.com/api/v1/sessions/project-id", want: true},
		{name: "base path", apiURL: "https://api.example.com/langsmith", method: "DELETE", path: "sessions/project-id", want: true},
		{name: "other method", apiURL: "https://api.example.com", method: "GET", path: "sessions/project-id"},
		{name: "other route", apiURL: "https://api.example.com", method: "DELETE", path: "datasets/dataset-id"},
		{name: "session descendant", apiURL: "https://api.example.com", method: "DELETE", path: "sessions/project-id/runs"},
		{name: "external host", apiURL: "https://api.example.com", method: "DELETE", path: "https://other.example.com/api/v1/sessions/project-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRawTracingProjectDelete(tt.apiURL, tt.method, tt.path); got != tt.want {
				t.Errorf("isRawTracingProjectDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}
