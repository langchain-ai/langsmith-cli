package introspect

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func expectedMcpConnectionEntry() []McpConnectionEntry {
	return []McpConnectionEntry{{
		ConnectorName: "mcp",
		ServerName:    "notion",
		URL:           "https://mcp.notion.com/mcp",
		Slug:          "notion",
		OwnerPolicy:   "user",
	}}
}

func mcpProjects(t *testing.T) map[string]*project.Project {
	return map[string]*project.Project{
		"typescript": typescriptProject(t, map[string]string{
			"agent.ts": tsAgent,
			"tools/mcp.ts": tsMcp + "export const mcp = defineMcp({ servers: {\n" +
				"  notion: { url: \"https://mcp.notion.com/mcp\", connection: { slug: \"notion\", owner: { type: \"user\" } } },\n" +
				"  docs: { url: \"https://docs.example.com/mcp\" },\n" +
				"} });\n",
		}),
		"python": pythonProject(t, map[string]string{
			"agent.py": pyAgent,
			"tools/mcp.py": "from managed_deepagents import Connection, define_mcp\n" +
				"mcp = define_mcp(servers={\n" +
				"    'notion': {'url': 'https://mcp.notion.com/mcp', 'connection': Connection(slug='notion')},\n" +
				"    'docs': {'url': 'https://docs.example.com/mcp'},\n" +
				"})\n",
		}),
	}
}

func TestEmitMcpConnectionEntriesReadsNamedServers(t *testing.T) {
	for language, p := range mcpProjects(t) {
		entries, err := ScanExportedConnectors(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		outDir := t.TempDir()
		connections, err := EmitMcpConnectionEntriesForEntries(t.Context(), p, outDir, entries)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(connections, expectedMcpConnectionEntry()) {
			t.Fatalf("%s: got %+v", language, connections)
		}
		written, err := os.ReadFile(filepath.Join(outDir, McpConnectionsFile))
		if err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\n    \"connector_name\": \"mcp\",\n    \"server_name\": \"notion\",\n" +
			"    \"url\": \"https://mcp.notion.com/mcp\",\n    \"slug\": \"notion\",\n    \"owner_policy\": \"user\"\n  }\n]\n"
		if string(written) != want {
			t.Fatalf("%s: wrote %q", language, written)
		}
	}
}

func TestEmitConnectorRequirementsCallsTheProjectsSDK(t *testing.T) {
	for language, p := range mcpProjects(t) {
		outDir := t.TempDir()
		requirements, err := EmitConnectorRequirementsForEntries(t.Context(), p, outDir,
			[]string{filepath.Join("tools", "mcp."+p.Language.Extension())})
		if err != nil {
			t.Fatal(err)
		}
		manifest, ok := requirements["mcp"].(map[string]any)
		if !ok || manifest["provider"] != "mcp" ||
			!reflect.DeepEqual(manifest["userConnections"], []any{"notion"}) {
			t.Fatalf("%s: got %#v", language, requirements)
		}
		if _, err := os.Stat(filepath.Join(outDir, ConnectorRequirementsFile)); err != nil {
			t.Fatalf("%s: %v", language, err)
		}
	}
}

func TestEmitChannelRequirements(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":          tsAgent,
		"channels/slack.ts": "import { channels } from \"managed-deepagents\";\nexport const channel = channels.slack({});\n",
	})
	scan, err := ScanExportedChannels(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	requirements, err := EmitChannelRequirementsForEntries(t.Context(), p, outDir, scan.Entries)
	if err != nil {
		t.Fatal(err)
	}
	want := ChannelRequirements{"slack": map[string]any{
		"name": "slack", "provider": "slack", "eventsPath": "/channels/slack/events",
		"requiredPermissions": []any{}, "requiredEnv": []any{},
	}}
	if !reflect.DeepEqual(requirements, want) {
		t.Fatalf("got %#v", requirements)
	}
	written, err := os.ReadFile(filepath.Join(outDir, ChannelRequirementsFile))
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"slack\": {\n    \"name\": \"slack\",\n    \"provider\": \"slack\",\n" +
		"    \"eventsPath\": \"/channels/slack/events\",\n    \"requiredPermissions\": [],\n" +
		"    \"requiredEnv\": []\n  }\n}\n"; string(written) != want {
		t.Fatalf("wrote %q", written)
	}
}

func TestEmitWithoutEntriesRemovesStaleArtifacts(t *testing.T) {
	p := pythonProject(t, map[string]string{"agent.py": pyAgent})
	outDir := t.TempDir()
	for _, file := range []string{ConnectorRequirementsFile, ChannelRequirementsFile, McpConnectionsFile} {
		if err := os.WriteFile(filepath.Join(outDir, file), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	connectors, err := EmitConnectorRequirementsForEntries(t.Context(), p, outDir, nil)
	if err != nil || len(connectors) != 0 {
		t.Fatalf("got %v, %v", connectors, err)
	}
	channels, err := EmitChannelRequirementsForEntries(t.Context(), p, outDir, nil)
	if err != nil || len(channels) != 0 {
		t.Fatalf("got %v, %v", channels, err)
	}
	connections, err := EmitMcpConnectionEntriesForEntries(t.Context(), p, outDir, nil)
	if err != nil || connections != nil {
		t.Fatalf("got %v, %v", connections, err)
	}
	for _, file := range []string{ConnectorRequirementsFile, ChannelRequirementsFile, McpConnectionsFile} {
		if _, err := os.Stat(filepath.Join(outDir, file)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", file, err)
		}
	}
}

func TestEmitRejectsDuplicateNames(t *testing.T) {
	p := pythonProject(t, map[string]string{"agent.py": pyAgent})
	_, err := EmitChannelRequirementsForEntries(t.Context(), p, t.TempDir(),
		[]string{"channels/slack.py", "other/slack.py"})
	requireErrorPrefix(t, err, "duplicate channel name 'slack' from channels/")
}

func TestEmitReportsAChannelThatFailsToImport(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":          tsAgent,
		"channels/slack.ts": "throw new Error(\"no token\");\n",
	})
	_, err := EmitChannelRequirementsForEntries(t.Context(), p, t.TempDir(), []string{"channels/slack.ts"})
	requireErrorContains(t, err, "failed to import channels/slack.ts: Error: no token")
}
