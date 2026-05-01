package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

func newSandboxSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage sandbox snapshots (ext4 rootfs images)",
		Long: `Manage sandbox snapshots — ext4 rootfs images built from Docker images
or captured from running sandboxes. Snapshots are used to create new sandboxes.

Examples:
  langsmith sandbox snapshot list
  langsmith sandbox snapshot build my-snap --docker-image ubuntu:24.04
  langsmith sandbox snapshot capture my-snap --box my-vm
  langsmith sandbox snapshot get <id>
  langsmith sandbox snapshot delete <id>
  langsmith sandbox snapshot wait <id>`,
	}

	cmd.AddCommand(newSnapshotListCmd())
	cmd.AddCommand(newSnapshotBuildCmd())
	cmd.AddCommand(newSnapshotCaptureCmd())
	cmd.AddCommand(newSnapshotGetCmd())
	cmd.AddCommand(newSnapshotDeleteCmd())
	cmd.AddCommand(newSnapshotWaitCmd())

	return cmd
}

func newSnapshotListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Sandboxes.Snapshots.List(ctx, langsmith.SandboxSnapshotListParams{})
			if err != nil {
				ExitErrorf("listing snapshots: %v", err)
			}

			fmt_ := GetFormat()

			if fmt_ == "pretty" && len(resp.Snapshots) > 0 {
				columns := []string{"ID", "Name", "Image", "Status", "Size", "Created"}
				var rows [][]string
				for _, s := range resp.Snapshots {
					image := "-"
					if s.DockerImage != "" {
						image = s.DockerImage
					}
					size := "-"
					if s.FsUsedBytes > 0 {
						size = formatBytes(s.FsUsedBytes)
					}
					rows = append(rows, []string{
						s.ID, s.Name, image, s.Status, size, formatTime(s.CreatedAt),
					})
				}
				output.OutputTable(columns, rows, "Snapshots")
			} else {
				output.OutputJSON(resp.Snapshots, "")
			}
		},
	}
	return cmd
}

func newSnapshotBuildCmd() *cobra.Command {
	var (
		dockerImage string
		capacity    string
		registryID  string
		wait        bool
		timeoutSec  int
	)

	cmd := &cobra.Command{
		Use:   "build <name>",
		Short: "Build a snapshot from a Docker image",
		Long: `Build a snapshot from a Docker image.

Examples:
  langsmith sandbox snapshot build my-snap --docker-image ubuntu:24.04
  langsmith sandbox snapshot build my-snap --docker-image ubuntu:24.04 --capacity 8gb --wait`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			if dockerImage == "" {
				ExitError("--docker-image is required")
			}

			capBytes, err := parseByteSize(capacity)
			if err != nil {
				ExitErrorf("invalid --capacity: %v", err)
			}

			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.SandboxSnapshotNewParams{
				Name:            langsmith.F(name),
				DockerImage:     langsmith.F(dockerImage),
				FsCapacityBytes: langsmith.F(capBytes),
			}
			if registryID != "" {
				params.RegistryID = langsmith.F(registryID)
			}

			resp, err := c.SDK.Sandboxes.Snapshots.New(ctx, params)
			if err != nil {
				ExitErrorf("building snapshot: %v", err)
			}

			var result any = resp
			if wait {
				result = waitForSnapshot(ctx, c, resp.ID, time.Duration(timeoutSec)*time.Second)
			}

			output.OutputJSON(result, "")
		},
	}

	cmd.Flags().StringVar(&dockerImage, "docker-image", "", "Docker image to build from (required)")
	cmd.Flags().StringVar(&capacity, "capacity", "4gb", "Filesystem capacity with unit (e.g. 4gb, 8gb)")
	cmd.Flags().StringVar(&registryID, "registry-id", "", "Registry ID for private images")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the snapshot to become ready")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "Timeout in seconds when using --wait")

	return cmd
}

