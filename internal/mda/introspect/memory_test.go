package introspect

import (
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

const tsMemoryImport = "import { defineMemory, memoryLayer } from \"managed-deepagents\";\n"
const pyMemoryImport = "from managed_deepagents import MemoryLayer, define_memory\n"

func tsLayers(t *testing.T, source string) (MemoryLayersValue, bool) {
	t.Helper()
	p := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "memory.ts": tsMemoryImport + source})
	value, found, err := extractMemoryLayers(t.Context(), p, "memory.ts")
	if err != nil {
		t.Fatal(err)
	}
	return value, found
}

func pyLayers(t *testing.T, source string) (MemoryLayersValue, bool) {
	t.Helper()
	p := pythonProject(t, map[string]string{"agent.py": pyAgent, "memory.py": pyMemoryImport + source})
	value, found, err := extractMemoryLayers(t.Context(), p, "memory.py")
	if err != nil {
		t.Fatal(err)
	}
	return value, found
}

func requireLayers(t *testing.T, source string, got MemoryLayersValue, found bool, want MemoryLayersValue) {
	t.Helper()
	if !found || got != want {
		t.Fatalf("%s: got %+v (found %v), want %+v", source, got, found, want)
	}
}

func TestReadsNamedLayersWithoutEvaluatingPolicies(t *testing.T) {
	for _, c := range []struct {
		ts, py string
		want   MemoryLayersKind
	}{
		{"agent: memoryLayer()", "agent=MemoryLayer()", MemoryLayersAgent},
		{"user: memoryLayer({ allow: () => { throw Error(); } })", "user=MemoryLayer(allow=lambda ctx: 1 / 0)", MemoryLayersUser},
		{"agent: memoryLayer(), user: memoryLayer()", "agent=MemoryLayer(), user=MemoryLayer()", MemoryLayersAgentAndUser},
	} {
		ts := "export const memory = defineMemory({ " + c.ts + " });\n"
		got, found := tsLayers(t, ts)
		requireLayers(t, ts, got, found, MemoryLayersValue{Kind: c.want})
		py := "memory = define_memory(" + c.py + ")\n"
		got, found = pyLayers(t, py)
		requireLayers(t, py, got, found, MemoryLayersValue{Kind: c.want})
	}
}

func TestReadsScopeLayers(t *testing.T) {
	for source, want := range map[string]MemoryLayersValue{
		`export const memory = defineMemory({ scope: { type: "user" } });`:                       {Kind: MemoryLayersUser},
		`export const memory = defineMemory({ scope: [{ type: "user" }, { type: "agent" }] });`:  {Kind: MemoryLayersAgentAndUser},
		`export const memory = defineMemory({ scope: [{ type: "user", id: "usr_1" }] });`:        {Kind: MemoryLayersUser},
		`export const memory = defineMemory({ scope: [{ allow: () => true, type: "agent" }] });`: {Kind: MemoryLayersAgent},
		`export const memory = defineMemory();`:                                                  {Kind: MemoryLayersNone},
		`export const memory = defineMemory({ scope: [] });`:                                     {Kind: MemoryLayersNone},
		`export const memory = defineMemory({ scope: [{ type: "user" }, { type: "user" }] });`:   {Kind: MemoryLayersDuplicate, Detail: "user"},
		`export const memory = defineMemory({ scope: [{ type: "workspace" }] });`:                {Kind: MemoryLayersUnknown, Detail: `type "workspace"`},
		`export const memory = defineMemory({ scope: [{}] });`:                                   {Kind: MemoryLayersUnknown, Detail: "a layer with no `type`"},
		`export const memory = defineMemory({ scope: [{ type: "agent", access: "rw" }] });`:      {Kind: MemoryLayersReserved, Detail: "the key `access`"},
		`export const memory = defineMemory({ scope: [{ type: "agent", id: "usr_1" }] });`:       {Kind: MemoryLayersUnknown, Detail: "an `id` on the agent layer"},
		`export const memory = defineMemory({ scope: ["agent"] });`:                              {Kind: MemoryLayersShorthand, Detail: `"agent"`},
	} {
		got, found := tsLayers(t, source+"\n")
		requireLayers(t, source, got, found, want)
	}
	for source, want := range map[string]MemoryLayersValue{
		"memory = define_memory(scope=[{'type': 'agent'}])":                               {Kind: MemoryLayersAgent},
		"memory = define_memory(scope={'type': 'agent'})":                                 {Kind: MemoryLayersAgent},
		"memory = define_memory()":                                                        {Kind: MemoryLayersNone},
		"memory = define_memory(scope=[{'type': 'user'}, {'type': 'user'}])":              {Kind: MemoryLayersDuplicate, Detail: "user"},
		"memory = define_memory(scope=[{'type': 'workspace'}])":                           {Kind: MemoryLayersUnknown, Detail: "type 'workspace'"},
		"memory = define_memory(scope=[{'allow': None, 'type': 'user', 'access': 'rw'}])": {Kind: MemoryLayersReserved, Detail: "the key `access`"},
	} {
		got, found := pyLayers(t, source+"\n")
		requireLayers(t, source, got, found, want)
	}
}

