package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ==================== extractOverviewTemplate ====================

func TestExtractOverviewTemplate_PromptTemplate(t *testing.T) {
	manifest := map[string]interface{}{
		"kwargs": map[string]interface{}{
			"template": "# Agent Overview\n\n## Purpose\n...",
		},
	}
	got := extractOverviewTemplate(manifest)
	want := "# Agent Overview\n\n## Purpose\n..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractOverviewTemplate_ChatPromptTemplate(t *testing.T) {
	manifest := map[string]interface{}{
		"kwargs": map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{
					"kwargs": map[string]interface{}{
						"prompt": map[string]interface{}{
							"kwargs": map[string]interface{}{
								"template": "# Agent Overview (hub-edited)",
							},
						},
					},
				},
			},
		},
	}
	got := extractOverviewTemplate(manifest)
	want := "# Agent Overview (hub-edited)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractOverviewTemplate_RunnableSequence(t *testing.T) {
	manifest := map[string]interface{}{
		"kwargs": map[string]interface{}{
			"first": map[string]interface{}{
				"kwargs": map[string]interface{}{
					"template": "# Agent Overview (prompt+model)",
				},
			},
		},
	}
	got := extractOverviewTemplate(manifest)
	want := "# Agent Overview (prompt+model)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractOverviewTemplate_Unreadable(t *testing.T) {
	for _, manifest := range []map[string]interface{}{
		nil,
		{},
		{"kwargs": map[string]interface{}{}},
		{"kwargs": map[string]interface{}{"messages": []interface{}{}}},
	} {
		if got := extractOverviewTemplate(manifest); got != "" {
			t.Errorf("extractOverviewTemplate(%v) = %q, want empty", manifest, got)
		}
	}
}

// ==================== overviewRepoHandle ====================

func TestOverviewRepoHandle(t *testing.T) {
	cases := []struct {
		sessionID string
		want      string
	}{
		{"abcdef1234567890", "ao-abcdef12"},
		{"short", "ao-short"},
	}
	for _, tc := range cases {
		if got := overviewRepoHandle(tc.sessionID); got != tc.want {
			t.Errorf("overviewRepoHandle(%q) = %q, want %q", tc.sessionID, got, tc.want)
		}
	}
}

// ==================== overview pull ====================

// newOverviewTestServer mocks /sessions (project name -> session ID) and the
// commits get-latest endpoint for the derived ao-<id> repo handle.
func newOverviewTestServer(t *testing.T, sessions map[string]string, manifest map[string]interface{}, commitHash string) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		id, ok := sessions[name]
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": id, "name": name}})
	})
	mux.HandleFunc("/api/v1/commits/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if manifest == nil {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": commitHash,
			"manifest":    manifest,
		})
	})
	return mux
}

func TestOverviewPull_WritesFileAndReportsStatus(t *testing.T) {
	manifest := map[string]interface{}{
		"kwargs": map[string]interface{}{"template": "# Agent Overview\n\nbody"},
	}
	mux := newOverviewTestServer(t, map[string]string{"my-app": "session-abcdef1234"}, manifest, "hash123")
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()
	flagOutputFormat = "json"

	dir := t.TempDir()
	out := filepath.Join(dir, "overview.md")

	cmd := newProjectIssuesOverviewPullCmd()
	_ = cmd.Flags().Set("project", "my-app")
	_ = cmd.Flags().Set("output", out)

	stdout := captureStdout(t, func() { cmd.Run(cmd, nil) })
	if !strings.Contains(stdout, `"status": "pulled"`) {
		t.Errorf("missing pulled status in output:\n%s", stdout)
	}

	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(content) != "# Agent Overview\n\nbody" {
		t.Errorf("file content = %q", content)
	}
}

func TestOverviewPull_NoOutputFile_PrintsToStdout(t *testing.T) {
	manifest := map[string]interface{}{
		"kwargs": map[string]interface{}{"template": "plain markdown"},
	}
	mux := newOverviewTestServer(t, map[string]string{"my-app": "session-abcdef1234"}, manifest, "hash123")
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	cmd := newProjectIssuesOverviewPullCmd()
	_ = cmd.Flags().Set("project", "my-app")

	stdout := captureStdout(t, func() { cmd.Run(cmd, nil) })
	if stdout != "plain markdown" {
		t.Errorf("stdout = %q, want %q", stdout, "plain markdown")
	}
}
