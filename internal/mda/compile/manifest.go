package compile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

// Injecting the managed dependency stack into the build manifest and
// rewriting the `managed-deepagents` dependency to point at the vendored
// runtime.

// npmLockfileTimeout caps `npm install --package-lock-only` so a hung registry
// fails the compile instead of stalling for tens of minutes.
const npmLockfileTimeout = 90 * time.Second

// pythonVersionConfig is the compatibility and deployment target the authored
// `pyproject.toml` declares; empty fields are absent.
type pythonVersionConfig struct {
	requiresPython *string
	pythonVersion  *string
}

// readPythonVersionConfig reads the compatibility and deployment target from
// the authored `pyproject.toml`.
func readPythonVersionConfig(projectRoot string) (pythonVersionConfig, error) {
	path := filepath.Join(projectRoot, "pyproject.toml")
	if !isFile(path) {
		return pythonVersionConfig{}, nil
	}
	doc, err := readTomlDocument(path)
	if err != nil {
		return pythonVersionConfig{}, err
	}
	readString := func(item tomlItem, ok bool, field string) (*string, error) {
		if !ok {
			return nil, nil
		}
		text, isString := tomlString(item)
		if !isString {
			return nil, fmt.Errorf("%s in %s must be a quoted string", field, path)
		}
		return &text, nil
	}
	requiresItem, hasRequires := tomlPath(doc.root, "project", "requires-python")
	requiresPython, err := readString(requiresItem, hasRequires, "[project] requires-python")
	if err != nil {
		return pythonVersionConfig{}, err
	}
	versionItem, hasVersion := tomlPath(doc.root, "tool", "mda", "python-version")
	pythonVersion, err := readString(versionItem, hasVersion, "[tool.mda] python-version")
	if err != nil {
		return pythonVersionConfig{}, err
	}
	return pythonVersionConfig{requiresPython: requiresPython, pythonVersion: pythonVersion}, nil
}

// readTomlDocument reads and parses the TOML file at path.
func readTomlDocument(path string) (*tomlDocument, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	doc, err := parseTomlDocument(string(contents))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return doc, nil
}

