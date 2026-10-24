package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/langchain-ai/langsmith-cli/internal/cmd"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd(version, fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date))
	if err := rootCmd.Execute(); err != nil {
		var childExit *exec.ExitError
		if errors.As(err, &childExit) && childExit.ExitCode() >= 0 {
			os.Exit(childExit.ExitCode())
		}
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
