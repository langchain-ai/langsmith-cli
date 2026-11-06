package proc

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

// interruptExitCode is the conventional 128 + SIGINT.
const interruptExitCode = 130

// terminating is set once a termination signal arrives, before any child is
// touched.
var terminating atomic.Bool

// InstallInterrupt returns a context the user's request to stop cancels, in
// place of [signal.NotifyContext].
//
// The first Ctrl-C only cancels the context: commands check it and return a
// normal value rather than exiting from the handler, which is what lets the
// command turn a Ctrl-C into an exit code. A second one means the command did
// not stop, such as during a long request, so it takes the children down and
// exits at once.
//
// A termination signal — SIGTERM from an agent's or CI's timeout, SIGHUP from
// a closed terminal — is not a request: the main goroutine may be blocked and
// never look at the context again. So it takes down every process group the
// CLI owns and exits with the conventional 128 + signal status. SIGKILL
// cannot be caught, so a caller that skips straight to it still leaves the
// children behind.
//
// stop releases the signals and cancels the context.
func InstallInterrupt(parent context.Context) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, os.Interrupt)
	terminations := make(chan os.Signal, 1)
	if len(terminationSignals) > 0 {
		signal.Notify(terminations, terminationSignals...)
	}
	done := make(chan struct{})
	go func() {
		var requested atomic.Bool
		for {
			select {
			case <-interrupts:
				onSignal(&requested, cancel, forceExit)
			case sig := <-terminations:
				terminate(sig)
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			signal.Stop(interrupts)
			signal.Stop(terminations)
			close(done)
			cancel()
		})
	}
}

// HandOverIfTerminating blocks forever if a termination signal is being
// handled, so the handler's exit decides the status instead of whatever the
// command made of its children dying. A no-op otherwise.
//
// Killing the children wakes the main goroutine too — it sees its server exit
// and would report that as the command's failure; this lets it step aside so
// the status and output are the signal's.
func HandOverIfTerminating() {
	if terminating.Load() {
		select {}
	}
}

// SleepUntilInterrupted sleeps up to total, returning as soon as ctx is
// cancelled.
func SleepUntilInterrupted(ctx context.Context, total time.Duration) {
	timer := time.NewTimer(total)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// Pause sleeps up to total between polls of a long wait, and fails with ctx's
// error if the user asked to stop.
func Pause(ctx context.Context, total time.Duration) error {
	SleepUntilInterrupted(ctx, total)
	return ctx.Err()
}

// onSignal cancels on the first signal, and forces on any after it.
func onSignal(requested *atomic.Bool, cancel, force func()) {
	if requested.Swap(true) {
		force()
		return
	}
	cancel()
}

// forceExit takes down every child group, as a termination signal would, and
// exits with the conventional 128 + SIGINT.
func forceExit() {
	TerminateRegistered()
	os.Exit(interruptExitCode)
}
