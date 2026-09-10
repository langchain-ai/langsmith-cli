package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const gatewayTestWorkspace = "519bb9dd-079b-4488-8610-e330951ea3e4"

func gatewayTestEnv(t *testing.T) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell setup only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(t.TempDir())
	for _, k := range append(append([]string{}, gatewayConflictingEnv...), "LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_PROFILE", "LANGSMITH_WORKSPACE_ID", "LANGSMITH_TENANT_ID", "ANTHROPIC_BASE_URL", "ANTHROPIC_CUSTOM_HEADERS") {
		t.Setenv(k, "")
	}
	for _, family := range gatewayModelFamilies {
		t.Setenv(gatewayModelEnv(family), "")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	cfg := filepath.Join(home, "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", cfg)
	require.NoError(t, os.WriteFile(cfg, []byte(`{"current_profile":"dev","profiles":{"dev":{"oauth":{"access_token":"synthetic-access-secret","refresh_token":"synthetic-refresh-secret","expires_at":"2000-01-01T00:00:00Z"}}}}`), 0o600))
	return filepath.Join(home, ".claude", "settings.json"), cfg
}

func gatewayRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd("test", "test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"--format=json", "gateway", "setup", "claude-code"}, args...))
	err := root.Execute()
	return out.String(), err
}

func gatewaySaveSettings(t *testing.T, path, data string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}

func TestGatewaySetupMergeIdempotent(t *testing.T) {
	settings, cfg := gatewayTestEnv(t)
	gatewaySaveSettings(t, settings, `{"model":"opus","unknown":{"n":900719925474099312345,"f":1.234567890123456789},"env":{"FOO":"bar","ANTHROPIC_CUSTOM_HEADERS":"X-Feature: keep-private"}}`)
	before, err := os.ReadFile(cfg)
	require.NoError(t, err)
	out, err := gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
	require.NoError(t, err)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	require.Equal(t, "configured", report["status"])
	for _, secret := range []string{"synthetic-access-secret", "synthetic-refresh-secret", "keep-private"} {
		require.NotContains(t, out, secret)
	}
	data, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Contains(t, string(data), "900719925474099312345")
	require.Contains(t, string(data), "1.234567890123456789")
	require.NotContains(t, string(data), "synthetic-")
	doc := readJSONFile(t, settings)
	exe, err := os.Executable()
	require.NoError(t, err)
	require.Equal(t, gatewayHelper(exe, "dev", "https://api.smith.langchain.com"), doc["apiKeyHelper"])
	require.Equal(t, map[string]any{
		"FOO": "bar", "ANTHROPIC_BASE_URL": "https://gateway.smith.langchain.com/anthropic",
		"CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "30000", "LANGSMITH_CONFIG_FILE": cfg,
		"ANTHROPIC_CUSTOM_HEADERS": "X-Feature: keep-private\nX-Tenant-Id: " + gatewayTestWorkspace + "\nX-LangSmith-Auth-Mode: oauth",
	}, doc["env"])
	assertPerm0600(t, settings)
	_, err = gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
	require.NoError(t, err)
	again, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, data, again)
	after, err := os.ReadFile(cfg)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = os.Stat(cfg + ".oauth.lock")
	require.True(t, os.IsNotExist(err))
}

