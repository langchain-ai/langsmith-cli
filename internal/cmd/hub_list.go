package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newHubListCmd() *cobra.Command {
	var (
		repoType   string
		query      string
		publicOnly bool
		limit      int
		offset     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List hub repos (optionally filtered by type)",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("limit", fmt.Sprintf("%d", limit))
			params.Set("offset", fmt.Sprintf("%d", offset))
			params.Set("is_archived", "false")
			if repoType != "" {
				if repoType != "agent" && repoType != "skill" {
					return fmt.Errorf("--type must be 'agent' or 'skill' when set (got %q)", repoType)
				}
				params.Set("repo_type", repoType)
			}
			if query != "" {
				params.Set("query", query)
				params.Set("match_prefix", "true")
			}
			if cmd.Flags().Changed("public") {
				if publicOnly {
					params.Set("is_public", "true")
				} else {
					params.Set("is_public", "false")
				}
			}

			c := MustGetClient()
			ctx := context.Background()

			var resp hubListResponse
			if err := c.RawGet(ctx, "/repos/?"+params.Encode(), &resp); err != nil {
				return fmt.Errorf("listing hub repos: %w", err)
			}

			if GetFormat() == "pretty" {
				columns := []string{"Full Name", "Type", "Public", "Commits", "Updated"}
				var rows [][]string
				for _, r := range resp.Repos {
					pub := "private"
					if r.IsPublic {
						pub = "public"
					}
					rows = append(rows, []string{
						r.FullName,
						r.RepoType,
						pub,
						fmt.Sprintf("%d", r.NumCommits),
						r.UpdatedAt,
					})
				}
				output.OutputTable(columns, rows, "Hub repos")
				return nil
			}

			output.OutputJSON(map[string]any{
				"total": resp.Total,
				"repos": resp.Repos,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoType, "type", "", "Filter by repo type: agent or skill")
	cmd.Flags().StringVar(&query, "query", "", "Filter by name substring")
	cmd.Flags().BoolVar(&publicOnly, "public", false, "Filter by public/private")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of repos to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	return cmd
}
