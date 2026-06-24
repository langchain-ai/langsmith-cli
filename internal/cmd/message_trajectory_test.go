package cmd

import (
	"strings"
	"testing"
)

// The trajectory must stay THIN — no message content / tool args / results.
// It is scanned across all traces, so per-trace detail belongs in the digest.
func TestBuildTraceTrajectory_IsThin(t *testing.T) {
	trace := map[string]any{
		"trace_id": "t1",
		"groups": []any{
			map[string]any{"type": "message", "message": map[string]any{"role": "human", "content": "x" + strings.Repeat("y", 5000)}},
			map[string]any{
				"type":      "tool_interaction",
				"aiMessage": map[string]any{"role": "ai", "content": "thinking"},
				"toolCalls": []any{map[string]any{"name": "search", "args": map[string]any{"q": "z"}, "result": map[string]any{"content": "404 not found"}}},
			},
		},
	}
	traj := buildTraceTrajectory(trace)
	if len(traj.Steps) != 3 { // human, ai, tool
		t.Fatalf("got %d steps, want 3", len(traj.Steps))
	}
	// Steps carry only role/tool_name/chars — no content fields exist on the struct.
	if traj.Steps[2].Role != "tool" || traj.Steps[2].ToolName != "search" {
		t.Errorf("tool step = %+v", traj.Steps[2])
	}
	if traj.Steps[0].Chars != 5001 {
		t.Errorf("human step chars = %d, want 5001 (count only, no content)", traj.Steps[0].Chars)
	}
}

// buildTraceDigest carries the detail: full first/final messages + a normalized
// per-group index with coerced content, tool args/results, and errorish flags.
func TestBuildTraceDigest_NormalizedGroups(t *testing.T) {
	trace := map[string]any{
		"trace_id":             "t1",
		"root_inputs_preview":  "human: how do I use MCP?",
		"root_outputs_preview": "tool: clipped preview that isn't the answer",
		"groups": []any{
			map[string]any{"type": "message", "message": map[string]any{"role": "human", "content": "how do I use MCP?"}},
			map[string]any{
				"type":      "tool_interaction",
				"aiMessage": map[string]any{"role": "ai", "content": []any{map[string]any{"type": "text", "text": "searching the docs"}}},
				"toolCalls": []any{map[string]any{"name": "search_docs", "args": map[string]any{"q": "mcp"}, "result": map[string]any{"content": "404 not found"}}},
			},
			map[string]any{"type": "message", "message": map[string]any{"role": "ai", "content": "Use create_agent with the mcp arg."}},
		},
	}
	d := buildTraceDigest(trace)

	if d.FirstHuman != "how do I use MCP?" {
		t.Errorf("first_human = %q, want full human message from groups", d.FirstHuman)
	}
	if d.FinalAI != "Use create_agent with the mcp arg." {
		t.Errorf("final_ai = %q, want final ai message from groups", d.FinalAI)
	}
	if d.NGroups != 3 || len(d.GroupIndex) != 3 {
		t.Fatalf("n_groups=%d group_index=%d, want 3/3", d.NGroups, len(d.GroupIndex))
	}
	// tool_interaction group: content coerced from list-of-blocks, tool fields present.
	tg := d.GroupIndex[1]
	if tg.Kind != "tool_interaction" || tg.Text != "searching the docs" {
		t.Errorf("group[1] kind/text = %q/%q", tg.Kind, tg.Text)
	}
	if len(tg.Tools) != 1 || tg.Tools[0] != "search_docs" {
		t.Errorf("group[1] tools = %v", tg.Tools)
	}
	if len(tg.ToolArgs) != 1 || tg.ToolArgs[0].Name != "search_docs" || !strings.Contains(tg.ToolArgs[0].Args, `"q":"mcp"`) {
		t.Errorf("group[1] tool_args = %+v", tg.ToolArgs)
	}
	if len(tg.ToolResult) != 1 || tg.ToolResult[0] != "404 not found" {
		t.Errorf("group[1] tool_result = %v", tg.ToolResult)
	}
}

