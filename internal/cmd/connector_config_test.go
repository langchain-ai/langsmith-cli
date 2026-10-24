package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-go/lib/connector"
	"github.com/stretchr/testify/require"
)

// clearConnectorEnv isolates a test from connector environment variables set
// on the machine or CI runner, since they win over file values. t.Setenv
// forbids t.Parallel, so callers of this helper run serially.
func clearConnectorEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		connectorEnvEndpoint, connectorEnvAPIKey, connectorEnvAPIKeyFile, connectorEnvConnectionID,
		connectorEnvTargets, connectorEnvRelayMode, connectorEnvRelayAllow, connectorEnvRelayDeny,
		connectorEnvConfig, connectorEnvLogLevel,
	} {
		t.Setenv(name, "")
	}
}

func writeConnectorTestFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func validConnectorConfigYAML(apiKeyFile string) string {
	return fmt.Sprintf(`endpoint: https://api.example.com
connection_id: %s
api_key_file: %q
targets:
  database: db.internal:5432
`, testConnectionID, apiKeyFile)
}

func lookupConnectorTarget(t *testing.T, config connector.Config, name string) string {
	t.Helper()
	address, ok := config.Targets.Lookup(name)
	require.True(t, ok)
	return address
}

func readConnectorAPIKey(t *testing.T, config connector.Config) string {
	t.Helper()
	apiKey, err := config.ReadAPIKey()
	require.NoError(t, err)
	return apiKey
}

func resolveConnectorConfig(t *testing.T, file connectorFileConfig, env map[string]string) (connector.Config, error) {
	t.Helper()
	settings, err := resolveConnectorSettings(file, func(name string) string { return env[name] })
	if err != nil {
		return connector.Config{}, err
	}
	return connector.NewConfig(settings)
}

func TestLoadConnectorConfigReadsYAMLFile(t *testing.T) {
	clearConnectorEnv(t)
	apiKeyFile := writeConnectorTestFile(t, "api-key", "placeholder")
	configFile := writeConnectorTestFile(t, "connector.yaml", validConnectorConfigYAML(apiKeyFile))

	config, err := loadConnectorConfig(configFile)
	require.NoError(t, err)
	require.Equal(t, "wss://api.example.com/v1/connections/connect", config.Endpoint)
	require.Equal(t, uuid.MustParse(testConnectionID), config.ConnectionID)
	require.Equal(t, "db.internal:5432", lookupConnectorTarget(t, config, "database"))
}

func TestLoadConnectorConfigRejectsUnknownAndDuplicateFields(t *testing.T) {
	clearConnectorEnv(t)
	apiKeyFile := writeConnectorTestFile(t, "api-key", "placeholder")
	for name, data := range map[string]string{
		"unknown key":   validConnectorConfigYAML(apiKeyFile) + "unknown_key: forbidden\n",
		"duplicate key": "endpoint: https://other.example.com\n" + validConnectorConfigYAML(apiKeyFile),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadConnectorConfig(writeConnectorTestFile(t, "connector.yaml", data))
			require.Error(t, err)
		})
	}
}

func TestLoadConnectorConfigRejectsUnreadableYAMLFile(t *testing.T) {
	clearConnectorEnv(t)
	_, err := loadConnectorConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
}

// TestLoadConnectorConfigWithoutAFileUsesTheEnvironment is the Helm path: no
// config file on disk, everything supplied as environment variables.
func TestLoadConnectorConfigWithoutAFileUsesTheEnvironment(t *testing.T) {
	clearConnectorEnv(t)
	t.Setenv(connectorEnvEndpoint, "https://langsmith.corp.example.com/api")
	t.Setenv(connectorEnvAPIKey, "lsv2_placeholder")
	t.Setenv(connectorEnvConnectionID, testConnectionID)
	t.Setenv(connectorEnvTargets, "database=db.internal:5432, cache=redis.internal:6379")

	config, err := loadConnectorConfig("")
	require.NoError(t, err)
	require.Equal(t, "wss://langsmith.corp.example.com/api/v1/connections/connect", config.Endpoint)
	require.Equal(t, []string{"cache", "database"}, config.Targets.Names())
	require.Equal(t, "redis.internal:6379", lookupConnectorTarget(t, config, "cache"))
	require.Equal(t, "lsv2_placeholder", readConnectorAPIKey(t, config))
}

