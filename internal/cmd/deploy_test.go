package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

const (
	deployTestWorkspace    = "11111111-2222-3333-4444-555555555555"
	deployTestDeploymentID = "7ad71a02-1c0e-4477-a0e4-d27e108e5ab6"
	deployTestAccessToken  = "deploy-oauth-token"
	deployTestDeviceToken  = "deploy-device-token"
	deployTestDashboard    = "https://smith.example.com"
)

// deployRequest is a request one of the fake LangSmith services received.
type deployRequest struct {
	method string
	// path is the path and query.
	path   string
	header http.Header
	body   []byte
}

func (r deployRequest) line() string {
	return r.method + " " + r.path
}

func (r deployRequest) json(t *testing.T) map[string]any {
	t.Helper()
	var object map[string]any
	require.NoError(t, json.Unmarshal(r.body, &object), string(r.body))
	return object
}

// fakeService is an httptest server that records every request and answers
// it with route.
type fakeService struct {
	*httptest.Server
	mu       sync.Mutex
	requests []deployRequest
}

func newFakeService(t *testing.T, route func(r deployRequest) (int, any)) *fakeService {
	t.Helper()
	service := &fakeService{}
	service.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		request := deployRequest{method: r.Method, path: r.URL.RequestURI(), header: r.Header.Clone(), body: body}
		service.mu.Lock()
		service.requests = append(service.requests, request)
		service.mu.Unlock()
		status, response := route(request)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if response != nil {
			if err := json.NewEncoder(w).Encode(response); err != nil {
				t.Errorf("writing response: %v", err)
			}
		}
	}))
	t.Cleanup(service.Close)
	return service
}

func (s *fakeService) received() []deployRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func (s *fakeService) lines() []string {
	var lines []string
	for _, request := range s.received() {
		lines = append(lines, request.line())
	}
	return lines
}

// find is the one request with line, failing the test when there is not
// exactly one.
func (s *fakeService) find(t *testing.T, line string) deployRequest {
	t.Helper()
	var found []deployRequest
	for _, request := range s.received() {
		if request.line() == line {
			found = append(found, request)
		}
	}
	require.Len(t, found, 1, "requests %q in %q", line, s.lines())
	return found[0]
}

// fakeLangSmith is a stand-in for the LangSmith services a deploy talks to: the
// host backend (deployments, and the deployed Agent Server's crons), the
// platform API (workspace info, Context Hub, agents, sandboxes), and the
// Google Cloud Storage bucket the source is uploaded to.
type fakeLangSmith struct {
	host, platform, gcs *fakeService
}

