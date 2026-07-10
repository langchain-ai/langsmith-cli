package cmdutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().String("api-key", "", "")
	root.PersistentFlags().String("api-url", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.PersistentFlags().String("workspace", "", "")
	root.PersistentFlags().String("workspace-id", "", "")
	root.PersistentFlags().String("format", "pretty", "")
	return root
}

func TestResolveAPIKey_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-key", "from-flag")
	if got := ResolveAPIKey(cmd); got != "from-flag" {
		t.Errorf("expected from-flag, got %q", got)
	}
}

func TestResolveAPIKey_Env(t *testing.T) {
	cmd := newTestCmd()
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	if got := ResolveAPIKey(cmd); got != "from-env" {
		t.Errorf("expected from-env, got %q", got)
	}
}

func TestResolveAPIKey_Empty(t *testing.T) {
	t.Setenv("LANGSMITH_API_KEY", "")
	cmd := newTestCmd()
	if got := ResolveAPIKey(cmd); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveAPIURL_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-url", "http://custom.example.com")
	if got := ResolveAPIURL(cmd); got != "http://custom.example.com" {
		t.Errorf("expected http://custom.example.com, got %q", got)
	}
}

func TestResolveAPIURL_Env(t *testing.T) {
	cmd := newTestCmd()
	t.Setenv("LANGSMITH_ENDPOINT", "http://env.example.com")
	if got := ResolveAPIURL(cmd); got != "http://env.example.com" {
		t.Errorf("expected http://env.example.com, got %q", got)
	}
}

func TestResolveAPIURL_Default(t *testing.T) {
	cmd := newTestCmd()
	if got := ResolveAPIURL(cmd); got != "https://api.smith.langchain.com" {
		t.Errorf("expected default, got %q", got)
	}
}

func TestResolveAPIURL_NormalizesTrailingAPIV1(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-url", "https://myhost.com/api/v1")
	if got := ResolveAPIURL(cmd); got != "https://myhost.com" {
		t.Errorf("expected normalized URL, got %q", got)
	}
}

func TestResolveFormat_Flag(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("format", "json")
	if got := ResolveFormat(cmd); got != "json" {
		t.Errorf("expected json, got %q", got)
	}
}

func TestResolveFormat_Default(t *testing.T) {
	cmd := newTestCmd()
	if got := ResolveFormat(cmd); got != "pretty" {
		t.Errorf("expected pretty, got %q", got)
	}
}

func TestResolveJQ_Flag(t *testing.T) {
	cmd := newTestCmd()
	cmd.PersistentFlags().String("jq", "", "")
	_ = cmd.PersistentFlags().Set("jq", ".name")
	require.Equal(t, ".name", ResolveJQ(cmd))
}

func TestGetClient_Success(t *testing.T) {
	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("api-key", "test-key")
	_ = cmd.PersistentFlags().Set("api-url", "http://localhost:1234")
	c, err := GetClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIKey() != "test-key" {
		t.Errorf("expected api key test-key, got %q", c.APIKey())
	}
	if c.APIURL() != "http://localhost:1234" {
		t.Errorf("expected api url http://localhost:1234, got %q", c.APIURL())
	}
}

