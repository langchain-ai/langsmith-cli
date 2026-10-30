package hostapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

func TestPATServiceAndFailedLookupsReportOnlyVerifiedIDs(t *testing.T) {
	for _, tt := range []struct {
		status          int
		body            string
		user, workspace string
		ok              bool
	}{
		{200, `{"ls_user_id":"person","organization_id":"org","tenant_id":"workspace","user_email":"excluded"}`, "person", "workspace", true},
		{200, `{"user_id":"service","organization_id":"org","tenant_id":"workspace"}`, "", "workspace", true},
		{200, `{"ls_user_id":"person","organization_id":"org","tenant_id":"other"}`, "", "", false},
		{401, `{}`, "", "", false},
		{200, `not json`, "", "", false},
	} {
		server := serveOnce(t, tt.status, tt.body)
		c := newClient(t, apiKeyCLIClient(t, "test-key", "workspace"), server.URL, server.URL)

		identity, ok := c.AuthIdentity(t.Context())

		wantOrg := ""
		if tt.ok {
			wantOrg = "org"
		}
		if ok != tt.ok || identity.UserID != tt.user || identity.WorkspaceID != tt.workspace || identity.OrgID != wantOrg {
			t.Errorf("%s: identity = %+v, %v; want user %q, workspace %q, org %q, %v", tt.body, identity, ok, tt.user, tt.workspace, wantOrg, tt.ok)
		}
		request := server.Only(t)
		if request.Line() != "GET /auth/public" || request.Header.Get("X-Tenant-Id") != "workspace" {
			t.Errorf("request = %s with tenant %q, want GET /auth/public in workspace", request.Line(), request.Header.Get("X-Tenant-Id"))
		}
		serialized := mustJSON(t, identity)
		if strings.Contains(serialized, "excluded") || strings.Contains(serialized, "test-key") {
			t.Errorf("serialized identity leaks: %s", serialized)
		}
	}
}

func TestLookupTimeoutDoesNotFailTheCommand(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	c := newClient(t, apiKeyCLIClient(t, "test-key", ""), server.URL, server.URL).WithTimeout(time.Second)

	start := time.Now()
	_, ok := c.AuthIdentity(t.Context())

	if ok {
		t.Fatal("a lookup that timed out reported an identity")
	}
	if elapsed := time.Since(start); elapsed >= 1750*time.Millisecond {
		t.Fatalf("lookup took %s, want under 1.75s", elapsed)
	}
}

func TestTheKeyTravelsInCleartextOnlyToThisMachine(t *testing.T) {
	ls := apiKeyCLIClient(t, "key", "")
	for platform, want := range map[string]bool{
		"https://api.smith.langchain.com": true,
		"http://localhost:1980":           true,
		"http://127.0.0.1:1980":           true,
		"http://[::1]:1980":               true,
		"http://langsmith.example.com":    false,
	} {
		if got := newClient(t, ls, platform, platform).SendsKeySecurely(); got != want {
			t.Errorf("SendsKeySecurely(%s) = %v, want %v", platform, got, want)
		}
	}
}

func TestRequestsCarryTheOAuthProfileTokenAndWorkspace(t *testing.T) {
	server := serveOnce(t, 200, `{"resources":[]}`)
	c := newClient(t, oauthCLIClient(t, "oauth-access-token", "ws-1"), server.URL, server.URL)

	if _, err := c.ListDeployments(t.Context(), "agent"); err != nil {
		t.Fatal(err)
	}

	request := server.Only(t)
	if got := request.Header.Get("Authorization"); got != "Bearer oauth-access-token" {
		t.Errorf("Authorization = %q, want the profile's bearer token", got)
	}
	if got := request.Header.Get("X-Api-Key"); got != "" {
		t.Errorf("X-Api-Key = %q, want none", got)
	}
	if got := request.Header.Get("X-Tenant-Id"); got != "ws-1" {
		t.Errorf("X-Tenant-Id = %q, want ws-1", got)
	}
}

func noProcessEnv(string) (string, bool) { return "", false }

func TestResolvePreferredEnvPrefersCLIOverride(t *testing.T) {
	envFile := map[string]string{APIKeyEnv: "from-env"}
	got, ok, err := ResolvePreferredEnv(APIKeyEnv, RemovedAPIKeyEnvs, "from-cli", envFile, noProcessEnv, " instead")
	if err != nil || !ok || got != "from-cli" {
		t.Fatalf("got %q, %v, %v; want from-cli", got, ok, err)
	}
}

