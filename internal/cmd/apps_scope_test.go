package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testWorkspaceAppID = "aaaaaaaa-1111-1111-1111-111111111111"
	testOrgAppID       = "bbbbbbbb-2222-2222-2222-222222222222"
	testOrgID          = "cccccccc-3333-3333-3333-333333333333"
	testTenantID       = "dddddddd-4444-4444-4444-444444444444"
)

// collidingApps: one app per tier.
func collidingApps() []customApp {
	return []customApp{
		{ID: testOrgAppID, Name: "shared-app", OrganizationID: testOrgID},
		{ID: testWorkspaceAppID, Name: "shared-app", TenantID: testTenantID, OrganizationID: testOrgID},
	}
}

func TestCustomAppTier(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  customApp
		want string
	}{
		{"null tenant is org", customApp{OrganizationID: testOrgID}, appScopeOrg},
		{"tenant set is workspace", customApp{TenantID: testTenantID, OrganizationID: testOrgID}, appScopeWorkspace},
		{"explicit organization scope wins", customApp{TenantID: testTenantID, Scope: "organization"}, appScopeOrg},
		{"explicit workspace scope", customApp{Scope: "workspace"}, appScopeWorkspace},
		{"legacy response is workspace", customApp{ID: "a1", Name: "one"}, appScopeWorkspace},
	} {
		if got := tc.app.tier(); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveCustomApp_PrefersWorkspaceOverOrgOnNameCollision(t *testing.T) {
	// Org tier listed first.
	mux := appsSourceServer(t, collidingApps(), nil)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	app, err := resolveCustomApp(t.Context(), MustGetClient(), "shared-app")
	if err != nil {
		t.Fatalf("resolveCustomApp: %v", err)
	}
	if app.ID != testWorkspaceAppID {
		t.Errorf("expected the workspace app to win, got %s", app.ID)
	}

	// Folded matching keeps preference.
	app, err = resolveCustomApp(t.Context(), MustGetClient(), "SHARED-APP")
	if err != nil {
		t.Fatalf("resolveCustomApp (fold): %v", err)
	}
	if app.ID != testWorkspaceAppID {
		t.Errorf("expected the workspace app to win case-insensitively, got %s", app.ID)
	}
}

func TestResolveCustomAppInScope_ForcesTier(t *testing.T) {
	mux := appsSourceServer(t, collidingApps(), nil)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	app, err := resolveCustomAppInScope(t.Context(), MustGetClient(), "shared-app", appScopeOrg)
	if err != nil {
		t.Fatalf("resolve org scope: %v", err)
	}
	if app.ID != testOrgAppID {
		t.Errorf("expected --scope org to pick the org app, got %s", app.ID)
	}

	app, err = resolveCustomAppInScope(t.Context(), MustGetClient(), "shared-app", appScopeWorkspace)
	if err != nil {
		t.Fatalf("resolve workspace scope: %v", err)
	}
	if app.ID != testWorkspaceAppID {
		t.Errorf("expected --scope workspace to pick the workspace app, got %s", app.ID)
	}

	// Forced tier rejects other IDs.
	if _, err := resolveCustomAppInScope(t.Context(), MustGetClient(), testOrgAppID, appScopeWorkspace); err == nil {
		t.Error("expected the org app's ID to miss under --scope workspace")
	}
}

func TestResolveCustomApp_NotFoundErrorsMentionOrgTier(t *testing.T) {
	mux := appsSourceServer(t, collidingApps(), nil)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	_, err := resolveCustomApp(t.Context(), MustGetClient(), "nope")
	if err == nil || !strings.Contains(err.Error(), "shared with its organization") {
		t.Errorf("expected the name error to cover the org tier, got %v", err)
	}

	_, err = resolveCustomApp(t.Context(), MustGetClient(), "eeeeeeee-5555-5555-5555-555555555555")
	if err == nil || !strings.Contains(err.Error(), "shared with its organization") {
		t.Errorf("expected the ID error to cover the org tier, got %v", err)
	}

	_, err = resolveCustomAppInScope(t.Context(), MustGetClient(), "nope", appScopeOrg)
	if err == nil || !strings.Contains(err.Error(), "shared with this organization") {
		t.Errorf("expected a scoped error naming the org tier, got %v", err)
	}
}

func TestAppsPull_ScopeFlag(t *testing.T) {
	var sourcePath string
	archive := tarGz(t, map[string]string{"src/App.tsx": "org copy"})
	mux := appsSourceServer(t, collidingApps(), map[string][]byte{
		testOrgAppID:       archive,
		testWorkspaceAppID: archive,
	})
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/source") {
			sourcePath = r.URL.Path
		}
		mux.ServeHTTP(w, r)
	})
	defer setupTestEnv(t, srv.URL)()
	t.Chdir(t.TempDir())

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", "shared-app", "--scope", "org", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if sourcePath != "/api/v1/platform/custom-apps/"+testOrgAppID+"/source" {
		t.Errorf("expected --scope org to pull the org app, got %q", sourcePath)
	}
	if !strings.Contains(out, `"scope": "org"`) {
		t.Errorf("expected the pulled app's scope in the output:\n%s", out)
	}

	// Workspace wins without --scope.
	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", "shared-app", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if sourcePath != "/api/v1/platform/custom-apps/"+testWorkspaceAppID+"/source" {
		t.Errorf("expected the workspace app by default, got %q", sourcePath)
	}
}

