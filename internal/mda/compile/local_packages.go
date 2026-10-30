package compile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Local packages that live outside the project root.
//
// Authors declare them the usual way: `npm install ../dep` writes
// `"dep": "file:../dep"` and `uv add ../dep` writes a `[tool.uv.sources]`
// path. The build only carries files under the project root, so each package
// is copied into `__local__/<name>/` and the build manifests point at the
// copy. Local packages of local packages are copied the same way.
//
// The relative imports of TypeScript sources that leave the project root are
// found by `introspect` (in [introspect.TransformTypescriptSources]).

const localPackagesDir = "__local__"

var skippedDirs = []string{
	"node_modules",
	".git",
	".mda",
	".venv",
	"venv",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	".ruff_cache",
}

// LocalPackage is a local package the build vendors.
type LocalPackage struct {
	Name   string
	Source string
	// Direct reports whether the project manifest declares the package, not
	// only another local package.
	Direct bool
	// LocalDependencies are the names of the local packages this package
	// declares.
	LocalDependencies []string
	// Extras are the Python extras requested by the manifests that declare
	// this package.
	Extras []string
}

// BuildDir is where the package is vendored, relative to the build.
func (p LocalPackage) BuildDir() string {
	return localPackagesDir + "/" + p.Name
}

// FindLocalPackages lists the local packages declared in the project manifest
// that resolve outside the project root, followed by the local packages they
// declare. Packages inside the root are already part of the build.
func FindLocalPackages(p *project.Project) ([]LocalPackage, error) {
	root, err := canonicalize(p.Root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", p.Root, err)
	}
	type declaration struct {
		declaredIn, name, path string
		direct                 bool
	}
	declared, err := declaredLocalPaths(p.Language, p.Root)
	if err != nil {
		return nil, err
	}
	var pending []declaration
	for _, local := range declared {
		pending = append(pending, declaration{p.Root, local.name, local.path, true})
	}
	var packages []LocalPackage
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		name, path := next.name, next.path
		source, err := canonicalize(filepath.Join(next.declaredIn, path))
		if err != nil {
			return nil, fmt.Errorf("local package `%s` points to `%s`, which does not exist", name, path)
		}
		if inside, ok := stripPathPrefix(source, root); next.direct && ok {
			// Already part of the build, so its manifest is copied as
			// written and a path outside the project would still point
			// outside the build.
			nested, err := declaredLocalPaths(p.Language, source)
			if err != nil {
				return nil, err
			}
			for _, local := range nested {
				resolved, err := canonicalize(filepath.Join(source, local.path))
				if _, within := stripPathPrefix(resolved, root); err == nil && !within {
					return nil, fmt.Errorf(
						"local package `%s` at `%s` is declared by `%s`, which is inside the project, "+
							"but resolves outside it; move `%s` into the project",
						local.name, local.path, inside, local.name)
				}
			}
			continue
		}
		if !isDir(source) {
			return nil, fmt.Errorf("local package `%s` points to `%s`, which is not a directory", name, path)
		}
		if !isPackageName(name) {
			return nil, fmt.Errorf("local package name `%s` is not a valid package name", name)
		}
		manifest := p.Language.ManifestName()
		if !isFile(filepath.Join(source, manifest)) {
			return nil, fmt.Errorf("local package `%s` at `%s` has no %s", name, path, manifest)
		}
		// The copy is linked under its own name, so an alias would not
		// resolve.
		actual, err := manifestPackageName(p.Language, source)
		if err != nil {
			return nil, err
		}
		sameName := actual == name
		if p.Language == project.Python {
			sameName = canonicalRequirementName(actual) == canonicalRequirementName(name)
		}
		if !sameName {
			return nil, fmt.Errorf("local package `%s` at `%s` is named `%s` in its %s; declare it as `%s`",
				name, path, actual, manifest, actual)
		}
		extras, err := requestedExtras(p.Language, next.declaredIn, name)
		if err != nil {
			return nil, err
		}
		if existing := slices.IndexFunc(packages, func(pkg LocalPackage) bool { return pkg.Source == source }); existing >= 0 {
			for _, extra := range extras {
				if !slices.Contains(packages[existing].Extras, extra) {
					packages[existing].Extras = append(packages[existing].Extras, extra)
				}
			}
			continue
		}
		if other := slices.IndexFunc(packages, func(pkg LocalPackage) bool { return pkg.Name == name }); other >= 0 {
			return nil, fmt.Errorf("two different local packages are named `%s`: %s and %s",
				name, packages[other].Source, source)
		}
		nested, err := declaredLocalPaths(p.Language, source)
		if err != nil {
			return nil, err
		}
		var localDependencies []string
		for _, local := range nested {
			localDependencies = append(localDependencies, local.name)
			pending = append(pending, declaration{source, local.name, local.path, false})
		}
		packages = append(packages, LocalPackage{
			Name:              name,
			Source:            source,
			Direct:            next.direct,
			LocalDependencies: localDependencies,
			Extras:            extras,
		})
	}
	return packages, nil
}

