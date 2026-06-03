package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	if cmd.Use != "trace" {
		t.Errorf("expected Use=trace, got %q", cmd.Use)
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
		{"one-per-thread", "false", ""},
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

// traceListTestServer returns a handler that mocks /sessions and /runs/query.
// sessions maps project name to session ID. requestBodies captures every
// /runs/query request body.
func traceListTestServer(t *testing.T, sessions map[string]string, requestBodies *[]map[string]any) *httptest.Server {
	t.Helper()
	return newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			name := r.URL.Query().Get("name")
			id, ok := sessions[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": id, "name": name}})
		case r.URL.Path == "/api/v1/runs/query" && r.Method == "POST":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decoding runs query body: %v", err)
			}
			*requestBodies = append(*requestBodies, body)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"runs": []map[string]any{{
					"id":         "run-1",
					"trace_id":   "trace-1",
					"name":       "agent",
					"run_type":   "chain",
					"start_time": "2026-01-01T00:00:00Z",
				}},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func TestTraceListCmd_OnePerThreadSendsServerParam(t *testing.T) {
	var requestBodies []map[string]any
	ts := traceListTestServer(t, map[string]string{"my-app": "session-123"}, &requestBodies)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	cmd := newTraceListCmd()
	_ = cmd.Flags().Set("project", "my-app")
	_ = cmd.Flags().Set("one-per-thread", "true")
	_ = cmd.Flags().Set("limit", "1")

	_ = captureStdout(t, func() { cmd.Run(cmd, nil) })

	if len(requestBodies) != 1 {
		t.Fatalf("expected 1 runs query request, got %d", len(requestBodies))
	}
	if got := requestBodies[0]["one_per_thread"]; got != true {
		t.Fatalf("expected one_per_thread=true, got %#v in %#v", got, requestBodies[0])
	}
}

func TestTraceListCmd_OnePerThreadDefaultOmitsServerParam(t *testing.T) {
	var requestBodies []map[string]any
	ts := traceListTestServer(t, map[string]string{"my-app": "session-123"}, &requestBodies)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	cmd := newTraceListCmd()
	_ = cmd.Flags().Set("project", "my-app")
	_ = cmd.Flags().Set("limit", "1")

	_ = captureStdout(t, func() { cmd.Run(cmd, nil) })

	if len(requestBodies) != 1 {
		t.Fatalf("expected 1 runs query request, got %d", len(requestBodies))
	}
	if _, ok := requestBodies[0]["one_per_thread"]; ok {
		t.Fatalf("expected one_per_thread to be omitted, got %#v", requestBodies[0])
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
