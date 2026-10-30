package deploy

// Build a `tar.gz` of the compiled project for remote (internal-source)
// builds (`deploy/archive.rs`).
//
// The compiled build directory (`.mda/build`) is already a clean, minimal
// tree — it contains only the user's source files plus the generated
// `langgraph.json` and managed entry module — so the archive includes every
// regular file under it except local-only emit scratch (an isolated Python
// venv and pip `--target` site, and the `node_modules` the TypeScript emit
// installs to import channels/connectors). The remote builder reinstalls
// dependencies from `package.json` / `package-lock.json`, so shipping a
// host-platform `node_modules` only bloats the upload. The `langgraph.json`
// therefore lives at the archive root (ConfigRel).

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/compile"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

// nodeModulesDir is the npm install tree the TypeScript emit step populates;
// rebuilt remotely.
const nodeModulesDir = "node_modules"

// maxArchiveSize is the hard archive size limit enforced by the host backend
// (200 MB).
const maxArchiveSize = 200 * 1024 * 1024

// Archive is a created in-memory source archive plus the metadata needed to
// upload it.
type Archive struct {
	// Bytes is the gzipped tarball.
	Bytes []byte
	// ConfigRel is the config path relative to the archive root (always
	// `langgraph.json`).
	ConfigRel string
}

// CreateArchive creates a gzipped tarball of the compiled build, omitting
// local emit scratch.
func CreateArchive(buildDir string) (*Archive, error) {
	configRel := "langgraph.json"
	if info, err := os.Stat(filepath.Join(buildDir, configRel)); err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("compiled build at %s is missing langgraph.json", buildDir)
	}

	var buffer bytes.Buffer
	encoder := gzip.NewWriter(&buffer)
	builder := tar.NewWriter(encoder)

	err := filepath.WalkDir(buildDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk build directory: %w", err)
		}
		if isLocalEmitScratch(buildDir, path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(buildDir, path)
		if err != nil {
			return fmt.Errorf("build file outside build directory: %w", err)
		}
		// Normalize to POSIX separators for portability across platforms.
		relPosix := proc.ToUnixPath(rel)
		if err := appendFile(builder, path, relPosix); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", relPosix, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := builder.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize tar archive: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("failed to gzip archive: %w", err)
	}

	if buffer.Len() > maxArchiveSize {
		return nil, fmt.Errorf("source archive is %.1f MB, which exceeds the 200 MB limit",
			float64(buffer.Len())/(1024.0*1024.0))
	}

	return &Archive{Bytes: buffer.Bytes(), ConfigRel: configRel}, nil
}

// appendFile adds the regular file at path to builder as name.
func appendFile(builder *tar.Writer, path, name string) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = name
	if err := builder.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(builder, file)
	return err
}

// isLocalEmitScratch reports whether path is under a local-only directory of
// the build: the connector emit venv + pip `--target` site, and
// `node_modules` from the TypeScript emit or `mda dev` (the remote builder
// reinstalls from the lockfile).
func isLocalEmitScratch(buildDir, path string) bool {
	rel, err := filepath.Rel(buildDir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	first, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	switch first {
	case proc.PythonEmitVenvDir, compile.PythonEmitSiteDir, nodeModulesDir:
		return true
	default:
		return false
	}
}
