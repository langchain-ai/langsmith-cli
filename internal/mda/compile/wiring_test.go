package compile

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Context Hub and sandbox wiring of the generated entry.

func TestCompileWiresTheLocalContextHubForTypescript(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":        tsAgent,
		"instructions.md": "# Assistant\n\nUse `code` and ${vars}.\n",
	})

	mustCompile(t, f.local)

	// The agent file is still carried over verbatim — never rewritten.
	if got := f.read(t, "agent.ts"); got != tsAgent {
		t.Fatalf("agent.ts: %q", got)
	}
	hub := filepath.Join(f.root(), ".mda", "__contexthub__")
	if got := readFile(t, filepath.Join(hub, "instructions.md")); got != "# Assistant\n\nUse `code` and ${vars}.\n" {
		t.Fatalf("instructions.md: %q", got)
	}
	// No memory declaration → no hot file seeded locally, matching deploy.
	if isFile(filepath.Join(hub, "memories", "agent", "AGENTS.md")) {
		t.Fatal("the hot memory file was seeded")
	}
	entry := f.read(t, "_mda_entry.ts")
	requireContains(t, entry,
		"const contextRoot = "+encodeStringLiteral(hub)+";",
		"const hasSkills = false;",
		"compileManagedAgent(definition, config, { contextRoot, hasSkills, asyncSubagentGraphs }, runtime)")
	requireNotContains(t, entry, "systemPrompt")
}

func TestCompileForDeployWiresContextHubMetadataForTypescript(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":        tsAgent,
		"instructions.md": "Do not embed me.\n",
	})

	_, err := CompileForDeploy(t.Context(), f.project, f.outDir, DeployContext{Repo: "my-agent", HasSkills: true}, f.installation)
	if err != nil {
		t.Fatal(err)
	}

	entry := f.read(t, "_mda_entry.ts")
	requireContains(t, entry,
		"const contextRepo = \"my-agent\";",
		"const hasSkills = true;",
		"compileManagedAgent(definition, config, { contextRepo, hasSkills, asyncSubagentGraphs }, runtime)")
	// Commit pinning is not wired in the runtime yet — do not emit it.
	requireNotContains(t, entry, "contextCommit", "systemPrompt", "Do not embed me")
	if isDir(filepath.Join(f.root(), ".mda", "__contexthub__")) {
		t.Fatal("a deploy build created the local Context Hub")
	}
}

func TestCompileWiresTheLocalContextHubWhenInstructionsAreAbsent(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})

	summary := mustCompile(t, f.local)

	hub := filepath.Join(f.root(), ".mda", "__contexthub__")
	if isFile(filepath.Join(hub, "instructions.md")) || isFile(filepath.Join(hub, "memories", "agent", "AGENTS.md")) {
		t.Fatal("the local Context Hub was seeded")
	}
	entry := f.read(t, "_mda_entry.ts")
	requireNotContains(t, entry, "systemPrompt")
	requireContains(t, entry, "const contextRoot = ",
		"compileManagedAgent(definition, config, { contextRoot, hasSkills, asyncSubagentGraphs }, runtime)")
	if !reflect.DeepEqual(summary.Warnings, []string{
		"instructions.md is missing. Add it next to the agent file to set the system prompt.",
	}) {
		t.Fatalf("warnings: %q", summary.Warnings)
	}
}

func TestCompileWiresTheLocalContextHubForPython(t *testing.T) {
	f := pythonProject(t, map[string]string{"agent.py": pyAgent, "instructions.md": "Be helpful.\n"})

	mustCompile(t, f.local)

	hub := filepath.Join(f.root(), ".mda", "__contexthub__")
	if got := readFile(t, filepath.Join(hub, "instructions.md")); got != "Be helpful.\n" {
		t.Fatalf("instructions.md: %q", got)
	}
	entry := f.read(t, "_mda_entry.py")
	requireContains(t, entry, "_context_root = ", "_has_skills = False",
		"await compile_managed_agent_async(_definition, config, context_root=_context_root, has_skills=_has_skills, "+
			"async_subagent_graphs=_async_subagent_graphs, runtime=runtime)")
	requireNotContains(t, entry, "_system_prompt")
}

