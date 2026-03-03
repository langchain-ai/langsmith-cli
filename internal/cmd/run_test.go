package cmd

import (
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
