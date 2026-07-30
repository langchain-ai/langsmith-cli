package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedAppDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "bundle.js"), []byte("module.exports = { render: function() {} }"), 0o644); err != nil {
		t.Fatalf("seed bundle.js: %v", err)
	}
}

func TestAppsPush_CreatesAndWritesLink(t *testing.T) {
	var sawPost bool
	var postBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			sawPost = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:         "app_new",
				Name:       "my-app",
				Entrypoint: "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	t.Chdir(dir)

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--name", "my-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if !sawPost {
		t.Fatal("expected POST to create the app")
	}
	if postBody["name"] != "my-app" {
		t.Errorf("expected name in create payload, got %v", postBody)
	}
	if !strings.Contains(out, `"status": "created"`) {
		t.Errorf("expected created status in output:\n%s", out)
	}

	link, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.AppID != "app_new" {
		t.Errorf("expected link file with app_new, got %+v", link)
	}
}

func TestAppsPush_UpdatesWhenAlreadyLinked(t *testing.T) {
	var sawPatch, sawPost bool
	var patchPath string

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			sawPost = true
		case r.Method == "PATCH":
			sawPatch = true
			patchPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:         "app_existing",
				Name:       "my-app",
				Entrypoint: "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	if err := writeAppLink(dir, appLink{AppID: "app_existing", Name: "my-app"}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if sawPost {
		t.Error("expected update path to PATCH, not POST (create)")
	}
	if !sawPatch {
		t.Fatal("expected PATCH to update the existing app")
	}
	if patchPath != "/api/v1/platform/custom-apps/app_existing" {
		t.Errorf("unexpected PATCH path: %s", patchPath)
	}
}

func TestAppsPush_CreatesWhenLinkedButNotYetCreated(t *testing.T) {
	var sawPost bool
	var postBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			sawPost = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:         "app_new",
				Name:       "my-aq-app",
				Entrypoint: "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	// Mirrors what "apps init --name my-aq-app --template annotation-queue" writes.
	if err := writeAppLink(dir, appLink{Name: "my-aq-app"}); err != nil {
		t.Fatalf("seed partial link: %v", err)
	}
	t.Chdir(dir)

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if !sawPost {
		t.Fatal("expected POST to create the app (app_id was empty, so this isn't an update)")
	}
	if postBody["name"] != "my-aq-app" {
		t.Errorf("expected name from the link file used as fallback, got %v", postBody)
	}
	if !strings.Contains(out, `"status": "created"`) {
		t.Errorf("expected created status, got:\n%s", out)
	}

	link, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.AppID != "app_new" {
		t.Errorf("expected link file updated with the real app_id, got %+v", link)
	}
}

func TestAppsPush_RecreatesWhenLinkedAppWasDeleted(t *testing.T) {
	var sawPatch, sawPost bool
	var postBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PATCH" && r.URL.Path == "/api/v1/platform/custom-apps/app_deleted":
			sawPatch = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "custom app not found"})
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			sawPost = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:         "app_recreated",
				Name:       "my-app",
				Entrypoint: "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	if err := writeAppLink(dir, appLink{AppID: "app_deleted", Name: "my-app"}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	t.Chdir(dir)

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if !sawPatch {
		t.Fatal("expected push to try PATCH against the linked (now-deleted) app_id first")
	}
	if !sawPost {
		t.Fatal("expected push to fall back to POST (create) after the PATCH 404s")
	}
	if postBody["name"] != "my-app" {
		t.Errorf("expected the recreated app to reuse the old app's name, got %v", postBody)
	}
	if !strings.Contains(out, `"status": "created"`) {
		t.Errorf("expected created status, got:\n%s", out)
	}

	link, err := readAppLink(dir)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.AppID != "app_recreated" {
		t.Errorf("expected link file relinked to the new app_id, got %+v", link)
	}
}

func TestAppsPush_ErrorsWhenEntrypointMissing(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.js"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	t.Chdir(dir)

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"push", "--name", "my-app"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when default entrypoint dist/bundle.js is missing")
	}
}