func TestGatewaySetupDryRunAndAbortNoWrites(t *testing.T) {
	for _, mode := range []string{"dry-run", "noninteractive", "declined"} {
		t.Run(mode, func(t *testing.T) {
			settings, cfg := gatewayTestEnv(t)
			before, err := os.ReadFile(cfg)
			require.NoError(t, err)
			oldTerm := inputIsTerminal
			inputIsTerminal = func(io.Reader) bool { return mode == "declined" }
			t.Cleanup(func() { inputIsTerminal = oldTerm })
			args := []string{}
			if mode == "dry-run" {
				args = append(args, "--dry-run")
			}
			if mode == "declined" {
				r, w, err := os.Pipe()
				require.NoError(t, err)
				_, err = w.WriteString("n\n")
				require.NoError(t, err)
				require.NoError(t, w.Close())
				stdin := os.Stdin
				os.Stdin = r
				t.Cleanup(func() { os.Stdin = stdin; _ = r.Close() })
			}
			out, err := gatewayRun(t, args...)
			if mode == "dry-run" {
				require.NoError(t, err)
				require.Contains(t, out, `"status":"dry-run"`)
				require.Contains(t, out, "Trust this destination")
			} else {
				require.Error(t, err)
			}
			_, err = os.Stat(filepath.Dir(settings))
			require.True(t, os.IsNotExist(err))
			after, err := os.ReadFile(cfg)
			require.NoError(t, err)
			require.Equal(t, before, after)
			entries, err := os.ReadDir(filepath.Dir(cfg))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

func TestGatewaySetupProfileSelectionUsesTokenDefaultWorkspace(t *testing.T) {
	for _, selection := range []string{"explicit", "env", "current", "default"} {
		t.Run(selection, func(t *testing.T) {
			_, cfg := gatewayTestEnv(t)
			name := selection
			args := []string{"--dry-run"}
			current := "current"
			switch selection {
			case "explicit":
				name = "--evil ' $(touch pwned)"
				args = append(args, "--profile="+name)
				t.Setenv("LANGSMITH_PROFILE", "env")
			case "env":
				t.Setenv("LANGSMITH_PROFILE", name)
			case "default":
				current = ""
			}
			profiles := map[string]any{}
			for _, n := range []string{name, "env", "current", "default"} {
				profiles[n] = map[string]any{"workspace_id": gatewayTestWorkspace, "oauth": map[string]string{"access_token": "synthetic-token"}}
			}
			b, err := json.Marshal(map[string]any{"current_profile": current, "profiles": profiles})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(cfg, b, 0o600))
			out, err := gatewayRun(t, args...)
			require.NoError(t, err)
			var report map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &report))
			require.Equal(t, name, report["profile"])
			require.Empty(t, report["workspace_id"])
			require.Equal(t, []any{"X-LangSmith-Auth-Mode"}, report["custom_header_names"])
			envID := "619bb9dd-079b-4488-8610-e330951ea3e4"
			flagID := "719bb9dd-079b-4488-8610-e330951ea3e4"
			t.Setenv("LANGSMITH_WORKSPACE_ID", envID)
			out, err = gatewayRun(t, args...)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal([]byte(out), &report))
			require.Empty(t, report["workspace_id"])
			require.NotContains(t, out, envID)
			out, err = gatewayRun(t, append(args, "--workspace", flagID)...)
			require.NoError(t, err)
			require.Contains(t, out, flagID)
		})
	}
}

func TestGatewaySetupShellQuoting(t *testing.T) {
	gatewayTestEnv(t)
	dir := filepath.Join(t.TempDir(), "bin ' $(touch pwned)")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	exe := filepath.Join(dir, "langsmith ; evil")
	require.NoError(t, os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o700))
	profile := "--profile='; $(touch pwned)\nnext"
	apiURL := "https://example.com/'; $(touch pwned)"
	out, err := exec.Command("/bin/sh", "-c", gatewayHelper(exe, profile, apiURL)).Output()
	require.NoError(t, err)
	require.Equal(t, "--profile="+profile+"\n--api-url="+apiURL+"\n--format=pretty\nauth\ntoken\n", string(out))
	_, err = os.Stat("pwned")
	require.True(t, os.IsNotExist(err))
}

func TestGatewaySetupURLValidation(t *testing.T) {
	good := map[string]string{
		"https://gateway.example":                   "https://gateway.example/anthropic",
		"https://gateway.example/prefix/":           "https://gateway.example/prefix/anthropic",
		"https://gateway.example/prefix/anthropic/": "https://gateway.example/prefix/anthropic",
		"http://localhost:8080":                     "http://localhost:8080/anthropic",
		"http://127.0.0.1:8080":                     "http://127.0.0.1:8080/anthropic",
		"http://[::1]:8080":                         "http://[::1]:8080/anthropic",
	}
	for raw, want := range good {
		got, err := gatewayAnthropicURL(raw)
		require.NoError(t, err, raw)
		require.Equal(t, want, got)
	}
	for _, raw := range []string{"gateway.example", "http://gateway.example", "ftp://localhost", "https://u:secret@gateway.example", "https://gateway.example?secret=1", "https://gateway.example?", "https://gateway.example#", "https://gateway.example#secret", "https://gateway.example/../x", "https://gateway.example/%2fsecret", "https://gateway.example//x", "https://gateway.example/anthropic/anthropic", "http://localhost.evil", "https:///foo", "https://gateway.example:\n", "https://gateway.example:99999", "https://gateway.example/%0a", "https://gateway.example/%20"} {
		_, err := gatewayAnthropicURL(raw)
		require.Error(t, err, raw)
		require.NotContains(t, err.Error(), "secret")
	}
}

