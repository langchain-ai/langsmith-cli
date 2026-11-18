package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-go/lib/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeConnectionsAPI serves the connections endpoints a quick tunnel uses.
type fakeConnectionsAPI struct {
	mu          sync.Mutex
	conflicts   int // name conflicts to answer before a create succeeds
	creates     []map[string]any
	targetCalls int
	deleted     []string
	requests    int
	heartbeats  []map[string]any
	expireAfter int // heartbeats to accept before answering 404; 0 accepts all
}

func (f *fakeConnectionsAPI) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/connections":
			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.creates = append(f.creates, body)
			if f.conflicts > 0 {
				f.conflicts--
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"detail": "a connection with this name already exists"}`))
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": testConnectionID, "name": body["name"], "kind": "tunnel",
				"personal": true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/connections/"+testConnectionID+"/targets":
			// A later page: the targets span two pages once registered.
			if r.URL.Query().Get("cursor") == "app" {
				_, _ = w.Write([]byte(`{"items": [{"name": "db", "kind": "tcp", "is_available": true}], "next_cursor": null}`))
				return
			}
			f.targetCalls++
			// The edge lists a target only once the connector has registered it.
			if f.targetCalls == 1 {
				_, _ = w.Write([]byte(`{"items": [], "next_cursor": null}`))
				return
			}
			_, _ = w.Write([]byte(`{"items": [
				{"name": "app", "kind": "http", "origin_scheme": "http", "is_available": true, "tunnel_url": "https://app.tunnel.example"}
			], "next_cursor": "app"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/connections/"+testConnectionID:
			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if f.expireAfter > 0 && len(f.heartbeats) >= f.expireAfter {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"detail": "connection not found"}`))
				return
			}
			f.heartbeats = append(f.heartbeats, body)
			_, _ = w.Write([]byte(`{"id": "` + testConnectionID + `"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/connections/"+testConnectionID:
			f.deleted = append(f.deleted, testConnectionID)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func (f *fakeConnectionsAPI) snapshot() fakeConnectionsAPI {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeConnectionsAPI{
		creates:     append([]map[string]any(nil), f.creates...),
		targetCalls: f.targetCalls,
		deleted:     append([]string(nil), f.deleted...),
		requests:    f.requests,
		heartbeats:  append([]map[string]any(nil), f.heartbeats...),
	}
}

// fakeConnector replaces serveConnector for one test. run is called with the
// connector's config in place of a real connector.
func fakeConnector(t *testing.T, run func(context.Context, connector.Config) error) *[]connector.Config {
	t.Helper()
	var (
		mu      sync.Mutex
		configs []connector.Config
	)
	original := serveConnector
	serveConnector = func(ctx context.Context, config connector.Config, _ string) error {
		mu.Lock()
		configs = append(configs, config)
		mu.Unlock()
		return run(ctx, config)
	}
	t.Cleanup(func() { serveConnector = original })
	return &configs
}

func untilCancelled(ctx context.Context, _ connector.Config) error {
	<-ctx.Done()
	return nil
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// runQuickTunnelUntilReady runs a quick tunnel until it reports ready, then
// interrupts it and returns its output.
func runQuickTunnelUntilReady(t *testing.T, args []string, ready string) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- executeCommandTo(ctx, &out, args...) }()

	require.Eventually(t, func() bool { return strings.Contains(out.String(), ready) },
		10*time.Second, 20*time.Millisecond, "the tunnel never reported ready; output:\n%s", out.String())
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "an interrupted quick tunnel exits cleanly")
	case <-time.After(10 * time.Second):
		t.Fatal("command did not stop after its context was cancelled")
	}
	return out.String()
}

func TestTunnelConnectQuickServesAPersonalTunnelAndDeletesIt(t *testing.T) {
	isolateConnectorConfig(t)
	api := &fakeConnectionsAPI{conflicts: 1}
	server := api.serve(t)
	configs := fakeConnector(t, untilCancelled)

	out := runQuickTunnelUntilReady(t, []string{"tunnel", "connect", "--quick",
		"--target", "app=http://localhost:2024", "--target", "db=localhost:5432",
		"--api-key", "test-api-key", "--api-url", server.URL}, "Tunnel ready")

	got := api.snapshot()
	require.Len(t, got.creates, 2, "a name conflict is retried with a new name")
	for _, body := range got.creates {
		assert.Regexp(t, `^quick-[0-9a-f]{8}$`, body["name"])
		assert.Equal(t, "tunnel", body["kind"])
		assert.Equal(t, true, body["personal"])
		assert.Equal(t, "Quick tunnel from langsmith tunnel connect --quick", body["description"])
		assert.EqualValues(t, 300, body["expires_in_seconds"], "the tunnel expires unless the CLI keeps it alive")
	}
	assert.NotEqual(t, got.creates[0]["name"], got.creates[1]["name"])

	require.Len(t, *configs, 1)
	config := (*configs)[0]
	assert.Equal(t, testConnectionID, config.ConnectionID.String())
	assert.Equal(t, "tunnel", config.Targets.Kind())

	assert.Contains(t, out, "Tunnel ready: app → https://app.tunnel.example\n")
	assert.NotContains(t, out, "Tunnel ready: db", "a TCP target has no tunnel URL")
	assert.Equal(t, []string{testConnectionID}, got.deleted)
}