func newFakeLangSmith(t *testing.T) *fakeLangSmith {
	t.Helper()
	fake := &fakeLangSmith{}
	var (
		mu          sync.Mutex
		repoCreated bool
		committed   bool
		created     bool
	)
	fake.gcs = newFakeService(t, func(r deployRequest) (int, any) {
		if r.line() != "PUT /upload/source.tar.gz?X-Goog-Signature=signed" {
			t.Errorf("unexpected upload request %s", r.line())
			return http.StatusNotFound, nil
		}
		return http.StatusOK, nil
	})
	fake.platform = newFakeService(t, func(r deployRequest) (int, any) {
		mu.Lock()
		defer mu.Unlock()
		path, _, _ := strings.Cut(r.path, "?")
		switch r.method + " " + path {
		case "GET /v1/platform/workspaces/current/info":
			return http.StatusOK, map[string]any{"dashboard_url": deployTestDashboard}
		case "GET /api/v1/workspaces":
			return http.StatusOK, []any{map[string]any{"id": deployTestWorkspace, "display_name": "Fixture Workspace"}}
		case "GET /api/v1/orgs/current":
			return http.StatusOK, map[string]any{"display_name": "Fixture Org"}
		case "GET /api/v1/repos/-/fixture-agent":
			if !repoCreated {
				return http.StatusNotFound, map[string]any{"detail": "not found"}
			}
			return http.StatusOK, map[string]any{"repo": map[string]any{"id": "repo-1", "repo_handle": "fixture-agent", "tags": []any{"managed-deepagents"}}}
		case "POST /api/v1/repos":
			repoCreated = true
			return http.StatusOK, map[string]any{"repo": map[string]any{"id": "repo-1", "repo_handle": "fixture-agent"}}
		case "GET /api/v1/platform/hub/repos/-/fixture-agent/directories":
			if !committed {
				return http.StatusNotFound, map[string]any{"detail": "not found"}
			}
			return http.StatusOK, map[string]any{"commit_hash": "commit-1", "files": map[string]any{}}
		case "POST /api/v1/platform/hub/repos/-/fixture-agent/directories/commits":
			committed = true
			return http.StatusOK, map[string]any{"commit": map[string]any{"commit_hash": "commit-1"}}
		case "GET /api/v1/agents":
			return http.StatusOK, map[string]any{"items": []any{}}
		case "GET /api/v2/sandboxes/snapshots":
			return http.StatusOK, map[string]any{"snapshots": []any{}}
		case "POST /oauth/device/code":
			return http.StatusOK, deviceCodeResponse{
				DeviceCode:      "device-code",
				UserCode:        "WXYZ-1234",
				VerificationURI: fake.platform.URL + "/activate",
				ExpiresIn:       60,
			}
		case "POST /oauth/token":
			return http.StatusOK, oauthTokenResponse{AccessToken: deployTestDeviceToken, ExpiresIn: 3600, RefreshToken: "device-refresh"}
		}
		if strings.Contains(path, "/.well-known/") {
			// No OAuth metadata: the CLI falls back to <api>/oauth/*.
			return http.StatusNotFound, nil
		}
		t.Errorf("unexpected platform request %s", r.line())
		return http.StatusNotFound, map[string]any{"detail": "not found"}
	})
	fake.host = newFakeService(t, func(r deployRequest) (int, any) {
		mu.Lock()
		defer mu.Unlock()
		deployment := map[string]any{
			"id":             deployTestDeploymentID,
			"name":           "fixture-agent",
			"tenant_id":      deployTestWorkspace,
			"source":         "internal_source",
			"deployment_url": fake.host.URL,
		}
		path, _, _ := strings.Cut(r.path, "?")
		switch r.method + " " + path {
		case "GET /v2/deployments":
			if !created {
				return http.StatusOK, map[string]any{"resources": []any{}}
			}
			return http.StatusOK, map[string]any{"resources": []any{deployment}}
		case "POST /v2/deployments":
			created = true
			return http.StatusOK, deployment
		case "GET /v2/deployments/" + deployTestDeploymentID, "PATCH /v2/deployments/" + deployTestDeploymentID:
			return http.StatusOK, deployment
		case "POST /v2/deployments/" + deployTestDeploymentID + "/upload-url":
			return http.StatusOK, map[string]any{
				"upload_url":  fake.gcs.URL + "/upload/source.tar.gz?X-Goog-Signature=signed",
				"object_path": "uploads/source.tar.gz",
			}
		case "GET /v2/deployments/" + deployTestDeploymentID + "/revisions":
			return http.StatusOK, map[string]any{"resources": []any{map[string]any{"id": "revision-1", "status": "QUEUED"}}}
		case "GET /v2/deployments/" + deployTestDeploymentID + "/revisions/revision-1":
			return http.StatusOK, map[string]any{"id": "revision-1", "status": "DEPLOYED"}
		case "POST /runs/crons/search":
			return http.StatusOK, []any{}
		}
		t.Errorf("unexpected host request %s", r.line())
		return http.StatusNotFound, map[string]any{"detail": "not found"}
	})
	return fake
}

