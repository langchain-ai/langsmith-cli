package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/langchain-ai/langsmith-cli/internal/client"
	"github.com/langchain-ai/langsmith-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAppsPullCmd() *cobra.Command {
	var (
		force bool
		scope string
	)

	cmd := &cobra.Command{
		Use:   "pull APP_ID_OR_NAME",
		Short: "Download a custom app's source into a new directory",
		Long: `Download the source a previous "langsmith apps push" uploaded.

Takes an app ID or name, creates a directory named after the app in the
current directory, and extracts the source into it. Apps pushed before
source upload existed have no stored source — re-push them first.

The target directory is replaced, not merged. Run "npm install" after.

A name matches this workspace's own apps before those shared with its
organization. Pass --scope to force one tier.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]
			if err := validateAppScope(scope); err != nil {
				return err
			}
			c := MustGetClient()
			ctx := cmd.Context()

			app, err := resolveCustomAppInScope(ctx, c, nameOrID, scope)
			if err != nil {
				return err
			}

			archive, err := c.FetchCustomAppSource(ctx, app.ID)
			if err != nil {
				return customAppSourceError(err, app.Name)
			}

			dirName := slugifyAppName(app.Name)
			if dirName == "" {
				dirName = app.ID
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			dest := filepath.Join(cwd, dirName)
			if err := confirmSourceDirReplace(cmd, dest, force); err != nil {
				return err
			}
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("clearing %s: %w", dest, err)
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", dest, err)
			}

			written, err := extractSourceArchive(dest, archive)
			if err != nil {
				return err
			}
			if len(written) == 0 {
				return fmt.Errorf("the stored source archive for %q is empty", app.Name)
			}
			if err := writeAppLink(dest, appLink{AppID: app.ID, Name: app.Name}); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Pulled %q into %s (%d files).\n", app.Name, dest, len(written))
			fmt.Fprintf(os.Stderr, "Next: cd %s && npm install && langsmith apps dev.\n", dirName)
			output.OutputJSON(map[string]any{
				"status": "pulled",
				"app_id": app.ID,
				"name":   app.Name,
				"scope":  app.tier(),
				"dir":    dest,
				"files":  written,
			}, "")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Replace the target directory without confirming")
	cmd.Flags().StringVar(&scope, "scope", "", "Only match apps at this tier: \"workspace\" or \"org\"")
	return cmd
}

// resolveCustomApp accepts an app ID or name.
func resolveCustomApp(ctx context.Context, c *client.Client, nameOrID string) (*customApp, error) {
	return resolveCustomAppInScope(ctx, c, nameOrID, "")
}

// resolveCustomAppInScope filters by tier.
// Workspace tier is preferred.
func resolveCustomAppInScope(ctx context.Context, c *client.Client, nameOrID, scope string) (*customApp, error) {
	var apps []customApp
	if err := c.RawGet(ctx, c.CustomAppsPath(), &apps); err != nil {
		return nil, fmt.Errorf("looking up custom app %q: %w", nameOrID, err)
	}

	tiers := appScopeSearchOrder(scope)

	if _, err := uuid.Parse(nameOrID); err == nil {
		for _, tier := range tiers {
			for i := range apps {
				if apps[i].ID == nameOrID && apps[i].tier() == tier {
					return &apps[i], nil
				}
			}
		}
		return nil, fmt.Errorf("custom app %s not found %s (run `langsmith apps list`)", nameOrID, appScopeSearchedIn(scope))
	}

	// Nearest scope wins.
	for _, tier := range tiers {
		for i := range apps {
			if apps[i].Name == nameOrID && apps[i].tier() == tier {
				return &apps[i], nil
			}
		}
	}
	for _, tier := range tiers {
		for i := range apps {
			if strings.EqualFold(apps[i].Name, nameOrID) && apps[i].tier() == tier {
				return &apps[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no custom app named %q %s (run `langsmith apps list`)", nameOrID, appScopeSearchedIn(scope))
}

// appScopeSearchOrder lists tiers, nearest first.
func appScopeSearchOrder(scope string) []string {
	switch scope {
	case appScopeOrg:
		return []string{appScopeOrg}
	case appScopeWorkspace:
		return []string{appScopeWorkspace}
	default:
		return []string{appScopeWorkspace, appScopeOrg}
	}
}

// appScopeSearchedIn describes the tiers searched.
func appScopeSearchedIn(scope string) string {
	switch scope {
	case appScopeOrg:
		return "among the apps shared with this organization"
	case appScopeWorkspace:
		return "in this workspace"
	default:
		return "in this workspace or shared with its organization"
	}
}

// validateAppScope guards the --scope value.
func validateAppScope(scope string) error {
	switch scope {
	case "", appScopeOrg, appScopeWorkspace:
		return nil
	default:
		return fmt.Errorf("invalid --scope %q: must be %q or %q", scope, appScopeOrg, appScopeWorkspace)
	}
}

func customAppSourceError(err error, name string) error {
	msg := strings.ToLower(err.Error())
	switch {
	case client.IsNotFound(err) && strings.Contains(msg, "no source archive"):
		return fmt.Errorf("custom app %q has no stored source — it was pushed before source upload existed. Ask whoever owns it to run `langsmith apps push` again, then retry", name)
	case client.IsNotFound(err):
		return fmt.Errorf("custom app %q not found (run `langsmith apps list`)", name)
	case client.IsBadRequest(err):
		return fmt.Errorf("the server rejected this app ID: %w", err)
	default:
		return fmt.Errorf("downloading source for %q: %w", name, err)
	}
}

// confirmSourceDirReplace gates the destructive replace.
func confirmSourceDirReplace(cmd *cobra.Command, dest string, force bool) error {
	info, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", dest, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", dest)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dest, err)
	}
	if len(entries) == 0 || force {
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s already exists and is not empty. Replace its contents? [y/N] ", dest)
	var confirm string
	_, _ = fmt.Fscanln(cmd.InOrStdin(), &confirm)
	if ans := strings.ToLower(strings.TrimSpace(confirm)); ans != "y" && ans != "yes" {
		return errors.New("aborted: pass --force to replace the directory")
	}
	return nil
}

// extractSourceArchive gunzips and untars into dest.
func extractSourceArchive(dest string, archive []byte) ([]string, error) {
	if err := validateSourceArchivePaths(dest, archive); err != nil {
		return nil, err
	}

	tr, closeFn, err := newSourceArchiveReader(archive)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	var (
		written []string
		total   int64
	)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading source archive: %w", err)
		}
		path, ok, err := safeSourceArchivePath(dest, hdr.Name)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return nil, fmt.Errorf("creating %s: %w", hdr.Name, err)
			}
		case tar.TypeReg:
			total += hdr.Size
			if total > appsMaxSourceRawBytes {
				return nil, fmt.Errorf("source archive expands beyond %s; refusing to extract", formatArchiveBytes(appsMaxSourceRawBytes))
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, fmt.Errorf("creating directory for %s: %w", hdr.Name, err)
			}
			perm := hdr.FileInfo().Mode().Perm()
			if perm == 0 {
				perm = 0o644
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
			if err != nil {
				return nil, fmt.Errorf("creating %s: %w", hdr.Name, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return nil, fmt.Errorf("writing %s: %w", hdr.Name, err)
			}
			if err := f.Close(); err != nil {
				return nil, fmt.Errorf("writing %s: %w", hdr.Name, err)
			}
			written = append(written, filepath.ToSlash(hdr.Name))
		default:
			// Symlinks and devices are skipped.
			continue
		}
	}
	sort.Strings(written)
	return written, nil
}

// validateSourceArchivePaths rejects traversal before writing anything.
func validateSourceArchivePaths(dest string, archive []byte) error {
	tr, closeFn, err := newSourceArchiveReader(archive)
	if err != nil {
		return err
	}
	defer closeFn()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading source archive: %w", err)
		}
		if _, _, err := safeSourceArchivePath(dest, hdr.Name); err != nil {
			return err
		}
	}
}

func newSourceArchiveReader(archive []byte) (*tar.Reader, func(), error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, nil, fmt.Errorf("the stored source archive is not a valid gzip file: %w", err)
	}
	return tar.NewReader(gz), func() { gz.Close() }, nil
}

// safeSourceArchivePath resolves an entry inside dest.
func safeSourceArchivePath(dest, name string) (string, bool, error) {
	slash := filepath.ToSlash(name)
	if slash == "" || slash == "." || slash == "./" {
		return "", false, nil
	}
	if strings.HasPrefix(slash, "/") || filepath.IsAbs(name) || strings.Contains(name, `\`) {
		return "", false, fmt.Errorf("source archive entry %q is not a relative path; refusing to extract", name)
	}
	if !filepath.IsLocal(slash) {
		return "", false, fmt.Errorf("source archive entry %q would escape the target directory; refusing to extract", name)
	}
	clean := filepath.Clean(filepath.FromSlash(slash))
	for _, segment := range strings.Split(filepath.ToSlash(clean), "/") {
		if segment == ".." {
			return "", false, fmt.Errorf("source archive entry %q would escape the target directory; refusing to extract", name)
		}
	}
	path := filepath.Join(dest, clean)
	if path != dest && !strings.HasPrefix(path, dest+string(filepath.Separator)) {
		return "", false, fmt.Errorf("source archive entry %q would escape the target directory; refusing to extract", name)
	}
	return path, true, nil
}
