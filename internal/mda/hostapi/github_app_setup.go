package hostapi

// GitHub Apps as Agent Auth apps.
//
// The CLI creates the deployment's app from an App manifest and gets back the
// session that creates and installs it: Agent Auth's page sends the manifest
// to GitHub, stores the created App with its connection, and sends the user on
// to install it; the session completes with the agent's credential on the
// app's connection. The CLI never sees the private key of an App Agent Auth
// creates; an App the user brings sends its key straight to Agent Auth, which
// reads the App from GitHub with it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

const appsPath = "/v1/agent-auth/apps"

func appPath(appID string) string {
	return appsPath + "/" + appID
}

// GitHubAppManifest is the part of the GitHub App manifest the caller
// controls.
type GitHubAppManifest struct {
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	WebhookURL  string            `json:"webhook_url,omitempty"`
	Permissions map[string]string `json:"permissions"`
	Events      []string          `json:"events"`
}

// GitHubAppState is where an agent's App stands.
type GitHubAppState int

// The states.
const (
	// GitHubAppStatePending: GitHub has not created the App from its manifest
	// yet.
	GitHubAppStatePending GitHubAppState = iota
	// GitHubAppStateCreated: Agent Auth had GitHub create the App, so GitHub's
	// install redirect comes back to Agent Auth.
	GitHubAppStateCreated
	// GitHubAppStateExternal: the App was brought in by its ID and key;
	// GitHub's redirects go wherever its owner set them.
	GitHubAppStateExternal
)

// AgentAuthApp is an Agent Auth app, as the CLI needs it.
type AgentAuthApp struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	// Source is `manifest` or `existing`.
	Source string `json:"source"`
	// Slug is the slug of the app's OAuth connection.
	Slug string `json:"slug"`
	// AppID is GitHub's App ID; empty while the App is pending.
	AppID       string         `json:"app_id"`
	Manifest    StoredManifest `json:"manifest"`
	AppMetadata *AppMetadata   `json:"app_metadata"`
	// StrandedApps are Apps GitHub created for this app that Agent Auth could
	// not store, oldest first. They stay on GitHub until someone deletes them
	// there.
	StrandedApps []AppMetadata `json:"stranded_apps"`
	ConnectionID string        `json:"connection_id"`
}

// UnmarshalJSON requires the id and slug.
func (a *AgentAuthApp) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id", "slug"); err != nil {
		return err
	}
	type plain AgentAuthApp
	return json.Unmarshal(data, (*plain)(a))
}

// StoredManifest is what the CLI reads back from an app's manifest.
type StoredManifest struct {
	// WebhookURL is where Agent Auth last pointed the App's webhook.
	WebhookURL string `json:"webhook_url"`
}

// AppMetadata is what GitHub assigned an app, or an App it created that Agent
// Auth could not store.
type AppMetadata struct {
	Slug    string `json:"slug"`
	HTMLURL string `json:"html_url"`
}

// UnmarshalJSON requires the slug and page.
func (m *AppMetadata) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "slug", "html_url"); err != nil {
		return err
	}
	type plain AppMetadata
	return json.Unmarshal(data, (*plain)(m))
}

// State is where the App stands.
func (a AgentAuthApp) State() GitHubAppState {
	switch {
	case a.AppID == "":
		return GitHubAppStatePending
	case a.Source == "existing":
		return GitHubAppStateExternal
	default:
		return GitHubAppStateCreated
	}
}

// StrandedURLs are the pages of the Apps GitHub created that Agent Auth could
// not store, oldest first.
func (a AgentAuthApp) StrandedURLs() []string {
	urls := []string{}
	for _, stranded := range a.StrandedApps {
		if page, ok := githubPage(stranded.HTMLURL); ok {
			urls = append(urls, page)
		}
	}
	return urls
}

func githubPage(url string) (string, bool) {
	return url, strings.HasPrefix(url, "https://github.com/")
}

// NewApp is a new app and the session that creates and installs it.
type NewApp struct {
	App     AgentAuthApp              `json:"app"`
	Session OAuthAuthorizationSession `json:"session"`
}

