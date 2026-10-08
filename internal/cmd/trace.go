package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
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
	cmd.AddCommand(newTraceMessagesCmd())
	cmd.AddCommand(newTraceStatsCmd())
	cmd.AddCommand(newTraceSetupCmd())
	return cmd
}

func newTraceListCmd() *cobra.Command {
	var (
		ff              FilterFlags
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
		includeFlagged  bool
		full            bool
		showHierarchy   bool
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List traces (root runs) matching filter criteria (default: 20, newest first)",
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

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "trace list")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
			}

			var flaggedByTrace map[string]string
			if includeFlagged {
				flagged := FetchFlaggedTraces(ctx, c, sessionID, ff.LastNMinutes)
				flaggedByTrace = make(map[string]string, len(flagged))
				for _, ft := range flagged {
					flaggedByTrace[ft.TraceID] = ft.Comment
				}
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				if showHierarchy {
					for _, run := range runs {
						childParams := langsmith.RunQueryParams{
							Trace: langsmith.F(run.TraceID),
							Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
						}
						// Bound v2's min_start_time to the root's start so older
						// traces aren't clipped by v2's default 1-day window.
						if !run.StartTime.IsZero() {
							childParams.StartTime = langsmith.F(run.StartTime)
						}
						allRuns, err := queryRunsAuto(ctx, c, childParams, buildRunSelectV2(includeIO, includeFeedback), sessionID, 1000, 0)
						if err != nil {
							ExitErrorf("%v", err)
						}
						output.OutputTree(runsToTreeData(allRuns), "")
					}
				} else {
					data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
					output.PrintRunsTable(os.Stdout, data, includeMetadata, "Traces")
				}
			} else {
				if showHierarchy {
					baseSelect := buildRunSelect(includeIO, includeFeedback)
					v2Select := buildRunSelectV2(includeIO, includeFeedback)
					var result []map[string]any
					for _, run := range runs {
						childParams := langsmith.RunQueryParams{
							Trace: langsmith.F(run.TraceID),
							Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
						}
						if baseSelect != nil {
							childParams.Select = langsmith.F(baseSelect)
						}
						if !run.StartTime.IsZero() {
							childParams.StartTime = langsmith.F(run.StartTime)
						}
						allRuns, err := queryRunsAuto(ctx, c, childParams, v2Select, sessionID, 1000, 0)
						if err != nil {
							ExitErrorf("%v", err)
						}
						result = append(result, map[string]any{
							"trace_id":  run.TraceID,
							"run_count": len(allRuns),
							"runs":      extractRunsToMaps(allRuns, includeMetadata, includeIO, includeFeedback),
						})
					}
					if err := output.OutputJSON(result, outputFile); err != nil {
						ExitErrorf("%v", err)
					}
				} else {
					data := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)
					if flaggedByTrace != nil {
						annotateFlagged(data, flaggedByTrace)
					}
					if err := output.OutputJSON(data, outputFile); err != nil {
						ExitErrorf("%v", err)
					}
				}
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, false)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&includeFlagged, "include-flagged", false, "Add flagged_comment field populated from user-flagged trace feedback")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().BoolVar(&showHierarchy, "show-hierarchy", false, "Fetch the full run tree for each trace")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newTraceGetCmd() *cobra.Command {
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

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, project, projectID, "trace get")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := langsmith.RunQueryParams{
				Trace:     langsmith.F(traceID),
				StartTime: langsmith.F(resolveStartTime(since, lastNMinutes)),
				Order:     langsmith.F(langsmith.RunQueryParamsOrderAsc),
			}
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}

			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, 1000, 0)
			if err != nil {
				ExitErrorf("%v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				output.OutputTree(runsToTreeData(runs), "")
			} else {
				data := map[string]any{
					"trace_id":  traceID,
					"run_count": len(runs),
					"runs":      extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback),
				}
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
				ExitErrorf("creating output directory: %v", err)
			}

			c := MustGetClient()
			ctx := context.Background()
			sessionID, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "trace export")
			if err != nil {
				ExitErrorf("%v", err)
			}

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			sel := buildRunSelect(includeIO, includeFeedback)
			if sel != nil {
				params.Select = langsmith.F(sel)
			}
			v2Select := buildRunSelectV2(includeIO, includeFeedback)
			rootRuns, err := queryRunsAuto(ctx, c, params, v2Select, sessionID, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
			}

			// Resolve every destination before creating any files. A pattern such as
			// "trace.jsonl" or {name} can map multiple traces to one path; allowing
			// os.Create to handle that would silently truncate an earlier export.
			paths := make(map[string]string, len(rootRuns))
			for _, root := range rootRuns {
				fpath := traceExportPath(outputDir, filenamePattern, root)
				if previous, ok := paths[fpath]; ok {
					ExitErrorf("filename pattern resolves multiple traces to %s (trace IDs %s and %s); include {trace_id} or another unique placeholder", fpath, previous, root.TraceID)
				}
				paths[fpath] = root.TraceID
			}

			exported := 0
			for _, root := range rootRuns {
				tid := root.TraceID

				childParams := langsmith.RunQueryParams{
					Trace: langsmith.F(tid),
					Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
				}
				if sel != nil {
					childParams.Select = langsmith.F(sel)
				}
				if !root.StartTime.IsZero() {
					childParams.StartTime = langsmith.F(root.StartTime)
				}
				allRuns, err := queryRunsAuto(ctx, c, childParams, v2Select, sessionID, 1000, 0)
				if err != nil {
					ExitErrorf("%v", err)
				}

				fpath := traceExportPath(outputDir, filenamePattern, root)

				f, err := os.Create(fpath)
				if err != nil {
					ExitErrorf("creating file %s: %v", fpath, err)
				}

				for _, run := range allRuns {
					data := extractRunsToMaps([]langsmith.RunSchema{run}, includeMetadata, includeIO, includeFeedback)
					line, _ := json.Marshal(data[0])
					_, _ = f.Write(line)
					_, _ = f.WriteString("\n")
				}
				f.Close()
				exported++
			}
			if err := output.OutputJSON(map[string]any{
				"status":     "exported",
				"count":      exported,
				"output_dir": outputDir,
			}, ""); err != nil {
				ExitErrorf("%v", err)
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, false)
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().StringVar(&filenamePattern, "filename-pattern", "{trace_id}.jsonl",
		"Filename pattern. Supports {trace_id} and {name} placeholders.")

	return cmd
}

func traceExportPath(outputDir, pattern string, root langsmith.RunSchema) string {
	name := root.Name
	if name == "" {
		name = "unknown"
	}
	filename := strings.ReplaceAll(pattern, "{trace_id}", root.TraceID)
	filename = strings.ReplaceAll(filename, "{name}", name)
	return filepath.Join(outputDir, filepath.Base(filename))
}

// annotateFlagged mutates each run map to add a "flagged_comment" field
// (string or null) based on whether the trace_id has a user-flagged feedback
// entry. Runs without a match get "flagged_comment": null so downstream jq
// filters can distinguish "missing" from "not flagged".
func annotateFlagged(data []map[string]any, flaggedByTrace map[string]string) {
	for _, row := range data {
		tid, _ := row["trace_id"].(string)
		if comment, ok := flaggedByTrace[tid]; ok {
			if comment == "" {
				comment = "User flagged for review"
			}
			row["flagged_comment"] = comment
		} else {
			row["flagged_comment"] = nil
		}
	}
}
