package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestTraceCmd_Subcommands(t *testing.T) {
	cmd := newTraceCmd()
	expected := map[string]bool{"list": false, "get": false, "export": false}
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