func TestAppsPush_BuildsFromPackageJSONByDefault(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: "app_new", Name: "my-app", Entrypoint: "dist/bundle.js"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	fakeNpm(t, `mkdir -p dist && printf 'module.exports={render:function(){}}' > dist/bundle.js`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatalf("seed package.json: %v", err)
	}
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--name", "my-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(dir, "dist", "bundle.js")); err != nil {
		t.Errorf("expected the auto-detected \"build\" script to produce dist/bundle.js: %v", err)
	}
}

func TestAppsPush_NoBuildSkipsAutomaticBuild(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	fakeNpm(t, `mkdir -p dist && printf 'module.exports={render:function(){}}' > dist/bundle.js`)
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatalf("seed package.json: %v", err)
	}
	t.Chdir(dir)

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"push", "--name", "my-app", "--no-build"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error: --no-build should skip the build, leaving dist/bundle.js missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "bundle.js")); err == nil {
		t.Error("--no-build must not run the package.json \"build\" script")
	}
}

func TestAppsPush_UploadsSourceArchiveOnCreate(t *testing.T) {
	var postBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/platform/custom-apps":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: "app_new", Name: "my-app", Entrypoint: "dist/bundle.js"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	seedFile(t, dir, "src/App.tsx", "export default function App() {}")
	seedFile(t, dir, "node_modules/react/index.js", "dep")
	seedFile(t, dir, ".env", "LANGSMITH_API_KEY=lsv2_secret")
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--name", "my-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	encoded, ok := postBody["source_archive"].(string)
	if !ok || encoded == "" {
		t.Fatalf("expected source_archive in the create payload, got %v", keysOfAny(postBody))
	}
	entries := untarBase64(t, encoded)
	if _, ok := entries["src/App.tsx"]; !ok {
		t.Errorf("expected source files in the archive, got %v", keysOf(entries))
	}
	for _, unwanted := range []string{"node_modules/react/index.js", ".env", "dist/bundle.js"} {
		if _, ok := entries[unwanted]; ok {
			t.Errorf("%q must not be uploaded in the source archive", unwanted)
		}
	}
	// The runnable-files upload is unchanged and still carries the bundle.
	files, ok := postBody["files"].(map[string]any)
	if !ok || files["dist/bundle.js"] == nil {
		t.Errorf("expected the files map to still carry dist/bundle.js, got %v", postBody["files"])
	}
}

func TestAppsPush_UploadsSourceArchiveOnUpdate(t *testing.T) {
	var patchBody map[string]any

	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PATCH" && r.URL.Path == "/api/v1/platform/custom-apps/app_existing":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &patchBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: "app_existing", Name: "my-app", Entrypoint: "dist/bundle.js"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	seedFile(t, dir, "src/App.tsx", "export default function App() {}")
	if err := writeAppLink(dir, appLink{AppID: "app_existing", Name: "my-app"}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	encoded, ok := patchBody["source_archive"].(string)
	if !ok || encoded == "" {
		t.Fatalf("expected source_archive in the update payload, got %v", keysOfAny(patchBody))
	}
	if _, ok := untarBase64(t, encoded)["src/App.tsx"]; !ok {
		t.Error("expected the update archive to carry the source")
	}
	// --name was not passed, so it must stay absent rather than blank.
	if _, present := patchBody["name"]; present {
		t.Errorf("expected no name in the update payload, got %v", patchBody["name"])
	}
}

func TestAppsPush_SurfacesBackendSourceArchiveRejection(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"detail": "source archive exceeds the maximum size"})
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	seedFile(t, dir, "src/App.tsx", "app")
	t.Chdir(dir)

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"push", "--name", "my-app"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected the backend 400 to surface as an error")
	}
	if !strings.Contains(err.Error(), "source archive") {
		t.Errorf("expected a source-archive-specific message, got: %v", err)
	}
}

func keysOfAny(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
