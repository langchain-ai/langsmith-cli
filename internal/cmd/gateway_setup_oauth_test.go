package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

const gatewayTestOAuthMarker = "X-LangSmith-Auth-Mode: oauth"

func TestGatewayMergeHeadersOAuthMarker(t *testing.T) {
	for _, raw := range []string{"", "X-Feature: keep-private", gatewayTestOAuthMarker, "x-lAnGsMiTh-aUtH-mOdE:  oauth  \nX-Feature: keep-private"} {
		t.Run(raw, func(t *testing.T) {
			merged, names, err := gatewayMergeHeaders(raw, "")
			require.NoError(t, err)
			require.Equal(t, raw, merged, "merge validates and preserves; only setup adds the marker")
			if strings.Contains(strings.ToLower(raw), "x-langsmith-auth-mode") {
				require.Len(t, names, strings.Count(raw, "\n")+1)
			} else {
				require.NotContains(t, names, "X-LangSmith-Auth-Mode")
			}
		})
	}
	merged, _, err := gatewayMergeHeaders(gatewayTestOAuthMarker, gatewayTestWorkspace)
	require.NoError(t, err)
	require.Equal(t, gatewayTestOAuthMarker+"\nX-Tenant-Id: "+gatewayTestWorkspace, merged)
}

func TestGatewayMergeHeadersRejectsInvalidOAuthMarkersAndCredentials(t *testing.T) {
	for _, raw := range []string{
		"X-LangSmith-Auth-Mode:",
		"X-LangSmith-Auth-Mode: OAuth",
		"X-LangSmith-Auth-Mode: OAUTH",
		"X-LangSmith-Auth-Mode: bearer do-not-print-secret",
		"X-LangSmith-Auth-Mode: oauth do-not-print-secret",
		"X-LangSmith-Auth-Mode: oauth\t",
		gatewayTestOAuthMarker + "\nx-langsmith-auth-mode: oauth",
		"X_LangSmith_Auth_Mode: oauth",
		"X-LangSmith-Auth-Mode-Extra: oauth",
		gatewayTestOAuthMarker + "\nAuthorization: do-not-print-secret",
		gatewayTestOAuthMarker + "\nX-Api-Key: do-not-print-secret",
		gatewayTestOAuthMarker + "\nX-Auth-Token: do-not-print-secret",
		gatewayTestOAuthMarker + "\nCookie: do-not-print-secret",
	} {
		t.Run(raw, func(t *testing.T) {
			merged, names, err := gatewayMergeHeaders(raw, "")
			require.Error(t, err)
			require.NotContains(t, err.Error(), "do-not-print-secret")
			require.Empty(t, merged)
			require.Empty(t, names)
		})
	}
}

func TestGatewaySetupOAuthMarkerIdempotent(t *testing.T) {
	for i, initial := range []string{"", "X-Feature: keep-private", gatewayTestOAuthMarker, "x-langsmith-auth-mode:  oauth  \nX-Feature: keep-private"} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			data, err := json.Marshal(map[string]any{"env": map[string]string{"ANTHROPIC_CUSTOM_HEADERS": initial}})
			require.NoError(t, err)
			gatewaySaveSettings(t, settings, string(data))
			out, err := gatewayRun(t, "--yes")
			require.NoError(t, err)
			require.NotContains(t, out, "keep-private")
			require.NotContains(t, out, "synthetic-")
			headers := readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"]
			want := initial
			if !strings.Contains(strings.ToLower(initial), "x-langsmith-auth-mode") {
				if want != "" {
					want += "\n"
				}
				want += gatewayTestOAuthMarker
			}
			require.Equal(t, want, headers)
			before, err := os.ReadFile(settings)
			require.NoError(t, err)
			_, err = gatewayRun(t, "--yes")
			require.NoError(t, err)
			after, err := os.ReadFile(settings)
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.NotContains(t, string(after), "synthetic-")
		})
	}
}

