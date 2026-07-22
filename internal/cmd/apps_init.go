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

// Small files shared across templates (SearchableSelect, Spinner, cn(), ...);
// scaffoldCustomAppStarter copies in only the ones a template actually imports.
//
//go:embed all:templates/_shared
var sharedFS embed.FS

const sharedRoot = "templates/_shared"

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
	Name        string
	Description string
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
		Short: "Scaffold a starter custom app in the current directory",
		Long: `Scaffold a starter custom app in the current directory.

--template picks which starter gets scaffolded; omit it for a blank single-file starter.

  annotation-queue        A queue-review UI: run list, inputs/outputs, feedback rubric, reviewer notes.
  annotation-queue-grid   Same review workflow, as an editable spreadsheet.
  coding-agent-dashboard  Charts over coding-agent runs: usage, cost, errors, activity over time.
  experiment-comparison   Compare evaluation experiments against a baseline.

Installs dependencies as the last step, so you can run "langsmith apps dev" next.
Run "langsmith apps push" to upload.`,
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
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
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
			fmt.Fprintln(os.Stderr, "Next: langsmith apps dev.")
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
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the current directory is non-empty")
	_ = cmd.MarkFlagRequired("name")
	return cmd
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

	var written []string
	err := fs.WalkDir(at.templateFS, at.templateRoot, func(path string, d fs.DirEntry, walkErr error) error {
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

	sharedWritten, err := writeUsedSharedFiles(dir, at)
	if err != nil {
		return nil, err
	}
	written = append(written, sharedWritten...)

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

// writeUsedSharedFiles copies whichever templates/_shared/ files this
// template's source actually imports into dir/src/.
func writeUsedSharedFiles(dir string, at appType) ([]string, error) {
	sharedPaths, err := allSharedFilePaths()
	if err != nil {
		return nil, err
	}
	if len(sharedPaths) == 0 {
		return nil, nil
	}

	src, err := concatenatedTemplateSource(at)
	if err != nil {
		return nil, err
	}

	var written []string
	for _, relPath := range sharedPaths {
		used := false
		for _, spec := range sharedFileImportSpecifiers(relPath) {
			if strings.Contains(src, "'"+spec+"'") || strings.Contains(src, "\""+spec+"\"") {
				used = true
				break
			}
		}
		if !used {
			continue
		}

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

// concatenatedTemplateSource concatenates every .ts/.tsx file in the template.
func concatenatedTemplateSource(at appType) (string, error) {
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
	"README.md":    true,
	"package.json": true,
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
