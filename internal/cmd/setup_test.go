package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// hermeticSetupEnv points the agent config dirs and LangSmith credential
// resolution at a throwaway HOME so setup tests never touch the real machine.
func hermeticSetupEnv(t *testing.T) (claudeDir, codexDir string) {
	t.Helper()
	home := t.TempDir()
	claudeDir = filepath.Join(home, ".claude")
	codexDir = filepath.Join(home, ".codex")
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("CODEX_HOME", codexDir)
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(home, "ls-config.json"))
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
	t.Setenv("LANGSMITH_PROJECT", "")
	t.Setenv("LANGSMITH_AGENT_PROJECT", "")
	t.Setenv("LANGSMITH_WORKSPACE_ID", "")
	t.Setenv("LANGSMITH_TENANT_ID", "")
	return claudeDir, codexDir
}

// stubRunSetupCommand records install shell-outs instead of executing them.
func stubRunSetupCommand(t *testing.T) *[][]string {
	t.Helper()
	old := runSetupCommand
	var calls [][]string
	runSetupCommand = func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	t.Cleanup(func() { runSetupCommand = old })
	return &calls
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing %s: %v\n%s", path, err, data)
	}
	return doc
}

func assertPerm0600(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected %s to be 0600, got %o", path, perm)
	}
}

