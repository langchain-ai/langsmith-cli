package compile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

func TestLanggraphAPIVersionDefaultsToTheDevelopmentRelease(t *testing.T) {
	if LanggraphAPIVersion != "0.18.0.dev12" {
		t.Fatalf("got %s", LanggraphAPIVersion)
	}
	for _, override := range []string{"", "   "} {
		if got, err := resolveLanggraphAPIVersion(override); err != nil || got != LanggraphAPIVersion {
			t.Fatalf("%q: got %q, %v", override, got, err)
		}
	}
}

func TestLanggraphAPIVersionAcceptsATrimmedStemOverride(t *testing.T) {
	for _, override := range []string{"0.13.0rc1", "  0.13.0rc1  "} {
		if got, err := resolveLanggraphAPIVersion(override); err != nil || got != "0.13.0rc1" {
			t.Fatalf("%q: got %q, %v", override, got, err)
		}
	}
}

func TestLanggraphAPIVersionRejectsTagsAndWhitespace(t *testing.T) {
	for override, want := range map[string]string{
		"0.13.0 rc1":                          "must not contain whitespace",
		"0.13.0rc1-node22":                    "version stem only",
		"0.13.0rc1-py3.11":                    "version stem only",
		"registry.example/langgraph-api:0.13": "version stem only",
	} {
		_, err := resolveLanggraphAPIVersion(override)
		requireErrorContains(t, err, want)
	}
}

func TestResolvedLanggraphAPIVersionReadsTheEnvironment(t *testing.T) {
	t.Setenv(LanggraphAPIVersionEnv, "0.13.0rc1")
	if got, err := ResolvedLanggraphAPIVersion(); err != nil || got != "0.13.0rc1" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func ptr(value string) *string {
	return &value
}

func TestMissingRequirementsUseTheNewestSupportedRuntime(t *testing.T) {
	for _, spec := range []*string{nil, ptr(" ")} {
		resolved, err := resolvePythonVersion(spec, nil, nil)
		if err != nil || resolved != (ResolvedPython{Version: "3.14"}) {
			t.Fatalf("%v: got %+v, %v", spec, resolved, err)
		}
	}
	if defaultPythonVersion != supportedPythonVersions[len(supportedPythonVersions)-1] {
		t.Fatal("the default is not the newest supported runtime")
	}
}

func TestRequirementsSelectTheNewestCompatibleSeriesAndWarnOnlyForPatchConstraints(t *testing.T) {
	for _, tc := range []struct {
		spec    string
		version string
		warns   bool
	}{
		{">=3.11", "3.14", false},
		{">=3.9", "3.14", false},
		{">=3.12,<3.13", "3.12", false},
		{"==3.13.*", "3.13", false},
		{"~=3.12.0", "3.12", false},
		{">=3.11,!=3.14.*", "3.13", false},
		{">=3.11,!=3.14.*,!=3.13.*", "3.12", false},
		{">=3.11.0", "3.14", false},
		{">=3.13.2", "3.14", false},
		{">3.12,<3.13", "3.12", true},
		{">=3.13.2,<3.14", "3.13", true},
		{"<=3.12.1", "3.12", true},
		{">=3.11,<3.11.100", "3.11", true},
		{">=3.11.1,<3.11.100", "3.11", true},
		{">=3.11.10000,<3.12", "3.11", true},
		{"==3.13", "3.13", true},
		{"==3.11.16", "3.11", true},
		{">=3.11,!=3.14.16", "3.14", true},
		{">= 3.11, < 3.13", "3.12", false},
	} {
		resolved, err := resolvePythonVersion(ptr(tc.spec), nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.spec, err)
		}
		if resolved.Version != tc.version || (resolved.Warning != "") != tc.warns {
			t.Fatalf("%s: got %+v", tc.spec, resolved)
		}
	}
}

func TestTheEnvironmentOverridesTheProjectTargetAndABlankEnvironmentFallsBack(t *testing.T) {
	for _, tc := range []struct {
		configured, environment *string
		want                    string
	}{
		{ptr("3.12"), nil, "3.12"},
		{ptr("3.12"), ptr("3.13"), "3.13"},
		{ptr("3.12"), ptr(" "), "3.12"},
		{nil, ptr("3.13"), "3.13"},
		{nil, ptr(" "), "3.14"},
	} {
		resolved, err := resolvePythonVersion(ptr(">=3.11"), tc.configured, tc.environment)
		if err != nil || resolved.Version != tc.want {
			t.Fatalf("%v, %v: got %+v, %v", tc.configured, tc.environment, resolved, err)
		}
	}
}

