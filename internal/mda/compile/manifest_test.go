package compile

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestPythonManagedExtrasAreNotSuppressedByAnAuthorPin(t *testing.T) {
	dependency := project.Dependency{Name: "langchain[mcp]", Version: ">=1.4.3,<2"}
	for _, tc := range []struct {
		authored string
		want     string
	}{
		{"langchain==1.4.3", "[\"langchain==1.4.3\", \"langchain[mcp]>=1.4.3,<2\"]"},
		{"LangChain[other]==1.4.3", "[\"LangChain[other]==1.4.3\", \"langchain[mcp]>=1.4.3,<2\"]"},
		{"langchain[mcp]==1.4.3", "[\"langchain[mcp]==1.4.3\"]"},
		{"LangChain[other, MCP]==1.4.3", "[\"LangChain[other, MCP]==1.4.3\"]"},
	} {
		root := t.TempDir()
		writeFile(t, root, "pyproject.toml", "[project]\ndependencies = [\""+tc.authored+"\"]\n")
		// Repeated compilation neither rewrites the pin nor duplicates extras.
		for range 2 {
			if err := mergePyprojectDeps(root, []project.Dependency{dependency}); err != nil {
				t.Fatal(err)
			}
		}
		if got := readFile(t, filepath.Join(root, "pyproject.toml")); got != "[project]\ndependencies = "+tc.want+"\n" {
			t.Fatalf("%s: got %q", tc.authored, got)
		}
	}
}

func TestMergePyprojectDepsMatchesTheArrayStyle(t *testing.T) {
	dependencies := []project.Dependency{{Name: "a", Version: ">=1"}, {Name: "b", Version: ">=2"}}
	for authored, want := range map[string]string{
		// An empty array is laid out one entry per line.
		"[project]\ndependencies = []\n": "[project]\ndependencies = [\n    \"a>=1\",\n    \"b>=2\"\n]\n",
		// A multi-line array keeps its layout and loses its trailing comma.
		"[project]\ndependencies = [\n  \"x\",\n]\n": "[project]\ndependencies = [\n  \"x\",\n    \"a>=1\",\n    \"b>=2\"\n]\n",
		// A missing array is created; so is a missing `[project]`.
		"[project]\nname = \"x\"\n": "[project]\nname = \"x\"\ndependencies = [\n    \"a>=1\",\n    \"b>=2\"\n]\n",
		"[tool.x]\ny = 1\n":         "[tool.x]\ny = 1\n\n[project]\ndependencies = [\n    \"a>=1\",\n    \"b>=2\"\n]\n",
		// The last element loses its suffix even when nothing is added.
		"[project]\ndependencies = [\"a\" , \"b\" ]\n": "[project]\ndependencies = [\"a\" , \"b\"]\n",
	} {
		root := t.TempDir()
		writeFile(t, root, "pyproject.toml", authored)
		if err := mergePyprojectDeps(root, dependencies); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, filepath.Join(root, "pyproject.toml")); got != want {
			t.Fatalf("%q: got %q, want %q", authored, got, want)
		}
	}
}

func TestPythonVersionConfigReadsOptionalFieldsAndRejectsNonStrings(t *testing.T) {
	root := t.TempDir()
	config, err := readPythonVersionConfig(root)
	if err != nil || config.pythonVersion != nil || config.requiresPython != nil {
		t.Fatalf("got %+v, %v", config, err)
	}
	for _, source := range []string{"[project]\n", "[tool.mda]\nother = true\n"} {
		writeFile(t, root, "pyproject.toml", source)
		config, err := readPythonVersionConfig(root)
		if err != nil || config.pythonVersion != nil || config.requiresPython != nil {
			t.Fatalf("%q: got %+v, %v", source, config, err)
		}
	}
	writeFile(t, root, "pyproject.toml", "[project]\nrequires-python = \">=3.11\"\n[tool.mda]\npython-version = \"3.12\"\n")
	config, err = readPythonVersionConfig(root)
	if err != nil || *config.requiresPython != ">=3.11" || *config.pythonVersion != "3.12" {
		t.Fatalf("got %+v, %v", config, err)
	}
	path := filepath.Join(root, "pyproject.toml")
	for source, field := range map[string]string{
		"[project]\nrequires-python = 3.11\n": "[project] requires-python",
		"[tool.mda]\npython-version = 3.14\n": "[tool.mda] python-version",
	} {
		writeFile(t, root, "pyproject.toml", source)
		_, err := readPythonVersionConfig(root)
		if err == nil || err.Error() != field+" in "+path+" must be a quoted string" {
			t.Fatalf("%q: got %v", source, err)
		}
	}
	writeFile(t, root, "pyproject.toml", "[project\n")
	_, err = readPythonVersionConfig(root)
	requireErrorContains(t, err, "failed to parse "+path+": ")
}

