package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
  langsmith insights create --project my-app --name "Reliability review" --summary-prompt 'Summarize {{run.inputs}} and {{run.outputs}}'
  langsmith insights get INSIGHT_ID --project my-app
  langsmith insights get INSIGHT_ID --project my-app --format pretty`,
	}

	cmd.AddCommand(newInsightsCreateCmd())
	cmd.AddCommand(newInsightsListCmd())
	cmd.AddCommand(newInsightsGetCmd())
	return cmd
}

type insightCreateOptions struct {
	project           string
	projectID         string
	name              string
	since             string
	before            string
	lastNHours        int64
	filter            string
	sample            float64
	model             string
	clusterModel      string
	summaryModel      string
	summaryPrompt     string
	summaryPromptFile string
	partitions        string
	attributeSchemas  string
	outputFile        string
	sampleSet         bool
	lastNHoursSet     bool
}

func newInsightsCreateCmd() *cobra.Command {
	var opts insightCreateOptions

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an insight report for a project",
		Long: `Create an insight report using a manually specified configuration.

The summary prompt controls how each trace is summarized before patterns are
identified. It is a Mustache template and must reference at least one supported
trace field, such as {{run.inputs}}, {{run.outputs}}, {{run.error}},
{{run.feedback}}, or {{all_thread_messages}}.

