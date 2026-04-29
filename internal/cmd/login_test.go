package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

func TestLoginDeviceFlowSavesOAuthProfile(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldFormat := flagOutputFormat
	oldOpenBrowser := openBrowser
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagOutputFormat = oldFormat
		openBrowser = oldOpenBrowser
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""
	flagOutputFormat = "json"
	openBrowser = func(string) error { return nil }

	configPath := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")

	accessToken := "test-access-token"
	workspaceID := "00000000-0000-0000-0000-000000000123"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if got := r.FormValue("client_id"); got != oauthClientID {
				t.Fatalf("expected client_id %q, got %q", oauthClientID, got)
			}
			_ = json.NewEncoder(w).Encode(deviceCodeResponse{
				DeviceCode:      "device-code",
				UserCode:        "ABCD-EFGH",
				VerificationURI: tsActivateURL(r),
				ExpiresIn:       60,
				Interval:        0,
			})
		case "/oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if got := r.FormValue("grant_type"); got != "urn:ietf:params:oauth:grant-type:device_code" {
				t.Fatalf("unexpected grant_type %q", got)
			}
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{
				AccessToken:  accessToken,
				ExpiresIn:    300,
				RefreshToken: "test-refresh-token",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	root := NewRootCmd("test", "test")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"login", "--api-url", ts.URL, "--no-browser", "--workspace-id", workspaceID})

	if err := root.Execute(); err != nil {
		t.Fatalf("login returned error: %v\nstderr: %s", err, stderr.String())
	}

	if strings.Contains(stdout.String(), accessToken) || strings.Contains(stderr.String(), accessToken) {
		t.Fatalf("login output exposed access token")
	}
	if !strings.Contains(stderr.String(), "ABCD-EFGH") {
		t.Fatalf("expected login instructions to include user code, got %q", stderr.String())
	}

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if result["status"] != "logged_in" || result["profile"] != "default" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	if result["workspace_id"] != workspaceID {
		t.Fatalf("expected workspace_id %q in result, got %q", workspaceID, result["workspace_id"])
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["default"]
	if profile.OAuth.AccessToken != accessToken {
		t.Fatalf("access token was not saved")
	}
	if profile.OAuth.RefreshToken != "test-refresh-token" {
		t.Fatalf("refresh token was not saved")
	}
	if profile.WorkspaceID != workspaceID {
		t.Fatalf("workspace ID was not saved")
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected config permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestRefreshProfileToken(t *testing.T) {
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
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken:  "new-access-token",
			ExpiresIn:    300,
			RefreshToken: "new-refresh-token",
		})
	}))
	defer ts.Close()

	token, err := refreshProfileToken(t.Context(), ts.URL, "old-refresh-token")
	if err != nil {
		t.Fatalf("refreshProfileToken returned error: %v", err)
	}
	if token.AccessToken != "new-access-token" || token.RefreshToken != "new-refresh-token" {
		t.Fatalf("unexpected token response: %+v", token)
	}
}

func tsActivateURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/activate"
}
