package compile

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Compile tests build fixture projects end to end. Their authored modules
// import the stub `managed-deepagents` SDKs introspect's tests use
// (`../introspect/testdata`), so introspection runs the real helpers against
// them. The TypeScript helper is installed once into the CLI's cache, as it is
// for `langsmith deploy`.

// stubPackageVersion is the version the stub runtime installations report.
const stubPackageVersion = "0.0.0-stub"

// stubLockfile is the placeholder `package-lock.json` compile tests write
// instead of calling the npm registry. It deliberately omits registry
// packages so assertions on resolved dependencies fail closed.
const stubLockfile = "{\n  \"name\": \"mda-test-stub\",\n  \"lockfileVersion\": 3,\n  \"requires\": true,\n  \"packages\": {\n    \"\": {\n      \"name\": \"mda-test-stub\"\n    }\n  }\n}\n"

func TestMain(m *testing.M) {
	materializeLockfile = writeStubLockfile
	code := m.Run()
	if err := os.RemoveAll(venvDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func writeStubLockfile(_ context.Context, outDir string, pkg []byte) error {
	if err := os.WriteFile(filepath.Join(outDir, "package-lock.json"), []byte(stubLockfile), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, lockInputMarker), pkg, 0o644)
}

// withLiveNpmLockfile runs `npm install --package-lock-only` for real for the
// rest of the test.
func withLiveNpmLockfile(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("npm"); err != nil {
		t.Fatalf("npm is required for live lockfile tests: %v", err)
	}
	materializeLockfile = materializeNpmLockfile
	t.Cleanup(func() { materializeLockfile = writeStubLockfile })
}

// stubInstallation is an installed `managed-deepagents` with a mock runtime.
type stubInstallation struct {
	runtimeSource string
	manifest      project.RuntimeManifest
}

func (i stubInstallation) RuntimeSource() string                    { return i.runtimeSource }
func (i stubInstallation) PackageVersion() string                   { return stubPackageVersion }
func (i stubInstallation) RuntimeManifest() project.RuntimeManifest { return i.manifest }

// stubTypescriptManifest and stubPythonManifest are what the stub
// installations' manifests declare: the ranges the mda CLI wrote from its
// constants, so a build against an SDK that agrees with the CLI matches the
// fidelity goldens.
var (
	stubTypescriptManifest = project.RuntimeManifest{
		Dependencies: []project.Dependency{
			{Name: "@langchain/core", Version: "^1.2.17"},
			{Name: "@langchain/langgraph", Version: "^1.4.18"},
			{Name: "@langchain/langgraph-sdk", Version: "^1.12.0"},
			{Name: "@langchain/mcp-adapters", Version: "^2.0.0"},
			{Name: "deepagents", Version: "^1.15.1"},
			{Name: "hono", Version: "^4.12.27"},
			{Name: "jose", Version: "^6.2.3"},
			{Name: "langchain", Version: "^1.5.12"},
			{Name: "langsmith", Version: "^0.8.9"},
			{Name: "uuid", Version: "^13.0.2"},
			{Name: "zod", Version: "^4.4.3"},
		},
		Main: "./index.js",
		Exports: []project.Export{
			{Subpath: ".", Target: "./index.js"},
			{Subpath: "./runtime", Target: "./runtime/index.js"},
		},
	}
	stubPythonManifest = project.RuntimeManifest{
		Dependencies: []project.Dependency{
			{Name: "pydantic", Version: ">=2.7.4,<2.15"},
			{Name: "httpx", Version: ">=0.28.1,<1"},
			{Name: "typing-extensions", Version: ">=4.12.2,<5"},
			{Name: "deepagents", Version: ">=0.7.30,<1"},
			{Name: "langchain", Version: ">=1.4.6,<2"},
			{Name: "langchain-core", Version: ">=1.6.2,<2"},
			{Name: "langgraph", Version: ">=1.2.15,<2"},
			{Name: "langgraph-prebuilt", Version: ">=1.1.1,<2"},
			{Name: "langgraph-sdk", Version: ">=0.4.6,<0.5"},
		},
		RequiresPython: ">=3.11",
	}
)

// fixture is a fixture project, its installation, and its default build
// directory.
type fixture struct {
	project      *project.Project
	installation stubInstallation
	outDir       string
}

func (f fixture) root() string {
	return f.project.Root
}

// reload reloads the project after the test changed its files.
func (f *fixture) reload(t *testing.T) {
	t.Helper()
	f.project = loadProject(t, f.project.Root)
}

func (f fixture) deploy(t *testing.T) (*CompileSummary, error) {
	t.Helper()
	return CompileForDeploy(t.Context(), f.project, f.outDir, DeployContext{Repo: "context-repo"}, f.installation)
}

func (f fixture) local(t *testing.T) (*CompileSummary, error) {
	t.Helper()
	return Compile(t.Context(), f.project, f.outDir, f.installation)
}

func mustCompile(t *testing.T, compile func(*testing.T) (*CompileSummary, error)) *CompileSummary {
	t.Helper()
	summary, err := compile(t)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}

func (f fixture) read(t *testing.T, rel string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(f.outDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func (f fixture) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(f.outDir, filepath.FromSlash(rel)))
	return err == nil
}

// typescriptProject writes a TypeScript project with files (path → contents),
// the stub `managed-deepagents` package installed with a mock runtime under
// its `dist/`, and loads it.
func typescriptProject(t *testing.T, files map[string]string) fixture {
	t.Helper()
	return typescriptProjectAt(t, canonicalTempDir(t), files)
}

// typescriptProjectAt is [typescriptProject] rooted at root.
func typescriptProjectAt(t *testing.T, root string, files map[string]string) fixture {
	t.Helper()
	if _, ok := files["package.json"]; !ok {
		files["package.json"] = `{"name": "fixture", "type": "module"}` + "\n"
	}
	writeFiles(t, root, files)
	return fixture{
		installation: installTypescriptStub(t, root),
		project:      loadProject(t, root),
		outDir:       filepath.Join(root, ".mda", "build"),
	}
}

// installTypescriptStub installs the stub `managed-deepagents` package into
// root's `node_modules`, with a mock runtime under its `dist/`.
func installTypescriptStub(t *testing.T, root string) stubInstallation {
	t.Helper()
	pkg := filepath.Join(root, "node_modules", "managed-deepagents")
	copyDir(t, filepath.Join("..", "introspect", "testdata", "npm", "managed-deepagents"), pkg)
	writeFiles(t, pkg, map[string]string{
		"dist/index.js":         "// mock runtime\n",
		"dist/runtime/index.js": "// mock runtime entry\n",
	})
	return stubInstallation{runtimeSource: filepath.Join(pkg, "dist"), manifest: stubTypescriptManifest}
}

// pythonProject writes a Python project with files and loads it. Its
// interpreter is [stubVenv]'s; the mock runtime lives outside the project.
func pythonProject(t *testing.T, files map[string]string) fixture {
	t.Helper()
	return pythonProjectAt(t, canonicalTempDir(t), files)
}

// pythonProjectAt is [pythonProject] rooted at root.
func pythonProjectAt(t *testing.T, root string, files map[string]string) fixture {
	t.Helper()
	writeFiles(t, root, files)
	t.Setenv("VIRTUAL_ENV", stubVenv(t))
	runtime := t.TempDir()
	writeFiles(t, runtime, map[string]string{
		"__init__.py":    "# mock\n",
		"runtime.py":     "# mock runtime\n",
		"_connectors.py": "# mock connectors\n",
		"schedules.py":   "# mock schedules\n",
	})
	return fixture{
		project:      loadProject(t, root),
		installation: stubInstallation{runtimeSource: runtime, manifest: stubPythonManifest},
		outDir:       filepath.Join(root, ".mda", "build"),
	}
}

var (
	venvOnce sync.Once
	venvDir  string
	venvErr  error
)

// stubVenv is a virtualenv over the `python3` on PATH whose site-packages
// point at the stub `managed_deepagents` package introspect's tests use, with a
// `.pth` file, which the helper's isolated (`-I`) interpreter still reads.
func stubVenv(t *testing.T) string {
	t.Helper()
	venvOnce.Do(func() {
		venvDir, venvErr = createStubVenv(filepath.Join("..", "introspect", "testdata", "python"))
	})
	if venvErr != nil {
		t.Fatal(venvErr)
	}
	return venvDir
}

// createStubVenv creates a virtualenv in a fresh temporary directory whose
// site-packages put packages on `sys.path`.
func createStubVenv(packages string) (string, error) {
	packages, err := filepath.Abs(packages)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "mda-compile-venv-*")
	if err != nil {
		return "", err
	}
	output, err := exec.Command(proc.Python3Program(), "-m", "venv", "--without-pip", dir).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("python3 -m venv failed (python3 is required for the Python compile tests): %w\n%s", err, output)
	}
	sitePackages, err := filepath.Glob(filepath.Join(dir, "lib", "python3*", "site-packages"))
	if err != nil {
		return "", err
	}
	windows, err := filepath.Glob(filepath.Join(dir, "Lib", "site-packages"))
	if err != nil {
		return "", err
	}
	sitePackages = append(sitePackages, windows...)
	if len(sitePackages) != 1 {
		return "", fmt.Errorf("expected one site-packages in %s, found %v", dir, sitePackages)
	}
	return dir, os.WriteFile(filepath.Join(sitePackages[0], "managed_deepagents_stub.pth"), []byte(packages+"\n"), 0o644)
}