func TestTunnelConnectQuickPrintsJSON(t *testing.T) {
	isolateConnectorConfig(t)
	api := &fakeConnectionsAPI{}
	server := api.serve(t)
	fakeConnector(t, untilCancelled)

	out := runQuickTunnelUntilReady(t, []string{"tunnel", "connect", "--quick",
		"--target", "app=http://localhost:2024", "--format", "json",
		"--api-key", "test-api-key", "--api-url", server.URL}, "tunnel_url")

	start := strings.Index(out, "{")
	require.GreaterOrEqual(t, start, 0, out)
	var report map[string]any
	require.NoError(t, json.NewDecoder(strings.NewReader(out[start:])).Decode(&report), out)
	assert.Equal(t, testConnectionID, report["connection_id"])
	assert.Regexp(t, `^quick-[0-9a-f]{8}$`, report["name"])
	assert.Equal(t, []any{map[string]any{"name": "app", "tunnel_url": "https://app.tunnel.example"}}, report["targets"])
}

func TestTunnelConnectQuickDeletesTheTunnelWhenTheConnectorFails(t *testing.T) {
	isolateConnectorConfig(t)
	api := &fakeConnectionsAPI{}
	server := api.serve(t)
	fakeConnector(t, func(context.Context, connector.Config) error {
		return errors.New("edge refused the connector")
	})

	_, err := executeCommand(t, "tunnel", "connect", "--quick", "--target", "app=http://localhost:2024",
		"--api-key", "test-api-key", "--api-url", server.URL)

	require.EqualError(t, err, "edge refused the connector")
	assert.Equal(t, []string{testConnectionID}, api.snapshot().deleted)
}

func TestTunnelConnectQuickKeepsTheTunnelAliveAndStopsWhenItExpires(t *testing.T) {
	isolateConnectorConfig(t)
	original := quickTunnelHeartbeatInterval
	quickTunnelHeartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { quickTunnelHeartbeatInterval = original })
	api := &fakeConnectionsAPI{expireAfter: 2}
	server := api.serve(t)
	fakeConnector(t, untilCancelled)

	_, err := executeCommand(t, "tunnel", "connect", "--quick", "--target", "app=http://localhost:2024",
		"--api-key", "test-api-key", "--api-url", server.URL)

	require.ErrorIs(t, err, errQuickTunnelExpired)
	got := api.snapshot()
	require.Len(t, got.heartbeats, 2)
	for _, body := range got.heartbeats {
		assert.EqualValues(t, 300, body["expires_in_seconds"])
	}
}

func TestTunnelConnectQuickGivesUpAfterRepeatedNameConflicts(t *testing.T) {
	isolateConnectorConfig(t)
	api := &fakeConnectionsAPI{conflicts: 100}
	server := api.serve(t)
	configs := fakeConnector(t, untilCancelled)

	_, err := executeCommand(t, "tunnel", "connect", "--quick", "--target", "app=http://localhost:2024",
		"--api-key", "test-api-key", "--api-url", server.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating a quick tunnel")
	got := api.snapshot()
	assert.Len(t, got.creates, 4, "the first create and three retries")
	assert.Empty(t, got.deleted)
	assert.Empty(t, *configs)
}

func TestTunnelConnectQuickRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		profile string
		want    string
	}{
		{name: "connection ID", args: []string{testConnectionID, "--target", "app=http://localhost:2024", "--api-key", "test-api-key"}, want: "--quick creates its own tunnel connection"},
		{name: "missing target", args: []string{"--api-key", "test-api-key"}, want: "target"},
		{name: "invalid target", args: []string{"--target", "app=ftp://localhost:21", "--api-key", "test-api-key"}, want: "target"},
		{name: "OAuth-only profile", args: []string{"--target", "app=http://localhost:2024"}, profile: `{` + testOAuthLogin + `}`, want: "needs an API key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConnectorConfig(t)
			api := &fakeConnectionsAPI{}
			server := api.serve(t)
			configs := fakeConnector(t, untilCancelled)
			if tc.profile != "" {
				writeConnectorProfile(t, tc.profile)
			}

			_, err := executeCommand(t, append([]string{"tunnel", "connect", "--quick", "--api-url", server.URL}, tc.args...)...)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Zero(t, api.snapshot().requests, "nothing is created")
			assert.Empty(t, *configs)
		})
	}
}

func TestTunnelConnectRequiresAConnectionIDWithoutQuick(t *testing.T) {
	isolateConnectorConfig(t)
	_, err := executeCommand(t, "tunnel", "connect", "--target", "app=http://localhost:2024", "--api-key", "test-api-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<connection-id>")
}
