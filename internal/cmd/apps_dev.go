package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsDevCmd() *cobra.Command {
	var (
		entrypoint string
		queueID    string
		noOpen     bool
		noWatch    bool
	)

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the current directory's custom app locally in a real sandbox",
		Long: `Serve the current directory's custom app inside a real sandboxed iframe
(sandbox="allow-scripts", no allow-same-origin — identical restrictions to
production) and open it in your browser. This is entirely self-contained:
no LangSmith web app involved.

The app's window.langsmith.call(operation, args) is proxied to the real
LangSmith API using your local LANGSMITH_API_KEY — the same generic
passthrough production uses (proxied through the injected platform token
there instead). It is not a curated allowlist: operation is a
"<METHOD> <path>" string (e.g. "GET /api/v1/annotation-queues/{id}/runs"),
forwarded as-is.

If the current directory has a package.json with a "watch" script (both
starter templates do), this runs it for you automatically as a managed
child process, so editing source files and saving is enough to see the
change — no second terminal needed. The page itself polls for the
entrypoint changing and reloads whenever the watcher rebuilds it. Pass
--no-watch to skip this (e.g. you're already running your own build
process, or want to use a different one).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			link, err := readAppLink(dir)
			if err != nil {
				return err
			}
			if link != nil && link.ContextType == contextTypeAnnotationQueue && queueID == "" {
				fmt.Fprintln(os.Stderr, "note: this app is linked as context_type annotation_queue but no --queue-id was passed — it will get an empty queueId until you pass one")
			}

			showQueueSelector := link != nil && link.ContextType == contextTypeAnnotationQueue

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			return runAppsDev(ctx, dir, entrypoint, devData(link, queueID), showQueueSelector, noOpen, noWatch)
		},
	}

	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to render")
	cmd.Flags().StringVar(&queueID, "queue-id", "", "Annotation queue ID to use as context (only meaningful for context_type: annotation_queue apps)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the local URL instead of opening a browser")
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "Don't automatically run the build's \"watch\" script (e.g. \"npm run watch\") — use this if you're already running your own build process")
	return cmd
}

const contextTypeAnnotationQueue = "annotation_queue"

// devData mirrors what the real host hands a custom app at render time: an
// annotation_queue app gets only the queue's ID (it fetches everything else
// itself via window.langsmith.call); everything else gets {}.
func devData(link *appLink, queueID string) map[string]any {
	if link != nil && link.ContextType == contextTypeAnnotationQueue {
		return map[string]any{"queueId": queueID}
	}
	return map[string]any{}
}

// runAppsDev starts the local sandboxed-preview server for dir, opens it in
// a browser, and blocks until ctx is cancelled (e.g. by SIGINT) or the
// server fails.
func runAppsDev(ctx context.Context, dir, entrypoint string, data map[string]any, showQueueSelector, noOpen, noWatch bool) error {
	srv, ln, previewURL, err := prepareAppsDevServer(dir, entrypoint, data, showQueueSelector)
	if err != nil {
		return err
	}

	if !noWatch {
		startWatchProcess(ctx, dir)
	}

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve(ln) }()

	if !noOpen {
		_ = openBrowser(previewURL)
	}
	output.OutputJSON(map[string]any{
		"status": "serving",
		"url":    previewURL,
	}, "")
	fmt.Fprintf(os.Stderr, "Serving %s at %s (sandboxed) — press Ctrl+C to stop\n", dir, previewURL)

	select {
	case <-ctx.Done():
	case serveErr := <-serveErrCh:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			return fmt.Errorf("local server: %w", serveErr)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("shutting down local server: %w", err)
	}
	return nil
}

// startWatchProcess runs dir/package.json's "watch" script (e.g.
// "vite build --watch", aliased to "npm run watch" by both starter
// templates) as a child process tied to ctx, so editing source files
// rebuilds the entrypoint without a second terminal. It never fails
// runAppsDev — if there's no package.json, no "watch" script, or no npm on
// PATH, it just prints a note and returns, leaving the existing
// run-your-own-build workflow intact.
func startWatchProcess(ctx context.Context, dir string) {
	script, pkgJSONExists, err := packageJSONScript(dir, "watch")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: couldn't read package.json to find a \"watch\" script: %v\n", err)
		return
	}
	if script == "" {
		if pkgJSONExists {
			fmt.Fprintln(os.Stderr, `note: no "watch" script in package.json — start your own build/watch process to see live updates`)
		}
		return
	}
	if _, lookErr := exec.LookPath("npm"); lookErr != nil {
		fmt.Fprintln(os.Stderr, `note: npm not found on PATH — run "npm run watch" yourself to see live updates`)
		return
	}

	fmt.Fprintln(os.Stderr, "Starting build watcher: npm run watch")
	watchCmd := exec.CommandContext(ctx, "npm", "run", "watch")
	watchCmd.Dir = dir
	watchCmd.Stdout = os.Stderr
	watchCmd.Stderr = os.Stderr
	if startErr := watchCmd.Start(); startErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to start \"npm run watch\": %v\n", startErr)
		return
	}
	// Reap the process (and honor ctx cancellation, which exec.CommandContext
	// enforces via Wait) without blocking runAppsDev on it.
	go func() { _ = watchCmd.Wait() }()
}

// packageJSONScript reads dir/package.json and returns the named script.
// pkgJSONExists distinguishes "no package.json at all" (nothing to warn
// about — this may not be an npm project) from "package.json exists but has
// no such script" (worth telling the user about).
func packageJSONScript(dir, name string) (script string, pkgJSONExists bool, err error) {
	raw, readErr := os.ReadFile(filepath.Join(dir, "package.json"))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", false, nil
		}
		return "", false, readErr
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if jsonErr := json.Unmarshal(raw, &pkg); jsonErr != nil {
		return "", true, jsonErr
	}
	return pkg.Scripts[name], true, nil
}

// prepareAppsDevServer builds (but does not start) an HTTP server, bound to
// an OS-assigned free port on 127.0.0.1, that serves:
//
//   - "/"                — a host page embedding the app in a real
//     sandbox="allow-scripts" iframe (no allow-same-origin), re-read from
//     disk on every request so a rebuild is reflected on the next reload.
//   - "/__ls_dev/mtime"  — polled by the host page to detect a rebuild (or
//     the entrypoint appearing for the first time) and trigger a reload.
//   - "/__ls_dev/call"   — the generic window.langsmith.call(...) proxy,
//     forwarding to the real LangSmith API via the authenticated CLI client.
//
// The directory is read the same way "apps push" reads it
// (readDirectoryAsAppFiles), so local dev sees exactly what would be
// uploaded — including the same .env/.git/node_modules exclusions.
func prepareAppsDevServer(dir, entrypoint string, data map[string]any, showQueueSelector bool) (srv *http.Server, ln net.Listener, previewURL string, err error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		files, readErr := readDirectoryAsAppFiles(dir)
		if readErr != nil {
			_, _ = w.Write([]byte(devWaitingHTML(fmt.Sprintf("reading %s: %s", dir, readErr))))
			return
		}
		if _, ok := files[entrypoint]; !ok {
			_, _ = w.Write([]byte(devWaitingHTML(fmt.Sprintf("entrypoint %q does not exist yet in %s — waiting for the initial build to finish", entrypoint, dir))))
			return
		}
		_, _ = w.Write([]byte(renderDevHostHTML(files, entrypoint, data, showQueueSelector)))
	})

	mux.HandleFunc("/__ls_dev/mtime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		info, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(entrypoint)))
		if statErr != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"exists": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"exists": true, "mtime": info.ModTime().UnixNano()})
	})

	mux.HandleFunc("/__ls_dev/call", handleLsDevCall)

	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, "", fmt.Errorf("starting local server: %w", err)
	}
	return &http.Server{Handler: mux}, ln, "http://" + ln.Addr().String() + "/", nil
}

var allowedProxyMethods = map[string]bool{
	"GET": true, "POST": true, "PATCH": true, "PUT": true, "DELETE": true,
}

type lsDevCallRequest struct {
	Operation string        `json:"operation"`
	Args      lsDevCallArgs `json:"args"`
}

type lsDevCallArgs struct {
	Params map[string]any `json:"params,omitempty"`
	Body   any            `json:"body,omitempty"`
}

// handleLsDevCall is the local-dev twin of smith-frontend's apiProxy.ts
// createLangSmithApiProxy: a generic passthrough, not a curated allowlist.
// It forwards operation ("<METHOD> <path>") to the real LangSmith API using
// the CLI's authenticated client (the same one "langsmith api" uses), so a
// standalone/annotation_queue app can exercise real endpoints while it's
// being developed.
func handleLsDevCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req lsDevCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	spaceIdx := strings.IndexByte(req.Operation, ' ')
	if spaceIdx == -1 {
		http.Error(w, fmt.Sprintf("invalid operation %q — expected \"<METHOD> <path>\"", req.Operation), http.StatusBadRequest)
		return
	}
	method := strings.ToUpper(req.Operation[:spaceIdx])
	path := req.Operation[spaceIdx+1:]

	if !allowedProxyMethods[method] {
		http.Error(w, fmt.Sprintf("method %q is not permitted", method), http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "://") {
		http.Error(w, fmt.Sprintf("path %q must be a relative path starting with \"/\"", path), http.StatusBadRequest)
		return
	}
	if len(req.Args.Params) > 0 {
		path += "?" + encodeProxyParams(req.Args.Params)
	}

	var bodyReader io.Reader
	if req.Args.Body != nil {
		b, marshalErr := json.Marshal(req.Args.Body)
		if marshalErr != nil {
			http.Error(w, "encoding body: "+marshalErr.Error(), http.StatusInternalServerError)
			return
		}
		bodyReader = bytes.NewReader(b)
	}

	c := MustGetClient()
	status, _, respHeaders, respBody, err := c.RawDo(r.Context(), method, path, bodyReader, nil)
	if err != nil {
		http.Error(w, "request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if ct := respHeaders.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

func encodeProxyParams(params map[string]any) string {
	values := url.Values{}
	for k, v := range params {
		switch val := v.(type) {
		case []any:
			for _, item := range val {
				values.Add(k, fmt.Sprintf("%v", item))
			}
		default:
			values.Set(k, fmt.Sprintf("%v", val))
		}
	}
	return values.Encode()
}

var scriptCloseTagPattern = regexp.MustCompile(`(?i)</script>`)

func escapeForScript(jsonBytes []byte) string {
	return scriptCloseTagPattern.ReplaceAllString(string(jsonBytes), "<\\/script>")
}

// devWaitingHTML is served at "/" when the entrypoint doesn't exist yet (or
// the directory can't be read) — it polls the same mtime endpoint the real
// host page does, so it reloads itself into the real preview the moment a
// build produces the entrypoint.
func devWaitingHTML(message string) string {
	return `<!doctype html>
<html><head><meta charset="utf-8"><title>Waiting for build…</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;padding:24px;color:#334155;">
<p>` + html.EscapeString(message) + `</p>
<script>
var seen = false, lastKey = null;
setInterval(function() {
  fetch('/__ls_dev/mtime', { cache: 'no-store' }).then(function(r){ return r.json(); }).then(function(j){
    var key = j.exists ? String(j.mtime) : 'missing';
    if (!seen) { seen = true; lastKey = key; return; }
    if (key !== lastKey) { location.reload(); }
  }).catch(function(){});
}, 500);
</script>
</body></html>`
}

// renderDevHostHTML builds the top-level host page: it embeds the app in a
// real sandboxed iframe (mirroring smith-frontend's sandbox.ts
// buildSandboxSrcdoc as closely as possible, including the multi-file
// require() loader, so local dev exercises the exact same module-loading
// behavior as production) and implements the postMessage bridge from the
// host side — LANGSMITH_READY/LANGSMITH_HEIGHT/LS_API/LS_MUTATION — with
// LS_API forwarded to /__ls_dev/call, which is real network access the
// iframe itself can never have.
//
// When showQueueSelector is set (the app is linked as context_type
// annotation_queue), the page also gets a queue-picker bar — fetched live
// from the real API via the same /__ls_dev/call proxy — mirroring the
// queue picker CustomAppPreviewPanel.tsx already shows when previewing a
// contextual app from the Custom Apps tab. Standalone apps get no context
// to pick, so no bar.
func renderDevHostHTML(files map[string]string, entrypoint string, data map[string]any, showQueueSelector bool) string {
	filesJSON, _ := json.Marshal(files)
	entrypointJSON, _ := json.Marshal(entrypoint)
	dataJSON, _ := json.Marshal(data)

	inner := strings.NewReplacer(
		"__FILES_JSON__", escapeForScript(filesJSON),
		"__ENTRYPOINT_JSON__", escapeForScript(entrypointJSON),
	).Replace(sandboxInnerHTMLTemplate)

	queueBarHTML := ""
	queueBarScript := ""
	if showQueueSelector {
		queueBarHTML = queueSelectorBarHTML
		queueBarScript = queueSelectorScript
	}

	return strings.NewReplacer(
		"__SANDBOX_SRCDOC__", html.EscapeString(inner),
		"__DATA_JSON__", escapeForScript(dataJSON),
		"__QUEUE_BAR_HTML__", queueBarHTML,
		"__QUEUE_BAR_SCRIPT__", queueBarScript,
	).Replace(devHostHTMLTemplate)
}

const queueSelectorBarHTML = `<div id="ls-dev-queue-bar">
  <label for="ls-dev-queue-select">Annotation queue:</label>
  <select id="ls-dev-queue-select"><option value="">Loading queues…</option></select>
</div>
`

const queueSelectorScript = `
  var queueSelect = document.getElementById('ls-dev-queue-select');

  function callProxy(operation, args) {
    return fetch('/__ls_dev/call', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operation: operation, args: args || {} }),
    }).then(function(r) {
      return r.text().then(function(text) {
        var parsed;
        try { parsed = text ? JSON.parse(text) : null; } catch (e) { parsed = text; }
        if (!r.ok) throw new Error(typeof parsed === 'string' ? parsed : JSON.stringify(parsed));
        return parsed;
      });
    });
  }

  function loadQueues() {
    callProxy('GET /api/v1/annotation-queues', { params: { limit: '50', offset: '0' } })
      .then(function(queues) {
        queueSelect.innerHTML = '';
        var placeholder = document.createElement('option');
        placeholder.value = '';
        placeholder.textContent = 'Select a queue…';
        queueSelect.appendChild(placeholder);
        var found = false;
        (queues || []).forEach(function(q) {
          var opt = document.createElement('option');
          opt.value = q.id;
          opt.textContent = q.name + ' (' + q.id.slice(0, 8) + ')';
          if (q.id === data.queueId) { opt.selected = true; found = true; }
          queueSelect.appendChild(opt);
        });
        if (!found && data.queueId) {
          var custom = document.createElement('option');
          custom.value = data.queueId;
          custom.textContent = data.queueId + ' (from --queue-id, not in first 50)';
          custom.selected = true;
          queueSelect.appendChild(custom);
        }
      })
      .catch(function(err) {
        queueSelect.innerHTML = '<option value="">Failed to load queues</option>';
        console.error('[langsmith apps dev] failed to load annotation queues:', err);
      });
  }

  queueSelect.addEventListener('change', function() {
    data = Object.assign({}, data, { queueId: queueSelect.value });
    post({ type: 'LANGSMITH_DATA', payload: data });
  });

  loadQueues();
