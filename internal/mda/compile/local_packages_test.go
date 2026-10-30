package compile

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestIsVendoredPathSkipsInstallsCachesAndEnvFiles(t *testing.T) {
	for path, want := range map[string]bool{
		"src/index.ts":              true,
		"dist/index.js":             true,
		"node_modules/zod/index.js": false,
		"pkg/__pycache__/a.pyc":     false,
		".env":                      false,
	} {
		if got := IsVendoredPath(filepath.FromSlash(path)); got != want {
			t.Fatalf("%s: got %v", path, got)
		}
	}
}

func TestUvPathSourcesSkipSourcesOutsideProjectDependencies(t *testing.T) {
	sources, err := uvPathSources("[project]\n" +
		"dependencies = [\"My_Dep[fast]>=1\"]\n\n" +
		"[dependency-groups]\ndev = [\"dev-tool\"]\n\n" +
		"[tool.uv.sources]\n" +
		"my-dep = { path = \"../dep\" }\n" +
		"dev-tool = { path = \"../dev-tool\" }\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sources, []localPath{{"my-dep", "../dep"}}) {
		t.Fatalf("got %v", sources)
	}
}

func TestUvPathSourcesReadOnlyPathSources(t *testing.T) {
	sources, err := uvPathSources("[project]\n" +
		"dependencies = [\"my-dep\", \"other\"]\n\n" +
		"[tool.uv.sources]\n" +
		"my-dep = { path = \"../dep\", editable = true }\n" +
		"other = { git = \"https://example.com/other\" }\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sources, []localPath{{"my-dep", "../dep"}}) {
		t.Fatalf("got %v", sources)
	}
}

func TestIsPackageNameAcceptsOnlyPlainComponents(t *testing.T) {
	for name, want := range map[string]bool{
		"dep":         true,
		"@scope/dep":  true,
		"a/./b":       true,
		"../dep":      false,
		"./dep":       false,
		"a/../b":      false,
		"/abs":        false,
		"dep/..":      false,
		".hidden-dep": true,
	} {
		if got := isPackageName(name); got != want {
			t.Fatalf("%s: got %v", name, got)
		}
	}
}

// workspace is a directory holding a project in `app/` and the local packages
// next to it.
type workspace struct {
	dir string
	fixture
}

func tsProjectWithLocalPackage(t *testing.T, dependency string) workspace {
	t.Helper()
	dir := canonicalTempDir(t)
	writeFiles(t, dir, map[string]string{
		"dep/package.json":              `{"name":"dep","version":"1.0.0","main":"index.js","dependencies":{"zod":"^3.0.0"},"devDependencies":{"vitest":"^3.0.0"}}`,
		"dep/index.js":                  "export const helper = () => 1;\n",
		"dep/node_modules/zod/index.js": "",
		"dep/.env":                      "SECRET=1\n",
	})
	f := typescriptProjectAt(t, filepath.Join(dir, "app"), map[string]string{
		"agent.ts":     tsAgent,
		"package.json": fmt.Sprintf(`{"name":"app","type":"module","dependencies":{"managed-deepagents":"^0.0.2","dep":"%s"}}`, dependency),
	})
	return workspace{dir: dir, fixture: f}
}

func (w workspace) readJSON(t *testing.T, rel string) map[string]any {
	t.Helper()
	return readJSON(t, w.read(t, rel))
}

func TestCompileVendorsATypescriptLocalPackageAsAWorkspace(t *testing.T) {
	for _, spec := range []string{"file:../dep", "link:../dep"} {
		t.Run(spec, func(t *testing.T) {
			w := tsProjectWithLocalPackage(t, spec)

			mustCompile(t, w.local)

			if !w.exists("__local__/dep/index.js") || !w.exists("__local__/dep/package.json") ||
				w.exists("__local__/dep/node_modules") || w.exists("__local__/dep/.env") {
				t.Fatal("the local package was not vendored as expected")
			}
			manifest := w.readJSON(t, "package.json")
			if _, ok := manifest["dependencies"].(map[string]any)["dep"]; ok {
				t.Fatalf("package.json: %v", manifest)
			}
			if !slices.Contains(manifest["workspaces"].([]any), any("__local__/dep")) {
				t.Fatalf("workspaces: %v", manifest["workspaces"])
			}
		})
	}
}

