package compile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

var languages = []project.Language{project.TypeScript, project.Python}

func diagnosticsProject(root string, language project.Language) *project.Project {
	return &project.Project{Root: root, Language: language, AgentFile: "agent." + language.Extension()}
}

func declaredMemory(language project.Language, agent bool) introspect.MemoryDecision {
	return introspect.MemoryDecision{Declared: &introspect.MemorySummary{
		Path:  "memory." + language.Extension(),
		Scope: introspect.MemoryLayers{Agent: agent, User: true},
	}}
}

func local(root string) runtimeContext {
	return runtimeContext{kind: runtimeContextLocal, contextRoot: root}
}

func warnings(t *testing.T, p *project.Project, memory introspect.MemoryDecision, context runtimeContext, factory bool) []string {
	t.Helper()
	got, err := projectWarnings(p, memory, context, factory)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestMissingInstructionsWarnInBothLanguages(t *testing.T) {
	root := t.TempDir()
	for _, language := range languages {
		got := warnings(t, diagnosticsProject(root, language), introspect.MemoryDecision{}, local(root), false)
		if !reflect.DeepEqual(got, []string{"instructions.md is missing. Add it next to the agent file to set the system prompt."}) {
			t.Fatalf("%v: %q", language, got)
		}
	}
}

func TestFactoriesDoNotRequireAStaticPromptInBothLanguages(t *testing.T) {
	for _, language := range languages {
		root := t.TempDir()
		if got := warnings(t, diagnosticsProject(root, language), introspect.MemoryDecision{}, local(root), true); len(got) != 0 {
			t.Fatalf("%v: %q", language, got)
		}
	}
}

func TestFactoriesStillWarnAboutEmptyAgentMemory(t *testing.T) {
	root := t.TempDir()
	for _, language := range languages {
		got := warnings(t, diagnosticsProject(root, language), declaredMemory(language, true), local(root), true)
		if len(got) != 1 || !strings.Contains(got[0], "enables agent memory") {
			t.Fatalf("%v: %q", language, got)
		}
	}
}

func TestInstructionsAreCheckedNextToTheAgent(t *testing.T) {
	root := t.TempDir()
	p := diagnosticsProject(root, project.TypeScript)
	p.AgentFile = filepath.Join("src", "agent.ts")
	writeFile(t, root, "instructions.md", "root instructions")
	if got := warnings(t, p, introspect.MemoryDecision{}, local(root), false); len(got) != 1 {
		t.Fatalf("got %q", got)
	}
	writeFile(t, root, "src/instructions.md", "")
	if got := warnings(t, p, introspect.MemoryDecision{}, local(root), false); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestAbsentOrUserOnlyMemoryDoesNotWarn(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "instructions.md", "instructions")
	for _, memory := range []introspect.MemoryDecision{{}, declaredMemory(project.Python, false)} {
		if got := warnings(t, diagnosticsProject(root, project.Python), memory, local(root), false); len(got) != 0 {
			t.Fatalf("got %q", got)
		}
	}
}

func TestEmptyAgentMemoryWarnsWithoutChangingFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "instructions.md", "instructions")
	writeFile(t, root, "memories/agent/AGENTS.md", "")
	for _, language := range languages {
		got := warnings(t, diagnosticsProject(root, language), declaredMemory(language, true), local(root), false)
		want := "memory." + language.Extension() + " enables agent memory, but " +
			filepath.Join("memories", "agent") + "/ is empty. Add initial memory there if needed."
		if !reflect.DeepEqual(got, []string{want}) {
			t.Fatalf("%v: %q", language, got)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "memories", "agent", "AGENTS.md")); err != nil || info.Size() != 0 {
		t.Fatalf("the memory file changed: %v", err)
	}
}

func TestNonEmptyAgentMemoryDoesNotWarn(t *testing.T) {
	for _, file := range []string{"memories/agent/AGENTS.md", "memories/agent/nested/notes.md"} {
		root := t.TempDir()
		writeFile(t, root, "instructions.md", "instructions")
		writeFile(t, root, "memories/agent/AGENTS.md", "")
		writeFile(t, root, file, "fixture data")
		if got := warnings(t, diagnosticsProject(root, project.Python), declaredMemory(project.Python, true), local(root), false); len(got) != 0 {
			t.Fatalf("%s: %q", file, got)
		}
	}
}

func TestRemoteMemoryDoesNotWarn(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "instructions.md", "instructions")
	context := runtimeContext{kind: runtimeContextContextHub, repo: "test"}
	if got := warnings(t, diagnosticsProject(root, project.Python), declaredMemory(project.Python, true), context, false); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}

func TestJunkFilesDoNotCountAsMemory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "instructions.md", "instructions")
	writeFile(t, root, "memories/agent/.DS_Store", "junk")
	writeFile(t, root, "memories/agent/__pycache__/memory.pyc", "junk")
	if got := warnings(t, diagnosticsProject(root, project.Python), declaredMemory(project.Python, true), local(root), false); len(got) != 1 {
		t.Fatalf("got %q", got)
	}
}
