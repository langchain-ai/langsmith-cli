package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const appsMaxSourceRawBytes = 1 << 30

// Backend cap on the decoded .tar.gz; var so tests can shrink it.
var appsMaxSourceArchiveBytes int64 = 50 << 20

// Never archived, whatever .gitignore says.
var appsSourceDenyDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	"dist":          true,
	"build":         true,
	"out":           true,
	"coverage":      true,
	".next":         true,
	".nuxt":         true,
	".svelte-kit":   true,
	".turbo":        true,
	".cache":        true,
	".parcel-cache": true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
}

var appsSourceDenyFiles = map[string]bool{
	".DS_Store":        true,
	"Thumbs.db":        true,
	".npmrc":           true,
	".netrc":           true,
	".pgpass":          true,
	".htpasswd":        true,
	"id_rsa":           true,
	"id_dsa":           true,
	"id_ecdsa":         true,
	"id_ed25519":       true,
	"credentials.json": true,
	"secrets.json":     true,
}

var appsSourceDenySuffixes = []string{
	".pem", ".key", ".p12", ".pfx", ".jks", ".keystore", ".ppk", ".local",
}

type sourceFile struct {
	rel  string
	size int64
	mode fs.FileMode
}

func deniedSourceFile(base string) bool {
	if appsSourceDenyFiles[base] || strings.HasPrefix(base, ".env") {
		return true
	}
	lower := strings.ToLower(base)
	for _, suffix := range appsSourceDenySuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// buildSourceArchive returns the base64 of a .tar.gz of root.
func buildSourceArchive(root string) (string, error) {
	files, err := collectSourceFiles(root)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		hdr := &tar.Header{
			Name:     f.rel,
			Mode:     int64(f.mode.Perm()),
			Size:     f.size,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return "", fmt.Errorf("archiving %s: %w", f.rel, err)
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.rel)))
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", f.rel, err)
		}
		// Guard against a file changing size mid-walk.
		if int64(len(data)) != f.size {
			return "", fmt.Errorf("%s changed while archiving; try again", f.rel)
		}
		if _, err := tw.Write(data); err != nil {
			return "", fmt.Errorf("archiving %s: %w", f.rel, err)
		}
	}
	if err := tw.Close(); err != nil {
		return "", fmt.Errorf("finalizing source archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("compressing source archive: %w", err)
	}

	if int64(buf.Len()) > appsMaxSourceArchiveBytes {
		return "", oversizeSourceArchiveError(int64(buf.Len()), files)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func collectSourceFiles(root string) ([]sourceFile, error) {
	rules := loadGitignore(root)

	var (
		files []sourceFile
		total int64
	)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := d.Name()

		if d.IsDir() {
			if appsSourceDenyDirs[base] || rules.ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if deniedSourceFile(base) || rules.ignored(rel, false) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("reading %s: %w", rel, err)
		}
		total += info.Size()
		if total > appsMaxSourceRawBytes {
			return fmt.Errorf("source directory exceeds %s of files; nothing was uploaded", formatArchiveBytes(appsMaxSourceRawBytes))
		}
		files = append(files, sourceFile{rel: rel, size: info.Size(), mode: info.Mode()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, nil
}

func oversizeSourceArchiveError(compressed int64, files []sourceFile) error {
	msg := fmt.Sprintf("source archive is %s compressed, over the %s limit",
		formatArchiveBytes(compressed), formatArchiveBytes(appsMaxSourceArchiveBytes))
	if top := biggestSourceContributors(files, 3); top != "" {
		msg += "; biggest contributors: " + top
	}
	return fmt.Errorf("%s. Delete or .gitignore those paths, then push again", msg)
}

// biggestSourceContributors summarizes the heaviest top-level paths.
func biggestSourceContributors(files []sourceFile, n int) string {
	sizes := map[string]int64{}
	for _, f := range files {
		key := f.rel
		if i := strings.Index(f.rel, "/"); i != -1 {
			key = f.rel[:i] + "/"
		}
		sizes[key] += f.size
	}
	keys := make([]string, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if sizes[keys[i]] != sizes[keys[j]] {
			return sizes[keys[i]] > sizes[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s (%s)", k, formatArchiveBytes(sizes[k])))
	}
	return strings.Join(parts, ", ")
}

func formatArchiveBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

type gitignoreRule struct {
	pattern  string
	dirOnly  bool
	negate   bool
	anchored bool
}

type gitignoreRules []gitignoreRule

// loadGitignore parses root/.gitignore; best-effort, never fatal.
func loadGitignore(root string) gitignoreRules {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	var rules gitignoreRules
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var r gitignoreRule
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			r.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		line = strings.TrimPrefix(line, "**/")
		if strings.HasPrefix(line, "/") {
			r.anchored = true
			line = strings.TrimPrefix(line, "/")
		}
		if line == "" {
			continue
		}
		r.pattern = line
		rules = append(rules, r)
	}
	return rules
}

// ignored reports whether rel matches; last matching rule wins.
func (rules gitignoreRules) ignored(rel string, isDir bool) bool {
	result := false
	for _, r := range rules {
		if r.dirOnly && !isDir {
			continue
		}
		if r.matches(rel) {
			result = !r.negate
		}
	}
	return result
}

func (r gitignoreRule) matches(rel string) bool {
	if r.anchored || strings.Contains(r.pattern, "/") {
		if ok, _ := path.Match(r.pattern, rel); ok {
			return true
		}
		return strings.HasPrefix(rel, r.pattern+"/")
	}
	for _, segment := range strings.Split(rel, "/") {
		if ok, _ := path.Match(r.pattern, segment); ok {
			return true
		}
	}
	return false
}
