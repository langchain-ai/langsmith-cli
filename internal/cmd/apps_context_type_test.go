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

// --- init: --context and the thread template ---

func TestAppsInit_ContextTypeWrittenToLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", appContextThread, appTypes["blank"], false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.ContextType != appContextThread {
		t.Errorf("expected context_type %q in link, got %+v", appContextThread, link)
	}
}

func TestAppsInit_NoContextTypeLeavesLinkClean(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	if _, err := scaffoldCustomAppStarter(target, "my-app", "", "", appTypes["blank"], false); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	// "none" normalizes to empty and must not be persisted.
	if _, err := scaffoldCustomAppStarter(filepath.Join(dir, "n"), "n", "", appContextNone, appTypes["blank"], false); err != nil {
		t.Fatalf("scaffold none: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(target, appsLinkDir, appsLinkFile))
	if err != nil {
		t.Fatalf("read link: %v", err)
	}
	if strings.Contains(string(raw), "context_type") {
		t.Errorf("expected no context_type key for a contextless app, got:\n%s", raw)
	}
}

func TestAppsInit_ThreadTemplateScaffoldsFilesAndDefaultsContext(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my-app")

	// The thread template implies context thread even without --context.
	resolved, err := resolveInitContextType("", appTypes["thread"])
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != appContextThread {
		t.Fatalf("expected thread template to default context to %q, got %q", appContextThread, resolved)
	}

	written, err := scaffoldCustomAppStarter(target, "my-app", "", resolved, appTypes["thread"], false)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	got := map[string]bool{}
	for _, w := range written {
		got[w] = true
	}
	for _, want := range []string{
		"package.json",
		"README.md",
		"AGENTS.md",
		"src/entry.tsx",
		"src/App.tsx",
		"src/global.d.ts",
	} {
		if !got[want] {
			t.Errorf("expected thread template to scaffold %q; written: %v", want, written)
		}
	}

	link, err := readAppLink(target)
	if err != nil {
		t.Fatalf("readAppLink: %v", err)
	}
	if link == nil || link.ContextType != appContextThread {
		t.Errorf("expected thread link context_type, got %+v", link)
	}

	agentsMD, err := os.ReadFile(filepath.Join(target, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsMD), "thread context") {
		t.Errorf("expected the thread AGENTS.md fragment to be included:\n%s", agentsMD)
	}
}

func TestResolveInitContextType_ConflictErrors(t *testing.T) {
	// "none" normalizes to "" (the default / omitted), so it does not fight the
	// template — the template's context wins.
	if got, err := resolveInitContextType(appContextNone, appTypes["thread"]); err != nil || got != appContextThread {
		t.Errorf("expected --context none to defer to the thread template, got %q err=%v", got, err)
	}
	// Matching is fine.
	if _, err := resolveInitContextType(appContextThread, appTypes["thread"]); err != nil {
		t.Errorf("expected thread+thread to be allowed, got %v", err)
	}
	// A contextless template accepts any explicit context.
	if got, err := resolveInitContextType(appContextThread, appTypes["blank"]); err != nil || got != appContextThread {
		t.Errorf("expected blank template to accept explicit thread context, got %q err=%v", got, err)
	}
}

func TestValidateAppContextType(t *testing.T) {
	for _, ok := range []string{"", appContextNone, appContextThread} {
		if err := validateAppContextType(ok); err != nil {
			t.Errorf("expected %q valid, got %v", ok, err)
		}
	}
	// annotation_queue has no CLI workflow and must now be rejected.
	if err := validateAppContextType("annotation_queue"); err == nil {
		t.Error("expected annotation_queue to be rejected")
	}
	if err := validateAppContextType("bogus"); err == nil {
		t.Error("expected an error for an unknown context type")
	}
}

// --- push: sends on create, omits on update, guards immutability ---

func TestAppsPush_SendsContextTypeOnCreate(t *testing.T) {
	var postBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/platform/custom-apps" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID: "app_new", Name: "my-app", Entrypoint: "dist/bundle.js", ContextType: appContextThread,
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	if err := writeAppLink(dir, appLink{Name: "my-app", ContextType: appContextThread}); err != nil {
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

	if postBody["context_type"] != appContextThread {
		t.Errorf("expected context_type %q in create payload, got %v", appContextThread, postBody["context_type"])
	}
	if !strings.Contains(out, `"context_type": "thread"`) {
		t.Errorf("expected context_type in output:\n%s", out)
	}

	// Server value round-trips back into the link.
	link, _ := readAppLink(dir)
	if link == nil || link.ContextType != appContextThread {
		t.Errorf("expected link to record context_type thread, got %+v", link)
	}
}

func TestAppsPush_OmitsContextTypeOnCreateWhenNone(t *testing.T) {
	var postBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/v1/platform/custom-apps" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &postBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{ID: "app_new", Name: "my-app", Entrypoint: "dist/bundle.js"})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	t.Chdir(dir)

	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push", "--name", "my-app"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	if _, ok := postBody["context_type"]; ok {
		t.Errorf("expected no context_type in create payload for a contextless app, got %v", postBody["context_type"])
	}
}

func TestAppsPush_NeverSendsContextTypeOnUpdate(t *testing.T) {
	var patchBody map[string]any
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &patchBody)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(customApp{
				ID: "app_existing", Name: "my-app", Entrypoint: "dist/bundle.js", ContextType: appContextThread,
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	// Configured context matches the server's — no mismatch, and update must
	// still not send the field.
	if err := writeAppLink(dir, appLink{AppID: "app_existing", Name: "my-app", ContextType: appContextThread}); err != nil {
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

	if _, ok := patchBody["context_type"]; ok {
		t.Errorf("expected update payload to never carry context_type, got %v", patchBody["context_type"])
	}
}

func TestAppsPush_FailsOnContextTypeMismatch(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			w.Header().Set("Content-Type", "application/json")
			// Server still reports the original context (none) even though the
			// config now asks for thread — the update silently ignored it.
			_ = json.NewEncoder(w).Encode(customApp{
				ID: "app_existing", Name: "my-app", Entrypoint: "dist/bundle.js", ContextType: appContextNone,
			})
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	})
	defer setupTestEnv(t, srv.URL)()

	dir := t.TempDir()
	seedAppDir(t, dir)
	if err := writeAppLink(dir, appLink{AppID: "app_existing", Name: "my-app", ContextType: appContextThread}); err != nil {
		t.Fatalf("seed link: %v", err)
	}
	t.Chdir(dir)

	var execErr error
	captureStdout(t, func() {
		cmd := newAppsCmd()
		cmd.SetArgs([]string{"push"})
		execErr = cmd.Execute()
	})
	if execErr == nil {
		t.Fatal("expected push to fail when configured context type differs from the created app's")
	}
	msg := strings.ToLower(execErr.Error())
	if !strings.Contains(msg, "context type is fixed") || !strings.Contains(msg, "delete and recreate") {
		t.Errorf("expected a delete-and-recreate immutability error, got: %v", execErr)
	}
}

func TestCheckContextTypeImmutable_SkipsWhenServerSilent(t *testing.T) {
	// An older server that doesn't echo context_type: treat as unknown, don't
	// produce a spurious mismatch.
	if err := checkContextTypeImmutable(customApp{Name: "x"}, appContextThread); err != nil {
		t.Errorf("expected no error when the server reports no context type, got %v", err)
	}
	// "none" configured vs "none" server (empty): no error.
	if err := checkContextTypeImmutable(customApp{Name: "x", ContextType: appContextNone}, ""); err != nil {
		t.Errorf("expected none==none to be fine, got %v", err)
	}
}

// --- dev: --thread-id/--project-id supply render data ---

func TestAppsDev_ThreadContextFlagsExist(t *testing.T) {
	dev, _, err := newAppsCmd().Find([]string{"dev"})
	if err != nil {
		t.Fatalf("find dev: %v", err)
	}
	for _, name := range []string{"thread-id", "project-id"} {
		if dev.Flags().Lookup(name) == nil {
			t.Errorf("expected apps dev to have --%s", name)
		}
	}
	// It must NOT gain a --project flag: --project-id here is thread context,
	// not project selection, and pairing it would drag in resolveSessionID.
	if dev.Flags().Lookup("project") != nil {
		t.Error("apps dev should not have a --project flag; --project-id is thread context, not selection")
	}
}

func TestRenderDevHostHTML_EmbedsThreadRenderData(t *testing.T) {
	page := renderDevHostHTML(
		map[string]string{"dist/bundle.js": "module.exports={render:function(){}}"},
		"dist/bundle.js", "tok", "ws", "https://smith.langchain.com",
		devContext{threadID: "thr_123", projectID: "prj_456"}.renderData(),
	)
	if !strings.Contains(page, `"threadId":"thr_123"`) {
		t.Errorf("expected threadId embedded in render data:\n%s", page)
	}
	if !strings.Contains(page, `"projectId":"prj_456"`) {
		t.Errorf("expected projectId embedded in render data:\n%s", page)
	}
	if strings.Contains(page, "var data = {};") {
		t.Error("expected the hard-coded empty render data to be replaced")
	}
}

func TestRenderDevHostHTML_EmptyContextStaysEmpty(t *testing.T) {
	page := renderDevHostHTML(
		map[string]string{"dist/bundle.js": "x"},
		"dist/bundle.js", "tok", "ws", "https://smith.langchain.com",
		devContext{}.renderData(),
	)
	if !strings.Contains(page, "var data = {};") {
		t.Errorf("expected empty ({}) render data for a contextless app:\n%s", page)
	}
}

func TestRenderDevHostHTML_EscapesScriptCloseInIDs(t *testing.T) {
	// A malicious/odd ID must not break out of the <script> block. Go's
	// json.Marshal is HTML-safe (escapes < and > as < / >), and
	// escapeForScript is a further backstop against a literal </script>.
	page := renderDevHostHTML(
		map[string]string{"dist/bundle.js": "x"},
		"dist/bundle.js", "tok", "ws", "https://smith.langchain.com",
		devContext{threadID: "</script><script>alert(1)</script>"}.renderData(),
	)
	if strings.Contains(page, "</script><script>alert(1)") {
		t.Errorf("thread id broke out of the script context (not escaped):\n%s", page)
	}
	if !strings.Contains(page, `</script>`) {
		t.Errorf("expected the injected id to be \\u003c-escaped in the embedded JSON:\n%s", page)
	}
}
