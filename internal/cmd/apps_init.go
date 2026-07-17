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

// The all: prefix makes embed.FS include dotfiles (.gitignore) and _-prefixed
// files; plain embed drops any path segment starting with "." or "_".
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

// appType is one --template choice: which starter to scaffold and which
// AGENTS.md ships with it. Map key = the --template value.
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

// appTypeNames returns the valid --template values, sorted, keeping error
// text and help in sync with the map.
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
		skipInstall  bool
	)

	cmd := &cobra.Command{
		Use:   "init --name NAME [--template annotation-queue|annotation-queue-grid|coding-agent-dashboard|experiment-comparison]",
		Short: "Scaffold a starter custom app in the current directory",
		Long: `Scaffold a starter custom app in the current directory.

--template picks which starter gets scaffolded; omit it for a blank
single-file starter. Every app is uploaded and rendered the same way
regardless of choice.

  annotation-queue   A real, working queue-review UI (run list,
                     inputs/outputs viewer, feedback rubric, reviewer
                     notes). It picks a queue itself and fetches everything
                     from the LangSmith API.
  annotation-queue-grid
                     Same queue-review workflow as annotation-queue, but
                     rendered as a spreadsheet: rows are queue runs, columns
                     are rubric keys, cells edited inline, Done per row.
  coding-agent-dashboard
                     A charts dashboard over coding-agent runs: pick a
                     project (or scan all projects), see integration/model
                     share, token/cost/cache economics, tool and subagent
                     usage, errors, activity over time, and per-thread
                     breakdowns.
  experiment-comparison
                     Compare evaluation experiments: pick a dataset, a
                     baseline, and comparisons; see aggregates and a
                     per-example table colored vs the baseline.

This also writes an AGENTS.md describing the LangSmith API
surface available to this app, and a README explaining the bridge contract.
This only writes local files — it does not create anything remotely. Run
"langsmith apps push" once you're ready to upload it.

By default this also runs "npm install" and "npm run build" in the new
directory, so "langsmith apps dev" has a dist/bundle.js to serve right away
instead of 404ing until you build it yourself. Pass --skip-install to just
write the files. A failed install/build doesn't fail the command — it's a
convenience, not a requirement — but you'll need to build manually before
"apps dev" or "apps push" will work.`,
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

			built := false
			if !skipInstall {
				if buildErr := installAndBuildCustomAppStarter(dir); buildErr != nil {
					fmt.Fprintf(os.Stderr, "warning: automatic \"npm install && npm run build\" failed: %v\n(run those yourself — or \"npm run watch\" — before \"langsmith apps dev\"/\"apps push\")\n", buildErr)
				} else {
					built = true
				}
			}

			output.OutputJSON(map[string]any{
				"status":   "scaffolded",
				"dir":      dir,
				"name":     name,
				"template": templateName,
				"files":    written,
				"built":    built,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the app, written into package.json/README (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into README.md")
	cmd.Flags().StringVar(&templateFlag, "template", "", "Starter template: "+strings.Join(appTypeNames(), ", ")+" (omit for a blank starter)")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the current directory is non-empty")
	cmd.Flags().BoolVar(&skipInstall, "skip-install", false, "Skip running \"npm install && npm run build\" after scaffolding")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// installAndBuildCustomAppStarter runs "npm install" then "npm run build" in
// dir, so a freshly scaffolded app has a dist/bundle.js immediately instead
// of only after the user remembers to build it themselves.
func installAndBuildCustomAppStarter(dir string) error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found on PATH")
	}
	if err := runInDir(dir, "npm", "install"); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}
	if err := runInDir(dir, "npm", "run", "build"); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	return nil
}

func runInDir(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	return c.Run()
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

		// Only README.md/package.json actually reference {{.Name}}/{{.Description}}.
		// Running every file through text/template would also try to parse any
		// literal "{{"/"}}" elsewhere as template actions — e.g. React's
		// style={{...}} inline-style syntax — and fail. Copy everything else
		// byte-for-byte.
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
	written = append(written, filepath.Join(appsLinkDir, appsLinkFile))

	return written, nil
}

var templatedStarterFiles = map[string]bool{
	"README.md":    true,
	"package.json": true,
}

// assembleAgentsMD builds an app's AGENTS.md from the generic base
// (_common.md) with the template-specific fragment injected at the marker.
// A blank template (agentsMD == "") yields the base alone.
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
