package cmd

import (
	"testing"
)

// ==================== Command structure ====================

func TestInsightsCmd_Subcommands(t *testing.T) {
	cmd := newInsightsCmd()
	expected := map[string]bool{"list": false, "get": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("insights missing subcommand %q", name)
		}
	}
}

func TestInsightsCmd_UseField(t *testing.T) {
	cmd := newInsightsCmd()
	if cmd.Use != "insights" {
		t.Errorf("expected Use=insights, got %q", cmd.Use)
	}
}

// ==================== insights list flags ====================

func TestInsightsListCmd_Flags(t *testing.T) {
	cmd := newInsightsListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
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

func TestInsightsListCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newInsightsListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestInsightsListCmd_ProjectEnvFallback(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "env-project")
	result := ResolveProject("")
	if result != "env-project" {
		t.Errorf("expected ResolveProject to return env-project, got %q", result)
	}
}

func TestInsightsListCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newInsightsListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

// ==================== insights get flags ====================

func TestInsightsGetCmd_Flags(t *testing.T) {
	cmd := newInsightsGetCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", ""},
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

func TestInsightsGetCmd_ExactArgs(t *testing.T) {
	cmd := newInsightsGetCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for 0 args")
	}
	if err := cmd.Args(cmd, []string{"insight-123"}); err != nil {
		t.Errorf("expected no error for 1 arg, got %v", err)
	}
}

func TestInsightsGetCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newInsightsGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestInsightsGetCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newInsightsGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}

// ==================== Helper functions ====================

func TestFormatShape_Nil(t *testing.T) {
	if got := formatShape(nil); got != "N/A" {
		t.Errorf("expected N/A for nil shape, got %q", got)
	}
}

func TestFormatShape_Empty(t *testing.T) {
	if got := formatShape(map[string]any{}); got != "N/A" {
		t.Errorf("expected N/A for empty shape, got %q", got)
	}
}

func TestFormatShape_WithData(t *testing.T) {
	shape := map[string]any{"Tooling": 19, "Integrations": 53}
	got := formatShape(shape)
	expected := "Integrations:53, Tooling:19"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatInsightTime_Empty(t *testing.T) {
	if got := formatInsightTime(""); got != "N/A" {
		t.Errorf("expected N/A for empty time, got %q", got)
	}
}

func TestFormatInsightTime_ValidRFC3339(t *testing.T) {
	got := formatInsightTime("2026-03-17T12:58:12.701921+00:00")
	expected := "2026-03-17 12:58"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatInsightTime_ValidNoTimezone(t *testing.T) {
	got := formatInsightTime("2026-03-17T12:58:12.701921")
	expected := "2026-03-17 12:58"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatInsightTime_InvalidFallback(t *testing.T) {
	got := formatInsightTime("not-a-valid-timestamp-at-all")
	if got != "not-a-valid-time" {
		t.Errorf("expected truncated fallback, got %q", got)
	}
}

func TestFormatInsightTime_ShortInvalid(t *testing.T) {
	got := formatInsightTime("bad")
	if got != "bad" {
		t.Errorf("expected 'bad' for short invalid input, got %q", got)
	}
}