func TestGatewaySetupNonDefaultAndNoNetwork(t *testing.T) {
	_, cfg := gatewayTestEnv(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer server.Close()
	data := `{"current_profile":"dev","profiles":{"dev":{"api_url":"` + server.URL + `","oauth":{"refresh_token":"synthetic-refresh-secret","issuer":"` + server.URL + `"}}}}`
	require.NoError(t, os.WriteFile(cfg, []byte(data), 0o600))
	_, err := gatewayRun(t, "--dry-run")
	require.ErrorContains(t, err, "--gateway-url")
	_, err = gatewayRun(t, "--yes", "--gateway-url", server.URL)
	require.NoError(t, err)
	require.Zero(t, requests)
	for _, api := range []string{"https://eu.api.smith.langchain.com", "https://api.smith.langchain.com.evil", "https://self-hosted.example"} {
		t.Setenv("LANGSMITH_ENDPOINT", api)
		_, err := gatewayRun(t, "--dry-run")
		require.ErrorContains(t, err, "--gateway-url")
	}
}

func TestGatewaySetupConflictsUntouched(t *testing.T) {
	cases := []string{
		``, `null`, `[]`, `{bad`, `{"env":null}`, `{"env":[]}`, `{"env":{"FOO":123}}`, `{"env":{"FOO":null}}`, `{"apiKeyHelper":42}`, `{"apiKeyHelper":null}`,
		`{"env":{"ANTHROPIC_API_KEY":"do-not-print-secret"}}`, `{"env":{"ANTHROPIC_AUTH_TOKEN":"do-not-print-secret"}}`, `{"env":{"CLAUDE_CODE_OAUTH_TOKEN":"do-not-print-secret"}}`,
		`{"env":{"CLAUDE_CODE_USE_BEDROCK":"1"}}`, `{"env":{"CLAUDE_CODE_USE_VERTEX":"1"}}`, `{"env":{"CLAUDE_CODE_USE_FOUNDRY":"1"}}`,
		`{"env":{"LANGSMITH_CONFIG_FILE":"do-not-print-secret"}}`, `{"env":{"CLAUDE_CONFIG_DIR":"do-not-print-secret"}}`, `{"env":{"CLAUDE_CONFIG_DIR":""}}`,
		`{"env":{"ANTHROPIC_CUSTOM_HEADERS":"Authorization: do-not-print-secret"}}`, `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"x-API-key: do-not-print-secret"}}`,
		`{"env":{"ANTHROPIC_CUSTOM_HEADERS":"Cookie: do-not-print-secret"}}`, `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"bad do-not-print-secret"}}`,
		`{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Tenant-Id: do-not-print-secret"}}`,
	}
	for _, initial := range cases {
		t.Run(initial, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			gatewaySaveSettings(t, settings, initial)
			out, err := gatewayRun(t, "--yes")
			require.Error(t, err)
			require.NotContains(t, out+err.Error(), "do-not-print-secret")
			after, err := os.ReadFile(settings)
			require.NoError(t, err)
			require.Equal(t, initial, string(after))
		})
	}
}

