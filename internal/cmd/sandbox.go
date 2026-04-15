package cmd

import "github.com/spf13/cobra"

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage and interact with sandboxes (experimental)",
		Long: `Manage and interact with sandboxes (currently in experimental preview).

Common workflows:

  # Build a snapshot from a Docker image and wait for it to be ready
  langsmith sandbox snapshot build my-snap --docker-image ubuntu:24.04 --wait

  # Create a sandbox from the snapshot and wait for it to boot
  langsmith sandbox create my-vm --snapshot-id <id> --wait

  # Run a command inside the sandbox
  langsmith sandbox exec my-vm -- uname -a

  # Open an interactive shell
  langsmith sandbox console my-vm

  # Tunnel a remote port (e.g. Postgres) to localhost
  langsmith sandbox tunnel my-vm --remote-port 5432

  # Set up SSH access (writes ~/.ssh/config so "ssh sandbox-my-vm" works)
  # Requires sshd to be installed in your Docker image.
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
