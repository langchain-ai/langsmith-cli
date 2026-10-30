package hostapi

// Linear apps as Agent Auth apps.
//
// Linear has no API to create an app, so a workspace admin creates it and
// deploy brings it in by its client ID and secret. Agent Auth builds the
// Linear OAuth provider for it, keeps its webhook signing secret at
// `{slug}/webhook-secret`, and deletes all of it with the app.

import (
	"context"
	"net/http"
)

// ExistingLinearApp is a Linear app the workspace has, as its settings page
// shows it.
type ExistingLinearApp struct {
	ClientID      string
	ClientSecret  string
	WebhookSecret string
	// Scopes are what the install asks for.
	Scopes []string
	// AllowedScopes are what the agent's tokens may be narrowed to.
	AllowedScopes []string
}

// BringLinearApp stores an existing Linear app as the agent's. Neither secret
// ever shows up in errors.
func (c *Client) BringLinearApp(ctx context.Context, ownerID, slug string, app ExistingLinearApp) (AgentAuthApp, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return AgentAuthApp{}, err
	}
	payload := map[string]any{
		"provider": "linear",
		"source":   "existing",
		"owner_id": ownerID,
		"slug":     slug,
		"existing": map[string]any{
			"client_id":      app.ClientID,
			"client_secret":  app.ClientSecret,
			"webhook_secret": app.WebhookSecret,
			"scopes":         listOrEmpty(app.Scopes),
			"allowed_scopes": listOrEmpty(app.AllowedScopes),
		},
	}
	raw, err := c.request(ctx, base, http.MethodPost, appsPath, nil, payload)
	if err != nil {
		return AgentAuthApp{}, RedactSecret(RedactSecret(err, app.ClientSecret), app.WebhookSecret)
	}
	return decodeCreatedApp(raw, "Agent Auth did not return the Linear app", "failed to decode the Linear app")
}