// deployFixture writes a Python Managed Deep Agent project into a temporary
// directory, with a virtualenv holding the stub `managed-deepagents` 0.10.0.
func deployFixture(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	require.NoError(t, err, "python3 is required for the deploy command tests")
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	files := map[string]string{
		"agent.py": "from managed_deepagents import define_deep_agent\n\n" +
			"agent = define_deep_agent(name=\"fixture-agent\", model=\"openai:gpt-5.5\")\n",
		"instructions.md": "Be helpful.\n",
		".env":            "OPENAI_API_KEY=sk-fixture\n",
	}
	for rel, contents := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(contents), 0o600))
	}

	venv := filepath.Join(root, ".venv")
	output, err := exec.Command(python, "-m", "venv", "--without-pip", venv).CombinedOutput()
	require.NoError(t, err, string(output))
	sitePackages, err := filepath.Glob(filepath.Join(venv, "lib", "python3*", "site-packages"))
	require.NoError(t, err)
	windows, err := filepath.Glob(filepath.Join(venv, "Lib", "site-packages"))
	require.NoError(t, err)
	sitePackages = append(sitePackages, windows...)
	require.Len(t, sitePackages, 1)
	pkg := filepath.Join(sitePackages[0], "managed_deepagents")
	copyTree(t, filepath.Join("..", "mda", "introspect", "testdata", "python", "managed_deepagents"), pkg)
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "runtime.py"), []byte("# stub runtime\n"), 0o600))
	distInfo := filepath.Join(sitePackages[0], "managed_deepagents-0.10.0.dist-info")
	require.NoError(t, os.MkdirAll(distInfo, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(distInfo, "METADATA"),
		[]byte("Metadata-Version: 2.1\nName: managed-deepagents\nVersion: 0.10.0\n"), 0o600))
	return root
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, contents, 0o600)
	})
	require.NoError(t, err)
}

// isolateDeployEnv keeps the developer's LangSmith environment and flags out
// of the command and gives it config as its LangSmith config file, whose path
// it returns.
func isolateDeployEnv(t *testing.T, config map[string]any) string {
	t.Helper()
	for _, key := range []string{
		"LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_TENANT_ID", "LANGSMITH_WORKSPACE_ID",
		"LANGSMITH_PROFILE", "LANGCHAIN_API_KEY", "LANGCHAIN_CUSTOM_HEADERS", "VIRTUAL_ENV",
		"MDA_HOST_BUILDS_IMAGES", "OPENAI_API_KEY",
	} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	data, err := json.Marshal(config)
	require.NoError(t, err)
	configPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(configPath, data, 0o600))
	t.Setenv("LANGSMITH_CONFIG_FILE", configPath)

	oldKey, oldURL, oldProfile, oldWorkspace := flagAPIKey, flagAPIURL, flagProfile, flagWorkspaceID
	oldOpenBrowser := openBrowser
	t.Cleanup(func() {
		flagAPIKey, flagAPIURL, flagProfile, flagWorkspaceID = oldKey, oldURL, oldProfile, oldWorkspace
		openBrowser = oldOpenBrowser
	})
	flagAPIKey, flagAPIURL, flagProfile, flagWorkspaceID = "", "", "", ""
	openBrowser = func(string) error { return nil }
	return configPath
}

// signInWithOAuthProfile isolates the CLI's environment and signs it in with
// an OAuth profile for the fake platform API, scoped to the test workspace.
func signInWithOAuthProfile(t *testing.T, platformURL string) {
	t.Helper()
	isolateDeployEnv(t, map[string]any{
		"current_profile": "default",
		"profiles": map[string]any{"default": map[string]any{
			"api_url":      platformURL,
			"workspace_id": deployTestWorkspace,
			"oauth": map[string]any{
				"access_token": deployTestAccessToken,
				"expires_at":   "2099-01-01T00:00:00Z",
			},
		}},
	})
}

