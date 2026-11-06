package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// TestPromptPush_SendsDescription verifies that the --description flag is
// forwarded to the commit-create request body.
func TestPromptPush_SendsDescription(t *testing.T) {
	var gotBody map[string]any

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/commits/acme/my-prompt" {
			http.NotFound(w, r)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Fatalf("unmarshaling request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit": map[string]any{
				"id":          "11111111-1111-1111-1111-111111111111",
				"commit_hash": "abc123",
				"created_at":  "2026-06-08T12:00:00Z",
			},
		})
	})

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"foo":"bar"}`), 0o600); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	captureStdout(t, func() {
		if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
			"prompt", "push", "acme/my-prompt",
			"--file", manifestPath,
			"--description", "tightened system prompt"); err != nil {
			t.Fatalf("push command returned error: %v", err)
		}
	})

	if gotBody == nil {
		t.Fatal("server never received the commit request")
	}
	if got, want := gotBody["description"], "tightened system prompt"; got != want {
		t.Fatalf("description in request body = %v, want %q", got, want)
	}
	if _, ok := gotBody["manifest"]; !ok {
		t.Fatalf("manifest missing from request body: %v", gotBody)
	}
}

// TestPromptPush_OmitsDescriptionWhenUnset verifies that no description field
// is sent when the flag is not provided.
func TestPromptPush_OmitsDescriptionWhenUnset(t *testing.T) {
	var gotBody map[string]any

	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/commits/acme/my-prompt" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit": map[string]any{
				"id":          "11111111-1111-1111-1111-111111111111",
				"commit_hash": "abc123",
				"created_at":  "2026-06-08T12:00:00Z",
			},
		})
	})

	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"foo":"bar"}`), 0o600); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	captureStdout(t, func() {
		if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
			"prompt", "push", "acme/my-prompt",
			"--file", manifestPath); err != nil {
			t.Fatalf("push command returned error: %v", err)
		}
	})

	if _, ok := gotBody["description"]; ok {
		t.Fatalf("description should be omitted when --description is unset, body: %v", gotBody)
	}
}

// TestPromptCommits_ShowsDescription verifies the commits read path surfaces
// the per-commit description in JSON output.
func TestPromptCommits_ShowsDescription(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/commits/acme/my-prompt" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Return the commit only on the first page; an empty second page
		// terminates the offset pager.
		if off := r.URL.Query().Get("offset"); off != "" && off != "0" {
			_ = json.NewEncoder(w).Encode(map[string]any{"total": 1, "commits": []map[string]any{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total": 1,
			"commits": []map[string]any{
				{
					"id":          "11111111-1111-1111-1111-111111111111",
					"commit_hash": "abc123",
					"description": "first commit via CLI",
					"created_at":  "2026-06-08T12:00:00Z",
				},
			},
		})
	})

	out := captureStdout(t, func() {
		if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
			"prompt", "commits", "acme/my-prompt", "--format", "json"); err != nil {
			t.Fatalf("commits command returned error: %v", err)
		}
	})

	var got []map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parsing commits output %q: %v", out, err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 commit, got %d: %v", len(got), got)
	}
	if d := got[0]["description"]; d != "first commit via CLI" {
		t.Fatalf("description in commits output = %v, want %q", d, "first commit via CLI")
	}
}

// TestPromptCommits_Cursor verifies --cursor sends page_size and cursor without
// limit or offset (the server rejects the mix) and adds cursors.next to the
// output.
func TestPromptCommits_Cursor(t *testing.T) {
	var got []url.Values
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total":       5,
			"commits":     []map[string]any{{"commit_hash": "abc123"}, {"commit_hash": "def456"}},
			"next_cursor": "c-next",
		})
	})

	out := captureStdout(t, func() {
		if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
			"prompt", "commits", "acme/my-prompt", "--format", "json", "--limit", "2", "--cursor", "c-prev"); err != nil {
			t.Fatalf("commits command returned error: %v", err)
		}
	})

	if len(got) != 1 {
		t.Fatalf("want one request, got %v", got)
	}
	if q := got[0]; q.Get("cursor") != "c-prev" || q.Get("page_size") != "2" || q.Has("limit") || q.Has("offset") {
		t.Errorf("query = %v, want cursor=c-prev, page_size=2 and no limit/offset", q)
	}
	var body struct {
		Commits []map[string]any `json:"commits"`
		Cursors map[string]any   `json:"cursors"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("parsing %q: %v", out, err)
	}
	if len(body.Commits) != 2 || body.Cursors["next"] != "c-next" {
		t.Errorf("output = %+v, want 2 commits and cursors.next c-next", body)
	}
}

// TestPromptDelete_AcceptsJSONAndNoContent verifies that delete succeeds whether
// the API answers 200 with a JSON body or 204 No Content.
func TestPromptDelete_AcceptsJSONAndNoContent(t *testing.T) {
	tests := []struct {
		name  string
		write func(w http.ResponseWriter)
	}{
		{
			name: "200 with JSON",
			write: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			},
		},
		{
			name: "204 No Content",
			write: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusNoContent)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleted := false
			ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/repos/acme/my-prompt" {
					http.NotFound(w, r)
					return
				}
				deleted = true
				tt.write(w)
			})

			out := captureStdout(t, func() {
				if _, err := executeCommand(t, "--api-key", "test-key", "--api-url", ts.URL,
					"prompt", "delete", "acme/my-prompt", "--yes"); err != nil {
					t.Fatalf("delete command returned error: %v", err)
				}
			})

			if !deleted {
				t.Fatal("server never received the delete request")
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("unmarshaling output %q: %v", out, err)
			}
			if got["status"] != "deleted" || got["owner"] != "acme" || got["repo"] != "my-prompt" {
				t.Fatalf("output = %v, want deleted acme/my-prompt", got)
			}
		})
	}
}
