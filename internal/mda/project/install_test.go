package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

// writeNpmPackage installs a stub `managed-deepagents` npm package of version
// into dir's node_modules.
func writeNpmPackage(t *testing.T, dir, version string) string {
	t.Helper()
	pkg := filepath.Join(dir, "node_modules", "managed-deepagents")
	writeFile(t, pkg, "package.json", `{"name":"managed-deepagents","version":"`+version+`"}`)
	writeFile(t, pkg, "dist/runtime/index.js", "export {};\n")
	return pkg
}

func TestLoadProjectResolvesTheNpmPackageAboveTheProject(t *testing.T) {
	workspace := canonicalTempDir(t)
	pkg := writeNpmPackage(t, workspace, "0.10.0")
	root := filepath.Join(workspace, "agents", "support")
	writeStubFile(t, root, "agent.ts")
	writeFile(t, root, "package.json", `{"dependencies":{"managed-deepagents":"0.10.0"}}`)

	p, installation, err := LoadProject(t.Context(), root)
	require.NoError(t, err)

	assert.Equal(t, root, p.Root)
	assert.Equal(t, filepath.Join(pkg, "dist"), installation.RuntimeSource())
	assert.Equal(t, "0.10.0", installation.PackageVersion())
}

func TestLoadProjectRejectsAnInstalledVersionTheManifestDoesNotPin(t *testing.T) {
	root := canonicalTempDir(t)
	writeNpmPackage(t, root, "0.10.0")
	writeStubFile(t, root, "agent.ts")
	writeFile(t, root, "package.json", `{"dependencies":{"managed-deepagents":"0.10.1"}}`)

	_, _, err := LoadProject(t.Context(), root)

	require.ErrorContains(t, err, "project declares 0.10.1, but 0.10.0 is installed")
}

