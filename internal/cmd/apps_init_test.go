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

	written, err := scaffoldCustomAppStarter(target, "my-app", "Does the thing", "annotation_queue", false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	writtenSet := map[string]bool{}
	for _, w := range written {
		writtenSet[w] = true
	}
	for _, want := range []string{
		"package.json",
		"README.md",
		"AGENTS.md",
		".gitignore",
		"vite.config.ts",
		"tailwind.config.js",
		"tsconfig.json",
		"src/entry.tsx",
		"src/App.tsx",
		"src/api.ts",
		"src/global.d.ts",
	} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
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

// Non-templated files (anything but README.md/package.json) must be copied
// byte-for-byte — running them through text/template would choke on, or
// silently mangle, any literal "{{"/"}}" they contain, e.g. React's
// style={{...}} inline-style syntax.
func TestAppsInit_CopiesNonTemplatedFilesVerbatim(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", "annotation_queue", false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "src", "components", "FeedbackChip.tsx"))
	if err != nil {
		t.Fatalf("read FeedbackChip.tsx: %v", err)
	}
	if !strings.Contains(string(got), "style={{ backgroundColor: color") {
		t.Errorf("expected literal style={{...}} to survive scaffolding unmodified, got:\n%s", got)
	}
}

func TestAppsInit_DefaultsDescription(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", "annotation_queue", false); err != nil {
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
	_, err := scaffoldCustomAppStarter(dir, "my-app", "", "annotation_queue", false)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected not-empty error, got %v", err)
	}
}

func TestAppsInit_ForceWritesOverNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := scaffoldCustomAppStarter(dir, "my-app", "", "annotation_queue", true); err != nil {
		t.Fatalf("scaffold with force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Errorf("package.json not written: %v", err)
	}
}

func TestAppsInit_RequiresName(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldCustomAppStarter(filepath.Join(dir, "app"), "", "", "annotation_queue", false); err == nil {
		t.Fatal("expected error when --name is empty")
	}
}

func TestAppsInit_WritesContextSpecificAgentsMD(t *testing.T) {
	dir := t.TempDir()

	noneTarget := filepath.Join(dir, "none-app")
	if _, err := scaffoldCustomAppStarter(noneTarget, "none-app", "", "none", false); err != nil {
		t.Fatalf("scaffold none: %v", err)
	}
	noneAgents, err := os.ReadFile(filepath.Join(noneTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(noneAgents), "standalone") {
		t.Errorf("expected the none-context AGENTS.md, got:\n%s", noneAgents)
	}

	aqTarget := filepath.Join(dir, "aq-app")
	if _, err := scaffoldCustomAppStarter(aqTarget, "aq-app", "", "annotation_queue", false); err != nil {
		t.Fatalf("scaffold annotation_queue: %v", err)
	}
	aqAgents, err := os.ReadFile(filepath.Join(aqTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(aqAgents), "queueId") {
		t.Errorf("expected the annotation_queue-context AGENTS.md, got:\n%s", aqAgents)
	}
}

func TestAppsInit_RejectsInvalidContextType(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldCustomAppStarter(dir, "my-app", "", "bogus", false); err == nil {
		t.Fatal("expected error for an invalid --context-type")
	}
}

func TestAppsInitCmd_HasContextTypeFlag(t *testing.T) {
	cmd := newAppsCmd()
	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	f := initCmd.Flags().Lookup("context-type")
	if f == nil {
		t.Fatal("expected --context-type flag to exist")
	}
	if f.DefValue != "annotation_queue" {
		t.Errorf("expected --context-type to default to annotation_queue, got %q", f.DefValue)
	}
}

func TestAppsInitCmd_HasSkipInstallFlag(t *testing.T) {
	cmd := newAppsCmd()
	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	if f := initCmd.Flags().Lookup("skip-install"); f == nil {
		t.Error("expected --skip-install flag to exist")
	}
}

func TestInstallAndBuildCustomAppStarter_ErrorsWhenNpmMissing(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	if err := installAndBuildCustomAppStarter(dir); err == nil {
		t.Error("expected error when npm is not on PATH")
	}
}

// fakeNpm writes an executable named "npm" into a fresh directory prepended
// to PATH, standing in for a real npm install: no network access, no real
// esbuild download. It only needs to satisfy the two invocations
// installAndBuildCustomAppStarter makes: "install" and "run build".
func fakeNpm(t *testing.T, onRunBuild string) {
	t.Helper()
	binDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"install\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"run\" ] && [ \"$2\" = \"build\" ]; then\n" + onRunBuild + "\nexit 0\nfi\n" +
		"exit 1\n"
	npmPath := filepath.Join(binDir, "npm")
	if err := os.WriteFile(npmPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInstallAndBuildCustomAppStarter_RunsInstallThenBuild(t *testing.T) {
	dir := t.TempDir()
	fakeNpm(t, `mkdir -p dist && printf 'module.exports={render:function(){}}' > dist/bundle.js`)

	if err := installAndBuildCustomAppStarter(dir); err != nil {
		t.Fatalf("installAndBuildCustomAppStarter: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "bundle.js")); err != nil {
		t.Errorf("expected dist/bundle.js to be produced by the build step: %v", err)
	}
}

func TestInstallAndBuildCustomAppStarter_ErrorsWhenBuildFails(t *testing.T) {
	dir := t.TempDir()
	fakeNpm(t, `exit 1`)

	if err := installAndBuildCustomAppStarter(dir); err == nil {
		t.Error("expected error when the build step fails")
	}
}
