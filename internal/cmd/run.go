package cmd

import (
	"context"
	"os"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
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

Results are sorted oldest-first by start time.

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
		Short: "List runs matching filter criteria (any run type at any depth)",
		Run: func(cmd *cobra.Command, args []string) {
			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			if ff.Limit == 0 {
				ff.Limit = 50
			}

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)

			params := BuildRunQueryParams(&ff, false, ff.Limit)
			if includeFeedback {
				params.Select = langsmith.F([]langsmith.RunQueryParamsSelect{langsmith.RunQueryParamsSelectFeedbackStats})
			}
			runs, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				exitErrorf("%v", err)
			}

			fmt_ := getFormat()

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
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newRunGetCmd() *cobra.Command {
	var (
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

			c := mustGetClient()
			ctx := context.Background()

			// Query for the specific run by ID
			params := langsmith.RunQueryParams{
				ID:    langsmith.F([]string{runID}),
				Limit: langsmith.F(int64(1)),
			}
			if includeFeedback {
				params.Select = langsmith.F([]langsmith.RunQueryParamsSelect{langsmith.RunQueryParamsSelectFeedbackStats})
			}

			resp, err := c.SDK.Runs.Query(ctx, params)
			if err != nil {
				exitErrorf("fetching run: %v", err)
			}
			if len(resp.Runs) == 0 {
				exitErrorf("run not found: %s", runID)
			}

			data := extract.ExtractRun(resp.Runs[0], includeMetadata, includeIO, includeFeedback)
			fmt_ := getFormat()

			if fmt_ == "pretty" {
				output.PrintOutput(data, "pretty", outputFile)
			} else {
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags")
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

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)

			params := BuildRunQueryParams(&ff, false, ff.Limit)
			if includeFeedback {
				params.Select = langsmith.F([]langsmith.RunQueryParamsSelect{langsmith.RunQueryParamsSelectFeedbackStats})
			}
			runs, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				exitErrorf("%v", err)
			}

			data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
			output.OutputJSONL(data, outputFile)
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")

	return cmd
}
