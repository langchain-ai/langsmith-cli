package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	lsconfig "github.com/langchain-ai/langsmith-cli/internal/config"
)

const defaultTestProject = "7ca0e2a0-2865-4689-b60f-f6fbfb74bc40"

func setupProjectDefaultConfig(t *testing.T, endpoint string) {
	t.Helper()
	t.Setenv("LANGSMITH_CONFIG_FILE", filepath.Join(t.TempDir(), "config.json"))
	for _, key := range []string{"LANGSMITH_PROJECT", "LANGSMITH_PROFILE", "LANGSMITH_WORKSPACE_ID", "LANGSMITH_TENANT_ID", "LANGSMITH_ENDPOINT", "LANGSMITH_API_KEY", "LANGCHAIN_API_KEY"} {
		t.Setenv(key, "")
	}
	cfg := &lsconfig.Config{CurrentProfile: "demo", Profiles: map[string]lsconfig.Profile{
		"demo":  {APIKey: "test-key", APIURL: endpoint, WorkspaceID: "workspace"},
		"other": {APIKey: "other-key", APIURL: endpoint, WorkspaceID: "workspace"},
	}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDefaultSelectionAndClear(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sessions/"+defaultTestProject {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%q,"name":"renamed-project","tenant_id":"workspace"}`, defaultTestProject)
	})
	defer setupTestEnv(t, ts.URL)()
	setupProjectDefaultConfig(t, ts.URL)
	flagOutputFormat = "json"
	t.Setenv("LANGSMITH_PROJECT", "environment-project")
	cmd := newProjectSetDefaultCmd()
	cmd.SetArgs([]string{defaultTestProject})
	var err error
	stdout := captureStdout(t, func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result["project_id"] != defaultTestProject || len(result["warnings"].([]any)) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	t.Setenv("LANGSMITH_PROJECT", "")
	c, err := getClient()
	if err != nil {
		t.Fatal(err)
	}
	id, err := resolveSessionID(context.Background(), c, "", "", "test")
	if err != nil || id != defaultTestProject {
		t.Fatalf("id=%q error=%v", id, err)
	}
	flagProfile = "other"
	saved, err := savedProjectDefault()
	if err != nil || saved != nil {
		t.Fatalf("profile isolation failed: %#v %v", saved, err)
	}
	flagProfile = "demo"
	captureStdout(t, func() { err = newProjectClearDefaultCmd().Execute() })
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := lsconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles["demo"].ProjectDefault != nil || cfg.Profiles["demo"].APIKey != "test-key" || cfg.Profiles["other"].APIKey != "other-key" {
		t.Fatal("clear modified unrelated profile data")
	}
}

func TestProjectDefaultResolution(t *testing.T) {
	for _, tc := range []struct {
		name, explicit, env, workspace, endpoint string
		status                                   int
		wantErr                                  bool
	}{
		{name: "saved ID survives rename"},
		{name: "workspace mismatch", workspace: "different", wantErr: true},
		{name: "endpoint mismatch", endpoint: "https://example.invalid", wantErr: true},
		{name: "explicit ID overrides mismatch", explicit: defaultTestProject, workspace: "different"},
		{name: "environment overrides mismatch", env: "env-project", workspace: "different"},
		{name: "deleted project", status: 404, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if tc.status != 0 {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, `{}`)
					return
				}
				body := fmt.Sprintf(`{"id":%q,"name":"renamed","tenant_id":"workspace"}`, defaultTestProject)
				if r.URL.Path == "/api/v1/sessions" {
					body = "[" + body + "]"
				}
				fmt.Fprint(w, body)
			})
			defer setupTestEnv(t, ts.URL)()
			setupProjectDefaultConfig(t, ts.URL)
			if _, err := saveProjectDefault(defaultTestProject, "old-name", "workspace"); err != nil {
				t.Fatal(err)
			}
			flagWorkspaceID = tc.workspace
			if tc.endpoint != "" {
				flagAPIURL = tc.endpoint
			}
			t.Setenv("LANGSMITH_PROJECT", tc.env)
			c, err := getClient()
			if err != nil {
				t.Fatal(err)
			}
			id, err := resolveSessionID(context.Background(), c, "", tc.explicit, "test")
			if (err != nil) != tc.wantErr {
				t.Fatalf("id=%q error=%v", id, err)
			}
			if !tc.wantErr && id != defaultTestProject {
				t.Fatalf("unexpected id %q", id)
			}
			if (tc.explicit != "" || (tc.wantErr && tc.status == 0)) && calls != 0 {
				t.Fatal("unexpected discovery request")
			}
		})
	}
}

func TestProjectCreateSetDefault(t *testing.T) {
	for _, missingProfile := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-profile-%t", missingProfile), func(t *testing.T) {
			calls := 0
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sessions" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"id":%q,"name":"new-project","tenant_id":"workspace"}`, defaultTestProject)
			})
			defer setupTestEnv(t, ts.URL)()
			setupProjectDefaultConfig(t, ts.URL)
			flagOutputFormat = "json"
			flagWorkspaceID = "workspace"
			if missingProfile {
				cfg := &lsconfig.Config{}
				if err := cfg.Save(); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newProjectCreateCmd()
			cmd.SetArgs([]string{"--name", "new-project", "--set-default"})
			var err error
			stdout := captureStdout(t, func() { err = cmd.Execute() })
			if (err != nil) != missingProfile {
				t.Fatalf("unexpected error: %v", err)
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || result["id"] != defaultTestProject || result["default_saved"] != !missingProfile {
				t.Fatalf("calls=%d result=%#v", calls, result)
			}
		})
	}
}
