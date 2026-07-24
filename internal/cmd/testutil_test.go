package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// testDeploymentVersion is the version newTestServer reports at /info, driving
// v1/v2 selection. Defaults to "dev" (Cloud → v2); override for the v1 path.
var testDeploymentVersion = "dev"

// withDeploymentVersion overrides the reported /info version for one test.
func withDeploymentVersion(t *testing.T, version string) {
	t.Helper()
	prev := testDeploymentVersion
	testDeploymentVersion = version
	t.Cleanup(func() { testDeploymentVersion = prev })
}

// captureStdout redirects os.Stdout during fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return buf.String()
}

// newTestServer creates an httptest server with the given handler, serving a
// default /info (version testDeploymentVersion) so UseV2API works without every
// test mocking it; other paths fall through to handler.
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/info" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"version": testDeploymentVersion})
			return
		}
		handler(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// setupTestEnv sets global flag variables for testing and returns a cleanup function.
func setupTestEnv(t *testing.T, serverURL string) func() {
	t.Helper()
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldWorkspace := flagWorkspaceID
	oldFmt := flagOutputFormat

	flagAPIKey = "test-api-key"
	flagAPIURL = serverURL
	flagProfile = ""
	flagWorkspaceID = ""
	flagOutputFormat = "pretty"

	return func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagWorkspaceID = oldWorkspace
		flagOutputFormat = oldFmt
	}
}

// executeCommand creates a new root command, sets the args, executes it,
// and returns captured stdout and any error.
func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldWorkspace := flagWorkspaceID
	oldFormat := flagOutputFormat
	defer func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagWorkspaceID = oldWorkspace
		flagOutputFormat = oldFormat
	}()

	cmd := NewRootCmd("test", "test")
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return outBuf.String(), err
}