func TestCompileForDeployWiresContextHubMetadataForPython(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":        pyAgent,
		"instructions.md": "Do not embed me.\n",
		"pyproject.toml":  "[project]\nname = \"example\"\nversion = \"0.0.0\"\ndependencies = [\"managed-deepagents>=0.1.2\"]\n",
	})

	_, err := CompileForDeploy(t.Context(), f.project, f.outDir, DeployContext{Repo: "my-agent"}, f.installation)
	if err != nil {
		t.Fatal(err)
	}

	entry := f.read(t, "_mda_entry.py")
	requireContains(t, entry, "_context_repo = \"my-agent\"", "_has_skills = False",
		"await compile_managed_agent_async(_definition, config, context_repo=_context_repo, has_skills=_has_skills, "+
			"async_subagent_graphs=_async_subagent_graphs, runtime=runtime)")
	// Commit pinning is not wired in the runtime yet — do not emit it.
	requireNotContains(t, entry, "context_commit", "Do not embed me")

	// The runtime requirement is dropped: `langgraph.json` installs the
	// vendored runtime. Local-only dependencies are not injected.
	want := "[project]\nname = \"example\"\nversion = \"0.0.0\"\n" +
		"dependencies = [ \"deepagents>=0.7.30,<1\", \"langchain[mcp]>=1.4.6,<2\", \"langgraph>=1.2.15,<2\", " +
		"\"langgraph-prebuilt>=1.1.1,<2\", \"langgraph-checkpoint>=4.2.0\", \"langgraph-sdk>=0.4.6,<0.5\", " +
		"\"pyjwt[crypto]>=2.8.0\", \"fastmcp>=4.0.10,<5\", \"mcp>=2.2.0,<3\", \"langsmith>=0.10.17\", " +
		"\"langchain-openai>=1.4.2\"]\n" +
		"\n[build-system]\nrequires = [\"setuptools>=61\"]\nbuild-backend = \"setuptools.build_meta\"\n" +
		"\n[tool]\nuv = {}\n" +
		"\n[tool.setuptools]\npy-modules = []\n"
	if got := f.read(t, "pyproject.toml"); got != want {
		t.Fatalf("pyproject.toml:\n%s", got)
	}
	wantManifest := `{
  "python_version": "3.14",
  "_INTERNAL_docker_tag": "0.18.0.dev12-py3.14",
  "dependencies": [
    ".",
    "./__runtime__"
  ],
  "graphs": {
    "agent": "./_mda_entry.py:agent",
    "agent--general-purpose": "./_mda_entry.py:agent__general_purpose"
  },
  "env": ".env"
}
`
	if got := f.read(t, "langgraph.json"); got != wantManifest {
		t.Fatalf("langgraph.json:\n%s", got)
	}
}

func TestCompileFindsInstructionsNextToANestedAgentFile(t *testing.T) {
	root := canonicalTempDir(t)
	writeFiles(t, root, map[string]string{
		"src/agent.ts":        tsAgent,
		"src/instructions.md": "Nested prompt.\n",
		// A root-level instructions.md is ignored — only the one next to the
		// agent file counts.
		"instructions.md": "Root prompt.\n",
		"package.json":    `{"name": "fixture", "type": "module"}`,
	})
	f := fixture{installation: installTypescriptStub(t, root), outDir: filepath.Join(root, ".mda", "build")}
	f.project = &project.Project{
		Root:      root,
		Language:  project.TypeScript,
		AgentFile: filepath.Join("src", "agent.ts"),
		Files: []string{
			filepath.Join("src", "agent.ts"),
			filepath.Join("src", "instructions.md"),
			"instructions.md",
			"package.json",
		},
	}

	summary := mustCompile(t, f.local)

	hub := filepath.Join(f.root(), ".mda", "__contexthub__")
	if got := readFile(t, filepath.Join(hub, "instructions.md")); got != "Nested prompt.\n" {
		t.Fatalf("instructions.md: %q", got)
	}
	entry := f.read(t, "_mda_entry.ts")
	requireContains(t, entry, "const contextRoot = ", "import { agent as definition } from \"./src/agent\";")
	requireNotContains(t, entry, "Root prompt", "Nested prompt")
	if len(summary.Warnings) != 0 {
		t.Fatalf("warnings: %q", summary.Warnings)
	}
}

