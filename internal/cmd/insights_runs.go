package cmd

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newInsightsRunsCmd() *cobra.Command {
	var project, projectID, clusterID, sortKey, sortOrder, outputFile string
	var limit, offset int64
	cmd := &cobra.Command{
		Use: "runs ID", Short: "List evidence runs from an Insights report or category",
		Long: "Read one bounded page of report runs, with extracted summaries and attributes.\nInputs and outputs are previews, not guaranteed full payloads; use run get --full\nfor full evidence. Attribute sorting applies within each page, not across the report.\nUse the returned next_offset for the next request; missing source runs may reduce coverage.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := uuid.Parse(args[0]); err != nil {
				return fmt.Errorf("report ID must be a UUID")
			}
			if cmd.Flags().Changed("cluster-id") {
				if _, err := uuid.Parse(clusterID); err != nil {
					return fmt.Errorf("--cluster-id must be a UUID")
				}
			}
			if limit < 1 || limit > 100 || offset < 0 {
				return fmt.Errorf("--limit must be 1-100 and --offset must be nonnegative")
			}
			if sortOrder != "asc" && sortOrder != "desc" {
				return fmt.Errorf("--sort-order must be asc or desc")
			}
			if cmd.Flags().Changed("sort-order") && sortKey == "" {
				return fmt.Errorf("--sort-order requires --sort-by")
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			id, err := resolveSessionID(cmd.Context(), c, project, projectID, "insights runs")
			if err != nil {
				return err
			}
			params := langsmith.SessionInsightGetRunsParams{Limit: langsmith.F(limit), Offset: langsmith.F(offset)}
			if clusterID != "" {
				params.ClusterID = langsmith.F(clusterID)
			}
			if sortKey != "" {
				params.AttributeSortKey = langsmith.F(sortKey)
				params.AttributeSortOrder = langsmith.F(langsmith.SessionInsightGetRunsParamsAttributeSortOrder(sortOrder))
			}
			page, err := c.SDK.Sessions.Insights.GetRuns(cmd.Context(), id, args[0], params)
			if err != nil {
				return fmt.Errorf("fetching Insights runs: %w", err)
			}
			var next any
			if !page.JSON.Offset.IsNull() && page.Offset > offset {
				next = page.Offset
			}
			if page.Runs == nil {
				page.Runs = []map[string]interface{}{}
			}
			if GetFormat() == "pretty" && outputFile == "" {
				rows := make([][]string, 0, len(page.Runs))
				for _, run := range page.Runs {
					rows = append(rows, []string{fmt.Sprint(run["id"]), fmt.Sprint(run["name"]), fmt.Sprint(run["summary"])})
				}
				output.OutputTable([]string{"ID", "Name", "Summary"}, rows, "Insights Runs")
				fmt.Fprintf(cmd.ErrOrStderr(), "Returned %d runs; next offset: %v. IO is preview-only.\n", len(rows), next)
				return nil
			}
			return output.OutputJSON(map[string]any{"project_id": id, "workspace_id": nilStr(GetWorkspaceID()), "job_id": args[0], "cluster_id": nilStr(clusterID),
				"runs": page.Runs, "io_mode": "preview", "sort_scope": "page", "pagination": map[string]any{"limit": limit, "offset": offset, "returned": len(page.Runs), "next_offset": next}}, outputFile)
		},
	}
	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&clusterID, "cluster-id", "", "Restrict evidence to this category UUID")
	cmd.Flags().Int64VarP(&limit, "limit", "n", 20, "Page size (1-100)")
	cmd.Flags().Int64Var(&offset, "offset", 0, "Offset returned by the previous page")
	cmd.Flags().StringVar(&sortKey, "sort-by", "", "Extracted attribute name; sorts this page only")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "asc", "Attribute sort order: asc or desc")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}
