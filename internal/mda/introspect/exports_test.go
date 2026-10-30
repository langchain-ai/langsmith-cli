package introspect

import (
	"path/filepath"
	"reflect"
	"testing"
)

const tsMcp = "import { defineMcp } from \"managed-deepagents\";\n"

func TestTypescriptToolsMcpExport(t *testing.T) {
	named := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent, "tools/mcp.ts": tsMcp + "export const mcp = defineMcp({ servers: {} });\n",
	})
	entries, err := ScanExportedConnectors(t.Context(), named)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join("tools", "mcp.ts")}; !reflect.DeepEqual(entries, want) {
		t.Fatalf("got %v", entries)
	}

	defaulted := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent, "tools/mcp.ts": tsMcp + "export default defineMcp({ servers: {} });\n",
	})
	_, err = ScanExportedConnectors(t.Context(), defaulted)
	requireErrorContains(t, err, "default export but no `mcp` export")

	missing := pythonProject(t, map[string]string{"agent.py": pyAgent, "tools/mcp.py": "servers = {}\n"})
	_, err = ScanExportedConnectors(t.Context(), missing)
	requireErrorPrefix(t, err, "tools/mcp.py must export a named `mcp` declaration")

	broken := typescriptProject(t, map[string]string{
		"agent.ts": tsAgent, "tools/mcp.ts": "export const mcp = defineMcp({{});\n",
	})
	_, err = ScanExportedConnectors(t.Context(), broken)
	requireErrorPrefix(t, err, "failed to parse tools/mcp.ts:")
}

func TestTheRemovedConnectorsLayoutIsRejected(t *testing.T) {
	for _, source := range []string{
		tsMcp + "export const connector = defineMcp({ servers: {} });\n",
		tsMcp + "const connector = defineMcp({ servers: {} });\nexport { connector };\n",
		tsMcp + "export default defineMcp({ servers: {} });\n",
	} {
		p := typescriptProject(t, map[string]string{"agent.ts": tsAgent, "connectors/mcp.ts": source})
		_, err := ScanExportedConnectors(t.Context(), p)
		requireErrorPrefix(t, err, filepath.Join("connectors", "mcp.ts")+" uses the removed `connectors/` MCP layout.")
	}
	for _, source := range []string{
		"from managed_deepagents import define_mcp\nconnector = define_mcp(servers={})\n",
		"from managed_deepagents import define_mcp as connector\n",
	} {
		p := pythonProject(t, map[string]string{"agent.py": pyAgent, "connectors/mcp.py": source})
		_, err := ScanExportedConnectors(t.Context(), p)
		requireErrorPrefix(t, err, filepath.Join("connectors", "mcp.py")+" uses the removed `connectors/` MCP layout.")
	}

	helpers := pythonProject(t, map[string]string{
		"agent.py": pyAgent, "connectors/helper.py": "mcp_connector = {}\n",
	})
	entries, err := ScanExportedConnectors(t.Context(), helpers)
	if err != nil || entries != nil {
		t.Fatalf("got %v, %v", entries, err)
	}

	broken := pythonProject(t, map[string]string{"agent.py": pyAgent, "connectors/mcp.py": "connector = define_mcp(servers={}\n"})
	_, err = ScanExportedConnectors(t.Context(), broken)
	requireErrorPrefix(t, err, "failed to parse connectors/mcp.py:")
}

func TestScanExportedChannels(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":               tsAgent,
		"channels/slack.ts":      "import { channels } from \"managed-deepagents\";\nexport const channel = channels.slack({ name: \"Support Bot\" });\n",
		"channels/github.ts":     "import * as mda from \"managed-deepagents\";\nconst github = mda.channels.github({ onEvent: {} });\nexport const channel = github;\n",
		"channels/webhook.ts":    "import { channels } from \"managed-deepagents\";\nexport const channel = channels.http({ provider: \"webhook\" });\n",
		"channels/helpers.ts":    "export const format = (text: string) => text.trim();\n",
		"channels/slack.test.ts": "throw new Error(\"tests are never imported\");\n",
	})

	scan, err := ScanExportedChannels(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := ChannelScan{
		Entries: []string{
			filepath.Join("channels", "github.ts"),
			filepath.Join("channels", "slack.ts"),
			filepath.Join("channels", "webhook.ts"),
		},
		RequiresIdentity: true,
		NeedsPublicURL:   true,
		Providers:        []string{"github", "slack"},
	}
	if !reflect.DeepEqual(scan, want) {
		t.Fatalf("got %+v", scan)
	}
	needs, err := ProjectNeedsEventsPublicURL(t.Context(), p)
	if err != nil || !needs {
		t.Fatalf("got %v, %v", needs, err)
	}
}

func TestPythonChannelProviders(t *testing.T) {
	p := pythonProject(t, map[string]string{
		"agent.py":             pyAgent,
		"channels/linear.py":   "import managed_deepagents as mda\nchannel = mda.channels.linear(on_event={})\n",
		"channels/test_x.py":   "raise RuntimeError('tests are never imported')\n",
		"channels/__init__.py": "",
	})
	scan, err := ScanExportedChannels(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := ChannelScan{
		Entries:          []string{filepath.Join("channels", "linear.py")},
		RequiresIdentity: true,
		Providers:        []string{"linear"},
	}
	if !reflect.DeepEqual(scan, want) {
		t.Fatalf("got %+v", scan)
	}
}

func TestChannelModulesWithADefaultExportAreRejected(t *testing.T) {
	p := typescriptProject(t, map[string]string{
		"agent.ts":          tsAgent,
		"channels/slack.ts": "import { channels } from \"managed-deepagents\";\nexport default channels.slack({});\n",
	})
	_, err := ScanExportedChannels(t.Context(), p)
	requireErrorContains(t, err, "has a default export but no `channel` export")
}

func TestParseChannelIngress(t *testing.T) {
	if got := parseChannelIngress(nil); got.isIngress() || got.needsPublicURL() {
		t.Fatalf("got %+v", got)
	}
	for provider, needsURL := range map[string]bool{"slack": true, "github": false, "linear": false} {
		got := parseChannelIngress(&provider)
		if got != (channelIngress{kind: ingressProvider, provider: provider}) || got.needsPublicURL() != needsURL {
			t.Fatalf("%s: got %+v", provider, got)
		}
	}
	unknown := "unknown"
	if got := parseChannelIngress(&unknown); got.kind != ingressUnresolved || !got.isIngress() || !got.needsPublicURL() {
		t.Fatalf("got %+v", got)
	}
}
