package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHubPush_CreatesRepoAndPostsCommit(t *testing.T) {
	var sawCreate, sawCommit bool
	var commitBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/repos/-/my-skill":
			http.Error(w, "HTTP 404: not found", http.StatusNotFound)
		case r.Method == "POST" && r.URL.Path == "/api/v1/repos":
			sawCreate = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/hub/repos/-/my-skill/directories/commits":
			sawCommit = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &commitBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"commit": map[string]any{
					"id":          "c1",
					"commit_hash": "h1",
					"created_at":  "2026-04-30T00:00:00Z",
				},
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatalf("seed SKILL.md: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "x.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("seed sub/x.txt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref"), 0o644); err != nil {
		t.Fatalf("seed .git/HEAD: %v", err)
	}

	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"push", "my-skill", "--type", "skill", "--dir", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if !sawCreate {
		t.Error("repo create not called")
	}
	if !sawCommit {
		t.Error("directory commit not called")
	}
	files, _ := commitBody["files"].(map[string]any)
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("commit missing SKILL.md")
	}
	if _, ok := files["sub/x.txt"]; !ok {
		t.Error("commit missing sub/x.txt")
	}
	for k := range files {
		if strings.HasPrefix(k, ".git/") {
			t.Errorf(".git contents leaked into commit: %s", k)
		}
	}
	if !strings.Contains(out, `"commit_hash": "h1"`) {
		t.Errorf("missing commit hash in output:\n%s", out)
	}
}

func TestHubPush_RejectsBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{0, 1, 2, 3, 0}, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := readDirectoryAsFiles(dir)
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Errorf("expected binary error, got %v", err)
	}
}

func TestHubPush_ExcludesSecretsAndJunk(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"SKILL.md":      []byte("# hi"),
		".env":          []byte("API_KEY=lsv2_pt_secret"),
		".env.local":    []byte("DB=postgres://user:pw@host"),
		".env.staging":  []byte("STAGING_TOKEN=secret"),
		".env.test":     []byte("TEST_TOKEN=secret"),
		".env.ci":       []byte("CI_TOKEN=secret"),
		".envrc":        []byte("export API_KEY=secret"),
		"id_rsa.pem":    []byte(strings.Join([]string{"-----BEGIN RSA", "PRIVATE KEY-----"}, " ")),
		"server.crt":    []byte("-----BEGIN CERTIFICATE-----"),
		"keystore.p12":  []byte("binary-ish"),
		"Thumbs.db":     []byte("windows junk"),
		"safe.env.json": []byte("{\"ok\": true}"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	got, err := readDirectoryAsFiles(dir)
	if err != nil {
		t.Fatalf("readDirectoryAsFiles: %v", err)
	}
	if _, ok := got["SKILL.md"]; !ok {
		t.Error("SKILL.md should be included")
	}
	if _, ok := got["safe.env.json"]; !ok {
		t.Error("safe.env.json should be included")
	}
	for _, leaked := range []string{".env", ".env.local", ".env.staging", ".env.test", ".env.ci", ".envrc", "id_rsa.pem", "server.crt", "keystore.p12", "Thumbs.db"} {
		if _, ok := got[leaked]; ok {
			t.Errorf("%q should be excluded but was included", leaked)
		}
	}
}

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("plain text")) {
		t.Error("plain text should not be binary")
	}
	if !isBinary([]byte{0x00, 0x01, 0x02}) {
		t.Error("zero byte should mark binary")
	}
}
