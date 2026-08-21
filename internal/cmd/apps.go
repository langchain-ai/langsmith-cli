package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	appsLinkDir  = ".langsmith"
	appsLinkFile = "app.json"
)

const (
	appsMaxFileEntries   = 500
	appsMaxFileSizeBytes = 1 << 20 // 1MB per file, matches hub push's limit.
)

// appLink is the contents of <dir>/.langsmith/app.json.
type appLink struct {
	AppID string `json:"app_id"`
	Name  string `json:"name"`
}

// Custom app visibility tiers.
const (
	appScopeWorkspace = "workspace"
	appScopeOrg       = "org"
)

// customApp mirrors smith-go's /v1/platform/custom-apps JSON shape.
type customApp struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id,omitempty"`
	OrganizationID string            `json:"organization_id,omitempty"`
	Scope          string            `json:"scope,omitempty"`
	Name           string            `json:"name"`
	Description    *string           `json:"description,omitempty"`
	Files          map[string]string `json:"files,omitempty"`
	Entrypoint     string            `json:"entrypoint"`
	IsEnabled      bool              `json:"is_enabled"`
	CreatedAt      string            `json:"created_at,omitempty"`
	UpdatedAt      string            `json:"updated_at,omitempty"`
	CreatedBy      *string           `json:"created_by,omitempty"`
}

// tier reports the visibility tier.
// Org apps have no tenant_id.
func (a customApp) tier() string {
	if scope := strings.ToLower(a.Scope); scope != "" {
		if strings.HasPrefix(scope, "org") {
			return appScopeOrg
		}
		return appScopeWorkspace
	}
	if a.TenantID == "" && a.OrganizationID != "" {
		return appScopeOrg
	}
	return appScopeWorkspace
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

Pick a starter with --template (blank, annotation-queue,
annotation-queue-grid).

Examples:
  langsmith apps init --name my-app
  langsmith apps init --name my-queue-app --template annotation-queue
  langsmith apps dev
  langsmith apps push
  langsmith apps pull my-app
  langsmith apps list
  langsmith apps delete my-app --yes

"init" and "pull" create a new directory named after the app. Except
init/pull/list/delete, these act on the current directory — cd into your
app's directory first.

Apps shared with your organization are visible from every workspace in it.
Workspace apps win over org apps when both use the same name.`,
	}
	cmd.AddCommand(newAppsInitCmd())
	cmd.AddCommand(newAppsDevCmd())
	cmd.AddCommand(newAppsPushCmd())
	cmd.AddCommand(newAppsPullCmd())
	cmd.AddCommand(newAppsListCmd())
	cmd.AddCommand(newAppsShareCmd())
	cmd.AddCommand(newAppsClaimCmd())
	cmd.AddCommand(newAppsDeleteCmd())
	return cmd
}

func customAppWebURL(apiURL, workspaceID, appID string) string {
	if workspaceID == "" || appID == "" {
		return ""
	}
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(u.Host, "api.")
	host = strings.Replace(host, ".api.", ".", 1)
	if !strings.HasSuffix(host, "smith.langchain.com") {
		return ""
	}
	return u.Scheme + "://" + host + "/o/" + workspaceID + "/custom-apps/" + appID
}

// langsmithWebOrigin maps an API URL to its UI origin.
func langsmithWebOrigin(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(u.Host, "api.")
	host = strings.Replace(host, ".api.", ".", 1)
	return u.Scheme + "://" + host
}

// readAppLink returns (nil, nil) if not linked yet.
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

// readDirectoryAsAppFiles walks root into a relative-path → content map.
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
