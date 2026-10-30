package hostapi

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// appTestClient is a client for a stub Agent Auth at base, with the API key
// "lsv2_key" in the workspace "tenant-1".
func appTestClient(t *testing.T, base string) *Client {
	t.Helper()
	return newClient(t, apiKeyCLIClient(t, "lsv2_key", "tenant-1"), base, base)
}

func manifest() GitHubAppManifest {
	return GitHubAppManifest{
		Name:        "acme-triage",
		URL:         "https://agent.example",
		WebhookURL:  "https://agent.example/channels/github/events",
		Permissions: map[string]string{"issues": "write"},
		Events:      []string{"issues"},
	}
}

func appJSON(source, appID string) string {
	app := map[string]any{
		"id": "app-1", "provider": "github", "source": source, "slug": "mda/github-app/x",
		"app_id": nil, "connection_id": nil, "app_metadata": nil,
		"manifest": map[string]any{"webhook_url": "https://agent.example/channels/github/events"},
	}
	if appID != "" {
		app["app_id"] = appID
		app["connection_id"] = "conn-1"
		app["app_metadata"] = map[string]any{"slug": "acme-triage", "html_url": "https://github.com/apps/acme-triage", "bot_id": "9"}
	}
	data, _ := json.Marshal(app)
	return string(data)
}

