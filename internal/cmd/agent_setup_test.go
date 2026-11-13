package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// testSkills is the fake langsmith-skills package served to setup tests.
var testSkills = map[string]string{
	"langsmith-trace/SKILL.md":              "---\nname: langsmith-trace\n---\ntrace\n",
	"langsmith-trace/scripts/query.sh":      "#!/bin/sh\necho hi\n",
	"langsmith-dataset/SKILL.md":            "---\nname: langsmith-dataset\n---\ndataset\n",
	"langsmith-dataset/references/extra.md": "more\n",
	"not-a-skill/README.md":                 "no SKILL.md here\n",
}

type tarEntry struct {
	name     string
	body     string
	mode     int64
	typeflag byte
	linkname string
}

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.body)), Typeflag: e.typeflag, Linkname: e.linkname}
		if hdr.Typeflag == 0 {
			hdr.Typeflag = tar.TypeReg
		}
		if hdr.Mode == 0 {
			hdr.Mode = 0o644
		}
		if hdr.Typeflag != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testSkillsArchive(t *testing.T) []byte {
	t.Helper()
	entries := []tarEntry{{name: "langsmith-skills-abc123/README.md", body: "readme"}}
	for rel, body := range testSkills {
		mode := int64(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		entries = append(entries, tarEntry{name: "langsmith-skills-abc123/config/skills/" + rel, body: body, mode: mode})
	}
	return buildTarGz(t, entries)
}

// hermeticAgentSetupEnv gives each test a fake home, points every agent's
// config dir into it, serves a fake skills archive, and reports only the named
// binaries as installed. It returns the fake home.
func hermeticAgentSetupEnv(t *testing.T, binaries ...string) string {
	t.Helper()
	hermeticSetupEnv(t)
	home := os.Getenv("HOME")
	t.Setenv("USERPROFILE", home)
	t.Setenv("LANGSMITH_CLAUDE_BIN", "")
	t.Setenv("LANGSMITH_CODEX_BIN", "")

	oldLook := setupLookPath
	setupLookPath = func(name string) (string, error) {
		if slices.Contains(binaries, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { setupLookPath = oldLook })

	archive := testSkillsArchive(t)
	oldFetch := fetchSkillsArchive
	fetchSkillsArchive = func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(archive)), nil
	}
	t.Cleanup(func() { fetchSkillsArchive = oldFetch })
	return home
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeSetupFile(t *testing.T, path, content string) {
	t.Helper()
	mkdirs(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

type setupResult struct {
	Status string      `json:"status"`
	MCPURL string      `json:"mcp_url"`
	Steps  []setupStep `json:"steps"`
}

func runSetupJSON(t *testing.T, args ...string) (setupResult, error) {
	t.Helper()
	out, err := executeCommand(t, append([]string{"--format=json", "setup"}, args...)...)
	var res setupResult
	if out != "" {
		if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
			t.Fatalf("stdout not JSON: %v\n%s", jerr, out)
		}
	}
	return res, err
}

func stepFor(t *testing.T, res setupResult, agent, component string) setupStep {
	t.Helper()
	for _, s := range res.Steps {
		if s.Agent == agent && s.Component == component {
			return s
		}
	}
	t.Fatalf("no %s/%s step in %+v", agent, component, res.Steps)
	return setupStep{}
}

func TestAgentSetupDetectsOnlyInstalledAgents(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	mkdirs(t, filepath.Join(home, ".cursor"))

	res, err := runSetupJSON(t, "--dry-run")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if res.Status != "dry_run" {
		t.Fatalf("status = %q", res.Status)
	}
	for _, s := range res.Steps {
		if s.Agent != "cursor" {
			t.Fatalf("undetected agent planned: %+v", s)
		}
	}
	if len(res.Steps) != 2 {
		t.Fatalf("want skills + mcp steps for cursor, got %+v", res.Steps)
	}
}

func TestAgentSetupNoAgentFound(t *testing.T) {
	hermeticAgentSetupEnv(t)
	_, err := runSetupJSON(t, "--yes")
	if err == nil || !strings.Contains(err.Error(), "no supported coding agent found") {
		t.Fatalf("expected no-agent error, got %v", err)
	}
}

func TestAgentSetupRejectsUnknownAgent(t *testing.T) {
	hermeticAgentSetupEnv(t)
	_, err := runSetupJSON(t, "--agent", "emacs", "--yes")
	if err == nil || !strings.Contains(err.Error(), `unknown --agent "emacs"`) {
		t.Fatalf("expected unknown-agent error, got %v", err)
	}
}

func TestAgentSetupCursor(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	cursorDir := filepath.Join(home, ".cursor")
	mcpPath := filepath.Join(cursorDir, "mcp.json")
	writeSetupFile(t, mcpPath, `{"mcpServers":{"github":{"url":"https://example.com/mcp"}},"other":1}`)

	res, err := runSetupJSON(t, "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if res.Status != "applied" || res.MCPURL != "https://api.smith.langchain.com/mcp" {
		t.Fatalf("unexpected result: %+v", res)
	}

	doc := readJSONFile(t, mcpPath)
	servers := doc["mcpServers"].(map[string]any)
	if got := servers["langsmith"].(map[string]any)["url"]; got != "https://api.smith.langchain.com/mcp" {
		t.Fatalf("langsmith url = %v", got)
	}
	if servers["github"] == nil || doc["other"] != float64(1) {
		t.Fatalf("unrelated config lost: %+v", doc)
	}

	skillsDir := filepath.Join(cursorDir, "skills")
	if got := readFile(t, filepath.Join(skillsDir, "langsmith-trace", "SKILL.md")); got != testSkills["langsmith-trace/SKILL.md"] {
		t.Fatalf("SKILL.md = %q", got)
	}
	if got := readFile(t, filepath.Join(skillsDir, "langsmith-dataset", "references", "extra.md")); got != "more\n" {
		t.Fatalf("nested file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "not-a-skill")); !os.IsNotExist(err) {
		t.Fatal("directory without SKILL.md must not be installed")
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(skillsDir, "langsmith-trace", "scripts", "query.sh"))
		if err != nil || fi.Mode().Perm()&0o100 == 0 {
			t.Fatalf("script lost its exec bit: %v %v", fi, err)
		}
	}

	// A second run changes nothing.
	before := readFile(t, mcpPath)
	res, err = runSetupJSON(t, "--yes")
	if err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if res.Status != "unchanged" {
		t.Fatalf("second run status = %q, steps %+v", res.Status, res.Steps)
	}
	if readFile(t, mcpPath) != before {
		t.Fatal("second run rewrote mcp.json")
	}

	// A script that lost its exec bit is put back.
	if runtime.GOOS != "windows" {
		script := filepath.Join(skillsDir, "langsmith-trace", "scripts", "query.sh")
		if err := os.Chmod(script, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := runSetupJSON(t, "--yes"); err != nil {
			t.Fatalf("third setup: %v", err)
		}
		if fi, err := os.Stat(script); err != nil || fi.Mode().Perm()&0o100 == 0 {
			t.Fatalf("exec bit not restored: %v %v", fi, err)
		}
	}
}

// After a profile switch, setup moves the entry it wrote to the new URL
// without --force, but still leaves an entry the user wrote alone.
func TestAgentSetupUpdatesItsOwnMCPEntryAfterProfileSwitch(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	mkdirs(t, filepath.Join(home, ".cursor"), filepath.Join(home, ".codex"))

	if _, err := runSetupJSON(t, "--no-skills", "--yes"); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	res, err := runSetupJSON(t, "--no-skills", "--yes", "--api-url", "https://ls.example.com/api/v1")
	if err != nil || res.Status != "applied" {
		t.Fatalf("second setup: status %q err %v steps %+v", res.Status, err, res.Steps)
	}
	entry := readJSONFile(t, filepath.Join(home, ".cursor", "mcp.json"))["mcpServers"].(map[string]any)["langsmith"].(map[string]any)
	if entry["url"] != "https://ls.example.com/api/mcp" {
		t.Fatalf("cursor entry not moved: %+v", entry)
	}
	if !strings.Contains(readFile(t, filepath.Join(home, ".codex", "config.toml")), `url = "https://ls.example.com/api/mcp"`) {
		t.Fatal("codex entry not moved")
	}

	// A different Codex home holds a hand-written entry at the same URL. Setup
	// wrote to the old home, not this one, so it must leave the entry alone.
	otherCodex := filepath.Join(home, "other-codex")
	otherToml := filepath.Join(otherCodex, "config.toml")
	writeSetupFile(t, otherToml, "[mcp_servers.langsmith]\nurl = \"https://ls.example.com/api/mcp\"\n")
	t.Setenv("CODEX_HOME", otherCodex)
	res, err = runSetupJSON(t, "--agent", "codex", "--no-skills", "--yes", "--api-url", "https://other.example.com/api/v1")
	if err != nil {
		t.Fatalf("setup with another CODEX_HOME: %v", err)
	}
	if s := stepFor(t, res, "codex", "mcp"); s.Action != stepSkip {
		t.Fatalf("entry in another CODEX_HOME should be skipped, got %+v", s)
	}
	if !strings.Contains(readFile(t, otherToml), "https://ls.example.com/api/mcp") {
		t.Fatal("entry in another CODEX_HOME was changed")
	}

	// The user adds a setting but keeps the URL; setup no longer owns it.
	writeSetupFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"langsmith":{"url":"https://ls.example.com/api/mcp","headers":{"X-Team":"a"}}}}`)
	res, err = runSetupJSON(t, "--agent", "cursor", "--no-skills", "--yes")
	if err != nil {
		t.Fatalf("setup after adding headers: %v", err)
	}
	if s := stepFor(t, res, "cursor", "mcp"); s.Action != stepSkip {
		t.Fatalf("entry with user settings should be skipped, got %+v", s)
	}
	if !strings.Contains(readFile(t, filepath.Join(home, ".cursor", "mcp.json")), "X-Team") {
		t.Fatal("user's headers were dropped")
	}

	// The user edits the entry by hand; setup no longer owns it.
	writeSetupFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"langsmith":{"url":"https://mine.example.com/mcp"}}}`)
	res, err = runSetupJSON(t, "--agent", "cursor", "--no-skills", "--yes")
	if err != nil {
		t.Fatalf("third setup: %v", err)
	}
	if s := stepFor(t, res, "cursor", "mcp"); s.Action != stepSkip {
		t.Fatalf("user-edited entry should be skipped, got %+v", s)
	}
}

