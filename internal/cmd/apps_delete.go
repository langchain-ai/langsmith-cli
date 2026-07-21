package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete APP_ID",
		Short: "Delete a custom app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := uuid.Parse(id); err != nil {
				return fmt.Errorf("invalid app ID %q: expected a UUID (run `langsmith apps list` to find it)", id)
			}

			c := MustGetClient()
			ctx := cmd.Context()

			var apps []customApp
			if err := c.RawGet(ctx, "/v1/platform/custom-apps", &apps); err != nil {
				return fmt.Errorf("looking up custom app %s: %w", id, err)
			}
			var found bool
			var name string
			for _, a := range apps {
				if a.ID == id {
					found, name = true, a.Name
					break
				}
			}
			if !found {
				return fmt.Errorf("custom app %s not found (run `langsmith apps list`)", id)
			}

			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete custom app %q (%s)? [y/N] ", name, id)
				var confirm string
				_, _ = fmt.Fscanln(cmd.InOrStdin(), &confirm)
				if ans := strings.ToLower(strings.TrimSpace(confirm)); ans != "y" && ans != "yes" {
					return errors.New("aborted")
				}
			}

			if err := c.RawDelete(ctx, "/v1/platform/custom-apps/"+id, nil); err != nil {
				return fmt.Errorf("deleting custom app %s: %w", id, err)
			}
			output.OutputJSON(map[string]any{
				"status": "deleted",
				"app_id": id,
			}, "")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
