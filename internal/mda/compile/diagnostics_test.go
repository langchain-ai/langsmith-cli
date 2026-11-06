package compile

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

var languages = []project.Language{project.TypeScript, project.Python}

func diagnosticsProject(root string, language project.Language) *project.Project {
	return &project.Project{Root: root, Language: language, AgentFile: "agent." + language.Extension()}
}

func TestMissingInstructionsWarnInBothLanguages(t *testing.T) {
	root := t.TempDir()
	for _, language := range languages {
		got := projectWarnings(diagnosticsProject(root, language), false)
		if !reflect.DeepEqual(got, []string{"instructions.md is missing. Add it next to the agent file to set the system prompt."}) {
			t.Fatalf("%v: %q", language, got)
		}
	}
}

func TestFactoriesDoNotRequireAStaticPromptInBothLanguages(t *testing.T) {
	for _, language := range languages {
		root := t.TempDir()
		if got := projectWarnings(diagnosticsProject(root, language), true); len(got) != 0 {
			t.Fatalf("%v: %q", language, got)
		}
	}
}

func TestInstructionsAreCheckedNextToTheAgent(t *testing.T) {
	root := t.TempDir()
	p := diagnosticsProject(root, project.TypeScript)
	p.AgentFile = filepath.Join("src", "agent.ts")
	writeFile(t, root, "instructions.md", "root instructions")
	if got := projectWarnings(p, false); len(got) != 1 {
		t.Fatalf("got %q", got)
	}
	writeFile(t, root, "src/instructions.md", "")
	if got := projectWarnings(p, false); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
