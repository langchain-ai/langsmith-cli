package tunnel

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestBridgeBidirectional(t *testing.T) {
	streamClient, streamServer := net.Pipe()
	tcpClient, tcpServer := net.Pipe()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		Bridge(ctx, streamServer, tcpServer)
		close(done)
	}()

	// Write through stream side, read from TCP side.
	if _, err := streamClient.Write([]byte("from-stream")); err != nil {
		t.Fatalf("stream Write: %v", err)
	}

	buf := make([]byte, 64)
	n, err := tcpClient.Read(buf)
	if err != nil {
		t.Fatalf("tcp Read: %v", err)
	}
	if string(buf[:n]) != "from-stream" {
		t.Errorf("tcp Read: expected %q, got %q", "from-stream", buf[:n])
	}

	// Write through TCP side, read from stream side.
	if _, err := tcpClient.Write([]byte("from-tcp")); err != nil {
		t.Fatalf("tcp Write: %v", err)
	}

	n, err = streamClient.Read(buf)
	if err != nil {
		t.Fatalf("stream Read: %v", err)
	}
	if string(buf[:n]) != "from-tcp" {
		t.Errorf("stream Read: expected %q, got %q", "from-tcp", buf[:n])
	}

	_ = streamClient.Close()
	_ = tcpClient.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bridge did not finish after connections closed")
	}
}

func TestBridgeContextCancel(t *testing.T) {
	streamClient, streamServer := net.Pipe()
	tcpClient, tcpServer := net.Pipe()
	defer func() { _ = streamClient.Close() }()
	defer func() { _ = tcpClient.Close() }()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Bridge(ctx, streamServer, tcpServer)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bridge did not finish after context cancel")
	}
}

func TestBridgeOneSideClose(t *testing.T) {
	streamClient, streamServer := net.Pipe()
	tcpClient, tcpServer := net.Pipe()

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		Bridge(ctx, streamServer, tcpServer)
		close(done)
	}()

	_ = tcpClient.Close()

	buf := make([]byte, 64)
	_, err := streamClient.Read(buf)
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}

	_ = streamClient.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("bridge did not finish after one side closed")
	}
}
