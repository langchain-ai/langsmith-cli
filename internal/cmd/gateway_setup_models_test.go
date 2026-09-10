package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// Keep the public flag/env contract independent of implementation constants.
var gatewayModelTestFamilies = []struct {
	flag string
	env  string
}{
	{"--haiku-model", "ANTHROPIC_DEFAULT_HAIKU_MODEL"},
	{"--sonnet-model", "ANTHROPIC_DEFAULT_SONNET_MODEL"},
	{"--opus-model", "ANTHROPIC_DEFAULT_OPUS_MODEL"},
	{"--fable-model", "ANTHROPIC_DEFAULT_FABLE_MODEL"},
}

func gatewayModelTestValues(provider string) map[string]string {
	values := map[string]string{}
	for i, family := range gatewayModelTestFamilies {
		values[family.env] = fmt.Sprintf("%s/model-%d", provider, i)
	}
	return values
}

func gatewayModelTestFlags(values map[string]string) []string {
	var args []string
	for _, family := range gatewayModelTestFamilies {
		if value, ok := values[family.env]; ok {
			args = append(args, family.flag+"="+value)
		}
	}
	return args
}

func gatewayModelTestSave(t *testing.T, path string, env map[string]string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"model": "opus", "unrelated": map[string]any{"keep": true}, "env": env})
	require.NoError(t, err)
	gatewaySaveSettings(t, path, string(data))
}

func gatewayModelTestReport(t *testing.T, out string) map[string]any {
	t.Helper()
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	return report
}

func gatewayModelTestAssertEnv(t *testing.T, doc map[string]any, want map[string]string, baseURL string) {
	t.Helper()
	env, ok := doc["env"].(map[string]any)
	require.True(t, ok, "expected env object: %v", doc)
	require.Equal(t, baseURL, env["ANTHROPIC_BASE_URL"])
	for _, family := range gatewayModelTestFamilies {
		if value, present := want[family.env]; present {
			require.Equal(t, value, env[family.env], family.env)
		} else {
			require.NotContains(t, env, family.env)
		}
	}
}

// Compare entire isolated home/project trees, not just settings contents: a
// failed validation or dry-run must not create directories, locks or temp files,
// rewrite OAuth credentials, or change existing file permissions.
func gatewayModelTestNoWrites(t *testing.T) func() {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	type entry struct {
		Mode fs.FileMode
		Data string
	}
	snapshot := func() map[string]entry {
		t.Helper()
		entries := map[string]entry{}
		for _, root := range []string{os.Getenv("HOME"), cwd} {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				var data []byte
				if !d.IsDir() {
					data, err = os.ReadFile(path)
					if err != nil {
						return err
					}
				}
				entries[path] = entry{Mode: info.Mode(), Data: string(data)}
				return nil
			})
			require.NoError(t, err)
		}
		return entries
	}
	before := snapshot()
	return func() {
		t.Helper()
		require.Equal(t, before, snapshot(), "setup must leave home and project untouched")
	}
}

func TestGatewaySetupModelsFlagEnvironmentMixtures(t *testing.T) {
	// Every flag/env partition, including all flags, all env, and fable alone
	// from either source. Flags must win even when shell and saved values differ.
	for mask := 0; mask < 1<<len(gatewayModelTestFamilies); mask++ {
		t.Run(fmt.Sprintf("flags-%04b", mask), func(t *testing.T) {
			settings, cfg := gatewayTestEnv(t)
			gatewayModelTestSave(t, settings, gatewayModelTestValues("saved"))
			beforeConfig, err := os.ReadFile(cfg)
			require.NoError(t, err)
			want := gatewayModelTestValues("shell")
			args := []string{"--yes"}
			for i, family := range gatewayModelTestFamilies {
				t.Setenv(family.env, want[family.env])
				if mask&(1<<i) != 0 {
					want[family.env] = fmt.Sprintf("flag/model-%d", i)
					args = append(args, family.flag+"="+want[family.env])
				}
			}
			out, err := gatewayRun(t, args...)
			require.NoError(t, err)
			report := gatewayModelTestReport(t, out)
			require.Equal(t, "configured", report["status"])
			gatewayModelTestAssertEnv(t, report, want, "https://gateway.smith.langchain.com")
			doc := readJSONFile(t, settings)
			gatewayModelTestAssertEnv(t, doc, want, "https://gateway.smith.langchain.com")
			require.Equal(t, "opus", doc["model"])
			require.Equal(t, map[string]any{"keep": true}, doc["unrelated"])
			assertPerm0600(t, settings)
			afterConfig, err := os.ReadFile(cfg)
			require.NoError(t, err)
			require.Equal(t, beforeConfig, afterConfig)
		})
	}
}

