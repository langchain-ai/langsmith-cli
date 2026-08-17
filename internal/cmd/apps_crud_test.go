package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAppsList_ListsAllApps(t *testing.T) {
	var sawQuery string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: "a1", Name: "one"},
		})
	})
	defer setupTestEnv(t, srv.URL)()
	flagOutputFormat = "json"

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if sawQuery != "" {
		t.Errorf("expected no query string (no context_type filter), got %q", sawQuery)
	}
	if !strings.Contains(out, `"id": "a1"`) {
		t.Errorf("expected app in output:\n%s", out)
	}

	// The context_type filter flag must be gone.
	listCmd, _, err := newAppsCmd().Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list: %v", err)
	}
	if f := listCmd.Flags().Lookup("context-type"); f != nil {
		t.Error("expected --context-type flag to be gone from apps list")
	}
}

func TestAppsList_PrettyFormatRendersTable(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{
			{ID: "a1", Name: "one", Entrypoint: "dist/bundle.js", IsEnabled: true},
		})
	})
	defer setupTestEnv(t, srv.URL)()
	flagOutputFormat = "pretty"

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	for _, want := range []string{"Custom apps", "one", "a1", "dist/bundle.js"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected table output to contain %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"id": "a1"`) {
		t.Errorf("expected table output, not raw JSON:\n%s", out)
	}
}

func TestAppsListCmd_HasOutputFlag(t *testing.T) {
	listCmd, _, err := newAppsCmd().Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list: %v", err)
	}
	if f := listCmd.Flags().Lookup("output"); f == nil || f.Shorthand != "o" {
		t.Errorf("expected --output/-o flag on apps list, got %+v", f)
	}
}

func TestAppsDelete_SkipsConfirmationWithYes(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	var sawDelete bool
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]customApp{{ID: id, Name: "one"}})
		case r.Method == "DELETE" && r.URL.Path == "/v1/platform/custom-apps/"+id:
			sawDelete = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"delete", id, "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !sawDelete {
		t.Error("expected DELETE request")
	}
	if !strings.Contains(out, `"status": "deleted"`) {
		t.Errorf("expected deleted status:\n%s", out)
	}
}

func TestAppsDelete_ResolvesNameToID(t *testing.T) {
	const id = "33333333-3333-3333-3333-333333333333"
	var deletePath string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/platform/custom-apps":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]customApp{
				{ID: "44444444-4444-4444-4444-444444444444", Name: "other"},
				{ID: id, Name: "My App"},
			})
		case r.Method == "DELETE":
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"delete", "My App", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if deletePath != "/v1/platform/custom-apps/"+id {
		t.Errorf("expected the name resolved to its ID before deleting, got %q", deletePath)
	}
	if !strings.Contains(out, `"name": "My App"`) {
		t.Errorf("expected the resolved name in the output:\n%s", out)
	}
}

func TestAppsDelete_RejectsUnknownName(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			t.Error("delete should not run for an unknown name")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{{ID: "55555555-5555-5555-5555-555555555555", Name: "one"}})
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"delete", "myapp", "--yes"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no custom app named") {
		t.Fatalf("expected an unknown-name error, got %v", err)
	}
}

func TestAppsDelete_RejectsMissingApp(t *testing.T) {
	const id = "22222222-2222-2222-2222-222222222222"
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			t.Errorf("delete should not run for a missing app")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]customApp{})
	})
	defer setupTestEnv(t, srv.URL)()

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"delete", id, "--yes"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found rejection, got %v", err)
	}
}
