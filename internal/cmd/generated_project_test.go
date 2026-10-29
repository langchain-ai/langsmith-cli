package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const generatedProjectID = "11111111-2222-3333-4444-555555555555"

// fakeProjectAPI answers a project lookup by name with generatedProjectID and
// records every other request as "METHOD path?query".
func fakeProjectAPI(t *testing.T, lookups, requests *[]string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/sessions" {
			*lookups = append(*lookups, r.URL.Query().Get("name"))
			_, _ = w.Write([]byte(`[{"id":"` + generatedProjectID + `","name":"` + r.URL.Query().Get("name") + `"}]`))
			return
		}
		*requests = append(*requests, r.Method+" "+r.URL.RequestURI())
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestGeneratedCommandsSelectProjectByName(t *testing.T) {
	const otherID = "99999999-8888-7777-6666-555555555555"
	tests := []struct {
		name        string
		env         string
		stdin       string
		args        []string
		wantLookups []string
		wantRequest string
		wantErr     string
	}{
		{
			name:        "--project fills a path parameter",
			args:        []string{"projects", "dashboard", "--project", "my-proj"},
			wantLookups: []string{"my-proj"},
			wantRequest: "POST /api/v1/sessions/" + generatedProjectID + "/dashboard",
		},
		{
			name:        "LANGSMITH_PROJECT fills a required project",
			env:         "env-proj",
			args:        []string{"projects", "dashboard"},
			wantLookups: []string{"env-proj"},
			wantRequest: "POST /api/v1/sessions/" + generatedProjectID + "/dashboard",
		},
		{
			name:        "a positional ID wins over LANGSMITH_PROJECT",
			env:         "env-proj",
			args:        []string{"projects", "dashboard", otherID},
			wantRequest: "POST /api/v1/sessions/" + otherID + "/dashboard",
		},
		{
			name:        "--session-id wins over LANGSMITH_PROJECT",
			env:         "env-proj",
			args:        []string{"projects", "dashboard", "--session-id", otherID},
			wantRequest: "POST /api/v1/sessions/" + otherID + "/dashboard",
		},
		{
			name:        "a positional run ID leaves the project to LANGSMITH_PROJECT",
			env:         "env-proj",
			args:        []string{"runs", "share", "create", otherID, "--trace-id", otherID},
			wantLookups: []string{"env-proj"},
			wantRequest: "POST /api/v2/runs/" + otherID + "/share",
		},
		{
			name:        "a piped project wins over LANGSMITH_PROJECT",
			env:         "env-proj",
			stdin:       `{"session_id":"` + otherID + `","name":"renamed"}`,
			args:        []string{"projects", "update"},
			wantRequest: "PATCH /api/v1/sessions/" + otherID,
		},
		{
			name:        "--project-id stands in for --session-id without a lookup",
			args:        []string{"projects", "dashboard", "--project-id=" + otherID},
			wantRequest: "POST /api/v1/sessions/" + otherID + "/dashboard",
		},
		{
			name:        "--project fills a generated --project-id",
			args:        []string{"threads", "query", "--project", "my-proj"},
			wantLookups: []string{"my-proj"},
			wantRequest: "POST /api/v2/threads/query",
		},
		{
			name:        "LANGSMITH_PROJECT fills a project the server requires",
			env:         "env-proj",
			args:        []string{"threads", "query"},
			wantLookups: []string{"env-proj"},
			wantRequest: "POST /api/v2/threads/query",
		},
		{
			name: "an optional project filter ignores LANGSMITH_PROJECT",
			env:  "env-proj",
			args: []string{"online-evaluators", "spend", "--group-by", "evaluator", "--period-start", "2026-01-01"},
		},
		{
			name:    "--project with --session-id",
			args:    []string{"projects", "dashboard", "--project", "my-proj", "--session-id", otherID},
			wantErr: "can't be combined with --session-id",
		},
		{
			name:    "malformed --project-id",
			args:    []string{"projects", "dashboard", "--project-id", "my-proj"},
			wantErr: "must be a project (session) UUID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each CLI invocation needs fresh generated flag state.
			const caseEnv = "LANGSMITH_CLI_PROJECT_CASE"
			if os.Getenv(caseEnv) != t.Name() {
				child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$")
				child.Env = append(os.Environ(), caseEnv+"="+t.Name())
				if tt.stdin != "" {
					child.Stdin = strings.NewReader(tt.stdin)
				}
				if out, err := child.CombinedOutput(); err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				return
			}
			isolateGeneratedAuth(t)
			t.Setenv("LANGSMITH_PROJECT", tt.env)
			var lookups, requests []string
			ts := fakeProjectAPI(t, &lookups, &requests)
			args := append([]string{"--api-url", ts.URL, "--api-key", "k"}, tt.args...)

			stderr, exitCode, err := runGeneratedCommand(t, "", args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantErr)
				}
				if len(requests) != 0 {
					t.Errorf("sent %q, want no request", requests)
				}
				return
			}
			if err != nil || exitCode != 0 {
				t.Fatalf("err = %v, exit code %d, stderr: %s", err, exitCode, stderr)
			}
			if strings.Join(lookups, ",") != strings.Join(tt.wantLookups, ",") {
				t.Errorf("looked up %q, want %q", lookups, tt.wantLookups)
			}
			if tt.wantRequest != "" && (len(requests) != 1 || !strings.HasPrefix(requests[0], tt.wantRequest)) {
				t.Errorf("requests = %q, want one %q", requests, tt.wantRequest)
			}
		})
	}
}
