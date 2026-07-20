package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

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
			if !yes {
				fmt.Fprintf(os.Stderr, "Delete custom app '%s'? [y/N] ", id)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return errors.New("aborted")
				}
			}

			c := MustGetClient()
			ctx := context.Background()

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
