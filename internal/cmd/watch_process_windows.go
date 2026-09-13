package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func terminateWatchProcess(process *os.Process) error {
	return exec.CommandContext(context.Background(), "taskkill", "/T", "/F", "/PID", fmt.Sprint(process.Pid)).Run()
}
