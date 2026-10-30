package hostapi

// Agent-auth connection endpoints on the LangSmith platform API.
//
// Backs `langsmith deploy`'s connections, which are workspace-scoped (opaque
// secrets and OAuth configuration). Workspace connections live on
// `/v1/agent-auth/connections`; linked credential rows are rotated through
// `/v1/agent-auth/credentials` when the connections API has no secret patch.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

const (
	connectionsPath                = "/v1/agent-auth/connections"
	oauthProvidersPath             = "/v1/agent-auth/oauth-providers"
	credentialsPath                = "/v1/agent-auth/credentials"
	oauthAuthorizationSessionsPath = "/v1/agent-auth/oauth-authorization-sessions"
	oauthCallbackPath              = "/v1/agent-auth/oauth/callback"
	oauthClientMetadataPath        = "/v1/agent-auth/oauth/client-metadata.json"
)

const (
	kindSecret = "secret"
	kindOAuth2 = "oauth2"
)

const (
	ownerPolicyAgent = "agent"
	ownerPolicyUser  = "user"
)

// ConnectionOwnerPolicy says who supplies material for a workspace connection
// at runtime.
type ConnectionOwnerPolicy int

// The owner policies.
const (
	ConnectionOwnerPolicyAgent ConnectionOwnerPolicy = iota
	ConnectionOwnerPolicyUser
)

// String is the policy's wire name.
func (p ConnectionOwnerPolicy) String() string {
	if p == ConnectionOwnerPolicyUser {
		return ownerPolicyUser
	}
	return ownerPolicyAgent
}

// agentAuthPageSize is the items per list request. This is exactly the
// server's cap; asking for more is a 400 rather than a clamp.
const agentAuthPageSize = 100

// ConnectionSummary is a workspace connection.
//
// Deliberately carries no secret field: this is what `mda connections --json`
// serializes, so a field here would be a field on stdout.
type ConnectionSummary struct {
	// ConnectionID is read from `connection_id`, `credential_id` or `id`.
	ConnectionID string `json:"connection_id"`
	Slug         string `json:"slug"`
	DisplayName  string `json:"display_name"`
	// Kind is `secret` or `oauth2`.
	Kind            string  `json:"kind"`
	OAuthProviderID *string `json:"oauth_provider_id"`
	// CreatedAt and UpdatedAt default to empty so a missing timestamp cannot
	// fail a whole listing.
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// OAuthAppSlug is the workspace OAuth provider slug when `kind=oauth2`
	// (enriched client-side).
	OAuthAppSlug string `json:"oauth_app_slug,omitempty"`
	// Service is the catalog or custom service when `kind=oauth2` (enriched
	// client-side).
	Service string `json:"service,omitempty"`
	// OwnerPolicy is `agent` (shared) or `user` (per-caller grant). Empty when
	// the server does not expose ownership on the connection row.
	OwnerPolicy string `json:"owner_policy,omitempty"`
}