func writeTomlDocument(path string, doc *tomlDocument) error {
	if err := os.WriteFile(path, []byte(doc.String()), 0o666); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// synthesizedPyproject is the manifest of a Python build whose project has
// none.
const synthesizedPyproject = `[project]
name = "mda-agent"
version = "0.0.0"
requires-python = ">=3.11"
dependencies = []
`

// configurePythonPackaging makes a compiled Python build installable as a
// dependency-only project.
//
// LangSmith installs the project with `pip install -e .`, which triggers
// setuptools flat-layout auto-discovery — and that fails on the MDA folder
// convention (`tools/`, `middleware/`, `skills/` look like competing top-level
// packages). The agent itself runs from source on `sys.path` (the runtime
// loads `_mda_entry.py` by path), so the install only needs to bring in
// dependencies, not package the user's modules.
//
// Project discovery allows `agent.py` without an authored `pyproject.toml`.
// When the compile output has no manifest, synthesize a minimal one so
// dependency injection and `pip install -e .` still work for deploy.
func configurePythonPackaging(outDir string) error {
	path := filepath.Join(outDir, "pyproject.toml")
	if !isFile(path) {
		if err := os.WriteFile(path, []byte(synthesizedPyproject), 0o666); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}
	doc, err := readTomlDocument(path)
	if err != nil {
		return err
	}

	// Respect an author-declared build system; only fix the default case.
	if _, ok := doc.root.items.get("build-system"); ok {
		return nil
	}

	requires := &tomlArray{values: []tomlValue{newTomlString("setuptools>=61")}}
	buildSystem := &tomlTable{}
	buildSystem.items.put(newTomlKey("requires"), requires)
	buildSystem.items.put(newTomlKey("build-backend"), newTomlString("setuptools.build_meta"))
	doc.root.insertItem("build-system", buildSystem)

	// Disable setuptools package auto-discovery: no user modules are
	// installed, only dependencies.
	tool, ok := doc.root.items.entry("tool", func() tomlItem { return &tomlTable{} }).(*tomlTable)
	if !ok {
		return fmt.Errorf("[tool] in %s is not a table", path)
	}
	setuptools, ok := tool.items.entry("setuptools", func() tomlItem { return &tomlTable{} }).(*tomlTable)
	if !ok {
		return fmt.Errorf("[tool.setuptools] in %s is not a table", path)
	}
	setuptools.items.put(newTomlKey("py-modules"), &tomlArray{})

	return writeTomlDocument(path, doc)
}

// injectManagedDependencies injects the managed dependency stack into the
// build manifest.
//
// Combines the always-required LangChain/LangGraph runtime dependencies with
// the chat-model provider package inferred from the agent's `model`, then
// merges them into the copied manifest. A managed package the vendored
// runtime also depends on takes the runtime's range (runtime). An
// author-declared version always wins, so a power user can still pin or
// override any package by listing it themselves.
func injectManagedDependencies(
	p *project.Project,
	outDir string,
	model *string,
	browser bool,
	runtime []project.Dependency,
) error {
	deps := alignWithRuntime(project.ManagedDependencies(p.Language), runtime, p.Language)
	if browser {
		deps = append(deps, project.BrowserDependencies(p.Language)...)
	}
	if model != nil {
		if provider, ok := project.ProviderDependencyForModel(*model, p.Language); ok {
			deps = append(deps, provider)
		}
	}
	if p.Language == project.Python {
		return mergePyprojectDeps(outDir, deps)
	}
	return mergePackageJSONDeps(outDir, deps)
}

// alignWithRuntime gives each of deps that the vendored runtime also depends
// on the runtime's range: the runtime was released against it, and a build
// pinning another would install a version under the runtime it does not
// support (npm) or fail to resolve at all (pip). Python names are compared
// PEP 503-canonical and without extras, keeping the managed extras.
func alignWithRuntime(deps, runtime []project.Dependency, language project.Language) []project.Dependency {
	key := func(name string) string {
		if language == project.Python {
			return canonicalRequirementName(name)
		}
		return name
	}
	ranges := map[string]string{}
	for _, dep := range runtime {
		ranges[key(dep.Name)] = dep.Version
	}
	aligned := make([]project.Dependency, len(deps))
	for i, dep := range deps {
		if version, ok := ranges[key(dep.Name)]; ok {
			dep.Version = version
		}
		aligned[i] = dep
	}
	return aligned
}

// rewriteRuntimeDependency rewrites the `managed-deepagents` dependency for
// the build language.
//
// TypeScript builds vendor the runtime under `__runtime__/` and expose it as
// an npm workspace package instead of a `file:` dependency — hosted npm 11.x
// has crashed with `Exit handler never called!` when installing `file:` deps.
// Python builds drop the requirement: `langgraph.json` installs the vendored
// copy as its own local dependency.
func rewriteRuntimeDependency(language project.Language, outDir string) error {
	if language == project.Python {
		return rewritePyRuntimeDep(outDir)
	}
	return rewriteTsRuntimeDep(outDir)
}

// readPackageJSON reads a `package.json` whose root must be an object.
func readPackageJSON(path string) (*jsonObject, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	manifest, ok, err := parseJSONObject(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if !ok {
		return nil, fmt.Errorf("%s root must be a JSON object", path)
	}
	return manifest, nil
}

// writePackageJSON writes manifest pretty-printed, leaving an unchanged file
// alone.
func writePackageJSON(path string, manifest *jsonObject) error {
	if _, err := writeIfChanged(path, []byte(prettyJSON(manifest)+"\n")); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func rewriteTsRuntimeDep(outDir string) error {
	path := filepath.Join(outDir, "package.json")
	if !isFile(path) {
		return nil
	}
	manifest, err := readPackageJSON(path)
	if err != nil {
		return err
	}

	// Drop any registry/`file:` entry; the workspace package supplies the
	// name.
	if deps, ok := manifest.object("dependencies"); ok {
		deps.remove(runtimePackage)
	}

	ensureWorkspace(manifest, runtimeDir)
	// Workspace roots are not published; keep npm from treating this as a
	// publishable package when it sees `workspaces`.
	manifest.insert("private", true)
	return writePackageJSON(path, manifest)
}

// ensureWorkspace ensures `workspaces` lists entry.
func ensureWorkspace(object *jsonObject, entry string) {
	listed := func(packages []any) bool {
		return slices.ContainsFunc(packages, func(value any) bool {
			text, ok := value.(string)
			return ok && (text == entry || text == "./"+entry)
		})
	}
	switch workspaces := object.values["workspaces"].(type) {
	case []any:
		if !listed(workspaces) {
			object.insert("workspaces", append(workspaces, entry))
		}
	case *jsonObject:
		packages, ok := workspaces.entry("packages", []any{}).([]any)
		if !ok {
			workspaces.insert("packages", []any{entry})
		} else if !listed(packages) {
			workspaces.insert("packages", append(packages, entry))
		}
	default:
		object.insert("workspaces", []any{entry})
	}
}

func rewritePyRuntimeDep(outDir string) error {
	path := filepath.Join(outDir, "pyproject.toml")
	if !isFile(path) {
		return nil
	}
	doc, err := readTomlDocument(path)
	if err != nil {
		return err
	}

	// A uv source for the runtime would win over the vendored copy, and its
	// path is relative to the project, not the build.
	if slot := tomlGetMutPath(doc.root, "tool", "uv", "sources"); slot != nil {
		if sources, ok := asTableLike(*slot); ok {
			for _, name := range sources.keyNames() {
				if canonicalRequirementName(name) == runtimePackage {
					sources.removeItem(name)
				}
			}
		}
	}

	if slot := tomlGetMutPath(doc.root, "project", "dependencies"); slot != nil {
		if deps, ok := (*slot).(*tomlArray); ok {
			deps.values = slices.DeleteFunc(deps.values, func(value tomlValue) bool {
				text, ok := tomlString(value)
				return ok && canonicalRequirementName(text) == runtimePackage
			})
		}
	}
	return writeTomlDocument(path, doc)
}

// rewriteLocalPackageDependencies points the build manifest at the local
// packages vendored under `__local__/`.
//
// TypeScript links each copy as an npm workspace, like the runtime. Python
// drops the requirement because `langgraph.json` installs the copy as its own
// local dependency.
func rewriteLocalPackageDependencies(language project.Language, outDir string, packages []LocalPackage) error {
	if len(packages) == 0 {
		return nil
	}
	if language == project.Python {
		return rewritePyLocalDeps(outDir, packages)
	}
	return rewriteTsLocalDeps(outDir, packages)
}

func rewriteTsLocalDeps(outDir string, packages []LocalPackage) error {
	path := filepath.Join(outDir, "package.json")
	manifest, err := readPackageJSON(path)
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		if deps, ok := manifest.object("dependencies"); ok {
			deps.remove(pkg.Name)
		}
		ensureWorkspace(manifest, pkg.BuildDir())
	}
	manifest.insert("private", true)
	return writePackageJSON(path, manifest)
}

func rewritePyLocalDeps(outDir string, packages []LocalPackage) error {
	path := filepath.Join(outDir, "pyproject.toml")
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	var direct []string
	for _, pkg := range packages {
		if pkg.Direct {
			direct = append(direct, pkg.Name)
		}
	}
	rewritten, err := dropPyLocalRequirements(string(contents), direct)
	if err != nil {
		return fmt.Errorf("failed to rewrite %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(rewritten), 0o666); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// rewriteVendoredManifest rewrites the manifest of a vendored local package so
// its own local packages resolve to their sibling copies under `__local__/`.
func rewriteVendoredManifest(language project.Language, contents string, pkg LocalPackage) (string, error) {
	if language == project.Python {
		rewritten, err := dropPyLocalRequirements(contents, pkg.LocalDependencies)
		if err != nil {
			return "", err
		}
		if len(pkg.Extras) > 0 {
			return foldRequestedExtras(rewritten, pkg)
		}
		return rewritten, nil
	}
	manifest, err := parseJSON([]byte(contents))
	if err != nil {
		return "", err
	}
	if object, ok := manifest.(*jsonObject); ok {
		if deps, ok := object.object("dependencies"); ok {
			for _, name := range pkg.LocalDependencies {
				deps.remove(name)
			}
		}
	}
	return prettyJSON(manifest) + "\n", nil
}

// dropPyLocalRequirements drops the requirements and uv path sources named in
// names: `langgraph.json` installs each vendored copy separately.
func dropPyLocalRequirements(contents string, names []string) (string, error) {
	doc, err := parseTomlDocument(contents)
	if err != nil {
		return "", err
	}
	for _, name := range names {
		var sources tomlTableLike
		if slot := tomlGetMutPath(doc.root, "tool", "uv", "sources"); slot != nil {
			sources, _ = asTableLike(*slot)
		}
		if sources == nil {
			return "", errors.New("[tool.uv.sources] is not a table")
		}
		sources.removeItem(name)

		slot := tomlGetMutPath(doc.root, "project", "dependencies")
		if slot == nil {
			continue
		}
		deps, ok := (*slot).(*tomlArray)
		if !ok {
			continue
		}
		canonical := canonicalRequirementName(name)
		for index := 0; index < len(deps.values); {
			requirement, ok := tomlString(deps.values[index])
			if !ok || canonicalRequirementName(requirement) != canonical {
				index++
				continue
			}
			if marker, ok := requirementMarker(requirement); ok {
				return "", fmt.Errorf(
					"langsmith deploy always installs local package `%s`, so it cannot honor the "+
						"marker `%s`. Remove the marker from `%s`.", name, marker, requirement)
			}
			deps.remove(index)
		}
	}
	return doc.String(), nil
}

// foldRequestedExtras adds the requested extras of a vendored Python package
// to its own dependencies. Deploy installs the copy as a bare
// `langgraph.json` path, which cannot select extras.
func foldRequestedExtras(contents string, pkg LocalPackage) (string, error) {
	doc, err := parseTomlDocument(contents)
	if err != nil {
		return "", err
	}
	own := canonicalRequirementName(pkg.Name)
	type extraRequirements struct {
		name         string
		requirements []string
	}
	var optional []extraRequirements
	if item, ok := tomlPath(doc.root, "project", "optional-dependencies"); ok {
		if table, ok := asTableLike(item); ok {
			for _, extra := range table.keyNames() {
				value, _ := table.getItem(extra)
				var requirements []string
				if array, ok := value.(*tomlArray); ok {
					requirements = array.strings()
				}
				optional = append(optional, extraRequirements{normalizeExtraName(extra), requirements})
			}
		}
	}

	pending := slices.Clone(pkg.Extras)
	var seen, added []string
	for len(pending) > 0 {
		extra := pending[0]
		pending = pending[1:]
		if slices.Contains(seen, extra) {
			continue
		}
		seen = append(seen, extra)
		index := slices.IndexFunc(optional, func(candidate extraRequirements) bool { return candidate.name == extra })
		if index < 0 {
			return "", fmt.Errorf("local package `%s` has no extra `%s`", pkg.Name, extra)
		}
		for _, requirement := range optional[index].requirements {
			if canonicalRequirementName(requirement) == own {
				pending = append(pending, requirementExtras(requirement)...)
			} else {
				added = append(added, requirement)
			}
		}
	}

	projectItem := doc.root.items.entry("project", func() tomlItem { return &tomlTable{} })
	projectTable, ok := asTableLike(projectItem)
	if !ok {
		return "", errors.New("[project] is not a table")
	}
	depsItem, ok := projectTable.getItem("dependencies")
	if !ok {
		depsItem = &tomlArray{}
		projectTable.insertItem("dependencies", depsItem)
	}
	deps, ok := depsItem.(*tomlArray)
	if !ok {
		return "", errors.New("[project].dependencies is not an array")
	}
	for _, requirement := range added {
		if !slices.Contains(deps.strings(), requirement) {
			deps.values = append(deps.values, newTomlString(requirement))
		}
	}
	return doc.String(), nil
}

// mergePackageJSONDeps merges deps into the build's `package.json`, adding
// only packages the author has not already declared. The dependency map is
// rewritten in sorted order for stable, diff-friendly output.
func mergePackageJSONDeps(outDir string, deps []project.Dependency) error {
	path := filepath.Join(outDir, "package.json")
	if !isFile(path) {
		// No manifest to augment. A TypeScript build needs one to install
		// against, but that is reported by the downstream langgraph build;
		// the compile step should not invent a manifest out of nothing here.
		return nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	manifest, ok, err := parseJSONObject(contents)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if !ok {
		return fmt.Errorf("%s is not a JSON object", path)
	}
	depsObject, ok := manifest.entry("dependencies", newJSONObject()).(*jsonObject)
	if !ok {
		return fmt.Errorf("\"dependencies\" in %s is not an object", path)
	}
	for _, dep := range deps {
		depsObject.entry(dep.Name, dep.Version)
	}
	depsObject.sortKeys()

	// `deepagents` still peers `langsmith@^0.7.1`. Mirror the build's
	// langsmith pin into `overrides` so `npm install --package-lock-only`
	// (and Host `npm ci`) can resolve until that peer range widens.
	if err := ensureLangsmithNpmOverride(manifest); err != nil {
		return err
	}
	return writePackageJSON(path, manifest)
}

// ensureLangsmithNpmOverride sets `overrides.langsmith` to the same range as
// `dependencies.langsmith`.
//
// No-op when the build has no langsmith dependency. Overwrites any prior
// langsmith override so it cannot drift from the direct dependency (npm
// rejects that with `EOVERRIDE`).
func ensureLangsmithNpmOverride(manifest *jsonObject) error {
	deps, ok := manifest.object("dependencies")
	if !ok {
		return nil
	}
	langsmith, ok := deps.str("langsmith")
	if !ok {
		return nil
	}
	overrides, ok := manifest.entry("overrides", newJSONObject()).(*jsonObject)
	if !ok {
		return errors.New("\"overrides\" in package.json is not an object")
	}
	overrides.insert("langsmith", langsmith)
	return nil
}

// mergePyprojectDeps merges deps into the build's `pyproject.toml`
// `[project].dependencies`, adding only packages the author has not already
// declared.
func mergePyprojectDeps(outDir string, deps []project.Dependency) error {
	path := filepath.Join(outDir, "pyproject.toml")
	if !isFile(path) {
		return nil
	}
	doc, err := readTomlDocument(path)
	if err != nil {
		return err
	}
	// Get or create `[project].dependencies`: a freshly scaffolded project may
	// have a `[project]` table without a `dependencies` array yet.
	projectTable, ok := doc.root.items.entry("project", func() tomlItem { return &tomlTable{} }).(*tomlTable)
	if !ok {
		return fmt.Errorf("[project] in %s is not a table", path)
	}
	array, ok := projectTable.items.entry("dependencies", func() tomlItem { return &tomlArray{} }).(*tomlArray)
	if !ok {
		return fmt.Errorf("[project].dependencies in %s is not an array", path)
	}

	// Whether the author's array spans multiple lines, so injected entries can
	// match that style instead of being crammed onto the closing-bracket line.
	var rendered strings.Builder
	writeTomlValue(&rendered, array, defaultLeadingValueDecor)
	multiline := len(array.values) == 0 || strings.Contains(rendered.String(), "\n")
	// The previous last element often carries the pre-`]` newline as its
	// suffix decor, which would otherwise push the separating comma onto its
	// own line.
	if len(array.values) > 0 {
		empty := ""
		array.values[len(array.values)-1].valueDecor().suffix = &empty
	}
	appended := false
	for _, dep := range deps {
		// Compare PEP 503-canonical names so an author-declared package is
		// matched regardless of `-`/`_`/`.` or case differences in its
		// spelling.
		depName := canonicalRequirementName(dep.Name)
		alreadyDeclared := slices.ContainsFunc(array.strings(), func(requirement string) bool {
			if canonicalRequirementName(requirement) != depName {
				return false
			}
			declared := requirementExtras(requirement)
			for _, extra := range requirementExtras(dep.Name) {
				if !slices.Contains(declared, extra) {
					return false
				}
			}
			return true
		})
		if alreadyDeclared {
			continue
		}
		value := newTomlString(dep.Name + dep.Version)
		prefix := " "
		if multiline {
			prefix = "\n    "
		}
		value.decor.prefix = &prefix
		array.values = append(array.values, value)
		appended = true
	}
	if appended && multiline {
		// Drop the closing bracket onto its own line for a tidy array.
		array.trailing = "\n"
		array.trailingComma = false
	}
	return writeTomlDocument(path, doc)
}

// requirementName extracts the package name from a PEP 508 requirement string
// (the leading run of name characters before any version specifier or
// extras).
func requirementName(requirement string) string {
	trimmed := strings.TrimLeftFunc(requirement, unicode.IsSpace)
	end := strings.IndexFunc(trimmed, func(c rune) bool {
		return c >= 0x80 || !(isBareKeyByte(byte(c)) || c == '.')
	})
	if end < 0 {
		return trimmed
	}
	return trimmed[:end]
}

// canonicalRequirementName is the PEP 503-style canonical package name used to
// compare requirement strings: the leading name lowercased with `_` and `.`
// folded to `-`. This lets the dependency dedup treat `langchain_openai`,
// `langchain-openai`, and `Langchain.OpenAI` as the same package, so an author
// who declares a managed package under any equivalent spelling never gets a
// duplicate injected.
func canonicalRequirementName(requirement string) string {
	return normalizeExtraName(requirementName(requirement))
}

// normalizeExtraName lowercases an ASCII name and folds `_` and `.` to `-`.
func normalizeExtraName(name string) string {
	return strings.NewReplacer("_", "-", ".", "-").Replace(asciiLower(name))
}

func asciiLower(text string) string {
	return strings.Map(func(c rune) rune {
		if 'A' <= c && c <= 'Z' {
			return c + ('a' - 'A')
		}
		return c
	}, text)
}

// requirementMarker is the environment marker after `;`, if any.
func requirementMarker(requirement string) (string, bool) {
	_, marker, ok := strings.Cut(requirement, ";")
	marker = strings.TrimSpace(marker)
	return marker, ok && marker != ""
}

// requirementExtras are the extras a requirement requests. Extras are
// additive: a bare author pin does not supply managed runtime extras.
func requirementExtras(requirement string) []string {
	name := requirementName(requirement)
	rest := strings.TrimLeftFunc(strings.TrimLeftFunc(requirement, unicode.IsSpace)[len(name):], unicode.IsSpace)
	inner, ok := strings.CutPrefix(rest, "[")
	if !ok {
		return nil
	}
	extras, _, ok := strings.Cut(inner, "]")
	if !ok {
		return nil
	}
	var out []string
	for _, extra := range strings.Split(extras, ",") {
		out = append(out, normalizeExtraName(strings.TrimSpace(extra)))
	}
	return out
}

// stripPackageJSONDevDependencies drops author `devDependencies` from the
// deploy build's `package.json`.
//
// Hosted builds run `npm ci` with no `--omit=dev`, so UI tooling (Vite,
// wrangler, Tailwind) would otherwise be installed on Linux. Those packages
// pull optional `@emnapi/*` peers that macOS lockfile generation often omits,
// which then fails remote `npm ci`. The agent runtime does not need them.
func stripPackageJSONDevDependencies(outDir string) error {
	path := filepath.Join(outDir, "package.json")
	if !isFile(path) {
		return nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	manifest, ok, err := parseJSONObject(contents)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if !ok || !manifest.remove("devDependencies") {
		return nil
	}
	if err := os.WriteFile(path, []byte(prettyJSON(manifest)+"\n"), 0o666); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// lockInputMarker records the input that produced the current lockfile.
//
// TypeScript: the `package.json` bytes, plus vendored local package manifests,
// that generated `package-lock.json`. Rebuilds skip
// `npm install --package-lock-only` when this matches.
//
// Python: the author's `uv.lock` bytes last copied into the build.
const lockInputMarker = ".mda-lock-input"

// generateLockfile runs `npm install --package-lock-only` in the build output
// to generate a `package-lock.json` that matches the rewritten
// `package.json`. The remote builder runs `npm ci`, which requires a lockfile.
//
// Skips when `package-lock.json` already exists and was generated from the
// same manifest bytes (see [lockInputMarker]).
func generateLockfile(ctx context.Context, outDir string) error {
	pkgPath := filepath.Join(outDir, "package.json")
	if !isFile(pkgPath) {
		return nil
	}
	// npm regenerates into an existing `npm-shrinkwrap.json`, so drop it and
	// the marker that may vouch for a lock npm never wrote.
	removed, err := removeIfPresent(filepath.Join(outDir, "npm-shrinkwrap.json"))
	if err != nil {
		return err
	}
	if removed {
		if _, err := removeIfPresent(filepath.Join(outDir, lockInputMarker)); err != nil {
			return err
		}
	}
	pkg, err := os.ReadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pkgPath, err)
	}
	manifests, err := vendoredManifests(outDir, "package.json")
	if err != nil {
		return err
	}
	for _, manifest := range manifests {
		path := filepath.Join(outDir, manifest)
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		pkg = append(append(pkg, manifest...), contents...)
	}
	lockPath := filepath.Join(outDir, "package-lock.json")
	marker, err := os.ReadFile(filepath.Join(outDir, lockInputMarker))
	if isFile(lockPath) && err == nil && slices.Equal(marker, pkg) {
		return nil
	}
	return materializeLockfile(ctx, outDir, pkg)
}

// materializeLockfile writes `package-lock.json` for the build's manifests;
// tests replace it with a stub that stays off the registry.
var materializeLockfile = materializeNpmLockfile

func materializeNpmLockfile(ctx context.Context, outDir string, pkg []byte) error {
	args := []string{"install", "--package-lock-only", "--include=optional"}
	command := proc.NpmProgram() + " " + strings.Join(args, " ")
	npm := proc.Npm(args...)
	npm.Dir = outDir
	output, err := proc.OutputWithTimeout(ctx, npm, npmLockfileTimeout)
	if err != nil {
		if errs.IsCancellation(err) {
			return err
		}
		return fmt.Errorf("failed to run %s: %w", command, err)
	}
	if !output.Status.Success() {
		return errs.ChildFailed(output, command+" failed")
	}
	marker := filepath.Join(outDir, lockInputMarker)
	if err := os.WriteFile(marker, pkg, 0o666); err != nil {
		return fmt.Errorf("failed to write %s: %w", marker, err)
	}
	return nil
}

// removeIfPresent deletes path if it exists; false when it was already
// absent.
func removeIfPresent(path string) (bool, error) {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to remove %s: %w", path, err)
	}
	return true, nil
}
