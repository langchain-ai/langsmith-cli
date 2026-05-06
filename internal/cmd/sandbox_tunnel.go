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

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newSandboxTunnelCmd() *cobra.Command {
	var (
		sandboxURL  string
		sandboxName string
		remotePort  int
		localPort   int
		logLevel    string
		stdio       bool
	)

	cmd := &cobra.Command{
		Use:   "tunnel <name> --remote-port <port>",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve name: positional arg takes precedence over --name flag.
			name := sandboxName
			if len(args) > 0 {
				name = args[0]
			}
			if name == "" && sandboxURL == "" {
				return fmt.Errorf("provide a sandbox name or --url")
			}
			client := MustGetClient()
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancel()
			if sandboxURL == "" {
				box, err := client.SDK.Sandboxes.Boxes.Get(ctx, name)
				if err != nil {
					return fmt.Errorf("getting sandbox: %w", err)
				}
				if box.DataplaneURL == "" {
					return fmt.Errorf("sandbox %q has no dataplane URL", name)
				}
				sandboxURL = box.DataplaneURL
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("--remote-port must be between 1 and 65535 (got %d)", remotePort)
			}

			if stdio {
				logger := slog.New(slog.NewTextHandler(io.Discard, nil))
				return runTunnelStdio(ctx, logger, client.SDK, sandboxURL, remotePort)
			}

			if localPort == 0 {
				localPort = remotePort
			}
			if localPort < 1 || localPort > 65535 {
				return fmt.Errorf("--local-port must be between 1 and 65535 (got %d)", localPort)
			}

			logger := newTunnelLogger(logLevel)

			return runTunnel(ctx, logger, client.SDK, sandboxURL, remotePort, localPort)
		},
	}

	cmd.Flags().StringVar(&sandboxURL, "url", "", "Sandbox URL (override; skips name resolution)")
	cmd.Flags().StringVar(&sandboxName, "name", "", "")
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "Port inside the sandbox to tunnel to")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port to listen on (defaults to remote-port)")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	cmd.Flags().BoolVar(&stdio, "stdio", false, "Bridge stdin/stdout to the remote port (for use as SSH ProxyCommand)")

	_ = cmd.Flags().MarkHidden("name")
	_ = cmd.MarkFlagRequired("remote-port")

	return cmd
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

	logger.Info("tunnel ready",
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

	logger.Debug("tunnel stream established", "remote_port", remotePort, "sandbox_url", sandboxURL)
	return langsmith.BridgeSandboxTunnelIO(ctx, stream, os.Stdin, os.Stdout)
}
