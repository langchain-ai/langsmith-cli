package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// generatedTestsDir holds one YAML file of tests per generated resource
// command. The files are generated with the commands; see
// internal/generated/README.md.
var generatedTestsDir = filepath.Join("..", "generated", "tests")

type generatedTestFile struct {
	// Resource is the generated command, for example "prompt-webhooks".
	Resource  string                  `yaml:"resource"`
	Contract  []generatedContractCase `yaml:"contract"`
	Lifecycle generatedLifecycle      `yaml:"lifecycle"`
	// LifecycleSkip explains why the resource has no live lifecycle test.
	LifecycleSkip string `yaml:"lifecycle_skip"`
}

// generatedContractCase is one `langsmith <resource> <op>` invocation against
// a fake API and what it must send and print.
type generatedContractCase struct {
	Name string `yaml:"name"`
	// Op is the generated operation, for example "create".
	Op string `yaml:"op"`
	// Args follow `langsmith <resource> <op>`.
	Args []string `yaml:"args"`
	// Stdin is fed to the command, for example "y\n" to confirm a delete.
	Stdin    string `yaml:"stdin"`
	Response struct {
		// Status defaults to 200.
		Status int `yaml:"status"`
		// Body is returned as JSON; it defaults to {}.
		Body any `yaml:"body"`
	} `yaml:"response"`
	Want struct {
		Method  string            `yaml:"method"`
		Path    string            `yaml:"path"`
		Query   map[string]string `yaml:"query"`
		Headers map[string]string `yaml:"headers"`
		// Body is the exact JSON request body; omitted means no body.
		Body any `yaml:"body"`
		// Requests is the number of requests sent; it defaults to 1.
		Requests *int `yaml:"requests"`
		// ExitCode is the process exit code; it defaults to 0.
		ExitCode int `yaml:"exit_code"`
		// Error is a substring of the error returned before any request,
		// for example an aborted delete.
		Error string `yaml:"error"`
		// Stdout and Stderr are substrings the output must contain.
		Stdout string `yaml:"stdout"`
		Stderr string `yaml:"stderr"`
	} `yaml:"want"`
}

// generatedLifecycle runs every operation of a resource against a live API
// (-tags=integration), each step as its own `langsmith <resource>` process.
// Args may use {{run}} (unique per run) and {{name}} for captured values.
type generatedLifecycle struct {
	Steps []generatedLifecycleStep `yaml:"steps"`
	// Cleanup runs after the steps, pass or fail, to delete what they made.
	Cleanup []generatedLifecycleStep `yaml:"cleanup"`
}

type generatedLifecycleStep struct {
	Name string `yaml:"name"`
	// Args follow `langsmith <resource>`.
	Args []string `yaml:"args"`
	// ExitCode is the expected exit code; it defaults to 0.
	ExitCode int `yaml:"exit_code"`
	// Capture stores JSON output values (GJSON paths) for later steps.
	Capture map[string]string `yaml:"capture"`
	// Expect maps GJSON paths in the output to their expected values.
	Expect map[string]string `yaml:"expect"`
	// Exists lists GJSON paths that must match something in the output.
	Exists []string `yaml:"exists"`
	// Forget marks captured values as cleaned up, so cleanup skips them.
	Forget []string `yaml:"forget"`
	// Needs skips a cleanup step unless these values were captured and not
	// forgotten.
	Needs []string `yaml:"needs"`
}

// TestEveryGeneratedResourceHasALifecycle keeps each resource either tested
// against a live API or explicitly excluded with a reason.
func TestEveryGeneratedResourceHasALifecycle(t *testing.T) {
	tests := loadGeneratedTests(t)
	for _, resource := range generatedResources() {
		file := tests[resource.Name]
		if len(file.Lifecycle.Steps) == 0 && strings.TrimSpace(file.LifecycleSkip) == "" {
			t.Errorf("%q has no lifecycle steps and no lifecycle_skip reason", resource.Name)
		}
	}
}

// loadGeneratedTests returns the test files keyed by resource command.
func loadGeneratedTests(t *testing.T) map[string]generatedTestFile {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(generatedTestsDir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]generatedTestFile{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file generatedTestFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if file.Resource == "" {
			t.Fatalf("%s: missing resource", path)
		}
		if _, dup := files[file.Resource]; dup {
			t.Fatalf("%s: second test file for %q", path, file.Resource)
		}
		files[file.Resource] = file
	}
	return files
}

func TestGeneratedCommandContracts(t *testing.T) {
	tests := loadGeneratedTests(t)
	for _, resource := range generatedResources() {
		for _, c := range tests[resource.Name].Contract {
			t.Run(resource.Name+"/"+c.Name, func(t *testing.T) {
				runGeneratedContractCase(t, resource.Name, c)
			})
		}
	}
}

