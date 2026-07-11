package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAppsList_PassesContextTypeFilter(t *testing.T) {
	var sawQuery string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: "a1", Name: "one", ContextType: "annotation_queue"},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list", "--context-type", "annotation_queue"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if sawQuery != "context_type=annotation_queue" {
		t.Errorf("unexpected query: %q", sawQuery)
	}
	if !strings.Contains(out, `"id": "a1"`) {
		t.Errorf("expected app in output:\n%s", out)
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
