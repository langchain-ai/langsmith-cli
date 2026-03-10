package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ==================== Command structure ====================

func TestRunCmd_Subcommands(t *testing.T) {
	cmd := newRunCmd()
	expected := map[string]bool{"list": false, "get": false, "export": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("run missing subcommand %q", name)
		}
	}
}

func TestRunCmd_UseField(t *testing.T) {
	cmd := newRunCmd()
	if cmd.Use != "run" {
		t.Errorf("expected Use=run, got %q", cmd.Use)
	}
}

// ==================== run list flags ====================

func TestRunListCmd_Flags(t *testing.T) {
	cmd := newRunListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"include-metadata", "false", ""},
		{"include-io", "false", ""},
		{"full", "false", ""},
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

func TestRunListCmd_HasRunTypeFlag(t *testing.T) {
	cmd := newRunListCmd()
	f := cmd.Flags().Lookup("run-type")
	if f == nil {
		t.Fatal("--run-type flag not found on run list")
	}
	if f.DefValue != "" {
		t.Errorf("expected default empty, got %q", f.DefValue)
	}
}

func TestRunListCmd_HasCommonFilterFlags(t *testing.T) {
	cmd := newRunListCmd()
	common := []string{"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency", "min-tokens", "tags", "filter", "run-type"}
	for _, name := range common {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("run list missing filter flag --%s", name)
		}
	}
}

// ==================== run get flags ====================

func TestRunGetCmd_Flags(t *testing.T) {
	cmd := newRunGetCmd()
	tests := []struct {
		name   string
		defVal string
	}{
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

func TestRunGetCmd_OutputShorthand(t *testing.T) {
	cmd := newRunGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestRunGetCmd_ExactArgs(t *testing.T) {
	cmd := newRunGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"run-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

// ==================== run export flags ====================

func TestRunExportCmd_Flags(t *testing.T) {
	cmd := newRunExportCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"include-metadata", "false"},
		{"include-io", "false"},
		{"full", "false"},
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

func TestRunExportCmd_HasRunTypeFlag(t *testing.T) {
	cmd := newRunExportCmd()
	if cmd.Flags().Lookup("run-type") == nil {
		t.Error("run export missing --run-type flag")
	}
}

func TestRunExportCmd_HasCommonFilterFlags(t *testing.T) {
	cmd := newRunExportCmd()
	common := []string{"trace-ids", "limit", "project", "last-n-minutes", "since",
		"error", "no-error", "name", "min-latency", "max-latency", "min-tokens", "tags", "filter", "run-type"}
	for _, name := range common {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("run export missing filter flag --%s", name)
		}
	}
}

func TestRunExportCmd_ExactArgs(t *testing.T) {
	cmd := newRunExportCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"output.jsonl"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

// ==================== run export no --output flag ====================

func TestRunExportCmd_NoOutputFlag(t *testing.T) {
	cmd := newRunExportCmd()
	// run export takes OUTPUT_FILE as positional arg, not --output flag
	if cmd.Flags().Lookup("output") != nil {
		t.Error("run export should not have --output flag (uses positional arg)")
	}
}

// ==================== Execution tests ====================

// runTestServer returns a handler that mocks /sessions and /runs/query.
// sessions maps project name → session ID.
// runs is returned for any /runs/query POST.
func newRunTestServer(t *testing.T, sessions map[string]string, runs []map[string]any) *httptest.Server {
	t.Helper()
	return newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			name := r.URL.Query().Get("name")
			id, ok := sessions[name]
			if !ok {
				w.WriteHeader(404)
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": id, "name": name}})
		case r.URL.Path == "/api/v1/runs/query" && r.Method == "POST":
			_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs})
		default:
			http.Error(w, "not found", 404)
		}
	})
}

func TestRunListCmd_Execute_WithProject_Succeeds(t *testing.T) {
	ts := newRunTestServer(t,
		map[string]string{"my-app": "session-123"},
		[]map[string]any{
			{"id": "run-1", "name": "ChatOpenAI", "run_type": "llm"},
			{"id": "run-2", "name": "tool_call", "run_type": "tool"},
		},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	cmd := newRunListCmd()
	_ = cmd.Flags().Set("project", "my-app")

	out := captureStdout(t, func() { cmd.Run(cmd, nil) })

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result))
	}
	if result[0]["run_id"] != "run-1" {
		t.Errorf("expected run_id=run-1, got %v", result[0]["run_id"])
	}
}

func TestRunListCmd_Execute_WithEnvProject_Succeeds(t *testing.T) {
	ts := newRunTestServer(t,
		map[string]string{"env-project": "session-456"},
		[]map[string]any{{"id": "run-3", "name": "agent", "run_type": "chain"}},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	t.Setenv("LANGSMITH_PROJECT", "env-project")

	cmd := newRunListCmd()
	out := captureStdout(t, func() { cmd.Run(cmd, nil) })

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 run, got %d", len(result))
	}
	if result[0]["run_id"] != "run-3" {
		t.Errorf("expected run_id=run-3, got %v", result[0]["run_id"])
	}
}

func TestRunGetCmd_Execute_NoProjectNeeded(t *testing.T) {
	// run get should NOT require --project; it fetches by ID directly
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/runs/query" && r.Method == "POST" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"runs": []map[string]any{
					{"id": "run-abc", "name": "my_run", "run_type": "llm"},
				},
			})
			return
		}
		http.Error(w, "not found", 404)
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	t.Setenv("LANGSMITH_PROJECT", "")

	cmd := newRunGetCmd()
	out := captureStdout(t, func() { cmd.Run(cmd, []string{"run-abc"}) })

	if !strings.Contains(out, "run-abc") {
		t.Errorf("expected output to contain run-abc, got: %s", out)
	}
}
