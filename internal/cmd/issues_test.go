package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
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

// ==================== Propose example request body ====================

func TestProposeExampleBody(t *testing.T) {
	assertions := []exampleAssertion{{Key: "must_refuse", Comment: "Does not send the email"}}

	t.Run("includes start_time when set", func(t *testing.T) {
		body := proposeExampleBody("run-1", "2026-04-10T00:00:00Z", assertions)
		if got := body["start_time"]; got != "2026-04-10T00:00:00Z" {
			t.Errorf("expected start_time to be sent, got %v", got)
		}
	})

	// Older callers must keep their exact request shape so this release can
	// ship before the server that reads the field.
	t.Run("omits start_time when unset", func(t *testing.T) {
		body := proposeExampleBody("run-1", "", assertions)
		if _, ok := body["start_time"]; ok {
			t.Error("expected start_time to be absent, not empty")
		}
		if body["run_id"] != "run-1" {
			t.Errorf("expected run_id run-1, got %v", body["run_id"])
		}
	})
}

// ==================== Propose example flags ====================

func TestProjectIssuesProposeExampleCmd_Flags(t *testing.T) {
	cmd := newProjectIssuesProposeExampleCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"run-id", ""},
		{"start-time", ""},
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

// ==================== Update status and reason ====================

// The update command used to hard-reject anything but "open" client-side, so a
// dismissal never reached the server. Which transitions a caller may make is
// role-dependent and belongs to the server; this only catches typos.
func TestValidIssueUpdateStatus(t *testing.T) {
	for _, status := range []string{"open", "fixing", "watching", "completed", "ignored"} {
		if !validIssueUpdateStatus(status) {
			t.Errorf("validIssueUpdateStatus(%q) = false, want true", status)
		}
	}
	for _, status := range []string{"closed", "dismissed", "resolved", "Open", "", "bogus"} {
		if validIssueUpdateStatus(status) {
			t.Errorf("validIssueUpdateStatus(%q) = true, want false", status)
		}
	}
}

func TestProjectIssuesUpdateCmd_Flags(t *testing.T) {
	cmd := newProjectIssuesUpdateCmd()
	for _, name := range []string{"name", "description", "proposed-fix", "status", "reason", "evaluator"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("flag --%s not found", name)
		} else if f.DefValue != "" {
			t.Errorf("flag --%s: expected empty default, got %q", name, f.DefValue)
		}
	}
}

// The old help text told callers closing was UI-only. Leaving that in place
// while the server accepts a dismissal is how the --title flag went stale for
// months: documented, never accepted, failing quietly.
func TestProjectIssuesUpdateCmd_HelpDocumentsDismissal(t *testing.T) {
	cmd := newProjectIssuesUpdateCmd()
	for _, want := range []string{"ignored", "Incorrectly Flagged", "--reason"} {
		if !strings.Contains(cmd.Long, want) {
			t.Errorf("update help does not mention %q", want)
		}
	}
	if strings.Contains(cmd.Long, "only accepted value is 'open'") {
		t.Error("update help still claims only 'open' is accepted")
	}
}