// Null/missing content coerces to empty (no panic); first/final fall back to previews.
func TestBuildTraceDigest_NullAndFallback(t *testing.T) {
	trace := map[string]any{
		"trace_id":             "t2",
		"root_inputs_preview":  "human: hi",
		"root_outputs_preview": "the answer",
		"groups": []any{
			map[string]any{
				"type":      "tool_interaction",
				"aiMessage": map[string]any{"role": "ai", "content": nil},
				"toolCalls": []any{map[string]any{"name": "read_url", "result": map[string]any{"content": nil}}},
			},
		},
	}
	d := buildTraceDigest(trace)
	if d.FirstHuman != "human: hi" || d.FinalAI != "the answer" {
		t.Errorf("fallbacks: first=%q final=%q", d.FirstHuman, d.FinalAI)
	}
	g := d.GroupIndex[0]
	if g.Text != "" || g.ToolResult[0] != "" || g.Errorish {
		t.Errorf("null content should be empty/non-errorish, got %+v", g)
	}
}

// Non-null structured content (image / tool_use blocks, or a map with no
// text/content field) must NOT collapse to "" — that is indistinguishable from
// a genuinely empty turn and misleads a downstream reader into thinking the
// content is absent. Only nil stays empty.
func TestCoerceContent_UnrenderedStructuredContent(t *testing.T) {
	if got := coerceContent(nil); got != "" {
		t.Errorf("nil content = %q, want empty", got)
	}
	// A structured object with no text/content field (e.g. an image block).
	img := map[string]any{"type": "image", "source": map[string]any{"url": "https://x/y.png"}}
	if got := coerceContent(img); got == "" || !strings.Contains(got, "image") {
		t.Errorf("image block coerced to %q, want non-empty mentioning its type", got)
	}
	// A list with a renderable text block and an unrenderable tool_use block:
	// the tool_use must still contribute non-empty content, not be dropped.
	blocks := []any{
		map[string]any{"type": "text", "text": "calling a tool"},
		map[string]any{"type": "tool_use", "name": "search", "input": map[string]any{"q": "mcp"}},
	}
	if got := coerceContent(blocks); !strings.Contains(got, "calling a tool") || !strings.Contains(got, "tool_use") {
		t.Errorf("block list coerced to %q, want both the text and the tool_use block", got)
	}
}

// errorish must flag error-SHAPED results, not text that merely mentions "error".
func TestBuildTraceDigest_ErrorishPrecision(t *testing.T) {
	cases := []struct {
		content string
		want    bool
	}{
		{"Error: connection refused", true},
		{"404 not found", true},
		{`{"error": "rate limited"}`, true},
		{"HTTP 503 service unavailable", true},
		{"Traceback (most recent call last):", true},
		{"Evaluators help you catch error cases in your app.", false},
		{"This guide covers error handling best practices.", false},
		{`{"level": "error", "matched": 4}`, false},
	}
	for _, c := range cases {
		trace := map[string]any{"groups": []any{map[string]any{
			"type":      "tool_interaction",
			"aiMessage": map[string]any{"role": "ai", "content": ""},
			"toolCalls": []any{map[string]any{"name": "t", "result": map[string]any{"content": c.content}}},
		}}}
		got := buildTraceDigest(trace).GroupIndex[0].Errorish
		if got != c.want {
			t.Errorf("errorish(%q) = %v, want %v", c.content, got, c.want)
		}
	}
}

// Long content is bounded to the digest caps.
func TestBuildTraceDigest_Truncation(t *testing.T) {
	big := strings.Repeat("x", 5000)
	trace := map[string]any{"groups": []any{
		map[string]any{"type": "message", "message": map[string]any{"role": "human", "content": big}},
		map[string]any{
			"type":      "tool_interaction",
			"aiMessage": map[string]any{"role": "ai", "content": "ok"},
			"toolCalls": []any{map[string]any{"name": "f", "args": map[string]any{"k": big}, "result": map[string]any{"content": big}}},
		},
	}}
	d := buildTraceDigest(trace)
	if got := len(d.GroupIndex[0].Text); got != digestTextMax {
		t.Errorf("group text len = %d, want %d", got, digestTextMax)
	}
	tg := d.GroupIndex[1]
	if got := len(tg.ToolResult[0]); got != digestResultMax {
		t.Errorf("tool_result len = %d, want %d", got, digestResultMax)
	}
	if got := len(tg.ToolArgs[0].Args); got != digestArgsMax {
		t.Errorf("tool_args len = %d, want %d", got, digestArgsMax)
	}
}
