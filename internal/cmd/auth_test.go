package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthCmdHasSubcommands(t *testing.T) {
	cmd := authCommand.Cobra()
	names := make(map[string]bool, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		names[child.Name()] = true
	}
	for _, expected := range []string{"login", "info", "token"} {
		if !names[expected] {
			t.Fatalf("auth command missing subcommand %q", expected)
		}
	}
}

func TestAuthInfoShowsCurrentOAuthProfile(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "https://profile.example.com/api/v1",
      "workspace_id": "00000000-0000-0000-0000-000000000123",
      "oauth": {
        "access_token": "test-access-token",
        "refresh_token": "test-refresh-token",
        "expires_at": "2999-01-01T00:00:00Z"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t, "--format=json", "auth", "info")
	if err != nil {
		t.Fatalf("auth info returned error: %v", err)
	}
	if strings.Contains(stdout, "test-access-token") || strings.Contains(stdout, "test-refresh-token") {
		t.Fatalf("auth info exposed OAuth token: %s", stdout)
	}

	var result authInfoResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout)
	}
	if !result.Authenticated || result.Auth != "oauth" || result.AuthSource != "profile" {
		t.Fatalf("unexpected auth info: %+v", result)
	}
	if result.Profile != "dev" || result.APIURL != "https://profile.example.com" {
		t.Fatalf("unexpected profile info: %+v", result)
	}
	if !result.OAuthAccessToken || !result.OAuthRefreshToken || result.OAuthExpired == nil || *result.OAuthExpired {
		t.Fatalf("unexpected OAuth token info: %+v", result)
	}
}

func TestAuthTokenPrintsSavedOAuthToken(t *testing.T) {
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
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "oauth": {
        "access_token": "saved-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t, "auth", "token")
	if err != nil {
		t.Fatalf("auth token returned error: %v", err)
	}
	if stdout != "saved-access-token\n" {
		t.Fatalf("expected saved token, got %q", stdout)
	}
}

func TestAuthTokenRefreshesExpiredOAuthToken(t *testing.T) {
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
		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Fatalf("unexpected grant_type %q", got)
		}
		if got := r.FormValue("refresh_token"); got != "old-refresh-token" {
			t.Fatalf("unexpected refresh token %q", got)
		}
		assertOAuthResource(t, r)
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
        "access_token": "expired-access-token",
        "refresh_token": "old-refresh-token",
        "expires_at": "2000-01-01T00:00:00Z"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t, "auth", "token")
	if err != nil {
		t.Fatalf("auth token returned error: %v", err)
	}
	if stdout != "new-access-token\n" {
		t.Fatalf("expected refreshed token, got %q", stdout)
	}

	cfg, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "new-refresh-token") {
		t.Fatalf("expected refreshed token to be saved, got %s", string(cfg))
	}
}

func TestAuthInfoShowsEnvAPIKeyPrecedence(t *testing.T) {
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

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "env-api-key-secret")
	t.Setenv("LANGSMITH_ENDPOINT", "https://env.example.com/api/v1")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "oauth": {
        "access_token": "test-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t, "--format=json", "auth", "info")
	if err != nil {
		t.Fatalf("auth info returned error: %v", err)
	}
	if strings.Contains(stdout, "env-api-key-secret") || strings.Contains(stdout, "test-access-token") {
		t.Fatalf("auth info exposed secret: %s", stdout)
	}

	var result authInfoResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout)
	}
	if !result.Authenticated || result.Auth != "api_key" || result.AuthSource != "env" {
		t.Fatalf("unexpected auth info: %+v", result)
	}
	if result.AuthNote != "LANGSMITH_API_KEY is set and takes precedence over saved profile auth." {
		t.Fatalf("expected env precedence note, got %q", result.AuthNote)
	}
	if result.APIURL != "https://env.example.com" || result.APIKey != "env-api-...cret" {
		t.Fatalf("unexpected env auth info: %+v", result)
	}
}

func TestAuthTokenRequiresOAuthToken(t *testing.T) {
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

	isolateConfig(t)
	t.Setenv("LANGSMITH_API_KEY", "from-env")

	_, err := executeCommand(t, "auth", "token")
	if err == nil || !strings.Contains(err.Error(), "no OAuth token") {
		t.Fatalf("expected missing OAuth token error, got %v", err)
	}
}
