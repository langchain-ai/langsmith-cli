package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// writeFile writes contents to root/rel, creating parent directories as
// needed.
func writeFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

// writeStubFile writes a placeholder file at root/rel, for tests that care
// about project layout rather than file contents.
func writeStubFile(t *testing.T, root, rel string) {
	t.Helper()
	writeFile(t, root, rel, "// stub\n")
}

// canonicalTempDir is a canonical temp root, so expected paths match what
// messages print.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := canonicalize(t.TempDir())
	require.NoError(t, err)
	return dir
}

func TestIsIgnoredDirMatchesKnownDirectories(t *testing.T) {
	for _, dir := range []string{
		"node_modules", ".git", "dist", ".venv", "venv", "__pycache__", ".pytest_cache",
		".agents", ".claude", ".cursor", ".factory", ".gemini", ".opencode",
	} {
		assert.True(t, IsIgnoredDir(filepath.Join("/proj", dir)), dir)
	}
	assert.False(t, IsIgnoredDir("/proj/src"))
	assert.False(t, IsIgnoredDir("/proj/tools"))
}

func TestIsIgnoredFileMatchesSecretEnvAndOSJunk(t *testing.T) {
	for _, path := range []string{
		"/proj/.env",
		"/proj/.env.local",
		"/proj/.env.production",
		"/proj/.DS_Store",
		"/proj/skills/foo/.DS_Store",
		"/proj/Thumbs.db",
		"/proj/desktop.ini",
		"/proj/__pycache__/mod.pyc",
	} {
		assert.True(t, IsIgnoredFile(path), path)
	}
	assert.False(t, IsIgnoredFile("/proj/env.example"))
	assert.False(t, IsIgnoredFile("/proj/package.json"))
}

func TestIsJunkPathSkipsOSCachesButNotSkillLayoutNames(t *testing.T) {
	assert.True(t, IsJunkDir("/proj/skills/foo/__pycache__"))
	assert.True(t, IsJunkDir("/proj/skills/foo/.pytest_cache"))
	assert.False(t, IsJunkDir("/proj/skills/release/build"))
	assert.False(t, IsJunkDir("/proj/skills/release/dist"))
	assert.False(t, IsJunkDir("/proj/skills/release/evals"))
	assert.False(t, IsJunkDir("/proj/skills/release/memories"))

	assert.True(t, IsJunkFile("/proj/skills/foo/.DS_Store"))
	assert.True(t, IsJunkFile("/proj/skills/foo/mod.pyc"))
	assert.False(t, IsJunkFile("/proj/skills/foo/.env"))
	assert.False(t, IsJunkFile("/proj/skills/release/build/steps.md"))
}

func TestLoadDiscoversAgentEntryAndFiles(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"agent.ts", "tools/query-db.ts", ".env", ".env.local", ".DS_Store", "Thumbs.db"} {
		writeStubFile(t, root, rel)
	}

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, TypeScript, p.Language)
	assert.Equal(t, "agent.ts", p.AgentFile)
	assert.Equal(t, []string{"agent.ts", filepath.Join("tools", "query-db.ts")}, p.Files)
}

func TestLoadSkipsOtherPackageManagerLockfiles(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.ts")
	writeStubFile(t, root, "package-lock.json")
	for _, name := range []string{"pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "npm-shrinkwrap.json"} {
		writeStubFile(t, root, name)
	}

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, []string{"agent.ts", "package-lock.json"}, p.Files)
}

func TestLoadReturnsFilesSorted(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"agent.ts", "z.ts", "a.ts", "a-c.ts", "a/b.ts"} {
		writeStubFile(t, root, rel)
	}

	p, err := Load(root)
	require.NoError(t, err)

	// Component by component, as Rust orders paths: `a/b.ts` before `a-c.ts`.
	assert.Equal(t, []string{filepath.Join("a", "b.ts"), "a-c.ts", "a.ts", "agent.ts", "z.ts"}, p.Files)
}

func TestLoadSkipsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"agent.ts",
		"node_modules/dep/index.ts",
		"dist/old.ts",
		"mda_evals/smoke/task/tests/answer.test.ts",
		"evals/smoke/task/tests/answer.test.ts",
		"memories/AGENTS.md",
	} {
		writeStubFile(t, root, rel)
	}

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, []string{"agent.ts"}, p.Files)
}

func TestLoadWalksAProjectWhoseRootHasAnIgnoredName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "evals")
	writeStubFile(t, root, "agent.ts")

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, []string{"agent.ts"}, p.Files)
}

func TestLoadDetectsPythonProjects(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.py")
	writeStubFile(t, root, "tools/query_db.py")

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, Python, p.Language)
	assert.Equal(t, "agent.py", p.AgentFile)
}

func TestLoadPrefersAgentTSOverAgentTSX(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.ts")
	writeStubFile(t, root, "agent.tsx")

	p, err := Load(root)
	require.NoError(t, err)

	assert.Equal(t, "agent.ts", p.AgentFile)
}