func app(t *testing.T, source, appID string) AgentAuthApp {
	t.Helper()
	var decoded AgentAuthApp
	if err := json.Unmarshal([]byte(appJSON(source, appID)), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestANewAppCarriesTheManifestAndReturnsItsSession(t *testing.T) {
	server := serveOnce(t, 201, `{"app":{"id":"app-1","source":"manifest","slug":"mda/github-app/x"},
		"session":{"id":"11111111-2222-3333-4444-555555555555","status":"pending","verification_url":"https://aa.example/v1/agent-auth/github-app/start?state=s"}}`)

	created, err := appTestClient(t, server.URL).CreateGitHubApp(t.Context(), "agent-1", "mda/github-app/x", manifest(), "acme")

	if err != nil || created.App.ID != "app-1" || created.App.State() != GitHubAppStatePending || created.Session.Status != "pending" {
		t.Fatalf("created = %+v, %v", created, err)
	}
	request := server.Only(t)
	want := `{"provider":"github","source":"manifest","owner_id":"agent-1","slug":"mda/github-app/x","organization":"acme",
		"manifest":{"name":"acme-triage","url":"https://agent.example","webhook_url":"https://agent.example/channels/github/events","permissions":{"issues":"write"},"events":["issues"]}}`
	if request.Line() != "POST /v1/agent-auth/apps" || !jsonEqual(t, request.JSON(t), want) {
		t.Fatalf("request = %s %s", request.Line(), request.Body)
	}
}

func TestANewAppWithoutASetupLinkIsAnError(t *testing.T) {
	server := serveOnce(t, 201, `{"app":{"id":"app-1","slug":"x"},"session":{"id":"s","status":"pending","verification_url":" "}}`)
	_, err := appTestClient(t, server.URL).CreateGitHubApp(t.Context(), "agent-1", "x", manifest(), "")
	if err == nil || err.Error() != "Agent Auth returned the GitHub App without a setup link" {
		t.Fatalf("err = %v", err)
	}
}

func TestAnExistingAppNeverLeaksItsKeyOrSecretInErrors(t *testing.T) {
	key := "-----BEGIN RSA PRIVATE KEY-----\nSECRETKEY\n-----END RSA PRIVATE KEY-----\n"
	server := serveOnce(t, 422, `{"detail":"GitHub rejected the App's credentials SECRETKEY whsec-123"}`)

	_, err := appTestClient(t, server.URL).BringGitHubApp(t.Context(), "agent-1", "mda/github-app/x", ExistingApp{AppID: "42", PrivateKey: key, WebhookSecret: "whsec-123"})

	if err == nil || strings.Contains(err.Error(), "SECRETKEY") || strings.Contains(err.Error(), "whsec-123") {
		t.Fatalf("err = %v", err)
	}
	want := `{"provider":"github","source":"existing","owner_id":"agent-1","slug":"mda/github-app/x","existing":{"app_id":"42","private_key":` + mustJSON(t, key) + `,"webhook_secret":"whsec-123"}}`
	if !jsonEqual(t, server.Only(t).JSON(t), want) {
		t.Fatalf("body = %s", server.Only(t).Body)
	}
}

func TestAppsAreListedByOwnerAndReadTheirState(t *testing.T) {
	stranded := strings.TrimSuffix(appJSON("manifest", ""), "}") +
		`,"stranded_apps":[{"slug":"acme-triage","html_url":"https://github.com/apps/acme-triage"},{"slug":"evil","html_url":"https://evil.example/apps/evil"}]}`
	server := serveOnce(t, 200, "["+appJSON("manifest", "42")+","+appJSON("existing", "43")+","+appJSON("manifest", "")+","+stranded+"]")

	apps, err := appTestClient(t, server.URL).AgentApps(t.Context(), "agent-1")
	if err != nil {
		t.Fatal(err)
	}

	var states []GitHubAppState
	for _, a := range apps {
		states = append(states, a.State())
	}
	if !slices.Equal(states, []GitHubAppState{GitHubAppStateCreated, GitHubAppStateExternal, GitHubAppStatePending, GitHubAppStatePending}) {
		t.Errorf("states = %v", states)
	}
	if url, ok := apps[0].HTMLURL(); !ok || url != "https://github.com/apps/acme-triage" {
		t.Errorf("HTMLURL = %q, %v", url, ok)
	}
	if _, ok := apps[2].HTMLURL(); ok || len(apps[2].StrandedURLs()) != 0 {
		t.Errorf("a pending app has pages")
	}
	if _, ok := apps[3].HTMLURL(); ok || !slices.Equal(apps[3].StrandedURLs(), []string{"https://github.com/apps/acme-triage"}) {
		t.Errorf("stranded = %v", apps[3].StrandedURLs())
	}
	if line := server.Only(t).Line(); line != "GET /v1/agent-auth/apps?owner_id=agent-1" {
		t.Errorf("request = %s", line)
	}

	_, err = appTestClient(t, serveOnce(t, 404, "{}").URL).AgentApps(t.Context(), "agent-1")
	if err == nil || err.Error() != "this LangSmith does not support GitHub Apps through Agent Auth yet" {
		t.Errorf("err = %v", err)
	}
	apps, supported, err := appTestClient(t, serveOnce(t, 404, "{}").URL).AgentAppsIfSupported(t.Context(), "agent-1")
	if err != nil || supported || apps != nil {
		t.Errorf("AgentAppsIfSupported = %v, %v, %v", apps, supported, err)
	}
}

func TestSessionsCreateThePendingAppOrInstallTheCreatedOne(t *testing.T) {
	pending := `{"id":"11111111-2222-3333-4444-555555555555","status":"pending","verification_url":"https://aa.example/start?state=s"}`
	for _, tt := range []struct {
		app   AgentAuthApp
		start GitHubAppSessionStart
		want  string
	}{
		{app(t, "manifest", ""), GitHubAppSessionStart{Kind: GitHubAppSessionStartCreate, Organization: "acme"},
			`{"agent_auth_app_id":"app-1","owner_type":"agent","owner_id":"agent-1","organization":"acme"}`},
		{app(t, "manifest", "42"), GitHubAppSessionStart{Kind: GitHubAppSessionStartInstall},
			`{"connection_id":"conn-1","owner_type":"agent","owner_id":"agent-1"}`},
		{app(t, "existing", "42"), GitHubAppSessionStart{Kind: GitHubAppSessionStartInstallation, InstallationID: "777"},
			`{"connection_id":"conn-1","owner_type":"agent","owner_id":"agent-1","installation_id":"777"}`},
	} {
		server := serveOnce(t, 201, pending)
		session, err := appTestClient(t, server.URL).StartGitHubAppSession(t.Context(), tt.app, "agent-1", tt.start)
		if err != nil || session.Status != "pending" {
			t.Fatalf("session = %+v, %v", session, err)
		}
		request := server.Only(t)
		if request.Line() != "POST /v1/agent-auth/oauth-authorization-sessions" || !jsonEqual(t, request.JSON(t), tt.want) {
			t.Errorf("request = %s %s, want %s", request.Line(), request.Body, tt.want)
		}
	}
	_, err := appTestClient(t, "http://unused.invalid").StartGitHubAppSession(t.Context(), app(t, "manifest", ""), "agent-1", GitHubAppSessionStart{Kind: GitHubAppSessionStartInstall})
	if err == nil || err.Error() != "the GitHub App has no connection yet" {
		t.Errorf("err = %v", err)
	}
}

func TestScopesAreReadAndChangedOnTheProvider(t *testing.T) {
	const provider = "11111111-2222-3333-4444-555555555555"
	server := serveSequence(t, reply{status: 200, body: `{"allowed_scopes":["metadata:read","issues:write"]}`}, reply{status: 200, body: "{}"})
	c := appTestClient(t, server.URL)

	scopes, err := c.GitHubAppScopes(t.Context(), provider)
	if err != nil || !slices.Equal(scopes, []string{"issues:write", "metadata:read"}) {
		t.Fatalf("scopes = %v, %v", scopes, err)
	}
	if err := c.UpdateGitHubAppScopes(t.Context(), provider, []string{"issues:read"}); err != nil {
		t.Fatal(err)
	}

	requests := server.Requests()
	if requests[0].Line() != "GET /v1/agent-auth/oauth-providers/"+provider {
		t.Errorf("read = %s", requests[0].Line())
	}
	if requests[1].Line() != "PATCH /v1/agent-auth/oauth-providers/"+provider ||
		!jsonEqual(t, requests[1].JSON(t), `{"allowed_scopes":["issues:read"],"default_scopes":["issues:read"]}`) {
		t.Errorf("update = %s %s", requests[1].Line(), requests[1].Body)
	}
}

func TestDeletingAGoneAppIsNotAnError(t *testing.T) {
	server := serveOnce(t, 404, "{}")
	deleted, err := appTestClient(t, server.URL).DeleteAgentApp(t.Context(), "app-1")
	if err != nil || deleted {
		t.Fatalf("DeleteAgentApp = %v, %v", deleted, err)
	}
	if line := server.Only(t).Line(); line != "DELETE /v1/agent-auth/apps/app-1" {
		t.Fatalf("request = %s", line)
	}
}
