package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// trajectoryStep is a compact single-step view of one message/tool-call in a trace.
type trajectoryStep struct {
	Role      string `json:"role"`
	ToolName  string `json:"tool_name,omitempty"`
	Tokens    *int64 `json:"tokens,omitempty"`
	LatencyMS *int64 `json:"latency_ms,omitempty"`
	Model     string `json:"model,omitempty"`
}

// traceTrajectory is the compact trajectory for a single trace.
type traceTrajectory struct {
	TraceID string           `json:"trace_id"`
	Steps   []trajectoryStep `json:"steps"`
}

func newTraceMessagesCmd() *cobra.Command {
	var (
		ff         FilterFlags
		outputFile string
		trajectory bool
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

Requires --project. Results are paginated internally (default limit: 10, max: 100).

Examples:
  langsmith trace messages --project my-chatbot --limit 5
  langsmith trace messages --project my-chatbot --filter "eq(status, \"error\")"
  langsmith trace messages --project my-chatbot --since 2024-01-15T00:00:00Z`,
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

			fmt_ := GetFormat()

			if trajectory {
				var trajs []traceTrajectory
				for _, t := range allTraces {
					trace, _ := t.(map[string]any)
					trajs = append(trajs, buildTraceTrajectory(trace))
				}
				if fmt_ == "pretty" {
					printTrajectories(trajs)
				} else {
					output.OutputJSON(map[string]any{"traces": trajs}, outputFile)
				}
			} else {
				combined := map[string]any{
					"traces":  allTraces,
					"cursors": map[string]any{},
				}
				if fmt_ == "pretty" {
					printTraceMessages(combined)
				} else {
					output.OutputJSON(combined, outputFile)
				}
			}
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.Flags().BoolVar(&trajectory, "trajectory", false, "Output compact trajectory (role, tool_name, tokens, latency_ms) instead of full messages")

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

// buildTraceTrajectory converts a raw trace map into a compact trajectory.
func buildTraceTrajectory(trace map[string]any) traceTrajectory {
	traceID, _ := trace["trace_id"].(string)
	groups, _ := trace["groups"].([]any)

	var steps []trajectoryStep
	for _, g := range groups {
		group, _ := g.(map[string]any)
		gType, _ := group["type"].(string)
		meta, _ := group["metadata"].(map[string]any)

		tokens := trajTokens(meta)
		latency := trajLatency(meta)
		model := trajModel(meta)

		switch gType {
		case "message":
			msg, _ := group["message"].(map[string]any)
			role, _ := msg["role"].(string)
			step := trajectoryStep{Role: role}
			if role == "ai" {
				step.Tokens = tokens
				step.LatencyMS = latency
				step.Model = model
			}
			steps = append(steps, step)
		case "tool_interaction":
			aiMsg, _ := group["aiMessage"].(map[string]any)
			role, _ := aiMsg["role"].(string)
			steps = append(steps, trajectoryStep{
				Role:      role,
				Tokens:    tokens,
				LatencyMS: latency,
				Model:     model,
			})
			toolCalls, _ := group["toolCalls"].([]any)
			for _, tc := range toolCalls {
				call, _ := tc.(map[string]any)
				name, _ := call["name"].(string)
				steps = append(steps, trajectoryStep{
					Role:     "tool",
					ToolName: name,
				})
			}
		}
	}

	return traceTrajectory{TraceID: traceID, Steps: steps}
}

func trajTokens(meta map[string]any) *int64 {
	if meta == nil {
		return nil
	}
	tu, ok := meta["token_usage"].(map[string]any)
	if !ok {
		return nil
	}
	if v, ok := tu["total_tokens"].(float64); ok {
		n := int64(v)
		return &n
	}
	return nil
}

func trajLatency(meta map[string]any) *int64 {
	if meta == nil {
		return nil
	}
	if v, ok := meta["latency_ms"].(float64); ok {
		n := int64(v)
		return &n
	}
	return nil
}

func trajModel(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	s, _ := meta["model_name"].(string)
	return s
}

// printTrajectories prints a compact human-readable trajectory view.
func printTrajectories(trajs []traceTrajectory) {
	if len(trajs) == 0 {
		fmt.Println("No traces found.")
		return
	}
	for i, traj := range trajs {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("--- Trace %s (%d steps) ---\n", traj.TraceID, len(traj.Steps))
		for _, step := range traj.Steps {
			var parts []string
			if step.ToolName != "" {
				parts = append(parts, fmt.Sprintf("  [%-6s] %s", step.Role, step.ToolName))
			} else {
				parts = append(parts, fmt.Sprintf("  [%-6s]", step.Role))
			}
			if step.Tokens != nil {
				parts = append(parts, fmt.Sprintf("%d tok", *step.Tokens))
			}
			if step.LatencyMS != nil {
				parts = append(parts, fmt.Sprintf("%dms", *step.LatencyMS))
			}
			if step.Model != "" {
				parts = append(parts, step.Model)
			}
			fmt.Println(strings.Join(parts, " | "))
		}
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
