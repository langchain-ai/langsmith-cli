package introspect

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// withHelperCache points the TypeScript helper at a fresh cache for the rest
// of the test.
func withHelperCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	previous := helperCacheRoot
	helperCacheRoot = func() (string, error) { return cache, nil }
	t.Cleanup(func() { helperCacheRoot = previous })
	return cache
}

func TestAHelperWhosePackagesDoNotLoadIsReinstalled(t *testing.T) {
	withHelperCache(t)
	var stages []ImportStage
	opts := ImportOptions{Progress: func(stage ImportStage) { stages = append(stages, stage) }}

	if err := ImportProject(t.Context(), typescriptProject(t, map[string]string{"agent.ts": tsAgent}), opts); err != nil {
		t.Fatal(err)
	}
	if want := []ImportStage{ImportStageInstallingHelper, ImportStageImporting}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("first import: got stages %v, want %v", stages, want)
	}

	toolchain, err := openTypescriptToolchain(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tsx := filepath.Join(toolchain.deps, "node_modules", "tsx")
	if err := os.RemoveAll(tsx); err != nil {
		t.Fatal(err)
	}
	stages = nil

	if err := ImportProject(t.Context(), typescriptProject(t, map[string]string{"agent.ts": tsAgent}), opts); err != nil {
		t.Fatal(err)
	}
	want := []ImportStage{ImportStageImporting, ImportStageInstallingHelper, ImportStageImporting}
	if !reflect.DeepEqual(stages, want) {
		t.Fatalf("broken install: got stages %v, want %v", stages, want)
	}
	if !isFile(filepath.Join(tsx, "package.json")) {
		t.Fatal("tsx was not reinstalled")
	}
}

func TestTheHelperNamesTheNodeVersionItNeeds(t *testing.T) {
	toolchain, err := openTypescriptToolchain(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := toolchain.ensure(t.Context(), func() {}); err != nil {
		t.Fatal(err)
	}
	preload := preloadModule(t, `Object.defineProperty(process.versions, "node", { value: "18.19.0" });`)
	cmd := proc.Node("--import", preload, filepath.Join(toolchain.dir, "introspect.mjs"))
	var checked struct {
		Version int `json:"version"`
	}

	err = toolchain.explain(runHelper(t.Context(), cmd, &requestHeader{Mode: "check"}, "a check", &checked))

	requireErrorPrefix(t, err, "the TypeScript introspection helper needs Node.js 20.6.0 or newer, but ")
	if !strings.HasSuffix(err.Error(), " is 18.19.0. Install a newer Node.js and try again.") {
		t.Fatalf("got %v", err)
	}
}

// preloadModule writes source to a module and returns its file URL, for
// `node --import`.
func preloadModule(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"preload.mjs": source + "\n"})
	url := filepath.ToSlash(filepath.Join(dir, "preload.mjs"))
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	return "file://" + url
}

func TestAnInstallWhosePackagesDoNotLoadIsNotMarkedComplete(t *testing.T) {
	withHelperCache(t)
	t.Setenv("NODE_OPTIONS", "--import="+preloadModule(t, `import { registerHooks } from "node:module";
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "tsx/esm/api") throw new Error("tsx is broken");
    return nextResolve(specifier, context);
  },
});`))
	p := typescriptProject(t, map[string]string{"agent.ts": tsAgent})

	_, err := ReadAgentStaticConfig(t.Context(), p)

	toolchain, openErr := openTypescriptToolchain(t.Context())
	if openErr != nil {
		t.Fatal(openErr)
	}
	want := "the TypeScript introspection helper installed into " + toolchain.deps +
		" does not load: the TypeScript toolchain does not load: Error: tsx is broken"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %s", err, want)
	}
	if _, statErr := os.Stat(toolchain.deps); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the install was published: %v", statErr)
	}
}

