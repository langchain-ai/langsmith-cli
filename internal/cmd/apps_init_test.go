package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "apps init" records the name in .langsmith/app.json so a later push reuses
// it instead of the directory basename.
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
	if link.Name != "my-app" {
		t.Errorf("expected --name to be recorded in the link file so a later \"apps push\" doesn't fall back to the directory basename, got %q", link.Name)
	}
}

func TestAppsInit_BlankAlsoWritesPartialAppLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["blank"], false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.Name != "my-app" || link.AppID != "" {
		t.Errorf("expected a partial link recording the name with no app_id, got %+v", link)
	}
}

// The grid variant scaffolds its own spreadsheet UI, not the 3-pane components.
func TestAppsInit_ScaffoldsAnnotationQueueGridFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["annotation-queue-grid"], false)
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
		"tsconfig.json",
		"src/entry.tsx",
		"src/App.tsx",
		"src/api.ts",
		"src/types.ts",
		"src/components/DataGrid.tsx",
		"src/components/GridCell.tsx",
	} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
		}
	}
	// The grid variant must not carry the 3-pane app's components.
	for _, unwanted := range []string{
		"src/components/RunList.tsx",
		"src/components/RunViewer.tsx",
		"src/components/FeedbackPanel.tsx",
	} {
		if writtenSet[unwanted] {
			t.Errorf("grid variant should not scaffold the 3-pane component %q", unwanted)
		}
	}

	// A partial link is written recording the name (no app_id until push).
	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.Name != "my-app" {
		t.Errorf("expected a partial link recording the name, got %+v", link)
	}
}

// The two AQ templates must get distinct AGENTS.md files — guards the per-template agentsMD selection against regression.
func TestAppsInit_GridGetsDistinctAgentsMD(t *testing.T) {
	dir := t.TempDir()

	gridTarget := filepath.Join(dir, "grid-app")
	if _, err := scaffoldCustomAppStarter(gridTarget, "grid-app", "", appTypes["annotation-queue-grid"], false); err != nil {
		t.Fatalf("scaffold grid: %v", err)
	}
	gridAgents, err := os.ReadFile(filepath.Join(gridTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read grid AGENTS.md: %v", err)
	}
	if !strings.Contains(string(gridAgents), "spreadsheet") || !strings.Contains(string(gridAgents), "DataGrid.tsx") {
		t.Errorf("expected the grid-specific AGENTS.md, got:\n%s", gridAgents)
	}

	paneTarget := filepath.Join(dir, "pane-app")
	if _, err := scaffoldCustomAppStarter(paneTarget, "pane-app", "", appTypes["annotation-queue"], false); err != nil {
		t.Fatalf("scaffold 3-pane: %v", err)
	}
	paneAgents, err := os.ReadFile(filepath.Join(paneTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read 3-pane AGENTS.md: %v", err)
	}
	if string(gridAgents) == string(paneAgents) {
		t.Error("expected the grid variant to get a different AGENTS.md than the 3-pane template")
	}
	if strings.Contains(string(paneAgents), "DataGrid.tsx") {
		t.Error("the 3-pane AGENTS.md should not mention the grid's DataGrid component")
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

func TestAppsInit_ScaffoldsBlankFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["blank"], false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	writtenSet := map[string]bool{}
	for _, w := range written {
		writtenSet[w] = true
	}
	for _, want := range []string{"package.json", "README.md", "AGENTS.md", ".gitignore", "src/App.tsx", "src/entry.tsx"} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
		}
	}
	// The blank template must never scaffold the annotation-queue app's own
	// components — it has its own (much simpler) App.tsx.
	if writtenSet["src/components/RunList.tsx"] {
		t.Error("blank template should not scaffold the annotation-queue app's components")
	}

	appTsx, err := os.ReadFile(filepath.Join(target, "src", "App.tsx"))
	if err != nil {
		t.Fatalf("read App.tsx: %v", err)
	}
	if strings.Contains(string(appTsx), "queueId") {
		t.Errorf("expected the blank App.tsx to have no queue-specific code, got:\n%s", appTsx)
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

func TestAppsInit_WritesTemplateSpecificAgentsMD(t *testing.T) {
	dir := t.TempDir()

	blankTarget := filepath.Join(dir, "blank-app")
	if _, err := scaffoldCustomAppStarter(blankTarget, "blank-app", "", appTypes["blank"], false); err != nil {
		t.Fatalf("scaffold blank: %v", err)
	}
	blankAgents, err := os.ReadFile(filepath.Join(blankTarget, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(blankAgents), "standalone") {
		t.Errorf("expected the blank template's AGENTS.md, got:\n%s", blankAgents)
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
		t.Errorf("expected the annotation-queue template's AGENTS.md, got:\n%s", aqAgents)
	}
}

func TestAppsInitCmd_TemplateDefaultsToBlank(t *testing.T) {
	cmd := newAppsCmd()
	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	f := initCmd.Flags().Lookup("template")
	if f == nil {
		t.Fatal("expected --template flag to exist")
	}
	if f.DefValue != "blank" {
		t.Errorf("expected --template to default to \"blank\", got %q", f.DefValue)
	}
	// The old --type flag must be gone, not just aliased.
	if got := initCmd.Flags().Lookup("type"); got != nil {
		t.Error("expected --type to be gone from apps init (renamed to --template)")
	}
}

func TestAppsInitCmd_DefaultsToBlankWhenTemplateOmitted(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app", "--skip-install"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to succeed with --template omitted, got: %v", err)
	}

	// Omitting --template scaffolds the blank starter: a partial link and a
	// queue-free App.tsx.
	link, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.Name != "my-app" {
		t.Errorf("expected a partial link recording the name, got %+v", link)
	}
	appTsx, err := os.ReadFile(filepath.Join(dir, "src", "App.tsx"))
	if err != nil {
		t.Fatalf("read App.tsx: %v", err)
	}
	if strings.Contains(string(appTsx), "queueId") {
		t.Errorf("expected the blank App.tsx (no queue code) when --template is omitted, got:\n%s", appTsx)
	}
}

func TestAppsInitCmd_AcceptsAnnotationQueueGridTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "grid-app", "--template", "annotation-queue-grid", "--skip-install"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to succeed for --template annotation-queue-grid, got: %v", err)
	}

	// The grid variant's own component is scaffolded.
	if _, err := os.Stat(filepath.Join(dir, "src", "components", "DataGrid.tsx")); err != nil {
		t.Errorf("expected the grid template's DataGrid.tsx to be scaffolded: %v", err)
	}
	link, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.Name != "grid-app" {
		t.Errorf("expected a partial link recording the name, got %+v", link)
	}
}

func TestAppsInitCmd_RejectsInvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app", "--template", "bogus", "--skip-install"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for an invalid --template")
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
