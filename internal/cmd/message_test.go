package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// ==================== Command structure ====================

func TestTraceMessagesCmd_UseField(t *testing.T) {
	cmd := newTraceMessagesCmd()
	if cmd.Use != "messages" {
		t.Errorf("expected Use=messages, got %q", cmd.Use)
	}
}

// ==================== trace messages flags ====================

func TestTraceMessagesCmd_Flags(t *testing.T) {
	cmd := newTraceMessagesCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestTraceMessagesCmd_HasCommonFilterFlags(t *testing.T) {
	cmd := newTraceMessagesCmd()
	common := []string{"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency", "min-tokens", "tags", "filter"}
	for _, name := range common {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("trace messages missing common filter flag --%s", name)
		}
	}
}

func TestTraceMessagesCmd_HasRunTypeFlag(t *testing.T) {
	cmd := newTraceMessagesCmd()
	if cmd.Flags().Lookup("run-type") == nil {
		t.Error("trace messages should have --run-type flag")
	}
}

// ==================== integration-style tests with mock server ====================

func TestTraceMessages_Success(t *testing.T) {
	callCount := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-123", "name": "my-project"},
			})
		case r.URL.Path == "/api/v1/v2/traces/messages" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)

			// Verify required fields
			session, _ := body["session"].([]any)
			if len(session) != 1 || session[0] != "sess-123" {
				t.Errorf("expected session [sess-123], got %v", session)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"traces": []map[string]any{
					{
						"trace_id": "trace-1",
						"groups": []map[string]any{
							{
								"type": "message",
								"message": map[string]any{
									"role":    "human",
									"content": "hello",
								},
							},
						},
					},
				},
				"cursors": map[string]string{},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "my-project", "--limit", "5"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}

	traces, _ := result["traces"].([]any)
	if len(traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traces))
	}
	trace, _ := traces[0].(map[string]any)
	if trace["trace_id"] != "trace-1" {
		t.Errorf("expected trace_id=trace-1, got %v", trace["trace_id"])
	}
}

func TestTraceMessages_PassesFilterAndRunType(t *testing.T) {
	var receivedBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-456", "name": "test-proj"},
			})
		case r.URL.Path == "/api/v1/v2/traces/messages":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"traces":  []any{},
				"cursors": map[string]string{},
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{
			"--project", "test-proj",
			"--run-type", "llm",
			"--error",
			"--filter", "gte(latency, 5)",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["run_type"] != "llm" {
		t.Errorf("expected run_type=llm, got %v", receivedBody["run_type"])
	}
	if receivedBody["error"] != true {
		t.Errorf("expected error=true, got %v", receivedBody["error"])
	}
	if receivedBody["filter"] != "gte(latency, 5)" {
		t.Errorf("expected filter passthrough, got %v", receivedBody["filter"])
	}
}

func TestTraceMessages_PrettyFormat(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-pretty", "name": "my-project"},
			})
		case r.URL.Path == "/api/v1/v2/traces/messages":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"traces": []map[string]any{
					{
						"trace_id": "trace-aaa",
						"groups": []map[string]any{
							{
								"type": "message",
								"message": map[string]any{
									"role":    "human",
									"content": "What is the weather?",
								},
							},
							{
								"type": "tool_interaction",
								"aiMessage": map[string]any{
									"role": "ai",
									"content": []map[string]any{
										{"type": "text", "text": "Let me check the weather."},
										{"type": "tool_call", "name": "get_weather"},
									},
								},
								"toolCalls": []map[string]any{
									{
										"id":   "tc-1",
										"name": "get_weather",
										"args": map[string]any{"city": "SF"},
										"result": map[string]any{
											"role":    "tool",
											"content": "72°F and sunny",
										},
									},
								},
							},
							{
								"type": "message",
								"message": map[string]any{
									"role":    "ai",
									"content": "The weather in SF is 72°F and sunny!",
								},
							},
						},
					},
					{
						"trace_id": "trace-bbb",
						"groups": []map[string]any{
							{
								"type": "message",
								"message": map[string]any{
									"role":    "human",
									"content": "Hello",
								},
							},
						},
					},
				},
				"cursors": map[string]any{"next": "cursor-xyz"},
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "pretty"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "my-project"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify pretty output contains expected elements
	expects := []string{
		"--- Trace trace-aaa (3 groups) ---",
		"[human] What is the weather?",
		"[ai] Let me check the weather.",
		"[ai] tool_call: get_weather",
		"[tool_call] get_weather",
		"[tool] 72°F and sunny",
		"[ai] The weather in SF is 72°F and sunny!",
		"--- Trace trace-bbb (1 groups) ---",
		"[human] Hello",
		"Next cursor: cursor-xyz",
	}
	for _, s := range expects {
		if !strings.Contains(out, s) {
			t.Errorf("pretty output missing %q\nfull output:\n%s", s, out)
		}
	}
}

func TestTraceMessages_EmptyResult(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-789", "name": "empty-proj"},
			})
		case r.URL.Path == "/api/v1/v2/traces/messages":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"traces":  []any{},
				"cursors": map[string]string{},
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "empty-proj"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	traces, _ := result["traces"].([]any)
	if len(traces) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traces))
	}
}