func TestCompileWiresATypescriptSandboxWithoutTheSetupScript(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"sandbox/index.ts": "import { defineSandbox } from \"managed-deepagents\";\n" +
			"export const sandbox = defineSandbox({ defaultTimeout: 600 });\n",
		"sandbox/setup.sh": "#!/usr/bin/env bash\necho hello > /tmp/marker\n",
	})

	summary := mustCompile(t, f.local)

	entry := f.read(t, "_mda_entry.ts")
	requireContains(t, entry, "import { sandbox } from \"./sandbox/index\";",
		"compileManagedAgent(definition, config, { contextRoot, hasSkills, sandbox, asyncSubagentGraphs }, runtime)",
		"import { compileManagedAgent } from \"managed-deepagents/runtime\";")
	requireNotContains(t, entry, "setupScript", "sandboxSetup")
	// The sandbox module and setup script are still carried for bake/recipe
	// digest.
	if !f.exists("sandbox/index.ts") || !f.exists("sandbox/setup.sh") {
		t.Fatal("the sandbox files were not copied")
	}
	if !summary.Sandbox {
		t.Fatal("summary does not report the sandbox")
	}
}

func TestCompileWiresAPythonSandboxWithoutTheSetupScript(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyAgent,
		"sandbox/__init__.py": "from managed_deepagents import define_sandbox\n" +
			"sandbox = define_sandbox(default_timeout=600)\n",
		"sandbox/setup.sh": "#!/usr/bin/env bash\necho hi\n",
	})

	mustCompile(t, f.local)

	entry := f.read(t, "_mda_entry.py")
	requireContains(t, entry, "from sandbox import sandbox as _sandbox",
		"await compile_managed_agent_async(_definition, config, context_root=_context_root, has_skills=_has_skills, "+
			"sandbox=_sandbox, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)",
		"from managed_deepagents.runtime import compile_managed_agent_async")
	requireNotContains(t, entry, "_setup_script", "setup_script=", "sandbox_setup")
}

func TestCompileOmitsSandboxWiringWhenAbsent(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})

	summary := mustCompile(t, f.local)

	entry := f.read(t, "_mda_entry.ts")
	requireNotContains(t, entry, "import { sandbox }", "setupScript")
	requireContains(t, entry, "compileManagedAgent(definition, config, { contextRoot, hasSkills, asyncSubagentGraphs }, runtime)")
	if summary.Sandbox {
		t.Fatal("summary reports a sandbox")
	}
}

func TestCompileIgnoresASetupScriptWithoutASandboxDeclaration(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":         tsAgent,
		"sandbox/setup.sh": "#!/usr/bin/env bash\necho orphaned\n",
	})

	mustCompile(t, f.local)

	entry := f.read(t, "_mda_entry.ts")
	requireNotContains(t, entry, "import { sandbox }", "setupScript", "sandboxSetup")
	requireContains(t, entry, "compileManagedAgent(definition, config, { contextRoot, hasSkills, asyncSubagentGraphs }, runtime)")
	if f.exists(SandboxRecipeFile) {
		t.Fatal("a recipe was written")
	}
}

func TestCompileBakesAFactorySetupScriptWithoutASandboxDeclaration(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":         tsImport + "export const agent = defineDeepAgent(\"agent\", () => defineDeepAgent({}));\n",
		"sandbox/setup.sh": "#!/usr/bin/env bash\necho ready\n",
	})

	summary := mustCompile(t, f.local)

	entry := f.read(t, "_mda_entry.ts")
	requireContains(t, entry, "const sandboxSetup = true;",
		"compileManagedAgent(definition, config, { contextRoot, hasSkills, sandboxSetup, asyncSubagentGraphs }, runtime)")
	manifest, err := LoadSandboxRecipeManifest(f.outDir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest == nil || !manifest.HasSetupScript || manifest.DockerImage != nil {
		t.Fatalf("recipe: %+v", manifest)
	}
	if !summary.Sandbox {
		t.Fatal("summary does not report the sandbox")
	}
}

func TestCompileMarksPythonFactorySandboxSetup(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyImport + "def build(runtime):\n    return define_deep_agent(name='agent')\n" +
			"agent = define_deep_agent('agent', build)\n",
		"sandbox/__init__.py": "from managed_deepagents import define_sandbox\n" +
			"sandbox = define_sandbox(docker_image=\"python:3.12-slim\")\n",
		"sandbox/setup.sh": "#!/usr/bin/env bash\necho ready\n",
	})

	mustCompile(t, f.deploy)

	requireContains(t, f.read(t, "_mda_entry.py"),
		"await compile_managed_agent_async(_definition, config, context_repo=_context_repo, has_skills=_has_skills, "+
			"sandbox=_sandbox, sandbox_setup=True, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)")
	recipe := f.read(t, SandboxRecipeFile)
	if !strings.Contains(recipe, "\"dockerImage\": \"python:3.12-slim\"") {
		t.Fatalf("recipe:\n%s", recipe)
	}
}
