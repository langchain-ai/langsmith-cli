package cmdutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	// "langsmith login" is not a command; the OAuth flow lives under "auth".
	require.Contains(t, err.Error(), "langsmith auth login")
}

func TestGetClient_ProfileBearer(t *testing.T) {
	var gotAuth, gotTenant string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTenant = r.Header.Get("X-Tenant-Id")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "local",
  "profiles": {
    "local": {
      "api_url": "`+ts.URL+`",
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

	c, err := GetClient(newTestCmd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.APIURL() != ts.URL {
		t.Fatalf("expected profile API URL, got %q", c.APIURL())
	}
	if err := c.RawGet(t.Context(), "/api/v1/ping", nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-access-token" || gotTenant != "ws-123" {
		t.Fatalf("expected profile bearer and tenant, got auth=%q tenant=%q", gotAuth, gotTenant)
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
	opts, err := ResolveClientOptions(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "prod" {
		t.Fatalf("expected profile name prod, got %q", opts.ProfileName)
	}
	if opts.OAuthAccessToken != "" {
		t.Fatalf("expected the SDK to own the profile token, got %q", opts.OAuthAccessToken)
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
	opts, err := ResolveClientOptions(cmd)
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
	opts, err := ResolveClientOptions(cmd)
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
	opts, err := ResolveClientOptions(cmd)
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
	opts, err := ResolveClientOptions(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "from-env" {
		t.Fatalf("expected env API key, got %q", opts.APIKey)
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

	opts, err := ResolveClientOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, "from-env", opts.APIKey)
	require.Empty(t, opts.OAuthAccessToken)
	require.Contains(t, stderr.String(), "warning: --profile was specified, but LANGSMITH_API_KEY is set")
}

func TestResolveClientOptionsSelectsOAuthProfileWithoutRefreshing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("resolving options unexpectedly contacted %s", r.URL.Path)
	}))
	defer ts.Close()
	writeExpiredOAuthProfile(t, ts.URL, "")

	opts, err := ResolveClientOptions(newTestCmd())
	if err != nil {
		t.Fatalf("ResolveClientOptions returned error: %v", err)
	}
	if opts.ProfileName != "dev" || !opts.HasAuth() {
		t.Fatalf("expected the OAuth profile to be selected, got %+v", opts)
	}
}

// Agents often start several CLI commands at once. Once the access token has
// expired they must share one refresh: the server revokes every session for the
// user when a rotated refresh token is replayed.
func TestConcurrentClientsShareOneRefresh(t *testing.T) {
	var tokenRequests atomic.Int32
	var mu sync.Mutex
	validRefresh := "old-refresh-token"
	var authServer *httptest.Server
	authServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                        authServer.URL,
				"device_authorization_endpoint": authServer.URL + "/oauth/device/code",
				"token_endpoint":                authServer.URL + "/oauth/token",
			})
		case "/oauth/token":
			tokenRequests.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if got := r.FormValue("resource"); got != authServer.URL {
				t.Errorf("expected resource %q, got %q", authServer.URL, got)
			}
			mu.Lock()
			defer mu.Unlock()
			if r.FormValue("refresh_token") != validRefresh {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token has been revoked"}`))
				return
			}
			validRefresh = "new-refresh-token"
			time.Sleep(20 * time.Millisecond)
			_, _ = w.Write([]byte(`{"access_token":"new-access-token","expires_in":300,"refresh_token":"new-refresh-token"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer authServer.Close()

	var dataPlaneRequests atomic.Int32
	dataPlane := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ping" {
			t.Errorf("data plane got unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		dataPlaneRequests.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer new-access-token" {
			t.Errorf("expected refreshed bearer, got %q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer dataPlane.Close()
	path := writeExpiredOAuthProfile(t, dataPlane.URL, authServer.URL)

	const commands = 6
	var wg sync.WaitGroup
	for range commands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := GetClient(newTestCmd())
			if err != nil {
				t.Error(err)
				return
			}
			if err := c.RawGet(t.Context(), "/api/v1/ping", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := tokenRequests.Load(); got != 1 {
		t.Fatalf("expected one refresh across %d commands, got %d", commands, got)
	}
	if got := dataPlaneRequests.Load(); got != commands {
		t.Fatalf("expected %d API requests, got %d", commands, got)
	}
	cfg, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"issuer": "` + authServer.URL + `"`, `"refresh_token": "new-refresh-token"`} {
		if !bytes.Contains(cfg, []byte(want)) {
			t.Fatalf("expected %s in saved config:\n%s", want, cfg)
		}
	}
}

func TestRejectedRefreshAsksForReauthentication(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"refresh token has been revoked"}`))
		case "/api/v1/ping":
			t.Error("request sent with an expired token")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	writeExpiredOAuthProfile(t, ts.URL, "")

	c, err := GetClient(newTestCmd())
	if err != nil {
		t.Fatal(err)
	}
	err = c.RawGet(t.Context(), "/api/v1/ping", nil)
	if err == nil || !strings.Contains(err.Error(), "langsmith auth login --profile dev") {
		t.Fatalf("expected reauthentication hint, got %v", err)
	}
}

func writeExpiredOAuthProfile(t *testing.T, apiURL, issuer string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	issuerField := ""
	if issuer != "" {
		issuerField = `"issuer": "` + issuer + `",`
	}
	if err := os.WriteFile(path, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "`+apiURL+`",
      "oauth": {
        `+issuerField+`
        "access_token": "old-access-token",
        "refresh_token": "old-refresh-token",
        "expires_at": "`+time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)+`"
      }
    }
  }
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
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
	opts, err := ResolveClientOptions(cmd)
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
	opts, err := ResolveClientOptions(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.APIKey != "profile-api-key" {
		t.Fatalf("expected profile API key, got %q", opts.APIKey)
	}
}

// writeEndpointConfig writes a config whose "dev" profile carries an api_url and
// whose "bare" profile omits one.
func writeEndpointConfig(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", path)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "")
	t.Setenv("LANGSMITH_TENANT_ID", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "https://dev.api.smith.langchain.com",
      "api_key": "dev-key"
    },
    "bare": {
      "api_key": "bare-key"
    }
  }
}
`), 0600))
}

