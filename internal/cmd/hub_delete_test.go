package cmd

import (
	"net/http"
	"strings"
	"testing"
)

func TestHubDelete_WithYesFlag(t *testing.T) {
	called := false
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "DELETE" {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/v1/platform/hub/repos/acme/my-skill/directories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"delete", "acme/my-skill", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !called {
		t.Error("DELETE was not called")
	}
	if !strings.Contains(out, `"status": "deleted"`) {
		t.Errorf("missing status in output:\n%s", out)
	}
}
