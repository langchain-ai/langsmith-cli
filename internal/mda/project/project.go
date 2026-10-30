// Package project discovers a Managed Deep Agent project on disk and the
// facts about it that compile and deploy share (`project.rs`, `naming.rs`,
// `context_layout.rs`, `install.rs`, `version.rs`, `dependencies.rs`,
// `local_context_hub.rs`, `deploy/env_file.rs`).
//
// Load scans the project root, collects every file that should be carried into
// the build, and locates the agent entry (`agent.ts`/`agent.tsx`/`agent.py`)
// that exports the named `agent` defined via `defineDeepAgent` /
// `define_deep_agent`.
//
// The mda CLI is started by a launcher in the npm or PyPI package, which tells
// it where its bundled runtime is and which version it is. `langsmith deploy`
// has no launcher: [LoadProject] loads the project and resolves an
// [Installation] from the `managed-deepagents` package the project itself has
// installed. The installed version is [Installation.PackageVersion], which
// stands in for the Rust `version::package_version()`; functions that pin it
// ([PackageJSONEntries], [PyprojectEntries]) take it as an argument.
package project

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// junkDirs are tooling / OS cache directories that are never meaningful source
// or skill content.
var junkDirs = []string{"__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache"}

// junkFiles are OS junk files that are often binary (and abort UTF-8 Context
// Hub sync).
var junkFiles = []string{".DS_Store", "Thumbs.db", "desktop.ini"}

// ignoredDirs are directories never carried into the compiled output.
//
// Includes project-layout names (`build`, `dist`, legacy `mda_evals`, `evals`,
// …) that must not be applied when walking `skills/**` — use [IsJunkDir] there
// instead.
var ignoredDirs = []string{
	"node_modules",
	".git",
	".mda",
	"mda_evals",
	"evals",
	".deepagents",
	".agents",
	".claude",
	".cursor",
	".factory",
	".gemini",
	".opencode",
	"memories",
	"dist",
	"build",
	// Python virtual environments: platform-specific and reinstalled from the
	// generated dependency manifest on deploy. Carrying them bloats the build and
	// lets vendored packages (e.g. a nested `connectors/`) get misdiscovered.
	".venv",
	"venv",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	".ruff_cache",
}

// ignoredFiles are files never carried into the compiled output.
//
// Other lockfiles would override the installer and the lock generated for the
// rewritten `package.json`.
var ignoredFiles = []string{
	".env",
	".DS_Store",
	"Thumbs.db",
	"desktop.ini",
	"pnpm-lock.yaml",
	"yarn.lock",
	"bun.lock",
	"bun.lockb",
	"npm-shrinkwrap.json",
}

// agentEntrypoint is a candidate agent entry file paired with its language.
type agentEntrypoint struct {
	name     string
	language Language
}

// agentEntrypoints are the candidate agent entry files, in priority order.
var agentEntrypoints = []agentEntrypoint{
	{"agent.ts", TypeScript},
	{"agent.tsx", TypeScript},
	{"agent.py", Python},
}

// Language is the authoring language of a project.
type Language int

// The supported authoring languages.
const (
	TypeScript Language = iota
	Python
)

// Label is the human-readable label for CLI messaging.
func (l Language) Label() string {
	switch l {
	case Python:
		return "Python"
	default:
		return "TypeScript"
	}
}

// Extension is the source file extension (no dot) for authored modules.
func (l Language) Extension() string {
	return l.Ident("ts", "py")
}

// ManifestName is the package manifest file name for projects in this
// language.
func (l Language) ManifestName() string {
	return l.Ident("package.json", "pyproject.toml")
}

// Ident is the spelling an author uses for an API name in this language.
//
// CLI messages quote authored code, so a Python project must never be told to
// edit `defineDeepAgent` (and a TypeScript one never `define_deep_agent`).
func (l Language) Ident(typescript, python string) string {
	switch l {
	case Python:
		return python
	default:
		return typescript
	}
}

// StringArgument is a string argument written the way this language passes it
// (`name: "research-assistant"` / `name="research-assistant"`).
func (l Language) StringArgument(key, value string) string {
	switch l {
	case Python:
		return fmt.Sprintf(`%s="%s"`, key, value)
	default:
		return fmt.Sprintf(`%s: "%s"`, key, value)
	}
}

// Project is a loaded Managed Deep Agent project.
type Project struct {
	// Root is the absolute path to the project root.
	Root string
	// Language is the detected project language.
	Language Language
	// AgentFile is the agent entry file, relative to Root.
	AgentFile string
	// Files are all files to carry into the build, relative to Root.
	Files []string
}

