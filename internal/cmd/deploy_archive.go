package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	deployArchiveWarnBytes int64 = 50 << 20
	deployArchiveMaxBytes  int64 = 200 << 20
)

var deployArchiveExcludedDirs = map[string]bool{
	"__pycache__":  true,
	".git":         true,
	".venv":        true,
	"venv":         true,
	"node_modules": true,
	".tox":         true,
	".mypy_cache":  true,
}

type sourceArchive struct {
	dir       string
	path      string
	size      int64
	configRel string
}

func (a *sourceArchive) Close() error { return os.RemoveAll(a.dir) }

// createSourceArchive roots the tar at the deps' common ancestor.
func createSourceArchive(cfg *langgraphConfig, p *deployProgress) (*sourceArchive, error) {
	contextDir := cfg.dir()
	dirs := []string{contextDir}
	for _, dep := range cfg.dependencies {
		if !strings.HasPrefix(dep, ".") {
			continue
		}
		resolved := filepath.Join(contextDir, dep)
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("could not find local dependency: %s", resolved)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("local dependency must be a directory: %s", resolved)
		}
		if !isWithinDir(contextDir, resolved) {
			dirs = append(dirs, resolved)
		}
	}
	common := contextDir
	for _, d := range dirs[1:] {
		common = commonDir(common, d)
	}

	tmp, err := os.MkdirTemp("", "langsmith-deploy-")
	if err != nil {
		return nil, err
	}
	archive := &sourceArchive{dir: tmp, path: filepath.Join(tmp, "source.tar.gz")}
	configRel, err := filepath.Rel(common, cfg.path)
	if err != nil {
		archive.Close()
		return nil, err
	}
	archive.configRel = filepath.ToSlash(configRel)

	written, err := writeSourceArchive(archive.path, common, dirs)
	if err != nil {
		archive.Close()
		return nil, err
	}
	if !written[archive.configRel] {
		archive.Close()
		return nil, fmt.Errorf("archive validation failed: %s not found in archive (is it ignored by .dockerignore or .gitignore?)", archive.configRel)
	}
	info, err := os.Stat(archive.path)
	if err != nil {
		archive.Close()
		return nil, err
	}
	archive.size = info.Size()
	if archive.size > deployArchiveMaxBytes {
		archive.Close()
		return nil, fmt.Errorf("source archive is %s, which exceeds the %s limit; add large files (model weights, data sets, etc.) to .dockerignore or .gitignore",
			formatArchiveBytes(archive.size), formatArchiveBytes(deployArchiveMaxBytes))
	}
	if archive.size > deployArchiveWarnBytes {
		p.Info("Warning: source archive is %s. Consider adding large files to .dockerignore or .gitignore.", formatArchiveBytes(archive.size))
	}
	return archive, nil
}

func writeSourceArchive(dest, common string, dirs []string) (map[string]bool, error) {
	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	written := map[string]bool{}
	seen := map[string]bool{}
	for _, dir := range dirs {
		prefix, err := filepath.Rel(common, dir)
		if err != nil {
			return nil, err
		}
		prefix = filepath.ToSlash(prefix)
		if prefix == "." {
			prefix = ""
		}
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		if err := addArchiveDir(tw, dir, prefix, written); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("finalizing source archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("compressing source archive: %w", err)
	}
	return written, f.Close()
}

func addArchiveDir(tw *tar.Writer, root, prefix string, written map[string]bool) error {
	rules := loadIgnoreFiles(root, ".dockerignore", ".gitignore")
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
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
		if d.IsDir() {
			if deployArchiveExcludedDirs[d.Name()] || rules.ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || rules.ignored(rel, false) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		name := rel
		if prefix != "" {
			name = path.Join(prefix, rel)
		}
		hdr := &tar.Header{
			Name:     name,
			Mode:     int64(info.Mode().Perm()),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("archiving %s: %w", name, err)
		}
		src, err := os.Open(p)
		if err != nil {
			return fmt.Errorf("reading %s: %w", p, err)
		}
		defer src.Close()
		if _, err := io.Copy(tw, src); err != nil {
			return fmt.Errorf("archiving %s: %w", name, err)
		}
		written[name] = true
		return nil
	})
}

func isWithinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func commonDir(a, b string) string {
	for !isWithinDir(a, b) {
		parent := filepath.Dir(a)
		if parent == a {
			return a
		}
		a = parent
	}
	return a
}
