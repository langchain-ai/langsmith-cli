package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

const defaultBoxPollInterval = 2 * time.Second

func waitForBoxReady(ctx context.Context, c *client.Client, name string, timeout time.Duration) (*langsmith.SandboxBoxGetStatusResponse, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := c.SDK.Sandboxes.Boxes.GetStatus(ctx, name)
		if err != nil {
			return nil, err
		}
		switch resp.Status {
		case "ready":
			return resp, nil
		case "failed":
			return resp, fmt.Errorf("sandbox entered %s state", resp.Status)
		}
		time.Sleep(defaultBoxPollInterval)
	}
	return nil, fmt.Errorf("timed out after %s waiting for sandbox", timeout)
}

func newSandboxCreateCmd() *cobra.Command {
	var (
		snapshotID  string
		vcpus       int
		memory      string
		rootfs      string
		proxyConfig string
		wait        bool
		timeoutSec  int
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a sandbox VM from a snapshot",
		Long: `Create a sandbox VM from a snapshot.

The --proxy-config flag accepts inline JSON or a file path prefixed with @.
The proxy config controls which outbound HTTP requests the sandbox proxy
allows and what headers to inject. Format:

  {
    "rules": [{
      "name": "openai",
      "match_hosts": ["api.openai.com"],
      "match_paths": [],
      "headers": [
        {"name": "Authorization", "type": "opaque", "value": "Bearer sk-..."},
        {"name": "X-Key", "type": "workspace_secret", "value": "Bearer {OPENAI_API_KEY}"}
      ],
      "enabled": true
    }],
    "no_proxy": ["internal.example.com"],
    "access_control": {
      "allow_list": ["*.openai.com", "*.anthropic.com"],
      "deny_list": []
    }
  }

Header types: "plaintext" (literal value), "opaque" (encrypted, hidden in API
responses), "workspace_secret" (resolved from workspace secrets via {KEY}).

Examples:
  langsmith sandbox create my-vm --snapshot-id <id>
  langsmith sandbox create my-vm --snapshot-id <id> --vcpus 4 --memory 1gb
  langsmith sandbox create my-vm --snapshot-id <id> --rootfs-capacity 8gb --wait
  langsmith sandbox create my-vm --snapshot-id <id> --proxy-config @proxy.json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			if snapshotID == "" {
				ExitError("--snapshot-id is required")
			}

			c := MustGetClient()
			ctx := context.Background()

			memBytes, err := parseByteSize(memory)
			if err != nil {
				ExitErrorf("invalid --memory: %v", err)
			}

			params := langsmith.SandboxBoxNewParams{
				Name:       langsmith.F(name),
				SnapshotID: langsmith.F(snapshotID),
				Vcpus:      langsmith.F(int64(vcpus)),
				MemBytes:   langsmith.F(memBytes),
			}
			if rootfs != "" {
				rootfsBytes, err := parseByteSize(rootfs)
				if err != nil {
					ExitErrorf("invalid --rootfs-capacity: %v", err)
				}
				params.FsCapacityBytes = langsmith.F(rootfsBytes)
			}
			if proxyConfig != "" {
				pc, err := loadJSONArg(proxyConfig)
				if err != nil {
					ExitErrorf("invalid --proxy-config: %v", err)
				}
				params.ProxyConfig = langsmith.Raw[langsmith.SandboxBoxNewParamsProxyConfig](pc)
			}
			if wait {
				params.WaitForReady = langsmith.F(true)
				params.Timeout = langsmith.F(int64(timeoutSec))
			}

			resp, err := c.SDK.Sandboxes.Boxes.New(ctx, params)
			if err != nil {
				ExitErrorf("creating sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().StringVar(&snapshotID, "snapshot-id", "", "Snapshot ID to boot from (required)")
	cmd.Flags().IntVar(&vcpus, "vcpus", 2, "Number of vCPUs")
	cmd.Flags().StringVar(&memory, "memory", "512mb", "Memory with unit (e.g. 512mb, 1gb)")
	cmd.Flags().StringVar(&rootfs, "rootfs-capacity", "", "Root filesystem capacity with unit (e.g. 4gb, 8gb)")
	cmd.Flags().StringVar(&proxyConfig, "proxy-config", "", "Proxy config as JSON or @file.json")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the sandbox to become ready")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Timeout in seconds when using --wait")

	return cmd
}

func newSandboxListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all sandboxes",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Sandboxes.Boxes.List(ctx, langsmith.SandboxBoxListParams{})
			if err != nil {
				ExitErrorf("listing sandboxes: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" && len(resp.Sandboxes) > 0 {
				columns := []string{"Name", "Status", "VCPUs", "Mem", "Rootfs", "Snapshot", "Created"}
				var rows [][]string
				for _, b := range resp.Sandboxes {
					snap := "-"
					if b.SnapshotID != "" {
						snap = b.SnapshotID
						if len(snap) > 8 {
							snap = snap[:8] + "..."
						}
					}
					diskStr := "-"
					if b.FsCapacityBytes > 0 {
						diskStr = formatBytes(b.FsCapacityBytes)
					}
					mem := "-"
					if b.MemBytes > 0 {
						mem = formatBytes(b.MemBytes)
					}
					vcpusStr := "-"
					if b.Vcpus > 0 {
						vcpusStr = fmt.Sprintf("%d", b.Vcpus)
					}
					rows = append(rows, []string{
						b.Name, b.Status, vcpusStr, mem, diskStr, snap, formatTime(b.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, "Sandboxes")
			} else {
				output.OutputJSON(resp.Sandboxes, "")
			}
		},
	}
	return cmd
}

func newSandboxGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get a sandbox by name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Sandboxes.Boxes.Get(ctx, args[0])
			if err != nil {
				ExitErrorf("getting sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}
	return cmd
}

func newSandboxUpdateCmd() *cobra.Command {
	var (
		vcpus       int
		memory      string
		rootfs      string
		proxyConfig string
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update sandbox resources (takes effect on next start)",
		Long: `Update sandbox resources or proxy configuration.

Resource changes (--vcpus, --memory, --rootfs-capacity) take effect on next start.
Proxy config changes take effect immediately.

The --proxy-config flag accepts inline JSON or @file.json. See "create --help"
for the proxy config JSON format.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.SandboxBoxUpdateParams{}
			if cmd.Flags().Changed("vcpus") {
				params.Vcpus = langsmith.F(int64(vcpus))
			}
			if cmd.Flags().Changed("memory") {
				memBytes, err := parseByteSize(memory)
				if err != nil {
					ExitErrorf("invalid --memory: %v", err)
				}
				params.MemBytes = langsmith.F(memBytes)
			}
			if cmd.Flags().Changed("rootfs-capacity") {
				rootfsBytes, err := parseByteSize(rootfs)
				if err != nil {
					ExitErrorf("invalid --rootfs-capacity: %v", err)
				}
				params.FsCapacityBytes = langsmith.F(rootfsBytes)
			}
			if cmd.Flags().Changed("proxy-config") {
				pc, err := loadJSONArg(proxyConfig)
				if err != nil {
					ExitErrorf("invalid --proxy-config: %v", err)
				}
				params.ProxyConfig = langsmith.Raw[langsmith.SandboxBoxUpdateParamsProxyConfig](pc)
			}

			if !cmd.Flags().Changed("vcpus") && !cmd.Flags().Changed("memory") && !cmd.Flags().Changed("rootfs-capacity") && !cmd.Flags().Changed("proxy-config") {
				ExitError("nothing to update (use --vcpus, --memory, --rootfs-capacity, or --proxy-config)")
			}

			resp, err := c.SDK.Sandboxes.Boxes.Update(ctx, args[0], params)
			if err != nil {
				ExitErrorf("updating sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().IntVar(&vcpus, "vcpus", 0, "Number of vCPUs")
	cmd.Flags().StringVar(&memory, "memory", "", "Memory with unit (e.g. 512mb, 1gb)")
	cmd.Flags().StringVar(&rootfs, "rootfs-capacity", "", "Root filesystem capacity with unit (e.g. 4gb, 8gb)")
	cmd.Flags().StringVar(&proxyConfig, "proxy-config", "", "Proxy config as JSON or @file.json")

	return cmd
}

func newSandboxDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a sandbox",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			if err := c.SDK.Sandboxes.Boxes.Delete(ctx, args[0]); err != nil {
				ExitErrorf("deleting sandbox: %v", err)
			}

			fmt.Println("Sandbox deleted.")
		},
	}
	return cmd
}

