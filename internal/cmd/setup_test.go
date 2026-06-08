package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// stubRunSetupCommand records plugin-install shell-outs instead of executing
// them, so tests never spawn the real claude/codex binaries.
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

func TestSetupClaudeWritesSettings(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	calls := stubRunSetupCommand(t)

	// Pre-existing settings with an unrelated top-level key and env key that
	// must survive the merge.
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	// Pre-create world-readable to prove setup tightens it to 0600.
	if err := os.WriteFile(settingsPath, []byte(`{"model":"opus","env":{"FOO":"bar"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t,
		"--format=json",
		"trace", "setup", "claude",
		"--api-key", "test-key-abc",
		"--project", "demo",
		"--user", "Jane Doe",
		"--email", "jane@example.com",
		"--yes",
	)
	if err != nil {
		t.Fatalf("claude setup error: %v\n%s", err, stdout)
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
	// The generic LANGSMITH_* vars must NOT be written — they would leak into
	// every session and any LangSmith SDK code running inside it.
	for _, k := range []string{"LANGSMITH_API_KEY", "LANGSMITH_PROJECT", "LANGSMITH_ENDPOINT"} {
		if _, ok := env[k]; ok {
			t.Fatalf("generic %s should not be written to the env block: %+v", k, env)
		}
	}
	if _, ok := env["CC_LANGSMITH_RUNS_ENDPOINTS"]; ok {
		t.Fatalf("runs endpoints should be omitted for the default SaaS URL: %+v", env)
	}

	// User identity is attached as run metadata (JSON-encoded string).
	rawMeta, ok := env["CC_LANGSMITH_METADATA"].(string)
	if !ok {
		t.Fatalf("expected CC_LANGSMITH_METADATA in env: %+v", env)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		t.Fatalf("metadata is not valid JSON: %v\n%s", err, rawMeta)
	}
	if meta["user_name"] != "Jane Doe" || meta["user_email"] != "jane@example.com" {
		t.Fatalf("unexpected user metadata: %+v", meta)
	}
	assertPerm0600(t, settingsPath)

	// The plugin install shelled out: marketplace add + install.
	if len(*calls) != 2 {
		t.Fatalf("expected 2 install commands, got %d: %+v", len(*calls), *calls)
	}
	if got := strings.Join((*calls)[0], " "); got != "claude plugin marketplace add "+claudeMarketplaceURL {
		t.Fatalf("unexpected marketplace-add command: %s", got)
	}
	if got := strings.Join((*calls)[1], " "); got != "claude plugin install langsmith-tracing@langsmith-claude-code-plugins --scope user" {
		t.Fatalf("unexpected install command: %s", got)
	}
}

func TestSetupClaudeSelfHostedEndpoint(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	if _, err := executeCommand(t,
		"trace", "setup", "claude",
		"--api-key", "k", "--project", "p", "--yes",
		"--api-url", "https://ls.internal.example.com/api/v1",
	); err != nil {
		t.Fatalf("claude setup error: %v", err)
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	raw, ok := env["CC_LANGSMITH_RUNS_ENDPOINTS"].(string)
	if !ok {
		t.Fatalf("expected CC_LANGSMITH_RUNS_ENDPOINTS for a self-hosted endpoint: %+v", env)
	}
	var replicas []map[string]any
	if err := json.Unmarshal([]byte(raw), &replicas); err != nil {
		t.Fatalf("runs endpoints is not a JSON array: %v\n%s", err, raw)
	}
	if len(replicas) != 1 || replicas[0]["apiUrl"] != "https://ls.internal.example.com" ||
		replicas[0]["apiKey"] != "k" || replicas[0]["projectName"] != "p" {
		t.Fatalf("unexpected replica: %+v", replicas)
	}
}

func TestSetupClaudeRequiresAPIKey(t *testing.T) {
	hermeticSetupEnv(t)

	_, err := executeCommand(t, "trace", "setup", "claude")
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
			"trace", "setup", "claude", "--api-key", "k", "--project", "p", "--yes",
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
	calls := stubRunSetupCommand(t)

	// Pre-existing config.toml with an unrelated table, plus a world-readable
	// langsmith.json — both must end up 0600 after setup.
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[model]\nname = \"gpt-5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "langsmith.json"), []byte(`{"foo":"bar"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, err := executeCommand(t,
		"--format=json",
		"trace", "setup", "codex",
		"--api-key", "test-key-xyz",
		"--project", "demo",
		"--api-url", "https://ls.internal.example.com",
		"--user", "Jane Doe",
		"--email", "jane@example.com",
		"--yes",
	)
	if err != nil {
		t.Fatalf("codex setup error: %v\n%s", err, stdout)
	}

	// Credentials file.
	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if cred["enabled"] != true || cred["api_key"] != "test-key-xyz" ||
		cred["project"] != "demo" || cred["api_url"] != "https://ls.internal.example.com" {
		t.Fatalf("langsmith.json not written correctly: %+v", cred)
	}
	meta, _ := cred["metadata"].(map[string]any)
	if meta["user_name"] != "Jane Doe" || meta["user_email"] != "jane@example.com" {
		t.Fatalf("unexpected user metadata: %+v", cred["metadata"])
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

	// The marketplace-add shelled out.
	if len(*calls) != 1 {
		t.Fatalf("expected 1 install command, got %d: %+v", len(*calls), *calls)
	}
	if got := strings.Join((*calls)[0], " "); got != "codex plugin marketplace add "+codexMarketplaceURL {
		t.Fatalf("unexpected marketplace-add command: %s", got)
	}
}

func TestSetupCodexDefaultEndpointOmitsURL(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	if _, err := executeCommand(t,
		"trace", "setup", "codex", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("codex setup error: %v", err)
	}

	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if _, ok := cred["api_url"]; ok {
		t.Fatalf("api_url should be omitted for the default SaaS endpoint: %+v", cred)
	}
}

func TestSetupClaudeAbortsWithoutConfirmation(t *testing.T) {
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// Force the non-interactive branch deterministically (independent of the
	// test runner's stdin), so this never hangs on Scanln.
	oldTerm := inputIsTerminal
	inputIsTerminal = func(io.Reader) bool { return false }
	t.Cleanup(func() { inputIsTerminal = oldTerm })

	// Non-interactive shell without --yes must refuse rather than apply silently.
	_, err := executeCommand(t, "trace", "setup", "claude", "--api-key", "k", "--project", "p")
	if err == nil {
		t.Fatal("expected an error when not confirmed and no --yes")
	}
	if !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("expected a confirmation error, got: %v", err)
	}
}

func TestSetupPositionalArgs(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// `claude setup KEY URL PROJECT` applies positional args; a bare host gains
	// an https:// scheme.
	if _, err := executeCommand(t,
		"trace", "setup", "claude", "demo-key-pos", "dev.smith.com", "posproj", "--yes",
	); err != nil {
		t.Fatalf("setup positional error: %v", err)
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	if env["CC_LANGSMITH_API_KEY"] != "demo-key-pos" || env["CC_LANGSMITH_PROJECT"] != "posproj" {
		t.Fatalf("positional key/project not applied: %+v", env)
	}
	raw, _ := env["CC_LANGSMITH_RUNS_ENDPOINTS"].(string)
	var replicas []map[string]any
	if err := json.Unmarshal([]byte(raw), &replicas); err != nil || len(replicas) != 1 {
		t.Fatalf("expected one replica from positional URL, got %q (%v)", raw, err)
	}
	if replicas[0]["apiUrl"] != "https://dev.smith.com" {
		t.Fatalf("bare host should be scheme-qualified, got %v", replicas[0]["apiUrl"])
	}
}

func TestSetupPositionalAPIKeySkipsOAuthRefresh(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	refreshRequests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshRequests++
		http.Error(w, "refresh should not be attempted", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	if err := os.WriteFile(configPath, []byte(`{
  "current_profile": "dev",
  "profiles": {
    "dev": {
      "api_url": "`+ts.URL+`",
      "oauth": {
        "refresh_token": "old-refresh-token"
      }
    }
  }
}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(t,
		"trace", "setup", "claude", "positional-key", "dev.smith.com", "posproj", "--yes", "--no-install",
	); err != nil {
		t.Fatalf("setup positional should ignore OAuth refresh failure: %v", err)
	}
	if refreshRequests != 0 {
		t.Fatalf("setup should not refresh OAuth profiles, got %d refresh requests", refreshRequests)
	}

	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	if env["CC_LANGSMITH_API_KEY"] != "positional-key" || env["CC_LANGSMITH_PROJECT"] != "posproj" {
		t.Fatalf("positional setup did not apply API key/project: %+v", env)
	}
}

func TestSetupPositionalConflictsWithFlag(t *testing.T) {
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	_, err := executeCommand(t,
		"trace", "setup", "claude", "demo-key-pos", "--api-key", "other-key", "--yes",
	)
	if err == nil {
		t.Fatal("expected an error when API key is given both positionally and via --api-key")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Fatalf("expected a conflict error, got: %v", err)
	}
}

func TestSetupCodexClearsStaleEndpoint(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Pre-existing langsmith.json from an earlier self-hosted setup.
	if err := os.WriteFile(filepath.Join(codexDir, "langsmith.json"),
		[]byte(`{"enabled":true,"api_url":"https://old-self-hosted.example.com","project":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Re-run against the default SaaS endpoint (no --api-url).
	if _, err := executeCommand(t,
		"trace", "setup", "codex", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("codex setup error: %v", err)
	}

	cred := readJSONFile(t, filepath.Join(codexDir, "langsmith.json"))
	if _, ok := cred["api_url"]; ok {
		t.Fatalf("stale api_url should be removed when re-run on the default endpoint: %+v", cred)
	}
}

func TestSetupClaudeRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on Windows")
	}
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	proj := t.TempDir()
	t.Chdir(proj)

	// A malicious repo plants a symlink at the project config path pointing at a
	// sensitive file; setup must refuse rather than overwrite it with the key.
	victim := filepath.Join(proj, "victim.txt")
	if err := os.WriteFile(victim, []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proj, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(proj, ".claude", "settings.local.json")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err := executeCommand(t,
		"trace", "setup", "claude", "--scope", "project",
		"--api-key", "k", "--project", "p", "--yes", "--no-install",
	)
	if err == nil {
		t.Fatal("expected setup to refuse writing through a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink error, got: %v", err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "SECRET" {
		t.Fatalf("victim file was overwritten through the symlink: %q", b)
	}
}

func TestSetupClaudeRejectsSymlinkedProjectDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on Windows")
	}
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	proj := t.TempDir()
	t.Chdir(proj)

	if err := os.Symlink(".", filepath.Join(proj, ".claude")); err != nil {
		t.Fatalf("creating directory symlink: %v", err)
	}

	_, err := executeCommand(t,
		"trace", "setup", "claude", "--scope", "project",
		"--api-key", "k", "--project", "p", "--yes", "--no-install",
	)
	if err == nil {
		t.Fatal("expected setup to refuse a symlinked project config directory")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink error, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(proj, "settings.local.json")); !os.IsNotExist(statErr) {
		t.Fatalf("settings.local.json should not be written through .claude symlink: %v", statErr)
	}
}

func TestSetupCodexRejectsSymlinkedProjectDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on Windows")
	}
	hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	proj := t.TempDir()
	t.Chdir(proj)

	if err := os.Symlink(".", filepath.Join(proj, ".codex")); err != nil {
		t.Fatalf("creating directory symlink: %v", err)
	}

	_, err := executeCommand(t,
		"trace", "setup", "codex", "--scope", "project",
		"--api-key", "k", "--project", "p", "--yes", "--no-install",
	)
	if err == nil {
		t.Fatal("expected setup to refuse a symlinked project config directory")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected a symlink error, got: %v", err)
	}
	for _, name := range []string{"langsmith.json", "config.toml"} {
		if _, statErr := os.Stat(filepath.Join(proj, name)); !os.IsNotExist(statErr) {
			t.Fatalf("%s should not be written through .codex symlink: %v", name, statErr)
		}
	}
}

func TestTraceSetupConfiguresBothAgents(t *testing.T) {
	claudeDir, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// Bare `trace setup` (no agent subcommand) tries both Claude Code and Codex.
	if _, err := executeCommand(t,
		"trace", "setup", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("trace setup error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claudeDir, "settings.json")); err != nil {
		t.Fatalf("claude settings not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexDir, "langsmith.json")); err != nil {
		t.Fatalf("codex langsmith.json not written: %v", err)
	}
}

func TestSetupPositionalKeyWithCorruptConfig(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	// A corrupt ~/.langsmith/config.json must not block a positional API key.
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)
	if err := os.WriteFile(configPath, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(t,
		"trace", "setup", "claude", "positional-key", "--project", "p", "--yes", "--no-install",
	); err != nil {
		t.Fatalf("positional key should work despite a corrupt config: %v", err)
	}
	doc := readJSONFile(t, filepath.Join(claudeDir, "settings.json"))
	env, _ := doc["env"].(map[string]any)
	if env["CC_LANGSMITH_API_KEY"] != "positional-key" {
		t.Fatalf("positional key not applied: %+v", env)
	}
}

func TestSetupCodexPreservesComments(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := "# my codex config\n[model]\nname = \"gpt-5\"  # keep this comment\n"
	tomlPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(t,
		"trace", "setup", "codex", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("codex setup error: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "# my codex config") || !strings.Contains(text, "# keep this comment") {
		t.Fatalf("comments were stripped from config.toml:\n%s", text)
	}
	var conf map[string]any
	if err := toml.Unmarshal(data, &conf); err != nil {
		t.Fatalf("config.toml is not valid TOML: %v\n%s", err, text)
	}
	if f, _ := conf["features"].(map[string]any); f["plugin_hooks"] != true {
		t.Fatalf("plugin_hooks not enabled: %+v", conf)
	}
	plugins, _ := conf["plugins"].(map[string]any)
	if e, _ := plugins["tracing@langsmith-codex-plugins"].(map[string]any); e["enabled"] != true {
		t.Fatalf("plugin not enabled: %+v", conf)
	}
	if m, _ := conf["model"].(map[string]any); m["name"] != "gpt-5" {
		t.Fatalf("[model] table not preserved: %+v", conf)
	}
}

func TestSetupCodexLeavesConfiguredFileUnchanged(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	original := "# hand-written, keep me\n[features]\nplugin_hooks = true\n\n[plugins.\"tracing@langsmith-codex-plugins\"]\nenabled = true\n"
	tomlPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(t,
		"trace", "setup", "codex", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("codex setup error: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("already-configured config.toml should be left untouched.\n got: %q\nwant: %q", string(data), original)
	}
}

func TestSetupCodexInlinePluginTable(t *testing.T) {
	_, codexDir := hermeticSetupEnv(t)
	stubRunSetupCommand(t)
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Inline-table form with the plugin disabled: the line-based edit can't find
	// a [plugins."..."] header, so setup must fall back to a re-marshal rather
	// than append a duplicate (invalid) table.
	original := "[features]\nplugin_hooks = false\n\n[plugins]\n\"tracing@langsmith-codex-plugins\" = { enabled = false }\n"
	tomlPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCommand(t,
		"trace", "setup", "codex", "--api-key", "k", "--project", "p", "--yes",
	); err != nil {
		t.Fatalf("codex setup error: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	var conf map[string]any
	if err := toml.Unmarshal(data, &conf); err != nil {
		t.Fatalf("result is not valid TOML: %v\n%s", err, data)
	}
	if f, _ := conf["features"].(map[string]any); f["plugin_hooks"] != true {
		t.Fatalf("plugin_hooks not enabled: %+v", conf)
	}
	plugins, _ := conf["plugins"].(map[string]any)
	if e, _ := plugins["tracing@langsmith-codex-plugins"].(map[string]any); e["enabled"] != true {
		t.Fatalf("plugin not enabled: %+v", conf)
	}
}

func TestSetupRootCommandRemoved(t *testing.T) {
	claudeDir, _ := hermeticSetupEnv(t)
	stubRunSetupCommand(t)

	out, err := executeCommand(t, "setup")
	if err == nil {
		t.Fatalf("expected root setup command to be unavailable, got output: %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(claudeDir, "settings.json")); !os.IsNotExist(statErr) {
		t.Fatal("removed setup command must not write settings.json")
	}
}
