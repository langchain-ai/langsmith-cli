package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

const defaultBoxPollInterval = 2 * time.Second

func waitForBoxReady(ctx context.Context, c *client.Client, name string) (*langsmith.SandboxStatusResponse, error) {
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
	Snapshot    string
	SnapshotID  string
	VCPUs       int
	Memory      string
	RootFS      string
	ProxyConfig string

	Console         bool
	Shell           string
	ForwardSSHAgent bool
	Env             []string
}

type sandboxServiceURLInput struct {
	Port             int
	ExpiresInSeconds int64
}

type sandboxDownloadURLInput struct {
	Path               string
	ExpiresInSeconds   int64
	ContentType        string
	ContentDisposition string
}

var sandboxBoxDetailRender = structured.PropertyList{
	Properties: []structured.Property{
		{Label: "Name", Template: "{{.Name}}"},
		{Label: "ID", Template: "{{.ID}}"},
		{Label: "Status", Template: "{{.Status}}"},
		{Label: "Size", Template: "{{.SizeClass}}"},
		{Label: "vCPU", Template: "{{formatCount .Vcpus}}"},
		{Label: "Memory", Template: "{{formatBytesOrDash .MemBytes}}"},
		{Label: "Rootfs", Template: "{{formatBytesOrDash .FsCapacityBytes}}"},
		{Label: "Snapshot", Template: "{{shortID .SnapshotID}}"},
		{Label: "Idle TTL", Template: "{{formatCount .IdleTtlSeconds}}s"},
		{Label: "Created", Template: "{{formatTime .CreatedAt}}"},
	},
}

func sandboxCreateParams(name string, in *sandboxCreateInput) (langsmith.SandboxBoxNewParams, error) {
	params := langsmith.SandboxBoxNewParams{}
	if in.VCPUs != 0 {
		params.Vcpus = langsmith.F(int64(in.VCPUs))
	}
	if in.Memory != "" {
		memBytes, err := parseByteSize(in.Memory)
		if err != nil {
			return langsmith.SandboxBoxNewParams{}, fmt.Errorf("invalid --memory: %w", err)
		}
		params.MemBytes = langsmith.F(memBytes)
	}
	if name != "" {
		params.Name = langsmith.F(name)
	}
	if in.SnapshotID != "" {
		params.SnapshotID = langsmith.F(in.SnapshotID)
	}
	if in.Snapshot != "" {
		// The server takes snapshot_id (UUID) and snapshot_name (name or name:tag) as distinct fields.
		if _, err := uuid.Parse(in.Snapshot); err == nil {
			params.SnapshotID = langsmith.F(in.Snapshot)
		} else {
			params.SnapshotName = langsmith.F(in.Snapshot)
		}
	}
	if in.RootFS != "" {
		rootfsBytes, err := parseByteSize(in.RootFS)
		if err != nil {
			return langsmith.SandboxBoxNewParams{}, fmt.Errorf("invalid --rootfs-capacity: %w", err)
		}
		params.FsCapacityBytes = langsmith.F(rootfsBytes)
	}
	if in.ProxyConfig != "" {
		pc, err := loadJSONArg(in.ProxyConfig)
		if err != nil {
			return langsmith.SandboxBoxNewParams{}, fmt.Errorf("invalid --proxy-config: %w", err)
		}
		params.ProxyConfig = langsmith.Raw[langsmith.SandboxBoxNewParamsProxyConfig](pc)
	}
	return params, nil
}

