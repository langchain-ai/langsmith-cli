//go:build unix

package proc

import (
	"context"
	"io"
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshot returns the groups r holds.
func (r *registry) snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Collect(maps.Keys(r.groups))
}

// spawnTree spawns a shell script as an owned child, stopping it when the test
// ends.
func spawnTree(t *testing.T, script string) *OwnedChild {
	t.Helper()
	child, err := Spawn(exec.Command("sh", "-c", script))
	require.NoError(t, err, "failed to spawn the test tree")
	t.Cleanup(func() {
		_, err := child.Stop()
		assert.NoError(t, err)
	})
	return child
}

// stubbornTree is a child that ignores SIGINT and holds a grandchild open,
// which is the shape `uv run` wrapping a server has. Killing the child alone
// would leave the grandchild behind.
func stubbornTree(t *testing.T) *OwnedChild {
	return spawnTree(t, `trap "" INT; sleep 30 & wait`)
}

// lingeringWorkerTree is a worker that ignores SIGINT under a leader that
// exits at once. Trusting the leader's exit would leave it serving the old
// build.
func lingeringWorkerTree(t *testing.T) *OwnedChild {
	return spawnTree(t, `sh -c 'trap "" INT; sleep 30' &`)
}

// groupMembers lists the stat of every process in group.
func groupMembers(t *testing.T, group int) []string {
	t.Helper()
	out, err := exec.Command("ps", "-eo", "pgid=,stat=").Output()
	require.NoError(t, err, "ps failed")
	var stats []string
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == strconv.Itoa(group) {
			stats = append(stats, fields[1])
		}
	}
	return stats
}

func descendantsOf(t *testing.T, child *OwnedChild) int {
	return len(groupMembers(t, child.Pid()))
}

// runningMembersOf counts the members of group still running. A killed leader
// stays a zombie until its owner reaps it, which is not the same as still
// running.
func runningMembersOf(t *testing.T, group int) int {
	running := 0
	for _, stat := range groupMembers(t, group) {
		if !strings.HasPrefix(stat, "Z") {
			running++
		}
	}
	return running
}

// eventually polls condition until it holds or 3s pass.
func eventually(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, condition(), message)
}

