package tunnel

import (
	"context"
	"io"
	"net"
	"sync"
)

// Bridge copies data bidirectionally between a yamux stream and a TCP
// connection until one side closes or ctx is cancelled. Both connections
// are closed when Bridge returns.
func Bridge(ctx context.Context, stream net.Conn, tcp net.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer cancel()
		copyUntilDone(ctx, tcp, stream, stream)
	}()

	go func() {
		defer wg.Done()
		defer cancel()
		copyUntilDone(ctx, stream, tcp, tcp)
	}()

	wg.Wait()
	_ = stream.Close()
	_ = tcp.Close()
}

// copyUntilDone copies from src to dst until the copy completes or ctx is
// cancelled. On cancellation it closes srcConn to unblock the io.Copy
// goroutine, then waits for it to exit before returning.
func copyUntilDone(ctx context.Context, dst io.Writer, src io.Reader, srcConn net.Conn) {
	done := make(chan struct{})
	go func() {
		io.Copy(dst, src) //nolint:errcheck
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		_ = srcConn.Close()
		<-done
	}
}
