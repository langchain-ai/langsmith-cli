package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

func TestTraceExportPathUsesTraceID(t *testing.T) {
	first := langsmith.RunSchema{TraceID: "trace-1", Name: "same"}
	second := langsmith.RunSchema{TraceID: "trace-2", Name: "same"}
	if got := traceExportPath("out", "{trace_id}.jsonl", first); got == traceExportPath("out", "{trace_id}.jsonl", second) {
		t.Fatal("trace_id pattern should produce unique paths")
	}
}

func TestTraceExportPathCollidesForConstantAndNamePatterns(t *testing.T) {
	first := langsmith.RunSchema{TraceID: "trace-1", Name: "same"}
	second := langsmith.RunSchema{TraceID: "trace-2", Name: "same"}
	for _, pattern := range []string{"trace.jsonl", "{name}.jsonl"} {
		if traceExportPath("out", pattern, first) != traceExportPath("out", pattern, second) {
			t.Errorf("pattern %q should collide for equal names", pattern)
		}
	}
}

// ==================== Command structure ====================

func TestTraceCmd_Subcommands(t *testing.T) {
	cmd := newTraceCmd()
	expected := map[string]bool{"list": false, "get": false, "export": false, "messages": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("trace missing subcommand %q", name)
		}
	}
}

func TestTraceCmd_UseField(t *testing.T) {
	cmd := newTraceCmd()
	if cmd.Use != "traces" {
		t.Errorf("expected Use=traces, got %q", cmd.Use)
	}
}

// ==================== trace list flags ====================

func TestTraceListCmd_Flags(t *testing.T) {
	cmd := newTraceListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"include-metadata", "false", ""},
		{"include-io", "false", ""},
		{"full", "false", ""},
		{"show-hierarchy", "false", ""},
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

func TestTraceListCmd_HasCommonFilterFlags(t *testing.T) {
	cmd := newTraceListCmd()
	common := []string{"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency", "min-tokens", "tags", "filter"}
	for _, name := range common {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("trace list missing common filter flag --%s", name)
		}
	}
}

func TestTraceListCmd_NoRunTypeFlag(t *testing.T) {
	cmd := newTraceListCmd()
	if cmd.Flags().Lookup("run-type") != nil {
		t.Error("trace list should not have --run-type flag")
	}
}

// ==================== trace get flags ====================

func TestTraceGetCmd_Flags(t *testing.T) {
	cmd := newTraceGetCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"project", ""},
		{"include-metadata", "false"},
		{"include-io", "false"},
		{"full", "false"},
		{"output", ""},
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
	}
}

func TestTraceGetCmd_OutputShorthand(t *testing.T) {
	cmd := newTraceGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestTraceGetCmd_TimeFlags(t *testing.T) {
	cmd := newTraceGetCmd()
	for _, name := range []string{"since", "last-n-minutes"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("trace get missing flag --%s", name)
		}
	}
}

