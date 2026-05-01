package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHubGet_Success(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/my-skill" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"repo": map[string]any{
				"id":          "abc",
				"full_name":   "acme/my-skill",
				"repo_handle": "my-skill",
				"repo_type":   "skill",
				"is_public":   true,
				"tags":        []string{"x"},
				"num_commits": 3,
				"created_at":  "2026-04-01T00:00:00Z",
				"updated_at":  "2026-04-02T00:00:00Z",
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"get", "acme/my-skill"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if !strings.Contains(out, `"full_name": "acme/my-skill"`) {
		t.Errorf("missing full_name in output:\n%s", out)
	}
	if strings.Contains(out, `"repo":`) {
		t.Errorf("output should be unwrapped; got:\n%s", out)
	}
}

func TestHubGet_DefaultsOwnerToDash(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/-/my-skill" {
			t.Errorf("path = %q (want /repos/-/my-skill)", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"repo": map[string]any{"full_name": "-/my-skill"}})
	})
	defer setupTestEnv(t, srv.URL)()

	captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"get", "my-skill"})
		_ = cmd.Execute()
	})
}
