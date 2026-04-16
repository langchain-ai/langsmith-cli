package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		outDir        string
		fromList      string
		flaggedList   string
		includeRootIO bool
	)

	cmd := &cobra.Command{
		Use:    "messages",
		Short:  "[Private Beta] Get conversation messages for multiple traces (batch)",
		Hidden: true,
		Long: `[Private Beta] Get conversation messages for multiple traces in a single request.
This feature is currently in private beta and may not be available to all users.

Queries root runs matching the given filters (or reads IDs from --from-list),
then returns normalized conversation messages extracted from each trace's
LLM and tool runs.

Each trace in the response contains a list of conversation groups:
  - "message" groups contain a single normalized message (human, ai, system, tool)
  - "tool_interaction" groups contain an AI message with tool calls and their results

Modes:
  1. stdout (default): paginate up to --limit (max 100), emit a single JSON blob.
  2. --out-dir <dir>: write one plain-text conversation file per trace to the
     directory. Use with --from-list to consume a file of IDs produced by
     `+"`langsmith trace list --out-dir`"+`. When --from-list is omitted, the
     command scans the lookback window exhaustively (no --limit cap).

Examples:
  langsmith trace messages --project my-chatbot --limit 5
  langsmith trace messages --project my-chatbot --out-dir /tmp/convos --last-n-minutes 360
  langsmith trace list --project my-chatbot --last-n-minutes 360 --out-dir /tmp/ids --include-flagged
  langsmith trace messages --project my-chatbot --from-list /tmp/ids/trace_ids.txt --flagged-list /tmp/ids/flagged.tsv --out-dir /tmp/convos --include-root-io`,
		Run: func(cmd *cobra.Command, args []string) {
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

			startTime := resolveStartTime(ff.Since, ff.LastNMinutes)

			if outDir != "" {
				if fromList != "" {
					runMessagesFromList(ctx, c, sessionID, startTime, &ff, outDir, fromList, flaggedList, includeRootIO)
				} else {
					runMessagesScanWindow(ctx, c, sessionID, startTime, &ff, outDir, includeRootIO)
				}
				return
			}

			if fromList != "" {
				ExitError("--from-list requires --out-dir")
			}
			if flaggedList != "" {
				ExitError("--flagged-list requires --out-dir")
			}

			runMessagesStdout(ctx, c, sessionID, startTime, &ff, outputFile)
		},
	}

	addCommonFilterFlags(cmd, &ff, true)
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON output to a file")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "Write one plain-text conversation file per trace into this directory. Required for --from-list / --flagged-list / --include-root-io.")
	cmd.Flags().StringVar(&fromList, "from-list", "", "Path to a plain-text file of trace IDs (one per line, as produced by `langsmith trace list --out-dir`). When set, the command fetches conversations for exactly these IDs and skips the lookback scan.")
	cmd.Flags().StringVar(&flaggedList, "flagged-list", "", "Path to a TSV file of flagged trace IDs (`<trace_id>\\t<comment>` per line, as produced by `langsmith trace list --out-dir --include-flagged`). Traces listed here get a [USER_FLAGGED] header in their conversation file.")
	cmd.Flags().BoolVar(&includeRootIO, "include-root-io", false, "Enrich each conversation file with [ROOT INPUT] and [ROOT OUTPUT] blocks from the root run's inputs_preview/outputs_preview. Requires --out-dir.")

	return cmd
}

// runMessagesStdout is the original stdout-mode path: paginate up to --limit
// (capped at 100) and emit the combined JSON / pretty output.
func runMessagesStdout(
	ctx context.Context,
	c *client.Client,
	sessionID string,
	startTime time.Time,
	ff *FilterFlags,
	outputFile string,
) {
	defaultLimit := 10
	if ff.Limit == 0 {
		ff.Limit = defaultLimit
	}
	if ff.Limit > 100 {
		ExitError("--limit cannot exceed 100 for trace messages (unless using --out-dir)")
	}

	body := buildMessagesBody(sessionID, startTime, ff)

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

		cursors, _ := result["cursors"].(map[string]any)
		next, _ := cursors["next"].(string)
		if next == "" || remaining <= 0 {
			break
		}
		body["cursor"] = next
	}

	if len(allTraces) > ff.Limit {
		allTraces = allTraces[:ff.Limit]
	}

	combined := map[string]any{
		"traces":  allTraces,
		"cursors": map[string]any{},
	}

	if GetFormat() == "pretty" {
		printTraceMessages(combined)
	} else {
		output.OutputJSON(combined, outputFile)
	}
}

