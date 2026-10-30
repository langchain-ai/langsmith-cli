package hostapi

// LangSmith platform API endpoints for workspace secrets.
//
// Used by the deploy pre-flight to confirm a model provider's API key is
// available to the deployment: either forwarded from the project's `.env` or
// already configured as a workspace secret in LangSmith (Settings > Workspaces >
// Secrets). Only the secret names are returned by the API (never the values),
// which is all the credential check needs. Calls go to the platform API
// (`api.smith.langchain.com`).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// secretKey is a single workspace secret entry. The LangSmith API returns only
// the key name, never the value.
type secretKey struct {
	Key string `json:"key"`
}

// UnmarshalJSON requires the key.
func (s *secretKey) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "key"); err != nil {
		return err
	}
	type plain secretKey
	return json.Unmarshal(data, (*plain)(s))
}

// WorkspaceSecretKeys fetches the names of the secrets configured for the
// current LangSmith workspace from `GET /api/v1/workspaces/current/secrets`.
//
// Returns the secret key names (values are never exposed by the API). Errors
// when the request fails; the caller decides whether an inability to verify
// should block the deploy.
func (c *Client) WorkspaceSecretKeys(ctx context.Context) ([]string, error) {
	raw, err := c.request(ctx, c.PlatformBase(), http.MethodGet, "/api/v1/workspaces/current/secrets", nil, nil)
	if err != nil {
		return nil, err
	}

	// An empty body (null) or an empty array both mean "no secrets
	// configured", which is a valid state.
	if raw == nil {
		return []string{}, nil
	}
	var secrets []secretKey
	if err := json.Unmarshal(raw, &secrets); err != nil {
		return nil, fmt.Errorf("failed to decode the workspace secrets response: %w", err)
	}
	keys := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		keys = append(keys, secret.Key)
	}
	return keys, nil
}
