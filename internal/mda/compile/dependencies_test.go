package compile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Managed dependencies, lockfiles, and Python packaging of the build.

func TestCompileVendorsTheTypescriptRuntimeIntoTheBuild(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})

	mustCompile(t, f.deploy)

	if f.read(t, "__runtime__/index.js") != "// mock runtime\n" ||
		f.read(t, "__runtime__/runtime/index.js") != "// mock runtime entry\n" {
		t.Fatal("the runtime was not vendored")
	}
	manifest, _, err := parseJSONObject([]byte(f.read(t, "__runtime__/package.json")))
	if err != nil {
		t.Fatal(err)
	}
	if version, _ := manifest.str("version"); version != "0.0.0-stub" {
		t.Fatalf("__runtime__/package.json version %q", version)
	}
	if langsmith, _ := manifest.values["dependencies"].(*jsonObject).str("langsmith"); langsmith != "^0.8.9" {
		t.Fatalf("__runtime__/package.json langsmith %q", langsmith)
	}
}

func TestCompileBuildsAgainstTheInstalledRuntimesDependencies(t *testing.T) {
	ts := typescriptProject(t, map[string]string{"agent.ts": tsAgent})
	// npm 0.10.0's ranges, which the managed stack's constants predate.
	ts.installation.manifest.Dependencies = []project.Dependency{
		{Name: "langsmith", Version: "^0.10.2"},
		{Name: "uuid", Version: "^14.0.2"},
		{Name: "@standard-schema/spec", Version: "^1.1.0"},
	}
	py := pythonProject(t, map[string]string{"agent.py": pyAgent})
	py.installation.manifest.Dependencies = []project.Dependency{{Name: "langchain", Version: "<3,>=2.0"}}

	mustCompile(t, ts.deploy)
	mustCompile(t, py.deploy)

	root, _, err := parseJSONObject([]byte(ts.read(t, "package.json")))
	if err != nil {
		t.Fatal(err)
	}
	dependencies, _ := root.object("dependencies")
	overrides, _ := root.object("overrides")
	for name, want := range map[string]string{"langsmith": "^0.10.2", "uuid": "^14.0.2", "deepagents": "^1.15.1"} {
		if got, _ := dependencies.str(name); got != want {
			t.Fatalf("package.json dependencies.%s = %q, want %q", name, got, want)
		}
	}
	if got, _ := overrides.str("langsmith"); got != "^0.10.2" {
		t.Fatalf("package.json overrides.langsmith = %q", got)
	}
	runtime := ts.read(t, "__runtime__/package.json")
	requireContains(t, runtime, "\"@standard-schema/spec\": \"^1.1.0\"")
	requireContains(t, py.read(t, "pyproject.toml"), "\"langchain[mcp]<3,>=2.0\"")
	requireContains(t, py.read(t, "__runtime__/pyproject.toml"), "dependencies = [\n\"langchain<3,>=2.0\",\n]")
}

func TestCompileVendorsThePythonRuntimeIntoTheBuild(t *testing.T) {
	f := pythonProject(t, map[string]string{"agent.py": pyAgent})

	mustCompile(t, f.deploy)

	requireContains(t, f.read(t, "__runtime__/pyproject.toml"), "version = \"0.0.0-stub\"", "\"pydantic>=2.7.4,<2.15\",\n")
	for _, rel := range []string{"__init__.py", "runtime.py", "_connectors.py"} {
		if !f.exists("__runtime__/managed_deepagents/" + rel) {
			t.Fatalf("%s was not vendored", rel)
		}
	}
}

func TestCompileMakesPythonBuildsDependencyOnlyInstallable(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":            pyAgent,
		"tools/query_db.py":   "def query_db():\n    pass\n",
		"middleware/audit.py": "def audit():\n    pass\n",
		"pyproject.toml": "[project]\nname = \"example\"\nversion = \"0.0.0\"\n" +
			"dependencies = [\"deepagents>=0.7.3\", \"managed-deepagents>=0.1.0\"]\n",
	})

	mustCompile(t, f.deploy)

	pyproject := f.read(t, "pyproject.toml")
	requireContains(t, pyproject, "build-backend = \"setuptools.build_meta\"", "py-modules = []", "deepagents>=0.7.3")
	// `langgraph.json` installs the vendored runtime instead.
	requireNotContains(t, pyproject, "managed-deepagents")
}

