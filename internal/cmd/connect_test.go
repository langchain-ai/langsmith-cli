package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConnectionID = "0b6e8a54-7d2c-4f43-9a51-3c1e2f6d8a90"

func isolateConnectorConfig(t *testing.T) {
	t.Helper()
	// A flag error writes the feedback-hint marker under the home directory,
	// which is USERPROFILE on Windows.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGSMITH_ENDPOINT", "")
	t.Setenv("LANGSMITH_PROFILE", "")
}

// dialEdge runs a connect command against a fake edge until the connector
// dials it, then interrupts the command and returns the dialed path and
// headers. args receives the edge's URL and returns the command line.
func dialEdge(t *testing.T, args func(edgeURL string) []string) (string, http.Header) {
	t.Helper()
	type dial struct {
		path   string
		header http.Header
	}
	dialed := make(chan dial, 16)
	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dialed <- dial{path: r.URL.Path, header: r.Header.Clone()}:
		default:
		}
		http.Error(w, "rejected by test edge", http.StatusConflict)
	}))
	t.Cleanup(edge.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	commandLine := args(edge.URL)
	go func() {
		_, err := executeCommandContext(t, ctx, commandLine...)
		done <- err
	}()

	var got dial
	select {
	case got = <-dialed:
	case err := <-done:
		t.Fatalf("command exited before dialing the edge: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("command never dialed the edge")
	}
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "an interrupted connector exits cleanly")
	case <-time.After(10 * time.Second):
		t.Fatal("command did not stop after its context was cancelled")
	}
	return got.path, got.header
}

func writeConnectorProfile(t *testing.T, profile string) {
	t.Helper()
	require.NoError(t, os.WriteFile(os.Getenv("LANGSMITH_CONFIG_FILE"),
		[]byte(`{"current_profile": "dev", "profiles": {"dev": `+profile+`}}`), 0o600))
}

const testOAuthLogin = `"oauth": {
	"access_token": "test-access-token",
	"refresh_token": "test-refresh-token",
	"expires_at": "2999-01-01T00:00:00Z"
}`

func TestConnectCommandsDialTheEdgeAsTheirKind(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		kind string
	}{
		{name: "tunnel", args: []string{"tunnel", "connect", testConnectionID, "--target", "app=http://localhost:8080", "--target", "db=localhost:5432"}, kind: "tunnel"},
		{name: "relay", args: []string{"relay", "connect", testConnectionID, "--allow", "10.0.0.0/8"}, kind: "relay"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConnectorConfig(t)
			path, headers := dialEdge(t, func(edgeURL string) []string {
				return append(tc.args, "--api-key", "test-api-key", "--api-url", edgeURL)
			})

			assert.Equal(t, "/v1/connections/connect", path)
			assert.Equal(t, "test-api-key", headers.Get("X-Api-Key"))
			assert.Equal(t, testConnectionID, headers.Get("X-LangSmith-Connection-Id"))
			assert.Equal(t, tc.kind, headers.Get("X-LangSmith-Connector-Kind"))
		})
	}
}

// TestConnectCommandsUseTheSavedProfile covers a profile written by an older
// login: an API URL ending in /api/v1, and an API key kept alongside a later
// OAuth login. Other commands normalize the URL and use OAuth; a connector
// normalizes the URL the same way and uses the key, the only credential the
// edge accepts.
func TestConnectCommandsUseTheSavedProfile(t *testing.T) {
	isolateConnectorConfig(t)
	path, headers := dialEdge(t, func(edgeURL string) []string {
		writeConnectorProfile(t, `{"api_url": "`+edgeURL+`/api/v1", "api_key": "profile-api-key", `+testOAuthLogin+`}`)
		return []string{"relay", "connect", testConnectionID}
	})

	assert.Equal(t, "/v1/connections/connect", path)
	assert.Equal(t, "profile-api-key", headers.Get("X-Api-Key"))
}

func TestConnectCommandsRejectOAuthOnlyProfiles(t *testing.T) {
	isolateConnectorConfig(t)
	writeConnectorProfile(t, `{`+testOAuthLogin+`}`)

	_, err := executeCommand(t, "relay", "connect", testConnectionID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs an API key")
}

func TestConnectCommandsRejectInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "target without address", args: []string{"tunnel", "connect", testConnectionID, "--target", "app"}, want: "--target must be name=host:port"},
		{name: "duplicate target", args: []string{"tunnel", "connect", testConnectionID, "--target", "app=a:1", "--target", "app=b:2"}, want: `--target "app" is given twice`},
		{name: "missing target", args: []string{"tunnel", "connect", testConnectionID}, want: "target"},
		{name: "invalid connection ID", args: []string{"relay", "connect", "not-a-uuid"}, want: "connection ID must be a non-zero UUID"},
		{name: "invalid relay rule", args: []string{"relay", "connect", testConnectionID, "--allow", "10.0.0.0/99"}, want: "relay allow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConnectorConfig(t)
			_, err := executeCommand(t, append(tc.args, "--api-key", "test-api-key", "--api-url", "http://127.0.0.1:1")...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
