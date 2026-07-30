package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/extract"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newThreadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Query multi-turn conversation threads",
		Long: `Query multi-turn conversation threads.

A thread groups multiple root runs that share a thread_id, representing
a multi-turn conversation.

Results are paginated and return at most 20 threads by default
(use --limit to change). Threads are sorted by most recent activity
(newest first).

Examples:
  langsmith thread list --project my-chatbot --limit 10
  langsmith thread get <thread-id> --project my-chatbot --full`,
	}

	cmd.AddCommand(newThreadListCmd())
	cmd.AddCommand(newThreadGetCmd())
	cmd.AddCommand(newThreadMessagesCmd())
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
		Short: "List conversation threads in a project (default: 20, newest first)",
		Run: func(cmd *cobra.Command, args []string) {
			project = ResolveProject(project)
			if project == "" {
				ExitError("--project is required for thread list (or set LANGSMITH_PROJECT)")
			}

			c := MustGetClient()
			ctx := context.Background()

			// Resolve project to session ID
			sessionID, err := c.ResolveSessionID(ctx, project)
			if err != nil {
				ExitErrorf("%v", err)
			}

			// Default to last 24h if no time filter
			startTime := time.Now().UTC().Add(-24 * time.Hour)
			if lastNMinutes > 0 {
				startTime = time.Now().UTC().Add(-time.Duration(lastNMinutes) * time.Minute)
			}

			// Limit is the per-request page size; paginate all root runs, then
			// group by thread_id below.
			params := langsmith.RunQueryParams{
				IsRoot:    langsmith.F(true),
				Limit:     langsmith.F(int64(100)),
				StartTime: langsmith.F(startTime),
			}
			if rawFilter != "" {
				params.Filter = langsmith.F(rawFilter)
			}
			if sel := threadListSelect(); sel != nil {
				params.Select = langsmith.F(sel)
			}

			runs, err := queryRunsAuto(ctx, c, params, threadListSelectV2(), sessionID, math.MaxInt32, 0)
			if err != nil {
				ExitErrorf("querying runs: %v", err)
			}

			// Group runs by thread_id
			threadsMap := make(map[string][]map[string]any)
			for _, run := range runs {
				tid := run.ThreadID
				if tid != "" {
					m := extract.ExtractRun(run, true, true, false)
					threadsMap[tid] = append(threadsMap[tid], m)
				}
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

			fmt_ := GetFormat()

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

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of threads to return")
	cmd.Flags().StringVar(&rawFilter, "filter", "", "Raw LangSmith filter DSL string")
	cmd.Flags().IntVar(&lastNMinutes, "last-n-minutes", 0, "Only include threads active in last N minutes")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newThreadGetCmd() *cobra.Command {
	var (
		project         string
		includeMetadata bool
		includeIO       bool
		includeFeedback bool
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
				includeFeedback = true
			}

			project = ResolveProject(project)
			if project == "" {
				ExitError("--project is required for thread get (or set LANGSMITH_PROJECT)")
			}

			c := MustGetClient()
			ctx := context.Background()

			// Resolve project to session ID
			sessionID, err := c.ResolveSessionID(ctx, project)
			if err != nil {
				ExitErrorf("%v", err)
			}

			// Query root runs filtered by thread_id
			filterDSL := fmt.Sprintf("eq(thread_id, %q)", threadID)
			queryLimit := 100
			if limit > 0 {
				queryLimit = limit
			}
			params := langsmith.RunQueryParams{
				IsRoot: langsmith.F(true),
				Filter: langsmith.F(filterDSL),
				Limit:  langsmith.F(int64(queryLimit)),
			}
			if sel := buildRunSelect(includeIO, includeFeedback); sel != nil {
				params.Select = langsmith.F(sel)
			}

			runs, err := queryRunsAuto(ctx, c, params, buildRunSelectV2(includeIO, includeFeedback), sessionID, queryLimit, 0)
			if err != nil {
				ExitErrorf("querying thread runs: %v", err)
			}

			extracted := extractRunsToMaps(runs, includeMetadata, includeIO, includeFeedback)

			fmt_ := GetFormat()

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

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", false, "Add status, duration_ms, first_token_time, token_usage, costs, tags, custom_metadata (incl. revision_id)")
	cmd.Flags().BoolVar(&includeIO, "include-io", false, "Add inputs, outputs, error, and events fields")
	cmd.Flags().BoolVar(&includeFeedback, "include-feedback", false, "Add feedback_stats field")
	cmd.Flags().BoolVar(&full, "full", false, "Shorthand for --include-metadata --include-io --include-feedback")
	cmd.Flags().IntVarP(&limit, "limit", "n", 0, "Maximum number of runs (turns) to return")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

func newThreadMessagesCmd() *cobra.Command {
	var (
		project    string
		limit      int
		cursor     string
		traceID    string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "messages THREAD_ID",
		Short: "Get conversation messages for a thread",
		Long: `Stream conversation messages across all turns in a thread.

Each response turn is preceded by a turn_boundary marker that carries the
trace_id and turn index, so you can deeplink to the originating trace.

Results are paginated (default: 10 turns, max: 100). Pass --cursor with the
value from a previous response to advance forward or backward.

Examples:
  langsmith thread messages <thread-id> --project my-chatbot
  langsmith thread messages <thread-id> --project my-chatbot --limit 5
  langsmith thread messages <thread-id> --project my-chatbot --cursor <cursor>
  langsmith thread messages <thread-id> --project my-chatbot --trace-id <trace-id>`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			threadID := args[0]

			project = ResolveProject(project)
			if project == "" {
				ExitError("--project is required for thread messages (or set LANGSMITH_PROJECT)")
			}

			c := MustGetClient()
			ctx := context.Background()

			requireV2Feature(ctx, c, "thread messages")

			sessionID, err := c.ResolveSessionID(ctx, project)
			if err != nil {
				ExitErrorf("%v", err)
			}

			q := url.Values{}
			q.Set("project_id", sessionID)
			if limit > 0 {
				q.Set("limit", strconv.Itoa(limit))
			}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			if traceID != "" {
				q.Set("trace_id", traceID)
			}

			path := fmt.Sprintf("/v2/threads/%s/messages?%s", url.PathEscape(threadID), q.Encode())

			extraHeaders := http.Header{"Accept": {"text/event-stream"}}
			_, _, _, body, err := c.RawDo(ctx, http.MethodGet, path, nil, extraHeaders)
			if err != nil {
				ExitErrorf("%v", err)
			}

			groups, nextCursor, prevCursor, sseErr := parseSSEGroups(body)
			if sseErr != "" {
				ExitErrorf("server error: %s", sseErr)
			}

			result := map[string]any{
				"thread_id": threadID,
				"groups":    groups,
				"cursors": map[string]any{
					"next": nextCursor,
					"prev": prevCursor,
				},
			}

			fmt_ := GetFormat()
			if fmt_ == "pretty" {
				printThreadMessages(result)
			} else {
				output.OutputJSON(result, outputFile)
			}
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name [env: LANGSMITH_PROJECT]")
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Maximum number of turns to return (max 100)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor from a previous response")
	cmd.Flags().StringVar(&traceID, "trace-id", "", "Start the page at a specific trace ID")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
}

// sseEvent is a parsed Server-Sent Event.
type sseEvent struct {
	Event string
	Data  string
}

// parseSSE parses a raw SSE response body into discrete events.
func parseSSE(body []byte) []sseEvent {
	var events []sseEvent
	var currentEvent, currentData string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if currentData != "" || currentEvent != "" {
				events = append(events, sseEvent{Event: currentEvent, Data: currentData})
				currentEvent = ""
				currentData = ""
			}
			continue
		}
		if after, ok := strings.CutPrefix(line, "event: "); ok {
			currentEvent = after
		} else if after, ok := strings.CutPrefix(line, "data: "); ok {
			currentData = after
		}
	}
	if currentData != "" || currentEvent != "" {
		events = append(events, sseEvent{Event: currentEvent, Data: currentData})
	}
	return events
}

// parseSSEGroups collects ConversationGroups and cursors from a thread messages SSE stream.
func parseSSEGroups(body []byte) (groups []any, nextCursor, prevCursor, errMsg string) {
	for _, ev := range parseSSE(body) {
		switch ev.Event {
		case "data", "":
			if ev.Data == "" {
				continue
			}
			var group map[string]any
			if err := json.Unmarshal([]byte(ev.Data), &group); err != nil {
				continue
			}
			groups = append(groups, group)
		case "metadata":
			var meta struct {
				NextCursor *string `json:"next_cursor"`
				PrevCursor *string `json:"prev_cursor"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &meta); err == nil {
				if meta.NextCursor != nil {
					nextCursor = *meta.NextCursor
				}
				if meta.PrevCursor != nil {
					prevCursor = *meta.PrevCursor
				}
			}
		case "error":
			var errPayload struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &errPayload); err == nil {
				errMsg = errPayload.Error
			}
		}
	}
	return
}

// printThreadMessages prints a human-readable view of thread messages grouped by turn.
func printThreadMessages(result map[string]any) {
	threadID, _ := result["thread_id"].(string)
	groups, _ := result["groups"].([]any)

	if len(groups) == 0 {
		fmt.Println("No messages found.")
		return
	}

	fmt.Printf("Thread: %s\n", threadID)

	for _, g := range groups {
		group, _ := g.(map[string]any)
		gType, _ := group["type"].(string)

		switch gType {
		case "turn_boundary":
			boundary, _ := group["turnBoundary"].(map[string]any)
			turnIndex, _ := boundary["turn_index"].(float64)
			traceID, _ := boundary["trace_id"].(string)
			fmt.Printf("\n--- Turn %d (trace: %s) ---\n", int(turnIndex)+1, traceID)
		case "message":
			msg, _ := group["message"].(map[string]any)
			printMessage(msg)
		case "tool_interaction":
			aiMsg, _ := group["aiMessage"].(map[string]any)
			printMessage(aiMsg)
			toolCalls, _ := group["toolCalls"].([]any)
			for _, tc := range toolCalls {
				call, _ := tc.(map[string]any)
				name, _ := call["name"].(string)
				fmt.Printf("  [tool_call] %s\n", name)
				if resultMsg, ok := call["result"].(map[string]any); ok {
					printMessage(resultMsg)
				}
			}
		}
	}

	cursors, _ := result["cursors"].(map[string]any)
	if next, ok := cursors["next"].(string); ok && next != "" {
		fmt.Printf("\nNext cursor: %s\n", next)
	}
	if prev, ok := cursors["prev"].(string); ok && prev != "" {
		fmt.Printf("Prev cursor: %s\n", prev)
	}
}
