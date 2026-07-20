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

Builds first if package.json has a "build" script, so the upload matches
your current source. Pass --build to use a different command, or --no-build
to upload as-is.

The first push creates the app and links this directory to it; later pushes
update the same app. Commit .langsmith/app.json so teammates push to the
same app too.

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
			// A link file can exist with no app_id yet (from "apps init").
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
				case client.IsConflict(err):
					return fmt.Errorf("a custom app named %q already exists in this workspace", name)
				case client.IsNotFound(err):
					// Stale link (app deleted server-side) — recreate instead of failing.
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
					if client.IsConflict(err) {
						return fmt.Errorf("a custom app named %q already exists in this workspace. try `langsmith apps push --name \"New Name\"` instead", appName)
					}
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
	cmd.Flags().StringVar(&buildCmd, "build", "", "Shell command to run before uploading, overriding the auto-detected build script")
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
