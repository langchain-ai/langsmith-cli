package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// appsLinkDir and appsLinkFile name the local file that records which
// remote custom app a directory is linked to, mirroring how `vercel link`/
// `netlify link` avoid needing a server-side unique name to upsert against.
const (
	appsLinkDir  = ".langsmith"
	appsLinkFile = "app.json"
)

const (
	appsMaxFileEntries   = 500
	appsMaxFileSizeBytes = 1 << 20 // 1MB per file, matches hub push's limit.
)

// appLink is the contents of <dir>/.langsmith/app.json. It is the durable
// record of "what this app is" that `apps push` reads on every run to decide
// whether to create a new custom app or update the one it created last time.
type appLink struct {
	AppID string `json:"app_id"`
	Name  string `json:"name"`
}

// customApp mirrors the JSON shape returned by smith-go's
// /v1/platform/custom-apps endpoints (smith-go/customapps/types.go).
type customApp struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id,omitempty"`
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	ContextType string            `json:"context_type"`
	Files       map[string]string `json:"files,omitempty"`
	Entrypoint  string            `json:"entrypoint"`
	IsEnabled   bool              `json:"is_enabled"`
	CreatedAt   string            `json:"created_at,omitempty"`
	UpdatedAt   string            `json:"updated_at,omitempty"`
	CreatedBy   *string           `json:"created_by,omitempty"`
}

var appsExcludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".langsmith":   true,
	".cache":       true,
	".next":        true,
}

var appsExcludedFiles = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
}

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Build, upload, and manage Custom Apps",
		Long: `Build, upload, and manage Custom Apps — UIs you build locally against the
LangSmith API and run inside LangSmith.

A custom app is a small set of files rendered in a locked-down sandbox: no
direct network access, no npm modules at runtime — only a postMessage
bridge (window.langsmith.call/setData) proxied through the viewer's own
LangSmith session. Use React/JSX or npm dependencies freely; the scaffolded
starter bundles them locally (Vite) into the single dependency-free file the
sandbox expects.

Pick a starter with --template (blank, annotation-queue,
annotation-queue-grid); they differ only in the UI they scaffold — every app
is uploaded and rendered the same way.

Examples:
  langsmith apps init --name my-app
  langsmith apps init --name my-queue-app --template annotation-queue
  langsmith apps dev
  langsmith apps push
  langsmith apps list
  langsmith apps delete <app-id> --yes

All of the above (except list/delete, which take an app ID) act on the
current directory — cd into your app's directory first.`,
	}
	cmd.AddCommand(newAppsInitCmd())
	cmd.AddCommand(newAppsDevCmd())
	cmd.AddCommand(newAppsPushCmd())
	cmd.AddCommand(newAppsListCmd())
	cmd.AddCommand(newAppsDeleteCmd())
	return cmd
}

// readAppLink reads <dir>/.langsmith/app.json. It returns (nil, nil) if the
// file does not exist — callers treat that as "not linked yet".
func readAppLink(dir string) (*appLink, error) {
	path := filepath.Join(dir, appsLinkDir, appsLinkFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var link appLink
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &link, nil
}

// writeAppLink writes <dir>/.langsmith/app.json, creating the directory if
// needed.
func writeAppLink(dir string, link appLink) error {
	linkDir := filepath.Join(dir, appsLinkDir)
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", linkDir, err)
	}
	data, err := json.MarshalIndent(link, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling app link: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(linkDir, appsLinkFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// readDirectoryAsAppFiles walks root and returns a flat relative-path →
// content map suitable for custom_apps.files. Unlike hub's equivalent
// (readDirectoryAsFiles in hub_push.go), there is no {type, content}
// wrapper — the custom-apps API takes a plain map[string]string — and
// .langsmith/ itself is always excluded so the link file never gets
// uploaded as an app file.
func readDirectoryAsAppFiles(root string) (map[string]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	files := make(map[string]string)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.IsDir() {
			if appsExcludedDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if appsExcludedFiles[base] || strings.HasPrefix(base, ".env") {
			return nil
		}

		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		if len(data) > appsMaxFileSizeBytes {
			return fmt.Errorf("file %s is %d bytes; exceeds %d-byte limit", rel, len(data), appsMaxFileSizeBytes)
		}
		if isAppFileBinary(data) {
			return fmt.Errorf("file %s appears to be binary; custom apps do not support binary assets yet", rel)
		}

		files[rel] = string(data)
		if len(files) > appsMaxFileEntries {
			return fmt.Errorf("directory contains more than %d files; reduce before pushing", appsMaxFileEntries)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func isAppFileBinary(data []byte) bool {
	n := len(data)
	if n > 512 {
		n = 512
	}
	return bytes.IndexByte(data[:n], 0) != -1
}
