package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	// --queue-id is gone: apps are uniform now and the sandbox always gets {}.
	for _, gone := range []string{"url", "web-url", "data", "queue-id"} {
		if f := dev.Flags().Lookup(gone); f != nil {
			t.Errorf("expected --%s flag to be gone from apps dev", gone)
		}
	}
	if f := dev.Flags().Lookup("entrypoint"); f == nil || f.DefValue != "dist/bundle.js" {
		t.Errorf("expected --entrypoint flag defaulting to dist/bundle.js, got %+v", f)
	}
	if f := dev.Flags().Lookup("no-open"); f == nil {
		t.Error("expected --no-open flag to exist")
	}
	if f := dev.Flags().Lookup("no-watch"); f == nil {
		t.Error("expected --no-watch flag to exist")
	}
}

func seedDevApp(t *testing.T, dir, entrypointContent string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatalf("mkdir dist: %v", err)
	}
	entrypointAbs := filepath.Join(dir, "dist", "bundle.js")
	if err := os.WriteFile(entrypointAbs, []byte(entrypointContent), 0o644); err != nil {
		t.Fatalf("seed bundle.js: %v", err)
	}
	return entrypointAbs
}

func TestPrepareAppsDevServer_ServesRealSandboxedIframe(t *testing.T) {
	dir := t.TempDir()
	entrypointAbs := seedDevApp(t, dir, "module.exports = { render: function(d, r) { r.textContent = 'v1'; } }")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SUPER_SECRET_VALUE"), 0o644); err != nil {
		t.Fatalf("seed .env: %v", err)
	}

	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js")
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	resp, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s: %v", previewURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(body)

	if !strings.Contains(page, `sandbox="allow-scripts"`) {
		t.Errorf("expected a real sandbox=\"allow-scripts\" iframe:\n%s", page)
	}
	if strings.Contains(page, `sandbox="allow-scripts allow-same-origin"`) ||
		strings.Contains(page, `sandbox="allow-same-origin allow-scripts"`) {
		t.Errorf("must never grant allow-same-origin — that defeats the sandbox:\n%s", page)
	}
	if !strings.Contains(page, "v1") {
		t.Errorf("expected entrypoint content to be embedded in the sandboxed srcdoc:\n%s", page)
	}
	if strings.Contains(page, "SUPER_SECRET_VALUE") {
		t.Errorf(".env content leaked into the served page:\n%s", page)
	}

	// Rewriting the entrypoint must be reflected on the next request.
	if err := os.WriteFile(entrypointAbs, []byte("module.exports = { render: function(d, r) { r.textContent = 'v2'; } }"), 0o644); err != nil {
		t.Fatalf("rewrite bundle.js: %v", err)
	}
	resp2, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s (2nd): %v", previewURL, err)
	}
	defer func() { _ = resp2.Body.Close() }()
	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), "v2") {
		t.Error("expected rewritten entrypoint content to be reflected on next request")
	}
}

// Regression test: renderApp() must not clear #root before re-rendering —
// that corrupts React's cached root and leaves the app blank after the first render.
func TestPrepareAppsDevServer_DoesNotClearRootBeforeSuccessfulRender(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "module.exports = { render: function(d, r) { r.textContent = 'ok'; } }")

	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js")
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	resp, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s: %v", previewURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(body)

	if strings.Contains(page, "root.innerHTML = '';") {
		t.Errorf("renderApp() must not unconditionally clear #root before calling window.__render — it corrupts the entrypoint's cached React root on every render after the first:\n%s", page)
	}
}

// The config toolbar (and its Light/Dark mode toggle) renders for every app.
func TestPrepareAppsDevServer_ServesModeToggle(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "module.exports = { render: function(){} }")

	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js")
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	resp, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s: %v", previewURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	page := string(body)

	if !strings.Contains(page, `id="ls-dev-mode-light"`) || !strings.Contains(page, `id="ls-dev-mode-dark"`) {
		t.Errorf("every app should get always-visible Light/Dark buttons:\n%s", page)
	}
	// The queue selector is gone entirely now.
	if strings.Contains(page, `id="ls-dev-queue-select"`) {
		t.Errorf("the queue selector should no longer be served:\n%s", page)
	}
	// On READY the host posts LANGSMITH_METADATA, and the toggle re-posts it.
	if !strings.Contains(page, "LANGSMITH_METADATA") {
		t.Errorf("expected the host page to post LANGSMITH_METADATA:\n%s", page)
	}
	// html.EscapeString doesn't touch this substring, so it appears verbatim.
	if !strings.Contains(page, "window.__render(currentData, root, currentMetadata)") {
		t.Errorf("expected the sandbox srcdoc to call render with (data, root, metadata):\n%s", page)
	}
}