// Load loads and validates the project rooted at root.
func Load(root string) (*Project, error) {
	if err := EnsureProjectDir(root); err != nil {
		return nil, err
	}
	resolved, err := canonicalize(root)
	if err != nil {
		return nil, errs.Categorized(errs.Project,
			fmt.Sprintf("failed to resolve project root %s: %v", root, err))
	}
	root = resolved

	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && IsIgnoredDir(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !IsIgnoredFile(rel) {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(files, comparePaths)

	hasTypeScriptEntry := slices.ContainsFunc(agentEntrypoints, func(entry agentEntrypoint) bool {
		return entry.language == TypeScript && slices.Contains(files, entry.name)
	})
	hasPythonEntry := slices.Contains(files, "agent.py")
	if hasTypeScriptEntry && hasPythonEntry {
		return nil, errs.Categorized(errs.Project, fmt.Sprintf(
			"ambiguous project in %s: found both TypeScript and Python agent entry files", root))
	}

	index := slices.IndexFunc(agentEntrypoints, func(entry agentEntrypoint) bool {
		return slices.Contains(files, entry.name)
	})
	if index < 0 {
		return nil, notAProject(root)
	}

	return &Project{
		Root:      root,
		Language:  agentEntrypoints[index].language,
		AgentFile: agentEntrypoints[index].name,
		Files:     files,
	}, nil
}

// EnsureProjectDir rejects path unless it is a directory holding an agent
// entry file.
//
// Every project command runs this before any other work, so a wrong working
// directory gets the same message, with the likeliest fix, from all of them.
func EnsureProjectDir(path string) error {
	if !isDir(path) {
		return errs.Categorized(errs.Project, fmt.Sprintf("project root %s is not a directory", path))
	}
	if !hasAgentEntry(path) {
		return notAProject(path)
	}
	return nil
}

// hasAgentEntry reports whether dir directly holds an agent entry file.
func hasAgentEntry(dir string) bool {
	return slices.ContainsFunc(agentEntrypoints, func(entry agentEntrypoint) bool {
		return isFile(filepath.Join(dir, entry.name))
	})
}

// notAProject is the error for a directory without an agent entry.
func notAProject(dir string) error {
	if resolved, err := canonicalize(dir); err == nil {
		dir = resolved
	}
	cwd := ""
	if wd, err := os.Getwd(); err == nil {
		if resolved, err := canonicalize(wd); err == nil {
			cwd = resolved
		}
	}
	return errs.Categorized(errs.Project, notAProjectMessage(dir, cwd))
}

// notAProjectMessage explains that dir is not a project and names the
// likeliest way to reach one.
//
// cwd decides how the suggested `cd` is spelled: relative when dir is the
// working directory, absolute otherwise. An empty cwd is unknown.
func notAProjectMessage(dir, cwd string) string {
	names := make([]string, len(agentEntrypoints))
	for i, entry := range agentEntrypoints {
		names[i] = entry.name
	}
	expected := strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]

	inCwd := cwd != "" && cwd == dir
	var hint string
	switch nearby := findNearbyProject(dir); {
	case nearby == nil:
		hint = "Run `mda init <name>` to create one, or `cd` into an existing project."
	case nearby.child != "":
		target := nearby.child
		if inCwd {
			target = filepath.Base(nearby.child)
		}
		hint = fmt.Sprintf("Found a project in %s. Run `cd %s` first.", target, shellWord(target))
	default:
		target := nearby.root
		if inCwd {
			target = filepath.Join(slices.Repeat([]string{".."}, nearby.depth)...)
		}
		hint = fmt.Sprintf("This directory is inside the project at %s. Run `cd %s` first.",
			nearby.root, shellWord(target))
	}
	return fmt.Sprintf("%s is not a Managed Deep Agent project (no %s found).\n%s", dir, expected, hint)
}

// nearbyProject is a project next to the directory a command was pointed at:
// either child or root is set.
type nearbyProject struct {
	// child is the only immediate subdirectory holding an agent entry: the
	// usual result of forgetting to `cd` after `mda init`.
	child string
	// root is the closest enclosing project, depth levels up (for example when
	// working in its `tools/` directory).
	root  string
	depth int
}

func findNearbyProject(dir string) *nearbyProject {
	entries, _ := os.ReadDir(dir) // An unreadable dir has no children to suggest.
	var children []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if isDir(path) &&
			!IsIgnoredDir(path) &&
			!strings.HasPrefix(entry.Name(), ".") &&
			hasAgentEntry(path) {
			children = append(children, path)
		}
	}
	if len(children) == 1 {
		return &nearbyProject{child: children[0]}
	}
	for current, depth := dir, 0; ; {
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
		depth++
		if hasAgentEntry(current) {
			return &nearbyProject{root: current, depth: depth}
		}
	}
}