func TestCompileSynthesizesAPyprojectWhenTheAuthorOmitsIt(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_deep_agent(name=\"agent\", model=\"langsmith:moonshotai/kimi-k3\")\n",
	})

	mustCompile(t, f.deploy)

	want := "[project]\nname = \"mda-agent\"\nversion = \"0.0.0\"\nrequires-python = \">=3.11\"\n" +
		"dependencies = [\n    \"deepagents>=0.7.30,<1\",\n    \"langchain[mcp]>=1.4.6,<2\",\n" +
		"    \"langgraph>=1.2.15,<2\",\n    \"langgraph-prebuilt>=1.1.1,<2\",\n    \"langgraph-checkpoint>=4.2.0\",\n" +
		"    \"langgraph-sdk>=0.4.6,<0.5\",\n    \"pyjwt[crypto]>=2.8.0\",\n    \"fastmcp>=4.0.10,<5\",\n" +
		"    \"mcp>=2.2.0,<3\",\n    \"langsmith>=0.10.17\",\n" +
		// The LangSmith provider speaks the Gateway's OpenAI-compatible API.
		"    \"langchain-openai>=1.4.2\"\n]\n" +
		"\n[build-system]\nrequires = [\"setuptools>=61\"]\nbuild-backend = \"setuptools.build_meta\"\n" +
		"\n[tool]\nuv = {}\n" +
		"\n[tool.setuptools]\npy-modules = []\n"
	if got := f.read(t, "pyproject.toml"); got != want {
		t.Fatalf("pyproject.toml:\n%s", got)
	}
}

func TestCompileInjectsTheManagedStackAndProviderForTypescript(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "export const agent = defineDeepAgent({ name: \"agent\", model: \"langsmith:moonshotai/kimi-k3\" });\n",
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n" +
			"  \"dependencies\": { \"managed-deepagents\": \"^0.0.2\", \"deepagents\": \"1.12.1\" }\n}\n",
	})

	mustCompile(t, f.deploy)

	manifest := readJSON(t, f.read(t, "package.json"))
	deps := manifest["dependencies"].(map[string]any)
	// The runtime is an npm workspace package, not a `file:` dependency.
	if _, ok := deps["managed-deepagents"]; ok {
		t.Fatalf("dependencies: %v", deps)
	}
	if !reflect.DeepEqual(manifest["workspaces"], []any{"__runtime__"}) || manifest["private"] != true {
		t.Fatalf("package.json: %v", manifest)
	}
	for name, version := range map[string]string{
		// Author-declared packages are preserved.
		"deepagents": "1.12.1",
		// The managed runtime stack is injected as compatible ranges.
		"@langchain/core":                 "^1.2.17",
		"@langchain/langgraph":            "^1.4.18",
		"@langchain/langgraph-checkpoint": "^1.1.3",
		"@langchain/langgraph-sdk":        "^1.12.0",
		"langchain":                       "^1.5.12",
		"uuid":                            "^13.0.2",
		// The `langsmith:` provider uses the OpenAI integration package to
		// speak the LangSmith Gateway's OpenAI-compatible API.
		"@langchain/openai": "^1.5.5",
	} {
		if deps[name] != version {
			t.Fatalf("%s: got %v, want %s", name, deps[name], version)
		}
	}
}