// deployRun is what a `langsmith deploy` run printed.
type deployRun struct {
	// session is what the deploy session drew.
	session string
	// stderr is the command's own error stream.
	stderr string
}

// runDeployCommand runs `langsmith deploy` with args on a terminal nobody
// answers on, and returns what the deploy session drew.
func runDeployCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	run, err := runDeployCommandOn(t, "", false, args...)
	return run.session, err
}

// runDeployCommandOn runs `langsmith deploy` with args on a terminal that is
// interactive or not, where input is typed.
func runDeployCommandOn(t *testing.T, input string, interactive bool, args ...string) (deployRun, error) {
	t.Helper()
	session := &strings.Builder{}
	t.Cleanup(ui.SetDefault(ui.NewTerminal(strings.NewReader(input), session, ui.WithInteractive(interactive), ui.WithColors(false))))
	root := NewRootCmd("test", "test")
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"deploy"}, args...))
	root.SetContext(t.Context())
	err := root.Execute()
	return deployRun{session: session.String(), stderr: stderr.String()}, err
}

// assertLangSmithAuth checks every request to a LangSmith service, sign-in
// aside, carried the OAuth token and the workspace.
func assertLangSmithAuth(t *testing.T, token string, services ...*fakeService) {
	t.Helper()
	for _, service := range services {
		for _, request := range service.received() {
			if strings.HasPrefix(request.path, "/oauth/") || strings.Contains(request.path, "/.well-known/") {
				continue
			}
			assert.Equal(t, "Bearer "+token, request.header.Get("Authorization"), request.line())
			assert.Equal(t, deployTestWorkspace, request.header.Get("X-Tenant-Id"), request.line())
			assert.Empty(t, request.header.Get("X-Api-Key"), request.line())
		}
	}
}

// assertDeploymentCreated checks the deployment was created with the
// project's secrets and the ones deploy owns, and returns its handoff secret.
func assertDeploymentCreated(t *testing.T, fake *fakeLangSmith) {
	t.Helper()
	create := fake.host.find(t, "POST /v2/deployments").json(t)
	assert.Equal(t, "fixture-agent", create["name"])
	assert.Equal(t, map[string]any{"deployment_type": "dev"}, create["source_config"])
	secrets := secretValues(t, create["secrets"])
	assert.Len(t, secrets["MDA_HANDOFF_SECRET"], 64)
	delete(secrets, "MDA_HANDOFF_SECRET")
	assert.Equal(t, map[string]string{
		"OPENAI_API_KEY":         "sk-fixture",
		"LANGSMITH_WORKSPACE_ID": deployTestWorkspace,
		"MDA_DEPLOYMENT_NAME":    "fixture-agent",
		"MDA_ASSISTANT_ID":       "fixture-agent",
		"MDA_LANGSMITH_URL":      deployTestDashboard,
	}, secrets)

	update := fake.host.find(t, "PATCH /v2/deployments/"+deployTestDeploymentID).json(t)
	assert.Equal(t, "internal_source", update["revision_source"])
	assert.Equal(t, map[string]any{
		"source_tarball_path":   "uploads/source.tar.gz",
		"langgraph_config_path": "langgraph.json",
	}, update["source_revision_config"])
	assert.Equal(t, secretValues(t, create["secrets"]), secretValues(t, update["secrets"]))
}

func secretValues(t *testing.T, value any) map[string]string {
	t.Helper()
	list, ok := value.([]any)
	require.True(t, ok, "secrets %v are not a list", value)
	secrets := map[string]string{}
	for _, item := range list {
		secret, ok := item.(map[string]any)
		require.True(t, ok)
		name, _ := secret["name"].(string)
		secretValue, _ := secret["value"].(string)
		secrets[name] = secretValue
	}
	return secrets
}

