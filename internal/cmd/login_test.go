package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

func TestLoginDoesNotDefineLocalWorkspaceIDFlag(t *testing.T) {
	cmd := newLoginCmd()
	if f := cmd.Flags().Lookup("workspace-id"); f != nil {
		t.Fatal("auth login should use the inherited workspace flag, not a local --workspace-id flag")
	}
}

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

	configPath := filepath.Join(t.TempDir(), "config.json")
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
			assertOAuthResource(t, r)
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
			assertOAuthResource(t, r)
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
	root.SetArgs([]string{"--format=json", "auth", "login", "--api-url", ts.URL + "/api/v1", "--no-browser", "--workspace-id", workspaceID})

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
	if result["api_url"] != ts.URL {
		t.Fatalf("expected normalized api_url %q, got %q", ts.URL, result["api_url"])
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("expected config permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestLoginDoesNotSaveTokenWorkspaceByDefault(t *testing.T) {
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

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")

	accessToken := "test-access-token"
	tokenWorkspaceID := "00000000-0000-0000-0000-000000000456"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			assertOAuthResource(t, r)
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
			assertOAuthResource(t, r)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  accessToken,
				"expires_in":    300,
				"refresh_token": "test-refresh-token",
				"workspace_id":  tokenWorkspaceID,
			})
		case "/api/v1/workspaces":
			t.Fatal("did not expect workspace list request")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	root := NewRootCmd("test", "test")
	root.SetIn(strings.NewReader(""))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--format=json", "auth", "login", "--api-url", ts.URL, "--no-browser"})

	if err := root.Execute(); err != nil {
		t.Fatalf("login returned error: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), accessToken) || strings.Contains(stderr.String(), accessToken) {
		t.Fatalf("login output exposed access token")
	}

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if result["workspace_id"] != "" {
		t.Fatalf("expected workspace_id to be omitted, got %q", result["workspace_id"])
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["default"]
	if profile.OAuth.AccessToken != accessToken {
		t.Fatalf("access token was not saved")
	}
	if profile.WorkspaceID != "" {
		t.Fatalf("expected token workspace ID not to be saved, got %q", profile.WorkspaceID)
	}
}

func TestLoginPromptsWorkspaceSelectionWhenRequested(t *testing.T) {
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldFormat := flagOutputFormat
	oldOpenBrowser := openBrowser
	oldInputIsTerminal := inputIsTerminal
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagOutputFormat = oldFormat
		openBrowser = oldOpenBrowser
		inputIsTerminal = oldInputIsTerminal
	}()
	flagAPIKey = ""
	flagAPIURL = ""
	flagProfile = ""
	flagOutputFormat = "json"
	openBrowser = func(string) error { return nil }
	inputIsTerminal = func(io.Reader) bool { return true }

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")

	accessToken := "test-access-token"
	secondWorkspaceID := "00000000-0000-0000-0000-000000000222"
	receivedWorkspaceAuth := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			assertOAuthResource(t, r)
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
			assertOAuthResource(t, r)
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{
				AccessToken:  accessToken,
				ExpiresIn:    300,
				RefreshToken: "test-refresh-token",
			})
		case "/api/v1/workspaces":
			receivedWorkspaceAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":           "00000000-0000-0000-0000-000000000111",
					"display_name": "First Workspace",
				},
				{
					"id":           secondWorkspaceID,
					"display_name": "Second Workspace",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	root := NewRootCmd("test", "test")
	root.SetIn(strings.NewReader("2\n"))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--format=json", "auth", "login", "--api-url", ts.URL, "--no-browser", "--prompt-workspace"})

	if err := root.Execute(); err != nil {
		t.Fatalf("login returned error: %v\nstderr: %s", err, stderr.String())
	}
	if receivedWorkspaceAuth != "Bearer "+accessToken {
		t.Fatalf("expected workspace list to use OAuth bearer token, got %q", receivedWorkspaceAuth)
	}

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if result["workspace_id"] != secondWorkspaceID {
		t.Fatalf("expected selected workspace ID %q, got %q", secondWorkspaceID, result["workspace_id"])
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Profiles["default"].WorkspaceID; got != secondWorkspaceID {
		t.Fatalf("expected saved workspace ID %q, got %q", secondWorkspaceID, got)
	}
}

func TestLoginSkipsWorkspaceSelectionByDefault(t *testing.T) {
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

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")

	workspaceListCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			assertOAuthResource(t, r)
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
			assertOAuthResource(t, r)
			_ = json.NewEncoder(w).Encode(oauthTokenResponse{
				AccessToken:  "test-access-token",
				ExpiresIn:    300,
				RefreshToken: "test-refresh-token",
			})
		case "/api/v1/workspaces":
			workspaceListCalled = true
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	root := NewRootCmd("test", "test")
	root.SetIn(strings.NewReader(""))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--format=json", "auth", "login", "--api-url", ts.URL, "--no-browser"})

	if err := root.Execute(); err != nil {
		t.Fatalf("login returned error: %v\nstderr: %s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "workspace") {
		t.Fatalf("did not expect workspace prompt or warning, got %q", stderr.String())
	}
	if workspaceListCalled {
		t.Fatal("did not expect workspace list request for non-interactive login")
	}

	var result map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout was not JSON: %v\n%s", err, stdout.String())
	}
	if result["workspace_id"] != "" {
		t.Fatalf("expected workspace_id to be omitted, got %q", result["workspace_id"])
	}

	cfg, err := lsconfig.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.Profiles["default"]
	if profile.OAuth.AccessToken != "test-access-token" {
		t.Fatal("access token was not saved")
	}
	if profile.WorkspaceID != "" {
		t.Fatalf("expected saved profile to omit workspace ID, got %q", profile.WorkspaceID)
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
		assertOAuthResource(t, r)
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			AccessToken:  "new-access-token",
			ExpiresIn:    300,
			RefreshToken: "new-refresh-token",
		})
	}))
	defer ts.Close()

	token, err := refreshProfileToken(t.Context(), ts.URL+"/api/v1", "old-refresh-token")
	if err != nil {
		t.Fatalf("refreshProfileToken returned error: %v", err)
	}
	if token.AccessToken != "new-access-token" || token.RefreshToken != "new-refresh-token" {
		t.Fatalf("unexpected token response: %+v", token)
	}
}

func TestRefreshProfileTokenRequiresAccessToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(oauthTokenResponse{
			ExpiresIn:    300,
			RefreshToken: "new-refresh-token",
		})
	}))
	defer ts.Close()

	_, err := refreshProfileToken(t.Context(), ts.URL, "old-refresh-token")
	if err == nil || !strings.Contains(err.Error(), "access token") {
		t.Fatalf("expected missing access token error, got %v", err)
	}
}

func TestNormalizeDeviceCodePollInterval(t *testing.T) {
	if got := normalizeDeviceCodePollInterval(0); got != defaultDeviceCodePollInterval {
		t.Fatalf("expected default interval, got %s", got)
	}
	if got := normalizeDeviceCodePollInterval(2 * time.Second); got != defaultDeviceCodePollInterval {
		t.Fatalf("expected default interval for low value, got %s", got)
	}
	if got := normalizeDeviceCodePollInterval(7 * time.Second); got != 7*time.Second {
		t.Fatalf("expected explicit interval, got %s", got)
	}
}

func tsActivateURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/activate"
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