func TestCompileReplacesTheAuthorLockfileWithAGeneratedOne(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n" +
			"  \"dependencies\": { \"managed-deepagents\": \"^0.0.2\" }\n}\n",
		// A stale lockfile copied from the author's tree must not survive.
		"package-lock.json": "{\n  \"name\": \"example\",\n  \"lockfileVersion\": 3,\n  \"requires\": true,\n" +
			"  \"packages\": {\n    \"\": {\n      \"name\": \"example\",\n      \"dependencies\": {\n" +
			"        \"managed-deepagents\": \"^0.0.2\"\n      }\n    }\n  }\n}\n",
		"pnpm-lock.yaml": "author\n",
	})

	summary := mustCompile(t, f.deploy)

	if got := f.read(t, "package-lock.json"); got != stubLockfile {
		t.Fatalf("package-lock.json:\n%s", got)
	}
	if got := f.read(t, lockInputMarker); got != f.read(t, "package.json") {
		t.Fatalf("lock input marker: %q", got)
	}
	// Other package managers' lockfiles are never carried.
	if f.exists("pnpm-lock.yaml") {
		t.Fatal("pnpm-lock.yaml was copied")
	}
	for _, rel := range summary.Copied {
		if rel == "package-lock.json" {
			t.Fatalf("copied: %v", summary.Copied)
		}
	}
}

func TestCompileRecordsTheAuthorsPythonLockfile(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":       pyAgent,
		"pyproject.toml": "[project]\nname = \"example\"\nversion = \"0.0.0\"\ndependencies = [\"managed-deepagents>=0.1.2\"]\n",
		"uv.lock":        "version = 1\n# author lock\n",
	})

	mustCompile(t, f.deploy)

	// A build seeds its lock from the author's lock and remembers what it
	// copied.
	if f.read(t, "uv.lock") != "version = 1\n# author lock\n" || f.read(t, lockInputMarker) != "version = 1\n# author lock\n" {
		t.Fatal("the author's lock was not recorded")
	}
}

func TestCompileResolvesAuthoredLangchainAndMcpToolDependencies(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "import { authoredTool } from \"./tools/authored-tool\";\n" +
			"export const agent = defineDeepAgent({\n  name: \"agent\",\n  model: \"openai:gpt-5.5\",\n  tools: [authoredTool],\n});\n",
		"tools/authored-tool.ts": "export const authoredTool = { name: \"search\" };\n",
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n" +
			"  \"dependencies\": { \"langchain\": \"1.5.10\" }\n}\n",
	})

	mustCompile(t, f.deploy)

	deps := readJSON(t, f.read(t, "package.json"))["dependencies"].(map[string]any)
	if deps["@langchain/core"] != "^1.2.17" || deps["langchain"] != "1.5.10" ||
		deps["@langchain/mcp-adapters"] != managedMcpAdapterVersion(t) || deps["deepagents"] != "^1.15.1" {
		t.Fatalf("dependencies: %v", deps)
	}
	if !f.exists("package-lock.json") {
		t.Fatal("no lockfile")
	}
}

func TestCompileForDeployStripsDevDependenciesBeforeTheLockfile(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "export const agent = defineDeepAgent({ name: \"agent\", model: \"openai:gpt-5.5\" });\n",
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n" +
			"  \"dependencies\": { \"managed-deepagents\": \"^0.0.2\" },\n" +
			"  \"devDependencies\": { \"vite\": \"^8.0.14\", \"wrangler\": \"^4.112.0\" }\n}\n",
	})

	mustCompile(t, f.deploy)

	// `serde_json`'s `remove` swaps the last key into the removed one's place:
	// `zod` takes `managed-deepagents`', `private` takes `devDependencies`'.
	want := `{
  "name": "example",
  "type": "module",
  "dependencies": {
    "@langchain/core": "^1.2.17",
    "@langchain/langgraph": "^1.4.18",
    "@langchain/langgraph-checkpoint": "^1.1.3",
    "@langchain/langgraph-sdk": "^1.12.0",
    "@langchain/mcp-adapters": "^2.0.0",
    "@langchain/openai": "^1.5.5",
    "deepagents": "^1.15.1",
    "hono": "^4.12.27",
    "jose": "^6.2.3",
    "langchain": "^1.5.12",
    "langsmith": "^0.8.9",
    "zod": "^4.4.3",
    "uuid": "^13.0.2"
  },
  "private": true,
  "overrides": {
    "langsmith": "^0.8.9"
  },
  "workspaces": [
    "__runtime__"
  ]
}
`
	if got := f.read(t, "package.json"); got != want {
		t.Fatalf("package.json:\n%s", got)
	}
	// The lockfile is generated from the stripped manifest.
	if got := f.read(t, lockInputMarker); got != want {
		t.Fatalf("lock input marker:\n%s", got)
	}
}

