package hostapi

// Workspace and organization lookup on the LangSmith platform API
// (`api.smith.langchain.com`).
//
// Best-effort workspace/organization names and authenticated identity for CLI
// output and invocation metadata. An ambiguous workspace stays unknown rather
// than guessing which accessible workspace the credential selected.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	langsmith "github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

// lookupTimeout is the timeout for the optional workspace lookup.
//
// These lookups only ever decorate output: a dashboard link, or the lines
// naming where a deploy is going. At the shared 30-second [RequestTimeout] an
// unreachable platform API would sit there for another half minute, which
// reads as a hung CLI, so cap it at something a developer will not notice.
const lookupTimeout = 5 * time.Second

// WorkspaceSummary is a workspace as listed by the platform API.
type WorkspaceSummary struct {
	// ID is the workspace id, the `/o/{id}` segment of every dashboard link.
	ID string
	// DisplayName is the workspace name as shown in the dashboard, when the
	// listing carried one.
	DisplayName string
}

// Label is the workspace's name, falling back to its id when unnamed.
func (w WorkspaceSummary) Label() string {
	if w.DisplayName != "" {
		return w.DisplayName
	}
	return w.ID
}

// DeployTarget is the LangSmith organization and workspace a client acts in.
//
// Both fields are optional: this exists to tell a developer where their
// deploy is going, so an unavailable platform API or a credential without
// organization read degrades to saying less rather than to an error.
type DeployTarget struct {
	// Organization is the organization display name.
	Organization string
	// Workspace is the workspace, when exactly one could be identified.
	Workspace *WorkspaceSummary
	// workspaces is every workspace the lookup saw, for naming an id that
	// turns up later.
	workspaces []WorkspaceSummary
}

// LabelFor names workspaceID, falling back to the id when it was not listed.
func (t DeployTarget) LabelFor(workspaceID string) string {
	for _, workspace := range t.workspaces {
		if workspace.ID == workspaceID {
			return workspace.Label()
		}
	}
	return workspaceID
}

// AuthIdentity is the user, organization and workspace LangSmith verifies
// behind a credential. It serializes as `user_id`, `org_id` and
// `workspace_id`; empty fields are unknown.
type AuthIdentity struct {
	UserID      string `json:"user_id"`
	OrgID       string `json:"org_id"`
	WorkspaceID string `json:"workspace_id"`
}

