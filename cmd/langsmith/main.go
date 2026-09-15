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
	if executed, err := rootCmd.ExecuteC(); err != nil {
		format := cmd.GetFormat()
		// Only early command-discovery failures need an argument fallback.
		// Parsed string values may themselves look like output-format flags.
		if executed == nil || !executed.Flags().Parsed() {
			format = errorOutputFormat(os.Args[1:], format)
		}
		_ = writeCommandError(os.Stderr, err, format)
		os.Exit(1)
	}
}