func TestGenerateLockfileRerunsWhenALocalPackageManifestChanges(t *testing.T) {
	out := t.TempDir()
	writeFiles(t, out, map[string]string{
		"package.json":               `{"name":"x","workspaces":["__local__/dep"]}`,
		"__local__/dep/package.json": `{"name":"dep"}`,
	})
	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}
	writeFile(t, out, "package-lock.json", "sentinel")

	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "package-lock.json")); got != "sentinel" {
		t.Fatalf("an unchanged manifest regenerated the lock: %q", got)
	}

	writeFile(t, out, "__local__/dep/package.json", `{"name":"dep","dependencies":{"zod":"^3.0.0"}}`)
	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "package-lock.json")); got != stubLockfile {
		t.Fatalf("a changed local manifest did not regenerate the lock: %q", got)
	}
}

func TestGenerateLockfileSkipsWhenThePackageJSONMatchesTheMarker(t *testing.T) {
	out := t.TempDir()
	pkg := `{"name":"x","private":true,"dependencies":{}}`
	writeFiles(t, out, map[string]string{
		"package.json":      pkg,
		"package-lock.json": "{\"lockfileVersion\":3}\n",
		lockInputMarker:     pkg,
	})
	materializeLockfile = func(context.Context, string, []byte) error {
		t.Fatal("the lock was materialized again")
		return nil
	}
	t.Cleanup(func() { materializeLockfile = writeStubLockfile })

	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(out, "package-lock.json")); got != "{\"lockfileVersion\":3}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateLockfileRecordsTheManifestItWasGeneratedFrom(t *testing.T) {
	out := t.TempDir()
	pkg := `{"name":"x","private":true,"dependencies":{"langchain":"^1.0.0"}}`
	writeFile(t, out, "package.json", pkg)

	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}

	if readFile(t, filepath.Join(out, "package-lock.json")) != stubLockfile || readFile(t, filepath.Join(out, lockInputMarker)) != pkg {
		t.Fatal("the lock or its marker is wrong")
	}
}

// An older CLI let npm regenerate into a copied shrinkwrap and then recorded
// the marker, so a matching marker can vouch for a stale package lock.
func TestGenerateLockfileRegeneratesAfterRemovingAStaleShrinkwrap(t *testing.T) {
	out := t.TempDir()
	pkg := `{"name":"x","private":true,"dependencies":{}}`
	writeFiles(t, out, map[string]string{
		"package.json":        pkg,
		"package-lock.json":   "stale\n",
		lockInputMarker:       pkg,
		"npm-shrinkwrap.json": "{\"lockfileVersion\":3}\n",
	})

	if err := generateLockfile(t.Context(), out); err != nil {
		t.Fatal(err)
	}

	if isFile(filepath.Join(out, "npm-shrinkwrap.json")) {
		t.Fatal("the shrinkwrap survived")
	}
	if readFile(t, filepath.Join(out, "package-lock.json")) != stubLockfile || readFile(t, filepath.Join(out, lockInputMarker)) != pkg {
		t.Fatal("the lock was not regenerated")
	}
}

// Empty dependencies keep npm off the registry, so this live run is cheap.
func TestGenerateLockfileWritesThePackageLockWithLiveNpmAfterAShrinkwrap(t *testing.T) {
	withLiveNpmLockfile(t)
	const oldLock = `{"name":"old","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"old","version":"1.0.0"}}}`
	for _, staleLock := range []string{oldLock, ""} {
		out := t.TempDir()
		pkg := `{"name":"current","version":"2.0.0","private":true,"dependencies":{}}`
		writeFiles(t, out, map[string]string{
			"package.json":        pkg,
			"npm-shrinkwrap.json": oldLock,
			lockInputMarker:       pkg,
		})
		if staleLock != "" {
			writeFile(t, out, "package-lock.json", staleLock)
		}

		if err := generateLockfile(t.Context(), out); err != nil {
			t.Fatal(err)
		}

		if isFile(filepath.Join(out, "npm-shrinkwrap.json")) {
			t.Fatal("the shrinkwrap survived")
		}
		lock := readJSON(t, readFile(t, filepath.Join(out, "package-lock.json")))
		if lock["name"] != "current" || lock["version"] != "2.0.0" {
			t.Fatalf("stale lock %q: got %v", staleLock, lock)
		}
	}
}

func workspaces(t *testing.T, object *jsonObject) string {
	t.Helper()
	return prettyJSON(object.values["workspaces"])
}

