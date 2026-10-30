package introspect

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

const tsImport = "import { defineAgent, defineDeepAgent } from \"managed-deepagents\";\n"
const pyImport = "from managed_deepagents import define_agent, define_deep_agent\n"

func TestAgentNamePayloadsDecodeIntoAgentNameValues(t *testing.T) {
	for payload, want := range map[string]AgentNameValue{
		`{"kind":"STATIC","value":"x"}`: {Kind: AgentNameStatic, Value: "x"},
		`{"kind":"DYNAMIC"}`:            {Kind: AgentNameDynamic},
		`{"kind":"ABSENT"}`:             {Kind: AgentNameAbsent},
	} {
		var got AgentNameValue
		if err := json.Unmarshal([]byte(payload), &got); err != nil {
			t.Fatalf("%s: %v", payload, err)
		}
		if got != want {
			t.Fatalf("%s: got %+v, want %+v", payload, got, want)
		}
	}
	for _, payload := range []string{`{"kind":"STATIC"}`, `{"kind":"OTHER"}`} {
		var got AgentNameValue
		if err := json.Unmarshal([]byte(payload), &got); err == nil {
			t.Fatalf("%s: expected an error, got %+v", payload, got)
		}
	}
}

func TestTypescriptStaticEntryKeepsAuthoredNameAndModel(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "console.log(\"printed while importing\");\n" +
			"export const agent = defineDeepAgent({ name: \"support\", model: \"openai:gpt-5\" });\n",
	})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := AgentStaticConfig{AssistantID: "support", Model: config.Model}
	if !reflect.DeepEqual(config, want) || config.Model == nil || *config.Model != "openai:gpt-5" {
		t.Fatalf("got %+v", config)
	}
}

func TestTypescriptNamedFactoryIsReadWithoutRunningIt(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsImport +
			"export const agent = defineAgent(\"Lead Agent\", () => {\n" +
			"  throw new Error(\"factory must not run during build\");\n" +
			"}, { asyncSubagents: [\"Deep Researcher\"] });\n",
	})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := AgentStaticConfig{
		AssistantID: "lead-agent",
		IsFactory:   true,
		AsyncSubagents: []AsyncSubagentGraph{{
			Name:    "Deep Researcher",
			GraphID: "lead-agent--deep-researcher",
			Export:  "agent__deep_researcher",
		}},
	}
	if !reflect.DeepEqual(config, want) {
		t.Fatalf("got %+v, want %+v", config, want)
	}
}

func TestPythonNamedFactoryIsReadWithoutRunningIt(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py": pyImport +
			"def build(runtime):\n    raise RuntimeError('factory must not run during build')\n" +
			"agent = define_agent('support', build, async_subagents=['researcher'])\n",
	})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := AgentStaticConfig{
		AssistantID: "support",
		IsFactory:   true,
		AsyncSubagents: []AsyncSubagentGraph{{
			Name: "researcher", GraphID: "support--researcher", Export: "agent__researcher",
		}},
	}
	if !reflect.DeepEqual(config, want) {
		t.Fatalf("got %+v, want %+v", config, want)
	}
}

func TestBareFactoryFunctionsAskForTheNamedForm(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "export const agent = () => defineDeepAgent({ name: \"plain\" });\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), ts)
	requireErrorPrefix(t, err, "agent.ts exports `agent` as a bare factory function")
	requireErrorContains(t, err,
		"`export const agent = defineAgent(\"research-assistant\", factory, { asyncSubagents: [...] })`")

	py := pythonProject(t, map[string]string{
		"agent.py": pyImport + "def agent(runtime):\n    return define_deep_agent(name='plain')\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), py)
	requireErrorContains(t, err,
		"`agent = define_agent(\"research-assistant\", factory, async_subagents=[...])`")
}

func TestStaticAgentsRejectManagedSelections(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "export const agent = defineDeepAgent({ name: \"research\", memory: null });\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), p)
	requireErrorPrefix(t, err, "agent.ts: Remove `memory` from defineDeepAgent(...). "+
		"Use a factory entrypoint to select memory per run; static agents use memory.ts.")
}

func TestPythonAgentWithoutNameIsRejected(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_agent(model='openai:gpt-5')\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), p)
	requireErrorPrefix(t, err, "agent `name` is required in agent.py. "+
		"Name the exported agent first, `define_agent(\"research-assistant\", ...)`;")
}

