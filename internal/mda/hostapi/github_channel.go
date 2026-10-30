package hostapi

// An agent's GitHub App, as Agent Auth reports it.
//
// Agent Auth holds the App's private key, so it, not the CLI or the
// deployment, signs GitHub App requests. `GET /v1/agent-auth/apps/{id}`
// returns the App's state as GitHub reports it for the CLI to report on, and
// `PATCH` points its webhook at the deployment.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// GitHubAccount is a GitHub user or organization.
type GitHubAccount struct {
	Login string `json:"login"`
	Kind  string `json:"type"`
}

// UnmarshalJSON requires the login.
func (a *GitHubAccount) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "login"); err != nil {
		return err
	}
	type plain GitHubAccount
	return json.Unmarshal(data, (*plain)(a))
}

// IsOrganization reports whether the account is an organization.
func (a GitHubAccount) IsOrganization() bool {
	return a.Kind == "Organization"
}

// GitHubApp is a GitHub App as GitHub reports it.
type GitHubApp struct {
	ID          uint64            `json:"id"`
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Owner       GitHubAccount     `json:"owner"`
	Permissions map[string]string `json:"permissions"`
	Events      []string          `json:"events"`
	// BotID is the App's bot account, once Agent Auth has recorded it; 0 until
	// then.
	BotID uint64 `json:"bot_id"`
	// WebhookURL is where the App's webhook points; empty when it has none.
	WebhookURL string `json:"webhook_url"`
}

// UnmarshalJSON requires the id, slug and owner.
func (a *GitHubApp) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id", "slug", "owner"); err != nil {
		return err
	}
	type plain GitHubApp
	return json.Unmarshal(data, (*plain)(a))
}

// GitHubInstallation is an installation of a GitHub App.
type GitHubInstallation struct {
	ID      uint64        `json:"id"`
	Account GitHubAccount `json:"account"`
	// RepositorySelection is `all` or `selected`.
	RepositorySelection   string   `json:"repository_selection"`
	Repositories          []string `json:"repositories"`
	RepositoriesTruncated bool     `json:"repositories_truncated"`
	// Permissions are the permissions the installation has accepted.
	Permissions map[string]string `json:"permissions"`
	SuspendedAt string            `json:"suspended_at"`
	SettingsURL string            `json:"settings_url"`
}

// UnmarshalJSON requires the id and account.
func (i *GitHubInstallation) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id", "account"); err != nil {
		return err
	}
	type plain GitHubInstallation
	return json.Unmarshal(data, (*plain)(i))
}

// GitHubInstallationRequest is a request to install a GitHub App.
type GitHubInstallationRequest struct {
	Account   GitHubAccount  `json:"account"`
	Requester *GitHubAccount `json:"requester"`
}

// UnmarshalJSON requires the account.
func (r *GitHubInstallationRequest) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "account"); err != nil {
		return err
	}
	type plain GitHubInstallationRequest
	return json.Unmarshal(data, (*plain)(r))
}

// GitHubChannelStatus is the App as GitHub reports it.
type GitHubChannelStatus struct {
	App                  GitHubApp                   `json:"app"`
	Installations        []GitHubInstallation        `json:"installations"`
	InstallationRequests []GitHubInstallationRequest `json:"installation_requests"`
}

// UnmarshalJSON requires the app.
func (s *GitHubChannelStatus) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "app"); err != nil {
		return err
	}
	type plain GitHubChannelStatus
	return json.Unmarshal(data, (*plain)(s))
}

// GitHubSyncOutcomeKind is the result of one report. Only
// [GitHubSyncOutcomeUnavailable] is not the developer's to fix.
type GitHubSyncOutcomeKind int

