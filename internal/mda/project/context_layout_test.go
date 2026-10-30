package project

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveLeavesDeployedMemoryAlone(t *testing.T) {
	// Deploy must never rewrite memory. That includes files an older CLI seeded
	// with its own instructions: they are the agent's file now, and the runtime
	// states the policy regardless of what the file happens to contain.
	for _, content := range []string{
		"",
		"- favorite color: blue\n",
		"<!-- MANAGED-DEEPAGENTS MEMORY INSTRUCTIONS: DO NOT DELETE -->\n# Memory Instructions\n",
	} {
		assert.Equal(t, AgentMemorySeed{}, ResolveAgentMemorySeed(&content, nil))
	}
}

func TestResolveSeedsAnEmptyFileForAFreshDeploy(t *testing.T) {
	// The instructions live in the runtime, so a new deployment starts with a
	// file that holds nothing but what its agent goes on to write.
	assert.Equal(t, AgentMemorySeed{Write: true, Content: ""}, ResolveAgentMemorySeed(nil, nil))
	assert.Empty(t, DefaultMemoryContent)
}

func TestResolveMigratesLegacyFlatMemoryVerbatim(t *testing.T) {
	flat := "- from the flat layout\n"

	assert.Equal(t, AgentMemorySeed{Write: true, Content: flat}, ResolveAgentMemorySeed(nil, &flat))
}

func TestProjectContextRootIsTheAgentFilesDirectory(t *testing.T) {
	root := filepath.FromSlash("/proj")
	assert.Equal(t, root, ProjectContextRoot(&Project{Root: root, AgentFile: "agent.ts"}))
	assert.Equal(t, filepath.Join(root, "src"),
		ProjectContextRoot(&Project{Root: root, AgentFile: filepath.Join("src", "agent.ts")}))
}
