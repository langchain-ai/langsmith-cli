package cmd

import (
	"context"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Query and export individual runs (LLM calls, tool calls, chain steps, etc.)",
		Long: `Query and export individual runs (LLM calls, tool calls, chain steps, etc.).

A run is a single step within a trace. Unlike trace commands (which
filter on root runs only), run commands can query any run at any
depth in the hierarchy.

Results are sorted newest-first by start time.

Examples:
  langsmith run list --project my-app --run-type llm --limit 10
  langsmith run get <run-id> --full
  langsmith run export runs.jsonl --project my-app --run-type llm`,
	}

	cmd.AddCommand(newRunListCmd())
	cmd.AddCommand(newRunGetCmd())
	cmd.AddCommand(newRunExportCmd())
	return cmd
}

func newRunListCmd() *cobra.Command {
	var (
		ff              FilterFlags
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List runs matching filter criteria (default: 50, newest first)",
		Run: func(cmd *cobra.Command, args []string) {
			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			if ff.Limit == 0 {
				ff.Limit = 50
			}

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "run list")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := BuildRunQueryParams(&ff, false, ff.Limit)
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
				output.PrintRunsTable(os.Stdout, data, includeMetadata, "Runs")
			} else {
				data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
				if err := output.OutputJSON(data, outputFile); err != nil {
					ExitErrorf("%v", err)
				}
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newRunGetCmd() *cobra.Command {
	var (
		project         string
		projectID       string
		since           string
		lastNMinutes    int
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "get RUN_ID",
		Short: "Fetch a single run by its run ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runID := args[0]

			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, project, projectID, "run get")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := langsmith.RunQueryParams{
				ID:        langsmith.F([]string{runID}),
				Limit:     langsmith.F(int64(1)),
				StartTime: langsmith.F(resolveStartTime(since, lastNMinutes)),
			}
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, 1, 0)
			if err != nil {
				ExitErrorf("fetching run: %v", err)
			}
			if len(runs) == 0 {
				ExitErrorf("run not found: %s", runID)
			}

			data := extract.ExtractRun(runs[0], includeMetadata, includeIO, includeFeedback)
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

	addProjectFlags(cmd, &project, &projectID)
	cmd.Flags().StringVar(&since, "since", "", "Only include runs after this timestamp, e.g. 2024-01-15T00:00:00Z (overrides 7-day default)")
	cmd.Flags().IntVar(&lastNMinutes, "last-n-minutes", 0, "Only include runs from the last N minutes, e.g. 60 (overrides 7-day default)")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newRunExportCmd() *cobra.Command {
	var (
		ff              FilterFlags
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
	)

	cmd := &cobra.Command{
		Use:   "export OUTPUT_FILE",
		Short: "Export runs to a JSONL file (one JSON object per line)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			outputFile := args[0]

			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			if ff.Limit == 0 {
				ff.Limit = 100
			}

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "run export")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := BuildRunQueryParams(&ff, false, ff.Limit)
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
			}

			data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
			if err := output.OutputJSONL(data, outputFile); err != nil {
				ExitErrorf("%v", err)
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")

	return cmd
}
