package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hubProject(root, agent string) *Project {
	agent = filepath.FromSlash(agent)
	return &Project{Root: root, Language: TypeScript, AgentFile: agent, Files: []string{agent}}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(contents)
}

func TestEnsureLocalContextHubSeedsAgentMemory(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")
	writeFile(t, tmp, "instructions.md", "Be helpful.\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)

	assert.True(t, strings.HasSuffix(hub.Root, filepath.Join(".mda", LocalContextHubDir)), hub.Root)
	assert.Equal(t, "Be helpful.\n", readText(t, filepath.Join(hub.Root, InstructionsFile)))
	assert.FileExists(t, filepath.Join(hub.Root, AgentMemoryFile))
	assert.False(t, hub.HasSkills)
}

func TestEnsureLocalContextHubPreservesExistingMemory(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")
	writeFile(t, tmp, "instructions.md", "v1\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)
	memory := filepath.Join(hub.Root, AgentMemoryFile)
	require.NoError(t, os.WriteFile(memory, []byte("custom preference\n"), 0o644))

	writeFile(t, tmp, "instructions.md", "v2\n")
	hub2, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)

	assert.Equal(t, "v2\n", readText(t, filepath.Join(hub2.Root, InstructionsFile)))
	assert.Equal(t, "custom preference\n", readText(t, memory))
}

func TestEnsureLocalContextHubRemovesStaleInstructions(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")
	writeFile(t, tmp, "instructions.md", "v1\n")
	_, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), false)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(tmp, "instructions.md")))
	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), false)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(hub.Root, InstructionsFile))
}

func TestEnsureLocalContextHubSeedsAnEmptyHotFile(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)

	// The file exists so edit_file works on turn one.
	assert.Equal(t, "", readText(t, filepath.Join(hub.Root, AgentMemoryFile)))
}

func TestEnsureLocalContextHubSkipsTheSeedWhenMemoryIsOff(t *testing.T) {
	// A fresh project that declares no memory gets no local hot file. A
	// dynamic run can create memory later if it selects the agent layer.
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), false)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(hub.Root, AgentMemoryFile))
	assert.NoFileExists(t, filepath.Join(hub.Root, FlatMemoryFile))
	assert.NoDirExists(t, filepath.Join(hub.Root, MemoriesDir))
}

func TestEnsureLocalContextHubPreservesMemoryWhenDisabledAfterUse(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)
	memory := filepath.Join(hub.Root, AgentMemoryFile)
	require.NoError(t, os.WriteFile(memory, []byte("remember this\n"), 0o644))
	writeFile(t, hub.Root, "memories/agent/notes.md", "extra\n")
	writeFile(t, hub.Root, "__users__/opaque/AGENTS.md", "user memory\n")

	hub2, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), false)
	require.NoError(t, err)

	assert.Equal(t, "remember this\n", readText(t, memory))
	assert.Equal(t, "extra\n", readText(t, filepath.Join(hub2.Root, "memories", "agent", "notes.md")))
	assert.Equal(t, "user memory\n", readText(t, filepath.Join(hub2.Root, "__users__", "opaque", "AGENTS.md")))
}

func TestEnsureLocalContextHubSyncsSkillsNextToAgent(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "src/agent.ts", "export const agent = {};\n")
	writeFile(t, tmp, "src/skills/research/SKILL.md", "Research.\n")
	writeFile(t, tmp, "src/skills/research/build/steps.md", "Steps.\n")
	writeFile(t, tmp, "src/skills/research/.DS_Store", "junk")
	writeFile(t, tmp, "src/skills/research/__pycache__/mod.pyc", "junk")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "src/agent.ts"), true)
	require.NoError(t, err)

	assert.True(t, hub.HasSkills)
	assert.Equal(t, "Research.\n", readText(t, filepath.Join(hub.Root, "skills", "research", "SKILL.md")))
	assert.Equal(t, "Steps.\n", readText(t, filepath.Join(hub.Root, "skills", "research", "build", "steps.md")))
	assert.NoFileExists(t, filepath.Join(hub.Root, "skills", "research", ".DS_Store"))
	assert.NoDirExists(t, filepath.Join(hub.Root, "skills", "research", "__pycache__"))
}

func TestEnsureLocalContextHubMigratesFlatMemoryIntoAgentSlice(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, "agent.ts", "export const agent = {};\n")
	hubRoot := filepath.Join(tmp, ".mda", LocalContextHubDir)
	writeFile(t, hubRoot, FlatMemoryFile, "legacy hot\n")

	hub, err := EnsureLocalContextHub(hubProject(tmp, "agent.ts"), true)
	require.NoError(t, err)

	assert.Equal(t, "legacy hot\n", readText(t, filepath.Join(hub.Root, AgentMemoryFile)))
	assert.Equal(t, "legacy hot\n", readText(t, filepath.Join(hub.Root, FlatMemoryFile)))
}
