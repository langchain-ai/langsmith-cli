package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

// ==================== Command structure ====================

func TestTraceCmd_Subcommands(t *testing.T) {
	cmd := newTraceCmd()
	expected := map[string]bool{"list": false, "get": false, "export": false, "messages": false, "stats": false}
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

// ==================== trace stats flags ====================

func TestTraceStatsCmd_Flags(t *testing.T) {
	cmd := newTraceStatsCmd()
	for _, name := range []string{
		"project", "since", "before", "last-n-minutes",
		"compare-since", "compare-before", "compare-last-n-minutes",
		"filter", "output",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("trace stats missing flag --%s", name)
		}
	}
}

// ==================== trace stats: end-to-end ====================

// TestTraceStats_TotalCostFetched verifies that total_cost survives the
// /api/v1/runs/stats round-trip. The SDK's typed wrapper drops it (it models
// the field as string while the API emits a number, mis-discriminating the
// response union and zeroing every field), so this command goes through raw
// HTTP. If the build ever flips back to the SDK without an upstream fix,
// total_cost will silently disappear from the output and break the issues
// agent's cost baseline — this test is the canary.
func TestTraceStats_TotalCostFetched(t *testing.T) {
	var receivedBody map[string]any
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "sess-cost", "name": "cost-proj"},
			})
		case r.URL.Path == "/api/v1/runs/stats" && r.Method == "POST":
			_ = json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run_count":         42,
				"error_rate":        0.05,
				"latency_p50":       1.23,
				"latency_p99":       4.56,
				"total_tokens":      12345,
				"prompt_tokens":     10000,
				"completion_tokens": 2345,
				"total_cost":        8.2e-6,
				"feedback_stats":    map[string]any{},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newTraceStatsCmd()
		cmd.SetArgs([]string{"--project", "cost-proj", "--last-n-minutes", "60"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify total_cost was requested in the select
	sel, _ := receivedBody["select"].([]any)
	hasTotalCost := false
	for _, s := range sel {
		if s == "total_cost" {
			hasTotalCost = true
			break
		}
	}
	if !hasTotalCost {
		t.Errorf("expected total_cost in select, got %v", receivedBody["select"])
	}
	if receivedBody["is_root"] != true {
		t.Errorf("expected is_root=true, got %v", receivedBody["is_root"])
	}

	// Verify total_cost made it into the output
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\noutput: %s", err, out)
	}
	stats, ok := result["stats"].(map[string]any)
	if !ok {
		t.Fatalf("expected stats object in output, got %v", result)
	}
	if stats["total_cost"] == nil {
		t.Errorf("expected total_cost to be set in output, got nil. full output: %s", out)
	}
}
