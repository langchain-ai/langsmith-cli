//go:build integration

package cmd

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireIntegrationEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("LANGSMITH_API_KEY") == "" {
		t.Skip("LANGSMITH_API_KEY not set; skipping integration test")
	}
}

func randomHandle(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

// runHub invokes `langsmith hub <args>` in-process and returns the parsed JSON
// output. Fails the test on a non-nil execute error or non-JSON stdout.
func runHub(t *testing.T, args ...string) map[string]any {
	t.Helper()
	full := append([]string{"hub"}, args...)
	out := captureStdout(t, func() {
		cmd := NewRootCmd("dev", "dev")
		cmd.SetArgs(full)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("langsmith %s: %v", strings.Join(full, " "), err)
		}
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("could not parse JSON output of `langsmith %s`: %v\noutput:\n%s", strings.Join(full, " "), err, out)
	}
	return result
}

// runHubExpectError invokes the command and asserts it returned a non-nil error.
func runHubExpectError(t *testing.T, args ...string) error {
	t.Helper()
	full := append([]string{"hub"}, args...)
	var execErr error
	captureStdout(t, func() {
		cmd := NewRootCmd("dev", "dev")
		cmd.SetArgs(full)
		execErr = cmd.Execute()
	})
	if execErr == nil {
		t.Fatalf("expected `langsmith %s` to fail, but it succeeded", strings.Join(full, " "))
	}
	return execErr
}

// scheduleDelete registers a t.Cleanup that deletes the hub repo, swallowing
// errors. Runs even on test failure so test repos do not pile up.
func scheduleDelete(t *testing.T, handle string) {
	t.Cleanup(func() {
		full := []string{"hub", "delete", handle, "--yes"}
		captureStdout(t, func() {
			cmd := NewRootCmd("dev", "dev")
			cmd.SetArgs(full)
			_ = cmd.Execute()
		})
	})
}

// seedSkillContent writes a realistic skill into dir and returns the
// path -> content map for later checksum comparison.
func seedSkillContent(t *testing.T, dir, name string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{
		"SKILL.md":         []byte(fmt.Sprintf("---\nname: %s\ndescription: Integration test skill (auto-deletes)\n---\n\n# %s\n\nThis skill exists only to validate the round-trip between the langsmith CLI\nand the hub backend. It is created and deleted by an automated integration\ntest run; do not depend on it.\n", name, name)),
		"examples/a.md":    []byte("# Example A\n\nFirst example for the integration test skill.\n"),
		"examples/b.md":    []byte("# Example B\n\nSecond example for the integration test skill.\n"),
		"references/r1.md": []byte("# Reference 1\n\nReferenced from SKILL.md for the round-trip check.\n"),
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return files
}

func hashContents(files map[string][]byte) map[string]string {
	out := make(map[string]string)
	for path, content := range files {
		sum := sha256.Sum256(content)
		out[path] = hex.EncodeToString(sum[:])
	}
	return out
}

func walkAndHash(t *testing.T, dir string) map[string]string {
	t.Helper()
	hashes := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return hashes
}

func assertSameTree(t *testing.T, want, got map[string]string) {
	t.Helper()
	for path, wantHash := range want {
		gotHash, ok := got[path]
		if !ok {
			t.Errorf("missing file in pulled tree: %s", path)
			continue
		}
		if gotHash != wantHash {
			t.Errorf("hash mismatch for %s: want %s..., got %s...", path, wantHash[:8], gotHash[:8])
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("unexpected extra file in pulled tree: %s", path)
		}
	}
}

func stringSliceField(m map[string]any, key string) []string {
	raw, _ := m[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func bytesSum(m map[string][]byte) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}

// TestHubIntegration_UserJourney walks through the full sequence a real user
// would run: init, author, push, get, list, pull, pin-by-hash, secret-exclusion,
// delete, verify-deleted. Each step asserts on JSON output and filesystem
// state to prove the wire format and behavior match against the live API.
func TestHubIntegration_UserJourney(t *testing.T) {
	requireIntegrationEnv(t)

	handle := randomHandle("cli-int")
	scheduleDelete(t, handle)
	localDir := filepath.Join(t.TempDir(), handle)

	// Step 1: init scaffolds the dir and writes SKILL.md with frontmatter.
	initOut := runHub(t, "init", "--type", "skill", "--dir", localDir, "--name", handle, "--description", "Integration test skill")
	if initOut["status"] != "scaffolded" {
		t.Errorf("init status = %v, want scaffolded", initOut["status"])
	}
	skillData, err := os.ReadFile(filepath.Join(localDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if !strings.Contains(string(skillData), "name: "+handle) {
		t.Errorf("SKILL.md missing name in frontmatter:\n%s", string(skillData))
	}
	t.Logf("step 1: scaffolded %s", localDir)

	// Step 2: author realistic content (overwrites the init stub).
	seeded := seedSkillContent(t, localDir, handle)
	seededHashes := hashContents(seeded)
	t.Logf("step 2: authored %d files (total %d bytes)", len(seeded), bytesSum(seeded))

	// Step 3: push creates the repo and posts a directory commit.
	pushOut := runHub(t, "push", handle, "--type", "skill", "--dir", localDir, "--description", "Integration test skill")
	if pushOut["status"] != "pushed" {
		t.Errorf("push status = %v, want pushed", pushOut["status"])
	}
	commitHash, _ := pushOut["commit_hash"].(string)
	if commitHash == "" {
		t.Fatalf("push did not return commit_hash; output=%v", pushOut)
	}
	pushedFiles := stringSliceField(pushOut, "files")
	for path := range seeded {
		found := false
		for _, p := range pushedFiles {
			if p == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("push did not include %s; got %v", path, pushedFiles)
		}
	}
	t.Logf("step 3: pushed commit %s with %d files", short(commitHash), len(pushedFiles))

	// Step 4: get returns the repo metadata we just pushed.
	getOut := runHub(t, "get", handle)
	if getOut["repo_handle"] != handle {
		t.Errorf("get repo_handle = %v, want %v", getOut["repo_handle"], handle)
	}
	if getOut["repo_type"] != "skill" {
		t.Errorf("get repo_type = %v, want skill", getOut["repo_type"])
	}
	numCommits, _ := getOut["num_commits"].(float64)
	if numCommits < 1 {
		t.Errorf("get num_commits = %v, want >= 1", getOut["num_commits"])
	}
	t.Logf("step 4: get returned repo with %v commit(s)", getOut["num_commits"])

	// Step 5: list with a query should surface our repo.
	listOut := runHub(t, "list", "--query", handle, "--type", "skill")
	total, _ := listOut["total"].(float64)
	if total < 1 {
		t.Errorf("list total = %v, want >= 1", listOut["total"])
	}
	repos, _ := listOut["repos"].([]any)
	foundInList := false
	for _, repo := range repos {
		r, _ := repo.(map[string]any)
		if rh, _ := r["repo_handle"].(string); rh == handle {
			foundInList = true
			break
		}
	}
	if !foundInList {
		t.Errorf("list did not include our repo (handle=%s)", handle)
	}
	t.Logf("step 5: list returned %v matching repo(s)", listOut["total"])

	// Step 6: pull (latest) should round-trip every file we authored.
	pulledDir := filepath.Join(t.TempDir(), handle+"-pulled")
	runHub(t, "pull", handle, "--dir", pulledDir)
	assertSameTree(t, seededHashes, walkAndHash(t, pulledDir))
	t.Logf("step 6: pulled %d files (matched authored content)", len(seededHashes))

	// Step 7: pull pinned to the specific commit hash.
	pinnedDir := filepath.Join(t.TempDir(), handle+"-pinned")
	runHub(t, "pull", handle+":"+commitHash, "--dir", pinnedDir)
	assertSameTree(t, seededHashes, walkAndHash(t, pinnedDir))
	t.Logf("step 7: pulled commit %s by hash (matched)", short(commitHash))

	// Step 8: seed a .env, re-push, verify .env never reaches the server.
	if err := os.WriteFile(filepath.Join(localDir, ".env"), []byte("API_KEY=fake_secret\nDB=postgres://x\n"), 0o644); err != nil {
		t.Fatalf("seed .env: %v", err)
	}
	runHub(t, "push", handle, "--type", "skill", "--dir", localDir)
	leakDir := filepath.Join(t.TempDir(), handle+"-leak-check")
	runHub(t, "pull", handle, "--dir", leakDir)
	if _, err := os.Stat(filepath.Join(leakDir, ".env")); !os.IsNotExist(err) {
		t.Errorf(".env was uploaded; secret-exclusion failed against the live API")
	}
	t.Logf("step 8: .env correctly excluded from push")

	// Step 9: delete removes the repo.
	deleteOut := runHub(t, "delete", handle, "--yes")
	if deleteOut["status"] != "deleted" {
		t.Errorf("delete status = %v, want deleted", deleteOut["status"])
	}
	t.Logf("step 9: deleted %s", handle)

	// Step 10: get after delete should 404.
	err = runHubExpectError(t, "get", handle)
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 after delete; got %v", err)
	}
	t.Logf("step 10: get after delete returned 404 as expected")
}

func TestHubIntegration_GetMissingRepo(t *testing.T) {
	requireIntegrationEnv(t)
	handle := randomHandle("cli-int-missing")
	err := runHubExpectError(t, "get", handle)
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 for missing repo; got %v", err)
	}
}

func TestHubIntegration_PullWithCommitRef(t *testing.T) {
	requireIntegrationEnv(t)
	handle := randomHandle("cli-int")
	scheduleDelete(t, handle)
	localDir := filepath.Join(t.TempDir(), handle)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	v1 := fmt.Sprintf("---\nname: %s\ndescription: v1\n---\n\n# v1\n", handle)
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte(v1), 0o644); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	push1 := runHub(t, "push", handle, "--type", "skill", "--dir", localDir)
	v1Hash, _ := push1["commit_hash"].(string)

	v2 := fmt.Sprintf("---\nname: %s\ndescription: v2\n---\n\n# v2\n", handle)
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte(v2), 0o644); err != nil {
		t.Fatalf("write v2: %v", err)
	}
	runHub(t, "push", handle, "--type", "skill", "--dir", localDir)

	pullDir := filepath.Join(t.TempDir(), "v1")
	runHub(t, "pull", handle+":"+v1Hash, "--dir", pullDir)
	got, err := os.ReadFile(filepath.Join(pullDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read pulled v1: %v", err)
	}
	if !strings.Contains(string(got), "description: v1") {
		t.Errorf("pulled v1 by hash but content does not match v1:\n%s", string(got))
	}
}

func TestHubIntegration_NonHubDirGuard(t *testing.T) {
	requireIntegrationEnv(t)
	handle := randomHandle("cli-int")
	scheduleDelete(t, handle)

	localDir := filepath.Join(t.TempDir(), handle)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte(fmt.Sprintf("---\nname: %s\ndescription: x\n---\n\n# x\n", handle)), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	runHub(t, "push", handle, "--type", "skill", "--dir", localDir)

	userDir := t.TempDir()
	userFile := filepath.Join(userDir, "important.txt")
	if err := os.WriteFile(userFile, []byte("user data"), 0o644); err != nil {
		t.Fatalf("seed user file: %v", err)
	}
	err := runHubExpectError(t, "pull", handle, "--dir", userDir)
	if !strings.Contains(err.Error(), "not a hub directory") {
		t.Errorf("expected 'not a hub directory' error; got %v", err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Errorf("user file was deleted; wipe guard failed: %v", err)
	}
}

func short(hash string) string {
	if len(hash) < 8 {
		return hash
	}
	return hash[:8]
}
