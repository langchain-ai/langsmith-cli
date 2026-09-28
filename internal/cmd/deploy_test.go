package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeControlPlane struct {
	t   *testing.T
	srv *httptest.Server

	mu               sync.Mutex
	deployments      []map[string]any
	revisions        []map[string]any
	revisionStatuses []string
	logs             []map[string]any
	created          []map[string]any
	patches          []map[string]any
	logRequests      []map[string]any
	listQueries      []string
	deleted          []string
	uploaded         []byte
	uploadHeaders    http.Header
	listStatus       int
	listBody         string
}

func newFakeControlPlane(t *testing.T) *fakeControlPlane {
	t.Helper()
	f := &fakeControlPlane{t: t}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api-host/v2/deployments", f.listDeployments)
	mux.HandleFunc("POST /api-host/v2/deployments", f.createDeployment)
	mux.HandleFunc("GET /api-host/v2/deployments/{id}", f.getDeployment)
	mux.HandleFunc("PATCH /api-host/v2/deployments/{id}", f.patchDeployment)
	mux.HandleFunc("DELETE /api-host/v2/deployments/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.deleted = append(f.deleted, r.PathValue("id"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api-host/v2/deployments/{id}/upload-url", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"upload_url": f.srv.URL + "/upload", "object_path": "uploads/source.tar.gz"})
	})
	mux.HandleFunc("PUT /upload", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		f.mu.Lock()
		f.uploaded, f.uploadHeaders = body, r.Header.Clone()
		f.mu.Unlock()
	})
	mux.HandleFunc("GET /api-host/v2/deployments/{id}/revisions", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, map[string]any{"resources": f.revisions, "offset": 0})
	})
	mux.HandleFunc("GET /api-host/v2/deployments/{id}/revisions/{rev}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		status := f.revisionStatuses[0]
		if len(f.revisionStatuses) > 1 {
			f.revisionStatuses = f.revisionStatuses[1:]
		}
		writeJSON(w, map[string]any{"id": r.PathValue("rev"), "status": status})
	})
	logsHandler := func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		body["path"] = r.URL.Path
		f.mu.Lock()
		defer f.mu.Unlock()
		f.logRequests = append(f.logRequests, body)
		writeJSON(w, map[string]any{"logs": f.logs})
	}
	mux.HandleFunc("POST /api-host/v1/projects/{id}/revisions/{rev}/build_logs", logsHandler)
	mux.HandleFunc("POST /api-host/v1/projects/{id}/deploy_logs", logsHandler)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

func (f *fakeControlPlane) listDeployments(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listQueries = append(f.listQueries, r.URL.RawQuery)
	if f.listStatus != 0 {
		w.WriteHeader(f.listStatus)
		_, err := io.WriteString(w, f.listBody)
		require.NoError(f.t, err)
		return
	}
	contains := r.URL.Query().Get("name_contains")
	matches := []map[string]any{}
	for _, d := range f.deployments {
		if strings.Contains(d["name"].(string), contains) {
			matches = append(matches, d)
		}
	}
	writeJSON(w, map[string]any{"resources": matches, "offset": 0})
}

func (f *fakeControlPlane) createDeployment(w http.ResponseWriter, r *http.Request) {
	body := decodeBody(f.t, r)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, body)
	d := map[string]any{"id": "dep-new", "name": body["name"], "source": body["source"], "tenant_id": "tenant-1"}
	f.deployments = append(f.deployments, d)
	writeJSON(w, d)
}

func (f *fakeControlPlane) find(id string) map[string]any {
	for _, d := range f.deployments {
		if d["id"] == id {
			return d
		}
	}
	return nil
}

