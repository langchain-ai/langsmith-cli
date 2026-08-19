package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

// A snapshot of the LangSmith shadcn registry — the design system's theme,
// utilities, and components — vendored so a scaffolded app is styled and
// componentized without a network round-trip at init time. Refresh it with
// "make sync-design-system"; see scripts/syncdesignsystem.
//
//go:embed all:templates/design-system
var designSystemFS embed.FS

const designSystemSnapshotRoot = "templates/design-system"

// designSystemManifest is written by scripts/syncdesignsystem alongside the
// vendored files.
type designSystemManifest struct {
	Source     string                      `json:"source"`
	SyncedAt   string                      `json:"syncedAt"`
	Items      map[string]designSystemItem `json:"items"`
	Files      map[string]designSystemFile `json:"files"`
	Versions   map[string]string           `json:"versions"`
	ThemeFiles []string                    `json:"themeFiles"`
}

type designSystemItem struct {
	Type  string   `json:"type"`
	Title string   `json:"title"`
	Entry string   `json:"entry"` // e.g. "components/Button"; empty for theme/utils
	Files []string `json:"files"`
}

type designSystemFile struct {
	Relative []string `json:"relative"` // imports of other vendored files, extensionless
	Packages []string `json:"packages"` // bare npm imports
}

var (
	designSystemOnce sync.Once
	designSystem     *designSystemManifest
	designSystemErr  error
)

func loadDesignSystem() (*designSystemManifest, error) {
	designSystemOnce.Do(func() {
		raw, err := designSystemFS.ReadFile(designSystemSnapshotRoot + "/manifest.json")
		if err != nil {
			designSystemErr = fmt.Errorf("reading design-system manifest: %w", err)
			return
		}
		var m designSystemManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			designSystemErr = fmt.Errorf("parsing design-system manifest: %w", err)
			return
		}
		designSystem = &m
	})
	return designSystem, designSystemErr
}

// designSystemImportRoot is what a template writes to import a component:
// import { Button } from '@/components/langsmith/design-system/components/Button'.
const designSystemImportRoot = "components/langsmith/design-system/"

// selectDesignSystemFiles returns the vendored files a template needs: the
// theme (always), plus every component whose directory the template imports and
// everything those components import in turn.
//
// Resolving the real import graph — rather than copying the whole registry or
// trusting item-level registryDependencies — is what keeps a template's
// package.json down to the packages its own components actually pull in.
func (m *designSystemManifest) selectDesignSystemFiles(src string) ([]string, error) {
	selected := map[string]bool{}
	var queue []string

	for _, f := range m.ThemeFiles {
		queue = append(queue, f)
	}

	for file := range m.Files {
		specifier, ok := designSystemImportSpecifier(file)
		if ok && importedBy(src, specifier) {
			queue = append(queue, file)
		}
	}

	for len(queue) > 0 {
		file := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if selected[file] {
			continue
		}
		selected[file] = true
		for _, rel := range m.Files[file].Relative {
			resolved, ok := m.resolveVendoredImport(rel)
			if !ok {
				return nil, fmt.Errorf(
					"design-system file %s imports %q, which the vendored registry snapshot does not contain — re-run \"make sync-design-system\"",
					file, rel)
			}
			queue = append(queue, resolved)
		}
	}

	out := make([]string, 0, len(selected))
	for file := range selected {
		out = append(out, file)
	}
	sort.Strings(out)
	return out, nil
}

// designSystemImportSpecifier turns a vendored file path into the specifier a
// template writes to import it — src/components/langsmith/design-system/
// components/Button/index.ts becomes components/langsmith/design-system/
// components/Button.
func designSystemImportSpecifier(file string) (string, bool) {
	if !strings.HasPrefix(file, "src/") {
		return "", false
	}
	spec := strings.TrimPrefix(file, "src/")
	if ext := path.Ext(spec); ext == ".ts" || ext == ".tsx" {
		spec = strings.TrimSuffix(spec, ext)
	} else {
		return "", false
	}
	spec = strings.TrimSuffix(spec, "/index")
	return spec, strings.HasPrefix(spec, designSystemImportRoot)
}

// importedBy reports whether src imports specifier. The trailing delimiter
// keeps a prefix (".../components/Icon") from matching a longer sibling
// (".../components/IconButton").
func importedBy(src, specifier string) bool {
	for _, end := range []string{"'", `"`, "/"} {
		if strings.Contains(src, specifier+end) {
			return true
		}
	}
	return false
}

// resolveVendoredImport maps an extensionless relative import to a vendored file.
func (m *designSystemManifest) resolveVendoredImport(rel string) (string, bool) {
	for _, candidate := range []string{rel + ".ts", rel + ".tsx", rel + "/index.ts", rel + "/index.tsx", rel} {
		if _, ok := m.Files[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// designSystemPackages splits the npm packages the given vendored files import
// into runtime dependencies (imported by app source) and build-time ones
// (required by the Tailwind preset).
func (m *designSystemManifest) designSystemPackages(files []string) (runtime, build []npmDependency) {
	runtimeSet := map[string]bool{}
	buildSet := map[string]bool{}
	for _, file := range files {
		into := runtimeSet
		if path.Ext(file) == ".cjs" || path.Ext(file) == ".js" {
			into = buildSet
		}
		for _, pkg := range m.Files[file].Packages {
			into[pkg] = true
		}
	}
	return m.npmDependencies(runtimeSet), m.npmDependencies(buildSet)
}

// npmDependency is one "name": "version" line in a rendered package.json.
type npmDependency struct {
	Name    string
	Version string
}

func (m *designSystemManifest) npmDependencies(set map[string]bool) []npmDependency {
	out := make([]npmDependency, 0, len(set))
	for name := range set {
		version := m.Versions[name]
		if version == "" {
			version = "*"
		}
		out = append(out, npmDependency{Name: name, Version: version})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// typesPackages maps a design-system runtime package that ships no TypeScript
// types of its own to the DefinitelyTyped package that supplies them. Every
// scaffolded app gets all of these as devDependencies — see
// resolveAppDependencies.
var typesPackages = map[string]string{
	"d3-scale-chromatic": "^3.0.0",
}