// assertSourceUploaded checks the build was uploaded to the signed URL as a
// gzipped tarball, with no LangSmith credential.
func assertSourceUploaded(t *testing.T, fake *fakeLangSmith) {
	t.Helper()
	upload := fake.gcs.find(t, "PUT /upload/source.tar.gz?X-Goog-Signature=signed")
	assert.Empty(t, upload.header.Get("Authorization"))
	assert.Empty(t, upload.header.Get("X-Api-Key"))
	assert.Empty(t, upload.header.Get("X-Tenant-Id"))
	assert.Equal(t, "application/gzip", upload.header.Get("Content-Type"))
	assert.Equal(t, "0,209715200", upload.header.Get("X-Goog-Content-Length-Range"))

	decoder, err := gzip.NewReader(bytes.NewReader(upload.body))
	require.NoError(t, err)
	archive := tar.NewReader(decoder)
	var names []string
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		names = append(names, header.Name)
	}
	assert.Contains(t, names, "langgraph.json")
	assert.Contains(t, names, "agent.py")
	assert.Contains(t, names, "_mda_entry.py")
}

func TestDeployNoWaitCreatesAndUploadsWithTheOAuthProfile(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	signInWithOAuthProfile(t, fake.platform.URL)

	session, err := runDeployCommand(t, project, "--host-url", fake.host.URL, "--no-wait")

	require.NoError(t, err, session)
	assert.Contains(t, session, "Organization: Fixture Org")
	assert.Contains(t, session, "Workspace: Fixture Workspace")
	assert.Contains(t, session, deployTestDashboard+"/o/"+deployTestWorkspace+"/host/deployments/"+deployTestDeploymentID)
	assert.Contains(t, session, "Build triggered for 'fixture-agent'. Not waiting (--no-wait).")
	assertLangSmithAuth(t, deployTestAccessToken, fake.host, fake.platform)
	assertDeploymentCreated(t, fake)
	assertSourceUploaded(t, fake)
	commit := fake.platform.find(t, "POST /api/v1/platform/hub/repos/-/fixture-agent/directories/commits").json(t)
	files, ok := commit["files"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"type": "file", "content": "Be helpful.\n"}, files["instructions.md"])
	assert.NotContains(t, strings.Join(fake.host.lines(), "\n"), "/revisions")
}

func TestDeployWaitsForTheRevisionToGoLive(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	signInWithOAuthProfile(t, fake.platform.URL)

	session, err := runDeployCommand(t, project, "--host-url", fake.host.URL)

	require.NoError(t, err, session)
	assert.Contains(t, session, "Deployment live")
	assert.Contains(t, session, "Deployed 'fixture-agent' to LangSmith.")
	assertLangSmithAuth(t, deployTestAccessToken, fake.host, fake.platform)
	assertDeploymentCreated(t, fake)
	assertSourceUploaded(t, fake)
	assert.Equal(t, "/v2/deployments/"+deployTestDeploymentID+"/revisions?limit=1",
		fake.host.find(t, "GET /v2/deployments/"+deployTestDeploymentID+"/revisions?limit=1").path)
	fake.host.find(t, "GET /v2/deployments/"+deployTestDeploymentID+"/revisions/revision-1")
	search := fake.host.find(t, "POST /runs/crons/search")
	assert.Equal(t, "langsmith", search.header.Get("X-Auth-Scheme"))
}

func TestDeploySignsInWithTheDeviceFlowWhenTheCLIHasNoCredential(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	configPath := isolateDeployEnv(t, map[string]any{})

	run, err := runDeployCommandOn(t, "", true,
		project, "--api-url", fake.platform.URL, "--host-url", fake.host.URL,
		"--workspace", deployTestWorkspace, "--no-wait")

	require.NoError(t, err, run.session)
	assert.Contains(t, run.stderr, "Signing in to LangSmith at "+fake.platform.URL+"\n\nOpen this URL to authorize the LangSmith CLI:\n"+fake.platform.URL+"/activate")
	assert.Contains(t, run.stderr, "WXYZ-1234")
	assert.Contains(t, run.session, "Build triggered for 'fixture-agent'. Not waiting (--no-wait).")
	assertLangSmithAuth(t, deployTestDeviceToken, fake.host, fake.platform)
	assertDeploymentCreated(t, fake)
	config, err := lsconfig.LoadFrom(configPath)
	require.NoError(t, err)
	assert.Equal(t, "default", config.CurrentProfile)
	profile := config.Profiles["default"]
	assert.Equal(t, fake.platform.URL, profile.APIURL)
	assert.Equal(t, deployTestDeviceToken, profile.OAuth.AccessToken)
	assert.Equal(t, "device-refresh", profile.OAuth.RefreshToken)
}

