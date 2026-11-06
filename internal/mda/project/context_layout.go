package project

import "path/filepath"

// Context Hub layout constants for deploy sync (`context_layout.rs`).
const (
	// InstructionsFile is the deploy-owned system prompt file at the Hub root.
	InstructionsFile = "instructions.md"

	// SkillsDir is the deploy-owned skills directory at the Hub root.
	SkillsDir = "skills"

	// MemoriesDir is the agent-owned memory tree at the Hub root (sliced per
	// scope before remount).
	MemoriesDir = "memories"

	// AgentMemoryFile is the hot memory file for the agent-scoped slice
	// (`memory: agent` / no identity).
	AgentMemoryFile = "memories/agent/AGENTS.md"

	// FlatMemoryFile is the legacy flat hot memory path (pre-slice layout).
	// Preserved when already present.
	FlatMemoryFile = "memories/AGENTS.md"

	// DefaultMemoryContent is the seed content for a new agent-scoped hot
	// memory file.
	//
	// Empty on purpose. The instructions that used to live here — how to use
	// memory, which paths exist, what not to store — are stated by the runtime
	// instead (`packages/npm/src/memory/instructions-middleware.ts` and its
	// Python mirror), because prose committed into a Hub file is customer data:
	// it renders as "agent memory" in the Context Hub UI, and it goes stale in
	// place with no way to correct it except rewriting someone's file.
	//
	// A zero-byte file rather than no file at all: it is prompt-identical (both
	// render as no memory loaded), but it keeps `edit_file` — the verb
	// deepagents teaches — working on the agent's first save.
	DefaultMemoryContent = ""
)

// AgentMemorySeed is the content that should be written to [AgentMemoryFile],
// if any.
type AgentMemorySeed struct {
	// Write is false when existing agent-scoped memory is current — leave it
	// alone — and true to write Content to [AgentMemoryFile] (seed, migrate, or
	// refresh).
	Write   bool
	Content string
}

// ResolveAgentMemorySeed decides what to do with agent-scoped hot memory given
// the current Hub state. A nil content is a file that does not exist.
//
// Deploy never rewrites memory. An existing [AgentMemoryFile] is left exactly
// as it is, whatever is in it; a legacy [FlatMemoryFile] is copied across
// verbatim when it is the only hot memory present, so the pre-slice layout
// still remounts. Only a repo with neither gets the empty seed.
func ResolveAgentMemorySeed(agentContent, flatContent *string) AgentMemorySeed {
	if agentContent != nil {
		return AgentMemorySeed{}
	}
	if flatContent != nil {
		return AgentMemorySeed{Write: true, Content: *flatContent}
	}
	return AgentMemorySeed{Write: true, Content: DefaultMemoryContent}
}

// ProjectContextRoot is the project directory that owns `instructions.md`,
// `skills/`, and `memories/`.
//
// Usually the agent file's parent; the project root when the agent sits at the
// root.
func ProjectContextRoot(p *Project) string {
	return filepath.Join(p.Root, filepath.Dir(p.AgentFile))
}
