package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const testProjectID = "018e4c7e-a9fb-7ef0-a5b6-6ea3a82e9327"

// runTraceCmdJSON executes a trace subcommand with JSON output and decodes it.
func runTraceCmdJSON(t *testing.T, ts string, args ...string) any {
	t.Helper()
	cleanup := setupTestEnv(t, ts)
	defer cleanup()
	flagOutputFormat = "json"

	cmd := newTraceCmd()
	cmd.SetArgs(args)
	out := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	var got any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decoding output %q: %v", out, err)
	}
	return got
}

func TestTraceList_V2SelectsByTraceTotals(t *testing.T) {
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/traces/query" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{
			"root_run":{"id":"t1","trace_id":"t1","name":"agent","prompt_tokens":0,"total_tokens":0},
			"trace_aggregates":{"total_tokens":700,"total_cost":0.25}
		}]}`))
	})

	got := runTraceCmdJSON(t, ts.URL, "list", "--project-id", testProjectID,
		"--name", "agent", "--error", "--min-tokens", "500", "--trace-ids", "t1", "--include-metadata")

	if body["project_id"] != testProjectID {
		t.Errorf("project_id = %v", body["project_id"])
	}
	if want := `and(and(eq(name, "agent"), eq(status, "error")), gte(total_tokens, 500))`; body["trace_filter"] != want {
		t.Errorf("trace_filter = %v, want %s", body["trace_filter"], want)
	}
	if ids, _ := body["trace_ids"].([]any); len(ids) != 1 || ids[0] != "t1" {
		t.Errorf("trace_ids = %v, want [t1]", body["trace_ids"])
	}

	rows := got.([]any)
	if len(rows) != 1 {
		t.Fatalf("rows = %v", rows)
	}
	row := rows[0].(map[string]any)
	usage := row["token_usage"].(map[string]any)
	if usage["total_tokens"] != float64(700) || usage["prompt_tokens"] != nil {
		t.Errorf("token_usage = %v, want only the trace total", usage)
	}
	if costs := row["costs"].(map[string]any); costs["total_cost"] != 0.25 {
		t.Errorf("costs = %v", costs)
	}
}

func TestTraceGet_V2ReadsWholeTraceOldestFirst(t *testing.T) {
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	var query map[string][]string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/traces/"+traceID+"/runs" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"c","start_time":"2026-01-01T00:00:03Z"},
			{"id":"a","start_time":"2026-01-01T00:00:01Z"},
			{"id":"b","start_time":"2026-01-01T00:00:02Z"}
		]}`))
	})

	got := runTraceCmdJSON(t, ts.URL, "get", traceID, "--project-id", testProjectID).(map[string]any)

	if query["project_id"][0] != testProjectID {
		t.Errorf("project_id = %v", query["project_id"])
	}
	if _, ok := query["min_start_time"]; ok {
		t.Errorf("sent a start-time window without --since: %v", query)
	}
	var ids []string
	for _, r := range got["runs"].([]any) {
		ids = append(ids, r.(map[string]any)["run_id"].(string))
	}
	if strings.Join(ids, ",") != "a,b,c" {
		t.Errorf("run order = %v, want oldest first", ids)
	}
}

func TestTraceGet_V2SinceSendsBothBounds(t *testing.T) {
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	var query map[string][]string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	})

	runTraceCmdJSON(t, ts.URL, "get", traceID, "--project-id", testProjectID, "--since", "2026-01-01T00:00:00Z")

	if got := query["min_start_time"]; len(got) != 1 || got[0] != "2026-01-01T00:00:00Z" {
		t.Errorf("min_start_time = %v", got)
	}
	if len(query["max_start_time"]) != 1 {
		t.Errorf("max_start_time = %v, want it set alongside min_start_time", query["max_start_time"])
	}
}

func TestTraceList_FeedbackWindowOnTracesQuery(t *testing.T) {
	for _, version := range []string{"dev", "0.18.15", "0.19.0"} {
		t.Run(version, func(t *testing.T) {
			withDeploymentVersion(t, version)
			var paths []string
			var body map[string]any
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"items":[{"root_run":{"trace_id":"t1"},"trace_aggregates":{"total_tokens":700}}]}`))
			})

			runTraceCmdJSON(t, ts.URL, "list", "--project-id", testProjectID, "--min-tokens", "500",
				"--filter", `eq(feedback_key, "correctness")`,
				"--feedback-since", "2026-01-01", "--feedback-before", "2026-01-02")

			if strings.Join(paths, ",") != "/api/v2/traces/query" {
				t.Errorf("requests = %v, want one /api/v2/traces/query", paths)
			}
			if body["min_feedback_modified_at"] != "2026-01-01T00:00:00Z" || body["max_feedback_modified_at"] != "2026-01-02T00:00:00Z" {
				t.Errorf("feedback window = %v / %v", body["min_feedback_modified_at"], body["max_feedback_modified_at"])
			}
			if f, _ := body["trace_filter"].(string); !strings.Contains(f, "gte(total_tokens, 500)") {
				t.Errorf("trace_filter = %q, want the --min-tokens bound", f)
			}
		})
	}
}

func TestTraceList_FeedbackWindowFallbackAddsTraceTotals(t *testing.T) {
	withDeploymentVersion(t, "0.18.14")
	var paths []string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/runs/query":
			_, _ = w.Write([]byte(`{"items":[{"id":"t1","trace_id":"t1","start_time":"2026-01-01T00:00:00Z"}]}`))
		case "/api/v2/traces/query":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if ids, _ := body["trace_ids"].([]any); len(ids) != 1 || ids[0] != "t1" {
				t.Errorf("trace_ids = %v, want [t1]", body["trace_ids"])
			}
			_, _ = w.Write([]byte(`{"items":[{"root_run":{"trace_id":"t1"},"trace_aggregates":{"total_tokens":42}}]}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	})

	got := runTraceCmdJSON(t, ts.URL, "list", "--project-id", testProjectID, "--include-metadata",
		"--filter", `eq(feedback_key, "correctness")`, "--feedback-since", "2026-01-01")

	if strings.Join(paths, ",") != "/api/v2/runs/query,/api/v2/traces/query" {
		t.Errorf("requests = %v", paths)
	}
	row := got.([]any)[0].(map[string]any)
	if usage := row["token_usage"].(map[string]any); usage["total_tokens"] != float64(42) {
		t.Errorf("token_usage = %v, want the trace total", usage)
	}
}

func TestTraceCommands_V1KeepsRunsQuery(t *testing.T) {
	withDeploymentVersion(t, "0.15.0")
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	for name, args := range map[string][]string{
		"list": {"list", "--project-id", testProjectID, "--min-tokens", "500"},
		"get":  {"get", traceID, "--project-id", testProjectID},
	} {
		t.Run(name, func(t *testing.T) {
			var paths []string
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"runs":[{"id":"r1","trace_id":"r1","total_tokens":700}],"cursors":{}}`))
			})
			runTraceCmdJSON(t, ts.URL, args...)
			if len(paths) != 1 || paths[0] != "/api/v1/runs/query" {
				t.Errorf("requests = %v, want one /api/v1/runs/query", paths)
			}
		})
	}
}