func TestExplicitTargetsObeyRequirementsAndPreservePatchWarnings(t *testing.T) {
	for _, tc := range []struct{ configured, environment *string }{{ptr("3.13"), nil}, {nil, ptr("3.13")}} {
		_, err := resolvePythonVersion(ptr(">=3.14"), tc.configured, tc.environment)
		requireErrorContains(t, err, "contradicts requires-python")
		resolved, err := resolvePythonVersion(ptr(">=3.13.2"), tc.configured, tc.environment)
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Warning != "requires-python \">=3.13.2\" constrains the patch level, which LangGraph images "+
			"do not pin; building Python 3.13, whose patch comes from the image and may not satisfy it" {
			t.Fatalf("warning: %q", resolved.Warning)
		}
	}
	_, err := resolvePythonVersion(ptr(">=3.14"), ptr("3.13"), nil)
	if err == nil || err.Error() != "[tool.mda] python-version=3.13 contradicts requires-python \">=3.14\" in "+
		"pyproject.toml; `pip install -e .` would fail in the built image" {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidOrUnsatisfiableRequirementsFail(t *testing.T) {
	for spec, want := range map[string]string{
		"not a specifier": "failed to parse requires-python",
		">=3.9,<3.11":     "3.11, 3.12, 3.13, 3.14",
		">3.11.1,<3.11.2": "allows no Python",
		"3.11":            "failed to parse requires-python",
		">=3.11 || <3.9":  "failed to parse requires-python",
		">=3.11,":         "failed to parse requires-python",
		">=3.11.*":        "failed to parse requires-python",
		"=3.11":           "failed to parse requires-python",
		">=3.9,<3.11,!=x": "failed to parse requires-python",
	} {
		_, err := resolvePythonVersion(ptr(spec), nil, nil)
		requireErrorContains(t, err, want)
	}
	_, err := resolvePythonVersion(ptr(">=3.9,<3.11"), nil, nil)
	if err == nil || err.Error() != "requires-python \">=3.9,<3.11\" in pyproject.toml allows no Python that "+
		"LangGraph ships an image for (3.11, 3.12, 3.13, 3.14)" {
		t.Fatalf("got %v", err)
	}
}

func TestExplicitTargetsRequireASupportedMinorVersion(t *testing.T) {
	for _, raw := range []string{"3.10", "3.15", "3.14.1", ">=3.14", "3", "3.13-wolfi", "../3.13", "3.13:latest", "3.13@sha256:0000"} {
		for _, tc := range []struct {
			configured, environment *string
			source                  string
		}{
			{ptr(raw), nil, "[tool.mda] python-version"},
			{nil, ptr(raw), PythonVersionEnv},
		} {
			_, err := resolvePythonVersion(nil, tc.configured, tc.environment)
			requireErrorContains(t, err, tc.source)
		}
	}
	if _, err := resolvePythonVersion(nil, ptr(""), nil); err == nil {
		t.Fatal("an empty configured version was accepted")
	}
	_, err := resolvePythonVersion(nil, ptr("3.15"), nil)
	if err == nil || err.Error() != "Python 3.15 from [tool.mda] python-version is not supported; "+
		"LangGraph ships images for 3.11, 3.12, 3.13, 3.14" {
		t.Fatalf("got %v", err)
	}
}

func TestBuildResultUsesInitActionVocabularyAndHidesInternalDetails(t *testing.T) {
	want := []ui.SequenceItem{
		ui.NewSection("Managed app compiled", ".mda/build"),
		ui.NewSection("Run locally", "mda dev"),
		ui.NewSection("Deploy when ready", "langsmith deploy"),
	}
	if got := buildResultItems(".mda/build", ".", nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestBuildResultQualifiesCommandsForAnExplicitProjectPath(t *testing.T) {
	want := []ui.SequenceItem{
		ui.NewSection("Managed app compiled", "agents/research/.mda/build"),
		ui.NewSection("Run locally", "mda dev agents/research"),
		ui.NewSection("Deploy when ready", "langsmith deploy agents/research"),
	}
	if got := buildResultItems("agents/research/.mda/build", "agents/research", nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestBuildResultKeepsCompilerWarningsInTheSequence(t *testing.T) {
	want := []ui.SequenceItem{
		ui.NewSection("Managed app compiled", ".mda/build"),
		ui.NewSection("Build warnings", "Public HTTP route found"),
		ui.NewSection("Run locally", "mda dev"),
		ui.NewSection("Deploy when ready", "langsmith deploy"),
	}
	if got := buildResultItems(".mda/build", ".", []string{"Public HTTP route found"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestACompileCancellationStaysACancellation(t *testing.T) {
	interrupted := fmt.Errorf("failed to run npm install: %w", context.Canceled)

	if err := Compilation(interrupted); err != interrupted || !errs.IsCancellation(err) {
		t.Fatalf("got %v", err)
	}
}

func TestACompileFailureKeepsTheChildsStderr(t *testing.T) {
	err := Compilation(errs.StderrFailed([]byte("ImportError: x\n"), "failed to emit connector requirements"))

	if errs.CategoryOf(err) != errs.Compilation {
		t.Fatalf("category: %v", errs.CategoryOf(err))
	}
	if err.Error() != "failed to emit connector requirements: ImportError: x" {
		t.Fatalf("got %q", err.Error())
	}
	var stderr errs.ChildStderr
	if !errors.As(err, &stderr) || string(stderr) != "ImportError: x" {
		t.Fatalf("stderr: %q", stderr)
	}
}

func TestWriteIfChangedPreservesTheModificationTimeWhenBytesMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "langgraph.json")
	writeFile(t, filepath.Dir(path), "langgraph.json", "{\"graphs\":{}}\n")
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	modified := func() time.Time {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		return info.ModTime()
	}

	if wrote, err := writeIfChanged(path, []byte("{\"graphs\":{}}\n")); err != nil || wrote {
		t.Fatalf("got %v, %v", wrote, err)
	}
	if !modified().Equal(past) {
		t.Fatal("an unchanged write touched the file")
	}
	if wrote, err := writeIfChanged(path, []byte("{\"graphs\":{\"a\":\"./x\"}}\n")); err != nil || !wrote {
		t.Fatalf("got %v, %v", wrote, err)
	}
	if modified().Equal(past) {
		t.Fatal("a changed write did not touch the file")
	}
}

func TestCompileKeepsDependencyCachesButDropsStagedEnvAndStaleSources(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "tools/query.ts": "export const q = 1;\n"})
	mustCompile(t, f.deploy)

	// Simulate a prior install, emit site, and staged env.
	writeFiles(t, f.outDir, map[string]string{
		"node_modules/left-pad/index.js":            "module.exports=1\n",
		PythonEmitSiteDir + "/pydantic/__init__.py": "",
		".env":                 "LANGSMITH_API_KEY=test\n",
		".langgraph_api/state": "",
	})
	if err := os.Remove(filepath.Join(f.root(), "tools", "query.ts")); err != nil {
		t.Fatal(err)
	}
	f.reload(t)
	mustCompile(t, f.deploy)

	if !f.exists("node_modules/left-pad/index.js") || !f.exists(PythonEmitSiteDir+"/pydantic") || !f.exists("agent.ts") {
		t.Fatal("a dependency cache was dropped")
	}
	if f.exists("tools/query.ts") || f.exists(".env") || f.exists(".langgraph_api") {
		t.Fatal("stale output survived")
	}
}

func TestCompileReplacesABuildDirectoryWrittenBeforeTheStampExisted(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})
	f.outDir = filepath.Join(f.root(), "build")
	// A build from a CLI version that predates the stamp: generated
	// artifacts, no marker file.
	writeFiles(t, f.outDir, map[string]string{
		"langgraph.json": "{}",
		"_mda_entry.ts":  "// old entry\n",
		"stale.txt":      "old",
	})

	mustCompile(t, f.deploy)

	if f.exists("stale.txt") || !isFile(filepath.Join(f.outDir, buildMarker)) {
		t.Fatal("the old build was not replaced")
	}
}

func TestCompileRefusesAnOutDirItDidNotWrite(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})
	f.outDir = filepath.Join(f.root(), "notes")
	writeFiles(t, f.outDir, map[string]string{"NOTES.md": "keep me\n", "nested/.env": "OPENAI_API_KEY=secret\n"})

	_, err := f.deploy(t)

	if err == nil || err.Error() != "refusing to clean build directory "+f.outDir+": it is not empty and was not "+
		"written by `mda`. Point --out at an empty or previously built directory, or remove this one yourself." {
		t.Fatalf("got %v", err)
	}
	if f.read(t, "NOTES.md") != "keep me\n" || !f.exists("nested/.env") {
		t.Fatal("the directory was changed")
	}
}

func TestCompileAcceptsAnEmptyOutDir(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})
	f.outDir = filepath.Join(f.root(), "empty")
	if err := os.MkdirAll(f.outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustCompile(t, f.deploy)

	if !f.exists("_mda_entry.ts") {
		t.Fatal("no build")
	}
}

