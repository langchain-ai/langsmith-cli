//go:build unix

package deploy

// Creating a link needs privileges on Windows, so these run where links are
// ordinary files.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectLocalContextFollowsASymlinkedSkillsDirectory(t *testing.T) {
	root := t.TempDir()
	shared := t.TempDir()
	writeFile(t, root, "agent.ts", "export const agent = {};\n")
	writeFile(t, shared, "research/SKILL.md", "Research skill.\n")
	writeFile(t, shared, "elsewhere.md", "Linked from inside.\n")
	require.NoError(t, os.Symlink(filepath.Join(shared, "elsewhere.md"), filepath.Join(shared, "research", "linked.md")))
	require.NoError(t, os.Symlink(shared, filepath.Join(root, "skills")))

	local, err := collectLocalContext(tsProject(root, "agent.ts"), true)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"skills/elsewhere.md":      "Linked from inside.\n",
		"skills/research/SKILL.md": "Research skill.\n",
	}, local.files)
	assert.True(t, local.hasSkills)
}
