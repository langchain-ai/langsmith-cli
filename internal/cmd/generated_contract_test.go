package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	generated "github.com/langchain-ai/langsmith-cli/internal/generated/pkg/cmd"
)

// generatedContractCase is one `langsmith <resource> <op>` invocation of a
// generated command and the HTTP request it must send.
type generatedContractCase struct {
	name string
	// op is the generated operation, for example "create".
	op string
	// args follow `langsmith <resource> <op>`.
	args []string
	// stdin is fed to the command, for example "y\n" to confirm a delete.
	stdin string

	wantMethod string
	wantPath   string
	wantQuery  map[string]string
	// wantHeaders are request headers the command must send, for example
	// X-Tenant-Id for --workspace.
	wantHeaders map[string]string
	// wantBody is the exact JSON request body; empty means no body.
	wantBody string

	// status and response are what the fake API returns (default 200, "{}").
	status   int
	response string

	// wantExitCode is the process exit code (default 0).
	wantExitCode int
	// wantRequests is the number of requests sent (default 1). A case that
	// sends none cannot assert a request.
	wantRequests int
	// wantStdout and wantStderr are substrings the output must contain.
	wantStdout string
	wantStderr string
}

// generatedContractCases holds the cases for each resource in
// generatedCommands, registered from generated_<resource>_test.go.
var generatedContractCases = map[string][]generatedContractCase{}

func TestGeneratedCommandContracts(t *testing.T) {
	for _, resource := range generatedCommands {
		for _, c := range generatedContractCases[resource] {
			t.Run(resource+"/"+c.name, func(t *testing.T) {
				runGeneratedContractCase(t, resource, c)
			})
		}
	}
}

// TestEveryGeneratedOperationHasAContractCase keeps a resource from shipping
// with an operation that no test has run.
func TestEveryGeneratedOperationHasAContractCase(t *testing.T) {
	for _, resource := range generatedCommands {
		covered := map[string]bool{}
		for _, c := range generatedContractCases[resource] {
			covered[c.op] = true
		}
		for _, op := range findGeneratedCommand(resource).Commands {
			// urfave/cli adds a help subcommand to every resource.
			if op.Name != "help" && !covered[op.Name] {
				t.Errorf("'langsmith %s %s' has no contract case; add one to generated_%s_test.go",
					resource, op.Name, strings.ReplaceAll(resource, "-", "_"))
			}
		}
	}
}

// TestGeneratedSubresourcesAreExposedWithTheirResource keeps a resource's
// subresources (generated as separate `<resource>:<sub>` commands) from being
// left out when the resource is exposed.
func TestGeneratedSubresourcesAreExposedWithTheirResource(t *testing.T) {
	exposed := map[string]bool{}
	for _, name := range generatedCommands {
		exposed[name] = true
	}
	for _, name := range generatedCommands {
		for _, c := range generated.Command.Commands {
			if strings.HasPrefix(c.Name, name+":") && !exposed[c.Name] {
				t.Errorf("%q is exposed but its subresource %q is not; add it to generatedCommands", name, c.Name)
			}
		}
	}
}

type recordedRequest struct {
	method, path string
	header       http.Header
	query        map[string][]string
	body         []byte
}

func runGeneratedContractCase(t *testing.T, resource string, c generatedContractCase) {
	t.Helper()
	isolateGeneratedAuth(t)

	var requests []recordedRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, recordedRequest{
			method: r.Method, path: r.URL.Path, header: r.Header.Clone(),
			query: r.URL.Query(), body: body,
		})
		status, response := c.status, c.response
		if status == 0 {
			status = http.StatusOK
		}
		if response == "" {
			response = "{}"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	defer ts.Close()

	exitCode := 0
	prevExit := exitGenerated
	exitGenerated = func(code int) { exitCode = code }
	defer func() { exitGenerated = prevExit }()

	args := append([]string{"--api-url", ts.URL, "--api-key", "test-key", resource, c.op}, c.args...)
	args = append(args, "--format", "json")
	root := NewRootCmd("dev", "dev")
	var stderr bytes.Buffer
	root.SetArgs(args)
	root.SetIn(strings.NewReader(c.stdin))
	root.SetErr(&stderr)

	var execErr error
	stdout := captureStdout(t, func() { execErr = root.Execute() })
	if execErr != nil {
		t.Fatalf("langsmith %s: %v\nstderr: %s", strings.Join(args, " "), execErr, stderr.String())
	}

	if exitCode != c.wantExitCode {
		t.Errorf("exit code = %d, want %d\nstderr: %s", exitCode, c.wantExitCode, stderr.String())
	}
	wantRequests := 1
	if c.wantMethod == "" {
		wantRequests = c.wantRequests
	}
	if len(requests) != wantRequests {
		t.Fatalf("sent %d requests, want %d", len(requests), wantRequests)
	}
	if wantRequests == 0 {
		if !strings.Contains(stderr.String(), c.wantStderr) {
			t.Errorf("stderr = %q, want it to contain %q", stderr.String(), c.wantStderr)
		}
		return
	}
	got := requests[0]
	if got.method != c.wantMethod || got.path != c.wantPath {
		t.Errorf("request = %s %s, want %s %s", got.method, got.path, c.wantMethod, c.wantPath)
	}
	if key := got.header.Get("X-Api-Key"); key != "test-key" {
		t.Errorf("X-Api-Key = %q, want the --api-key value", key)
	}
	for name, want := range c.wantHeaders {
		if value := got.header.Get(name); value != want {
			t.Errorf("header %s = %q, want %q", name, value, want)
		}
	}
	for key, want := range c.wantQuery {
		if values := got.query[key]; len(values) != 1 || values[0] != want {
			t.Errorf("query %s = %q, want %q", key, values, want)
		}
	}
	assertJSONBody(t, got.body, c.wantBody)
	if !strings.Contains(stdout, c.wantStdout) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, c.wantStdout)
	}
	if !strings.Contains(stderr.String(), c.wantStderr) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), c.wantStderr)
	}
}

// isolateGeneratedAuth keeps the developer's profile and LANGSMITH_* variables
// out of the request, since the Go SDK reads both by default.
func isolateGeneratedAuth(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, key := range []string{"LANGSMITH_API_KEY", "LANGSMITH_ENDPOINT", "LANGSMITH_TENANT_ID", "LANGSMITH_WORKSPACE_ID", "LANGSMITH_PROFILE"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}

func assertJSONBody(t *testing.T, got []byte, want string) {
	t.Helper()
	if want == "" {
		if len(bytes.TrimSpace(got)) != 0 {
			t.Errorf("request body = %s, want none", got)
		}
		return
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("request body %q is not JSON: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		panic(fmt.Sprintf("wantBody %q is not JSON: %v", want, err))
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("request body = %s, want %s", got, want)
	}
}