func TestGatewaySetupModelsCompleteRepeatAndPartialUpdateRejected(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			if scope == "project" {
				settings = filepath.Join(".claude", "settings.local.json")
			}
			old := gatewayModelTestValues("saved")
			old["UNRELATED"] = "keep-private"
			gatewayModelTestSave(t, settings, old)
			want := gatewayModelTestValues("requested")
			args := append([]string{"--yes", "--scope=" + scope}, gatewayModelTestFlags(want)...)
			out, err := gatewayRun(t, args...)
			require.NoError(t, err)
			gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
			doc := readJSONFile(t, settings)
			gatewayModelTestAssertEnv(t, doc, want, "https://gateway.smith.langchain.com")
			require.Equal(t, "keep-private", doc["env"].(map[string]any)["UNRELATED"])
			require.Equal(t, "opus", doc["model"])
			require.Equal(t, map[string]any{"keep": true}, doc["unrelated"])

			// Repeating a complete replacement must supply all four again.
			assertUnchanged := gatewayModelTestNoWrites(t)
			defer assertUnchanged()
			out, err = gatewayRun(t, args...)
			require.NoError(t, err)
			report := gatewayModelTestReport(t, out)
			require.Empty(t, report["replaced_keys"])
			require.Empty(t, report["removed_keys"])
			gatewayModelTestAssertEnv(t, report, want, "https://gateway.smith.langchain.com")
			assertUnchanged()
			for _, family := range gatewayModelTestFamilies {
				for _, mode := range []string{"--yes", "--dry-run"} {
					_, err := gatewayRun(t, mode, "--scope="+scope, family.flag+"=changed/model")
					require.ErrorContains(t, err, "all four")
					assertUnchanged()
				}
			}
		})
	}
}

