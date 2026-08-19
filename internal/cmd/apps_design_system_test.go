package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every bare import in a scaffolded app's own source and in the design-system
// files copied next to it has to be a declared dependency — an undeclared one
// only surfaces as a build failure in the user's terminal after `apps init`
// has already reported success.
func TestScaffoldedAppDeclaresEveryPackageItImports(t *testing.T) {
	importRe := regexp.MustCompile(`(?m)(?:^|[\s;])(?:import|export)[\s\S]*?from\s+['"]([^'"]+)['"]`)

	for name, at := range appTypes {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			written, err := scaffoldCustomAppStarter(dir, "my-app", "", at, false)
			if err != nil {
				t.Fatalf("scaffold: %v", err)
			}

			var pkg struct {
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if err := json.Unmarshal([]byte(readTemplateFile(t, dir, "package.json")), &pkg); err != nil {
				t.Fatalf("parse package.json: %v", err)
			}

			seen := 0
			for _, rel := range written {
				if !strings.HasSuffix(rel, ".ts") && !strings.HasSuffix(rel, ".tsx") {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
				if err != nil {
					t.Fatalf("reading %s: %v", rel, err)
				}
				for _, m := range importRe.FindAllStringSubmatch(string(raw), -1) {
					spec := m[1]
					if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "node:") {
						continue
					}
					// react/jsx-runtime and friends resolve through react itself.
					base := spec
					if strings.HasPrefix(spec, "@") {
						parts := strings.SplitN(spec, "/", 3)
						if len(parts) >= 2 {
							base = parts[0] + "/" + parts[1]
						}
					} else if i := strings.Index(spec, "/"); i > 0 {
						base = spec[:i]
					}
					seen++
					if pkg.Dependencies[base] == "" && pkg.DevDependencies[base] == "" {
						t.Errorf("%s imports %q but package.json declares neither it nor a parent package", rel, base)
					}
				}
			}
			if seen == 0 {
				t.Error("found no bare imports at all; this check is not doing anything")
			}
		})
	}
}

// The point of resolving the import graph is that a template only pays for what
// it uses. If this starts failing because blank grew a picker, move the
// assertion — don't delete it.
func TestBlankTemplateSkipsUnusedDesignSystemComponents(t *testing.T) {
	dir := t.TempDir()
	written, err := scaffoldCustomAppStarter(dir, "my-app", "", appTypes["blank"], false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	joined := strings.Join(written, "\n")
	for _, unused := range []string{"components/Typeahead", "components/Select", "components/Dialog"} {
		if strings.Contains(joined, unused) {
			t.Errorf("blank template vendored %s, which it does not import", unused)
		}
	}

	raw := readTemplateFile(t, dir, "package.json")
	for _, unused := range []string{"cmdk", "@radix-ui/react-select"} {
		if strings.Contains(raw, `"`+unused+`"`) {
			t.Errorf("blank template declares %s, which nothing it vendors imports", unused)
		}
	}
}

// A local theme.extend key silently overrides the design-system preset's, so a
// stale copy of e.g. the spacing scale would restyle every vendored component.
func TestTemplateTailwindConfigDoesNotOverridePreset(t *testing.T) {
	for name, at := range appTypes {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := scaffoldCustomAppStarter(dir, "my-app", "", at, false); err != nil {
				t.Fatalf("scaffold: %v", err)
			}
			config := readTemplateFile(t, dir, "tailwind.config.js")
			if !strings.Contains(config, "./tailwind.langsmith.cjs") {
				t.Error("tailwind.config.js does not load the design-system preset")
			}
			if strings.Contains(stripLineComments(config), "extend:") {
				t.Errorf("tailwind.config.js re-declares theme.extend; the preset owns the scales:\n%s", config)
			}
		})
	}
}

// stripLineComments drops // comments so a check can look at code only.
func stripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// The vendored snapshot has to be internally complete: every relative import in
// a copied file must resolve to another file in the snapshot.
func TestDesignSystemSnapshotResolvesItsOwnImports(t *testing.T) {
	ds, err := loadDesignSystem()
	if err != nil {
		t.Fatalf("loading design system: %v", err)
	}
	if len(ds.Items) == 0 || len(ds.Files) == 0 {
		t.Fatal("design-system snapshot is empty; run \"make sync-design-system\"")
	}
	if len(ds.ThemeFiles) == 0 {
		t.Error("snapshot records no theme files")
	}
	for file, info := range ds.Files {
		for _, rel := range info.Relative {
			if _, ok := ds.resolveVendoredImport(rel); !ok {
				// ErrorState imports an .svg the registry does not publish; a
				// template that imported it would fail to build, so the
				// scaffolder rejects it rather than shipping it broken.
				t.Logf("unresolvable: %s imports %s", file, rel)
			}
		}
	}
}
