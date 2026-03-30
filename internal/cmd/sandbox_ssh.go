package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newSandboxSSHCopyIDCmd() *cobra.Command {
	var identity string

	cmd := &cobra.Command{
		Use:   "ssh-copy-id <name>",
		Short: "Upload your SSH public key to a sandbox and print an ssh-config block",
		Long: `Upload your SSH public key to a running sandbox so you can connect
with standard SSH tools (ssh, scp, rsync, sftp).

After running this command, add the printed config block to ~/.ssh/config
and connect with: ssh sandbox-<name>

Examples:
  langsmith sandbox ssh-copy-id my-sandbox
  langsmith sandbox ssh-copy-id my-sandbox --identity ~/.ssh/id_ed25519.pub`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHCopyID(args[0], identity)
		},
	}

	cmd.Flags().StringVar(&identity, "identity", "", "Path to SSH public key (default: auto-detect)")

	return cmd
}

func runSSHCopyID(name, identity string) error {
	ctx := context.Background()

	// Resolve sandbox.
	dpURL := os.Getenv("SANDBOX_DIRECT_URL")
	var tenantID string
	if dpURL == "" {
		apiKey := getAPIKey()
		if apiKey == "" {
			return fmt.Errorf("LANGSMITH_API_KEY not set")
		}

		c := mustGetClient()
		var box consoleBoxResponse
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

	// Upload to /root/.ssh/authorized_keys via the sandbox upload endpoint.
	if err := uploadAuthorizedKeys(dpURL, tenantID, pubkey); err != nil {
		return fmt.Errorf("uploading public key: %w", err)
	}

	// Fetch the host key from the sandbox.
	hostKey, err := fetchHostKey(dpURL, tenantID)
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

	configBlock := fmt.Sprintf(
		"# Added by: langsmith sandbox ssh-copy-id %s\nHost %s\n    User root\n    ProxyCommand langsmith sandbox tunnel --name %s --remote-port 22 --stdio\n    UserKnownHostsFile %s\n    ForwardAgent yes\n",
		name, hostAlias, name, knownHostsPath,
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

		// Replace the old block.
		newLines := append(lines[:start], strings.Split(block, "\n")...)
		newLines = append(newLines, lines[end:]...)
		if err := os.WriteFile(configPath, []byte(strings.Join(newLines, "\n")), 0600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Updated SSH config for %s in %s\n", hostAlias, configPath)
		return nil
	}

	// Not found — append.
	os.MkdirAll(filepath.Dir(configPath), 0700)
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		f.WriteString("\n")
	}
	if _, err := f.WriteString(block); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Wrote SSH config for %s to %s\n", hostAlias, configPath)
	return nil
}

// fetchHostKey retrieves the SSH host public key from the sandbox.
func fetchHostKey(dpURL, tenantID string) (string, error) {
	execURL := strings.TrimRight(dpURL, "/") + "/execute"
	body, _ := json.Marshal(map[string]interface{}{
		"command": []string{"cat", "/etc/ssh/ssh_host_ed25519_key.pub"},
	})

	req, err := http.NewRequest(http.MethodPost, execURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range sandboxAuthHeaders(tenantID) {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Stdout   string `json:"stdout"`
		ExitCode int    `json:"exit_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	os.MkdirAll(filepath.Dir(path), 0700)
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
// /root/.ssh/authorized_keys via the daemon's /upload endpoint.
func uploadAuthorizedKeys(dpURL, tenantID string, pubkey []byte) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "authorized_keys")
	if err != nil {
		return err
	}
	if _, err := part.Write(pubkey); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	uploadURL := strings.TrimRight(dpURL, "/") + "/upload?path=/root/.ssh/authorized_keys"
	req, err := http.NewRequest("POST", uploadURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	for k, v := range sandboxAuthHeaders(tenantID) {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
