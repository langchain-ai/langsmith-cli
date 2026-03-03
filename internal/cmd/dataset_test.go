package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestDatasetCmd_Subcommands(t *testing.T) {
	cmd := newDatasetCmd()
	expected := map[string]bool{
		"list": false, "get": false, "create": false,
		"delete": false, "export": false, "upload": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("dataset missing subcommand %q", name)
		}
	}
}

func TestDatasetCmd_UseField(t *testing.T) {
	cmd := newDatasetCmd()
	if cmd.Use != "dataset" {
		t.Errorf("expected Use=dataset, got %q", cmd.Use)
	}
}

func TestDatasetCmd_SubcommandCount(t *testing.T) {
	cmd := newDatasetCmd()
	if got := len(cmd.Commands()); got != 6 {
		t.Errorf("expected 6 subcommands, got %d", got)
	}
}

// ==================== dataset list flags ====================

func TestDatasetListCmd_Flags(t *testing.T) {
	cmd := newDatasetListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"limit", "100", "n"},
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

func TestDatasetListCmd_UseField(t *testing.T) {
	cmd := newDatasetListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// ==================== dataset get flags ====================

func TestDatasetGetCmd_Flags(t *testing.T) {
	cmd := newDatasetGetCmd()
	f := cmd.Flags().Lookup("output")
	if f == nil {
		t.Fatal("--output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("expected shorthand 'o', got %q", f.Shorthand)
	}
}

func TestDatasetGetCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-dataset"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestDatasetGetCmd_UseField(t *testing.T) {
	cmd := newDatasetGetCmd()
	if cmd.Use != "get NAME_OR_ID" {
		t.Errorf("expected Use='get NAME_OR_ID', got %q", cmd.Use)
	}
}

// ==================== dataset create flags ====================

func TestDatasetCreateCmd_Flags(t *testing.T) {
	cmd := newDatasetCreateCmd()
	flags := map[string]string{
		"name":        "",
		"description": "",
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

func TestDatasetCreateCmd_RequiredName(t *testing.T) {
	cmd := newDatasetCreateCmd()
	f := cmd.Flags().Lookup("name")
	if f == nil {
		t.Fatal("--name flag not found")
	}
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--name has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--name not marked as required")
	}
}

func TestDatasetCreateCmd_UseField(t *testing.T) {
	cmd := newDatasetCreateCmd()
	if cmd.Use != "create" {
		t.Errorf("expected Use=create, got %q", cmd.Use)
	}
}

// ==================== dataset delete flags ====================

func TestDatasetDeleteCmd_Flags(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	f := cmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found")
	}
	if f.DefValue != "false" {
		t.Errorf("expected default false, got %q", f.DefValue)
	}
}

func TestDatasetDeleteCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"my-dataset"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestDatasetDeleteCmd_UseField(t *testing.T) {
	cmd := newDatasetDeleteCmd()
	if cmd.Use != "delete NAME_OR_ID" {
		t.Errorf("expected Use='delete NAME_OR_ID', got %q", cmd.Use)
	}
}

// ==================== dataset export flags ====================

func TestDatasetExportCmd_Flags(t *testing.T) {
	cmd := newDatasetExportCmd()
	f := cmd.Flags().Lookup("limit")
	if f == nil {
		t.Fatal("--limit flag not found")
	}
	if f.DefValue != "100" {
		t.Errorf("expected default 100, got %q", f.DefValue)
	}
	if f.Shorthand != "n" {
		t.Errorf("expected shorthand 'n', got %q", f.Shorthand)
	}
}

func TestDatasetExportCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetExportCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"ds"}); err == nil {
		t.Error("expected error for 1 arg")
	}
	if err := cmd.Args(cmd, []string{"ds", "out.json"}); err != nil {
		t.Errorf("expected no error for 2 args, got %v", err)
	}
}

func TestDatasetExportCmd_UseField(t *testing.T) {
	cmd := newDatasetExportCmd()
	if cmd.Use != "export NAME_OR_ID OUTPUT_FILE" {
		t.Errorf("expected Use='export NAME_OR_ID OUTPUT_FILE', got %q", cmd.Use)
	}
}

// ==================== dataset upload flags ====================

func TestDatasetUploadCmd_Flags(t *testing.T) {
	cmd := newDatasetUploadCmd()
	flags := map[string]string{
		"name":        "",
		"description": "",
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

func TestDatasetUploadCmd_RequiredName(t *testing.T) {
	cmd := newDatasetUploadCmd()
	f := cmd.Flags().Lookup("name")
	ann := f.Annotations
	if ann == nil {
		t.Fatal("--name has no annotations (not marked required)")
	}
	if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
		t.Error("--name not marked as required")
	}
}

func TestDatasetUploadCmd_ExactArgs(t *testing.T) {
	cmd := newDatasetUploadCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"data.json"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestDatasetUploadCmd_UseField(t *testing.T) {
	cmd := newDatasetUploadCmd()
	if cmd.Use != "upload FILE_PATH" {
		t.Errorf("expected Use='upload FILE_PATH', got %q", cmd.Use)
	}
}
