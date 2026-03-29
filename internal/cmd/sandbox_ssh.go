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

	// Print ssh-config block to stdout.
	fmt.Printf("# Added by: langsmith sandbox ssh-copy-id %s\n", name)
	fmt.Printf("Host sandbox-%s\n", name)
	fmt.Printf("    User root\n")
	fmt.Printf("    ProxyCommand langsmith sandbox tunnel --url %s --remote-port 22 --stdio\n", sandboxURL)
	fmt.Printf("    StrictHostKeyChecking no\n")

	// Hint on stderr so it doesn't pollute piped output.
	fmt.Fprintf(os.Stderr, "\nKey uploaded (%s). Add the above to ~/.ssh/config, then:\n  ssh sandbox-%s\n", pubkeyPath, name)

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
