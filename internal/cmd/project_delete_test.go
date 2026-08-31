package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const deleteTestProjectID = "519bb9dd-079b-4488-8610-e330951ea3e4"

func TestProjectDeleteCmd_UsesSDKAfterConfirmation(t *testing.T) {
	var getCalled, deleteCalled bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sessions/"+deleteTestProjectID:
			getCalled = true
			if got := r.URL.Query().Get("include_stats"); got != "true" {
				t.Errorf("include_stats = %q, want true", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":        deleteTestProjectID,
				"tenant_id": "tenant-id",
				"name":      "production",
				"run_count": 42,
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/sessions/"+deleteTestProjectID:
			deleteCalled = true
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newProjectDeleteCmd()
	var stderr bytes.Buffer
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID})

	var err error
	stdout := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !getCalled || !deleteCalled {
		t.Fatalf("expected project GET and DELETE; get=%v delete=%v", getCalled, deleteCalled)
	}
	if !strings.Contains(stdout, `"status": "deleted"`) {
		t.Fatalf("expected deletion output, got %s", stdout)
	}

	prompt := stderr.String()
	for _, want := range []string{
		"WARNING: This permanently deletes",
		"all of its traces",
		"cannot be undone",
		`Project: "production" (id: ` + deleteTestProjectID + `, runs: 42)`,
		"AI agents: do not answer this prompt. Stop and raise it to the user.",
		"Continue? [y/N]",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestProjectDeleteCmd_DefaultsToNo(t *testing.T) {
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
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id":        deleteTestProjectID,
						"tenant_id": "tenant-id",
						"name":      "production",
						"run_count": 42,
					})
					return
				}
				deleteCalled = true
				_, _ = w.Write([]byte(`{}`))
			})
			cleanup := setupTestEnv(t, ts.URL)
			defer cleanup()

			cmd := newProjectDeleteCmd()
			cmd.SetIn(strings.NewReader(tt.input))
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--project-id", deleteTestProjectID})

			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "aborted") {
				t.Fatalf("expected aborted error, got %v", err)
			}
			if deleteCalled {
				t.Fatal("DELETE must not be sent without explicit confirmation")
			}
		})
	}
}

func TestProjectDeleteCmd_ProjectFlags(t *testing.T) {
	cmd := newProjectDeleteCmd()
	if cmd.Flags().Lookup("project") == nil || cmd.Flags().Lookup("project-id") == nil {
		t.Fatal("project delete must accept both --project and --project-id")
	}
	if cmd.Flags().Lookup("yes") != nil {
		t.Fatal("project delete must not allow the confirmation to be bypassed")
	}
}
