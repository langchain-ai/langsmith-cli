package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// Sandbox API types.

type boxResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	SnapshotID      *string `json:"snapshot_id,omitempty"`
	VCPUs           int     `json:"vcpus,omitempty"`
	MemBytes        int64   `json:"mem_bytes,omitempty"`
	FsCapacityBytes *int64  `json:"fs_capacity_bytes,omitempty"`
	Status          string  `json:"status"`
	DataplaneURL    *string `json:"dataplane_url,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type boxListResponse struct {
	Sandboxes []boxResponse `json:"sandboxes"`
}

type boxStatusResponse struct {
	Status        string  `json:"status"`
	StatusMessage *string `json:"status_message,omitempty"`
}

const defaultBoxPollInterval = 2 * time.Second

// waitForBoxReady polls until the sandbox reaches "ready" or a terminal state.
func waitForBoxReady(ctx context.Context, c *client.Client, name string, timeout time.Duration) (boxStatusResponse, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var resp boxStatusResponse
		if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+name+"/status", &resp); err != nil {
			return boxStatusResponse{}, err
		}
		switch resp.Status {
		case "ready":
			return resp, nil
		case "failed", "stopped":
			return resp, fmt.Errorf("sandbox entered %s state", resp.Status)
		}
		time.Sleep(defaultBoxPollInterval)
	}
	return boxStatusResponse{}, fmt.Errorf("timed out after %s waiting for sandbox", timeout)
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

			body := map[string]any{
				"name":        name,
				"snapshot_id": snapshotID,
				"vcpus":       vcpus,
				"mem_bytes":   memBytes,
			}
			if rootfs != "" {
				rootfsBytes, err := parseByteSize(rootfs)
				if err != nil {
					ExitErrorf("invalid --rootfs-capacity: %v", err)
				}
				body["fs_capacity_bytes"] = rootfsBytes
			}
			if proxyConfig != "" {
				pc, err := loadJSONArg(proxyConfig)
				if err != nil {
					ExitErrorf("invalid --proxy-config: %v", err)
				}
				body["proxy_config"] = pc
			}
			if wait {
				body["wait_for_ready"] = true
				body["timeout"] = timeoutSec
			}

			var resp boxResponse
			if err := c.RawPost(ctx, "/v2/sandboxes/boxes", body, &resp); err != nil {
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

			var resp boxListResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/boxes", &resp); err != nil {
				ExitErrorf("listing sandboxes: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" && len(resp.Sandboxes) > 0 {
				columns := []string{"Name", "Status", "VCPUs", "Mem", "Rootfs", "Snapshot", "Created"}
				var rows [][]string
				for _, b := range resp.Sandboxes {
					snap := "-"
					if b.SnapshotID != nil {
						if len(*b.SnapshotID) > 8 {
							snap = (*b.SnapshotID)[:8] + "..."
						} else {
							snap = *b.SnapshotID
						}
					}
					diskStr := "-"
					if b.FsCapacityBytes != nil {
						diskStr = formatBytes(*b.FsCapacityBytes)
					}
					mem := "-"
					if b.MemBytes > 0 {
						mem = formatBytes(b.MemBytes)
					}
					vcpusStr := "-"
					if b.VCPUs > 0 {
						vcpusStr = fmt.Sprintf("%d", b.VCPUs)
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

			var resp boxResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+args[0], &resp); err != nil {
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

			body := map[string]any{}
			if cmd.Flags().Changed("vcpus") {
				body["vcpus"] = vcpus
			}
			if cmd.Flags().Changed("memory") {
				memBytes, err := parseByteSize(memory)
				if err != nil {
					ExitErrorf("invalid --memory: %v", err)
				}
				body["mem_bytes"] = memBytes
			}
			if cmd.Flags().Changed("rootfs-capacity") {
				rootfsBytes, err := parseByteSize(rootfs)
				if err != nil {
					ExitErrorf("invalid --rootfs-capacity: %v", err)
				}
				body["fs_capacity_bytes"] = rootfsBytes
			}
			if cmd.Flags().Changed("proxy-config") {
				pc, err := loadJSONArg(proxyConfig)
				if err != nil {
					ExitErrorf("invalid --proxy-config: %v", err)
				}
				body["proxy_config"] = pc
			}

			if len(body) == 0 {
				ExitError("nothing to update (use --vcpus, --memory, --rootfs-capacity, or --proxy-config)")
			}

			var resp boxResponse
			if err := c.RawPatch(ctx, "/v2/sandboxes/boxes/"+args[0], body, &resp); err != nil {
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

			if err := c.RawDelete(ctx, "/v2/sandboxes/boxes/"+args[0], nil); err != nil {
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

			if err := c.RawPost(ctx, "/v2/sandboxes/boxes/"+args[0]+"/start", nil, nil); err != nil {
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

			if err := c.RawPost(ctx, "/v2/sandboxes/boxes/"+args[0]+"/stop", nil, nil); err != nil {
				ExitErrorf("stopping sandbox: %v", err)
			}

			fmt.Println("Sandbox stopped.")
		},
	}
	return cmd
}