var sandboxCreateCommand = structured.Command[*sandboxCreateInput]{
	Use:   "create [name]",
	Short: "Create a sandbox VM",
	Long: `Create a sandbox VM.

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
      "env_vars": {"OPENAI_API_KEY": "proxy-injected"},
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

A rule's "env_vars" are plaintext variables set for every command in the sandbox
while the rule is enabled, for tools that refuse to run without a credential
variable even though the proxy injects the real credential on the wire.

Examples:
  langsmith sandbox create
  langsmith sandbox create my-vm
  langsmith sandbox create my-vm --snapshot <id-or-name>
  langsmith sandbox create my-vm --snapshot-id <id>
  langsmith sandbox create my-vm --snapshot-id <id> --rootfs-capacity 8gb
  langsmith sandbox create my-vm --snapshot-id <id> --proxy-config @proxy.json
  langsmith sandbox create my-vm --snapshot-id <id> --console`,
	Args: cobra.MaximumNArgs(1),
	Input: func(cmd *cobra.Command) *sandboxCreateInput {
		in := &sandboxCreateInput{}
		cmd.Flags().StringVar(&in.Snapshot, "snapshot", in.Snapshot, "Snapshot ID or name to boot from")
		cmd.Flags().StringVar(&in.SnapshotID, "snapshot-id", in.SnapshotID, "Snapshot ID to boot from")
		cmd.Flags().IntVar(&in.VCPUs, "vcpus", in.VCPUs, "Number of vCPU cores")
		cmd.Flags().StringVar(&in.Memory, "memory", in.Memory, "Memory with unit (e.g. 4gb, 8gb); must be within 50% of 4gb per vCPU")
		cmd.Flags().StringVar(&in.RootFS, "rootfs-capacity", in.RootFS, "Root filesystem capacity with unit (e.g. 4gb, 8gb)")
		cmd.Flags().StringVar(&in.ProxyConfig, "proxy-config", in.ProxyConfig, "Proxy config as JSON or @file.json")
		cmd.Flags().BoolVar(&in.Console, "console", in.Console, "Open an interactive console once the sandbox is ready")
		cmd.Flags().StringVar(&in.Shell, "shell", in.Shell, "Shell to use for --console (default: sandbox default, usually /bin/bash)")
		cmd.Flags().BoolVar(&in.ForwardSSHAgent, "forward-ssh-agent", in.ForwardSSHAgent, "Forward the local SSH agent (SSH_AUTH_SOCK) into the --console session")
		cmd.Flags().StringArrayVar(&in.Env, "env", nil, "Additional environment variable for the --console session (KEY or KEY=VALUE, repeatable)")
		cmd.Flags().String("jq", "", "Filter JSON output using a jq expression")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxCreateInput, args []string) (any, error) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		if in.Snapshot != "" && in.SnapshotID != "" {
			return nil, fmt.Errorf("use either --snapshot or --snapshot-id, not both")
		}

		params, err := sandboxCreateParams(name, in)
		if err != nil {
			return nil, err
		}
		resp, err := c.SDK.Sandboxes.Boxes.New(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("creating sandbox: %w", err)
		}

		if !in.Console {
			return nil, structured.Render(cmd, resp, sandboxBoxDetailRender)
		}

		if _, err := waitForBoxReady(ctx, c, resp.Name); err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "Sandbox %s ready, opening console...\n", resp.Name)
		return nil, runConsole(resp.Name, in.Shell, in.ForwardSSHAgent, in.Env)
	},
}

var sandboxServiceURLRender = structured.PropertyList{
	Properties: []structured.Property{
		{Label: "Browser URL", Template: "{{.BrowserURL}}"},
		{Label: "Service URL", Template: "{{.ServiceURL}}"},
		{Label: "Expires", Template: "{{formatTime .ExpiresAt}}"},
		{Label: "Service Token", Template: "{{.Token}}"},
	},
	Caption: `Example:
  curl -H "X-Langsmith-Sandbox-Service-Token: {{.Token}}" "{{.ServiceURL}}"`,
}

