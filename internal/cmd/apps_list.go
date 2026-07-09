package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsListCmd() *cobra.Command {
	var contextType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustGetClient()
			ctx := context.Background()

			path := "/v1/platform/custom-apps"
			if contextType != "" {
				path += "?context_type=" + urlEscape(contextType)
			}

			var apps []customApp
			if err := c.RawGet(ctx, path, &apps); err != nil {
				return fmt.Errorf("listing custom apps: %w", err)
			}

			data := make([]map[string]any, 0, len(apps))
			for _, a := range apps {
				data = append(data, map[string]any{
					"id":           a.ID,
					"name":         a.Name,
					"description":  a.Description,
					"context_type": a.ContextType,
					"entrypoint":   a.Entrypoint,
					"is_enabled":   a.IsEnabled,
					"updated_at":   a.UpdatedAt,
				})
			}
			output.OutputJSON(data, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&contextType, "context-type", "", "Filter by context type: none, annotation_queue, or experiment")
	return cmd
}
