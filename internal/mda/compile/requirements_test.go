package compile

import (
	"reflect"
	"strings"
	"testing"
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
