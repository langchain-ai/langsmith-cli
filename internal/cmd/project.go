package cmd

import (
	"context"
	"fmt"
	"strconv"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "List and inspect tracing projects (sessions)",
		Long: `List and inspect tracing projects (sessions).

Tracing projects collect runs from your application. Each project
is a namespace that groups related traces together.

Note: This lists tracing projects only (not experiments). Use
'langsmith experiment list' for experiments.

Examples:
  langsmith project list
  langsmith project list --limit 10
  langsmith project list --name-contains chatbot`,
	}

	cmd.AddCommand(newProjectListCmd())
	return cmd
}

func newProjectListCmd() *cobra.Command {
	var (
		limit        int
		nameContains string
		outputFile   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracing projects in the workspace",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			params := langsmith.SessionListParams{
				Limit:         langsmith.F(int64(limit)),
				ReferenceFree: langsmith.F(true),
			}
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
			}

			resp, err := c.SDK.Sessions.List(ctx, params)
			if err != nil {
				exitErrorf("listing projects: %v", err)
			}

			projects := resp.Items
			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Runs", "Latency p50", "Error Rate", "Last Active"}
				var rows [][]string
				for _, p := range projects {
					latency := "N/A"
					if p.LatencyP50 > 0 {
						latency = formatTimedelta(p.LatencyP50)
					}
					errorRate := "N/A"
					if p.ErrorRate > 0 {
						errorRate = fmt.Sprintf("%.1f%%", p.ErrorRate*100)
					}
					lastActive := formatTimeShort(p.LastRunStartTime)

					runCount := "N/A"
					if p.RunCount > 0 {
						runCount = strconv.FormatInt(p.RunCount, 10)
					}

					id := p.ID
					if len(id) > 16 {
						id = id[:16] + "..."
					}

					rows = append(rows, []string{p.Name, id, runCount, latency, errorRate, lastActive})
				}
				output.OutputTable(columns, rows, "Tracing Projects")
			} else {
				var data []map[string]any
				for _, p := range projects {
					var totalCost any
					if f, err := strconv.ParseFloat(p.TotalCost, 64); err == nil {
						totalCost = f
					}

					entry := map[string]any{
						"id":                  p.ID,
						"name":                p.Name,
						"description":         nilStr(p.Description),
						"run_count":           p.RunCount,
						"latency_p50":         nilFloat(p.LatencyP50),
						"latency_p99":         nilFloat(p.LatencyP99),
						"total_tokens":        p.TotalTokens,
						"total_cost":          totalCost,
						"error_rate":          nilFloat(p.ErrorRate),
						"last_run_start_time": formatTimeISO(p.LastRunStartTime),
						"start_time":          formatTimeISO(p.StartTime),
					}
					data = append(data, entry)
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of projects to return")
	cmd.Flags().StringVar(&nameContains, "name-contains", "", "Filter projects by name substring")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilFloat(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}
