package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"

	"github.com/langchain-ai/langsmith-cli/internal/tunnel"
)

// ==================== Command structure ====================

func TestSandboxCmd_Subcommands(t *testing.T) {
	cmd := newSandboxCmd()
	expected := map[string]bool{"tunnel": false}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("sandbox missing subcommand %q", name)
		}
	}
}

func TestSandboxCmd_UseField(t *testing.T) {
	cmd := newSandboxCmd()
	if cmd.Use != "sandbox" {
		t.Errorf("expected Use=sandbox, got %q", cmd.Use)
	}
}

func TestSandboxTunnelCmd_Flags(t *testing.T) {
	cmd := newSandboxTunnelCmd()
	tests := []struct {
		name   string
		defVal string
	}{
		{"url", ""},
		{"remote-port", "0"},
		{"local-port", "0"},
		{"log-level", "info"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
	}
}

func TestSandboxTunnelCmd_RequiredFlags(t *testing.T) {
	cmd := newSandboxTunnelCmd()
	for _, name := range []string{"url", "remote-port"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag --%s not found", name)
			continue
		}
		ann := f.Annotations
		if ann == nil {
			t.Errorf("flag --%s has no annotations (expected required)", name)
			continue
		}
		if _, ok := ann["cobra_annotation_bash_completion_one_required_flag"]; !ok {
			t.Errorf("flag --%s not marked as required", name)
		}
	}
}

// ==================== Integration test ====================

func startEchoServer(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				io.Copy(c, c) //nolint:errcheck
			}(conn)
		}
	}()

	return ln, ln.Addr().(*net.TCPAddr).Port
}

// startTunnelServer starts a WebSocket server that acts as the daemon-side
// tunnel handler: yamux server accepting streams, reading connect headers,
// dialing the target port, and bridging.
func startTunnelServer(t *testing.T) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		adapter := tunnel.NewWSAdapter(conn)

		cfg := yamux.DefaultConfig()
		cfg.LogOutput = io.Discard
		session, err := yamux.Server(adapter, cfg)
		if err != nil {
			t.Logf("yamux server error: %v", err)
			return
		}
		defer func() { _ = session.Close() }()

		for {
			stream, err := session.AcceptStream()
			if err != nil {
				return
			}
			go handleTestStream(t, r.Context(), stream)
		}
	}))
}

func handleTestStream(t *testing.T, ctx context.Context, stream *yamux.Stream) {
	t.Helper()
	defer func() { _ = stream.Close() }()

	hdr, err := tunnel.ReadConnectHeader(stream)
	if err != nil {
		return
	}
	if hdr.Version != tunnel.ProtocolVersion {
		tunnel.WriteStatus(stream, tunnel.StatusUnsupportedVersion) //nolint:errcheck
		return
	}

	addr := fmt.Sprintf("127.0.0.1:%d", hdr.Port)
	tcpConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		tunnel.WriteStatus(stream, tunnel.StatusDialFailed) //nolint:errcheck
		return
	}

	if err := tunnel.WriteStatus(stream, tunnel.StatusOK); err != nil {
		_ = tcpConn.Close()
		return
	}

	tunnel.Bridge(ctx, stream, tcpConn)
}

func TestSandboxTunnel_EndToEnd(t *testing.T) {
	echoLn, echoPort := startEchoServer(t)
	defer func() { _ = echoLn.Close() }()

	tunnelSrv := startTunnelServer(t)
	defer tunnelSrv.Close()

	// Convert httptest URL to a sandbox-like URL (the tunnel command appends /tunnel).
	// Strip the trailing path so connectTunnel appends /tunnel correctly.
	sandboxURL := tunnelSrv.URL

	// Pick a free local port.
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	localPort := localLn.Addr().(*net.TCPAddr).Port
	_ = localLn.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The tunnel server expects the WebSocket at /tunnel, but our test server
	// handles all paths. connectTunnel will append /tunnel to the URL, which is fine.

	errCh := make(chan error, 1)
	go func() {
		errCh <- runTunnel(ctx, logger, sandboxURL, "test-key", echoPort, localPort)
	}()

	// Wait for the listener to come up.
	var conn net.Conn
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		cancel()
		t.Fatalf("could not connect to tunnel local port: %v", err)
	}
	defer func() { _ = conn.Close() }()

	payload := "hello through tunnel"
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf[:n]) != payload {
		t.Errorf("expected %q, got %q", payload, buf[:n])
	}

	cancel()

	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Errorf("runTunnel returned error: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Error("runTunnel did not exit after cancel")
	}
}

func TestSandboxTunnel_MultipleConnections(t *testing.T) {
	echoLn, echoPort := startEchoServer(t)
	defer func() { _ = echoLn.Close() }()

	tunnelSrv := startTunnelServer(t)
	defer tunnelSrv.Close()

	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	localPort := localLn.Addr().(*net.TCPAddr).Port
	_ = localLn.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		runTunnel(ctx, logger, tunnelSrv.URL, "test-key", echoPort, localPort) //nolint:errcheck
	}()

	// Wait for listener.
	time.Sleep(200 * time.Millisecond)

	const numConns = 5
	var wg sync.WaitGroup
	wg.Add(numConns)

	for i := 0; i < numConns; i++ {
		go func(idx int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
			if err != nil {
				t.Errorf("conn %d dial: %v", idx, err)
				return
			}
			defer func() { _ = conn.Close() }()

			msg := fmt.Sprintf("conn-%d", idx)
			if _, err := conn.Write([]byte(msg)); err != nil {
				t.Errorf("conn %d write: %v", idx, err)
				return
			}

			buf := make([]byte, 64)
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Read(buf)
			if err != nil {
				t.Errorf("conn %d read: %v", idx, err)
				return
			}
			if string(buf[:n]) != msg {
				t.Errorf("conn %d: expected %q, got %q", idx, msg, buf[:n])
			}
		}(i)
	}

	wg.Wait()
	cancel()
}

func TestSandboxTunnel_MissingFlags(t *testing.T) {
	// --url and --remote-port are required.
	_, err := executeCommand(t, "sandbox", "tunnel")
	if err == nil {
		t.Error("expected error when required flags are missing")
	}
	if err != nil && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}
