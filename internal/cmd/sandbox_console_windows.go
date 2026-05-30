package cmd

import (
	"context"
	"fmt"

	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/spf13/cobra"
)

var sandboxConsoleCommand = structured.Command[struct{}]{
	Use:          "console [name]",
	Short:        "Open an interactive shell inside a sandbox (not supported on Windows)",
	Args:         cobra.MaximumNArgs(1),
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		name, err := resolveSandboxName(cmd, args)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("sandbox console is not supported on Windows; use SSH instead: langsmith sandbox ssh-setup %s", name)
	},
}

func newSandboxConsoleCmd() *cobra.Command {
	return sandboxConsoleCommand.Cobra()
}
