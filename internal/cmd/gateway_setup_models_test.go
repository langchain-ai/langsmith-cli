package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func TestGatewaySetupModelsFillMissingFamiliesFromSaved(t *testing.T) {
	for i, family := range gatewayModelTestFamilies {
		t.Run(family.flag, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			want := gatewayModelTestValues("saved")
			gatewayModelTestSave(t, settings, want)
			// One flag, a different family from env, and two from saved settings.
			other := gatewayModelTestFamilies[(i+1)%len(gatewayModelTestFamilies)]
			want[family.env] = "flag/replacement"
			want[other.env] = "shell/replacement"
			t.Setenv(other.env, want[other.env])
			out, err := gatewayRun(t, "--yes", family.flag+"="+want[family.env])
			require.NoError(t, err)
			gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
			gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, "https://gateway.smith.langchain.com")
		})
	}
}

func TestGatewaySetupModelsSavedCompleteRerunAndPartialUpdate(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			if scope == "project" {
				settings = filepath.Join(".claude", "settings.local.json")
			}
			want := gatewayModelTestValues("saved")
			gatewayModelTestSave(t, settings, want)
			args := []string{"--yes", "--scope=" + scope}
			_, err := gatewayRun(t, args...)
			require.NoError(t, err)
			gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, "https://gateway.smith.langchain.com")

			assertIdempotent := func() {
				t.Helper()
				before, err := os.ReadFile(settings)
				require.NoError(t, err)
				out, err := gatewayRun(t, args...)
				require.NoError(t, err)
				after, err := os.ReadFile(settings)
				require.NoError(t, err)
				require.Equal(t, before, after, "rerun without flags must retain model routing")
				report := gatewayModelTestReport(t, out)
				require.Empty(t, report["replaced_keys"])
				gatewayModelTestAssertEnv(t, report, want, "https://gateway.smith.langchain.com")
			}
			assertIdempotent()
			for _, family := range gatewayModelTestFamilies {
				want[family.env] = "updated/" + family.env
				out, err := gatewayRun(t, append(args, family.flag+"="+want[family.env])...)
				require.NoError(t, err)
				require.Equal(t, []any{"env." + family.env}, gatewayModelTestReport(t, out)["replaced_keys"])
				gatewayModelTestAssertEnv(t, readJSONFile(t, settings), want, "https://gateway.smith.langchain.com")
				assertIdempotent()
			}
		})
	}
}

func TestGatewaySetupModelsEffectiveSavedSettings(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			settings, _ := gatewayTestEnv(t)
			families := gatewayModelTestFamilies
			// Resolve each family separately, with user < shared < local. No
			// single settings file supplies the complete effective set.
			gatewayModelTestSave(t, settings, map[string]string{
				families[0].env: "user/haiku", families[1].env: "user/sonnet", families[2].env: "user/opus",
			})
			shared := filepath.Join(".claude", "settings.json")
			local := filepath.Join(".claude", "settings.local.json")
			gatewayModelTestSave(t, shared, map[string]string{families[1].env: "shared/sonnet", families[2].env: "shared/opus", families[3].env: "shared/fable"})
			gatewayModelTestSave(t, local, map[string]string{families[2].env: "local/opus", families[3].env: "local/fable"})
			want := map[string]string{families[0].env: "user/haiku", families[1].env: "shared/sonnet", families[2].env: "local/opus", families[3].env: "local/fable"}
			target := settings
			if scope == "project" {
				target = local
			}
			unchanged := map[string][]byte{}
			for _, path := range []string{settings, shared, local} {
				if path != target {
					data, err := os.ReadFile(path)
					require.NoError(t, err)
					unchanged[path] = data
				}
			}
			out, err := gatewayRun(t, "--yes", "--scope="+scope)
			require.NoError(t, err)
			gatewayModelTestAssertEnv(t, gatewayModelTestReport(t, out), want, "https://gateway.smith.langchain.com")
			gatewayModelTestAssertEnv(t, readJSONFile(t, target), want, "https://gateway.smith.langchain.com")
			for path, before := range unchanged {
				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before, after, path)
			}
		})
	}
}

func TestGatewaySetupModelsPartialSubsetsRejectedWithoutWrites(t *testing.T) {
	// All 14 nonempty proper subsets, not just the original three families:
	// fable-only and haiku+sonnet+opus without fable must both be rejected.
	for mask := 1; mask < (1<<len(gatewayModelTestFamilies))-1; mask++ {
		for _, source := range []string{"flags", "environment", "user", "shared", "local", "mixed"} {
			for _, mode := range []string{"--yes", "--dry-run", "both"} {
				t.Run(fmt.Sprintf("%04b/%s/%s", mask, source, mode), func(t *testing.T) {
					settings, _ := gatewayTestEnv(t)
					args := []string{mode}
					if mode == "both" {
						args = []string{"--dry-run", "--yes"}
					}
					saved := map[string]string{}
					for i, family := range gatewayModelTestFamilies {
						if mask&(1<<i) == 0 {
							continue
						}
						value := fmt.Sprintf("provider/model-%d", i)
						switch {
						case source == "flags" || source == "mixed" && i%3 == 0:
							args = append(args, family.flag+"="+value)
						case source == "environment" || source == "mixed" && i%3 == 1:
							t.Setenv(family.env, value)
						default:
							saved[family.env] = value
						}
					}
					if len(saved) != 0 {
						path := settings
						if source == "shared" {
							path = filepath.Join(".claude", "settings.json")
						} else if source == "local" {
							path = filepath.Join(".claude", "settings.local.json")
						}
						gatewayModelTestSave(t, path, saved)
					}
					defer gatewayModelTestNoWrites(t)()
					_, err := gatewayRun(t, args...)
					require.Error(t, err)
					require.NotContains(t, err.Error(), "unknown flag")
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

func TestGatewaySetupModelsExplicitEmptyFlagRejected(t *testing.T) {
	for _, family := range gatewayModelTestFamilies {
		for _, mode := range []string{"--yes", "--dry-run"} {
			t.Run(family.flag+"/"+mode, func(t *testing.T) {
				settings, _ := gatewayTestEnv(t)
				values := gatewayModelTestValues("saved")
				gatewayModelTestSave(t, settings, values)
				// An explicitly empty flag is invalid, not a request to fall back.
				t.Setenv(family.env, "shell/valid")
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
		for _, source := range []string{"flag", "environment", "saved"} {
			for _, tc := range invalid {
				// OS environments cannot represent NUL; flags and saved JSON can.
				if source == "environment" && tc.name == "nul" {
					continue
				}
				t.Run(family.flag+"/"+source+"/"+tc.name, func(t *testing.T) {
					settings, _ := gatewayTestEnv(t)
					values := gatewayModelTestValues("valid")
					args := []string{"--yes"}
					switch source {
					case "flag":
						args = append(args, family.flag+"="+tc.value)
					case "environment":
						t.Setenv(family.env, tc.value)
					case "saved":
						values[family.env] = tc.value
					}
					gatewayModelTestSave(t, settings, values)
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
						if source == "flag" {
							args = append(args, family.flag+"=requested/model")
						} else {
							t.Setenv(family.env, "requested/model")
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
					args = append(args, family.flag+"="+want[family.env])
				} else {
					t.Setenv(family.env, want[family.env])
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