// Asserts on the template constant directly to avoid fighting HTML-escaping.
func TestSandboxImplementsThemeMetadataContract(t *testing.T) {
	for _, want := range []string{
		"LANGSMITH_METADATA",
		"currentMetadata = msg.metadata",
		"classList.toggle('dark', currentMetadata && currentMetadata.mode === 'dark')",
		"window.__render(currentData, root, currentMetadata)",
		// First render gates on all three of theme, metadata, and data.
		"themeReady && currentMetadata !== null && currentData !== null",
	} {
		if !strings.Contains(sandboxInnerHTMLTemplate, want) {
			t.Errorf("sandboxInnerHTMLTemplate missing %q", want)
		}
	}
}

func TestPrepareAppsDevServer_ServesWaitingPageWhenEntrypointMissing(t *testing.T) {
	dir := t.TempDir()
	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js")
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	resp, err := http.Get(previewURL)
	if err != nil {
		t.Fatalf("GET %s: %v", previewURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (a friendly waiting page, not an error status), got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "/__ls_dev/mtime") {
		t.Error("expected the waiting page to poll for the entrypoint appearing")
	}
}

func TestPrepareAppsDevServer_MtimeReflectsFileState(t *testing.T) {
	dir := t.TempDir()
	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js")
	if err != nil {
		t.Fatalf("prepareAppsDevServer: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	mtimeURL := strings.TrimSuffix(previewURL, "/") + "/__ls_dev/mtime"
	get := func() (bool, int64) {
		resp, err := http.Get(mtimeURL)
		if err != nil {
			t.Fatalf("GET %s: %v", mtimeURL, err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body struct {
			Exists bool  `json:"exists"`
			MTime  int64 `json:"mtime"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body.Exists, body.MTime
	}

	if exists, _ := get(); exists {
		t.Error("expected exists=false before the entrypoint is built")
	}

	seedDevApp(t, dir, "module.exports = { render: function(){} }")
	exists, mtime := get()
	if !exists || mtime == 0 {
		t.Errorf("expected exists=true and nonzero mtime once built, got exists=%v mtime=%d", exists, mtime)
	}
}

func TestHandleLsDevCall_ForwardsOperationToRealClient(t *testing.T) {
	var sawMethod, sawPath, sawBody string
	upstream := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawMethod = r.Method
		sawPath = r.URL.Path + "?" + r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		sawBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": "fb_1"}`))
	})
	defer setupTestEnv(t, upstream.URL)()

	reqBody := `{"operation":"POST /api/v1/feedback","args":{"body":{"key":"correctness","score":1}}}`
	req := httptest.NewRequest(http.MethodPost, "/__ls_dev/call", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handleLsDevCall(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 passed through, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"fb_1"`) {
		t.Errorf("expected upstream response body passed through, got %s", rec.Body.String())
	}
	if sawMethod != "POST" {
		t.Errorf("expected upstream POST, got %s", sawMethod)
	}
	if !strings.HasPrefix(sawPath, "/api/v1/feedback") {
		t.Errorf("expected upstream path /api/v1/feedback, got %s", sawPath)
	}
	if !strings.Contains(sawBody, `"correctness"`) {
		t.Errorf("expected request body forwarded, got %s", sawBody)
	}
}

func TestHandleLsDevCall_ForwardsParamsAsQueryString(t *testing.T) {
	var sawQuery string
	upstream := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		sawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	defer setupTestEnv(t, upstream.URL)()

	reqBody := `{"operation":"GET /api/v1/annotation-queues/q1/runs","args":{"params":{"status":"needs_my_review"}}}`
	req := httptest.NewRequest(http.MethodPost, "/__ls_dev/call", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handleLsDevCall(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if sawQuery != "status=needs_my_review" {
		t.Errorf("expected query string forwarded, got %q", sawQuery)
	}
}

func TestHandleLsDevCall_RejectsMalformedOperation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/__ls_dev/call", strings.NewReader(`{"operation":"nospacehere"}`))
	rec := httptest.NewRecorder()
	handleLsDevCall(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed operation, got %d", rec.Code)
	}
}

func TestHandleLsDevCall_RejectsDisallowedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/__ls_dev/call", strings.NewReader(`{"operation":"TRACE /api/v1/runs"}`))
	rec := httptest.NewRecorder()
	handleLsDevCall(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for disallowed method, got %d", rec.Code)
	}
}

func TestHandleLsDevCall_RejectsPathEscapingOrigin(t *testing.T) {
	for _, op := range []string{
		"GET https://evil.example.com/steal",
		"GET //evil.example.com/steal",
		"GET not-a-path",
	} {
		req := httptest.NewRequest(http.MethodPost, "/__ls_dev/call", strings.NewReader(`{"operation":"`+op+`"}`))
		rec := httptest.NewRecorder()
		handleLsDevCall(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("operation %q: expected 400, got %d", op, rec.Code)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestPackageJSONScript_NoPackageJSON(t *testing.T) {
	dir := t.TempDir()
	script, exists, err := packageJSONScript(dir, "watch")
	if err != nil || exists || script != "" {
		t.Errorf("got script=%q exists=%v err=%v, want \"\", false, nil", script, exists, err)
	}
}

func TestPackageJSONScript_ExistsWithoutTheScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"build":"vite build"}}`)
	script, exists, err := packageJSONScript(dir, "watch")
	if err != nil || !exists || script != "" {
		t.Errorf("got script=%q exists=%v err=%v, want \"\", true, nil", script, exists, err)
	}
}

func TestPackageJSONScript_ReturnsTheScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"watch":"vite build --watch"}}`)
	script, exists, err := packageJSONScript(dir, "watch")
	if err != nil || !exists || script != "vite build --watch" {
		t.Errorf("got script=%q exists=%v err=%v, want \"vite build --watch\", true, nil", script, exists, err)
	}
}

func TestPackageJSONScript_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{not json`)
	if _, _, err := packageJSONScript(dir, "watch"); err == nil {
		t.Error("expected an error for malformed package.json")
	}
}

