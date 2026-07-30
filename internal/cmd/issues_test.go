package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

// ==================== Command structure ====================

func TestProjectIssuesCmd_Subcommands(t *testing.T) {
	cmd := newProjectIssuesCmd()
	expected := map[string]bool{
		"list":     false,
		"get":      false,
		"events":   false,
		"update":   false,
		"runs":     false,
		"examples": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("issues missing subcommand %q", name)
		}
	}
}

func TestProjectIssuesGetCmd(t *testing.T) {
	var method string
	var path string
	proposedFix := "Add the missing command"
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(forgeIssue{
			ID:          "issue-123",
			Name:        "Missing issues get command",
			ProposedFix: &proposedFix,
		})
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newProjectIssuesGetCmd()
		cmd.SetArgs([]string{"issue-123"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if method != http.MethodGet {
		t.Errorf("expected GET request, got %q", method)
	}
	if path != "/api/v1/platform/issues/issue-123" {
		t.Errorf("expected issue path, got %q", path)
	}

	var issue map[string]any
	if err := json.Unmarshal([]byte(out), &issue); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out)
	}
	if issue["id"] != "issue-123" {
		t.Errorf("expected issue ID in output, got %#v", issue["id"])
	}
	if issue["proposed_fix"] != proposedFix {
		t.Errorf("expected proposed fix in output, got %#v", issue["proposed_fix"])
	}
}

func TestProjectIssuesGetCmd_ArgsAndFlags(t *testing.T) {
	cmd := newProjectIssuesGetCmd()
	if cmd.Use != "get <issue-id>" {
		t.Errorf("expected Use=%q, got %q", "get <issue-id>", cmd.Use)
	}
	if err := cmd.Args(cmd, nil); err == nil {
		t.Error("expected missing issue ID to fail argument validation")
	}
	if f := cmd.Flags().Lookup("output"); f == nil || f.Shorthand != "o" {
		t.Errorf("expected --output/-o flag, got %+v", f)
	}
}

func TestProjectIssuesExamplesCmd_Subcommands(t *testing.T) {
	cmd := newProjectIssuesExamplesCmd()
	expected := map[string]bool{"propose": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("examples missing subcommand %q", name)
		}
	}
}

func TestProjectIssuesProposeExampleCmd_UseField(t *testing.T) {
	cmd := newProjectIssuesProposeExampleCmd()
	if cmd.Use != "propose <issue-id>" {
		t.Errorf("expected Use=%q, got %q", "propose <issue-id>", cmd.Use)
	}
}

// ==================== Propose example flags ====================

func TestProjectIssuesProposeExampleCmd_Flags(t *testing.T) {
	cmd := newProjectIssuesProposeExampleCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"run-id", ""},
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
	// --assertion is a StringArray; default is "[]"
	f := cmd.Flags().Lookup("assertion")
	if f == nil {
		t.Error("flag --assertion not found")
	}
}

// ==================== parseAssertion ====================

func TestParseAssertion_Valid(t *testing.T) {
	cases := []struct {
		input   string
		wantKey string
		wantCmt string
	}{
		{"correctness=Response must cite sources", "correctness", "Response must cite sources"},
		{"format=Output must be valid JSON", "format", "Output must be valid JSON"},
		// comment may itself contain '='
		{"eq=a=b is valid", "eq", "a=b is valid"},
		// whitespace is trimmed
		{"  key  =  comment here  ", "key", "comment here"},
	}
	for _, tc := range cases {
		a, err := parseAssertion(tc.input)
		if err != nil {
			t.Errorf("parseAssertion(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if a.Key != tc.wantKey {
			t.Errorf("parseAssertion(%q): key=%q, want %q", tc.input, a.Key, tc.wantKey)
		}
		if a.Comment != tc.wantCmt {
			t.Errorf("parseAssertion(%q): comment=%q, want %q", tc.input, a.Comment, tc.wantCmt)
		}
	}
}

func TestParseAssertion_Invalid(t *testing.T) {
	cases := []struct {
		input string
		desc  string
	}{
		{"no-equals", "missing '='"},
		{"=no-key", "empty key"},
		{"key=", "empty comment"},
		{"  =  ", "both empty after trim"},
	}
	for _, tc := range cases {
		_, err := parseAssertion(tc.input)
		if err == nil {
			t.Errorf("parseAssertion(%q) [%s]: expected error, got nil", tc.input, tc.desc)
		}
	}
}