func TestCompileForDeployStripsDevDependenciesFromTypescriptLocalPackages(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")

	mustCompile(t, w.deploy)

	want := "{\n  \"name\": \"dep\",\n  \"version\": \"1.0.0\",\n  \"main\": \"index.js\",\n" +
		"  \"dependencies\": {\n    \"zod\": \"^3.0.0\"\n  }\n}\n"
	if got := w.read(t, "__local__/dep/package.json"); got != want {
		t.Fatalf("__local__/dep/package.json:\n%s", got)
	}
}

func TestCompileLeavesTypescriptLocalPackagesInsideTheProjectAlone(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:./packages/dep")
	writeFile(t, w.root(), "packages/dep/package.json", `{"name":"dep","version":"1.0.0"}`)
	w.reload(t)

	mustCompile(t, w.local)

	if w.exists("__local__") {
		t.Fatal("a package inside the project was vendored")
	}
	if got := w.readJSON(t, "package.json")["dependencies"].(map[string]any)["dep"]; got != "file:./packages/dep" {
		t.Fatalf("dep: %v", got)
	}
}

func TestCompileRejectsAMissingLocalPackage(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../missing")

	_, err := w.local(t)
	if err == nil || err.Error() != "local package `dep` points to `../missing`, which does not exist" {
		t.Fatalf("got %v", err)
	}
}

func TestCompileVendorsNestedTypescriptLocalPackagesAsWorkspaces(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFiles(t, w.dir, map[string]string{
		"dep/package.json":   `{"name":"dep","version":"1.0.0","dependencies":{"zod":"^3.0.0","other":"file:../other"}}`,
		"other/package.json": `{"name":"other","version":"1.0.0","main":"index.js"}`,
		"other/index.js":     "export const other = 1;\n",
	})

	mustCompile(t, w.local)

	if !w.exists("__local__/other/index.js") {
		t.Fatal("the nested package was not vendored")
	}
	workspaces := w.readJSON(t, "package.json")["workspaces"]
	if !reflect.DeepEqual(workspaces, []any{"__runtime__", "__local__/dep", "__local__/other"}) {
		t.Fatalf("workspaces: %v", workspaces)
	}
	if got := w.read(t, "__local__/dep/package.json"); got != "{\n  \"name\": \"dep\",\n  \"version\": \"1.0.0\",\n"+
		"  \"dependencies\": {\n    \"zod\": \"^3.0.0\"\n  }\n}\n" {
		t.Fatalf("__local__/dep/package.json:\n%s", got)
	}
}

func TestCompileCopiesASharedLocalPackageOnce(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFiles(t, w.dir, map[string]string{
		"app/package.json":   `{"name":"app","type":"module","dependencies":{"dep":"file:../dep","other":"file:../other"}}`,
		"dep/package.json":   `{"name":"dep","version":"1.0.0","dependencies":{"other":"file:../other"}}`,
		"other/package.json": `{"name":"other"}`,
	})

	mustCompile(t, w.local)

	workspaces := w.readJSON(t, "package.json")["workspaces"]
	if !reflect.DeepEqual(workspaces, []any{"__runtime__", "__local__/dep", "__local__/other"}) {
		t.Fatalf("workspaces: %v", workspaces)
	}
}

func TestCompileRejectsTwoLocalPackagesWithTheSameName(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFiles(t, w.dir, map[string]string{
		"dep/package.json":  `{"name":"dep","version":"1.0.0","dependencies":{"dep":"file:../copy"}}`,
		"copy/package.json": `{"name":"dep"}`,
	})

	_, err := w.local(t)
	requireErrorContains(t, err, fmt.Sprintf("two different local packages are named `dep`: %s and %s",
		filepath.Join(w.dir, "dep"), filepath.Join(w.dir, "copy")))
}

func TestCompileRejectsALocalPackageDeclaredUnderAnotherName(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFile(t, w.dir, "dep/package.json", `{"name":"@acme/actual","version":"1.0.0","main":"index.js"}`)

	_, err := w.local(t)
	if err == nil || err.Error() != "local package `dep` at `../dep` is named `@acme/actual` in its package.json; "+
		"declare it as `@acme/actual`" {
		t.Fatalf("got %v", err)
	}
}

