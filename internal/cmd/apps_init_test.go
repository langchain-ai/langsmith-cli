package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Without this, "apps dev" run before the first "apps push" has no way to
// know this is an annotation_queue app at all (the queue-selector bar and
// --queue-id both key off .langsmith/app.json's context_type) — the file
// only existing after a push meant a brand new annotation-queue app was
// stuck showing "No queueId in context" until you pushed it first.
func TestAppsInit_WritesPartialAppLinkForImmediateAppsDevUse(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["annotation-queue"], false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil {
		t.Fatal("expected apps init to write .langsmith/app.json immediately")
	}
	if link.AppID != "" {
		t.Errorf("expected no app_id yet (app doesn't exist remotely until the first push), got %q", link.AppID)
	}
	if link.ContextType != "annotation_queue" {
		t.Errorf("expected context_type annotation_queue from --type, got %q", link.ContextType)
	}
}

func TestAppsInit_StandaloneAlsoWritesPartialAppLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["standalone"], false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.ContextType != "none" {
		t.Errorf("expected a partial link with context_type none, got %+v", link)
	}
}

func TestAppsInit_ScaffoldsAnnotationQueueFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "Does the thing", appTypes["annotation-queue"], false)
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

func TestAppsInit_ScaffoldsStandaloneFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["standalone"], false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	writtenSet := map[string]bool{}
	for _, w := range written {
		writtenSet[w] = true
	}
	for _, want := range []string{"package.json", "README.md", "AGENTS.md", ".gitignore", "src/index.js"} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
		}
	}
	// The standalone type must never scaffold the annotation-queue app's files.
	if writtenSet["src/App.tsx"] {
		t.Error("standalone type should not scaffold the annotation-queue React app")
	}
}

// Non-templated files (anything but README.md/package.json) must be copied
// byte-for-byte — running them through text/template would choke on, or
// silently mangle, any literal "{{"/"}}" they contain, e.g. React's
// style={{...}} inline-style syntax.
func TestAppsInit_CopiesNonTemplatedFilesVerbatim(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["annotation-queue"], false); err != nil {
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

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["annotation-queue"], false); err != nil {
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
	_, err := scaffoldCustomAppStarter(dir, "my-app", "", appTypes["annotation-queue"], false)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected not-empty error, got %v", err)
	}
}

func TestAppsInit_ForceWritesOverNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := scaffoldCustomAppStarter(dir, "my-app", "", appTypes["annotation-queue"], true); err != nil {
		t.Fatalf("scaffold with force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Errorf("package.json not written: %v", err)
	}
}

func TestAppsInit_RequiresName(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldCustomAppStarter(filepath.Join(dir, "app"), "", "", appTypes["annotation-queue"], false); err == nil {
		t.Fatal("expected error when --name is empty")
	}
}

func TestAppsInit_RequiresValidType(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffoldCustomAppStarter(dir, "my-app", "", appType{}, false); err == nil {
		t.Fatal("expected error for a zero-value (invalid) app type")
	}
}

func TestAppsInit_WritesContextSpecificAgentsMD(t *testing.T) {
	dir := t.TempDir()

	standaloneTarget := filepath.Join(dir, "standalone-app")
	if _, err := scaffoldCustomAppStarter(standaloneTarget, "standalone-app", "", appTypes["standalone"], false); err != nil {
		t.Fatalf("scaffold standalone: %v", err)
	}
	standaloneAgents, err := os.ReadFile(filepath.Join(standaloneTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(standaloneAgents), "standalone") {
		t.Errorf("expected the none-context AGENTS.md, got:\n%s", standaloneAgents)
	}

	aqTarget := filepath.Join(dir, "aq-app")
	if _, err := scaffoldCustomAppStarter(aqTarget, "aq-app", "", appTypes["annotation-queue"], false); err != nil {
		t.Fatalf("scaffold annotation-queue: %v", err)
	}
	aqAgents, err := os.ReadFile(filepath.Join(aqTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(aqAgents), "queueId") {
		t.Errorf("expected the annotation_queue-context AGENTS.md, got:\n%s", aqAgents)
	}
}

func TestAppsInitCmd_HasRequiredTypeFlag(t *testing.T) {
	cmd := newAppsCmd()
	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	f := initCmd.Flags().Lookup("type")
	if f == nil {
		t.Fatal("expected --type flag to exist")
	}
	if f.DefValue != "" {
		t.Errorf("expected --type to have no default (required), got %q", f.DefValue)
	}
	ann := f.Annotations
	if ann == nil || ann["cobra_annotation_bash_completion_one_required_flag"] == nil {
		t.Error("expected --type to be marked required")
	}
	// The old flag name/values must be gone, not just renamed silently.
	if got := initCmd.Flags().Lookup("context-type"); got != nil {
		t.Error("expected --context-type to be gone from apps init (renamed to --type)")
	}
}

func TestAppsInitCmd_RequiresType(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --type is missing")
	}
}

func TestAppsInitCmd_RejectsInvalidType(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app", "--type", "bogus", "--skip-install"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for an invalid --type")
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
