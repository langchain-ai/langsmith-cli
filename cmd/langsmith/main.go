package main

import (
	"fmt"
	"os"

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
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