func TestAgentSetupCodex(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	codexDir := filepath.Join(home, ".codex")
	tomlPath := filepath.Join(codexDir, "config.toml")
	original := "# my codex config\nmodel = \"gpt-5\"\n\n[mcp_servers.github]\nurl = \"https://example.com/mcp\"\n"
	writeSetupFile(t, tomlPath, original)

	if _, err := runSetupJSON(t, "--agent", "codex", "--yes", "--api-url", "https://ls.example.com/api/v1"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	text := readFile(t, tomlPath)
	if !strings.HasPrefix(text, original) {
		t.Fatalf("existing config.toml content not preserved:\n%s", text)
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("invalid TOML: %v\n%s", err, text)
	}
	servers := doc["mcp_servers"].(map[string]any)
	if got := servers["langsmith"].(map[string]any)["url"]; got != "https://ls.example.com/api/mcp" {
		t.Fatalf("langsmith url = %v", got)
	}
	if !pathExists(filepath.Join(codexDir, "skills", "langsmith-dataset", "SKILL.md")) {
		t.Fatal("skills not installed into $CODEX_HOME/skills")
	}

	res, err := runSetupJSON(t, "--agent", "codex", "--yes", "--api-url", "https://ls.example.com/api/v1")
	if err != nil || res.Status != "unchanged" {
		t.Fatalf("second run: status %q err %v", res.Status, err)
	}
	if readFile(t, tomlPath) != text {
		t.Fatal("second run rewrote config.toml")
	}
}

func TestAgentSetupCodexConflictNeedsForce(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	tomlPath := filepath.Join(home, ".codex", "config.toml")
	original := "[mcp_servers.langsmith]\n# keep me\nurl = \"https://old.example.com/mcp\"\n"
	writeSetupFile(t, tomlPath, original)

	res, err := runSetupJSON(t, "--agent", "codex", "--no-skills", "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if s := stepFor(t, res, "codex", "mcp"); s.Action != stepSkip || !strings.Contains(s.Detail, "--force") {
		t.Fatalf("expected skip with --force hint, got %+v", s)
	}
	if readFile(t, tomlPath) != original {
		t.Fatal("conflicting entry was changed without --force")
	}

	if _, err := runSetupJSON(t, "--agent", "codex", "--no-skills", "--yes", "--force"); err != nil {
		t.Fatalf("setup --force: %v", err)
	}
	want := "[mcp_servers.langsmith]\nurl = \"https://api.smith.langchain.com/mcp\"\n# keep me\n"
	if got := readFile(t, tomlPath); got != want {
		t.Fatalf("config.toml after --force:\n%s", got)
	}
}

func TestAgentSetupCodexForceReplacesStdioEntry(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	tomlPath := filepath.Join(home, ".codex", "config.toml")
	writeSetupFile(t, tomlPath, `model = "gpt-5"

[mcp_servers.langsmith]
# local server
command = "uvx"
args = [
  "langsmith-mcp-server",
]

[mcp_servers.langsmith.env]
LANGSMITH_API_KEY = "secret"

[mcp_servers.github]
url = "https://example.com/mcp"
`)

	if _, err := runSetupJSON(t, "--agent", "codex", "--no-skills", "--yes", "--force"); err != nil {
		t.Fatalf("setup --force: %v", err)
	}
	want := `model = "gpt-5"

[mcp_servers.langsmith]
url = "https://api.smith.langchain.com/mcp"
# local server

[mcp_servers.github]
url = "https://example.com/mcp"
`
	if got := readFile(t, tomlPath); got != want {
		t.Fatalf("config.toml after --force:\n%s", got)
	}
}

func TestAgentSetupCodexForceHandlesHeaderComment(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	tomlPath := filepath.Join(home, ".codex", "config.toml")
	other := "[mcp_servers.\"langsmith.foo\"]\nurl = \"https://foo.example.com/mcp\"\n\n[mcp_servers.\"langsmith \".env]\nKEY = \"v\"\n"
	writeSetupFile(t, tomlPath, "[ mcp_servers.\"langsmith\" ] # personal MCP\nurl = \"https://old.example.com/mcp\"\n\n"+other)

	res, err := runSetupJSON(t, "--agent", "codex", "--no-skills", "--yes", "--force")
	if err != nil {
		t.Fatalf("setup --force: %v (steps %+v)", err, res.Steps)
	}
	// A separate server whose quoted name contains a dot is not part of langsmith.
	want := "[ mcp_servers.\"langsmith\" ] # personal MCP\nurl = \"https://api.smith.langchain.com/mcp\"\n\n" + other
	if got := readFile(t, tomlPath); got != want {
		t.Fatalf("config.toml after --force:\n%s", got)
	}
}

func TestAgentSetupCursorConflictNeedsForce(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	mcpPath := filepath.Join(home, ".cursor", "mcp.json")
	original := `{"mcpServers":{"langsmith":{"command":"uvx","args":["langsmith-mcp-server"]}}}`
	writeSetupFile(t, mcpPath, original)

	res, err := runSetupJSON(t, "--agent", "cursor", "--no-skills", "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if s := stepFor(t, res, "cursor", "mcp"); s.Action != stepSkip {
		t.Fatalf("expected skip, got %+v", s)
	}
	if readFile(t, mcpPath) != original {
		t.Fatal("existing langsmith entry changed without --force")
	}

	if _, err := runSetupJSON(t, "--agent", "cursor", "--no-skills", "--yes", "--force"); err != nil {
		t.Fatalf("setup --force: %v", err)
	}
	entry := readJSONFile(t, mcpPath)["mcpServers"].(map[string]any)["langsmith"].(map[string]any)
	if entry["url"] != "https://api.smith.langchain.com/mcp" || entry["command"] != nil {
		t.Fatalf("entry after --force: %+v", entry)
	}
}

func TestAgentSetupSkillOwnership(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	skillsDir := filepath.Join(home, ".cursor", "skills")
	// The user's own langsmith-trace skill: no setup marker.
	writeSetupFile(t, filepath.Join(skillsDir, "langsmith-trace", "SKILL.md"), "my own version\n")
	// A dataset skill setup wrote earlier, now stale, with a file upstream removed.
	writeSetupFile(t, filepath.Join(skillsDir, "langsmith-dataset", "SKILL.md"), "old\n")
	writeSetupFile(t, filepath.Join(skillsDir, "langsmith-dataset", "removed.md"), "gone upstream\n")
	writeSetupFile(t, filepath.Join(skillsDir, "langsmith-dataset", skillMarkerFile), "{}\n")

	res, err := runSetupJSON(t, "--agent", "cursor", "--no-mcp", "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	s := stepFor(t, res, "cursor", "skills")
	if s.Action != stepUpdate || !strings.Contains(s.Detail, "skip langsmith-trace") {
		t.Fatalf("unexpected skills step: %+v", s)
	}
	if got := readFile(t, filepath.Join(skillsDir, "langsmith-trace", "SKILL.md")); got != "my own version\n" {
		t.Fatalf("user's skill was replaced: %q", got)
	}
	if got := readFile(t, filepath.Join(skillsDir, "langsmith-dataset", "SKILL.md")); got != testSkills["langsmith-dataset/SKILL.md"] {
		t.Fatalf("owned skill not updated: %q", got)
	}
	if pathExists(filepath.Join(skillsDir, "langsmith-dataset", "removed.md")) {
		t.Fatal("stale file left in an owned skill")
	}
	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("temp dir left behind: %s", e.Name())
		}
	}

	if _, err := runSetupJSON(t, "--agent", "cursor", "--no-mcp", "--yes", "--force"); err != nil {
		t.Fatalf("setup --force: %v", err)
	}
	if got := readFile(t, filepath.Join(skillsDir, "langsmith-trace", "SKILL.md")); got != testSkills["langsmith-trace/SKILL.md"] {
		t.Fatalf("--force did not replace the skill: %q", got)
	}
}

