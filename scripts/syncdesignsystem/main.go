// Command syncdesignsystem vendors a snapshot of the LangSmith shadcn registry
// into internal/cmd/templates/design-system/.
//
// Scaffolded custom apps run `npm install` with no network access to the
// registry, so the design system has to ship inside the CLI binary. This tool
// pulls the live registry, writes every item's files at the path the registry
// targets, and records a manifest that apps_init.go uses to copy only the
// components a template actually imports — along with exactly the npm packages
// those files import.
//
// Usage:
//
//	make sync-design-system            # or: go run ./scripts/syncdesignsystem
//	go run ./scripts/syncdesignsystem -base https://dev.smith.langchain.com/r
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultBase = "https://smith.langchain.com/r"

// registryIndex is the shape of <base>/registry.json.
type registryIndex struct {
	Name  string `json:"name"`
	Items []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"items"`
}

// registryItem is the shape of <base>/<name>.json.
type registryItem struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Dependencies         []string `json:"dependencies"`
	RegistryDependencies []string `json:"registryDependencies"`
	Files                []struct {
		Path    string `json:"path"`
		Type    string `json:"type"`
		Target  string `json:"target"`
		Content string `json:"content"`
	} `json:"files"`
}

// Manifest is written to design-system/manifest.json and embedded in the CLI.
type Manifest struct {
	Source    string            `json:"source"`
	SyncedAt  string            `json:"syncedAt"`
	Items     map[string]Item   `json:"items"`
	Files     map[string]File   `json:"files"`
	Versions  map[string]string `json:"versions"`
	ThemeFile []string          `json:"themeFiles"`
}

// Item is one registry entry: which vendored files it owns, and what it needs.
type Item struct {
	Type                 string   `json:"type"`
	Title                string   `json:"title"`
	Entry                string   `json:"entry,omitempty"` // import specifier suffix templates use, e.g. "components/Button"
	Files                []string `json:"files"`
	RegistryDependencies []string `json:"registryDependencies,omitempty"`
}

// File records one vendored file's imports, split into the relative ones (which
// pull in more vendored files) and the bare npm ones (which pull in packages).
type File struct {
	Relative []string `json:"relative,omitempty"`
	Packages []string `json:"packages,omitempty"`
}

func main() {
	base := flag.String("base", defaultBase, "registry base URL")
	out := flag.String("out", "internal/cmd/templates/design-system", "output directory (repo-relative)")
	flag.Parse()

	if err := run(*base, *out); err != nil {
		fmt.Fprintln(os.Stderr, "sync-design-system:", err)
		os.Exit(1)
	}
}

func run(base, outDir string) error {
	var index registryIndex
	if err := getJSON(base+"/registry.json", &index); err != nil {
		return fmt.Errorf("fetching registry index: %w", err)
	}
	if len(index.Items) == 0 {
		return fmt.Errorf("registry index has no items")
	}

	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("clearing %s: %w", outDir, err)
	}

	manifest := Manifest{
		Source:   base,
		SyncedAt: time.Now().UTC().Format("2006-01-02"),
		Items:    map[string]Item{},
		Files:    map[string]File{},
		Versions: map[string]string{},
	}

	// Pass 1: every declared npm dependency, so a bare import found in any file
	// can be resolved to the version range the registry pins it to.
	items := make([]registryItem, 0, len(index.Items))
	for _, stub := range index.Items {
		var item registryItem
		if err := getJSON(base+"/"+stub.Name+".json", &item); err != nil {
			return fmt.Errorf("fetching %s: %w", stub.Name, err)
		}
		items = append(items, item)
		for _, dep := range item.Dependencies {
			name, version := splitDependency(dep)
			manifest.Versions[name] = version
		}
	}

	// Pass 2: write files and record their imports.
	for _, item := range items {
		if item.Type == "registry:file" && !isVendorable(item) {
			// Agent-facing extras (skills, docs) that don't belong in an app's src/.
			continue
		}
		entry := Item{Type: item.Type, Title: item.Title, RegistryDependencies: item.RegistryDependencies}
		for _, f := range item.Files {
			rel, ok := targetPath(f.Target, f.Path)
			if !ok {
				return fmt.Errorf("%s: unsupported target %q", item.Name, f.Target)
			}
			dest := filepath.Join(outDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return fmt.Errorf("creating dir for %s: %w", rel, err)
			}
			if err := os.WriteFile(dest, []byte(f.Content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", rel, err)
			}
			entry.Files = append(entry.Files, rel)
			manifest.Files[rel] = fileImports(rel, f.Content, manifest.Versions)
		}
		sort.Strings(entry.Files)
		entry.Entry = componentEntry(entry.Files)
		manifest.Items[item.Name] = entry
	}

	theme, ok := manifest.Items["theme"]
	if !ok {
		return fmt.Errorf("registry has no theme item")
	}
	manifest.ThemeFile = theme.Files

	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Vendored %d registry items (%d files) from %s into %s\n",
		len(manifest.Items), len(manifest.Files), base, outDir)
	return nil
}

// isVendorable reports whether a registry:file item writes into the app itself.
// Items targeting the user's home (agent skills, editor config) are skipped.
func isVendorable(item registryItem) bool {
	for _, f := range item.Files {
		if strings.HasPrefix(f.Target, "~/.") {
			return false
		}
	}
	return true
}

// targetPath maps a registry target to a path inside a scaffolded app.
//
//	@components/x -> src/components/x     ~/x -> x (app root)
func targetPath(target, fallback string) (string, bool) {
	t := target
	if t == "" {
		t = fallback
	}
	switch {
	case strings.HasPrefix(t, "@components/"):
		return "src/components/" + strings.TrimPrefix(t, "@components/"), true
	case strings.HasPrefix(t, "@lib/"):
		return "src/lib/" + strings.TrimPrefix(t, "@lib/"), true
	case strings.HasPrefix(t, "@hooks/"):
		return "src/hooks/" + strings.TrimPrefix(t, "@hooks/"), true
	case strings.HasPrefix(t, "@ui/"):
		return "src/components/ui/" + strings.TrimPrefix(t, "@ui/"), true
	case strings.HasPrefix(t, "~/"):
		return strings.TrimPrefix(t, "~/"), true
	}
	return "", false
}

const designSystemRoot = "src/components/langsmith/design-system/"

// componentEntry returns the import specifier suffix a template would write to
// use this item — "components/Button" for a component directory. Empty for
// items with no single directory (theme, utils).
func componentEntry(files []string) string {
	dir := ""
	for _, f := range files {
		if !strings.HasPrefix(f, designSystemRoot) {
			return ""
		}
		rel := strings.TrimPrefix(f, designSystemRoot)
		d := path.Dir(rel)
		if !strings.HasPrefix(d, "components/") {
			return ""
		}
		if dir == "" {
			dir = d
		} else if dir != d {
			return ""
		}
	}
	return dir
}

var (
	importRe  = regexp.MustCompile(`(?m)(?:^|[\s;])(?:import|export)[\s\S]*?from\s+['"]([^'"]+)['"]`)
	requireRe = regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`)
)