// The outcomes.
const (
	// GitHubSyncOutcomeSynced: the report is [GitHubSyncOutcome.Status].
	GitHubSyncOutcomeSynced GitHubSyncOutcomeKind = iota
	// GitHubSyncOutcomeMissingCredentials: Agent Auth has no GitHub App for
	// this agent, or the caller cannot see it (403/404).
	GitHubSyncOutcomeMissingCredentials
	// GitHubSyncOutcomeInvalidCredentials: GitHub rejected the stored
	// credentials (422).
	GitHubSyncOutcomeInvalidCredentials
	// GitHubSyncOutcomeNotCreated: the App is still pending, so GitHub has
	// nothing to report.
	GitHubSyncOutcomeNotCreated
	// GitHubSyncOutcomeUnavailable: network, GitHub, or server trouble;
	// [GitHubSyncOutcome.Reason] says which.
	GitHubSyncOutcomeUnavailable
)

// GitHubSyncOutcome is the result of one report.
type GitHubSyncOutcome struct {
	Kind GitHubSyncOutcomeKind
	// Status is the report of [GitHubSyncOutcomeSynced].
	Status *GitHubChannelStatus
	// Reason is the short reason of [GitHubSyncOutcomeUnavailable].
	Reason string
}

// appDetails is an app as `GET /v1/agent-auth/apps/{id}` returns it.
type appDetails struct {
	AppMetadata *detailsMetadata     `json:"app_metadata"`
	GitHub      *GitHubChannelStatus `json:"github"`
}

type detailsMetadata struct {
	BotID string `json:"bot_id"`
}

// GitHubAppReport is the agent's App as GitHub reports it now.
func (c *Client) GitHubAppReport(ctx context.Context, appID string) GitHubSyncOutcome {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: err.Error()}
	}
	// Deploy checks the repositories a channel is limited to, so the report
	// lists each installation's; Agent Auth leaves them out otherwise.
	raw, err := c.request(ctx, base, http.MethodGet, appPath(appID), []queryParam{{"include", "repositories"}}, nil)
	return classifySyncResult(raw, err)
}

// PointGitHubAppWebhook points the agent's App's webhook at webhookURL,
// replacing whatever it points at now. Callers ask first when that is someone
// else's.
func (c *Client) PointGitHubAppWebhook(ctx context.Context, appID, webhookURL string) error {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"manifest":           map[string]any{"webhook_url": webhookURL},
		"overwrite_external": true,
	}
	_, err = c.request(ctx, base, http.MethodPatch, appPath(appID), nil, payload)
	return err
}

func classifySyncResult(raw json.RawMessage, err error) GitHubSyncOutcome {
	if err != nil {
		var api *APIError
		var unreachable *requestError
		switch {
		case errors.As(err, &api):
			switch api.Status {
			case http.StatusNotFound, http.StatusForbidden:
				return GitHubSyncOutcome{Kind: GitHubSyncOutcomeMissingCredentials}
			case http.StatusUnprocessableEntity:
				return GitHubSyncOutcome{Kind: GitHubSyncOutcomeInvalidCredentials}
			default:
				return GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: fmt.Sprintf("HTTP %d", api.Status)}
			}
		case errors.As(err, &unreachable):
			return GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: "Agent Auth could not be reached"}
		default:
			return GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: err.Error()}
		}
	}
	var details appDetails
	if err := json.Unmarshal(nullIfEmpty(raw), &details); err != nil || raw == nil {
		return GitHubSyncOutcome{Kind: GitHubSyncOutcomeUnavailable, Reason: "Agent Auth returned an unexpected GitHub App status"}
	}
	if details.GitHub == nil {
		return GitHubSyncOutcome{Kind: GitHubSyncOutcomeNotCreated}
	}
	status := details.GitHub
	status.App.BotID = 0
	if details.AppMetadata != nil {
		if botID, err := strconv.ParseUint(details.AppMetadata.BotID, 10, 64); err == nil {
			status.App.BotID = botID
		}
	}
	return GitHubSyncOutcome{Kind: GitHubSyncOutcomeSynced, Status: status}
}
