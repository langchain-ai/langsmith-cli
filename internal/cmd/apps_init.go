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

// all: is required so embed.FS includes dotfiles (.gitignore) — by default
// go:embed silently drops any path segment starting with "." or "_".
//
//go:embed all:templates/blank
var blankStarterFS embed.FS

//go:embed all:templates/annotation-queue
var annotationQueueStarterFS embed.FS

//go:embed templates/agents-md
var agentsMDFS embed.FS

// appType ties together the two things "--type" actually decides: which
// starter code gets scaffolded, and which context_type (and therefore which
// AGENTS.md) the app declares. They're not independent — the
// annotation-queue starter hard-requires a queueId, so there's no valid
// combination where it pairs with contextType "none".
type appType struct {
	templateFS   embed.FS
	templateRoot string
	contextType  string // selects templates/agents-md/<contextType>.md
}

var appTypes = map[string]appType{
	"standalone": {
		templateFS:   blankStarterFS,
		templateRoot: "templates/blank",
		contextType:  "none",
	},
	"annotation-queue": {
		templateFS:   annotationQueueStarterFS,
		templateRoot: "templates/annotation-queue",
		contextType:  "annotation_queue",
	},
}

type customAppStarterVars struct {
	Name        string
	Description string
}

func newAppsInitCmd() *cobra.Command {
	var (
		name        string
		description string
		appTypeFlag string
		force       bool
		skipInstall bool
	)

	cmd := &cobra.Command{
		Use:   "init --name NAME --type standalone|annotation-queue",
		Short: "Scaffold a starter custom app in the current directory",
		Long: `Scaffold a starter custom app in the current directory.

--type picks both what gets scaffolded and what this app declares itself as
(there's no independent template choice — each type has exactly one starter
that matches it):

  standalone        A blank single-file starter (render(data, root) that
                     just dumps data) — no assumptions about shape. Good for
                     a genuinely open-ended app you build up from scratch.
  annotation-queue   A real, working queue-review UI (run list,
                     inputs/outputs viewer, feedback rubric, reviewer
                     notes) — this app receives only { queueId } as context
                     and fetches everything else itself.

Either way this also writes an AGENTS.md describing the LangSmith API
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
			at, ok := appTypes[appTypeFlag]
			if !ok {
				return fmt.Errorf("--type must be one of: standalone, annotation-queue")
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
				"status":       "scaffolded",
				"dir":          dir,
				"name":         name,
				"type":         appTypeFlag,
				"context_type": at.contextType,
				"files":        written,
				"built":        built,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the app, written into package.json/README (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into README.md")
	cmd.Flags().StringVar(&appTypeFlag, "type", "", "App type: standalone or annotation-queue (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the current directory is non-empty")
	cmd.Flags().BoolVar(&skipInstall, "skip-install", false, "Skip running \"npm install && npm run build\" after scaffolding")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")
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
		return nil, fmt.Errorf("--type must be one of: standalone, annotation-queue")
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

	agentsMD, err := agentsMDFS.ReadFile("templates/agents-md/" + at.contextType + ".md")
	if err != nil {
		return nil, fmt.Errorf("reading embedded AGENTS.md for context type %q: %w", at.contextType, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), agentsMD, 0o644); err != nil {
		return nil, fmt.Errorf("writing AGENTS.md: %w", err)
	}
	written = append(written, "AGENTS.md")

	// Record --type's context_type immediately, with no app_id yet (that's
	// only known once "apps push" actually creates the app). Without this,
	// "apps dev" run before the first push has no way to know this is an
	// annotation_queue app at all — the queue-selector bar and --queue-id
	// both key off this file's context_type, not the app_id.
	if err := writeAppLink(dir, appLink{ContextType: at.contextType}); err != nil {
		return nil, fmt.Errorf("writing .langsmith/app.json: %w", err)
	}
	written = append(written, filepath.Join(appsLinkDir, appsLinkFile))

	return written, nil
}

var templatedStarterFiles = map[string]bool{
	"README.md":    true,
	"package.json": true,
}
