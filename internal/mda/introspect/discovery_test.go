package introspect

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func projectWithFiles(language project.Language, files ...string) *project.Project {
	native := make([]string, len(files))
	for i, file := range files {
		native[i] = filepath.FromSlash(file)
	}
	return &project.Project{Root: "/tmp", Language: language, AgentFile: "agent.ts", Files: native}
}

func TestPluginModules(t *testing.T) {
	for _, c := range []struct {
		path     string
		language project.Language
		want     bool
	}{
		{"channels/slack.py", project.Python, true},
		{"channels/slack.ts", project.TypeScript, true},
		{"channels/__init__.py", project.Python, false},
		{".venv/lib/python3.13/site-packages/managed_deepagents/channels/slack.py", project.Python, false},
		{"src/channels/foo.py", project.Python, false},
		{"channels/readme.md", project.Python, false},
		{"channels/slack.test.ts", project.TypeScript, false},
		{"channels/slack.spec.ts", project.TypeScript, false},
		{"channels/test_slack.py", project.Python, false},
		{"channels/slack_test.py", project.Python, false},
	} {
		if got := isPluginModule(filepath.FromSlash(c.path), "channels", c.language); got != c.want {
			t.Fatalf("%s: got %v", c.path, got)
		}
	}
}

func TestToolsMcpModuleIsDiscovered(t *testing.T) {
	p := projectWithFiles(project.TypeScript, "agent.ts", "tools/mcp.ts", "tools/mcp.mts")
	if got, ok := FindMcpEntry(p); !ok || got != filepath.FromSlash("tools/mcp.ts") {
		t.Fatalf("got %q %v", got, ok)
	}
}

func TestPluginEntriesAreSortedTopLevelModules(t *testing.T) {
	p := projectWithFiles(project.TypeScript,
		"channels/slack.ts", "connectors/mcp.ts", "channels/discord.ts", "channels/discord.test.ts",
		"channels/nested/teams.ts", "connectors/readme.md")
	if got, want := FindChannelEntries(p), []string{
		filepath.FromSlash("channels/discord.ts"), filepath.FromSlash("channels/slack.ts"),
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	if got, want := FindLegacyConnectorEntries(p), []string{filepath.FromSlash("connectors/mcp.ts")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestLanguageDirEntriesKeepOnlyLanguageModules(t *testing.T) {
	p := projectWithFiles(project.TypeScript,
		"schedules/a.ts", "schedules/b.tsx", "schedules/c.mts", "schedules/d.cts",
		"schedules/e.js", "schedules/notes.md", "schedules/nested/f.ts")
	want := []string{
		filepath.FromSlash("schedules/a.ts"), filepath.FromSlash("schedules/b.tsx"),
		filepath.FromSlash("schedules/c.mts"), filepath.FromSlash("schedules/d.cts"),
	}
	if got := LanguageDirEntries(p, "schedules"); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestRootEntriesFollowTheLanguage(t *testing.T) {
	p := projectWithFiles(project.Python, "agent.py", "identity.py", "memory.py", "sandbox/__init__.py", "identity.ts")
	if got, ok := FindIdentityEntry(p); !ok || got != "identity.py" {
		t.Fatalf("identity: %q %v", got, ok)
	}
	if got, ok := FindMemoryEntry(p); !ok || got != "memory.py" {
		t.Fatalf("memory: %q %v", got, ok)
	}
	if got, ok := FindSandboxEntry(p); !ok || got != filepath.FromSlash("sandbox/__init__.py") {
		t.Fatalf("sandbox: %q %v", got, ok)
	}
	none := projectWithFiles(project.TypeScript, "agent.ts")
	if _, ok := FindIdentityEntry(none); ok {
		t.Fatal("identity found in a project without one")
	}
}

func TestEntryStemName(t *testing.T) {
	if got, err := EntryStemName(filepath.FromSlash("schedules/daily.ts"), "schedule"); err != nil || got != "daily" {
		t.Fatalf("got %q %v", got, err)
	}
	_, err := EntryStemName("", "schedule")
	requireErrorPrefix(t, err, "invalid schedule file name")
}
