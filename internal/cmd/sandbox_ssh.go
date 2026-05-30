package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/cmdutil"
	"github.com/langchain-ai/langsmith-cli/internal/structured"
	"github.com/langchain-ai/langsmith-go"
	"github.com/spf13/cobra"
)

type sandboxSSHSetupInput struct {
	Identity string
}

var sandboxSSHSetupCommand = structured.Command[*sandboxSSHSetupInput]{
	Use:   "ssh-setup [name]",
	Short: "Upload your SSH public key and configure ~/.ssh/config for a sandbox",
	Long: `Upload your SSH public key to a running sandbox so you can connect
with standard SSH tools (ssh, scp, rsync, sftp).

This command uploads your key, fetches the host key, writes a
~/.ssh/config block, and configures known_hosts — so you can
immediately connect with: ssh sandbox-<name>

Examples:
  langsmith sandbox ssh-setup my-sandbox
  langsmith sandbox ssh-setup my-sandbox --identity ~/.ssh/id_ed25519.pub`,
	Args: cobra.MaximumNArgs(1),
	Input: func(cmd *cobra.Command) *sandboxSSHSetupInput {
		in := &sandboxSSHSetupInput{}
		cmd.Flags().StringVar(&in.Identity, "identity", in.Identity, "Path to SSH public key (default: auto-detect)")
		return in
	},
	CustomOutput: true,
	Action: func(ctx context.Context, cmd *cobra.Command, in *sandboxSSHSetupInput, args []string) (any, error) {
		name, err := resolveSandboxName(cmd, args)
		if err != nil {
			return nil, err
		}
		return nil, runSSHSetup(cmd, name, in.Identity)
	},
}

func newSandboxSSHSetupCmd() *cobra.Command {
	return sandboxSSHSetupCommand.Cobra()
}

func runSSHSetup(cmd *cobra.Command, name, identity string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := cmdutil.GetClient(cmd)
	if err != nil {
		return err
	}

	// Find the public key.
	pubkeyPath, err := resolvePublicKey(identity)
	if err != nil {
		return err
	}
	pubkey, err := os.ReadFile(pubkeyPath)
	if err != nil {
		return fmt.Errorf("reading public key %s: %w", pubkeyPath, err)
	}
	pubkey = bytes.TrimSpace(pubkey)

	// Check if sshd is running inside the sandbox.
	if !isSSHDRunning(ctx, c, name) {
		fmt.Fprintln(os.Stderr, "Warning: sshd does not appear to be running in the sandbox. SSH connections will likely not work.")
	}

	// Upload to /root/.ssh/authorized_keys via the sandbox upload endpoint.
	if err := uploadAuthorizedKeys(ctx, c, name, pubkey); err != nil {
		return fmt.Errorf("uploading public key: %w", err)
	}

	// Fetch the host key from the sandbox.
	hostKey, err := fetchHostKey(ctx, c, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch host key: %v\n", err)
	}

	hostAlias := "sandbox-" + name

	// Write host key to a dedicated known_hosts file for sandboxes.
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts_sandboxes")
	if hostKey != "" {
		if err := ensureKnownHost(knownHostsPath, hostAlias, hostKey); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not write known host: %v\n", err)
		}
	}

	proxyCmd := fmt.Sprintf("langsmith sandbox tunnel %s --remote-port 22 --stdio", shellQuote(name))
	if apiKey := GetAPIKey(); apiKey != "" {
		proxyCmd += " --api-key " + shellQuote(apiKey)
	}
	if apiURL := GetAPIURL(); apiURL != "https://api.smith.langchain.com" {
		proxyCmd += " --api-url " + shellQuote(apiURL)
	}

	configBlock := fmt.Sprintf(
		"# Added by: langsmith sandbox ssh-setup %s\nHost %s\n    User root\n    ProxyCommand %s\n    UserKnownHostsFile %s\n    ForwardAgent yes\n",
		name, hostAlias, proxyCmd, shellQuote(knownHostsPath),
	)

	if err := ensureSSHConfig(hostAlias, configBlock); err != nil {
		return fmt.Errorf("updating ssh config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Key uploaded (%s). Connect with:\n  ssh %s\n", pubkeyPath, hostAlias)

	return nil
}