var sandboxServiceURLCommand = structured.Command[*sandboxServiceURLInput]{
	Use:   "service-url <name> --port <port>",
	Short: "Generate an authenticated URL for a sandbox HTTP service",
	Long: `Generate an authenticated URL for an HTTP service running inside a sandbox.

Use the service URL with the X-Langsmith-Sandbox-Service-Token header, or open
the browser URL directly.

Examples:
  langsmith sandbox service-url my-vm --port 8000
  langsmith sandbox service-url my-vm --port 8000 --expires-in-seconds 3600`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *sandboxServiceURLInput {
		in := &sandboxServiceURLInput{}
		cmd.Flags().IntVar(&in.Port, "port", in.Port, "Port inside the sandbox")
		cmd.Flags().Int64Var(&in.ExpiresInSeconds, "expires-in-seconds", in.ExpiresInSeconds, "URL TTL in seconds")
		_ = cmd.MarkFlagRequired("port")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxServiceURLInput, args []string) (any, error) {
		if in.Port < 1 || in.Port > 65535 {
			return nil, fmt.Errorf("--port must be between 1 and 65535 (got %d)", in.Port)
		}
		if cmd.Flags().Changed("expires-in-seconds") && in.ExpiresInSeconds < 1 {
			return nil, fmt.Errorf("--expires-in-seconds must be greater than 0")
		}

		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		params := langsmith.SandboxBoxGenerateServiceURLParams{
			Port: langsmith.F(int64(in.Port)),
		}
		if cmd.Flags().Changed("expires-in-seconds") {
			params.ExpiresInSeconds = langsmith.F(in.ExpiresInSeconds)
		}

		resp, err := c.SDK.Sandboxes.Boxes.GenerateServiceURL(ctx, args[0], params)
		if err != nil {
			return nil, fmt.Errorf("generating service URL: %w", err)
		}
		return resp, nil
	},
	Render: sandboxServiceURLRender,
}

var sandboxDownloadURLRender = structured.PropertyList{
	Properties: []structured.Property{
		{Label: "Download URL", Template: "{{.DownloadURL}}"},
		{Label: "Expires", Template: "{{if .ExpiresAt}}{{formatTime .ExpiresAt}}{{else}}never{{end}}"},
	},
	Caption: `Example:
  curl -LO "{{.DownloadURL}}"`,
}

var sandboxDownloadURLCommand = structured.Command[*sandboxDownloadURLInput]{
	Use:   "generate-download-url <name> --path <path>",
	Short: "Generate a link that downloads a sandbox file without credentials",
	Long: `Generate a link that downloads a single file from a sandbox.

The link carries its own token, so anyone with the URL can fetch that one file
with no LangSmith credential. It is pinned to the sandbox and the exact path,
and cannot be repointed at another file. Fetching wakes a stopped sandbox.

Do not modify the file after minting a link for it. The link is pinned to a
path, not to a snapshot of the contents, so a later write to that path may or
may not be reflected in what the link serves. Write a new file and mint a new
link when the contents change.

Without --expires-in-seconds the link never expires.

Examples:
  langsmith sandbox generate-download-url my-vm --path /tmp/report.pdf
  langsmith sandbox generate-download-url my-vm --path /tmp/report.pdf --expires-in-seconds 3600
  langsmith sandbox generate-download-url my-vm --path /tmp/page.html --content-disposition inline`,
	Args: cobra.ExactArgs(1),
	Input: func(cmd *cobra.Command) *sandboxDownloadURLInput {
		in := &sandboxDownloadURLInput{}
		cmd.Flags().StringVar(&in.Path, "path", in.Path, "File path inside the sandbox")
		cmd.Flags().Int64Var(&in.ExpiresInSeconds, "expires-in-seconds", in.ExpiresInSeconds, "Link TTL in seconds (omit for a link that never expires)")
		cmd.Flags().StringVar(&in.ContentType, "content-type", in.ContentType, "Content-Type to serve the file as")
		cmd.Flags().StringVar(&in.ContentDisposition, "content-disposition", in.ContentDisposition, "Content-Disposition to serve the file with (attachment or inline)")
		_ = cmd.MarkFlagRequired("path")
		return in
	},
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxDownloadURLInput, args []string) (any, error) {
		if cmd.Flags().Changed("expires-in-seconds") && in.ExpiresInSeconds < 1 {
			return nil, fmt.Errorf("--expires-in-seconds must be greater than 0")
		}
		switch in.ContentDisposition {
		case "", "attachment", "inline":
		default:
			return nil, fmt.Errorf("--content-disposition must be attachment or inline (got %q)", in.ContentDisposition)
		}

		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		params := langsmith.SandboxBoxGenerateDownloadURLParams{
			Path: langsmith.F(in.Path),
		}
		if cmd.Flags().Changed("expires-in-seconds") {
			params.ExpiresInSeconds = langsmith.F(in.ExpiresInSeconds)
		}
		if in.ContentType != "" {
			params.ContentType = langsmith.F(in.ContentType)
		}
		if in.ContentDisposition != "" {
			params.ContentDisposition = langsmith.F(in.ContentDisposition)
		}

		resp, err := c.SDK.Sandboxes.Boxes.GenerateDownloadURL(ctx, args[0], params)
		if err != nil {
			return nil, fmt.Errorf("generating download URL: %w", err)
		}
		return resp, nil
	},
	Render: sandboxDownloadURLRender,
}

