package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempConfig writes a TOML config file in a temp dir and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return path
}

// ==================== profile create ====================

func TestProfileCreate_NewFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "create", "myprofile",
		"--api-key", "ls-testkey12345",
		"--api-url", "https://api.smith.langchain.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v (output: %s)", err, out)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if result["status"] != "created" {
		t.Errorf("expected status=created, got %q", result["status"])
	}
	if result["profile"] != "myprofile" {
		t.Errorf("expected profile=myprofile, got %q", result["profile"])
	}

	// Verify file was created
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestProfileCreate_AlreadyExists(t *testing.T) {
	cfgPath := writeTempConfig(t, `
[myprofile]
api_key = "ls-existing"
api_url = "https://api.smith.langchain.com"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "create", "myprofile",
		"--api-key", "ls-newkey",
		"--api-url", "https://api.smith.langchain.com",
	)
	if err == nil {
		t.Error("expected error when profile already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got %v", err)
	}
}

func TestProfileCreate_InvalidName(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "create", "my profile!",
		"--api-key", "ls-key",
		"--api-url", "https://api.smith.langchain.com",
	)
	if err == nil {
		t.Error("expected error for invalid profile name")
	}
}

func TestProfileCreate_FirstBecomesCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "create", "first",
		"--api-key", "ls-key12345",
		"--api-url", "https://api.smith.langchain.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read the config file to check current_profile was set
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	if !strings.Contains(string(data), `current_profile = "first"`) {
		t.Errorf("first profile should become current_profile, config:\n%s", data)
	}
}

// ==================== profile list ====================

func TestProfileList_Empty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d items", len(result))
	}
}

func TestProfileList_WithProfiles(t *testing.T) {
	cfgPath := writeTempConfig(t, `current_profile = "beta"

[alpha]
api_key = "ls-alpha"
api_url = "https://api.smith.langchain.com"

[beta]
api_key = "ls-beta"
api_url = "https://api.smith.langchain.com"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(result))
	}

	// Should be sorted by name: alpha first, beta second
	if result[0]["name"] != "alpha" {
		t.Errorf("expected first profile alpha, got %v", result[0]["name"])
	}
	if result[1]["name"] != "beta" {
		t.Errorf("expected second profile beta, got %v", result[1]["name"])
	}

	// beta should be marked active
	if result[0]["active"] != false {
		t.Errorf("expected alpha inactive, got %v", result[0]["active"])
	}
	if result[1]["active"] != true {
		t.Errorf("expected beta active, got %v", result[1]["active"])
	}
}

// ==================== profile show ====================

func TestProfileShow_Exists(t *testing.T) {
	cfgPath := writeTempConfig(t, `
[myprofile]
api_key = "ls-secretkey12345"
api_url = "https://api.smith.langchain.com"
workspace_id = "ws-abc"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "show", "myprofile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if result["name"] != "myprofile" {
		t.Errorf("expected name=myprofile, got %v", result["name"])
	}
	// Key should be masked
	apiKey, _ := result["api_key"].(string)
	if strings.Contains(apiKey, "secretkey") {
		t.Errorf("API key should be masked, got %q", apiKey)
	}
	if !strings.Contains(apiKey, "...") {
		t.Errorf("masked key should contain '...', got %q", apiKey)
	}
	if result["api_url"] != "https://api.smith.langchain.com" {
		t.Errorf("unexpected api_url: %v", result["api_url"])
	}
	if result["workspace_id"] != "ws-abc" {
		t.Errorf("expected workspace_id=ws-abc, got %v", result["workspace_id"])
	}
}

func TestProfileShow_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "show", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

// ==================== profile delete ====================

func TestProfileDelete_Exists(t *testing.T) {
	cfgPath := writeTempConfig(t, `
[myprofile]
api_key = "ls-key"
api_url = "https://api.smith.langchain.com"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "delete", "myprofile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if result["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %q", result["status"])
	}
	if result["profile"] != "myprofile" {
		t.Errorf("expected profile=myprofile, got %q", result["profile"])
	}
}

func TestProfileDelete_ClearsCurrentProfile(t *testing.T) {
	cfgPath := writeTempConfig(t, `current_profile = "active"

[active]
api_key = "ls-key"
api_url = "https://api.smith.langchain.com"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "delete", "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}
	if strings.Contains(string(data), "current_profile") {
		t.Errorf("current_profile should be cleared after deleting active profile, config:\n%s", data)
	}
}

func TestProfileDelete_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "delete", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}

// ==================== profile use ====================

func TestProfileUse_Exists(t *testing.T) {
	cfgPath := writeTempConfig(t, `
[myprofile]
api_key = "ls-key"
api_url = "https://api.smith.langchain.com"
`)
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	out, err := executeCommand(t, "profile", "use", "myprofile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &result); err != nil {
		t.Fatalf("parsing output JSON: %v (output: %s)", err, out)
	}
	if result["status"] != "switched" {
		t.Errorf("expected status=switched, got %q", result["status"])
	}
	if result["profile"] != "myprofile" {
		t.Errorf("expected profile=myprofile, got %q", result["profile"])
	}
}

func TestProfileUse_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfgPath)

	_, err := executeCommand(t, "profile", "use", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %v", err)
	}
}
