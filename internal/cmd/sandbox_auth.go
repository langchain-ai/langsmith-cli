package cmd

import (
	"context"
	"fmt"
)

// sandboxEndpoint holds the resolved dataplane URL for a sandbox.
type sandboxEndpoint struct {
	DataplaneURL string
}

// resolveSandbox resolves a sandbox name to its dataplane endpoint.
// Requires the sandbox to be in "ready" state. Use resolveSandboxURL
// for operations that only need the dataplane URL regardless of status.
func resolveSandbox(ctx context.Context, name string) (sandboxEndpoint, error) {
	c := MustGetClient()
	box, err := c.SDK.Sandboxes.Boxes.Get(ctx, name)
	if err != nil {
		return sandboxEndpoint{}, fmt.Errorf("getting sandbox: %w", err)
	}
	if box.Status != "ready" {
		return sandboxEndpoint{}, fmt.Errorf("sandbox %q is not ready (status: %s)", name, box.Status)
	}
	if box.DataplaneURL == "" {
		return sandboxEndpoint{}, fmt.Errorf("sandbox %q has no dataplane URL", name)
	}
	return sandboxEndpoint{DataplaneURL: box.DataplaneURL}, nil
}

// resolveSandboxURL resolves a sandbox name to its dataplane URL without
// requiring "ready" status. Used by tunnel which may connect while the
// sandbox is still starting.
func resolveSandboxURL(ctx context.Context, name string) (string, error) {
	c := MustGetClient()
	box, err := c.SDK.Sandboxes.Boxes.Get(ctx, name)
	if err != nil {
		return "", fmt.Errorf("getting sandbox: %w", err)
	}
	if box.DataplaneURL == "" {
		return "", fmt.Errorf("sandbox %q has no dataplane URL", name)
	}
	return box.DataplaneURL, nil
}

// sandboxAuthHeaders returns the auth headers for sandbox dataplane requests.
// API keys take precedence; OAuth profiles use bearer auth and are refreshed
// before runtime operations.
func sandboxAuthHeaders() (map[string]string, error) {
	opts, err := resolveClientOptions(true)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{}
	if opts.WorkspaceID != "" {
		headers["X-Tenant-Id"] = opts.WorkspaceID
	}
	if opts.APIKey != "" {
		headers["X-Api-Key"] = opts.APIKey
		return headers, nil
	}
	if opts.OAuthAccessToken != "" {
		headers["Authorization"] = "Bearer " + opts.OAuthAccessToken
		return headers, nil
	}
	return nil, nil
}
