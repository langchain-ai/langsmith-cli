package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// Sandbox box (claim) API types.

type boxResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	TenantID    string  `json:"tenant_id"`
	SnapshotID  *string `json:"snapshot_id,omitempty"`
	VCPUs       int     `json:"vcpus,omitempty"`
	MemMB       int     `json:"mem_mb,omitempty"`
	FsSizeBytes *int64  `json:"fs_size_bytes,omitempty"`
	Status      string  `json:"status"`
	DataplaneURL *string `json:"dataplane_url,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type boxListResponse struct {
	Sandboxes []boxResponse `json:"sandboxes"`
}

type boxStatusResponse struct {
	Status string `json:"status"`
	PodIP  string `json:"pod_ip,omitempty"`
}

func newSandboxBoxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "box",
		Short: "Create, manage, and connect to sandbox VMs",
		Long: `Create, manage, and connect to sandbox VMs.

Sandboxes are Firecracker microVMs booted from snapshots. Use 'snapshot create'
to build a snapshot first, then 'box create' to launch a VM from it.

Examples:
  langsmith sandbox box create --name my-vm --snapshot-id <id> --vcpus 2 --mem-mb 512
  langsmith sandbox box list
  langsmith sandbox box get my-vm
  langsmith sandbox box wait my-vm
  langsmith sandbox box delete my-vm`,
	}

	cmd.AddCommand(newBoxCreateCmd())
	cmd.AddCommand(newBoxListCmd())
	cmd.AddCommand(newBoxGetCmd())
	cmd.AddCommand(newBoxUpdateCmd())
	cmd.AddCommand(newBoxDeleteCmd())
	cmd.AddCommand(newBoxWaitCmd())
	cmd.AddCommand(newBoxExecCmd())
	cmd.AddCommand(newBoxStartCmd())
	cmd.AddCommand(newBoxStopCmd())

	return cmd
}

func newBoxCreateCmd() *cobra.Command {
	var (
		name       string
		snapshotID string
		vcpus      int
		memMB      int
		fsSizeGB   int
		wait       bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a sandbox VM from a snapshot",
		Run: func(cmd *cobra.Command, args []string) {
			if name == "" {
				exitError("--name is required")
			}
			if snapshotID == "" {
				exitError("--snapshot-id is required")
			}

			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"name":        name,
				"snapshot_id": snapshotID,
				"vcpus":       vcpus,
				"mem_mb":      memMB,
			}
			if fsSizeGB > 0 {
				body["fs_size_bytes"] = int64(fsSizeGB) * 1024 * 1024 * 1024
			}
			if wait {
				body["wait_for_ready"] = true
				body["timeout"] = 60
			}

			var resp boxResponse
			if err := c.RawPost(ctx, "/v2/sandboxes/boxes", body, &resp); err != nil {
				exitErrorf("creating sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Sandbox name (required)")
	cmd.Flags().StringVar(&snapshotID, "snapshot-id", "", "Snapshot ID to boot from (required)")
	cmd.Flags().IntVar(&vcpus, "vcpus", 2, "Number of vCPUs (default: 2)")
	cmd.Flags().IntVar(&memMB, "mem-mb", 512, "Memory in MB (default: 512)")
	cmd.Flags().IntVar(&fsSizeGB, "fs-size-gb", 0, "Disk size in GB (default: inherit from snapshot)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the sandbox to become ready")

	return cmd
}

func newBoxListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all sandboxes",
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			var resp boxListResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/boxes", &resp); err != nil {
				exitErrorf("listing sandboxes: %v", err)
			}

			if len(resp.Sandboxes) == 0 {
				output.OutputJSON(resp.Sandboxes, "")
				return
			}

			columns := []string{"Name", "Status", "VCPUs", "Mem", "Disk", "Snapshot", "Created"}
			var rows [][]string
			for _, b := range resp.Sandboxes {
				snap := "-"
				if b.SnapshotID != nil {
					snap = (*b.SnapshotID)[:8] + "..."
				}
				disk := "-"
				if b.FsSizeBytes != nil {
					disk = formatBytes(*b.FsSizeBytes)
				}
				mem := "-"
				if b.MemMB > 0 {
					mem = fmt.Sprintf("%d MB", b.MemMB)
				}
				vcpus := "-"
				if b.VCPUs > 0 {
					vcpus = fmt.Sprintf("%d", b.VCPUs)
				}
				rows = append(rows, []string{
					b.Name, b.Status, vcpus, mem, disk, snap, formatTime(b.CreatedAt),
				})
			}
			output.OutputTable(columns, rows, "Sandboxes")
		},
	}
	return cmd
}

func newBoxGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get a sandbox by name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			var resp boxResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+args[0], &resp); err != nil {
				exitErrorf("getting sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}
	return cmd
}

func newBoxUpdateCmd() *cobra.Command {
	var (
		vcpus    int
		memMB    int
		fsSizeGB int
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update sandbox resources (takes effect on next start)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{}
			if cmd.Flags().Changed("vcpus") {
				body["vcpus"] = vcpus
			}
			if cmd.Flags().Changed("mem-mb") {
				body["mem_mb"] = memMB
			}
			if cmd.Flags().Changed("fs-size-gb") {
				body["fs_size_bytes"] = int64(fsSizeGB) * 1024 * 1024 * 1024
			}

			if len(body) == 0 {
				exitError("nothing to update (use --vcpus, --mem-mb, or --fs-size-gb)")
			}

			var resp boxResponse
			if err := c.RawPatch(ctx, "/v2/sandboxes/boxes/"+args[0], body, &resp); err != nil {
				exitErrorf("updating sandbox: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().IntVar(&vcpus, "vcpus", 0, "Number of vCPUs")
	cmd.Flags().IntVar(&memMB, "mem-mb", 0, "Memory in MB")
	cmd.Flags().IntVar(&fsSizeGB, "fs-size-gb", 0, "Disk size in GB")

	return cmd
}

func newBoxDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a sandbox",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			if err := c.RawDelete(ctx, "/v2/sandboxes/boxes/"+args[0], nil); err != nil {
				exitErrorf("deleting sandbox: %v", err)
			}

			fmt.Println("Sandbox deleted.")
		},
	}
	return cmd
}

func newBoxWaitCmd() *cobra.Command {
	var timeoutSec int

	cmd := &cobra.Command{
		Use:   "wait <name>",
		Short: "Wait for a sandbox to become ready",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()
			deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

			for time.Now().Before(deadline) {
				var resp boxStatusResponse
				if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+args[0]+"/status", &resp); err != nil {
					exitErrorf("getting sandbox status: %v", err)
				}

				switch resp.Status {
				case "ready":
					fmt.Printf("Sandbox %s is ready (pod IP: %s)\n", args[0], resp.PodIP)
					return
				case "failed":
					exitErrorf("sandbox failed to start")
				}

				time.Sleep(2 * time.Second)
			}

			exitErrorf("timed out after %ds waiting for sandbox", timeoutSec)
		},
	}

	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Timeout in seconds (default: 120)")

	return cmd
}

func newBoxExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec <name> -- <command>",
		Short: "Execute a command inside a sandbox",
		Long: `Execute a one-off command inside a running sandbox and print its output.

