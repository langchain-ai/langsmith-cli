package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// ==================== Command structure ====================

// trace messages rejects invocations without --since/--last-n-minutes, so every
// example in its help text must carry one. Three examples did not, which is
// where callers copying the help got commands that always exit 1.
func TestTraceMessagesCmd_HelpExamplesHaveStartTime(t *testing.T) {
	cmd := newTraceMessagesCmd()
	var examples int
	for _, line := range strings.Split(cmd.Long, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "langsmith trace messages") {
			continue
		}
		examples++
		if !strings.Contains(line, "--since") && !strings.Contains(line, "--last-n-minutes") {
			t.Errorf("help example lacks a start time (always exits 1): %q", line)
		}
	}
	if examples == 0 {
		t.Fatal("no help examples found; the assertion above would vacuously pass")
	}
}

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
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-123", "name": "my-project"},
			})
		case r.URL.Path == "/api/v2/traces/messages" && r.Method == "POST":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)

			// Verify required fields
			projectIDs, _ := body["project_ids"].([]any)
			if len(projectIDs) != 1 || projectIDs[0] != "sess-123" {
				t.Errorf("expected project_ids [sess-123], got %v", projectIDs)
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
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
			})
		case r.URL.Path == "/api/v2/runs/query" && r.Method == "POST":
			// attachRootIO always fetches root run previews (v2 backend here)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "my-project", "--limit", "5", "--since", "2024-01-01T00:00:00Z"})
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
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-456", "name": "test-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{},
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
			"--since", "2024-01-01T00:00:00Z",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// min_start_time is a required request param; verify it is forwarded.
	if receivedBody["min_start_time"] != "2024-01-01T00:00:00Z" {
		t.Errorf("expected min_start_time=2024-01-01T00:00:00Z, got %v", receivedBody["min_start_time"])
	}
	if receivedBody["run_type"] != "llm" {
		t.Errorf("expected run_type=llm, got %v", receivedBody["run_type"])
	}
	if receivedBody["has_error"] != true {
		t.Errorf("expected has_error=true, got %v", receivedBody["has_error"])
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
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-pretty", "name": "my-project"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
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
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "pretty"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "my-project", "--limit", "2", "--since", "2024-01-01T00:00:00Z"})
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
	}
	for _, s := range expects {
		if !strings.Contains(out, s) {
			t.Errorf("pretty output missing %q\nfull output:\n%s", s, out)
		}
	}
}

func TestTraceMessages_Pagination(t *testing.T) {
	pageCount := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-pag", "name": "pag-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			pageCount++

			// Verify page size is <= 10
			pageSize, _ := body["page_size"].(float64)
			if int(pageSize) > 10 {
				t.Errorf("page %d: page_size sent to API was %d, expected <= 10", pageCount, int(pageSize))
			}

			w.Header().Set("Content-Type", "application/json")
			switch pageCount {
			case 1:
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{
						{"trace_id": "trace-1", "groups": []any{}},
						{"trace_id": "trace-2", "groups": []any{}},
						{"trace_id": "trace-3", "groups": []any{}},
						{"trace_id": "trace-4", "groups": []any{}},
						{"trace_id": "trace-5", "groups": []any{}},
						{"trace_id": "trace-6", "groups": []any{}},
						{"trace_id": "trace-7", "groups": []any{}},
						{"trace_id": "trace-8", "groups": []any{}},
						{"trace_id": "trace-9", "groups": []any{}},
						{"trace_id": "trace-10", "groups": []any{}},
					},
					"next_cursor": "cursor-page2",
				})
			case 2:
				// Verify cursor was passed
				if body["cursor"] != "cursor-page2" {
					t.Errorf("page 2: expected cursor=cursor-page2, got %v", body["cursor"])
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []map[string]any{
						{"trace_id": "trace-11", "groups": []any{}},
						{"trace_id": "trace-12", "groups": []any{}},
						{"trace_id": "trace-13", "groups": []any{}},
						{"trace_id": "trace-14", "groups": []any{}},
						{"trace_id": "trace-15", "groups": []any{}},
					},
				})
			default:
				t.Errorf("unexpected page %d", pageCount)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"items": []any{},
				})
			}
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "pag-proj", "--limit", "15", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	traces, _ := result["traces"].([]any)
	if len(traces) != 15 {
		t.Errorf("expected 15 traces, got %d", len(traces))
	}
	if pageCount != 2 {
		t.Errorf("expected 2 API calls, got %d", pageCount)
	}
}

func TestTraceMessages_PaginationStopsAtLimit(t *testing.T) {
	pageCount := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-lim", "name": "lim-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			pageCount++

			w.Header().Set("Content-Type", "application/json")
			// Page 1: return traces up to page_size with a next cursor.
			// The user only asked for 5, so the CLI should request page_size=5
			// and not make a second call.
			pageSize, _ := body["page_size"].(float64)
			traces := make([]map[string]any, int(pageSize))
			for i := range traces {
				traces[i] = map[string]any{
					"trace_id": fmt.Sprintf("trace-%d", i+1),
					"groups":   []any{},
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":       traces,
				"next_cursor": "cursor-more",
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "lim-proj", "--limit", "5", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	traces, _ := result["traces"].([]any)
	if len(traces) != 5 {
		t.Errorf("expected 5 traces, got %d", len(traces))
	}
	if pageCount != 1 {
		t.Errorf("expected 1 API call (limit < page size), got %d", pageCount)
	}
}

func TestTraceMessages_TraceIDsBoundPaginationByRequestedPages(t *testing.T) {
	pageCount := 0
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/traces/messages":
			pageCount++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"trace_id": "trace-1", "groups": []any{}},
					{"trace_id": "trace-2", "groups": []any{}},
					{"trace_id": "trace-3", "groups": []any{}},
					{"trace_id": "trace-4", "groups": []any{}},
				},
				"next_cursor": "cursor-over-root-scan",
			})
		case r.URL.Path == "/api/v2/runs/query":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{
			"--project-id", "11111111-1111-1111-1111-111111111111",
			"--trace-ids", "trace-1,trace-2,trace-3,trace-4,missing-trace",
			"--limit", "100",
			"--since", "2024-01-01T00:00:00Z",
		})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if pageCount != 1 {
		t.Fatalf("trace messages requests = %d, want 1", pageCount)
	}
}

