package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
)

// Installation is the `managed-deepagents` package a project has installed
// (`install.rs`): the runtime compile bundles and its version.
//
// The mda CLI reads this from the environment its npm or PyPI launcher sets.
// `langsmith deploy` has no launcher, so [ResolveInstallation] finds the
// package the way the project's own code would import it.
type Installation struct {
	language      Language
	runtimeSource string
	version       string
	manifest      RuntimeManifest
	// local is a package installed from a local directory rather than a
	// registry release ([checkSupportedSDK]).
	local bool
}

// Language is the authoring language supported by this installation.
func (i *Installation) Language() Language {
	return i.language
}

// ChannelName is the registry the installed package comes from: `npm` or
// `pypi`.
func (i *Installation) ChannelName() string {
	return i.language.Ident("npm", "pypi")
}

// RuntimeSource is the absolute path to the installed runtime package: the npm
// package's `dist` directory, or the `managed_deepagents` Python package
// directory.
func (i *Installation) RuntimeSource() string {
	return i.runtimeSource
}

// PackageVersion is the installed registry package version, without build
// metadata. Use it in generated dependency manifests and compatibility checks;
// it replaces the mda CLI's `version::package_version()`.
func (i *Installation) PackageVersion() string {
	return i.version
}

// RuntimeManifest is what the installed package's manifest declares about
// the runtime compile vendors.
func (i *Installation) RuntimeManifest() RuntimeManifest {
	return i.manifest
}

// LoadProject loads the project at root, resolves its installation, and
// validates the project's declared package version against the installed one.
func LoadProject(ctx context.Context, root string) (*Project, *Installation, error) {
	p, err := Load(root)
	if err != nil {
		return nil, nil, err
	}
	installation, err := ResolveInstallation(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	if err := ValidateAuthoringPackageVersion(p, installation.PackageVersion()); err != nil {
		return nil, nil, err
	}
	return p, installation, nil
}

// ResolveInstallation finds the `managed-deepagents` package installed for p,
// and fails unless it is a release `langsmith deploy` builds for
// ([SupportedSDKRange]).
//
// TypeScript: Node resolution of `managed-deepagents/package.json` from the
// project root. Python: the project interpreter, asked where it imports
// `managed_deepagents` from and what its metadata records.
func ResolveInstallation(ctx context.Context, p *Project) (*Installation, error) {
	var (
		installation *Installation
		err          error
	)
	switch p.Language {
	case Python:
		installation, err = resolvePythonInstallation(ctx, p.Root)
	default:
		installation, err = resolveNpmInstallation(p.Root)
	}
	if err != nil {
		return nil, err
	}
	if err := validateRuntimeSource(installation.language, installation.runtimeSource); err != nil {
		return nil, err
	}
	if err := checkSupportedSDK(p.Root, installation); err != nil {
		return nil, err
	}
	return installation, nil
}

func resolveNpmInstallation(root string) (*Installation, error) {
	packageDir, ok := nodeModulesPackage(root, UserRuntimePackage)
	if !ok {
		return nil, notInstalled(TypeScript, root)
	}
	if resolved, err := filepath.EvalSymlinks(packageDir); err == nil {
		packageDir = resolved
	}
	// A package linked in from outside any `node_modules` is a source tree.
	local := !slices.Contains(strings.Split(filepath.ToSlash(packageDir), "/"), "node_modules")
	manifestPath := filepath.Join(packageDir, "package.json")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", manifestPath, err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", manifestPath, err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return nil, errs.Categorized(errs.Project, fmt.Sprintf(
			"the managed-deepagents installation is incomplete: %s has no version; %s",
			manifestPath, reinstallHint(TypeScript)))
	}
	runtime, err := parseNpmRuntimeManifest(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to read the runtime of %s: %w", manifestPath, err)
	}
	return &Installation{
		language:      TypeScript,
		runtimeSource: filepath.Join(packageDir, "dist"),
		version:       manifest.Version,
		manifest:      runtime,
		local:         local,
	}, nil
}

