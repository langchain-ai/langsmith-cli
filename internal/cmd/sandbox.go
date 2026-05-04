package cmd

import "github.com/spf13/cobra"

type sandboxMessage struct {
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

func newSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage and interact with sandboxes (experimental)",
		Long: `Manage and interact with sandboxes (currently in experimental preview).

Common workflows:

  # Build a snapshot from a Docker image
  langsmith sandbox snapshot build my-snap --docker-image ubuntu:24.04

  # Create a sandbox from the snapshot
  langsmith sandbox create my-vm --snapshot-id <id>

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
	cmd.AddCommand(sandboxCreateCommand.Cobra())
	cmd.AddCommand(sandboxListCommand.Cobra())
	cmd.AddCommand(sandboxGetCommand.Cobra())
	cmd.AddCommand(sandboxUpdateCommand.Cobra())
	cmd.AddCommand(sandboxDeleteCommand.Cobra())
	cmd.AddCommand(sandboxStartCommand.Cobra())
	cmd.AddCommand(sandboxStopCommand.Cobra())

	// Connectivity
	cmd.AddCommand(newSandboxExecCmd())
	cmd.AddCommand(newSandboxConsoleCmd())
	cmd.AddCommand(newSandboxTunnelCmd())
	cmd.AddCommand(newSandboxSSHSetupCmd())

	// Sub-resources
	cmd.AddCommand(sandboxSnapshotCommand.Cobra())

	return cmd
}