func TestResolvePreferredEnvProcessBeatsEnvFile(t *testing.T) {
	captureUI(t)
	envFile := map[string]string{APIKeyEnv: "from-dotenv"}
	process := map[string]string{APIKeyEnv: "from-process"}
	got, ok, err := ResolvePreferredEnv(APIKeyEnv, RemovedAPIKeyEnvs, "", envFile, func(key string) (string, bool) {
		value, ok := process[key]
		return value, ok
	}, " instead")
	if err != nil || !ok || got != "from-process" {
		t.Fatalf("got %q, %v, %v; want from-process", got, ok, err)
	}
}

func TestShellShadowsEnvFileOnlyOnARealDifference(t *testing.T) {
	out := captureUI(t)
	shadowWarningsMu.Lock()
	delete(shadowWarnings, "LANGSMITH_ENDPOINT")
	shadowWarningsMu.Unlock()
	envFile := map[string]string{
		"LANGSMITH_ENDPOINT": "https://eu.example/",
		"LANGSMITH_PROJECT":  "  ",
	}
	if !WarnIfShellShadowsEnvFile("LANGSMITH_ENDPOINT", "https://us.example", envFile) {
		t.Error("a different shell value does not shadow")
	}
	if WarnIfShellShadowsEnvFile("LANGSMITH_ENDPOINT", " https://eu.example ", envFile) {
		t.Error("a trailing slash and whitespace count as a difference")
	}
	// Blank or missing in `.env`: nothing is shadowed.
	if WarnIfShellShadowsEnvFile("LANGSMITH_PROJECT", "from-shell", envFile) {
		t.Error("a blank .env value is shadowed")
	}
	if WarnIfShellShadowsEnvFile("LANGSMITH_WORKSPACE_ID", "from-shell", envFile) {
		t.Error("a missing .env value is shadowed")
	}
	// The warning names the key, once.
	if !WarnIfShellShadowsEnvFile("LANGSMITH_ENDPOINT", "https://us.example", envFile) {
		t.Error("a repeated difference does not shadow")
	}
	want := "LANGSMITH_ENDPOINT is set in both your shell and .env with different values; using the shell value. Unset it in your shell to use the .env value."
	if got := out.String(); strings.Count(got, want) != 1 || strings.Count(got, "set in both") != 1 {
		t.Errorf("warnings = %q, want one for LANGSMITH_ENDPOINT", got)
	}
}

func TestLookupEnvFallsBackToEnvFileWhenProcessBlank(t *testing.T) {
	captureUI(t)
	envFile := map[string]string{APIKeyEnv: " from-dotenv "}
	value, origin, ok := LookupEnvWithOrigin(APIKeyEnv, envFile, func(string) (string, bool) { return "   ", true })
	if !ok || value != "from-dotenv" || origin != EnvOriginEnvFile {
		t.Errorf("got %q, %v, %v; want from-dotenv from the env file", value, origin, ok)
	}
	value, origin, ok = LookupEnvWithOrigin(APIKeyEnv, map[string]string{}, func(string) (string, bool) { return " from-shell ", true })
	if !ok || value != "from-shell" || origin != EnvOriginShell {
		t.Errorf("got %q, %v, %v; want from-shell from the shell", value, origin, ok)
	}
}

func TestResolvePreferredEnvAllowsRemovedAliasWhenPreferredSet(t *testing.T) {
	envFile := map[string]string{APIKeyEnv: " ls-key ", "LANGCHAIN_API_KEY": "legacy"}
	got, ok, err := ResolvePreferredEnv(APIKeyEnv, RemovedAPIKeyEnvs, "", envFile, noProcessEnv, " instead (same value, renamed)")
	if err != nil || !ok || got != "ls-key" {
		t.Fatalf("got %q, %v, %v; want ls-key", got, ok, err)
	}
}

func TestResolvePreferredEnvErrorsOnRemovedAlias(t *testing.T) {
	envFile := map[string]string{"LANGCHAIN_API_KEY": "legacy"}
	_, _, err := ResolvePreferredEnv(APIKeyEnv, RemovedAPIKeyEnvs, "", envFile, noProcessEnv, " instead (same value, renamed)")
	if err == nil || err.Error() != "LANGCHAIN_API_KEY is no longer supported; set LANGSMITH_API_KEY instead (same value, renamed)" {
		t.Fatalf("err = %v, want the removed alias error", err)
	}
}