func TestGatewaySetupModelsClearTargetAndPreview(t *testing.T) {
	// Include every partial target map, an empty map, and a complete map.
	for _, scope := range []string{"user", "project"} {
		for mask := 0; mask < 1<<len(gatewayModelTestFamilies); mask++ {
			for _, mode := range []string{"--yes", "--dry-run", "both"} {
				t.Run(fmt.Sprintf("%s/%04b/%s", scope, mask, mode), func(t *testing.T) {
					settings, cfg := gatewayTestEnv(t)
					if scope == "project" {
						settings = filepath.Join(".claude", "settings.local.json")
					}
					old := map[string]string{"UNRELATED": "keep-private", "ANTHROPIC_BASE_URL": "https://gateway.smith.langchain.com"}
					removed := []any{}
					keys := []string{}
					for i, family := range gatewayModelTestFamilies {
						if mask&(1<<i) != 0 {
							// Saved values are not inputs: even empty/invalid aliases can be cleaned.
							old[family.env] = []string{"private-old/model", "", "not-a-slug", "private-old/fable"}[i]
							keys = append(keys, "env."+family.env)
						}
					}
					sort.Strings(keys)
					for _, key := range keys {
						removed = append(removed, key)
					}
					gatewayModelTestSave(t, settings, old)
					beforeConfig, err := os.ReadFile(cfg)
					require.NoError(t, err)
					args := []string{mode, "--scope=" + scope}
					if mode == "both" {
						args = []string{"--yes", "--dry-run", "--scope=" + scope}
					}
					if mode != "--yes" {
						defer gatewayModelTestNoWrites(t)()
					}
					out, err := gatewayRun(t, args...)
					require.NoError(t, err)
					report := gatewayModelTestReport(t, out)
					status := "dry-run"
					if mode == "--yes" {
						status = "configured"
					}
					require.Equal(t, status, report["status"])
					require.Equal(t, removed, report["removed_keys"], "removals must be a sorted JSON array of keys only")
					require.Equal(t, []any{"env.ANTHROPIC_BASE_URL"}, report["replaced_keys"])
					gatewayModelTestAssertEnv(t, report, nil, "https://gateway.smith.langchain.com/anthropic")
					for _, private := range []string{"private-old", "not-a-slug", "keep-private", "synthetic-access-secret", "synthetic-refresh-secret"} {
						require.NotContains(t, out, private)
					}
					if mode == "--yes" {
						doc := readJSONFile(t, settings)
						gatewayModelTestAssertEnv(t, doc, nil, "https://gateway.smith.langchain.com/anthropic")
						require.Equal(t, "keep-private", doc["env"].(map[string]any)["UNRELATED"])
						require.Equal(t, "opus", doc["model"])
						require.Equal(t, map[string]any{"keep": true}, doc["unrelated"])
						assertPerm0600(t, settings)
						assertUnchanged := gatewayModelTestNoWrites(t)
						out, err = gatewayRun(t, args...)
						require.NoError(t, err)
						report = gatewayModelTestReport(t, out)
						require.Equal(t, []any{}, report["removed_keys"])
						require.Empty(t, report["replaced_keys"])
						gatewayModelTestAssertEnv(t, report, nil, "https://gateway.smith.langchain.com/anthropic")
						assertUnchanged()
					}
					afterConfig, err := os.ReadFile(cfg)
					require.NoError(t, err)
					require.Equal(t, beforeConfig, afterConfig)
				})
			}
		}
	}
}

func TestGatewaySetupModelsPartialSubsetsRejectedWithoutWrites(t *testing.T) {
	// All 14 nonempty proper subsets, including fable-only and missing-fable.
	// Saved complete maps at any scope must NEVER supply the missing inputs.
	for mask := 1; mask < (1<<len(gatewayModelTestFamilies))-1; mask++ {
		for _, source := range []string{"flags", "environment", "mixed"} {
			for _, savedScope := range []string{"none", "user", "shared", "local"} {
				for _, mode := range []string{"--yes", "--dry-run", "both"} {
					t.Run(fmt.Sprintf("%04b/%s/saved-%s/%s", mask, source, savedScope, mode), func(t *testing.T) {
						settings, _ := gatewayTestEnv(t)
						if savedScope != "none" {
							if savedScope == "shared" {
								settings = filepath.Join(".claude", "settings.json")
							} else if savedScope == "local" {
								settings = filepath.Join(".claude", "settings.local.json")
							}
							gatewayModelTestSave(t, settings, gatewayModelTestValues("saved"))
						}
						args := []string{mode}
						if mode == "both" {
							args = []string{"--dry-run", "--yes"}
						}
						for i, family := range gatewayModelTestFamilies {
							if mask&(1<<i) == 0 {
								continue
							}
							value := fmt.Sprintf("provider/model-%d", i)
							if source == "flags" || source == "mixed" && i%2 == 0 {
								args = append(args, family.flag+"="+value)
							} else {
								t.Setenv(family.env, value)
							}
						}
						defer gatewayModelTestNoWrites(t)()
						_, err := gatewayRun(t, args...)
						require.ErrorContains(t, err, "all four")
						for i, family := range gatewayModelTestFamilies {
							if mask&(1<<i) == 0 {
								require.Contains(t, err.Error(), family.flag, "missing family must identify its flag")
								require.Contains(t, err.Error(), family.env, "missing family must identify its env alternative")
							}
						}
					})
				}
			}
		}
	}
}