func TestCompileRefusesAnOutDirEqualToTheProjectRoot(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent, ".env": "OPENAI_API_KEY=secret\n"})
	f.outDir = f.root()

	_, err := f.deploy(t)

	if err == nil || err.Error() != "refusing to build into the project directory "+f.root()+": the build directory "+
		"is emptied first, which would delete the project. Pass --out a separate directory." {
		t.Fatalf("got %v", err)
	}
	if !isFile(filepath.Join(f.root(), "agent.ts")) || !isFile(filepath.Join(f.root(), ".env")) {
		t.Fatal("the project was changed")
	}
}

func TestCompileRefusesAnOutDirThatContainsTheProject(t *testing.T) {
	dir := canonicalTempDir(t)
	f := typescriptProjectAt(t, filepath.Join(dir, "project"), map[string]string{"agent.ts": tsAgent})
	f.outDir = dir

	_, err := f.deploy(t)

	if err == nil || !strings.HasPrefix(err.Error(), "refusing to build into "+dir+": it contains the project at ") {
		t.Fatalf("got %v", err)
	}
	if !isFile(filepath.Join(f.root(), "agent.ts")) {
		t.Fatal("the project was changed")
	}
}

func TestCompileGeneratesAPythonEntry(t *testing.T) {
	f := pythonProject(t, map[string]string{"agent.py": pyAgent, "instructions.md": instructionMD})

	summary := mustCompile(t, f.deploy)

	if summary.AssistantID != "agent" || f.read(t, "agent.py") != pyAgent {
		t.Fatalf("summary: %+v", summary)
	}
	want := `# Generated by ` + "`mda deploy`" + ` — do not edit.
#
# Wraps the user's ` + "`define_deep_agent(...)`" + ` definition into a managed LangGraph
# graph factory. The platform-owned backend (configured sandbox when declared,
# otherwise StateBackend scratch), store, checkpointer, and runtime context are
# injected by the managed runtime, not by the agent author.
from contextlib import asynccontextmanager

from managed_deepagents.runtime import compile_managed_agent_async, parent_run
from managed_deepagents._static_entry import assert_static_agent_definition
from langgraph_sdk.runtime import ServerRuntime

from agent import agent as _definition

_context_repo = "context-repo"
_has_skills = False

_async_subagent_graphs = {"general-purpose": "agent--general-purpose"}

@asynccontextmanager
async def agent(config, runtime: ServerRuntime):
    assert_static_agent_definition(_definition)
    async with parent_run(config):
        yield await compile_managed_agent_async(_definition, config, context_repo=_context_repo, has_skills=_has_skills, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)


@asynccontextmanager
async def agent__general_purpose(config, runtime: ServerRuntime):
    async with parent_run(config):
        yield await compile_managed_agent_async(_definition, config, context_repo=_context_repo, has_skills=_has_skills, runtime=runtime, async_subagent="general-purpose")
`
	if got := f.read(t, "_mda_entry.py"); got != want {
		t.Fatalf("_mda_entry.py:\n%s", got)
	}
	manifest := readJSON(t, f.read(t, "langgraph.json"))
	if manifest["python_version"] != "3.14" || manifest["_INTERNAL_docker_tag"] != LanggraphAPIVersion+"-py3.14" {
		t.Fatalf("langgraph.json: %v", manifest)
	}
}

