package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ==================== Command structure ====================

func TestDeploymentCmd_Subcommands(t *testing.T) {
	cmd := newDeploymentCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	if !names["list"] {
		t.Error("deployment missing subcommand 'list'")
	}
}

func TestDeploymentCmd_UseField(t *testing.T) {
	cmd := newDeploymentCmd()
	if cmd.Use != "deployment" {
		t.Errorf("expected Use=deployment, got %q", cmd.Use)
	}
}

// ==================== deployment list flags ====================

func TestDeploymentListCmd_Flags(t *testing.T) {
	cmd := newDeploymentListCmd()
	tests := []struct {
		name   string
		defVal string
		short  string
	}{
		{"project", "", "p"},
		{"output", "", "o"},
	}
	for _, tc := range tests {
		f := cmd.Flags().Lookup(tc.name)
		if f == nil {
			t.Errorf("flag --%s not found", tc.name)
			continue
		}
		if f.DefValue != tc.defVal {
			t.Errorf("flag --%s: expected default %q, got %q", tc.name, tc.defVal, f.DefValue)
		}
		if tc.short != "" && f.Shorthand != tc.short {
			t.Errorf("flag --%s: expected shorthand %q, got %q", tc.name, tc.short, f.Shorthand)
		}
	}
}

func TestDeploymentListCmd_UseField(t *testing.T) {
	cmd := newDeploymentListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// ==================== deployment list execution ====================

func newDeploymentTestServer(t *testing.T, sessions map[string]string, deployments map[string][]map[string]any) *httptest.Server {
	t.Helper()
	return newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sessions" && r.Method == "GET":
			name := r.URL.Query().Get("name")
			id, ok := sessions[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": id, "name": name}},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/deployments") && r.Method == "GET":
			parts := strings.Split(r.URL.Path, "/")
			sessionID := parts[len(parts)-2]
			d, ok := deployments[sessionID]
			if !ok {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			_ = json.NewEncoder(w).Encode(d)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func TestDeploymentList_JSONOutput(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000001"
	ts := newDeploymentTestServer(t,
		map[string]string{"my-agent": sessionID},
		map[string][]map[string]any{
			sessionID: {
				{"commit_sha": "abc123", "first_seen_at": "2026-03-17T10:00:00Z"},
				{"commit_sha": "def456", "first_seen_at": "2026-03-15T08:30:00Z"},
			},
		},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newDeploymentListCmd()
		cmd.SetArgs([]string{"--project", "my-agent"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 deployments, got %d", len(result))
	}
	if result[0]["commit_sha"] != "abc123" {
		t.Errorf("expected first commit_sha=abc123, got %v", result[0]["commit_sha"])
	}
	if result[1]["commit_sha"] != "def456" {
		t.Errorf("expected second commit_sha=def456, got %v", result[1]["commit_sha"])
	}
}

func TestDeploymentList_EmptyResult(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000002"
	ts := newDeploymentTestServer(t,
		map[string]string{"no-deployments": sessionID},
		map[string][]map[string]any{},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newDeploymentListCmd()
		cmd.SetArgs([]string{"--project", "no-deployments"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 deployments, got %d", len(result))
	}
}

func TestDeploymentList_MissingProjectFlag(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	// Should exit with error when --project is not provided
	out, err := executeCommand(t, "deployment", "list")
	if err == nil && !strings.Contains(out, "error") {
		t.Error("expected error when --project is missing")
	}
}

func TestDeploymentList_FieldsPresent(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000003"
	ts := newDeploymentTestServer(t,
		map[string]string{"my-agent": sessionID},
		map[string][]map[string]any{
			sessionID: {
				{"commit_sha": "abc123", "first_seen_at": "2026-03-17T10:00:00Z"},
			},
		},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newDeploymentListCmd()
		cmd.SetArgs([]string{"--project", "my-agent"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one deployment")
	}
	entry := result[0]
	if _, ok := entry["commit_sha"]; !ok {
		t.Error("deployment entry missing 'commit_sha' field")
	}
	if _, ok := entry["first_seen_at"]; !ok {
		t.Error("deployment entry missing 'first_seen_at' field")
	}
}
