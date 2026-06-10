package cmd

import (
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

func claudeEnv(t *testing.T, claudeDir string) map[string]any {
	t.Helper()
	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	return env
}

func TestSetupClaudeWritesSettings(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)

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
		env["CC_LANGSMITH_PROJECT"] != "demo" {
		t.Fatalf("tracing env not written correctly: %+v", env)
	}
	for _, key := range []string{"LANGSMITH_API_KEY", "LANGSMITH_PROJECT", "LANGSMITH_ENDPOINT", "CC_LANGSMITH_RUNS_ENDPOINTS"} {
		if _, ok := env[key]; ok {
			t.Fatalf("env key %s should not be written: %+v", key, env)
		}
	}
	assertPerm0600(t, settingsPath)
}

func TestSetupClaudePositionalKeyDefaults(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)

	stdout, err := executeCommand(t, "--format=json", "setup", "claude", "test-key-abc")
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
	if _, ok := result["project"]; ok {
		t.Fatalf("project should be omitted when not configured (plugin default applies): %+v", result)
	}

	env := claudeEnv(t, claudeDir)
	if env["TRACE_TO_LANGSMITH"] != "true" || env["CC_LANGSMITH_API_KEY"] != "test-key-abc" {
		t.Fatalf("tracing env not written correctly: %+v", env)
	}
	if _, ok := env["CC_LANGSMITH_PROJECT"]; ok {
		t.Fatalf("CC_LANGSMITH_PROJECT should be omitted so the plugin default applies: %+v", env)
	}
	if _, ok := env["CC_LANGSMITH_RUNS_ENDPOINTS"]; ok {
		t.Fatalf("runs endpoints should be omitted for the default SaaS URL: %+v", env)
	}
}

func TestSetupClaudeKeyConflict(t *testing.T) {
	hermeticSetupEnv(t)

	_, err := executeCommand(t, "setup", "claude", "key-one", "--api-key", "key-two")
	if err == nil {
		t.Fatal("expected an error when the key is passed both positionally and via --api-key")
	}
	if !strings.Contains(err.Error(), "--api-key") {
		t.Fatalf("expected conflicting-key error, got: %v", err)
	}
}

func TestSetupClaudeSelfHostedEndpoint(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)

	if _, err := executeCommand(t,
		"setup", "claude",
		"--api-key", "k", "--project", "p",
		"--api-url", "https://ls.internal.example.com/api/v1",
	); err != nil {
		t.Fatalf("setup claude error: %v", err)
	}

	env := claudeEnv(t, claudeDir)
	raw, _ := env["CC_LANGSMITH_RUNS_ENDPOINTS"].(string)
	if raw == "" {
		t.Fatalf("expected CC_LANGSMITH_RUNS_ENDPOINTS for a self-hosted URL: %+v", env)
	}
	var replicas []map[string]any
	if err := json.Unmarshal([]byte(raw), &replicas); err != nil {
		t.Fatalf("runs endpoints not valid JSON: %v\n%s", err, raw)
	}
	if len(replicas) != 1 {
		t.Fatalf("expected one replica, got %+v", replicas)
	}
	if replicas[0]["apiUrl"] != "https://ls.internal.example.com" || replicas[0]["apiKey"] != "k" ||
		replicas[0]["projectName"] != "p" {
		t.Fatalf("unexpected replica: %+v", replicas[0])
	}
}

func TestSetupClaudeSelfHostedEndpointNoProject(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)

	if _, err := executeCommand(t,
		"setup", "claude", "k",
		"--api-url", "https://ls.internal.example.com",
	); err != nil {
		t.Fatalf("setup claude error: %v", err)
	}

	env := claudeEnv(t, claudeDir)
	raw, _ := env["CC_LANGSMITH_RUNS_ENDPOINTS"].(string)
	var replicas []map[string]any
	if err := json.Unmarshal([]byte(raw), &replicas); err != nil {
		t.Fatalf("runs endpoints not valid JSON: %v\n%s", err, raw)
	}
	if len(replicas) != 1 {
		t.Fatalf("expected one replica, got %+v", replicas)
	}
	if _, ok := replicas[0]["projectName"]; ok {
		t.Fatalf("projectName should be omitted when no project is configured: %+v", replicas[0])
	}
}

func TestSetupClaudeRequiresAPIKey(t *testing.T) {
	hermeticSetupEnv(t)

	_, err := executeCommand(t, "setup", "claude")
	if err == nil {
		t.Fatal("expected an error when no API key is available")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected API-key error, got: %v", err)
	}
}

func TestSetupClaudeIdempotentAndDropsStaleProject(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)

	if _, err := executeCommand(t, "setup", "claude", "k", "--project", "p"); err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if env := claudeEnv(t, claudeDir); env["CC_LANGSMITH_PROJECT"] != "p" {
		t.Fatalf("first run did not write the project: %+v", env)
	}

	// Re-running without --project removes the stale value so the plugin
	// default applies again.
	if _, err := executeCommand(t, "setup", "claude", "k"); err != nil {
		t.Fatalf("second run error: %v", err)
	}
	env := claudeEnv(t, claudeDir)
	if _, ok := env["CC_LANGSMITH_PROJECT"]; ok {
		t.Fatalf("re-run without --project should remove CC_LANGSMITH_PROJECT: %+v", env)
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	enabled, _ := doc["enabledPlugins"].(map[string]any)
	if len(enabled) != 1 || enabled["langsmith-tracing@langsmith-claude-code-plugins"] != true {
		t.Fatalf("re-run should not duplicate enabled plugins: %+v", enabled)
	}
}

func TestSetupCodexWritesConfig(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)

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

func TestSetupCodexPositionalKeyDefaults(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)

	stdout, err := executeCommand(t, "--format=json", "setup", "codex", "test-key-xyz")
	if err != nil {
		t.Fatalf("setup codex error: %v\n%s", err, stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, stdout)
	}
	if _, ok := result["project"]; ok {
		t.Fatalf("project should be omitted when not configured (plugin default applies): %+v", result)
	}

	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if cred["enabled"] != true || cred["api_key"] != "test-key-xyz" {
		t.Fatalf("langsmith.json not written correctly: %+v", cred)
	}
	for _, key := range []string{"project", "api_url"} {
		if _, ok := cred[key]; ok {
			t.Fatalf("%s should be omitted so the plugin default applies: %+v", key, cred)
		}
	}
}

func TestSetupAllPositionalKey(t *testing.T) {
	claudeDir, codexDir := hermeticSetupEnv(t)

	if _, err := executeCommand(t, "setup", "all", "shared-key"); err != nil {
		t.Fatalf("setup all error: %v", err)
	}

	if env := claudeEnv(t, claudeDir); env["CC_LANGSMITH_API_KEY"] != "shared-key" {
		t.Fatalf("claude settings not written by setup all: %+v", env)
	}
	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if cred["api_key"] != "shared-key" {
		t.Fatalf("codex credentials not written by setup all: %+v", cred)
	}
	if _, err := os.Stat(filepath.Join(codexDir, "config.toml")); err != nil {
		t.Fatalf("codex config.toml not written by setup all: %v", err)
	}
}