func TestGatewaySetupOAuthMarkerConflictsNoWrites(t *testing.T) {
	for _, source := range []string{"target", "environment", "higher scope"} {
		for _, headers := range []string{"X-LangSmith-Auth-Mode: do-not-print-secret", gatewayTestOAuthMarker + "\nx-langsmith-auth-mode: oauth", gatewayTestOAuthMarker + "\nAuthorization: do-not-print-secret"} {
			t.Run(source+"/"+headers, func(t *testing.T) {
				settings, cfg := gatewayTestEnv(t)
				initial := `{"model":"opus"}`
				data, err := json.Marshal(map[string]any{"env": map[string]string{"ANTHROPIC_CUSTOM_HEADERS": headers}})
				require.NoError(t, err)
				switch source {
				case "target":
					initial = string(data)
				case "environment":
					t.Setenv("ANTHROPIC_CUSTOM_HEADERS", headers)
				case "higher scope":
					gatewaySaveSettings(t, filepath.Join(".claude", "settings.json"), string(data))
				}
				gatewaySaveSettings(t, settings, initial)
				before, err := os.ReadFile(cfg)
				require.NoError(t, err)
				out, err := gatewayRun(t, "--yes")
				require.Error(t, err)
				require.NotContains(t, out+err.Error(), "do-not-print-secret")
				after, err := os.ReadFile(settings)
				require.NoError(t, err)
				require.Equal(t, initial, string(after))
				after, err = os.ReadFile(cfg)
				require.NoError(t, err)
				require.Equal(t, before, after)
			})
		}
	}
}

func TestGatewaySetupOAuthMarkerHigherScopePrecedence(t *testing.T) {
	for i, headers := range []string{"", "X-Feature: keep-private", gatewayTestOAuthMarker} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			other := filepath.Join(".claude", "settings.json")
			data, err := json.Marshal(map[string]any{"env": map[string]string{"ANTHROPIC_CUSTOM_HEADERS": headers}})
			require.NoError(t, err)
			gatewaySaveSettings(t, other, string(data))
			out, err := gatewayRun(t, "--yes")
			if headers == gatewayTestOAuthMarker {
				require.NoError(t, err)
				require.Equal(t, gatewayTestOAuthMarker, readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
			} else {
				require.ErrorContains(t, err, "conflicting env.ANTHROPIC_CUSTOM_HEADERS")
				require.NotContains(t, out+err.Error(), "keep-private")
				_, err = os.Stat(settings)
				require.True(t, os.IsNotExist(err))
			}
			after, err := os.ReadFile(other)
			require.NoError(t, err)
			require.Equal(t, data, after)
		})
	}
}

func TestGatewaySetupTokenDefaultIgnoresWorkspaceSources(t *testing.T) {
	for _, source := range []string{"profile", "LANGSMITH_TENANT_ID", "LANGSMITH_WORKSPACE_ID", "all"} {
		for i, value := range []string{gatewayTestWorkspace, "invalid-workspace-secret"} {
			t.Run(source+"/"+strconv.Itoa(i), func(t *testing.T) {
				settings, cfg := gatewayTestEnv(t)
				profile := map[string]any{"oauth": map[string]string{"access_token": "opaque-access-secret"}}
				if source == "profile" || source == "all" {
					profile["workspace_id"] = value
				}
				for _, key := range []string{"LANGSMITH_TENANT_ID", "LANGSMITH_WORKSPACE_ID"} {
					if source == key || source == "all" {
						t.Setenv(key, value)
					}
				}
				data, err := json.Marshal(map[string]any{"profiles": map[string]any{"default": profile}})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(cfg, data, 0o600))
				out, err := gatewayRun(t, "--yes")
				require.NoError(t, err)
				var report map[string]any
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				require.Empty(t, report["workspace_id"])
				require.Equal(t, []any{"X-LangSmith-Auth-Mode"}, report["custom_header_names"])
				require.NotContains(t, out, value)
				require.NotContains(t, out, "opaque-access-secret")
				require.Equal(t, gatewayTestOAuthMarker, readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
			})
		}
	}
}

