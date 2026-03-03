package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestThreadCmd_Subcommands(t *testing.T) {
	cmd := newThreadCmd()
	expected := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("thread missing subcommand %q", name)
		}
	}
}

func TestThreadCmd_UseField(t *testing.T) {
	cmd := newThreadCmd()
	if cmd.Use != "thread" {
		t.Errorf("expected Use=thread, got %q", cmd.Use)
	}
}

// ==================== thread list flags ====================

func TestThreadListCmd_Flags(t *testing.T) {
	cmd := newThreadListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"limit", "20", "n"},
		{"filter", "", ""},
		{"last-n-minutes", "0", ""},
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

func TestThreadListCmd_ProjectRequired(t *testing.T) {
	cmd := newThreadListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--project has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--project not marked as required")
	}
}

// ==================== thread get flags ====================

func TestThreadGetCmd_Flags(t *testing.T) {
	cmd := newThreadGetCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
		{"include-metadata", "false", ""},
		{"include-io", "false", ""},
		{"full", "false", ""},
		{"limit", "0", "n"},
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

func TestThreadGetCmd_ExactArgs(t *testing.T) {
	cmd := newThreadGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"thread-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestThreadGetCmd_ProjectRequired(t *testing.T) {
	cmd := newThreadGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--project has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--project not marked as required")
	}
}
