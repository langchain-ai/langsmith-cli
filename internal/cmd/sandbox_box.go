package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

const defaultBoxPollInterval = 2 * time.Second

func waitForBoxReady(ctx context.Context, c *client.Client, name string) (*langsmith.SandboxBoxGetStatusResponse, error) {
	for {
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
		timer := time.NewTimer(defaultBoxPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

type sandboxCreateInput struct {
	SnapshotID  string
	VCPUs       int
	Memory      string
	RootFS      string
	ProxyConfig string
}

var sandboxBoxDetailRender = structured.PropertyList{
	Properties: []structured.Property{
		{Label: "Name", Template: "{{.Name}}"},
		{Label: "ID", Template: "{{.ID}}"},
		{Label: "Status", Template: "{{.Status}}"},
		{Label: "Size", Template: "{{.SizeClass}}"},
		{Label: "VCPUs", Template: "{{formatCount .Vcpus}}"},
		{Label: "Memory", Template: "{{formatBytesOrDash .MemBytes}}"},
		{Label: "Rootfs", Template: "{{formatBytesOrDash .FsCapacityBytes}}"},
		{Label: "Snapshot", Template: "{{shortID .SnapshotID}}"},
		{Label: "Idle TTL", Template: "{{formatCount .IdleTtlSeconds}}s"},
		{Label: "Created", Template: "{{formatTime .CreatedAt}}"},
	},
}

var sandboxCreateCommand = structured.Command[*sandboxCreateInput]{
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
  langsmith sandbox create my-vm --snapshot-id <id> --rootfs-capacity 8gb
  langsmith sandbox create my-vm --snapshot-id <id> --proxy-config @proxy.json`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *sandboxCreateInput {
		in := &sandboxCreateInput{
			VCPUs:  2,
			Memory: "512mb",
		}
		cmd.Flags().StringVar(&in.SnapshotID, "snapshot-id", in.SnapshotID, "Snapshot ID to boot from (required)")
		cmd.Flags().IntVar(&in.VCPUs, "vcpus", in.VCPUs, "Number of vCPUs")
		cmd.Flags().StringVar(&in.Memory, "memory", in.Memory, "Memory with unit (e.g. 512mb, 1gb)")
		cmd.Flags().StringVar(&in.RootFS, "rootfs-capacity", in.RootFS, "Root filesystem capacity with unit (e.g. 4gb, 8gb)")
		cmd.Flags().StringVar(&in.ProxyConfig, "proxy-config", in.ProxyConfig, "Proxy config as JSON or @file.json")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxCreateInput, args []string) (any, error) {
		name := args[0]
		if in.SnapshotID == "" {
			return nil, fmt.Errorf("--snapshot-id is required")
		}

		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		memBytes, err := parseByteSize(in.Memory)
		if err != nil {
			return nil, fmt.Errorf("invalid --memory: %w", err)
		}

		params := langsmith.SandboxBoxNewParams{
			Name:       langsmith.F(name),
			SnapshotID: langsmith.F(in.SnapshotID),
			Vcpus:      langsmith.F(int64(in.VCPUs)),
			MemBytes:   langsmith.F(memBytes),
		}
		if in.RootFS != "" {
			rootfsBytes, err := parseByteSize(in.RootFS)
			if err != nil {
				return nil, fmt.Errorf("invalid --rootfs-capacity: %w", err)
			}
			params.FsCapacityBytes = langsmith.F(rootfsBytes)
		}
		if in.ProxyConfig != "" {
			pc, err := loadJSONArg(in.ProxyConfig)
			if err != nil {
				return nil, fmt.Errorf("invalid --proxy-config: %w", err)
			}
			params.ProxyConfig = langsmith.Raw[langsmith.SandboxBoxNewParamsProxyConfig](pc)
		}
		resp, err := c.SDK.Sandboxes.Boxes.New(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("creating sandbox: %w", err)
		}

		return resp, nil
	},
	Render: sandboxBoxDetailRender,
}

var sandboxListCommand = structured.Command[struct{}]{
	Use:   "list",
	Short: "List all sandboxes",
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		resp, err := c.SDK.Sandboxes.Boxes.List(ctx, langsmith.SandboxBoxListParams{})
		if err != nil {
			return nil, fmt.Errorf("listing sandboxes: %w", err)
		}
		return resp.Sandboxes, nil
	},
	Render: structured.Table{
		Title: "Sandboxes",
		Rows:  ".",
		Columns: []structured.Column{
			{Header: "Name", Template: "{{.Name}}"},
			{Header: "Status", Template: "{{.Status}}"},
			{Header: "VCPUs", Template: "{{formatCount .Vcpus}}"},
			{Header: "Mem", Template: "{{formatBytesOrDash .MemBytes}}"},
			{Header: "Rootfs", Template: "{{formatBytesOrDash .FsCapacityBytes}}"},
			{Header: "Snapshot", Template: "{{shortID .SnapshotID}}"},
			{Header: "Created", Template: "{{formatTime .CreatedAt}}"},
		},
	},
}

var sandboxGetCommand = structured.Command[struct{}]{
	Use:   "get <name>",
	Short: "Get a sandbox by name",
	Args:  cobra.ExactArgs(1),
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		resp, err := c.SDK.Sandboxes.Boxes.Get(ctx, args[0])
		if err != nil {
			return nil, fmt.Errorf("getting sandbox: %w", err)
		}

		return resp, nil
	},
	Render: sandboxBoxDetailRender,
}

type sandboxUpdateInput struct {
	VCPUs       int
	Memory      string
	RootFS      string
	ProxyConfig string
}

var sandboxUpdateCommand = structured.Command[*sandboxUpdateInput]{
	Use:   "update <name>",
	Short: "Update sandbox resources (takes effect on next start)",
	Long: `Update sandbox resources or proxy configuration.

Resource changes (--vcpus, --memory, --rootfs-capacity) take effect on next start.
Proxy config changes take effect immediately.

The --proxy-config flag accepts inline JSON or @file.json. See "create --help"
for the proxy config JSON format.`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *sandboxUpdateInput {
		in := &sandboxUpdateInput{}
		cmd.Flags().IntVar(&in.VCPUs, "vcpus", in.VCPUs, "Number of vCPUs")
		cmd.Flags().StringVar(&in.Memory, "memory", in.Memory, "Memory with unit (e.g. 512mb, 1gb)")
		cmd.Flags().StringVar(&in.RootFS, "rootfs-capacity", in.RootFS, "Root filesystem capacity with unit (e.g. 4gb, 8gb)")
		cmd.Flags().StringVar(&in.ProxyConfig, "proxy-config", in.ProxyConfig, "Proxy config as JSON or @file.json")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxUpdateInput, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		params := langsmith.SandboxBoxUpdateParams{}
		if cmd.Flags().Changed("vcpus") {
			params.Vcpus = langsmith.F(int64(in.VCPUs))
		}
		if cmd.Flags().Changed("memory") {
			memBytes, err := parseByteSize(in.Memory)
			if err != nil {
				return nil, fmt.Errorf("invalid --memory: %w", err)
			}
			params.MemBytes = langsmith.F(memBytes)
		}
		if cmd.Flags().Changed("rootfs-capacity") {
			rootfsBytes, err := parseByteSize(in.RootFS)
			if err != nil {
				return nil, fmt.Errorf("invalid --rootfs-capacity: %w", err)
			}
			params.FsCapacityBytes = langsmith.F(rootfsBytes)
		}
		if cmd.Flags().Changed("proxy-config") {
			pc, err := loadJSONArg(in.ProxyConfig)
			if err != nil {
				return nil, fmt.Errorf("invalid --proxy-config: %w", err)
			}
			params.ProxyConfig = langsmith.Raw[langsmith.SandboxBoxUpdateParamsProxyConfig](pc)
		}

		if !cmd.Flags().Changed("vcpus") && !cmd.Flags().Changed("memory") && !cmd.Flags().Changed("rootfs-capacity") && !cmd.Flags().Changed("proxy-config") {
			return nil, fmt.Errorf("nothing to update (use --vcpus, --memory, --rootfs-capacity, or --proxy-config)")
		}

		resp, err := c.SDK.Sandboxes.Boxes.Update(ctx, args[0], params)
		if err != nil {
			return nil, fmt.Errorf("updating sandbox: %w", err)
		}

		return resp, nil
	},
	Render: sandboxBoxDetailRender,
}

var sandboxDeleteCommand = structured.Command[struct{}]{
	Use:   "delete <name>",
	Short: "Delete a sandbox",
	Args:  cobra.ExactArgs(1),
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		if err := c.SDK.Sandboxes.Boxes.Delete(ctx, args[0]); err != nil {
			return nil, fmt.Errorf("deleting sandbox: %w", err)
		}

		return sandboxMessage{Name: args[0], Message: "Sandbox deleted."}, nil
	},
	Render: structured.Template(`{{.Message}}
`),
}

var sandboxStartCommand = structured.Command[struct{}]{
	Use:   "start <name>",
	Short: "Start a stopped sandbox",
	Args:  cobra.ExactArgs(1),
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		if _, err := c.SDK.Sandboxes.Boxes.Start(ctx, args[0]); err != nil {
			return nil, fmt.Errorf("starting sandbox: %w", err)
		}

		if _, err := waitForBoxReady(ctx, c, args[0]); err != nil {
			return nil, err
		}
		return sandboxMessage{Name: args[0], Message: fmt.Sprintf("Sandbox %s started", args[0])}, nil
	},
	Render: structured.Template(`{{.Message}}
`),
}

var sandboxStopCommand = structured.Command[struct{}]{
	Use:   "stop <name>",
	Short: "Stop a running sandbox (preserves data)",
	Args:  cobra.ExactArgs(1),
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		if err := c.SDK.Sandboxes.Boxes.Stop(ctx, args[0]); err != nil {
			return nil, fmt.Errorf("stopping sandbox: %w", err)
		}

		return sandboxMessage{Name: args[0], Message: "Sandbox stopped."}, nil
	},
	Render: structured.Template(`{{.Message}}
`),
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

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			c, err := cmdutil.GetClient(cmd)
			if err != nil {
				return err
			}

			result, err := c.SDK.Sandboxes.Boxes.Run(ctx, name, langsmith.SandboxBoxRunParams{
				Command: langsmith.F(sandboxShellCommand(command)),
			})
			if err != nil {
				return fmt.Errorf("execute: %w", err)
			}

			if result.Stdout != "" {
				fmt.Print(result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Fprint(os.Stderr, result.Stderr)
			}
			if result.ExitCode != 0 {
				os.Exit(int(result.ExitCode))
			}
			return nil
		},
	}
	return cmd
}

func sandboxShellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}
