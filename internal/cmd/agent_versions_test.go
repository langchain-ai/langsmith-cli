package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ==================== Command structure ====================

func TestAgentVersionsCmd_Subcommands(t *testing.T) {
	cmd := newAgentVersionsCmd()
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	if !names["list"] {
		t.Error("agent-versions missing subcommand 'list'")
	}
}

func TestAgentVersionsCmd_UseField(t *testing.T) {
	cmd := newAgentVersionsCmd()
	if cmd.Use != "agent-versions" {
		t.Errorf("expected Use=agent-versions, got %q", cmd.Use)
	}
}

// ==================== agent-versions list flags ====================

func TestAgentVersionsListCmd_Flags(t *testing.T) {
	cmd := newAgentVersionsListCmd()
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

func TestAgentVersionsListCmd_UseField(t *testing.T) {
	cmd := newAgentVersionsListCmd()
	if cmd.Use != "list" {
		t.Errorf("expected Use=list, got %q", cmd.Use)
	}
}

// ==================== agent-versions list execution ====================

func newAgentVersionsTestServer(t *testing.T, sessions map[string]string, versions map[string][]map[string]any) *httptest.Server {
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
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": id, "name": name}})
		case strings.HasPrefix(r.URL.Path, "/v1/platform/sessions/") && strings.HasSuffix(r.URL.Path, "/agent-versions") && r.Method == "GET":
			parts := strings.Split(r.URL.Path, "/")
			sessionID := parts[len(parts)-2]
			v, ok := versions[sessionID]
			if !ok {
				_ = json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			_ = json.NewEncoder(w).Encode(v)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})
}

func TestAgentVersionsList_JSONOutput(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000001"
	ts := newAgentVersionsTestServer(t,
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
		cmd := newAgentVersionsListCmd()
		cmd.SetArgs([]string{"--project", "my-agent"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 agent versions, got %d", len(result))
	}
	if result[0]["commit_sha"] != "abc123" {
		t.Errorf("expected first commit_sha=abc123, got %v", result[0]["commit_sha"])
	}
	if result[1]["commit_sha"] != "def456" {
		t.Errorf("expected second commit_sha=def456, got %v", result[1]["commit_sha"])
	}
}

func TestAgentVersionsList_EmptyResult(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000002"
	ts := newAgentVersionsTestServer(t,
		map[string]string{"no-versions": sessionID},
		map[string][]map[string]any{},
	)
	cleanup := setupTestEnv(t, ts.URL)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := newAgentVersionsListCmd()
		cmd.SetArgs([]string{"--project", "no-versions"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 agent versions, got %d", len(result))
	}
}


func TestAgentVersionsList_FieldsPresent(t *testing.T) {
	sessionID := "aaaaaaaa-0000-0000-0000-000000000003"
	ts := newAgentVersionsTestServer(t,
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
		cmd := newAgentVersionsListCmd()
		cmd.SetArgs([]string{"--project", "my-agent"})
		_ = cmd.Execute()
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nout: %s", err, out)
	}
	if len(result) == 0 {
		t.Fatal("expected at least one agent version")
	}
	entry := result[0]
	if _, ok := entry["commit_sha"]; !ok {
		t.Error("agent version entry missing 'commit_sha' field")
	}
	if _, ok := entry["first_seen_at"]; !ok {
		t.Error("agent version entry missing 'first_seen_at' field")
	}
}
