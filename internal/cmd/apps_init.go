package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// all: includes dotfiles like .gitignore; plain embed would drop them.
//
//go:embed all:templates/blank
var blankStarterFS embed.FS

//go:embed all:templates/annotation-queue
var annotationQueueStarterFS embed.FS

//go:embed all:templates/annotation-queue-grid
var annotationQueueGridStarterFS embed.FS

//go:embed all:templates/coding-agent-dashboard
var codingAgentDashboardStarterFS embed.FS

//go:embed all:templates/experiment-comparison
var experimentComparisonStarterFS embed.FS

//go:embed all:templates/agents-md
var agentsMDFS embed.FS

// Small files shared across templates (SearchableSelect, cn(), ...);
// scaffoldCustomAppStarter copies in only the ones a template actually imports.
//
//go:embed all:templates/_shared
var sharedFS embed.FS

const sharedRoot = "templates/_shared"

// One package.json rendered for every template; its dependency lists are
// resolved per template from what the scaffolded source actually imports.
//
//go:embed templates/package.json.tmpl
var sharedPackageJSONTmpl string

// Packages a template's own source (not the vendored design system) may import.
// Versions come from the registry snapshot, so a template and the design-system
// components it sits next to never disagree on an icon set version.
var templateOnlyPackages = []string{"@langchain/untitled-ui-icons", "@radix-ui/react-tooltip"}

// The toolchain every scaffolded app builds with, regardless of template.
// Design-system packages are added on top, resolved from what the template
// actually imports — see apps_design_system.go.
// React 19: the design-system components are authored against @types/react 19
// (`useRef<T | null>` yielding a nullable RefObject, and so on), so a scaffolded
// app on 18 fails `tsc` the moment `shadcn add` pulls one in.
var baseDependencies = []npmDependency{
	{Name: "react", Version: "^19.2.0"},
	{Name: "react-dom", Version: "^19.2.0"},
}

var baseDevDependencies = []npmDependency{
	{Name: "@types/react", Version: "^19.2.0"},
	{Name: "@types/react-dom", Version: "^19.2.0"},
	{Name: "@vitejs/plugin-react", Version: "^4.3.1"},
	{Name: "autoprefixer", Version: "^10.4.19"},
	{Name: "postcss", Version: "^8.4.39"},
	{Name: "shadcn", Version: "^4.18.0"},
	{Name: "tailwindcss", Version: "^3.4.4"},
	{Name: "typescript", Version: "^5.5.3"},
	{Name: "vite", Version: "^5.4.0"},
}

// appType is one --template choice. Map key = the --template value.
type appType struct {
	templateFS   embed.FS
	templateRoot string
	agentsMD     string // template-specific AGENTS.md fragment; "" = none (generic base only)
}

var appTypes = map[string]appType{
	"blank": {
		templateFS:   blankStarterFS,
		templateRoot: "templates/blank",
		agentsMD:     "",
	},
	"annotation-queue": {
		templateFS:   annotationQueueStarterFS,
		templateRoot: "templates/annotation-queue",
		agentsMD:     "annotation_queue",
	},
	"annotation-queue-grid": {
		templateFS:   annotationQueueGridStarterFS,
		templateRoot: "templates/annotation-queue-grid",
		agentsMD:     "annotation_queue_grid",
	},
	"coding-agent-dashboard": {
		templateFS:   codingAgentDashboardStarterFS,
		templateRoot: "templates/coding-agent-dashboard",
		agentsMD:     "coding_agent_dashboard",
	},
	"experiment-comparison": {
		templateFS:   experimentComparisonStarterFS,
		templateRoot: "templates/experiment-comparison",
		agentsMD:     "experiment_comparison",
	},
}