func TestCompileInjectsTheManagedStackAndProviderForPython(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_deep_agent(name=\"agent\", model=\"anthropic:claude-sonnet-4-5\")\n",
		"pyproject.toml": "[project]\nname = \"example\"\nversion = \"0.0.0\"\ndependencies = [\n" +
			"    \"deepagents>=0.7.3\",\n    \"managed-deepagents>=0.1.2\",\n]\n",
	})

	mustCompile(t, f.deploy)

	pyproject := f.read(t, "pyproject.toml")
	// Author-declared packages are preserved (not duplicated).
	if strings.Count(pyproject, "deepagents>=0.7.3") != 1 {
		t.Fatalf("pyproject.toml:\n%s", pyproject)
	}
	requireContains(t, pyproject, "langgraph>=1.2.15,<2", "langgraph-prebuilt>=1.1.1,<2", "langgraph-checkpoint>=4.2.0",
		"langsmith>=0.10.17", "langchain[mcp]>=1.4.6,<2", "langchain-anthropic>=1.5.4")
}

func TestCompileDedupsAPythonDependencyDeclaredWithUnderscores(t *testing.T) {
	// The author declares a managed package using the PEP 503-equivalent
	// underscore spelling; no hyphenated duplicate is injected.
	f := pythonProject(t, map[string]string{
		"agent.py":       pyAgent,
		"pyproject.toml": "[project]\nname = \"example\"\nversion = \"0.0.0\"\ndependencies = [\"langgraph_checkpoint>=4.1.1\"]\n",
	})

	mustCompile(t, f.deploy)

	pyproject := f.read(t, "pyproject.toml")
	if strings.Count(pyproject, "langgraph_checkpoint") != 1 || strings.Contains(pyproject, "langgraph-checkpoint") {
		t.Fatalf("pyproject.toml:\n%s", pyproject)
	}
}

func TestCompileRespectsAnAuthorPinnedManagedDependency(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "export const agent = defineDeepAgent({ name: \"agent\", model: \"openai:gpt-5.5\" });\n",
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n" +
			"  \"dependencies\": { \"langchain\": \"1.5.10\" }\n}\n",
	})

	mustCompile(t, f.deploy)

	// The author's version wins; the managed pin does not clobber it.
	if got := readJSON(t, f.read(t, "package.json"))["dependencies"].(map[string]any)["langchain"]; got != "1.5.10" {
		t.Fatalf("langchain: %v", got)
	}
}

func TestCompileSkipsProviderInjectionForModelsThatAreNotStrings(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "const m = { provider: \"openai\" };\n" +
			"export const agent = defineDeepAgent({ name: \"agent\", model: m });\n",
		"package.json": "{\n  \"name\": \"example\",\n  \"type\": \"module\",\n  \"dependencies\": {} \n}\n",
	})

	mustCompile(t, f.deploy)

	deps := readJSON(t, f.read(t, "package.json"))["dependencies"].(map[string]any)
	// The managed runtime stack is still injected, but no provider package
	// is guessed when the model is not a string.
	if _, ok := deps["@langchain/core"]; !ok {
		t.Fatalf("dependencies: %v", deps)
	}
	for name := range deps {
		if strings.HasPrefix(name, "@langchain/openai") {
			t.Fatalf("dependencies: %v", deps)
		}
	}
}

func TestCompileRespectsAnExistingPythonBuildSystem(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyAgent,
		"pyproject.toml": "[build-system]\nrequires = [\"hatchling\"]\nbuild-backend = \"hatchling.build\"\n\n" +
			"[project]\nname = \"example\"\nversion = \"0.0.0\"\n",
	})

	mustCompile(t, f.deploy)

	pyproject := f.read(t, "pyproject.toml")
	requireContains(t, pyproject, "hatchling.build")
	requireNotContains(t, pyproject, "setuptools.build_meta", "py-modules")
}

