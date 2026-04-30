package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestWorkspaceList(t *testing.T) {
	cleanup := setupTestEnv(t, "")
	defer cleanup()
	isolateConfig(t)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")

	receivedKey := ""
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" {
			http.NotFound(w, r)
			return
		}
		receivedKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id":            "00000000-0000-0000-0000-000000000123",
			"display_name":  "Default Workspace",
			"tenant_handle": "default",
			"role_name":     "Admin",
		}})
	})
	stdout, err := executeCommand(t,
		"--api-key", "test-api-key",
		"--api-url", ts.URL,
		"workspace", "list",
	)
	if err != nil {
		t.Fatalf("workspace list returned error: %v\nstdout: %s", err, stdout)
	}
	if receivedKey != "test-api-key" {
		t.Fatalf("expected API key header, got %q", receivedKey)
	}
	if !strings.Contains(stdout, "Default Workspace") {
		t.Fatalf("expected workspace in output, got %s", stdout)
	}
}

func TestWorkspaceSetDefault(t *testing.T) {
	cleanup := setupTestEnv(t, "")
	defer cleanup()
	isolateConfig(t)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")

	workspaceID := "00000000-0000-0000-0000-000000000789"
	stdout, err := executeCommand(t, "workspace", "set-default", workspaceID)
	if err != nil {
		t.Fatalf("workspace set-default returned error: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, workspaceID) {
		t.Fatalf("expected workspace ID in output, got %s", stdout)
	}
}
