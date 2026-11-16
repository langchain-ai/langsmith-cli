package cmd

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newExperimentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "experiments",
		Aliases: []string{"experiment"},
		Short:   "Query evaluation experiments and their results",
		Long: `Query evaluation experiments and their results.

Experiments are evaluation runs that test your application against a
dataset. Each experiment produces feedback scores and run statistics.

List results are paginated and return at most 20 experiments by default
(use --limit to change).

Examples:
  langsmith experiments list
  langsmith experiments list --dataset my-eval-dataset
  langsmith experiments get my-experiment-name`,
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
		Short: "List experiments, optionally filtered by dataset (default: 20)",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
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
					ExitErrorf("%v", err)
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
				ExitErrorf("listing experiments: %v", err)
			}
			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Dataset ID", "Runs"}
				var rows [][]string
				for _, p := range projects {
					id := p.ID
					dsID := "N/A"
					if p.ReferenceDatasetID != "" {
						dsID = p.ReferenceDatasetID
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
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
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

			c := MustGetClient()
			ctx := context.Background()

			var data map[string]any
			// Try UUID first (direct GET), fall back to name search
			if _, err := uuid.Parse(nameOrID); err == nil {
				session, err := c.SDK.Sessions.GetV2(ctx, nameOrID, langsmith.SessionGetV2Params{
					IncludeStats: langsmith.F(true),
				})
				if err != nil {
					ExitErrorf("fetching experiment by ID: %v", err)
				}
				var totalCost any
				if !session.JSON.TotalCost.IsNull() {
					totalCost = session.TotalCost
				}
				data = experimentData(session.ID, session.Name, lowerFeedbackStatTypes(session.JSON.FeedbackStats.Raw()),
					session.LatencyP50, session.TotalTokens, session.ErrorRate, session.RunCount, totalCost)
			} else {
				params := langsmith.SessionListParams{
					Name:          langsmith.F(nameOrID),
					Limit:         langsmith.F(int64(1)),
					ReferenceFree: langsmith.F(false),
					IncludeStats:  langsmith.F(true),
				}
				resp, err := c.SDK.Sessions.List(ctx, params)
				if err != nil {
					ExitErrorf("fetching experiment: %v", err)
				}
				if len(resp.Items) == 0 {
					ExitErrorf("experiment not found: %s", nameOrID)
				}
				p := resp.Items[0]
				data = experimentData(p.ID, p.Name, p.FeedbackStats,
					p.LatencyP50, p.TotalTokens, p.ErrorRate, p.RunCount, projectTotalCost(p))
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				if err := output.PrintOutput(data, "pretty", outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			} else {
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			}
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	return cmd
}

func experimentData(id, name string, feedbackStats map[string]any, latency float64, tokens int64, errorRate float64, runCount int64, totalCost any) map[string]any {
	runStats := map[string]any{
		"latency":     latency,
		"token_count": tokens,
		"error_rate":  errorRate,
	}
	if totalCost != nil {
		runStats["total_cost"] = totalCost
	}
	return map[string]any{
		"id":             id,
		"name":           name,
		"feedback_stats": feedbackStats,
		"run_stats":      runStats,
		"example_count":  runCount,
	}
}

// lowerFeedbackStatTypes decodes a v2 feedback_stats object. The v2 API
// returns each stat's type in upper case; it is printed in lower case.
func lowerFeedbackStatTypes(raw string) map[string]any {
	var stats map[string]any
	if json.Unmarshal([]byte(raw), &stats) != nil {
		return nil
	}
	for _, v := range stats {
		if stat, ok := v.(map[string]any); ok {
			if t, ok := stat["type"].(string); ok {
				stat["type"] = strings.ToLower(t)
			}
		}
	}
	return stats
}