func TestCompileWiresTypescriptMemory(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"memory.ts": "import { defineMemory } from \"managed-deepagents\";\n" +
			"export const memory = defineMemory({ scope: [{ type: \"agent\" }] });\n",
	})

	mustCompile(t, f.deploy)

	// Agent memory does not require identity.
	requireContains(t, f.read(t, "_mda_entry.ts"), "import { memory } from \"./memory\";",
		"compileManagedAgent(definition, config, { contextRepo, hasSkills, memory, asyncSubagentGraphs }, runtime)")
	if !f.exists("memory.ts") {
		t.Fatal("memory.ts was not copied")
	}
}

func TestCompilePreservesNamedMemoryPolicies(t *testing.T) {
	for _, tc := range []struct {
		name    string
		project func(*testing.T, map[string]string) fixture
		files   map[string]string
		memory  string
		entry   []string
	}{
		{"typescript", typescriptProject, map[string]string{"agent.ts": tsAgent, "identity.ts": tsIdentity}, "memory.ts", []string{
			"import { memory } from \"./memory\";",
			"identity, memory, asyncSubagentGraphs }, runtime)",
		}},
		{"python", pythonProject, map[string]string{"agent.py": pyAgent, "identity.py": pyIdentity}, "memory.py", []string{
			"from memory import memory as _memory",
			"identity=_identity, memory=_memory, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := map[string]string{
				"memory.ts": "import { defineMemory as dm, memoryLayer } from 'managed-deepagents';\n" +
					"const allow = async () => { throw new Error('must not run at build time'); };\n" +
					"export const memory = dm({ agent: memoryLayer({ allow }), user: memoryLayer({ allow }) });\n",
				"memory.py": "from managed_deepagents import define_memory as dm, MemoryLayer\n" +
					"async def allow(context):\n    raise RuntimeError('must not run at build time')\n" +
					"memory = dm(agent=MemoryLayer(allow=allow), user=MemoryLayer(allow=allow))\n",
			}[tc.memory]
			tc.files[tc.memory] = source
			f := tc.project(t, tc.files)

			mustCompile(t, f.deploy)

			if got := f.read(t, tc.memory); got != source {
				t.Fatalf("%s: %q", tc.memory, got)
			}
			entry := f.read(t, "_mda_entry."+f.project.Language.Extension())
			requireContains(t, entry, tc.entry...)
			requireNotContains(t, entry, "must not run at build time")
		})
	}
}

func TestCompileWiresPythonMemory(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":  pyAgent,
		"memory.py": "from managed_deepagents import define_memory\nmemory = define_memory(scope=[{\"type\": \"agent\"}])\n",
	})

	mustCompile(t, f.deploy)

	requireContains(t, f.read(t, "_mda_entry.py"), "from memory import memory as _memory",
		"await compile_managed_agent_async(_definition, config, context_repo=_context_repo, "+
			"has_skills=_has_skills, memory=_memory, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)")
}

func TestCompileWiresAPythonBrowserAndAddsStagehand(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py":   pyAgent,
		"browser.py": "from managed_deepagents import define_browser\nbrowser = define_browser(scope=\"user\")\n",
		"pyproject.toml": "[project]\nname = \"example\"\nversion = \"0.0.0\"\n" +
			"dependencies = [\"deepagents>=0.7.3\"]\n",
	})

	mustCompile(t, f.deploy)

	requireContains(t, f.read(t, "_mda_entry.py"), "from browser import browser as _browser",
		"await compile_managed_agent_async(_definition, config, context_repo=_context_repo, "+
			"has_skills=_has_skills, browser=_browser, async_subagent_graphs=_async_subagent_graphs, runtime=runtime)",
		// The handoff watcher's graph, beside the agent's.
		"from managed_deepagents.browser import handoff_watcher as _handoff_watcher",
		"\n\n@asynccontextmanager\nasync def browser_handoff_watcher(config, runtime: ServerRuntime):\n"+
			"    async with parent_run(config):\n        yield _handoff_watcher\n")
	graphs := readJSON(t, f.read(t, "langgraph.json"))["graphs"].(map[string]any)
	if graphs["mda-browser-handoff"] != "./_mda_entry.py:browser_handoff_watcher" {
		t.Fatalf("graphs: %v", graphs)
	}
	requireContains(t, f.read(t, "pyproject.toml"), "stagehand==4.3.0", "stagehand-middleware==0.1.0")
}

