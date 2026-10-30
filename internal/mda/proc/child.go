// Package proc runs the child processes `langsmith deploy` owns and finds the
// toolchain binaries they run (`child.rs`, `toolchain.rs`), turns the mda
// CLI's Ctrl-C flag into a [context.Context] (`interrupt.rs`,
// `termination.rs`), and holds small path helpers (`path_util.rs`).
//
// An owned child runs in its own process group, so a terminal Ctrl-C reaches
// only us: every stop is one we sent, and no caller has to work out from an
// exit status whether the user or the child ended it.
//
// The flip side is that a signal sent to us alone — SIGTERM from an agent's
// timeout, SIGHUP from a closed terminal — never reaches them either. Every
// group is therefore registered while it lives, so [TerminateRegistered] can
// take them all down from a signal handler that holds no [OwnedChild].
//
// [OwnedChild.Wait] takes the interrupt as a context rather than reading a
// process-global flag, so tests can fabricate one and run without installing a
// real signal handler.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"sync"
	"time"
)

// Outcome is how an owned child ended.
type Outcome struct {
	// Status is the child's own exit status when it ended on its own, so this
	// status is its own verdict. It is nil when we asked it to stop and it did.
	Status *os.ProcessState
}

// Stopped reports whether we asked the child to stop and it did.
func (o Outcome) Stopped() bool {
	return o.Status == nil
}

// registry holds the process groups this CLI started and has not reaped, by
// leader pid.
type registry struct {
	mu     sync.Mutex
	groups map[int]struct{}
	// closed is set once termination has taken its snapshot; no group may
	// join after it.
	closed bool
}

var liveGroups = &registry{groups: map[int]struct{}{}}

// groupRegistration keeps a spawned child's group in the registry until it is
// released, once [OwnedChild.Stop] has taken the group down.
type groupRegistration struct {
	registry *registry
	group    int
	once     sync.Once
}

// register adds group to r, or kills it on the spot if termination already
// took its snapshot — the main goroutine keeps running through the grace
// period, and a group it starts then would otherwise outlive us.
func (r *registry) register(group int) *groupRegistration {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		killLateGroup(group)
	} else {
		r.groups[group] = struct{}{}
		r.mu.Unlock()
	}
	return &groupRegistration{registry: r, group: group}
}

func (g *groupRegistration) release() {
	g.once.Do(func() {
		g.registry.mu.Lock()
		delete(g.registry.groups, g.group)
		g.registry.mu.Unlock()
	})
}

// close shuts r to new groups and returns the ones it holds, in one step under
// its lock, so no group can register between the snapshot and the close.
func (r *registry) close() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return slices.Collect(maps.Keys(r.groups))
}

// TerminateRegistered takes down every process group this CLI still owns, and
// any it starts from here on.
//
// For a termination signal, which can arrive while the main goroutine is
// blocked and will never look at its context.
func TerminateRegistered() {
	terminateGroups(liveGroups.close())
}

// OwnedChild is a child process whose lifetime this CLI controls.
type OwnedChild struct {
	cmd          *exec.Cmd
	registration *groupRegistration
	// done closes once the leader has exited and been reaped.
	done    chan struct{}
	waitErr error
	// drains are the goroutines moving output from the child's pipes into
	// the caller's writers. They end once every process holding a pipe is
	// gone.
	drains sync.WaitGroup

	stopOnce   sync.Once
	stopStatus *os.ProcessState
	stopErr    error
}

// Spawn starts cmd in its own process group.
//
// cmd's Stdin, Stdout and Stderr may be anything [exec.Cmd] takes. Ones that
// are not files are fed through pipes this child owns, so a grandchild that
// keeps a pipe open cannot keep the child from being reaped, and every byte it
// wrote has been delivered once the child is stopped. As with [exec.Cmd], and
// unlike a Rust `Command`, a nil stream is the null device rather than ours.
func Spawn(cmd *exec.Cmd) (*OwnedChild, error) {
	isolate(cmd)
	child := &OwnedChild{cmd: cmd, done: make(chan struct{})}
	// The child's ends of the pipes, closed here once it has them, and ours,
	// handed to the goroutines that feed and drain them.
	var theirs, ours []*os.File
	// feeds write to the child's stdin and are not waited for; drains read
	// its output and are.
	var feeds, drains []func()
	fail := func(err error) (*OwnedChild, error) {
		closeFiles(theirs)
		closeFiles(ours)
		return nil, err
	}
	if cmd.Stdin != nil {
		if _, isFile := cmd.Stdin.(*os.File); !isFile {
			source := cmd.Stdin
			read, write, err := os.Pipe()
			if err != nil {
				return fail(err)
			}
			cmd.Stdin = read
			theirs, ours = append(theirs, read), append(ours, write)
			// A failed write is not reported: the child's status and stderr
			// say why it stopped reading.
			feeds = append(feeds, func() {
				_, _ = io.Copy(write, source)
				_ = write.Close()
			})
		}
	}
	shared := cmd.Stderr != nil && sameWriter(cmd.Stdout, cmd.Stderr)
	for _, stream := range []*io.Writer{&cmd.Stdout, &cmd.Stderr} {
		if *stream == nil {
			continue
		}
		if shared && stream == &cmd.Stderr {
			cmd.Stderr = cmd.Stdout
			continue
		}
		if _, isFile := (*stream).(*os.File); isFile {
			continue
		}
		sink := *stream
		read, write, err := os.Pipe()
		if err != nil {
			return fail(err)
		}
		*stream = write
		theirs, ours = append(theirs, write), append(ours, read)
		drains = append(drains, func() {
			defer child.drains.Done()
			// A read error ends the copy early; what was read still helps.
			_, _ = io.Copy(sink, read)
			_ = read.Close()
		})
	}
	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	closeFiles(theirs)
	for _, feed := range feeds {
		go feed()
	}
	child.drains.Add(len(drains))
	for _, drain := range drains {
		go drain()
	}
	child.registration = liveGroups.register(cmd.Process.Pid)
	go func() {
		err := cmd.Wait()
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			child.waitErr = err
		}
		close(child.done)
	}()
	return child, nil
}