func TestCompileBuildsAGraphPerAsyncSubagent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		project func(*testing.T, map[string]string) fixture
		files   map[string]string
		graphs  map[string]any
		entry   []string
	}{
		{"python", pythonProject, map[string]string{
			"agent.py": pyImport + "agent = define_deep_agent(name=\"lead\", subagents=[\n" +
				"    {\"name\": \"researcher\", \"description\": \"d\", \"async\": True},\n" +
				"    {\"name\": \"helper\", \"description\": \"d\"},\n])\n",
		}, map[string]any{
			"lead":                  "./_mda_entry.py:agent",
			"lead--researcher":      "./_mda_entry.py:agent__researcher",
			"lead--general-purpose": "./_mda_entry.py:agent__general_purpose",
		}, []string{
			"_async_subagent_graphs = {\"researcher\": \"lead--researcher\", \"general-purpose\": \"lead--general-purpose\"}",
			"async_subagent_graphs=_async_subagent_graphs",
			"@asynccontextmanager\nasync def agent__researcher(config, runtime: ServerRuntime):\n    async with parent_run(config):",
			"async_subagent=\"researcher\")",
		}},
		{"typescript", typescriptProject, map[string]string{
			"agent.ts": tsImport + "export const agent = defineDeepAgent({ name: \"lead\", subagents: [\n" +
				"  { name: \"researcher\", description: \"d\", async: true },\n" +
				"  { name: \"helper\", description: \"d\" },\n] });\n",
		}, map[string]any{
			"lead":                  "./_mda_entry.ts:agent",
			"lead--researcher":      "./_mda_entry.ts:agent__researcher",
			"lead--general-purpose": "./_mda_entry.ts:agent__general_purpose",
		}, []string{
			"const asyncSubagentGraphs = { \"researcher\": \"lead--researcher\", \"general-purpose\": \"lead--general-purpose\" };",
			"asyncSubagentGraphs }, runtime);",
			"export function agent__researcher(config: ManagedRunConfig, runtime: ManagedServerRuntime) {",
			"asyncSubagent: \"researcher\" }, runtime);",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.project(t, tc.files)

			mustCompile(t, f.deploy)

			if graphs := readJSON(t, f.read(t, "langgraph.json"))["graphs"]; !reflect.DeepEqual(graphs, tc.graphs) {
				t.Fatalf("graphs: %v", graphs)
			}
			requireContains(t, f.read(t, "_mda_entry."+f.project.Language.Extension()), tc.entry...)
		})
	}
}

