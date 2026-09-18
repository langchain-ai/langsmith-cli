package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestProjectCreateCmd_UsesSDKAndWorkspace(t *testing.T) {
	for _, description := range []bool{false, true} {
		t.Run(map[bool]string{false: "no description", true: "description"}[description], func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sessions" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("x-tenant-id") != "demo-workspace" {
					t.Error("missing explicit workspace header")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["name"] != "demo app; $(not-a-command)" {
					t.Errorf("name not preserved: %#v", body["name"])
				}
				if _, ok := body["description"]; ok != description {
					t.Errorf("unexpected description presence: %#v", body)
				}
				if description && body["description"] != "Synthetic traces" {
					t.Errorf("description mismatch: %#v", body)
				}
				if _, ok := body["reference_dataset_id"]; ok {
					t.Error("must create a tracing project, not an experiment")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"new-project","name":"demo app; $(not-a-command)","tenant_id":"demo-workspace"}`))
			})
			defer setupTestEnv(t, ts.URL)()
			flagOutputFormat = "json"
			flagWorkspaceID = "demo-workspace"
			cmd := newProjectCreateCmd()
			args := []string{"--name", "demo app; $(not-a-command)"}
			if description {
				args = append(args, "--description", "Synthetic traces")
			}
			cmd.SetArgs(args)
			var err error
			stdout := captureStdout(t, func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || result["status"] != "created" || result["id"] != "new-project" || result["workspace_id"] != "demo-workspace" {
				t.Fatalf("calls=%d result=%#v", calls, result)
			}
		})
	}
}

func TestProjectCreateOutputAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, format, response, workspace, code string
		status                                  int
	}{
		{"pretty", "pretty", `{"id":"project-id","name":"my-app","tenant_id":"actual-workspace"}`, "selected-workspace", "", 200},
		{"fallback", "json", `{"id":"project-id","name":"my-app"}`, "selected-workspace", "", 200},
		{"unknown", "json", `{"id":"project-id","name":"my-app"}`, "", "", 200},
		{"empty-response", "json", `{}`, "", "project_creation_unverified", 200},
		{"server-error", "json", `{"detail":"private-response-text"}`, "", "project_creation_unverified", 500},
		{"forbidden", "json", `{"detail":"private-response-text"}`, "", "project_permission_denied", 403},
		{"invalid", "json", `{"detail":"private-response-text"}`, "", "invalid_project_request", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			})
			defer setupTestEnv(t, ts.URL)()
			isolateConfig(t)
			t.Setenv("LANGSMITH_WORKSPACE_ID", "")
			flagOutputFormat, flagWorkspaceID = tc.format, tc.workspace
			cmd := newProjectCreateCmd()
			cmd.SetArgs([]string{"--name", "my-app"})
			var err error
			out := captureStdout(t, func() { err = cmd.Execute() })
			if calls != 1 {
				t.Fatalf("unexpected retries: %d", calls)
			}
			if tc.code != "" {
				d, ok := err.(commandDiagnostic)
				if !ok || d.code != tc.code || d.next == "" || out != "" || strings.Contains(d.Error(), "private-response-text") {
					t.Fatalf("incorrect recovery output: %v %s", err, out)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.format == "pretty" {
				if !strings.Contains(out, `Created tracing project "my-app"`) || !strings.Contains(out, "actual-workspace") || strings.Contains(out, "selected-workspace") {
					t.Fatalf("incorrect confirmation: %s", out)
				}
				return
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if tc.workspace == "" && result["workspace_id"] != nil {
				t.Fatal("unknown workspace must remain null")
			}
			if tc.workspace != "" && result["workspace_id"] != tc.workspace {
				t.Fatal("selected workspace missing")
			}
		})
	}
}

func TestProjectCreateCmd_RejectsInvalidArgumentsBeforeRequest(t *testing.T) {
	for _, args := range [][]string{{}, {"--name", ""}, {"--name", "  "}, {"--name", "valid", "extra"}} {
		cmd := newProjectCreateCmd()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("expected validation error for %q", args)
		}
	}
}

func TestProjectCreateCmd_ReportsConflict(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"detail":"Project already exists"}`))
	})
	defer setupTestEnv(t, ts.URL)()
	cmd := newProjectCreateCmd()
	cmd.SetArgs([]string{"--name", "existing"})
	var err error
	stdout := captureStdout(t, func() { err = cmd.Execute() })
	if err == nil || !strings.Contains(err.Error(), "creating tracing project") {
		t.Fatalf("expected contextual API error, got %v", err)
	}
	if strings.Contains(stdout, `"created"`) {
		t.Fatalf("must not report success on conflict: %s", stdout)
	}
}