func TestCompileWiresATypescriptBrowserAndAddsStagehand(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":   tsAgent,
		"browser.ts": "export const browser = defineBrowser({ scope: \"user\" });\n",
	})

	mustCompile(t, f.deploy)

	requireContains(t, f.read(t, "_mda_entry.ts"), "import { browser } from \"./browser\";",
		"compileManagedAgent(definition, config, { contextRepo, hasSkills, browser, asyncSubagentGraphs }, runtime)",
		// Async subagents run with the lead's browser.
		"compileManagedAgent(definition, config, { contextRepo, hasSkills, browser, asyncSubagent: \"general-purpose\" }, runtime)",
		// The handoff watcher's graph, beside the agent's.
		"export { browserHandoffWatcher as browser_handoff_watcher } from \"managed-deepagents/runtime\";")
	graphs := readJSON(t, f.read(t, "langgraph.json"))["graphs"].(map[string]any)
	if graphs["mda-browser-handoff"] != "./_mda_entry.ts:browser_handoff_watcher" {
		t.Fatalf("graphs: %v", graphs)
	}
	deps := readJSON(t, f.read(t, "package.json"))["dependencies"].(map[string]any)
	if deps["@browserbasehq/stagehand"] != "4.3.0" || deps["stagehand-middleware"] != "0.1.1" {
		t.Fatalf("dependencies: %v", deps)
	}
}

func TestCompileRejectsTheBrowserHandoffGraphAsAnAgentName(t *testing.T) {
	f := typescriptProject(t, map[string]string{
		"agent.ts":   tsImport + "export const agent = defineDeepAgent({ name: \"mda-browser-handoff\" });\n",
		"browser.ts": "export const browser = defineBrowser({ scope: \"user\" });\n",
	})

	_, err := f.deploy(t)
	if err == nil || err.Error() != "agent name `mda-browser-handoff` is reserved for browser use" {
		t.Fatalf("got %v", err)
	}
}

func TestCompileIgnoresAPythonBrowserInTypescript(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "browser.py": "browser = None\n"})

	mustCompile(t, f.deploy)

	requireNotContains(t, f.read(t, "_mda_entry.ts"), "browser")
}

func TestCompileOmitsMemoryWhenTheProjectDeclaresNone(t *testing.T) {
	f := typescriptProject(t, map[string]string{"agent.ts": tsAgent})

	mustCompile(t, f.deploy)

	requireNotContains(t, f.read(t, "_mda_entry.ts"), "memory")
}

func TestCompileKeepsMemoryOffTheGeneratedAuthModule(t *testing.T) {
	for _, tc := range []struct {
		name    string
		project func(*testing.T, map[string]string) fixture
		files   map[string]string
		auth    string
		entry   string
	}{
		{"typescript", typescriptProject, map[string]string{
			"agent.ts":    tsAgent,
			"identity.ts": tsIdentity,
			"memory.ts":   "import { defineMemory } from \"managed-deepagents\";\nexport const memory = defineMemory({ scope: [{ type: \"agent\" }] });\n",
		}, "export const auth = buildManagedAuth(identity);", "import { memory } from \"./memory\";"},
		// Passing memory crashed Host boot with `build_managed_auth() takes 1
		// positional argument but 2 were given`.
		{"python", pythonProject, map[string]string{
			"agent.py":    pyAgent,
			"identity.py": pyIdentity,
			"memory.py":   "from managed_deepagents import define_memory\nmemory = define_memory(scope=[{\"type\": \"agent\"}])\n",
		}, "auth = build_managed_auth(_identity)", "from memory import memory as _memory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.project(t, tc.files)

			mustCompile(t, f.deploy)

			extension := f.project.Language.Extension()
			auth := f.read(t, "_mda_auth."+extension)
			requireContains(t, auth, tc.auth)
			requireNotContains(t, auth, "memory")
			requireContains(t, f.read(t, "_mda_entry."+extension), tc.entry)
		})
	}
}
