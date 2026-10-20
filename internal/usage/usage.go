// Package usage adds the headers LangSmith uses to count CLI usage: the CLI
// version, the command name as typed (never its arguments), and the coding
// agent running the CLI, if any.
package usage

import (
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-go/option"
	"github.com/spf13/cobra"
)

const (
	CommandHeader = "X-LangSmith-CLI-Command"
	AgentHeader   = "X-LangSmith-CLI-Agent"
)

var (
	userAgent string
	command   string
	agent     string
)

// SetVersion sets the CLI version reported in the User-Agent.
func SetVersion(version string) {
	userAgent = "langsmith-cli/" + version
	if sdk := sdkVersion(); sdk != "" {
		userAgent += " langsmith-go/" + sdk
	}
}

// SetCommand records the running command and detects the calling agent.
func SetCommand(cmd *cobra.Command) {
	command = invokedPath(cmd, os.Args[1:])
	agent = detectAgent(os.Getenv)
}

// Apply sets the usage headers on an outgoing LangSmith API request.
func Apply(h http.Header) {
	if userAgent != "" {
		h.Set("User-Agent", userAgent)
	}
	if command != "" {
		h.Set(CommandHeader, command)
		h.Set(AgentHeader, agent)
	}
}

// Option applies the usage headers to every request an SDK client sends.
func Option() option.RequestOption {
	return option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		Apply(req.Header)
		return next(req)
	})
}

// WrapDefaultTransport tags requests sent through http.DefaultTransport, which
// the SDK uses by default, until restore is called. The SDK keeps a wrapped
// transport as-is, so the clone carries the SDK's response header timeout.
func WrapDefaultTransport() (restore func()) {
	prev := http.DefaultTransport
	base := prev
	if t, ok := prev.(*http.Transport); ok {
		t = t.Clone()
		t.ResponseHeaderTimeout = 10 * time.Minute
		base = t
	}
	http.DefaultTransport = roundTripper{base}
	return func() { http.DefaultTransport = prev }
}

type roundTripper struct{ base http.RoundTripper }

func (t roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	Apply(req.Header)
	return t.base.RoundTrip(req)
}

// invokedPath is the command path below the root, using the alias the user
// typed at each level (so alias use can be measured before removing one).
func invokedPath(cmd *cobra.Command, args []string) string {
	var parts []string
	for c := cmd; c.HasParent(); c = c.Parent() {
		parts = append([]string{invokedName(c, args)}, parts...)
	}
	return strings.Join(parts, " ")
}

func invokedName(c *cobra.Command, args []string) string {
	if name := c.CalledAs(); name != "" {
		return name
	}
	for _, alias := range c.Aliases {
		for _, arg := range args {
			if arg == alias {
				return alias
			}
		}
	}
	return c.Name()
}

func detectAgent(getenv func(string) string) string {
	switch {
	case getenv("CLAUDECODE") != "":
		return "claude-code"
	case getenv("CODEX_SANDBOX") != "" || getenv("CODEX_SANDBOX_NETWORK_DISABLED") != "":
		return "codex"
	case getenv("CURSOR_AGENT") != "":
		return "cursor"
	default:
		return "none"
	}
}

func sdkVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/langchain-ai/langsmith-go" {
			return strings.TrimPrefix(dep.Version, "v")
		}
	}
	return ""
}
