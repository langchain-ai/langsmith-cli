package compile

import (
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// pythonVersion is any resolved runtime; these tests cover manifest shape,
// not resolution.
const pythonVersion = "3.11"

var servedContexts = []runtimeContext{
	{kind: runtimeContextLocal, contextRoot: "/tmp/context"},
	{kind: runtimeContextContextHub, repo: "context-repo"},
}

func TestStaticEntriesCheckDynamicOnlyFieldsBeforeCompilation(t *testing.T) {
	for _, context := range servedContexts {
		inputs := entryInputs{agentImport: "./agent", context: context}
		for language, check := range map[project.Language]string{
			project.TypeScript: "assertStaticAgentDefinition(definition);",
			project.Python:     "assert_static_agent_definition(_definition)",
		} {
			entry := generateEntry(language, inputs)
			checkAt := strings.Index(entry, check)
			// Served Python entries yield the graph instead of returning it.
			compileAt := max(strings.LastIndex(entry, "return "), strings.LastIndex(entry, "yield "))
			if checkAt < 0 || compileAt < checkAt {
				t.Fatalf("%v:\n%s", context, entry)
			}
		}
	}
}

func TestFactoryEntriesOmitTheStaticDefinitionCheck(t *testing.T) {
	for _, context := range servedContexts {
		inputs := entryInputs{agentImport: "./agent", factory: true, context: context}
		for _, language := range []project.Language{project.TypeScript, project.Python} {
			entry := generateEntry(language, inputs)
			requireNotContains(t, entry, "assertStaticAgentDefinition", "assert_static_agent_definition")
		}
	}
}

func TestPythonServedEntriesHoldParentRunForTheRun(t *testing.T) {
	entry := generateEntry(project.Python, entryInputs{agentImport: "agent", context: servedContexts[0]})

	requireContains(t, entry,
		"@asynccontextmanager\nasync def agent(config, runtime: ServerRuntime):",
		"    async with parent_run(config):\n        yield await compile_managed_agent_async(")
}

func TestEncodeStringLiteralEscapesForBothLanguages(t *testing.T) {
	got := encodeStringLiteral("a \"quote\" \\ back\nline\r\ttab\x01 é")
	if want := `"a \"quote\" \\ back\nline\r\ttab\u0001 é"`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestGeneratedAuthLoadsConnectorsBeforeConstructingAuth(t *testing.T) {
	for _, tc := range []struct {
		language                         project.Language
		identity, connectors, wantImport string
		assignment                       string
	}{
		{project.TypeScript, "./identity", "./_mda_connectors", "import \"./_mda_connectors\";",
			"export const auth = buildManagedAuth(identity);"},
		{project.Python, "identity", "_mda_connectors", "import _mda_connectors",
			"auth = build_managed_auth(_identity)"},
	} {
		module := generateAuthModule(tc.language, tc.identity, tc.connectors, "")
		imports, _, ok := strings.Cut(module, tc.assignment)
		if !ok || !strings.Contains(imports, tc.wantImport) || strings.Contains(module, "_mda_channels") {
			t.Fatalf("%s", module)
		}
	}
}

func manifestInputs(language project.Language) langgraphManifestInputs {
	graph := "./_mda_entry.ts:agent"
	if language == project.Python {
		graph = "./_mda_entry.py:agent"
	}
	return langgraphManifestInputs{
		graph:         graph,
		language:      language,
		apiVersion:    LanggraphAPIVersion,
		pythonVersion: pythonVersion,
		assistantID:   "agent",
	}
}

func TestGenerateLanggraphJSONIsValidAndComplete(t *testing.T) {
	got := generateLanggraphJSON(manifestInputs(project.TypeScript))

	// Without identity there is no auth block.
	want := `{
  "node_version": "22",
  "_INTERNAL_docker_tag": "0.18.0.dev12-node22",
  "dependencies": [
    "."
  ],
  "graphs": {
    "agent": "./_mda_entry.ts:agent"
  },
  "env": ".env"
}
`
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestGenerateLanggraphJSONUsesThePythonVersionForPython(t *testing.T) {
	got := generateLanggraphJSON(manifestInputs(project.Python))

	want := `{
  "python_version": "3.11",
  "_INTERNAL_docker_tag": "0.18.0.dev12-py3.11",
  "dependencies": [
    "."
  ],
  "graphs": {
    "agent": "./_mda_entry.py:agent"
  },
  "env": ".env"
}
`
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestGenerateLanggraphJSONInstallsTheVendoredRuntimeAndLocalPackagesForPythonDeploys(t *testing.T) {
	inputs := manifestInputs(project.Python)
	inputs.includeVendoredRuntime = true
	inputs.localPackages = []string{"__local__/my-dep"}

	dependencies := readJSON(t, generateLanggraphJSON(inputs))["dependencies"]

	if got := prettyJSON(dependencies.([]any)); got != "[\n  \".\",\n  \"./__runtime__\",\n  \"./__local__/my-dep\"\n]" {
		t.Fatalf("dependencies: %s", got)
	}
}

func TestGenerateLanggraphJSONUsesTheOverrideAPIVersionStem(t *testing.T) {
	inputs := manifestInputs(project.TypeScript)
	inputs.apiVersion = "0.13.0rc1"

	if got := readJSON(t, generateLanggraphJSON(inputs))["_INTERNAL_docker_tag"]; got != "0.13.0rc1-node22" {
		t.Fatalf("got %v", got)
	}
}

func TestGenerateLanggraphJSONUsesTheAssistantIDAsGraphKey(t *testing.T) {
	inputs := manifestInputs(project.TypeScript)
	inputs.assistantID = "research-assistant"
	inputs.extraGraphs = [][2]string{{"research-assistant--general-purpose", "./_mda_entry.ts:agent__general_purpose"}}

	graphs := readJSON(t, generateLanggraphJSON(inputs))["graphs"].(map[string]any)

	if len(graphs) != 2 || graphs["research-assistant"] != "./_mda_entry.ts:agent" ||
		graphs["research-assistant--general-purpose"] != "./_mda_entry.ts:agent__general_purpose" {
		t.Fatalf("graphs: %v", graphs)
	}
}

func TestGenerateLanggraphJSONDisablesStudioAuthForLocalBuilds(t *testing.T) {
	// The dev server grants Studio identity on `x-auth-scheme: langsmith`
	// with no credential check, so local builds turn the scheme off.
	inputs := manifestInputs(project.Python)
	inputs.authPath = "./_mda_auth.py:auth"
	inputs.disableStudioAuth = true

	if got := readJSON(t, generateLanggraphJSON(inputs))["auth"].(map[string]any)["disable_studio_auth"]; got != true {
		t.Fatalf("got %v", got)
	}
}

func TestGenerateLanggraphJSONAddsAnAuthBlockWhenIdentityIsPresent(t *testing.T) {
	inputs := manifestInputs(project.Python)
	inputs.authPath = "./_mda_auth.py:auth"

	got := generateLanggraphJSON(inputs)

	// The ingress secret is never allowlisted into config.configurable, and
	// without a managed HTTP app auth runs first.
	want := `{
  "python_version": "3.11",
  "_INTERNAL_docker_tag": "0.18.0.dev12-py3.11",
  "dependencies": [
    "."
  ],
  "graphs": {
    "agent": "./_mda_entry.py:agent"
  },
  "env": ".env",
  "auth": {
    "path": "./_mda_auth.py:auth",
    "disable_studio_auth": false
  },
  "http": {
    "configurable_headers": {
      "includes": [
        "x-mda-user-id"
      ]
    },
    "middleware_order": "auth_first"
  }
}
`
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestGenerateLanggraphJSONMountsTheManagedHTTPAppWithoutIdentity(t *testing.T) {
	inputs := manifestInputs(project.TypeScript)
	inputs.managedHTTPPath = "./_mda_http.ts:app"

	manifest := readJSON(t, generateLanggraphJSON(inputs))

	if _, ok := manifest["auth"]; ok {
		t.Fatalf("manifest: %v", manifest)
	}
	if http := manifest["http"].(map[string]any); len(http) != 1 || http["app"] != "./_mda_http.ts:app" {
		t.Fatalf("http: %v", http)
	}
}