// UnmarshalJSON reads the identity from the `/auth/public` field names.
func (a *AuthIdentity) UnmarshalJSON(data []byte) error {
	var wire struct {
		UserID      *string `json:"ls_user_id"`
		OrgID       *string `json:"organization_id"`
		WorkspaceID *string `json:"tenant_id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*a = AuthIdentity{UserID: deref(wire.UserID), OrgID: deref(wire.OrgID), WorkspaceID: deref(wire.WorkspaceID)}
	return nil
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// AuthIdentityWithin is [Client.AuthIdentity], giving up after timeout.
// Skipped when the credential would cross the network in cleartext, since
// nothing the user asked for depends on it.
func (c *Client) AuthIdentityWithin(ctx context.Context, timeout time.Duration) (AuthIdentity, bool) {
	if !c.SendsKeySecurely() {
		return AuthIdentity{}, false
	}
	return c.WithTimeout(timeout).AuthIdentity(ctx)
}

// AuthIdentity is a best-effort identity lookup; errors leave optional
// metadata unknown.
func (c *Client) AuthIdentity(ctx context.Context) (AuthIdentity, bool) {
	raw, err := c.request(ctx, c.PlatformBase(), http.MethodGet, "/auth/public", nil, nil)
	if err != nil {
		slog.Debug("LangSmith identity lookup failed", "error", err)
		return AuthIdentity{}, false
	}
	var metadata AuthIdentity
	if err := json.Unmarshal(raw, &metadata); err != nil {
		slog.Debug("LangSmith identity lookup returned an unexpected body", "error", err)
		return AuthIdentity{}, false
	}
	for _, id := range []*string{&metadata.UserID, &metadata.OrgID, &metadata.WorkspaceID} {
		value := strings.TrimSpace(*id)
		if strings.ContainsAny(value, "\n\r") {
			value = ""
		}
		*id = value
	}
	if tenant := c.WorkspaceID(); tenant != "" && metadata.WorkspaceID != tenant {
		return AuthIdentity{}, false
	}
	return metadata, true
}

// WorkspaceIDForLinks resolves a workspace id for this client, for use in
// dashboard links.
//
// Prefers the workspace the client is already scoped to and otherwise asks
// `GET /api/v1/workspaces`, which lists the workspaces the credential can see
// in its own organization. Any of them is good enough for an
// organization-level page (billing, plans): the route segment selects the org
// through the workspace, so every workspace in the org lands on the same
// settings page.
//
// Best effort by design: this only ever decorates a message, so an
// unavailable platform API, a credential without organization read, or an
// unexpected body all read as "no link to offer" rather than an error. The
// lookup is bounded by [lookupTimeout] for the same reason: failing to
// decorate a message must not hold up printing it.
func (c *Client) WorkspaceIDForLinks(ctx context.Context) (string, bool) {
	if tenant := c.WorkspaceID(); tenant != "" {
		return tenant, true
	}
	id, ok, err := c.firstWorkspaceID(ctx)
	if err != nil {
		slog.Debug("workspace lookup failed", "error", err)
		return "", false
	}
	return id, ok
}

// DescribeDeployTarget names the organization and workspace this client acts
// in.
//
// The workspace is only claimed when it is unambiguous: either the client is
// scoped to one, or the credential can see exactly one. A personal access
// token that can see several resolves its default workspace server-side, and
// guessing which one that is would be worse than saying nothing:
// [DeployTarget.LabelFor] names it from the same listing once a response
// reveals the id.
//
// Best effort, like [Client.WorkspaceIDForLinks]: every lookup failure leaves
// the corresponding field empty.
func (c *Client) DescribeDeployTarget(ctx context.Context) DeployTarget {
	workspaces, err := c.listWorkspaces(ctx)
	if err != nil {
		slog.Debug("workspace lookup failed", "error", err)
	}
	var workspace *WorkspaceSummary
	if id := c.WorkspaceID(); id != "" {
		scoped := WorkspaceSummary{ID: id}
		for _, listed := range workspaces {
			if listed.ID == id {
				scoped.DisplayName = listed.DisplayName
				break
			}
		}
		workspace = &scoped
	} else if len(workspaces) == 1 {
		only := workspaces[0]
		workspace = &only
	}
	organization, err := c.organizationName(ctx)
	if err != nil {
		slog.Debug("organization lookup failed", "error", err)
	}
	return DeployTarget{
		Organization: organization,
		Workspace:    workspace,
		workspaces:   workspaces,
	}
}

// organizationName is the organization display name from
// `GET /api/v1/orgs/current`.
func (c *Client) organizationName(ctx context.Context) (string, error) {
	raw, err := c.lookup(ctx, "/api/v1/orgs/current")
	if err != nil {
		return "", err
	}
	object, err := decodeAny(raw)
	if err != nil {
		return "", err
	}
	name, _ := nonEmptyStr(field(object, "display_name"))
	return name, nil
}

// listWorkspaces lists the workspaces the credential can see, from
// `GET /api/v1/workspaces`.
func (c *Client) listWorkspaces(ctx context.Context) ([]WorkspaceSummary, error) {
	raw, err := c.sdkRequest(ctx, http.MethodGet, "/api/v1/workspaces", requestOptions{timeout: lookupTimeout},
		func(ctx context.Context, opts ...option.RequestOption) error {
			_, err := c.ls.SDK.Workspaces.List(ctx, langsmith.WorkspaceListParams{}, opts...)
			return err
		})
	if err != nil {
		return nil, err
	}
	value, err := decodeAny(raw)
	if err != nil {
		return nil, err
	}
	items, _ := value.([]any)
	var workspaces []WorkspaceSummary
	for _, item := range items {
		if workspace, ok := decodeWorkspace(item); ok {
			workspaces = append(workspaces, workspace)
		}
	}
	return workspaces, nil
}

// AgentPlatformReady reports whether the workspace uses the Agent platform,
// false when unknown.
func (c *Client) AgentPlatformReady(ctx context.Context) bool {
	raw, err := c.lookup(ctx, "/v1/platform/workspaces/current/info")
	if err != nil {
		slog.Debug("workspace info lookup failed", "error", err)
		return false
	}
	info, err := decodeAny(raw)
	if err != nil {
		slog.Debug("workspace info lookup returned an unexpected body", "error", err)
		return false
	}
	ready, _ := field(field(info, "config"), "agent_platform_workspace_ready").(bool)
	return ready
}

// DashboardURL is the authoritative dashboard base URL for this LangSmith
// installation.
func (c *Client) DashboardURL(ctx context.Context) (string, error) {
	raw, err := c.lookup(ctx, "/v1/platform/workspaces/current/info")
	if err != nil {
		return "", err
	}
	info, err := decodeAny(raw)
	if err != nil {
		return "", err
	}
	value, ok := field(info, "dashboard_url").(string)
	if !ok {
		return "", errors.New("workspace info did not include dashboard_url")
	}
	return normalizeDashboardURL(value)
}

func (c *Client) firstWorkspaceID(ctx context.Context) (string, bool, error) {
	workspaces, err := c.listWorkspaces(ctx)
	if err != nil {
		return "", false, err
	}
	if len(workspaces) == 0 {
		return "", false, nil
	}
	return workspaces[0].ID, true, nil
}

// lookup is a bounded, read-only GET against the platform API.
func (c *Client) lookup(ctx context.Context, path string) (json.RawMessage, error) {
	return c.requestWithTimeout(ctx, c.PlatformBase(), http.MethodGet, path, nil, nil, lookupTimeout)
}

func decodeWorkspace(value any) (WorkspaceSummary, bool) {
	id, ok := nonEmptyStr(field(value, "id"))
	if !ok {
		return WorkspaceSummary{}, false
	}
	name, _ := nonEmptyStr(field(value, "display_name"))
	return WorkspaceSummary{ID: id, DisplayName: name}, true
}

func nonEmptyStr(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return nonEmptyTrimmed(text)
}

// field is value[key] for a JSON object, nil for anything else.
func field(value any, key string) any {
	object, _ := value.(map[string]any)
	return object[key]
}

// decodeAny decodes a JSON response the way serde_json reads a Value: nil for
// an empty or null body.
func decodeAny(raw json.RawMessage) (any, error) {
	if raw == nil {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func normalizeDashboardURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("dashboard_url is not a valid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") ||
		u.Host == "" ||
		u.User != nil ||
		u.RawQuery != "" || u.ForceQuery ||
		u.Fragment != "" || strings.Contains(trimmed, "#") {
		return "", errors.New("dashboard_url must be an absolute HTTP(S) URL without credentials or query data")
	}
	return strings.TrimRight(u.String(), "/"), nil
}
