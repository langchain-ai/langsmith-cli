package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRunListAndTraceListHaveAISearchFlag(t *testing.T) {
	if newRunListCmd().Flags().Lookup("ai-search") == nil {
		t.Error("run list missing --ai-search")
	}
	if newTraceListCmd().Flags().Lookup("ai-search") == nil {
		t.Error("trace list missing --ai-search")
	}
}

func TestRunListCmd_AISearchReadsEventStream(t *testing.T) {
	const stream = "event: data\n" +
		"data: [{\"op\":\"add\",\"path\":\"\",\"value\":{\"items\":[{\"id\":\"run-1\",\"name\":\"lookup\",\"run_type\":\"chain\"}],\"next_cursor\":null}}]\n" +
		"\n" +
		"event: data\n" +
		"data: [{\"op\":\"add\",\"path\":\"/items/-\",\"value\":{\"id\":\"run-2\",\"name\":\"refund\",\"run_type\":\"llm\"}}]\n" +
		"\n" +
		"event: end\n" +
		"data: \n" +
		"\n"

	var accept, search string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "session-123", "name": "my-app"}})
		case r.URL.Path == "/api/v2/runs/query" && r.Method == http.MethodPost:
			accept = r.Header.Get("Accept")
			body, _ := io.ReadAll(r.Body)
			search = string(body)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, stream)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	cmd := newRunListCmd()
	_ = cmd.Flags().Set("project", "my-app")
	_ = cmd.Flags().Set("ai-search", "a refund")

	out := captureStdout(t, func() { cmd.Run(cmd, nil) })

	if accept != "text/event-stream" {
		t.Fatalf("Accept = %q", accept)
	}
	if !strings.Contains(search, `"ai_search":"a refund"`) {
		t.Fatalf("body missing ai_search: %s", search)
	}
	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 2 || result[1]["name"] != "refund" {
		t.Fatalf("runs = %#v", result)
	}
}

func TestCollectSSEPatchesRequiresEndEvent(t *testing.T) {
	const stream = "event: data\n" +
		"data: [{\"op\":\"add\",\"path\":\"\",\"value\":{\"items\":[{\"id\":\"run-1\"}],\"next_cursor\":null}}]\n" +
		"\n"
	_, err := collectSSEPatches(strings.NewReader(stream))
	if err == nil || !strings.Contains(err.Error(), "end event") {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, errSSEEnd) {
		t.Fatal("end sentinel leaked")
	}
}
