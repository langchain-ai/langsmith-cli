package cmd

import (
	"encoding/json"
	"io"
	"net/http"
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