// runMessagesScanWindow paginates the full lookback window (no ID filter) and
// writes one conversation file per trace. Used when the caller wants
// `trace messages --out-dir` as a standalone one-shot. Prefer
// `trace list --out-dir` → `trace messages --from-list` for the agent flow.
func runMessagesScanWindow(
	ctx context.Context,
	c *client.Client,
	sessionID string,
	startTime time.Time,
	ff *FilterFlags,
	outDir string,
	includeRootIO bool,
) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		ExitErrorf("creating --out-dir: %v", err)
	}

	body := buildMessagesBody(sessionID, startTime, ff)
	allTraces := paginateAllMessages(ctx, c, body, 10)

	rootIO := map[string]rootPreview{}
	if includeRootIO {
		rootIO = fetchRootPreviews(ctx, c, sessionID, startTime)
	}

	written := writeTraceFiles(allTraces, outDir, nil, rootIO)
	output.OutputJSON(map[string]any{
		"status":          "written",
		"traces_written":  written,
		"out_dir":         outDir,
		"include_root_io": includeRootIO,
	}, "")
}

// runMessagesFromList reads trace IDs from a file, fetches conversations in
// batches, and writes one file per trace. This is the primary path in the
// agent flow — trace list writes the ID file, trace messages consumes it.
func runMessagesFromList(
	ctx context.Context,
	c *client.Client,
	sessionID string,
	startTime time.Time,
	ff *FilterFlags,
	outDir string,
	fromList string,
	flaggedList string,
	includeRootIO bool,
) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		ExitErrorf("creating --out-dir: %v", err)
	}

	ids, err := readIDsFile(fromList)
	if err != nil {
		ExitErrorf("reading --from-list: %v", err)
	}
	if len(ids) == 0 {
		output.OutputJSON(map[string]any{
			"status":         "empty",
			"traces_written": 0,
			"out_dir":        outDir,
			"note":           "--from-list file had no trace IDs",
		}, "")
		return
	}

	flagged := map[string]string{}
	if flaggedList != "" {
		flagged, err = readFlaggedFile(flaggedList)
		if err != nil {
			ExitErrorf("reading --flagged-list: %v", err)
		}
	}

	rootIO := map[string]rootPreview{}
	if includeRootIO {
		rootIO = fetchRootPreviews(ctx, c, sessionID, startTime)
	}

	// Batch IDs so the request body stays small and the server's per-call
	// filter stays well under any list-length cap. Within each batch we
	// paginate the response cursor until exhausted.
	const batchSize = 50
	totalWritten := 0
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		body := buildMessagesBody(sessionID, startTime, ff)
		body["id"] = batch
		delete(body, "trace")
		traces := paginateAllMessages(ctx, c, body, 10)

		totalWritten += writeTraceFiles(traces, outDir, flagged, rootIO)
	}

	output.OutputJSON(map[string]any{
		"status":          "written",
		"traces_written":  totalWritten,
		"ids_requested":   len(ids),
		"flagged_count":   len(flagged),
		"out_dir":         outDir,
		"include_root_io": includeRootIO,
	}, "")
}

// writeTraceFiles formats each trace as plain text and writes one file per
// trace into outDir. Returns the number of files written. Flagged IDs get a
// [USER_FLAGGED] header; rootIO entries add [ROOT INPUT]/[ROOT OUTPUT] blocks.
func writeTraceFiles(traces []any, outDir string, flagged map[string]string, rootIO map[string]rootPreview) int {
	written := 0
	for _, t := range traces {
		trace, _ := t.(map[string]any)
		traceID, _ := trace["trace_id"].(string)
		if traceID == "" {
			continue
		}

		var b strings.Builder
		if note, ok := flagged[traceID]; ok {
			if note == "" {
				note = "User flagged for review"
			}
			fmt.Fprintf(&b, "[USER_FLAGGED]: %s\n\n", note)
		}
		if preview, ok := rootIO[traceID]; ok && preview.inputs != "" {
			fmt.Fprintf(&b, "[ROOT INPUT]: %s\n", preview.inputs)
		}
		writeConversationLines(&b, trace)
		if preview, ok := rootIO[traceID]; ok && preview.outputs != "" {
			fmt.Fprintf(&b, "\n[ROOT OUTPUT / FINAL AGENT RESPONSE]: %s", preview.outputs)
		}

		fpath := filepath.Join(outDir, traceID+".txt")
		if err := os.WriteFile(fpath, []byte(b.String()), 0o644); err != nil {
			ExitErrorf("writing %s: %v", fpath, err)
		}
		written++
	}
	return written
}

// readIDsFile reads a plain-text file of trace IDs (one per line). Blank lines
// and leading/trailing whitespace are ignored.
func readIDsFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ids []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		ids = append(ids, line)
	}
	return ids, scanner.Err()
}

// readFlaggedFile reads a TSV file of `<trace_id>\t<comment>` per line and
// returns a map of trace_id → comment.
func readFlaggedFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		tid, comment, _ := strings.Cut(line, "\t")
		tid = strings.TrimSpace(tid)
		if tid == "" {
			continue
		}
		out[tid] = comment
	}
	return out, scanner.Err()
}

// buildMessagesBody builds the base request body shared by all calls to
// /v2/traces/messages in this command.
func buildMessagesBody(sessionID string, startTime time.Time, ff *FilterFlags) map[string]any {
	body := map[string]any{
		"session":        []string{sessionID},
		"min_start_time": startTime.Format("2006-01-02T15:04:05Z07:00"),
	}

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

	if filterStr := buildFilterDSL(ff); filterStr != "" {
		body["filter"] = filterStr
	}

	return body
}