func TestGatewaySetupModelsExplicitEmptyFlagRejected(t *testing.T) {
	for _, family := range gatewayModelTestFamilies {
		for _, mode := range []string{"--yes", "--dry-run"} {
			t.Run(family.flag+"/"+mode, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				values := gatewayModelTestValues("saved")
				gatewayModelTestSave(t, settings, values)
				// An explicitly empty flag is invalid, not a request to fall back.
				for key, value := range gatewayModelTestValues("shell") {
					t.Setenv(key, value)
				}
				defer gatewayModelTestNoWrites(t)()
				_, err := gatewayRun(t, mode, family.flag+"=")
				require.ErrorContains(t, err, family.flag)
				require.NotContains(t, err.Error(), "unknown flag")
			})
		}
	}
}

func TestGatewaySetupModelsInvalidSlugs(t *testing.T) {
	invalid := []struct{ name, value string }{
		{"no-provider-separator", "claude-sonnet-4-6"},
		{"empty-provider", "/model"},
		{"empty-model", "provider/"},
		{"both-empty", "/"},
		{"double-slash", "provider/model//version"},
		{"trailing-slash", "provider/model/"},
		{"backslash", "provider/mo\\del"},
		{"query", "provider/model?version=1"},
		{"fragment", "provider/model#version"},
		{"leading-space", " provider/model"},
		{"trailing-space", "provider/model "},
		{"provider-space", "pro vider/model"},
		{"model-space", "provider/mo del"},
		{"tab", "provider/mo\tdel"},
		{"newline", "provider/model\n"},
		{"carriage-return", "provider/mo\rdel"},
		{"nul", "provider/mo\x00del"},
		{"control", "provider/mo\x01del"},
		{"delete", "provider/mo\x7fdel"},
		{"unicode-control", "provider/mo\u009fdel"},
		{"unicode-space", "provider/mo\u00a0del"},
		{"unicode-line-separator", "provider/mo\u2028del"},
	}
	for _, family := range gatewayModelTestFamilies {
		for _, source := range []string{"flag", "environment"} {
			for _, tc := range invalid {
				// OS environments cannot represent NUL; flags can.
				if source == "environment" && tc.name == "nul" {
					continue
				}
				t.Run(family.flag+"/"+source+"/"+tc.name, func(t *testing.T) {
					settings, _ := gatewayTestEnv(t)
					values := gatewayModelTestValues("valid")
					args := []string{"--yes"}
					gatewayModelTestSave(t, settings, values)
					values[family.env] = tc.value
					if source == "flag" {
						args = append(args, gatewayModelTestFlags(values)...)
					} else {
						for key, value := range values {
							t.Setenv(key, value)
						}
					}
					defer gatewayModelTestNoWrites(t)()
					_, err := gatewayRun(t, args...)
					require.Error(t, err)
					require.NotContains(t, err.Error(), "unknown flag")
					if source == "flag" {
						require.Contains(t, err.Error(), family.flag)
					} else {
						require.Contains(t, err.Error(), family.env)
					}
				})
			}
		}
	}
}

func TestGatewaySetupModelsGatewayURLRouting(t *testing.T) {
	urls := []struct{ name, raw, root string }{
		{"inferred", "", "https://gateway.smith.langchain.com"},
		{"root", "https://gateway.example", "https://gateway.example"},
		{"root-slash", "https://gateway.example/", "https://gateway.example"},
		{"anthropic", "https://gateway.example/anthropic", "https://gateway.example"},
		{"anthropic-slash", "https://gateway.example/anthropic/", "https://gateway.example"},
		{"prefix", "https://gateway.example/deploy/gateway", "https://gateway.example/deploy/gateway"},
		{"prefix-slash", "https://gateway.example/deploy/gateway/", "https://gateway.example/deploy/gateway"},
		{"prefix-anthropic", "https://gateway.example/deploy/gateway/anthropic", "https://gateway.example/deploy/gateway"},
		{"prefix-anthropic-slash", "https://gateway.example/deploy/gateway/anthropic/", "https://gateway.example/deploy/gateway"},
		{"suffix-not-segment", "https://gateway.example/custom-anthropic", "https://gateway.example/custom-anthropic"},
		{"loopback-prefix", "http://localhost:8080/deploy/anthropic", "http://localhost:8080/deploy"},
	}
	for _, tc := range urls {
		for _, source := range []string{"none", "flags", "environment", "saved"} {
			t.Run(tc.name+"/"+source, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				args := []string{"--yes"}
				if tc.raw != "" {
					args = append(args, "--gateway-url="+tc.raw)
				}
				want := gatewayModelTestValues("provider")
				baseURL := tc.root
				switch source {
				case "none":
					want = nil
					baseURL += "/anthropic"
				case "flags":
					args = append(args, gatewayModelTestFlags(want)...)
				case "environment":
					for key, value := range want {
						t.Setenv(key, value)
					}
				case "saved":
					gatewayModelTestSave(t, settings, want)
					want = nil
					baseURL += "/anthropic"
				}
				out, err := gatewayRun(t, args...)
				require.NoError(t, err)
				report := gatewayModelTestReport(t, out)
				gatewayModelTestAssertEnv(t, report, want, baseURL)
				require.Contains(t, report["consent"], baseURL)
				gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, baseURL)
			})
		}
	}
}

