package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedCommandsAreRegistered(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	for _, name := range generatedCommands {
		matches := 0
		for _, c := range root.Commands() {
			if c.Name() == name {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("command %q registered %d times, want 1", name, matches)
		}
	}
}

func TestTranslateGlobalFlags(t *testing.T) {
	rest, profile := translateGlobalFlags([]string{
		"list", "--api-url", "http://x", "--workspace=ws", "--profile", "dev", "--format", "json",
	})
	want := []string{"list", "--base-url", "http://x", "--tenant-id=ws", "--format", "json"}
	if strings.Join(rest, " ") != strings.Join(want, " ") {
		t.Errorf("rest = %q, want %q", rest, want)
	}
	if profile != "dev" {
		t.Errorf("profile = %q, want dev", profile)
	}
}

func TestGeneratedDeleteRequiresConfirmation(t *testing.T) {
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer ts.Close()
	t.Setenv("HOME", t.TempDir())

	root := NewRootCmd("dev", "dev")
	var stderr bytes.Buffer
	root.SetIn(strings.NewReader("n\n"))
	root.SetErr(&stderr)
	root.SetArgs([]string{"--api-url", ts.URL, "--api-key", "k", "prompt-webhooks", "delete", "--webhook-id", "abc"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "not confirmed") {
		t.Fatalf("Execute error = %v, want a not-confirmed error", err)
	}
	if requests != 0 {
		t.Errorf("server received %d requests, want 0", requests)
	}
	if !strings.Contains(stderr.String(), "AI agents: do not answer this prompt") {
		t.Errorf("stderr = %q, want the delete warning", stderr.String())
	}
}
