package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
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

type customAppStarterVars struct {
	Name        string
	Description string
}

func newAppsInitCmd() *cobra.Command {
	var (
		name        string
		description string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init --name NAME",
		Short: "Scaffold a starter custom app in the current directory",
		Long: `Scaffold a starter custom app in the current directory.

Writes a small npm project with an esbuild build step, a render() entrypoint,
and a README explaining the bridge contract. This only writes local files —
it does not create anything remotely. Run "langsmith apps push" once you're
ready to upload it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			written, err := scaffoldCustomAppStarter(dir, name, description, force)
			if err != nil {
				return err
			}
			sort.Strings(written)
			output.OutputJSON(map[string]any{
				"status": "scaffolded",
				"dir":    dir,
				"name":   name,
				"files":  written,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for the app, written into package.json/README (required)")
	cmd.Flags().StringVar(&description, "description", "", "One-line description written into README.md")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if the current directory is non-empty")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func scaffoldCustomAppStarter(dir, name, description string, force bool) ([]string, error) {
	if name == "" {
		return nil, fmt.Errorf("--name is required")
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

		tmpl, err := template.New(rel).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", rel, err)
		}

		dest := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", rel, err)
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
	return written, nil
}
