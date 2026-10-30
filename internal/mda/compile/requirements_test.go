package compile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
)

// manifest decodes a requirement manifest literal, the way the emitted
// artifacts are decoded.
func manifest(fields map[string][]string) map[string]any {
	out := map[string]any{}
	for field, values := range fields {
		entries := make([]any, len(values))
		for i, value := range values {
			entries[i] = value
		}
		out[field] = entries
	}
	return out
}

func TestRequiredEnvFromConnectorRequirementsDedupesAndSorts(t *testing.T) {
	requirements := ConnectorRequirements{
		"slack":  manifest(map[string][]string{"requiredEnv": {"SLACK_SIGNING_SECRET", "SLACK_BOT_TOKEN"}}),
		"notion": manifest(map[string][]string{"requiredEnv": {"SLACK_BOT_TOKEN"}}),
	}
	if got := RequiredEnvFromConnectorRequirements(requirements); !reflect.DeepEqual(got,
		[]string{"SLACK_BOT_TOKEN", "SLACK_SIGNING_SECRET"}) {
		t.Fatalf("got %v", got)
	}
}

func TestReferencedConnectionSlugsDedupeUserAndAgent(t *testing.T) {
	requirements := ConnectorRequirements{"mcp": manifest(map[string][]string{
		"userConnections":  {"notion"},
		"agentConnections": {"docs-token", "notion"},
	})}
	if got := ReferencedConnectionSlugs(requirements); !reflect.DeepEqual(got, []string{"docs-token", "notion"}) {
		t.Fatalf("got %v", got)
	}
}

func TestUserConnectionsFromConnectorRequirementsDedupeAndSort(t *testing.T) {
	requirements := ConnectorRequirements{
		"first":  manifest(map[string][]string{"userConnections": {"tavily", "notion"}}),
		"second": manifest(map[string][]string{"userConnections": {"notion"}}),
	}
	if got := UserConnectionsFromConnectorRequirements(requirements); !reflect.DeepEqual(got, []string{"notion", "tavily"}) {
		t.Fatalf("got %v", got)
	}
}

func TestAgentConnectionsFromConnectorRequirementsDedupeAndSort(t *testing.T) {
	requirements := ConnectorRequirements{
		"first":  manifest(map[string][]string{"agentConnections": {"docs", "tavily"}}),
		"second": manifest(map[string][]string{"agentConnections": {"docs"}}),
	}
	if got := AgentConnectionsFromConnectorRequirements(requirements); !reflect.DeepEqual(got, []string{"docs", "tavily"}) {
		t.Fatalf("got %v", got)
	}
}

func TestChannelReservedAgentConnectionsAreNotWorkspaceConnections(t *testing.T) {
	// The GitHub channel's setup provides the App's installation token, so
	// deploy neither checks for it nor creates it.
	requirements := ConnectorRequirements{"mcp": manifest(map[string][]string{
		"userConnections":  {"notion"},
		"agentConnections": {"mda/github-token", "mda/slack-bot-token", "docs-token"},
	})}
	if got := ReferencedConnectionSlugs(requirements); !reflect.DeepEqual(got, []string{"docs-token", "notion"}) {
		t.Fatalf("got %v", got)
	}
}

func TestAChannelReservedConnectionIsNotSupportedAsAUserConnection(t *testing.T) {
	requirements := ConnectorRequirements{"mcp": manifest(map[string][]string{"userConnections": {"mda/github-token"}})}

	// It stays listed, so deploy can refuse it.
	if got := UserConnectionsFromConnectorRequirements(requirements); !reflect.DeepEqual(got, []string{"mda/github-token"}) {
		t.Fatalf("got %v", got)
	}
	channels := ChannelRequirements{"github": map[string]any{
		"provider":         "github",
		"agentConnections": []any{"mda/github-token"},
	}}
	provided := ChannelProvidedConnections(channels)
	if !reflect.DeepEqual(provided, map[string]string{"mda/github-token": "github"}) {
		t.Fatalf("provided: %v", provided)
	}
	message, ok := ReservedUserConnectionError("mda/github-token", provided)
	if !ok || message != "'mda/github-token' is not supported as a user connection yet: the github channel's app "+
		"provides it as an agent connection. Use connections.get(\"mda/github-token\", { type: \"agent\" }) to act as the app." {
		t.Fatalf("got %q", message)
	}
	// The whole namespace is reserved, provided by a channel or not.
	message, ok = ReservedUserConnectionError("mda/acme-token", provided)
	if !ok || !strings.Contains(message, "names under mda/ are reserved") {
		t.Fatalf("got %q", message)
	}
	if message, ok := ReservedUserConnectionError("notion", provided); ok {
		t.Fatalf("got %q", message)
	}
}