func newSandboxWaitCmd() *cobra.Command {
	var timeoutSec int

	cmd := &cobra.Command{
		Use:   "wait <name>",
		Short: "Wait for a sandbox to become ready",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			_, err := waitForBoxReady(ctx, c, args[0], time.Duration(timeoutSec)*time.Second)
			if err != nil {
				ExitErrorf("%v", err)
			}
			fmt.Printf("Sandbox %s is ready\n", args[0])
		},
	}

	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Timeout in seconds")

	return cmd
}

func newSandboxExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <name> -- <command>",
		Short: "Execute a command inside a sandbox",
		Long: `Execute a one-off command inside a running sandbox and print its output.

Examples:
  langsmith sandbox exec my-vm -- uname -a
  langsmith sandbox exec my-vm -- ls -la /
  langsmith sandbox exec my-vm -- cat /etc/os-release`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Everything after "--" is the command.
			cmdArgs := cmd.ArgsLenAtDash()
			if cmdArgs < 0 || cmdArgs >= len(args) {
				return fmt.Errorf("usage: langsmith sandbox exec <name> -- <command>")
			}
			command := args[cmdArgs:]
			if len(command) == 0 {
				return fmt.Errorf("no command specified")
			}

			ctx := context.Background()
			ep, err := resolveSandbox(ctx, name)
			if err != nil {
				return err
			}

			var result execResult
			if err := dataplanePost(ep.DataplaneURL, "/execute", map[string]interface{}{"command": command}, &result); err != nil {
				return fmt.Errorf("execute: %w", err)
			}

			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Fprint(os.Stderr, result.Stderr)
			}
			if result.ExitCode != 0 {
				os.Exit(result.ExitCode)
			}
			return nil
		},
	}
	return cmd
}

func newSandboxStartCmd() *cobra.Command {
	var (
		wait       bool
		timeoutSec int
	)

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a stopped sandbox",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			if _, err := c.SDK.Sandboxes.Boxes.Start(ctx, args[0]); err != nil {
				ExitErrorf("starting sandbox: %v", err)
			}

			if wait {
				if _, err := waitForBoxReady(ctx, c, args[0], time.Duration(timeoutSec)*time.Second); err != nil {
					ExitErrorf("%v", err)
				}
			}
			fmt.Printf("Sandbox %s started\n", args[0])
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the sandbox to become ready")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Timeout in seconds when using --wait")

	return cmd
}

func newSandboxStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running sandbox (preserves data)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			if err := c.SDK.Sandboxes.Boxes.Stop(ctx, args[0]); err != nil {
				ExitErrorf("stopping sandbox: %v", err)
			}

			fmt.Println("Sandbox stopped.")
		},
	}
	return cmd
}
