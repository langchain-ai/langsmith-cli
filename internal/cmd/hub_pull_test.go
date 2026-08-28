package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHubPull_WritesFilesAndReportsLinks(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/platform/hub/repos/acme/my-skill/directories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("commit") != "production" {
			t.Errorf("commit = %q", r.URL.Query().Get("commit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_id":   "c1",
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md":     map[string]any{"type": "file", "content": "# hi"},
				"sub/x.txt":    map[string]any{"type": "file", "content": "abc"},
				"linked-child": map[string]any{"type": "skill", "repo_handle": "child", "owner": "acme", "commit_hash": "deadbeef"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	out := captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"pull", "acme/my-skill:production", "--dir", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if data, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md: %v", err)
	} else if string(data) != "# hi" {
		t.Errorf("SKILL.md content = %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(dir, "sub", "x.txt")); err != nil {
		t.Errorf("sub/x.txt: %v", err)
	}
	if !strings.Contains(out, "linked_children") {
		t.Errorf("expected linked_children in output:\n%s", out)
	}
	if !strings.Contains(out, `"commit_hash": "h1"`) {
		t.Errorf("expected commit_hash in output:\n%s", out)
	}
}

func TestHubPull_RejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"../escape.txt": map[string]any{"type": "file", "content": "nope"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	cmd := newHubCmd()
	cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "escape") {
		t.Errorf("expected traversal error; got %v", err)
	}
	parent := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(parent, "escape.txt")); !os.IsNotExist(err) {
		t.Errorf("traversal write was NOT prevented")
	}
}

func TestHubPull_RejectsAbsolutePath(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"/etc/passwd": map[string]any{"type": "file", "content": "nope"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	cmd := newHubCmd()
	cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("expected absolute-path error; got %v", err)
	}
}

func TestHubPull_WipesDestWithMarker(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"new.txt": map[string]any{"type": "file", "content": "new"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---"), 0o644); err != nil {
		t.Fatalf("seed marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(dir, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("stale.txt should be wiped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("new.txt should be written: %v", err)
	}
}

func TestHubPull_RefusesNonHubDirWithoutYes(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files":       map[string]any{"SKILL.md": map[string]any{"type": "file", "content": "x"}},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "important.txt"), []byte("user data"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := newHubCmd()
	cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a hub directory") {
		t.Errorf("expected refusal for non-hub dir; got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "important.txt")); err != nil {
		t.Errorf("important.txt should be preserved: %v", err)
	}
}

func TestHubPull_AcceptsNonHubDirWithYes(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files":       map[string]any{"SKILL.md": map[string]any{"type": "file", "content": "x"}},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "important.txt"), []byte("user data"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	captureStdout(t, func() {
		cmd := newHubCmd()
		cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir, "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, "important.txt")); !os.IsNotExist(err) {
		t.Errorf("important.txt should be wiped with --yes, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md should be written: %v", err)
	}
}

// TestHubPull_PreservesExistingDirOnMidWriteFailure exercises the case where
// a file write fails partway through pulling a new set of files (here,
// triggered deterministically by a path collision: "bad" is requested as
// both a file and, via "bad/x", a directory). Before staging writes in a
// temporary sibling directory, this used to leave dest wiped-but-incomplete
// since files were written directly into dest after it was removed and
// recreated. The pre-existing dest contents must survive untouched, and the
// command must report an error rather than a partial success.
func TestHubPull_PreservesExistingDirOnMidWriteFailure(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"good.txt": map[string]any{"type": "file", "content": "fine"},
				"bad/x":    map[string]any{"type": "file", "content": "will fail"},
				"bad":      map[string]any{"type": "file", "content": "collides with bad/x's parent dir"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("original content"), 0o644); err != nil {
		t.Fatalf("seed SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("old file"), 0o644); err != nil {
		t.Fatalf("seed old.txt: %v", err)
	}

	cmd := newHubCmd()
	cmd.SetArgs([]string{"pull", "my-skill", "--dir", dir, "--yes"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error from the colliding paths, got nil")
	}

	if data, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("original SKILL.md should be preserved: %v", err)
	} else if string(data) != "original content" {
		t.Errorf("SKILL.md content = %q, want %q", string(data), "original content")
	}
	if _, err := os.Stat(filepath.Join(dir, "old.txt")); err != nil {
		t.Errorf("original old.txt should be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "good.txt")); !os.IsNotExist(err) {
		t.Errorf("good.txt should NOT have been partially applied to dest, got err=%v", err)
	}

	// No leftover staging directory next to dir.
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatalf("reading parent dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(dir) && strings.HasPrefix(e.Name(), ".hub-pull-") {
			t.Errorf("leftover staging directory not cleaned up: %s", e.Name())
		}
	}
}
