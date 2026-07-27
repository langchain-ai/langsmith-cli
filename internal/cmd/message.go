package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// Char caps for the per-trace digest fields. Bounded so a single huge message
// or tool payload can't blow up the digest (and the context of any agent
// reading it). The full untruncated content is still available in `groups`.
const (
	digestTextMax   = 600
	digestArgsMax   = 200
	digestResultMax = 200
)

// errorishPattern is a heuristic flag for a tool result whose content looks
// like an *error response* (not merely text that mentions the word "error" —
// technical docs and search results say "error" constantly). It requires
// error-shaped context: an error/exception token with `:`/`=`, a quoted
// `"error":` key, an HTTP/status 4xx-5xx, a 4xx-5xx code next to a failure
// word, a traceback, or a common shell/tool failure string. Advisory, not
// authoritative — a reader still confirms by looking at the result.
var errorishPattern = regexp.MustCompile(`(?i)<tool_use_error>|traceback|command not found|permission denied|no such file|\b(error|exception)\b\s*[:=]|"error"\s*[:=]|\bhttp[ /]?[45]\d\d\b|\b[45]\d\d\b\s+\w*\s*(error|not found|forbidden|unauthorized|internal server)`)

// trajectoryStep is a compact single-step view of one message/tool-call. It is
// deliberately THIN: the trajectory is scanned across *all* traces in a batch
// (triage), so per-trace content lives in the separate `digest` field, read
// per-trace — not crammed into the bulk-scanned trajectory.
type trajectoryStep struct {
	Role      string `json:"role"`
	ToolName  string `json:"tool_name,omitempty"`
	Tokens    *int64 `json:"tokens,omitempty"`
	LatencyMS *int64 `json:"latency_ms,omitempty"`
	Chars     int    `json:"chars,omitempty"`
}

// traceTrajectory is the compact trajectory for a single trace.
type traceTrajectory struct {
	TraceID string           `json:"trace_id"`
	Steps   []trajectoryStep `json:"steps"`
}

// digestToolArg is one tool call's name and its (bounded) JSON args.
type digestToolArg struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// digestGroup is one normalized group in a trace's digest — enough to verdict
// the group without re-parsing the heterogeneously-shaped raw `groups`.
type digestGroup struct {
	I          int             `json:"i"`
	Kind       string          `json:"kind"`
	Role       string          `json:"role,omitempty"`
	Chars      int             `json:"chars"`
	Text       string          `json:"text,omitempty"`        // coerced content, ≤digestTextMax
	Tools      []string        `json:"tools,omitempty"`       // tool-call names in this group
	ToolArgs   []digestToolArg `json:"tool_args,omitempty"`   // per call: name + ≤digestArgsMax JSON args
	ToolResult []string        `json:"tool_result,omitempty"` // per call: coerced result, ≤digestResultMax
	Errorish   bool            `json:"errorish,omitempty"`    // any tool result looks like an error
}

// traceDigest is the detailed per-trace view a screener reads for the ONE trace
// it is judging. It is emitted as its own field (NOT folded into the trajectory)
// so this bulk detail is read per-trace, never scanned across the whole batch.
type traceDigest struct {
	FirstHuman string        `json:"first_human,omitempty"`
	FinalAI    string        `json:"final_ai,omitempty"`
	NGroups    int           `json:"n_groups"`
	GroupIndex []digestGroup `json:"group_index"`
}

// attachTrajectory attaches the thin trajectory and the per-trace digest to a
// trace map for JSON output. The trajectory is scanned across all traces; the
// digest is read per-trace.
func attachTrajectory(trace map[string]any, traj traceTrajectory, digest traceDigest) {
	trace["trajectory"] = traj.Steps
	trace["digest"] = digest
}