func TestGatewaySetupInheritedConflicts(t *testing.T) {
	for _, key := range append(append([]string{}, gatewayConflictingEnv...), "ANTHROPIC_BASE_URL", "ANTHROPIC_CUSTOM_HEADERS") {
		t.Run(key, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			value := "do-not-print-secret"
			if key == "ANTHROPIC_CUSTOM_HEADERS" {
				value = "X-Private: " + value
			}
			t.Setenv(key, value)
			out, err := gatewayRun(t, "--yes")
			require.Error(t, err)
			require.NotContains(t, out+err.Error(), "do-not-print-secret")
			_, err = os.Stat(settings)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestGatewaySetupHeaderMerge(t *testing.T) {
	original := "X-Private: keep\nx-tenant-id: " + gatewayTestWorkspace
	merged, names, err := gatewayMergeHeaders(original, gatewayTestWorkspace)
	require.NoError(t, err)
	require.Equal(t, original, merged)
	require.Len(t, names, 2)
	for _, raw := range []string{original, "X-Tenant-Id: " + gatewayTestWorkspace + "\nX-Tenant-Id: " + gatewayTestWorkspace, "X-Foo: x\r\nX-Other: y", "X-Foo: x\nX-Foo: y"} {
		_, _, err := gatewayMergeHeaders(raw, "")
		require.Error(t, err)
	}
}

func TestGatewaySetupReplacementsAndOtherScopes(t *testing.T) {
	settings, _ := gatewayTestEnv(t)
	gatewaySaveSettings(t, settings, `{"apiKeyHelper":"old-secret-command","env":{"ANTHROPIC_BASE_URL":"https://old-secret.example"}}`)
	out, err := gatewayRun(t, "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, `"replaced_keys":["apiKeyHelper","env.ANTHROPIC_BASE_URL"]`)
	require.NotContains(t, out, "old-secret")
	_, err = gatewayRun(t, "--yes")
	require.NoError(t, err)
	gatewaySaveSettings(t, filepath.Join(".claude", "settings.local.json"), `{"env":{"ANTHROPIC_AUTH_TOKEN":"other-secret"}}`)
	out, err = gatewayRun(t, "--yes")
	require.Error(t, err)
	require.NotContains(t, out+err.Error(), "other-secret")
}

// Empty values in higher-precedence settings still override generated values.
func TestGatewaySetupEmptyOtherScopeConflictsNoWrites(t *testing.T) {
	cases := []struct {
		key     string
		initial string
	}{
		{"apiKeyHelper", `{"apiKeyHelper":""}`},
		{"env.ANTHROPIC_BASE_URL", `{"env":{"ANTHROPIC_BASE_URL":""}}`},
		{"env.LANGSMITH_CONFIG_FILE", `{"env":{"LANGSMITH_CONFIG_FILE":""}}`},
		{"env.CLAUDE_CODE_API_KEY_HELPER_TTL_MS", `{"env":{"CLAUDE_CODE_API_KEY_HELPER_TTL_MS":""}}`},
		{"env.ANTHROPIC_CUSTOM_HEADERS", `{"env":{"ANTHROPIC_CUSTOM_HEADERS":""}}`},
		{"env.CLAUDE_CONFIG_DIR", `{"env":{"CLAUDE_CONFIG_DIR":""}}`},
	}
	for _, otherFile := range []string{"settings.json", "settings.local.json"} {
		for _, tc := range cases {
			for _, existing := range []bool{false, true} {
				name := otherFile + "/" + tc.key + "/new"
				if existing {
					name = otherFile + "/" + tc.key + "/existing"
				}
				t.Run(name, func(t *testing.T) {
					settings, cfg := gatewayTestEnv(t)
					const original = `{"model":"opus","env":{"FOO":"bar"}}`
					if existing {
						gatewaySaveSettings(t, settings, original)
					}
					other := filepath.Join(".claude", otherFile)
					gatewaySaveSettings(t, other, tc.initial)
					before, err := os.ReadFile(cfg)
					require.NoError(t, err)
					_, err = gatewayRun(t, "--yes", "--scope=user", "--workspace", gatewayTestWorkspace)
					require.ErrorContains(t, err, "conflicting "+tc.key)
					require.ErrorContains(t, err, "other Claude settings scopes")
					if existing {
						after, err := os.ReadFile(settings)
						require.NoError(t, err)
						require.Equal(t, original, string(after))
					} else {
						_, err := os.Stat(filepath.Dir(settings))
						require.True(t, os.IsNotExist(err))
					}
					after, err := os.ReadFile(other)
					require.NoError(t, err)
					require.Equal(t, tc.initial, string(after))
					after, err = os.ReadFile(cfg)
					require.NoError(t, err)
					require.Equal(t, before, after)
					_, err = os.Stat(cfg + ".oauth.lock")
					require.True(t, os.IsNotExist(err))
					entries, err := os.ReadDir(".claude")
					require.NoError(t, err)
					require.Len(t, entries, 1)
				})
			}
		}
	}
}

func TestGatewaySetupLocalOverridesLowerScopeGeneratedSettings(t *testing.T) {
	for _, lowerScope := range []string{"user", "shared project"} {
		for _, selection := range []string{"workspace", "profile and config"} {
			t.Run(lowerScope+"/"+selection, func(t *testing.T) {
				settings, cfg := gatewayTestEnv(t)
				_, err := gatewayRun(t, "--yes", "--scope=user", "--workspace", gatewayTestWorkspace)
				require.NoError(t, err)
				original, err := os.ReadFile(settings)
				require.NoError(t, err)
				lower := settings
				if lowerScope == "shared project" {
					lower = filepath.Join(".claude", "settings.json")
					gatewaySaveSettings(t, lower, string(original))
					require.NoError(t, os.Remove(settings))
				}
				const workspaceB = "619bb9dd-079b-4488-8610-e330951ea3e4"
				profile := "dev"
				if selection == "profile and config" {
					profile = "other"
					cfg = filepath.Join(t.TempDir(), "other-config.json")
					require.NoError(t, os.WriteFile(cfg, []byte(`{"profiles":{"other":{"oauth":{"access_token":"other-secret"}}}}`), 0o600))
					t.Setenv("LANGSMITH_CONFIG_FILE", cfg)
				}
				out, err := gatewayRun(t, "--yes", "--scope=project", "--workspace", workspaceB, "--profile="+profile)
				require.NoError(t, err)
				require.NotContains(t, out, "other-secret")
				doc := readJSONFile(t, filepath.Join(".claude", "settings.local.json"))
				exe, err := os.Executable()
				require.NoError(t, err)
				require.Equal(t, gatewayHelper(exe, profile, "https://api.smith.langchain.com"), doc["apiKeyHelper"])
				env := doc["env"].(map[string]any)
				require.Equal(t, "X-Tenant-Id: "+workspaceB+"\nX-LangSmith-Auth-Mode: oauth", env["ANTHROPIC_CUSTOM_HEADERS"])
				require.Equal(t, cfg, env["LANGSMITH_CONFIG_FILE"])
				after, err := os.ReadFile(lower)
				require.NoError(t, err)
				require.Equal(t, original, after)
			})
		}
	}
}

func TestGatewaySetupLocalOverridesLowerScopeEmptyValues(t *testing.T) {
	for _, lowerScope := range []string{"user", "shared project"} {
		t.Run(lowerScope, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			lower := settings
			if lowerScope == "shared project" {
				lower = filepath.Join(".claude", "settings.json")
			}
			gatewaySaveSettings(t, lower, `{"apiKeyHelper":"","env":{"ANTHROPIC_BASE_URL":"","LANGSMITH_CONFIG_FILE":"","CLAUDE_CODE_API_KEY_HELPER_TTL_MS":"","ANTHROPIC_CUSTOM_HEADERS":""}}`)
			_, err := gatewayRun(t, "--yes", "--scope=project", "--workspace", gatewayTestWorkspace)
			require.NoError(t, err)
			doc := readJSONFile(t, filepath.Join(".claude", "settings.local.json"))
			require.NotEmpty(t, doc["apiKeyHelper"])
			for _, key := range []string{"ANTHROPIC_BASE_URL", "LANGSMITH_CONFIG_FILE", "CLAUDE_CODE_API_KEY_HELPER_TTL_MS", "ANTHROPIC_CUSTOM_HEADERS"} {
				require.NotEmpty(t, doc["env"].(map[string]any)[key], key)
			}
		})
	}
}

func TestGatewaySetupLocalInheritedHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers string
		wantErr string
	}{
		{"unrelated", "X-Feature: do-not-print-secret", ""},
		{"empty", "", ""},
		{"oauth marker", "X-LangSmith-Auth-Mode: oauth", ""},
		{"invalid oauth marker", "X-LangSmith-Auth-Mode: do-not-print-secret", "X-LangSmith-Auth-Mode"},
		{"duplicate oauth marker", "X-LangSmith-Auth-Mode: oauth\nx-langsmith-auth-mode: oauth", "duplicate"},
		{"tenant", "X-Tenant-Id: " + gatewayTestWorkspace, "conflicting X-Tenant-Id"},
		{"authorization", "Authorization: do-not-print-secret", "conflicting header"},
		{"api key", "X-Api-Key: do-not-print-secret", "conflicting header"},
		{"malformed", "do-not-print-secret", "malformed"},
		{"duplicate", "X-Feature: one\nX-Feature: do-not-print-secret", "duplicate"},
	}
	for _, lowerScope := range []string{"user", "shared project"} {
		for _, tc := range cases {
			t.Run(lowerScope+"/"+tc.name, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				lower := settings
				if lowerScope == "shared project" {
					lower = filepath.Join(".claude", "settings.json")
				}
				data, err := json.Marshal(map[string]any{"env": map[string]string{"ANTHROPIC_CUSTOM_HEADERS": tc.headers}})
				require.NoError(t, err)
				gatewaySaveSettings(t, lower, string(data))
				out, err := gatewayRun(t, "--yes", "--scope=project")
				local := filepath.Join(".claude", "settings.local.json")
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					require.NotContains(t, err.Error(), "do-not-print-secret")
					_, err = os.Stat(local)
					require.True(t, os.IsNotExist(err))
				} else {
					require.NoError(t, err)
					require.Equal(t, "X-LangSmith-Auth-Mode: oauth", readJSONFile(t, local)["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
				}
				require.NotContains(t, out, "do-not-print-secret")
				after, err := os.ReadFile(lower)
				require.NoError(t, err)
				require.Equal(t, data, after)
			})
		}
	}
}

func TestGatewaySetupHeadersUseEffectiveScope(t *testing.T) {
	for _, replacement := range []string{"shared project", "local empty"} {
		t.Run(replacement, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			gatewaySaveSettings(t, settings, `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Tenant-Id: `+gatewayTestWorkspace+`"}}`)
			if replacement == "shared project" {
				gatewaySaveSettings(t, filepath.Join(".claude", "settings.json"), `{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Feature: keep-private"}}`)
			} else {
				gatewaySaveSettings(t, filepath.Join(".claude", "settings.local.json"), `{"env":{"ANTHROPIC_CUSTOM_HEADERS":""}}`)
			}
			out, err := gatewayRun(t, "--yes", "--scope=project")
			require.NoError(t, err)
			require.NotContains(t, out, "keep-private")
		})
	}
}

