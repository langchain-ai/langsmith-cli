package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// ==================== Command structure ====================

func TestThreadCmd_Subcommands(t *testing.T) {
	cmd := newThreadCmd()
	expected := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("thread missing subcommand %q", name)
		}
	}
}

func TestThreadCmd_UseField(t *testing.T) {
	cmd := newThreadCmd()
	if cmd.Use != "thread" {
		t.Errorf("expected Use=thread, got %q", cmd.Use)
	}
}

// ==================== thread list flags ====================

func TestThreadListCmd_Flags(t *testing.T) {
	cmd := newThreadListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"limit", "20", "n"},
		{"filter", "", ""},
		{"last-n-minutes", "0", ""},
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

func TestThreadListCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newThreadListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	// project should NOT be marked as cobra-required so that
	// ResolveProject can fall back to LANGSMITH_PROJECT env var
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestThreadListCmd_ProjectEnvFallback(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "env-project")
	result := ResolveProject("")
	if result != "env-project" {
		t.Errorf("expected ResolveProject to return env-project, got %q", result)
	}
}

func TestThreadListCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newThreadListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

// ==================== thread get flags ====================

func TestThreadGetCmd_Flags(t *testing.T) {
	cmd := newThreadGetCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"include-metadata", "false", ""},
		{"include-io", "false", ""},
		{"full", "false", ""},
		{"limit", "0", "n"},
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

func TestThreadGetCmd_ExactArgs(t *testing.T) {
	cmd := newThreadGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"thread-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestThreadGetCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newThreadGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	// project should NOT be marked as cobra-required so that
	// ResolveProject can fall back to LANGSMITH_PROJECT env var
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestThreadGetCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newThreadGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

func TestBlankThreadTurnPreviewRequest(t *testing.T) {
	groups := []any{
		threadTestBoundary("trace-1", 0, "2026-07-20T12:00:00Z"),
		threadTestBoundary("trace-2", 1, "2026-07-20T12:01:00Z"),
		threadTestBoundary("trace-3", 2, "2026-07-20T12:02:00Z"),
		map[string]any{
			"type": "message",
			"message": map[string]any{
				"role":    "human",
				"content": "streamed question",
			},
		},
		threadTestBoundary("trace-4", 3, "2026-07-20T12:03:00Z"),
		threadTestBoundary("trace-5", 4, "2026-07-20T12:04:00Z"),
	}

	traceIDs, minStartTime := blankThreadTurnPreviewRequest(groups)
	if got, want := strings.Join(traceIDs, ","), "trace-1,trace-2,trace-4,trace-5"; got != want {
		t.Fatalf("blank trace IDs = %q, want %q", got, want)
	}
	if got, want := minStartTime.Format(time.RFC3339), "2026-07-20T12:00:00Z"; got != want {
		t.Fatalf("minimum start time = %q, want %q", got, want)
	}
}

func TestFillBlankThreadTurnsAddsMarkedPreviewsOnlyToBlankTurns(t *testing.T) {
	groups := []any{
		threadTestBoundary("trace-1", 0, "2026-07-20T12:00:00Z"),
		threadTestBoundary("trace-2", 1, "2026-07-20T12:01:00Z"),
		map[string]any{
			"type": "message",
			"message": map[string]any{
				"role":    "ai",
				"content": "streamed answer",
			},
		},
	}
	previews := map[string]rootPreview{
		"trace-1": {inputs: "preview question", outputs: "preview answer"},
		"trace-2": {inputs: "must not be inserted", outputs: "must not be inserted"},
	}

	filled := fillBlankThreadTurns(groups, previews)
	if got, want := len(filled), 5; got != want {
		t.Fatalf("group count = %d, want %d", got, want)
	}

	human := filled[1].(map[string]any)
	ai := filled[2].(map[string]any)
	for _, group := range []map[string]any{human, ai} {
		metadata, _ := group["metadata"].(map[string]any)
		if metadata["source"] != "root_io_preview" || metadata["trace_id"] != "trace-1" {
			t.Fatalf("fallback metadata = %v", metadata)
		}
	}
	if role := human["message"].(map[string]any)["role"]; role != "human" {
		t.Fatalf("input preview role = %v, want human", role)
	}
	if role := ai["message"].(map[string]any)["role"]; role != "ai" {
		t.Fatalf("output preview role = %v, want ai", role)
	}
	for _, group := range filled {
		if strings.Contains(fmt.Sprint(group), "must not be inserted") {
			t.Fatal("populated turn received duplicate root previews")
		}
	}
}

func TestParseSSEGroupsSupportsLegacyAndProblemDetailErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "legacy", data: `{"error":"turn could not be parsed"}`, want: "turn could not be parsed"},
		{name: "problem details", data: `{"title":"Bad Request","status":400,"detail":"unsupported trace format"}`, want: "unsupported trace format"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := []byte("event: error\ndata: " + test.data + "\n\n")
			_, _, _, got := parseSSEGroups(body)
			if got != test.want {
				t.Fatalf("SSE error = %q, want %q", got, test.want)
			}
		})
	}
}