func TestCompileRejectsAProjectPackageThatDeclaresAPackageOutsideTheProject(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:./packages/dep")
	writeFiles(t, w.dir, map[string]string{
		"app/packages/dep/package.json": `{"name":"dep","version":"1.0.0","dependencies":{"shared":"file:../../../shared"}}`,
		"shared/package.json":           `{"name":"shared"}`,
	})
	w.reload(t)

	_, err := w.local(t)
	if err == nil || err.Error() != "local package `shared` at `../../../shared` is declared by `"+
		filepath.Join("packages", "dep")+"`, which is inside the project, but resolves outside it; "+
		"move `shared` into the project" {
		t.Fatalf("got %v", err)
	}
}

func TestCompileWarnsAboutTypescriptImportsOutsideTheProject(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFiles(t, w.dir, map[string]string{
		"dep/helper.js": "export const helper = 1;\n",
		"app/local.ts":  "export const local = 1;\n",
		"app/agent.ts": "import { helper } from \"../dep/helper.js\";\n" +
			"import type { Shape } from \"../dep/types\";\n" +
			"import { local } from \"./local\";\n" + tsAgent,
	})
	w.reload(t)

	summary := mustCompile(t, w.local)

	var outside []string
	for _, warning := range summary.Warnings {
		if strings.Contains(warning, "outside the project") {
			outside = append(outside, warning)
		}
	}
	if !reflect.DeepEqual(outside, []string{
		"agent.ts imports \"../dep/helper.js\" from outside the project, so the build does not include it. " +
			"Make it a local package (`npm install <path>`) or move it into the project.",
	}) {
		t.Fatalf("warnings: %q", summary.Warnings)
	}
}

func TestCompileLeavesALocalRuntimeDependencyToTheVendoredRuntime(t *testing.T) {
	w := tsProjectWithLocalPackage(t, "file:../dep")
	writeFiles(t, w.dir, map[string]string{
		"app/package.json":     `{"name":"app","type":"module","dependencies":{"managed-deepagents":"file:../runtime"}}`,
		"runtime/package.json": `{"name":"managed-deepagents","version":"0.0.0"}`,
	})

	mustCompile(t, w.local)

	if w.exists("__local__") {
		t.Fatal("the runtime was vendored as a local package")
	}
	if got := w.readJSON(t, "package.json")["workspaces"]; !reflect.DeepEqual(got, []any{"__runtime__"}) {
		t.Fatalf("workspaces: %v", got)
	}
}

func pyProjectWithLocalPackage(t *testing.T, pyproject string) workspace {
	t.Helper()
	dir := canonicalTempDir(t)
	writeFiles(t, dir, map[string]string{
		"dep/pyproject.toml":           "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\ndependencies = [\"httpx\"]\n",
		"dep/my_dep/__init__.py":       "VALUE = 1\n",
		"dep/my_dep/__pycache__/x.pyc": "",
		"dep/.venv/bin/python":         "",
	})
	if pyproject == "" {
		pyproject = "[project]\nname = \"app\"\nversion = \"0.0.0\"\n" +
			"dependencies = [\"managed-deepagents\", \"my-dep\"]\n\n" +
			"[tool.uv.sources]\nmy-dep = { path = \"../dep\" }\n"
	}
	f := pythonProjectAt(t, filepath.Join(dir, "app"), map[string]string{"agent.py": pyAgent, "pyproject.toml": pyproject})
	return workspace{dir: dir, fixture: f}
}

