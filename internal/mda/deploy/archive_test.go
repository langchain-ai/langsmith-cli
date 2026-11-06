package deploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

func archivedPaths(t *testing.T, archive []byte) []string {
	t.Helper()
	decoder, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	reader := tar.NewReader(decoder)
	var paths []string
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		paths = append(paths, header.Name)
	}
	slices.Sort(paths)
	return paths
}

func TestCreateArchiveOmitsLocalEmitScratch(t *testing.T) {
	build := t.TempDir()
	writeFile(t, build, "langgraph.json", "{}\n")
	writeFile(t, build, "agent.py", "agent = {}\n")
	writeFile(t, build, proc.PythonEmitVenvDir+"/pyvenv.cfg", "home = /usr/bin\n")
	writeFile(t, build, proc.PythonEmitVenvDir+"/bin/python", "#!/bin/sh\n")
	writeFile(t, build, compile.PythonEmitSiteDir+"/pydantic/__init__.py", "")
	writeFile(t, build, "node_modules/left-pad/index.js", "")
	// Only the top-level install tree is local; a nested source dir that
	// happens to share the name still ships.
	writeFile(t, build, "vendor/node_modules/keep.js", "")

	archive, err := CreateArchive(build)
	require.NoError(t, err)

	assert.Equal(t, []string{"agent.py", "langgraph.json", "vendor/node_modules/keep.js"}, archivedPaths(t, archive.Bytes))
	assert.Equal(t, "langgraph.json", archive.ConfigRel)
}

func TestCreateArchiveOmitsTypescriptEmitNodeModules(t *testing.T) {
	build := t.TempDir()
	writeFile(t, build, "langgraph.json", "{}\n")
	writeFile(t, build, "package.json", "{}\n")
	writeFile(t, build, "package-lock.json", "{}\n")
	writeFile(t, build, "__runtime__/index.js", "export {};\n")
	writeFile(t, build, "node_modules/zod/package.json", "{}\n")
	writeFile(t, build, "node_modules/.bin/tsx", "#!/bin/sh\n")

	archive, err := CreateArchive(build)
	require.NoError(t, err)

	assert.Equal(t, []string{"__runtime__/index.js", "langgraph.json", "package-lock.json", "package.json"}, archivedPaths(t, archive.Bytes))
}
