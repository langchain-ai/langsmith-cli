package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSandboxConsoleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "console <name>",
		Short: "Open an interactive shell inside a sandbox (not supported on Windows)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("sandbox console is not supported on Windows; use SSH instead: langsmith sandbox ssh-setup %s", args[0])
		},
	}
}
