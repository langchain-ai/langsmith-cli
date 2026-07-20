package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsPushCmd() *cobra.Command {
	var (
		name        string
		description string
		entrypoint  string
		buildCmd    string
		noBuild     bool
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload the current directory as a custom app",
		Long: `Upload the current directory as a custom app.

Builds before uploading: if package.json has a "build" script (both starter
templates do), this runs it first (like "npm run build"), so the upload
always reflects your current source — not whatever happened to be left in
dist/ by a previous "apps dev" session (e.g. one interrupted mid-rebuild).
Pass --build to run a different command instead, or --no-build to upload
the directory exactly as-is with no build step. The first push creates the
app and writes .langsmith/app.json recording its ID; every push after that
reads that file and updates the same app in place. Commit .langsmith/app.json
so teammates' pushes update the same app instead of creating new ones.

--name only takes effect on the first push (creation).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			if noBuild && buildCmd != "" {
				return fmt.Errorf("--build and --no-build are mutually exclusive")
			}

			if !noBuild {
				cmdToRun := buildCmd
				if cmdToRun == "" {
					script, pkgJSONExists, scriptErr := packageJSONScript(dir, "build")
					if scriptErr != nil {
						return fmt.Errorf("reading package.json to find a \"build\" script: %w", scriptErr)
					}
					if script != "" {
						cmdToRun = "npm run build"
					} else if pkgJSONExists {
						fmt.Fprintln(os.Stderr, `note: no "build" script in package.json — skipping automatic build; pass --build to run one`)
					}
				}
				if cmdToRun != "" {
					if err := runAppsBuildCmd(dir, cmdToRun); err != nil {
						return err
					}
				}
			}

			files, err := readDirectoryAsAppFiles(dir)
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("no files found under %s (after applying exclusions)", dir)
			}
			if _, ok := files[entrypoint]; !ok {
				return fmt.Errorf("entrypoint %q not found among uploaded files; pass --entrypoint or check --build produced it", entrypoint)
			}

			link, err := readAppLink(dir)
			if err != nil {
				return err
			}
			// "apps init" writes .langsmith/app.json immediately (recording the
			// name) with no app_id yet — that's only known once an app is
			// actually created. So a link file existing isn't enough to mean
			// "already created"; app_id being set is.
			notYetCreated := link == nil || link.AppID == ""

			c := MustGetClient()
			ctx := context.Background()

			var app customApp
			updated := false
			if !notYetCreated {
				payload := map[string]any{
					"files":      files,
					"entrypoint": entrypoint,
				}
				if name != "" {
					payload["name"] = name
				}
				if description != "" {
					payload["description"] = description
				}
				err := c.RawPatch(ctx, "/v1/platform/custom-apps/"+link.AppID, payload, &app)
				switch {
				case err == nil:
					updated = true
				case client.IsNotFound(err):
					// The linked app_id no longer exists server-side (e.g. it
					// was deleted through the UI) — .langsmith/app.json is
					// stale. Recreate under the same name rather than failing,
					// and relink to the new app below.
					fmt.Fprintf(os.Stderr, "note: custom app %s no longer exists (it may have been deleted) — creating a new one\n", link.AppID)
				default:
					return fmt.Errorf("updating custom app %s: %w", link.AppID, err)
				}
			}

			if updated {
				if err := writeAppLink(dir, appLink{
					AppID: app.ID,
					Name:  app.Name,
				}); err != nil {
					return err
				}
			} else {
				appName := name
				if appName == "" && link != nil && link.Name != "" {
					appName = link.Name
				}
				if appName == "" {
					appName = filepath.Base(filepath.Clean(dir))
				}
				payload := map[string]any{
					"name":       appName,
					"files":      files,
					"entrypoint": entrypoint,
				}
				if description != "" {
					payload["description"] = description
				}
				if err := c.RawPost(ctx, "/v1/platform/custom-apps", payload, &app); err != nil {
					return fmt.Errorf("creating custom app: %w", err)
				}
				if err := writeAppLink(dir, appLink{
					AppID: app.ID,
					Name:  app.Name,
				}); err != nil {
					return err
				}
			}

			paths := make([]string, 0, len(files))
			for k := range files {
				paths = append(paths, k)
			}
			sort.Strings(paths)

			status := "created"
			if updated {
				status = "updated"
			}
			output.OutputJSON(map[string]any{
				"status":     status,
				"app_id":     app.ID,
				"name":       app.Name,
				"entrypoint": app.Entrypoint,
				"files":      paths,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "App name (required on first push; renames on later pushes if passed)")
	cmd.Flags().StringVar(&description, "description", "", "App description")
	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to render")
	cmd.Flags().StringVar(&buildCmd, "build", "", "Shell command to run in the current directory before uploading, overriding the auto-detected package.json \"build\" script (e.g. \"npm run build\")")
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip building before uploading, even if package.json has a \"build\" script")
	return cmd
}

func runAppsBuildCmd(dir, buildCmd string) error {
	c := exec.Command("sh", "-c", buildCmd)
	c.Dir = dir
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("--build command %q failed: %w", buildCmd, err)
	}
	return nil
}