func TestReservedAgentConnectionsNoChannelProvidesAreListed(t *testing.T) {
	connectors := ConnectorRequirements{"mcp": manifest(map[string][]string{
		"agentConnections": {"mda/github-token", "mda/linear-token", "docs-token"},
	})}
	channels := ChannelRequirements{"linear": map[string]any{
		"provider":         "linear",
		"agentConnections": []any{"mda/linear-token"},
	}}
	if got := UnprovidedReservedConnections(connectors, channels); !reflect.DeepEqual(got, []string{"mda/github-token"}) {
		t.Fatalf("got %v", got)
	}
	if got := UnprovidedReservedConnections(connectors, ChannelRequirements{}); !reflect.DeepEqual(got,
		[]string{"mda/github-token", "mda/linear-token"}) {
		t.Fatalf("got %v", got)
	}
}

func TestConnectionOwnerPolicyForSlugReadsManifestLists(t *testing.T) {
	requirements := ConnectorRequirements{"mcp": manifest(map[string][]string{
		"userConnections":  {"notion"},
		"agentConnections": {"docs-token"},
	})}
	if policy, ok := ConnectionOwnerPolicyForSlug(requirements, "notion"); !ok || policy != hostapi.ConnectionOwnerPolicyUser {
		t.Fatalf("notion: %v %v", policy, ok)
	}
	if policy, ok := ConnectionOwnerPolicyForSlug(requirements, "docs-token"); !ok || policy != hostapi.ConnectionOwnerPolicyAgent {
		t.Fatalf("docs-token: %v %v", policy, ok)
	}
	if _, ok := ConnectionOwnerPolicyForSlug(requirements, "missing"); ok {
		t.Fatal("missing slug has a policy")
	}
	shared := ConnectorRequirements{"mcp": manifest(map[string][]string{
		"userConnections":  {"shared"},
		"agentConnections": {"shared"},
	})}
	if _, ok := ConnectionOwnerPolicyForSlug(shared, "shared"); ok {
		t.Fatal("a slug with both owners has a policy")
	}
}

func TestLoadRequirementsReadsTheEmittedArtifacts(t *testing.T) {
	out := t.TempDir()
	for _, load := range []func(string) (ConnectorRequirements, error){LoadConnectorRequirements, LoadChannelRequirements} {
		if got, err := load(out); err != nil || len(got) != 0 {
			t.Fatalf("missing artifact: %v, %v", got, err)
		}
	}
	if got, err := LoadMcpConnectionEntries(out); err != nil || got != nil {
		t.Fatalf("missing artifact: %v, %v", got, err)
	}

	writeFiles(t, out, map[string]string{
		ConnectorRequirementsFile: `{"mcp": {"userConnections": ["notion"]}}`,
		ChannelRequirementsFile:   `["not", "an", "object"]`,
		McpConnectionsFile: `[{"connector_name": "mcp", "server_name": "notion", "url": "https://mcp.notion.com/mcp", ` +
			`"slug": "notion", "owner_policy": "user"}]`,
	})
	connectors, err := LoadConnectorRequirements(out)
	if err != nil || !reflect.DeepEqual(UserConnectionsFromConnectorRequirements(connectors), []string{"notion"}) {
		t.Fatalf("connectors: %v, %v", connectors, err)
	}
	if _, err := LoadChannelRequirements(out); err == nil || err.Error() != "channel-manifests.json must be a JSON object" {
		t.Fatalf("channels: %v", err)
	}
	entries, err := LoadMcpConnectionEntries(out)
	want := []McpConnectionEntry{{
		ConnectorName: "mcp", ServerName: "notion", URL: "https://mcp.notion.com/mcp", Slug: "notion", OwnerPolicy: "user",
	}}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries: %v, %v", entries, err)
	}
}

func TestConnectorRequirementsForProjectCompilesAndEmits(t *testing.T) {
	f := pythonProject(t, map[string]string{
		"agent.py": pyAgent,
		"tools/mcp.py": "from managed_deepagents import define_mcp\n" +
			"from managed_deepagents import Connection\n" +
			"mcp = define_mcp(servers={'notion': {'transport': 'http', 'url': 'https://mcp.notion.com/mcp', " +
			"'connection': Connection('notion')}})\n",
	})

	requirements, err := ConnectorRequirementsForProject(t.Context(), f.project, f.outDir, f.installation)
	if err != nil {
		t.Fatal(err)
	}
	if got := UserConnectionsFromConnectorRequirements(requirements); !reflect.DeepEqual(got, []string{"notion"}) {
		t.Fatalf("got %v", requirements)
	}
	loaded, err := LoadConnectorRequirements(f.outDir)
	if err != nil || !reflect.DeepEqual(loaded, requirements) {
		t.Fatalf("loaded: %v, %v", loaded, err)
	}
}
