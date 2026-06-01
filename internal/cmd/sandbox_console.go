//go:build !windows

package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type sandboxConsoleInput struct {
	Shell           string
	ForwardSSHAgent bool
	Env             []string
}

var sandboxConsoleCommand = structured.Command[*sandboxConsoleInput]{
	Use:   "console <name>",
	Short: "Open an interactive shell inside a sandbox",
	Long: `Open an interactive terminal session inside a running sandbox.

Connects via WebSocket to the sandbox daemon and allocates a PTY,
giving you a full interactive shell (bash by default).

Examples:
  langsmith sandbox console my-vm
  langsmith sandbox console my-vm --shell /bin/sh
  langsmith sandbox console my-vm --env TERM=xterm-256color --env FOO
  langsmith sandbox console my-vm --forward-ssh-agent`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *sandboxConsoleInput {
		in := &sandboxConsoleInput{}
		cmd.Flags().StringVar(&in.Shell, "shell", in.Shell, "Shell to use (default: sandbox default, usually /bin/bash)")
		cmd.Flags().BoolVar(&in.ForwardSSHAgent, "forward-ssh-agent", in.ForwardSSHAgent, "Forward the local SSH agent (SSH_AUTH_SOCK) into the sandbox")
		cmd.Flags().StringArrayVar(&in.Env, "env", nil, "Additional environment variable to pass into the sandbox (KEY or KEY=VALUE, repeatable)")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxConsoleInput, args []string) (any, error) {
		return nil, runConsole(args[0], in.Shell, in.ForwardSSHAgent, in.Env)
	},
}

func newSandboxConsoleCmd() *cobra.Command {
	return sandboxConsoleCommand.Cobra()
}

func sandboxConsoleEnv(extra []string) (map[string]string, error) {
	return sandboxConsoleEnvFrom(os.Environ(), extra)
}

func sandboxConsoleEnvFrom(environ []string, extra []string) (map[string]string, error) {
	env := make(map[string]string)
	local := make(map[string]string)
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			continue
		}
		local[key] = value
		if value != "" && sandboxConsoleEnvAllowed(key) {
			env[key] = value
		}
	}
	for _, spec := range extra {
		key, value, hasValue := strings.Cut(spec, "=")
		if key == "" {
			return nil, fmt.Errorf("--env must be KEY or KEY=VALUE")
		}
		if hasValue {
			env[key] = value
			continue
		}
		value, ok := local[key]
		if !ok {
			return nil, fmt.Errorf("--env %s is not set in the local environment", key)
		}
		env[key] = value
	}
	return env, nil
}

func sandboxConsoleEnvAllowed(key string) bool {
	switch key {
	case "TERM", "COLORTERM", "LANG", "CLICOLOR", "CLICOLOR_FORCE", "FORCE_COLOR", "NO_COLOR":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}

func runConsole(name, shell string, forwardSSHAgent bool, extraEnv []string) error {
	client := MustGetClient()
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	agentSock := ""
	if forwardSSHAgent {
		agentSock = os.Getenv("SSH_AUTH_SOCK")
		if agentSock == "" {
			return fmt.Errorf("--forward-ssh-agent requires SSH_AUTH_SOCK to be set (is ssh-agent running?)")
		}
	}

	agent := newConsoleAgentForwarder(agentSock)
	defer agent.Close()

	var handle *langsmith.SandboxCommandHandle
	handleReady := make(chan struct{})
	callbacks := langsmith.SandboxCommandCallbacks{
		OnSSHAgentData: func(channelID string, data []byte) {
			<-handleReady
			agent.HandleData(handle, channelID, data)
		},
		OnSSHAgentClose: func(channelID string) {
			agent.CloseChannel(channelID)
		},
	}

	params := langsmith.SandboxCommandStartParams{
		Pty:                langsmith.Bool(true),
		TimeoutSeconds:     langsmith.Int(0),
		IdleTimeoutSeconds: langsmith.Int(-1),
		SSHAgentForward:    langsmith.Bool(forwardSSHAgent),
	}
	env, err := sandboxConsoleEnv(extraEnv)
	if err != nil {
		return err
	}
	if len(env) > 0 {
		params.Env = langsmith.F(env)
	}
	if shell != "" {
		params.Shell = langsmith.String(shell)
	}
	handle, err = client.SDK.Sandboxes.Boxes.StartCommandWithCallbacks(ctx, name, params, callbacks)
	close(handleReady)
	if err != nil {
		return fmt.Errorf("starting console: %w", err)
	}
	defer handle.Close()

	// Put terminal in raw mode.
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("make raw: %w", err)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	// Send initial terminal size.
	sendResize(handle, fd)

	// Handle SIGWINCH (terminal resize).
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			sendResize(handle, fd)
		}
	}()

	// Read from stdin into the SDK command stream.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			if err := handle.SendInput(string(buf[:n])); err != nil {
				return
			}
		}
	}()

	for {
		chunk, ok, err := handle.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if !ok {
			_, err := handle.Result(ctx)
			return err
		}
		_, _ = os.Stdout.WriteString(chunk.Data)
	}
}

func sendResize(handle *langsmith.SandboxCommandHandle, fd int) {
	w, h, err := term.GetSize(fd)
	if err != nil {
		return
	}
	_ = handle.Resize(w, h)
}

type consoleAgentForwarder struct {
	sock  string
	mu    sync.Mutex
	conns map[string]net.Conn
}

func newConsoleAgentForwarder(sock string) *consoleAgentForwarder {
	return &consoleAgentForwarder{
		sock:  sock,
		conns: make(map[string]net.Conn),
	}
}

func (a *consoleAgentForwarder) HandleData(handle *langsmith.SandboxCommandHandle, channelID string, data []byte) {
	if a.sock == "" {
		return
	}

	a.mu.Lock()
	conn, ok := a.conns[channelID]
	if !ok {
		var err error
		conn, err = net.Dial("unix", a.sock)
		if err != nil {
			a.mu.Unlock()
			return
		}
		a.conns[channelID] = conn
		go a.readResponses(handle, channelID, conn)
	}
	a.mu.Unlock()

	_, _ = conn.Write(data)
}

func (a *consoleAgentForwarder) readResponses(handle *langsmith.SandboxCommandHandle, channelID string, conn net.Conn) {
	buf := make([]byte, 16384)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			_ = handle.CloseSSHAgentChannel(channelID)
			return
		}
		_ = handle.SendSSHAgentData(channelID, buf[:n])
	}
}

func (a *consoleAgentForwarder) CloseChannel(channelID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if conn, ok := a.conns[channelID]; ok {
		_ = conn.Close()
		delete(a.conns, channelID)
	}
}

func (a *consoleAgentForwarder) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for channelID, conn := range a.conns {
		_ = conn.Close()
		delete(a.conns, channelID)
	}
}
