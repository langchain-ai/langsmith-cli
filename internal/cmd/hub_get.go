package cmd

import (
	"context"
	"fmt"

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

			resp, err := c.SDK.Repos.Get(ctx, owner, name)
			if err != nil {
				return fmt.Errorf("getting %s/%s: %w", owner, name, err)
			}
			outputJSON(sdkRepoToHubRepo(resp.Repo), "")
			return nil
		},
	}
	return cmd
}