func TestIgnoresModulesWithoutAMemoryDeclaration(t *testing.T) {
	_, found := tsLayers(t, "// export const memory = defineMemory({ agent: memoryLayer() });\nconst other = defineMemory({ agent: memoryLayer() });\n")
	if found {
		t.Fatal("expected no declaration")
	}
	_, found = pyLayers(t, "other = define_memory(agent=MemoryLayer())\n")
	if found {
		t.Fatal("expected no declaration")
	}
}

func TestNonMountingValuesFailWithActionableErrors(t *testing.T) {
	_, err := MemoryLayersValue{Kind: MemoryLayersShorthand, Detail: "`scope`"}.MountedLayers("memory.ts")
	requireErrorContains(t, err, "names whose memory")
	requireErrorContains(t, err, "named `agent` and `user`")

	_, err = MemoryLayersValue{Kind: MemoryLayersReserved, Detail: "the key `access`"}.MountedLayers("memory.ts")
	requireErrorContains(t, err, "not supported yet")
	requireErrorContains(t, err, "memory.ts")

	_, err = MemoryLayersValue{Kind: MemoryLayersUnknown, Detail: `type "workspace"`}.MountedLayers("memory.py")
	requireErrorContains(t, err, "unknown memory layer")

	_, err = MemoryLayersValue{Kind: MemoryLayersDynamic, Detail: "a computed value"}.MountedLayers("memory.ts")
	requireErrorContains(t, err, "not a static literal")
	requireErrorContains(t, err, "Deploy seeds durable memory")

	_, err = MemoryLayersValue{Kind: MemoryLayersDuplicate, Detail: "agent"}.MountedLayers("memory.ts")
	requireErrorContains(t, err, "memory.ts declares the agent memory layer more than once")
}

func TestTheDeclarationSnippetFollowsTheProjectLanguage(t *testing.T) {
	if got := DeclareMemorySnippet(project.Python); got != "define_memory(agent=MemoryLayer())" {
		t.Fatal(got)
	}
	if got := DeclareMemorySnippet(project.TypeScript); got != "defineMemory({ agent: memoryLayer() })" {
		t.Fatal(got)
	}
}

func TestResolveMemory(t *testing.T) {
	none := typescriptProject(t, map[string]string{"agent.ts": tsAgent})
	decision, err := ResolveMemory(t.Context(), none)
	if err != nil || decision.Declared != nil || decision.AgentEnabled() || decision.UserEnabled() {
		t.Fatalf("got %+v, %v", decision, err)
	}

	agent := typescriptProject(t, map[string]string{
		"agent.ts":  tsAgent,
		"memory.ts": tsMemoryImport + "export const memory = defineMemory({ agent: memoryLayer() });\n",
	})
	decision, err = ResolveMemory(t.Context(), agent)
	if err != nil || !decision.AgentEnabled() || decision.UserEnabled() || decision.Declared.Path != "memory.ts" {
		t.Fatalf("got %+v, %v", decision, err)
	}

	user := pythonProject(t, map[string]string{
		"agent.py":    pyAgent,
		"memory.py":   pyMemoryImport + "memory = define_memory(user=MemoryLayer())\n",
		"identity.py": "from managed_deepagents import define_identity\nidentity = define_identity(auth='backend')\n",
	})
	decision, err = ResolveMemory(t.Context(), user)
	if err != nil || decision.AgentEnabled() || !decision.UserEnabled() {
		t.Fatalf("got %+v, %v", decision, err)
	}
}

func TestResolveMemoryRejectsUserMemoryWithoutIdentity(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py":  pyAgent,
		"memory.py": pyMemoryImport + "memory = define_memory(user=MemoryLayer())\n",
	})
	_, err := ResolveMemory(t.Context(), p)
	requireErrorPrefix(t, err, "memory.py enables user memory, but user memory requires identity.py")
}

func TestResolveMemoryRejectsAModuleWithoutADeclaration(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":  tsAgent,
		"memory.ts": "export const notMemory = 1;\n",
	})
	_, err := ResolveMemory(t.Context(), p)
	requireErrorPrefix(t, err, "memory.ts does not export a static `memory` declaration. "+
		"Export `memory` from a `defineMemory({ agent: memoryLayer() })` call")
}

func TestResolveMemoryRejectsTheRetiredKnobs(t *testing.T) {
	disabled := typescriptProject(t, map[string]string{
		"agent.ts": "export const agent = { kind: \"deep-agent\", config: { name: \"a\", disableMemory: false } };\n",
	})
	_, err := ResolveMemory(t.Context(), disabled)
	requireErrorPrefix(t, err, "agent.ts sets `disableMemory`, which is no longer supported.")

	identity := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent,
		"identity.ts": "export const identity = { kind: \"identity\", config: " +
			"{ ingress: { http: \"backend\" }, scope: { memory: \"agent\" } } };\n",
	})
	_, err = ResolveMemory(t.Context(), identity)
	requireErrorPrefix(t, err, "identity.ts sets `scope.memory`, which is no longer supported.")

	sdk := pythonProject(t, map[string]string{
		"agent.py": pyImport + "agent = define_deep_agent(name='a', disable_memory=True)\n",
	})
	_, err = ResolveMemory(t.Context(), sdk)
	requireErrorPrefix(t, err, "failed to import agent.py: TypeError: disable_memory is no longer supported")
}