func TestAppsPull_RejectsInvalidScope(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()
	t.Chdir(t.TempDir())

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"pull", "shared-app", "--scope", "team"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `invalid --scope "team"`) {
		t.Errorf("expected an invalid-scope error, got %v", err)
	}
}

func TestAppsList_ShowsScope(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(collidingApps())
	})
	defer setupTestEnv(t, srv.URL)()

	flagOutputFormat = "pretty"
	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	for _, want := range []string{"SCOPE", "org", "workspace"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the table:\n%s", want, out)
		}
	}

	flagOutputFormat = "json"
	out = captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"scope": "org"`) || !strings.Contains(out, `"scope": "workspace"`) {
		t.Errorf("expected both scopes in JSON output:\n%s", out)
	}
	if !strings.Contains(out, `"organization_id": "`+testOrgID+`"`) {
		t.Errorf("expected organization_id in JSON output:\n%s", out)
	}
}

func TestAppsShare_PromotesWorkspaceApp(t *testing.T) {
	var sharePath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collidingApps())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/share"):
			sharePath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: testWorkspaceAppID, Name: "shared-app", OrganizationID: testOrgID})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"share", "shared-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if sharePath != "/api/v1/platform/custom-apps/"+testWorkspaceAppID+"/share" {
		t.Errorf("expected the workspace app shared, got %q", sharePath)
	}
	if !strings.Contains(out, `"status": "shared"`) || !strings.Contains(out, `"scope": "org"`) {
		t.Errorf("expected shared status and org scope:\n%s", out)
	}
}

func TestAppsShare_RefusesAlreadySharedApp(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			t.Error("share should not run for an already-shared app")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: testOrgAppID, Name: "shared-app", OrganizationID: testOrgID},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"share", "shared-app"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already shared") {
		t.Fatalf("expected an already-shared error, got %v", err)
	}
}

func TestAppsShareAndClaim_AreHiddenButRunnable(t *testing.T) {
	for _, name := range []string{"share", "claim"} {
		sub, _, err := newAppsCmd().Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if !sub.Hidden {
			t.Errorf("expected apps %s hidden from help", name)
		}
		if sub.RunE == nil {
			t.Errorf("expected apps %s still runnable", name)
		}
	}

	// Hidden commands stay out of help.
	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"--help"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("help: %v", err)
		}
	})
	for _, name := range []string{"share", "claim"} {
		if strings.Contains(out, "  "+name+" ") {
			t.Errorf("expected %q absent from the command list:\n%s", name, out)
		}
	}
}

func TestAppsDelete_PointsAtClaimForOrgApp(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]customApp{
				{ID: testOrgAppID, Name: "shared-app", OrganizationID: testOrgID, Scope: "organization"},
			})
		case r.Method == "DELETE":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "custom app not found"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"delete", "shared-app", "--yes"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected the 404 to surface as an error")
	}
	if !strings.Contains(err.Error(), "shared with the organization") || !strings.Contains(err.Error(), "apps claim") {
		t.Errorf("expected a claim hint, got %v", err)
	}
}

// Workspace 404s keep the old message.
func TestAppsDelete_WorkspaceAppKeepsGenericNotFound(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]customApp{
				{ID: testWorkspaceAppID, Name: "mine", TenantID: testTenantID, OrganizationID: testOrgID},
			})
		case r.Method == "DELETE":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "custom app not found"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"delete", "mine", "--yes"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "deleting custom app") {
		t.Fatalf("expected the generic delete error, got %v", err)
	}
	if strings.Contains(err.Error(), "apps claim") {
		t.Errorf("expected no claim hint for a workspace app, got %v", err)
	}
}

func TestAppsClaim_ClaimsOrgAppWithNameOverride(t *testing.T) {
	var claimPath string
	var claimBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collidingApps())
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/claim"):
			claimPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &claimBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:       "eeeeeeee-6666-6666-6666-666666666666",
				Name:     "my-copy",
				TenantID: testTenantID,
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"claim", "shared-app", "--as", "my-copy"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	// Claim targets the org app.
	if claimPath != "/api/v1/platform/custom-apps/"+testOrgAppID+"/claim" {
		t.Errorf("expected the org app claimed, got %q", claimPath)
	}
	if claimBody["name"] != "my-copy" {
		t.Errorf("expected --as carried in the body, got %v", claimBody)
	}
	if !strings.Contains(out, `"status": "claimed"`) || !strings.Contains(out, `"name": "my-copy"`) {
		t.Errorf("expected claimed status and new name:\n%s", out)
	}
	if !strings.Contains(out, `"scope": "workspace"`) {
		t.Errorf("expected the claim to report workspace scope:\n%s", out)
	}
}

func TestAppsClaim_OmitsNameWhenNoOverride(t *testing.T) {
	var claimBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collidingApps())
		case r.Method == "POST":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &claimBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: testWorkspaceAppID, Name: "shared-app", TenantID: testTenantID})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"claim", "shared-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if _, ok := claimBody["name"]; ok {
		t.Errorf("expected no name key without --as, got %v", claimBody)
	}
}

func TestAppsClaim_SurfacesNameConflictWithAsHint(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(collidingApps())
		case r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "name already in use"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"claim", "shared-app"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a 409 to surface as an error")
	}
	if !strings.Contains(err.Error(), "already has a custom app named") || !strings.Contains(err.Error(), "--as") {
		t.Errorf("expected an actionable --as hint, got %v", err)
	}
}

func TestAppsClaim_RejectsWorkspaceOnlyApp(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			t.Error("claim should not run for a workspace-only app")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: testWorkspaceAppID, Name: "mine", TenantID: testTenantID, OrganizationID: testOrgID},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"claim", "mine"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "shared with this organization") {
		t.Fatalf("expected claim to only match org apps, got %v", err)
	}
}

// Editing an org app in place.
func TestAppsPullThenPush_UpdatesOrgAppInPlace(t *testing.T) {
	orgApp := customApp{ID: testOrgAppID, Name: "shared-app", OrganizationID: testOrgID, Scope: "organization"}
	archive := tarGz(t, map[string]string{
		"package.json":   `{"name":"shared-app"}`,
		"dist/bundle.js": "module.exports = { render: function() {} }",
		"src/App.tsx":    "export default function App() {}",
	})

	var patchPath string
	var sawPost bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]customApp{orgApp})
		case r.Method == "GET" && r.URL.Path == "/api/v1/platform/custom-apps/"+testOrgAppID+"/source":
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(archive)
		case r.Method == "PATCH":
			patchPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(orgApp)
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			sawPost = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: "app_forked", Name: "shared-app"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)
	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", "shared-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("pull: %v", err)
		}
	})

	pulled := filepath.Join(dir, "shared-app")
	link, err := readAppLink(pulled)
	if err != nil || link == nil || link.AppID != testOrgAppID {
		t.Fatalf("expected the org app's ID in the link file, got %+v (%v)", link, err)
	}

	t.Chdir(pulled)
	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--no-build"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("push: %v", err)
		}
	})

	if sawPost {
		t.Error("expected push to update the org app, not create a workspace copy")
	}
	if patchPath != "/api/v1/platform/custom-apps/"+testOrgAppID {
		t.Errorf("expected a PATCH against the pulled app's ID, got %q", patchPath)
	}
	if !strings.Contains(out, `"status": "updated"`) {
		t.Errorf("expected updated status:\n%s", out)
	}
	if link, err = readAppLink(pulled); err != nil || link == nil || link.AppID != testOrgAppID {
		t.Errorf("expected the link still pointing at the org app, got %+v (%v)", link, err)
	}
}
