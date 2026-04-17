package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTraceMessagesCmd() *cobra.Command {
	var (
		ff            FilterFlags
		outputFile    string
		includeRootIO bool
	)

	cmd := &cobra.Command{
		Use:    "messages",
		Short:  "[Private Beta] Get conversation messages for multiple traces (batch)",
		Hidden: true,
		Long: `[Private Beta] Get conversation messages for multiple traces in a single request.
This feature is currently in private beta and may not be available to all users.

Queries root runs matching the given filters, then returns normalized
conversation messages extracted from each trace's LLM and tool runs.

Each trace in the response contains a list of conversation groups:
  - "message" groups contain a single normalized message (human, ai, system, tool)
  - "tool_interaction" groups contain an AI message with tool calls and their results

Requires --project. Results are paginated internally (default limit: 10, max: 100).

Examples:
  langsmith trace messages --project my-chatbot --limit 5
  langsmith trace messages --project my-chatbot --filter "eq(status, \"error\")"
  langsmith trace messages --project my-chatbot --since 2024-01-15T00:00:00Z
  langsmith trace messages --project my-chatbot --trace-ids <id1,id2> --include-root-io`,
		Run: func(cmd *cobra.Command, args []string) {
			defaultLimit := 10
			if ff.Limit == 0 {
				ff.Limit = defaultLimit
			}
			if ff.Limit > 100 {
				ExitError("--limit cannot exceed 100 for trace messages")
			}

			c := MustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				ExitError("--project is required for trace messages (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				ExitErrorf("%v", err)
			}

			// Build base request body for POST /v2/traces/messages
			body := map[string]any{
				"session": []string{sessionID},
			}

			startTime := resolveStartTime(ff.Since, ff.LastNMinutes)
			body["min_start_time"] = startTime.Format("2006-01-02T15:04:05Z07:00")

			if ff.TraceIDs != "" {
				ids := splitTrim(ff.TraceIDs)
				if len(ids) == 1 {
					body["trace"] = ids[0]
				} else {
					body["id"] = ids
				}
			}

			if ff.RunType != "" {
				body["run_type"] = ff.RunType
			}

			if ff.ErrorFlag {
				body["error"] = true
			} else if ff.NoErrorFlag {
				body["error"] = false
			}

			filterStr := buildFilterDSL(&ff)
			if filterStr != "" {
				body["filter"] = filterStr
			}

			// Paginate: fetch up to ff.Limit traces using pages of <= maxPageSize
			const maxPageSize = 10
			remaining := ff.Limit
			var allTraces []any

			for {
				pageSize := maxPageSize
				if remaining < pageSize {
					pageSize = remaining
				}
				body["limit"] = pageSize

				var result map[string]any
				if err := c.RawPost(ctx, "/v2/traces/messages", body, &result); err != nil {
					ExitErrorf("%v", err)
				}

				traces, _ := result["traces"].([]any)
				allTraces = append(allTraces, traces...)
				remaining -= len(traces)

				// Stop if we have enough or no more pages
				cursors, _ := result["cursors"].(map[string]any)
				next, _ := cursors["next"].(string)
				if next == "" || remaining <= 0 {
					break
				}
				body["cursor"] = next
			}

			// Trim to exact limit if we overshot
			if len(allTraces) > ff.Limit {
				allTraces = allTraces[:ff.Limit]
			}

			if includeRootIO {
				attachRootIO(ctx, c, sessionID, startTime, allTraces)
			}

			combined := map[string]any{
				"traces":  allTraces,
				"cursors": map[string]any{},
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" {
				printTraceMessages(combined)
			} else {
				output.OutputJSON(combined, outputFile)
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.Flags().BoolVar(&includeRootIO, "include-root-io", false, "Add root_inputs_preview and root_outputs_preview fields per trace")

	return cmd
}

type rootPreview struct {
	inputs  string
	outputs string
}

// attachRootIO looks up inputs_preview/outputs_preview for the root runs of
// every trace in `traces` and attaches them as root_inputs_preview /
// root_outputs_preview fields. Failures are logged to stderr — the traces
// are returned without enrichment rather than erroring out, so callers never
// lose the main payload to a preview-lookup hiccup.
func attachRootIO(ctx context.Context, c *client.Client, sessionID string, startTime time.Time, traces []any) {
	if len(traces) == 0 {
		return
	}
	ids := make([]string, 0, len(traces))
	for _, t := range traces {
		trace, _ := t.(map[string]any)
		if tid, _ := trace["trace_id"].(string); tid != "" {
			ids = append(ids, tid)
		}
	}
	previews := fetchRootPreviews(ctx, c, sessionID, startTime, ids)
	for _, t := range traces {
		trace, _ := t.(map[string]any)
		if trace == nil {
			continue
		}
		tid, _ := trace["trace_id"].(string)
		if p, ok := previews[tid]; ok {
			trace["root_inputs_preview"] = p.inputs
			trace["root_outputs_preview"] = p.outputs
		} else {
			trace["root_inputs_preview"] = nil
			trace["root_outputs_preview"] = nil
		}
	}
}

// fetchRootPreviews queries root runs for the given trace IDs and returns a
// map trace_id → (inputs_preview, outputs_preview). For root runs, id ==
// trace_id, so we filter by ID (the SDK's RunQueryParams has no list field
// for trace IDs — Trace is singular).
func fetchRootPreviews(ctx context.Context, c *client.Client, sessionID string, startTime time.Time, ids []string) map[string]rootPreview {
	out := map[string]rootPreview{}
	if len(ids) == 0 {
		return out
	}
	params := langsmith.RunQueryParams{
		Session:   langsmith.F([]string{sessionID}),
		IsRoot:    langsmith.F(true),
		ID:        langsmith.F(ids),
		StartTime: langsmith.F(startTime),
		Order:     langsmith.F(langsmith.RunQueryParamsOrderDesc),
		Limit:     langsmith.F(int64(len(ids))),
		Select: langsmith.F([]langsmith.RunQueryParamsSelect{
			langsmith.RunQueryParamsSelectTraceID,
			langsmith.RunQueryParamsSelectInputsPreview,
			langsmith.RunQueryParamsSelectOutputsPreview,
		}),
	}
	resp, err := c.SDK.Runs.Query(ctx, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: fetching root previews failed: %v\n", err)
		return out
	}
	for _, run := range resp.Runs {
		tid := run.TraceID
		if tid == "" {
			tid = run.ID
		}
		if tid == "" {
			continue
		}
		out[tid] = rootPreview{
			inputs:  truncateHard(run.InputsPreview, 2000),
			outputs: truncateHard(run.OutputsPreview, 2000),
		}
	}
	return out
}

func truncateHard(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// printTraceMessages prints a human-readable summary of batch trace messages.
func printTraceMessages(result map[string]any) {
	traces, _ := result["traces"].([]any)
	if len(traces) == 0 {
		fmt.Println("No traces found.")
		return
	}

	for i, t := range traces {
		trace, _ := t.(map[string]any)
		traceID, _ := trace["trace_id"].(string)
		groups, _ := trace["groups"].([]any)

		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("--- Trace %s (%d groups) ---\n", traceID, len(groups))

		for _, g := range groups {
			group, _ := g.(map[string]any)
			gType, _ := group["type"].(string)

			switch gType {
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
	}

	cursors, _ := result["cursors"].(map[string]any)
	if next, ok := cursors["next"].(string); ok && next != "" {
		fmt.Printf("\nNext cursor: %s\n", next)
	}
}

// printMessage prints a single message in a compact format.
func printMessage(msg map[string]any) {
	if msg == nil {
		return
	}
	role, _ := msg["role"].(string)
	switch content := msg["content"].(type) {
	case string:
		text := content
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		fmt.Printf("  [%s] %s\n", role, text)
	case []any:
		for _, block := range content {
			b, _ := block.(map[string]any)
			bType, _ := b["type"].(string)
			switch bType {
			case "text":
				text, _ := b["text"].(string)
				if len(text) > 200 {
					text = text[:200] + "..."
				}
				fmt.Printf("  [%s] %s\n", role, text)
			case "tool_call":
				name, _ := b["name"].(string)
				fmt.Printf("  [%s] tool_call: %s\n", role, name)
			case "reasoning":
				fmt.Printf("  [%s] <reasoning>\n", role)
			default:
				fmt.Printf("  [%s] <%s>\n", role, bType)
			}
		}
	}
}
