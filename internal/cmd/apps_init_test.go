package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppsInit_ScaffoldsExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "Does the thing", false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	want := map[string]bool{
		"package.json": false,
		"src/index.js": false,
		"README.md":    false,
		".gitignore":   false,
	}
	for _, w := range written {
		if _, ok := want[w]; !ok {
			t.Errorf("unexpected file %q", w)
		}
		want[w] = true
	}
	for f, seen := range want {
		if !seen {
			t.Errorf("expected %q to be written", f)
		}
	}

	pkg, err := os.ReadFile(filepath.Join(target, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if !strings.Contains(string(pkg), `"name": "my-app"`) {
		t.Errorf("package.json missing templated name:\n%s", pkg)
	}

	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "Does the thing") {
		t.Errorf("README.md missing templated description:\n%s", readme)
	}
}

func TestAppsInit_DefaultsDescription(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readme), "TODO: one-sentence description") {
		t.Errorf("expected default description placeholder:\n%s", readme)
	}
}

func TestAppsInit_RejectsNonEmptyDirWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := scaffoldCustomAppStarter(dir, "my-app", "", false)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected not-empty error, got %v", err)
	}
}

func TestAppsInit_ForceWritesOverNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := scaffoldCustomAppStarter(dir, "my-app", "", true); err != nil {
		t.Fatalf("scaffold with force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Errorf("package.json not written: %v", err)
	}
}

func TestAppsInit_RequiresName(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldCustomAppStarter(filepath.Join(dir, "app"), "", "", false); err == nil {
		t.Fatal("expected error when --name is empty")
	}
}
