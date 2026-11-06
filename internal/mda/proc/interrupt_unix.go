//go:build unix

package proc

import (
	"os"
	"syscall"
)

var terminationSignals = []os.Signal{syscall.SIGTERM, syscall.SIGHUP}

// terminate takes down every child group and exits with 128 + sig.
func terminate(sig os.Signal) {
	terminating.Store(true)
	TerminateRegistered()
	code := 128
	if number, ok := sig.(syscall.Signal); ok {
		code += int(number)
	}
	os.Exit(code)
}