func TestGetClient_MissingKey(t *testing.T) {
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.json"))
	cmd := newTestCmd()
	_, err := GetClient(cmd)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestGetClient_ProfileBearer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "local",
  "profiles": {
    "local": {
      "api_url": "http://localhost:1980",
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

	cmd := newTestCmd()
	c, err := GetClient(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.OAuthAccessToken() != "test-access-token" {
		t.Fatalf("expected OAuth access token from profile")
	}
	if c.APIURL() != "http://localhost:1980" {
		t.Fatalf("expected profile API URL, got %q", c.APIURL())
	}
}

func TestResolveClientOptions_ProfileFlagSetsProfileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "default",
  "profiles": {
    "default": {
      "api_key": "default-key"
    },
    "prod": {
      "api_url": "http://localhost:1980/api/v1",
      "workspace_id": "ws-prod",
      "oauth": {
        "access_token": "prod-access-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("profile", "prod")
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "prod" {
		t.Fatalf("expected profile name prod, got %q", opts.ProfileName)
	}
	if opts.OAuthAccessToken != "prod-access-token" {
		t.Fatalf("expected profile OAuth token, got %q", opts.OAuthAccessToken)
	}
	if opts.APIURL != "http://localhost:1980/api/v1" {
		t.Fatalf("expected profile API URL, got %q", opts.APIURL)
	}
	if opts.WorkspaceID != "ws-prod" {
		t.Fatalf("expected profile workspace ID, got %q", opts.WorkspaceID)
	}
}

func TestResolveClientOptions_APIKeyProfileSetsProfileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "prod",
  "profiles": {
    "prod": {
      "api_key": "prod-api-key",
      "workspace_id": "ws-prod"
    },
    "aws": {
      "api_key": "aws-api-key"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("profile", "aws")
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "aws-api-key" {
		t.Fatalf("expected profile api key, got %q", opts.APIKey)
	}
	// An api-key profile must route through WithProfile too, so it replaces
	// current_profile and clears the inherited tenant. This resolver (used by the
	// api/sandbox/ssh subcommands) previously omitted ProfileName here.
	if opts.ProfileName != "aws" {
		t.Fatalf("expected ProfileName=aws, got %q", opts.ProfileName)
	}
}

func TestResolveClientOptions_WorkspaceFlagOverridesEnvAndProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-env")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "default",
  "profiles": {
    "default": {
      "api_key": "default-key",
      "workspace_id": "ws-profile"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("workspace", "ws-flag")
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkspaceID != "ws-flag" {
		t.Fatalf("expected flag workspace ID, got %q", opts.WorkspaceID)
	}
}

func TestResolveClientOptions_WorkspaceIDAliasOverridesEnvAndProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "ws-env")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "default",
  "profiles": {
    "default": {
      "api_key": "default-key",
      "workspace_id": "ws-profile"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	_ = cmd.PersistentFlags().Set("workspace-id", "ws-alias")
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkspaceID != "ws-alias" {
		t.Fatalf("expected alias workspace ID, got %q", opts.WorkspaceID)
	}
}

func TestResolveClientOptions_EnvAPIKeyOverridesProfileBearer(t *testing.T) {
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

	cmd := newTestCmd()
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "from-env" {
		t.Fatalf("expected env API key, got %q", opts.APIKey)
	}
	if opts.OAuthAccessToken != "" {
		t.Fatalf("expected profile OAuth access token to be ignored")
	}
	if opts.ProfileName != "" {
		t.Fatalf("expected profile name to be ignored when API key auth wins, got %q", opts.ProfileName)
	}
}

func TestResolveClientOptions_ProfileFlagWarnsWhenEnvAPIKeyOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	err := os.WriteFile(path, []byte(`{
  "profiles": {
    "prod": {
      "oauth": {
        "access_token": "test-access-token"
      }
    }
  }
}
`), 0600)
	require.NoError(t, err)

	cmd := newTestCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	err = cmd.PersistentFlags().Set("profile", "prod")
	require.NoError(t, err)

	opts, err := ResolveClientOptions(cmd, false)
	require.NoError(t, err)
	require.Equal(t, "from-env", opts.APIKey)
	require.Empty(t, opts.OAuthAccessToken)
	require.Contains(t, stderr.String(), "warning: --profile was specified, but LANGSMITH_API_KEY is set")
}

func TestResolveClientOptionsRefreshesProfileWithoutAccessToken(t *testing.T) {
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
      "api_url": "`+ts.URL+`/api/v1",
      "oauth": {
        "refresh_token": "old-refresh-token"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	opts, err := ResolveClientOptions(cmd, true)
	if err != nil {
		t.Fatalf("ResolveClientOptions returned error: %v", err)
	}
	if opts.OAuthAccessToken != "new-access-token" {
		t.Fatalf("expected refreshed OAuth token, got %q", opts.OAuthAccessToken)
	}
}

func assertOAuthResource(t *testing.T, r *http.Request) {
	t.Helper()
	expected := "http://" + r.Host
	if r.TLS != nil {
		expected = "https://" + r.Host
	}
	if got := r.FormValue("resource"); got != expected {
		t.Fatalf("expected resource %q, got %q", expected, got)
	}
}

func TestResolveClientOptions_EnvAPIKeyWithMalformedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "from-env")
	t.Setenv("LANGSMITH_ENDPOINT", "https://env.example.com/api/v1")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "from-env" {
		t.Fatalf("expected env API key, got %q", opts.APIKey)
	}
	if opts.APIURL != "https://env.example.com" {
		t.Fatalf("expected normalized env URL, got %q", opts.APIURL)
	}
}

func TestResolveClientOptions_ProfileEnvTrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", " local ")
	if err := os.WriteFile(path, []byte(`{
  "profiles": {
    "local": {
      "api_key": "profile-api-key"
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newTestCmd()
	opts, err := ResolveClientOptions(cmd, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "profile-api-key" {
		t.Fatalf("expected profile API key, got %q", opts.APIKey)
	}
}