func TestResolveClientOptions_ProfileFlagIgnoresEnvEndpoint(t *testing.T) {
	writeEndpointConfig(t)
	t.Setenv("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com")

	cmd := newTestCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.PersistentFlags().Set("profile", "dev"))

	opts, err := ResolveClientOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, "https://dev.api.smith.langchain.com", opts.APIURL)
	require.Contains(t, stderr.String(), `warning: ignoring LANGSMITH_ENDPOINT because profile "dev" was selected with --profile`)
}

func TestResolveClientOptions_ProfileFlagWithoutAPIURLHonorsEnvEndpoint(t *testing.T) {
	writeEndpointConfig(t)
	t.Setenv("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com")

	cmd := newTestCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.PersistentFlags().Set("profile", "bare"))

	opts, err := ResolveClientOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, "https://api.smith.langchain.com", opts.APIURL)
	require.Empty(t, stderr.String())
}

func TestResolveClientOptions_ImplicitProfileHonorsEnvEndpoint(t *testing.T) {
	writeEndpointConfig(t)
	t.Setenv("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com")

	cmd := newTestCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	opts, err := ResolveClientOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, "https://api.smith.langchain.com", opts.APIURL)
	require.Empty(t, stderr.String())
}

func TestResolveClientOptions_APIURLFlagBeatsProfile(t *testing.T) {
	writeEndpointConfig(t)
	t.Setenv("LANGSMITH_ENDPOINT", "https://api.smith.langchain.com")

	cmd := newTestCmd()
	require.NoError(t, cmd.PersistentFlags().Set("profile", "dev"))
	require.NoError(t, cmd.PersistentFlags().Set("api-url", "https://flag.example.com"))

	opts, err := ResolveClientOptions(cmd)
	require.NoError(t, err)
	require.Equal(t, "https://flag.example.com", opts.APIURL)
}
