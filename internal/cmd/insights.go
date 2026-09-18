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
		Short: "Create and query insight reports for a project",
		Long: `Create and query insight reports for a project.

The Insights Agent automatically analyzes traces to detect usage patterns,
common agent behaviors, and failure modes using hierarchical categorization.
Each report organizes traces into top-level categories and subcategories,
with an executive summary of key findings and highlighted traces.

Examples:
  langsmith insights list --project my-app
  langsmith insights create --project my-app --config insights.json
  langsmith insights run CONFIG_ID --project my-app
  langsmith insights config list --project my-app
  langsmith insights get INSIGHT_ID --project my-app
  langsmith insights get INSIGHT_ID --project my-app --format pretty`,
	}

	cmd.AddCommand(newInsightsCreateCmd())
	cmd.AddCommand(newInsightsRunCmd())
	cmd.AddCommand(newInsightsListCmd())
	cmd.AddCommand(newInsightsGetCmd())
	cmd.AddCommand(newInsightsConfigCmd())
	return cmd
}

type insightCreateOptions struct {
	project    string
	projectID  string
	configFile string
	wait       bool
	outputFile string
}

// insightConfigFile is the public, JSON-serializable report definition used by
// the Insights config API. A separate input type is necessary because the
// generated SDK's param.Field wrappers are designed for marshaling requests,
// not unmarshaling user-authored JSON.
type insightConfigFile struct {
	AttributeSchemas map[string]any                                `json:"attribute_schemas"`
	ClusterModel     *string                                       `json:"cluster_model"`
	Description      nullableStringInput                           `json:"description"`
	EndTime          *time.Time                                    `json:"end_time"`
	Filter           *string                                       `json:"filter"`
	Hierarchy        []int64                                       `json:"hierarchy"`
	LastNHours       *int64                                        `json:"last_n_hours"`
	Model            *langsmith.CreateRunClusteringJobRequestModel `json:"model"`
	Name             string                                        `json:"name"`
	Partitions       map[string]string                             `json:"partitions"`
	Sample           *float64                                      `json:"sample"`
	ScheduleCron     nullableStringInput                           `json:"schedule_cron"`
	StartTime        *time.Time                                    `json:"start_time"`
	SummaryModel     *string                                       `json:"summary_model"`
	SummaryPrompt    string                                        `json:"summary_prompt"`
}

const (
	insightWaitInterval = 30 * time.Second
	insightWaitTimeout  = 30 * time.Minute
)

func newInsightsCreateCmd() *cobra.Command {
	var opts insightCreateOptions

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an insight report for a project",
		Long: `Create an insight report using a manual-mode JSON configuration.

The summary prompt controls how each trace is summarized before patterns are
identified. It is a Mustache template and must reference at least one supported
trace field, such as {{run.inputs}}, {{run.outputs}}, {{run.error}},
{{run.feedback}}, or {{all_thread_messages}}.

Auto mode and user_context are not supported. Express the report's intent
directly in summary_prompt.

When selecting models explicitly, cluster_model and summary_model must be set
together. Each value must be openai, anthropic, or the UUID of a workspace
model configuration; model display names are not accepted.

Set schedule_cron in the JSON configuration to run the report repeatedly. The
first job starts immediately, and future jobs start on the specified schedule.
Omit the field for a one-time report.

Report generation runs asynchronously and may incur model costs. The workspace
must have secrets configured for the selected models. Use --wait to poll for a
terminal status; waiting can take up to 30 minutes.`,
		Example: `  langsmith insights create --project my-app \
    --config insights.json

  langsmith insights create --project-id PROJECT_ID \
    --config insights.json --wait --format json

  cat insights.json | langsmith insights create --project my-app --config -`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configInput, err := loadInsightConfigFile(opts.configFile)
			if err != nil {
				return err
			}
			request, err := configInput.toSDKParams()
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

			configParams := langsmith.SessionInsightConfigNewParams{
				Name:   langsmith.F(strings.TrimSpace(configInput.Name)),
				Config: langsmith.F(request),
			}
			applyInsightConfigMetadataToNewParams(configInput, &configParams)
			config, err := c.SDK.Sessions.Insights.Configs.New(ctx, sessionID, configParams)
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
			if opts.wait {
				waitCtx, cancel := context.WithTimeout(ctx, insightWaitTimeout)
				defer cancel()
				completed, err := waitForInsight(waitCtx, c.SDK, sessionID, created.ID, insightWaitInterval)
				if err != nil {
					return err
				}
				created.Name = completed.Name
				created.Status = completed.Status
			}
			schedule := ""
			if !config.JSON.ScheduleCron.IsNull() {
				schedule = config.ScheduleCron
			}

			if GetFormat() == "pretty" {
				printInsightCreatePretty(created, sessionID, config.ID, opts.wait, schedule)
				return nil
			}
			result := insightCreateResponseToMap(created, sessionID, config.ID)
			if config.JSON.ScheduleCron.IsNull() {
				result["schedule_cron"] = nil
			} else {
				result["schedule_cron"] = config.ScheduleCron
			}
			if err := output.OutputJSON(result, opts.outputFile); err != nil {
				return err
			}
			return nil
		},
	}

	addProjectFlags(cmd, &opts.project, &opts.projectID)
	cmd.Flags().StringVar(&opts.configFile, "config", "", `Report definition JSON file (use "-" for stdin)`)
	cmd.Flags().BoolVar(&opts.wait, "wait", false, "Wait for the report to succeed or fail")
	cmd.Flags().StringVarP(&opts.outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("config")

	return cmd
}

