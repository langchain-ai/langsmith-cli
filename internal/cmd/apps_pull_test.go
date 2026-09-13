package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tarGz builds a .tar.gz from path → content.
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func tarGzWithSymlink(t *testing.T, target string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := "keep me"
	if err := tw.WriteHeader(&tar.Header{Name: "src/App.tsx", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatalf("tar write: %v", err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "link.txt", Linkname: target, Mode: 0o777, Typeflag: tar.TypeSymlink}); err != nil {
		t.Fatalf("tar symlink header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// appsSourceServer serves the list and source endpoints.
func appsSourceServer(t *testing.T, apps []customApp, source map[string][]byte) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/platform/custom-apps", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apps)
	})
	mux.HandleFunc("/v1/platform/custom-apps/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/platform/custom-apps/"), "/source")
		archive, ok := source[id]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "no source archive stored for this custom app"})
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive)
	})
	return mux
}

const testAppID = "6f1c9b0e-6b3e-4a0e-9a4a-2c1d3e4f5a6b"

func TestAppsPull_ResolvesNameToIDAndExtracts(t *testing.T) {
	var sourcePath string
	archive := tarGz(t, map[string]string{
		"package.json":     `{"name":"my-app"}`,
		"src/App.tsx":      "export default function App() {}",
		"src/lib/utils.ts": "export const cn = () => {}",
	})
	mux := appsSourceServer(t,
		[]customApp{{ID: testAppID, Name: "My App"}},
		map[string][]byte{testAppID: archive},
	)
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/source") {
			sourcePath = r.URL.Path
		}
		mux.ServeHTTP(w, r)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", "My App"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if sourcePath != "/v1/platform/custom-apps/"+testAppID+"/source" {
		t.Errorf("expected the name to resolve to an ID before fetching source, got %q", sourcePath)
	}

	target := filepath.Join(dir, "my-app")
	for rel, want := range map[string]string{
		"package.json":     `{"name":"my-app"}`,
		"src/App.tsx":      "export default function App() {}",
		"src/lib/utils.ts": "export const cn = () => {}",
	} {
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("expected %s extracted: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s content mismatch: got %q", rel, got)
		}
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.AppID != testAppID || link.Name != "My App" {
		t.Errorf("expected the pulled directory linked to the app, got %+v", link)
	}
	if !strings.Contains(out, `"status": "pulled"`) {
		t.Errorf("expected pulled status in output:\n%s", out)
	}
}

func TestAppsPull_AcceptsAppID(t *testing.T) {
	archive := tarGz(t, map[string]string{"src/App.tsx": "app"})
	mux := appsSourceServer(t,
		[]customApp{{ID: testAppID, Name: "my-app"}},
		map[string][]byte{testAppID: archive},
	)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", testAppID})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(dir, "my-app", "src", "App.tsx")); err != nil {
		t.Errorf("expected the source extracted under the app's name: %v", err)
	}
}

func TestAppsPull_ErrorsOnUnknownName(t *testing.T) {
	mux := appsSourceServer(t, []customApp{{ID: testAppID, Name: "my-app"}}, nil)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()
	t.Chdir(t.TempDir())

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"pull", "nope"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no custom app named") {
		t.Errorf("expected an actionable not-found error, got %v", err)
	}
}

func TestAppsPull_SurfacesNoSourceArchive404(t *testing.T) {
	mux := appsSourceServer(t, []customApp{{ID: testAppID, Name: "my-app"}}, nil)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"pull", "my-app"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when no source archive is stored")
	}
	if !strings.Contains(err.Error(), "no stored source") || !strings.Contains(err.Error(), "push") {
		t.Errorf("expected a re-push hint, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "my-app")); !os.IsNotExist(statErr) {
		t.Error("expected no directory created when the fetch failed")
	}
}

func TestAppsPull_RefusesNonEmptyDirWithoutForce(t *testing.T) {
	archive := tarGz(t, map[string]string{"src/App.tsx": "fresh"})
	mux := appsSourceServer(t,
		[]customApp{{ID: testAppID, Name: "my-app"}},
		map[string][]byte{testAppID: archive},
	)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)
	seedFile(t, dir, "my-app/existing.txt", "mine")

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"pull", "my-app"})
	cmd.SetIn(strings.NewReader("n\n"))
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected the non-empty target to be refused, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "my-app", "existing.txt")); statErr != nil {
		t.Errorf("expected the existing file left alone after aborting: %v", statErr)
	}

	// --force replaces the directory outright.
	captureStdout(t, func() {
		cmd = newAppsCmd()
		cmd.SetArgs([]string{"pull", "my-app", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute with --force: %v", err)
		}
	})
	if _, statErr := os.Stat(filepath.Join(dir, "my-app", "existing.txt")); !os.IsNotExist(statErr) {
		t.Error("expected --force to replace, not merge into, the target directory")
	}
	got, err := os.ReadFile(filepath.Join(dir, "my-app", "src", "App.tsx"))
	if err != nil || string(got) != "fresh" {
		t.Errorf("expected the pulled source after --force, got %q (%v)", got, err)
	}
}

