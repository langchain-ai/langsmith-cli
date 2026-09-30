package main

import (
	"fmt"
	"os"

	"github.com/langchain-ai/langsmith-cli/internal/cmd"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd(version, fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date))
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		os.Exit(structured.WriteError(os.Stdout, os.Stderr, cmd, err))
	}
}
