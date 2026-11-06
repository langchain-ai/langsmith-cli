package hostapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// servePaths is a platform API that answers a fixed body per path.
func servePaths(t *testing.T, routes map[string]reply) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer, ok := routes[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			answer = reply{status: http.StatusNotFound, body: "{}"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(answer.status)
		_, _ = w.Write([]byte(answer.body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func clientFor(t *testing.T, platformAPIURL, tenant string) *Client {
	t.Helper()
	return newClient(t, apiKeyCLIClient(t, "key", tenant), "http://unused.invalid", platformAPIURL)
}

const twoWorkspaces = `[{"id":"ws-1","display_name":"Production"},{"id":"ws-2","display_name":"Development"}]`

const currentOrg = `{"id":"org-1","display_name":"LangChain Inc."}`

func TestAScopedClientNamesItsOwnWorkspaceAndOrganization(t *testing.T) {
	base := servePaths(t, map[string]reply{
		"/api/v1/workspaces":   {status: 200, body: twoWorkspaces},
		"/api/v1/orgs/current": {status: 200, body: currentOrg},
	})

	target := clientFor(t, base, "ws-2").DescribeDeployTarget(t.Context())

	if target.Organization != "LangChain Inc." || target.Workspace == nil || target.Workspace.Label() != "Development" {
		t.Fatalf("target = %+v", target)
	}
}

func TestACredentialThatSeesOneWorkspaceNamesItUnscoped(t *testing.T) {
	base := servePaths(t, map[string]reply{
		"/api/v1/workspaces":   {status: 200, body: `[{"id":"ws-1","display_name":"Production"}]`},
		"/api/v1/orgs/current": {status: 200, body: currentOrg},
	})

	target := clientFor(t, base, "").DescribeDeployTarget(t.Context())

	if target.Workspace == nil || target.Workspace.Label() != "Production" {
		t.Fatalf("workspace = %+v, want Production", target.Workspace)
	}
}

func TestAnUnscopedCredentialThatSeesSeveralWorkspacesNamesNone(t *testing.T) {
	// The host backend resolves the credential's default workspace itself, so
	// naming one of these would be a guess. The organization still holds.
	base := servePaths(t, map[string]reply{
		"/api/v1/workspaces":   {status: 200, body: twoWorkspaces},
		"/api/v1/orgs/current": {status: 200, body: currentOrg},
	})

	target := clientFor(t, base, "").DescribeDeployTarget(t.Context())

	if target.Workspace != nil || target.Organization != "LangChain Inc." {
		t.Fatalf("target = %+v, want the organization and no workspace", target)
	}
}

func TestAScopedWorkspaceTheCredentialCannotListKeepsItsID(t *testing.T) {
	base := servePaths(t, map[string]reply{
		"/api/v1/workspaces":   {status: 403, body: "{}"},
		"/api/v1/orgs/current": {status: 403, body: "{}"},
	})

	target := clientFor(t, base, "ws-9").DescribeDeployTarget(t.Context())

	if target.Organization != "" || target.Workspace == nil || target.Workspace.Label() != "ws-9" {
		t.Fatalf("target = %+v, want workspace ws-9 and no organization", target)
	}
}

func TestAWorkspaceNamedLaterComesFromTheSameListing(t *testing.T) {
	// The unscoped case above leaves Workspace unset; the id the host backend
	// later reports still resolves without a second lookup.
	base := servePaths(t, map[string]reply{
		"/api/v1/workspaces":   {status: 200, body: twoWorkspaces},
		"/api/v1/orgs/current": {status: 200, body: currentOrg},
	})

	target := clientFor(t, base, "").DescribeDeployTarget(t.Context())

	if target.LabelFor("ws-1") != "Production" || target.LabelFor("ws-unlisted") != "ws-unlisted" {
		t.Fatalf("labels = %q, %q", target.LabelFor("ws-1"), target.LabelFor("ws-unlisted"))
	}
}

func TestAScopedClientAnswersWithItsOwnWorkspace(t *testing.T) {
	id, ok := clientFor(t, "http://unused.invalid", "ws-1").WorkspaceIDForLinks(t.Context())
	if !ok || id != "ws-1" {
		t.Fatalf("WorkspaceIDForLinks = %q, %v", id, ok)
	}
}

func TestAnUnscopedClientLinksTheFirstListedWorkspace(t *testing.T) {
	base := servePaths(t, map[string]reply{"/api/v1/workspaces": {status: 200, body: twoWorkspaces}})
	id, ok := clientFor(t, base, "").WorkspaceIDForLinks(t.Context())
	if !ok || id != "ws-1" {
		t.Fatalf("WorkspaceIDForLinks = %q, %v", id, ok)
	}
}

func TestAWorkspaceWithoutANameIsLabeledByItsID(t *testing.T) {
	workspace, ok := decodeWorkspace(map[string]any{"id": "ws-1", "display_name": "  "})
	if !ok || workspace.DisplayName != "" || workspace.Label() != "ws-1" {
		t.Fatalf("workspace = %+v, %v", workspace, ok)
	}
}

func TestAWorkspaceWithoutAnIDIsNotAWorkspace(t *testing.T) {
	if workspace, ok := decodeWorkspace(map[string]any{"display_name": "Production"}); ok {
		t.Fatalf("decoded %+v", workspace)
	}
}

func TestDashboardURLUsesTheAuthoritativeBYOCBase(t *testing.T) {
	base := servePaths(t, map[string]reply{
		"/v1/platform/workspaces/current/info": {status: 200, body: `{"dashboard_url":"https://aws.smith.langchain.com/"}`},
	})
	got, err := clientFor(t, base, "ws-1").DashboardURL(t.Context())
	if err != nil || got != "https://aws.smith.langchain.com" {
		t.Fatalf("DashboardURL = %q, %v", got, err)
	}
}

func TestDashboardURLPreservesASelfHostedBasePath(t *testing.T) {
	got, err := normalizeDashboardURL("https://corp.example.com/langsmith/")
	if err != nil || got != "https://corp.example.com/langsmith" {
		t.Fatalf("normalizeDashboardURL = %q, %v", got, err)
	}
}

func TestDashboardURLRejectsQueryData(t *testing.T) {
	for _, value := range []string{"https://smith.example.com/?tenant=other", "https://user:pw@smith.example.com", "https://smith.example.com/#x"} {
		if got, err := normalizeDashboardURL(value); err == nil {
			t.Errorf("normalizeDashboardURL(%s) = %q, want an error", value, got)
		}
	}
}
