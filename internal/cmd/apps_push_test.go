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
		case r.Method == "POST" && r.URL.Path == "/v1/platform/custom-apps":
			sawPost = true
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:          "app_new",
				Name:        "my-app",
				ContextType: "none",
				Entrypoint:  "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)

	out := captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--dir", dir, "--name", "my-app"})
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
		case r.Method == "POST" && r.URL.Path == "/v1/platform/custom-apps":
			sawPost = true
		case r.Method == "PATCH":
			sawPatch = true
			patchPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID:          "app_existing",
				Name:        "my-app",
				ContextType: "annotation_queue",
				Entrypoint:  "dist/bundle.js",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	if err := writeAppLink(dir, appLink{AppID: "app_existing", Name: "my-app", ContextType: "annotation_queue"}); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--dir", dir})
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
	if patchPath != "/v1/platform/custom-apps/app_existing" {
		t.Errorf("unexpected PATCH path: %s", patchPath)
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

	cmd := newAppsCmd()
	cmd.SetArgs([]string{"push", "--dir", dir, "--name", "my-app"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when default entrypoint dist/bundle.js is missing")
	}
}
