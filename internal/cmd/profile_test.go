package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

func TestProfileCreate(t *testing.T) {
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
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")

	apiKey := "test-api-key"
	workspaceID := "00000000-0000-0000-0000-000000000123"
	stdout, err := executeCommand(t,
		"profile", "create", "dev",
		"--api-key", apiKey,
		"--api-url", "https://api.smith.langchain.com/api/v1",
		"--workspace-id", workspaceID,
	)
	if err != nil {
		t.Fatalf("profile create returned error: %v\nstdout: %s", err, stdout)
	}
	if strings.Contains(stdout, apiKey) {
		t.Fatalf("profile create output exposed api key: %s", stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "created" || result["profile"] != "dev" || result["auth"] != "api_key" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result["api_url"] != "https://api.smith.langchain.com" {
		t.Fatalf("expected normalized api_url, got %+v", result["api_url"])
	}
	if result["workspace_id"] != workspaceID {
		t.Fatalf("expected workspace_id %q, got %+v", workspaceID, result["workspace_id"])
	}
	if result["active"] != true {
		t.Fatalf("expected first profile to be active, got %+v", result["active"])
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentProfile != "dev" {
		t.Fatalf("expected dev current profile, got %q", cfg.CurrentProfile)
	}
	profile := cfg.Profiles["dev"]
	if profile.APIKey != apiKey {
		t.Fatalf("api key was not saved")
	}
	if profile.APIURL != "https://api.smith.langchain.com" {
		t.Fatalf("expected normalized profile api url, got %q", profile.APIURL)
	}
	if profile.WorkspaceID != workspaceID {
		t.Fatalf("expected workspace ID %q, got %q", workspaceID, profile.WorkspaceID)
	}
}

func TestProfileCreateUsesEnvAPIKeyAndEndpoint(t *testing.T) {
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
	t.Setenv("LANGSMITH_API_KEY", "env-api-key")
	t.Setenv("LANGSMITH_ENDPOINT", "http://localhost:1980/api/v1")
	t.Setenv("LANGSMITH_PROFILE", "")

	stdout, err := executeCommand(t, "profile", "create", "local")
	if err != nil {
		t.Fatalf("profile create returned error: %v\nstdout: %s", err, stdout)
	}
	if strings.Contains(stdout, "env-api-key") {
		t.Fatalf("profile create output exposed api key: %s", stdout)
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["local"]
	if profile.APIKey != "env-api-key" {
		t.Fatalf("expected api key from env")
	}
	if profile.APIURL != "http://localhost:1980" {
		t.Fatalf("expected normalized env endpoint, got %q", profile.APIURL)
	}
}

func TestProfileCreateRequiresAPIKey(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""

	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")

	_, err := executeCommand(t, "profile", "create", "dev")
	if err == nil {
		t.Fatal("expected missing api key error")
	}
	if !strings.Contains(err.Error(), "api key required") {
		t.Fatalf("expected api key required error, got %v", err)
	}
}

func TestProfileCreateAlreadyExists(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(configPath, []byte(`{
  "profiles": {
    "dev": {
      "api_key": "existing-api-key",
      "api_url": "https://api.smith.langchain.com"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := executeCommand(t, "profile", "create", "dev", "--api-key", "new-api-key")
	if err == nil {
		t.Fatal("expected duplicate profile error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles["dev"].APIKey != "existing-api-key" {
		t.Fatal("existing profile was overwritten")
	}
}

func TestProfileCreateInvalidWorkspaceID(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""

	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")

	_, err := executeCommand(t, "profile", "create", "dev", "--api-key", "test-api-key", "--workspace-id", "not-a-uuid")
	if err == nil {
		t.Fatal("expected invalid workspace ID error")
	}
}

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
