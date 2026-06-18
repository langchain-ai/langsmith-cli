package cmd

import (
	"strings"
	"testing"
)

// buildTraceTrajectory should normalize heterogeneously-shaped group content
// into the trajectory's text / tool_args / tool_result / errorish fields, and
// source the full first-human / final-AI messages from groups (not the clipped
// previews).
func TestBuildTraceTrajectory_NormalizedFields(t *testing.T) {
	trace := map[string]any{
		"trace_id":             "t1",
		"root_inputs_preview":  "human: how do I use MCP?",
		"root_outputs_preview": "tool: a truncated preview that is not the answer",
		"groups": []any{
			map[string]any{
				"type":    "message",
				"message": map[string]any{"role": "human", "content": "how do I use MCP?"},
			},
			map[string]any{
				"type": "tool_interaction",
				// aiMessage content as a list of blocks (must coerce to a string)
				"aiMessage": map[string]any{
					"role":    "ai",
					"content": []any{map[string]any{"type": "text", "text": "searching the docs"}},
				},
				"toolCalls": []any{
					map[string]any{
						"name":   "search_docs",
						"args":   map[string]any{"q": "mcp"},
						"result": map[string]any{"content": "404 not found"},
					},
				},
			},
			map[string]any{
				"type":    "message",
				"message": map[string]any{"role": "ai", "content": "Use create_agent with the mcp arg."},
			},
		},
	}

	traj := buildTraceTrajectory(trace)

	// first_human / final_ai come from groups (full), not the previews.
	if traj.InputMessage != "how do I use MCP?" {
		t.Errorf("InputMessage = %q, want full human message from groups", traj.InputMessage)
	}
	if traj.OutputMessage != "Use create_agent with the mcp arg." {
		t.Errorf("OutputMessage = %q, want final ai message from groups", traj.OutputMessage)
	}

	// steps: human, ai (tool_interaction), tool, ai (final message)
	if len(traj.Steps) != 4 {
		t.Fatalf("got %d steps, want 4", len(traj.Steps))
	}
	if traj.Steps[1].Text != "searching the docs" {
		t.Errorf("ai step Text = %q, want list-of-blocks coerced to a string", traj.Steps[1].Text)
	}
	tool := traj.Steps[2]
	if tool.Role != "tool" || tool.ToolName != "search_docs" {
		t.Errorf("tool step = %+v, want role=tool name=search_docs", tool)
	}
	if !strings.Contains(tool.ToolArgs, `"q":"mcp"`) {
		t.Errorf("tool_args = %q, want JSON of the call args", tool.ToolArgs)
	}
	if tool.ToolResult != "404 not found" {
		t.Errorf("tool_result = %q, want coerced result content", tool.ToolResult)
	}
	if !tool.Errorish {
		t.Errorf("errorish = false, want true for a '404 not found' result")
	}
}

// Null / missing content must coerce to empty without panicking, and the
// trajectory must fall back to previews when groups carry no human/ai message.
func TestBuildTraceTrajectory_NullContentAndFallback(t *testing.T) {
	trace := map[string]any{
		"trace_id":             "t2",
		"root_inputs_preview":  "human: hi",
		"root_outputs_preview": "the answer",
		"groups": []any{
			map[string]any{
				"type":      "tool_interaction",
				"aiMessage": map[string]any{"role": "ai", "content": nil}, // null content
				"toolCalls": []any{
					map[string]any{"name": "read_url", "result": map[string]any{"content": nil}}, // null result
				},
			},
		},
	}

	traj := buildTraceTrajectory(trace)

	// No human msg and no non-empty ai content in groups → fall back to previews.
	if traj.InputMessage != "human: hi" {
		t.Errorf("InputMessage = %q, want preview fallback", traj.InputMessage)
	}
	if traj.OutputMessage != "the answer" {
		t.Errorf("OutputMessage = %q, want preview fallback", traj.OutputMessage)
	}
	tool := traj.Steps[len(traj.Steps)-1]
	if tool.ToolResult != "" || tool.Errorish {
		t.Errorf("null result should yield empty tool_result and errorish=false, got %+v", tool)
	}
}

// Long content is bounded to the trajectory caps.
func TestBuildTraceTrajectory_Truncation(t *testing.T) {
	big := strings.Repeat("x", 5000)
	trace := map[string]any{
		"trace_id": "t3",
		"groups": []any{
			map[string]any{"type": "message", "message": map[string]any{"role": "human", "content": big}},
			map[string]any{
				"type":      "tool_interaction",
				"aiMessage": map[string]any{"role": "ai", "content": "ok"},
				"toolCalls": []any{
					map[string]any{"name": "f", "result": map[string]any{"content": big}},
				},
			},
		},
	}

	traj := buildTraceTrajectory(trace)
	if got := len(traj.Steps[0].Text); got != trajTextMax {
		t.Errorf("human step Text len = %d, want %d", got, trajTextMax)
	}
	if got := len(traj.Steps[2].ToolResult); got != trajResultMax {
		t.Errorf("tool_result len = %d, want %d", got, trajResultMax)
	}
}
