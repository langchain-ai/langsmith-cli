package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/urfave/cli/v3"
)

// commandsWithoutJSONOutput are the leaf commands that never print JSON, so
// they take no --jq. Every other command must accept it.
var commandsWithoutJSONOutput = []string{
	"langsmith connector run",
	"langsmith docs agents",
	"langsmith relay connect",
	"langsmith sandboxes console",
	"langsmith sandboxes exec",
	"langsmith sandboxes ssh-setup",
	"langsmith sandboxes tunnel",
	"langsmith tunnel connect",
}

func TestEveryStructuredCommandAcceptsJQ(t *testing.T) {
	var optedOut []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			walk(sub)
		}
		if !c.Runnable() {
			return
		}
		switch {
		case c.Annotations[generatedAnnotation] != "":
			// The generated tree parses these arguments and documents --jq.
			for _, op := range generatedOperations(t, c) {
				if !slices.ContainsFunc(op.Flags, func(f cli.Flag) bool { return f.Names()[0] == "jq" }) {
					t.Errorf("%s %s has no --jq", c.CommandPath(), op.Name)
				}
			}
		case c.Flags().Lookup("jq") != nil:
		case c.Annotations[cmdutil.NoJSONOutputAnnotation] != "":
			if !c.HasSubCommands() {
				optedOut = append(optedOut, c.CommandPath())
			}
		default:
			t.Errorf("%s has no --jq; it prints JSON, or mark it with cmdutil.NoJSONOutputAnnotation", c.CommandPath())
		}
	}
	walk(NewRootCmd("dev", "dev"))

	slices.Sort(optedOut)
	if !slices.Equal(optedOut, commandsWithoutJSONOutput) {
		t.Errorf("commands without --jq = %q; want %q", optedOut, commandsWithoutJSONOutput)
	}
}

// generatedOperations returns the generated operations a mounted command runs.
func generatedOperations(t *testing.T, c *cobra.Command) []*cli.Command {
	t.Helper()
	for _, r := range generated.Command.Commands {
		if r.Name != c.Annotations[generatedAnnotation] {
			continue
		}
		for _, op := range r.Commands {
			if op.Name == c.Name() {
				return []*cli.Command{op}
			}
		}
		return r.Commands
	}
	t.Fatalf("%s runs unknown generated resource %q", c.CommandPath(), c.Annotations[generatedAnnotation])
	return nil
}

func writeProfilesConfig(t *testing.T) {
	t.Helper()
	writeGeneratedConfig(t, `{"current_profile":"dev","profiles":{"dev":{"api_key":"k1"},"prod":{"api_key":"k2"}}}`)
	for _, key := range []string{"LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_PROFILE"} {
		t.Setenv(key, "")
	}
}

func TestJQFiltersHandWrittenJSON(t *testing.T) {
	writeProfilesConfig(t)
	for _, format := range []string{"json", "pretty"} {
		t.Run(format, func(t *testing.T) {
			stdout, err := executeCommand(t, "--format", format, "profiles", "list", "--jq", ".[].name")
			if err != nil {
				t.Fatalf("profiles list --jq: %v\n%s", err, stdout)
			}
			if got := strings.Fields(stdout); !slices.Equal(got, []string{"dev", "prod"}) {
				t.Fatalf("stdout = %q; want the profile names, one per line", stdout)
			}
		})
	}
}

func TestJQFiltersOutputPackageJSON(t *testing.T) {
	t.Cleanup(func() { output.SetJQ("") })
	output.SetJQ(".items | length")
	stdout := captureStdout(t, func() {
		if err := output.OutputJSON(map[string]any{"items": []int{1, 2, 3}}, ""); err != nil {
			t.Fatal(err)
		}
	})
	if stdout != "3\n" {
		t.Fatalf("stdout = %q; want 3", stdout)
	}
}

func TestJQRejectsInvalidExpressionBeforeRunning(t *testing.T) {
	isolateGeneratedAuth(t)
	requests := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer ts.Close()

	for _, args := range [][]string{
		{"profiles", "list", "--jq", ".["},
		{"--api-url", ts.URL, "--api-key", "k", "agents", "list", "--jq", ".["},
	} {
		t.Run(args[len(args)-3], func(t *testing.T) {
			writeProfilesConfig(t)
			var err error
			stdout := captureStdout(t, func() { _, err = executeCommand(t, args...) })
			if err == nil || !strings.Contains(err.Error(), "failed to parse jq expression (line 1, column 3)") {
				t.Fatalf("err = %v; want a jq parse error", err)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q; want nothing", stdout)
			}
		})
	}
	if requests != 0 {
		t.Fatalf("sent %d requests; want none for an invalid expression", requests)
	}
}

func TestJQFiltersGeneratedOutput(t *testing.T) {
	isolateGeneratedAuth(t)
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	agent := map[string]any{"id": "agent-1", "name": "First"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := any(agent)
		if r.URL.Path == "/api/v1/agents" {
			body = map[string]any{"items": []any{agent, map[string]any{"id": "agent-2", "name": "Second"}}, "next_cursor": nil}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer ts.Close()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"list is an array", []string{"agents", "list", "--jq", "map(.id) | join(\",\")"}, "agent-1,agent-2\n"},
		{"list length", []string{"agents", "list", "--jq=length"}, "2\n"},
		{"single response", []string{"agents", "retrieve", "--agent-id", "agent-1", "--jq", ".name"}, "First\n"},
		{"overrides --format", []string{"agents", "retrieve", "--agent-id", "agent-1", "--format", "yaml", "--raw-output", "--jq", "{id}"}, "{\"id\":\"agent-1\"}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousExit := exitGenerated
			exitGenerated = func(code int) { t.Errorf("unexpected CLI exit: %d", code) }
			t.Cleanup(func() { exitGenerated = previousExit })
			t.Cleanup(func() { output.SetJQ("") })
			root := NewRootCmd("dev", "dev")
			root.SetArgs(append([]string{"--api-url", ts.URL, "--api-key", "k"}, tt.args...))
			stdout := captureStdout(t, func() {
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			if stdout != tt.want {
				t.Fatalf("stdout = %q; want %q", stdout, tt.want)
			}
		})
	}
}

func TestWithJQOutputFlagsKeepsPositionalArguments(t *testing.T) {
	got := withJQOutputFlags([]string{"list", "--format", "yaml", "--", "--literal"})
	want := []string{"list", "--format", "yaml", "--format=jsonl", "--raw-output=false", "--", "--literal"}
	if !slices.Equal(got, want) {
		t.Fatalf("withJQOutputFlags = %q; want %q", got, want)
	}
}

func TestJQFiltersAPIResponse(t *testing.T) {
	isolateGeneratedAuth(t)
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"a"},{"name":"b"}]`))
	}))
	defer ts.Close()

	stdout, err := executeCommand(t, "--api-url", ts.URL, "--api-key", "k", "api", "sessions", "--jq", ".[1].name")
	if err != nil {
		t.Fatalf("api --jq: %v\n%s", err, stdout)
	}
	if stdout != "b\n" {
		t.Fatalf("stdout = %q; want b", stdout)
	}

	stdout, err = executeCommand(t, "--api-url", ts.URL, "--api-key", "k", "api", "sessions", "--include", "--jq", ".[1].name")
	if err == nil || !strings.Contains(err.Error(), "--include cannot be combined with --jq") {
		t.Fatalf("api --include --jq: err = %v; want a flag conflict error", err)
	}
	if strings.Contains(stdout, "HTTP/") {
		t.Fatalf("stdout = %q; want no response headers", stdout)
	}
}