// fakeNpmOnPath stubs "npm" so tests don't need a real install.
func fakeNpmOnPath(t *testing.T, markerPath string) {
	t.Helper()
	fakeNpmDir := t.TempDir()
	script := "#!/bin/sh\ntouch \"" + markerPath + "\"\nwhile true; do sleep 0.1; done\n"
	if err := os.WriteFile(filepath.Join(fakeNpmDir, "npm"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake npm: %v", err)
	}
	t.Setenv("PATH", fakeNpmDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestStartWatchProcess_SpawnsWatchScriptAndIsKilledOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"watch":"anything"}}`)
	marker := filepath.Join(dir, "watch-ran.marker")
	fakeNpmOnPath(t, marker)

	ctx, cancel := context.WithCancel(context.Background())
	if started := startWatchProcess(ctx, dir); !started {
		t.Fatal("expected startWatchProcess to report started=true")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watch script never ran (marker file not created)")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// ctx cancellation should kill the process, not hang.
	cancel()
	time.Sleep(200 * time.Millisecond)
}

func TestStartWatchProcess_NoWatchScriptDoesNotSpawn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"build":"vite build"}}`)
	marker := filepath.Join(dir, "watch-ran.marker")
	fakeNpmOnPath(t, marker)

	if started := startWatchProcess(context.Background(), dir); started {
		t.Error("expected started=false when package.json has no \"watch\" script")
	}
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(marker); err == nil {
		t.Error("expected no process to be spawned when package.json has no \"watch\" script")
	}
}

func TestStartWatchProcess_NoPackageJSONDoesNotSpawn(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "watch-ran.marker")
	fakeNpmOnPath(t, marker)

	if started := startWatchProcess(context.Background(), dir); started {
		t.Error("expected started=false when there's no package.json")
	}
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(marker); err == nil {
		t.Error("expected no process to be spawned when there's no package.json")
	}
}

func TestWaitForFreshEntrypoint_ReturnsOnceFileFirstAppears(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dist", "bundle.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(path, []byte("v1"), 0o644)
	}()

	start := time.Now()
	waitForFreshEntrypoint(context.Background(), path, time.Time{}, 5*time.Second)
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("returned too early (%v) — should have waited for the file to appear", elapsed)
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("expected the file to exist once waitForFreshEntrypoint returns")
	}
}

func TestWaitForFreshEntrypoint_WaitsForNewerMTimeThanPrevBuild(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dist", "bundle.js")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale build: %v", err)
	}
	staleInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	prevBuildTime := staleInfo.ModTime()

	// The file briefly disappears, then reappears with a newer mtime.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.Remove(path)
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(path, []byte("fresh"), 0o644)
	}()

	done := make(chan struct{})
	go func() {
		waitForFreshEntrypoint(context.Background(), path, prevBuildTime, 5*time.Second)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("waitForFreshEntrypoint did not return once a fresh build appeared")
	}

	info, err := os.Stat(path)
	if err != nil || !info.ModTime().After(prevBuildTime) {
		t.Error("expected the entrypoint's mtime to be newer than prevBuildTime once returned")
	}
}

func TestWaitForFreshEntrypoint_GivesUpAfterTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dist", "bundle.js") // never created

	start := time.Now()
	waitForFreshEntrypoint(context.Background(), path, time.Time{}, 200*time.Millisecond)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected to give up close to the timeout, took %v", elapsed)
	}
}

func TestWaitForFreshEntrypoint_ReturnsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dist", "bundle.js") // never created
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	waitForFreshEntrypoint(ctx, path, time.Time{}, 5*time.Second)
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Errorf("expected an already-cancelled context to return immediately, took %v", elapsed)
	}
}

func TestRunAppsDev_ExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "hi")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runAppsDev(ctx, dir, "dist/bundle.js", true, true)
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
