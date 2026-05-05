package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

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

// newTestServer creates an httptest server with the given handler.
func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

// setupTestEnv sets global flag variables for testing and returns a cleanup function.
func setupTestEnv(t *testing.T, serverURL string) func() {
	t.Helper()
	oldKey := flagAPIKey
	oldURL := flagAPIURL
	oldProfile := flagProfile
	oldFmt := flagOutputFormat
	oldJSON := flagOutputJSON

	flagAPIKey = "test-api-key"
	flagAPIURL = serverURL
	flagProfile = ""
	flagOutputFormat = "json"
	flagOutputJSON = false

	return func() {
		flagAPIKey = oldKey
		flagAPIURL = oldURL
		flagProfile = oldProfile
		flagOutputFormat = oldFmt
		flagOutputJSON = oldJSON
	}
}

// executeCommand creates a new root command, sets the args, executes it,
// and returns captured stdout and any error.
func executeCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	format := flagOutputFormat
	jsonOutput := flagOutputJSON
	defer func() {
		flagOutputFormat = format
		flagOutputJSON = jsonOutput
	}()
	cmd := NewRootCmd("test", "test")
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)
	if format != "" {
		hasFormat := false
		for _, arg := range args {
			if arg == "--json" || arg == "--format" || strings.HasPrefix(arg, "--format=") {
				hasFormat = true
				break
			}
		}
		if !hasFormat {
			if format == "json" {
				args = append([]string{"--json"}, args...)
			} else {
				args = append([]string{"--format", format}, args...)
			}
		}
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return outBuf.String(), err
}
