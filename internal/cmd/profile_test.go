package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

func TestProfileSetWorkspace(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldFormat := flagOutputFormat
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagOutputFormat = oldFormat
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""
	flagOutputFormat = "json"

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_PROFILE", "")
	accessToken := "test-access-token"
	if err := os.WriteFile(configPath, []byte(`{
  "current_profile": "local",
  "profiles": {
    "local": {
      "oauth": {
        "access_token": "`+accessToken+`"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	workspaceID := "00000000-0000-0000-0000-000000000456"
	stdout, err := executeCommand(t, "profile", "set-workspace", workspaceID)
	if err != nil {
		t.Fatalf("set-workspace returned error: %v\nstdout: %s", err, stdout)
	}
	if strings.Contains(stdout, accessToken) {
		t.Fatalf("profile output exposed access token")
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout)
	}
	if result["profile"] != "local" || result["workspace_id"] != workspaceID {
		t.Fatalf("unexpected result: %+v", result)
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["local"]
	if profile.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace ID %q, got %q", workspaceID, profile.WorkspaceID)
	}
	if profile.OAuth.AccessToken != accessToken {
		t.Fatalf("access token was not preserved")
	}
}

func TestProfileSetWorkspaceInvalidID(t *testing.T) {
	oldProfile := flagProfile
	defer func() { flagProfile = oldProfile }()
	flagProfile = ""
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))

	_, err := executeCommand(t, "profile", "set-workspace", "not-a-uuid")
	if err == nil {
		t.Fatal("expected invalid workspace ID error")
	}
}
