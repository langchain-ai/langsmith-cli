package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Vendoring the installed `managed-deepagents` runtime into the build output
// so the deployed app carries the exact runtime that matches the CLI.

// vendorRuntime copies the `managed-deepagents` runtime from the developer's
// installed environment into the build output so the deployed app carries the
// exact runtime that matches the CLI.
func vendorRuntime(language project.Language, installation Installation, outDir string) error {
	target := filepath.Join(outDir, runtimeDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", target, err)
	}
	manifest := installation.RuntimeManifest()
	if language == project.Python {
		return vendorPyRuntime(installation.RuntimeSource(), target, installation.PackageVersion(), manifest)
	}
	return vendorTsRuntime(installation.RuntimeSource(), target, installation.PackageVersion(), manifest)
}

// vendorTsRuntime copies TypeScript runtime files (`dist/` contents) into
// `__runtime__/` and writes a minimal `package.json` so the directory is a
// valid npm workspace package named `managed-deepagents`.
//
// Its entry points and dependencies are the installed package's, so the
// runtime barrel (`managed-deepagents/runtime`, …) resolves from the vendored
// copy and the build installs what the runtime imports.
func vendorTsRuntime(sourceDist, target, version string, manifest project.RuntimeManifest) error {
	if err := copyDirRecursive(sourceDist, target); err != nil {
		return err
	}

	pkg := jsonObjectOf(
		"name", "managed-deepagents",
		"version", version,
		"type", "module",
	)
	if manifest.Main != "" {
		pkg.insert("main", manifest.Main)
	}
	if len(manifest.Exports) > 0 {
		exports := newJSONObject()
		for _, export := range manifest.Exports {
			exports.insert(export.Subpath, export.Target)
		}
		pkg.insert("exports", exports)
	}
	if len(manifest.Dependencies) > 0 {
		dependencies := newJSONObject()
		for _, dep := range manifest.Dependencies {
			dependencies.insert(dep.Name, dep.Version)
		}
		pkg.insert("dependencies", dependencies)
	}
	if _, err := writeIfChanged(filepath.Join(target, "package.json"), []byte(prettyJSON(pkg)+"\n")); err != nil {
		return fmt.Errorf("failed to write vendored package.json: %w", err)
	}
	return nil
}

// vendorPyRuntime copies Python runtime files into `__runtime__/` as a
// pip-installable package, declaring the installed distribution's Python and
// runtime requirements.
//
// Layout:
//
//	__runtime__/
//	  pyproject.toml
//	  managed_deepagents/
//	    __init__.py
//	    connectors/
//	    runtime.py
func vendorPyRuntime(sourcePkg, target, version string, manifest project.RuntimeManifest) error {
	pkgDir := filepath.Join(target, "managed_deepagents")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", pkgDir, err)
	}

	// Copy the full package tree so subpackages (e.g. `connectors/`) and
	// package data (e.g. `py.typed`) resolve in the vendored runtime. Skip
	// cache dirs and the platform-specific CLI binary bundled for local
	// installs.
	if err := copyPkgDirRecursive(sourcePkg, pkgDir); err != nil {
		return err
	}

	quote := func(text string) string { return encodeTomlString(text, "\"", true, false) }
	var pyproject strings.Builder
	pyproject.WriteString("[project]\n")
	pyproject.WriteString("name = \"managed-deepagents\"\n")
	pyproject.WriteString("version = " + quote(version) + "\n")
	if manifest.RequiresPython != "" {
		pyproject.WriteString("requires-python = " + quote(manifest.RequiresPython) + "\n")
	}
	pyproject.WriteString("dependencies = [\n")
	for _, dep := range manifest.Dependencies {
		pyproject.WriteString(quote(dep.Name+dep.Version) + ",\n")
	}
	pyproject.WriteString("]\n" +
		"\n" +
		"[build-system]\n" +
		"requires = [\"setuptools>=61\"]\n" +
		"build-backend = \"setuptools.build_meta\"\n")
	if _, err := writeIfChanged(filepath.Join(target, "pyproject.toml"), []byte(pyproject.String())); err != nil {
		return fmt.Errorf("failed to write vendored pyproject.toml: %w", err)
	}
	return nil
}

// shouldVendorPyDir reports whether a directory should be copied into the
// vendored Python runtime.
func shouldVendorPyDir(name string) bool {
	return name != "__pycache__" && name != "_binaries"
}

// copyPkgDirRecursive recursively copies a Python package directory into the
// vendored runtime.
func copyPkgDirRecursive(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(src, name)
		if isDir(path) {
			if !shouldVendorPyDir(name) {
				continue
			}
			subDst := filepath.Join(dst, name)
			if err := os.MkdirAll(subDst, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", subDst, err)
			}
			if err := copyPkgDirRecursive(path, subDst); err != nil {
				return err
			}
			continue
		}
		if filepath.Ext(name) == ".pyc" {
			continue
		}
		if err := copyFileIfChanged(path, filepath.Join(dst, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyDirRecursive(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	for _, entry := range entries {
		path := filepath.Join(src, entry.Name())
		switch {
		case isFile(path):
			if err := copyFileIfChanged(path, filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		case isDir(path):
			sub := filepath.Join(dst, entry.Name())
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", sub, err)
			}
			if err := copyDirRecursive(path, sub); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFileIfChanged copies src onto dst without touching mtime when the bytes
// already match.
//
// A rebuild re-vendors the whole runtime tree; files that did not change
// keep their mtime.
func copyFileIfChanged(src, dst string) error {
	contents, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}
	if _, err := writeIfChanged(dst, contents); err != nil {
		return fmt.Errorf("failed to copy %s: %w", src, err)
	}
	return syncFilePermissions(src, dst)
}
