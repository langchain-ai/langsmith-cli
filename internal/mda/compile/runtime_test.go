package compile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

func TestVendorPyRuntimeDeclaresTheInstalledRequirements(t *testing.T) {
	source := t.TempDir()
	writeFile(t, source, "__init__.py", "")
	out := t.TempDir()
	manifest := project.RuntimeManifest{
		Dependencies: []project.Dependency{
			{Name: "pydantic", Version: "<2.15,>=2.7.4"},
			{Name: "langgraph-sdk", Version: "<0.6,>=0.5"},
			{Name: "tomli", Version: `>=2; python_version < "3.12"`},
		},
		RequiresPython: ">=3.12",
	}

	if err := vendorPyRuntime(source, out, "1.2.3", manifest); err != nil {
		t.Fatal(err)
	}

	want := "[project]\nname = \"managed-deepagents\"\nversion = \"1.2.3\"\nrequires-python = \">=3.12\"\n" +
		"dependencies = [\n\"pydantic<2.15,>=2.7.4\",\n\"langgraph-sdk<0.6,>=0.5\",\n" +
		"\"tomli>=2; python_version < \\\"3.12\\\"\",\n]\n\n" +
		"[build-system]\nrequires = [\"setuptools>=61\"]\nbuild-backend = \"setuptools.build_meta\"\n"
	if got := readFile(t, filepath.Join(out, "pyproject.toml")); got != want {
		t.Fatalf("pyproject.toml:\n%s", got)
	}
}

func TestVendorTsRuntimeCarriesTheInstalledEntryPointsAndDependencies(t *testing.T) {
	source := t.TempDir()
	writeFiles(t, source, map[string]string{"index.js": "export {};\n", "runtime/index.js": "export {};\n"})
	out := t.TempDir()
	manifest := project.RuntimeManifest{
		Dependencies: []project.Dependency{
			{Name: "langsmith", Version: "^0.10.2"},
			{Name: "@standard-schema/spec", Version: "^1.1.0"},
		},
		Main:    "./index.js",
		Exports: []project.Export{{Subpath: ".", Target: "./index.js"}, {Subpath: "./runtime", Target: "./runtime/index.js"}},
	}

	if err := vendorTsRuntime(source, out, "1.2.3", manifest); err != nil {
		t.Fatal(err)
	}

	want := "{\n  \"name\": \"managed-deepagents\",\n  \"version\": \"1.2.3\",\n  \"type\": \"module\",\n" +
		"  \"main\": \"./index.js\",\n  \"exports\": {\n    \".\": \"./index.js\",\n" +
		"    \"./runtime\": \"./runtime/index.js\"\n  },\n  \"dependencies\": {\n" +
		"    \"langsmith\": \"^0.10.2\",\n    \"@standard-schema/spec\": \"^1.1.0\"\n  }\n}\n"
	if got := readFile(t, filepath.Join(out, "package.json")); got != want {
		t.Fatalf("package.json:\n%s", got)
	}
	if !isFile(filepath.Join(out, "runtime", "index.js")) {
		t.Fatal("the runtime was not copied")
	}
}

func TestCopyDirRecursiveNamesAnUnreadableSource(t *testing.T) {
	out := t.TempDir()
	missing := filepath.Join(out, "missing")

	err := copyDirRecursive(missing, out)

	if err == nil || !strings.HasPrefix(err.Error(), "failed to read "+missing+": ") {
		t.Fatalf("got %v", err)
	}
}

func TestReVendoringLeavesUnchangedRuntimeFilesUntouched(t *testing.T) {
	pkg := filepath.Join(t.TempDir(), "managed_deepagents")
	writeFiles(t, pkg, map[string]string{"__init__.py": "# pkg\n", "connectors/__init__.py": "# connectors\n"})
	out := t.TempDir()
	if err := vendorPyRuntime(pkg, out, "1.2.3", project.RuntimeManifest{}); err != nil {
		t.Fatal(err)
	}
	vendored := filepath.Join(out, "managed_deepagents", "connectors", "__init__.py")
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(vendored, past, past); err != nil {
		t.Fatal(err)
	}
	modified := func() time.Time {
		info, err := os.Stat(vendored)
		if err != nil {
			t.Fatal(err)
		}
		return info.ModTime()
	}

	// A rebuild re-vendors the tree; identical files keep their mtime.
	if err := vendorPyRuntime(pkg, out, "1.2.3", project.RuntimeManifest{}); err != nil {
		t.Fatal(err)
	}
	if !modified().Equal(past) {
		t.Fatal("an unchanged file was rewritten")
	}

	writeFile(t, pkg, "connectors/__init__.py", "# changed\n")
	if err := vendorPyRuntime(pkg, out, "1.2.3", project.RuntimeManifest{}); err != nil {
		t.Fatal(err)
	}
	if modified().Equal(past) {
		t.Fatal("a changed file was not rewritten")
	}
}

func TestVendorPyRuntimeSkipsCachesBytecodeAndBinaries(t *testing.T) {
	pkg := filepath.Join(t.TempDir(), "managed_deepagents")
	writeFiles(t, pkg, map[string]string{
		"__init__.py":                     "# pkg\n",
		"py.typed":                        "",
		"connectors/__init__.py":          "# connectors\n",
		"__pycache__/mod.cpython-311.pyc": "cache",
		"connectors/stale.pyc":            "cache",
		"_binaries/mda":                   "binary",
	})
	out := t.TempDir()

	if err := vendorPyRuntime(pkg, out, "1.2.3", project.RuntimeManifest{}); err != nil {
		t.Fatal(err)
	}

	vendored := filepath.Join(out, "managed_deepagents")
	if !isFile(filepath.Join(vendored, "connectors", "__init__.py")) || !isFile(filepath.Join(vendored, "py.typed")) {
		t.Fatal("the package was not copied")
	}
	for _, skipped := range []string{"__pycache__", filepath.Join("connectors", "stale.pyc"), "_binaries"} {
		if _, err := os.Stat(filepath.Join(vendored, skipped)); !os.IsNotExist(err) {
			t.Fatalf("%s was vendored", skipped)
		}
	}
}