Examples:
  langsmith sandbox box exec my-vm -- uname -a
  langsmith sandbox box exec my-vm -- ls -la /
  langsmith sandbox box exec my-vm -- cat /etc/os-release`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Everything after "--" is the command.
			cmdArgs := cmd.ArgsLenAtDash()
			if cmdArgs < 0 || cmdArgs >= len(args) {
				return fmt.Errorf("usage: langsmith sandbox box exec <name> -- <command>")
			}
			command := args[cmdArgs:]
			if len(command) == 0 {
				return fmt.Errorf("no command specified")
			}

			// Resolve dataplane URL.
			dpURL := os.Getenv("SANDBOX_DIRECT_URL")
			var tenantID string
			if dpURL == "" {
				c := mustGetClient()
				ctx := context.Background()

				var box boxResponse
				if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+name, &box); err != nil {
					return fmt.Errorf("getting sandbox: %w", err)
				}
				if box.Status != "ready" {
					return fmt.Errorf("sandbox %q is not ready (status: %s)", name, box.Status)
				}
				if box.DataplaneURL == nil || *box.DataplaneURL == "" {
					return fmt.Errorf("sandbox %q has no dataplane URL", name)
				}
				dpURL = *box.DataplaneURL
				tenantID = box.TenantID
			}

			// POST /execute
			execURL := strings.TrimRight(dpURL, "/") + "/execute"
			body, _ := json.Marshal(map[string]interface{}{"command": command})

			req, err := http.NewRequest(http.MethodPost, execURL, bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("build request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			for k, v := range sandboxAuthHeaders(tenantID) {
				req.Header.Set(k, v)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("execute: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("execute failed (HTTP %d): %s", resp.StatusCode, string(body))
			}

			var result struct {
				Stdout   string `json:"stdout"`
				Stderr   string `json:"stderr"`
				ExitCode int    `json:"exit_code"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("decode response: %w", err)
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

func newBoxStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a stopped sandbox",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			var resp boxResponse
			if err := c.RawPost(ctx, "/v2/sandboxes/boxes/"+args[0]+"/start", nil, &resp); err != nil {
				exitErrorf("starting sandbox: %v", err)
			}

			// Poll for ready.
			deadline := time.Now().Add(120 * time.Second)
			for time.Now().Before(deadline) {
				var status boxStatusResponse
				if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+args[0]+"/status", &status); err != nil {
					exitErrorf("getting sandbox status: %v", err)
				}
				if status.Status == "ready" {
					fmt.Printf("Sandbox %s is ready\n", args[0])
					return
				}
				if status.Status == "failed" || status.Status == "stopped" {
					exitErrorf("sandbox failed to start (status: %s)", status.Status)
				}
				time.Sleep(2 * time.Second)
			}
			exitErrorf("timed out waiting for sandbox to start")
		},
	}
	return cmd
}

func newBoxStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running sandbox (preserves data)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			if err := c.RawPost(ctx, "/v2/sandboxes/boxes/"+args[0]+"/stop", nil, nil); err != nil {
				exitErrorf("stopping sandbox: %v", err)
			}

			fmt.Println("Sandbox stopped.")
		},
	}
	return cmd
}