// nodeModulesPackage finds name the way Node resolves a bare specifier: in the
// `node_modules` of root and of each directory above it.
func nodeModulesPackage(root, name string) (string, bool) {
	for dir := root; ; {
		if filepath.Base(dir) != "node_modules" {
			candidate := filepath.Join(dir, "node_modules", filepath.FromSlash(name))
			if isFile(filepath.Join(candidate, "package.json")) {
				return candidate, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// pythonInstallationProbe prints where the interpreter imports
// `managed_deepagents` from and what the installed distribution's metadata
// records (its version, requirements, and whether it was installed from a
// local directory, PEP 610), without importing the package.
const pythonInstallationProbe = `import importlib.metadata, importlib.util, json
spec = importlib.util.find_spec("managed_deepagents")
locations = list(spec.submodule_search_locations or []) if spec else []
try:
    dist = importlib.metadata.distribution("managed-deepagents")
except importlib.metadata.PackageNotFoundError:
    dist = None
direct_url = json.loads(dist.read_text("direct_url.json") or "{}") if dist else {}
print(json.dumps({
    "package_dir": locations[0] if locations else None,
    "version": dist.version if dist else None,
    "requires": (dist.requires or []) if dist else [],
    "requires_python": dist.metadata["Requires-Python"] if dist else None,
    "local": "dir_info" in direct_url,
}))
`

func resolvePythonInstallation(ctx context.Context, root string) (*Installation, error) {
	python, err := proc.ProjectPython(ctx, root)
	if err != nil {
		return nil, notInstalled(Python, root)
	}
	cmd := exec.Command(python, "-I", "-c", pythonInstallationProbe)
	cmd.Dir = root
	output, err := proc.Output(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run %s: %w", python, err)
	}
	if !output.Status.Success() {
		return nil, errs.WithCategory(errs.Toolchain,
			errs.ChildFailed(output, fmt.Sprintf("failed to locate managed-deepagents with %s", python)))
	}
	var found struct {
		PackageDir     *string  `json:"package_dir"`
		Version        *string  `json:"version"`
		Requires       []string `json:"requires"`
		RequiresPython *string  `json:"requires_python"`
		Local          bool     `json:"local"`
	}
	if err := json.Unmarshal(output.Stdout, &found); err != nil {
		return nil, fmt.Errorf("failed to parse the managed-deepagents location from %s: %w", python, err)
	}
	if found.PackageDir == nil || found.Version == nil {
		return nil, notInstalled(Python, root)
	}
	manifest := RuntimeManifest{Dependencies: pythonRuntimeDependencies(found.Requires)}
	if found.RequiresPython != nil {
		manifest.RequiresPython = *found.RequiresPython
	}
	return &Installation{
		language:      Python,
		runtimeSource: *found.PackageDir,
		version:       *found.Version,
		manifest:      manifest,
		local:         found.Local,
	}, nil
}

func validateRuntimeSource(language Language, source string) error {
	marker := filepath.Join(source, language.Ident(filepath.Join("runtime", "index.js"), "runtime.py"))
	if isFile(marker) {
		return nil
	}

	return errs.Categorized(errs.Project, fmt.Sprintf(
		"the managed-deepagents installation is incomplete: bundled runtime not found at %s "+
			"(expected %s); %s",
		source, marker, reinstallHint(language)))
}

// notInstalled is the error for a project whose dependencies do not include an
// installed `managed-deepagents`.
func notInstalled(language Language, root string) error {
	return errs.Categorized(errs.Project, fmt.Sprintf(
		"managed-deepagents is not installed for the project at %s.\n"+
			"Install the project's dependencies with `%s`, then try again.",
		root, installCommand(language)))
}

// installCommand installs a project's dependencies, `managed-deepagents`
// among them.
func installCommand(language Language) string {
	return language.Ident("npm install", "uv sync")
}

func reinstallHint(language Language) string {
	return fmt.Sprintf("reinstall it with `%s`", installCommand(language))
}
