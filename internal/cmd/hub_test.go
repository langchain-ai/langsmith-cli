package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReadDirectoryAsFiles_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "hi\n")
	writeFile(t, dir, "src/main.py", "print(1)\n")
	writeFile(t, dir, "src/util/helpers.py", "def f(): pass\n")

	got, err := readDirectoryAsFiles(dir)
	if err != nil {
		t.Fatalf("readDirectoryAsFiles: %v", err)
	}
	want := map[string]string{
		"README.md":           "hi\n",
		"src/main.py":         "print(1)\n",
		"src/util/helpers.py": "def f(): pass\n",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(got), len(want), keysOf(got))
	}
	for path, content := range want {
		entry, ok := got[path]
		if !ok {
			t.Errorf("missing entry for %q", path)
			continue
		}
		if entry.Type != "file" {
			t.Errorf("%s: type = %q, want %q", path, entry.Type, "file")
		}
		if entry.Content != content {
			t.Errorf("%s: content = %q, want %q", path, entry.Content, content)
		}
	}
}

func TestReadDirectoryAsFiles_ExclusionsAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.txt", "ok\n")
	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, dir, "node_modules/foo/bar.js", "x\n")
	writeFile(t, dir, "__pycache__/foo.pyc", "cached\n")
	writeFile(t, dir, ".DS_Store", "crap\n")
	writeFile(t, dir, "scripts/thing.pyc", "compiled\n")
	writeFile(t, dir, "target.txt", "symlink target\n")

	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := readDirectoryAsFiles(dir)
	if err != nil {
		t.Fatalf("readDirectoryAsFiles: %v", err)
	}

	want := map[string]bool{
		"keep.txt":   true,
		"target.txt": true,
	}
	if len(got) != len(want) {
		t.Errorf("got %d files, want %d: %v", len(got), len(want), keysOf(got))
	}
	for path := range want {
		if _, ok := got[path]; !ok {
			t.Errorf("missing expected path %q", path)
		}
	}
	for bad := range got {
		if !want[bad] {
			t.Errorf("unexpected path in result: %q", bad)
		}
	}
}

func TestReadDirectoryAsFiles_BinaryFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bin/blob", "abc\x00def")

	_, err := readDirectoryAsFiles(dir)
	if err == nil {
		t.Fatal("expected error for binary file")
	}
	if !strings.Contains(err.Error(), "bin/blob") {
		t.Errorf("error should name the offending path; got: %v", err)
	}
	if !strings.Contains(err.Error(), "binary") {
		t.Errorf("error should mention 'binary'; got: %v", err)
	}
}

func TestReadDirectoryAsFiles_EntryLimit(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i <= hubMaxFileEntries; i++ {
		writeFile(t, dir, "files/f"+itoa(i)+".txt", "x")
	}
	_, err := readDirectoryAsFiles(dir)
	if err == nil {
		t.Fatal("expected error for too many entries")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention the 500 limit; got: %v", err)
	}
}

func TestReadDirectoryAsFiles_PerFileSizeCap(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, hubMaxFileSizeBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(dir, "huge.txt"), big, 0o644); err != nil {
		t.Fatalf("writing huge.txt: %v", err)
	}

	_, err := readDirectoryAsFiles(dir)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	if !strings.Contains(err.Error(), "huge.txt") {
		t.Errorf("error should name the path; got: %v", err)
	}
}

func TestWriteFilesToDirectory_WipesBeforeWriting(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, dest, "stale/leftover.txt", "old\n")

	files := map[string]hubFileEntry{
		"README.md":    {Type: "file", Content: "new\n"},
		"sub/a.py":     {Type: "file", Content: "print(1)\n"},
		"linked-child": {Type: "agent", RepoHandle: "child-agent", Owner: "myorg"},
	}

	written, linked, err := writeFilesToDirectory(dest, files)
	if err != nil {
		t.Fatalf("writeFilesToDirectory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale/leftover.txt")); !os.IsNotExist(err) {
		t.Errorf("stale file should have been removed (err=%v)", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "README.md")); string(got) != "new\n" {
		t.Errorf("README.md = %q, want %q", string(got), "new\n")
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "sub/a.py")); string(got) != "print(1)\n" {
		t.Errorf("sub/a.py = %q, want %q", string(got), "print(1)\n")
	}
	if len(written) != 2 {
		t.Errorf("written = %v (len %d), want 2 entries", written, len(written))
	}
	if len(linked) != 1 || !strings.Contains(linked[0], "linked-child") {
		t.Errorf("linked = %v, want one entry naming linked-child", linked)
	}
}

func TestScaffoldHubDirectory_Skill(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-skill")
	written, err := scaffoldHubDirectory(dir, "skill", "my-skill", "Do a thing", false)
	if err != nil {
		t.Fatalf("scaffoldHubDirectory: %v", err)
	}
	if len(written) != 1 || written[0] != "SKILL.md" {
		t.Errorf("written = %v, want [SKILL.md]", written)
	}

	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading SKILL.md: %v", err)
	}
	re := regexp.MustCompile(`(?s)^---\s*\nname: my-skill\s*\ndescription: .+\n---\s*\n`)
	if !re.Match(content) {
		t.Errorf("SKILL.md does not have the expected frontmatter shape:\n%s", content)
	}
	if !strings.Contains(string(content), "Do a thing") {
		t.Errorf("SKILL.md should contain the description; got:\n%s", content)
	}
}

