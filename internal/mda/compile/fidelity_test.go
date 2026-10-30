package compile

import (
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The generated build must match what the mda CLI writes for the same project
// byte for byte. `testdata/fidelity/<name>` are fixture projects and
// `testdata/fidelity/<name>.golden` the local build (`mda build`) the Rust CLI
// wrote for each, with the project root replaced by `$ROOT`. Regenerate them
// with a reference binary built from the Managed Deep Agents SDK:
//
//	go test ./internal/mda/compile -run TestCompileMatchesTheMdaCLI -mda-binary=/path/to/mda
//
// `package-lock.json` is left out: the Rust CLI resolves it from the npm
// registry, and these tests stub it.
//
// One difference is intended ([departFromTheMdaCLI]): the vendored
// `__runtime__/package.json` lists the installed package's dependencies, which
// the Rust CLI, shipping in lockstep with its runtime, left to the managed
// stack it injects at the root.

var mdaBinary = flag.String("mda-binary", "", "regenerate the fidelity goldens by running this mda CLI binary")

const fidelityDir = "testdata/fidelity"

// unmatchedArtifacts are the build files the goldens do not record.
var unmatchedArtifacts = []string{"package-lock.json"}

func TestCompileMatchesTheMdaCLI(t *testing.T) {
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
			if *mdaBinary != "" {
				reference := fidelityFixture(t, name)
				runReferenceBuild(t, reference)
				writeGolden(t, golden, normalizedBuild(t, reference))
			}
			f := fidelityFixture(t, name)
			mustCompile(t, f.local)
			got := normalizedBuild(t, f)
			want := normalizedBuild(t, fixture{outDir: golden, project: f.project})
			departFromTheMdaCLI(t, want, f.installation)
			for _, rel := range sortedKeys(want) {
				if got[rel] != want[rel] {
					at := firstDifference(got[rel], want[rel])
					t.Errorf("%s differs from the mda CLI's build at byte %d (mda %q, langsmith %q):\n--- mda\n%s\n--- langsmith\n%s",
						rel, at, excerpt(want[rel], at), excerpt(got[rel], at), want[rel], got[rel])
				}
			}
			for _, rel := range sortedKeys(got) {
				if _, ok := want[rel]; !ok {
					t.Errorf("%s is not in the mda CLI's build", rel)
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

// departFromTheMdaCLI makes the intended differences from the mda CLI's build
// in want: the vendored npm runtime declares the installed package's
// dependencies.
func departFromTheMdaCLI(t *testing.T, want map[string]string, installation stubInstallation) {
	t.Helper()
	const rel = "__runtime__/package.json"
	contents, ok := want[rel]
	if !ok {
		return
	}
	manifest, _, err := parseJSONObject([]byte(contents))
	if err != nil {
		t.Fatal(err)
	}
	dependencies := newJSONObject()
	for _, dep := range installation.manifest.Dependencies {
		dependencies.insert(dep.Name, dep.Version)
	}
	manifest.insert("dependencies", dependencies)
	want[rel] = prettyJSON(manifest) + "\n"
}

// runReferenceBuild builds f with the mda CLI at mdaBinary.
func runReferenceBuild(t *testing.T, f fixture) {
	t.Helper()
	cmd := exec.Command(*mdaBinary, "build", f.root())
	cmd.Env = append(os.Environ(),
		"MDA_INSTALL_CHANNEL="+f.project.Language.Ident("npm", "pypi"),
		"MDA_BUNDLED_RUNTIME_PATH="+f.installation.RuntimeSource(),
		"MDA_RUNTIME_VERSION="+stubPackageVersion,
		"MDA_TELEMETRY_DISABLED=1",
		"DO_NOT_TRACK=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mda build failed: %v\n%s", err, output)
	}
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
