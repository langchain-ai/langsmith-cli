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

func TestProfileListDoesNotExposeSecrets(t *testing.T) {
	oldProfile := flagProfile
	oldFormat := flagOutputFormat
	defer func() {
		flagProfile = oldProfile
		flagOutputFormat = oldFormat
	}()
	flagProfile = ""
	flagOutputFormat = "json"

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_PROFILE", "")
	accessToken := "test-access-token"
	refreshToken := "test-refresh-token"
	apiKey := "test-api-key"
	if err := os.WriteFile(configPath, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "https://dev.api.smith.langchain.com",
      "workspace_id": "00000000-0000-0000-0000-000000000123",
      "oauth": {
        "access_token": "`+accessToken+`",
        "refresh_token": "`+refreshToken+`",
        "expires_at": "2026-04-30T00:00:00Z"
      }
    },
    "local": {
      "api_key": "`+apiKey+`",
      "api_url": "http://localhost:1980"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t, "profile", "list")
	if err != nil {
		t.Fatalf("profile list returned error: %v\nstdout: %s", err, stdout)
	}
	if strings.Contains(stdout, accessToken) || strings.Contains(stdout, refreshToken) || strings.Contains(stdout, apiKey) {
		t.Fatalf("profile list output exposed a secret: %s", stdout)
	}

	var result []profileListItem
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(result))
	}
	if result[0].Name != "dev" || !result[0].Active || result[0].Auth != "oauth" {
		t.Fatalf("unexpected active profile: %+v", result[0])
	}
	if result[1].Name != "local" || result[1].Auth != "api_key" {
		t.Fatalf("unexpected second profile: %+v", result[1])
	}
}
