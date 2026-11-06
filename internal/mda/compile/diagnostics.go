package compile

import (
	"path/filepath"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func projectWarnings(p *project.Project, isFactory bool) []string {
	var warnings []string
	instructions := filepath.Join(project.ProjectContextRoot(p), project.InstructionsFile)
	if !isFactory && !isFile(instructions) {
		warnings = append(warnings,
			"instructions.md is missing. Add it next to the agent file to set the system prompt.")
	}
	return warnings
}
