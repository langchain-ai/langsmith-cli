package cmd

import (
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsClaimCmd() *cobra.Command {
	var as string

	cmd := &cobra.Command{
		Use:   "claim APP_ID_OR_NAME",
		Short: "Claim an organization-shared custom app into this workspace",
		Long: `Claim a custom app shared with your organization into this workspace.

Only org-shared apps can be claimed — this workspace's own apps already
belong to it. Pass --as to claim it under a different name, which is what
you need when this workspace already has an app by that name.`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := cmd.Context()

			// Only org-tier apps are claimable.
			app, err := resolveCustomAppInScope(ctx, c, args[0], appScopeOrg)
			if err != nil {
				return err
			}

			payload := client.CustomAppRequest{Name: optionalString(as)}
			var claimed customApp
			if err := c.RawPost(ctx, c.CustomAppPath(app.ID)+"/claim", payload, &claimed); err != nil {
				if client.IsConflict(err) {
					taken := as
					if taken == "" {
						taken = app.Name
					}
					return fmt.Errorf("this workspace already has a custom app named %q — claim it under a different name with `langsmith apps claim %q --as \"New Name\"`", taken, args[0])
				}
				return fmt.Errorf("claiming custom app %s: %w", app.ID, err)
			}

			id := claimed.ID
			if id == "" {
				id = app.ID
			}
			name := claimed.Name
			if name == "" {
				name = app.Name
				if as != "" {
					name = as
				}
			}
			output.OutputJSON(map[string]any{
				"status": "claimed",
				"app_id": id,
				"name":   name,
				"scope":  appScopeWorkspace,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&as, "as", "", "Claim the app under this name instead of its shared name")
	return cmd
}
