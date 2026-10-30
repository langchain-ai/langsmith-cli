package hostapi

import (
	"slices"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

const conn = "77777777-7777-7777-7777-777777777777"

// selfHostedClientFor is a client for a self-hosted installation at
// platformAPIURL under basePath.
func selfHostedClientFor(t *testing.T, platformAPIURL, basePath string) *Client {
	t.Helper()
	basePath = strings.Trim(basePath, "/")
	if basePath != "" {
		platformAPIURL += "/" + basePath
	}
	return newClient(t, apiKeyCLIClient(t, "key", "tenant"), platformAPIURL+"/api-host", platformAPIURL)
}

func workspaceConnectionJSON(slug string) string {
	return `{"id":"` + conn + `","slug":"` + slug + `","display_name":"` + slug + `","credential":{"kind":"secret"},"created_at":"2026-08-27T00:00:00Z","updated_at":"2026-08-27T00:00:00Z"}`
}

func oauthConnectionJSON(slug, providerID string) string {
	return `{"id":"` + conn + `","slug":"` + slug + `","display_name":"` + slug + `","credential":{"kind":"oauth2","oauth_provider_id":"` + providerID + `"},"created_at":"2026-08-27T00:00:00Z","updated_at":"2026-08-27T00:00:00Z"}`
}

func TestSelfHostedConnectionsUseThePublicAPIPrefix(t *testing.T) {
	server := serveOnce(t, 200, `{"items":[]}`)
	found, err := selfHostedClientFor(t, server.URL, "").ListWorkspaceConnections(t.Context())
	if err != nil || len(found) != 0 {
		t.Fatalf("found = %v, %v", found, err)
	}
	if line := server.Only(t).Line(); line != "GET /api/v1/agent-auth/connections?page_size=100" {
		t.Fatalf("request = %s", line)
	}
}

func TestSelfHostedAgentAuthPreservesTheFrontendBasePath(t *testing.T) {
	uri, err := selfHostedClientFor(t, "https://langsmith.example.com", "langsmith").OAuthRedirectURI()
	if err != nil || uri != "https://langsmith.example.com/langsmith/api/v1/agent-auth/oauth/callback" {
		t.Fatalf("OAuthRedirectURI = %q, %v", uri, err)
	}
}

func TestSaaSAgentAuthKeepsTheDirectPlatformRoute(t *testing.T) {
	uri, err := testClient(t, "https://api.smith.langchain.com").OAuthRedirectURI()
	if err != nil || uri != "https://api.smith.langchain.com/v1/agent-auth/oauth/callback" {
		t.Fatalf("OAuthRedirectURI = %q, %v", uri, err)
	}
}

func TestCreateAgentConnectionForExistingDeploymentPostsAgentOwner(t *testing.T) {
	server := serveOnce(t, 201, workspaceConnectionJSON("my-tavily-key"))

	created, err := testClient(t, server.URL).CreateWorkspaceConnection(t.Context(), "my-tavily-key", ConnectionOwnerPolicyAgent, "11111111-1111-4111-8111-111111111111", "s3cr3t")

	if err != nil || created.Slug != "my-tavily-key" || created.Kind != "secret" || created.OwnerPolicy != "agent" || created.ConnectionID != conn {
		t.Fatalf("created = %+v, %v", created, err)
	}
	request := server.Only(t)
	want := `{"slug":"my-tavily-key","display_name":"my-tavily-key","credential":{"kind":"secret","owner_type":"agent","owner_id":"11111111-1111-4111-8111-111111111111","value":"s3cr3t"}}`
	if request.Line() != "POST /v1/agent-auth/connections" || !jsonEqual(t, request.JSON(t), want) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func TestCreateAgentConnectionRequiresADeploymentID(t *testing.T) {
	_, err := testClient(t, "http://unused.invalid").CreateWorkspaceConnection(t.Context(), "my-tavily-key", ConnectionOwnerPolicyAgent, "", "s3cr3t")
	if err == nil || err.Error() != "a deployment id is required to create an agent-owned opaque connection" {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateUserOpaqueSlotPostsIncompleteSecretCredential(t *testing.T) {
	server := serveOnce(t, 201, workspaceConnectionJSON("tavily-user"))
	created, err := testClient(t, server.URL).CreateWorkspaceConnection(t.Context(), "tavily-user", ConnectionOwnerPolicyUser, "", "")
	if err != nil || created.OwnerPolicy != "user" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	if !jsonEqual(t, server.Only(t).JSON(t), `{"slug":"tavily-user","display_name":"tavily-user","credential":{"kind":"secret"}}`) {
		t.Fatalf("body = %s", server.Only(t).Body)
	}
}

func TestListWorkspaceConnectionsReadsConnectionsAPI(t *testing.T) {
	server := serveOnce(t, 200, `{"items":[],"next_cursor":null}`)
	found, err := testClient(t, server.URL).ListWorkspaceConnections(t.Context())
	if err != nil || len(found) != 0 {
		t.Fatalf("found = %v, %v", found, err)
	}
	if line := server.Only(t).Line(); line != "GET /v1/agent-auth/connections?page_size=100" {
		t.Fatalf("request = %s", line)
	}
}

func TestConnectionHasAgentCredentialReadsTheOwnerFilteredConnection(t *testing.T) {
	body := strings.Replace(workspaceConnectionJSON("my-tavily-key"), `{"kind":"secret"}`, `{"kind":"secret","credential_id":"`+conn+`"}`, 1)
	server := serveOnce(t, 200, body)

	owned, err := testClient(t, server.URL).ConnectionHasAgentCredential(t.Context(), conn, "11111111-1111-4111-8111-111111111111")

	if err != nil || !owned {
		t.Fatalf("owned = %v, %v", owned, err)
	}
	if line := server.Only(t).Line(); line != "GET /v1/agent-auth/connections/"+conn+"?credential_owner_type=agent&credential_owner_id=11111111-1111-4111-8111-111111111111" {
		t.Fatalf("request = %s", line)
	}
}

func TestGrantStatusesMapToWhatSetupDoesWithThem(t *testing.T) {
	for _, tt := range []struct {
		status  string
		present bool
		want    AgentGrant
	}{
		{"", false, AgentGrant{Kind: AgentGrantActive}},
		{"active", true, AgentGrant{Kind: AgentGrantActive}},
		{"authorization_required", true, AgentGrant{Kind: AgentGrantAuthorizationRequired}},
		// A revoke keeps the app and asks for consent again; it is not missing.
		{"revocation_pending", true, AgentGrant{Kind: AgentGrantRevoked}},
		{"something_new", true, AgentGrant{Kind: AgentGrantUnknown, Status: "something_new"}},
	} {
		if got := agentGrantFromStatus(tt.status, tt.present); got != tt.want {
			t.Errorf("agentGrantFromStatus(%q, %v) = %+v, want %+v", tt.status, tt.present, got, tt.want)
		}
	}
}

func TestAgentOAuthGrantReadsTheCredentialsStatusNeverItsToken(t *testing.T) {
	const credential = "88888888-8888-4888-8888-888888888888"
	const agent = "11111111-1111-4111-8111-111111111111"
	connection := strings.Replace(workspaceConnectionJSON("mda/linear-app/x"), `{"kind":"secret"}`, `{"kind":"secret","credential_id":"`+credential+`"}`, 1)
	metadata := func(status string) string {
		return `{"credential_id":"` + credential + `","kind":"oauth2","oauth2":{"status":"` + status + `","token_type":"Bearer","scopes":["read"]}}`
	}

	server := serveSequence(t, reply{status: 200, body: connection}, reply{status: 200, body: metadata("active")})
	grant, err := testClient(t, server.URL).AgentOAuthGrant(t.Context(), conn, agent)
	if err != nil || grant != (AgentGrant{Kind: AgentGrantActive}) {
		t.Fatalf("grant = %+v, %v", grant, err)
	}
	requests := server.Requests()
	if strings.Join(requests[0].Query(t)["credential_owner_id"], ",") != agent || requests[1].Line() != "GET /v1/agent-auth/credentials/"+credential {
		t.Fatalf("requests = %s, %s", requests[0].Line(), requests[1].Line())
	}

	for status, want := range map[string]AgentGrant{
		// A lapsed grant keeps its credential, marked for consent.
		"authorization_required": {Kind: AgentGrantAuthorizationRequired},
		"revocation_pending":     {Kind: AgentGrantRevoked},
		// A status this CLI does not know is named, not taken as active.
		"suspended": {Kind: AgentGrantUnknown, Status: "suspended"},
	} {
		server := serveSequence(t, reply{status: 200, body: connection}, reply{status: 200, body: metadata(status)})
		if grant, err := testClient(t, server.URL).AgentOAuthGrant(t.Context(), conn, agent); err != nil || grant != want {
			t.Errorf("%s: grant = %+v, %v", status, grant, err)
		}
	}

	// No credential for the agent at all.
	grant, err = testClient(t, serveOnce(t, 200, workspaceConnectionJSON("mda/linear-app/x")).URL).AgentOAuthGrant(t.Context(), conn, agent)
	if err != nil || grant != (AgentGrant{Kind: AgentGrantMissing}) {
		t.Fatalf("grant = %+v, %v", grant, err)
	}
}

func TestPublishedOAuthRedirectURIReadsAgentAuthsClientMetadata(t *testing.T) {
	server := serveOnce(t, 200, `{"client_id":"https://api.smith.example/v1/agent-auth/oauth/client-metadata.json","redirect_uris":["https://api.smith.example/v1/agent-auth/oauth/callback"]}`)
	uri, ok, err := testClient(t, server.URL).PublishedOAuthRedirectURI(t.Context())
	if err != nil || !ok || uri != "https://api.smith.example/v1/agent-auth/oauth/callback" {
		t.Fatalf("uri = %q, %v, %v", uri, ok, err)
	}
	if line := server.Only(t).Line(); line != "GET /v1/agent-auth/oauth/client-metadata.json" {
		t.Fatalf("request = %s", line)
	}
	// Not published, e.g. for an http:// callback.
	uri, ok, err = testClient(t, serveOnce(t, 404, "{}").URL).PublishedOAuthRedirectURI(t.Context())
	if err != nil || ok || uri != "" {
		t.Fatalf("uri = %q, %v, %v; want none", uri, ok, err)
	}
}

func TestAConnectionWithoutACredentialForTheOwnerIsNotOwned(t *testing.T) {
	owned, err := testClient(t, serveOnce(t, 200, workspaceConnectionJSON("another-projects-key")).URL).ConnectionHasAgentCredential(t.Context(), conn, "11111111-1111-4111-8111-111111111111")
	if err != nil || owned {
		t.Fatalf("owned = %v, %v", owned, err)
	}
}

func TestAMissingConnectionIsNotOwned(t *testing.T) {
	owned, err := testClient(t, serveOnce(t, 404, `{"detail":"not found"}`).URL).ConnectionHasAgentCredential(t.Context(), conn, "11111111-1111-4111-8111-111111111111")
	if err != nil || owned {
		t.Fatalf("owned = %v, %v", owned, err)
	}
}

func TestListWorkspaceConnectionsFollowsNextCursor(t *testing.T) {
	server := serveSequence(t,
		reply{status: 200, body: `{"items":[` + workspaceConnectionJSON("alpha") + `],"next_cursor":"page-2"}`},
		reply{status: 200, body: `{"items":[` + workspaceConnectionJSON("beta") + `],"next_cursor":null}`},
	)

	found, err := testClient(t, server.URL).ListWorkspaceConnections(t.Context())

	if err != nil || len(found) != 2 || found[0].Slug != "alpha" || found[1].Slug != "beta" || found[0].OwnerPolicy != "" {
		t.Fatalf("found = %+v, %v", found, err)
	}
	requests := server.Requests()
	if requests[0].Line() != "GET /v1/agent-auth/connections?page_size=100" || requests[1].Line() != "GET /v1/agent-auth/connections?page_size=100&cursor=page-2" {
		t.Fatalf("requests = %s, %s", requests[0].Line(), requests[1].Line())
	}
}

func TestListConnectionsRefusesARepeatedCursor(t *testing.T) {
	page := `{"items":[],"next_cursor":"same"}`
	_, err := testClient(t, serveSequence(t, reply{status: 200, body: page}, reply{status: 200, body: page}).URL).ListConnectionsRaw(t.Context())
	if err == nil || err.Error() != "connection list returned a repeated cursor" {
		t.Fatalf("err = %v", err)
	}
}

func TestGitHubAppProviderIDsMatchTheAppID(t *testing.T) {
	server := serveOnce(t, 200, `{"items":[
		{"id":"p-1","slug":"mda-github-app-dep-1","client_id":"42","grant_type":"github_app"},
		{"id":"p-2","slug":"mda-github-app-dep-2","client_id":"43","grant_type":"github_app"},
		{"id":"p-3","slug":"notion","client_id":"42","grant_type":"authorization_code"}],"next_cursor":null}`)
	ids, err := testClient(t, server.URL).GitHubAppProviderIDs(t.Context(), "42")
	if err != nil || !slices.Equal(ids, []string{"p-1"}) {
		t.Fatalf("ids = %v, %v", ids, err)
	}
}

func linearOAuth() CreateOAuthConnection {
	return CreateOAuthConnection{
		Slug:                    "mda/linear-app/dep-1",
		OwnerPolicy:             ConnectionOwnerPolicyAgent,
		ClientID:                "client-id",
		ClientSecret:            "client-secret",
		Scopes:                  []string{"read"},
		AuthorizeURL:            "https://linear.app/oauth/authorize",
		TokenURL:                "https://api.linear.app/oauth/token",
		TokenEndpointAuthMethod: "client_secret_post",
		GrantType:               "authorization_code",
		DisplayName:             "Linear",
		AllowedScopes:           []string{"read"},
	}
}

func TestCreateOAuthWorkspaceConnectionNamesANamespacedProvider(t *testing.T) {
	server := serveOnce(t, 201, oauthConnectionJSON("mda/linear-app/dep-1", "11111111-1111-1111-1111-111111111111"))
	if _, err := testClient(t, server.URL).CreateOAuthWorkspaceConnection(t.Context(), linearOAuth()); err != nil {
		t.Fatal(err)
	}
	body := server.Only(t).JSON(t)
	if field(body, "slug") != "mda/linear-app/dep-1" || field(field(field(body, "credential"), "provider"), "slug") != "mda-linear-app-dep-1" {
		t.Fatalf("body = %s", mustJSON(t, body))
	}
}

func TestCreateOAuthWorkspaceConnectionPostsNestedProvider(t *testing.T) {
	providerID := "11111111-1111-1111-1111-111111111111"
	server := serveOnce(t, 201, oauthConnectionJSON("gtm-readonly", providerID))

	created, err := testClient(t, server.URL).CreateOAuthWorkspaceConnection(t.Context(), CreateOAuthConnection{
		Slug:                    "gtm-readonly",
		OwnerPolicy:             ConnectionOwnerPolicyAgent,
		Service:                 "google",
		ClientID:                "client-id",
		ClientSecret:            "client-secret",
		Scopes:                  []string{"openid"},
		AuthorizeURL:            "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:                "https://oauth2.googleapis.com/token",
		TokenEndpointAuthMethod: "client_secret_post",
		AuthorizationParams:     map[string]any{"access_type": "offline"},
		GrantType:               "authorization_code",
		TokenRequestHeaders:     map[string]any{"Authorization": "Bearer publishable-key"},
		DisplayName:             "Google",
		AllowedScopes:           []string{"openid", "email"},
	})

	if err != nil || created.Kind != "oauth2" || created.OAuthProviderID == nil || *created.OAuthProviderID != providerID || created.OAuthAppSlug != "" || created.Service != "google" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	request := server.Only(t)
	want := `{"slug":"gtm-readonly","display_name":"gtm-readonly","credential":{"kind":"oauth2","provider":{
		"display_name":"Google","client_id":"client-id","client_secret":"client-secret",
		"authorization_url":"https://accounts.google.com/o/oauth2/v2/auth","token_url":"https://oauth2.googleapis.com/token",
		"token_endpoint_auth_method":"client_secret_post","grant_type":"authorization_code",
		"authorization_params":{"access_type":"offline"},"token_params":{},
		"token_request_headers":{"Authorization":"Bearer publishable-key"},
		"default_scopes":["openid"],"allowed_scopes":["openid","email"]}}}`
	if request.Line() != "POST /v1/agent-auth/connections" || !jsonEqual(t, request.JSON(t), want) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func spotify(secret string) CreateOAuthConnection {
	return CreateOAuthConnection{
		Slug:                    "spotify",
		OwnerPolicy:             ConnectionOwnerPolicyAgent,
		Service:                 "spotify",
		ClientID:                "client-id",
		ClientSecret:            secret,
		DisplayName:             "Spotify",
		TokenURL:                "https://accounts.spotify.com/api/token",
		TokenEndpointAuthMethod: "client_secret_basic",
		GrantType:               "client_credentials",
	}
}

func TestCreateClientCredentialsConnectionPostsTokenOnlyProvider(t *testing.T) {
	server := serveOnce(t, 201, oauthConnectionJSON("spotify", "11111111-1111-1111-1111-111111111111"))
	options := spotify("fixture-secret")
	options.Scopes = []string{"playlist-read"}
	options.TokenParams = map[string]any{"audience": "music-api"}
	options.TokenRequestHeaders = map[string]any{"X-Provider-Version": "2024-01"}

	if _, err := testClient(t, server.URL).CreateOAuthWorkspaceConnection(t.Context(), options); err != nil {
		t.Fatal(err)
	}

	want := `{"slug":"spotify","display_name":"spotify","credential":{"kind":"oauth2","provider":{
		"display_name":"Spotify","client_id":"client-id","client_secret":"fixture-secret",
		"token_url":"https://accounts.spotify.com/api/token","token_endpoint_auth_method":"client_secret_basic",
		"grant_type":"client_credentials","token_params":{"audience":"music-api"},
		"token_request_headers":{"X-Provider-Version":"2024-01"},
		"default_scopes":["playlist-read"],"allowed_scopes":["playlist-read"]}}}`
	if !jsonEqual(t, server.Only(t).JSON(t), want) {
		t.Fatalf("body = %s", server.Only(t).Body)
	}
}

func TestClientCredentialsProviderAllowsEmptyScopes(t *testing.T) {
	provider, err := oauthProviderCreateBody(spotify("fixture-secret"))
	if err != nil || !jsonEqual(t, provider["default_scopes"], `[]`) || !jsonEqual(t, provider["allowed_scopes"], `[]`) {
		t.Fatalf("provider = %v, %v", provider, err)
	}
}

func TestCreateOAuthRedactsHeaderValuesFromAPIErrors(t *testing.T) {
	for _, tt := range []struct{ body, header, secret string }{
		{`{"detail": "invalid header value: Bearer pk_live_secret"}`, "Bearer pk_live_secret", "pk_live_secret"},
		// Echoed without its scheme.
		{`{"detail": "invalid token: sk_live_partial"}`, "Bearer sk_live_partial", "sk_live_partial"},
		// Echoed raw, not even JSON.
		{"Bearer sk_live_raw not even json", "sk_live_raw", "sk_live_raw"},
	} {
		options := spotify("client-secret")
		options.TokenRequestHeaders = map[string]any{"Authorization": tt.header}

		_, err := testClient(t, serveOnce(t, 400, tt.body).URL).CreateOAuthWorkspaceConnection(t.Context(), options)

		if err == nil || strings.Contains(err.Error(), tt.secret) || !strings.Contains(err.Error(), "withheld") {
			t.Errorf("%s: err = %v", tt.body, err)
		}
	}
}

func TestClientCredentialsProviderValidation(t *testing.T) {
	noSecret := spotify("")
	publicClient := spotify("fixture-secret")
	publicClient.TokenEndpointAuthMethod = "none"
	noTokenURL := spotify("fixture-secret")
	noTokenURL.TokenURL = ""
	for options, want := range map[*CreateOAuthConnection]string{
		&noSecret:     "OAuth client credentials requires a client secret.",
		&publicClient: "OAuth client credentials requires client_secret_basic or client_secret_post token endpoint authentication.",
		&noTokenURL:   "This OAuth connection needs a token endpoint. Name a catalog service with --oauth, or pass --token-url.",
	} {
		_, err := oauthProviderCreateBody(*options)
		if err == nil || err.Error() != want || errs.CategoryOf(err) != errs.UserInput {
			t.Errorf("err = %v, want user input error %q", err, want)
		}
	}
}

func TestCreateAgentOAuthAuthorizationSessionPostsAgentScope(t *testing.T) {
	server := serveOnce(t, 201, `{"id":"88888888-8888-8888-8888-888888888888","status":"pending","verification_url":"https://accounts.example.com/authorize?state=session"}`)
	session, err := testClient(t, server.URL).CreateAgentOAuthAuthorizationSession(t.Context(), conn, "agent-id")
	if err != nil || session.VerificationURL != "https://accounts.example.com/authorize?state=session" {
		t.Fatalf("session = %+v, %v", session, err)
	}
	if !jsonEqual(t, server.Only(t).JSON(t), `{"connection_id":"`+conn+`","owner_type":"agent","owner_id":"agent-id"}`) {
		t.Fatalf("body = %s", server.Only(t).Body)
	}
}

func TestCompletedAuthorizationSessionMayOmitVerificationURL(t *testing.T) {
	server := serveOnce(t, 201, `{"id":"88888888-8888-8888-8888-888888888888","status":"completed","credential_id":"99999999-9999-9999-9999-999999999999"}`)
	session, err := testClient(t, server.URL).CreateAgentOAuthAuthorizationSession(t.Context(), conn, "agent-id")
	if err != nil || session.Status != "completed" || session.VerificationURL != "" {
		t.Fatalf("session = %+v, %v", session, err)
	}
}

func TestPendingAuthorizationSessionRequiresVerificationURL(t *testing.T) {
	server := serveOnce(t, 201, `{"id":"88888888-8888-8888-8888-888888888888","status":"pending"}`)
	_, err := testClient(t, server.URL).CreateAgentOAuthAuthorizationSession(t.Context(), conn, "agent-id")
	if err == nil || err.Error() != "OAuth authorization session response missing verification_url" {
		t.Fatalf("err = %v", err)
	}
}

func TestGetOAuthAuthorizationSessionLongPollsForCompletion(t *testing.T) {
	server := serveOnce(t, 200, `{"id":"88888888-8888-8888-8888-888888888888","status":"completed","credential_id":"99999999-9999-9999-9999-999999999999"}`)
	session, err := testClient(t, server.URL).GetOAuthAuthorizationSession(t.Context(), "88888888-8888-8888-8888-888888888888", 25)
	if err != nil || session.Status != "completed" {
		t.Fatalf("session = %+v, %v", session, err)
	}
	if line := server.Only(t).Line(); line != "GET /v1/agent-auth/oauth-authorization-sessions/88888888-8888-8888-8888-888888888888?wait_seconds=25" {
		t.Fatalf("request = %s", line)
	}
}

func TestNonUUIDIDsDoNotChangeTheRequestPath(t *testing.T) {
	server := serveOnce(t, 404, `{}`)
	if _, err := testClient(t, server.URL).DeleteConnection(t.Context(), "../../workspaces"); err != nil {
		t.Fatal(err)
	}
	if line := server.Only(t).Line(); line != "DELETE /v1/agent-auth/connections/invalid" {
		t.Fatalf("request = %s", line)
	}
}

func TestCreateOAuthWithoutClientIDFailsClosed(t *testing.T) {
	_, err := testClient(t, "http://unused.invalid").CreateOAuthWorkspaceConnection(t.Context(), CreateOAuthConnection{
		Slug:                    "notion-docs",
		OwnerPolicy:             ConnectionOwnerPolicyUser,
		Service:                 "notion",
		DisplayName:             "Notion",
		AuthorizeURL:            "https://api.notion.com/v1/oauth/authorize",
		TokenURL:                "https://api.notion.com/v1/oauth/token",
		TokenEndpointAuthMethod: "client_secret_basic",
		GrantType:               "authorization_code",
		ClientRegistrationURL:   "https://www.notion.so/my-integrations",
	})
	want := "This OAuth connection needs --client-id (and usually --secret-from-env for the client secret). Register an app at https://www.notion.so/my-integrations, then pass the credentials."
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v", err)
	}
}

func TestRegisteringWithoutKnownEndpointsFailsClosed(t *testing.T) {
	_, err := testClient(t, "http://unused.invalid").CreateOAuthWorkspaceConnection(t.Context(), CreateOAuthConnection{
		Slug:                    "acme-api",
		OwnerPolicy:             ConnectionOwnerPolicyUser,
		ClientID:                "client-id",
		ClientSecret:            "client-secret",
		DisplayName:             "acme-oauth",
		TokenEndpointAuthMethod: "client_secret_basic",
		GrantType:               "authorization_code",
	})
	if err == nil || err.Error() != "This OAuth connection needs endpoints. Name a catalog service with --oauth, or pass --authorize-url and --token-url." {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateOAuthMCPDCRPostsMCPServerURL(t *testing.T) {
	providerID := "11111111-1111-1111-1111-111111111111"
	server := serveOnce(t, 201, oauthConnectionJSON("notion-mcp", providerID))

	created, err := testClient(t, server.URL).CreateOAuthWorkspaceConnection(t.Context(), CreateOAuthConnection{
		Slug:                    "notion-mcp",
		OwnerPolicy:             ConnectionOwnerPolicyUser,
		Service:                 "notion",
		DisplayName:             "Notion MCP",
		TokenEndpointAuthMethod: "none",
		GrantType:               "authorization_code",
		MCPServerURL:            "https://mcp.notion.com/mcp",
	})

	if err != nil || created.OAuthProviderID == nil || *created.OAuthProviderID != providerID {
		t.Fatalf("created = %+v, %v", created, err)
	}
	want := `{"slug":"notion-mcp","display_name":"notion-mcp","credential":{"kind":"oauth2","provider":{"display_name":"Notion MCP","mcp_server_url":"https://mcp.notion.com/mcp"}}}`
	if !jsonEqual(t, server.Only(t).JSON(t), want) {
		t.Fatalf("body = %s", server.Only(t).Body)
	}
}

func TestListWorkspaceConnectionsEnrichesOAuthAppSlug(t *testing.T) {
	providerID := "11111111-1111-1111-1111-111111111111"
	server := serveSequence(t,
		reply{status: 200, body: `{"items":[` + oauthConnectionJSON("notion-docs", providerID) + `],"next_cursor":null}`},
		reply{status: 200, body: `{"items":[{"id":"` + providerID + `","slug":"notion"}],"next_cursor":null}`},
	)

	found, err := testClient(t, server.URL).ListWorkspaceConnections(t.Context())

	if err != nil || len(found) != 1 || found[0].Kind != "oauth2" || found[0].OAuthAppSlug != "notion" {
		t.Fatalf("found = %+v, %v", found, err)
	}
	requests := server.Requests()
	if !strings.HasPrefix(requests[0].Line(), "GET /v1/agent-auth/connections") || !strings.HasPrefix(requests[1].Line(), "GET /v1/agent-auth/oauth-providers") {
		t.Fatalf("requests = %s, %s", requests[0].Line(), requests[1].Line())
	}
}

func TestAResponseThatEchoesTheSecretIsNotPrinted(t *testing.T) {
	secret := strings.Join([]string{"fixture", "value"}, "-")
	// Items with no `msg` are the shape humanizeErrorBody gives up on, which
	// makes APIError fall back to the entire raw response body.
	body := `{"detail":[{"loc":["body","secret"],"type":"string_too_long","input":"` + secret + `"}]}`

	_, err := testClient(t, serveOnce(t, 422, body).URL).CreateWorkspaceConnection(t.Context(), "openai-key", ConnectionOwnerPolicyAgent, "11111111-1111-4111-8111-111111111111", secret)

	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "422") {
		t.Fatalf("err = %v", err)
	}
	// Redacting must not cost the status, which is what classifies the failure.
	if errs.CategoryOf(err) != errs.Remote {
		t.Fatalf("category = %v, want remote", errs.CategoryOf(err))
	}
}

func TestClientCredentialsResponseCannotEchoTheClientSecret(t *testing.T) {
	secret := strings.Join([]string{"oauth-fixture", "value"}, "-")
	_, err := testClient(t, serveOnce(t, 422, `{"detail":[{"input":"`+secret+`"}]}`).URL).CreateOAuthWorkspaceConnection(t.Context(), spotify(secret))
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "422") {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectionSummaryReadsItsIDAliases(t *testing.T) {
	for _, body := range []string{
		`{"connection_id":"c1","slug":"s","kind":"secret"}`,
		`{"credential_id":"c1","slug":"s","kind":"secret"}`,
		`{"id":"c1","slug":"s","kind":"secret"}`,
	} {
		connection, err := decodeConnection([]byte(body))
		if err != nil || connection.ConnectionID != "c1" {
			t.Errorf("%s: connection = %+v, %v", body, connection, err)
		}
	}
	if _, err := decodeConnection([]byte(`{"slug":"s","kind":"secret"}`)); err == nil {
		t.Error("a connection without an id decoded")
	}
	if _, err := decodeConnection([]byte(`{"id":"c1","slug":"s"}`)); err == nil || err.Error() != "connection response missing credential.kind" {
		t.Errorf("err = %v", err)
	}
	encoded := mustJSON(t, ConnectionSummary{ConnectionID: "c1", Slug: "s", Kind: "secret"})
	if encoded != `{"connection_id":"c1","slug":"s","display_name":"","kind":"secret","oauth_provider_id":null,"created_at":"","updated_at":""}` {
		t.Errorf("serialized = %s", encoded)
	}
}
