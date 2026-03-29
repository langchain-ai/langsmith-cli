package cmd

import (
	"bytes"
	"context"
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

	// Build the sandbox URL for ProxyCommand.
	sandboxURL := dpURL
	hostAlias := "sandbox-" + name

	configBlock := fmt.Sprintf(
		"# Added by: langsmith sandbox ssh-copy-id %s\nHost %s\n    User root\n    ProxyCommand langsmith sandbox tunnel --url %s --remote-port 22 --stdio\n    StrictHostKeyChecking no\n    ForwardAgent yes\n",
		name, hostAlias, sandboxURL,
	)

	// Write to ~/.ssh/config if this host isn't already configured.
	if err := ensureSSHConfig(hostAlias, configBlock); err != nil {
		return fmt.Errorf("updating ssh config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Key uploaded (%s). Connect with:\n  ssh %s\n", pubkeyPath, hostAlias)

	return nil
}

// ensureSSHConfig appends a config block to ~/.ssh/config if the host
// alias is not already present. Existing entries are left untouched.
func ensureSSHConfig(hostAlias, block string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".ssh", "config")

	existing, _ := os.ReadFile(configPath)
	// Check if "Host <alias>" already exists.
	for _, line := range strings.Split(string(existing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Host "+hostAlias {
			fmt.Fprintf(os.Stderr, "SSH config for %s already exists in %s\n", hostAlias, configPath)
			return nil
		}
	}

	os.MkdirAll(filepath.Dir(configPath), 0700)
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add a blank line separator if file is non-empty and doesn't end with newline.
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		f.WriteString("\n")
	}
	_, err = f.WriteString(block)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Wrote SSH config for %s to %s\n", hostAlias, configPath)
	return nil
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
