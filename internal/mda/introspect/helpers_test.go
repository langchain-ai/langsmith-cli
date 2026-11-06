package introspect

import (
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

// TestMain installs the TypeScript helper once per test run into a private
// cache, so tests never touch the user's.
func TestMain(m *testing.M) {
	cache, err := os.MkdirTemp("", "mda-helper-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	helperCacheRoot = func() (string, error) { return cache, nil }
	code := m.Run()
	for _, dir := range []string{cache, venvDir} {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	os.Exit(code)
}

// typescriptProject writes a TypeScript project with files (path → contents)
// and the stub `managed-deepagents` package installed, and loads it.
func typescriptProject(t *testing.T, files map[string]string) *project.Project {
	t.Helper()
	root := t.TempDir()
	if _, ok := files["package.json"]; !ok {
		files["package.json"] = `{"name": "fixture", "type": "module"}` + "\n"
	}
	writeFiles(t, root, files)
	copyDir(t, filepath.Join("testdata", "npm", "managed-deepagents"),
		filepath.Join(root, "node_modules", "managed-deepagents"))
	return loadProject(t, root)
}

// pythonProject writes a Python project with files and loads it. Its
// interpreter is [stubVenv]'s.
func pythonProject(t *testing.T, files map[string]string) *project.Project {
	t.Helper()
	return pythonProjectIn(t, t.TempDir(), files)
}

// pythonProjectIn is [pythonProject] rooted at root.
func pythonProjectIn(t *testing.T, root string, files map[string]string) *project.Project {
	t.Helper()
	writeFiles(t, root, files)
	t.Setenv("VIRTUAL_ENV", stubVenv(t))
	return loadProject(t, root)
}

var (
	venvOnce sync.Once
	venvDir  string
	venvErr  error
)

// stubVenv is a virtualenv over the `python3` on PATH whose site-packages
// point at the stub `managed_deepagents` package (`testdata/python`) with a
// `.pth` file, which the helper's isolated (`-I`) interpreter still reads.
func stubVenv(t *testing.T) string {
	t.Helper()
	venvOnce.Do(func() {
		venvDir, venvErr = createStubVenv(filepath.Join("testdata", "python"))
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
	dir, err := os.MkdirTemp("", "mda-stub-venv-*")
	if err != nil {
		return "", err
	}
	output, err := exec.Command(proc.Python3Program(), "-m", "venv", "--without-pip", dir).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("python3 -m venv failed (python3 is required for the Python introspection tests): %w\n%s", err, output)
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
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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

// requireErrorPrefix fails unless err's message starts with want.
func requireErrorPrefix(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error starting with %q, got nil", want)
	}
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("expected an error starting with %q, got %q", want, err.Error())
	}
}