func TestSetupClaudeWritesSettings(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// Pre-existing settings with an unrelated top-level key and env key that
	// must survive the merge.
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"model":"opus","env":{"FOO":"bar"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t,
		"--format=json",
		"setup", "claude",
		"--api-key", "test-key-abc",
		"--project", "demo",
		"--no-install",
	)
	if err != nil {
		t.Fatalf("setup claude error: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "test-key-abc") {
		t.Fatalf("setup output leaked the api key: %s", stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if result["status"] != "configured" || result["agent"] != "claude-code" || result["project"] != "demo" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result["plugin"] != "langsmith-tracing@langsmith-claude-code-plugins" {
		t.Fatalf("unexpected plugin ref: %+v", result["plugin"])
	}
	if _, ok := result["endpoint"]; ok {
		t.Fatalf("endpoint should be omitted for the default SaaS URL: %+v", result)
	}

	doc := readJSONFile(t, settingsPath)
	if doc["model"] != "opus" {
		t.Fatalf("unrelated key 'model' was not preserved: %+v", doc)
	}
	markets, _ := doc["extraKnownMarketplaces"].(map[string]any)
	mk, _ := markets["langsmith-claude-code-plugins"].(map[string]any)
	src, _ := mk["source"].(map[string]any)
	if src["source"] != "github" || src["repo"] != "langchain-ai/langsmith-claude-code-plugins" {
		t.Fatalf("marketplace not written correctly: %+v", markets)
	}
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	if enabled["langsmith-tracing@langsmith-claude-code-plugins"] != true {
		t.Fatalf("plugin not enabled: %+v", enabled)
	}
	env, _ := doc["env"].(map[string]any)
	if env["FOO"] != "bar" {
		t.Fatalf("unrelated env key was not preserved: %+v", env)
	}
	if env["TRACE_TO_LANGSMITH"] != "true" || env["CC_LANGSMITH_API_KEY"] != "test-key-abc" ||
		env["CC_LANGSMITH_PROJECT"] != "demo" || env["LANGSMITH_API_KEY"] != "test-key-abc" ||
		env["LANGSMITH_PROJECT"] != "demo" {
		t.Fatalf("tracing env not written correctly: %+v", env)
	}
	if _, ok := env["LANGSMITH_ENDPOINT"]; ok {
		t.Fatalf("endpoint env should be omitted for default URL: %+v", env)
	}
	assertPerm0600(t, settingsPath)
}

func TestSetupClaudeInstallShellOut(t *testing.T) {
	hermeticSetupEnv(t)
	calls := stubRunSetupCommand(t)

	if _, err := executeCommand(t,
		"setup", "claude", "--api-key", "k", "--project", "p",
	); err != nil {
		t.Fatalf("setup claude error: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("expected one install shell-out, got %d: %+v", len(*calls), *calls)
	}
	got := (*calls)[0]
	want := []string{"claude", "plugin", "marketplace", "add", claudeMarketplaceURL}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("unexpected install command\n got: %v\nwant: %v", got, want)
	}
}

func TestSetupClaudeSelfHostedEndpoint(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	if _, err := executeCommand(t,
		"setup", "claude",
		"--api-key", "k", "--project", "p", "--no-install",
		"--api-url", "https://ls.internal.example.com/api/v1",
	); err != nil {
		t.Fatalf("setup claude error: %v", err)
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	if env["LANGSMITH_ENDPOINT"] != "https://ls.internal.example.com" {
		t.Fatalf("expected normalized self-hosted endpoint, got %+v", env["LANGSMITH_ENDPOINT"])
	}
}

func TestSetupClaudeRequiresAPIKey(t *testing.T) {
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	_, err := executeCommand(t, "setup", "claude", "--no-install")
	if err == nil {
		t.Fatal("expected an error when no API key is available")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API-key error, got: %v", err)
	}
}

func TestSetupClaudeIdempotent(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	for i := 0; i < 2; i++ {
		if _, err := executeCommand(t,
			"setup", "claude", "--api-key", "k", "--project", "p", "--no-install",
		); err != nil {
			t.Fatalf("run %d error: %v", i, err)
		}
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	if len(enabled) != 1 || enabled["langsmith-tracing@langsmith-claude-code-plugins"] != true {
		t.Fatalf("re-run should not duplicate enabled plugins: %+v", enabled)
	}
}

func TestSetupCodexWritesConfig(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// Pre-existing config.toml with an unrelated table that must survive.
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[model]\nname = \"gpt-5\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t,
		"--format=json",
		"setup", "codex",
		"--api-key", "test-key-xyz",
		"--project", "demo",
		"--api-url", "https://ls.internal.example.com",
		"--no-install",
	)
	if err != nil {
		t.Fatalf("setup codex error: %v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "test-key-xyz") {
		t.Fatalf("setup output leaked the api key: %s", stdout)
	}

	// Credentials file.
	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if cred["enabled"] != true || cred["api_key"] != "test-key-xyz" ||
		cred["project"] != "demo" || cred["api_url"] != "https://ls.internal.example.com" {
		t.Fatalf("langsmith.json not written correctly: %+v", cred)
	}
	assertPerm0600(t, filepath.Join(codexDir, "langsmith.json"))

	// config.toml round-trips and preserves the unrelated table.
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	var conf map[string]any
	if err := toml.Unmarshal(data, &conf); err != nil {
		t.Fatalf("config.toml is not valid TOML: %v\n%s", err, data)
	}
	model, _ := conf["model"].(map[string]any)
	if model["name"] != "gpt-5" {
		t.Fatalf("unrelated [model] table not preserved: %+v", conf)
	}
	features, _ := conf["features"].(map[string]any)
	if features["plugin_hooks"] != true {
		t.Fatalf("plugin_hooks not enabled: %+v", features)
	}
	plugins, _ := conf["plugins"].(map[string]any)
	entry, _ := plugins["tracing@langsmith-codex-plugins"].(map[string]any)
	if entry["enabled"] != true {
		t.Fatalf("codex plugin not enabled: %+v", plugins)
	}
	assertPerm0600(t, tomlPath)
}

func TestSetupCodexInstallShellOut(t *testing.T) {
	hermeticSetupEnv(t)
	calls := stubRunSetupCommand(t)

	if _, err := executeCommand(t,
		"setup", "codex", "--api-key", "k", "--project", "p",
	); err != nil {
		t.Fatalf("setup codex error: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("expected one install shell-out, got %d: %+v", len(*calls), *calls)
	}
	got := strings.Join((*calls)[0], " ")
	want := "codex plugin marketplace add " + codexMarketplaceURL
	if got != want {
		t.Fatalf("unexpected install command\n got: %s\nwant: %s", got, want)
	}
}