func TestAgentModulesThatDoNotParseAreReported(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts": "export const agent = { name: \"research\" };\nconst x = ;\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), ts)
	requireErrorPrefix(t, err, "failed to parse agent.ts:2:11:")

	py := pythonProject(t, map[string]string{
		"agent.py": "agent = define_deep_agent(name=\"research\"\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), py)
	requireErrorPrefix(t, err, "failed to parse agent.py:1:")
}

func TestAgentModulesThatThrowAreReported(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts": "throw new Error(\"boom\");\nexport const agent = 1;\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), ts)
	requireErrorPrefix(t, err, "failed to import agent.ts: Error: boom")

	py := pythonProject(t, map[string]string{
		"agent.py": "raise ValueError('boom')\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), py)
	requireErrorPrefix(t, err, "failed to import agent.py: ValueError: boom")
}

func TestTypescriptAsyncSubagentsAreReadFromTheDefinition(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsImport +
			"const writerName = 'writer';\n" +
			"const writer = { name: writerName, description: 'd', async: true };\n" +
			"const team = [writer, { name: 'editor', description: 'd', async: true }];\n" +
			"export const agent = defineDeepAgent({\n" +
			"  name: 'lead',\n" +
			"  subagents: [\n" +
			"    { name: 'researcher', description: 'd', async: true },\n" +
			"    { name: 'helper', description: 'd' },\n" +
			"    { name: 'sync', description: 'd', async: false },\n" +
			"    ...team,\n" +
			"  ],\n" +
			"});\n",
	})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, graph := range config.AsyncSubagents {
		names = append(names, graph.Name)
	}
	if want := []string{"researcher", "writer", "editor"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestPythonAsyncSubagentsAreReadFromTheDefinition(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py": pyImport +
			"writer = {'name': 'writer', 'description': 'd', 'async': True}\n" +
			"agent = define_deep_agent(name='lead', subagents=[\n" +
			"    {'name': 'researcher', 'description': 'd', 'async': True},\n" +
			"    writer,\n" +
			"    {'name': 'helper', 'description': 'd'},\n" +
			"    {'name': 'off', 'description': 'd', 'async': False},\n" +
			"])\n",
	})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, graph := range config.AsyncSubagents {
		names = append(names, graph.Name)
	}
	if want := []string{"researcher", "writer"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
}

func TestTextModuleImportsLoadWhileIntrospecting(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"name.md": "from-text\n",
		"agent.ts": tsImport + "import name from \"./name.md\" with { type: \"text\" };\n" +
			"export const agent = defineDeepAgent({ name: name.trim() });\n",
	})

	id, err := AgentAssistantID(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if id != "from-text" {
		t.Fatalf("got %q", id)
	}
}

func TestTextModuleImportsLoadOnANodeWithoutRegisterHooks(t *testing.T) {
	t.Setenv("NODE_OPTIONS", "--import="+preloadModule(t, `import module, { syncBuiltinESMExports } from "node:module";
delete module.registerHooks;
syncBuiltinESMExports();`))
	p := typescriptProject(t, map[string]string{
		"name.md": "from-text\n",
		"agent.ts": tsImport + "import name from \"./name.md\" with { type: \"text\" };\n" +
			"export const agent = defineDeepAgent({ name: name.trim() });\n",
	})

	id, err := AgentAssistantID(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if id != "from-text" {
		t.Fatalf("got %q", id)
	}
}

func TestTextModuleImportsLoadInCommonJSProjects(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"package.json": `{"name": "fixture"}` + "\n",
		"name.md":      "from-text\n",
		"suffix.md":    "-cts\n",
		"suffix.cts":   "import suffix from \"./suffix.md\" with { type: \"text\" };\nexport const text: string = suffix.trim();\n",
		"agent.ts": tsImport + "import name from \"./name.md\" with { type: \"text\" };\n" +
			"const { text } = require(\"./suffix.cts\");\n" +
			"export const agent = defineDeepAgent({ name: name.trim() + text });\n",
	})

	id, err := AgentAssistantID(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if id != "from-text-cts" {
		t.Fatalf("got %q", id)
	}
}

func TestBareFactoriesNameAnSDKWithoutNamedFactories(t *testing.T) {
	ts := typescriptProject(t, map[string]string{"agent.ts": "export const agent = () => ({});\n"})
	// A release from before named factories: no AGENT_FACTORY export.
	writeFiles(t, filepath.Join(ts.Root, "node_modules", "managed-deepagents"), map[string]string{
		"package.json": `{"name": "managed-deepagents", "version": "0.10.0", "type": "module", "exports": {".": "./index.js"}}`,
		"index.js":     "export function defineDeepAgent(config) { return config; }\n",
	})
	_, err := ReadAgentStaticConfig(t.Context(), ts)
	if err == nil || err.Error() != "agent.ts exports `agent` as a bare factory function, so its name is known "+
		"only by running it, which `langsmith deploy` does not do. It reads a factory's name from "+
		"defineAgent(name, factory), but the installed managed-deepagents (0.10.0) does not have "+
		"defineAgent. Deploy factory agents with a managed-deepagents release that has it, or with "+
		"the SDK's own `mda deploy`." {
		t.Fatalf("got %v", err)
	}

	py := pythonProject(t, map[string]string{
		"agent.py": "def agent(runtime):\n    return None\n",
		// The project root comes first on sys.path, so this shadows the stub.
		"managed_deepagents/__init__.py": "class DeepAgentDefinition:\n    pass\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), py)
	requireErrorContains(t, err, "define_agent(name, factory), but the installed managed-deepagents (0.10.0) "+
		"does not have define_agent.")
}

func TestReadAgentModel(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "const tool = { model: \"openai:gpt-4\" };\n" +
			"export const agent = defineDeepAgent({\n" +
			"  name: \"agent\", model: \"openai:gpt-5\",\n" +
			"  subagents: [{ name: \"sub\", description: \"d\", model: \"anthropic:claude\" }],\n" +
			"});\n",
	})
	model, ok, err := ReadAgentModel(t.Context(), p)
	if err != nil || !ok || model != "openai:gpt-5" {
		t.Fatalf("got %q %v %v", model, ok, err)
	}

	factory := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_agent('a', lambda runtime: None)\n",
	})
	model, ok, err = ReadAgentModel(t.Context(), factory)
	if err != nil || ok || model != "" {
		t.Fatalf("got %q %v %v", model, ok, err)
	}

	dynamic := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_deep_agent(name='a', model=object())\n",
	})
	_, ok, err = ReadAgentModel(t.Context(), dynamic)
	if err != nil || ok {
		t.Fatalf("got %v %v", ok, err)
	}
}

