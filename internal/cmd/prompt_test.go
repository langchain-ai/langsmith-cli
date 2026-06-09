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
		if r.Method != http.MethodPost || r.URL.Path != "/commits/acme/my-prompt" {
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
		if r.Method != http.MethodPost || r.URL.Path != "/commits/acme/my-prompt" {
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