func TestGatewaySetupModelsDryRunPreviewReplacements(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing-%t", scope, existing), func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				if scope == "project" {
					settings = filepath.Join(".claude", "settings.local.json")
				}
				want := gatewayModelTestValues("new-provider")
				if existing {
					old := gatewayModelTestValues("private-old-provider")
					old["ANTHROPIC_BASE_URL"] = "https://gateway.example/prefix/anthropic"
					old["UNRELATED"] = "private-unrelated-value"
					// Unchanged values must not be advertised as replacements.
					old[gatewayModelTestFamilies[1].env] = want[gatewayModelTestFamilies[1].env]
					gatewayModelTestSave(t, settings, old)
				}
				defer gatewayModelTestNoWrites(t)()
				args := []string{"--dry-run", "--yes", "--scope=" + scope, "--gateway-url=https://gateway.example/prefix/anthropic/"}
				args = append(args, gatewayModelTestFlags(want)...)
				out, err := gatewayRun(t, args...)
				require.NoError(t, err)
				report := gatewayModelTestReport(t, out)
				require.Equal(t, "dry-run", report["status"])
				require.Equal(t, []any{}, report["removed_keys"])
				gatewayModelTestAssertEnv(t, report, want, "https://gateway.example/prefix")
				if existing {
					require.ElementsMatch(t, []any{
						"env.ANTHROPIC_BASE_URL", "env.ANTHROPIC_DEFAULT_HAIKU_MODEL",
						"env.ANTHROPIC_DEFAULT_OPUS_MODEL", "env.ANTHROPIC_DEFAULT_FABLE_MODEL",
					}, report["replaced_keys"])
				} else {
					require.Empty(t, report["replaced_keys"])
				}
				require.NotContains(t, out, "private-old-provider")
				require.NotContains(t, out, "private-unrelated-value")
				require.NotContains(t, out, "synthetic-access-secret")
				require.NotContains(t, out, "synthetic-refresh-secret")
			})
		}
	}
}

func TestGatewaySetupModelsHigherEffectiveConflicts(t *testing.T) {
	for _, family := range gatewayModelTestFamilies {
		for _, higher := range []string{"settings.json", "settings.local.json"} {
			for _, source := range []string{"flag", "environment"} {
				for _, mode := range []string{"--yes", "--dry-run"} {
					t.Run(family.flag+"/"+higher+"/"+source+"/"+mode, func(t *testing.T) {
						settings, _ := gatewayTestEnv(t)
						gatewayModelTestSave(t, settings, gatewayModelTestValues("saved"))
						gatewayModelTestSave(t, filepath.Join(".claude", higher), map[string]string{family.env: "higher/conflict"})
						args := []string{mode, "--scope=user"}
						want := gatewayModelTestValues("requested")
						if source == "flag" {
							args = append(args, gatewayModelTestFlags(want)...)
						} else {
							for key, value := range want {
								t.Setenv(key, value)
							}
						}
						defer gatewayModelTestNoWrites(t)()
						_, err := gatewayRun(t, args...)
						require.ErrorContains(t, err, family.env)
						require.NotContains(t, err.Error(), "unknown flag")
					})
				}
			}
		}
	}
}

