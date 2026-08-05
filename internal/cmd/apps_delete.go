package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete APP_ID_OR_NAME",
		Short: "Delete a custom app by ID or name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := cmd.Context()

			app, err := resolveCustomApp(ctx, c, args[0])
			if err != nil {
				return err
			}
			id, name := app.ID, app.Name

			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete custom app %q (%s)? [y/N] ", name, id)
				var confirm string
				_, _ = fmt.Fscanln(cmd.InOrStdin(), &confirm)
				if ans := strings.ToLower(strings.TrimSpace(confirm)); ans != "y" && ans != "yes" {
					return errors.New("aborted")
				}
			}

			if err := c.RawDelete(ctx, client.CustomAppPath(id), nil); err != nil {
				// Org apps can't be deleted here.
				if client.IsNotFound(err) && app.tier() == appScopeOrg {
					return fmt.Errorf("custom app %q (%s) is shared with the organization, so this workspace can't delete it. Claim a workspace copy with `langsmith apps claim %s` and delete that, or ask an organization admin", name, id, id)
				}
				return fmt.Errorf("deleting custom app %s: %w", id, err)
			}
			output.OutputJSON(map[string]any{
				"status": "deleted",
				"app_id": id,
				"name":   name,
			}, "")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