func TestResolvePreferredEnvReturnsNoneWhenUnset(t *testing.T) {
	got, ok, err := ResolvePreferredEnv(APIKeyEnv, RemovedAPIKeyEnvs, "", map[string]string{}, noProcessEnv, " instead")
	if err != nil || ok || got != "" {
		t.Fatalf("got %q, %v, %v; want nothing", got, ok, err)
	}
}

func TestHumanizeErrorBody(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{`{"detail":[{"type":"missing","loc":["body","secrets"],"msg":"Field required"}]}`, "Missing required field 'secrets'"},
		{`{"detail":[{"type":"string_type","loc":["body","source_config","deployment_type"],"msg":"Input should be a valid string"}]}`,
			"Invalid value for 'source_config.deployment_type': Input should be a valid string"},
		{`{"detail":[{"type":"missing","loc":["body","name"],"msg":"Field required"},{"type":"missing","loc":["body","secrets"],"msg":"Field required"}]}`,
			"Missing required field 'name'; Missing required field 'secrets'"},
		{`{"detail":"Deployment not found"}`, "Deployment not found"},
		{`{"detail":{"error":"FeatureDisabled","message":"Sandbox feature is not enabled for this organization"}}`,
			"Sandbox feature is not enabled for this organization"},
		{`{"detail":[{"type":"too_long","loc":["body","items",0],"msg":"Too long"}]}`, "Invalid value for 'items.0': Too long"},
	} {
		got, ok := humanizeErrorBody([]byte(tt.body))
		if !ok || got != tt.want {
			t.Errorf("humanizeErrorBody(%s) = %q, %v; want %q", tt.body, got, ok, tt.want)
		}
	}
}

func TestReturnsNoneForUnrecognizedBody(t *testing.T) {
	for _, body := range []string{"not json", "{}"} {
		if got, ok := humanizeErrorBody([]byte(body)); ok {
			t.Errorf("humanizeErrorBody(%q) = %q, want none", body, got)
		}
	}
}

func TestAPIErrorsReadAsDetailThenStatus(t *testing.T) {
	err := apiErrorFromResponse(http.StatusForbidden, []byte(`{"detail":"Managed Deep Agents is not enabled for this organization"}`))
	if err.Error() != "Managed Deep Agents is not enabled for this organization (HTTP 403)" {
		t.Fatalf("err = %q", err)
	}
	if errs.CategoryOf(err) != errs.Permission {
		t.Fatalf("category = %v, want permission", errs.CategoryOf(err))
	}
}

func TestAPIErrorsKeepAnUnrecognizedBodyVerbatim(t *testing.T) {
	err := apiErrorFromResponse(http.StatusBadGateway, []byte("upstream exploded"))
	if err.Detail != "upstream exploded" || err.Error() != "upstream exploded (HTTP 502)" {
		t.Fatalf("err = %#v", err)
	}
}

func TestAPIErrorsStateTheStatusOnceWhenThereIsNoDetail(t *testing.T) {
	if err := apiErrorFromResponse(http.StatusForbidden, nil); err.Error() != "the server sent no detail (HTTP 403)" {
		t.Fatalf("err = %q", err)
	}
}

func TestLangsmithAPIURLMapsRegionalAndProdHosts(t *testing.T) {
	for host, want := range map[string]string{
		"https://api.host.langchain.com":       "https://api.smith.langchain.com",
		"https://eu.api.host.langchain.com/":   "https://eu.api.smith.langchain.com",
		"https://beta.api.host.langchain.com":  "https://beta.api.smith.langchain.com",
		"https://dev.api.host.langchain.com":   "https://dev.api.smith.langchain.com",
		"http://localhost:8080":                "",
		"https://example.com":                  "",
		"https://eu.api.host.langchain.com/x/": "https://eu.api.smith.langchain.com",
	} {
		got, ok := LangsmithAPIBaseURL(host)
		if got != want || ok != (want != "") {
			t.Errorf("LangsmithAPIBaseURL(%s) = %q, %v; want %q", host, got, ok, want)
		}
	}
}

