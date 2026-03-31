package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "List and inspect tracing projects (sessions)",
		Long: `List and inspect tracing projects (sessions).

Tracing projects collect runs from your application. Each project
is a namespace that groups related traces together.

Results are paginated and return at most 20 projects by default
(use --limit to change). Projects are sorted by most recent activity
(last_run_start_time, descending).

Note: This lists tracing projects only (not experiments). Use
'langsmith experiment list' for experiments.

Examples:
  langsmith project list                        # first 20 projects, most recently active first
  langsmith project list --limit 10             # first 10 projects
  langsmith project list --name-contains chatbot`,
	}

	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newAgentVersionsCmd())
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
		Short: "List tracing projects in the workspace (default: 20, sorted by most recent activity)",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			pageSize := int64(20)
			if limit > 0 && int64(limit) < pageSize {
				pageSize = int64(limit)
			}
			params := langsmith.SessionListParams{
				Limit:         langsmith.F(pageSize),
				ReferenceFree: langsmith.F(true),
				IncludeStats:  langsmith.F(true),
			}
			if nameContains != "" {
				params.NameContains = langsmith.F(nameContains)
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
				exitErrorf("listing projects: %v", err)
			}
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