func TestEnsureWorkspaceAddsAnArrayEntry(t *testing.T) {
	object := newJSONObject()
	ensureWorkspace(object, runtimeDir)
	if got := workspaces(t, object); got != "[\n  \"__runtime__\"\n]" {
		t.Fatalf("got %s", got)
	}
}

func TestEnsureWorkspaceMergesIntoAnExistingArray(t *testing.T) {
	object := jsonObjectOf("workspaces", []any{"packages/*"})
	ensureWorkspace(object, runtimeDir)
	ensureWorkspace(object, runtimeDir)
	if got := workspaces(t, object); got != "[\n  \"packages/*\",\n  \"__runtime__\"\n]" {
		t.Fatalf("got %s", got)
	}
	// A `./` spelling already lists the entry.
	object = jsonObjectOf("workspaces", []any{"./" + runtimeDir})
	ensureWorkspace(object, runtimeDir)
	if got := workspaces(t, object); got != "[\n  \"./__runtime__\"\n]" {
		t.Fatalf("got %s", got)
	}
}

func TestEnsureWorkspaceMergesIntoAPackagesObject(t *testing.T) {
	object := jsonObjectOf("workspaces", jsonObjectOf("packages", []any{"packages/*"}))
	ensureWorkspace(object, runtimeDir)
	if got := workspaces(t, object); got != "{\n  \"packages\": [\n    \"packages/*\",\n    \"__runtime__\"\n  ]\n}" {
		t.Fatalf("got %s", got)
	}
}

func TestLangsmithOverrideMirrorsTheDirectDependency(t *testing.T) {
	manifest := jsonObjectOf(
		"dependencies", jsonObjectOf("langsmith", "^0.8.9", "zod", "^4.0.0"),
		"overrides", jsonObjectOf("other", "1.0.0"),
	)
	if err := ensureLangsmithNpmOverride(manifest); err != nil {
		t.Fatal(err)
	}
	overrides, _ := manifest.object("overrides")
	if got := prettyJSON(overrides); got != "{\n  \"other\": \"1.0.0\",\n  \"langsmith\": \"^0.8.9\"\n}" {
		t.Fatalf("got %s", got)
	}
}

func TestLangsmithOverrideIsANoopWithoutTheDependency(t *testing.T) {
	manifest := jsonObjectOf("dependencies", jsonObjectOf("zod", "^4.0.0"))
	if err := ensureLangsmithNpmOverride(manifest); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest.object("overrides"); ok {
		t.Fatalf("got %s", prettyJSON(manifest))
	}
}

func TestLangsmithOverrideReplacesAStaleOverride(t *testing.T) {
	manifest := jsonObjectOf(
		"dependencies", jsonObjectOf("langsmith", "^0.8.9"),
		"overrides", jsonObjectOf("langsmith", "^0.7.16"),
	)
	if err := ensureLangsmithNpmOverride(manifest); err != nil {
		t.Fatal(err)
	}
	overrides, _ := manifest.object("overrides")
	if got := prettyJSON(overrides); got != "{\n  \"langsmith\": \"^0.8.9\"\n}" {
		t.Fatalf("got %s", got)
	}
}

func TestRequirementNamesExtrasAndMarkers(t *testing.T) {
	for requirement, want := range map[string]struct {
		canonical string
		extras    []string
		marker    string
	}{
		"Langchain.OpenAI>=1":                       {"langchain-openai", nil, ""},
		"  my_dep [Fast, all] ; python_version>'3'": {"my-dep", []string{"fast", "all"}, "python_version>'3'"},
		"pkg[x]": {"pkg", []string{"x"}, ""},
		"pkg;":   {"pkg", nil, ""},
	} {
		marker, _ := requirementMarker(requirement)
		if got := canonicalRequirementName(requirement); got != want.canonical {
			t.Fatalf("%q: name %q", requirement, got)
		}
		if got := requirementExtras(requirement); !reflect.DeepEqual(got, want.extras) {
			t.Fatalf("%q: extras %q", requirement, got)
		}
		if marker != want.marker {
			t.Fatalf("%q: marker %q", requirement, marker)
		}
	}
}

func TestStripPackageJSONDevDependenciesLeavesAManifestWithoutThemAlone(t *testing.T) {
	out := t.TempDir()
	writeFile(t, out, "package.json", `{"name":"x"}`)

	if err := stripPackageJSONDevDependencies(out); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(out, "package.json")); got != `{"name":"x"}` {
		t.Fatalf("got %q", got)
	}
	if err := os.Remove(filepath.Join(out, "package.json")); err != nil {
		t.Fatal(err)
	}
	if err := stripPackageJSONDevDependencies(out); err != nil {
		t.Fatal(err)
	}
}