func TestTraceGet_V1WithoutProjectQueriesByTraceOnly(t *testing.T) {
	withDeploymentVersion(t, "0.15.0")
	t.Setenv("LANGSMITH_PROJECT", "")
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	var paths []string
	var body map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"runs":[{"id":"r1","trace_id":"` + traceID + `"}],"cursors":{}}`))
	})

	got := runTraceCmdJSON(t, ts.URL, "get", traceID).(map[string]any)

	if len(paths) != 1 || paths[0] != "/api/v1/runs/query" {
		t.Fatalf("requests = %v, want one /api/v1/runs/query", paths)
	}
	if body["trace"] != traceID {
		t.Errorf("trace = %v, want %s", body["trace"], traceID)
	}
	if _, ok := body["session"]; ok {
		t.Errorf("sent a session filter without --project: %v", body["session"])
	}
	if _, ok := body["start_time"]; !ok {
		t.Error("start_time missing: the v1 lookup should keep its default window")
	}
	if got["run_count"] != float64(1) {
		t.Errorf("run_count = %v, want 1", got["run_count"])
	}
}

func TestTraceGet_V2WithoutProjectFindsProjectByTrace(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "")
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	var paths []string
	var lookup map[string]any
	var query map[string][]string
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/runs/query":
			_ = json.NewDecoder(r.Body).Decode(&lookup)
			_, _ = w.Write([]byte(`{"runs":[{"id":"r1","trace_id":"` + traceID + `","session_id":"` + testProjectID + `"}],"cursors":{}}`))
		case "/api/v2/traces/" + traceID + "/runs":
			query = r.URL.Query()
			_, _ = w.Write([]byte(`{"items":[{"id":"r1","start_time":"2026-01-01T00:00:01Z"}]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	got := runTraceCmdJSON(t, ts.URL, "get", traceID).(map[string]any)

	if len(paths) != 2 {
		t.Fatalf("requests = %v, want the workspace lookup then the v2 trace read", paths)
	}
	if lookup["trace"] != traceID {
		t.Errorf("lookup trace = %v, want %s", lookup["trace"], traceID)
	}
	// The server ignores limit for a trace filter; is_root bounds the response.
	if lookup["is_root"] != true {
		t.Errorf("lookup is_root = %v, want true", lookup["is_root"])
	}
	if _, ok := lookup["session"]; ok {
		t.Errorf("lookup sent a session filter without --project: %v", lookup["session"])
	}
	if _, ok := lookup["start_time"]; !ok {
		t.Error("lookup start_time missing: it should keep the default window")
	}
	if query["project_id"][0] != testProjectID {
		t.Errorf("project_id = %v, want the project from the lookup", query["project_id"])
	}
	if got["run_count"] != float64(1) {
		t.Errorf("run_count = %v, want 1", got["run_count"])
	}
}

func TestTraceGetRuns_NotFound(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "")
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	cases := []struct {
		name, version, projectID string
		want                     []string
	}{
		{"v2 workspace", "dev", "", []string{"not found in the current workspace", "--since"}},
		{"v1 workspace", "0.15.0", "", []string{"not found in the current workspace", "--since"}},
		{"v2 project", "dev", testProjectID, []string{"not found in the given project"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withDeploymentVersion(t, tc.version)
			var paths []string
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if strings.HasPrefix(r.URL.Path, "/api/v2/") {
					_, _ = w.Write([]byte(`{"items":[]}`))
					return
				}
				_, _ = w.Write([]byte(`{"runs":[],"cursors":{}}`))
			})
			cleanup := setupTestEnv(t, ts.URL)
			defer cleanup()

			_, err := traceGetRuns(context.Background(), MustGetClient(), "", tc.projectID, traceID, "", 0, nil, nil)
			if err == nil {
				t.Fatal("want a not-found error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want it to mention %q", err, w)
				}
			}
			if tc.version == "dev" && tc.projectID == "" && len(paths) != 1 {
				t.Errorf("requests = %v, want only the workspace lookup", paths)
			}
		})
	}
}

func TestTraceGetCmd_ExactArgs(t *testing.T) {
	cmd := newTraceGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"trace-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

// ==================== trace export flags ====================

func TestTraceExportCmd_Flags(t *testing.T) {
	cmd := newTraceExportCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"include-metadata", "false"},
		{"include-io", "false"},
		{"full", "false"},
		{"filename-pattern", "{trace_id}.jsonl"},
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
	}
}

func TestTraceExportCmd_HasCommonFilterFlags(t *testing.T) {
	cmd := newTraceExportCmd()
	common := []string{"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency", "min-tokens", "tags", "filter"}
	for _, name := range common {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("trace export missing common filter flag --%s", name)
		}
	}
}

func TestTraceExportCmd_NoRunTypeFlag(t *testing.T) {
	cmd := newTraceExportCmd()
	if cmd.Flags().Lookup("run-type") != nil {
		t.Error("trace export should not have --run-type flag")
	}
}

func TestTraceExportCmd_ExactArgs(t *testing.T) {
	cmd := newTraceExportCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"./output"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestTraceGetRuns_V2LookupUnavailableAsksForProject(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "")
	withDeploymentVersion(t, "dev")
	const traceID = "01a10aa3-0e17-7eb6-b255-d8a470732c91"
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/query" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"detail":"Runs query requires ClickHouse"}`))
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	_, err := traceGetRuns(context.Background(), MustGetClient(), "", "", traceID, "", 0, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--project-id") {
		t.Errorf("err = %v, want it to ask for --project or --project-id", err)
	}
}