`

const devHostHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Local sandboxed preview</title>
<style>
  html, body { height: 100%; margin: 0; }
  body { display: flex; flex-direction: column; font-family: -apple-system, BlinkMacSystemFont, 'Inter', 'Segoe UI', system-ui, sans-serif; }

  /* Mirrors CustomAppPreviewPanel.tsx's queue picker: a level-2 surface bar
     with a subtle divider, a secondary-text label, and a Select-style
     bordered control (see smith-frontend/STYLES.md and
     design-system/components/Select). Hand-ported since this page is a
     standalone Go-templated HTML string with no access to Tailwind/CSS
     variables. */
  #ls-dev-queue-bar {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    background: #f5f8fb;
    border-bottom: 1px solid #e2e8f0;
    padding: 8px 16px;
    font-size: 13px;
  }
  #ls-dev-queue-bar label { font-weight: 500; color: #334155; }
  #ls-dev-queue-bar select {
    font-family: inherit;
    font-size: 13px;
    color: #0f172a;
    padding: 5px 28px 5px 10px;
    border-radius: 6px;
    border: 1px solid #cbd5e1;
    background-color: #ffffff;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%23475569' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 8px center;
    background-size: 14px;
    -webkit-appearance: none;
    appearance: none;
    max-width: 360px;
    cursor: pointer;
    transition: border-color 120ms ease;
  }
  #ls-dev-queue-bar select:hover { border-color: #94a3b8; }
  #ls-dev-queue-bar select:focus-visible {
    outline: none;
    border-color: #006ddd;
    box-shadow: 0 0 0 2px rgba(0, 109, 221, 0.15);
  }

  /* No prefers-color-scheme variant here on purpose: the sandboxed app
     below never actually receives a real theme (renderDevHostHTML posts a
     permanently empty LANGSMITH_THEME cssText), so it always renders with
     its hardcoded light-mode fallback regardless of OS setting. A
     dark-mode bar would mismatch that light content instead of matching
     the real (currently light-only) rendered page. */

  iframe { display: block; width: 100%; border: none; flex: 1 1 auto; }
</style>
</head>
<body>
__QUEUE_BAR_HTML__<iframe id="ls-app" sandbox="allow-scripts" srcdoc="__SANDBOX_SRCDOC__"></iframe>
<script>
(function() {
  var iframe = document.getElementById('ls-app');
  var data = __DATA_JSON__;

  function post(msg) {
    iframe.contentWindow.postMessage(msg, '*');
  }

  window.addEventListener('message', function(event) {
    if (event.source !== iframe.contentWindow) return;
    var msg = event.data;
    if (!msg || typeof msg.type !== 'string') return;

    if (msg.type === 'LANGSMITH_READY') {
      post({ type: 'LANGSMITH_THEME', cssText: ':root {}' });
      post({ type: 'LANGSMITH_DATA', payload: data });
    }

    if (msg.type === 'LANGSMITH_HEIGHT') {
      // Local preview always fills the viewport; no resize needed.
    }

    if (msg.type === 'LS_MUTATION') {
      console.log('[langsmith apps dev] setData (not persisted locally):', msg.patch);
    }

    if (msg.type === 'LS_API') {
      fetch('/__ls_dev/call', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ operation: msg.operation, args: msg.args || {} }),
      })
        .then(function(r) {
          return r.text().then(function(text) {
            var parsed;
            try { parsed = text ? JSON.parse(text) : null; } catch (e) { parsed = text; }
            if (!r.ok) throw new Error(typeof parsed === 'string' ? parsed : JSON.stringify(parsed));
            return parsed;
          });
        })
        .then(function(result) {
          post({ type: 'LS_API_RESPONSE', callId: msg.callId, result: result });
        })
        .catch(function(err) {
          post({ type: 'LS_API_RESPONSE', callId: msg.callId, error: String((err && err.message) || err) });
        });
    }
  });
__QUEUE_BAR_SCRIPT__
  // Poll for the entrypoint changing (or appearing for the first time) and
  // reload the whole page -- simplest reliable way to pick up a rebuild.
  var seen = false, lastKey = null;
  setInterval(function() {
    fetch('/__ls_dev/mtime', { cache: 'no-store' })
      .then(function(r) { return r.json(); })
      .then(function(j) {
        var key = j.exists ? String(j.mtime) : 'missing';
        if (!seen) { seen = true; lastKey = key; return; }
        if (key !== lastKey) { location.reload(); }
      })
      .catch(function() {});
  }, 500);
})();
</script>
</body>
</html>`