func TestGatewaySetupLowerScopeCredentialsAndConfigDirStillRejected(t *testing.T) {
	for _, lowerScope := range []string{"user", "shared project"} {
		for _, key := range append(append([]string{}, gatewayConflictingEnv...), "CLAUDE_CONFIG_DIR") {
			t.Run(lowerScope+"/"+key, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				lower := settings
				if lowerScope == "shared project" {
					lower = filepath.Join(".claude", "settings.json")
				}
				data, err := json.Marshal(map[string]any{"env": map[string]string{key: "do-not-print-secret"}})
				require.NoError(t, err)
				gatewaySaveSettings(t, lower, string(data))
				out, err := gatewayRun(t, "--yes", "--scope=project", "--workspace", gatewayTestWorkspace)
				require.ErrorContains(t, err, key)
				require.NotContains(t, out+err.Error(), "do-not-print-secret")
				_, err = os.Stat(filepath.Join(".claude", "settings.local.json"))
				require.True(t, os.IsNotExist(err))
			})
		}
	}
}

func TestGatewaySetupOtherScopesEffectiveGeneratedConflicts(t *testing.T) {
	for _, initial := range []string{
		`{"apiKeyHelper":"other-secret"}`,
		`{"env":{"ANTHROPIC_BASE_URL":"https://other-secret.example"}}`,
		`{"env":{"LANGSMITH_CONFIG_FILE":"other-secret"}}`,
		`{"env":{"CLAUDE_CODE_API_KEY_HELPER_TTL_MS":"other-secret"}}`,
		`{"env":{"ANTHROPIC_CUSTOM_HEADERS":"X-Feature: other-secret"}}`,
	} {
		t.Run(initial, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			_, err := gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
			require.NoError(t, err)
			original, err := os.ReadFile(settings)
			require.NoError(t, err)
			gatewaySaveSettings(t, filepath.Join(".claude", "settings.json"), initial)
			out, err := gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
			require.ErrorContains(t, err, "other Claude settings scopes")
			require.NotContains(t, out+err.Error(), "other-secret")
			// A matching local value masks the shared project conflict.
			gatewaySaveSettings(t, filepath.Join(".claude", "settings.local.json"), string(original))
			_, err = gatewayRun(t, "--yes", "--workspace", gatewayTestWorkspace)
			require.NoError(t, err)
		})
	}
}