func TestTraceMessages_CursorFlag_SinglePage(t *testing.T) {
	callCount := 0
	var receivedBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-cur", "name": "cur-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			callCount++
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"trace_id": "trace-A", "groups": []any{}},
					{"trace_id": "trace-B", "groups": []any{}},
				},
				"next_cursor": "cursor-next-page",
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "cur-proj", "--limit", "20", "--cursor", "cursor-abc", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Only one API call despite there being more pages (single-page mode)
	if callCount != 1 {
		t.Errorf("expected exactly 1 API call in cursor mode, got %d", callCount)
	}
	// Cursor was forwarded to the API
	if receivedBody["cursor"] != "cursor-abc" {
		t.Errorf("expected cursor=cursor-abc in request, got %v", receivedBody["cursor"])
	}
	// cursors.next is present in output so caller can continue paginating
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	cursors, _ := result["cursors"].(map[string]any)
	if cursors["next"] != "cursor-next-page" {
		t.Errorf("expected cursors.next=cursor-next-page in output, got %v", cursors["next"])
	}
	traces, _ := result["traces"].([]any)
	if len(traces) != 2 {
		t.Errorf("expected 2 traces, got %d", len(traces))
	}
}

func TestTraceMessages_CursorFlag_EmptyCursorIsFirstPage(t *testing.T) {
	callCount := 0
	var receivedBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-first", "name": "first-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			callCount++
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":       []map[string]any{{"trace_id": "trace-1", "groups": []any{}}},
				"next_cursor": "cursor-page2",
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		// --cursor "" means first page in single-page mode
		cmd.SetArgs([]string{"--project", "first-proj", "--limit", "20", "--cursor", "", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}
	// Empty cursor should NOT be forwarded to the API
	if _, ok := receivedBody["cursor"]; ok {
		t.Errorf("empty --cursor should not send cursor field to API, got %v", receivedBody["cursor"])
	}
	// cursors.next is still exposed
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	cursors, _ := result["cursors"].(map[string]any)
	if cursors["next"] != "cursor-page2" {
		t.Errorf("expected cursors.next=cursor-page2, got %v", cursors["next"])
	}
}

func TestTraceMessages_BeforeFlag(t *testing.T) {
	var receivedBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-bef", "name": "bef-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{},
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "bef-proj", "--before", "2024-01-15T00:00:00Z", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["max_start_time"] != "2024-01-15T00:00:00Z" {
		t.Errorf("expected max_start_time=2024-01-15T00:00:00Z, got %v", receivedBody["max_start_time"])
	}
}

// TestTraceMessages_FeedbackStats verifies that feedback_stats returned by the
// /v2/traces/messages API is preserved in the CLI output without being
// overwritten or dropped by attachRootIO.
func TestTraceMessages_FeedbackStats(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-fb", "name": "fb-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages" && r.Method == "POST":
			// API returns feedback_stats directly on each trace
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"trace_id": "trace-with-feedback",
						"groups":   []any{},
						"feedback_stats": map[string]any{
							"thumbs_up": map[string]any{"n": 1, "avg": 0},
						},
					},
					{
						"trace_id":       "trace-no-feedback",
						"groups":         []any{},
						"feedback_stats": map[string]any{},
					},
				},
				"next_cursor": "",
			})
		case r.URL.Path == "/api/v2/runs/query" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	})
	t.Setenv("LANGSMITH_ENDPOINT", ts.URL)
	t.Setenv("LANGSMITH_API_KEY", "test-api-key")
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "fb-proj", "--limit", "20", "--since", "2024-01-01T00:00:00Z"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	traces, _ := result["traces"].([]any)
	if len(traces) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(traces))
	}

	// feedback_stats from the API must be preserved as-is
	traceWith, _ := traces[0].(map[string]any)
	fs, _ := traceWith["feedback_stats"].(map[string]any)
	if len(fs) == 0 {
		t.Errorf("expected non-empty feedback_stats for trace-with-feedback, got %v", fs)
	}

	traceWithout, _ := traces[1].(map[string]any)
	fsEmpty, _ := traceWithout["feedback_stats"].(map[string]any)
	if len(fsEmpty) != 0 {
		t.Errorf("expected empty feedback_stats for trace-no-feedback, got %v", fsEmpty)
	}
}

func TestTraceMessages_EmptyResult(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-789", "name": "empty-proj"},
			})
		case r.URL.Path == "/api/v2/traces/messages":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []any{},
			})
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceMessagesCmd()
		cmd.SetArgs([]string{"--project", "empty-proj", "--since", "2024-01-01T00:00:00Z"})
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
