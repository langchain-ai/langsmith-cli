//go:build unix

package proc

import (
	"os/exec"
	"syscall"
	"time"
)

// pollInterval is how often a group is checked while termination waits for
// it.
const pollInterval = 100 * time.Millisecond

// stopGracePeriod is how long a child gets to act on the forwarded interrupt
// before we escalate to a kill.
//
// A reloading server's workers do not always take the interrupt, and this is
// the delay the user sees at the prompt when they do not. Measured clean
// shutdowns land near 300ms, so this is generous without being felt.
const stopGracePeriod = time.Second

// isolate makes cmd lead its own process group.
func isolate(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.SysProcAttr.Pgid = 0
}

// stopTree asks the child's whole group to stop, then insists.
//
// The child leads its own group, so its pid doubles as the group id. The
// interrupt is forwarded because the terminal no longer delivers it, and a
// server killed outright never flushes.
//
// The kill is unconditional because the leader exiting does not mean the
// group is empty: a reloading server's worker can ignore the interrupt and
// outlive the supervisor that took it, and it would then keep serving the old
// build.
func stopTree(c *OwnedChild) error {
	signalGroup(c.Pid(), syscall.SIGINT)
	c.waitForExit(stopGracePeriod)
	signalGroup(c.Pid(), syscall.SIGKILL)
	return nil
}

// signalGroup sends signal to group. The escalating KILL routinely finds a
// group the INT already emptied; the caller chose to insist, so that is not a
// failure.
func signalGroup(group int, signal syscall.Signal) {
	_ = syscall.Kill(-group, signal)
}

// groupExists reports whether any process, zombies included, is still in
// group.
func groupExists(group int) bool {
	return syscall.Kill(-group, 0) == nil
}

func killLateGroup(group int) {
	signalGroup(group, syscall.SIGKILL)
}

// terminateGroups interrupts groups together, gives them one shared grace
// period, then kills whatever is left — the same escalation as stopTree,
// without the children to reap.
func terminateGroups(groups []int) {
	for _, group := range groups {
		signalGroup(group, syscall.SIGINT)
	}
	deadline := time.Now().Add(stopGracePeriod)
	for time.Now().Before(deadline) && anyGroupExists(groups) {
		time.Sleep(pollInterval)
	}
	for _, group := range groups {
		signalGroup(group, syscall.SIGKILL)
	}
}

func anyGroupExists(groups []int) bool {
	for _, group := range groups {
		if groupExists(group) {
			return true
		}
	}
	return false
}

// waitForExit waits up to timeout for the child to exit, and reports whether
// it did.
func (c *OwnedChild) waitForExit(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-c.done:
		return true
	case <-timer.C:
		return false
	}
}