func TestAsyncSubagentsResolveToGraphsUnderTheAssistant(t *testing.T) {
	graphs, err := resolveAsyncSubagents(
		[]AgentNameValue{{Kind: AgentNameStatic, Value: "Deep Researcher"}}, "lead", "agent.py", project.Python)
	if err != nil {
		t.Fatal(err)
	}
	want := []AsyncSubagentGraph{{
		Name: "Deep Researcher", GraphID: "lead--deep-researcher", Export: "agent__deep_researcher",
	}}
	if !reflect.DeepEqual(graphs, want) {
		t.Fatalf("got %+v", graphs)
	}
}

func TestAsyncSubagentsRejectDynamicAndDuplicateNames(t *testing.T) {
	_, err := resolveAsyncSubagents([]AgentNameValue{{Kind: AgentNameDynamic}}, "lead", "agent.ts", project.TypeScript)
	requireErrorContains(t, err, "`async: true`")
	requireErrorContains(t, err, "`...spread`")

	_, err = resolveAsyncSubagents([]AgentNameValue{{Kind: AgentNameDynamic}}, "lead", "agent.py", project.Python)
	requireErrorContains(t, err, "as literals")

	_, err = resolveAsyncSubagents([]AgentNameValue{
		{Kind: AgentNameStatic, Value: "researcher"},
		{Kind: AgentNameStatic, Value: "Researcher"},
	}, "lead", "agent.py", project.Python)
	requireErrorContains(t, err, "names are compared after normalization")
}

func staticConfig(asyncSubagents []AsyncSubagentGraph) AgentStaticConfig {
	return AgentStaticConfig{AssistantID: "lead", AsyncSubagents: asyncSubagents}
}

func TestServedAgentsAlsoGetAnAsyncGeneralPurposeSubagent(t *testing.T) {
	graphs := staticConfig(nil).ServedAsyncSubagents()
	want := []AsyncSubagentGraph{{
		Name: "general-purpose", GraphID: "lead--general-purpose", Export: "agent__general_purpose",
	}}
	if !reflect.DeepEqual(graphs, want) {
		t.Fatalf("got %+v", graphs)
	}
}

func TestADeclaredGeneralPurposeSubagentIsNotAddedTwice(t *testing.T) {
	for _, name := range []string{"general-purpose", "General Purpose"} {
		declared, err := resolveAsyncSubagents(
			[]AgentNameValue{{Kind: AgentNameStatic, Value: name}}, "lead", "agent.py", project.Python)
		if err != nil {
			t.Fatal(err)
		}
		if got := staticConfig(declared).ServedAsyncSubagents(); !reflect.DeepEqual(got, declared) {
			t.Fatalf("%s: got %+v, want %+v", name, got, declared)
		}
	}
}

func TestValidateAssistantID(t *testing.T) {
	for _, id := range []string{"agent", "research-assistant", "Research_Bot1"} {
		if err := ValidateAssistantID(id, "agent.ts"); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	for _, id := range []string{"", "1agent", "research assistant", "foo/bar"} {
		if err := ValidateAssistantID(id, "agent.ts"); err == nil {
			t.Fatalf("%q: expected an error", id)
		}
	}
	if !IsValidAssistantID("my-agent") || IsValidAssistantID("1bad") || IsValidAssistantID("") {
		t.Fatal("IsValidAssistantID disagrees with ValidateAssistantID")
	}
}
