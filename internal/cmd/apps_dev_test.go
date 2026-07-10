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
	for _, gone := range []string{"url", "web-url", "data"} {
		if f := dev.Flags().Lookup(gone); f != nil {
			t.Errorf("expected --%s flag to be gone from apps dev", gone)
		}
	}
	if f := dev.Flags().Lookup("entrypoint"); f == nil || f.DefValue != "dist/bundle.js" {
		t.Errorf("expected --entrypoint flag defaulting to dist/bundle.js, got %+v", f)
	}
	if f := dev.Flags().Lookup("queue-id"); f == nil {
		t.Error("expected --queue-id flag to exist")
	}
	if f := dev.Flags().Lookup("no-open"); f == nil {
		t.Error("expected --no-open flag to exist")
	}
}

func TestDevData_AnnotationQueueLinkUsesQueueID(t *testing.T) {
	got := devData(&appLink{ContextType: "annotation_queue"}, "q_123")
	want := map[string]any{"queueId": "q_123"}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("got %s, want %s", gotJSON, wantJSON)
	}
}

func TestDevData_NoneOrUnlinkedGetsEmptyObject(t *testing.T) {
	for _, link := range []*appLink{nil, {ContextType: "none"}} {
		got := devData(link, "q_123")
		gotJSON, _ := json.Marshal(got)
		if string(gotJSON) != "{}" {
			t.Errorf("link %+v: got %s, want {}", link, gotJSON)
		}
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

	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{"queueId": "q_1"}, true)
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

	if !strings.Contains(page, `id="ls-dev-queue-select"`) {
		t.Errorf("expected the annotation-queue selector bar when showQueueSelector=true:\n%s", page)
	}

	// The banner text is allowed to mention "allow-same-origin" for the
	// user's benefit — what must never happen is the sandbox *attribute*
	// actually granting it.
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
	if !strings.Contains(page, `queueId`) || !strings.Contains(page, `q_1`) {
		t.Errorf("expected sample data to be embedded:\n%s", page)
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

func TestPrepareAppsDevServer_OmitsQueueSelectorForStandaloneApps(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "module.exports = { render: function(){} }")

	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{}, false)
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
	if strings.Contains(string(body), `id="ls-dev-queue-select"`) {
		t.Errorf("standalone apps should not get a queue selector bar:\n%s", body)
	}
}

func TestPrepareAppsDevServer_ServesWaitingPageWhenEntrypointMissing(t *testing.T) {
	dir := t.TempDir()
	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{}, false)
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
	srv, ln, previewURL, err := prepareAppsDevServer(dir, "dist/bundle.js", map[string]any{}, false)
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

func TestRunAppsDev_ExitsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	seedDevApp(t, dir, "hi")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runAppsDev(ctx, dir, "dist/bundle.js", map[string]any{}, false, true)
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
