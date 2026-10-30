package proc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNpmCandidatesPreferCmdOnWindows(t *testing.T) {
	assert.Equal(t, []string{"npm.cmd", "npm"}, npmCandidates(true))
	assert.Equal(t, []string{"npm"}, npmCandidates(false))
}

func TestNpxCandidatesPreferCmdOnWindows(t *testing.T) {
	assert.Equal(t, []string{"npx.cmd", "npx"}, npxCandidates(true))
	assert.Equal(t, []string{"npx"}, npxCandidates(false))
}

func TestPython3CandidatesIncludePythonOnWindows(t *testing.T) {
	assert.Equal(t, []string{"python3", "python"}, python3Candidates(true))
	assert.Equal(t, []string{"python3"}, python3Candidates(false))
}

func TestResolvedProgramsAreCachedCandidates(t *testing.T) {
	assert.Contains(t, npmCandidates(isWindows), NpmProgram())
	assert.Contains(t, npxCandidates(isWindows), NpxProgram())
	assert.Contains(t, python3Candidates(isWindows), Python3Program())
}

// writeUsableVenvInterpreter writes an interpreter that answers `--version`
// where a venv keeps it, and returns its path.
func writeUsableVenvInterpreter(t *testing.T, venv string) string {
	t.Helper()
	python := venvInterpreterCandidates(venv, isWindows)[0]
	require.NoError(t, os.MkdirAll(filepath.Dir(python), 0o755))
	writeUsablePythonStub(t, python)
	return python
}

func writeVenvPython(t *testing.T, venv string, usable bool) string {
	t.Helper()
	if usable {
		return writeUsableVenvInterpreter(t, venv)
	}
	python := venvInterpreterCandidates(venv, isWindows)[0]
	require.NoError(t, os.MkdirAll(filepath.Dir(python), 0o755))
	require.NoError(t, os.WriteFile(python, nil, 0o644))
	return python
}

func TestProjectPythonPrefersDotVenvOverVirtualEnv(t *testing.T) {
	root := t.TempDir()
	expected := writeVenvPython(t, filepath.Join(root, ".venv"), true)
	writeVenvPython(t, filepath.Join(root, "other-env"), true)

	resolved, err := resolveProjectPython(t.Context(), root, filepath.Join(root, "other-env"), "")

	require.NoError(t, err, "project .venv")
	assert.Equal(t, expected, resolved)
}

func TestProjectPythonUsesVirtualEnvWhenDotVenvIsMissing(t *testing.T) {
	root := t.TempDir()
	env := filepath.Join(root, "custom-env")
	expected := writeVenvPython(t, env, true)

	resolved, err := resolveProjectPython(t.Context(), root, env, "")

	require.NoError(t, err, "VIRTUAL_ENV")
	assert.Equal(t, expected, resolved)
}

func TestProjectPythonSkipsUnusableDotVenvForVirtualEnv(t *testing.T) {
	root := t.TempDir()
	writeVenvPython(t, filepath.Join(root, ".venv"), false)
	env := filepath.Join(root, "active-env")
	expected := writeVenvPython(t, env, true)

	resolved, err := resolveProjectPython(t.Context(), root, env, "")

	require.NoError(t, err, "working VIRTUAL_ENV after stale .venv")
	assert.Equal(t, expected, resolved)
}

func TestProjectPythonDoesNotFallBackToPathPython(t *testing.T) {
	root := t.TempDir()
	writeVenvPython(t, filepath.Join(root, ".venv"), false)

	_, err := resolveProjectPython(t.Context(), root, "", "")

	assert.EqualError(t, err, "no usable Python virtualenv for connector discovery\n"+
		"Looked for "+filepath.Join(root, ".venv")+", $VIRTUAL_ENV and $CONDA_PREFIX.\n"+
		"Run `uv sync` so `.venv` exists, or invoke `langsmith` via `uv run`.")
}

func TestProjectPythonUsesTheCondaEnvWithoutAVirtualenv(t *testing.T) {
	root := t.TempDir()
	conda := filepath.Join(root, "conda-env")
	expected := condaInterpreterCandidates(conda, isWindows)[0]
	require.NoError(t, os.MkdirAll(filepath.Dir(expected), 0o755))
	writeUsablePythonStub(t, expected)

	resolved, err := resolveProjectPython(t.Context(), root, filepath.Join(root, "missing-env"), conda)

	require.NoError(t, err, "CONDA_PREFIX")
	assert.Equal(t, expected, resolved)
}

func TestProjectPythonPrefersVirtualEnvOverCondaEnv(t *testing.T) {
	root := t.TempDir()
	env := filepath.Join(root, "active-env")
	expected := writeVenvPython(t, env, true)
	conda := filepath.Join(root, "conda-env")
	python := condaInterpreterCandidates(conda, isWindows)[0]
	require.NoError(t, os.MkdirAll(filepath.Dir(python), 0o755))
	writeUsablePythonStub(t, python)

	resolved, err := resolveProjectPython(t.Context(), root, env, conda)

	require.NoError(t, err, "VIRTUAL_ENV before CONDA_PREFIX")
	assert.Equal(t, expected, resolved)
}