func TestLoadRejectsEntriesForBothLanguages(t *testing.T) {
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.ts")
	writeStubFile(t, root, "agent.py")

	_, err := Load(root)

	require.EqualError(t, err, "ambiguous project in "+root+
		": found both TypeScript and Python agent entry files")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestValidateAuthoringPackageVersionRejectsTypeScriptMismatch(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.ts")
	writeFile(t, root, "package.json", `{"dependencies":{"managed-deepagents":"0.3.0"}}`)
	p, err := Load(root)
	require.NoError(t, err)

	err = ValidateAuthoringPackageVersion(p, "0.4.0")

	require.EqualError(t, err, "managed-deepagents version mismatch: project declares 0.3.0, "+
		"but 0.4.0 is installed.\n\n"+
		"The project's installed dependencies are out of date. Install them again:\n\n"+
		"  npm install")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestValidateAuthoringPackageVersionRejectsPythonMismatch(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.py")
	writeFile(t, root, "pyproject.toml", "[project]\ndependencies = [\"managed-deepagents==0.3.0\"]\n")
	p, err := Load(root)
	require.NoError(t, err)

	err = ValidateAuthoringPackageVersion(p, "0.4.0")

	require.EqualError(t, err, "managed-deepagents version mismatch: project declares 0.3.0, "+
		"but 0.4.0 is installed.\n\n"+
		"The project's installed dependencies are out of date. Install them again:\n\n"+
		"  uv sync")
}

func TestValidateAuthoringPackageVersionAcceptsMatchingPins(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.py")
	writeFile(t, root, "pyproject.toml", "[project]\ndependencies = [\"Managed_DeepAgents == 0.4.0\"]\n")
	p, err := Load(root)
	require.NoError(t, err)

	assert.NoError(t, ValidateAuthoringPackageVersion(p, "0.4.0"))
}

func TestValidateAuthoringPackageVersionIgnoresRanges(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "agent.ts")
	writeFile(t, root, "package.json", `{"dependencies":{"managed-deepagents":"^0.3.0"}}`)
	p, err := Load(root)
	require.NoError(t, err)

	assert.NoError(t, ValidateAuthoringPackageVersion(p, "0.4.0"))
}

func TestLoadErrorsWhenNoAgentEntryExists(t *testing.T) {
	root := t.TempDir()
	writeStubFile(t, root, "index.ts")

	_, err := Load(root)

	require.ErrorContains(t, err,
		"is not a Managed Deep Agent project (no agent.ts, agent.tsx, or agent.py found)")
	assert.Equal(t, errs.Project, errs.CategoryOf(err))
}

func TestMissingProjectSuggestsCdIntoTheOnlyChildProject(t *testing.T) {
	dir := canonicalTempDir(t)
	writeStubFile(t, dir, "my-agent/agent.ts")
	writeStubFile(t, dir, "notes/readme.md")

	message := notAProjectMessage(dir, dir)

	assert.True(t, strings.HasSuffix(message, "\nFound a project in my-agent. Run `cd my-agent` first."), message)
}

func TestMissingProjectSpellsTheChildAbsolutelyOutsideTheWorkingDirectory(t *testing.T) {
	dir := canonicalTempDir(t)
	writeStubFile(t, dir, "my-agent/agent.py")

	message := notAProjectMessage(dir, "")

	child := filepath.Join(dir, "my-agent")
	assert.Contains(t, message, "Run `cd "+shellWord(child)+"` first.")
}

func TestMissingProjectDoesNotGuessBetweenSeveralChildProjects(t *testing.T) {
	dir := canonicalTempDir(t)
	writeStubFile(t, dir, "first/agent.ts")
	writeStubFile(t, dir, "second/agent.py")

	message := notAProjectMessage(dir, dir)

	assert.True(t, strings.HasSuffix(message,
		"\nRun `mda init <name>` to create one, or `cd` into an existing project."), message)
}

func TestMissingProjectIgnoresEntriesUnderDependencyAndHiddenDirs(t *testing.T) {
	dir := canonicalTempDir(t)
	writeStubFile(t, dir, "node_modules/pkg/agent.ts")
	writeStubFile(t, dir, ".cache/agent.py")

	assert.Nil(t, findNearbyProject(dir))
}

func TestMissingProjectPointsBackUpToTheEnclosingProject(t *testing.T) {
	root := canonicalTempDir(t)
	writeStubFile(t, root, "agent.ts")
	writeStubFile(t, root, "tools/search/index.ts")
	dir := filepath.Join(root, "tools", "search")

	message := notAProjectMessage(dir, dir)

	assert.True(t, strings.HasSuffix(message, "\nThis directory is inside the project at "+root+
		". Run `cd "+filepath.Join("..", "..")+"` first."), message)
}

func TestLoadErrorsWhenRootIsNotADirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir.ts")
	require.NoError(t, os.WriteFile(file, []byte("// stub\n"), 0o644))

	_, err := Load(file)

	require.EqualError(t, err, "project root "+file+" is not a directory")
}

func TestLanguageSpellsAuthoredCode(t *testing.T) {
	assert.Equal(t, `name: "research-assistant"`, TypeScript.StringArgument("name", "research-assistant"))
	assert.Equal(t, `name="research-assistant"`, Python.StringArgument("name", "research-assistant"))
	assert.Equal(t, "define_deep_agent", Python.Ident("defineDeepAgent", "define_deep_agent"))
	assert.Equal(t, []string{"TypeScript", "ts", "package.json"},
		[]string{TypeScript.Label(), TypeScript.Extension(), TypeScript.ManifestName()})
	assert.Equal(t, []string{"Python", "py", "pyproject.toml"},
		[]string{Python.Label(), Python.Extension(), Python.ManifestName()})
}

func TestComparePathsOrdersByComponent(t *testing.T) {
	paths := []string{"a-c", filepath.Join("a", "b"), "a"}
	slices.SortFunc(paths, comparePaths)
	assert.Equal(t, []string{"a", filepath.Join("a", "b"), "a-c"}, paths)
}
