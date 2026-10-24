package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go/lib/connector"
	"github.com/spf13/cobra"
)

type tunnelConnectInput struct {
	Targets  []string
	LogLevel string
}

var tunnelConnectCommand = structured.Command[*tunnelConnectInput]{
	Use:   "connect <connection-id> --target <name=address>",
	Short: "Serve a private network tunnel from this machine",
	Long: `Serve a private network tunnel from this machine until interrupted.

LangSmith reaches each target through this process, so a target only has to
be reachable from here. Create the tunnel connection in LangSmith first and
pass its ID.

A target is name=host:port for raw TCP, or name=http://host:port (or https)
for an HTTP service, which LangSmith also serves at a tunnel URL.

The connector authenticates with an API key: set LANGSMITH_API_KEY or use a
profile with an API key. OAuth logins are not accepted.

Examples:
  langsmith tunnel connect <connection-id> --target app=http://localhost:8080
  langsmith tunnel connect <connection-id> --target db=10.0.0.5:5432 --target api=https://api.internal:443`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *tunnelConnectInput {
		in := &tunnelConnectInput{LogLevel: "info"}
		cmd.Flags().StringSliceVar(&in.Targets, "target", nil, "Target to serve as name=host:port or name=http(s)://host:port (repeatable)")
		cmd.Flags().StringVar(&in.LogLevel, "log-level", in.LogLevel, "Log level: debug, info, warn, error")
		_ = cmd.MarkFlagRequired("target")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *tunnelConnectInput, args []string) (any, error) {
		targets, err := parseConnectorTargets(in.Targets)
		if err != nil {
			return nil, err
		}
		return nil, runConnector(ctx, connector.Settings{ConnectionID: args[0], Targets: targets}, in.LogLevel)
	},
}

type relayConnectInput struct {
	Allow    []string
	Deny     []string
	LogLevel string
}

var relayConnectCommand = structured.Command[*relayConnectInput]{
	Use:   "connect <relay-id>",
	Short: "Serve a LangSmith relay from this machine",
	Long: `Serve a LangSmith relay from this machine until interrupted.

Sandbox connections that match a relay rule leave through this process, so
they reach this machine's network. Create the relay in LangSmith first and
pass its ID.

By default the relay reaches public addresses on ports 80 and 443. --allow
widens that and --deny narrows it. Each rule is a CIDR (10.0.0.0/8), an IP
address, a hostname (db.corp.internal), or a wildcard (*.corp.internal),
optionally followed by :PORT. Cloud metadata addresses are always refused.

The connector authenticates with an API key: set LANGSMITH_API_KEY or use a
profile with an API key. OAuth logins are not accepted.

Examples:
  langsmith relay connect <relay-id> --allow 10.0.0.0/8
  langsmith relay connect <relay-id> --allow db.corp.internal:5432 --deny 10.0.9.0/24`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *relayConnectInput {
		in := &relayConnectInput{LogLevel: "info"}
		cmd.Flags().StringSliceVar(&in.Allow, "allow", nil, "Destinations to allow beyond the default (repeatable)")
		cmd.Flags().StringSliceVar(&in.Deny, "deny", nil, "Destinations to refuse (repeatable)")
		cmd.Flags().StringVar(&in.LogLevel, "log-level", in.LogLevel, "Log level: debug, info, warn, error")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *relayConnectInput, args []string) (any, error) {
		return nil, runConnector(ctx, connector.Settings{
			ConnectionID: args[0],
			Relay:        true,
			RelayAllow:   in.Allow,
			RelayDeny:    in.Deny,
		}, in.LogLevel)
	},
}

func newTunnelCmd() *cobra.Command {
	return structured.Parent{
		Use:      "tunnel",
		Short:    "Serve private network tunnels (experimental)",
		Children: []func() *cobra.Command{tunnelConnectCommand.Cobra},
	}.Cobra()
}

func newRelayCmd() *cobra.Command {
	return structured.Parent{
		Use:      "relay",
		Short:    "Serve LangSmith relays for sandbox traffic (experimental)",
		Children: []func() *cobra.Command{relayConnectCommand.Cobra},
	}.Cobra()
}

// runConnector serves a connection with the CLI's endpoint and API key until
// ctx is cancelled or the process is interrupted.
func runConnector(ctx context.Context, settings connector.Settings, logLevel string) error {
	opts, err := resolveClientOptions()
	if err != nil {
		return err
	}
	apiKey, err := connectorAPIKey(opts)
	if err != nil {
		return err
	}
	if apiKey == "" {
		return errors.New("a connector needs an API key: set LANGSMITH_API_KEY or use a profile with an API key (OAuth logins are not accepted)")
	}
	settings.Endpoint = client.NormalizeURL(opts.APIURL)
	settings.APIKey = apiKey
	config, err := connector.NewConfig(settings)
	if err != nil {
		return err
	}
	logger := newTunnelLogger(logLevel)
	connectorClient, err := connector.NewClient(config, connector.WithLogger(logger))
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	logger.InfoContext(ctx, "connecting to LangSmith",
		"connection_id", config.ConnectionID, "kind", config.Targets.Kind())
	if err := connectorClient.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// connectorAPIKey returns the API key a connector authenticates with. Other
// commands use a profile's OAuth login when it has one, but the edge accepts
// only API keys, so a profile's saved key is used even then.
func connectorAPIKey(opts client.Options) (string, error) {
	if opts.APIKey != "" || opts.ProfileName == "" {
		return opts.APIKey, nil
	}
	cfg, err := lsconfig.Load()
	if err != nil {
		return "", err
	}
	return cfg.Profiles[opts.ProfileName].APIKey, nil
}

// parseConnectorTargets parses --target values of the form name=host:port.
func parseConnectorTargets(entries []string) (map[string]string, error) {
	targets := make(map[string]string, len(entries))
	for _, entry := range entries {
		name, address, found := strings.Cut(entry, "=")
		name, address = strings.TrimSpace(name), strings.TrimSpace(address)
		if !found || name == "" || address == "" {
			return nil, fmt.Errorf("--target must be name=host:port or name=http(s)://host:port, got %q", entry)
		}
		if _, duplicate := targets[name]; duplicate {
			return nil, fmt.Errorf("--target %q is given twice", name)
		}
		targets[name] = address
	}
	return targets, nil
}
