package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestProjectCmd_Subcommands(t *testing.T) {
	cmd := newProjectCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	if !names["list"] {
		t.Error("project missing subcommand 'list'")
	}
}

func TestProjectCmd_UseField(t *testing.T) {
	cmd := newProjectCmd()
	if cmd.Use != "project" {
		t.Errorf("expected Use=project, got %q", cmd.Use)
	}
}

// ==================== project list flags ====================

func TestProjectListCmd_Flags(t *testing.T) {
	cmd := newProjectListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"limit", "20", "n"},
		{"name-contains", "", ""},
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

func TestProjectListCmd_UseField(t *testing.T) {
	cmd := newProjectListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

func TestProjectListCmd_HasNoArgsValidator(t *testing.T) {
	cmd := newProjectListCmd()
	// project list takes no positional args — it should not set an Args validator
	if cmd.Args != nil {
		t.Error("expected no Args validator on project list (takes no positional args)")
	}
}