func newTraceMessagesCmd() *cobra.Command {
	var (
		ff         FilterFlags
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Get conversation messages for multiple traces (batch)",
		Long: `Get conversation messages for multiple traces in a single request.

Queries root runs matching the given filters, then returns normalized
conversation messages extracted from each trace's LLM and tool runs.

Each trace in the response contains a list of conversation groups:
  - "message" groups contain a single normalized message (human, ai, system, tool)
  - "tool_interaction" groups contain an AI message with tool calls and their results

Requires --project and an explicit start time (--since or --last-n-minutes);
unlike the read/query commands this one has no implicit time window.
Default limit: 10, max: 100.

Examples:
  langsmith trace messages --project my-chatbot --last-n-minutes 60 --limit 5
  langsmith trace messages --project my-chatbot --last-n-minutes 60 --filter "eq(status, \"error\")"
  langsmith trace messages --project my-chatbot --since <YYYY-MM-DDTHH:MM:SSZ>
  langsmith trace messages --project my-chatbot --last-n-minutes 1440 --trace-ids <id1,id2>`,
		Run: func(cmd *cobra.Command, args []string) {
			defaultLimit := 10
			if ff.Limit == 0 {
				ff.Limit = defaultLimit
			}
			if ff.Limit > 100 {
				ExitError("--limit cannot exceed 100 for trace messages")
			}

			// min_start_time is a required request param for POST /v2/traces/messages.
			// Unlike the read/query commands, this command has no implicit time window:
			// require the caller to scope it explicitly rather than silently defaulting.
			if ff.Since == "" && ff.LastNMinutes <= 0 {
				ExitError("trace messages requires an explicit start time: pass --since <timestamp> or --last-n-minutes <N>")
			}

			c := MustGetClient()
			ctx := context.Background()

			requireV2Feature(ctx, c, "trace messages")

			sessionID, err := resolveSessionID(ctx, c, ff.Project, ff.ProjectID, "trace messages")
			if err != nil {
				ExitErrorf("%v", err)
			}

			// Build base request body for POST /v2/traces/messages
			body := map[string]any{
				"project_ids": []string{sessionID},
			}

			startTime := resolveStartTime(ff.Since, ff.LastNMinutes)
			body["min_start_time"] = startTime.Format("2006-01-02T15:04:05Z07:00")

			if ff.Before != "" {
				body["max_start_time"] = ff.Before
			}

			if ff.TraceIDs != "" {
				ids := splitTrim(ff.TraceIDs)
				body["ids"] = ids
			}

			if ff.RunType != "" {
				body["run_type"] = ff.RunType
			}

			if ff.ErrorFlag {
				body["has_error"] = true
			} else if ff.NoErrorFlag {
				body["has_error"] = false
			}

			filterStr := buildFilterDSL(&ff)
			if filterStr != "" {
				body["filter"] = filterStr
			}

			// Single-page mode: when --cursor is explicitly set, make one API call
			// and expose the real cursors.next so callers can paginate externally.
			if cmd.Flags().Changed("cursor") {
				pageSize := ff.Limit
				if pageSize == 0 {
					pageSize = defaultLimit
				}
				if ff.Cursor != "" {
					body["cursor"] = ff.Cursor
				}
				body["page_size"] = pageSize

				var result map[string]any
				if err := c.RawPost(ctx, "/v2/traces/messages", body, &result); err != nil {
					ExitErrorf("%v", err)
				}

				traces, _ := result["items"].([]any)
				if traces == nil {
					traces = []any{}
				}

				attachRootIO(ctx, c, sessionID, startTime, traces)
				for _, t := range traces {
					trace, _ := t.(map[string]any)
					traj := buildTraceTrajectory(trace)
					attachTrajectory(trace, traj, buildTraceDigest(trace))
				}

				nextCursor, _ := result["next_cursor"].(string)
				cursors := map[string]any{}
				if nextCursor != "" {
					cursors["next"] = nextCursor
				}

				combined := map[string]any{
					"traces":  traces,
					"cursors": cursors,
				}
				fmt_ := GetFormat()
				if fmt_ == "pretty" {
					printTraceMessages(combined)
				} else {
					output.OutputJSON(combined, outputFile)
				}
				return
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
				body["page_size"] = pageSize

				var result map[string]any
				if err := c.RawPost(ctx, "/v2/traces/messages", body, &result); err != nil {
					ExitErrorf("%v", err)
				}

				traces, _ := result["items"].([]any)
				allTraces = append(allTraces, traces...)
				remaining -= len(traces)

				// Stop if we have enough or no more pages
				next, _ := result["next_cursor"].(string)
				if next == "" || remaining <= 0 {
					break
				}
				body["cursor"] = next
			}

			// Trim to exact limit if we overshot
			if len(allTraces) > ff.Limit {
				allTraces = allTraces[:ff.Limit]
			}

			attachRootIO(ctx, c, sessionID, startTime, allTraces)

			for _, t := range allTraces {
				trace, _ := t.(map[string]any)
				traj := buildTraceTrajectory(trace)
				attachTrajectory(trace, traj, buildTraceDigest(trace))
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
	cmd.Flags().StringVar(&ff.ProjectID, "project-id", "", "Project (session) UUID; skips the name lookup. Takes precedence over --project / $LANGSMITH_PROJECT")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.MarkFlagsMutuallyExclusive("project", "project-id")

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
	v2Select := []langsmith.RunQueryV2ParamsSelect{
		langsmith.RunQueryV2ParamsSelectTraceID,
		langsmith.RunQueryV2ParamsSelectInputsPreview,
		langsmith.RunQueryV2ParamsSelectOutputsPreview,
	}
	runs, err := queryRunsAuto(ctx, c, params, v2Select, sessionID, len(ids), 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: fetching root previews failed: %v\n", err)
		return out
	}
	for _, run := range runs {
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

// buildTraceTrajectory converts a raw trace map into a compact, THIN trajectory
// (role/tool/tokens/latency/chars per step). It carries no message content —
// the trajectory is scanned across all traces for triage, so detail lives in
// buildTraceDigest and is read per-trace.
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

		switch gType {
		case "message":
			msg, _ := group["message"].(map[string]any)
			role, _ := msg["role"].(string)
			step := trajectoryStep{Role: role, Chars: msgChars(msg)}
			if role == "ai" {
				step.Tokens = tokens
				step.LatencyMS = latency
			}
			steps = append(steps, step)
		case "tool_interaction":
			aiMsg, _ := group["aiMessage"].(map[string]any)
			role, _ := aiMsg["role"].(string)
			steps = append(steps, trajectoryStep{
				Role:      role,
				Tokens:    tokens,
				LatencyMS: latency,
				Chars:     msgChars(aiMsg),
			})
			toolCalls, _ := group["toolCalls"].([]any)
			for _, tc := range toolCalls {
				call, _ := tc.(map[string]any)
				name, _ := call["name"].(string)
				result, _ := call["result"].(map[string]any)
				steps = append(steps, trajectoryStep{
					Role:     "tool",
					ToolName: name,
					Chars:    msgChars(result),
				})
			}
		}
	}
	return traceTrajectory{TraceID: traceID, Steps: steps}
}

// buildTraceDigest builds the detailed per-trace digest: the full first-human /
// final-AI messages plus a normalized per-group index (content coerced to
// strings, fields length-bounded). Read per-trace by a screener — kept OUT of
// the bulk-scanned trajectory on purpose. Mirrors the fetch-time digest the
// issues-agent screeners verdict from.
func buildTraceDigest(trace map[string]any) traceDigest {
	inputsPreview, _ := trace["root_inputs_preview"].(string)
	outputsPreview, _ := trace["root_outputs_preview"].(string)
	groups, _ := trace["groups"].([]any)

	var firstHuman, finalAI string
	index := make([]digestGroup, 0, len(groups))
	for i, g := range groups {
		group, _ := g.(map[string]any)
		gType, _ := group["type"].(string)
		dg := digestGroup{I: i, Kind: gType}
		if dg.Kind == "" {
			dg.Kind = "message"
		}

		var text string
		switch gType {
		case "message":
			msg, _ := group["message"].(map[string]any)
			dg.Role, _ = msg["role"].(string)
			text = msgText(msg)
			if dg.Role == "human" && firstHuman == "" {
				firstHuman = text
			}
			if dg.Role == "ai" && text != "" {
				finalAI = text
			}
		case "tool_interaction":
			aiMsg, _ := group["aiMessage"].(map[string]any)
			text = msgText(aiMsg)
			if text != "" {
				finalAI = text
			}
			toolCalls, _ := group["toolCalls"].([]any)
			for _, tc := range toolCalls {
				call, _ := tc.(map[string]any)
				name, _ := call["name"].(string)
				dg.Tools = append(dg.Tools, name)
				dg.ToolArgs = append(dg.ToolArgs, digestToolArg{
					Name: name,
					Args: truncateHard(marshalArgs(call["args"]), digestArgsMax),
				})
				result, _ := call["result"].(map[string]any)
				resultText := ""
				if result != nil {
					resultText = coerceContent(result["content"])
				}
				dg.ToolResult = append(dg.ToolResult, truncateHard(resultText, digestResultMax))
				if resultText != "" && errorishPattern.MatchString(resultText) {
					dg.Errorish = true
				}
			}
		}
		dg.Chars = len(text)
		dg.Text = truncateHard(text, digestTextMax)
		index = append(index, dg)
	}

	// Prefer the full message from groups; fall back to the (~150-char) preview
	// only when groups don't carry it.
	if firstHuman == "" {
		firstHuman = inputsPreview
	}
	if finalAI == "" {
		finalAI = outputsPreview
	}
	return traceDigest{
		FirstHuman: firstHuman,
		FinalAI:    finalAI,
		NGroups:    len(groups),
		GroupIndex: index,
	}
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
	if v, ok := meta["latency_milli_seconds"].(float64); ok {
		n := int64(v)
		return &n
	}
	return nil
}

// msgChars returns the total character count of a message's content.
func msgChars(msg map[string]any) int {
	if msg == nil {
		return 0
	}
	total := 0
	switch c := msg["content"].(type) {
	case string:
		total = len(c)
	case []any:
		for _, block := range c {
			b, _ := block.(map[string]any)
			if text, ok := b["text"].(string); ok {
				total += len(text)
			}
		}
	}
	return total
}

// coerceContent flattens a message/tool `content` value — which may be a
// string, a list of content blocks, an object, or null — into a plain string.
// Content shape is heterogeneous across integrations; normalizing here lets a
// reader verdict off the trajectory without re-parsing each shape.
func coerceContent(v any) string {
	switch c := v.(type) {
	case nil:
		return ""
	case string:
		return c
	case []any:
		var parts []string
		for _, block := range c {
			switch b := block.(type) {
			case string:
				parts = append(parts, b)
			case map[string]any:
				if t, ok := b["text"].(string); ok {
					parts = append(parts, t)
				} else if t, ok := b["content"].(string); ok {
					parts = append(parts, t)
				} else {
					parts = append(parts, unrenderedContent(b))
				}
			default:
				parts = append(parts, fmt.Sprintf("%v", b))
			}
		}
		return strings.Join(parts, " ")
	case map[string]any:
		if t, ok := c["text"].(string); ok {
			return t
		}
		if t, ok := c["content"].(string); ok {
			return t
		}
		return unrenderedContent(c)
	default:
		return fmt.Sprintf("%v", c)
	}
}

// unrenderedContent represents a non-null content block that carries no
// plain-text `text`/`content` field (image, tool_use, or other structured
// blocks) as a non-empty string. Collapsing these to "" makes structured
// content indistinguishable from a genuinely empty turn, which misleads a
// downstream reader (e.g. the engine screener) into treating the content as
// absent. Prefer compact JSON (the downstream char caps truncate it); fall back
// to a typed sentinel if the block can't be marshaled.
func unrenderedContent(m map[string]any) string {
	if b, err := json.Marshal(m); err == nil {
		return string(b)
	}
	if t, ok := m["type"].(string); ok && t != "" {
		return "[" + t + " content]"
	}
	return "[unrenderable content]"
}

// msgText returns a message's content coerced to a plain string.
func msgText(msg map[string]any) string {
	if msg == nil {
		return ""
	}
	return coerceContent(msg["content"])
}

// marshalArgs renders tool-call args as compact JSON, or "" if absent/unmarshalable.
func marshalArgs(args any) string {
	if args == nil {
		return ""
	}
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(b)
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
