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
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go/lib/connector"
	"github.com/spf13/cobra"
)

const (
	connectorEnvConfig   = "LANGSMITH_CONNECTOR_CONFIG"
	connectorEnvLogLevel = "LOG_LEVEL"

	connectorInitialReconnectBackoff = 500 * time.Millisecond
	connectorMaxReconnectBackoff     = 15 * time.Second
	connectorHealthyCarrierMinUptime = 60 * time.Second
)

type connectorRunner interface {
	Run(context.Context) error
}

type connectorServiceDeps struct {
	loadConfig       func(path string) (connector.Config, error)
	newClient        func(connector.Config, *slog.Logger) (connectorRunner, error)
	reconnectBackoff time.Duration
}

func defaultConnectorServiceDeps() connectorServiceDeps {
	return connectorServiceDeps{
		loadConfig: loadConnectorConfig,
		newClient: func(config connector.Config, logger *slog.Logger) (connectorRunner, error) {
			return connector.NewClient(config, connector.WithLogger(logger))
		},
		reconnectBackoff: connectorInitialReconnectBackoff,
	}
}

type connectorRunInput struct {
	Config string
}

func newConnectorCmd(version, build string) *cobra.Command {
	run := structured.Command[*connectorRunInput]{
		Use:   "run [--config <path>]",
		Short: "Run a connector configured by environment variables",
		Long: `Run a LangSmith connector until it is stopped, configured by environment
variables and an optional YAML file, as a container or system service does.
The langchain/langsmith-connector image runs this command.

Environment variables win over file values:

  LANGSMITH_ENDPOINT               LangSmith API URL (default the SaaS URL)
  LANGSMITH_API_KEY                the API key itself
  LANGSMITH_API_KEY_FILE           a file holding the key, re-read on every
                                   reconnect so a rotated key needs no restart
  LANGSMITH_CONNECTION_ID          the tunnel or relay this connector serves
  LANGSMITH_CONNECTOR_TARGETS      name=host:port pairs, comma separated, for a tunnel
  LANGSMITH_CONNECTOR_RELAY_MODE   true to serve a relay
  LANGSMITH_CONNECTOR_RELAY_ALLOW  relay allow rules, comma separated
  LANGSMITH_CONNECTOR_RELAY_DENY   relay deny rules, comma separated
  LANGSMITH_CONNECTOR_CONFIG       the YAML file, instead of --config
  LOG_LEVEL                        debug, info, warn, or error (default info)

Logs are JSON on stdout. For an interactive session, use tunnel connect or
relay connect instead.`,
		Args: cobra.NoArgs,
		Input: func(cmd *cobra.Command) *connectorRunInput {
			in := &connectorRunInput{}
			cmd.Flags().StringVar(&in.Config, "config", os.Getenv(connectorEnvConfig),
				"YAML config file [env: "+connectorEnvConfig+"]")
			return in
		},
		CustomOutput: true,
		Action: func(ctx context.Context, cmd *cobra.Command, in *connectorRunInput, _ []string) (any, error) {
			// The logger exists before anything can fail, so every fatal error is
			// a JSON record like the rest of the connector's output.
			level, levelErr := parseConnectorLogLevel(os.Getenv(connectorEnvLogLevel))
			if levelErr != nil {
				level = slog.LevelInfo
			}
			logger := newConnectorLogger(cmd.OutOrStdout(), level).With("version", version, "build", build)
			if levelErr != nil {
				return nil, connectorExit(logger, levelErr)
			}
			ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			if err := runConnectorService(ctx, in.Config, defaultConnectorServiceDeps(), logger); err != nil {
				return nil, connectorExit(logger, err)
			}
			return nil, nil
		},
	}
	return structured.Parent{
		Use:      "connector",
		Short:    "Run a LangSmith connector as a long-running service",
		Children: []func() *cobra.Command{run.Cobra},
	}.Cobra()
}

// connectorExit logs a fatal error as the connector's last JSON record and
// returns it marked as reported, so the CLI exits without printing it again.
func connectorExit(logger *slog.Logger, err error) error {
	logger.Error("langsmith connector exiting", "error", err)
	return ReportedError{Err: err}
}

// runConnectorService runs the connector until ctx is cancelled. It reconnects
// on transient carrier loss (an edge or database rollout, for example) so the
// connector rides through it; a cancelled context stops it after Run has
// drained.
func runConnectorService(ctx context.Context, configPath string, deps connectorServiceDeps, logger *slog.Logger) error {
	config, err := deps.loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load connector config: %w", err)
	}
	client, err := deps.newClient(config, logger)
	if err != nil {
		return fmt.Errorf("create connector client: %w", err)
	}
	logger.InfoContext(ctx, "langsmith connector starting")
	defer logger.InfoContext(context.WithoutCancel(ctx), "langsmith connector stopped")

	initialBackoff := max(deps.reconnectBackoff, connectorInitialReconnectBackoff)
	backoff := initialBackoff
	for {
		start := time.Now()
		runErr := client.Run(ctx)
		if ctx.Err() != nil || runErr == nil {
			return nil
		}
		// A blip after a long healthy period reconnects promptly rather than
		// waiting out backoff accrued during earlier startup failures.
		if time.Since(start) >= connectorHealthyCarrierMinUptime {
			backoff = initialBackoff
		}
		logger.WarnContext(ctx, "connector carrier ended; reconnecting", "error", runErr, "backoff", backoff.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, connectorMaxReconnectBackoff)
	}
}

// parseConnectorLogLevel reads LOG_LEVEL: debug, info, warn, or error in any
// case, defaulting to info when unset. Any other value stops startup so a typo
// cannot hide the connector's registration logs.
func parseConnectorLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%s must be debug, info, warn, or error, got %q", connectorEnvLogLevel, raw)
	}
}

// newConnectorLogger writes JSON records at level, so log pipelines parse
// connector logs the way they parse other LangSmith services'.
func newConnectorLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
