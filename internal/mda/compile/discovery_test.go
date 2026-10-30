package compile

import (
	"path/filepath"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestAgentImportSpecifiers(t *testing.T) {
	for _, tc := range []struct {
		file     string
		language project.Language
		want     string
	}{
		{"agent.ts", project.TypeScript, "./agent"},
		{filepath.Join("src", "agent.ts"), project.TypeScript, "./src/agent"},
		{"agent.py", project.Python, "agent"},
		{filepath.Join("src", "agent.py"), project.Python, "src.agent"},
	} {
		if got := agentImportSpecifier(tc.file, tc.language); got != tc.want {
			t.Fatalf("%s: got %q", tc.file, got)
		}
	}
}

func TestSandboxImportSpecifiers(t *testing.T) {
	if got := sandboxImportSpecifier(filepath.Join("sandbox", "index.ts"), project.TypeScript); got != "./sandbox/index" {
		t.Fatalf("got %q", got)
	}
	// Python collapses the package's `__init__` module to the package.
	if got := sandboxImportSpecifier(filepath.Join("sandbox", "__init__.py"), project.Python); got != "sandbox" {
		t.Fatalf("got %q", got)
	}
}

func filesProject(files ...string) *project.Project {
	for i, file := range files {
		files[i] = filepath.FromSlash(file)
	}
	return &project.Project{Root: "/tmp", Language: project.TypeScript, AgentFile: "agent.ts", Files: files}
}

func TestMisspelledChannelDirectoryWarning(t *testing.T) {
	warning, ok := misspelledChannelDirectoryWarning(filesProject("agent.ts", "channel/slack.ts"))
	if !ok || warning != "found `channel/`, but channel deploys are discovered from `channels/` — check the directory name" {
		t.Fatalf("got %q, %v", warning, ok)
	}
	for _, p := range []*project.Project{
		filesProject("agent.ts", "channels/slack.ts", "channel/slack.ts"),
		filesProject("agent.ts"),
	} {
		if warning, ok := misspelledChannelDirectoryWarning(p); ok {
			t.Fatalf("%v: %q", p.Files, warning)
		}
	}
}

func TestFindBrowserEntryFollowsTheLanguage(t *testing.T) {
	ts := filesProject("agent.ts", "browser.tsx", "browser.py")
	if entry, ok := findBrowserEntry(ts); !ok || entry != "browser.tsx" {
		t.Fatalf("got %q, %v", entry, ok)
	}
	py := &project.Project{Language: project.Python, AgentFile: "agent.py", Files: []string{"agent.py", "browser.ts"}}
	if entry, ok := findBrowserEntry(py); ok {
		t.Fatalf("got %q", entry)
	}
}

func TestCompileWarnsAboutAMisspelledChannelDirectory(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "channel/slack.ts": "export const x = 1;\n"})

	summary := mustCompile(t, f.deploy)

	if len(summary.Warnings) == 0 || summary.Warnings[0] !=
		"found `channel/`, but channel deploys are discovered from `channels/` — check the directory name" {
		t.Fatalf("warnings: %q", summary.Warnings)
	}
}
