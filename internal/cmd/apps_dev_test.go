package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppsDevCmd_Flags(t *testing.T) {
	cmd := newAppsCmd()
	dev, _, err := cmd.Find([]string{"dev"})
	if err != nil {
		t.Fatalf("find dev command: %v", err)
	}
	for _, gone := range []string{"url", "web-url"} {
		if f := dev.Flags().Lookup(gone); f != nil {
			t.Errorf("expected --%s flag to be gone from apps dev", gone)
		}
	}
	if f := dev.Flags().Lookup("entrypoint"); f == nil || f.DefValue != "dist/bundle.js" {
		t.Errorf("expected --entrypoint flag defaulting to dist/bundle.js, got %+v", f)
	}
	if f := dev.Flags().Lookup("data"); f == nil {
		t.Error("expected --data flag to exist")
	}
	if f := dev.Flags().Lookup("no-open"); f == nil {
		t.Error("expected --no-open flag to exist")
	}
}

func TestDefaultDevData_UsesLinkedContextType(t *testing.T) {
	got := defaultDevData(&appLink{ContextType: "annotation_queue"})
	want := map[string]any{"inputs": map[string]any{}, "outputs": map[string]any{}}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("got %s, want %s", gotJSON, wantJSON)
	}

	for _, link := range []*appLink{nil, {ContextType: "none"}, {ContextType: "experiment"}} {
		got := defaultDevData(link)
		gotJSON, _ := json.Marshal(got)
		if string(gotJSON) != "{}" {
			t.Errorf("link %+v: got %s, want {}", link, gotJSON)
		}
	}
}

func TestLoadDevDataOverride_ReadsJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(path, []byte(`{"inputs": {"question": "hi"}}`), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := loadDevDataOverride(path)
	if err != nil {
		t.Fatalf("loadDevDataOverride: %v", err)
	}
	gotJSON, _ := json.Marshal(got)
	if !strings.Contains(string(gotJSON), `"question":"hi"`) {
		t.Errorf("unexpected data: %s", gotJSON)
	}
}

func TestLoadDevDataOverride_ErrorsOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := loadDevDataOverride(path); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPrepareAppsDevServer_ServesHarnessEntrypointAndMtime(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	entrypointAbs := filepath.Join(dir, "dist", "bundle.js")
	if err := os.WriteFile(entrypointAbs, []byte("module.exports = { render: function(d, r) { r.textContent = 'v1'; } }"), 0o644); err != nil {
		t.Fatalf("seed bundle.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	srv, ln, previewURL, entrypointURL, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{"inputs": map[string]any{}})
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	if !strings.HasSuffix(previewURL, "/") {
		t.Errorf("expected previewURL to end in /, got %s", previewURL)
	}
	if !strings.HasSuffix(entrypointURL, "/dist/bundle.js") {
		t.Errorf("expected entrypointURL to end with /dist/bundle.js, got %s", entrypointURL)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	// The harness page loads and references the entrypoint + sample data.
	resp, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s: %v", previewURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)
	if !strings.Contains(html, `"/dist/bundle.js"`) {
		t.Errorf("expected harness HTML to reference entrypoint path:\n%s", html)
	}
	if !strings.Contains(html, `"inputs":{}`) {
		t.Errorf("expected harness HTML to embed sample data:\n%s", html)
	}

	// The entrypoint itself is served fresh on every request.
	getEntrypoint := func() string {
		resp, err := http.Get(entrypointURL)
		if err != nil {
			t.Fatalf("GET %s: %v", entrypointURL, err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("expected no-store Cache-Control, got %q", resp.Header.Get("Cache-Control"))
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if !strings.Contains(getEntrypoint(), "v1") {
		t.Error("expected entrypoint content to be served")
	}
	if err := os.WriteFile(entrypointAbs, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite bundle.js: %v", err)
	}
	if getEntrypoint() != "v2" {
		t.Error("expected rewritten entrypoint content to be served immediately")
	}

	// Only the entrypoint is served — not the rest of the directory.
	envURL := strings.TrimSuffix(entrypointURL, "dist/bundle.js") + ".env"
	envResp, err := http.Get(envURL)
	if err != nil {
		t.Fatalf("GET %s: %v", envURL, err)
	}
	defer func() { _ = envResp.Body.Close() }()
	if envResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected .env to be unreachable (404), got %d", envResp.StatusCode)
	}

	// mtime endpoint reflects the current file state and changes on rewrite.
	mtimeURL := strings.TrimSuffix(previewURL, "/") + "/__ls_dev/mtime"
	mtimeResp, err := http.Get(mtimeURL)
	if err != nil {
		t.Fatalf("GET %s: %v", mtimeURL, err)
	}
	defer func() { _ = mtimeResp.Body.Close() }()
	var mtimeBody struct {
		Exists bool  `json:"exists"`
		MTime  int64 `json:"mtime"`
	}
	if err := json.NewDecoder(mtimeResp.Body).Decode(&mtimeBody); err != nil {
		t.Fatalf("decode mtime response: %v", err)
	}
	if !mtimeBody.Exists || mtimeBody.MTime == 0 {
		t.Errorf("expected exists=true and nonzero mtime, got %+v", mtimeBody)
	}
}

func TestPrepareAppsDevServer_MtimeReportsMissingEntrypoint(t *testing.T) {
	dir := t.TempDir()
	srv, ln, previewURL, _, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{})
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	mtimeURL := strings.TrimSuffix(previewURL, "/") + "/__ls_dev/mtime"
	resp, err := http.Get(mtimeURL)
	if err != nil {
		t.Fatalf("GET %s: %v", mtimeURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Exists bool `json:"exists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Exists {
		t.Error("expected exists=false for a missing entrypoint")
	}
}

func TestRunAppsDev_ExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "bundle.js"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("seed bundle.js: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runAppsDev(ctx, dir, "dist/bundle.js", map[string]any{}, true)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("runAppsDev returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("runAppsDev did not exit after context cancel")
	}
}