func cancelled(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func TestStopTakesDownTheWholeTree(t *testing.T) {
	child := stubbornTree(t)
	eventually(t, func() bool { return descendantsOf(t, child) >= 2 }, "expected a child and a grandchild")

	status, err := child.Stop()

	require.NoError(t, err)
	assert.Nil(t, status, "it was still running")
	assert.Equal(t, 0, descendantsOf(t, child))
}

// Through Poll with no interrupt, which is how a caller sees a server that
// exited on its own. Reporting that exit without taking the group down leaves
// the workers holding the port after the command is gone.
func TestPollTakesDownAWorkerThatOutlivesItsLeader(t *testing.T) {
	child := lingeringWorkerTree(t)
	require.True(t, child.waitForExit(3*time.Second), "the leader should exit at once")
	eventually(t, func() bool { return descendantsOf(t, child) >= 1 }, "expected a lingering worker")

	outcome, ok, err := child.Poll(t.Context())

	require.NoError(t, err)
	require.True(t, ok)
	assert.False(t, outcome.Stopped())
	assert.Equal(t, 0, descendantsOf(t, child))
}

// What a termination signal runs while the main goroutine may be blocked:
// every group this process owns goes down without its OwnedChild.
func TestTerminatingTakesDownEveryRegisteredTree(t *testing.T) {
	first := stubbornTree(t)
	second := stubbornTree(t)
	groups := []int{first.Pid(), second.Pid()}
	for _, group := range groups {
		eventually(t, func() bool { return runningMembersOf(t, group) >= 2 }, "expected a child and a grandchild")
	}
	registered := liveGroups.snapshot()
	assert.Subset(t, registered, groups, "spawned groups should be registered")

	terminateGroups(groups)

	for _, group := range groups {
		eventually(t, func() bool { return runningMembersOf(t, group) == 0 }, "group "+strconv.Itoa(group)+" survived")
	}
	_, err := first.Stop()
	require.NoError(t, err)
	_, err = second.Stop()
	require.NoError(t, err)
	registered = liveGroups.snapshot()
	for _, group := range groups {
		assert.False(t, slices.Contains(registered, group), "stopped group %d should be unregistered: %v", group, registered)
	}
}

// Termination snapshots the registry once, then spends a grace period on it.
// A child the main goroutine starts in that window — the next toolchain step —
// must not escape the snapshot and outlive us.
func TestAGroupStartedAfterTerminationBeganIsKilled(t *testing.T) {
	registry := &registry{groups: map[int]struct{}{}}
	require.Empty(t, registry.close())

	command := exec.Command("sh", "-c", `trap "" INT TERM; sleep 30 & wait`)
	isolate(command)
	require.NoError(t, command.Start(), "failed to spawn the test tree")
	group := command.Process.Pid

	registration := registry.register(group)

	eventually(t, func() bool { return runningMembersOf(t, group) == 0 }, "the late group survived")
	assert.Empty(t, registry.snapshot(), "a closed registry grew")

	registration.release()
	err := command.Wait()
	assert.EqualError(t, err, "signal: killed")
}

func TestAnInterruptStopsARunningChild(t *testing.T) {
	child := stubbornTree(t)
	started := time.Now()

	outcome, err := child.Wait(cancelled(t))

	require.NoError(t, err)
	assert.True(t, outcome.Stopped())
	assert.Less(t, time.Since(started), stopGracePeriod+3*time.Second)
}

// Nothing asked it to stop, so the status is its own and reaches the caller.
func TestAnUninterruptedChildReportsItsOwnStatus(t *testing.T) {
	child := spawnTree(t, "exit 3")

	outcome, err := child.Wait(t.Context())

	require.NoError(t, err)
	require.False(t, outcome.Stopped(), "nothing stopped it")
	assert.Equal(t, 3, outcome.Status.ExitCode())
}

// So a crash racing a Ctrl-C is not reported as a clean stop.
func TestStoppingAnAlreadyExitedChildReturnsItsStatus(t *testing.T) {
	child := spawnTree(t, "exit 3")
	require.True(t, child.waitForExit(stopGracePeriod), "the test child should have exited")

	status, err := child.Stop()

	require.NoError(t, err)
	require.NotNil(t, status, "it exited on its own")
	assert.Equal(t, 3, status.ExitCode())
}

// The same guarantee through Poll, which is what an interrupted command
// actually calls.
func TestAnExitedChildKeepsItsStatusOverAnInterrupt(t *testing.T) {
	child := spawnTree(t, "exit 3")
	require.True(t, child.waitForExit(stopGracePeriod), "the test child should have exited")

	outcome, ok, err := child.Poll(cancelled(t))

	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, outcome.Stopped(), "expected the child's own status")
	assert.Equal(t, 3, outcome.Status.ExitCode())
}

// A leader that ignores SIGINT and keeps a grandchild alive — the shape of a
// hung `npm` wrapper. Killing only the leader would leave the grandchild
// holding stdout and stderr and hang the pipe readers.
func TestACaptureTimeoutKillsATreeThatHoldsPipes(t *testing.T) {
	started := time.Now()

	_, err := OutputWithTimeout(t.Context(), exec.Command("sh", "-c", `trap "" INT; sleep 30 & wait`), time.Second)

	assert.EqualError(t, err, "subprocess timed out after 1s")
	assert.Less(t, time.Since(started), 5*time.Second, "timeout path hung joining pipe readers")
}

func TestACaptureReturnsWhatTheChildWroteAndRead(t *testing.T) {
	output, err := OutputWithStdin(t.Context(), exec.Command("sh", "-c", "cat; echo oops >&2; exit 3"), []byte("in"))

	require.NoError(t, err)
	assert.Equal(t, 3, output.Status.ExitCode())
	assert.Equal(t, "in", string(output.Stdout))
	assert.Equal(t, "oops\n", string(output.Stderr))
}

func TestAStderrTailKeepsTheEnd(t *testing.T) {
	command := exec.Command("sh", "-c", "{ printf start; printf '%0100d' 0; printf end; } >&2; exit 1")
	tail := teeStderr(command, 10, io.Discard)
	child, err := Spawn(command)
	require.NoError(t, err)

	_, err = child.Wait(t.Context())

	require.NoError(t, err)
	assert.Equal(t, "0000000end", string(tail.Finish()))
}

func TestAnInterruptedCaptureIsACancellation(t *testing.T) {
	started := time.Now()

	_, err := Output(cancelled(t), exec.Command("sleep", "30"))

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(started), 5*time.Second)
}