func TestGatewaySetupProjectAndRelativeConfig(t *testing.T) {
	_, cfg := gatewayTestEnv(t)
	data, err := os.ReadFile(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("relative-config.json", data, 0o600))
	t.Setenv("LANGSMITH_CONFIG_FILE", "relative-config.json")
	_, err = gatewayRun(t, "--yes", "--scope=project")
	require.NoError(t, err)
	doc := readJSONFile(t, filepath.Join(".claude", "settings.local.json"))
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cwd, "relative-config.json"), doc["env"].(map[string]any)["LANGSMITH_CONFIG_FILE"])
	require.Equal(t, "X-LangSmith-Auth-Mode: oauth", doc["env"].(map[string]any)["ANTHROPIC_CUSTOM_HEADERS"])
}

func TestGatewaySetupProjectSymlink(t *testing.T) {
	gatewayTestEnv(t)
	victim := t.TempDir()
	require.NoError(t, os.Symlink(victim, ".claude"))
	_, err := gatewayRun(t, "--yes", "--scope=project")
	require.ErrorContains(t, err, "symlink")
	entries, err := os.ReadDir(victim)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestGatewaySetupMissingOAuthAndInvalidWorkspace(t *testing.T) {
	settings, cfg := gatewayTestEnv(t)
	for _, data := range []string{`{"profiles":{}}`, `{"profiles":{"default":{"api_key":"synthetic-api-secret"}}}`} {
		require.NoError(t, os.WriteFile(cfg, []byte(data), 0o600))
		out, err := gatewayRun(t, "--yes")
		require.ErrorContains(t, err, "OAuth")
		require.NotContains(t, out, "synthetic-api-secret")
	}
	require.NoError(t, os.WriteFile(cfg, []byte(`{"profiles":{"default":{"oauth":{"access_token":"synthetic-token"}}}}`), 0o600))
	_, err := gatewayRun(t, "--yes", "--workspace", "bad\r\nAuthorization: secret")
	require.ErrorContains(t, err, "UUID")
	require.NotContains(t, err.Error(), "secret")
	_, err = os.Stat(settings)
	require.True(t, os.IsNotExist(err))
}

func TestGatewaySetupWriterDetectsConcurrentEdit(t *testing.T) {
	settings, _ := gatewayTestEnv(t)
	gatewaySaveSettings(t, settings, `{"model":"new"}`)
	err := gatewayWriteSettings(settings, []byte(`{}`), []byte(`{"env":{}}`), false)
	require.ErrorContains(t, err, "changed after preview")
	data, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(data), "new"))
}