func TestCompileIgnoresPythonConfigForTypescript(t *testing.T) {
	// A TypeScript manifest keys the runtime on `node_version`, so Python
	// configuration is irrelevant to it. An unparseable `requires-python` at
	// the project root must not fail a build that never reads it.
	t.Setenv(PythonVersionEnv, "not a version")
	f := typescriptProject(t, map[string]string{
		"agent.ts":       tsAgent,
		"pyproject.toml": "[project]\nname = \"unrelated\"\nrequires-python = \"not a specifier\"\n",
	})

	mustCompile(t, f.deploy)

	manifest := readJSON(t, f.read(t, "langgraph.json"))
	if manifest["node_version"] != "22" {
		t.Fatalf("langgraph.json: %v", manifest)
	}
	if _, ok := manifest["python_version"]; ok {
		t.Fatalf("langgraph.json: %v", manifest)
	}
}

func TestCompileAppliesPythonVersionConfiguration(t *testing.T) {
	for _, tc := range []struct {
		requirement, target, version string
		warns                        bool
	}{
		{">=3.11", "", "3.14", false},
		{">=3.11,<3.14", "", "3.13", false},
		{">=3.13.2,<3.14", "", "3.13", true},
		{">=3.11", "[tool.mda]\npython-version = \"3.12\"\n", "3.12", false},
		{">=3.9,<3.11", "", "", false},
	} {
		pyproject := "[project]\nname = \"agent\"\nversion = \"0.0.0\"\nrequires-python = \"" + tc.requirement + "\"\n" + tc.target
		f := pythonProject(t, map[string]string{"agent.py": pyAgent, "pyproject.toml": pyproject})

		summary, err := f.deploy(t)
		if tc.version == "" {
			requireErrorContains(t, err, "allows no Python")
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		warned := false
		for _, warning := range summary.Warnings {
			warned = warned || strings.HasPrefix(warning, "requires-python ")
		}
		if warned != tc.warns {
			t.Fatalf("%s %s: warnings %q", tc.requirement, tc.target, summary.Warnings)
		}
		manifest := readJSON(t, f.read(t, "langgraph.json"))
		if manifest["python_version"] != tc.version || manifest["_INTERNAL_docker_tag"] != LanggraphAPIVersion+"-py"+tc.version {
			t.Fatalf("%s %s: %v", tc.requirement, tc.target, manifest)
		}
		requireContains(t, f.read(t, "pyproject.toml"), "requires-python = \""+tc.requirement+"\"")
	}
}
