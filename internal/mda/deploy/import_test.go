package deploy

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Imports a project against the stub `managed-deepagents` introspect's tests
// use. The TypeScript helper installs into the CLI's cache, as it does for
// `langsmith deploy`.
func TestImportProjectModulesShowsProgressAndHandsOverTheEnv(t *testing.T) {
	output := unattended(t)
	root := t.TempDir()
	writeFile(t, root, "package.json", `{"name": "fixture", "type": "module"}`+"\n")
	writeFile(t, root, "agent.ts", "import { defineDeepAgent } from \"managed-deepagents\";\n"+
		"export const agent = defineDeepAgent({ name: process.env.MDA_DEPLOY_TEST_NAME });\n")
	stub := filepath.Join("..", "introspect", "testdata", "npm", "managed-deepagents")
	require.NoError(t, filepath.WalkDir(stub, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(stub, path)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, "node_modules", "managed-deepagents"), filepath.ToSlash(rel), string(contents))
		return nil
	}))
	p, err := project.Load(root)
	require.NoError(t, err)

	require.NoError(t, importProjectModules(t.Context(), p, map[string]string{"MDA_DEPLOY_TEST_NAME": "from-dotenv"}))

	config, err := introspect.ReadAgentStaticConfig(t.Context(), p)
	require.NoError(t, err)
	assert.Equal(t, "from-dotenv", config.AssistantID)
	assert.Contains(t, output.String(), "Importing project modules…")
	assert.Contains(t, output.String(), "Imported project modules")
}
