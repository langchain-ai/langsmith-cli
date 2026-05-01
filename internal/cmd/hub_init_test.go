package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHubInit_SkillScaffold(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-skill")

	written, err := scaffoldHubDirectory(target, "skill", "my-skill", "Does the thing", false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if len(written) != 1 || written[0] != "SKILL.md" {
		t.Errorf("expected [SKILL.md], got %v", written)
	}
	data, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "name: my-skill") {
		t.Errorf("missing name in frontmatter:\n%s", body)
	}
	if !strings.Contains(body, "description: Does the thing") {
		t.Errorf("missing description in frontmatter:\n%s", body)
	}
}

func TestHubInit_AgentScaffold(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-agent")

	written, err := scaffoldHubDirectory(target, "agent", "my-agent", "", false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	want := map[string]bool{"AGENTS.md": false, "tools.json": false, "config.json": false}
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
}

func TestHubInit_RejectsNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := scaffoldHubDirectory(dir, "skill", "my-skill", "", false)
	if err == nil {
		t.Fatal("expected error for non-empty dir without --force")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHubInit_ForceWritesOverNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := scaffoldHubDirectory(dir, "skill", "my-skill", "", true)
	if err != nil {
		t.Fatalf("scaffold with force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not written: %v", err)
	}
}

func TestHubInit_RejectsBadType(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldHubDirectory(dir, "prompt", "x", "", false); err == nil {
		t.Fatal("expected error for invalid type")
	}
}
