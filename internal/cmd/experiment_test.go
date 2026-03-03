package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestExperimentCmd_Subcommands(t *testing.T) {
	cmd := newExperimentCmd()
	expected := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("experiment missing subcommand %q", name)
		}
	}
}

func TestExperimentCmd_UseField(t *testing.T) {
	cmd := newExperimentCmd()
	if cmd.Use != "experiment" {
		t.Errorf("expected Use=experiment, got %q", cmd.Use)
	}
}

func TestExperimentCmd_SubcommandCount(t *testing.T) {
	cmd := newExperimentCmd()
	if got := len(cmd.Commands()); got != 2 {
		t.Errorf("expected 2 subcommands, got %d", got)
	}
}

// ==================== experiment list flags ====================

func TestExperimentListCmd_Flags(t *testing.T) {
	cmd := newExperimentListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"dataset", "", ""},
		{"limit", "20", "n"},
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

func TestExperimentListCmd_UseField(t *testing.T) {
	cmd := newExperimentListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// ==================== experiment get flags ====================

func TestExperimentGetCmd_Flags(t *testing.T) {
	cmd := newExperimentGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestExperimentGetCmd_ExactArgs(t *testing.T) {
	cmd := newExperimentGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-exp"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestExperimentGetCmd_UseField(t *testing.T) {
	cmd := newExperimentGetCmd()
	if cmd.Use != "get NAME_OR_ID" {
		t.Errorf("expected Use='get NAME_OR_ID', got %q", cmd.Use)
	}
}