// shellWord is path as a single shell word, quoted only when it needs to be.
func shellWord(path string) string {
	plain := !strings.ContainsFunc(path, func(c rune) bool {
		return !(c < 0x80 && (isASCIIAlphanumeric(byte(c)) || strings.ContainsRune("/._-", c)))
	})
	switch {
	case plain:
		return path
	case runtime.GOOS == "windows":
		return "'" + strings.ReplaceAll(path, "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
	}
}

// ValidateAuthoringPackageVersion rejects an exact Authoring Package version
// that differs from the installed one.
//
// Ranges and non-registry sources are intentionally left to their package
// managers; this check only handles the exact pins generated by `mda init`.
func ValidateAuthoringPackageVersion(p *Project, expected string) error {
	var (
		declared string
		ok       bool
		err      error
	)
	switch p.Language {
	case Python:
		declared, ok, err = declaredPythonAuthoringVersion(p.Root)
	default:
		declared, ok, err = declaredTypeScriptAuthoringVersion(p.Root)
	}
	if err != nil {
		return err
	}
	if !ok || declared == expected {
		return nil
	}

	return errs.Categorized(errs.Project, fmt.Sprintf(
		"managed-deepagents version mismatch: project declares %s, but %s is installed.\n"+
			"\n"+
			"The project's installed dependencies are out of date. Install them again:\n"+
			"\n"+
			"  %s",
		declared, expected, installCommand(p.Language)))
}

func declaredTypeScriptAuthoringVersion(root string) (string, bool, error) {
	path := filepath.Join(root, "package.json")
	if !isFile(path) {
		return "", false, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var manifest any
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return "", false, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	for _, section := range []string{"dependencies", "devDependencies"} {
		dependencies, _ := objectField(manifest, section)
		field, _ := objectField(dependencies, UserRuntimePackage)
		specifier, ok := field.(string)
		if !ok {
			continue
		}
		version, ok := exactRegistryVersion(specifier)
		return version, ok, nil
	}
	return "", false, nil
}

func declaredPythonAuthoringVersion(root string) (string, bool, error) {
	path := filepath.Join(root, "pyproject.toml")
	if !isFile(path) {
		return "", false, nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to read %s: %w", path, err)
	}
	var manifest map[string]any
	if err := toml.Unmarshal(contents, &manifest); err != nil {
		return "", false, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	project, _ := objectField(manifest, "project")
	field, _ := objectField(project, "dependencies")
	dependencies, ok := field.([]any)
	if !ok {
		return "", false, nil
	}

	for _, item := range dependencies {
		dependency, ok := item.(string)
		if !ok {
			continue
		}
		name, version, ok := strings.Cut(dependency, "==")
		if !ok || canonicalPackageName(strings.TrimSpace(name)) != UserRuntimePackage {
			continue
		}
		version, ok = exactRegistryVersion(version)
		return version, ok, nil
	}
	return "", false, nil
}

// objectField is value[key] when value is an object holding key.
func objectField(value any, key string) (any, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	field, ok := object[key]
	return field, ok
}

func exactRegistryVersion(specifier string) (string, bool) {
	version := strings.TrimSpace(specifier)
	valid := version != "" &&
		version[0] >= '0' && version[0] <= '9' &&
		!strings.ContainsFunc(version, func(c rune) bool {
			return !(c < 0x80 && (isASCIIAlphanumeric(byte(c)) || strings.ContainsRune(".+-", c)))
		})
	return version, valid
}

func canonicalPackageName(name string) string {
	var out strings.Builder
	for i := range len(name) {
		switch c := name[i]; {
		case c == '_' || c == '.':
			out.WriteByte('-')
		case c >= 'A' && c <= 'Z':
			out.WriteByte(c + ('a' - 'A'))
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// IsIgnoredDir reports whether path's final component is a directory that
// should be skipped when scanning a project for compile/deploy inputs.
func IsIgnoredDir(path string) bool {
	return slices.Contains(ignoredDirs, filepath.Base(path))
}

// IsIgnoredFile reports whether path's final component is a file that should
// be skipped when scanning a project for compile/deploy inputs.
func IsIgnoredFile(path string) bool {
	name := filepath.Base(path)
	return slices.Contains(ignoredFiles, name) ||
		strings.HasPrefix(name, ".env.") ||
		isBytecodeFileName(name)
}

// IsJunkDir reports whether path's final component is a tooling/OS cache
// directory.
//
// Safe to apply under `skills/**`, unlike [IsIgnoredDir], which also skips
// ordinary names such as `build` / `dist` / `evals` / `memories`.
func IsJunkDir(path string) bool {
	return slices.Contains(junkDirs, filepath.Base(path))
}

// IsJunkFile reports whether path's final component is OS junk or Python
// bytecode.
//
// Safe to apply under `skills/**` (does not treat project secrets/layout rules
// as skill denylist entries beyond binary OS junk).
func IsJunkFile(path string) bool {
	name := filepath.Base(path)
	return slices.Contains(junkFiles, name) || isBytecodeFileName(name)
}

func isBytecodeFileName(name string) bool {
	return strings.HasSuffix(name, ".pyc") || strings.HasSuffix(name, ".pyo")
}

// comparePaths orders relative paths component by component, as Rust's
// `PathBuf` does, so `a/b` sorts before `a-c`.
func comparePaths(a, b string) int {
	return slices.CompareFunc(
		strings.Split(a, string(filepath.Separator)),
		strings.Split(b, string(filepath.Separator)),
		cmp.Compare[string],
	)
}

func isASCIIAlphanumeric(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// canonicalize is the absolute path with every symlink resolved.
func canonicalize(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// readToString reads path as UTF-8 text, failing on invalid UTF-8 like Rust's
// `fs::read_to_string`.
func readToString(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(contents) {
		return "", errors.New("stream did not contain valid UTF-8")
	}
	return string(contents), nil
}
