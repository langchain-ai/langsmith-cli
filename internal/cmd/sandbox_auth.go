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
	var box boxResponse
	if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+name, &box); err != nil {
		return sandboxEndpoint{}, fmt.Errorf("getting sandbox: %w", err)
	}
	if box.Status != "ready" {
		return sandboxEndpoint{}, fmt.Errorf("sandbox %q is not ready (status: %s)", name, box.Status)
	}
	if box.DataplaneURL == nil || *box.DataplaneURL == "" {
		return sandboxEndpoint{}, fmt.Errorf("sandbox %q has no dataplane URL", name)
	}
	return sandboxEndpoint{DataplaneURL: *box.DataplaneURL}, nil
}

// resolveSandboxURL resolves a sandbox name to its dataplane URL without
// requiring "ready" status. Used by tunnel which may connect while the
// sandbox is still starting.
func resolveSandboxURL(ctx context.Context, name string) (string, error) {
	c := MustGetClient()
	var box boxResponse
	if err := c.RawGet(ctx, "/v2/sandboxes/boxes/"+name, &box); err != nil {
		return "", fmt.Errorf("getting sandbox: %w", err)
	}
	if box.DataplaneURL == nil || *box.DataplaneURL == "" {
		return "", fmt.Errorf("sandbox %q has no dataplane URL", name)
	}
	return *box.DataplaneURL, nil
}

// sandboxAuthHeaders returns the auth headers for sandbox dataplane requests.
// Uses the user's API key via X-Api-Key.
func sandboxAuthHeaders() map[string]string {
	if apiKey := GetAPIKey(); apiKey != "" {
		return map[string]string{"X-Api-Key": apiKey}
	}
	return nil
}