// TestEveryGeneratedOperationHasAContractCase keeps a resource from shipping
// with an operation that no test has run, and tests from outliving their
// resource.
func TestEveryGeneratedOperationHasAContractCase(t *testing.T) {
	tests := loadGeneratedTests(t)
	generatedNames := map[string]bool{}
	for _, resource := range generatedResources() {
		generatedNames[resource.Name] = true
		covered := map[string]bool{}
		for _, c := range tests[resource.Name].Contract {
			covered[c.Op] = true
		}
		for _, op := range resource.Commands {
			// urfave/cli adds a help subcommand to every resource.
			if op.Name != "help" && !covered[op.Name] {
				t.Errorf("'langsmith %s %s' has no contract case", resource.Name, op.Name)
			}
		}
	}
	for name := range tests {
		if !generatedNames[name] {
			t.Errorf("tests exist for %q, which is not a generated command", name)
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

	response := []byte("{}")
	switch body := c.Response.Body.(type) {
	case nil:
	case string:
		response = []byte(body)
	default:
		var err error
		if response, err = json.Marshal(body); err != nil {
			t.Fatalf("response body: %v", err)
		}
	}
	status := c.Response.Status
	if status == 0 {
		status = http.StatusOK
	}

	var requests []recordedRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, recordedRequest{
			method: r.Method, path: r.URL.Path, header: r.Header.Clone(),
			query: r.URL.Query(), body: body,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	defer ts.Close()

	exitCode := 0
	prevExit := exitGenerated
	exitGenerated = func(code int) { exitCode = code }
	defer func() { exitGenerated = prevExit }()

	args := append([]string{"--api-url", ts.URL, "--api-key", "test-key", resource, c.Op}, c.Args...)
	args = append(args, "--format", "json")
	root := NewRootCmd("dev", "dev")
	var stderr bytes.Buffer
	root.SetArgs(args)
	root.SetIn(strings.NewReader(c.Stdin))
	root.SetErr(&stderr)

	var execErr error
	stdout := captureStdout(t, func() { execErr = root.Execute() })
	want := c.Want
	switch {
	case want.Error != "" && (execErr == nil || !strings.Contains(execErr.Error(), want.Error)):
		t.Errorf("error = %v, want it to contain %q", execErr, want.Error)
	case want.Error == "" && execErr != nil:
		t.Fatalf("langsmith %s: %v\nstderr: %s", strings.Join(args, " "), execErr, stderr.String())
	}

	if exitCode != want.ExitCode {
		t.Errorf("exit code = %d, want %d\nstderr: %s", exitCode, want.ExitCode, stderr.String())
	}
	if !strings.Contains(stdout, want.Stdout) {
		t.Errorf("stdout = %q, want it to contain %q", stdout, want.Stdout)
	}
	if !strings.Contains(stderr.String(), want.Stderr) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want.Stderr)
	}

	wantRequests := 1
	if want.Requests != nil {
		wantRequests = *want.Requests
	}
	if len(requests) != wantRequests {
		t.Fatalf("sent %d requests, want %d", len(requests), wantRequests)
	}
	if wantRequests == 0 {
		return
	}
	got := requests[0]
	if got.method != want.Method || got.path != want.Path {
		t.Errorf("request = %s %s, want %s %s", got.method, got.path, want.Method, want.Path)
	}
	if key := got.header.Get("X-Api-Key"); key != "test-key" {
		t.Errorf("X-Api-Key = %q, want the --api-key value", key)
	}
	for name, value := range want.Headers {
		if gotValue := got.header.Get(name); gotValue != value {
			t.Errorf("header %s = %q, want %q", name, gotValue, value)
		}
	}
	for key, value := range want.Query {
		if values := got.query[key]; len(values) != 1 || values[0] != value {
			t.Errorf("query %s = %q, want %q", key, values, value)
		}
	}
	assertJSONBody(t, got.body, want.Body)
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

// assertJSONBody compares a request body with a YAML value as JSON.
func assertJSONBody(t *testing.T, got []byte, want any) {
	t.Helper()
	if want == nil {
		if len(bytes.TrimSpace(got)) != 0 {
			t.Errorf("request body = %s, want none", got)
		}
		return
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("want body: %v", err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("request body %q is not JSON: %v", got, err)
	}
	if err := json.Unmarshal(wantJSON, &wantValue); err != nil {
		t.Fatalf("want body: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("request body = %s, want %s", got, wantJSON)
	}
}
