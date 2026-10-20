package usage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

func reset(t *testing.T) {
	t.Cleanup(func() { userAgent, command, agent = "", "", "" })
}

func TestApply(t *testing.T) {
	reset(t)
	SetVersion("0.5.1")
	command, agent = "traces query", "codex"

	h := http.Header{}
	Apply(h)

	if got := h.Get("User-Agent"); !strings.HasPrefix(got, "langsmith-cli/0.5.1") {
		t.Errorf("User-Agent = %q", got)
	}
	if got := h.Get(CommandHeader); got != "traces query" {
		t.Errorf("%s = %q", CommandHeader, got)
	}
	if got := h.Get(AgentHeader); got != "codex" {
		t.Errorf("%s = %q", AgentHeader, got)
	}
}

func TestApplyWithoutCommand(t *testing.T) {
	reset(t)
	h := http.Header{}
	Apply(h)
	if len(h) != 0 {
		t.Errorf("headers = %v, want none", h)
	}
}

func TestInvokedPathUsesTypedAliases(t *testing.T) {
	root := &cobra.Command{Use: "langsmith"}
	traces := &cobra.Command{Use: "traces", Aliases: []string{"trace"}}
	list := &cobra.Command{Use: "list", Aliases: []string{"ls"}, Run: func(*cobra.Command, []string) {}}
	root.PersistentFlags().String("format", "", "")
	list.Flags().String("project", "", "")
	root.AddCommand(traces)
	traces.AddCommand(list)

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"traces", "list", "--project", "p"}, "traces list"},
		{[]string{"trace", "ls"}, "trace ls"},
		{[]string{"--format", "json", "trace", "list"}, "trace list"},
	}
	for _, tt := range tests {
		root.SetArgs(tt.args)
		cmd, err := root.ExecuteC()
		if err != nil {
			t.Fatal(err)
		}
		if got := invokedPath(cmd, tt.args); got != tt.want {
			t.Errorf("invokedPath(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestDetectAgent(t *testing.T) {
	tests := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"CLAUDECODE": "1"}, "claude-code"},
		{map[string]string{"CODEX_SANDBOX": "seatbelt"}, "codex"},
		{map[string]string{"CURSOR_AGENT": "1"}, "cursor"},
		{nil, "none"},
	}
	for _, tt := range tests {
		if got := detectAgent(func(k string) string { return tt.env[k] }); got != tt.want {
			t.Errorf("detectAgent(%v) = %q, want %q", tt.env, got, tt.want)
		}
	}
}

func TestWrapDefaultTransport(t *testing.T) {
	reset(t)
	SetVersion("0.5.1")
	command, agent = "runs list", "none"

	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	prev := http.DefaultTransport
	restore := WrapDefaultTransport()
	client := langsmith.NewClient(option.WithBaseURL(srv.URL), option.WithAPIKey("test"))
	err := client.Execute(context.Background(), http.MethodGet, "api/v1/sessions", nil, nil)
	restore()
	if err != nil {
		t.Fatal(err)
	}

	if got.Get(CommandHeader) != "runs list" || !strings.HasPrefix(got.Get("User-Agent"), "langsmith-cli/0.5.1") {
		t.Errorf("headers = %v", got)
	}
	if http.DefaultTransport != prev {
		t.Error("http.DefaultTransport was not restored")
	}
}
