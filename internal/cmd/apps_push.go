package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsPushCmd() *cobra.Command {
	var (
		name        string
		description string
		contextType string
		entrypoint  string
		buildCmd    string
	)

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload the current directory as a custom app",
		Long: `Upload the current directory as a custom app.

Uploads exactly what's in the current directory, as-is — it does not run a
build for you unless you pass --build. The first push creates the app and
writes .langsmith/app.json recording its ID; every push after that reads
that file and updates the same app in place. Commit .langsmith/app.json so
teammates' pushes update the same app instead of creating new ones.

--context-type and --name only take effect on the first push (creation);
context_type cannot be changed after an app is created.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			if buildCmd != "" {
				if err := runAppsBuildCmd(dir, buildCmd); err != nil {
					return err
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

			c := MustGetClient()
			ctx := context.Background()

			var app customApp
			if link == nil {
				appName := name
				if appName == "" {
					appName = filepath.Base(filepath.Clean(dir))
				}
				payload := map[string]any{
					"name":         appName,
					"files":        files,
					"entrypoint":   entrypoint,
					"context_type": contextType,
				}
				if description != "" {
					payload["description"] = description
				}
				if err := c.RawPost(ctx, "/v1/platform/custom-apps", payload, &app); err != nil {
					return fmt.Errorf("creating custom app: %w", err)
				}
				if err := writeAppLink(dir, appLink{
					AppID:       app.ID,
					Name:        app.Name,
					ContextType: app.ContextType,
				}); err != nil {
					return err
				}
			} else {
				if contextType != "" && contextType != link.ContextType {
					fmt.Fprintf(os.Stderr, "note: --context-type is ignored on update (context_type can't change after creation; this app is %q)\n", link.ContextType)
				}
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
				if err := c.RawPatch(ctx, "/v1/platform/custom-apps/"+link.AppID, payload, &app); err != nil {
					return fmt.Errorf("updating custom app %s: %w", link.AppID, err)
				}
				if err := writeAppLink(dir, appLink{
					AppID:       app.ID,
					Name:        app.Name,
					ContextType: app.ContextType,
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
			if link != nil {
				status = "updated"
			}
			output.OutputJSON(map[string]any{
				"status":       status,
				"app_id":       app.ID,
				"name":         app.Name,
				"context_type": app.ContextType,
				"entrypoint":   app.Entrypoint,
				"files":        paths,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "App name (required on first push; renames on later pushes if passed)")
	cmd.Flags().StringVar(&description, "description", "", "App description")
	cmd.Flags().StringVar(&contextType, "context-type", "none", "Context type on creation: none, annotation_queue, or experiment")
	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to render")
	cmd.Flags().StringVar(&buildCmd, "build", "", "Shell command to run in the current directory before uploading (e.g. \"npm run build\")")
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
