package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-go/lib/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runFunc func(context.Context) error

func (f runFunc) Run(ctx context.Context) error { return f(ctx) }

// connectorTestDeps returns dependencies whose client runs run.
func connectorTestDeps(run runFunc) connectorServiceDeps {
	return connectorServiceDeps{
		loadConfig: func(string) (connector.Config, error) { return connector.Config{}, nil },
		newClient: func(connector.Config, *slog.Logger) (connectorRunner, error) {
			return run, nil
		},
		reconnectBackoff: time.Millisecond,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestConnectorRunServesFromTheEnvironment is the image's path: everything
// comes from environment variables, as the Helm chart sets them.
func TestConnectorRunServesFromTheEnvironment(t *testing.T) {
	isolateConnectorConfig(t)
	clearConnectorEnv(t)
	path, headers := dialEdge(t, func(edgeURL string) []string {
		t.Setenv(connectorEnvEndpoint, edgeURL)
		t.Setenv(connectorEnvAPIKey, "env-api-key")
		t.Setenv(connectorEnvConnectionID, testConnectionID)
		t.Setenv(connectorEnvRelayMode, "true")
		return []string{"connector", "run"}
	})

	assert.Equal(t, "/v1/connections/connect", path)
	assert.Equal(t, "env-api-key", headers.Get("X-Api-Key"))
	assert.Equal(t, testConnectionID, headers.Get("X-LangSmith-Connection-Id"))
	assert.Equal(t, "relay", headers.Get("X-LangSmith-Connector-Kind"))
}

func TestConnectorRunRejectsArguments(t *testing.T) {
	isolateConnectorConfig(t)
	clearConnectorEnv(t)
	for _, args := range [][]string{
		{"connector", "run", "connector.yaml"},
		{"connector", "run", "--config"},
		{"connector", "run", "--settings", "connector.yaml"},
	} {
		_, err := executeCommand(t, args...)
		assert.Error(t, err, "%v", args)
	}
}

// TestConnectorRunLogsFatalErrorsAsJSON: a deployment's log pipeline parses
// every record, so startup failures are JSON too, and the CLI does not print
// them a second time.
func TestConnectorRunLogsFatalErrorsAsJSON(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{name: "invalid log level", env: map[string]string{connectorEnvLogLevel: "verbose"}, wantErr: "LOG_LEVEL"},
		{name: "no connection ID", env: map[string]string{connectorEnvAPIKey: "placeholder"}, wantErr: "connection ID is required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConnectorConfig(t)
			clearConnectorEnv(t)
			for name, value := range tc.env {
				t.Setenv(name, value)
			}

			output, err := executeCommand(t, "connector", "run")
			require.ErrorContains(t, err, tc.wantErr)
			var reported ReportedError
			require.ErrorAs(t, err, &reported)

			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(output)), &record), "output: %q", output)
			assert.Equal(t, "ERROR", record["level"])
			assert.Equal(t, "langsmith connector exiting", record["msg"])
			assert.Contains(t, record["error"], tc.wantErr)
		})
	}
}

func TestConnectorServiceLoadsConfigAndRunsTheClient(t *testing.T) {
	t.Parallel()

	wantConfig := connector.Config{Endpoint: "wss://edge.example.test/v1/connections/connect"}
	var loadedPath string
	var constructedConfig connector.Config
	ran := false
	deps := connectorServiceDeps{
		loadConfig: func(path string) (connector.Config, error) {
			loadedPath = path
			return wantConfig, nil
		},
		newClient: func(config connector.Config, _ *slog.Logger) (connectorRunner, error) {
			constructedConfig = config
			return runFunc(func(context.Context) error {
				ran = true
				return nil
			}), nil
		},
	}

	require.NoError(t, runConnectorService(t.Context(), "/etc/langsmith/connector.yaml", deps, discardLogger()))
	assert.Equal(t, "/etc/langsmith/connector.yaml", loadedPath)
	assert.Equal(t, wantConfig, constructedConfig)
	assert.True(t, ran)
}

func TestConnectorServiceLogsItsLifecycle(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := newConnectorLogger(&output, slog.LevelInfo).With("version", "1.2.3")
	require.NoError(t, runConnectorService(t.Context(), "",
		connectorTestDeps(func(context.Context) error { return nil }), logger))

	var messages []string
	decoder := json.NewDecoder(&output)
	for {
		var record map[string]any
		if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
			break
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, "1.2.3", record["version"])
		messages = append(messages, record["msg"].(string))
	}
	assert.Equal(t, []string{"langsmith connector starting", "langsmith connector stopped"}, messages)
}

func TestConnectorServiceStopsWhenCancelled(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		result <- runConnectorService(ctx, "", connectorTestDeps(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}), discardLogger())
	}()

	<-started
	cancel()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("the connector did not stop after cancellation")
	}
}

func TestConnectorServiceReconnectsAfterATransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	reconnected := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		result <- runConnectorService(ctx, "", connectorTestDeps(func(ctx context.Context) error {
			if calls.Add(1) == 1 {
				return errors.New("transient carrier failure")
			}
			close(reconnected)
			<-ctx.Done()
			return ctx.Err()
		}), discardLogger())
	}()

	select {
	case <-reconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("the connector did not reconnect")
	}
	cancel()
	require.NoError(t, <-result)
}

func TestConnectorServiceWaitsBeforeReconnecting(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	deps := connectorTestDeps(func(ctx context.Context) error {
		calls.Add(1)
		return errors.New("transient carrier failure")
	})
	deps.reconnectBackoff = 0 // the default floor applies
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	require.NoError(t, runConnectorService(ctx, "", deps, discardLogger()))
	assert.Equal(t, int32(1), calls.Load(), "no reconnect inside the initial backoff")
}

func TestConnectorServiceWrapsDependencyErrors(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("load failure")
	deps := connectorTestDeps(nil)
	deps.loadConfig = func(string) (connector.Config, error) { return connector.Config{}, loadErr }
	err := runConnectorService(t.Context(), "", deps, discardLogger())
	require.ErrorIs(t, err, loadErr)
}

func TestParseConnectorLogLevel(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"WARN":    slog.LevelWarn,
		" error ": slog.LevelError,
	} {
		got, err := parseConnectorLogLevel(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"verbose", "INFO+1"} {
		_, err := parseConnectorLogLevel(raw)
		assert.Error(t, err, raw)
	}
}

func TestNewConnectorLoggerWritesJSONAtLevel(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := newConnectorLogger(&output, slog.LevelWarn)
	logger.Info("filtered out")
	logger.Warn("connector could not reach target", "target", "api")

	var record map[string]any
	require.NoError(t, json.NewDecoder(&output).Decode(&record))
	assert.Equal(t, "connector could not reach target", record["msg"])
	assert.Equal(t, "WARN", record["level"])
	assert.Equal(t, "api", record["target"])
}
