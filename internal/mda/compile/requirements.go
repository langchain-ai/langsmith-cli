package compile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/hostapi"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
)

// Connector and channel requirements, after the emit.
//
// A connector may declare deploy/runtime requirements (required env,
// permissions, opaque provider config) via an optional `requirements(name)`
// hook. `introspect` computes them from the live `tools/mcp` and `channels/`
// exports (`EmitConnectorRequirementsForEntries` and friends) and writes them
// into the build; this file reads them back and answers what deploy asks of
// them.

// ConnectorRequirements are connector requirements keyed by their file-derived
// connector name.
type ConnectorRequirements = introspect.ConnectorRequirements

// ChannelRequirements have the same manifest shape as connector requirements.
type ChannelRequirements = introspect.ChannelRequirements

// McpConnectionEntry is one connection declaration of a live
// `defineMcp(...)` / `define_mcp(...)` module.
type McpConnectionEntry = introspect.McpConnectionEntry

// The artifacts the requirement emits write into the build.
const (
	ConnectorRequirementsFile = introspect.ConnectorRequirementsFile
	ChannelRequirementsFile   = introspect.ChannelRequirementsFile
	McpConnectionsFile        = introspect.McpConnectionsFile
)

// PythonEmitSiteDir is the build-local site-packages directory older builds
// installed the requirement emit's dependencies into. The emit now runs in the
// project's own environment; a full build still keeps the directory, and the
// deploy archive still omits it.
const PythonEmitSiteDir = ".mda-emit-site"

// RequiredEnvFromConnectorRequirements collects unique `requiredEnv` entries
// from compiled connector requirements.
func RequiredEnvFromConnectorRequirements(requirements ConnectorRequirements) []string {
	return collectSlugs(requirements, "requiredEnv", func(string) bool { return true })
}

// reservedConnectionPrefix: connection names under `mda/` belong to MDA: a
// channel's setup provides them, such as `mda/github-token`, the GitHub App's
// installation token. They are not workspace connections, so deploy neither
// checks for them nor creates them, and a new channel needs no change here.
const reservedConnectionPrefix = "mda/"

func isChannelReservedAgentConnection(slug string) bool {
	return strings.HasPrefix(slug, reservedConnectionPrefix)
}

// ChannelProvidedConnections are the `mda/` agent connections the project's
// channels say their setup provides, such as `mda/github-token`, each with the
// name of the channel that provides it.
func ChannelProvidedConnections(channels ChannelRequirements) map[string]string {
	provided := map[string]string{}
	for _, channel := range sortedKeys(channels) {
		for _, slug := range declaredSlugs(channels[channel], "agentConnections") {
			if isChannelReservedAgentConnection(slug) {
				provided[slug] = channel
			}
		}
	}
	return provided
}

// UnprovidedReservedConnections are the `mda/` agent connections the
// project's connectors read that none of its channels provides, so reading
// them fails at runtime.
func UnprovidedReservedConnections(connectors ConnectorRequirements, channels ChannelRequirements) []string {
	provided := ChannelProvidedConnections(channels)
	return collectSlugs(connectors, "agentConnections", func(slug string) bool {
		_, ok := provided[slug]
		return isChannelReservedAgentConnection(slug) && !ok
	})
}

