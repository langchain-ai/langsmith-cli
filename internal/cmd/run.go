package cmd

import (
	"context"
	"os"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/lib/runsv2"
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
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				ExitError("--project is required for run list (or set LANGSMITH_PROJECT)")
			}

			var runs []langsmith.RunSchema
			var err error
			if ff.Version == "v2" {
				body := buildRunQueryV2Params(&ff, false, ff.Limit)
				body.Selects = buildRunSelectV2(includeIO, includeFeedback)
				runs, err = queryRunsV2(ctx, c, body, projectName, ff.Limit, ff.MinTokens)
			} else {
				params := BuildRunQueryParams(&ff, false, ff.Limit)
				if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
					params.Select = langsmith.F(sel)
				}
				runs, err = queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			}
			if err != nil {
				ExitErrorf("%v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
				output.PrintRunsTable(os.Stdout, data, includeMetadata, "Runs")
			} else {
				data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
				output.OutputJSON(data, outputFile)
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	addVersionFlag(cmd, &ff)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newRunGetCmd() *cobra.Command {
	var (
		project         string
		since           string
		lastNMinutes    int
		version         string
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
			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required for run get (or set LANGSMITH_PROJECT)")
			}

			var runs []langsmith.RunSchema
			var err error
			if version == "v2" {
				minStart := resolveStartTime(since, lastNMinutes).UTC().Format(time.RFC3339)
				body := runsv2.QueryRequest{
					IDs:          []string{runID},
					MinStartTime: &minStart,
					PageSize:     runsv2.Ptr(uint64(1)),
					SortOrder:    runsv2.Ptr(runsv2.SortOrderDesc),
					Selects:      buildRunSelectV2(includeIO, includeFeedback),
				}
				runs, err = queryRunsV2(ctx, c, body, projectName, 1, 0)
			} else {
				params := langsmith.RunQueryParams{
					ID:        langsmith.F([]string{runID}),
					Limit:     langsmith.F(int64(1)),
					StartTime: langsmith.F(resolveStartTime(since, lastNMinutes)),
				}
				if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
					params.Select = langsmith.F(sel)
				}
				runs, err = queryRuns(ctx, c, params, projectName, 1, 0)
			}
			if err != nil {
				ExitErrorf("fetching run: %v", err)
			}
			if len(runs) == 0 {
				ExitErrorf("run not found: %s", runID)
			}

			data := extract.ExtractRun(runs[0], includeMetadata, includeIO, includeFeedback)
			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				output.PrintOutput(data, "pretty", outputFile)
			} else {
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&since, "since", "", "Only include runs after this timestamp, e.g. 2024-01-15T00:00:00Z (overrides 7-day default)")
	cmd.Flags().IntVar(&lastNMinutes, "last-n-minutes", 0, "Only include runs from the last N minutes, e.g. 60 (overrides 7-day default)")
	cmd.Flags().StringVar(&version, "version", "", `Query API version: "" (v1, default) or "v2" (SmithDB)`)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
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
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				ExitError("--project is required for run export (or set LANGSMITH_PROJECT)")
			}

			var runs []langsmith.RunSchema
			var err error
			if ff.Version == "v2" {
				body := buildRunQueryV2Params(&ff, false, ff.Limit)
				body.Selects = buildRunSelectV2(includeIO, includeFeedback)
				runs, err = queryRunsV2(ctx, c, body, projectName, ff.Limit, ff.MinTokens)
			} else {
				params := BuildRunQueryParams(&ff, false, ff.Limit)
				if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
					params.Select = langsmith.F(sel)
				}
				runs, err = queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			}
			if err != nil {
				ExitErrorf("%v", err)
			}

			data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
			output.OutputJSONL(data, outputFile)
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	addVersionFlag(cmd, &ff)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")

	return cmd
}