// UnmarshalJSON requires the app and session.
func (n *NewApp) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "app", "session"); err != nil {
		return err
	}
	type plain NewApp
	return json.Unmarshal(data, (*plain)(n))
}

// GitHubAppSessionStartKind is how a session on an agent's App should start.
type GitHubAppSessionStartKind int

// The ways to start.
const (
	// GitHubAppSessionStartCreate: create the pending App, under
	// [GitHubAppSessionStart.Organization] or the personal account.
	GitHubAppSessionStartCreate GitHubAppSessionStartKind = iota
	// GitHubAppSessionStartInstall: install the App Agent Auth created.
	GitHubAppSessionStartInstall
	// GitHubAppSessionStartInstallation: record the existing installation
	// [GitHubAppSessionStart.InstallationID].
	GitHubAppSessionStartInstallation
)

// GitHubAppSessionStart is how a session on an agent's App should start.
type GitHubAppSessionStart struct {
	Kind GitHubAppSessionStartKind
	// Organization is the organization of a create, "" for the personal
	// account.
	Organization string
	// InstallationID is the installation an installation start records.
	InstallationID string
}

// ExistingApp is an App the user brings in by its ID and key. Agent Auth mints
// its tokens with the App's own permissions until deploy narrows them to the
// channel's.
type ExistingApp struct {
	AppID         string
	PrivateKey    string
	WebhookSecret string
}

// CreateGitHubApp creates the agent's App from manifest, with the session that
// creates and installs it. organization is "" for the personal account.
func (c *Client) CreateGitHubApp(ctx context.Context, ownerID, slug string, manifest GitHubAppManifest, organization string) (NewApp, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return NewApp{}, err
	}
	payload := map[string]any{
		"provider": "github",
		"source":   "manifest",
		"owner_id": ownerID,
		"slug":     slug,
		"manifest": manifest,
	}
	if organization != "" {
		payload["organization"] = organization
	}
	raw, err := c.request(ctx, base, http.MethodPost, appsPath, nil, payload)
	if err != nil {
		return NewApp{}, err
	}
	var created NewApp
	if err := json.Unmarshal(nullIfEmpty(raw), &created); err != nil {
		return NewApp{}, fmt.Errorf("failed to decode the GitHub App: %w", err)
	}
	if strings.TrimSpace(created.Session.VerificationURL) == "" {
		return NewApp{}, errors.New("Agent Auth returned the GitHub App without a setup link")
	}
	return created, nil
}

// BringGitHubApp stores an existing App as the agent's. Agent Auth reads the
// App from GitHub with its key, which never shows up in errors.
func (c *Client) BringGitHubApp(ctx context.Context, ownerID, slug string, app ExistingApp) (AgentAuthApp, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return AgentAuthApp{}, err
	}
	payload := map[string]any{
		"provider": "github",
		"source":   "existing",
		"owner_id": ownerID,
		"slug":     slug,
		"existing": map[string]any{
			"app_id":         app.AppID,
			"private_key":    app.PrivateKey,
			"webhook_secret": app.WebhookSecret,
		},
	}
	raw, err := c.request(ctx, base, http.MethodPost, appsPath, nil, payload)
	if err != nil {
		return AgentAuthApp{}, RedactSecret(RedactSecret(err, app.PrivateKey), app.WebhookSecret)
	}
	return decodeCreatedApp(raw, "Agent Auth did not return the GitHub App", "failed to decode the GitHub App")
}

// decodeCreatedApp reads the `app` of a create response.
func decodeCreatedApp(raw json.RawMessage, missing, invalid string) (AgentAuthApp, error) {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(nullIfEmpty(raw), &response); err != nil {
		return AgentAuthApp{}, errors.New(missing)
	}
	created, ok := response["app"]
	if !ok {
		return AgentAuthApp{}, errors.New(missing)
	}
	var app AgentAuthApp
	if err := json.Unmarshal(created, &app); err != nil {
		return AgentAuthApp{}, fmt.Errorf("%s: %w", invalid, err)
	}
	return app, nil
}

// AgentApps lists the apps the agent owns.
func (c *Client) AgentApps(ctx context.Context, ownerID string) ([]AgentAuthApp, error) {
	apps, supported, err := c.AgentAppsIfSupported(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, errors.New("this LangSmith does not support GitHub Apps through Agent Auth yet")
	}
	return apps, nil
}

