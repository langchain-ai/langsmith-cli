package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func newHubListCmd() *cobra.Command {
	var (
		repoType   string
		source     string
		query      string
		publicOnly bool
		limit      int
		offset     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List hub repos (optionally filtered by type)",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := langsmith.RepoListParams{
				Limit:      langsmith.F(int64(limit)),
				Offset:     langsmith.F(int64(offset)),
				IsArchived: langsmith.F(langsmith.RepoListParamsIsArchivedFalse),
			}
			opts := make([]option.RequestOption, 0, 1)
			if repoType != "" {
				if repoType != "agent" && repoType != "skill" {
					return fmt.Errorf("--type must be 'agent' or 'skill' when set (got %q)", repoType)
				}
				params.SingleRepoType = langsmith.F(langsmith.RepoListParamsRepoType(repoType))
			}
			if source != "" {
				if source != "internal" && source != "external" {
					return fmt.Errorf("--source must be 'internal' or 'external' when set (got %q)", source)
				}
				params.Source = langsmith.F(langsmith.RepoListParamsSource(source))
			}
			if query != "" {
				params.Query = langsmith.F(query)
				opts = append(opts, option.WithQuery("match_prefix", "true"))
			}
			if cmd.Flags().Changed("public") {
				if publicOnly {
					params.IsPublic = langsmith.F(langsmith.RepoListParamsIsPublicTrue)
				} else {
					params.IsPublic = langsmith.F(langsmith.RepoListParamsIsPublicFalse)
				}
			}

			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Repos.List(ctx, params, opts...)
			if err != nil {
				return fmt.Errorf("listing hub repos: %w", err)
			}
			repos := make([]hubRepo, 0, len(resp.Repos))
			for _, repo := range resp.Repos {
				repos = append(repos, sdkRepoToHubRepo(repo))
			}

			if GetFormat() == "pretty" {
				columns := []string{"Full Name", "Type", "Public", "Commits", "Updated"}
				var rows [][]string
				for _, r := range repos {
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
				"total": int(resp.Total),
				"repos": repos,
			}, "")
			return nil
		},
	}

	cmd.Flags().StringVar(&repoType, "type", "", "Filter by repo type: agent or skill")
	cmd.Flags().StringVar(&source, "source", "", "Filter by source: internal or external")
	cmd.Flags().StringVar(&query, "query", "", "Filter by name substring")
	cmd.Flags().BoolVar(&publicOnly, "public", false, "Filter by public/private")
	cmd.Flags().IntVarP(&limit, "limit", "n", 100, "Maximum number of repos to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	return cmd
}