func TestAgentSetupClaudeCode(t *testing.T) {
	home := hermeticAgentSetupEnv(t, "claude")
	calls := stubRunSetupCommand(t)
	claudeDir := filepath.Join(home, ".claude")
	mkdirs(t, claudeDir)

	if _, err := runSetupJSON(t, "--yes"); err != nil {
		t.Fatalf("setup: %v", err)
	}
	want := [][]string{
		{"claude", "plugin", "marketplace", "add", skillsMarketplaceURL},
		{"claude", "plugin", "install", skillsClaudePluginRef, "--scope", "user"},
		{"claude", "mcp", "add", "--transport", "http", "--scope", "user", "langsmith", "https://api.smith.langchain.com/mcp"},
	}
	if !slices.EqualFunc(*calls, want, slices.Equal[[]string]) {
		t.Fatalf("claude calls:\n got %v\nwant %v", *calls, want)
	}

	// Once Claude Code reports both as installed, a rerun runs nothing.
	writeSetupFile(t, filepath.Join(claudeDir, "plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"`+skillsClaudePluginRef+`":[{"scope":"user"}]}}`)
	writeSetupFile(t, filepath.Join(claudeDir, ".claude.json"),
		`{"mcpServers":{"langsmith":{"type":"http","url":"https://api.smith.langchain.com/mcp"}}}`)
	*calls = nil
	res, err := runSetupJSON(t, "--yes")
	if err != nil || res.Status != "unchanged" {
		t.Fatalf("second run: status %q err %v steps %+v", res.Status, err, res.Steps)
	}
	if len(*calls) != 0 {
		t.Fatalf("second run ran commands: %v", *calls)
	}
}

func TestAgentSetupClaudeCodeSkillsPlugin(t *testing.T) {
	home := hermeticAgentSetupEnv(t, "claude")
	calls := stubRunSetupCommand(t)
	// Installed for one project only: still needs the user-scope install.
	writeSetupFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"`+skillsClaudePluginRef+`":[{"scope":"project","projectPath":"/work/a"}]}}`)

	res, err := runSetupJSON(t, "--no-mcp", "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if s := stepFor(t, res, "claude-code", "skills"); s.Action != stepCreate {
		t.Fatalf("project-only plugin should not count as installed: %+v", s)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected marketplace add + install, got %v", *calls)
	}

	// A pinned --skills-ref can't be honored by the marketplace plugin.
	*calls = nil
	res, err = runSetupJSON(t, "--no-mcp", "--yes", "--skills-ref", "v1.2.3")
	if err != nil {
		t.Fatalf("setup --skills-ref: %v", err)
	}
	if s := stepFor(t, res, "claude-code", "skills"); s.Action != stepSkip || !strings.Contains(s.Detail, "--skills-ref") {
		t.Fatalf("expected skip for --skills-ref, got %+v", s)
	}
	if len(*calls) != 0 {
		t.Fatalf("ran commands: %v", *calls)
	}
}

func TestAgentSetupClaudeCodeReplaceWithForce(t *testing.T) {
	home := hermeticAgentSetupEnv(t, "claude")
	calls := stubRunSetupCommand(t)
	writeSetupFile(t, filepath.Join(home, ".claude", ".claude.json"),
		`{"mcpServers":{"langsmith":{"type":"http","url":"https://old.example.com/mcp"}}}`)

	res, err := runSetupJSON(t, "--no-skills", "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if s := stepFor(t, res, "claude-code", "mcp"); s.Action != stepSkip {
		t.Fatalf("expected skip without --force, got %+v", s)
	}
	if len(*calls) != 0 {
		t.Fatalf("ran commands without --force: %v", *calls)
	}

	if _, err := runSetupJSON(t, "--no-skills", "--yes", "--force"); err != nil {
		t.Fatalf("setup --force: %v", err)
	}
	if len(*calls) != 2 || (*calls)[0][2] != "remove" || (*calls)[1][2] != "add" {
		t.Fatalf("expected mcp remove then add, got %v", *calls)
	}
}

func TestAgentSetupClaudeCodeWithoutBinarySkips(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	calls := stubRunSetupCommand(t)
	mkdirs(t, filepath.Join(home, ".claude"))

	res, err := runSetupJSON(t, "--yes")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if res.Status != "incomplete" {
		t.Fatalf("status = %q, want incomplete", res.Status)
	}
	for _, s := range res.Steps {
		if s.Action != stepSkip || !strings.Contains(s.Detail, "not found on PATH") {
			t.Fatalf("expected skip for missing claude CLI, got %+v", s)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("ran commands: %v", *calls)
	}
}

func TestAgentSetupDryRunWritesNothing(t *testing.T) {
	home := hermeticAgentSetupEnv(t, "claude")
	calls := stubRunSetupCommand(t)
	mkdirs(t, filepath.Join(home, ".cursor"), filepath.Join(home, ".codex"), filepath.Join(home, ".claude"))

	out, err := executeCommand(t, "setup", "--dry-run")
	if err != nil {
		t.Fatalf("setup --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, "https://api.smith.langchain.com/mcp") {
		t.Fatalf("dry run output missing plan:\n%s", out)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry run ran commands: %v", *calls)
	}
	for _, p := range []string{".cursor/mcp.json", ".cursor/skills", ".codex/config.toml", ".codex/skills"} {
		if pathExists(filepath.Join(home, p)) {
			t.Fatalf("dry run wrote %s", p)
		}
	}
}

func TestAgentSetupRefusesWithoutConfirmation(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	mkdirs(t, filepath.Join(home, ".cursor"))
	old := inputIsTerminal
	inputIsTerminal = func(io.Reader) bool { return false }
	t.Cleanup(func() { inputIsTerminal = old })

	_, err := runSetupJSON(t)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected refusal without --yes, got %v", err)
	}
	if pathExists(filepath.Join(home, ".cursor", "mcp.json")) {
		t.Fatal("wrote config without confirmation")
	}
}

func TestAgentSetupMCPURLOverride(t *testing.T) {
	home := hermeticAgentSetupEnv(t)
	mkdirs(t, filepath.Join(home, ".cursor"))

	res, err := runSetupJSON(t, "--dry-run", "--mcp-url", "https://mcp.example.com/mcp")
	if err != nil || res.MCPURL != "https://mcp.example.com/mcp" {
		t.Fatalf("override: %+v %v", res, err)
	}
	for _, bad := range []string{"ftp://nope", "http://ls.example.com/api/mcp"} {
		if _, err := runSetupJSON(t, "--dry-run", "--mcp-url", bad); err == nil {
			t.Fatalf("expected --mcp-url %s to fail", bad)
		}
	}
	if _, err := runSetupJSON(t, "--dry-run", "--mcp-url", "http://localhost:1984/api/mcp"); err != nil {
		t.Fatalf("http on localhost should be allowed: %v", err)
	}
}

func TestParseSkillsArchiveIgnoresUnsafeEntries(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "root/config/skills/good/SKILL.md", body: "ok"},
		{name: "root/config/skills/good/../../../evil", body: "x"},
		{name: "root/config/skills/good/link", typeflag: tar.TypeSymlink, linkname: "/etc/passwd"},
		{name: "root/config/skills/good/" + skillMarkerFile, body: "{}"},
		{name: "root/config/skills/Bad Name/SKILL.md", body: "x"},
	})
	pkg, err := parseSkillsArchive(bytes.NewReader(archive), "main")
	if err != nil {
		t.Fatal(err)
	}
	if got := pkg.names(); !slices.Equal(got, []string{"good"}) {
		t.Fatalf("skills = %v", got)
	}
	if files := pkg.skills["good"]; len(files) != 1 {
		t.Fatalf("unsafe entries kept: %v", files)
	}

	if _, err := parseSkillsArchive(bytes.NewReader(buildTarGz(t, []tarEntry{{name: "root/README.md", body: "x"}})), "main"); err == nil {
		t.Fatal("expected an error for an archive with no skills")
	}
}

func TestMCPURLForAPIBase(t *testing.T) {
	for in, want := range map[string]string{
		"":                                    "https://api.smith.langchain.com/mcp",
		"https://api.smith.langchain.com":     "https://api.smith.langchain.com/mcp",
		"https://eu.api.smith.langchain.com/": "https://eu.api.smith.langchain.com/mcp",
		"https://ls.example.com/api/v1":       "https://ls.example.com/api/mcp",
		"https://ls.example.com":              "https://ls.example.com/api/mcp",
		"https://ls.example.com/api":          "https://ls.example.com/api/mcp",
	} {
		got, err := mcpURLForAPIBase(in)
		if err != nil || got != want {
			t.Errorf("mcpURLForAPIBase(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestTOMLHeaderKeys(t *testing.T) {
	for line, want := range map[string][]string{
		`[mcp_servers.langsmith]`:              {"mcp_servers", "langsmith"},
		`[ mcp_servers . "langsmith" ] # note`: {"mcp_servers", "langsmith"},
		`[mcp_servers."langsmith.foo"]`:        {"mcp_servers", "langsmith.foo"},
		`[mcp_servers.'langsmith.foo'.env]`:    {"mcp_servers", "langsmith.foo", "env"},
		`[mcp_servers."lang\"smith"]`:          {"mcp_servers", `lang"smith`},
		`[mcp_servers."langsmith ".env]`:       {"mcp_servers", "langsmith ", "env"},
		`[[mcp_servers.langsmith]]`:            nil,
		`[mcp_servers.langsmith] trailing`:     nil,
		`[mcp_servers."unterminated]`:          nil,
	} {
		if got := tomlHeaderKeys(line); !slices.Equal(got, want) {
			t.Errorf("tomlHeaderKeys(%q) = %q, want %q", line, got, want)
		}
	}
}