// isPackageName reports whether name is made of plain path components only
// (`dep`, `@scope/dep`), so its copy stays under `__local__/`.
func isPackageName(name string) bool {
	if name == "" {
		return true
	}
	if filepath.VolumeName(name) != "" || os.IsPathSeparator(name[0]) {
		return false
	}
	parts := strings.FieldsFunc(name, func(c rune) bool { return c < 0x80 && os.IsPathSeparator(uint8(c)) })
	for i, part := range parts {
		if part == ".." || part == "." && i == 0 {
			return false
		}
	}
	return true
}

// vendorLocalPackages copies each local package into the build.
//
// A package that declares local packages gets its manifest rewritten to
// point at their copies.
func vendorLocalPackages(language project.Language, packages []LocalPackage, outDir string, mode runtimePathMode) error {
	for _, pkg := range packages {
		buildDir := filepath.FromSlash(pkg.BuildDir())
		manifest := language.ManifestName()
		err := filepath.WalkDir(pkg.Source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("failed to read local package %s: %w", pkg.Source, err)
			}
			if path != pkg.Source && isSkippedDir(path) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(pkg.Source, path)
			if err != nil {
				return err
			}
			if project.IsIgnoredFile(rel) {
				return nil
			}
			dst := filepath.Join(outDir, buildDir, rel)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", filepath.Dir(dst), err)
			}
			if rel == manifest && !(len(pkg.LocalDependencies) == 0 && len(pkg.Extras) == 0) {
				contents, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", path, err)
				}
				rewritten, err := rewriteVendoredManifest(language, string(contents), pkg, mode)
				if err != nil {
					return fmt.Errorf("failed to rewrite %s: %w", path, err)
				}
				if _, err := writeIfChanged(dst, []byte(rewritten)); err != nil {
					return fmt.Errorf("failed to write %s: %w", dst, err)
				}
				return nil
			}
			return copyFileIfChanged(path, dst)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// vendoredManifests are the manifests named fileName of the packages vendored
// into outDir, relative to outDir.
func vendoredManifests(outDir, fileName string) ([]string, error) {
	base := filepath.Join(outDir, localPackagesDir)
	if !isDir(base) {
		return nil, nil
	}
	var manifests []string
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Unreadable entries are skipped, as `walkdir`'s `filter_map(Ok)`
			// skips them.
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(filepath.Separator)))
		}
		if entry.IsDir() && depth >= 3 {
			return filepath.SkipDir
		}
		if depth >= 2 && entry.Type().IsRegular() && entry.Name() == fileName {
			manifest, err := filepath.Rel(outDir, path)
			if err != nil {
				return err
			}
			manifests = append(manifests, manifest)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(manifests, comparePathComponents)
	return manifests, nil
}

// IsVendoredPath reports whether a change at rel inside a local package
// affects the build.
func IsVendoredPath(rel string) bool {
	for _, component := range strings.Split(filepath.ToSlash(rel), "/") {
		if slices.Contains(skippedDirs, component) {
			return false
		}
	}
	return !project.IsIgnoredFile(rel)
}

// isRuntimePackage reports whether name is the runtime dependency, which is
// always replaced by the vendored `__runtime__`.
func isRuntimePackage(name string) bool {
	return canonicalRequirementName(name) == runtimePackage
}

func isSkippedDir(path string) bool {
	return slices.Contains(skippedDirs, filepath.Base(path))
}

