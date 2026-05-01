package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newHubGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [OWNER/]REPO",
		Short: "Get metadata for a hub repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseHubOwnerRepo(args[0])
			if err != nil {
				return err
			}
			c := MustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/repos/%s/%s", owner, name)
			var envelope struct {
				Repo hubRepo `json:"repo"`
			}
			if err := c.RawGet(ctx, path, &envelope); err != nil {
				return fmt.Errorf("getting %s/%s: %w", owner, name, err)
			}
			output.OutputJSON(envelope.Repo, "")
			return nil
		},
	}
	return cmd
}
