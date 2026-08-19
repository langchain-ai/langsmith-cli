//go:build !windows

package cmd

import "os"

func terminateWatchProcess(process *os.Process) error {
	return process.Kill()
}