func TestGatewaySetupModelsMatchingLocalMasksSharedConflict(t *testing.T) {
	for _, family := range gatewayModelTestFamilies {
		for _, source := range []string{"flag", "environment"} {
			t.Run(family.flag+"/"+source, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				want := gatewayModelTestValues("saved")
				gatewayModelTestSave(t, settings, want)
				gatewayModelTestSave(t, filepath.Join(".claude", "settings.json"), map[string]string{family.env: "shared/conflict"})
				want[family.env] = "requested/model"
				gatewayModelTestSave(t, filepath.Join(".claude", "settings.local.json"), map[string]string{family.env: want[family.env]})
				args := []string{"--yes", "--scope=user"}
				if source == "flag" {
					args = append(args, gatewayModelTestFlags(want)...)
				} else {
					for key, value := range want {
						t.Setenv(key, value)
					}
				}
				out, err := gatewayRun(t, args...)
				require.NoError(t, err)
				gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
				gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, "https://gateway.smith.langchain.com")
			})
		}
	}
}

func TestGatewaySetupModelsLocalOverridesLowerSettings(t *testing.T) {
	for _, lower := range []string{"user", "shared"} {
		for _, source := range []string{"flag", "environment"} {
			t.Run(lower+"/"+source, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				if lower == "shared" {
					settings = filepath.Join(".claude", "settings.json")
				}
				gatewayModelTestSave(t, settings, gatewayModelTestValues("lower"))
				before, err := os.ReadFile(settings)
				require.NoError(t, err)
				want := gatewayModelTestValues("requested")
				args := []string{"--yes", "--scope=project"}
				if source == "flag" {
					args = append(args, gatewayModelTestFlags(want)...)
				} else {
					for key, value := range want {
						t.Setenv(key, value)
					}
				}
				out, err := gatewayRun(t, args...)
				require.NoError(t, err)
				gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
				gatewayModelTestAssertEnv(t, readJSONFile(t, filepath.Join(".claude", "settings.local.json")), want, "https://gateway.smith.langchain.com")
				after, err := os.ReadFile(settings)
				require.NoError(t, err)
				require.Equal(t, before, after, "lower settings must not be rewritten")
			})
		}
	}
}

