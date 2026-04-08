package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTraceMessagesCmd() *cobra.Command {
	var (
		ff         FilterFlags
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "messages",
		Short:  "[Private Beta] Get conversation messages for multiple traces (batch)",
		Hidden: true,
		Long: `[Private Beta] Get conversation messages for multiple traces in a single request.
This feature is currently in private beta and may not be available to all users.

Queries root runs matching the given filters, then returns normalized
conversation messages extracted from each trace's LLM and tool runs.

Each trace in the response contains a list of conversation groups:
  - "message" groups contain a single normalized message (human, ai, system, tool)
  - "tool_interaction" groups contain an AI message with tool calls and their results

Requires --project. Results are paginated internally (default limit: 10).

Examples:
  langsmith trace messages --project my-chatbot --limit 5
  langsmith trace messages --project my-chatbot --filter "eq(status, \"error\")"
  langsmith trace messages --project my-chatbot --since 2024-01-15T00:00:00Z`,
		Run: func(cmd *cobra.Command, args []string) {
			defaultLimit := 10
			if ff.Limit == 0 {
				ff.Limit = defaultLimit
			}

			c := mustGetClient()
			ctx := context.Background()
			projectName := ResolveProject(ff.Project)
			if projectName == "" {
				exitError("--project is required for trace messages (or set LANGSMITH_PROJECT)")
			}

			sessionID, err := c.ResolveSessionID(ctx, projectName)
			if err != nil {
				exitErrorf("%v", err)
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
					exitErrorf("%v", err)
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

			combined := map[string]any{
				"traces":  allTraces,
				"cursors": map[string]any{},
			}

			fmt_ := getFormat()

			if fmt_ == "pretty" {
				printTraceMessages(combined)
			} else {
				output.OutputJSON(combined, outputFile)
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")

	return cmd
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
