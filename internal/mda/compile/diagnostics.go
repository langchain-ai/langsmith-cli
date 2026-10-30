package compile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func projectWarnings(
	p *project.Project,
	memory introspect.MemoryDecision,
	context runtimeContext,
	isFactory bool,
) ([]string, error) {
	var warnings []string
	instructions := filepath.Join(project.ProjectContextRoot(p), project.InstructionsFile)
	if !isFactory && !isFile(instructions) {
		warnings = append(warnings,
			"instructions.md is missing. Add it next to the agent file to set the system prompt.")
	}
	if context.kind != runtimeContextLocal {
		return warnings, nil
	}
	declaration := memory.Declared
	if declaration == nil {
		return warnings, nil
	}
	if declaration.Scope.Agent {
		hasMemory, err := hasAgentMemory(context.contextRoot)
		if err != nil {
			return nil, err
		}
		if !hasMemory {
			memoryRoot := filepath.Join(context.contextRoot, project.MemoriesDir, "agent")
			location := memoryRoot
			if rel, ok := stripPathPrefix(memoryRoot, p.Root); ok {
				location = rel
			}
			warnings = append(warnings, fmt.Sprintf(
				"%s enables agent memory, but %s/ is empty. Add initial memory there if needed.",
				declaration.Path, location))
		}
	}
	return warnings, nil
}

// hasAgentMemory reports whether the local agent memory holds a non-empty
// file. Memory contents are never read; they can contain private data.
func hasAgentMemory(contextRoot string) (bool, error) {
	memories := filepath.Join(contextRoot, project.MemoriesDir)
	agent := filepath.Join(memories, "agent")
	for _, path := range []string{contextRoot, memories, agent} {
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("could not inspect local agent memory: %w", err)
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	found := false
	err := filepath.WalkDir(agent, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != agent && (project.IsJunkDir(path) || project.IsJunkFile(path)) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 0 {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("could not inspect local agent memory: %w", err)
	}
	return found, nil
}
