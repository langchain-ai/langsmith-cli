//go:build unix

package compile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestCompilePreservesExecutablePermissionsOnCopiedHelpers(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "scripts/run.sh": "#!/bin/sh\necho hi\n"})
	if err := os.Chmod(filepath.Join(f.root(), "scripts", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	mode := func() os.FileMode {
		info, err := os.Stat(filepath.Join(f.outDir, "scripts", "run.sh"))
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}

	mustCompile(t, f.deploy)
	if mode() != 0o755 {
		t.Fatalf("mode %v", mode())
	}

	// Mode-only drift on the build copy is repaired even when the bytes are
	// unchanged.
	if err := os.Chmod(filepath.Join(f.outDir, "scripts", "run.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncFilePermissions(filepath.Join(f.root(), "scripts", "run.sh"), filepath.Join(f.outDir, "scripts", "run.sh")); err != nil {
		t.Fatal(err)
	}
	if mode() != 0o755 {
		t.Fatalf("mode %v", mode())
	}
}

func TestSymlinksDoNotCountAsMemory(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"instructions.md":          "instructions",
		"outside/notes.md":         "fixture data",
		"memories/agent/AGENTS.md": "",
	})
	for link, target := range map[string]string{"linked": "outside", "notes.md": "outside/notes.md"} {
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(root, "memories", "agent", link)); err != nil {
			t.Fatal(err)
		}
	}

	got := warnings(t, diagnosticsProject(root, project.Python),
		introspect.MemoryDecision{Declared: &introspect.MemorySummary{Path: "memory.py", Scope: introspect.MemoryLayers{Agent: true}}},
		local(root), false)

	if len(got) != 1 {
		t.Fatalf("got %q", got)
	}
}