func TestAppsPull_ConfirmedNonEmptyDirProceeds(t *testing.T) {
	archive := tarGz(t, map[string]string{"src/App.tsx": "fresh"})
	mux := appsSourceServer(t,
		[]customApp{{ID: testAppID, Name: "my-app"}},
		map[string][]byte{testAppID: archive},
	)
	srv := newTestServer(t, mux.ServeHTTP)
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	t.Chdir(dir)
	seedFile(t, dir, "my-app/existing.txt", "mine")

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"pull", "my-app"})
		cmd.SetIn(strings.NewReader("y\n"))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, "my-app", "src", "App.tsx")); err != nil {
		t.Errorf("expected the pull to proceed after confirmation: %v", err)
	}
}

func TestExtractSourceArchive_RejectsPathTraversal(t *testing.T) {
	for _, name := range []string{
		"../escaped.txt",
		"src/../../escaped.txt",
		"/etc/passwd",
		"nested/../../../escaped.txt",
	} {
		dir := t.TempDir()
		dest := filepath.Join(dir, "app")
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatalf("mkdir dest: %v", err)
		}
		archive := tarGz(t, map[string]string{name: "pwned", "src/App.tsx": "app"})

		if _, err := extractSourceArchive(dest, archive); err == nil {
			t.Errorf("expected entry %q to be rejected", name)
		} else if !strings.Contains(err.Error(), "refusing to extract") {
			t.Errorf("expected a refusal for %q, got: %v", name, err)
		}
		// Rejection happens before anything is written.
		if _, err := os.Stat(filepath.Join(dest, "src", "App.tsx")); !os.IsNotExist(err) {
			t.Errorf("expected nothing extracted from an archive containing %q", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); !os.IsNotExist(err) {
			t.Errorf("entry %q escaped the target directory", name)
		}
	}
}

func TestExtractSourceArchive_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "app")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	written, err := extractSourceArchive(dest, tarGzWithSymlink(t, secret))
	if err != nil {
		t.Fatalf("extractSourceArchive: %v", err)
	}
	if len(written) != 1 || written[0] != "src/App.tsx" {
		t.Errorf("expected only the regular file extracted, got %v", written)
	}
	if _, err := os.Lstat(filepath.Join(dest, "link.txt")); !os.IsNotExist(err) {
		t.Error("expected symlink entries to be skipped")
	}
}

func TestExtractSourceArchive_RejectsNonGzipBody(t *testing.T) {
	dest := t.TempDir()
	if _, err := extractSourceArchive(dest, []byte("not a gzip archive")); err == nil {
		t.Fatal("expected an error for a non-gzip body")
	}
}

// Round-trips what push builds through what pull extracts.
func TestSourceArchive_PushBuildToPullExtractRoundTrip(t *testing.T) {
	src := t.TempDir()
	seedFile(t, src, "package.json", `{"name":"my-app"}`)
	seedFile(t, src, "src/App.tsx", "export default function App() {}")
	seedFile(t, src, "src/components/DataGrid.tsx", "grid")
	seedFile(t, src, "node_modules/react/index.js", "dep")
	seedFile(t, src, ".env", "SECRET=1")

	encoded, err := buildSourceArchive(src)
	if err != nil {
		t.Fatalf("buildSourceArchive: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "pulled")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	written, err := extractSourceArchive(dest, raw)
	if err != nil {
		t.Fatalf("extractSourceArchive: %v", err)
	}

	want := []string{"package.json", "src/App.tsx", "src/components/DataGrid.tsx"}
	if strings.Join(written, ",") != strings.Join(want, ",") {
		t.Errorf("round trip changed the file set: got %v, want %v", written, want)
	}
	got, err := os.ReadFile(filepath.Join(dest, "src", "components", "DataGrid.tsx"))
	if err != nil || string(got) != "grid" {
		t.Errorf("nested file did not survive the round trip: %q (%v)", got, err)
	}
}