func loadInsightConfigFile(path string) (insightConfigFile, error) {
	var config insightConfigFile
	if strings.TrimSpace(path) == "" {
		return config, fmt.Errorf("--config must not be empty")
	}

	var reader io.Reader
	if path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return config, fmt.Errorf("opening --config file %q: %w", path, err)
		}
		defer file.Close()
		reader = file
	}

	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("parsing --config file %q: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return config, fmt.Errorf("parsing --config file %q: multiple JSON values", path)
		}
		return config, fmt.Errorf("parsing --config file %q: %w", path, err)
	}
	return config, nil
}

func (c insightConfigFile) toSDKParams() (langsmith.CreateRunClusteringJobRequestParam, error) {
	var request langsmith.CreateRunClusteringJobRequestParam
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return request, fmt.Errorf("--config field name must not be empty")
	}
	if strings.TrimSpace(c.SummaryPrompt) == "" {
		return request, fmt.Errorf("--config field summary_prompt must not be empty")
	}
	if c.Model != nil && !c.Model.IsKnown() {
		return request, fmt.Errorf("--config field model %q must be openai or anthropic", *c.Model)
	}
	if (c.ClusterModel == nil) != (c.SummaryModel == nil) {
		return request, fmt.Errorf("--config fields cluster_model and summary_model must be specified together")
	}
	if c.EndTime != nil && c.StartTime == nil {
		return request, fmt.Errorf("--config field end_time requires start_time")
	}
	if c.StartTime != nil && c.EndTime != nil && !c.EndTime.After(*c.StartTime) {
		return request, fmt.Errorf("--config field end_time must be after start_time")
	}
	if c.LastNHours != nil {
		if *c.LastNHours <= 0 {
			return request, fmt.Errorf("--config field last_n_hours must be greater than 0")
		}
		if c.StartTime != nil || c.EndTime != nil {
			return request, fmt.Errorf("--config field last_n_hours cannot be combined with start_time or end_time")
		}
	}
	if c.Sample != nil && *c.Sample <= 0 {
		return request, fmt.Errorf("--config field sample must be greater than 0")
	}

	request.Name = langsmith.F(c.Name)
	request.SummaryPrompt = langsmith.F(c.SummaryPrompt)
	if c.Model == nil {
		request.Model = langsmith.F(langsmith.CreateRunClusteringJobRequestModelOpenAI)
	} else {
		request.Model = langsmith.F(*c.Model)
	}
	if c.AttributeSchemas != nil {
		request.AttributeSchemas = langsmith.F(c.AttributeSchemas)
	}
	if c.ClusterModel != nil {
		request.ClusterModel = langsmith.F(*c.ClusterModel)
		request.SummaryModel = langsmith.F(*c.SummaryModel)
	}
	if c.EndTime != nil {
		request.EndTime = langsmith.F(*c.EndTime)
	}
	if c.Filter != nil {
		request.Filter = langsmith.F(*c.Filter)
	}
	if c.Hierarchy != nil {
		request.Hierarchy = langsmith.F(c.Hierarchy)
	}
	if c.LastNHours != nil {
		request.LastNHours = langsmith.F(*c.LastNHours)
	}
	if c.Partitions != nil {
		request.Partitions = langsmith.F(c.Partitions)
	}
	if c.Sample != nil {
		request.Sample = langsmith.F(*c.Sample)
	}
	if c.StartTime != nil {
		request.StartTime = langsmith.F(*c.StartTime)
	}
	return request, nil
}

func waitForInsight(ctx context.Context, sdk *langsmith.Client, sessionID, insightID string, interval time.Duration) (*langsmith.SessionInsightGetJobResponse, error) {
	for {
		detail, err := sdk.Sessions.Insights.GetJob(ctx, sessionID, insightID)
		if err != nil {
			return nil, fmt.Errorf("polling insight report %s: %w", insightID, err)
		}
		switch detail.Status {
		case "success":
			return detail, nil
		case "error":
			if detail.Error == "" {
				return nil, fmt.Errorf("insight report %s failed", insightID)
			}
			return nil, fmt.Errorf("insight report %s failed: %s", insightID, detail.Error)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if ctx.Err() == context.DeadlineExceeded {
				return nil, fmt.Errorf("timed out waiting for insight report %s", insightID)
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
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

func printInsightCreatePretty(created *langsmith.SessionInsightNewResponse, sessionID, configID string, waited bool, schedule string) {
	if waited {
		fmt.Println("Insight report completed")
	} else {
		fmt.Println("Insight report created")
	}
	fmt.Println()
	fmt.Printf("Name:       %s\n", created.Name)
	fmt.Printf("ID:         %s\n", created.ID)
	fmt.Printf("Config ID:  %s\n", configID)
	fmt.Printf("Status:     %s\n", created.Status)
	if schedule != "" {
		fmt.Printf("Schedule:   %s\n", schedule)
	}
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