func TestResolveInstallationAsksForNpmInstallWhenThePackageIsMissing(t *testing.T) {
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.ts")
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	require.EqualError(t, err, "managed-deepagents is not installed for the project at "+root+
		".\nInstall the project's dependencies with `npm install`, then try again.")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestResolveInstallationRejectsAnIncompleteNpmPackage(t *testing.T) {
	root := canonicalTempDir(t)
	pkg := writeNpmPackage(t, root, "0.10.0")
	require.NoError(t, os.Remove(filepath.Join(pkg, "dist", "runtime", "index.js")))
	writeStubFile(t, root, "agent.ts")
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	dist := filepath.Join(pkg, "dist")
	require.EqualError(t, err, "the managed-deepagents installation is incomplete: bundled runtime not found at "+
		dist+" (expected "+filepath.Join(dist, "runtime", "index.js")+"); reinstall it with `npm install`")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestResolveInstallationAsksForUvSyncWithoutAPythonEnvironment(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.py")
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	require.EqualError(t, err, "managed-deepagents is not installed for the project at "+root+
		".\nInstall the project's dependencies with `uv sync`, then try again.")
}

// pythonVenv creates a real `.venv` in root and returns its site-packages.
func pythonVenv(t *testing.T, root string) string {
	t.Helper()
	venv := filepath.Join(root, ".venv")
	output, err := proc.Output(t.Context(), exec.Command(proc.Python3Program(), "-m", "venv", "--without-pip", venv))
	require.NoError(t, err)
	require.True(t, output.Status.Success(), string(output.Stderr))
	sitePackages, err := filepath.Glob(filepath.Join(venv, "lib", "python3*", "site-packages"))
	require.NoError(t, err)
	windows, err := filepath.Glob(filepath.Join(venv, "Lib", "site-packages"))
	require.NoError(t, err)
	sitePackages = append(sitePackages, windows...)
	require.Len(t, sitePackages, 1)
	return sitePackages[0]
}

func TestResolveInstallationFindsThePackageInTheProjectVenv(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.py")
	sitePackages := pythonVenv(t, root)
	writeFile(t, sitePackages, "managed_deepagents/__init__.py", "raise ImportError('not imported')\n")
	writeFile(t, sitePackages, "managed_deepagents/runtime.py", "")
	writeFile(t, sitePackages, "managed_deepagents-0.10.0.dist-info/METADATA",
		"Metadata-Version: 2.1\nName: managed-deepagents\nVersion: 0.10.0\nRequires-Python: >=3.11\n"+
			"Requires-Dist: pydantic<2.15,>=2.7.4\n"+
			"Requires-Dist: langgraph-sdk[auth]<0.5,>=0.4.6\n"+
			"Requires-Dist: tomli>=2; python_version < \"3.12\"\n"+
			"Requires-Dist: pytest>=9.1.1; extra == \"test\"\n")
	p, err := Load(root)
	require.NoError(t, err)

	installation, err := ResolveInstallation(t.Context(), p)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(sitePackages, "managed_deepagents"), installation.RuntimeSource())
	assert.Equal(t, "0.10.0", installation.PackageVersion())
	assert.Equal(t, RuntimeManifest{
		Dependencies: []Dependency{
			{Name: "pydantic", Version: "<2.15,>=2.7.4"},
			{Name: "langgraph-sdk[auth]", Version: "<0.5,>=0.4.6"},
			{Name: "tomli", Version: `>=2; python_version < "3.12"`},
		},
		RequiresPython: ">=3.11",
	}, installation.RuntimeManifest())
}

func TestResolveInstallationRejectsAnUnsupportedPythonRelease(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.py")
	sitePackages := pythonVenv(t, root)
	writeFile(t, sitePackages, "managed_deepagents/runtime.py", "")
	writeFile(t, sitePackages, "managed_deepagents-0.9.5.dist-info/METADATA",
		"Metadata-Version: 2.1\nName: managed-deepagents\nVersion: 0.9.5\n")
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	require.EqualError(t, err, "managed-deepagents 0.9.5 is installed for the project at "+root+
		", but `langsmith deploy` builds managed-deepagents >=0.10.0, <0.11.0: the entry it generates "+
		"calls the runtime API of those releases.\nInstall a supported release with "+
		"`uv add 'managed-deepagents>=0.10.0,<0.11.0'`, or deploy with the SDK's own `mda deploy`.")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestResolveInstallationTakesAnEditablePythonInstallAsItIs(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.py")
	sitePackages := pythonVenv(t, root)
	writeFile(t, sitePackages, "managed_deepagents/runtime.py", "")
	writeFile(t, sitePackages, "managed_deepagents-0.9.5.dist-info/METADATA",
		"Metadata-Version: 2.1\nName: managed-deepagents\nVersion: 0.9.5\n")
	writeFile(t, sitePackages, "managed_deepagents-0.9.5.dist-info/direct_url.json",
		`{"url": "file:///src/managed-deepagents-sdk/packages/pypi", "dir_info": {"editable": true}}`)
	p, err := Load(root)
	require.NoError(t, err)

	installation, err := ResolveInstallation(t.Context(), p)

	require.NoError(t, err)
	assert.Equal(t, "0.9.5", installation.PackageVersion())
}

func TestResolveInstallationRejectsAnUnsupportedNpmRelease(t *testing.T) {
	root := canonicalTempDir(t)
	writeNpmPackage(t, root, "0.11.0-dev.1")
	writeStubFile(t, root, "agent.ts")
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	require.EqualError(t, err, "managed-deepagents 0.11.0-dev.1 is installed for the project at "+root+
		", but `langsmith deploy` builds managed-deepagents >=0.10.0, <0.11.0: the entry it generates "+
		"calls the runtime API of those releases.\nInstall a supported release with "+
		"`npm install managed-deepagents@0.10`, or deploy with the SDK's own `mda deploy`.")
}

func TestResolveInstallationReadsTheNpmRuntimeManifest(t *testing.T) {
	root := canonicalTempDir(t)
	pkg := writeNpmPackage(t, root, "0.10.0")
	writeFile(t, pkg, "package.json", `{
  "name": "managed-deepagents",
  "version": "0.10.0",
  "main": "./dist/index.js",
  "exports": {
    ".": {"types": "./types/index.d.ts", "default": "./dist/index.js"},
    "./runtime": {"types": "./dist/runtime/index.d.ts", "default": "./dist/runtime/index.js"},
    "./package.json": "./package.json"
  },
  "dependencies": {"langsmith": "^0.10.2", "@langchain/core": "1.2.17", "uuid": "^14.0.2"},
  "optionalDependencies": {"@langchain/managed-deepagents-darwin-arm64": "0.10.0"}
}`)
	writeStubFile(t, root, "agent.ts")
	p, err := Load(root)
	require.NoError(t, err)

	installation, err := ResolveInstallation(t.Context(), p)
	require.NoError(t, err)

	assert.Equal(t, RuntimeManifest{
		Dependencies: []Dependency{
			{Name: "langsmith", Version: "^0.10.2"},
			{Name: "@langchain/core", Version: "1.2.17"},
			{Name: "uuid", Version: "^14.0.2"},
		},
		Main:    "./index.js",
		Exports: []Export{{Subpath: ".", Target: "./index.js"}, {Subpath: "./runtime", Target: "./runtime/index.js"}},
	}, installation.RuntimeManifest())
}

func TestResolveInstallationAsksForUvSyncWhenTheVenvLacksThePackage(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	t.Setenv("CONDA_PREFIX", "")
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.py")
	pythonVenv(t, root)
	p, err := Load(root)
	require.NoError(t, err)

	_, err = ResolveInstallation(t.Context(), p)

	require.EqualError(t, err, "managed-deepagents is not installed for the project at "+root+
		".\nInstall the project's dependencies with `uv sync`, then try again.")
}
