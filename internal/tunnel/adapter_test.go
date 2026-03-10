package tunnel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func setupWSPair(t *testing.T) (client *WSAdapter, server *WSAdapter, cleanup func()) {
	t.Helper()

	upgrader := websocket.Upgrader{
		CheckOrigin:     func(r *http.Request) bool { return true },
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("server upgrade: %v", err)
		}
		serverConnCh <- conn
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}

	serverConn := <-serverConnCh

	return NewWSAdapter(clientConn), NewWSAdapter(serverConn), func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		srv.Close()
	}
}

func TestWSAdapterWriteRead(t *testing.T) {
	client, server, cleanup := setupWSPair(t)
	defer cleanup()

	payload := []byte("hello tunnel")
	n, err := client.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write: expected %d bytes, got %d", len(payload), n)
	}

	buf := make([]byte, 64)
	n, err = server.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != string(payload) {
		t.Errorf("Read: expected %q, got %q", payload, buf[:n])
	}
}

func TestWSAdapterPartialRead(t *testing.T) {
	client, server, cleanup := setupWSPair(t)
	defer cleanup()

	payload := []byte("abcdefghijklmnop")
	if _, err := client.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var got []byte
	buf := make([]byte, 4)
	for len(got) < len(payload) {
		n, err := server.Read(buf)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, buf[:n]...)
	}
	if string(got) != string(payload) {
		t.Errorf("expected %q, got %q", payload, got)
	}
}

func TestWSAdapterBidirectional(t *testing.T) {
	client, server, cleanup := setupWSPair(t)
	defer cleanup()

	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("client Write: %v", err)
	}

	buf := make([]byte, 64)
	n, err := server.Read(buf)
	if err != nil {
		t.Fatalf("server Read: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("server Read: expected %q, got %q", "ping", buf[:n])
	}

	if _, err := server.Write([]byte("pong")); err != nil {
		t.Fatalf("server Write: %v", err)
	}

	n, err = client.Read(buf)
	if err != nil {
		t.Fatalf("client Read: %v", err)
	}
	if string(buf[:n]) != "pong" {
		t.Errorf("client Read: expected %q, got %q", "pong", buf[:n])
	}
}

func TestWSAdapterClose(t *testing.T) {
	client, server, cleanup := setupWSPair(t)
	defer cleanup()

	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	buf := make([]byte, 64)
	if _, err := server.Read(buf); err == nil {
		t.Error("expected error after client close, got nil")
	}
}
