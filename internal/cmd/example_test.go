package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestExampleCmd_Subcommands(t *testing.T) {
	cmd := newExampleCmd()
	expected := map[string]bool{"list": false, "create": false, "delete": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("example missing subcommand %q", name)
		}
	}
}

func TestExampleCmd_UseField(t *testing.T) {
	cmd := newExampleCmd()
	if cmd.Use != "example" {
		t.Errorf("expected Use=example, got %q", cmd.Use)
	}
}

// ==================== Subcommand Use fields ====================

func TestExampleListCmd_UseField(t *testing.T) {
	cmd := newExampleListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

func TestExampleCreateCmd_UseField(t *testing.T) {
	cmd := newExampleCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use=create, got %q", cmd.Use)
	}
}

func TestExampleDeleteCmd_UseField(t *testing.T) {
	cmd := newExampleDeleteCmd()
	if cmd.Use != "delete EXAMPLE_ID" {
		t.Errorf("expected Use='delete EXAMPLE_ID', got %q", cmd.Use)
	}
}

// ==================== example list flags ====================

func TestExampleListCmd_Flags(t *testing.T) {
	cmd := newExampleListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"dataset", "", ""},
		{"limit", "20", "n"},
		{"offset", "0", ""},
		{"split", "", ""},
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

func TestExampleListCmd_RequiredDataset(t *testing.T) {
	cmd := newExampleListCmd()
	f := cmd.Flags().Lookup("dataset")
	if f == nil {
		t.Fatal("--dataset not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--dataset has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--dataset not marked as required")
	}
}

// ==================== example create flags ====================

func TestExampleCreateCmd_Flags(t *testing.T) {
	cmd := newExampleCreateCmd()
	flags := map[string]string{
		"dataset":  "",
		"inputs":   "",
		"outputs":  "",
		"metadata": "",
		"split":    "",
	}
	for name, defVal := range flags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
			continue
		}
		if f.DefValue != defVal {
			t.Errorf("flag --%s: expected default %q, got %q", name, defVal, f.DefValue)
		}
	}
}

func TestExampleCreateCmd_RequiredFlags(t *testing.T) {
	cmd := newExampleCreateCmd()
	for _, name := range []string{"dataset", "inputs"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("flag --%s not found", name)
		}
		ann := f.Annotations
		if ann == nil {
			t.Errorf("flag --%s has no annotations (not marked required)", name)
			continue
		}
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
			t.Errorf("flag --%s not marked as required", name)
		}
	}
}

// ==================== example delete flags ====================

func TestExampleDeleteCmd_Flags(t *testing.T) {
	cmd := newExampleDeleteCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestExampleDeleteCmd_ExactArgs(t *testing.T) {
	cmd := newExampleDeleteCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"ex-id"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}