func TestGatewaySetupNeverOverwritesCredentialStore(t *testing.T) {
	_, cfg := gatewayTestEnv(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Dir(cfg))
	// The selected store may be named settings.json; it is still credentials.
	settings := filepath.Join(filepath.Dir(cfg), "settings.json")
	data, err := os.ReadFile(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(settings, data, 0o600))
	t.Setenv("LANGSMITH_CONFIG_FILE", settings)
	_, err = gatewayRun(t, "--yes")
	require.ErrorContains(t, err, "different files")
	after, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, data, after)
}

func TestGatewaySetupDryRunExistingSettingsUntouched(t *testing.T) {
	settings, _ := gatewayTestEnv(t)
	original := `{"model":"opus","env":{"FOO":"bar"}}`
	gatewaySaveSettings(t, settings, original)
	out, err := gatewayRun(t, "--dry-run")
	require.NoError(t, err)
	require.Contains(t, out, "apiKeyHelper")
	after, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, original, string(after))
}

func TestGatewaySetupTightensIdenticalSettings(t *testing.T) {
	settings, _ := gatewayTestEnv(t)
	_, err := gatewayRun(t, "--yes")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(settings, 0o644))
	_, err = gatewayRun(t, "--yes")
	require.NoError(t, err)
	assertPerm0600(t, settings)
}
