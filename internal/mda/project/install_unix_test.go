//go:build unix

package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInstallationTakesALinkedNpmPackageAsItIs(t *testing.T) {
	workspace := canonicalTempDir(t)
	source := filepath.Join(workspace, "managed-deepagents-sdk", "packages", "npm")
	writeFile(t, source, "package.json", `{"name":"managed-deepagents","version":"0.9.5"}`)
	writeFile(t, source, "dist/runtime/index.js", "export {};\n")
	root := filepath.Join(workspace, "agent")
	writeStubFile(t, root, "agent.ts")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "node_modules"), 0o755))
	require.NoError(t, os.Symlink(source, filepath.Join(root, "node_modules", "managed-deepagents")))
	p, err := Load(root)
	require.NoError(t, err)

	installation, err := ResolveInstallation(t.Context(), p)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(source, "dist"), installation.RuntimeSource())
}
