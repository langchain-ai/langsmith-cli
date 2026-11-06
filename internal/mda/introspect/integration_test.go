//go:build integration

package introspect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// pythonSDKVenvEnv names the virtualenv with the real `managed_deepagents`
// installed, e.g. open-swe's `mda/.venv`, which installs the vendored SDK
// editable.
const pythonSDKVenvEnv = "MDA_INTROSPECT_PYTHON_VENV"

// realPythonProject writes a Python project run by the virtualenv named by
// [pythonSDKVenvEnv].
func realPythonProject(t *testing.T, files map[string]string) *project.Project {
	t.Helper()
	venv := os.Getenv(pythonSDKVenvEnv)
	if venv == "" {
		t.Fatalf("set %s to a virtualenv with managed-deepagents installed", pythonSDKVenvEnv)
	}
	root := t.TempDir()
	writeFiles(t, root, files)
	t.Setenv("VIRTUAL_ENV", venv)
	return loadProject(t, root)
}

func TestRealPythonSDKProject(t *testing.T) {
	p := realPythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import define_agent\n" +
			"def build(runtime):\n    raise RuntimeError('the factory must not run')\n" +
			"agent = define_agent('Research Assistant', build, async_subagents=['researcher'])\n",
		"identity.py": "from managed_deepagents import define_identity\nidentity = define_identity(auth='backend')\n",
		"memory.py": "from managed_deepagents import MemoryLayer, define_memory\n" +
			"memory = define_memory(agent=MemoryLayer(), user=MemoryLayer(allow=lambda context: True))\n",
		"sandbox/__init__.py": "from managed_deepagents import define_sandbox\n" +
			"sandbox = define_sandbox(docker_image='python:3.12', fs_capacity_bytes=2 * 1024**3, idle_ttl_seconds=600)\n",
		"sandbox/setup.sh": "echo ready\n",
		"schedules/daily.py": "from managed_deepagents import define_schedule\n" +
			"schedule = define_schedule(cron='0 9 * * *', prompt='Summarize')\n",
		"tools/mcp.py": "from managed_deepagents import connections, define_mcp\n" +
			"mcp = define_mcp(servers={\n" +
			"    'docs': {'transport': 'http', 'url': 'https://docs.langchain.com/mcp'},\n" +
			"    'notion': {'transport': 'http', 'url': 'https://mcp.notion.com/mcp',\n" +
			"               'connection': connections.get('notion', {'type': 'user'})},\n" +
			"})\n",
		"channels/support.py": "from managed_deepagents import channels\n" +
			"channel = channels.slack(name='Support Bot', description='Answers support questions')\n",
	})
	ctx := t.Context()

	config, err := ReadAgentStaticConfig(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	want := AgentStaticConfig{
		AssistantID: "research-assistant",
		IsFactory:   true,
		AsyncSubagents: []AsyncSubagentGraph{{
			Name: "researcher", GraphID: "research-assistant--researcher", Export: "agent__researcher",
		}},
	}
	if !reflect.DeepEqual(config, want) {
		t.Fatalf("agent: got %+v", config)
	}

	requires, err := IdentityRequiresIngressSecret(ctx, p)
	if err != nil || !requires {
		t.Fatalf("identity: got %v, %v", requires, err)
	}

	memory, err := ResolveMemory(ctx, p)
	if err != nil || memory.Declared == nil || memory.Declared.Scope != (MemoryLayers{Agent: true, User: true}) {
		t.Fatalf("memory: got %+v, %v", memory, err)
	}

	bases, err := ExtractSandboxBases(ctx, p)
	if err != nil || *bases.DockerImage != "python:3.12" || *bases.FsCapacityBytes != 2<<30 {
		t.Fatalf("sandbox: got %+v, %v", bases, err)
	}
	if err := RejectRemovedSandboxScope(ctx, p); err != nil {
		t.Fatal(err)
	}

	schedules, err := ReadScheduleConfigs(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"cron": "0 9 * * *", "prompt": "Summarize"}; len(schedules) != 1 ||
		!reflect.DeepEqual(schedules[0].Config, want) {
		t.Fatalf("schedules: got %+v", schedules)
	}

	connectors, err := ScanExportedConnectors(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	connections, err := EmitMcpConnectionEntriesForEntries(ctx, p, outDir, connectors)
	if err != nil {
		t.Fatal(err)
	}
	if want := []McpConnectionEntry{{
		ConnectorName: "mcp", ServerName: "notion", URL: "https://mcp.notion.com/mcp", Slug: "notion", OwnerPolicy: "user",
	}}; !reflect.DeepEqual(connections, want) {
		t.Fatalf("mcp connections: got %+v", connections)
	}
	requirements, err := EmitConnectorRequirementsForEntries(ctx, p, outDir, connectors)
	if err != nil {
		t.Fatal(err)
	}
	manifest, ok := requirements["mcp"].(map[string]any)
	if !ok || !reflect.DeepEqual(manifest["userConnections"], []any{"notion"}) {
		t.Fatalf("connector requirements: got %#v", requirements)
	}

	scan, err := ScanExportedChannels(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if want := (ChannelScan{
		Entries:          []string{filepath.Join("channels", "support.py")},
		RequiresIdentity: true,
	}); !reflect.DeepEqual(scan, want) {
		t.Fatalf("channels: got %+v", scan)
	}
	channelRequirements, err := EmitChannelRequirementsForEntries(ctx, p, outDir, scan.Entries)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := channelRequirements["support"]; !ok {
		t.Fatalf("channel requirements: got %#v", channelRequirements)
	}
}

func TestRealPythonSDKStaticAndBareAgents(t *testing.T) {
	static := realPythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import define_deep_agent\n" +
			"agent = define_deep_agent(name='lead', model='openai:gpt-5', subagents=[\n" +
			"    {'name': 'writer', 'description': 'd', 'system_prompt': 'p', 'async': True},\n" +
			"    {'name': 'editor', 'description': 'd', 'system_prompt': 'p'},\n" +
			"])\n",
	})
	config, err := ReadAgentStaticConfig(t.Context(), static)
	if err != nil {
		t.Fatal(err)
	}
	if config.AssistantID != "lead" || config.IsFactory || *config.Model != "openai:gpt-5" ||
		len(config.AsyncSubagents) != 1 || config.AsyncSubagents[0].GraphID != "lead--writer" {
		t.Fatalf("got %+v", config)
	}

	bare := realPythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import define_deep_agent\n" +
			"def agent(runtime):\n    return define_deep_agent(name='plain')\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), bare)
	requireErrorPrefix(t, err, "agent.py exports `agent` as a bare factory function")

	named := realPythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import define_agent\n" +
			"agent = define_agent('lead', model='openai:gpt-5')\n",
	})
	config, err = ReadAgentStaticConfig(t.Context(), named)
	if err != nil {
		t.Fatal(err)
	}
	if config.AssistantID != "lead" || config.IsFactory || *config.Model != "openai:gpt-5" {
		t.Fatalf("got %+v", config)
	}

	nameless := realPythonProject(t, map[string]string{
		"agent.py": "from managed_deepagents import define_agent\n" +
			"agent = define_agent(model='openai:gpt-5')\n",
	})
	_, err = ReadAgentStaticConfig(t.Context(), nameless)
	requireErrorPrefix(t, err, "agent `name` is required in agent.py. "+
		"Name the exported agent first, `define_agent(\"research-assistant\", ...)`;")
}