// UnmarshalJSON reads a connection row, taking the id from `connection_id`
// or its aliases `credential_id` and `id`.
func (s *ConnectionSummary) UnmarshalJSON(data []byte) error {
	type plain ConnectionSummary
	var wire struct {
		plain
		CredentialID *string `json:"credential_id"`
		ID           *string `json:"id"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if _, ok := object["connection_id"]; !ok {
		switch {
		case wire.CredentialID != nil:
			wire.ConnectionID = *wire.CredentialID
		case wire.ID != nil:
			wire.ConnectionID = *wire.ID
		default:
			return errors.New("missing field `connection_id`")
		}
	}
	if err := requireFields(data, "slug", "kind"); err != nil {
		return err
	}
	*s = ConnectionSummary(wire.plain)
	return nil
}

// CreateOAuthConnection holds the fields for creating a workspace OAuth
// connection row. Empty optional strings are unset.
//
// Agent Auth creates the OAuth provider in the same transaction as the
// connection (`credential.provider`). Linking an existing provider by id is no
// longer supported.
type CreateOAuthConnection struct {
	Slug                    string
	OwnerPolicy             ConnectionOwnerPolicy
	Service                 string
	ClientID                string
	ClientSecret            string
	DisplayName             string
	Scopes                  []string
	AllowedScopes           []string
	AuthorizeURL            string
	TokenURL                string
	TokenEndpointAuthMethod string
	AuthorizationParams     map[string]any
	GrantType               string
	TokenParams             map[string]any
	// TokenRequestHeaders are write-only headers for token and revocation
	// requests. The server encrypts them and never returns them.
	TokenRequestHeaders   map[string]any
	ClientRegistrationURL string
	// MCPServerURL, when set, has Agent Auth run MCP discovery + DCR instead of
	// the manual fields.
	MCPServerURL string
}

type oauthProviderRecord struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ClientID  string `json:"client_id"`
	GrantType string `json:"grant_type"`
}

// UnmarshalJSON requires the id and slug.
func (r *oauthProviderRecord) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id", "slug"); err != nil {
		return err
	}
	type plain oauthProviderRecord
	return json.Unmarshal(data, (*plain)(r))
}

type connectionListPage struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

type oauthProviderListPage struct {
	Items      []oauthProviderRecord `json:"items"`
	NextCursor *string               `json:"next_cursor"`
}

// OAuthAuthorizationSession is an Agent Auth OAuth authorization session.
type OAuthAuthorizationSession struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	VerificationURL string `json:"verification_url"`
	// ErrorCode is why a failed session failed, e.g. `app_store_failed`.
	ErrorCode string `json:"error_code"`
}

// UnmarshalJSON requires the id and status.
func (s *OAuthAuthorizationSession) UnmarshalJSON(data []byte) error {
	if err := requireFields(data, "id", "status"); err != nil {
		return err
	}
	type plain OAuthAuthorizationSession
	return json.Unmarshal(data, (*plain)(s))
}

// OAuthRedirectURI is the redirect URI to register with the OAuth provider.
func (c *Client) OAuthRedirectURI() (string, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return "", err
	}
	return base + oauthCallbackPath, nil
}

// CreateWorkspaceConnection creates a workspace-scoped opaque connection.
// agentOwnerID and secret are "" when absent.
func (c *Client) CreateWorkspaceConnection(ctx context.Context, slug string, ownerPolicy ConnectionOwnerPolicy, agentOwnerID, secret string) (ConnectionSummary, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return ConnectionSummary{}, err
	}
	var payload map[string]any
	switch ownerPolicy {
	case ConnectionOwnerPolicyUser:
		if secret != "" {
			return ConnectionSummary{}, errors.New("user-owned opaque connections are slots only; omit the secret value")
		}
		// Incomplete secret credential creates the connection row only;
		// per-caller values are collected at runtime.
		payload = map[string]any{
			"slug":         slug,
			"display_name": slug,
			"credential":   map[string]any{"kind": kindSecret},
		}
	default:
		if secret == "" {
			return ConnectionSummary{}, errors.New("a secret value is required to create an agent-owned opaque connection")
		}
		if agentOwnerID == "" {
			return ConnectionSummary{}, errors.New("a deployment id is required to create an agent-owned opaque connection")
		}
		payload = map[string]any{
			"slug":         slug,
			"display_name": slug,
			"credential": map[string]any{
				"kind":       kindSecret,
				"owner_type": "agent",
				"owner_id":   agentOwnerID,
				"value":      secret,
			},
		}
	}
	raw, err := c.request(ctx, base, http.MethodPost, connectionsPath, nil, payload)
	if err != nil {
		if secret != "" {
			err = RedactSecret(err, secret)
		}
		return ConnectionSummary{}, err
	}
	connection, err := decodeConnection(raw)
	if err != nil {
		return ConnectionSummary{}, err
	}
	connection.OwnerPolicy = ownerPolicy.String()
	return connection, nil
}

// CreateOAuthWorkspaceConnection creates a workspace-scoped OAuth connection
// with a nested provider.
func (c *Client) CreateOAuthWorkspaceConnection(ctx context.Context, options CreateOAuthConnection) (ConnectionSummary, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return ConnectionSummary{}, err
	}
	provider, err := oauthProviderCreateBody(options)
	if err != nil {
		return ConnectionSummary{}, err
	}
	// The provider is named after the connection unless it has its own slug,
	// and provider slugs can't have the "/" namespaces connection slugs can
	// (`mda/linear-app/<deployment>`).
	if strings.Contains(options.Slug, "/") {
		provider["slug"] = strings.ReplaceAll(options.Slug, "/", "-")
	}
	payload := map[string]any{
		"slug":         options.Slug,
		"display_name": options.Slug,
		"credential": map[string]any{
			"kind":     kindOAuth2,
			"provider": provider,
		},
	}
	raw, err := c.request(ctx, base, http.MethodPost, connectionsPath, nil, payload)
	if err != nil {
		return ConnectionSummary{}, redactCreateSecrets(err, options.ClientSecret, options)
	}
	connection, err := decodeConnection(raw)
	if err != nil {
		return ConnectionSummary{}, err
	}
	connection.OwnerPolicy = options.OwnerPolicy.String()
	if options.Service != "" {
		connection.Service = options.Service
	}
	return connection, nil
}

// CreateAgentOAuthAuthorizationSession starts OAuth for an agent-owned
// credential.
func (c *Client) CreateAgentOAuthAuthorizationSession(ctx context.Context, connectionID, ownerID string) (OAuthAuthorizationSession, error) {
	return c.StartAgentOAuthAuthorizationSession(ctx, connectionID, ownerID, map[string]any{})
}

// StartAgentOAuthAuthorizationSession is
// [Client.CreateAgentOAuthAuthorizationSession] with fields a grant adds, such
// as a GitHub App's `installation_id`.
func (c *Client) StartAgentOAuthAuthorizationSession(ctx context.Context, connectionID, ownerID string, fields map[string]any) (OAuthAuthorizationSession, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return OAuthAuthorizationSession{}, err
	}
	payload := map[string]any{}
	for key, value := range fields {
		payload[key] = value
	}
	payload["connection_id"] = connectionID
	payload["owner_type"] = "agent"
	payload["owner_id"] = ownerID
	raw, err := c.request(ctx, base, http.MethodPost, oauthAuthorizationSessionsPath, nil, payload)
	if err != nil {
		return OAuthAuthorizationSession{}, err
	}
	var session OAuthAuthorizationSession
	if err := json.Unmarshal(nullIfEmpty(raw), &session); err != nil {
		return OAuthAuthorizationSession{}, fmt.Errorf("failed to decode the OAuth authorization session: %w", err)
	}
	if strings.TrimSpace(session.ID) == "" {
		return OAuthAuthorizationSession{}, errors.New("OAuth authorization session response missing id")
	}
	if session.Status != "completed" && strings.TrimSpace(session.VerificationURL) == "" {
		return OAuthAuthorizationSession{}, errors.New("OAuth authorization session response missing verification_url")
	}
	return session, nil
}

// GetOAuthAuthorizationSession waits for an OAuth authorization session to
// change state.
func (c *Client) GetOAuthAuthorizationSession(ctx context.Context, sessionID string, waitSeconds uint8) (OAuthAuthorizationSession, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return OAuthAuthorizationSession{}, err
	}
	raw, err := c.request(ctx, base, http.MethodGet, oauthAuthorizationSessionPath(sessionID),
		[]queryParam{{"wait_seconds", strconv.Itoa(int(waitSeconds))}}, nil)
	if err != nil {
		return OAuthAuthorizationSession{}, err
	}
	var session OAuthAuthorizationSession
	if err := json.Unmarshal(nullIfEmpty(raw), &session); err != nil {
		return OAuthAuthorizationSession{}, fmt.Errorf("failed to decode the OAuth authorization session status: %w", err)
	}
	return session, nil
}

// ListWorkspaceConnections lists every connection row in the authenticated
// workspace.
func (c *Client) ListWorkspaceConnections(ctx context.Context) ([]ConnectionSummary, error) {
	connections, err := c.ListConnectionsRaw(ctx)
	if err != nil {
		return nil, err
	}
	providers := map[string]oauthProviderRecord{}
	if slices.ContainsFunc(connections, func(connection ConnectionSummary) bool { return connection.OAuthProviderID != nil }) {
		if providers, err = c.oauthProviderIndex(ctx); err != nil {
			return nil, err
		}
	}
	for i := range connections {
		enrichConnection(&connections[i], providers)
	}
	return connections, nil
}

// ConnectionHasAgentCredential reports whether a connection carries a
// credential owned by agentOwnerID.
//
// The connections list is workspace-wide and takes no owner filter, so
// ownership is read one row at a time. A connection with no credential for the
// owner answers 200 without `credential.credential_id` rather than 404.
func (c *Client) ConnectionHasAgentCredential(ctx context.Context, connectionID, agentOwnerID string) (bool, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return false, err
	}
	raw, found, err := c.requestOptional(ctx, base, http.MethodGet, connectionPath(connectionID), agentOwnerParams(agentOwnerID), nil)
	if err != nil || !found {
		return false, err
	}
	connection, err := decodeAny(raw)
	if err != nil {
		return false, err
	}
	_, owned := credentialID(connection)
	return owned, nil
}

func agentOwnerParams(agentOwnerID string) []queryParam {
	return []queryParam{
		{"credential_owner_type", ownerPolicyAgent},
		{"credential_owner_id", agentOwnerID},
	}
}

// AgentOAuthGrant is the state of the OAuth grant an agent holds on a
// connection, from Agent Auth's credential metadata. Never reads the token
// itself.
func (c *Client) AgentOAuthGrant(ctx context.Context, connectionID, agentOwnerID string) (AgentGrant, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return AgentGrant{}, err
	}
	raw, found, err := c.requestOptional(ctx, base, http.MethodGet, connectionPath(connectionID), agentOwnerParams(agentOwnerID), nil)
	if err != nil {
		return AgentGrant{}, err
	}
	if !found {
		return AgentGrant{Kind: AgentGrantMissing}, nil
	}
	connection, err := decodeAny(raw)
	if err != nil {
		return AgentGrant{}, err
	}
	credential, ok := credentialID(connection)
	if !ok {
		return AgentGrant{Kind: AgentGrantMissing}, nil
	}
	// The credential's metadata: its stored status, no token.
	raw, found, err = c.requestOptional(ctx, base, http.MethodGet, credentialsPath+"/"+pathID(credential), nil, nil)
	if err != nil {
		return AgentGrant{}, err
	}
	if !found {
		return AgentGrant{Kind: AgentGrantMissing}, nil
	}
	metadata, err := decodeAny(raw)
	if err != nil {
		return AgentGrant{}, err
	}
	status, hasStatus := field(field(metadata, "oauth2"), "status").(string)
	return agentGrantFromStatus(status, hasStatus), nil
}

// PublishedOAuthRedirectURI is the redirect URI Agent Auth sends OAuth
// providers, when it publishes it: its OAuth client metadata, served only for
// an HTTPS callback.
func (c *Client) PublishedOAuthRedirectURI(ctx context.Context) (string, bool, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return "", false, err
	}
	raw, found, err := c.requestOptional(ctx, base, http.MethodGet, oauthClientMetadataPath, nil, nil)
	if err != nil || !found {
		return "", false, err
	}
	metadata, err := decodeAny(raw)
	if err != nil {
		return "", false, err
	}
	uris, _ := field(metadata, "redirect_uris").([]any)
	if len(uris) == 0 {
		return "", false, nil
	}
	uri, ok := nonEmptyStr(uris[0])
	return uri, ok, nil
}

// DeleteConnection deletes a connection, false when it was already gone.
func (c *Client) DeleteConnection(ctx context.Context, connectionID string) (bool, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return false, err
	}
	_, deleted, err := c.requestOptional(ctx, base, http.MethodDelete, connectionPath(connectionID), nil, nil)
	return deleted, err
}

// GitHubAppProviderIDs are the OAuth providers that mint tokens for the
// GitHub App appID, sorted. Agent Auth registers an App's provider with the
// App ID as its `client_id`.
func (c *Client) GitHubAppProviderIDs(ctx context.Context, appID string) ([]string, error) {
	providers, err := c.oauthProviderIndex(ctx)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, provider := range providers {
		if provider.GrantType == "github_app" && provider.ClientID == appID {
			ids = append(ids, provider.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// ListConnectionsRaw lists every connection row, without the OAuth provider
// slugs [Client.ListWorkspaceConnections] looks up for display.
func (c *Client) ListConnectionsRaw(ctx context.Context) ([]ConnectionSummary, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return nil, err
	}
	cursor := ""
	connections := []ConnectionSummary{}

	for {
		params := []queryParam{{"page_size", strconv.Itoa(agentAuthPageSize)}}
		if cursor != "" {
			params = append(params, queryParam{"cursor", cursor})
		}
		raw, err := c.request(ctx, base, http.MethodGet, connectionsPath, params, nil)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return connections, nil
		}
		var page connectionListPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed to decode the connection list response: %w", err)
		}
		for _, item := range page.Items {
			connection, err := decodeConnection(item)
			if err != nil {
				return nil, err
			}
			connections = append(connections, connection)
		}

		switch {
		case page.NextCursor == nil:
			return connections, nil
		case *page.NextCursor == cursor:
			return nil, errors.New("connection list returned a repeated cursor")
		default:
			cursor = *page.NextCursor
		}
	}
}

func (c *Client) oauthProviderIndex(ctx context.Context) (map[string]oauthProviderRecord, error) {
	base, err := c.PublicPlatformBase()
	if err != nil {
		return nil, err
	}
	cursor := ""
	providers := map[string]oauthProviderRecord{}

	for {
		params := []queryParam{{"page_size", strconv.Itoa(agentAuthPageSize)}}
		if cursor != "" {
			params = append(params, queryParam{"cursor", cursor})
		}
		raw, err := c.request(ctx, base, http.MethodGet, oauthProvidersPath, params, nil)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return nil, errors.New("failed to decode the OAuth provider list response: invalid type: null")
		}
		var page oauthProviderListPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("failed to decode the OAuth provider list response: %w", err)
		}
		for _, provider := range page.Items {
			providers[provider.ID] = provider
		}

		switch {
		case page.NextCursor == nil:
			return providers, nil
		case *page.NextCursor == cursor:
			return nil, errors.New("OAuth provider list returned a repeated cursor")
		default:
			cursor = *page.NextCursor
		}
	}
}

// oauthProviderCreateBody builds the nested `credential.provider` object for
// connection create.
func oauthProviderCreateBody(options CreateOAuthConnection) (map[string]any, error) {
	const needsEndpoints = "This OAuth connection needs endpoints. Name a catalog service with --oauth, " +
		"or pass --authorize-url and --token-url."
	if options.MCPServerURL != "" {
		return map[string]any{
			"display_name":   options.DisplayName,
			"mcp_server_url": options.MCPServerURL,
		}, nil
	}
	if options.TokenURL == "" {
		if options.GrantType == "client_credentials" {
			return nil, errs.Categorized(errs.UserInput, "This OAuth connection needs a token endpoint. Name a catalog service with --oauth, "+
				"or pass --token-url.")
		}
		return nil, errs.Categorized(errs.UserInput, needsEndpoints)
	}
	if options.GrantType != "client_credentials" && options.AuthorizeURL == "" {
		return nil, errs.Categorized(errs.UserInput, needsEndpoints)
	}
	if options.ClientID == "" {
		if options.ClientRegistrationURL != "" {
			return nil, errs.Categorized(errs.UserInput, fmt.Sprintf("This OAuth connection needs --client-id (and usually --secret-from-env "+
				"for the client secret). Register an app at %s, then pass the "+
				"credentials.", options.ClientRegistrationURL))
		}
		return nil, errs.Categorized(errs.UserInput, "This OAuth connection needs --client-id (and usually --secret-from-env "+
			"for the client secret). Register an app, then pass the credentials.")
	}
	if options.GrantType == "client_credentials" {
		if options.TokenEndpointAuthMethod != "client_secret_basic" && options.TokenEndpointAuthMethod != "client_secret_post" {
			return nil, errs.Categorized(errs.UserInput, "OAuth client credentials requires client_secret_basic or client_secret_post token endpoint authentication.")
		}
		if options.ClientSecret == "" {
			return nil, errs.Categorized(errs.UserInput, "OAuth client credentials requires a client secret.")
		}
		return map[string]any{
			"display_name":               options.DisplayName,
			"client_id":                  options.ClientID,
			"token_url":                  options.TokenURL,
			"token_endpoint_auth_method": options.TokenEndpointAuthMethod,
			"grant_type":                 options.GrantType,
			"token_params":               objectOrEmpty(options.TokenParams),
			"token_request_headers":      objectOrEmpty(options.TokenRequestHeaders),
			"default_scopes":             listOrEmpty(options.Scopes),
			"allowed_scopes":             listOrEmpty(options.Scopes),
			"client_secret":              options.ClientSecret,
		}, nil
	}
	provider := map[string]any{
		"display_name":               options.DisplayName,
		"client_id":                  options.ClientID,
		"authorization_url":          options.AuthorizeURL,
		"token_url":                  options.TokenURL,
		"token_endpoint_auth_method": options.TokenEndpointAuthMethod,
		"grant_type":                 options.GrantType,
		"authorization_params":       objectOrEmpty(options.AuthorizationParams),
		"token_params":               objectOrEmpty(options.TokenParams),
		"token_request_headers":      objectOrEmpty(options.TokenRequestHeaders),
		"default_scopes":             listOrEmpty(options.Scopes),
		"allowed_scopes":             listOrEmpty(options.AllowedScopes),
	}
	if options.ClientSecret != "" {
		provider["client_secret"] = options.ClientSecret
	}
	return provider, nil
}

// objectOrEmpty is object, or an empty JSON object for nil.
func objectOrEmpty(object map[string]any) map[string]any {
	if object == nil {
		return map[string]any{}
	}
	return object
}

// listOrEmpty is list, or an empty JSON array for nil.
func listOrEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// enrichConnection fills display metadata and the OAuth provider slug.
//
// Does not invent OwnerPolicy when the connections API omits it. Deploy and
// `connection check` skip ownership mismatch when OwnerPolicy is absent; MCP
// user connections are `kind=oauth2`, not opaque secrets.
func enrichConnection(connection *ConnectionSummary, providers map[string]oauthProviderRecord) {
	if connection.DisplayName == "" {
		connection.DisplayName = connection.Slug
	}
	if connection.OAuthProviderID != nil {
		if provider, ok := providers[*connection.OAuthProviderID]; ok {
			connection.OAuthAppSlug = provider.Slug
		}
	}
}

func connectionPath(connectionID string) string {
	return connectionsPath + "/" + pathID(connectionID)
}

func oauthAuthorizationSessionPath(sessionID string) string {
	return oauthAuthorizationSessionsPath + "/" + pathID(sessionID)
}

func oauthProviderPath(providerID string) string {
	return oauthProvidersPath + "/" + pathID(providerID)
}

// AgentGrantKind is the kind of OAuth grant an agent holds on a connection.
type AgentGrantKind int

// The grant kinds.
const (
	// AgentGrantMissing: no credential; nobody approved the install, or it was
	// removed.
	AgentGrantMissing AgentGrantKind = iota
	AgentGrantActive
	// AgentGrantAuthorizationRequired: the grant lapsed (the provider refused
	// to renew it) and needs consent again. Agent Auth keeps the credential,
	// and a new authorization session on the same connection makes it active
	// again.
	AgentGrantAuthorizationRequired
	// AgentGrantRevoked: the grant was revoked and Agent Auth is removing the
	// credential. The app itself, its connection and webhook secret, is still
	// good, so setup keeps it and has an admin approve it again, as for
	// [AgentGrantAuthorizationRequired]. Counting it as [AgentGrantMissing]
	// would delete the connection and ask for a brand-new app instead. It is
	// its own kind so setup can say why consent is needed.
	AgentGrantRevoked
	// AgentGrantUnknown: a status this CLI does not know, kept verbatim in
	// [AgentGrant.Status] so setup can name it rather than quietly treating the
	// grant as healthy.
	AgentGrantUnknown
)

// AgentGrant is the OAuth grant an agent holds on a connection, as Agent Auth
// records it.
type AgentGrant struct {
	Kind AgentGrantKind
	// Status is the unknown status of [AgentGrantUnknown], empty otherwise.
	Status string
}

// agentGrantFromStatus reads Agent Auth's `oauth2.status`. A credential without
// a status is active, as Agent Auth treats it.
func agentGrantFromStatus(status string, present bool) AgentGrant {
	if !present {
		return AgentGrant{Kind: AgentGrantActive}
	}
	switch status {
	case "active":
		return AgentGrant{Kind: AgentGrantActive}
	case "authorization_required":
		return AgentGrant{Kind: AgentGrantAuthorizationRequired}
	case "revocation_pending":
		return AgentGrant{Kind: AgentGrantRevoked}
	default:
		return AgentGrant{Kind: AgentGrantUnknown, Status: status}
	}
}

// pathID keeps a request path to the Agent Auth id it names: ids are UUIDs,
// and anything else must not change the request path.
func pathID(id string) string {
	if _, err := uuid.Parse(id); err == nil {
		return id
	}
	return "invalid"
}

// credentialID is the linked credential id from a connection read with owner
// query parameters.
func credentialID(connection any) (string, bool) {
	return nonEmptyStr(field(field(connection, "credential"), "credential_id"))
}

// decodeConnection flattens nested `credential.{kind,oauth_provider_id}` onto
// the summary shape the CLI and deploy gates already speak.
func decodeConnection(raw json.RawMessage) (ConnectionSummary, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(nullIfEmpty(raw), &object); err != nil || object == nil {
		return ConnectionSummary{}, errors.New("connection response must be a JSON object")
	}
	if credential, ok := object["credential"]; ok {
		var fields map[string]json.RawMessage
		if json.Unmarshal(credential, &fields) == nil {
			if kind, ok := fields["kind"]; ok {
				object["kind"] = kind
			}
			if providerID, ok := fields["oauth_provider_id"]; ok {
				object["oauth_provider_id"] = providerID
			}
		}
	}
	if _, ok := object["kind"]; !ok {
		return ConnectionSummary{}, errors.New("connection response missing credential.kind")
	}
	flattened, err := json.Marshal(object)
	if err != nil {
		return ConnectionSummary{}, fmt.Errorf("failed to decode the connection response: %w", err)
	}
	var connection ConnectionSummary
	if err := json.Unmarshal(flattened, &connection); err != nil {
		return ConnectionSummary{}, fmt.Errorf("failed to decode the connection response: %w", err)
	}
	return connection, nil
}

// RedactSecret withholds an API error's detail when it echoes secret.
//
// [APIError] falls back to the whole response body when it finds no `detail`
// it can humanize, and the top-level handler prints the entire error chain, so
// a server that echoed its input would print the secret.
//
// A credential can also be echoed without its wrapper (a `Bearer <token>`
// header value rejected as `invalid token: <token>`), so the header test
// strips a `scheme ` prefix as well as matching the whole value.
func RedactSecret(err error, secret string) error {
	var api *APIError
	if !errors.As(err, &api) {
		return err
	}
	detail := strings.TrimLeftFunc(api.Detail, unicode.IsSpace)
	isRawBody := strings.HasPrefix(detail, "{") || strings.HasPrefix(detail, "[")
	if isRawBody || detailEchoesSecret(api.Detail, secret) {
		// Rebuilt rather than replaced, so errs.CategoryOf can still read the
		// status off it.
		return &APIError{
			Status: api.Status,
			Detail: "the response was withheld because it echoed the value",
		}
	}
	return err
}

// detailEchoesSecret reports whether an error detail repeats a secret, either
// whole or after one scheme token such as `Bearer` / `Basic` / `token`.
func detailEchoesSecret(detail, secret string) bool {
	if strings.Contains(detail, secret) {
		return true
	}
	scheme, credential, ok := strings.Cut(secret, " ")
	if !ok || strings.Contains(scheme, "=") {
		return false
	}
	return strings.Contains(detail, credential)
}

// redactCreateSecrets redacts every write-only value a connection create
// carries: the client secret and each token request header value. Any one of
// them failing the same echo test withholds the response.
func redactCreateSecrets(err error, clientSecret string, options CreateOAuthConnection) error {
	if clientSecret != "" {
		err = RedactSecret(err, clientSecret)
	}
	for _, value := range options.TokenRequestHeaders {
		if value, ok := value.(string); ok {
			err = RedactSecret(err, value)
		}
	}
	return err
}