// fileImports splits a source file's module specifiers into the relative ones
// (other vendored files) and the bare npm packages it needs installed. The
// Tailwind preset is CommonJS, so its require()s count too — those are the
// build-time plugins the preset can't load without.
func fileImports(rel, content string, versions map[string]string) File {
	var matches [][]string
	switch {
	case strings.HasSuffix(rel, ".ts"), strings.HasSuffix(rel, ".tsx"):
		matches = importRe.FindAllStringSubmatch(content, -1)
	case strings.HasSuffix(rel, ".cjs"), strings.HasSuffix(rel, ".js"):
		matches = requireRe.FindAllStringSubmatch(content, -1)
	default:
		return File{}
	}
	var out File
	seenRel := map[string]bool{}
	seenPkg := map[string]bool{}
	for _, m := range matches {
		spec := m[1]
		switch {
		case spec == ".", spec == "..", strings.HasPrefix(spec, "./"), strings.HasPrefix(spec, "../"):
			resolved := path.Join(path.Dir(rel), spec)
			if !seenRel[resolved] {
				seenRel[resolved] = true
				out.Relative = append(out.Relative, resolved)
			}
		default:
			pkg := packageName(spec)
			if pkg == "" || pkg == "react" || pkg == "react-dom" {
				continue
			}
			if _, ok := versions[pkg]; !ok {
				// Not pinned by the registry; record it anyway so the scaffold
				// can fail loudly rather than shipping a missing dependency.
				versions[pkg] = "*"
			}
			if !seenPkg[pkg] {
				seenPkg[pkg] = true
				out.Packages = append(out.Packages, pkg)
			}
		}
	}
	sort.Strings(out.Relative)
	sort.Strings(out.Packages)
	return out
}

// packageName strips a deep import path down to the installable package.
//
//	@radix-ui/react-tooltip/foo -> @radix-ui/react-tooltip
func packageName(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") {
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// splitDependency splits "pkg@^1.2.3" into name and version range.
func splitDependency(dep string) (string, string) {
	at := strings.LastIndex(dep, "@")
	if at <= 0 {
		return dep, "*"
	}
	return dep[:at], dep[at+1:]
}

func getJSON(url string, into any) error {
	resp, err := http.Get(url) //nolint:gosec // registry URL comes from a flag
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, into)
}