Report generation runs asynchronously and may incur model costs. The workspace
must have secrets configured for the selected models.`,
		Example: `  langsmith insights create --project my-app \
    --name "Weekly reliability review" \
    --summary-prompt 'Identify the request and outcome: {{run.inputs}} {{run.outputs}}'

  langsmith insights create --project my-app \
    --name "Weekly reliability review" \
    --summary-prompt-file ./summary-prompt.txt \
    --since 2026-09-01 --before 2026-09-08 \
    --filter 'eq(is_root, true)'

  cat summary-prompt.txt | langsmith insights create --project-id PROJECT_ID \
    --name "Weekly reliability review" \
    --summary-prompt-file - --format json`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.sampleSet = cmd.Flags().Changed("sample")
			opts.lastNHoursSet = cmd.Flags().Changed("last-n-hours")
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = strings.TrimSpace(opts.name)
			if opts.name == "" {
				return fmt.Errorf("--name must not be empty")
			}
			request, err := buildInsightCreateRequest(opts)
			if err != nil {
				return err
			}

			c, err := getClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			sessionID, err := resolveSessionID(ctx, c, opts.project, opts.projectID, "insights create")
			if err != nil {
				return err
			}

			config, err := c.SDK.Sessions.Insights.Configs.New(ctx, sessionID, langsmith.SessionInsightConfigNewParams{
				Name:   langsmith.F(opts.name),
				Config: langsmith.F(request),
			})
			if err != nil {
				return fmt.Errorf("creating insight config: %w", err)
			}

			created, err := c.SDK.Sessions.Insights.New(ctx, sessionID, langsmith.SessionInsightNewParams{
				CreateRunClusteringJobRequest: langsmith.CreateRunClusteringJobRequestParam{
					ConfigID: langsmith.F(config.ID),
				},
			})
			if err != nil {
				return fmt.Errorf("insight config %s was saved, but starting the report failed: %w", config.ID, err)
			}

			if GetFormat() == "pretty" {
				printInsightCreatePretty(created, sessionID, config.ID)
				return nil
			}
			if err := output.OutputJSON(insightCreateResponseToMap(created, sessionID, config.ID), opts.outputFile); err != nil {
				return err
			}
			return nil
		},
	}

	addProjectFlags(cmd, &opts.project, &opts.projectID)
	cmd.Flags().StringVar(&opts.name, "name", "", "Report name")
	cmd.Flags().StringVar(&opts.summaryPrompt, "summary-prompt", "", "Mustache template used to summarize each trace")
	cmd.Flags().StringVar(&opts.summaryPromptFile, "summary-prompt-file", "", `Read the summary prompt from a file (use "-" for stdin)`)
	cmd.Flags().StringVar(&opts.since, "since", "", "Start of time window (RFC3339 or YYYY-MM-DD; default: 7 days ago)")
	cmd.Flags().StringVar(&opts.before, "before", "", "End of time window (RFC3339 or YYYY-MM-DD; default: now)")
	cmd.Flags().Int64Var(&opts.lastNHours, "last-n-hours", 0, "Analyze traces from the last N hours")
	cmd.Flags().StringVar(&opts.filter, "filter", "", "LangSmith trace filter (default: root traces)")
	cmd.Flags().Float64Var(&opts.sample, "sample", 0, "Trace sample size or fraction (must be greater than 0)")
	cmd.Flags().StringVar(&opts.model, "model", "openai", "Model provider: openai or anthropic")
	cmd.Flags().StringVar(&opts.clusterModel, "cluster-model", "", "Model ID used to identify patterns; requires --summary-model")
	cmd.Flags().StringVar(&opts.summaryModel, "summary-model", "", "Model ID used to summarize traces; requires --cluster-model")
	cmd.Flags().StringVar(&opts.partitions, "partitions", "", "Partitions as a JSON object or @file.json")
	cmd.Flags().StringVar(&opts.attributeSchemas, "attribute-schemas", "", "Attribute schemas as a JSON object or @file.json")
	cmd.Flags().StringVarP(&opts.outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("name")
	cmd.MarkFlagsMutuallyExclusive("summary-prompt", "summary-prompt-file")
	cmd.MarkFlagsMutuallyExclusive("since", "last-n-hours")

	return cmd
}

func buildInsightCreateRequest(opts insightCreateOptions) (langsmith.CreateRunClusteringJobRequestParam, error) {
	var request langsmith.CreateRunClusteringJobRequestParam

	prompt, err := loadInsightSummaryPrompt(opts.summaryPrompt, opts.summaryPromptFile)
	if err != nil {
		return request, err
	}
	if strings.TrimSpace(prompt) == "" {
		return request, fmt.Errorf("one of --summary-prompt or --summary-prompt-file is required")
	}
	request.SummaryPrompt = langsmith.F(prompt)

	if opts.model != "openai" && opts.model != "anthropic" {
		return request, fmt.Errorf("invalid --model %q: must be openai or anthropic", opts.model)
	}
	request.Model = langsmith.F(langsmith.CreateRunClusteringJobRequestModel(opts.model))

	if (opts.clusterModel == "") != (opts.summaryModel == "") {
		return request, fmt.Errorf("--cluster-model and --summary-model must be specified together")
	}
	if opts.clusterModel != "" {
		request.ClusterModel = langsmith.F(opts.clusterModel)
		request.SummaryModel = langsmith.F(opts.summaryModel)
	}

	if opts.since != "" {
		start, err := parseFlexTime(opts.since)
		if err != nil {
			return request, fmt.Errorf("invalid --since timestamp %q: use RFC3339 or YYYY-MM-DD", opts.since)
		}
		request.StartTime = langsmith.F(start)
	}
	if opts.before != "" {
		if opts.since == "" {
			return request, fmt.Errorf("--before requires --since")
		}
		end, err := parseFlexTime(opts.before)
		if err != nil {
			return request, fmt.Errorf("invalid --before timestamp %q: use RFC3339 or YYYY-MM-DD", opts.before)
		}
		start := request.StartTime.Value
		if !end.After(start) {
			return request, fmt.Errorf("--before must be after --since")
		}
		request.EndTime = langsmith.F(end)
	}
	if opts.lastNHoursSet {
		if opts.lastNHours <= 0 {
			return request, fmt.Errorf("--last-n-hours must be greater than 0")
		}
		request.LastNHours = langsmith.F(opts.lastNHours)
	}
	if opts.sampleSet {
		if opts.sample <= 0 {
			return request, fmt.Errorf("--sample must be greater than 0")
		}
		request.Sample = langsmith.F(opts.sample)
	}

	if opts.name != "" {
		request.Name = langsmith.F(opts.name)
	}
	if opts.filter != "" {
		request.Filter = langsmith.F(opts.filter)
	}
	if opts.partitions != "" {
		partitions, err := parseInsightJSONObject[string]("--partitions", opts.partitions)
		if err != nil {
			return request, err
		}
		request.Partitions = langsmith.F(partitions)
	}
	if opts.attributeSchemas != "" {
		attributeSchemas, err := parseInsightJSONObject[any]("--attribute-schemas", opts.attributeSchemas)
		if err != nil {
			return request, err
		}
		request.AttributeSchemas = langsmith.F(attributeSchemas)
	}

	return request, nil
}

func loadInsightSummaryPrompt(inline, path string) (string, error) {
	if inline != "" && path != "" {
		return "", fmt.Errorf("specify only one of --summary-prompt or --summary-prompt-file")
	}
	if path == "" {
		return inline, nil
	}

	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("reading --summary-prompt-file %q: %w", path, err)
	}
	return string(data), nil
}

func parseInsightJSONObject[T any](flagName, raw string) (map[string]T, error) {
	data := []byte(raw)
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimPrefix(raw, "@")
		if path == "" {
			return nil, fmt.Errorf("%s requires a file path after @", flagName)
		}
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s file %q: %w", flagName, path, err)
		}
	}

	var value map[string]T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("parsing %s as a JSON object: %w", flagName, err)
	}
	if value == nil {
		return nil, fmt.Errorf("%s must be a JSON object", flagName)
	}
	return value, nil
}

func insightCreateResponseToMap(created *langsmith.SessionInsightNewResponse, sessionID, configID string) map[string]any {
	result := map[string]any{
		"id":         created.ID,
		"name":       created.Name,
		"status":     created.Status,
		"project_id": sessionID,
		"config_id":  configID,
	}
	if created.JSON.Error.IsNull() {
		result["error"] = nil
	} else {
		result["error"] = created.Error
	}
	return result
}

func printInsightCreatePretty(created *langsmith.SessionInsightNewResponse, sessionID, configID string) {
	fmt.Println("Insight report created")
	fmt.Println()
	fmt.Printf("Name:       %s\n", created.Name)
	fmt.Printf("ID:         %s\n", created.ID)
	fmt.Printf("Config ID:  %s\n", configID)
	fmt.Printf("Status:     %s\n", created.Status)
	fmt.Println()
	fmt.Println("View it with:")
	fmt.Printf("  langsmith insights get %s --project-id %s\n", created.ID, sessionID)
}

func newInsightsListCmd() *cobra.Command {
	var (
		project    string
		projectID  string
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

			sessionID, err := resolveSessionID(ctx, c, project, projectID, "insights list")
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

	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of reports to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newInsightsGetCmd() *cobra.Command {
	var (
		project    string
		projectID  string
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

			sessionID, err := resolveSessionID(ctx, c, project, projectID, "insights get")
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

	addProjectFlags(cmd, &project, &projectID)
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