// ensureSSHConfig writes a config block to ~/.ssh/config.
// If the host alias already exists with identical content, it's left untouched.
// If it exists with different content, it's replaced.
func ensureSSHConfig(hostAlias, block string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".ssh", "config")

	existing, _ := os.ReadFile(configPath)
	content := string(existing)

	// Find and replace existing Host block if present.
	hostLine := "Host " + hostAlias
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != hostLine {
			continue
		}
		// Found existing block. Find its extent (until next Host line or EOF).
		// Include the comment line above if it matches our format.
		start := i
		if start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "# Added by:") {
			start--
		}
		end := i + 1
		for end < len(lines) {
			if strings.HasPrefix(strings.TrimSpace(lines[end]), "Host ") ||
				strings.HasPrefix(strings.TrimSpace(lines[end]), "# Added by:") {
				break
			}
			end++
		}

		oldBlock := strings.Join(lines[start:end], "\n")
		if strings.TrimSpace(oldBlock) == strings.TrimSpace(block) {
			fmt.Fprintf(os.Stderr, "SSH config for %s already exists in %s\n", hostAlias, configPath)
			return nil
		}

		// Replace the old block. Use a fresh slice to avoid corrupting
		// the shared backing array of lines.
		newLines := make([]string, 0, len(lines[:start])+len(strings.Split(block, "\n"))+len(lines[end:]))
		newLines = append(newLines, lines[:start]...)
		newLines = append(newLines, strings.Split(block, "\n")...)
		newLines = append(newLines, lines[end:]...)
		if err := os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Updated SSH config for %s in %s\n", hostAlias, configPath)
		return nil
	}

	// Not found — append.
	_ = os.MkdirAll(filepath.Dir(configPath), 0700)
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		_, _ = f.WriteString("\n")
	}
	if _, err := f.WriteString(block); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Wrote SSH config for %s to %s\n", hostAlias, configPath)
	return nil
}

// fetchHostKey retrieves the SSH host public key from the sandbox.
func fetchHostKey(ctx context.Context, c *client.Client, name string) (string, error) {
	result, err := sandboxExec(ctx, c, name, []string{"cat", "/etc/ssh/ssh_host_ed25519_key.pub"})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("exit code %d", result.ExitCode)
	}
	return strings.TrimSpace(result.Stdout), nil
}

// ensureKnownHost writes or updates a host key in a known_hosts file.
func ensureKnownHost(path, hostAlias, hostKey string) error {
	// hostKey is like "ssh-ed25519 AAAA..."
	entry := hostAlias + " " + hostKey + "\n"

	existing, _ := os.ReadFile(path)
	lines := strings.Split(string(existing), "\n")

	// Replace existing entry for this alias.
	for i, line := range lines {
		if strings.HasPrefix(line, hostAlias+" ") {
			lines[i] = strings.TrimSuffix(entry, "\n")
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
		}
	}

	// Append.
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// resolvePublicKey finds the SSH public key to use.
func resolvePublicKey(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("public key not found: %s", explicit)
		}
		return explicit, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	candidates := []string{
		filepath.Join(home, ".ssh", "id_ed25519.pub"),
		filepath.Join(home, ".ssh", "id_rsa.pub"),
		filepath.Join(home, ".ssh", "id_ecdsa.pub"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("no SSH public key found (tried %s); use --identity to specify one",
		strings.Join(candidates, ", "))
}

// uploadAuthorizedKeys uploads the public key to the sandbox's
// /root/.ssh/authorized_keys via the SDK runtime file helper.
func uploadAuthorizedKeys(ctx context.Context, c *client.Client, name string, pubkey []byte) error {
	return c.SDK.Sandboxes.Boxes.WriteFile(ctx, name, "/root/.ssh/authorized_keys", pubkey)
}

// isSSHDRunning checks whether sshd is listening on port 22 inside the sandbox.
func isSSHDRunning(ctx context.Context, c *client.Client, name string) bool {
	result, err := sandboxExec(ctx, c, name, []string{"sh", "-c", "ss -tlnp 2>/dev/null | grep -q ':22 ' || netstat -tlnp 2>/dev/null | grep -q ':22 '"})
	if err != nil {
		return false
	}
	return result.ExitCode == 0
}

type execResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// sandboxExec runs a command inside the sandbox via the /execute endpoint.
func sandboxExec(ctx context.Context, c *client.Client, name string, command []string) (*execResult, error) {
	result, err := c.SDK.Sandboxes.Boxes.Run(ctx, name, langsmith.SandboxBoxRunParams{
		Command: langsmith.F(sandboxShellCommand(command)),
	})
	if err != nil {
		return nil, err
	}
	return &execResult{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: int(result.ExitCode),
	}, nil
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
// Safe for interpolation into shell command strings (e.g. SSH ProxyCommand).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
