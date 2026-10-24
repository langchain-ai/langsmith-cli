package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/langchain-ai/langsmith-go/lib/connector"
)

const (
	connectorEnvEndpoint     = "LANGSMITH_ENDPOINT"
	connectorEnvAPIKey       = "LANGSMITH_API_KEY"
	connectorEnvAPIKeyFile   = "LANGSMITH_API_KEY_FILE"
	connectorEnvConnectionID = "LANGSMITH_CONNECTION_ID"
	connectorEnvTargets      = "LANGSMITH_CONNECTOR_TARGETS"
	connectorEnvRelayMode    = "LANGSMITH_CONNECTOR_RELAY_MODE"
	connectorEnvRelayAllow   = "LANGSMITH_CONNECTOR_RELAY_ALLOW"
	connectorEnvRelayDeny    = "LANGSMITH_CONNECTOR_RELAY_DENY"
)

type connectorFileConfig struct {
	Endpoint     string                   `yaml:"endpoint"`
	ConnectionID string                   `yaml:"connection_id"`
	APIKey       string                   `yaml:"api_key"`
	APIKeyFile   string                   `yaml:"api_key_file"`
	Targets      map[string]string        `yaml:"targets"`
	Relay        connectorFileRelayConfig `yaml:"relay"`
}

type connectorFileRelayConfig struct {
	Enabled bool     `yaml:"enabled"`
	Allow   []string `yaml:"allow"`
	Deny    []string `yaml:"deny"`
}

// loadConnectorConfig resolves connector settings from the environment,
// optionally layered over a YAML file. Every setting can come from either
// source, so a deployment can supply the API key from a secret, the targets from
// a config map, and nothing else. Environment variables win over file values:
//
//	LANGSMITH_ENDPOINT            LangSmith API URL (default the SaaS URL)
//	LANGSMITH_API_KEY             the API key itself
//	LANGSMITH_API_KEY_FILE        path to a file holding the key, re-read on
//	                              every carrier attempt so rotation needs no restart
//	LANGSMITH_CONNECTION_ID       the connection this connector serves
//	LANGSMITH_CONNECTOR_TARGETS   name=host:port pairs, comma separated, for
//	                              a tunnel
//	LANGSMITH_CONNECTOR_RELAY_MODE  "true" to serve a relay instead: LangSmith
//	                              names each destination, and traffic to it
//	                              leaves from this connector's network
//	LANGSMITH_CONNECTOR_RELAY_ALLOW  relay allow rules, comma separated
//	LANGSMITH_CONNECTOR_RELAY_DENY   relay deny rules, comma separated
//
// path may be empty, in which case the environment must supply everything.
func loadConnectorConfig(path string) (connector.Config, error) {
	var file connectorFileConfig
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return connector.Config{}, fmt.Errorf("read connector config: %w", err)
		}
		if err := yaml.UnmarshalWithOptions(data, &file, yaml.DisallowUnknownField()); err != nil {
			return connector.Config{}, fmt.Errorf("parse connector config: %w", err)
		}
	}
	settings, err := resolveConnectorSettings(file, os.Getenv)
	if err != nil {
		return connector.Config{}, err
	}
	return connector.NewConfig(settings)
}

// resolveConnectorSettings overlays environment variables onto file values.
func resolveConnectorSettings(file connectorFileConfig, getenv func(string) string) (connector.Settings, error) {
	settings := connector.Settings{
		Endpoint:     firstNonEmptyString(getenv(connectorEnvEndpoint), file.Endpoint),
		ConnectionID: firstNonEmptyString(getenv(connectorEnvConnectionID), file.ConnectionID),
		Targets:      file.Targets,
		Relay:        file.Relay.Enabled,
		RelayAllow:   file.Relay.Allow,
		RelayDeny:    file.Relay.Deny,
	}
	if settings.ConnectionID == "" {
		return connector.Settings{}, fmt.Errorf("connection ID is required (set %s or connection_id)", connectorEnvConnectionID)
	}

	environmentAPIKey := strings.TrimSpace(getenv(connectorEnvAPIKey))
	environmentAPIKeyFile := getenv(connectorEnvAPIKeyFile)
	switch {
	case environmentAPIKey != "" && environmentAPIKeyFile != "":
		return connector.Settings{}, fmt.Errorf("%s and %s cannot both be set", connectorEnvAPIKey, connectorEnvAPIKeyFile)
	case environmentAPIKey != "":
		settings.APIKey = environmentAPIKey
	case environmentAPIKeyFile != "":
		settings.APIKeyFile = environmentAPIKeyFile
	case strings.TrimSpace(file.APIKey) != "":
		settings.APIKey = file.APIKey
	default:
		settings.APIKeyFile = file.APIKeyFile
	}
	if settings.APIKey == "" && settings.APIKeyFile == "" {
		return connector.Settings{}, fmt.Errorf("API key is required (set %s, %s, api_key, or api_key_file)", connectorEnvAPIKey, connectorEnvAPIKeyFile)
	}

	if fromEnv := getenv(connectorEnvTargets); fromEnv != "" {
		targets, err := parseConnectorTargetList(fromEnv)
		if err != nil {
			return connector.Settings{}, err
		}
		settings.Targets = targets
	}
	if fromEnv := strings.TrimSpace(getenv(connectorEnvRelayMode)); fromEnv != "" {
		enabled, err := strconv.ParseBool(fromEnv)
		if err != nil {
			return connector.Settings{}, fmt.Errorf("%s must be true or false", connectorEnvRelayMode)
		}
		settings.Relay = enabled
	}
	if fromEnv := getenv(connectorEnvRelayAllow); fromEnv != "" {
		settings.RelayAllow = splitCommaList(fromEnv)
	}
	if fromEnv := getenv(connectorEnvRelayDeny); fromEnv != "" {
		settings.RelayDeny = splitCommaList(fromEnv)
	}
	if !settings.Relay && (len(settings.RelayAllow) > 0 || len(settings.RelayDeny) > 0) {
		return connector.Settings{}, fmt.Errorf("relay allow and deny rules require relay mode (set %s=true or relay.enabled)", connectorEnvRelayMode)
	}
	return settings, nil
}

// parseConnectorTargetList parses "name=host:port,other=host:port" into a
// target map.
func parseConnectorTargetList(raw string) (map[string]string, error) {
	targets := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, address, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("%s entries must be name=host:port, got %q", connectorEnvTargets, entry)
		}
		name = strings.TrimSpace(name)
		if _, duplicate := targets[name]; duplicate {
			return nil, fmt.Errorf("%s lists target %q twice", connectorEnvTargets, name)
		}
		targets[name] = strings.TrimSpace(address)
	}
	return targets, nil
}

func splitCommaList(raw string) []string {
	var values []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