func TestThreadMessagesHTTPErrorDoesNotEchoUnknownBody(t *testing.T) {
	if got := threadMessagesHTTPError(http.StatusUnauthorized, []byte("opaque upstream body")); got != "Unauthorized" {
		t.Fatalf("HTTP error = %q, want Unauthorized", got)
	}
	if got := threadMessagesHTTPError(
		http.StatusBadRequest,
		[]byte(`{"detail":"invalid thread cursor"}`),
	); got != "invalid thread cursor" {
		t.Fatalf("problem detail = %q, want invalid thread cursor", got)
	}
}

func TestThreadMessagesCmdFillsBlankTurnsFromScopedRootPreviews(t *testing.T) {
	var previewQuery map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "session-123", "name": "test-project"},
			})
		case r.URL.Path == "/v2/threads/thread-123/messages" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("project_id"); got != "session-123" {
				t.Errorf("project_id = %q, want session-123", got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w,
				"event: data\ndata: {\"type\":\"turn_boundary\",\"turnBoundary\":{\"trace_id\":\"trace-blank\",\"turn_index\":0,\"start_time\":\"2026-07-20T12:00:00Z\"}}\n\n"+
					"event: data\ndata: {\"type\":\"turn_boundary\",\"turnBoundary\":{\"trace_id\":\"trace-populated\",\"turn_index\":1,\"start_time\":\"2026-07-20T12:01:00Z\"}}\n\n"+
					"event: data\ndata: {\"type\":\"message\",\"message\":{\"role\":\"ai\",\"content\":\"streamed answer\"}}\n\n"+
					"event: metadata\ndata: {\"next_cursor\":null,\"prev_cursor\":null}\n\n",
			)
		case r.URL.Path == "/api/v1/runs/query" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&previewQuery); err != nil {
				t.Fatalf("decode preview query: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"runs": []map[string]any{
					{
						"id":              "trace-blank",
						"trace_id":        "trace-blank",
						"name":            "root",
						"run_type":        "chain",
						"start_time":      "2026-07-20T12:00:00Z",
						"inputs_preview":  "preview question",
						"outputs_preview": "preview answer",
					},
				},
				"cursors": map[string]any{},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newThreadMessagesCmd()
		cmd.SetArgs([]string{"thread-123", "--project", "test-project"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("thread messages: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	groups, _ := result["groups"].([]any)
	if got, want := len(groups), 5; got != want {
		t.Fatalf("group count = %d, want %d: %s", got, want, out)
	}
	if got := groups[1].(map[string]any)["message"].(map[string]any)["content"]; got != "preview question" {
		t.Fatalf("fallback input = %v", got)
	}
	if got := groups[2].(map[string]any)["message"].(map[string]any)["content"]; got != "preview answer" {
		t.Fatalf("fallback output = %v", got)
	}
	if got := groups[4].(map[string]any)["message"].(map[string]any)["content"]; got != "streamed answer" {
		t.Fatalf("streamed content changed: %v", got)
	}

	if got := fmt.Sprint(previewQuery["session"]); got != "[session-123]" {
		t.Fatalf("preview session scope = %v", previewQuery["session"])
	}
	if got := fmt.Sprint(previewQuery["id"]); got != "[trace-blank]" {
		t.Fatalf("preview IDs = %v", previewQuery["id"])
	}
	if got := previewQuery["is_root"]; got != true {
		t.Fatalf("preview is_root = %v, want true", got)
	}
}

func TestPrintThreadMessagesLabelsRootPreviews(t *testing.T) {
	result := map[string]any{
		"thread_id": "thread-123",
		"groups": []any{
			threadTestBoundary("trace-1", 0, "2026-07-20T12:00:00Z"),
			rootPreviewMessageGroup("trace-1", "human", "preview question"),
		},
		"cursors": map[string]any{},
	}
	out := captureStdout(t, func() { printThreadMessages(result) })
	if !strings.Contains(out, "[human, root preview] preview question") {
		t.Fatalf("pretty output did not label fallback content:\n%s", out)
	}
}

func threadTestBoundary(traceID string, turnIndex int, startTime string) map[string]any {
	return map[string]any{
		"type": "turn_boundary",
		"turnBoundary": map[string]any{
			"trace_id":   traceID,
			"turn_index": float64(turnIndex),
			"start_time": startTime,
		},
	}
}
