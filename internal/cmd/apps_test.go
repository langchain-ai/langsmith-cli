package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppLink_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	if link, err := readAppLink(dir); err != nil || link != nil {
		t.Fatalf("expected (nil, nil) for missing link file, got (%v, %v)", link, err)
	}

	want := appLink{AppID: "app_1", Name: "my-app"}
	if err := writeAppLink(dir, want); err != nil {
		t.Fatalf("writeAppLink: %v", err)
	}

	got, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if got == nil || *got != want {
		t.Errorf("readAppLink = %+v, want %+v", got, want)
	}

	// Written under .langsmith/app.json specifically, not loose in dir.
	if _, err := os.Stat(filepath.Join(dir, appsLinkDir, appsLinkFile)); err != nil {
		t.Errorf(".langsmith/app.json not found: %v", err)
	}
}

func TestAppLink_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := writeAppLink(dir, appLink{AppID: "app_1"}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writeAppLink(dir, appLink{AppID: "app_2"}); err != nil {
		t.Fatalf("second write: %v", err)
	}
	got, err := readAppLink(dir)
	if err != nil || got == nil || got.AppID != "app_2" {
		t.Errorf("expected app_2 after overwrite, got %+v (err %v)", got, err)
	}
}

func TestReadDirectoryAsAppFiles_ExcludesConfiguredDirsAndFiles(t *testing.T) {
	dir := t.TempDir()
	seed := map[string]string{
		"index.js":                  "module.exports = {}",
		"node_modules/pkg/index.js": "should be excluded",
		".git/HEAD":                 "should be excluded",
		".langsmith/app.json":       "should be excluded",
		".DS_Store":                 "should be excluded",
		".env":                      "should be excluded",
		".env.production":           "should be excluded",
	}
	for rel, content := range seed {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	files, err := readDirectoryAsAppFiles(dir)
	if err != nil {
		t.Fatalf("readDirectoryAsAppFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected exactly 1 file, got %v", files)
	}
	if files["index.js"] != "module.exports = {}" {
		t.Errorf("unexpected content for index.js: %q", files["index.js"])
	}
}

func TestReadDirectoryAsAppFiles_RejectsBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := readDirectoryAsAppFiles(dir)
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Errorf("expected binary-rejection error, got %v", err)
	}
}

func TestReadDirectoryAsAppFiles_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, appsMaxFileSizeBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), big, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := readDirectoryAsAppFiles(dir)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size-limit error, got %v", err)
	}
}

func TestReadDirectoryAsAppFiles_ErrorsOnMissingDir(t *testing.T) {
	_, err := readDirectoryAsAppFiles(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}
