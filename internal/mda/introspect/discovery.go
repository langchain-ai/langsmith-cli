package introspect

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// The module locators of `compile/discovery.rs` that introspection needs to
// know which modules to import. They live here rather than in `compile`, which
// imports this package; `compile` uses them from here.

// SandboxDir is the project directory holding the sandbox declaration.
const SandboxDir = "sandbox"

// SandboxSetupFile is the setup script baked into the sandbox snapshot.
const SandboxSetupFile = "setup.sh"

// sandboxEntryCandidates are the candidate `sandbox/` index modules for
// language, in priority order.
func sandboxEntryCandidates(language project.Language) []string {
	if language == project.Python {
		return []string{"sandbox/__init__.py"}
	}
	return []string{
		"sandbox/index.ts",
		"sandbox/index.tsx",
		"sandbox/index.mts",
		"sandbox/index.cts",
	}
}

// FindSandboxEntry locates the `sandbox/` index module that exports the named
// `sandbox` declaration, if the project configures one.
func FindSandboxEntry(p *project.Project) (string, bool) {
	return firstCandidate(p, sandboxEntryCandidates(p.Language))
}

// pluginModuleExtensions are the module file extensions that can hold a
// connector or channel, per language.
func pluginModuleExtensions(language project.Language) []string {
	if language == project.Python {
		return []string{"py"}
	}
	return []string{"ts", "tsx", "mts", "cts"}
}

// isCoLocatedTestModule reports whether path looks like a co-located unit-test
// module under a plugin dir.
//
// These are never plugins even when they re-import a `connector` binding
// (Python) or sit next to a real module (TypeScript `*.test.ts`).
func isCoLocatedTestModule(path string) bool {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		return false
	}
	lower := strings.ToLower(stem)
	return strings.HasSuffix(lower, ".test") ||
		strings.HasSuffix(lower, ".spec") ||
		strings.HasPrefix(lower, "test_") ||
		strings.HasSuffix(lower, "_test")
}

// isPluginModule reports whether path is a plugin module directly under the
// project's top-level directory.
//
// Discovery is name-agnostic for real plugin filenames: any module directly
// under the root directory (e.g. `channels/`) with a supported extension is a
// candidate. Python package initializers (`__init__.py`) and co-located unit-test
// modules (`*.test.ts`, `test_*.py`, …) are excluded.
//
// The parent must be exactly directory (not merely *named* like it), so a nested
// directory of the same name anywhere else in the tree (for example a vendored
// package under `.venv/.../site-packages/`) is never misdiscovered as a project
// plugin.
func isPluginModule(path, directory string, language project.Language) bool {
	if filepath.Dir(path) != filepath.FromSlash(directory) {
		return false
	}
	if language == project.Python && filepath.Base(path) == "__init__.py" {
		return false
	}
	if isCoLocatedTestModule(path) {
		return false
	}
	return hasLanguageExtension(path, language)
}

func hasLanguageExtension(path string, language project.Language) bool {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	return ext != "" && slices.Contains(pluginModuleExtensions(language), ext)
}

// FindLegacyConnectorEntries lists the modules directly under the removed
// top-level `connectors/` layout. Compile rejects the ones that still export a
// `connector` so an old MCP declaration is not silently dropped from the
// deployment.
func FindLegacyConnectorEntries(p *project.Project) []string {
	return findPluginEntries(p, "connectors")
}

// mcpEntryCandidates are the candidate MCP declaration modules under `tools/`
// for language, in priority order.
func mcpEntryCandidates(language project.Language) []string {
	if language == project.Python {
		return []string{"tools/mcp.py"}
	}
	return []string{
		"tools/mcp.ts",
		"tools/mcp.tsx",
		"tools/mcp.mts",
		"tools/mcp.cts",
	}
}

// FindMcpEntry locates the canonical MCP declaration, if the project defines
// one.
func FindMcpEntry(p *project.Project) (string, bool) {
	return firstCandidate(p, mcpEntryCandidates(p.Language))
}

// FindChannelEntries locates every channel *candidate* module directly under
// `channels/`, sorted by path for deterministic generated output. Package
// initializers and co-located unit tests are skipped.
func FindChannelEntries(p *project.Project) []string {
	return findPluginEntries(p, "channels")
}

// findPluginEntries lists the plugin modules directly under the top-level
// directory, sorted by path.
func findPluginEntries(p *project.Project, directory string) []string {
	var entries []string
	for _, path := range p.Files {
		if isPluginModule(path, directory, p.Language) {
			entries = append(entries, path)
		}
	}
	slices.Sort(entries)
	return entries
}

// LanguageDirEntries lists the language source modules that are direct
// children of dir (e.g. `schedules/`). Unlike channels, Python `__init__.py`
// and co-located tests are included when present — callers that need plugin
// filtering should use [FindChannelEntries] instead.
func LanguageDirEntries(p *project.Project, dir string) []string {
	var entries []string
	for _, rel := range p.Files {
		if filepath.Dir(rel) == filepath.FromSlash(dir) && hasLanguageExtension(rel, p.Language) {
			entries = append(entries, rel)
		}
	}
	return entries
}

// EntryStemName is the file-stem name for a discovered schedule/connector
// entry.
func EntryStemName(rel, kind string) (string, error) {
	base := filepath.Base(rel)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" || rel == "" {
		return "", fmt.Errorf("invalid %s file name %s", kind, rel)
	}
	return stem, nil
}

// identityEntryCandidates are the candidate root `identity` modules for
// language, in priority order.
func identityEntryCandidates(language project.Language) []string {
	if language == project.Python {
		return []string{"identity.py"}
	}
	return []string{"identity.ts", "identity.tsx", "identity.mts", "identity.cts"}
}

// FindIdentityEntry locates the root `identity` module that exports the named
// `identity` declaration, if the project opts into managed identity.
func FindIdentityEntry(p *project.Project) (string, bool) {
	return firstCandidate(p, identityEntryCandidates(p.Language))
}

// memoryEntryCandidates are the candidate root `memory` modules for language,
// in priority order.
//
// Durable memory is opt-in: a project with none of these mounts nothing.
// Package-dir forms (`memory/index.ts`, `memory/__init__.py`) are intentionally
// unsupported, matching the identity entry candidates above.
func memoryEntryCandidates(language project.Language) []string {
	if language == project.Python {
		return []string{"memory.py"}
	}
	return []string{"memory.ts", "memory.tsx", "memory.mts", "memory.cts"}
}

// FindMemoryEntry locates the root `memory` module that exports the named
// `memory` declaration, if the project opts into durable memory.
func FindMemoryEntry(p *project.Project) (string, bool) {
	return firstCandidate(p, memoryEntryCandidates(p.Language))
}

// ReadSetupScript reads `sandbox/setup.sh`, if present.
func ReadSetupScript(p *project.Project) (string, bool, error) {
	rel := filepath.Join(SandboxDir, SandboxSetupFile)
	if !slices.Contains(p.Files, rel) {
		return "", false, nil
	}
	path := filepath.Join(p.Root, rel)
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return string(contents), true, nil
}

func firstCandidate(p *project.Project, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		rel := filepath.FromSlash(candidate)
		if slices.Contains(p.Files, rel) {
			return rel, true
		}
	}
	return "", false
}
