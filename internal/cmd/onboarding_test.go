package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
)

func TestWorkflowFailureReturnsOnlyTypedError(t *testing.T) {
	old := flagOutputFormat
	flagOutputFormat = "json"
	defer func() { flagOutputFormat = old }()
	var err error
	out := captureStdout(t, func() { err = workflowFailure("trace_not_found", "No trace found", "Run the app") })
	if out != "" {
		t.Fatal("error leaked to stdout")
	}
	if _, ok := err.(commandDiagnostic); !ok {
		t.Fatalf("untyped error %T", err)
	}
}

func TestOnboardingInitAndContext(t *testing.T) {
	t.Chdir(t.TempDir())
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/sessions/"+deleteTestProjectID {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%q,"name":"fixture"}`, deleteTestProjectID)
	})
	defer setupTestEnv(t, ts.URL)()
	flagWorkspaceID = "22222222-2222-4222-8222-222222222222"
	cmd := newOnboardingInitCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID})
	var err error
	captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(projectContextFile)
	if err != nil {
		t.Fatal(err)
	}
	var saved projectContext
	if err = json.Unmarshal(content, &saved); err != nil || saved.ProjectID != deleteTestProjectID || saved.APIURL != ts.URL {
		t.Fatalf("invalid context: %s %v", content, err)
	}
	again := newOnboardingInitCmd()
	again.SetArgs([]string{"--project-id", deleteTestProjectID})
	if again.Execute() == nil {
		t.Fatal("existing context overwritten")
	}
	root := NewRootCmd("test", "test")
	flagAPIURL = ts.URL
	if err = loadProjectContext(root); err != nil {
		t.Fatal(err)
	}
	if activeProjectContext == nil || activeProjectContext.ProjectID != deleteTestProjectID {
		t.Fatal("context was not loaded")
	}
	t.Cleanup(func() { activeProjectContext = nil })
}
func TestOnboardingRejectsInvalidContext(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, data := range []string{"invalid", `{"version":99}`, `{"version":1,"project_id":"invalid"}`} {
		if err := os.WriteFile(projectContextFile, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := loadProjectContext(NewRootCmd("test", "test")); err == nil {
			t.Fatal("invalid context accepted")
		}
	}
}
func TestTraceVerifyEmptyProject(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/query" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"runs":[]}`)
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newTraceVerifyCmd()
	cmd.SetArgs([]string{"--project-id", deleteTestProjectID})
	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	if err == nil || out != "" {
		t.Fatalf("empty project reported success: %s %v", out, err)
	}
}
