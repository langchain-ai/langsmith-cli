package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
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

	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

// appsDevLogLevel controls how much of the running app the dev server streams
// to the terminal. Set once from flags before the server starts.
type appsDevLogLevel int

const (
	logErrors  appsDevLogLevel = iota // failed API calls + app errors only (default)
	logQuiet                          // nothing from the app, build output only
	logVerbose                        // all API calls + all console output
)

var appsDevLogMode = logErrors

func newAppsDevCmd() *cobra.Command {
	var (
		entrypoint string
		noOpen     bool
		quiet      bool
		verbose    bool
	)

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the current directory's custom app locally in a real sandbox",
		Long: `Preview the current directory's custom app locally in your browser, in
the same kind of sandbox it runs in on LangSmith. Fully self-contained — no
LangSmith web app involved.

API calls the app makes are proxied through your local credentials.

Rebuilds automatically on save when package.json has a "watch" script.

The app's failed API calls and errors stream to this terminal so problems
show up without opening browser devtools. Use --verbose to also see every
successful call and all console output, or --quiet to silence app output
entirely (build output stays).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if quiet && verbose {
				return fmt.Errorf("--quiet and --verbose cannot be used together")
			}
			switch {
			case quiet:
				appsDevLogMode = logQuiet
			case verbose:
				appsDevLogMode = logVerbose
			default:
				appsDevLogMode = logErrors
			}

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}

			c, err := getClient()
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			return runAppsDev(ctx, c, dir, entrypoint, noOpen)
		},
	}

	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to render")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the local URL instead of opening a browser")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Silence the app's console output and API call logging (build output still shows)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Also log successful API calls and all console output, not just errors")
	return cmd
}

func runAppsDev(ctx context.Context, c *client.Client, dir, entrypoint string, noOpen bool) error {
	srv, ln, previewURL, err := prepareAppsDevServer(c, dir, entrypoint)
	if err != nil {
		return err
	}

	entrypointPath := filepath.Join(dir, filepath.FromSlash(entrypoint))
	var prevBuildTime time.Time
	if info, statErr := os.Stat(entrypointPath); statErr == nil {
		prevBuildTime = info.ModTime()
	}
	if startWatchProcess(ctx, dir) {
		// Build tools empty their output dir before rebuilding, which would
		// briefly hide an existing entrypoint — wait for a fresh build first.
		waitForFreshEntrypoint(ctx, entrypointPath, prevBuildTime, 10*time.Second)
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
	fmt.Fprintln(os.Stderr, appsDevLogModeBanner())

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

// startWatchProcess runs package.json's "watch" script tied to ctx. Never
// fails runAppsDev — returns false (with a note) if it can't start one.
func startWatchProcess(ctx context.Context, dir string) bool {
	script, pkgJSONExists, err := packageJSONScript(dir, "watch")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: couldn't read package.json to find a \"watch\" script: %v\n", err)
		return false
	}
	if script == "" {
		if pkgJSONExists {
			fmt.Fprintln(os.Stderr, `note: no "watch" script in package.json — start your own build/watch process to see live updates`)
		}
		return false
	}
	if _, lookErr := exec.LookPath("npm"); lookErr != nil {
		fmt.Fprintln(os.Stderr, `note: npm not found on PATH — run "npm run watch" yourself to see live updates`)
		return false
	}

	fmt.Fprintln(os.Stderr, "Starting build watcher: npm run watch")
	watchCmd := exec.CommandContext(ctx, "npm", "run", "watch")
	watchCmd.Dir = dir
	watchCmd.Stdout = os.Stderr
	watchCmd.Stderr = os.Stderr
	if startErr := watchCmd.Start(); startErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to start \"npm run watch\": %v\n", startErr)
		return false
	}
	go func() { _ = watchCmd.Wait() }()
	return true
}

// waitForFreshEntrypoint blocks until path's mtime is newer than after, ctx
// is cancelled, or timeout elapses.
func waitForFreshEntrypoint(ctx context.Context, path string, after time.Time, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(after) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

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

// prepareAppsDevServer builds (but does not start) an HTTP server on
// 127.0.0.1 serving the sandboxed preview ("/"), a rebuild-poll endpoint
// ("/__ls_dev/mtime"), and the API proxy ("/__ls_dev/call").
func prepareAppsDevServer(c *client.Client, dir, entrypoint string) (srv *http.Server, ln net.Listener, previewURL string, err error) {
	token, err := newDevToken()
	if err != nil {
		return nil, nil, "", err
	}

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
		_, _ = w.Write([]byte(renderDevHostHTML(files, entrypoint, token)))
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

	mux.HandleFunc("/__ls_dev/call", makeLsDevCallHandler(c, token))

	mux.HandleFunc("/__ls_dev/log", makeLsDevLogHandler(token))

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

// maxLsDevCallBody caps proxied request bodies.
const maxLsDevCallBody = 16 << 20

// newDevToken returns the proxy's per-session shared secret.
func newDevToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating dev token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// makeLsDevCallHandler guards the credentialed proxy against cross-site use.
func makeLsDevCallHandler(c *client.Client, token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-LS-Dev-Token")), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "content-type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxLsDevCallBody)
		var req lsDevCallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		handleLsDevCall(c, w, r, req)
	}
}

// makeLsDevLogHandler receives console output and uncaught errors forwarded
// from the sandboxed app and prints them to stderr.
func makeLsDevLogHandler(token string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-LS-Dev-Token")), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxLsDevCallBody)
		var entry struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		level := entry.Level
		if level == "" {
			level = "log"
		}
		// quiet drops everything; only verbose keeps non-error console output.
		if appsDevLogMode == logQuiet || (appsDevLogMode != logVerbose && level != "error") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		fmt.Fprintf(os.Stderr, "[app %s] %s\n", level, entry.Message)
		w.WriteHeader(http.StatusNoContent)
	}
}

var proxyPathPattern = regexp.MustCompile(`^/[A-Za-z0-9/_-]*$`)

func handleLsDevCall(c *client.Client, w http.ResponseWriter, r *http.Request, req lsDevCallRequest) {

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
	pathPart, queryPart, hasQuery := strings.Cut(path, "?")
	if !proxyPathPattern.MatchString(pathPart) || strings.ContainsAny(pathPart, `%\`) || strings.Contains(pathPart, "..") {
		http.Error(w, fmt.Sprintf("path %q must be a relative path starting with \"/\"", path), http.StatusBadRequest)
		return
	}
	if len(req.Args.Params) > 0 {
		if hasQuery {
			queryPart += "&"
		}
		queryPart += encodeProxyParams(req.Args.Params)
		hasQuery = true
	}
	if hasQuery {
		path = pathPart + "?" + queryPart
	} else {
		path = pathPart
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

	status, _, respHeaders, respBody, err := c.RawDo(r.Context(), method, path, bodyReader, nil)
	if err != nil {
		if appsDevLogMode != logQuiet {
			fmt.Fprintf(os.Stderr, "[app api] %s %s → error: %s\n", method, path, err)
		}
		http.Error(w, "request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	logProxyCall(method, path, status, respBody)
	if ct := respHeaders.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(status)
	_, _ = w.Write(respBody)
}

// logProxyCall prints proxied API calls to stderr per the current log mode,
// appending an error summary for non-2xx responses so failures surface here.
func logProxyCall(method, path string, status int, body []byte) {
	if appsDevLogMode == logQuiet {
		return
	}
	if appsDevLogMode != logVerbose && status < 400 {
		return
	}
	line := fmt.Sprintf("[app api] %s %s → %d", method, path, status)
	if status >= 400 {
		msg := proxyErrorSummary(body)
		if msg == "" {
			msg = http.StatusText(status)
		}
		if msg != "" {
			line += " " + msg
		}
	}
	fmt.Fprintln(os.Stderr, line)
}

func proxyErrorSummary(body []byte) string {
	var parsed struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Message != "" {
			return parsed.Message
		}
		if parsed.Detail != "" {
			return parsed.Detail
		}
	}
	s := strings.TrimSpace(string(body))
	// HTML error pages (e.g. a proxy's 429) are noise — the status text says enough.
	if s == "" || strings.HasPrefix(s, "<") {
		return ""
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func appsDevLogModeBanner() string {
	switch appsDevLogMode {
	case logQuiet:
		return "App logging: off (--quiet) — build output only"
	case logVerbose:
		return "App logging: streaming all console output + API calls (--verbose)"
	default:
		return "App logging: errors only (--verbose for all calls + console, --quiet to silence)"
	}
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

// devWaitingHTML is served at "/" until the entrypoint exists, then reloads
// into the real preview.
func devWaitingHTML(message string) string {
	return `<!doctype html>
<html><head><meta charset="utf-8"><title>Waiting for build…</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,sans-serif;padding:24px;color:#334155;">
<p>` + html.EscapeString(message) + `</p>
<script>
// Served only while the entrypoint is missing, so reload as soon as the
// build produces it — no need to track changes, any existing bundle is an
// improvement over this page.
setInterval(function() {
  fetch('/__ls_dev/mtime', { cache: 'no-store' }).then(function(r){ return r.json(); }).then(function(j){
    if (j.exists) { location.reload(); }
  }).catch(function(){});
}, 500);
</script>
</body></html>`
}

// renderDevHostHTML builds the top-level host page: the sandboxed iframe
// plus the postMessage bridge and a Light/Dark mode toolbar.
func renderDevHostHTML(files map[string]string, entrypoint, token string) string {
	filesJSON, _ := json.Marshal(files)
	entrypointJSON, _ := json.Marshal(entrypoint)

	inner := strings.NewReplacer(
		"__FILES_JSON__", escapeForScript(filesJSON),
		"__ENTRYPOINT_JSON__", escapeForScript(entrypointJSON),
	).Replace(sandboxInnerHTMLTemplate)

	return strings.NewReplacer(
		"__SANDBOX_SRCDOC__", html.EscapeString(inner),
		"__LS_DEV_TOKEN__", token,
	).Replace(devHostHTMLTemplate)
}

const devHostHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Local sandboxed preview</title>
<style>
  html, body { height: 100%; margin: 0; }
  body { display: flex; flex-direction: column; font-family: -apple-system, BlinkMacSystemFont, 'Inter', 'Segoe UI', system-ui, sans-serif; }

  /* Toolbar renders for every app.
     ls-dev-dark mirrors the sandbox's html.dark onto the host chrome. */
  :root {
    --ls-dev-bar-bg: #f5f8fb;
    --ls-dev-bar-border: #e2e8f0;
    --ls-dev-bar-label: #334155;
    --ls-dev-control-bg: #ffffff;
    --ls-dev-control-border: #cbd5e1;
    --ls-dev-control-border-hover: #94a3b8;
    --ls-dev-control-text: #0f172a;
    --ls-dev-chevron: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%23475569' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
  }
  html.ls-dev-dark {
    --ls-dev-bar-bg: #111521;
    --ls-dev-bar-border: #282e42;
    --ls-dev-bar-label: #e2e8f0;
    --ls-dev-control-bg: #1b2030;
    --ls-dev-control-border: #393f55;
    --ls-dev-control-border-hover: #555d78;
    --ls-dev-control-text: #f5f8fb;
    --ls-dev-chevron: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%238790ab' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'/%3E%3C/svg%3E");
  }

  #ls-dev-toolbar {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
    background: var(--ls-dev-bar-bg);
    border-bottom: 1px solid var(--ls-dev-bar-border);
    padding: 8px 16px;
    font-size: 13px;
  }
  #ls-dev-toolbar-title {
    font-weight: 600;
    color: var(--ls-dev-control-text);
  }
  .ls-dev-toolbar-controls { display: flex; align-items: center; gap: 20px; }

  #ls-dev-mode-group {
    display: inline-flex;
    gap: 2px;
    background: var(--ls-dev-bar-border);
    border-radius: 7px;
    padding: 2px;
  }
  .ls-dev-mode-btn {
    font-family: inherit;
    font-size: 13px;
    color: var(--ls-dev-bar-label);
    background-color: transparent;
    border: none;
    border-radius: 5px;
    padding: 4px 12px;
    cursor: pointer;
    transition: background-color 120ms ease, color 120ms ease, box-shadow 120ms ease;
  }
  .ls-dev-mode-btn:hover { color: var(--ls-dev-control-text); }
  .ls-dev-mode-btn[aria-pressed="true"] {
    background-color: var(--ls-dev-control-bg);
    color: var(--ls-dev-control-text);
    box-shadow: 0 1px 2px rgba(15, 23, 42, 0.16);
  }
  .ls-dev-mode-btn:focus-visible {
    outline: none;
    box-shadow: 0 0 0 2px #006ddd;
  }

  iframe { display: block; width: 100%; border: none; flex: 1 1 auto; }
</style>
</head>
<body>
<div id="ls-dev-toolbar">
  <span id="ls-dev-toolbar-title">LangSmith Custom Apps — local preview below</span>
  <div class="ls-dev-toolbar-controls">
    <div id="ls-dev-mode-group" role="group" aria-label="Appearance">
      <button id="ls-dev-mode-light" type="button" class="ls-dev-mode-btn" aria-pressed="false">Light</button>
      <button id="ls-dev-mode-dark" type="button" class="ls-dev-mode-btn" aria-pressed="false">Dark</button>
    </div>
  </div>
</div>
<iframe id="ls-app" sandbox="allow-scripts" srcdoc="__SANDBOX_SRCDOC__"></iframe>
<script>
(function() {
  var iframe = document.getElementById('ls-app');
  // Apps get no host context: the sandbox always receives {} as render data.
  var data = {};

  function post(msg) {
    iframe.contentWindow.postMessage(msg, '*');
  }

  // mode defaults from the OS and persists to localStorage.
  var MODE_STORAGE_KEY = 'langsmith:apps-dev:mode';
  var lightBtn = document.getElementById('ls-dev-mode-light');
  var darkBtn = document.getElementById('ls-dev-mode-dark');

  function initialMode() {
    try {
      var saved = localStorage.getItem(MODE_STORAGE_KEY);
      if (saved === 'dark' || saved === 'light') return saved;
    } catch (e) {}
    return (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
  }
  var mode = initialMode();

  function applyModeToHost() {
    document.documentElement.classList.toggle('ls-dev-dark', mode === 'dark');
    if (lightBtn) lightBtn.setAttribute('aria-pressed', mode === 'light' ? 'true' : 'false');
    if (darkBtn) darkBtn.setAttribute('aria-pressed', mode === 'dark' ? 'true' : 'false');
  }

  function postMetadata() {
    post({ type: 'LANGSMITH_METADATA', metadata: { mode: mode } });
  }

  function setMode(next) {
    if (next === mode) return;
    mode = next;
    try { localStorage.setItem(MODE_STORAGE_KEY, mode); } catch (e) {}
    applyModeToHost();
    postMetadata();
  }

  applyModeToHost();
  if (lightBtn) lightBtn.addEventListener('click', function() { setMode('light'); });
  if (darkBtn) darkBtn.addEventListener('click', function() { setMode('dark'); });

  window.addEventListener('message', function(event) {
    if (event.source !== iframe.contentWindow) return;
    var msg = event.data;
    if (!msg || typeof msg.type !== 'string') return;

    if (msg.type === 'LANGSMITH_READY') {
      post({ type: 'LANGSMITH_THEME', cssText: ':root {}' });
      postMetadata();
      post({ type: 'LANGSMITH_DATA', payload: data });
    }

    if (msg.type === 'LANGSMITH_HEIGHT') {
      // Local preview always fills the viewport; no resize needed.
    }

    if (msg.type === 'LS_MUTATION') {
      console.log('[langsmith apps dev] setData (not persisted locally):', msg.patch);
    }

    if (msg.type === 'LS_LOG') {
      fetch('/__ls_dev/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-LS-Dev-Token': '__LS_DEV_TOKEN__' },
        body: JSON.stringify({ level: msg.level, message: msg.message }),
      }).catch(function() {});
    }

    if (msg.type === 'LS_API') {
      fetch('/__ls_dev/call', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-LS-Dev-Token': '__LS_DEV_TOKEN__' },
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

  // Poll for a rebuild and reload once the new bundle is actually on disk.
  // Build tools empty dist/ before rewriting the entrypoint, so a rebuild
  // briefly reports exists:false. Reloading during that window would land on
  // the "waiting for build" page (a white flash), so ignore the transient
  // missing state entirely and reload only when the entrypoint exists with a
  // newer mtime than the last one we saw it have.
  var lastMtime = null;
  setInterval(function() {
    fetch('/__ls_dev/mtime', { cache: 'no-store' })
      .then(function(r) { return r.json(); })
      .then(function(j) {
        if (!j.exists) return;
        var mtime = String(j.mtime);
        if (lastMtime === null) { lastMtime = mtime; return; }
        if (mtime !== lastMtime) { location.reload(); }
      })
      .catch(function() {});
  }, 500);
})();
</script>
</body>
</html>`

// sandboxInnerHTMLTemplate ports smith-frontend's sandbox.ts srcdoc — same
// require() loader, render contract, and window.langsmith bridge. Kept in
// sync by hand.
const sandboxInnerHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval'; style-src 'unsafe-inline'; img-src data: blob:; form-action 'none'; base-uri 'none';">
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
  var currentMetadata = null;

  // Forward console output and uncaught errors to the host so they stream
  // into the terminal running "langsmith apps dev".
  function reportLog(level, message) {
    try { window.parent.postMessage({ type: 'LS_LOG', level: level, message: message }, '*'); } catch (e) {}
  }
  function formatArg(a) {
    if (typeof a === 'string') return a;
    if (a instanceof Error) return String(a.stack || a.message || a);
    try { return JSON.stringify(a); } catch (e) { return String(a); }
  }
  ['log', 'info', 'warn', 'error', 'debug'].forEach(function(level) {
    var orig = console[level];
    console[level] = function() {
      reportLog(level, Array.prototype.map.call(arguments, formatArg).join(' '));
      if (orig) return orig.apply(console, arguments);
    };
  });
  window.addEventListener('error', function(e) {
    reportLog('error', String((e.error && e.error.stack) || e.message || 'uncaught error'));
  });
  window.addEventListener('unhandledrejection', function(e) {
    var r = e.reason;
    reportLog('error', 'Unhandled rejection: ' + String((r && r.stack) || (r && r.message) || r));
  });

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
      maybeRender();
    }
    if (msg.type === 'LANGSMITH_METADATA') {
      // Load-bearing: the sandboxed origin can't set html.dark itself.
      currentMetadata = msg.metadata;
      document.documentElement.classList.toggle('dark', currentMetadata && currentMetadata.mode === 'dark');
      maybeRender();
    }
    if (msg.type === 'LANGSMITH_DATA') {
      if (JSON.stringify(msg.payload) !== JSON.stringify(currentData)) {
        currentData = msg.payload;
        maybeRender();
      }
    }
  });

  // Renders once all three are present, and on any later change.
  function maybeRender() {
    if (themeReady && currentMetadata !== null && currentData !== null) renderApp();
  }

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
      window.__render(currentData, root, currentMetadata);
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
