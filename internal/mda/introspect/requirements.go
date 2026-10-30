package introspect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The emit passes of `compile/requirements.rs`: connector requirements, MCP
// connection entries and channel requirements, as the project's own
// `managed-deepagents` computes them (`requirementsFromConnectors` /
// `requirements_from_connectors`, `requirementsFromChannels` /
// `requirements_from_channels`) from the live `tools/mcp` and `channels/`
// exports.
//
// The Rust CLI answered these by running scripts in the compiled build
// directory, after installing the build's dependencies there. The helper answers
// them from the source project instead, in the same import that answers every
// other question, so they need no build directory: the modules the build copies
// are the project's, and the project already has its dependencies installed.
// The emit functions still write their artifacts into the build directory, as
// the Rust ones did. The functions that read those artifacts and work with
// requirements (`required_env_from_connector_requirements`, …) stay in
// `compile`.

// ConnectorRequirements are connector requirements keyed by their file-derived
// connector name. Each value is the shape formerly known as a "channel
// manifest" — required env, permissions, and opaque provider config.
type ConnectorRequirements = map[string]any

// ChannelRequirements have the same manifest shape as connector requirements.
type ChannelRequirements = ConnectorRequirements

// ConnectorRequirementsFile is the artifact [EmitConnectorRequirementsForEntries]
// writes.
const ConnectorRequirementsFile = "connector-manifests.json"

// ChannelRequirementsFile is the artifact [EmitChannelRequirementsForEntries]
// writes.
const ChannelRequirementsFile = "channel-manifests.json"

// McpConnectionsFile is the artifact [EmitMcpConnectionEntriesForEntries]
// writes.
const McpConnectionsFile = "mcp-connections.json"

// McpConnectionEntry is one connection declaration of a live
// `defineMcp(...)` / `define_mcp(...)` module.
type McpConnectionEntry struct {
	ConnectorName string `json:"connector_name"`
	ServerName    string `json:"server_name"`
	URL           string `json:"url"`
	Slug          string `json:"slug"`
	OwnerPolicy   string `json:"owner_policy"`
}

// EmitChannelRequirementsForEntries computes the channel requirements of the
// exported channels and writes [ChannelRequirementsFile] into outDir.
//
// entries are the channel modules [ScanExportedChannels] found.
func EmitChannelRequirementsForEntries(
	ctx context.Context,
	p *project.Project,
	outDir string,
	entries []string,
) (ChannelRequirements, error) {
	names, err := uniqueStemNames(entries, "channel", "duplicate channel name '%s' from channels/")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		removeArtifact(outDir, ChannelRequirementsFile)
		return ChannelRequirements{}, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	raw, err := requirementsSection(doc.ChannelRequirements, "channels/")
	if err != nil {
		return nil, fmt.Errorf("failed to emit channel requirements. Ensure `channels/*` exports "+
			"branded Channel objects and that project dependencies are installed: %w", err)
	}
	return writeRequirements(outDir, ChannelRequirementsFile, raw)
}

// EmitConnectorRequirementsForEntries computes the connector requirements of
// the `tools/mcp` declaration and writes [ConnectorRequirementsFile] into
// outDir.
//
// entries are the connector modules [ScanExportedConnectors] found. Unlike the
// former channel manifests (where every channel always had one), not every
// connector declares requirements, so this never fails on a discovered name
// missing from the result.
func EmitConnectorRequirementsForEntries(
	ctx context.Context,
	p *project.Project,
	outDir string,
	entries []string,
) (ConnectorRequirements, error) {
	names, err := uniqueStemNames(entries, "connector", "duplicate connector name '%s'")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		removeArtifact(outDir, ConnectorRequirementsFile)
		return ConnectorRequirements{}, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	raw, err := requirementsSection(doc.ConnectorRequirements, "tools/mcp")
	if err != nil {
		return nil, fmt.Errorf("failed to emit connector requirements. Ensure `tools/mcp` exports a "+
			"valid `mcp` declaration and that project dependencies are installed: %w", err)
	}
	return writeRequirements(outDir, ConnectorRequirementsFile, raw)
}

// EmitMcpConnectionEntriesForEntries reads each `defineMcp(...)` declaration's
// named servers and writes [McpConnectionsFile] into outDir.
func EmitMcpConnectionEntriesForEntries(
	ctx context.Context,
	p *project.Project,
	outDir string,
	entries []string,
) ([]McpConnectionEntry, error) {
	names, err := uniqueStemNames(entries, "connector", "duplicate connector name '%s'")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		removeArtifact(outDir, McpConnectionsFile)
		return nil, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	var connections []McpConnectionEntry
	if doc.McpConnections != nil {
		connections, err = doc.McpConnections.get()
		if err != nil {
			return nil, fmt.Errorf("failed to emit MCP connection entries. Ensure `tools/mcp` exports "+
				"a valid `mcp` declaration and that project dependencies are installed: %w", err)
		}
	}
	if connections == nil {
		connections = []McpConnectionEntry{}
	}
	text, err := json.MarshalIndent(connections, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", McpConnectionsFile, err)
	}
	if err := writeArtifact(outDir, McpConnectionsFile, text); err != nil {
		return nil, err
	}
	return connections, nil
}

// uniqueStemNames derives the file-stem names of entries, sorted, failing on a
// duplicate with duplicate (a format taking the name).
func uniqueStemNames(entries []string, kind, duplicate string) ([]string, error) {
	var names []string
	for _, rel := range entries {
		name, err := EntryStemName(rel, kind)
		if err != nil {
			return nil, err
		}
		if slices.Contains(names, name) {
			return nil, fmt.Errorf(duplicate, name)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

// requirementsSection is the requirements object of a document section; a
// project whose modules export nothing to collect has none.
func requirementsSection(s *section[json.RawMessage], what string) (json.RawMessage, error) {
	if s == nil {
		return json.RawMessage("{}"), nil
	}
	raw, err := s.get()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("%s: %w", what, errNoHelperValue)
	}
	return raw, nil
}

// writeRequirements writes raw (the SDK's key order kept) as file and returns
// it decoded.
func writeRequirements(outDir, file string, raw json.RawMessage) (ConnectorRequirements, error) {
	var requirements ConnectorRequirements
	if err := json.Unmarshal(raw, &requirements); err != nil || requirements == nil {
		return nil, fmt.Errorf("%s must be a JSON object", file)
	}
	var text bytes.Buffer
	if err := json.Indent(&text, raw, "", "  "); err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", file, err)
	}
	if err := writeArtifact(outDir, file, text.Bytes()); err != nil {
		return nil, err
	}
	return requirements, nil
}

func writeArtifact(outDir, file string, text []byte) error {
	path := filepath.Join(outDir, file)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", outDir, err)
	}
	if err := os.WriteFile(path, append(text, '\n'), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// removeArtifact removes a stale artifact, as the Rust CLI did, which ignored a
// failure: a stale manifest is overwritten by the next emit.
func removeArtifact(outDir, file string) {
	path := filepath.Join(outDir, file)
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Debug("failed to remove a stale build artifact", "path", path, "error", err)
	}
}
