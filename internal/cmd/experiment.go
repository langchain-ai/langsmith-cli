package cmd

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newExperimentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experiment",
		Short: "Query evaluation experiments and their results",
		Long: `Query evaluation experiments and their results.

Experiments are evaluation runs that test your application against a
dataset. Each experiment produces feedback scores and run statistics.

Examples:
  langsmith experiment list
  langsmith experiment list --dataset my-eval-dataset
  langsmith experiment get my-experiment-name`,
	}

	cmd.AddCommand(newExperimentListCmd())
	cmd.AddCommand(newExperimentGetCmd())
	return cmd
}

func newExperimentListCmd() *cobra.Command {
	var (
		datasetName string
		limit       int
		outputFile  string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List experiments, optionally filtered by dataset",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}
			params := langsmith.SessionListParams{
				Limit:         langsmith.F(pageSize),
				ReferenceFree: langsmith.F(false),
				IncludeStats:  langsmith.F(true),
			}

			if datasetName != "" {
				ds, err := resolveDataset(ctx, c, datasetName)
				if err != nil {
					exitErrorf("%v", err)
				}
				params.ReferenceDataset = langsmith.F([]string{ds.ID})
			}

			var projects []langsmith.TracerSession
			pager := c.SDK.Sessions.ListAutoPaging(ctx, params)
			for pager.Next() {
				projects = append(projects, pager.Current())
				if limit > 0 && len(projects) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				exitErrorf("listing experiments: %v", err)
			}
			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Dataset ID", "Runs"}
				var rows [][]string
				for _, p := range projects {
					id := p.ID
					if len(id) > 16 {
						id = id[:16] + "..."
					}
					dsID := "N/A"
					if p.ReferenceDatasetID != "" {
						dsID = p.ReferenceDatasetID
						if len(dsID) > 16 {
							dsID = dsID[:16] + "..."
						}
					}
					runCount := "N/A"
					if p.RunCount > 0 {
						runCount = strconv.FormatInt(p.RunCount, 10)
					}
					rows = append(rows, []string{p.Name, id, dsID, runCount})
				}
				output.OutputTable(columns, rows, "Experiments")
			} else {
				var data []map[string]any
				for _, p := range projects {
					entry := map[string]any{
						"id":                   p.ID,
						"name":                 p.Name,
						"reference_dataset_id": nilStr(p.ReferenceDatasetID),
					}
					if p.RunCount > 0 {
						entry["run_count"] = p.RunCount
					}
					if p.FeedbackStats != nil {
						entry["feedback_stats"] = p.FeedbackStats
					}
					data = append(data, entry)
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&datasetName, "dataset", "", "Filter to experiments for this dataset")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of experiments to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newExperimentGetCmd() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "get NAME_OR_ID",
		Short: "Get detailed results for a specific experiment",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nameOrID := args[0]

			c := mustGetClient()
			ctx := context.Background()

			var p langsmith.TracerSession

			// Try UUID first (direct GET), fall back to name search
			if _, err := uuid.Parse(nameOrID); err == nil {
				session, err := c.SDK.Sessions.Get(ctx, nameOrID, langsmith.SessionGetParams{
					IncludeStats: langsmith.F(true),
				})
				if err != nil {
					exitErrorf("fetching experiment by ID: %v", err)
				}
				p = *session
			} else {
				params := langsmith.SessionListParams{
					Name:          langsmith.F(nameOrID),
					Limit:         langsmith.F(int64(1)),
					ReferenceFree: langsmith.F(false),
					IncludeStats:  langsmith.F(true),
				}
				resp, err := c.SDK.Sessions.List(ctx, params)
				if err != nil {
					exitErrorf("fetching experiment: %v", err)
				}
				if len(resp.Items) == 0 {
					exitErrorf("experiment not found: %s", nameOrID)
				}
				p = resp.Items[0]
			}

			// Build output
			data := map[string]any{
				"id":            p.ID,
				"name":          p.Name,
				"feedback_stats": p.FeedbackStats,
				"run_stats": map[string]any{
					"latency":     p.LatencyP50,
					"token_count": p.TotalTokens,
					"error_rate":  p.ErrorRate,
				},
				"example_count": p.RunCount,
			}

			// Try to get total cost
			if p.TotalCost != "" {
				if f, err := strconv.ParseFloat(p.TotalCost, 64); err == nil {
					data["run_stats"].(map[string]any)["total_cost"] = f
				}
			}

			fmt_ := getFormat()
			if fmt_ == "pretty" {
				output.PrintOutput(data, "pretty", outputFile)
			} else {
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