// signedInElsewhere signs the CLI in with an OAuth profile for another
// LangSmith, whose URL it returns, and points the project's .env at the fake
// platform API.
func signedInElsewhere(t *testing.T, project string, fake *fakeLangSmith) (elsewhere, configPath string) {
	t.Helper()
	elsewhere = "https://langsmith.example.com"
	configPath = isolateDeployEnv(t, map[string]any{
		"current_profile": "default",
		"profiles": map[string]any{"default": map[string]any{
			"api_url":      elsewhere,
			"workspace_id": "elsewhere-workspace",
			"oauth": map[string]any{
				"access_token": deployTestAccessToken,
				"expires_at":   "2099-01-01T00:00:00Z",
			},
		}},
	})
	appendEnv(t, project, "LANGSMITH_ENDPOINT="+fake.platform.URL)
	return elsewhere, configPath
}

// appendEnv adds line to the project's .env.
func appendEnv(t *testing.T, project, line string) {
	t.Helper()
	env, err := os.OpenFile(filepath.Join(project, ".env"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = env.WriteString(line + "\n")
	require.NoError(t, err)
	require.NoError(t, env.Close())
}

func TestDeployDoesNotSendTheProfileToAProjectEndpointElsewhere(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	elsewhere, _ := signedInElsewhere(t, project, fake)

	session, err := runDeployCommand(t, project, "--host-url", fake.host.URL)

	var reported ReportedError
	require.ErrorAs(t, err, &reported)
	assert.Contains(t, session, "The LangSmith CLI's credentials are for "+elsewhere+", not "+fake.platform.URL+
		", so they are not sent there. Run `langsmith auth login --api-url "+fake.platform.URL+"`")
	assert.Empty(t, fake.host.received())
	assert.Empty(t, fake.platform.received())
}

func TestDeployStopsWithoutFailingWhenSigningInElsewhereIsDeclined(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	elsewhere, configPath := signedInElsewhere(t, project, fake)

	run, err := runDeployCommandOn(t, "n", true, project, "--host-url", fake.host.URL)

	require.NoError(t, err, run.session)
	assert.Contains(t, run.session, "Sign in to LangSmith at "+fake.platform.URL+"? Profile 'default' is set up for "+
		elsewhere+" and will point at "+fake.platform.URL+" instead.")
	assert.Contains(t, run.session, "Cancelled")
	assert.Empty(t, fake.host.received())
	assert.Empty(t, fake.platform.received())
	config, err := lsconfig.LoadFrom(configPath)
	require.NoError(t, err)
	assert.Equal(t, elsewhere, config.Profiles["default"].APIURL)
	assert.Equal(t, "elsewhere-workspace", config.Profiles["default"].WorkspaceID)
}

func TestDeploySigningInElsewhereRepointsTheProfileWithoutItsWorkspace(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	_, configPath := signedInElsewhere(t, project, fake)

	run, err := runDeployCommandOn(t, "y", true,
		project, "--host-url", fake.host.URL, "--workspace", deployTestWorkspace, "--no-wait")

	require.NoError(t, err, run.session)
	assert.Contains(t, run.stderr, "Signing in to LangSmith at "+fake.platform.URL+"\n")
	assertLangSmithAuth(t, deployTestDeviceToken, fake.host, fake.platform)
	config, err := lsconfig.LoadFrom(configPath)
	require.NoError(t, err)
	profile := config.Profiles["default"]
	assert.Equal(t, fake.platform.URL, profile.APIURL)
	assert.Equal(t, "", profile.WorkspaceID)
	assert.Equal(t, deployTestDeviceToken, profile.OAuth.AccessToken)
}

func TestDeployWithAProjectAPIKeyIgnoresABrokenCLIConfig(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	configPath := isolateDeployEnv(t, map[string]any{})
	require.NoError(t, os.WriteFile(configPath, []byte("{not json"), 0o600))
	appendEnv(t, project, "LANGSMITH_ENDPOINT="+fake.platform.URL)
	appendEnv(t, project, "LANGSMITH_API_KEY=project-key")
	appendEnv(t, project, "LANGSMITH_WORKSPACE_ID="+deployTestWorkspace)

	session, err := runDeployCommand(t, project, "--host-url", fake.host.URL, "--no-wait")

	require.NoError(t, err, session)
	assert.Contains(t, session, "Build triggered for 'fixture-agent'. Not waiting (--no-wait).")
	for _, request := range append(fake.host.received(), fake.platform.received()...) {
		assert.Equal(t, "project-key", request.header.Get("X-Api-Key"), request.line())
		assert.Empty(t, request.header.Get("Authorization"), request.line())
		assert.Equal(t, deployTestWorkspace, request.header.Get("X-Tenant-Id"), request.line())
	}
}

func TestDeployWithoutACredentialOrTerminalNamesAuthLogin(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	isolateDeployEnv(t, map[string]any{})

	session, err := runDeployCommand(t, project, "--api-url", fake.platform.URL, "--host-url", fake.host.URL)

	var reported ReportedError
	require.ErrorAs(t, err, &reported)
	assert.Contains(t, session, "No LangSmith credentials found. Run `langsmith auth login`, or set LANGSMITH_API_KEY "+
		"in the environment or in "+filepath.Join(project, ".env"))
	assert.Empty(t, fake.host.received())
	assert.Empty(t, fake.platform.received())
}

func TestDeployOutsideAProjectFailsBeforeTheSession(t *testing.T) {
	fake := newFakeLangSmith(t)
	signInWithOAuthProfile(t, fake.platform.URL)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	session, err := runDeployCommand(t, dir)

	assert.EqualError(t, err, dir+" is not a Managed Deep Agent project (no agent.ts, agent.tsx, or agent.py found).\n"+
		"Run `mda init <name>` to create one, or `cd` into an existing project.")
	assert.Empty(t, session)
	assert.Empty(t, fake.host.received())
	assert.Empty(t, fake.platform.received())
}

func TestDeployRejectsAnOutOfRangeWaitTimeout(t *testing.T) {
	_, err := runDeployCommand(t, ".", "--wait-timeout-seconds", "3601")

	assert.EqualError(t, err, "invalid value 3601 for --wait-timeout-seconds: must be between 1 and 3600")
}

func TestDeployRejectsAnUnknownAgentEnvironment(t *testing.T) {
	_, err := runDeployCommand(t, ".", "--agent-environment", "dev")

	assert.EqualError(t, err, `invalid value "dev" for --agent-environment: expected one of development, staging, production`)
}

func TestDeployTreatsAnEmptyPushToAsGiven(t *testing.T) {
	project := deployFixture(t)
	fake := newFakeLangSmith(t)
	signInWithOAuthProfile(t, fake.platform.URL)

	session, err := runDeployCommand(t, project, "--host-url", fake.host.URL, "--push-to", "")

	var reported ReportedError
	require.ErrorAs(t, err, &reported)
	assert.Contains(t, session, "--push-to is only supported for self-hosted LangSmith deployments")
	assert.Empty(t, fake.host.received())
}
