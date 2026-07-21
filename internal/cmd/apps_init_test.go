package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

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

func TestAppsInit_ScaffoldsCodingAgentDashboardFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["coding-agent-dashboard"], false)
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
		"src/components/ProjectBar.tsx",
		"src/components/OverviewPanel.tsx",
		"src/components/RunsTable.tsx",
	} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
		}
	}
	// It must not carry the annotation-queue templates' components.
	if writtenSet["src/components/QueueBar.tsx"] || writtenSet["src/components/DataGrid.tsx"] {
		t.Error("coding-agent dashboard should not scaffold annotation-queue components")
	}

	agents, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), "ls_agent_purpose") || !strings.Contains(string(agents), "runs/query") {
		t.Errorf("expected the coding-agent dashboard's AGENTS.md, got:\n%s", agents)
	}
}

func TestAppsInit_ScaffoldsExperimentComparisonFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["experiment-comparison"], false)
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
		"src/lib/delta.ts",
		"src/lib/metrics.ts",
		"src/components/Pickers.tsx",
		"src/components/SummaryPanel.tsx",
		"src/components/ExampleTable.tsx",
		"src/components/Scorecard.tsx",
		"src/components/ScatterPlot.tsx",
	} {
		if !writtenSet[want] {
			t.Errorf("expected %q to be written, got %v", want, written)
		}
	}
	// It must not carry other templates' components.
	if writtenSet["src/components/QueueBar.tsx"] || writtenSet["src/components/PieChart.tsx"] {
		t.Error("experiment-comparison should not scaffold other templates' components")
	}

	agents, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agents), "datasets/{dataset_id}/runs") || !strings.Contains(string(agents), "delta.ts") {
		t.Errorf("expected the experiment-comparison AGENTS.md, got:\n%s", agents)
	}
}

// Guards the per-template agentsMD selection against regression.
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

// Non-templated files must be copied byte-for-byte, not parsed as templates.
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
	if !strings.Contains(string(blankAgents), "window.langsmith.call") {
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
	if !strings.Contains(string(aqAgents), "3-pane") {
		t.Errorf("expected the annotation-queue template's AGENTS.md, got:\n%s", aqAgents)
	}
}

func TestAppsInitCmd_TemplateFlagDefaultsEmpty(t *testing.T) {
	cmd := newAppsCmd()
	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	f := initCmd.Flags().Lookup("template")
	if f == nil {
		t.Fatal("expected --template flag to exist")
	}
	if f.DefValue != "" {
		t.Errorf("expected --template to default to \"\", got %q", f.DefValue)
	}
	// The old --type flag must be gone, not just aliased.
	if got := initCmd.Flags().Lookup("type"); got != nil {
		t.Error("expected --type to be gone from apps init (renamed to --template)")
	}
}

func TestAppsInitCmd_RejectsExplicitTemplateBlank(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app", "--template", "blank"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected --template blank to be rejected; omit --template instead")
	}
}

func TestAppsInitCmd_DefaultsToBlankWhenTemplateOmitted(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app"})
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
	cmd.SetArgs([]string{"init", "--name", "grid-app", "--template", "annotation-queue-grid"})
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

func TestAppsInitCmd_AcceptsCodingAgentDashboardTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "dash", "--template", "coding-agent-dashboard"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to succeed for --template coding-agent-dashboard, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "components", "OverviewPanel.tsx")); err != nil {
		t.Errorf("expected the dashboard's OverviewPanel.tsx to be scaffolded: %v", err)
	}
}

func TestAppsInitCmd_AcceptsExperimentComparisonTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "cmp", "--template", "experiment-comparison"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected init to succeed for --template experiment-comparison, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "components", "SummaryPanel.tsx")); err != nil {
		t.Errorf("expected the SummaryPanel.tsx to be scaffolded: %v", err)
	}
}

func TestAppsInitCmd_RejectsInvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := newAppsCmd()
	cmd.SetArgs([]string{"init", "--name", "my-app", "--template", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for an invalid --template")
	}
}

