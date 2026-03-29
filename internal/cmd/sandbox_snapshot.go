package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// Snapshot API types (v2/sandboxes/snapshots).

type snapshotResponse struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	DockerImage     *string `json:"docker_image,omitempty"`
	ImageDigest     *string `json:"image_digest,omitempty"`
	SourceSandboxID *string `json:"source_sandbox_id,omitempty"`
	Status          string  `json:"status"`
	StatusMessage   *string `json:"status_message,omitempty"`
	FsSizeBytes     int64   `json:"fs_size_bytes"`
	SizeBytes       *int64  `json:"size_bytes,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type snapshotListResponse struct {
	Snapshots []snapshotResponse `json:"snapshots"`
	Offset    int                `json:"offset"`
}

func newSandboxSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Manage sandbox snapshots (ext4 rootfs images)",
		Long: `Manage sandbox snapshots — ext4 rootfs images built from Docker images
or captured from running sandboxes. Snapshots are used to create new sandboxes.

Examples:
  langsmith sandbox snapshot list
  langsmith sandbox snapshot create --name my-snap --docker-image ubuntu:24.04
  langsmith sandbox snapshot get <id>
  langsmith sandbox snapshot delete <id>
  langsmith sandbox snapshot wait <id>`,
	}

	cmd.AddCommand(newSnapshotListCmd())
	cmd.AddCommand(newSnapshotCreateCmd())
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
			c := mustGetClient()
			ctx := context.Background()

			var resp snapshotListResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/snapshots", &resp); err != nil {
				exitErrorf("listing snapshots: %v", err)
			}

			if len(resp.Snapshots) == 0 {
				output.OutputJSON(resp.Snapshots, "")
				return
			}

			columns := []string{"ID", "Name", "Image", "Status", "Size", "Created"}
			var rows [][]string
			for _, s := range resp.Snapshots {
				image := "-"
				if s.DockerImage != nil {
					image = *s.DockerImage
				}
				size := "-"
				if s.SizeBytes != nil {
					size = formatBytes(*s.SizeBytes)
				}
				rows = append(rows, []string{
					s.ID, s.Name, image, s.Status, size, formatTime(s.CreatedAt),
				})
			}
			output.OutputTable(columns, rows, "Snapshots")
		},
	}
	return cmd
}

func newSnapshotCreateCmd() *cobra.Command {
	var (
		name        string
		dockerImage string
		fsSizeGB    int
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a snapshot from a Docker image",
		Run: func(cmd *cobra.Command, args []string) {
			if name == "" {
				exitError("--name is required")
			}
			if dockerImage == "" {
				exitError("--docker-image is required")
			}

			c := mustGetClient()
			ctx := context.Background()

			body := map[string]any{
				"name":          name,
				"docker_image":  dockerImage,
				"fs_size_bytes": int64(fsSizeGB) * 1024 * 1024 * 1024,
			}

			var resp snapshotResponse
			if err := c.RawPost(ctx, "/v2/sandboxes/snapshots", body, &resp); err != nil {
				exitErrorf("creating snapshot: %v", err)
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Snapshot name (required)")
	cmd.Flags().StringVar(&dockerImage, "docker-image", "", "Docker image to build from (required)")
	cmd.Flags().IntVar(&fsSizeGB, "fs-size-gb", 4, "Filesystem size in GB (default: 4)")

	return cmd
}

func newSnapshotCaptureCmd() *cobra.Command {
	var (
		name       string
		boxName    string
		checkpoint string
		fsSizeGB   int
		wait       bool
	)

	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Capture a snapshot from a sandbox",
		Long: `Capture a snapshot from a sandbox VM. If --checkpoint is specified, uses
that existing checkpoint (no VM interaction needed). Otherwise creates a
fresh checkpoint from the running VM's current state.

Examples:
  langsmith sandbox snapshot capture --name my-snap --box my-vm
  langsmith sandbox snapshot capture --name my-snap --box my-vm --checkpoint 2026-03-29T00:09:28Z`,
		Run: func(cmd *cobra.Command, args []string) {
			if name == "" {
				exitError("--name is required")
			}
			if boxName == "" {
				exitError("--box is required")
			}

			c := mustGetClient()
			ctx := context.Background()

			// Resolve sandbox name to ID.
			var box boxResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+boxName, &box); err != nil {
				exitErrorf("getting sandbox %q: %v", boxName, err)
			}

			body := map[string]any{
				"name":              name,
				"source_sandbox_id": box.ID,
				"fs_size_bytes":     int64(fsSizeGB) * 1024 * 1024 * 1024,
			}
			if checkpoint != "" {
				body["checkpoint"] = checkpoint
			}

			var resp snapshotResponse
			if err := c.RawPost(ctx, "/v2/sandboxes/snapshots", body, &resp); err != nil {
				exitErrorf("capturing snapshot: %v", err)
			}

			if wait {
				deadline := time.Now().Add(60 * time.Second)
				for time.Now().Before(deadline) {
					if err := c.RawGet(ctx, "/v2/sandboxes/snapshots/"+resp.ID, &resp); err != nil {
						exitErrorf("getting snapshot: %v", err)
					}
					if resp.Status == "ready" {
						break
					}
					if resp.Status == "failed" {
						msg := "unknown error"
						if resp.StatusMessage != nil {
							msg = *resp.StatusMessage
						}
						exitErrorf("capture failed: %s", msg)
					}
					time.Sleep(time.Second)
				}
			}

			output.OutputJSON(resp, "")
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Snapshot name (required)")
	cmd.Flags().StringVar(&boxName, "box", "", "Sandbox name to capture from (required)")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Checkpoint timestamp to use (omit for fresh checkpoint)")
	cmd.Flags().IntVar(&fsSizeGB, "fs-size-gb", 4, "Filesystem size in GB (default: 4)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the snapshot to become ready")

	return cmd
}

func newSnapshotGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <snapshot-id>",
		Short: "Get a snapshot by ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			c := mustGetClient()
			ctx := context.Background()

			var resp snapshotResponse
			if err := c.RawGet(ctx, "/v2/sandboxes/snapshots/"+args[0], &resp); err != nil {
				exitErrorf("getting snapshot: %v", err)
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
			c := mustGetClient()
			ctx := context.Background()

			if err := c.RawDelete(ctx, "/v2/sandboxes/snapshots/"+args[0], nil); err != nil {
				exitErrorf("deleting snapshot: %v", err)
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
			c := mustGetClient()
			ctx := context.Background()
			deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

			for time.Now().Before(deadline) {
				var resp snapshotResponse
				if err := c.RawGet(ctx, "/v2/sandboxes/snapshots/"+args[0], &resp); err != nil {
					exitErrorf("getting snapshot: %v", err)
				}

				switch resp.Status {
				case "ready":
					output.OutputJSON(resp, "")
					return
				case "failed":
					msg := "unknown error"
					if resp.StatusMessage != nil {
						msg = *resp.StatusMessage
					}
					exitErrorf("snapshot build failed: %s", msg)
				}

				time.Sleep(2 * time.Second)
			}

			exitErrorf("timed out after %ds waiting for snapshot to become ready", timeoutSec)
		},
	}

	cmd.Flags().IntVar(&timeoutSec, "timeout", 300, "Timeout in seconds (default: 300)")

	return cmd
}

func formatBytes(b int64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%.0f MB", float64(b)/float64(mb))
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04")
}
