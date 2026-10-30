package introspect

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Checks that discovered MCP and channel modules expose the export shape the
// runtime collector requires, plus whether any channel declares the ingress
// that makes the identity gate apply (`compile/exports.rs`).
//
// Channel discovery is name-agnostic. Modules without the required named
// `channel` export are skipped so helpers and co-located unit tests are not
// imported at startup. Brand validation still happens in the runtime collector.
//
// "Has no such export" and "could not be imported" must stay distinguishable: a
// module that fails to import is reported, never quietly skipped, or a typo'd
// brace would drop a connector from the deployment without a word.

// ChannelScan is the result of scanning `channels/` once for exports and
// ingress.
type ChannelScan struct {
	// Entries are the modules with a named `channel` export.
	Entries []string
	// RequiresIdentity reports whether any exported module is a first-party
	// ingress channel (`channels.slack(...)`, `channels.github(...)`, …), which
	// mounts Events ingress.
	RequiresIdentity bool
}

// ScanExportedConnectors lists the connector modules collected into
// `_mda_connectors`: the `tools/mcp.{ts,py}` declaration when the project has
// one.
//
// Fails when a module under the removed `connectors/` layout still exports a
// `connector` (or has a default export), so an old MCP declaration is not
// silently dropped from the deployment.
func ScanExportedConnectors(ctx context.Context, p *project.Project) ([]string, error) {
	legacy := FindLegacyConnectorEntries(p)
	rel, hasMcp := FindMcpEntry(p)
	if len(legacy) == 0 && !hasMcp {
		return nil, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := rejectLegacyConnectorModules(doc, legacy); err != nil {
		return nil, err
	}
	if !hasMcp {
		return nil, nil
	}
	if doc.Mcp == nil {
		return nil, fmt.Errorf("%s: %w", rel, errNoHelperValue)
	}
	exported, err := doc.Mcp.get()
	if err != nil {
		return nil, err
	}
	if !exported.HasExport && exported.HasDefault {
		return nil, fmt.Errorf("%s has a default export but no `mcp` export. MCP is collected by name: "+
			"write `export const mcp = defineMcp(...)`.", rel)
	}
	if !exported.HasExport {
		return nil, fmt.Errorf("%s must export a named `mcp` declaration: write `export const mcp = "+
			"defineMcp(...)` (TypeScript) or `mcp = define_mcp(...)` (Python).", rel)
	}
	return []string{rel}, nil
}

func rejectLegacyConnectorModules(doc *document, legacy []string) error {
	if len(doc.LegacyConnectors) != len(legacy) {
		return fmt.Errorf("connectors/: %w", errNoHelperValue)
	}
	for i, rel := range legacy {
		exported, err := doc.LegacyConnectors[i].get()
		if err != nil {
			return err
		}
		// A default export counts too: before the removal it was a hard error
		// here, so treating it as a helper would drop the MCP tools silently.
		if exported.HasExport || exported.HasDefault {
			return fmt.Errorf("%s uses the removed `connectors/` MCP layout. Move the declaration to "+
				"`tools/mcp.ts` or `tools/mcp.py` and export it as `mcp`: write `export const mcp = "+
				"defineMcp(...)` (TypeScript) or `mcp = define_mcp(...)` (Python).", rel)
		}
	}
	return nil
}

// ScanExportedChannels scans the channel modules once: the export gate and the
// ingress of each.
func ScanExportedChannels(ctx context.Context, p *project.Project) (ChannelScan, error) {
	candidates := FindChannelEntries(p)
	var scan ChannelScan
	if len(candidates) == 0 {
		return scan, nil
	}
	doc, err := projectDocument(ctx, p)
	if err != nil {
		return ChannelScan{}, err
	}
	if len(doc.Channels) != len(candidates) {
		return ChannelScan{}, fmt.Errorf("channels/: %w", errNoHelperValue)
	}
	for i, rel := range candidates {
		fields, err := doc.Channels[i].get()
		if err != nil {
			return ChannelScan{}, err
		}
		if !fields.HasExport {
			if fields.HasDefault {
				return ChannelScan{}, fmt.Errorf("%s has a default export but no `channel` export. "+
					"Channels are collected by name: write `export const channel = "+
					"channels.<provider>(...)`.", rel)
			}
			continue
		}
		scan.Entries = append(scan.Entries, rel)
		scan.RequiresIdentity = scan.RequiresIdentity || fields.Ingress != nil
	}
	return scan, nil
}
