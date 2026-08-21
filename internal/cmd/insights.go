package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

// --- Commands ---

func newInsightsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights",
		Short: "Query insights reports for a project",
		Long: `Query insights reports for a project.

The Insights Agent automatically analyzes traces to detect usage patterns,
common agent behaviors, and failure modes using hierarchical categorization.
Each report organizes traces into top-level categories and subcategories,
with an executive summary of key findings and highlighted traces.

Examples:
  langsmith insights list --project my-app
  langsmith insights get INSIGHT_ID --project my-app
  langsmith insights get INSIGHT_ID --project my-app --format pretty`,
	}

	cmd.AddCommand(newInsightsListCmd())
	cmd.AddCommand(newInsightsGetCmd())
	return cmd
}

func newInsightsListCmd() *cobra.Command {
	var (
		project    string
		limit      int
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List insight reports for a project",
		Long: `List all insight reports for a project.

Returns summary information for each report including name, status,
and category distribution. Use 'insights get' with the report ID
for full details including the executive summary and category breakdown.`,
		Example: `  langsmith insights list --project my-app
  langsmith insights list --project my-app --limit 5
  langsmith insights list --project my-app --format pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				ExitErrorf("%v", err)
			}

			var jobs []langsmith.SessionInsightListResponse
			pager := c.SDK.Sessions.Insights.ListAutoPaging(ctx, sessionID, langsmith.SessionInsightListParams{})
			for pager.Next() {
				jobs = append(jobs, pager.Current())
				if limit > 0 && len(jobs) >= limit {
					break
				}
			}
			if err := pager.Err(); err != nil {
				ExitErrorf("listing insights: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				columns := []string{"Name", "ID", "Status", "Created", "Clusters"}
				var rows [][]string
				for _, job := range jobs {
					rows = append(rows, []string{
						job.Name,
						job.ID,
						job.Status,
						formatInsightTime(job.CreatedAt),
						formatShape(job.Shape),
					})
				}
				output.OutputTable(columns, rows, "Insight Reports")
			} else {
				var data []map[string]any
				for _, job := range jobs {
					data = append(data, insightJobToMap(job))
				}
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of reports to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newInsightsGetCmd() *cobra.Command {
	var (
		project    string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "get ID",
		Short: "Get a detailed insight report including clusters and analysis",
		Long: `Get full details for a specific insight report.

Returns the executive summary (key findings and highlighted traces),
plus a breakdown of all categories and subcategories with their
statistics (error rates, latency, costs, token usage, feedback scores).`,
		Example: `  langsmith insights get e4040294-44af-4866-b1dd-3c566a8d42f0 --project my-app
  langsmith insights get e4040294-44af-4866-b1dd-3c566a8d42f0 --project my-app --format pretty`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			insightID := args[0]
			c := MustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				ExitErrorf("%v", err)
			}

			detail, err := c.SDK.Sessions.Insights.GetJob(ctx, sessionID, insightID)
			if err != nil {
				ExitErrorf("fetching insight: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				printInsightPretty(detail)
			} else {
				data := buildInsightDetailJSON(detail)
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

// --- Helpers ---

func insightJobToMap(job langsmith.SessionInsightListResponse) map[string]any {
	m := map[string]any{
		"id":         job.ID,
		"name":       job.Name,
		"status":     job.Status,
		"created_at": formatTimeISO(job.CreatedAt),
		"start_time": formatTimeISO(job.StartTime),
		"end_time":   formatTimeISO(job.EndTime),
		"shape":      job.Shape,
	}
	if job.JSON.Error.IsNull() {
		m["error"] = nil
	} else {
		m["error"] = job.Error
	}
	return m
}

func buildInsightDetailJSON(d *langsmith.SessionInsightGetJobResponse) map[string]any {
	data := map[string]any{
		"id":         d.ID,
		"name":       d.Name,
		"status":     d.Status,
		"created_at": formatTimeISO(d.CreatedAt),
		"start_time": formatTimeISO(d.StartTime),
		"end_time":   formatTimeISO(d.EndTime),
		"shape":      d.Shape,
		"config_id":  nilStr(d.ConfigID),
		"metadata":   d.Metadata,
	}
	if d.JSON.Error.IsNull() {
		data["error"] = nil
	} else {
		data["error"] = d.Error
	}

	var clusterData []map[string]any
	for _, cl := range d.Clusters {
		cm := map[string]any{
			"id":          cl.ID,
			"level":       cl.Level,
			"name":        cl.Name,
			"description": cl.Description,
			"num_runs":    cl.NumRuns,
			"stats":       cl.Stats,
		}
		if cl.JSON.ParentID.IsNull() {
			cm["parent_id"] = nil
		} else {
			cm["parent_id"] = cl.ParentID
		}
		if cl.JSON.ParentName.IsNull() {
			cm["parent_name"] = nil
		} else {
			cm["parent_name"] = cl.ParentName
		}
		clusterData = append(clusterData, cm)
	}
	data["clusters"] = clusterData

	if d.JSON.Report.IsNull() {
		data["report"] = nil
	} else {
		var traces []map[string]any
		for _, ht := range d.Report.HighlightedTraces {
			traces = append(traces, map[string]any{
				"run_id":           ht.RunID,
				"cluster_id":       ht.ClusterID,
				"cluster_name":     ht.ClusterName,
				"rank":             ht.Rank,
				"highlight_reason": ht.HighlightReason,
				"summary":          ht.Summary,
			})
		}
		data["report"] = map[string]any{
			"title":              d.Report.Title,
			"key_points":         d.Report.KeyPoints,
			"highlighted_traces": traces,
			"created_at":         formatTimeISO(d.Report.CreatedAt),
		}
	}

	return data
}

func printInsightPretty(d *langsmith.SessionInsightGetJobResponse) {
	// Header
	if !d.JSON.Report.IsNull() && d.Report.Title != "" {
		fmt.Println(d.Report.Title)
		fmt.Println(strings.Repeat("=", len(d.Report.Title)))
	} else {
		title := fmt.Sprintf("Insight: %s", d.Name)
		fmt.Println(title)
		fmt.Println(strings.Repeat("=", len(title)))
	}
	fmt.Printf("ID: %s  Status: %s  Created: %s\n\n", d.ID, d.Status, formatInsightTime(d.CreatedAt))

	// Key points
	if !d.JSON.Report.IsNull() && len(d.Report.KeyPoints) > 0 {
		fmt.Println("Key Points")
		fmt.Println(strings.Repeat("-", 10))
		for i, kp := range d.Report.KeyPoints {
			fmt.Printf("  %d. %s\n", i+1, kp)
		}
		fmt.Println()
	}

	// Highlighted traces
	if !d.JSON.Report.IsNull() && len(d.Report.HighlightedTraces) > 0 {
		fmt.Println("Highlighted Traces")
		fmt.Println(strings.Repeat("-", 18))
		for _, ht := range d.Report.HighlightedTraces {
			fmt.Printf("  #%d %s\n", ht.Rank, ht.Summary)
			fmt.Printf("     Reason: %s\n", ht.HighlightReason)
			fmt.Printf("     Run: %s\n", ht.RunID)
		}
		fmt.Println()
	}

	// Cluster table
	if len(d.Clusters) > 0 {
		columns := []string{"Name", "Level", "Runs", "Error Rate", "Latency p50", "Cost p50"}
		var rows [][]string
		for _, cl := range d.Clusters {
			levelStr := "category"
			if cl.Level == 0 {
				levelStr = "subcategory"
			}

			errRate := "0.0%"
			latency := "N/A"
			cost := "N/A"

			if stats := cl.Stats; stats != nil {
				if v, ok := stats["error_rate"].(float64); ok && v > 0 {
					errRate = fmt.Sprintf("%.1f%%", v*100)
				}
				if v, ok := stats["latency_p50"].(float64); ok && v > 0 {
					latency = formatTimedelta(v)
				}
				if v, ok := stats["cost_p50"].(float64); ok && v > 0 {
					cost = fmt.Sprintf("$%.4f", v)
				}
			}

			rows = append(rows, []string{
				cl.Name,
				levelStr,
				fmt.Sprintf("%d", cl.NumRuns),
				errRate,
				latency,
				cost,
			})
		}
		output.OutputTable(columns, rows, "Categories")
	}
}

func formatInsightTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

func formatShape(shape map[string]int64) string {
	if len(shape) == 0 {
		return "N/A"
	}
	var parts []string
	for k, v := range shape {
		parts = append(parts, fmt.Sprintf("%s:%d", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