// AgentAppsIfSupported lists the apps ownerID owns, or reports false when
// this LangSmith has no Agent Auth apps, so no channel could have set one up.
func (c *Client) AgentAppsIfSupported(ctx context.Context, ownerID string) ([]AgentAuthApp, bool, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return nil, false, err
	}
	raw, found, err := c.requestOptional(ctx, base, http.MethodGet, appsPath, []queryParam{{"owner_id", ownerID}}, nil)
	if err != nil || !found {
		return nil, false, err
	}
	if raw == nil {
		return nil, false, errors.New("failed to decode the agent's apps: invalid type: null")
	}
	var apps []AgentAuthApp
	if err := json.Unmarshal(raw, &apps); err != nil {
		return nil, false, fmt.Errorf("failed to decode the agent's apps: %w", err)
	}
	return apps, true, nil
}

// DeleteAgentApp deletes an app with everything Agent Auth stored for it.
// false when it was already gone.
func (c *Client) DeleteAgentApp(ctx context.Context, appID string) (bool, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return false, err
	}
	_, deleted, err := c.requestOptional(ctx, base, http.MethodDelete, appPath(appID), nil, nil)
	return deleted, err
}

// GitHubAppScopes are the `permission:level` scopes the provider providerID
// mints tokens with, sorted.
func (c *Client) GitHubAppScopes(ctx context.Context, providerID string) ([]string, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return nil, err
	}
	raw, err := c.request(ctx, base, http.MethodGet, oauthProviderPath(providerID), nil, nil)
	if err != nil {
		return nil, err
	}
	provider, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}
	allowed, _ := field(provider, "allowed_scopes").([]any)
	scopes := []string{}
	for _, scope := range allowed {
		if scope, ok := scope.(string); ok {
			scopes = append(scopes, scope)
		}
	}
	slices.Sort(scopes)
	return scopes, nil
}

// UpdateGitHubAppScopes has the provider mint its tokens with scopes from now
// on. Agent Auth invalidates existing credentials, so callers renew them right
// after.
func (c *Client) UpdateGitHubAppScopes(ctx context.Context, providerID string, scopes []string) error {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return err
	}
	payload := map[string]any{"allowed_scopes": listOrEmpty(scopes), "default_scopes": listOrEmpty(scopes)}
	_, err = c.request(ctx, base, http.MethodPatch, oauthProviderPath(providerID), nil, payload)
	return err
}

// StartGitHubAppSession starts a session that gives the agent ownerID a
// credential on its App: creating a pending App first, or installing a
// created one.
func (c *Client) StartGitHubAppSession(ctx context.Context, app AgentAuthApp, ownerID string, start GitHubAppSessionStart) (OAuthAuthorizationSession, error) {
	fields := map[string]any{}
	switch start.Kind {
	case GitHubAppSessionStartCreate:
		base, err := c.PublicPlatformBase()
		if err != nil {
			return OAuthAuthorizationSession{}, err
		}
		payload := map[string]any{
			"agent_auth_app_id": app.ID,
			"owner_type":        "agent",
			"owner_id":          ownerID,
		}
		if start.Organization != "" {
			payload["organization"] = start.Organization
		}
		raw, err := c.request(ctx, base, http.MethodPost, oauthAuthorizationSessionsPath, nil, payload)
		if err != nil {
			return OAuthAuthorizationSession{}, err
		}
		var session OAuthAuthorizationSession
		if err := json.Unmarshal(nullIfEmpty(raw), &session); err != nil {
			return OAuthAuthorizationSession{}, fmt.Errorf("failed to decode the GitHub App setup session: %w", err)
		}
		return session, nil
	case GitHubAppSessionStartInstallation:
		fields["installation_id"] = start.InstallationID
	case GitHubAppSessionStartInstall:
	}
	if app.ConnectionID == "" {
		return OAuthAuthorizationSession{}, errors.New("the GitHub App has no connection yet")
	}
	return c.StartAgentOAuthAuthorizationSession(ctx, app.ConnectionID, ownerID, fields)
}
