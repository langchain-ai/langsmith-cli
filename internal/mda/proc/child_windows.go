//go:build windows

package proc

import (
	"errors"
	"os/exec"
	"strconv"
	"syscall"
)

// isolate starts cmd in a new process group.
func isolate(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// stopTree takes the child's tree down in one step: Windows has no signal to
// forward.
//
// taskkill failing, as it does for a tree that already ended, is not an
// error; only failing to run it is.
func stopTree(c *OwnedChild) error {
	err := exec.Command("taskkill", "/PID", strconv.Itoa(c.Pid()), "/T", "/F").Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil
	}
	return err
}

// killLateGroup does nothing: only the unix termination watcher closes the
// registry.
func killLateGroup(int) {}

// terminateGroups does nothing: Windows has no group walk yet, so its
// children are not stopped here.
func terminateGroups([]int) {}
