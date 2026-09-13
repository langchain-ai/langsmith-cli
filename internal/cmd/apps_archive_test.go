package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// untarBase64 decodes a source_archive value into path → content.
func untarBase64(t *testing.T, encoded string) map[string]string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("source archive is not valid base64: %v", err)
	}
	if len(raw) < 2 || raw[0] != 0x1f || raw[1] != 0x8b {
		t.Fatalf("source archive is missing gzip magic bytes, got % x", raw[:min(2, len(raw))])
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gz.Close()

	out := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("reading tar entry %s: %v", hdr.Name, err)
		}
		out[hdr.Name] = string(data)
	}
	return out
}

func TestBuildSourceArchive_ExcludesDenylistedPathsEvenWhenGitignoreUnignoresThem(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "src/App.tsx", "export default function App() {}")
	seedFile(t, dir, "package.json", `{"name":"x"}`)
	seedFile(t, dir, "node_modules/react/index.js", "module.exports = {}")
	seedFile(t, dir, ".git/config", "[core]")
	seedFile(t, dir, "dist/bundle.js", "built")
	seedFile(t, dir, "build/out.js", "built")
	seedFile(t, dir, ".env", "LANGSMITH_API_KEY=lsv2_secret")
	seedFile(t, dir, ".env.production", "KEY=2")
	seedFile(t, dir, "settings.local", "local")
	seedFile(t, dir, "id_rsa", "PRIVATE KEY")
	seedFile(t, dir, "certs/server.pem", "PRIVATE KEY")
	seedFile(t, dir, "certs/server.key", "PRIVATE KEY")
	seedFile(t, dir, ".npmrc", "//registry:_authToken=secret")
	// Even an explicit un-ignore must not resurrect these.
	seedFile(t, dir, ".gitignore", "!node_modules\n!.env\n!id_rsa\n")

	encoded, err := buildSourceArchive(dir)
	if err != nil {
		t.Fatalf("buildSourceArchive: %v", err)
	}
	entries := untarBase64(t, encoded)

	for _, want := range []string{"src/App.tsx", "package.json", ".gitignore"} {
		if _, ok := entries[want]; !ok {
			t.Errorf("expected %q in the archive, got %v", want, keysOf(entries))
		}
	}
	for _, unwanted := range []string{
		"node_modules/react/index.js",
		".git/config",
		"dist/bundle.js",
		"build/out.js",
		".env",
		".env.production",
		"settings.local",
		"id_rsa",
		"certs/server.pem",
		"certs/server.key",
		".npmrc",
	} {
		if _, ok := entries[unwanted]; ok {
			t.Errorf("%q must never be archived", unwanted)
		}
	}
	if got := entries["src/App.tsx"]; got != "export default function App() {}" {
		t.Errorf("archived content mismatch: %q", got)
	}
}

func TestBuildSourceArchive_HonorsGitignoreAsAdditionalExcludes(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "src/App.tsx", "app")
	seedFile(t, dir, "src/debug.log", "noise")
	seedFile(t, dir, "keep.log", "kept")
	seedFile(t, dir, "tmp/scratch.txt", "scratch")
	seedFile(t, dir, "rootonly.txt", "root")
	seedFile(t, dir, "nested/rootonly.txt", "nested")
	seedFile(t, dir, "fixtures/big.bin", "data")
	seedFile(t, dir, ".gitignore", "# comment\n\n*.log\n!keep.log\ntmp/\n/rootonly.txt\nfixtures\n")

	encoded, err := buildSourceArchive(dir)
	if err != nil {
		t.Fatalf("buildSourceArchive: %v", err)
	}
	entries := untarBase64(t, encoded)

	for _, want := range []string{"src/App.tsx", "keep.log", "nested/rootonly.txt"} {
		if _, ok := entries[want]; !ok {
			t.Errorf("expected %q in the archive, got %v", want, keysOf(entries))
		}
	}
	for _, unwanted := range []string{"src/debug.log", "tmp/scratch.txt", "rootonly.txt", "fixtures/big.bin"} {
		if _, ok := entries[unwanted]; ok {
			t.Errorf("expected .gitignore to exclude %q, got %v", unwanted, keysOf(entries))
		}
	}
}

func TestBuildSourceArchive_NoGitignoreStillExcludesDenylist(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "src/App.tsx", "app")
	seedFile(t, dir, "node_modules/react/index.js", "dep")
	seedFile(t, dir, ".env.local", "SECRET=1")

	encoded, err := buildSourceArchive(dir)
	if err != nil {
		t.Fatalf("buildSourceArchive: %v", err)
	}
	entries := untarBase64(t, encoded)
	if _, ok := entries["src/App.tsx"]; !ok {
		t.Errorf("expected src/App.tsx, got %v", keysOf(entries))
	}
	if len(entries) != 1 {
		t.Errorf("expected only source files without a .gitignore, got %v", keysOf(entries))
	}
}

func TestBuildSourceArchive_RejectsOversizeBeforeUploading(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "src/App.tsx", "app")

	// Incompressible bytes so the gzip output really exceeds the cap.
	blob := make([]byte, 256*1024)
	rng := rand.New(rand.NewSource(1))
	if _, err := rng.Read(blob); err != nil {
		t.Fatalf("rand: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "blob.bin"), blob, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}

	prev := appsMaxSourceArchiveBytes
	appsMaxSourceArchiveBytes = 64 * 1024
	t.Cleanup(func() { appsMaxSourceArchiveBytes = prev })

	_, err := buildSourceArchive(dir)
	if err == nil {
		t.Fatal("expected an oversize archive to be rejected locally")
	}
	if !strings.Contains(err.Error(), "limit") || !strings.Contains(err.Error(), "assets/") {
		t.Errorf("expected an actionable oversize error naming the bloat, got: %v", err)
	}
}

func TestBuildSourceArchive_DefaultCapMatchesBackend(t *testing.T) {
	if appsMaxSourceArchiveBytes != 50<<20 {
		t.Errorf("expected the 50 MB backend cap, got %d", appsMaxSourceArchiveBytes)
	}
}

func TestBuildSourceArchive_EmptyDirProducesNoArchive(t *testing.T) {
	dir := t.TempDir()
	seedFile(t, dir, "node_modules/react/index.js", "dep")

	encoded, err := buildSourceArchive(dir)
	if err != nil {
		t.Fatalf("buildSourceArchive: %v", err)
	}
	if encoded != "" {
		t.Errorf("expected no archive when nothing is archivable, got %d bytes", len(encoded))
	}
}

func TestGitignoreRules_Matching(t *testing.T) {
	rules := loadGitignoreFromString("*.log\n!keep.log\ntmp/\n/root.txt\nsrc/generated\n**/cache\n")
	tests := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{"a.log", false, true},
		{"deep/a.log", false, true},
		{"keep.log", false, false},
		{"tmp", true, true},
		{"tmp.txt", false, false},
		{"root.txt", false, true},
		{"nested/root.txt", false, false},
		{"src/generated", true, true},
		{"src/generated/x.ts", false, true},
		{"any/where/cache", true, true},
		{"src/App.tsx", false, false},
	}
	for _, tt := range tests {
		if got := rules.ignored(tt.rel, tt.isDir); got != tt.want {
			t.Errorf("ignored(%q, dir=%v) = %v, want %v", tt.rel, tt.isDir, got, tt.want)
		}
	}
}

func loadGitignoreFromString(contents string) gitignoreRules {
	dir, err := os.MkdirTemp("", "gitignore")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(contents), 0o644); err != nil {
		panic(err)
	}
	return loadGitignore(dir)
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