// sandboxInnerHTMLTemplate is a close Go port of smith-frontend's
// sandbox.ts buildSandboxSrcdoc — same virtual filesystem + require()
// loader, same render(data, root) contract, same window.langsmith bridge
// (call/setData/feedback.create). Kept in sync by hand; there is no shared
// source between the two repos.
const sandboxInnerHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style id="ls-theme">/* theme injected via postMessage */</style>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Inter', 'Segoe UI', system-ui, sans-serif;
  background: var(--bg-surface-level-1, #ffffff);
  color: var(--text-primary, #0f172a);
  font-size: 14px;
  line-height: 1.5;
}
pre, code {
  font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', ui-monospace, monospace;
}
</style>
</head>
<body>
<div id="root"></div>
<script>
(function() {
  var themeReady = false;
  var currentData = null;

  var FILES = __FILES_JSON__;

  var moduleCache = {};

  function makeRequire(fromPath) {
    return function require(id) {
      var resolved = id;
      if (id.charAt(0) === '.') {
        var parts = fromPath.split('/');
        parts.pop();
        id.split('/').forEach(function(seg) {
          if (seg === '..') parts.pop();
          else if (seg !== '.') parts.push(seg);
        });
        resolved = parts.join('/');
      }
      if (moduleCache[resolved]) return moduleCache[resolved].exports;
      var src = FILES[resolved];
      if (src === undefined) throw new Error('Module not found: ' + resolved);
      var mod = { exports: {} };
      moduleCache[resolved] = mod;
      var fn = new Function('require', 'module', 'exports', src);
      fn(makeRequire(resolved), mod, mod.exports);
      return mod.exports;
    };
  }

  var bootErrorMessage = null;

  function bootRenderer() {
    moduleCache = {};
    bootErrorMessage = null;
    var entrypoint = __ENTRYPOINT_JSON__;
    try {
      var main = makeRequire(entrypoint)(entrypoint);
      window.__render = main.render || (main.default && main.default.render);
    } catch (bootErr) {
      window.__render = null;
      bootErrorMessage = String((bootErr && bootErr.stack) || bootErr);
    }
  }

  window.addEventListener('message', function(event) {
    var msg = event.data;
    if (!msg || typeof msg.type !== 'string') return;

    if (msg.type === 'LANGSMITH_THEME') {
      document.getElementById('ls-theme').textContent = msg.cssText;
      themeReady = true;
      if (currentData !== null) renderApp();
    }
    if (msg.type === 'LANGSMITH_DATA') {
      if (JSON.stringify(msg.payload) !== JSON.stringify(currentData)) {
        currentData = msg.payload;
        if (themeReady) renderApp();
      }
    }
  });

  function renderApp() {
    var root = document.getElementById('root');
    if (bootErrorMessage) {
      root.innerHTML =
        '<div style="background:#fef2f2;border:1px solid #fda29b;border-radius:8px;padding:12px;margin:16px;color:#b91c1c;font-size:13px;white-space:pre-wrap;">' +
        '<strong>Boot error:</strong><br>' + escapeHtml(bootErrorMessage) + '</div>';
      reportHeight();
      return;
    }
    if (typeof window.__render !== 'function') {
      root.innerHTML = '<div style="padding:16px;color:#94a3b8;font-style:italic;font-size:13px;">Renderer not ready.</div>';
      reportHeight();
      return;
    }
    try {
      window.__render(currentData, root);
    } catch (err) {
      root.innerHTML =
        '<div style="background:#fef2f2;border:1px solid #fda29b;border-radius:8px;padding:12px;margin:16px;color:#b91c1c;font-size:13px;">' +
        '<strong>Renderer error:</strong><br>' + escapeHtml(String(err.message ?? err)) + '</div>';
    }
    reportHeight();
  }

  function reportHeight() {
    window.parent.postMessage({ type: 'LANGSMITH_HEIGHT', height: document.documentElement.scrollHeight }, '*');
  }

  function escapeHtml(str) {
    return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  if (window.ResizeObserver) {
    new ResizeObserver(reportHeight).observe(document.body);
  }

  function call(operation, args) {
    return new Promise(function(resolve, reject) {
      var callId = 'ls_' + String(Date.now()) + '_' + String((Math.random() * 1e9) | 0);
      function handler(e) {
        if (!e.data || e.data.type !== 'LS_API_RESPONSE' || e.data.callId !== callId) return;
        window.removeEventListener('message', handler);
        clearTimeout(timer);
        if (e.data.error) reject(new Error(e.data.error)); else resolve(e.data.result);
      }
      window.addEventListener('message', handler);
      var timer = setTimeout(function() {
        window.removeEventListener('message', handler);
        reject(new Error('LangSmith API call timed out'));
      }, 10000);
      window.parent.postMessage({ type: 'LS_API', operation: operation, callId: callId, args: args || {} }, '*');
    });
  }

  function setData(patch) {
    window.parent.postMessage({ type: 'LS_MUTATION', patch: patch }, '*');
    reportHeight();
  }

  window.langsmith = {
    call: call,
    setData: setData,
    feedback: {
      create: function(args) { return call('POST /api/v1/feedback', { body: args }); }
    }
  };

  bootRenderer();
  window.parent.postMessage({ type: 'LANGSMITH_READY' }, '*');
})();
</script>
</body>
</html>`