func TestGatewaySetupModelsClearCrossScopeEffectivePrecedence(t *testing.T) {
	// Deleting the target key is not an empty override: lower values resurface.
	// Conversely, an empty value in a retained higher scope masks lower values.
	cases := []struct {
		name, scope         string
		user, shared, local *string
		conflict            bool
	}{
		{name: "user-clears-own", scope: "user", user: gatewayModelTestString("target/model")},
		{name: "user-shared-conflict", scope: "user", user: gatewayModelTestString("target/model"), shared: gatewayModelTestString("private-other/model"), conflict: true},
		{name: "user-local-conflict", scope: "user", user: gatewayModelTestString("target/model"), local: gatewayModelTestString("private-other/model"), conflict: true},
		{name: "user-local-empty-masks-shared", scope: "user", user: gatewayModelTestString("target/model"), shared: gatewayModelTestString("private-other/model"), local: gatewayModelTestString("")},
		{name: "user-shared-empty-does-not-mask-local", scope: "user", shared: gatewayModelTestString(""), local: gatewayModelTestString("private-other/model"), conflict: true},
		{name: "user-absent-target-other-complete", scope: "user", local: gatewayModelTestString("private-other/model"), conflict: true},
		{name: "project-clears-own", scope: "project", local: gatewayModelTestString("target/model")},
		{name: "project-inherits-user", scope: "project", user: gatewayModelTestString("private-other/model"), local: gatewayModelTestString("target/model"), conflict: true},
		{name: "project-inherits-shared", scope: "project", shared: gatewayModelTestString("private-other/model"), local: gatewayModelTestString("target/model"), conflict: true},
		{name: "project-target-empty-must-be-deleted", scope: "project", user: gatewayModelTestString("private-other/model"), local: gatewayModelTestString(""), conflict: true},
		{name: "project-shared-empty-masks-user", scope: "project", user: gatewayModelTestString("private-other/model"), shared: gatewayModelTestString(""), local: gatewayModelTestString("target/model")},
		{name: "project-empty-user-does-not-mask-shared", scope: "project", user: gatewayModelTestString(""), shared: gatewayModelTestString("private-other/model"), conflict: true},
		{name: "project-absent-target-inherits", scope: "project", user: gatewayModelTestString("private-other/model"), conflict: true},
	}
	for _, tc := range cases {
		for _, family := range gatewayModelTestFamilies {
			for _, mode := range []string{"--yes", "--dry-run", "both"} {
				t.Run(tc.name+"/"+family.flag+"/"+mode, func(t *testing.T) {
					user, cfg := gatewayTestEnv(t)
					shared := filepath.Join(".claude", "settings.json")
					local := filepath.Join(".claude", "settings.local.json")
					target := user
					if tc.scope == "project" {
						target = local
					}
					for path, value := range map[string]*string{user: tc.user, shared: tc.shared, local: tc.local} {
						if value != nil {
							env := map[string]string{family.env: *value, "UNRELATED": "keep-private"}
							if tc.name == "user-absent-target-other-complete" {
								env = gatewayModelTestValues("private-other")
							}
							gatewayModelTestSave(t, path, env)
						}
					}
					// All non-target files (including OAuth credentials) stay byte-for-byte intact.
					unchanged := map[string][]byte{}
					for _, path := range []string{user, shared, local, cfg} {
						if path == target {
							continue
						}
						data, err := os.ReadFile(path)
						if os.IsNotExist(err) {
							unchanged[path] = nil
						} else {
							require.NoError(t, err)
							unchanged[path] = data
						}
					}
					if tc.conflict || mode != "--yes" {
						defer gatewayModelTestNoWrites(t)()
					}
					args := []string{mode, "--scope=" + tc.scope}
					if mode == "both" {
						args = []string{"--yes", "--dry-run", "--scope=" + tc.scope}
					}
					out, err := gatewayRun(t, args...)
					if tc.conflict {
						require.Error(t, err)
						if tc.name != "user-absent-target-other-complete" {
							require.Contains(t, err.Error(), family.env)
						}
						require.Contains(t, err.Error(), "scope", "error must explain the cross-scope conflict")
						require.NotContains(t, err.Error(), "private-other")
					} else {
						require.NoError(t, err)
						report := gatewayModelTestReport(t, out)
						gatewayModelTestAssertEnv(t, report, nil, "https://gateway.smith.langchain.com/anthropic")
						require.Equal(t, []any{"env." + family.env}, report["removed_keys"])
						if mode == "--yes" {
							gatewayModelTestAssertEnv(t, readJSONFile(t, target), nil, "https://gateway.smith.langchain.com/anthropic")
						}
					}
					require.NotContains(t, out, "private-other")
					for path, before := range unchanged {
						after, err := os.ReadFile(path)
						if before == nil {
							require.True(t, os.IsNotExist(err), "must not create %s", path)
						} else {
							require.NoError(t, err)
							require.Equal(t, before, after, path)
							assertPerm0600(t, path)
						}
					}
				})
			}
		}
	}
}

func gatewayModelTestString(value string) *string { return &value }

func TestGatewaySetupModelsValidFlagsMaskInvalidShell(t *testing.T) {
	settings, _ := gatewayTestEnv(t)
	gatewayModelTestSave(t, settings, gatewayModelTestValues("saved"))
	for _, family := range gatewayModelTestFamilies {
		t.Setenv(family.env, "not a valid slug")
	}
	want := gatewayModelTestValues("flag")
	args := append([]string{"--yes"}, gatewayModelTestFlags(want)...)
	out, err := gatewayRun(t, args...)
	require.NoError(t, err)
	gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
	gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, "https://gateway.smith.langchain.com")
}