// manifestPackageName is the name a package gives itself in its manifest.
func manifestPackageName(language project.Language, dir string) (string, error) {
	path := filepath.Join(dir, language.ManifestName())
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	var name string
	var found bool
	if language == project.Python {
		doc, err := parseTomlDocument(string(contents))
		if err != nil {
			return "", fmt.Errorf("failed to parse %s: %w", path, err)
		}
		item, ok := tomlPath(doc.root, "project", "name")
		if ok {
			name, found = tomlString(item)
		}
	} else {
		manifest, err := parseJSON(contents)
		if err != nil {
			return "", fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if object, ok := manifest.(*jsonObject); ok {
			name, found = object.str("name")
		}
	}
	if !found {
		return "", fmt.Errorf("%s has no package name", path)
	}
	return name, nil
}

// localPath is a local package a manifest declares: its name and the path it
// points to, relative to the manifest.
type localPath struct {
	name, path string
}

func declaredLocalPaths(language project.Language, dir string) ([]localPath, error) {
	path := filepath.Join(dir, language.ManifestName())
	if !isFile(path) {
		return nil, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var declared []localPath
	if language == project.Python {
		declared, err = uvPathSources(string(contents))
	} else {
		declared, err = npmFileDependencies(contents)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return slices.DeleteFunc(declared, func(local localPath) bool { return isRuntimePackage(local.name) }), nil
}

// requestedExtras are the extras that the Python manifest in dir requests for
// name.
func requestedExtras(language project.Language, dir, name string) ([]string, error) {
	if language != project.Python {
		return nil, nil
	}
	path := filepath.Join(dir, "pyproject.toml")
	doc, err := readTomlDocument(path)
	if err != nil {
		return nil, err
	}
	canonical := canonicalRequirementName(name)
	var extras []string
	item, _ := tomlPath(doc.root, "project", "dependencies")
	if deps, ok := item.(*tomlArray); ok {
		for _, requirement := range deps.strings() {
			if canonicalRequirementName(requirement) != canonical {
				continue
			}
			for _, extra := range requirementExtras(requirement) {
				if !slices.Contains(extras, extra) {
					extras = append(extras, extra)
				}
			}
		}
	}
	return extras, nil
}

// npmFileDependencies are the `file:` specs, which `npm install ../dep`
// writes, and `link:` specs, which `pnpm add ../dep` writes.
func npmFileDependencies(contents []byte) ([]localPath, error) {
	manifest, err := parseJSON(contents)
	if err != nil {
		return nil, err
	}
	object, ok := manifest.(*jsonObject)
	if !ok {
		return nil, nil
	}
	dependencies, ok := object.object("dependencies")
	if !ok {
		return nil, nil
	}
	var declared []localPath
	for _, name := range dependencies.keys {
		spec, ok := dependencies.str(name)
		if !ok {
			continue
		}
		path, ok := strings.CutPrefix(spec, "file:")
		if !ok {
			path, ok = strings.CutPrefix(spec, "link:")
		}
		if ok {
			declared = append(declared, localPath{name, path})
		}
	}
	return declared, nil
}

// uvPathSources are the path sources of `[project.dependencies]`. A source
// that only serves a dependency group is not installed, so it is not vendored.
func uvPathSources(contents string) ([]localPath, error) {
	doc, err := parseTomlDocument(contents)
	if err != nil {
		return nil, err
	}
	var required []string
	item, _ := tomlPath(doc.root, "project", "dependencies")
	if deps, ok := item.(*tomlArray); ok {
		for _, requirement := range deps.strings() {
			required = append(required, canonicalRequirementName(requirement))
		}
	}
	item, ok := tomlPath(doc.root, "tool", "uv", "sources")
	if !ok {
		return nil, nil
	}
	sources, ok := asTableLike(item)
	if !ok {
		return nil, nil
	}
	var declared []localPath
	for _, name := range sources.keyNames() {
		if !slices.Contains(required, canonicalRequirementName(name)) {
			continue
		}
		source, _ := sources.getItem(name)
		pathItem, ok := tomlGet(source, "path")
		if !ok {
			continue
		}
		if path, ok := tomlString(pathItem); ok {
			declared = append(declared, localPath{name, path})
		}
	}
	return declared, nil
}