// paginateAllMessages exhaustively paginates /v2/traces/messages using the
// given body. `pageSize` is the per-page `limit` (max 10 server-side). Does
// not mutate `body` for the caller beyond adding the `cursor` key while paging.
func paginateAllMessages(ctx context.Context, c *client.Client, body map[string]any, pageSize int) []any {
	body["limit"] = pageSize
	delete(body, "cursor")

	var all []any
	for {
		var result map[string]any
		if err := c.RawPost(ctx, "/v2/traces/messages", body, &result); err != nil {
			ExitErrorf("%v", err)
		}
		traces, _ := result["traces"].([]any)
		all = append(all, traces...)

		cursors, _ := result["cursors"].(map[string]any)
		next, _ := cursors["next"].(string)
		if next == "" {
			break
		}
		body["cursor"] = next
	}
	return all
}

type rootPreview struct {
	inputs  string
	outputs string
}

// fetchRootPreviews queries root runs for the session within the time window
// and returns a map of trace_id → (inputs_preview, outputs_preview). Each
// preview is truncated to 2000 characters to match the legacy fetch script.
func fetchRootPreviews(ctx context.Context, c *client.Client, sessionID string, startTime time.Time) map[string]rootPreview {
	out := map[string]rootPreview{}

	params := langsmith.RunQueryParams{
		Session:   langsmith.F([]string{sessionID}),
		IsRoot:    langsmith.F(true),
		StartTime: langsmith.F(startTime),
		Order:     langsmith.F(langsmith.RunQueryParamsOrderDesc),
		Limit:     langsmith.F(int64(100)),
		Select: langsmith.F([]langsmith.RunQueryParamsSelect{
			langsmith.RunQueryParamsSelectTraceID,
			langsmith.RunQueryParamsSelectInputsPreview,
			langsmith.RunQueryParamsSelectOutputsPreview,
		}),
	}

	for {
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
		if resp.Cursors == nil || resp.Cursors["next"] == "" {
			break
		}
		params.Cursor = langsmith.F(resp.Cursors["next"])
	}

	return out
}

// writeConversationLines emits one line per message / tool call / tool result
// in the trace. Handles both the flat group shape (group.message,
// group.tool_call, group.tool_result) and the nested shape
// (group.type == "tool_interaction" with aiMessage + toolCalls[]) so the
// output stays stable if the server response evolves.
func writeConversationLines(b *strings.Builder, trace map[string]any) {
	groups, _ := trace["groups"].([]any)
	for _, g := range groups {
		group, _ := g.(map[string]any)
		if group == nil {
			continue
		}

		if msg, ok := group["message"].(map[string]any); ok {
			writeMessageLine(b, msg)
		}

		if tc, ok := group["tool_call"].(map[string]any); ok {
			writeToolCallLine(b, tc)
		}

		if tr, ok := group["tool_result"].(map[string]any); ok {
			writeToolResultLine(b, tr)
		}

		if gt, _ := group["type"].(string); gt == "tool_interaction" {
			if ai, ok := group["aiMessage"].(map[string]any); ok {
				writeMessageLine(b, ai)
			}
			toolCalls, _ := group["toolCalls"].([]any)
			for _, tc := range toolCalls {
				call, _ := tc.(map[string]any)
				if call == nil {
					continue
				}
				writeToolCallLine(b, call)
				if resultMsg, ok := call["result"].(map[string]any); ok {
					writeToolResultLine(b, resultMsg)
				}
			}
		}
	}
}

func writeMessageLine(b *strings.Builder, msg map[string]any) {
	role, _ := msg["role"].(string)
	if role == "" {
		role = "unknown"
	}
	fmt.Fprintf(b, "[%s]: %s\n", strings.ToUpper(role), messageContentAsString(msg["content"]))
}

func writeToolCallLine(b *strings.Builder, tc map[string]any) {
	name, _ := tc["name"].(string)
	if name == "" {
		name = "?"
	}
	args := tc["args"]
	if args == nil {
		args = tc["arguments"]
	}
	argsJSON, _ := json.Marshal(args)
	fmt.Fprintf(b, "[TOOL_CALL] %s: %s\n", name, truncateHard(string(argsJSON), 500))
}

func writeToolResultLine(b *strings.Builder, tr map[string]any) {
	content := tr["content"]
	if content == nil {
		content = tr["result"]
	}
	fmt.Fprintf(b, "[TOOL_RESULT]: %s\n", truncateHard(messageContentAsString(content), 1000))
}

// messageContentAsString renders a message content value (string or list of
// content blocks) as a single-line string.
func messageContentAsString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, block := range v {
			bm, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := bm["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case nil:
		return ""
	default:
		out, _ := json.Marshal(v)
		return string(out)
	}
}

// truncateHard chops the string to at most n bytes with no ellipsis. Matches
// the legacy fetch_traces.py truncation exactly so file contents stay
// byte-for-byte comparable when cutting over.
func truncateHard(s string, n int) string {
	if n <= 0 || len(s) <= n {
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
