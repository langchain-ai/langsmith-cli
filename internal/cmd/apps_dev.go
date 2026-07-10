package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
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
		dataPath   string
		noOpen     bool
	)

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the current directory's custom app locally in your browser",
		Long: `Serve the current directory's custom app in a local HTML page and open it
in your browser — entirely self-contained, no LangSmith involved.

Pair this with your build's watch mode (e.g. "npm run watch") running in
another terminal: the page polls for changes to the entrypoint file and
reloads itself automatically after each rebuild.

The page provides a stub window.langsmith bridge so render(data, root) can
run without crashing: setData/data.updateInputs/updateOutputs update the
local data in place and re-render, but call() (and anything routed through
it, like feedback.create) is a no-op that only logs to the console — there's
no LangSmith backend here to call. Pass --data to seed render() with sample
inputs/outputs instead of an empty object.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			link, err := readAppLink(dir)
			if err != nil {
				return err
			}

			data := defaultDevData(link)
			if dataPath != "" {
				data, err = loadDevDataOverride(dataPath)
				if err != nil {
					return err
				}
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			return runAppsDev(ctx, dir, entrypoint, data, noOpen)
		},
	}

	cmd.Flags().StringVar(&entrypoint, "entrypoint", "dist/bundle.js", "Path (relative to the current directory) of the file to serve and render")
	cmd.Flags().StringVar(&dataPath, "data", "", "Path to a JSON file with sample data to pass to render() (default: {} or, for a linked annotation_queue app, {inputs:{},outputs:{}})")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Print the local URL instead of opening a browser")
	return cmd
}

// defaultDevData picks a sample render() payload shape from the linked
// app's context_type, when known — an annotation-queue app's entrypoint
// expects {inputs, outputs}, everything else gets {}.
const contextTypeAnnotationQueue = "annotation_queue"

func defaultDevData(link *appLink) any {
	if link != nil && link.ContextType == contextTypeAnnotationQueue {
		return map[string]any{"inputs": map[string]any{}, "outputs": map[string]any{}}
	}
	return map[string]any{}
}

func loadDevDataOverride(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --data %s: %w", path, err)
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parsing --data %s: %w", path, err)
	}
	return data, nil
}

// runAppsDev starts the local preview server for entrypoint (relative to
// dir), opens it in a browser, and blocks until ctx is cancelled (e.g. by
// SIGINT) or the server fails.
func runAppsDev(ctx context.Context, dir, entrypoint string, data any, noOpen bool) error {
	if _, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(entrypoint))); statErr != nil {
		fmt.Fprintf(os.Stderr, "note: entrypoint %q does not exist yet in %s — start your build/watch command (e.g. \"npm run watch\"); the preview will pick it up and reload automatically once it appears\n", entrypoint, dir)
	}

	srv, ln, previewURL, entrypointURL, err := prepareAppsDevServer(dir, entrypoint, data)
	if err != nil {
		return err
	}

	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve(ln) }()

	if !noOpen {
		_ = openBrowser(previewURL)
	}
	output.OutputJSON(map[string]any{
		"status":         "serving",
		"url":            previewURL,
		"entrypoint_url": entrypointURL,
	}, "")
	fmt.Fprintf(os.Stderr, "Serving %s at %s — press Ctrl+C to stop\n", dir, previewURL)

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

// prepareAppsDevServer builds (but does not start) an HTTP server, bound to
// an OS-assigned free port on 127.0.0.1, that serves:
//
//   - "/"                     — a self-contained HTML harness that fetches
//     the entrypoint, evaluates it as a CJS module, and calls its
//     render(data, root) export.
//   - "/<entrypoint path>"    — the raw entrypoint file, re-read from disk
//     on every request (no caching), so a rebuild is visible immediately.
//   - "/__ls_dev/mtime"       — polled by the harness page to detect a
//     rebuild (or the entrypoint appearing for the first time) and trigger
//     an automatic reload.
//
// It deliberately serves only the entrypoint file, not the rest of dir: a
// directory-wide static server would expose .env, .git, node_modules, etc.
// to any local process that guesses the port.
func prepareAppsDevServer(dir, entrypoint string, data any) (srv *http.Server, ln net.Listener, previewURL, entrypointURL string, err error) {
	entrypointAbs := filepath.Join(dir, filepath.FromSlash(entrypoint))
	entrypointPath := "/" + strings.TrimPrefix(filepath.ToSlash(entrypoint), "/")

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("marshaling sample data: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(entrypointPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, entrypointAbs)
	})
	mux.HandleFunc("/__ls_dev/mtime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		info, statErr := os.Stat(entrypointAbs)
		if statErr != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"exists": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"exists": true, "mtime": info.ModTime().UnixNano()})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderDevHTML(filepath.Base(dir), entrypoint, entrypointPath, dataJSON)))
	})

	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("starting local server: %w", err)
	}

	origin := "http://" + ln.Addr().String()
	return &http.Server{Handler: mux}, ln, origin + "/", origin + entrypointPath, nil
}

var scriptCloseTagPattern = regexp.MustCompile(`(?i)</script>`)

func renderDevHTML(appName, entrypointDisplay, entrypointPath string, dataJSON []byte) string {
	safeDataJSON := scriptCloseTagPattern.ReplaceAllString(string(dataJSON), "<\\/script>")
	entrypointPathJSON, _ := json.Marshal(entrypointPath)

	return fmt.Sprintf(devHTMLTemplate, appName, entrypointDisplay, string(entrypointPathJSON), safeDataJSON)
}

const devHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — local preview</title>
<style>
  html, body { height: 100%%; margin: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: #0f172a; }
  #ls-dev-banner { background: #111827; color: #9ca3af; font-size: 12px; padding: 6px 12px; border-bottom: 1px solid #1f2937; }
  #ls-dev-banner code { color: #e5e7eb; }
  #root { padding: 16px; }
  .ls-dev-error { background: #fef2f2; border: 1px solid #fda29b; border-radius: 8px; padding: 12px; color: #b91c1c; font-size: 13px; white-space: pre-wrap; }
</style>
</head>
<body>
<div id="ls-dev-banner">Local preview of <code>%s</code> — save your code; this page reloads automatically after each rebuild.</div>
<div id="root"></div>
<script>
(function() {
  var ENTRYPOINT_PATH = %s;
  var data = %s;
  var root = document.getElementById('root');
  var renderFn = null;

  function showError(label, err) {
    root.innerHTML = '';
    var pre = document.createElement('pre');
    pre.className = 'ls-dev-error';
    pre.textContent = label + ':\n' + String((err && err.stack) || err);
    root.appendChild(pre);
  }

  function renderApp() {
    if (typeof renderFn !== 'function') {
      root.innerHTML = '<p style="color:#94a3b8;font-style:italic;">Waiting for a render() export…</p>';
      return;
    }
    root.innerHTML = '';
    try {
      renderFn(data, root);
    } catch (err) {
      showError('Renderer error', err);
    }
  }

  window.langsmith = {
    call: function(method, args) {
      console.warn('[langsmith apps dev] window.langsmith.call(' + method + ') is a local no-op — push the app and open it inside LangSmith to exercise real API calls.', args);
      return Promise.resolve(null);
    },
    setData: function(patch) {
      data = Object.assign({}, data, patch);
      renderApp();
    },
    feedback: {
      create: function(args) { return window.langsmith.call('feedback.create', args); }
    },
    data: {
      updateInputs: function(v) { window.langsmith.setData({ inputs: v }); },
      updateOutputs: function(v) { window.langsmith.setData({ outputs: v }); }
    }
  };

  function boot() {
    fetch(ENTRYPOINT_PATH, { cache: 'no-store' })
      .then(function(r) {
        if (!r.ok) throw new Error('fetching ' + ENTRYPOINT_PATH + ' failed: HTTP ' + r.status);
        return r.text();
      })
      .then(function(src) {
        var mod = { exports: {} };
        try {
          new Function('module', 'exports', 'require', src)(mod, mod.exports, function(id) {
            throw new Error('require(' + JSON.stringify(id) + ') is not supported in local dev — bundle your dependencies first (the starter template does this with esbuild)');
          });
        } catch (evalErr) {
          showError('Boot error', evalErr);
          return;
        }
        renderFn = mod.exports.render || (mod.exports.default && mod.exports.default.render);
        renderApp();
      })
      .catch(function(err) { showError('Load error', err); });
  }

  boot();

  // Poll for the entrypoint changing (or appearing for the first time) and
  // reload the whole page — simplest reliable way to pick up a rebuild.
  var seen = false;
  var lastKey = null;
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