func TestLangsmithHostBackendURLMapsFromPlatformAPI(t *testing.T) {
	for platform, want := range map[string]string{
		"https://api.smith.langchain.com":      "https://api.host.langchain.com",
		"https://eu.api.smith.langchain.com/":  "https://eu.api.host.langchain.com",
		"https://beta.api.smith.langchain.com": "https://beta.api.host.langchain.com",
		// Dashboard hosts are not platform API origins.
		"https://smith.langchain.com": "",
	} {
		got, ok := LangsmithHostBackendURL(platform)
		if got != want || ok != (want != "") {
			t.Errorf("LangsmithHostBackendURL(%s) = %q, %v; want %q", platform, got, ok, want)
		}
	}
}

func TestSelfHostedDetectionUsesTheAPIHostPathSegment(t *testing.T) {
	for url, want := range map[string]bool{
		"https://langsmith.example.com/api-host":       true,
		"https://langsmith.example.com/base/api-host/": true,
		"https://api.host.langchain.com":               false,
		"https://langsmith.example.com/api-hosted":     false,
	} {
		if got := IsSelfHostedHostBackendURL(url); got != want {
			t.Errorf("IsSelfHostedHostBackendURL(%s) = %v, want %v", url, got, want)
		}
	}
}

func TestSelfHostedURLDerivationPreservesTheInstallationBasePath(t *testing.T) {
	host, ok := SelfHostedHostBackendURL("https://langsmith.example.com/base/api/v1?x=1#y")
	if !ok || host != "https://langsmith.example.com/base/api-host" {
		t.Errorf("SelfHostedHostBackendURL = %q, %v", host, ok)
	}
	api, ok := SelfHostedPublicAPIBaseURL("https://langsmith.example.com/base/api/v1?x=1#y")
	if !ok || api != "https://langsmith.example.com/base/api" {
		t.Errorf("SelfHostedPublicAPIBaseURL = %q, %v", api, ok)
	}
}

func TestPlatformAPIURLFallsBackToTheHost(t *testing.T) {
	ls := apiKeyCLIClient(t, "k", "")
	if prod := newClient(t, ls, "https://api.host.langchain.com", ""); prod.platformAPIURL != "https://api.smith.langchain.com" {
		t.Errorf("prod platform = %q", prod.platformAPIURL)
	}
	if local := newClient(t, ls, "http://localhost:1980", ""); local.platformAPIURL != "http://localhost:1980" {
		t.Errorf("local platform = %q", local.platformAPIURL)
	}
	split := newClient(t, ls, "http://localhost:8300", "http://localhost:1980/")
	if split.hostBackendURL != "http://localhost:8300" || split.platformAPIURL != "http://localhost:1980" {
		t.Errorf("split = %q, %q", split.hostBackendURL, split.platformAPIURL)
	}
	if _, err := New(ls, "", ""); err == nil || err.Error() != "Host backend URL is required" {
		t.Errorf("New without a host = %v", err)
	}
}

func TestRequireStr(t *testing.T) {
	var value any
	if err := json.Unmarshal([]byte(`{"upload_url":"https://storage.example/x","n":1}`), &value); err != nil {
		t.Fatal(err)
	}
	if got, err := RequireStr(value, "upload_url", "upload URL"); err != nil || got != "https://storage.example/x" {
		t.Errorf("RequireStr = %q, %v", got, err)
	}
	if _, err := RequireStr(value, "n", "upload URL"); err == nil || err.Error() != "upload URL: response missing string field 'n'" {
		t.Errorf("RequireStr of a number = %v", err)
	}
}

func TestATransportFailureNamesTheRequest(t *testing.T) {
	c := newClient(t, apiKeyCLIClient(t, "key", ""), "http://127.0.0.1:1", "http://127.0.0.1:1")
	_, err := c.GetDeployment(t.Context(), "deployment-1")
	var failed *requestError
	if !errors.As(err, &failed) || !strings.HasPrefix(err.Error(), "GET /v2/deployments/deployment-1 request failed: ") {
		t.Fatalf("err = %v, want a request failure", err)
	}
	if errs.CategoryOf(err) != errs.Network {
		t.Fatalf("category = %v, want network", errs.CategoryOf(err))
	}
}

func TestTypedCallsRefusePlainHTTPToAnotherMachine(t *testing.T) {
	c := newClient(t, apiKeyCLIClient(t, "key", ""), "http://langsmith.example.com/api-host", "http://langsmith.example.com")

	_, err := c.listWorkspaces(t.Context())

	if err == nil || err.Error() != "GET /api/v1/workspaces request failed: refusing to send LangSmith credentials over plain HTTP to http://langsmith.example.com; use https" {
		t.Fatalf("err = %v, want the plain HTTP refusal", err)
	}
}