// declaredSlugs are the string entries of manifest's field array.
func declaredSlugs(manifest any, field string) []string {
	object, ok := manifest.(map[string]any)
	if !ok {
		return nil
	}
	entries, ok := object[field].([]any)
	if !ok {
		return nil
	}
	var slugs []string
	for _, entry := range entries {
		if slug, ok := entry.(string); ok {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// collectSlugs gathers the field entries of every manifest that keep accepts,
// deduplicated and sorted.
func collectSlugs(requirements ConnectorRequirements, field string, keep func(string) bool) []string {
	var out []string
	for _, manifest := range requirements {
		for _, slug := range declaredSlugs(manifest, field) {
			if keep(slug) && !slices.Contains(out, slug) {
				out = append(out, slug)
			}
		}
	}
	slices.Sort(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// ReservedUserConnectionError says why a project cannot declare slug as a user
// connection: names under `mda/` are agent connections a channel's setup
// provides, and reading an app's token as the person running the agent is not
// supported yet. provided is what [ChannelProvidedConnections] returns. False
// when slug may be a user connection.
func ReservedUserConnectionError(slug string, provided map[string]string) (string, bool) {
	if !isChannelReservedAgentConnection(slug) {
		return "", false
	}
	if channel, ok := provided[slug]; ok {
		return fmt.Sprintf(
			"'%s' is not supported as a user connection yet: the %s channel's app "+
				"provides it as an agent connection. Use connections.get(\"%s\", { type: "+
				"\"agent\" }) to act as the app.", slug, channel, slug), true
	}
	return fmt.Sprintf(
		"'%s' cannot be a user connection: names under mda/ are reserved for the "+
			"agent connections channels provide. Use connections.get(\"%s\", { type: "+
			"\"agent\" }) for a channel's token, or rename the connection.", slug, slug), true
}

// UserConnectionsFromConnectorRequirements collects unique user-owned
// connection slugs from compiled connector requirements.
func UserConnectionsFromConnectorRequirements(requirements ConnectorRequirements) []string {
	return collectSlugs(requirements, "userConnections", func(string) bool { return true })
}

// AgentConnectionsFromConnectorRequirements collects unique agent-owned
// connection slugs from compiled connector requirements.
func AgentConnectionsFromConnectorRequirements(requirements ConnectorRequirements) []string {
	return collectSlugs(requirements, "agentConnections", func(slug string) bool {
		return !isChannelReservedAgentConnection(slug)
	})
}

// ReferencedConnectionSlugs are the unique connection slugs referenced by
// compiled connector requirements.
func ReferencedConnectionSlugs(requirements ConnectorRequirements) []string {
	slugs := append(UserConnectionsFromConnectorRequirements(requirements),
		AgentConnectionsFromConnectorRequirements(requirements)...)
	slices.Sort(slugs)
	return slices.Compact(slugs)
}

// ConnectionOwnerPolicyForSlug resolves the owner policy for one slug from
// compiled connector requirements.
//
// False when the slug is absent from the manifest or listed under both agent
// and user ownership.
func ConnectionOwnerPolicyForSlug(requirements ConnectorRequirements, slug string) (hostapi.ConnectionOwnerPolicy, bool) {
	isAgent := slices.Contains(AgentConnectionsFromConnectorRequirements(requirements), slug)
	isUser := slices.Contains(UserConnectionsFromConnectorRequirements(requirements), slug)
	switch {
	case isAgent && !isUser:
		return hostapi.ConnectionOwnerPolicyAgent, true
	case isUser && !isAgent:
		return hostapi.ConnectionOwnerPolicyUser, true
	}
	return 0, false
}

// LoadConnectorRequirements loads requirements previously written by
// [introspect.EmitConnectorRequirementsForEntries].
func LoadConnectorRequirements(outDir string) (ConnectorRequirements, error) {
	return loadRequirements(outDir, ConnectorRequirementsFile)
}

// LoadMcpConnectionEntries loads MCP connection entries previously written by
// [introspect.EmitMcpConnectionEntriesForEntries].
func LoadMcpConnectionEntries(outDir string) ([]McpConnectionEntry, error) {
	path := filepath.Join(outDir, McpConnectionsFile)
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var entries []McpConnectionEntry
	if err := json.Unmarshal(text, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return entries, nil
}

// LoadChannelRequirements loads requirements previously written by
// [introspect.EmitChannelRequirementsForEntries].
func LoadChannelRequirements(outDir string) (ChannelRequirements, error) {
	return loadRequirements(outDir, ChannelRequirementsFile)
}

func loadRequirements(outDir, fileName string) (ConnectorRequirements, error) {
	path := filepath.Join(outDir, fileName)
	text, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ConnectorRequirements{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var value any
	if err := json.Unmarshal(text, &value); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object", fileName)
	}
	return object, nil
}