func TestAProjectWithoutItsSDKIsAskedToInstallIt(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"package.json": `{"type": "module"}`, "agent.ts": tsAgent})
	p := loadProject(t, root)

	_, err := ReadAgentStaticConfig(t.Context(), p)

	requireErrorPrefix(t, err, "cannot import the modules of "+p.Root+" to read its configuration: "+
		"Error: cannot import managed-deepagents from "+p.Root+": ")
	if !strings.HasSuffix(err.Error(), ". Install the project's dependencies (`npm install`).") {
		t.Fatalf("got %v", err)
	}
}

func TestARefusedRenameIsRetried(t *testing.T) {
	refused := errors.New("refused")
	attempts := 0
	previousRename, previousRefused := rename, renameRefused
	rename = func(from, to string) error {
		attempts++
		if attempts < 3 {
			return refused
		}
		return os.Rename(from, to)
	}
	renameRefused = func(err error) bool { return errors.Is(err, refused) }
	t.Cleanup(func() { rename, renameRefused = previousRename, previousRefused })
	dir := t.TempDir()
	staging := filepath.Join(dir, "staging")
	if err := os.Mkdir(staging, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := renameIntoPlace(t.Context(), staging, filepath.Join(dir, "target")); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("renamed after %d attempts", attempts)
	}
}

func TestModulesWritingToStdoutCannotCorruptTheDocument(t *testing.T) {
	ts := typescriptProject(t, map[string]string{
		"agent.ts": tsImport + "import { writeSync } from \"node:fs\";\n" +
			"writeSync(1, '{\"version\": 99}\\n');\n" +
			"export const agent = defineDeepAgent({ name: \"loud\" });\n",
	})
	py := pythonProject(t, map[string]string{
		"agent.py": "import os\nos.write(1, b'{\"version\": 99}\\n')\n" + pyImport +
			"agent = define_deep_agent(name='loud')\n",
	})
	for label, p := range map[string]*project.Project{"typescript": ts, "python": py} {
		config, err := ReadAgentStaticConfig(t.Context(), p)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if config.AssistantID != "loud" {
			t.Fatalf("%s: got %q", label, config.AssistantID)
		}
	}
}

func TestModulesImportWithTheProjectEnvUnderTheProcessEnv(t *testing.T) {
	t.Setenv("MDA_TEST_NAME_SUFFIX", "process")
	dotenv := "MDA_TEST_NAME_PREFIX=dotenv\nMDA_TEST_NAME_SUFFIX=dotenv\n"
	ts := typescriptProject(t, map[string]string{
		".env": dotenv,
		"agent.ts": tsImport + "export const agent = defineDeepAgent({\n" +
			"  name: `${process.env.MDA_TEST_NAME_PREFIX}-${process.env.MDA_TEST_NAME_SUFFIX}`,\n});\n",
	})
	py := pythonProject(t, map[string]string{
		".env": dotenv,
		"agent.py": "import os\n" + pyImport + "agent = define_deep_agent(\n" +
			"    name=os.environ['MDA_TEST_NAME_PREFIX'] + '-' + os.environ['MDA_TEST_NAME_SUFFIX'],\n)\n",
	})

	// Deploy hands over the `.env` it read; anything else reads it itself.
	if err := ImportProject(t.Context(), ts, ImportOptions{Env: map[string]string{
		"MDA_TEST_NAME_PREFIX": "dotenv", "MDA_TEST_NAME_SUFFIX": "dotenv",
	}}); err != nil {
		t.Fatal(err)
	}
	for label, p := range map[string]*project.Project{"typescript": ts, "python": py} {
		config, err := ReadAgentStaticConfig(t.Context(), p)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if config.AssistantID != "dotenv-process" {
			t.Fatalf("%s: got %q", label, config.AssistantID)
		}
	}
}

func TestThePythonHelperIgnoresPythonPath(t *testing.T) {
	shadow := t.TempDir()
	writeFiles(t, shadow, map[string]string{
		"managed_deepagents/__init__.py": "raise ImportError('PYTHONPATH reached the helper')\n",
	})
	t.Setenv("PYTHONPATH", shadow)
	p := pythonProject(t, map[string]string{"agent.py": pyAgent})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if config.AssistantID != "agent" {
		t.Fatalf("got %q", config.AssistantID)
	}
}

