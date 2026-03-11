package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Query and export traces (top-level agent runs and their full hierarchy)",
		Long: `Query and export traces (top-level agent runs and their full hierarchy).

A trace is a tree of runs representing one end-to-end invocation of your
application. The root run is the top-level entry point; child runs are
LLM calls, tool calls, retriever steps, etc.

Results are sorted newest-first by start time.

Examples:
  langsmith trace list --project my-app --limit 5
  langsmith trace get <trace-id> --project my-app --full
  langsmith trace export ./traces --project my-app --limit 20 --full`,
	}

	cmd.AddCommand(newTraceListCmd())
	cmd.AddCommand(newTraceGetCmd())
	cmd.AddCommand(newTraceExportCmd())
	return cmd
}

func newTraceListCmd() *cobra.Command {
	var (
		ff              FilterFlags
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
		showHierarchy   bool
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List traces (root runs) matching filter criteria",
		Run: func(cmd *cobra.Command, args []string) {
			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			defaultLimit := 20
			if ff.Limit == 0 {
				ff.Limit = defaultLimit
			}

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				exitErrorf("%v", err)
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				if showHierarchy {
					for _, run := range runs {
						allRuns, err := queryRuns(ctx, c, langsmith.RunQueryParams{
							Trace: langsmith.F(run.TraceID),
						}, projectName, 1000, 0)
						if err != nil {
							exitErrorf("%v", err)
						}
						output.OutputTree(runsToTreeData(allRuns), "")
					}
				} else {
					data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
					output.PrintRunsTable(os.Stdout, data, includeMetadata, "Traces")
				}
			} else {
				if showHierarchy {
					childParams := langsmith.RunQueryParams{}
					if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
						childParams.Select = langsmith.F(sel)
					}
					var result []map[string]any
					for _, run := range runs {
						childParams.Trace = langsmith.F(run.TraceID)
						allRuns, err := queryRuns(ctx, c, childParams, projectName, 1000, 0)
						if err != nil {
							exitErrorf("%v", err)
						}
						result = append(result, map[string]any{
							"trace_id":  run.TraceID,
							"run_count": len(allRuns),
							"runs":      extractRunsToMaps(allRuns, includeMetadata, includeIO, includeFeedback),
						})
					}
					output.OutputJSON(result, outputFile)
				} else {
					data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
					output.OutputJSON(data, outputFile)
				}
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, false)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().BoolVar(&showHierarchy, "show-hierarchy", false, "Fetch the full run tree for each trace")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newTraceGetCmd() *cobra.Command {
	var (
		project         string
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "get TRACE_ID",
		Short: "Fetch every run in a single trace",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			traceID := args[0]

			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(project)

			params := langsmith.RunQueryParams{
				Trace: langsmith.F(traceID),
			}
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}

			runs, err := queryRuns(ctx, c, params, projectName, 1000, 0)
			if err != nil {
				exitErrorf("%v", err)
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				output.OutputTree(runsToTreeData(runs), "")
			} else {
				data := map[string]any{
					"trace_id":  traceID,
					"run_count": len(runs),
					"runs":      extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback),
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newTraceExportCmd() *cobra.Command {
	var (
		ff              FilterFlags
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		full            bool
		filenamePattern string
	)

	cmd := &cobra.Command{
		Use:   "export OUTPUT_DIR",
		Short: "Export traces to a directory as JSONL files (one file per trace)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			outputDir := args[0]

			if full {
				includeMetadata = true
				includeIO = true
				includeFeedback = true
			}

			if ff.Limit == 0 {
				ff.Limit = 10
			}

			if err := os.MkdirAll(outputDir, 0755); err != nil {
				exitErrorf("creating output directory: %v", err)
			}

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			sel := buildRunSelect(includeIO, includeFeedback)
			if sel != nil {
				params.Select = langsmith.F(sel)
			}
			rootRuns, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				exitErrorf("%v", err)
			}

			exported := 0
			for _, root := range rootRuns {
				tid := root.TraceID

				childParams := langsmith.RunQueryParams{
					Trace: langsmith.F(tid),
				}
				if sel != nil {
					childParams.Select = langsmith.F(sel)
				}
				allRuns, err := queryRuns(ctx, c, childParams, projectName, 1000, 0)
				if err != nil {
					exitErrorf("%v", err)
				}

				name := root.Name
				if name == "" {
					name = "unknown"
				}

				filename := filenamePattern
				filename = strings.ReplaceAll(filename, "{trace_id}", tid)
				filename = strings.ReplaceAll(filename, "{name}", name)
				filename = filepath.Base(filename)
				fpath := filepath.Join(outputDir, filename)

				f, err := os.Create(fpath)
				if err != nil {
					exitErrorf("creating file %s: %v", fpath, err)
				}

				for _, run := range allRuns {
					data := extractRunsToMaps([]langsmith.RunQueryResponseRun{run}, includeMetadata, includeIO, includeFeedback)
					line, _ := json.Marshal(data[0])
					_, _ = f.Write(line)
					_, _ = f.WriteString("\n")
				}
				f.Close()
				exported++
			}

			output.OutputJSON(map[string]any{
				"status":     "exported",
				"count":      exported,
				"output_dir": outputDir,
			}, "")
		},
	}

	addCommonFilterFlags(cmd, &ff, false)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVar(&filenamePattern, "filename-pattern", "{trace_id}.jsonl",
		"Filename pattern. Supports {trace_id} and {name} placeholders.")

	return cmd
}