// appTypeNames returns the valid --template values, sorted.
func appTypeNames() []string {
	names := make([]string, 0, len(appTypes))
	for name := range appTypes {
		if name == "blank" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type customAppStarterVars struct {
	Name            string
	Description     string
	Dependencies    []npmDependency
	DevDependencies []npmDependency
}

func newAppsInitCmd() *cobra.Command {
	var (
		name         string
		description  string
		templateFlag string
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "init --name NAME [--template annotation-queue|annotation-queue-grid|coding-agent-dashboard|experiment-comparison]",
		Short: "Scaffold a starter custom app in a new directory named after the app",
		Long: `Scaffold a starter custom app into a new directory named after --name.

--template picks which starter gets scaffolded; omit it for a blank single-file starter.

  annotation-queue        A queue-review UI: RUN/THREAD items, type-specific viewer, feedback rubric.
  annotation-queue-grid   Same review workflow, as an editable spreadsheet.
  coding-agent-dashboard  Charts over coding-agent runs: usage, cost, errors, activity over time.
  experiment-comparison   Compare evaluation experiments against a baseline.

Installs dependencies as the last step, so you can cd in and run
"langsmith apps dev" next. Run "langsmith apps push" to upload.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := templateFlag
			if templateName == "" {
				templateName = "blank"
			} else if templateName == "blank" {
				return fmt.Errorf("--template must be one of: %s", strings.Join(appTypeNames(), ", "))
			}
			at, ok := appTypes[templateName]
			if !ok {
				return fmt.Errorf("--template must be one of: %s", strings.Join(appTypeNames(), ", "))
			}
			slug := slugifyAppName(name)
			if slug == "" {
				return fmt.Errorf("--name %q has no characters usable in a directory name", name)
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			dir := filepath.Join(cwd, slug)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", slug, err)
			}
			written, err := scaffoldCustomAppStarter(dir, name, description, at, force)
			if err != nil {
				return err
			}
			sort.Strings(written)

			fmt.Fprintf(os.Stderr, "Scaffolded %q in %s.\n", templateName, dir)
			if err := installAppDeps(dir); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Next: cd %s && langsmith apps dev.\n", slug)
			output.OutputJSON(map[string]any{
				"status":   "scaffolded",
				"dir":      dir,
				"name":     name,
				"template": templateName,
				"files":    written,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the app, written into package.json/README (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into README.md")
	cmd.Flags().StringVar(&templateFlag, "template", "", "Starter template. Omittable for a blank starter.")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the target directory already exists and is non-empty")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// slugifyAppName turns --name into a directory name: lowercase, non-alphanumeric
// runs collapsed to a single dash, no leading/trailing dashes.
func slugifyAppName(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// installAppDeps runs npm install
func installAppDeps(dir string) error {
	if _, err := exec.LookPath("npm"); err != nil {
		fmt.Fprintln(os.Stderr, `note: npm not found on PATH — run "npm install" before "langsmith apps dev"`)
		return nil
	}
	fmt.Fprintln(os.Stderr, "Installing dependencies: npm install")
	c := exec.Command("npm", "install")
	c.Dir = dir
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("\"npm install\" failed: %w", err)
	}
	return nil
}

// registryNamespace matches the "registries" key in each template's components.json.
const registryNamespace = "@langsmith"

func scaffoldCustomAppStarter(dir, name, description string, at appType, force bool) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if at.templateRoot == "" {
		return nil, fmt.Errorf("--template must be one of: %s", strings.Join(appTypeNames(), ", "))
	}

	if info, err := os.Stat(dir); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s exists and is not a directory", dir)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", dir, err)
		}
		if len(entries) > 0 && !force {
			return nil, fmt.Errorf("%s is not empty; pass --force to write anyway", dir)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", dir, err)
	}

	vars := customAppStarterVars{Name: name, Description: description}
	if vars.Description == "" {
		vars.Description = "TODO: one-sentence description of what this app does."
	}

	sharedPaths, err := usedSharedFiles(at)
	if err != nil {
		return nil, err
	}
	// The design system is resolved against the shared files too — they are
	// what a template's picker or spinner actually imports it through.
	src, err := templateSourceWithShared(at, sharedPaths)
	if err != nil {
		return nil, err
	}

	ds, err := loadDesignSystem()
	if err != nil {
		return nil, err
	}
	dsFiles, err := ds.selectDesignSystemFiles(src)
	if err != nil {
		return nil, err
	}
	vars.Dependencies, vars.DevDependencies = resolveAppDependencies(ds, dsFiles, src)

	var written []string
	err = fs.WalkDir(at.templateFS, at.templateRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(path, at.templateRoot+"/")
		if rel == at.templateRoot || d.IsDir() {
			return nil
		}

		raw, err := at.templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded template %s: %w", rel, err)
		}

		dest := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", rel, err)
		}

		// Only README.md/package.json use template vars; other files may
		// contain literal "{{"/"}}" (e.g. JSX style={{...}}) that would
		// break text/template parsing, so copy them byte-for-byte instead.
		if !templatedStarterFiles[rel] {
			if err := os.WriteFile(dest, raw, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", rel, err)
			}
			written = append(written, rel)
			return nil
		}

		tmpl, err := template.New(rel).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", rel, err)
		}
		f, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("creating %s: %w", rel, err)
		}
		defer f.Close()
		if err := tmpl.Execute(f, vars); err != nil {
			return fmt.Errorf("rendering %s: %w", rel, err)
		}

		written = append(written, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	pkgJSON, err := renderSharedPackageJSON(vars)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), pkgJSON, 0o644); err != nil {
		return nil, fmt.Errorf("writing package.json: %w", err)
	}
	written = append(written, "package.json")

	sharedWritten, err := writeSharedFiles(dir, sharedPaths)
	if err != nil {
		return nil, err
	}
	written = append(written, sharedWritten...)

	dsWritten, err := writeDesignSystemFiles(dir, dsFiles)
	if err != nil {
		return nil, err
	}
	written = append(written, dsWritten...)

	agentsMD, err := assembleAgentsMD(at.agentsMD)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), agentsMD, 0o644); err != nil {
		return nil, fmt.Errorf("writing AGENTS.md: %w", err)
	}
	written = append(written, "AGENTS.md")

	// Record the name now so a later push reuses it, not the directory basename.
	if err := writeAppLink(dir, appLink{Name: name}); err != nil {
		return nil, fmt.Errorf("writing .langsmith/app.json: %w", err)
	}
	written = append(written, filepath.ToSlash(filepath.Join(appsLinkDir, appsLinkFile)))

	return written, nil
}

// writeDesignSystemFiles copies the selected registry snapshot files into the
// scaffolded app, at the same paths `npx shadcn add @langsmith/...` would use —
// so a later `add` lands beside them instead of duplicating them.
func writeDesignSystemFiles(dir string, files []string) ([]string, error) {
	written := make([]string, 0, len(files))
	for _, rel := range files {
		raw, err := designSystemFS.ReadFile(designSystemSnapshotRoot + "/" + rel)
		if err != nil {
			return nil, fmt.Errorf("reading vendored design-system file %s: %w", rel, err)
		}
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", rel, err)
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", rel, err)
		}
		written = append(written, rel)
	}
	return written, nil
}

// resolveAppDependencies builds the package.json dependency lists: the fixed
// toolchain, plus whatever the vendored design-system files and the template's
// own source import.
func resolveAppDependencies(ds *designSystemManifest, dsFiles []string, src string) (deps, devDeps []npmDependency) {
	runtime, build := ds.designSystemPackages(dsFiles)

	deps = append(deps, baseDependencies...)
	deps = append(deps, runtime...)
	for _, pkg := range templateOnlyPackages {
		if !strings.Contains(src, "'"+pkg+"'") && !strings.Contains(src, "\""+pkg+"\"") {
			continue
		}
		deps = append(deps, npmDependency{Name: pkg, Version: ds.Versions[pkg]})
	}

	devDeps = append(devDeps, baseDevDependencies...)
	devDeps = append(devDeps, build...)
	// Unconditionally, not just for what this template vendors: `shadcn add`
	// installs a component's runtime packages but never their @types, so an
	// app that adds a design-system component later would otherwise fail
	// typecheck on a package that ships no types of its own.
	for pkg, version := range typesPackages {
		devDeps = append(devDeps, npmDependency{Name: "@types/" + pkg, Version: version})
	}

	return dedupeDependencies(deps), dedupeDependencies(devDeps)
}

// dedupeDependencies sorts by name and drops repeats (react-dom is in the base
// list; the Tailwind preset also requires tailwindcss, and so on).
func dedupeDependencies(deps []npmDependency) []npmDependency {
	sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })
	out := deps[:0]
	for i, dep := range deps {
		if i > 0 && deps[i-1].Name == dep.Name {
			continue
		}
		if dep.Version == "" {
			dep.Version = "*"
		}
		out = append(out, dep)
	}
	return out
}

// usedSharedFiles returns the templates/_shared/ files this template pulls in,
// following imports from shared file to shared file until the set stops growing.
func usedSharedFiles(at appType) ([]string, error) {
	sharedPaths, err := allSharedFilePaths()
	if err != nil {
		return nil, err
	}
	if len(sharedPaths) == 0 {
		return nil, nil
	}

	src, err := templateSource(at)
	if err != nil {
		return nil, err
	}

	used := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for _, relPath := range sharedPaths {
			if used[relPath] {
				continue
			}
			for _, spec := range sharedFileImportSpecifiers(relPath) {
				if !strings.Contains(src, "'"+spec+"'") && !strings.Contains(src, "\""+spec+"\"") {
					continue
				}
				raw, err := sharedFS.ReadFile(sharedRoot + "/" + relPath)
				if err != nil {
					return nil, fmt.Errorf("reading embedded shared file %s: %w", relPath, err)
				}
				used[relPath] = true
				// A newly pulled-in shared file's own imports count too.
				src += "\n" + string(raw)
				grew = true
				break
			}
		}
	}

	out := make([]string, 0, len(used))
	for relPath := range used {
		out = append(out, relPath)
	}
	sort.Strings(out)
	return out, nil
}

// writeSharedFiles copies the given templates/_shared/ files into dir/src/.
func writeSharedFiles(dir string, sharedPaths []string) ([]string, error) {
	var written []string
	for _, relPath := range sharedPaths {
		raw, err := sharedFS.ReadFile(sharedRoot + "/" + relPath)
		if err != nil {
			return nil, fmt.Errorf("reading embedded shared file %s: %w", relPath, err)
		}
		destRel := filepath.Join("src", relPath)
		dest := filepath.Join(dir, destRel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", destRel, err)
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", destRel, err)
		}
		written = append(written, filepath.ToSlash(destRel))
	}
	return written, nil
}

// templateSourceWithShared is every line of TypeScript that will end up in the
// scaffolded app's src/, template and shared alike.
func templateSourceWithShared(at appType, sharedPaths []string) (string, error) {
	src, err := templateSource(at)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(src)
	for _, relPath := range sharedPaths {
		raw, err := sharedFS.ReadFile(sharedRoot + "/" + relPath)
		if err != nil {
			return "", fmt.Errorf("reading embedded shared file %s: %w", relPath, err)
		}
		b.WriteByte('\n')
		b.Write(raw)
	}
	return b.String(), nil
}

// allSharedFilePaths lists files under templates/_shared/, relative to that root.
func allSharedFilePaths() ([]string, error) {
	var paths []string
	err := fs.WalkDir(sharedFS, sharedRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, strings.TrimPrefix(path, sharedRoot+"/"))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking embedded shared files: %w", err)
	}
	return paths, nil
}

// templateSource concatenates every .ts/.tsx file in the template.
func templateSource(at appType) (string, error) {
	var out strings.Builder
	err := fs.WalkDir(at.templateFS, at.templateRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")) {
			return nil
		}
		b, err := at.templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		out.Write(b)
		out.WriteByte('\n')
		return nil
	})
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

// sharedFileImportSpecifiers returns the plausible relative-import specifiers
// for the shared file at sharedRelPath (relative to templates/_shared/).
func sharedFileImportSpecifiers(sharedRelPath string) []string {
	base := strings.TrimSuffix(filepath.Base(sharedRelPath), filepath.Ext(sharedRelPath))
	dir := filepath.Dir(sharedRelPath) // "components", "lib", or "." for a top-level shared file
	specifiers := []string{"./" + base}
	if dir != "." {
		specifiers = append(specifiers,
			"./"+dir+"/"+base,  // top-level src/*.tsx importing src/<dir>/<base>
			"../"+dir+"/"+base, // a different src/<other>/ file importing src/<dir>/<base>
		)
	}
	return specifiers
}

var templatedStarterFiles = map[string]bool{
	"README.md": true,
}

func renderSharedPackageJSON(vars customAppStarterVars) ([]byte, error) {
	tmpl, err := template.New("package.json").Parse(sharedPackageJSONTmpl)
	if err != nil {
		return nil, fmt.Errorf("parsing shared package.json template: %w", err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("rendering package.json: %w", err)
	}
	return []byte(buf.String()), nil
}

// assembleAgentsMD injects the template-specific fragment into the base AGENTS.md.
func assembleAgentsMD(agentsMD string) ([]byte, error) {
	const marker = "<!-- TEMPLATE-SPECIFIC -->"
	common, err := agentsMDFS.ReadFile("templates/agents-md/_common.md")
	if err != nil {
		return nil, fmt.Errorf("reading embedded _common.md: %w", err)
	}
	fragment := ""
	if agentsMD != "" {
		b, err := agentsMDFS.ReadFile("templates/agents-md/" + agentsMD + ".md")
		if err != nil {
			return nil, fmt.Errorf("reading embedded AGENTS.md fragment %q: %w", agentsMD, err)
		}
		fragment = strings.TrimSpace(string(b))
	}
	out := strings.Replace(string(common), marker, fragment, 1)
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return []byte(strings.TrimSpace(out) + "\n"), nil
}
