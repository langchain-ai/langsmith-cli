package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/client"
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
	cmd.AddCommand(newTraceMessagesCmd())
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
		outDir          string
		includeFlagged  bool
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

			c := MustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				ExitError("--project is required for trace list (or set LANGSMITH_PROJECT)")
			}

			if outDir != "" {
				runTraceListOutDir(ctx, c, projectName, &ff, outDir, includeFlagged)
				return
			}

			defaultLimit := 20
			if ff.Limit == 0 {
				ff.Limit = defaultLimit
			}

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}
			runs, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				if showHierarchy {
					for _, run := range runs {
						allRuns, err := queryRuns(ctx, c, langsmith.RunQueryParams{
							Trace: langsmith.F(run.TraceID),
							Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
						}, projectName, 1000, 0)
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
					childParams := langsmith.RunQueryParams{
						Order: langsmith.F(langsmith.RunQueryParamsOrderAsc),
					}
					if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
						childParams.Select = langsmith.F(sel)
					}
					var result []map[string]any
					for _, run := range runs {
						childParams.Trace = langsmith.F(run.TraceID)
						allRuns, err := queryRuns(ctx, c, childParams, projectName, 1000, 0)
						if err != nil {
							ExitErrorf("%v", err)
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
	cmd.Flags().StringVar(&outDir, "out-dir", "", "Write trace IDs to plain-text files in this directory. Paginates exhaustively over the time window (--limit is ignored). Writes trace_ids.txt (one ID per line).")
	cmd.Flags().BoolVar(&includeFlagged, "include-flagged", false, "Also fetch user-flagged traces from /api/v1/feedback, union them into trace_ids.txt, and write flagged.tsv (<trace_id>\\t<comment> per line). Requires --out-dir.")

	return cmd
}

// runTraceListOutDir exhaustively paginates root runs in the window, unions
// in any user-flagged traces if requested, and writes plain-text ID lists
// suitable for feeding into `trace messages --from-list`.
func runTraceListOutDir(
	ctx context.Context,
	c *client.Client,
	projectName string,
	ff *FilterFlags,
	outDir string,
	includeFlagged bool,
) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		ExitErrorf("creating --out-dir: %v", err)
	}

	// 1. Exhaustive scan of root runs matching the filters.
	params := BuildRunQueryParams(ff, true, 0)
	// Only select trace_id — we don't need anything else for the ID list.
	params.Select = langsmith.F([]langsmith.RunQueryParamsSelect{
		langsmith.RunQueryParamsSelectID,
		langsmith.RunQueryParamsSelectTraceID,
	})

	// queryRuns uses the `limit` arg as an upper bound on total results;
	// pass math.MaxInt32 to paginate through everything that matches.
	const unbounded = 1 << 30
	runs, err := queryRuns(ctx, c, params, projectName, unbounded, ff.MinTokens)
	if err != nil {
		ExitErrorf("%v", err)
	}

	ids := make([]string, 0, len(runs))
	seen := make(map[string]bool, len(runs))
	for _, run := range runs {
		tid := run.TraceID
		if tid == "" {
			tid = run.ID
		}
		if tid == "" || seen[tid] {
			continue
		}
		seen[tid] = true
		ids = append(ids, tid)
	}

	// 2. Pull flagged traces and union any missing IDs.
	var flagged []FlaggedTrace
	if includeFlagged {
		sessionID, err := c.ResolveSessionID(ctx, projectName)
		if err != nil {
			ExitErrorf("resolving project: %v", err)
		}
		flagged = FetchFlaggedTraces(ctx, c, sessionID, ff.LastNMinutes)
		for _, ft := range flagged {
			if !seen[ft.TraceID] {
				seen[ft.TraceID] = true
				ids = append(ids, ft.TraceID)
			}
		}
	}

	// 3. Write trace_ids.txt (one ID per line).
	idsPath := filepath.Join(outDir, "trace_ids.txt")
	if err := os.WriteFile(idsPath, []byte(strings.Join(ids, "\n")+"\n"), 0o644); err != nil {
		ExitErrorf("writing %s: %v", idsPath, err)
	}

	// 4. Write flagged.tsv if we fetched flagged traces.
	flaggedPath := ""
	if includeFlagged {
		flaggedPath = filepath.Join(outDir, "flagged.tsv")
		var b strings.Builder
		for _, ft := range flagged {
			// Replace tabs/newlines in comments so the TSV stays parseable.
			comment := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(ft.Comment)
			fmt.Fprintf(&b, "%s\t%s\n", ft.TraceID, comment)
		}
		if err := os.WriteFile(flaggedPath, []byte(b.String()), 0o644); err != nil {
			ExitErrorf("writing %s: %v", flaggedPath, err)
		}
	}

	summary := map[string]any{
		"status":         "written",
		"traces_written": len(ids),
		"flagged_count":  len(flagged),
		"out_dir":        outDir,
		"trace_ids_path": idsPath,
	}
	if flaggedPath != "" {
		summary["flagged_path"] = flaggedPath
	}
	output.OutputJSON(summary, "")
}

func newTraceGetCmd() *cobra.Command {
	var (
		project         string
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
			projectName := ResolveProject(project)
			if projectName == "" {
				ExitError("--project is required for trace get (or set LANGSMITH_PROJECT)")
			}

			params := langsmith.RunQueryParams{
				Trace:     langsmith.F(traceID),
				StartTime: langsmith.F(resolveStartTime(since, lastNMinutes)),
				Order:     langsmith.F(langsmith.RunQueryParamsOrderAsc),
			}
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}

			runs, err := queryRuns(ctx, c, params, projectName, 1000, 0)
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
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().StringVar(&since, "since", "", "Only include runs after this timestamp, e.g. 2024-01-15T00:00:00Z (overrides 7-day default)")
	cmd.Flags().IntVar(&lastNMinutes, "last-n-minutes", 0, "Only include runs from the last N minutes, e.g. 60 (overrides 7-day default)")
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
				ExitErrorf("creating output directory: %v", err)
			}

			c := MustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				ExitError("--project is required for trace export (or set LANGSMITH_PROJECT)")
			}

			params := BuildRunQueryParams(&ff, true, ff.Limit)
			sel := buildRunSelect(includeIO, includeFeedback)
			if sel != nil {
				params.Select = langsmith.F(sel)
			}
			rootRuns, err := queryRuns(ctx, c, params, projectName, ff.Limit, ff.MinTokens)
			if err != nil {
				ExitErrorf("%v", err)
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
				allRuns, err := queryRuns(ctx, c, childParams, projectName, 1000, 0)
				if err != nil {
					ExitErrorf("%v", err)
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
