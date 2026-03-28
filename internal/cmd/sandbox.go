package cmd

import "github.com/spf13/cobra"

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage and interact with sandboxes (experimental)",
		Long: `Manage and interact with sandboxes (currently in experimental preview).

Sandboxes are isolated execution environments. Use subcommands to
create tunnels and interact with services running inside them.

Examples:
  langsmith sandbox tunnel --url https://sandboxes.langsmith.com/my-sandbox --remote-port 5432`,
	}

	cmd.AddCommand(newSandboxTunnelCmd())
	cmd.AddCommand(newSandboxSnapshotCmd())
	cmd.AddCommand(newSandboxBoxCmd())
	cmd.AddCommand(newSandboxConsoleCmd())
	return cmd
}