func (f *fakeControlPlane) getDeployment(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.find(r.PathValue("id"))
	if d == nil {
		http.Error(w, `{"detail":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, d)
}

func (f *fakeControlPlane) patchDeployment(w http.ResponseWriter, r *http.Request) {
	body := decodeBody(f.t, r)
	f.mu.Lock()
	defer f.mu.Unlock()
	body["id"] = r.PathValue("id")
	f.patches = append(f.patches, body)
	writeJSON(w, f.find(r.PathValue("id")))
}

func runDeployCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	for _, name := range []string{deploymentNameEnv, "LANGSMITH_AGENT_ID", "LANGSMITH_AGENT_ENVIRONMENT", "LANGGRAPH_HOST_URL", "LANGSMITH_WORKSPACE_ID", "LANGSMITH_TENANT_ID"} {
		t.Setenv(name, "")
	}
	oldKey, oldURL, oldProfile, oldWorkspace, oldFormat := flagAPIKey, flagAPIURL, flagProfile, flagWorkspaceID, flagOutputFormat
	t.Cleanup(func() {
		flagAPIKey, flagAPIURL, flagProfile, flagWorkspaceID, flagOutputFormat = oldKey, oldURL, oldProfile, oldWorkspace, oldFormat
	})
	root := NewRootCmd("test", "test")
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func tarEntries(t *testing.T, data []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return names
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
	}
}

func fastDeployWaits(t *testing.T) {
	t.Helper()
	remote, image, follow := deployRemoteBuildWait, deployImageWait, deployLogsFollowInterval
	deployRemoteBuildWait.interval, deployImageWait.interval, deployLogsFollowInterval = time.Millisecond, time.Millisecond, time.Millisecond
	t.Cleanup(func() { deployRemoteBuildWait, deployImageWait, deployLogsFollowInterval = remote, image, follow })
}

func TestDeployRemoteBuildCreatesDeploymentAndUploadsMonorepoSource(t *testing.T) {
	fastDeployWaits(t)
	cp := newFakeControlPlane(t)
	cp.revisions = []map[string]any{{"id": "rev-1", "status": "QUEUED"}}
	cp.revisionStatuses = []string{"BUILDING", "DEPLOYING", "DEPLOYED"}
	cp.deployments = []map[string]any{{"id": "other", "name": "my-agent-old"}}

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"My Agent/langgraph.json":         `{"dependencies": [".", "../shared"], "graphs": {"agent": "./agent.py:graph"}, "env": ".env"}`,
		"My Agent/agent.py":               "graph = None\n",
		"My Agent/.env":                   "OPENAI_API_KEY=sk-test\nLANGSMITH_API_KEY=reserved\nEMPTY=\n",
		"My Agent/.gitignore":             "secret.txt\n",
		"My Agent/.dockerignore":          "*.log\n",
		"My Agent/secret.txt":             "hidden",
		"My Agent/debug.log":              "hidden",
		"My Agent/node_modules/pkg/i.js":  "hidden",
		"My Agent/__pycache__/agent.pyc":  "hidden",
		"shared/lib.py":                   "x = 1\n",
		"shared/.venv/bin/python":         "hidden",
		"unrelated/should-not-upload.txt": "hidden",
	})
	t.Chdir(filepath.Join(root, "My Agent"))

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy")
	require.NoError(t, err, stderr)

	require.Len(t, cp.created, 1)
	created := cp.created[0]
	assert.Equal(t, "my-agent", created["name"])
	assert.Equal(t, "internal_source", created["source"])
	assert.Equal(t, map[string]any{"deployment_type": "dev"}, created["source_config"])
	assert.Equal(t, []any{map[string]any{"name": "OPENAI_API_KEY", "value": "sk-test"}}, created["secrets"])

	require.Len(t, cp.patches, 1)
	patch := cp.patches[0]
	assert.Equal(t, "dep-new", patch["id"])
	assert.Equal(t, "internal_source", patch["revision_source"])
	assert.Equal(t, map[string]any{
		"source_tarball_path":   "uploads/source.tar.gz",
		"langgraph_config_path": "My Agent/langgraph.json",
	}, patch["source_revision_config"])
	assert.Equal(t, created["secrets"], patch["secrets"])

	assert.Equal(t, "application/gzip", cp.uploadHeaders.Get("Content-Type"))
	assert.Equal(t, "0,209715200", cp.uploadHeaders.Get("X-Goog-Content-Length-Range"))
	assert.ElementsMatch(t, []string{
		"My Agent/.dockerignore",
		"My Agent/.env",
		"My Agent/.gitignore",
		"My Agent/agent.py",
		"My Agent/langgraph.json",
		"shared/lib.py",
	}, tarEntries(t, cp.uploaded))

	var result deployResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, deployResult{
		DeploymentID:   "dep-new",
		RevisionID:     "rev-1",
		Status:         "succeeded",
		RevisionStatus: "DEPLOYED",
		Message:        "Deployment successful!",
		StatusURL:      cp.srv.URL + "/o/tenant-1/host/deployments/dep-new",
	}, result)
	assert.Contains(t, stderr, "Skipping reserved env var: LANGSMITH_API_KEY")
	assert.Contains(t, stderr, "BUILDING")
}

func TestDeployUpdatesExistingDeploymentByNameWithoutWaiting(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.deployments = []map[string]any{
		{"id": "dep-prefix", "name": "my-agent-2"},
		{"id": "dep-1", "name": "my-agent", "tenant_id": "tenant-1"},
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"langgraph.json": `{"env": {"FEATURE_FLAG": "on", "RETRIES": 3}}`})
	t.Chdir(dir)

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy", "--name", "My_Agent", "--no-wait")
	require.NoError(t, err, stderr)

	assert.Empty(t, cp.created)
	require.Len(t, cp.patches, 1)
	assert.Equal(t, "dep-1", cp.patches[0]["id"])
	assert.Equal(t, []any{
		map[string]any{"name": "FEATURE_FLAG", "value": "on"},
		map[string]any{"name": "RETRIES", "value": "3"},
	}, cp.patches[0]["secrets"])
	assert.Contains(t, cp.listQueries[0], "name=my-agent")

	var result deployResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "submitted", result.Status)
	assert.Equal(t, "dep-1", result.DeploymentID)
}

func TestDeployBuildFailureShowsBuildLogTail(t *testing.T) {
	fastDeployWaits(t)
	cp := newFakeControlPlane(t)
	cp.deployments = []map[string]any{{"id": "dep-1", "name": "agent"}}
	cp.revisions = []map[string]any{{"id": "rev-9"}}
	cp.revisionStatuses = []string{"BUILDING", "BUILD_FAILED"}
	cp.logs = []map[string]any{{"message": "error: no such package"}, {"message": "step 2"}, {"message": "step 1"}}
	dir := filepath.Join(t.TempDir(), "agent")
	writeFiles(t, dir, map[string]string{"langgraph.json": `{}`})
	t.Chdir(dir)

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy")
	require.ErrorContains(t, err, "BUILD_FAILED")

	var result deployResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "failed", result.Status)
	require.Len(t, cp.logRequests, 1)
	assert.Equal(t, "desc", cp.logRequests[0]["order"])
	assert.Equal(t, "/api-host/v1/projects/dep-1/revisions/rev-9/build_logs", cp.logRequests[0]["path"])
	assert.Less(t, strings.Index(stderr, "step 1"), strings.Index(stderr, "no such package"))
}

func TestDeployRejectsInvalidFlags(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"langgraph.json": `{}`})
	t.Chdir(dir)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--listener-id", "l-1"}, "only apply when creating a deployment with --push-to"},
		{[]string{"--push-to", "registry.example.com/team/agent", "--image", "a:1", "--listener-id", "l-1", "--deployment-id", "d"}, "cannot be set for an existing --deployment-id"},
		{[]string{"--push-to", "registry.example.com/team/agent"}, "--push-to needs --image"},
		{[]string{"--push-to", "registry.example.com/team/agent:v1", "--tag", "v2", "--image", "a:1"}, "already includes a tag"},
		{[]string{"--push-to", "registry.example.com/team/agent@sha256:abc", "--image", "a:1"}, "not a digest"},
		{[]string{"--image", "a:1", "--tag", "bad/tag"}, "image tag may only contain"},
		{[]string{"--tag", "v1"}, "--tag only applies with --image or --push-to"},
		{[]string{"--agent-id", "agent-1"}, "required together"},
		{[]string{"--agent-id", "agent-1", "--agent-environment", "qa"}, "must be one of"},
		{[]string{"--agent-id", "agent-1", "--agent-environment", "production", "--name", "x"}, "cannot be combined with --name"},
		{[]string{"--deployment-type", "staging"}, "must be dev or prod"},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			_, _, err := runDeployCLI(t, "", append([]string{"--api-key", "test-key", "deploy"}, tc.args...)...)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestDeployWithoutConfigExplainsWhereToRun(t *testing.T) {
	t.Chdir(t.TempDir())
	_, _, err := runDeployCLI(t, "", "--api-key", "test-key", "deploy")
	require.ErrorContains(t, err, "no langgraph.json found")
}

func TestDeployExplainsOrgScopedKey(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.listStatus = http.StatusForbidden
	cp.listBody = `{"detail":"This API key requires workspace specification"}`
	_, _, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "list")
	require.ErrorContains(t, err, "pass --workspace")
}

func TestDeployListPaginatesAndRendersTable(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.deployments = []map[string]any{
		{"id": "dep-1", "name": "agent-one", "source_config": map[string]any{"custom_url": "https://one.example.com"}},
		{"id": "dep-2", "name": "agent-two"},
		{"id": "dep-3", "name": "other"},
	}

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "list", "--name-contains", "agent")
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "agent-one")
	assert.Contains(t, stdout, "https://one.example.com")
	assert.Contains(t, stdout, "agent-two")
	assert.NotContains(t, stdout, "other")
	assert.Equal(t, []string{"limit=100&name_contains=agent"}, cp.listQueries)

	stdout, _, err = runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy", "list", "--limit", "1")
	require.NoError(t, err)
	var listed []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &listed))
	require.NotEmpty(t, listed)
	assert.Equal(t, "limit=1", cp.listQueries[1])
}

func TestDeployRevisionsListMarksSupersededDeploymentsAsReplaced(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.revisions = []map[string]any{
		{"id": "rev-3", "status": "DEPLOYED", "created_at": "2026-03-08T12:00:00Z", "source": "internal_source"},
		{"id": "rev-2", "status": "BUILD_FAILED", "created_at": "2026-03-07T12:00:00Z"},
		{"id": "rev-1", "status": "DEPLOYED", "created_at": "2026-03-06T12:00:00Z"},
	}

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "revisions", "list", "dep-1")
	require.NoError(t, err, stderr)
	assert.Equal(t, 1, strings.Count(stdout, "REPLACED"))
	assert.Equal(t, 1, strings.Count(stdout, "DEPLOYED"))

	stdout, _, err = runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy", "revisions", "list", "dep-1")
	require.NoError(t, err)
	var revisions []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &revisions))
	assert.Equal(t, cp.revisions[0], revisions[0])
	assert.Equal(t, "DEPLOYED", revisions[2]["status"])
}

func TestDeployDeleteConfirms(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.deployments = []map[string]any{{"id": "dep-1", "name": "agent"}}

	_, stderr, err := runDeployCLI(t, "n\n", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "delete", "dep-1")
	require.ErrorContains(t, err, "aborted")
	assert.Contains(t, stderr, "Name: agent")
	assert.Empty(t, cp.deleted)

	stdout, _, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "delete", "dep-1", "--yes")
	require.NoError(t, err)
	assert.Equal(t, []string{"dep-1"}, cp.deleted)
	assert.Contains(t, stdout, "Deleted deployment dep-1.")
}

func TestDeployLogsFindsDeploymentByNameAndPrintsOldestFirst(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.deployments = []map[string]any{{"id": "dep-1", "name": "my-agent"}}
	newer := time.Date(2026, 3, 8, 0, 0, 5, 0, time.UTC).UnixMilli()
	cp.logs = []map[string]any{
		{"id": "2", "timestamp": newer, "level": "ERROR", "message": "boom again"},
		{"id": "1", "timestamp": "2026-03-08T00:00:00Z", "level": "ERROR", "message": "boom"},
	}

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "deploy", "logs", "--name", "my-agent", "--level", "error", "-q", "boom", "--limit", "5")
	require.NoError(t, err, stderr)
	assert.Equal(t, "[2026-03-08T00:00:00Z] [ERROR] boom\n[2026-03-08 00:00:05] [ERROR] boom again\n", stdout)
	require.Len(t, cp.logRequests, 1)
	assert.Equal(t, map[string]any{
		"path":  "/api-host/v1/projects/dep-1/deploy_logs",
		"limit": float64(5),
		"order": "desc",
		"level": "ERROR",
		"query": "boom",
	}, cp.logRequests[0])
}

func TestDeployBuildLogsDefaultToLatestRevision(t *testing.T) {
	cp := newFakeControlPlane(t)
	cp.revisions = []map[string]any{{"id": "rev-7"}}
	cp.logs = []map[string]any{{"message": "Step 1/3"}}

	stdout, stderr, err := runDeployCLI(t, "", "--api-key", "test-key", "--api-url", cp.srv.URL, "--format", "json", "deploy", "logs", "--deployment-id", "dep-1", "--type", "build")
	require.NoError(t, err, stderr)
	assert.Contains(t, stderr, "Using latest revision: rev-7")
	assert.Equal(t, "/api-host/v1/projects/dep-1/revisions/rev-7/build_logs", cp.logRequests[0]["path"])
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &entries))
	assert.Equal(t, []map[string]any{{"message": "Step 1/3"}}, entries)
}

func TestNormalizeDeploymentName(t *testing.T) {
	tests := map[string]string{
		"my-agent":       "my-agent",
		"My Agent":       "my-agent",
		"My_Agent.v2":    "my-agent-v2",
		"--Edge--":       "edge",
		"":               "app",
		"___":            "app",
		"ünïcode agent":  "n-code-agent",
		"a  b":           "a-b",
		"UPPER-case-123": "upper-case-123",
	}
	for input, want := range tests {
		assert.Equal(t, want, normalizeDeploymentName(input), input)
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		ref, repository, tag string
	}{
		{"agent", "agent", ""},
		{"agent:v1", "agent", "v1"},
		{"registry.example.com:5000/team/agent", "registry.example.com:5000/team/agent", ""},
		{"registry.example.com:5000/team/agent:v1", "registry.example.com:5000/team/agent", "v1"},
	}
	for _, tc := range tests {
		ref, err := parseImageReference(tc.ref)
		require.NoError(t, err)
		assert.Equal(t, imageReference{repository: tc.repository, tag: tc.tag}, ref)
		assert.Equal(t, tc.ref, ref.String())
	}
	_, err := parseImageReference("agent@sha256:abc")
	require.Error(t, err)
}

func TestCreateSourceArchiveFailsWhenConfigIsIgnored(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"langgraph.json": `{}`, ".dockerignore": "*.json\n"})
	cfg, err := loadLanggraphConfig(filepath.Join(dir, "langgraph.json"))
	require.NoError(t, err)
	_, err = createSourceArchive(cfg, &deployProgress{w: io.Discard})
	require.ErrorContains(t, err, "langgraph.json not found in archive")
}
