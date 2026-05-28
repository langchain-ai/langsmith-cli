package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

type sandboxTunnelInput struct {
	SandboxURL  string
	SandboxName string
	RemotePort  int
	LocalPort   int
	LogLevel    string
	Stdio       bool
}

var sandboxTunnelCommand = structured.Command[*sandboxTunnelInput]{
	Use:   "tunnel [name] --remote-port <port>",
	Short: "Create a TCP tunnel to a service inside a sandbox",
	Long: `Create a TCP tunnel from a local port to a port inside a remote sandbox.

The tunnel multiplexes many TCP connections over a single WebSocket,
so you can connect tools like psql, redis-cli, or curl to services
running in the sandbox as if they were local.

With --stdio, the tunnel bridges stdin/stdout directly to a single
remote port instead of listening locally. This is designed for use
as an SSH ProxyCommand.

Examples:
  langsmith sandbox tunnel my-vm --remote-port 5432
  langsmith sandbox tunnel my-vm --remote-port 5432 --local-port 15432
  langsmith sandbox tunnel my-vm --remote-port 22 --stdio
  langsmith sandbox tunnel --url https://sandboxes.langsmith.com/my-sandbox --remote-port 5432`,
	Args: cobra.MaximumNArgs(1),
	Input: func(cmd *cobra.Command) *sandboxTunnelInput {
		in := &sandboxTunnelInput{LogLevel: "info"}
		cmd.Flags().StringVar(&in.SandboxURL, "url", in.SandboxURL, "Sandbox URL (override; skips name resolution)")
		cmd.Flags().StringVar(&in.SandboxName, "name", in.SandboxName, "")
		cmd.Flags().IntVar(&in.RemotePort, "remote-port", in.RemotePort, "Port inside the sandbox to tunnel to")
		cmd.Flags().IntVar(&in.LocalPort, "local-port", in.LocalPort, "Local port to listen on (defaults to remote-port)")
		cmd.Flags().StringVar(&in.LogLevel, "log-level", in.LogLevel, "Log level: debug, info, warn, error")
		cmd.Flags().BoolVar(&in.Stdio, "stdio", in.Stdio, "Bridge stdin/stdout to the remote port (for use as SSH ProxyCommand)")
		_ = cmd.Flags().MarkHidden("name")
		_ = cmd.MarkFlagRequired("remote-port")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxTunnelInput, args []string) (any, error) {
		name := in.SandboxName
		if len(args) > 0 {
			name = args[0]
		}
		if name == "" && in.SandboxURL == "" {
			var err error
			name, err = defaultSandboxName(cmd)
			if err != nil {
				return nil, err
			}
		}
		client := MustGetClient()
		ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
		defer cancel()
		sandboxURL := in.SandboxURL
		if sandboxURL == "" {
			box, err := client.SDK.Sandboxes.Boxes.Get(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("getting sandbox: %w", err)
			}
			if box.DataplaneURL == "" {
				return nil, fmt.Errorf("sandbox %q has no dataplane URL", name)
			}
			sandboxURL = box.DataplaneURL
		}
		if in.RemotePort < 1 || in.RemotePort > 65535 {
			return nil, fmt.Errorf("--remote-port must be between 1 and 65535 (got %d)", in.RemotePort)
		}

		if in.Stdio {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			return nil, runTunnelStdio(ctx, logger, client.SDK, sandboxURL, in.RemotePort)
		}

		localPort := in.LocalPort
		if localPort == 0 {
			localPort = in.RemotePort
		}
		if localPort < 1 || localPort > 65535 {
			return nil, fmt.Errorf("--local-port must be between 1 and 65535 (got %d)", localPort)
		}

		logger := newTunnelLogger(in.LogLevel)
		return nil, runTunnel(ctx, logger, client.SDK, sandboxURL, in.RemotePort, localPort)
	},
}

func newSandboxTunnelCmd() *cobra.Command {
	return sandboxTunnelCommand.Cobra()
}

func newTunnelLogger(level string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}

func runTunnel(ctx context.Context, logger *slog.Logger, client *langsmith.Client, sandboxURL string, remotePort, localPort int) error {
	tunnel, err := client.Sandboxes.Boxes.TunnelWithDataplaneURL(ctx, sandboxURL, remotePort, langsmith.SandboxTunnelParams{
		LocalPort: localPort,
	})
	if err != nil {
		return fmt.Errorf("tunnel: %w", err)
	}
	defer func() { _ = tunnel.Close() }()

	logger.InfoContext(ctx, "tunnel ready",
		"local", fmt.Sprintf("127.0.0.1:%d", tunnel.LocalPort),
		"remote_port", remotePort,
		"sandbox_url", sandboxURL,
	)

	<-ctx.Done()
	return nil
}

// runTunnelStdio connects a single tunnel stream and bridges it to stdin/stdout.
// Intended for use as an SSH ProxyCommand.
func runTunnelStdio(ctx context.Context, logger *slog.Logger, client *langsmith.Client, sandboxURL string, remotePort int) error {
	stream, err := client.Sandboxes.Boxes.OpenTunnelStreamWithDataplaneURL(ctx, sandboxURL, remotePort)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = stream.Close() }()

	logger.DebugContext(ctx, "tunnel stream established", "remote_port", remotePort, "sandbox_url", sandboxURL)
	return langsmith.BridgeSandboxTunnelIO(ctx, stream, os.Stdin, os.Stdout)
}