func TestScaffoldHubDirectory_Agent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-agent")
	written, err := scaffoldHubDirectory(dir, "agent", "my-agent", "", false)
	if err != nil {
		t.Fatalf("scaffoldHubDirectory: %v", err)
	}
	expect := map[string]bool{"AGENTS.md": true, "tools.json": true, "config.json": true}
	if len(written) != len(expect) {
		t.Errorf("written = %v, want exactly %d files", written, len(expect))
	}
	for _, w := range written {
		if !expect[w] {
			t.Errorf("unexpected scaffolded file: %q", w)
		}
	}

	toolsRaw, err := os.ReadFile(filepath.Join(dir, "tools.json"))
	if err != nil {
		t.Fatalf("reading tools.json: %v", err)
	}
	var tools map[string]any
	if err := json.Unmarshal(toolsRaw, &tools); err != nil {
		t.Fatalf("tools.json is not valid JSON: %v\n%s", err, toolsRaw)
	}
	arr, ok := tools["tools"].([]any)
	if !ok || len(arr) != 0 {
		t.Errorf("tools.json: tools field = %v, want empty array", tools["tools"])
	}
	m, ok := tools["interrupt_config"].(map[string]any)
	if !ok || len(m) != 0 {
		t.Errorf("tools.json: interrupt_config = %v, want empty object", tools["interrupt_config"])
	}
}

func TestScaffoldHubDirectory_RefusesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "existing.txt", "hi\n")

	_, err := scaffoldHubDirectory(dir, "skill", "my-skill", "", false)
	if err == nil {
		t.Fatal("expected error for non-empty directory without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force; got: %v", err)
	}

	written, err := scaffoldHubDirectory(dir, "skill", "my-skill", "", true)
	if err != nil {
		t.Fatalf("scaffoldHubDirectory with force=true: %v", err)
	}
	if len(written) == 0 {
		t.Error("expected files written with force=true")
	}
}

func TestScaffoldHubDirectory_InvalidName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "target")
	_, err := scaffoldHubDirectory(dir, "skill", "Bad-Name", "", false)
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
	if !strings.Contains(err.Error(), "[a-z]") {
		t.Errorf("error should mention the name pattern; got: %v", err)
	}
}

func TestParseOwnerRepo(t *testing.T) {
	cases := []struct {
		in                  string
		wantOwner, wantRepo string
		wantCommit          string
		wantErr             bool
	}{
		{"my-skill", "-", "my-skill", "", false},
		{"my-skill:production", "-", "my-skill", "production", false},
		{"myorg/my-skill", "myorg", "my-skill", "", false},
		{"myorg/my-skill:abc123", "myorg", "my-skill", "abc123", false},
		{"-/my-skill", "-", "my-skill", "", false},
		{"", "", "", "", true},
		{"/", "", "", "", true},
		{"/foo", "", "", "", true},
		{"foo/", "", "", "", true},
		{":tag", "-", "", "tag", true},
	}
	for _, tc := range cases {
		owner, repo, commit, err := parseOwnerRepo(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got owner=%q repo=%q commit=%q", tc.in, owner, repo, commit)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.in, err)
			continue
		}
		if owner != tc.wantOwner || repo != tc.wantRepo || commit != tc.wantCommit {
			t.Errorf("%q: got (%q, %q, %q), want (%q, %q, %q)",
				tc.in, owner, repo, commit, tc.wantOwner, tc.wantRepo, tc.wantCommit)
		}
	}
}

func TestHubCommitPayloadShape(t *testing.T) {
	files := map[string]hubFileEntry{
		"SKILL.md":  {Type: "file", Content: "---\nname: x\ndescription: y\n---\n"},
		"sub/a.txt": {Type: "file", Content: "hello\n"},
	}
	parent := "abc12345"
	body := map[string]any{"files": files, "parent_commit": parent}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Shape must match the backend's CreateDirectoryCommitRequest: `files` is
	// a map of path to entry discriminated by `type`, `parent_commit` optional.
	var parsed struct {
		Files        map[string]map[string]any `json:"files"`
		ParentCommit *string                   `json:"parent_commit"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.ParentCommit == nil || *parsed.ParentCommit != parent {
		t.Errorf("parent_commit = %v, want %q", parsed.ParentCommit, parent)
	}
	for path, entry := range parsed.Files {
		if entry["type"] != "file" {
			t.Errorf("%s: type = %v, want \"file\"", path, entry["type"])
		}
		if _, ok := entry["content"].(string); !ok {
			t.Errorf("%s: content missing or non-string: %v", path, entry)
		}
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func keysOf(m map[string]hubFileEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return sign + string(buf[i:])
}
