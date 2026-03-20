package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// --- Response structs for the insights API ---

type insightsListResponse struct {
	ClusteringJobs []insightJob `json:"clustering_jobs"`
}

type insightJob struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	StartTime string         `json:"start_time"`
	EndTime   string         `json:"end_time"`
	CreatedAt string         `json:"created_at"`
	Error     *string        `json:"error"`
	ConfigID  string         `json:"config_id"`
	Shape     map[string]any `json:"shape"`
	Metadata  map[string]any `json:"metadata"`
}

type insightDetail struct {
	insightJob
	Clusters []insightCluster `json:"clusters"`
	Report   *insightReport   `json:"report"`
}

type insightCluster struct {
	ID          string         `json:"id"`
	ParentID    *string        `json:"parent_id"`
	Level       int            `json:"level"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ParentName  *string        `json:"parent_name"`
	NumRuns     int            `json:"num_runs"`
	Stats       map[string]any `json:"stats"`
}

type insightReport struct {
	Title             string             `json:"title"`
	KeyPoints         []string           `json:"key_points"`
	HighlightedTraces []highlightedTrace `json:"highlighted_traces"`
	CreatedAt         string             `json:"created_at"`
}

type highlightedTrace struct {
	RunID           string `json:"run_id"`
	ClusterID       string `json:"cluster_id"`
	ClusterName     string `json:"cluster_name"`
	Rank            int    `json:"rank"`
	HighlightReason string `json:"highlight_reason"`
	Summary         string `json:"summary"`
}

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
			c := mustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				exitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("%v", err)
			}

			var result insightsListResponse
			path := fmt.Sprintf("/api/v1/sessions/%s/insights", sessionID)
			if err := c.RawGet(ctx, path, &result); err != nil {
				exitErrorf("listing insights: %v", err)
			}

			jobs := result.ClusteringJobs
			if limit > 0 && len(jobs) > limit {
				jobs = jobs[:limit]
			}

			fmt_ := getFormat()

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
				output.OutputJSON(data, outputFile)
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
			c := mustGetClient()
			ctx := context.Background()

			projectName := ResolveProject(project)
			if projectName == "" {
				exitError("--project is required (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("%v", err)
			}

			var detail insightDetail
			path := fmt.Sprintf("/api/v1/sessions/%s/insights/%s", sessionID, insightID)
			if err := c.RawGet(ctx, path, &detail); err != nil {
				exitErrorf("fetching insight: %v", err)
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				printInsightPretty(detail)
			} else {
				data := buildInsightDetailJSON(detail)
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

// --- Helpers ---

func insightJobToMap(job insightJob) map[string]any {
	m := map[string]any{
		"id":         job.ID,
		"name":       job.Name,
		"status":     job.Status,
		"created_at": nilStr(job.CreatedAt),
		"start_time": nilStr(job.StartTime),
		"end_time":   nilStr(job.EndTime),
		"shape":      job.Shape,
	}
	if job.Error != nil {
		m["error"] = *job.Error
	} else {
		m["error"] = nil
	}
	return m
}

func buildInsightDetailJSON(d insightDetail) map[string]any {
	data := insightJobToMap(d.insightJob)
	data["config_id"] = nilStr(d.ConfigID)
	data["metadata"] = d.Metadata

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
		if cl.ParentID != nil {
			cm["parent_id"] = *cl.ParentID
		} else {
			cm["parent_id"] = nil
		}
		if cl.ParentName != nil {
			cm["parent_name"] = *cl.ParentName
		} else {
			cm["parent_name"] = nil
		}
		clusterData = append(clusterData, cm)
	}
	data["clusters"] = clusterData

	if d.Report != nil {
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
			"created_at":         nilStr(d.Report.CreatedAt),
		}
	} else {
		data["report"] = nil
	}

	return data
}

func printInsightPretty(d insightDetail) {
	// Header
	if d.Report != nil && d.Report.Title != "" {
		fmt.Println(d.Report.Title)
		fmt.Println(strings.Repeat("=", len(d.Report.Title)))
	} else {
		title := fmt.Sprintf("Insight: %s", d.Name)
		fmt.Println(title)
		fmt.Println(strings.Repeat("=", len(title)))
	}
	fmt.Printf("ID: %s  Status: %s  Created: %s\n\n", d.ID, d.Status, formatInsightTime(d.CreatedAt))

	// Key points
	if d.Report != nil && len(d.Report.KeyPoints) > 0 {
		fmt.Println("Key Points")
		fmt.Println(strings.Repeat("-", 10))
		for i, kp := range d.Report.KeyPoints {
			fmt.Printf("  %d. %s\n", i+1, kp)
		}
		fmt.Println()
	}

	// Highlighted traces
	if d.Report != nil && len(d.Report.HighlightedTraces) > 0 {
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

func formatInsightTime(ts string) string {
	if ts == "" {
		return "N/A"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.999999", ts)
		if err != nil {
			if len(ts) > 16 {
				return ts[:16]
			}
			return ts
		}
	}
	return t.Format("2006-01-02 15:04")
}

func formatShape(shape map[string]any) string {
	if len(shape) == 0 {
		return "N/A"
	}
	var parts []string
	for k, v := range shape {
		parts = append(parts, fmt.Sprintf("%s:%v", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