// readToml decodes a TOML file of the build.
func (w workspace) readToml(t *testing.T, rel string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := toml.Unmarshal([]byte(w.read(t, rel)), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func decodedTable(t *testing.T, doc map[string]any, path ...string) map[string]any {
	t.Helper()
	for _, name := range path {
		next, ok := doc[name].(map[string]any)
		if !ok {
			t.Fatalf("no table %v in %v", path, doc)
		}
		doc = next
	}
	return doc
}

func TestCompilePointsAPythonLocalBuildAtTheVendoredLocalPackage(t *testing.T) {
	w := pyProjectWithLocalPackage(t, "")

	mustCompile(t, w.local)

	if !w.exists("__local__/my-dep/my_dep/__init__.py") || w.exists("__local__/my-dep/.venv") ||
		w.exists("__local__/my-dep/my_dep/__pycache__") {
		t.Fatal("the local package was not vendored as expected")
	}
	pyproject := w.readToml(t, "pyproject.toml")
	source := decodedTable(t, pyproject, "tool", "uv", "sources", "my-dep")
	if source["path"] != "__local__/my-dep" || source["editable"] != true {
		t.Fatalf("source: %v", source)
	}
	if !slices.Contains(decodedTable(t, pyproject, "project")["dependencies"].([]any), any("my-dep")) {
		t.Fatalf("dependencies: %v", pyproject["project"])
	}
}

func TestCompileForDeployInstallsAPythonLocalPackageFromLanggraphJSON(t *testing.T) {
	w := pyProjectWithLocalPackage(t, "")

	mustCompile(t, w.deploy)

	if got := readJSON(t, w.read(t, "langgraph.json"))["dependencies"]; !reflect.DeepEqual(got,
		[]any{".", "./__runtime__", "./__local__/my-dep"}) {
		t.Fatalf("dependencies: %v", got)
	}
	want := "[project]\nname = \"app\"\nversion = \"0.0.0\"\n" +
		"dependencies = [ \"deepagents>=0.7.30,<1\", \"langchain[mcp]>=1.4.6,<2\", \"langgraph>=1.2.15,<2\", " +
		"\"langgraph-prebuilt>=1.1.1,<2\", \"langgraph-checkpoint>=4.2.0\", \"langgraph-sdk>=0.4.6,<0.5\", " +
		"\"pyjwt[crypto]>=2.8.0\", \"fastmcp>=4.0.10,<5\", \"mcp>=2.2.0,<3\", \"langsmith>=0.10.17\", " +
		"\"langchain-openai>=1.4.2\"]\n" +
		"\n[tool.uv.sources]\n" +
		"\n[tool.setuptools]\npy-modules = []\n" +
		"\n[build-system]\nrequires = [\"setuptools>=61\"]\nbuild-backend = \"setuptools.build_meta\"\n"
	if got := w.read(t, "pyproject.toml"); got != want {
		t.Fatalf("pyproject.toml:\n%s", got)
	}
}

func TestCompileLeavesALocalPythonRuntimeSourceToTheVendoredRuntime(t *testing.T) {
	w := pyProjectWithLocalPackage(t, "[project]\nname = \"app\"\nversion = \"0.0.0\"\n"+
		"dependencies = [\"managed-deepagents\"]\n\n"+
		"[tool.uv.sources]\nmanaged_deepagents = { path = \"../runtime\" }\n")
	writeFile(t, w.dir, "runtime/pyproject.toml", "[project]\nname = \"managed-deepagents\"\nversion = \"0.0.0\"\n")

	mustCompile(t, w.local)

	if w.exists("__local__") {
		t.Fatal("the runtime was vendored as a local package")
	}
	if sources := decodedTable(t, w.readToml(t, "pyproject.toml"), "tool", "uv", "sources"); len(sources) != 0 {
		t.Fatalf("sources: %v", sources)
	}
}

func nestedPythonLocalPackages(t *testing.T) workspace {
	t.Helper()
	w := pyProjectWithLocalPackage(t, "")
	writeFiles(t, w.dir, map[string]string{
		"dep/pyproject.toml": "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\n" +
			"dependencies = [\"httpx\", \"other\"]\n\n[tool.uv.sources]\nother = { path = \"../other\" }\n",
		"other/pyproject.toml":    "[project]\nname = \"other\"\nversion = \"0.1.0\"\n",
		"other/other/__init__.py": "VALUE = 2\n",
	})
	return w
}

func TestCompilePointsANestedPythonLocalPackageAtItsSiblingCopy(t *testing.T) {
	w := nestedPythonLocalPackages(t)

	mustCompile(t, w.local)

	if !w.exists("__local__/other/other/__init__.py") {
		t.Fatal("the nested package was not vendored")
	}
	want := "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\n" +
		"dependencies = [\"httpx\", \"other\"]\n\n[tool.uv.sources]\nother = { path = \"../other\", editable = true }\n"
	if got := w.read(t, "__local__/my-dep/pyproject.toml"); got != want {
		t.Fatalf("__local__/my-dep/pyproject.toml:\n%s", got)
	}
	if _, ok := decodedTable(t, w.readToml(t, "pyproject.toml"), "tool", "uv", "sources")["other"]; ok {
		t.Fatal("the nested package is a source of the project")
	}
}

func TestCompileForDeployInstallsNestedPythonLocalPackagesFromLanggraphJSON(t *testing.T) {
	w := nestedPythonLocalPackages(t)

	mustCompile(t, w.deploy)

	if got := readJSON(t, w.read(t, "langgraph.json"))["dependencies"]; !reflect.DeepEqual(got,
		[]any{".", "./__runtime__", "./__local__/my-dep", "./__local__/other"}) {
		t.Fatalf("dependencies: %v", got)
	}
	want := "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\ndependencies = [\"httpx\"]\n\n[tool.uv.sources]\n"
	if got := w.read(t, "__local__/my-dep/pyproject.toml"); got != want {
		t.Fatalf("__local__/my-dep/pyproject.toml:\n%s", got)
	}
}

func pythonProjectRequiringLocalPackage(t *testing.T, requirement string) workspace {
	t.Helper()
	w := pyProjectWithLocalPackage(t, fmt.Sprintf("[project]\nname = \"app\"\nversion = \"0.0.0\"\n"+
		"dependencies = [\"managed-deepagents\", %q]\n\n[tool.uv.sources]\nmy-dep = { path = \"../dep\" }\n", requirement))
	writeFile(t, w.dir, "dep/pyproject.toml", "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\n"+
		"dependencies = [\"httpx\"]\n\n[project.optional-dependencies]\nfast = [\"orjson\"]\n"+
		"all = [\"my-dep[fast]\", \"rich\"]\n")
	return w
}

func TestCompileForDeployInstallsTheExtrasOfAPythonLocalPackage(t *testing.T) {
	w := pythonProjectRequiringLocalPackage(t, "my-dep[all]")

	mustCompile(t, w.deploy)

	want := "[project]\nname = \"my-dep\"\nversion = \"0.1.0\"\ndependencies = [\"httpx\", \"rich\", \"orjson\"]\n\n" +
		"[project.optional-dependencies]\nfast = [\"orjson\"]\nall = [\"my-dep[fast]\", \"rich\"]\n"
	if got := w.read(t, "__local__/my-dep/pyproject.toml"); got != want {
		t.Fatalf("__local__/my-dep/pyproject.toml:\n%s", got)
	}
}

func TestCompileForDeployRejectsAMarkerOnAPythonLocalPackage(t *testing.T) {
	w := pythonProjectRequiringLocalPackage(t, "my-dep; sys_platform == 'linux'")

	_, err := w.deploy(t)
	requireErrorContains(t, err, "langsmith deploy always installs local package `my-dep`, so it cannot honor the "+
		"marker `sys_platform == 'linux'`. Remove the marker from `my-dep; sys_platform == 'linux'`.")
}

func TestVendoredManifestsListLocalPackageManifestsInPathOrder(t *testing.T) {
	out := t.TempDir()
	writeFiles(t, out, map[string]string{
		"__local__/b/package.json":             "{}",
		"__local__/@scope/a/package.json":      "{}",
		"__local__/a/package.json":             "{}",
		"__local__/a/nested/deep/package.json": "{}",
		"__local__/package.json":               "{}",
	})

	manifests, err := vendoredManifests(out, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join("__local__", "@scope", "a", "package.json"),
		filepath.Join("__local__", "a", "package.json"),
		filepath.Join("__local__", "b", "package.json"),
	}
	if !reflect.DeepEqual(manifests, want) {
		t.Fatalf("got %v", manifests)
	}
	if manifests, err := vendoredManifests(filepath.Join(out, "missing"), "package.json"); err != nil || manifests != nil {
		t.Fatalf("missing build: %v %v", manifests, err)
	}
}