func TestThePythonHelperReadsItsRequestAsUTF8(t *testing.T) {
	// A locale whose encoding is not UTF-8, which Python reads stdin with.
	t.Setenv("LC_ALL", "en_US.ISO8859-1")
	p := pythonProjectIn(t, filepath.Join(t.TempDir(), "agenté"), map[string]string{"agent.py": pyAgent})

	config, err := ReadAgentStaticConfig(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if config.AssistantID != "agent" {
		t.Fatalf("got %q", config.AssistantID)
	}
}

func TestConnectorNamesThatAreNotStringsAreLeftOut(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import DeepAgentDefinition\nagent = DeepAgentDefinition({'name': 'agent'})\n",
		// An SDK whose connectors are named with something JSON cannot carry,
		// shadowing the stub from the project root.
		"managed_deepagents/__init__.py": "class DeepAgentDefinition:\n" +
			"    def __init__(self, config):\n        self.config = config\n" +
			"class Mcp:\n    kind = 'mcp_servers'\n" +
			"    def __init__(self, servers):\n        self.config = {'servers': servers}\n        self.name = object()\n",
		"managed_deepagents/_connector.py": "def collect_connectors(modules):\n" +
			"    return [module.connector for _, module in modules]\n" +
			"def requirements_from_connectors(connectors):\n    return {}\n",
		// A thread that would keep a helper that does not exit alive.
		"tools/mcp.py": "import threading, time\nfrom types import SimpleNamespace\n" +
			"from managed_deepagents import Mcp\n" +
			"threading.Thread(target=time.sleep, args=(3600,)).start()\n" +
			"notion = {'url': 'https://mcp.notion.com/mcp', " +
			"'connection': SimpleNamespace(slug='notion', owner={'type': 'user'})}\n" +
			"mcp = Mcp({'notion': notion, 7: notion})\n",
	})
	connectors, err := ScanExportedConnectors(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := EmitMcpConnectionEntriesForEntries(t.Context(), p, t.TempDir(), connectors)
	if err != nil {
		t.Fatal(err)
	}
	want := []McpConnectionEntry{{
		ConnectorName: "", ServerName: "notion", URL: "https://mcp.notion.com/mcp", Slug: "notion", OwnerPolicy: "user",
	}}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("got %+v", entries)
	}
}

func TestDocumentsDecodeStrictly(t *testing.T) {
	valid := map[string]any{
		"version": 1,
		"sdk":     map[string]any{"version": "0.10.0", "named_factories": true},
		"agent": map[string]any{"file": "agent.ts", "error": nil, "value": map[string]any{
			"kind": "static", "name": map[string]any{"kind": "STATIC", "value": "agent"}, "model": nil,
			"declares_disable_memory": false, "unsupported_keys": []any{}, "async_subagents": []any{},
		}},
		"identity": nil, "memory": nil, "sandbox": nil, "mcp": nil,
		"legacy_connectors": []any{}, "channels": []any{}, "schedules": []any{},
	}
	decode := func(mutate func(map[string]any)) error {
		raw, err := json.Marshal(valid)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		mutate(doc)
		if raw, err = json.Marshal(doc); err != nil {
			t.Fatal(err)
		}
		return decodeDocument(raw, &document{})
	}

	if err := decode(func(map[string]any) {}); err != nil {
		t.Fatal(err)
	}
	if err := decode(func(d map[string]any) { d["memories"] = nil }); err == nil ||
		err.Error() != `json: unknown field "memories"` {
		t.Fatalf("unknown field: got %v", err)
	}
	if err := decode(func(d map[string]any) { delete(d, "schedules") }); err == nil ||
		err.Error() != `document has no field "schedules"` {
		t.Fatalf("missing field: got %v", err)
	}
	if err := decode(func(d map[string]any) {
		delete(d["agent"].(map[string]any)["value"].(map[string]any), "async_subagents")
	}); err == nil || err.Error() != `document.agent.value has no field "async_subagents"` {
		t.Fatalf("missing nested field: got %v", err)
	}
}
