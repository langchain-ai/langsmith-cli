package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewaySetupPinsAPISelection(t *testing.T) {
	for _, selection := range []string{"default", "profile", "environment", "flag", "issuer"} {
		t.Run(selection, func(t *testing.T) {
			settings, cfg := gatewayTestEnv(t)
			profile := map[string]any{"oauth": map[string]string{"refresh_token": "synthetic-refresh-secret"}}
			want := lsconfig.DefaultAPIURL
			args := []string{"--dry-run", "--gateway-url=https://gateway.example"}
			if selection != "default" {
				want = "https://profile.example"
				profile["api_url"] = want
			}
			if selection == "environment" || selection == "flag" {
				want = "https://environment.example"
				t.Setenv("LANGSMITH_ENDPOINT", want)
			}
			if selection == "flag" {
				want = "https://flag.example"
				args = append(args, "--api-url="+want)
			}
			wantAuthority := want
			if selection == "issuer" {
				wantAuthority = "https://issuer.example"
				profile["oauth"].(map[string]string)["issuer"] = wantAuthority
			}
			data, err := json.Marshal(map[string]any{"profiles": map[string]any{"default": profile}})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cfg, data, 0o600))
			// Saved Claude environment is not part of the reviewed API selection.
			gatewaySaveSettings(t, settings, `{"env":{"LANGSMITH_ENDPOINT":"https://unreviewed.example"}}`)
			out, err := gatewayRun(t, args...)
			require.NoError(t, err)
			var report map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &report))
			require.Equal(t, want, report["api_url"])
			require.Equal(t, wantAuthority, report["oauth_refresh_authority"])
			require.Contains(t, report["consent"], wantAuthority)
			exe, err := os.Executable()
			require.NoError(t, err)
			require.Equal(t, gatewayHelper(exe, "default", want), report["apiKeyHelper"])
			require.NotContains(t, out, "unreviewed.example")
		})
	}
}

// Exercise the installed command and its generated shell helper, not just the
// command string: auth token must refresh at the pinned API despite Claude's env.
func TestGatewaySetupHelperRefreshIgnoresClaudeEndpoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell setup only")
	}
	binary := filepath.Join(t.TempDir(), "langsmith")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/langsmith")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)

	for _, scope := range []string{"user", "shared project", "local project", "launch environment"} {
		t.Run(scope, func(t *testing.T) {
			settings, cfg := gatewayTestEnv(t)
			var trustedRequests, untrustedRequests atomic.Int32
			refreshTokens := make(chan string, 1)
			trusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trustedRequests.Add(1)
				if r.URL.Path != "/oauth/token" {
					http.NotFound(w, r)
					return
				}
				if err := r.ParseForm(); err != nil || r.Method != http.MethodPost || r.Form.Get("grant_type") != "refresh_token" {
					http.Error(w, "invalid refresh request", http.StatusBadRequest)
					return
				}
				refreshTokens <- r.Form.Get("refresh_token")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"refreshed-synthetic-access","refresh_token":"rotated-synthetic-refresh","expires_in":3600,"token_type":"Bearer"}`))
			}))
			defer trusted.Close()
			untrusted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				untrustedRequests.Add(1)
				http.Error(w, "unexpected request", http.StatusInternalServerError)
			}))
			defer untrusted.Close()
			// Deliberately omit issuer to exercise legacy API-based refresh.
			profile := map[string]any{"profiles": map[string]any{"dev": map[string]any{
				"api_url": trusted.URL,
				"oauth":   map[string]string{"refresh_token": "synthetic-refresh-secret"},
			}}}
			data, err := json.Marshal(profile)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cfg, data, 0o600))
			if scope != "launch environment" {
				other := settings
				if scope == "shared project" {
					other = filepath.Join(".claude", "settings.json")
				} else if scope == "local project" {
					other = filepath.Join(".claude", "settings.local.json")
				}
				data, err = json.Marshal(map[string]any{"env": map[string]string{"LANGSMITH_ENDPOINT": untrusted.URL}})
				require.NoError(t, err)
				gatewaySaveSettings(t, other, string(data))
			}
			setup := exec.Command(binary, "--format=json", "--profile=dev", "gateway", "setup", "claude-code", "--gateway-url="+trusted.URL, "--yes")
			output, err := setup.CombinedOutput()
			require.NoError(t, err, "%s", output)
			require.Zero(t, trustedRequests.Load(), "setup must not refresh or discover")
			require.Zero(t, untrustedRequests.Load())

			doc := readJSONFile(t, settings)
			// Emulate Claude's effective environment after settings merge. The
			// endpoint override is identical regardless of the supplying scope.
			for key, value := range doc["env"].(map[string]any) {
				t.Setenv(key, value.(string))
			}
			t.Setenv("LANGSMITH_ENDPOINT", untrusted.URL)
			helper := exec.Command("/bin/sh", "-c", doc["apiKeyHelper"].(string))
			output, err = helper.CombinedOutput()
			require.NoError(t, err, "%s", output)
			require.Equal(t, "refreshed-synthetic-access", strings.TrimSpace(string(output)))
			require.Zero(t, untrustedRequests.Load(), "Claude's endpoint must never receive discovery or refresh requests")
			select {
			case token := <-refreshTokens:
				require.Equal(t, "synthetic-refresh-secret", token)
			default:
				t.Fatal("pinned API did not receive the refresh token")
			}
			saved, err := os.ReadFile(cfg)
			require.NoError(t, err)
			require.Contains(t, string(saved), "rotated-synthetic-refresh")
		})
	}
}
