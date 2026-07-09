package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsGetCmd() *cobra.Command {
	var showFiles bool

	cmd := &cobra.Command{
		Use:   "get APP_ID",
		Short: "Get a custom app by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c := MustGetClient()
			ctx := context.Background()

			var app customApp
			if err := c.RawGet(ctx, "/v1/platform/custom-apps/"+id, &app); err != nil {
				return fmt.Errorf("getting custom app %s: %w", id, err)
			}
			if !showFiles {
				app.Files = nil
			}
			output.OutputJSON(app, "")
			return nil
		},
	}

	cmd.Flags().BoolVar(&showFiles, "files", false, "Include file contents in the output")
	return cmd
}