func newSnapshotCaptureCmd() *cobra.Command {
	var (
		boxName    string
		checkpoint string
		wait       bool
		timeoutSec int
	)

	cmd := &cobra.Command{
		Use:   "capture <name>",
		Short: "Capture a snapshot from a running sandbox",
		Long: `Capture a snapshot from a sandbox VM. If --checkpoint is specified, uses
that existing checkpoint (no VM interaction needed). Otherwise creates a
fresh checkpoint from the running VM's current state.

Examples:
  langsmith sandbox snapshot capture my-snap --box my-vm
  langsmith sandbox snapshot capture my-snap --box my-vm --checkpoint 2026-03-29T00:09:28Z`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			if boxName == "" {
				ExitError("--box is required")
			}

			c := MustGetClient()
			ctx := context.Background()

			params := langsmith.SandboxBoxNewSnapshotParams{
				Name: langsmith.F(name),
			}
			if checkpoint != "" {
				params.Checkpoint = langsmith.F(checkpoint)
			}

			resp, err := c.SDK.Sandboxes.Boxes.NewSnapshot(ctx, boxName, params)
			if err != nil {
				ExitErrorf("capturing snapshot: %v", err)
			}

			var result any = resp
			if wait {
				result = waitForSnapshot(ctx, c, resp.ID, time.Duration(timeoutSec)*time.Second)
			}

			output.OutputJSON(result, "")
		},
	}

	cmd.Flags().StringVar(&boxName, "box", "", "Sandbox name to capture from (required)")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint timestamp to use (omit for fresh checkpoint)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the snapshot to become ready")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "Timeout in seconds when using --wait")

	return cmd
}

func newSnapshotGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <snapshot-id>",
		Short: "Get a snapshot by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			resp, err := c.SDK.Sandboxes.Snapshots.Get(ctx, args[0])
			if err != nil {
				ExitErrorf("getting snapshot: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}
	return cmd
}

func newSnapshotDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <snapshot-id>",
		Short: "Delete a snapshot",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			if err := c.SDK.Sandboxes.Snapshots.Delete(ctx, args[0]); err != nil {
				ExitErrorf("deleting snapshot: %v", err)
			}

			fmt.Println("Snapshot deleted.")
		},
	}
	return cmd
}

func newSnapshotWaitCmd() *cobra.Command {
	var timeoutSec int

	cmd := &cobra.Command{
		Use:   "wait <snapshot-id>",
		Short: "Wait for a snapshot to become ready",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := MustGetClient()
			ctx := context.Background()

			resp := waitForSnapshot(ctx, c, args[0], time.Duration(timeoutSec)*time.Second)
			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "Timeout in seconds")

	return cmd
}

// waitForSnapshot polls until the snapshot is ready or fails.
func waitForSnapshot(ctx context.Context, c *client.Client, id string, timeout time.Duration) *langsmith.SandboxSnapshotGetResponse {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := c.SDK.Sandboxes.Snapshots.Get(ctx, id)
		if err != nil {
			ExitErrorf("getting snapshot: %v", err)
		}
		switch resp.Status {
		case "ready":
			return resp
		case "failed":
			msg := "unknown error"
			if resp.StatusMessage != "" {
				msg = resp.StatusMessage
			}
			ExitErrorf("snapshot build failed: %s", msg)
		}
		time.Sleep(2 * time.Second)
	}
	ExitErrorf("timed out after %s waiting for snapshot", timeout)
	return nil
}

func formatBytes(b int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%.0f MB", float64(b)/float64(mb))
}

// parseByteSize parses a human-readable size string into bytes.
// Accepts: "512mb", "2GB", "1g", "4GiB", "1024M", "1tb", etc.
// A bare number without a unit is rejected.
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	// Find where digits end and unit begins.
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, fmt.Errorf("invalid size %q: no number", s)
	}
	if i == len(s) {
		return 0, fmt.Errorf("invalid size %q: unit required (e.g. 512mb, 2gb)", s)
	}

	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}

	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	var multiplier float64
	switch unit {
	case "b":
		multiplier = 1
	case "k", "kb", "kib":
		multiplier = 1024
	case "m", "mb", "mib":
		multiplier = 1024 * 1024
	case "g", "gb", "gib":
		multiplier = 1024 * 1024 * 1024
	case "t", "tb", "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("invalid size %q: unknown unit %q", s, unit)
	}

	bytes := int64(num * multiplier)

	const minBytes = 1024 * 1024                      // 1 MB
	const maxBytes = 1024 * 1024 * 1024 * 1024 * 1024 // 1 PB
	if bytes < minBytes {
		return 0, fmt.Errorf("invalid size %q: must be at least 1mb", s)
	}
	if bytes > maxBytes {
		return 0, fmt.Errorf("invalid size %q: must be less than 1pb", s)
	}

	return bytes, nil
}

// loadJSONArg parses a JSON value from a CLI argument.
// If the string starts with "@", it reads the rest as a file path.
// Otherwise it parses it as inline JSON.
func loadJSONArg(s string) (json.RawMessage, error) {
	var data []byte
	if strings.HasPrefix(s, "@") {
		var err error
		data, err = os.ReadFile(s[1:])
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", s[1:], err)
		}
	} else {
		data = []byte(s)
	}
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return raw, nil
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}