// canonicalTempDir is a temporary directory with its symlinks resolved, as
// project roots are.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func loadProject(t *testing.T, root string) *project.Project {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, contents := range files {
		writeFile(t, root, rel, contents)
	}
}

func writeFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// requireErrorContains fails unless err's message contains want.
func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected an error containing %q, got %q", want, err.Error())
	}
}

// requireContains fails unless text contains every want.
func requireContains(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in:\n%s", want, text)
		}
	}
}

// requireNotContains fails if text contains any of unwanted.
func requireNotContains(t *testing.T, text string, unwanted ...string) {
	t.Helper()
	for _, want := range unwanted {
		if strings.Contains(text, want) {
			t.Fatalf("did not expect %q in:\n%s", want, text)
		}
	}
}

// Authored module snippets.
const (
	tsImport      = "import { defineAgent, defineDeepAgent } from \"managed-deepagents\";\n"
	pyImport      = "from managed_deepagents import define_agent, define_deep_agent\n"
	tsAgent       = tsImport + "export const agent = defineDeepAgent({ name: \"agent\", model: \"openai:gpt-5\" });\n"
	pyAgent       = pyImport + "agent = define_deep_agent(name=\"agent\", model=\"openai:gpt-5\")\n"
	tsIdentity    = "import { defineIdentity } from \"managed-deepagents\";\nexport const identity = defineIdentity({ auth: \"backend\" });\n"
	pyIdentity    = "from managed_deepagents import define_identity\nidentity = define_identity(auth='backend')\n"
	instructionMD = "You are a helpful assistant.\n"
)