var sandboxListCommand = structured.Command[struct{}]{
	Use:   "list",
	Short: "List all sandboxes",
	Action: func(ctx context.Context, cmd *cobra.Command, in struct{}, args []string) (any, error) {
		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		var sandboxes []langsmith.SandboxResponse
		pager := c.SDK.Sandboxes.Boxes.ListAutoPaging(ctx, langsmith.SandboxBoxListParams{})
		for pager.Next() {
			sandboxes = append(sandboxes, pager.Current())
		}
		if err := pager.Err(); err != nil {
			return nil, fmt.Errorf("listing sandboxes: %w", err)
		}
		return sandboxes, nil
	},
	Render: structured.Table{
		Title: "Sandboxes",
		Rows:  ".",
		Columns: []structured.Column{
			{Header: "Name", Template: "{{.Name}}"},
			{Header: "Status", Template: "{{.Status}}"},
			{Header: "vCPU", Template: "{{formatCount .Vcpus}}"},
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
		cmd.Flags().IntVar(&in.VCPUs, "vcpus", in.VCPUs, "Number of vCPU cores")
		cmd.Flags().StringVar(&in.Memory, "memory", in.Memory, "Memory with unit (e.g. 4gb, 8gb); must be within 50% of 4gb per vCPU")
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

type sandboxExecInput struct{}

var sandboxExecCommand = structured.Command[sandboxExecInput]{
	Use:   "exec <name> -- <command>",
	Short: "Execute a command inside a sandbox",
	Long: `Execute a one-off command inside a running sandbox and print its output.

Examples:
  langsmith sandbox exec my-vm -- uname -a
  langsmith sandbox exec my-vm -- ls -la /
  langsmith sandbox exec my-vm -- cat /etc/os-release`,
	Args:         cobra.MinimumNArgs(1),
	Input:        func(cmd *cobra.Command) sandboxExecInput { return sandboxExecInput{} },
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in sandboxExecInput, args []string) (any, error) {
		name := args[0]

		cmdArgs := cmd.ArgsLenAtDash()
		if cmdArgs < 0 || cmdArgs >= len(args) {
			return nil, fmt.Errorf("usage: langsmith sandbox exec <name> -- <command>")
		}
		command := args[cmdArgs:]
		if len(command) == 0 {
			return nil, fmt.Errorf("no command specified")
		}

		c, err := cmdutil.GetClient(cmd)
		if err != nil {
			return nil, err
		}

		result, err := c.SDK.Sandboxes.Boxes.Run(ctx, name, langsmith.SandboxBoxRunParams{
			Command: langsmith.F(sandboxShellCommand(command)),
		})
		if err != nil {
			return nil, fmt.Errorf("execute: %w", err)
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
		return nil, nil
	},
}

func newSandboxExecCmd() *cobra.Command {
	return sandboxExecCommand.Cobra()
}

func sandboxShellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}