// fakeNpm stubs "npm" on PATH so tests avoid a real network install.
func fakeNpm(t *testing.T, onRunBuild string) {
	t.Helper()
	binDir := t.TempDir()
	npmName := "npm"
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"install\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"run\" ] && [ \"$2\" = \"build\" ]; then\n" + onRunBuild + "\nexit 0\nfi\n" +
		"exit 1\n"
	if runtime.GOOS == "windows" {
		npmName = "npm.cmd"
		if onRunBuild == "exit 1" {
			script = "@if \"%1\"==\"install\" exit /b 0\r\n@if \"%1 %2\"==\"run build\" exit /b 1\r\n@exit /b 1\r\n"
		} else {
			script = "@if \"%1\"==\"install\" exit /b 0\r\n@if \"%1 %2\"==\"run build\" (mkdir dist 2>nul & echo module.exports={} > dist\\bundle.js & exit /b 0)\r\n@exit /b 1\r\n"
		}
		posixScript := "#!/bin/sh\n" +
			"if [ \"$1\" = \"install\" ]; then exit 0; fi\n" +
			"if [ \"$1\" = \"run\" ] && [ \"$2\" = \"build\" ]; then\n" + onRunBuild + "\nexit 0\nfi\n" +
			"exit 1\n"
		if err := os.WriteFile(filepath.Join(binDir, "npm"), []byte(posixScript), 0o755); err != nil {
			t.Fatalf("write POSIX fake npm: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(binDir, npmName), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSharedFileImportSpecifiers_MatchesTemplateConventions(t *testing.T) {
	tests := []struct {
		sharedRelPath string
		want          []string
	}{
		{
			sharedRelPath: "components/SearchableSelect.tsx",
			want: []string{
				"./SearchableSelect",
				"./components/SearchableSelect",
				"../components/SearchableSelect",
			},
		},
		{
			sharedRelPath: "lib/utils.ts",
			want: []string{
				"./utils",
				"./lib/utils",
				"../lib/utils",
			},
		},
	}
	for _, tt := range tests {
		got := sharedFileImportSpecifiers(tt.sharedRelPath)
		gotSet := map[string]bool{}
		for _, g := range got {
			gotSet[g] = true
		}
		for _, want := range tt.want {
			if !gotSet[want] {
				t.Errorf("sharedFileImportSpecifiers(%q) = %v, missing %q", tt.sharedRelPath, got, want)
			}
		}
	}
}

func TestAppsInit_PullsInEverySharedFileATemplateImports(t *testing.T) {
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
	for _, sharedRelPath := range []string{
		"src/components/SearchableSelect.tsx",
		"src/components/Spinner.tsx",
		"src/lib/utils.ts",
	} {
		if !writtenSet[sharedRelPath] {
			t.Errorf("expected %q to be pulled in from _shared/, got %v", sharedRelPath, written)
			continue
		}
		got, err := os.ReadFile(filepath.Join(target, sharedRelPath))
		if err != nil {
			t.Fatalf("reading scaffolded %s: %v", sharedRelPath, err)
		}
		want, err := sharedFS.ReadFile(sharedRoot + "/" + strings.TrimPrefix(sharedRelPath, "src/"))
		if err != nil {
			t.Fatalf("reading embedded shared %s: %v", sharedRelPath, err)
		}
		if string(got) != string(want) {
			t.Errorf("scaffolded %s does not match the canonical _shared/ source", sharedRelPath)
		}
	}
}

func TestAppsInit_OnlyPullsInSharedFilesActuallyImported(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	written, err := scaffoldCustomAppStarter(target, "my-app", "", appTypes["experiment-comparison"], false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	writtenSet := map[string]bool{}
	for _, w := range written {
		writtenSet[w] = true
	}
	for _, wanted := range []string{"src/components/SearchableSelect.tsx", "src/components/Spinner.tsx", "src/lib/utils.ts"} {
		if !writtenSet[wanted] {
			t.Errorf("expected %q to be pulled in, got %v", wanted, written)
		}
	}
}

// Embedded files ignore .gitignore, so stray build artifacts in a template's
// source tree get silently baked into the CLI binary. Checks the
// embedded FS content directly, not just the source tree.
func TestEmbeddedTemplates_CarryNoStrayBuildArtifacts(t *testing.T) {
	check := func(name string, embedded fs.FS) {
		t.Helper()
		err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			base := filepath.Base(path)
			if d.IsDir() && (base == "node_modules" || base == "dist") {
				t.Errorf("%s: embedded FS contains %q — a local build wasn't cleaned up before go build/go install", name, path)
				return fs.SkipDir
			}
			if !d.IsDir() && (base == "package-lock.json" || base == "pnpm-lock.yaml") {
				t.Errorf("%s: embedded FS contains %q — shouldn't be committed or embedded", name, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("%s: walking embedded FS: %v", name, err)
		}
	}

	for name, at := range appTypes {
		check(name, at.templateFS)
	}
	check("_shared", sharedFS)
}
