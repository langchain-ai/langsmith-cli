package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPull_WritesFiles(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/platform/hub/repos/-/my-skill/directories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md":  map[string]any{"type": "file", "content": "# hi"},
				"sub/x.txt": map[string]any{"type": "file", "content": "abc"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newFleetCmd()
		cmd.SetArgs([]string{"skills", "pull", "my-skill", "--global=false", "--agent", "claude"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	target := filepath.Join(dir, ".agents", "skills", "my-skill")
	if data, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md: %v", err)
	} else if string(data) != "# hi" {
		t.Errorf("SKILL.md content = %q", string(data))
	}
	if data, err := os.ReadFile(filepath.Join(target, "sub", "x.txt")); err != nil {
		t.Fatalf("sub/x.txt: %v", err)
	} else if string(data) != "abc" {
		t.Errorf("sub/x.txt content = %q", string(data))
	}
}

func TestSkillPull_RejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md":      map[string]any{"type": "file", "content": "# hi"},
				"../escape.txt": map[string]any{"type": "file", "content": "nope"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newFleetCmd()
	cmd.SetArgs([]string{"skills", "pull", "my-skill", "--global=false"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "escape") {
		t.Errorf("expected traversal error; got %v", err)
	}

	// The traversal write must never have happened, and nothing should
	// have been installed at all (validation runs before any write).
	if _, statErr := os.Stat(filepath.Join(dir, ".agents", "escape.txt")); !os.IsNotExist(statErr) {
		t.Errorf("traversal write was NOT prevented")
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".agents", "skills", "my-skill")); !os.IsNotExist(statErr) {
		t.Errorf("skill directory should not have been created when validation fails")
	}
}

func TestSkillPull_RejectsAbsolutePath(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md":    map[string]any{"type": "file", "content": "# hi"},
				"/etc/passwd": map[string]any{"type": "file", "content": "nope"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newFleetCmd()
	cmd.SetArgs([]string{"skills", "pull", "my-skill", "--global=false"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("expected absolute-path error; got %v", err)
	}
}

func TestSkillPull_WipesRemovedFilesOnReinstall(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md": map[string]any{"type": "file", "content": "# v2"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	target := filepath.Join(dir, ".agents", "skills", "my-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "stale.txt"), []byte("v1 leftover"), 0o644); err != nil {
		t.Fatalf("seed stale.txt: %v", err)
	}

	captureStdout(t, func() {
		cmd := newFleetCmd()
		cmd.SetArgs([]string{"skills", "pull", "my-skill", "--global=false"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(target, "stale.txt")); !os.IsNotExist(err) {
		t.Errorf("stale.txt from the previous install should be wiped, got err=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil || string(data) != "# v2" {
		t.Errorf("SKILL.md should be the new content, got data=%q err=%v", data, err)
	}
}

// TestSkillPull_PreservesExistingSkillOnMidWriteFailure exercises a
// deterministic mid-loop write failure (the server returns both "bad" and
// "bad/x" as file entries, so whichever one lands second in map iteration
// order fails). Before staging writes in a temporary sibling directory,
// this used to leave the previously installed skill wiped-but-incomplete,
// since files were written directly into the target dir after it was
// removed. The pre-existing installation must survive untouched, and the
// command must report an error rather than a partial success.
func TestSkillPull_PreservesExistingSkillOnMidWriteFailure(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"commit_hash": "h1",
			"files": map[string]any{
				"SKILL.md": map[string]any{"type": "file", "content": "# hi"},
				"good.txt": map[string]any{"type": "file", "content": "fine"},
				"bad/x":    map[string]any{"type": "file", "content": "will fail"},
				"bad":      map[string]any{"type": "file", "content": "collides with bad/x's parent dir"},
			},
		})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	target := filepath.Join(dir, ".agents", "skills", "my-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("original content"), 0o644); err != nil {
		t.Fatalf("seed SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old file"), 0o644); err != nil {
		t.Fatalf("seed old.txt: %v", err)
	}

	cmd := newFleetCmd()
	cmd.SetArgs([]string{"skills", "pull", "my-skill", "--global=false"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error from the colliding paths, got nil")
	}

	if data, err := os.ReadFile(filepath.Join(target, "SKILL.md")); err != nil {
		t.Errorf("original SKILL.md should be preserved: %v", err)
	} else if string(data) != "original content" {
		t.Errorf("SKILL.md content = %q, want %q", string(data), "original content")
	}
	if _, err := os.Stat(filepath.Join(target, "old.txt")); err != nil {
		t.Errorf("original old.txt should be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "good.txt")); !os.IsNotExist(err) {
		t.Errorf("good.txt should NOT have been partially applied to the skill dir, got err=%v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatalf("reading parent dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(target) && strings.HasPrefix(e.Name(), ".skill-pull-") {
			t.Errorf("leftover staging directory not cleaned up: %s", e.Name())
		}
	}
}
