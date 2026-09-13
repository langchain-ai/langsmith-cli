package cmd

import (
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
)

// ==================== Select fields ====================

// thread list groups runs by thread_id. If the field is missing from the
// select set the query succeeds and returns runs with an empty ThreadID, so
// the command reports zero threads with exit code 0 instead of failing.
func TestThreadListSelect_IncludesThreadID(t *testing.T) {
	var found bool
	for _, f := range threadListSelect() {
		if f == langsmith.RunQueryParamsSelectThreadID {
			found = true
			break
		}
	}
	if !found {
		t.Error("threadListSelect() must include thread_id; without it thread list returns zero threads")
	}
}

func TestThreadListSelectV2_IncludesThreadID(t *testing.T) {
	var found bool
	for _, f := range threadListSelectV2() {
		if f == langsmith.RunSelectFieldThreadID {
			found = true
			break
		}
	}
	if !found {
		t.Error("threadListSelectV2() must include THREAD_ID; without it thread list returns zero threads")
	}
}

// The base select sets feed every other run query; thread_id belongs only to
// the thread-list variants, so a regression there is visible here too.
func TestThreadListSelect_ExtendsBaseSet(t *testing.T) {
	if len(threadListSelect()) != len(buildRunSelect(true, false))+1 {
		t.Error("threadListSelect() should be the base v1 set plus thread_id")
	}
	if len(threadListSelectV2()) != len(buildRunSelectV2(true, false))+1 {
		t.Error("threadListSelectV2() should be the base v2 set plus THREAD_ID")
	}
}

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

func TestThreadListCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newThreadListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	// project should NOT be marked as cobra-required so that
	// ResolveProject can fall back to LANGSMITH_PROJECT env var
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestThreadListCmd_ProjectEnvFallback(t *testing.T) {
	t.Setenv("LANGSMITH_PROJECT", "env-project")
	result := ResolveProject("")
	if result != "env-project" {
		t.Errorf("expected ResolveProject to return env-project, got %q", result)
	}
}

func TestThreadListCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newThreadListCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
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

func TestThreadGetCmd_ProjectNotCobraRequired(t *testing.T) {
	cmd := newThreadGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	// project should NOT be marked as cobra-required so that
	// ResolveProject can fall back to LANGSMITH_PROJECT env var
	ann := f.Annotations
	if ann != nil {
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; ok {
			t.Error("--project should not be marked as cobra-required; use ResolveProject instead")
		}
	}
}

func TestThreadGetCmd_ProjectFlagHelpMentionsEnv(t *testing.T) {
	cmd := newThreadGetCmd()
	f := cmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("--project flag not found")
	}
	if f.Usage != "Project name [env: LANGSMITH_PROJECT]" {
		t.Errorf("expected project flag usage to mention env var, got %q", f.Usage)
	}
}
