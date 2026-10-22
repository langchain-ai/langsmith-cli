package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-cli/internal/config"
)

const appCredentialMarker = "INERT_CREDENTIAL_PLACEHOLDER"

func seedSensitiveAppFiles(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{
		".npmrc", ".netrc", ".pgpass", ".htpasswd", "credentials.json", "secrets.json",
		"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", ".env", ".env.production",
		"server.pem", "server.key", "server.p12", "server.pfx", "server.jks",
		"server.keystore", "server.ppk", "settings.local", "SERVER.KEY", "Credentials.JSON",
		".langsmith/config.json",
	} {
		seedFile(t, dir, name, appCredentialMarker)
		seedFile(t, dir, "nested/"+name, appCredentialMarker)
	}
	seedFile(t, dir, "cli-profile.json", `{"profiles":{"test":{"api_key":"`+appCredentialMarker+`"}}}`)
	t.Setenv(config.ConfigFileEnv, filepath.Join(dir, "cli-profile.json"))
	seedFile(t, dir, "src/App.tsx", "export const safeSource = true;")
	seedFile(t, dir, ".gitignore", "dist/\n!.npmrc\n!.netrc\n!credentials.json\n!server.key\n!.langsmith/\n!cli-profile.json\n")
}

func assertNoAppCredentials(t *testing.T, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if strings.Contains(content, appCredentialMarker) {
			t.Errorf("credential placeholder leaked in %s", name)
		}
	}
}

func TestAppsPush_ExcludesSensitiveFiles(t *testing.T) {
	var methods []string
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload struct {
			Files         map[string]string `json:"files"`
			SourceArchive string            `json:"source_archive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		methods = append(methods, r.Method)
		assertNoAppCredentials(t, payload.Files)
		if payload.Files["dist/bundle.js"] == "" {
			t.Error("gitignored runtime bundle missing from payload")
		}
		entries := untarBase64(t, payload.SourceArchive)
		assertNoAppCredentials(t, entries)
		if entries["src/App.tsx"] == "" {
			t.Error("source missing from archive")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(customApp{ID: "app_safe", Name: "safe-app", Entrypoint: "dist/bundle.js"})
	})
	defer setupTestEnv(t, srv.URL)()
	dir := t.TempDir()
	seedAppDir(t, dir)
	seedSensitiveAppFiles(t, dir)
	t.Chdir(dir)
	for range 2 {
		captureStdout(t, func() {
			cmd := newAppsCmd()
			cmd.SetArgs([]string{"push", "--name", "safe-app"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("push: %v", err)
			}
		})
	}
	if strings.Join(methods, ",") != "POST,PATCH" {
		t.Fatalf("expected create and update, got %v", methods)
	}
}

func TestPrepareAppsDevServer_ExcludesSensitiveFilesOnReload(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "module.exports = { render: function() {} }")
	srv, ln, previewURL, err := prepareAppsDevServer(nil, dir, "dist/bundle.js")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	for range 2 {
		resp, err := http.Get(previewURL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "module.exports") {
			t.Fatal("expected working preview with runtime bundle")
		}
		if strings.Contains(string(body), appCredentialMarker) {
			t.Error("credential placeholder leaked into preview HTML")
		}
		seedSensitiveAppFiles(t, dir)
	}
}

func TestAppFiles_ExcludeConfiguredCredentials(t *testing.T) {
	for _, location := range []string{"default", "relative", "symlink", "hardlink"} {
		t.Run(location, func(t *testing.T) {
			dir := t.TempDir()
			seedFile(t, dir, "index.js", "safe")
			seedFile(t, dir, "profile.json", appCredentialMarker)
			t.Chdir(dir)
			t.Setenv("HOME", dir)
			t.Setenv("USERPROFILE", dir)
			t.Setenv(config.ConfigFileEnv, "profile.json")
			switch location {
			case "default":
				seedFile(t, dir, ".langsmith/config.json", appCredentialMarker)
				if err := os.Remove(filepath.Join(dir, "profile.json")); err != nil {
					t.Fatal(err)
				}
				t.Setenv(config.ConfigFileEnv, "")
			case "symlink":
				alias := filepath.Join(t.TempDir(), "config.json")
				if err := os.Symlink(filepath.Join(dir, "profile.json"), alias); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				t.Setenv(config.ConfigFileEnv, alias)
			case "hardlink":
				if err := os.Link(filepath.Join(dir, "profile.json"), filepath.Join(dir, "alias.json")); err != nil {
					t.Skipf("hardlink unavailable: %v", err)
				}
			}
			files, err := readDirectoryAsAppFiles(dir)
			if err != nil {
				t.Fatal(err)
			}
			assertNoAppCredentials(t, files)
			encoded, err := buildSourceArchive(dir)
			if err != nil {
				t.Fatal(err)
			}
			assertNoAppCredentials(t, untarBase64(t, encoded))
		})
	}
}
