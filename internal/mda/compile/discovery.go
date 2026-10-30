package compile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Project discovery helpers: locating the optional browser declaration, and
// building the import specifiers the generated entry module uses to reach the
// author's declarations. The module locators introspection needs (sandbox,
// MCP, channels, identity, memory, schedules) are `introspect`'s.

func hasFilesUnder(p *project.Project, directory string) bool {
	return slices.ContainsFunc(p.Files, func(path string) bool {
		first, _, _ := strings.Cut(filepath.ToSlash(path), "/")
		return first == directory
	})
}

// misspelledChannelDirectoryWarning is the advisory for a project with a
// top-level `channel/` tree but no `channels/`.
//
// Channel deploys are discovered only from `channels/`, so a singular
// `channel/` directory is silently ignored — most often a misspelling. Returned
// rather than printed: compile runs inside a spinner under `langsmith deploy`,
// so writing to stderr here would race the spinner's steady tick. Callers
// render it once their spinner has stopped.
func misspelledChannelDirectoryWarning(p *project.Project) (string, bool) {
	if hasFilesUnder(p, "channels") || !hasFilesUnder(p, "channel") {
		return "", false
	}
	return "found `channel/`, but channel deploys are discovered from `channels/` — check the directory name", true
}

// browserEntryCandidates are the candidate root `browser` modules for
// language, in priority order.
//
// Browser use is opt-in: a project with none of these gets no browser.
// Package-dir forms are intentionally unsupported, matching the memory entry
// candidates.
func browserEntryCandidates(language project.Language) []string {
	if language == project.Python {
		return []string{"browser.py"}
	}
	return []string{"browser.ts", "browser.tsx", "browser.mts", "browser.cts"}
}

// findBrowserEntry locates the root `browser` module that exports the named
// `browser` declaration, if the project opts into browser use.
func findBrowserEntry(p *project.Project) (string, bool) {
	for _, candidate := range browserEntryCandidates(p.Language) {
		if slices.Contains(p.Files, candidate) {
			return candidate, true
		}
	}
	return "", false
}

// sandboxImportSpecifier builds the import specifier the generated entry uses
// to reach the user's `sandbox` declaration.
//
// TypeScript imports are extensionless and module-relative
// (`./sandbox/index`). Python imports use the dotted package path, collapsing
// the `__init__` module to its package (`sandbox`).
func sandboxImportSpecifier(sandboxEntry string, language project.Language) string {
	if language != project.Python {
		return agentImportSpecifier(sandboxEntry, language)
	}
	parts := slices.DeleteFunc(modulePathParts(sandboxEntry), func(part string) bool {
		return part == "__init__"
	})
	return strings.Join(parts, ".")
}

// agentImportSpecifier builds the import specifier the generated entry uses to
// reach the user's agent definition.
//
// TypeScript imports are extensionless and module-relative (`./agent`,
// `./src/agent`). Python imports use the dotted module path rooted at the
// build directory (`agent`, `src.agent`).
func agentImportSpecifier(agentFile string, language project.Language) string {
	parts := modulePathParts(agentFile)
	if language == project.Python {
		return strings.Join(parts, ".")
	}
	return "./" + strings.Join(parts, "/")
}

// modulePathParts splits a module path, without its extension, into its
// components.
func modulePathParts(path string) []string {
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	return strings.Split(proc.ToUnixPath(stem), "/")
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
