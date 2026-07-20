package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Apps are uniform now — list takes no context_type filter and hits the
// endpoint with no query string.
func TestAppsList_ListsAllApps(t *testing.T) {
	var sawQuery string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: "a1", Name: "one"},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if sawQuery != "" {
		t.Errorf("expected no query string (no context_type filter), got %q", sawQuery)
	}
	if !strings.Contains(out, `"id": "a1"`) {
		t.Errorf("expected app in output:\n%s", out)
	}

	// The context_type filter flag must be gone.
	listCmd, _, err := newAppsCmd().Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list: %v", err)
	}
	if f := listCmd.Flags().Lookup("context-type"); f != nil {
		t.Error("expected --context-type flag to be gone from apps list")
	}
}

func TestAppsDelete_SkipsConfirmationWithYes(t *testing.T) {
	var sawDelete bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/v1/platform/custom-apps/a1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		sawDelete = true
		w.WriteHeader(http.StatusNoContent)
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"delete", "a1", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !sawDelete {
		t.Error("expected DELETE request")
	}
	if !strings.Contains(out, `"status": "deleted"`) {
		t.Errorf("expected deleted status:\n%s", out)
	}
}
