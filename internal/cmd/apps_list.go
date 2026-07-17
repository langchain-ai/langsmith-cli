package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			var apps []customApp
			if err := c.RawGet(ctx, "/v1/platform/custom-apps", &apps); err != nil {
				return fmt.Errorf("listing custom apps: %w", err)
			}

			data := make([]map[string]any, 0, len(apps))
			for _, a := range apps {
				data = append(data, map[string]any{
					"id":          a.ID,
					"name":        a.Name,
					"description": a.Description,
					"entrypoint":  a.Entrypoint,
					"is_enabled":  a.IsEnabled,
					"updated_at":  a.UpdatedAt,
				})
			}
			output.OutputJSON(data, "")
			return nil
		},
	}

	return cmd
}