func TestGatewaySetupExplicitWorkspaceFlags(t *testing.T) {
	for _, flag := range []string{"--workspace", "--workspace-id"} {
		for _, value := range []string{gatewayTestWorkspace, "", " ", "invalid-workspace-secret"} {
			t.Run(flag+"/"+value, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				t.Setenv("LANGSMITH_WORKSPACE_ID", "invalid-env-secret")
				t.Setenv("LANGSMITH_TENANT_ID", "invalid-env-secret")
				if value == "" {
					t.Setenv("LANGSMITH_WORKSPACE_ID", gatewayTestWorkspace)
					t.Setenv("LANGSMITH_TENANT_ID", gatewayTestWorkspace)
				}
				out, err := gatewayRun(t, "--yes", flag+"="+value)
				if value != gatewayTestWorkspace {
					require.Error(t, err)
					require.NotContains(t, out+err.Error(), "secret")
					_, err = os.Stat(settings)
					require.True(t, os.IsNotExist(err))
					return
				}
				require.NoError(t, err)
				var report map[string]any
				require.NoError(t, json.Unmarshal([]byte(out), &report))
				require.Equal(t, gatewayTestWorkspace, report["workspace_id"])
				require.Equal(t, "X-Tenant-Id: "+gatewayTestWorkspace+"\n"+gatewayTestOAuthMarker, readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
			})
		}
	}
}

func TestGatewaySetupLegacyTenantRequiresExplicitWorkspace(t *testing.T) {
	settings, cfg := gatewayTestEnv(t)
	require.NoError(t, os.WriteFile(cfg, []byte(`{"profiles":{"default":{"workspace_id":"`+gatewayTestWorkspace+`","oauth":{"access_token":"opaque-access-secret"}}}}`), 0o600))
	t.Setenv("LANGSMITH_WORKSPACE_ID", gatewayTestWorkspace)
	t.Setenv("LANGSMITH_TENANT_ID", gatewayTestWorkspace)
	initial := `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Feature: keep-private\nX-Tenant-Id: ` + gatewayTestWorkspace + `"}}`
	gatewaySaveSettings(t, settings, initial)
	out, err := gatewayRun(t, "--yes")
	require.ErrorContains(t, err, "X-Tenant-Id")
	require.NotContains(t, out+err.Error(), "keep-private")
	after, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, initial, string(after))
	_, err = gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
	require.NoError(t, err)
	require.Equal(t, "X-Feature: keep-private\nX-Tenant-Id: "+gatewayTestWorkspace+"\n"+gatewayTestOAuthMarker, readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
}

func TestGatewaySetupTokenDefaultDoesNotDecodeOrRefresh(t *testing.T) {
	jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(`{"tenant_id":"`+gatewayTestWorkspace+`","workspace_id":"`+gatewayTestWorkspace+`","exp":1}`)) + ".synthetic-signature"
	for i, token := range []string{"opaque-access-secret", "malformed.jwt", jwt, ""} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			settings, cfg := gatewayTestEnv(t)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			data, err := json.Marshal(map[string]any{"profiles": map[string]any{"default": map[string]any{
				"api_url": server.URL,
				"oauth":   map[string]string{"access_token": token, "refresh_token": "refresh-secret", "issuer": server.URL, "expires_at": "2000-01-01T00:00:00Z"},
			}}})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cfg, data, 0o600))
			for _, mode := range []string{"--dry-run", "--yes"} {
				out, err := gatewayRun(t, mode, "--gateway-url", server.URL)
				require.NoError(t, err)
				require.NotContains(t, out, "refresh-secret")
				if token != "" {
					require.NotContains(t, out, token)
				}
			}
			require.Zero(t, requests.Load())
			require.Equal(t, gatewayTestOAuthMarker, readJSONFile(t, settings)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
			saved, err := os.ReadFile(settings)
			require.NoError(t, err)
			require.NotContains(t, string(saved), "refresh-secret")
			if token != "" {
				require.NotContains(t, string(saved), token)
			}
			after, err := os.ReadFile(cfg)
			require.NoError(t, err)
			require.Equal(t, data, after)
			_, err = os.Stat(cfg + ".oauth.lock")
			require.True(t, os.IsNotExist(err))
		})
	}
}
