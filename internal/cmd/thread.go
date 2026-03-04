package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Query multi-turn conversation threads",
		Long: `Query multi-turn conversation threads.

A thread groups multiple root runs that share a thread_id, representing
a multi-turn conversation.

Examples:
  langsmith thread list --project my-chatbot --limit 10
  langsmith thread get <thread-id> --project my-chatbot --full`,
	}

	cmd.AddCommand(newThreadListCmd())
	cmd.AddCommand(newThreadGetCmd())
	return cmd
}

func newThreadListCmd() *cobra.Command {
	var (
		project      string
		limit        int
		rawFilter    string
		lastNMinutes int
		outputFile   string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversation threads in a project",
		Run: func(cmd *cobra.Command, args []string) {
			if project == "" {
				exitError("--project is required for thread list")
			}

			c := mustGetClient()
			ctx := context.Background()

			// Resolve project to session ID
			sessionID, err := c.ResolveSessionID(ctx, project)
			if err != nil {
				exitErrorf("%v", err)
			}

			// Query root runs (like the Python SDK does)
			params := langsmith.RunQueryParams{
				Session: langsmith.F([]string{sessionID}),
				IsRoot:  langsmith.F(true),
				Limit:   langsmith.F(int64(100)),
			}

			if rawFilter != "" {
				params.Filter = langsmith.F(rawFilter)
			}

			// Default to last 24h if no time filter
			startTime := time.Now().UTC().Add(-24 * time.Hour)
			if lastNMinutes > 0 {
				startTime = time.Now().UTC().Add(-time.Duration(lastNMinutes) * time.Minute)
			}
			params.StartTime = langsmith.F(startTime)

			// Paginate all runs and group by thread_id
			threadsMap := make(map[string][]map[string]any)
			cursor := ""
			for {
				if cursor != "" {
					params.Cursor = langsmith.F(cursor)
				}
				resp, err := c.SDK.Runs.Query(ctx, params)
				if err != nil {
					exitErrorf("querying runs: %v", err)
				}
				for _, run := range resp.Runs {
					tid := run.ThreadID
					if tid != "" {
						m := extract.ExtractRun(run, true, true, false)
						threadsMap[tid] = append(threadsMap[tid], m)
					}
				}
				if resp.Cursors == nil || resp.Cursors["next"] == "" {
					break
				}
				cursor = resp.Cursors["next"]
			}

			// Build thread summaries
			type threadSummary struct {
				ThreadID     string
				RunCount     int
				MinStartTime string
				MaxStartTime string
			}

			var threads []threadSummary
			for tid, runs := range threadsMap {
				var startTimes []string
				for _, r := range runs {
					if st, ok := r["start_time"].(string); ok && st != "" {
						startTimes = append(startTimes, st)
					}
				}
				sort.Strings(startTimes)
				minST := ""
				maxST := ""
				if len(startTimes) > 0 {
					minST = startTimes[0]
					maxST = startTimes[len(startTimes)-1]
				}
				threads = append(threads, threadSummary{
					ThreadID:     tid,
					RunCount:     len(runs),
					MinStartTime: minST,
					MaxStartTime: maxST,
				})
			}

			// Sort by max_start_time descending
			sort.Slice(threads, func(i, j int) bool {
				return threads[i].MaxStartTime > threads[j].MaxStartTime
			})

			// Apply limit
			if limit > 0 && len(threads) > limit {
				threads = threads[:limit]
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				columns := []string{"Thread ID", "Run Count", "Min Start", "Max Start"}
				var rows [][]string
				for _, t := range threads {
					rows = append(rows, []string{
						t.ThreadID,
						fmt.Sprintf("%d", t.RunCount),
						t.MinStartTime,
						t.MaxStartTime,
					})
				}
				output.OutputTable(columns, rows, fmt.Sprintf("Threads in %s", project))
			} else {
				var data []map[string]any
				for _, t := range threads {
					data = append(data, map[string]any{
						"thread_id":      t.ThreadID,
						"run_count":      t.RunCount,
						"min_start_time": t.MinStartTime,
						"max_start_time": t.MaxStartTime,
					})
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name (required)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of threads to return")
	cmd.Flags().StringVar(&rawFilter, "filter", "", "Raw LangSmith filter DSL string")
	cmd.Flags().IntVar(&lastNMinutes, "last-n-minutes", 0, "Only include threads active in last N minutes")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}

func newThreadGetCmd() *cobra.Command {
	var (
		project         string
		includeMetadata bool
		includeIO       bool
		full            bool
		limit           int
		outputFile      string
	)

	cmd := &cobra.Command{
		Use:   "get THREAD_ID",
		Short: "Fetch all runs (turns) in a single conversation thread",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			threadID := args[0]

			if full {
				includeMetadata = true
				includeIO = true
			}

			if project == "" {
				exitError("--project is required for thread get")
			}

			c := mustGetClient()
			ctx := context.Background()

			// Resolve project to session ID
			sessionID, err := c.ResolveSessionID(ctx, project)
			if err != nil {
				exitErrorf("%v", err)
			}

			// Query root runs filtered by thread_id
			filterDSL := fmt.Sprintf("eq(thread_id, %q)", threadID)
			queryLimit := 100
			if limit > 0 {
				queryLimit = limit
			}
			params := langsmith.RunQueryParams{
				Session: langsmith.F([]string{sessionID}),
				IsRoot:  langsmith.F(true),
				Filter:  langsmith.F(filterDSL),
				Limit:   langsmith.F(int64(queryLimit)),
			}

			runs, err := queryRuns(ctx, c, params, "", queryLimit, 0)
			if err != nil {
				exitErrorf("querying thread runs: %v", err)
			}

			extracted := extractRunsToMaps(runs, includeMetadata, includeIO, false)

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				output.PrintRunsTable(os.Stdout, extracted, includeMetadata, fmt.Sprintf("Thread %s", threadID))
			} else {
				data := map[string]any{
					"thread_id": threadID,
					"run_count": len(extracted),
					"runs":      extracted,
				}
				output.OutputJSON(data, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name (required)")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, token_usage, costs, tags")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, and error fields")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of runs (turns) to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}
