package tunnel

import (
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSAdapter wraps a *websocket.Conn to implement io.ReadWriteCloser,
// bridging gorilla/websocket's message-based API to the byte-stream
// interface that yamux requires.
type WSAdapter struct {
	conn   *websocket.Conn
	reader io.Reader // buffered reader for the current message
	mu     sync.Mutex
}

// NewWSAdapter creates a new adapter around the given WebSocket connection.
func NewWSAdapter(conn *websocket.Conn) *WSAdapter {
	return &WSAdapter{conn: conn}
}

// Read implements io.Reader. It buffers partially-read WebSocket messages
// so callers can read arbitrary byte counts.
//
// Read is NOT safe for concurrent use. This is intentional: yamux
// guarantees a single reader goroutine (recvLoop). Write uses mu because
// yamux writes from multiple goroutines (data, pings, window updates).
//
// Do NOT add a mutex here. Close() must be callable concurrently to
// unblock a NextReader() blocked on the connection. If Read held a mutex
// that Close also acquired, Close would deadlock waiting for Read, while
// Read is blocked on NextReader waiting for Close to shut the connection.
func (a *WSAdapter) Read(p []byte) (int, error) {
	if a.reader == nil {
		_, r, err := a.conn.NextReader()
		if err != nil {
			return 0, err
		}
		a.reader = r
	}

	n, err := a.reader.Read(p)
	if err == io.EOF {
		a.reader = nil
		if n > 0 {
			return n, nil
		}
		return a.Read(p)
	}
	return n, err
}

// Write implements io.Writer. Each call sends one binary WebSocket message.
func (a *WSAdapter) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close sends a close frame and closes the underlying connection.
func (a *WSAdapter) Close() error {
	_ = a.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(5*time.Second),
	)
	return a.conn.Close()
}