func TestResolveConnectorSettingsPrefersEnvironmentOverFile(t *testing.T) {
	t.Parallel()

	file := connectorFileConfig{
		Endpoint:     "https://file.example.com",
		ConnectionID: testConnectionID,
		APIKeyFile:   writeConnectorTestFile(t, "api-key", "from-file-placeholder\n"),
		Targets:      map[string]string{"database": "db.internal:5432"},
	}
	config, err := resolveConnectorConfig(t, file, map[string]string{
		connectorEnvEndpoint:     "https://env.example.com",
		connectorEnvConnectionID: "00000000-0000-7000-8000-000000000002",
		connectorEnvTargets:      "cache=redis.internal:6379",
	})
	require.NoError(t, err)
	require.Equal(t, "wss://env.example.com/v1/connections/connect", config.Endpoint)
	require.Equal(t, uuid.MustParse("00000000-0000-7000-8000-000000000002"), config.ConnectionID)
	require.Equal(t, []string{"cache"}, config.Targets.Names())
	// The file still supplies the credential when the environment does not.
	require.Equal(t, "from-file-placeholder", readConnectorAPIKey(t, config))
}

func TestResolveConnectorSettingsCredentialPrecedence(t *testing.T) {
	t.Parallel()

	fileKeyPath := writeConnectorTestFile(t, "file-api-key", "from-file-placeholder\n")
	envKeyPath := writeConnectorTestFile(t, "env-api-key", "from-environment-file-placeholder\n")
	tests := []struct {
		name string
		file connectorFileConfig
		env  map[string]string
		want string
	}{
		{
			name: "environment key over file key path",
			file: connectorFileConfig{APIKeyFile: fileKeyPath},
			env:  map[string]string{connectorEnvAPIKey: "from-environment-placeholder"},
			want: "from-environment-placeholder",
		},
		{
			name: "environment key path over inline file key",
			file: connectorFileConfig{APIKey: "from-config-placeholder"},
			env:  map[string]string{connectorEnvAPIKeyFile: envKeyPath},
			want: "from-environment-file-placeholder",
		},
		{
			name: "inline file key over file key path",
			file: connectorFileConfig{APIKey: "from-config-placeholder", APIKeyFile: fileKeyPath},
			want: "from-config-placeholder",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			file := test.file
			file.ConnectionID = testConnectionID
			file.Targets = map[string]string{"database": "db.internal:5432"}
			config, err := resolveConnectorConfig(t, file, test.env)
			require.NoError(t, err)
			require.Equal(t, test.want, readConnectorAPIKey(t, config))
		})
	}
}

func TestResolveConnectorSettingsRejectsIncompleteSettings(t *testing.T) {
	t.Parallel()

	targets := map[string]string{"database": "db.internal:5432"}
	tests := []struct {
		name    string
		file    connectorFileConfig
		env     map[string]string
		wantErr string
	}{
		{
			name:    "no connection ID",
			file:    connectorFileConfig{APIKey: "placeholder", Targets: targets},
			wantErr: "connection ID is required",
		},
		{
			name:    "no credential",
			file:    connectorFileConfig{ConnectionID: testConnectionID, Targets: targets},
			wantErr: "API key is required",
		},
		{
			name:    "conflicting environment credentials",
			file:    connectorFileConfig{ConnectionID: testConnectionID, Targets: targets},
			env:     map[string]string{connectorEnvAPIKey: "placeholder", connectorEnvAPIKeyFile: "/run/secrets/langsmith-api-key"},
			wantErr: "cannot both be set",
		},
		{
			name:    "relay rules without relay mode",
			file:    connectorFileConfig{ConnectionID: testConnectionID, APIKey: "placeholder", Targets: targets},
			env:     map[string]string{connectorEnvRelayDeny: "*.example.com"},
			wantErr: "require relay mode",
		},
		{
			name:    "relay mode that is not a boolean",
			file:    connectorFileConfig{ConnectionID: testConnectionID, APIKey: "placeholder"},
			env:     map[string]string{connectorEnvRelayMode: "sometimes"},
			wantErr: "must be true or false",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveConnectorConfig(t, test.file, test.env)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestResolveConnectorSettingsRelayFromTheEnvironment(t *testing.T) {
	t.Parallel()

	config, err := resolveConnectorConfig(t, connectorFileConfig{}, map[string]string{
		connectorEnvConnectionID: testConnectionID,
		connectorEnvAPIKey:       "placeholder",
		connectorEnvRelayMode:    "true",
		connectorEnvRelayAllow:   "10.0.0.0/8, *.corp.internal:5432",
	})
	require.NoError(t, err)
	require.Equal(t, []string{connector.ProxyTargetName}, config.Targets.Names())
}

func TestParseConnectorTargetListRejectsMalformedEntries(t *testing.T) {
	t.Parallel()

	targets, err := parseConnectorTargetList("database=db.internal:5432, cache=redis.internal:6379,")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"database": "db.internal:5432", "cache": "redis.internal:6379"}, targets)

	for _, raw := range []string{"database", "database=db:5432,database=other:5432"} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			_, err := parseConnectorTargetList(raw)
			require.Error(t, err)
		})
	}
}
