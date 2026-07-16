package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.json"))
}

// ---------- Command tree structure ----------

func TestRootCmd_HasAllSubcommands(t *testing.T) {
	root := NewRootCmd("1.0.0", "1.0.0")
	expected := []string{"project", "trace", "run", "thread", "dataset", "example", "evaluator", "experiment", "sandbox", "auth", "profile", "workspace", "self-update", "api"}
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
	if f.DefValue != "pretty" {
		t.Errorf("expected default pretty, got %q", f.DefValue)
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

func TestRootCmd_PersistentFlags_Workspace(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("workspace")
	if f == nil {
		t.Fatal("--workspace flag not found")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

func TestRootCmd_PersistentFlags_WorkspaceIDAlias(t *testing.T) {
	root := NewRootCmd("dev", "dev")
	f := root.PersistentFlags().Lookup("workspace-id")
	if f == nil {
		t.Fatal("--workspace-id flag not found")
	}
	if !f.Hidden {
		t.Fatal("expected --workspace-id alias to be hidden")
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

func TestGetAPIKey_EnvFallbackWithMalformedConfig(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	t.Setenv("LANGSMITH_ENDPOINT", "https://env.example.com/api/v1")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}

	if got := GetAPIKey(); got != "from-env" {
		t.Fatalf("expected env API key despite malformed config, got %q", got)
	}
	if got := GetAPIURL(); got != "https://env.example.com" {
		t.Fatalf("expected normalized env URL despite malformed config, got %q", got)
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

func TestGetAPIURL_NormalizesOverrides(t *testing.T) {
	old := flagAPIURL
	defer func() { flagAPIURL = old }()
	flagAPIURL = ""
	isolateConfig(t)

	t.Setenv("LANGSMITH_ENDPOINT", "https://env.example.com/api/v1")
	if got := GetAPIURL(); got != "https://env.example.com" {
		t.Fatalf("expected normalized env URL, got %q", got)
	}

	flagAPIURL = "https://flag.example.com/api/v1"
	if got := GetAPIURL(); got != "https://flag.example.com" {
		t.Fatalf("expected normalized flag URL, got %q", got)
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

func TestResolveClientOptionsRefreshesProfileWithoutAccessToken(t *testing.T) {
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

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("refresh_token"); got != "old-refresh-token" {
			t.Fatalf("unexpected refresh token %q", got)
		}
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken:  "new-access-token",
			ExpiresIn:    300,
			RefreshToken: "new-refresh-token",
		})
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "`+ts.URL+`",
      "oauth": {
        "refresh_token": "old-refresh-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	opts, err := resolveClientOptions(true)
	if err != nil {
		t.Fatalf("resolveClientOptions returned error: %v", err)
	}
	if opts.OAuthAccessToken != "new-access-token" {
		t.Fatalf("expected refreshed OAuth token, got %q", opts.OAuthAccessToken)
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "prod",
  "profiles": {
    "prod": {
      "api_url": "https://profile.example.com",
      "workspace_id": "ws-123",
      "oauth": {
        "access_token": "test-access-token"
      }
    }
  }
}
`), 0600); err != nil {
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

func TestGetWorkspaceID_FlagOverridesEnv(t *testing.T) {
	isolateConfig(t)
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-env")

	oldWorkspace := flagWorkspaceID
	defer func() {
		flagWorkspaceID = oldWorkspace
	}()

	root := NewRootCmd("dev", "dev")
	if err := root.PersistentFlags().Set("workspace", "ws-flag"); err != nil {
		t.Fatalf("setting workspace flag: %v", err)
	}
	if got := GetWorkspaceID(); got != "ws-flag" {
		t.Fatalf("expected flag workspace, got %q", got)
	}
}

func TestGetWorkspaceID_WorkspaceIDAliasOverridesEnv(t *testing.T) {
	isolateConfig(t)
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-env")

	oldWorkspace := flagWorkspaceID
	defer func() {
		flagWorkspaceID = oldWorkspace
	}()

	root := NewRootCmd("dev", "dev")
	if err := root.PersistentFlags().Set("workspace-id", "ws-alias"); err != nil {
		t.Fatalf("setting workspace-id flag: %v", err)
	}
	if got := GetWorkspaceID(); got != "ws-alias" {
		t.Fatalf("expected alias workspace, got %q", got)
	}
}

func TestGetAPIKey_ProfileEnvTrimsWhitespace(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", " prod ")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "prod": {
      "api_key": "profile-api-key"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	if got := GetAPIKey(); got != "profile-api-key" {
		t.Fatalf("expected profile API key from trimmed env profile, got %q", got)
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "default": {
      "oauth": {
        "access_token": "test-access-token"
      }
    }
  }
}
`), 0600); err != nil {
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
	oldFormat := flagOutputFormat
	defer func() {
		flagOutputFormat = oldFormat
	}()

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
