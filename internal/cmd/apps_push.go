package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsPushCmd() *cobra.Command {
	var (
		name        string
		description string
		entrypoint  string
		noBuild     bool
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload the current directory as a custom app",
		Long: `Upload the current directory as a custom app.

Builds first if package.json has a "build" script, so the upload matches
your current source. Pass --no-build to upload as-is.

The first push creates the app and links this directory to it; later pushes
update the same app. Commit .langsmith/app.json so teammates push to the
same app too.

--name only takes effect on the first push (creation).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			if !noBuild {
				script, pkgJSONExists, scriptErr := packageJSONScript(dir, "build")
				if scriptErr != nil {
					return fmt.Errorf("reading package.json to find a \"build\" script: %w", scriptErr)
				}
				if script != "" {
					if err := runAppsBuildCmd(dir); err != nil {
						return err
					}
				} else if pkgJSONExists {
					fmt.Fprintln(os.Stderr, `note: no "build" script in package.json — skipping automatic build`)
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
				return fmt.Errorf("entrypoint %q not found among uploaded files; pass --entrypoint or check your build produced it", entrypoint)
			}

			sourceArchive, err := buildSourceArchive(dir)
			if err != nil {
				return err
			}

			link, err := readAppLink(dir)
			if err != nil {
				return err
			}
			// A link file can exist with no app_id yet (from "apps init").
			notYetCreated := link == nil || link.AppID == ""

			c := MustGetClient()
			ctx := cmd.Context()

			var app customApp
			updated := false
			if !notYetCreated {
				payload := client.CustomAppRequest{
					Files:         files,
					Entrypoint:    &entrypoint,
					SourceArchive: optionalString(sourceArchive),
					Name:          optionalString(name),
					Description:   optionalString(description),
				}
				err := c.RawPatch(ctx, c.CustomAppPath(link.AppID), payload, &app)
				switch {
				case err == nil:
					updated = true
				case isSourceArchiveRejection(err):
					return sourceArchiveRejectionError(err)
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
				payload := client.CustomAppRequest{
					Name:          &appName,
					Files:         files,
					Entrypoint:    &entrypoint,
					SourceArchive: optionalString(sourceArchive),
					Description:   optionalString(description),
				}
				if err := c.RawPost(ctx, c.CustomAppsPath(), payload, &app); err != nil {
					if isSourceArchiveRejection(err) {
						return sourceArchiveRejectionError(err)
					}
					if client.IsConflict(err) {
						return fmt.Errorf("%w — try `langsmith apps push --name \"New Name\"` instead", err)
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
			result := map[string]any{
				"status":     status,
				"app_id":     app.ID,
				"name":       app.Name,
				"entrypoint": app.Entrypoint,
				"files":      paths,
			}
			if err := output.OutputJSON(result, ""); err != nil {
				return err
			}

			workspaceID := app.TenantID
			if workspaceID == "" {
				workspaceID = GetWorkspaceID()
			}
			if webURL := customAppWebURL(c.APIURL(), workspaceID, app.ID); webURL != "" {
				fmt.Fprintf(os.Stderr, "View at %s\n", webURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "App name (required on first push; renames on later pushes if passed)")
	cmd.Flags().StringVar(&description, "description", "", "App description")
	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to render")
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip building before uploading, even if package.json has a \"build\" script")
	return cmd
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isSourceArchiveRejection(err error) bool {
	if !client.IsBadRequest(err) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "source archive") || strings.Contains(msg, "source_archive")
}

func sourceArchiveRejectionError(err error) error {
	return fmt.Errorf("the server rejected this app's source archive: %w\nRe-run with a smaller source directory, or report this if it persists", err)
}

func runAppsBuildCmd(dir string) error {
	c := exec.Command("npm", "run", "build")
	c.Dir = dir
	c.Stdout = os.Stderr
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("\"npm run build\" failed: %w", err)
	}
	return nil
}
