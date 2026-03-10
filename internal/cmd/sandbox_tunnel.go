package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/spf13/cobra"

	"github.com/langchain-ai/langsmith-cli/internal/tunnel"
)

func newSandboxTunnelCmd() *cobra.Command {
	var (
		sandboxURL string
		remotePort int
		localPort  int
		logLevel   string
	)

	cmd := &cobra.Command{
		Use:   "tunnel",
		Short: "Create a TCP tunnel to a service inside a sandbox",
		Long: `Create a TCP tunnel from a local port to a port inside a remote sandbox.

The tunnel multiplexes many TCP connections over a single WebSocket,
so you can connect tools like psql, redis-cli, or curl to services
running in the sandbox as if they were local.

Examples:
  langsmith sandbox tunnel --url https://sandboxes.langsmith.com/my-sandbox --remote-port 5432
  langsmith sandbox tunnel --url https://sandboxes.langsmith.com/my-sandbox --remote-port 5432 --local-port 15432`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sandboxURL == "" {
				return fmt.Errorf("--url is required")
			}
			if remotePort < 1 || remotePort > 65535 {
				return fmt.Errorf("--remote-port must be between 1 and 65535 (got %d)", remotePort)
			}
			if localPort == 0 {
				localPort = remotePort
			}
			if localPort < 1 || localPort > 65535 {
				return fmt.Errorf("--local-port must be between 1 and 65535 (got %d)", localPort)
			}

			apiKey := getAPIKey()
			if apiKey == "" {
				return fmt.Errorf("LANGSMITH_API_KEY not set (use --api-key or $LANGSMITH_API_KEY)")
			}

			logger := newTunnelLogger(logLevel)

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
			defer cancel()

			return runTunnel(ctx, logger, sandboxURL, apiKey, remotePort, localPort)
		},
	}

	cmd.Flags().StringVar(&sandboxURL, "url", "", "Sandbox URL (e.g. https://sandboxes.langsmith.com/my-sandbox)")
	cmd.Flags().IntVar(&remotePort, "remote-port", 0, "Port inside the sandbox to tunnel to")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port to listen on (defaults to remote-port)")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")

	_ = cmd.MarkFlagRequired("url")
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

func runTunnel(ctx context.Context, logger *slog.Logger, sandboxURL, apiKey string, remotePort, localPort int) error {
	session, err := connectTunnel(ctx, logger, sandboxURL, apiKey)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = session.Close() }()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = ln.Close() }()

	logger.Info("tunnel ready",
		"local", fmt.Sprintf("127.0.0.1:%d", localPort),
		"remote_port", remotePort,
		"sandbox_url", sandboxURL,
	)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		tcpConn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		go handleLocalConn(ctx, logger, session, tcpConn, uint16(remotePort))
	}
}

// connectTunnel dials a WebSocket to the sandbox and establishes a yamux
// session for multiplexing TCP streams.
func connectTunnel(ctx context.Context, logger *slog.Logger, sandboxURL, apiKey string) (*yamux.Session, error) {
	parsed, err := url.Parse(strings.TrimRight(sandboxURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid sandbox URL: %w", err)
	}

	wsScheme := "wss"
	if parsed.Scheme == "http" {
		wsScheme = "ws"
	}

	wsURL := fmt.Sprintf("%s://%s%s/tunnel", wsScheme, parsed.Host, parsed.Path)
	logger.Debug("dialing tunnel", "url", wsURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	header := http.Header{}
	header.Set("X-Api-Key", apiKey)

	wsConn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("websocket dial (%d): %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	adapter := tunnel.NewWSAdapter(wsConn)

	cfg := yamux.DefaultConfig()
	cfg.KeepAliveInterval = 30 * time.Second
	cfg.ConnectionWriteTimeout = 30 * time.Second
	cfg.MaxStreamWindowSize = 256 * 1024
	cfg.LogOutput = io.Discard

	session, err := yamux.Client(adapter, cfg)
	if err != nil {
		_ = wsConn.Close()
		return nil, fmt.Errorf("yamux client: %w", err)
	}

	logger.Info("tunnel session established", "url", wsURL)
	return session, nil
}

func handleLocalConn(ctx context.Context, logger *slog.Logger, session *yamux.Session, tcpConn net.Conn, remotePort uint16) {
	stream, err := session.OpenStream()
	if err != nil {
		logger.Error("failed to open stream", "error", err)
		_ = tcpConn.Close()
		return
	}

	if err := tunnel.WriteConnectHeader(stream, remotePort); err != nil {
		logger.Error("failed to write connect header", "error", err)
		_ = stream.Close()
		_ = tcpConn.Close()
		return
	}

	status, err := tunnel.ReadStatus(stream)
	if err != nil {
		logger.Error("failed to read status", "error", err)
		_ = stream.Close()
		_ = tcpConn.Close()
		return
	}

	if status != tunnel.StatusOK {
		reason := "unknown"
		switch status {
		case tunnel.StatusPortNotAllowed:
			reason = "port not allowed"
		case tunnel.StatusDialFailed:
			reason = "dial failed (is the service running?)"
		case tunnel.StatusUnsupportedVersion:
			reason = "unsupported protocol version (client/server mismatch)"
		}
		logger.Error("tunnel rejected", "status", status, "reason", reason, "port", remotePort)
		_ = stream.Close()
		_ = tcpConn.Close()
		return
	}

	logger.Debug("stream connected", "remote_port", remotePort, "local_addr", tcpConn.RemoteAddr())
	tunnel.Bridge(ctx, stream, tcpConn)
}
