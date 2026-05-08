package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
	"golang.org/x/net/websocket"
)

// ==================== Command structure ====================

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
	// Only remote-port is required; name is a positional arg and url is optional.
	for _, name := range []string{"remote-port"} {
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

// startTunnelServer starts a WebSocket server that acts as the daemon-side
// tunnel handler for the SDK tunnel client. It accepts yamux streams, reads
// connect headers, and echoes stream payloads.
func startTunnelServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		reader := &testWSFrameReader{ws: ws}
		connected := map[uint32]bool{}

		for {
			msgType, flags, streamID, payload, err := reader.readFrame()
			if err != nil {
				return
			}
			switch {
			case msgType == 1 && flags&1 != 0:
				connected[streamID] = false
			case msgType == 0 && len(payload) == 3 && !connected[streamID]:
				status := byte(langsmith.SandboxTunnelStatusOK)
				if payload[0] != byte(langsmith.SandboxTunnelProtocolVersion) {
					status = byte(langsmith.SandboxTunnelStatusUnsupportedVersion)
				}
				if err := sendTestYamuxFrame(ws, 0, 0, streamID, []byte{status}); err != nil {
					return
				}
				connected[streamID] = status == byte(langsmith.SandboxTunnelStatusOK)
			case msgType == 0 && connected[streamID]:
				if err := sendTestYamuxFrame(ws, 0, 0, streamID, payload); err != nil {
					return
				}
			}
		}
	}))
}

func TestSandboxTunnel_EndToEnd(t *testing.T) {
	tunnelSrv := startTunnelServer(t)
	defer tunnelSrv.Close()

	remotePort := 8080
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

	errCh := make(chan error, 1)
	go func() {
		errCh <- runTunnel(ctx, logger, newTestTunnelClient(), sandboxURL, remotePort, localPort)
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
	tunnelSrv := startTunnelServer(t)
	defer tunnelSrv.Close()

	remotePort := 8080

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
		runTunnel(ctx, logger, newTestTunnelClient(), tunnelSrv.URL, remotePort, localPort) //nolint:errcheck
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

func newTestTunnelClient() *langsmith.Client {
	return langsmith.NewClient(
		option.WithBaseURL("http://control-plane.test"),
		option.WithAPIKey("test-api-key"),
		option.WithMaxRetries(0),
	)
}

type testWSFrameReader struct {
	ws  *websocket.Conn
	buf []byte
}

func (r *testWSFrameReader) read(n int) ([]byte, error) {
	for len(r.buf) < n {
		var msg []byte
		if err := websocket.Message.Receive(r.ws, &msg); err != nil {
			return nil, err
		}
		r.buf = append(r.buf, msg...)
	}
	out := append([]byte(nil), r.buf[:n]...)
	r.buf = r.buf[n:]
	return out, nil
}

func (r *testWSFrameReader) readFrame() (msgType byte, flags uint16, streamID uint32, payload []byte, err error) {
	header, err := r.read(12)
	if err != nil {
		return 0, 0, 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[8:12])
	if length > 0 {
		payload, err = r.read(int(length))
		if err != nil {
			return 0, 0, 0, nil, err
		}
	}
	return header[1], binary.BigEndian.Uint16(header[2:4]), binary.BigEndian.Uint32(header[4:8]), payload, nil
}

func sendTestYamuxFrame(ws *websocket.Conn, msgType byte, flags uint16, streamID uint32, payload []byte) error {
	frame := make([]byte, 12+len(payload))
	frame[1] = msgType
	binary.BigEndian.PutUint16(frame[2:4], flags)
	binary.BigEndian.PutUint32(frame[4:8], streamID)
	binary.BigEndian.PutUint32(frame[8:12], uint32(len(payload)))
	copy(frame[12:], payload)
	return websocket.Message.Send(ws, frame)
}

func TestSandboxTunnel_MissingFlags(t *testing.T) {
	// --remote-port is required.
	_, err := executeCommand(t, "sandbox", "tunnel", "my-vm")
	if err == nil {
		t.Error("expected error when required flags are missing")
	}
	if err != nil && !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error, got: %v", err)
	}
}
