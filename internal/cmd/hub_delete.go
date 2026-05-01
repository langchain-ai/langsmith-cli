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

func newHubDeleteCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete [OWNER/]REPO",
		Short: "Delete an agent or skill repo (and its owned child repos)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			owner, name, _, err := parseHubOwnerRepo(args[0])
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(os.Stderr, "Delete hub repo '%s/%s'? [y/N] ", owner, name)
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if strings.ToLower(confirm) != "y" {
					return errors.New("aborted")
				}
			}
			c := MustGetClient()
			ctx := context.Background()

			path := fmt.Sprintf("/v1/platform/hub/repos/%s/%s/directories", owner, name)
			if err := c.RawDelete(ctx, path, nil); err != nil {
				return fmt.Errorf("deleting %s/%s: %w", owner, name, err)
			}
			output.OutputJSON(map[string]any{
				"status": "deleted",
				"owner":  owner,
				"repo":   name,
			}, "")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
