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
//go:embed all:templates/custom-app-starter
var customAppStarterFS embed.FS

const customAppStarterRoot = "templates/custom-app-starter"

//go:embed templates/agents-md
var agentsMDFS embed.FS

var validAppContextTypes = map[string]bool{
	"none":             true,
	"annotation_queue": true,
}

type customAppStarterVars struct {
	Name        string
	Description string
}

func newAppsInitCmd() *cobra.Command {
	var (
		name        string
		description string
		contextType string
		force       bool
		skipInstall bool
	)

	cmd := &cobra.Command{
		Use:   "init --name NAME",
		Short: "Scaffold a starter custom app in the current directory",
		Long: `Scaffold a starter custom app in the current directory.

Writes a small React/TS npm project (a real annotation-queue review UI —
run list, inputs/outputs, feedback rubric, reviewer notes) with a
render(data, root) entrypoint, an AGENTS.md describing the LangSmith API
surface it can call, and a README explaining the bridge contract. This only
writes local files — it does not create anything remotely. Run
"langsmith apps push" once you're ready to upload it.

--context-type selects which AGENTS.md gets written (none: full API catalog
+ docs link; annotation_queue: the curated annotation-queues/feedback
subset) — it does not change which files get scaffolded, since this is the
only starter template for now.

By default this also runs "npm install" and "npm run build" in the new
directory, so "langsmith apps dev" has a dist/bundle.js to serve right away
instead of 404ing until you build it yourself. Pass --skip-install to just
write the files. A failed install/build doesn't fail the command — it's a
convenience, not a requirement — but you'll need to build manually before
"apps dev" or "apps push" will work.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !validAppContextTypes[contextType] {
				return fmt.Errorf("--context-type must be one of: none, annotation_queue")
			}
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			written, err := scaffoldCustomAppStarter(dir, name, description, contextType, force)
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
				"context_type": contextType,
				"files":        written,
				"built":        built,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the app, written into package.json/README (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into README.md")
	cmd.Flags().StringVar(&contextType, "context-type", "annotation_queue", "Selects which AGENTS.md gets written: none or annotation_queue")
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

func scaffoldCustomAppStarter(dir, name, description, contextType string, force bool) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if contextType == "" {
		contextType = "annotation_queue"
	}
	if !validAppContextTypes[contextType] {
		return nil, fmt.Errorf("context type must be one of: none, annotation_queue")
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
	err := fs.WalkDir(customAppStarterFS, customAppStarterRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := strings.TrimPrefix(path, customAppStarterRoot+"/")
		if rel == customAppStarterRoot || d.IsDir() {
			return nil
		}

		raw, err := customAppStarterFS.ReadFile(path)
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

	agentsMD, err := agentsMDFS.ReadFile("templates/agents-md/" + contextType + ".md")
	if err != nil {
		return nil, fmt.Errorf("reading embedded AGENTS.md for context type %q: %w", contextType, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), agentsMD, 0o644); err != nil {
		return nil, fmt.Errorf("writing AGENTS.md: %w", err)
	}
	written = append(written, "AGENTS.md")

	return written, nil
}

var templatedStarterFiles = map[string]bool{
	"README.md":    true,
	"package.json": true,
}
