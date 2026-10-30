package compile

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The generated build must not change unnoticed. `testdata/fidelity/<name>`
// are fixture projects and `testdata/fidelity/<name>.golden` their deploy
// build ([CompileForDeploy]), with the project root replaced by `$ROOT`.
// Regenerate them with
//
//	go test ./internal/mda/compile -run TestCompileMatchesTheGoldens -update
//
// Until commit db46170 the goldens were the Rust CLI's local build
// (`mda build`), which the Go port of that build matched byte for byte. Only
// the deploy build is ported now, and no Rust command writes one offline, so
// the goldens are the Go deploy build. It differs from those local builds
// only where deploy does: the entry reads the Context Hub repo deploy names
// instead of a local context root; `langgraph.json` keeps Studio auth and, for
// Python, installs the vendored runtime and local packages itself, so the
// manifests drop those requirements instead of pointing at `file://` and
// editable copies; no dev server or devDependencies are installed; and the
// vendored `__runtime__/package.json` lists the installed package's
// dependencies, which the Rust CLI, shipping in lockstep with its runtime,
// left to the managed stack it injects at the root.
//
// `package-lock.json` is left out: it comes from the npm registry, and these
// tests stub it.

var update = flag.Bool("update", false, "rewrite the fidelity goldens from the deploy build")

const fidelityDir = "testdata/fidelity"

// unmatchedArtifacts are the build files the goldens do not record.
var unmatchedArtifacts = []string{"package-lock.json"}

func TestCompileMatchesTheGoldens(t *testing.T) {
	entries, err := os.ReadDir(fidelityDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasSuffix(entry.Name(), ".golden") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			golden := filepath.Join(fidelityDir, name+".golden")
			f := fidelityFixture(t, name)
			mustCompile(t, f.deploy)
			got := normalizedBuild(t, f)
			if *update {
				writeGolden(t, golden, got)
			}
			want := normalizedBuild(t, fixture{outDir: golden, project: f.project})
			for _, rel := range sortedKeys(want) {
				if got[rel] != want[rel] {
					at := firstDifference(got[rel], want[rel])
					t.Errorf("%s differs from the golden at byte %d (golden %q, build %q):\n--- golden\n%s\n--- build\n%s",
						rel, at, excerpt(want[rel], at), excerpt(got[rel], at), want[rel], got[rel])
				}
			}
			for _, rel := range sortedKeys(got) {
				if _, ok := want[rel]; !ok {
					t.Errorf("%s is not in the golden", rel)
				}
			}
		})
	}
}

// fidelityFixture copies the fixture name into a fresh directory. A fixture
// with a `project/` directory is rooted there, its other directories sitting
// next to it as local packages; any other fixture is the project itself.
func fidelityFixture(t *testing.T, name string) fixture {
	t.Helper()
	source := filepath.Join(fidelityDir, name)
	dir := canonicalTempDir(t)
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		writeFile(t, dir, filepath.ToSlash(rel), readFile(t, path))
		// Executable fixtures stay executable.
		return os.Chmod(filepath.Join(dir, rel), info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	root := dir
	if isDir(filepath.Join(dir, "project")) {
		root = filepath.Join(dir, "project")
	}
	if isFile(filepath.Join(root, "agent.py")) {
		return pythonProjectAt(t, root, map[string]string{})
	}
	return typescriptProjectAt(t, root, map[string]string{"package.json": readFile(t, filepath.Join(root, "package.json"))})
}

// normalizedBuild reads every file of f's build, with f's root replaced by
// `$ROOT`.
func normalizedBuild(t *testing.T, f fixture) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(f.outDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(f.outDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if slices.Contains(unmatchedArtifacts, rel) {
			return nil
		}
		files[rel] = strings.ReplaceAll(readFile(t, path), f.root(), "$ROOT")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func writeGolden(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	for rel, contents := range files {
		writeFile(t, dir, rel, contents)
	}
}

func firstDifference(a, b string) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

func excerpt(text string, at int) string {
	return text[max(at-20, 0):min(at+20, len(text))]
}
