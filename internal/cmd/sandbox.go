package cmd

import "github.com/spf13/cobra"

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage and interact with sandboxes (experimental)",
		Long: `Manage and interact with sandboxes (currently in experimental preview).

Sandboxes are isolated Firecracker microVMs booted from snapshots.
Use 'snapshot build' to create a snapshot from a Docker image, then
'sandbox create' to launch a VM from it.

Examples:
  langsmith sandbox create my-vm --snapshot-id <id>
  langsmith sandbox list
  langsmith sandbox console my-vm
  langsmith sandbox exec my-vm -- uname -a
  langsmith sandbox tunnel my-vm --remote-port 5432
  langsmith sandbox ssh-setup my-vm`,
	}

	// Lifecycle
	cmd.AddCommand(newSandboxCreateCmd())
	cmd.AddCommand(newSandboxListCmd())
	cmd.AddCommand(newSandboxGetCmd())
	cmd.AddCommand(newSandboxUpdateCmd())
	cmd.AddCommand(newSandboxDeleteCmd())
	cmd.AddCommand(newSandboxStartCmd())
	cmd.AddCommand(newSandboxStopCmd())
	cmd.AddCommand(newSandboxWaitCmd())

	// Connectivity
	cmd.AddCommand(newSandboxExecCmd())
	cmd.AddCommand(newSandboxConsoleCmd())
	cmd.AddCommand(newSandboxTunnelCmd())
	cmd.AddCommand(newSandboxSSHSetupCmd())

	// Sub-resources
	cmd.AddCommand(newSandboxSnapshotCmd())

	return cmd
}
