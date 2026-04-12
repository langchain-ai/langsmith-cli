package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Fatal("expected non-empty default cache dir")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
}

func TestPathForKey(t *testing.T) {
	dir := filepath.Join("tmp", "cache")
	p1 := PathForKey(dir, "openapi", "https://api.smith.langchain.com")
	p2 := PathForKey(dir, "openapi", "https://myhost.com")
	if p1 == p2 {
		t.Error("expected different paths for different keys")
	}
	if filepath.Dir(p1) != dir {
		t.Errorf("expected dir %s, got %s", dir, filepath.Dir(p1))
	}
}

func TestReadIfFresh_Missing(t *testing.T) {
	_, err := ReadIfFresh("/nonexistent/path", time.Hour)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadIfFresh_Expired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	_, err := ReadIfFresh(path, time.Hour)
	if err == nil {
		t.Fatal("expected error for expired cache")
	}
}

func TestReadIfFresh_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadIfFresh(path, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("unexpected data: %s", data)
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.json")
	if err := Write(path, []byte(`{"written":true}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != `{"written":true}` {
		t.Errorf("unexpected content: %s", data)
	}
}