// sameWriter reports whether a and b are the same writer, as [exec.Cmd] does
// to give stdout and stderr one pipe.
func sameWriter(a, b io.Writer) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return a == b
}

func closeFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

// Pid returns the child's process id, which also names its process group.
func (c *OwnedChild) Pid() int {
	return c.cmd.Process.Pid
}

// Poll reports whether the child is over, without blocking: ok is false while
// it runs and ctx is live. Takes its group down once it is, whether it exited
// on its own or we stopped it.
func (c *OwnedChild) Poll(ctx context.Context) (outcome Outcome, ok bool, err error) {
	if _, exited := c.tryWait(); !exited && ctx.Err() == nil {
		return Outcome{}, false, nil
	}
	status, err := c.Stop()
	if err != nil {
		return Outcome{}, false, err
	}
	return Outcome{Status: status}, true, nil
}

// Wait waits for the child, stopping it if ctx is cancelled first.
func (c *OwnedChild) Wait(ctx context.Context) (Outcome, error) {
	select {
	case <-c.done:
	case <-ctx.Done():
	}
	outcome, _, err := c.Poll(ctx)
	return outcome, err
}

// Stop stops the child and everything it spawned, then reaps it.
//
// Returns the child's own status when it had already exited, so a failure
// racing the Ctrl-C is not reported as a stop we chose. The group still goes
// down in that case, because the leader exiting does not empty it. Stopping a
// stopped child returns the first stop's result.
func (c *OwnedChild) Stop() (*os.ProcessState, error) {
	c.stopOnce.Do(func() {
		exited, _ := c.tryWait()
		c.stopErr = stopTree(c)
		c.registration.release()
		<-c.done
		c.drains.Wait()
		if c.stopErr == nil {
			c.stopErr = c.waitErr
		}
		c.stopStatus = exited
	})
	return c.stopStatus, c.stopErr
}

// tryWait returns the child's status once it has exited.
func (c *OwnedChild) tryWait() (*os.ProcessState, bool) {
	select {
	case <-c.done:
		return c.cmd.ProcessState, true
	default:
		return nil, false
	}
}

// Captured is what a captured child wrote, and how it exited.
type Captured struct {
	Status *os.ProcessState
	Stdout []byte
	Stderr []byte
}

// Output runs cmd to completion as an owned child and collects what it wrote.
//
// Cancelling ctx stops its group and returns ctx's error, which the command
// reports as a cancellation.
func Output(ctx context.Context, cmd *exec.Cmd) (*Captured, error) {
	return capture(ctx, cmd, nil, 0)
}

// OutputWithStdin is [Output], writing stdin to the child first.
func OutputWithStdin(ctx context.Context, cmd *exec.Cmd, stdin []byte) (*Captured, error) {
	if stdin == nil {
		stdin = []byte{}
	}
	return capture(ctx, cmd, stdin, 0)
}

// OutputWithTimeout is [Output], stopping the child's group if it runs past
// timeout.
func OutputWithTimeout(ctx context.Context, cmd *exec.Cmd, timeout time.Duration) (*Captured, error) {
	return capture(ctx, cmd, nil, timeout)
}

func capture(ctx context.Context, cmd *exec.Cmd, stdin []byte, timeout time.Duration) (*Captured, error) {
	cmd.Stdin = nil
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	owned, err := Spawn(cmd)
	if err != nil {
		return nil, err
	}
	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}
	select {
	case <-owned.done:
	case <-ctx.Done():
	case <-deadline:
		if _, err := owned.Stop(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("subprocess timed out after %ds", int64(timeout/time.Second))
	}
	outcome, _, err := owned.Poll(ctx)
	if err != nil {
		return nil, err
	}
	if outcome.Stopped() {
		return nil, ctx.Err()
	}
	return &Captured{Status: outcome.Status, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
}

// StderrTail is a child's stderr, passed through to ours as it arrives, with
// its last part kept for the error if the child fails.
type StderrTail struct {
	out  io.Writer
	keep int
	mu   sync.Mutex
	tail []byte
}

// TeeStderr routes cmd's stderr through a tail that keeps its last keep bytes.
// Call it before [Spawn].
func TeeStderr(cmd *exec.Cmd, keep int) *StderrTail {
	return teeStderr(cmd, keep, os.Stderr)
}

func teeStderr(cmd *exec.Cmd, keep int, out io.Writer) *StderrTail {
	tail := &StderrTail{out: out, keep: keep}
	cmd.Stderr = tail
	return tail
}

// Write passes p through to our stderr and keeps the end of it.
func (s *StderrTail) Write(p []byte) (int, error) {
	_, _ = s.out.Write(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tail = append(s.tail, p...)
	if excess := len(s.tail) - s.keep; excess > 0 {
		s.tail = append(s.tail[:0], s.tail[excess:]...)
	}
	return len(p), nil
}

// Finish returns the kept bytes. Call once the child has been stopped or
// waited for, so the pipe has closed.
func (s *StderrTail) Finish() []byte {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.tail)
}
