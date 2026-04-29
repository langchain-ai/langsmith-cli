package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.toml"))
}

// ---------- Command tree structure ----------

func TestRootCmd_HasAllSubcommands(t *testing.T) {
	root := NewRootCmd("1.0.0", "1.0.0")
	expected := []string{"project", "trace", "run", "thread", "dataset", "example", "evaluator", "experiment", "login", "profile", "workspace", "self-update", "api"}
	cmds := root.Commands()

	names := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		names[c.Name()] = true
	}
	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("root command missing subcommand %q", exp)
		}
	}
}

func TestRootCmd_Version(t *testing.T) {
	root := NewRootCmd("2.3.4", "2.3.4 (commit: abc, built: now)")
	if root.Version != "2.3.4 (commit: abc, built: now)" {
		t.Errorf("expected display version, got %s", root.Version)
	}
}

func TestRootCmd_UseField(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	if root.Use != "langsmith" {
		t.Errorf("expected Use=langsmith, got %q", root.Use)
	}
}

func TestRootCmd_SilenceFlags(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	if !root.SilenceUsage {
		t.Error("expected SilenceUsage=true")
	}
	if !root.SilenceErrors {
		t.Error("expected SilenceErrors=true")
	}
}

// ---------- Global persistent flags ----------

func TestRootCmd_PersistentFlags_APIKey(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("api-key")
	if f == nil {
		t.Fatal("--api-key flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

func TestRootCmd_PersistentFlags_APIURL(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("api-url")
	if f == nil {
		t.Fatal("--api-url flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

func TestRootCmd_PersistentFlags_Format(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("format")
	if f == nil {
		t.Fatal("--format flag not found")
	}
	if f.DefValue != "json" {
		t.Errorf("expected default json, got %q", f.DefValue)
	}
}

func TestRootCmd_PersistentFlags_Profile(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("profile")
	if f == nil {
		t.Fatal("--profile flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

// ---------- getAPIKey ----------

func TestGetAPIKey_FlagPrecedence(t *testing.T) {
	isolateConfig(t)
	old := flagAPIKey
	defer func() { flagAPIKey = old }()

	flagAPIKey = "from-flag"
	t.Setenv("LANGSMITH_API_KEY", "from-env")

	if got := GetAPIKey(); got != "from-flag" {
		t.Errorf("expected from-flag, got %q", got)
	}
}

func TestGetAPIKey_EnvFallback(t *testing.T) {
	isolateConfig(t)
	old := flagAPIKey
	defer func() { flagAPIKey = old }()

	flagAPIKey = ""
	t.Setenv("LANGSMITH_API_KEY", "from-env")

	if got := GetAPIKey(); got != "from-env" {
		t.Errorf("expected from-env, got %q", got)
	}
}

func TestGetAPIKey_Empty(t *testing.T) {
	isolateConfig(t)
	old := flagAPIKey
	defer func() { flagAPIKey = old }()

	flagAPIKey = ""
	os.Unsetenv("LANGSMITH_API_KEY")

	if got := GetAPIKey(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// ---------- getAPIURL ----------

func TestGetAPIURL_FlagPrecedence(t *testing.T) {
	isolateConfig(t)
	old := flagAPIURL
	defer func() { flagAPIURL = old }()

	flagAPIURL = "http://custom.example.com"
	t.Setenv("LANGSMITH_ENDPOINT", "http://env.example.com")

	if got := GetAPIURL(); got != "http://custom.example.com" {
		t.Errorf("expected http://custom.example.com, got %q", got)
	}
}

func TestGetAPIURL_EnvFallback(t *testing.T) {
	isolateConfig(t)
	old := flagAPIURL
	defer func() { flagAPIURL = old }()

	flagAPIURL = ""
	t.Setenv("LANGSMITH_ENDPOINT", "http://env.example.com")

	if got := GetAPIURL(); got != "http://env.example.com" {
		t.Errorf("expected http://env.example.com, got %q", got)
	}
}

func TestGetAPIURL_DefaultValue(t *testing.T) {
	isolateConfig(t)
	old := flagAPIURL
	defer func() { flagAPIURL = old }()

	flagAPIURL = ""
	os.Unsetenv("LANGSMITH_ENDPOINT")

	if got := GetAPIURL(); got != "https://api.smith.langchain.com" {
		t.Errorf("expected default URL, got %q", got)
	}
}

func TestGetOAuthAccessToken_ProfileFallback(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte("current_profile = \"prod\"\n\n[prod]\napi_url = \"https://profile.example.com\"\nworkspace_id = \"ws-123\"\n\n[prod.oauth]\naccess_token = \"test-access-token\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := GetOAuthAccessToken(); got != "test-access-token" {
		t.Fatalf("expected profile OAuth access token, got %q", got)
	}
	if got := GetAPIURL(); got != "https://profile.example.com" {
		t.Fatalf("expected profile API URL, got %q", got)
	}
	if got := GetWorkspaceID(); got != "ws-123" {
		t.Fatalf("expected profile workspace, got %q", got)
	}
}

func TestGetAPIKey_EnvOverridesProfileBearer(t *testing.T) {
	oldKey := flagAPIKey
	oldProfile := flagProfile
	defer func() {
		flagAPIKey = oldKey
		flagProfile = oldProfile
	}()
	flagAPIKey = ""
	flagProfile = ""

	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	if err := os.WriteFile(path, []byte("[default]\n\n[default.oauth]\naccess_token = \"test-access-token\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if got := GetAPIKey(); got != "from-env" {
		t.Fatalf("expected env API key, got %q", got)
	}
	if got := GetOAuthAccessToken(); got != "" {
		t.Fatalf("expected profile OAuth access token to be ignored, got %q", got)
	}
}

// ---------- getFormat ----------

func TestGetFormat_ReturnsValue(t *testing.T) {
	old := flagOutputFormat
	defer func() { flagOutputFormat = old }()

	flagOutputFormat = "pretty"
	if got := GetFormat(); got != "pretty" {
		t.Errorf("expected pretty, got %q", got)
	}

	flagOutputFormat = "json"
	if got := GetFormat(); got != "json" {
		t.Errorf("expected json, got %q", got)
	}
}

// ---------- Unknown subcommand ----------

func TestRootCmd_UnknownSubcommand(t *testing.T) {
	_, err := executeCommand(t, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}
