package cmd

import (
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share APP_ID_OR_NAME",
		Short: "Share a custom app with the whole organization",
		Long: `Share one of this workspace's custom apps with its organization.

Shared apps are visible from every workspace in the organization. Any
workspace with permission to update custom apps can move a shared app into
its own scope with "langsmith apps claim". Claim does not create a copy:
other workspaces lose access, and their annotation queues have this app
cleared as the active selection. The queues themselves are kept.`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := cmd.Context()

			app, err := resolveCustomApp(ctx, c, args[0])
			if err != nil {
				return err
			}
			if app.tier() == appScopeOrg {
				return fmt.Errorf("custom app %q (%s) is already shared with this organization", app.Name, app.ID)
			}

			if err := c.RawPost(ctx, c.CustomAppPath(app.ID)+"/share", nil, nil); err != nil {
				if client.IsConflict(err) {
					return fmt.Errorf("another custom app named %q is already shared with this organization; rename this one with `langsmith apps push --name \"New Name\"` first", app.Name)
				}
				return fmt.Errorf("sharing custom app %s: %w", app.ID, err)
			}

			return output.OutputJSON(map[string]any{
				"status": "shared",
				"app_id": app.ID,
				"name":   app.Name,
				"scope":  appScopeOrg,
			}, "")
		},
	}
	return cmd
}
