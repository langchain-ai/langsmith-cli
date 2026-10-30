// Package compile compiles a loaded project into a deployable managed
// LangGraph app (`compile/*` of the mda CLI).
//
// The output directory is a self-contained build artifact. Every user file is
// carried over verbatim — the agent file is **never** rewritten (aside from
// TypeScript text-module inlining). Instead, the CLI generates a small managed
// entry module that consumes the user's `defineDeepAgent(...)` definition and
// plugs it into an `agent(...)` graph factory, with the platform-owned
// backend, store, and checkpointer injected by the managed runtime.
// `langgraph.json` points at that generated entry.
//
// Everything compile reads from authored source comes from `introspect`, which
// imports the project's modules instead of parsing them: the agent config,
// identity, memory, sandbox, schedules, the exported connectors and channels
// and their requirements. The Rust CLI re-exported several of those from
// `compile`; here they are called on `introspect` directly, and compile
// aliases only the types its own API uses.
//
// Both agent forms build through the same entry: a static definition is
// checked with `assertStaticAgentDefinition` / `assert_static_agent_definition`
// before it is compiled, and a named factory (`defineAgent(name, factory)`
// / `define_agent(name, factory)`) is callable, so the runtime calls it
// per run like any factory.
//
// The generated files (entry modules, `langgraph.json`, manifests) are byte
// for byte what the mda CLI writes for the same project: JSON follows
// `serde_json` with `preserve_order` and TOML edits follow `toml_edit`.
//
// Only the builds `langsmith deploy` and its connection checks run are ported:
// [CompileForDeploy] and the local [Compile]. The mda CLI's in-place refresh
// for `mda dev` and its eval builds are not.
package compile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/introspect"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
	"github.com/langchain-ai/langsmith-cli/internal/mda/ui"
)

// Installation is what compile needs of the installed `managed-deepagents`:
// the runtime it vendors, the version it pins, and what the package's manifest
// declares about that runtime. [*project.Installation] implements it.
type Installation interface {
	RuntimeSource() string
	PackageVersion() string
	RuntimeManifest() project.RuntimeManifest
}

// MemoryDecision is what memory a project has ([introspect.ResolveMemory]).
type MemoryDecision = introspect.MemoryDecision

// IdentitySummary is the root identity declaration of a project.
type IdentitySummary = introspect.IdentitySummary

// nodeVersion is the LangGraph Node.js runtime version targeted by compiled
// TypeScript apps.
//
// LangGraph's `langgraph.json` schema (enforced by both the local dev server
// and the remote build) only accepts `"20"` or `"22"`, so this must stay on a
// supported LTS line — `"24"` is rejected with a Zod validation error before
// the server starts.
const nodeVersion = "22"

// pythonSeries is a Python `major.minor` series.
type pythonSeries struct {
	major, minor uint32
}

// supportedPythonVersions are the Python runtimes langgraph-api publishes
// images for, **lowest first**.
//
// The low end mirrors `MIN_PYTHON_VERSION` in `langgraph_cli/config.py`, which
// validates `python_version` against a 3.11 floor. The high end is what the
// registry actually carries for [LanggraphAPIVersion] — every entry must have
// a published `{stem}-py{version}` tag, so extend this list when langgraph-api
// starts shipping a newer series. Resolution picks the newest entry the
// project allows unless an explicit deployment target is set.
var supportedPythonVersions = []pythonSeries{{3, 11}, {3, 12}, {3, 13}, {3, 14}}

// defaultPythonVersion is the runtime built when a project declares no
// `requires-python`.
var defaultPythonVersion = pythonSeries{3, 14}

// PythonVersionEnv is the environment variable that overrides the Python
// runtime for one build.
//
// Overrides `[tool.mda].python-version` without editing project
// configuration.
const PythonVersionEnv = "MDA_PYTHON_VERSION"

// LanggraphAPIVersionEnv is the environment variable that opts into a
// non-default langgraph-api image stem.
//
// Value is a version stem only (e.g. `0.12.0rc9`); compile appends
// `-node{N}` / `-py{V}` when writing `_INTERNAL_docker_tag`.
const LanggraphAPIVersionEnv = "MDA_LANGGRAPH_API_VERSION"

// LanggraphAPIVersion is the default langgraph-api version stem forced into
// compiled `langgraph.json` image pins. Override with
// [LanggraphAPIVersionEnv].
const LanggraphAPIVersion = "0.18.0.dev12"

// LanggraphRuntimeInmemVersion is the in-memory runtime `mda dev` pairs with
// [LanggraphAPIVersion]. A pre-release Agent Server pulls a pre-release
// runtime, which uv only accepts when pinned explicitly, so this moves
// together with the version above.
const LanggraphRuntimeInmemVersion = "0.38.0.dev12"

// resolveLanggraphAPIVersion resolves the langgraph-api version stem for
// `_INTERNAL_docker_tag`.
//
// Prefers a non-empty overrideStem (typically from [LanggraphAPIVersionEnv]);
// otherwise the default.
func resolveLanggraphAPIVersion(overrideStem string) (string, error) {
	raw := strings.TrimFunc(overrideStem, isRustWhitespace)
	if raw == "" {
		return LanggraphAPIVersion, nil
	}
	return validateLanggraphAPIVersionStem(raw)
}

// ResolvedLanggraphAPIVersion is the langgraph-api version stem this build
// uses: [LanggraphAPIVersionEnv] when set, else [LanggraphAPIVersion].
func ResolvedLanggraphAPIVersion() (string, error) {
	return resolveLanggraphAPIVersion(os.Getenv(LanggraphAPIVersionEnv))
}

// validateLanggraphAPIVersionStem rejects empty, whitespace-containing, or
// fully-qualified tag values.
func validateLanggraphAPIVersionStem(stem string) (string, error) {
	if strings.IndexFunc(stem, isRustWhitespace) >= 0 {
		return "", fmt.Errorf("%s must not contain whitespace (got %q)", LanggraphAPIVersionEnv, stem)
	}
	if strings.Contains(stem, "/") || strings.Contains(stem, "-node") || strings.Contains(stem, "-py") {
		return "", fmt.Errorf("%s must be a version stem only (e.g. 0.12.0rc9), "+
			"not a full docker tag (got %q)", LanggraphAPIVersionEnv, stem)
	}
	return stem, nil
}

// ResolvedPythonVersion resolves the deployment target from the environment,
// `[tool.mda]`, or the newest supported series allowed by `requires-python`,
// in that order.
func ResolvedPythonVersion(projectRoot string) (ResolvedPython, error) {
	config, err := readPythonVersionConfig(projectRoot)
	if err != nil {
		return ResolvedPython{}, err
	}
	var envOverride *string
	if value, ok := os.LookupEnv(PythonVersionEnv); ok {
		envOverride = &value
	}
	return resolvePythonVersion(config.requiresPython, config.pythonVersion, envOverride)
}

// ResolvedPython is a resolved runtime, plus anything the author should know
// about how it was chosen.
type ResolvedPython struct {
	// Version is `major.minor`, as written into `langgraph.json`.
	Version string
	// Warning is set when the specifier asked for precision an image tag
	// cannot carry. Returned rather than printed so resolution stays pure and
	// testable.
	Warning string
}

func resolvePythonVersion(requiresPython, configuredVersion, envOverride *string) (ResolvedPython, error) {
	spec := ""
	if requiresPython != nil {
		spec = strings.TrimFunc(*requiresPython, isRustWhitespace)
	}
	var specifiers []versionSpecifier
	if spec != "" {
		parsed, err := parseVersionSpecifiers(spec)
		if err != nil {
			return ResolvedPython{}, fmt.Errorf("failed to parse requires-python %q in pyproject.toml: %w", spec, err)
		}
		specifiers = parsed
	}

	var raw, source string
	explicit := false
	if envOverride != nil {
		if value := strings.TrimFunc(*envOverride, isRustWhitespace); value != "" {
			raw, source, explicit = value, PythonVersionEnv, true
		}
	}
	if !explicit && configuredVersion != nil {
		raw, source, explicit = strings.TrimFunc(*configuredVersion, isRustWhitespace), "[tool.mda] python-version", true
	}
	var selected pythonSeries
	switch {
	case explicit:
		parsed, err := parsePythonVersion(raw, source)
		if err != nil {
			return ResolvedPython{}, err
		}
		if !slices.Contains(supportedPythonVersions, parsed) {
			return ResolvedPython{}, errors.New(unsupportedMessage(parsed, source))
		}
		if spec != "" && !seriesCanSatisfy(specifiers, parsed) {
			return ResolvedPython{}, fmt.Errorf("%s=%s contradicts requires-python %q in pyproject.toml; "+
				"`pip install -e .` would fail in the built image", source, raw, spec)
		}
		selected = parsed
	case spec != "":
		found := false
		for i := len(supportedPythonVersions) - 1; i >= 0; i-- {
			if seriesCanSatisfy(specifiers, supportedPythonVersions[i]) {
				selected, found = supportedPythonVersions[i], true
				break
			}
		}
		if !found {
			return ResolvedPython{}, fmt.Errorf("requires-python %q in pyproject.toml allows no Python that "+
				"LangGraph ships an image for (%s)", spec, supportedVersionsList())
		}
	default:
		selected = defaultPythonVersion
	}
	resolved := ResolvedPython{Version: formatPythonVersion(selected)}
	if spec != "" && !seriesAlwaysSatisfies(specifiers, selected) {
		resolved.Warning = fmt.Sprintf("requires-python %q constrains the patch level, which "+
			"LangGraph images do not pin; building Python %s, whose patch comes "+
			"from the image and may not satisfy it", spec, formatPythonVersion(selected))
	}
	return resolved, nil
}

// seriesCanSatisfy reports whether any stable patch in the series could
// satisfy the specifiers.
func seriesCanSatisfy(specifiers []versionSpecifier, series pythonSeries) bool {
	return slices.ContainsFunc(seriesPatchBoundaries(specifiers, series), func(probe []uint64) bool {
		return specifiersContain(specifiers, releaseVersion(probe...))
	})
}

// seriesAlwaysSatisfies reports whether every stable patch in the series
// satisfies the specifiers.
func seriesAlwaysSatisfies(specifiers []versionSpecifier, series pythonSeries) bool {
	return !slices.ContainsFunc(seriesPatchBoundaries(specifiers, series), func(probe []uint64) bool {
		return !specifiersContain(specifiers, releaseVersion(probe...))
	})
}

// seriesPatchBoundaries are the patch versions to probe: for stable X.Y.patch
// versions, a constraint can change membership only at its patch boundary or
// the next patch. Checking those points and patch zero covers each interval,
// including exact pins, exclusions, and bounded ranges. The PEP 440 library
// owns all comparison semantics; no image patch is assumed here.
func seriesPatchBoundaries(specifiers []versionSpecifier, series pythonSeries) [][]uint64 {
	patches := []uint64{0}
	for _, specifier := range specifiers {
		release := specifier.release
		if len(release) >= 2 && release[0] == uint64(series.major) && release[1] == uint64(series.minor) {
			var patch uint64
			if len(release) > 2 {
				patch = release[2]
			}
			patches = append(patches, patch)
			if patch < ^uint64(0) {
				patches = append(patches, patch+1)
			}
		}
	}
	slices.Sort(patches)
	patches = slices.Compact(patches)
	probes := make([][]uint64, len(patches))
	for i, patch := range patches {
		probes[i] = []uint64{uint64(series.major), uint64(series.minor), patch}
	}
	return probes
}

func formatPythonVersion(series pythonSeries) string {
	return fmt.Sprintf("%d.%d", series.major, series.minor)
}

func supportedVersionsList() string {
	versions := make([]string, len(supportedPythonVersions))
	for i, series := range supportedPythonVersions {
		versions[i] = formatPythonVersion(series)
	}
	return strings.Join(versions, ", ")
}

// parsePythonVersion parses a bare `major.minor` runtime version from an
// explicit override.
//
// Rebuilding the output from the parsed integers (see [formatPythonVersion])
// is what keeps the value safe to interpolate into `_INTERNAL_docker_tag`:
// registry syntax (`/`, `:`, `@`), whitespace, and tag suffixes like `-wolfi`
// cannot survive it, so an override can never redirect the image reference
// somewhere else.
func parsePythonVersion(raw, source string) (pythonSeries, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return pythonSeries{}, fmt.Errorf("invalid Python version %q from %s: use bare `major.minor` "+
			"(e.g. \"3.14\"), without a patch version or image suffix", raw, source)
	}
	major, majorErr := parseRustU32(parts[0])
	minor, minorErr := parseRustU32(parts[1])
	if majorErr != nil || minorErr != nil {
		return pythonSeries{}, fmt.Errorf("invalid Python version %q from %s: `major.minor` components "+
			"must be numbers (e.g. \"3.14\")", raw, source)
	}
	return pythonSeries{major, minor}, nil
}

// parseRustU32 parses like Rust's `u32::from_str`, which accepts one leading
// `+`.
func parseRustU32(text string) (uint32, error) {
	digits := strings.TrimPrefix(text, "+")
	if digits == "" || strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
		return 0, strconv.ErrSyntax
	}
	value, err := strconv.ParseUint(digits, 10, 32)
	return uint32(value), err
}

func unsupportedMessage(series pythonSeries, source string) string {
	return fmt.Sprintf("Python %d.%d from %s is not supported; LangGraph ships images for %s",
		series.major, series.minor, source, supportedVersionsList())
}

// entryStem is the basename (without extension) of the generated managed
// entry module.
const entryStem = "_mda_entry"

// connectorsStem is the basename (without extension) of the generated
// connectors aggregator module.
const connectorsStem = "_mda_connectors"

// channelsStem is the basename (without extension) of the generated channels
// aggregator module.
const channelsStem = "_mda_channels"

// identityExport is the named export the root `identity.{ts,py}` declaration
// exposes.
const identityExport = "identity"

// memoryExport is the canonical export name for the root memory declaration,
// in both languages.
const memoryExport = "memory"

const browserExport = "browser"

// browserHandoffGraph is the graph key of the browser handoff watcher, whose
// run waits out a handoff; the runtime's `HANDOFF_WATCHER_GRAPH` names it too.
const browserHandoffGraph = "mda-browser-handoff"

// browserHandoffExport is the generated entry's export of that graph, in both
// languages.
const browserHandoffExport = "browser_handoff_watcher"

// authStem is the basename (without extension) of the generated managed
// custom-auth module.
const authStem = "_mda_auth"

const managedHTTPStem = "_mda_http"

// generatedPrefix is the filename prefix shared by every module the compiler
// generates.
const generatedPrefix = "_mda_"

// buildMarker stamps a directory as build output this CLI owns and may delete.
const buildMarker = ".mda-build"

const buildMarkerContents = "# Written by `mda`. This directory is generated build output: everything in it\n" +
	"# is deleted and rewritten on the next build. Do not keep sources here.\n"

// fullPreserve are the top-level dependency caches a build keeps when it
// clears the build directory, so a deploy does not reinstall them from
// scratch. Never `.env` files: a build must not carry staged secrets forward.
// The deploy archive omits these directories.
var fullPreserve = []string{"node_modules", PythonEmitSiteDir}

// runtimeDir is the subdirectory of the build that holds the vendored runtime.
const runtimeDir = "__runtime__"

// runtimePackage is the package name of the managed runtime dependency.
const runtimePackage = project.UserRuntimePackage

// CompileSummary is the summary of a successful compilation.
type CompileSummary struct {
	OutDir string
	// Copied are the user files carried into the build verbatim, relative to
	// the root.
	Copied []string
	// AssistantID is the LangGraph `graphs` key / assistant id from the
	// authored name.
	AssistantID string
	// Connectors are the connector modules (the `tools/mcp.{ts,py}`
	// declaration), relative to the root. Their HTTP routes are enumerated at
	// runtime, not here.
	Connectors []string
	// Channels are the channel modules discovered under `channels/`, relative
	// to the root.
	Channels []string
	// ChannelProviders are the first-party ingress channel providers
	// (`slack`, `github`) those modules declare, sorted.
	ChannelProviders []string
	// Identity is the root `identity.{ts,py}` declaration, when present.
	Identity *IdentitySummary
	// Memory is what durable memory this project declares. Reports scope and
	// file only — never memory contents.
	Memory MemoryDecision
	// Sandbox reports whether the project uses a managed sandbox: a
	// declaration, or a factory `setup.sh` recipe.
	Sandbox bool
	// HasSkills reports whether deploy-owned skills were synced into the
	// Context Hub mock / repo.
	HasSkills bool
	// ScheduleCrons are the managed schedules discovered under `schedules/`,
	// keyed by file-derived name.
	ScheduleCrons ScheduleCrons
	// Warnings are non-fatal advisories about the project's shape, for the
	// caller to render in its own idiom. Compile never prints them itself: it
	// runs inside a spinner under `langsmith deploy`.
	Warnings []string
}

func buildResultItems(outDir, projectPath string, warnings []string) []ui.SequenceItem {
	command := func(program, verb string) string {
		if projectPath == "." {
			return program + " " + verb
		}
		return program + " " + verb + " " + projectPath
	}
	items := []ui.SequenceItem{ui.NewSection("Managed app compiled", outDir)}
	if len(warnings) > 0 {
		items = append(items, ui.NewSection("Build warnings", warnings...))
	}
	return append(items,
		ui.NewSection("Run locally", command("mda", "dev")),
		ui.NewSection("Deploy when ready", command("langsmith", "deploy")),
	)
}

// ShowBuildSummary shows the compiled app and the same action headings used by
// `mda init`.
func ShowBuildSummary(summary *CompileSummary, projectPath string) error {
	return ui.ShowSequence(buildResultItems(summary.OutDir, projectPath, summary.Warnings))
}

// DeployContext is the Context Hub metadata the generated deploy entry passes
// to the managed runtime.
type DeployContext struct {
	// Repo is the Context Hub repo handle mounted by `ContextHubBackend`.
	Repo string
	// HasSkills reports whether the synced repo contains any deploy-owned
	// skills.
	HasSkills bool
}

// Compile compiles p into outDir for a local build, replacing any previous
// build output. The Python `file://` dependency points at the canonicalized
// outDir.
func Compile(ctx context.Context, p *project.Project, outDir string, installation Installation) (*CompileSummary, error) {
	// One decision, read before the hub is touched: it gates the local
	// hot-file seed *and* what the generated entry mounts.
	memory, err := introspect.ResolveMemory(ctx, p)
	if err != nil {
		return nil, err
	}
	hub, err := project.EnsureLocalContextHub(p, memory.AgentEnabled())
	if err != nil {
		return nil, err
	}
	if !utf8.ValidString(hub.Root) {
		return nil, fmt.Errorf("non-UTF-8 local Context Hub path %s", hub.Root)
	}
	apiVersion, err := ResolvedLanggraphAPIVersion()
	if err != nil {
		return nil, err
	}
	return compileInner(ctx, p, outDir, runtimeContext{
		kind:        runtimeContextLocal,
		contextRoot: hub.Root,
		hasSkills:   hub.HasSkills,
	}, installation, memory, apiVersion)
}

// Compilation reports an unclassified compile failure as `compilation`,
// keeping its causes so a child's stderr still reaches the error record. A
// cancellation passes through unchanged.
func Compilation(err error) error {
	return errs.CategorizeUnclassified(errs.Compilation, err)
}

// CompileForDeploy compiles p for deployment, replacing local embedded context
// with Context Hub metadata consumed by the deployed runtime.
func CompileForDeploy(
	ctx context.Context,
	p *project.Project,
	outDir string,
	deploy DeployContext,
	installation Installation,
) (*CompileSummary, error) {
	memory, err := introspect.ResolveMemory(ctx, p)
	if err != nil {
		return nil, err
	}
	apiVersion, err := ResolvedLanggraphAPIVersion()
	if err != nil {
		return nil, err
	}
	return compileInner(ctx, p, outDir, runtimeContext{
		kind:      runtimeContextContextHub,
		repo:      deploy.Repo,
		hasSkills: deploy.HasSkills,
	}, installation, memory, apiVersion)
}

// ConnectorRequirementsForProject emits connector requirements for deploy and
// connection checks.
//
// Runs a full local compile first so connector modules and the vendored
// runtime are materialized in outDir before requirements are emitted.
func ConnectorRequirementsForProject(
	ctx context.Context,
	p *project.Project,
	outDir string,
	installation Installation,
) (ConnectorRequirements, error) {
	summary, err := Compile(ctx, p, outDir, installation)
	if err != nil {
		return nil, err
	}
	return introspect.EmitConnectorRequirementsForEntries(ctx, p, outDir, summary.Connectors)
}

// McpConnectionEntriesForProject compiles p locally and emits its MCP
// connection entries.
func McpConnectionEntriesForProject(
	ctx context.Context,
	p *project.Project,
	outDir string,
	installation Installation,
) ([]McpConnectionEntry, error) {
	summary, err := Compile(ctx, p, outDir, installation)
	if err != nil {
		return nil, err
	}
	return introspect.EmitMcpConnectionEntriesForEntries(ctx, p, outDir, summary.Connectors)
}

// writeModule writes a generated module, reporting a failure as what.
func writeModule(path, source, what string) error {
	if _, err := writeIfChanged(path, []byte(source)); err != nil {
		return fmt.Errorf("failed to write %s: %w", what, err)
	}
	return nil
}

func compileInner(
	ctx context.Context,
	p *project.Project,
	outDir string,
	context runtimeContext,
	installation Installation,
	memory MemoryDecision,
	apiVersion string,
) (*CompileSummary, error) {
	// Validate first: prepareOutDir clears existing files.
	agent, err := introspect.ReadAgentStaticConfig(ctx, p)
	if err != nil {
		return nil, err
	}
	// Served agents also get an async general-purpose subagent.
	asyncSubagents := agent.ServedAsyncSubagents()
	localPackages, err := FindLocalPackages(p)
	if err != nil {
		return nil, err
	}
	if err := prepareOutDir(p.Root, outDir); err != nil {
		return nil, err
	}

	copied, outsideImports, err := copyProjectFiles(ctx, p, outDir)
	if err != nil {
		return nil, err
	}

	if err := recordPythonLockInput(p, outDir); err != nil {
		return nil, err
	}

	extension := p.Language.Extension()
	entryFile := entryStem + "." + extension
	agentImport := agentImportSpecifier(p.AgentFile, p.Language)
	sandboxEntry, hasSandbox := introspect.FindSandboxEntry(p)
	sandboxImport := ""
	if hasSandbox {
		sandboxImport = sandboxImportSpecifier(sandboxEntry, p.Language)
	}

	// Discover connectors name-agnostically (skipping test filenames and
	// modules without the required export shape) and, when any remain,
	// generate an aggregator that validates brands at runtime. This release's
	// public authoring surface is MCP-only (`defineMcp` / `define_mcp`);
	// product toolserver / marketplace factories are not exported.
	connectorEntries, err := introspect.ScanExportedConnectors(ctx, p)
	if err != nil {
		return nil, err
	}
	connectorsImport := ""
	if len(connectorEntries) > 0 {
		connectorsFile := connectorsStem + "." + extension
		source := generateConnectorsModule(p.Language, moduleImports(connectorEntries, p.Language))
		if err := writeModule(filepath.Join(outDir, connectorsFile), source, "managed connectors module"); err != nil {
			return nil, err
		}
		connectorsImport = agentImportSpecifier(connectorsFile, p.Language)
	}

	var warnings []string
	if warning, ok := misspelledChannelDirectoryWarning(p); ok {
		warnings = append(warnings, warning)
	}
	warnings = append(warnings, outsideImports...)
	shapeWarnings, err := projectWarnings(p, memory, context, agent.IsFactory)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, shapeWarnings...)
	if agent.UnreadableSubagents > 0 {
		rule := p.Language.Ident(
			"An async subagent (`async: true`) must be an object literal, or a name "+
				"bound to one, in `subagents: [...]`.",
			"An async subagent (`\"async\": True`) must be a dict literal, or a name "+
				"bound to one, in `subagents=[...]`. Annotate a name spread there that holds "+
				"only sync subagents as `list[SubAgent]`.",
		)
		plural := "ies"
		if agent.UnreadableSubagents == 1 {
			plural = "y"
		}
		warnings = append(warnings, fmt.Sprintf("%d subagent entr%s in %s could not be read without running it. %s",
			agent.UnreadableSubagents, plural, p.AgentFile, rule))
	}
	if err := introspect.RejectRemovedSandboxScope(ctx, p); err != nil {
		return nil, err
	}

	channelScan, err := introspect.ScanExportedChannels(ctx, p)
	if err != nil {
		return nil, err
	}
	channelEntries := channelScan.Entries
	channelsImport := ""
	if len(channelEntries) > 0 {
		channelsFile := channelsStem + "." + extension
		source := generateChannelsModule(p.Language, moduleImports(channelEntries, p.Language))
		if err := writeModule(filepath.Join(outDir, channelsFile), source, "managed channels module"); err != nil {
			return nil, err
		}
		channelsImport = agentImportSpecifier(channelsFile, p.Language)
	}

	identityImport := ""
	if entry, ok := introspect.FindIdentityEntry(p); ok {
		identityImport = agentImportSpecifier(entry, p.Language)
	}
	identity, err := introspect.ReadIdentitySummary(ctx, p)
	if err != nil {
		return nil, err
	}
	if identity != nil && identity.AuthOmitted {
		return nil, fmt.Errorf("%s must select authentication explicitly; use auth.langsmithApiKey() / "+
			"auth.langsmith_api_key() for LangSmith workspace API keys", identity.Path)
	}

	// Durable memory is opt-in and independent of identity.
	memoryImport := ""
	if entry, ok := introspect.FindMemoryEntry(p); ok {
		memoryImport = agentImportSpecifier(entry, p.Language)
	}

	// Browser use is opt-in.
	browserImport := ""
	if entry, ok := findBrowserEntry(p); ok {
		browserImport = agentImportSpecifier(entry, p.Language)
	}

	// Ingress channels (`channels.slack(...)`, `channels.github(...)`) cannot
	// fall back to a deployment service principal: require a root identity
	// declaration.
	if identityImport == "" && channelScan.RequiresIdentity {
		return nil, errors.New("channels/ with an ingress trigger (e.g. Slack or GitHub channels) " +
			"require a root identity.{ts,py} declaration; ingress triggers cannot " +
			"fall back to a deployment service principal")
	}

	assistantID := agent.AssistantID

	recipe, err := writeSandboxRecipeManifest(ctx, p, outDir, agent.IsFactory)
	if err != nil {
		return nil, err
	}
	// Only a factory can select a sandbox whose base the recipe was not baked
	// on.
	sandboxSetup := agent.IsFactory && recipe != nil && recipe.HasSetupScript

	entrySource := generateEntry(p.Language, entryInputs{
		agentImport:      agentImport,
		factory:          agent.IsFactory,
		context:          context,
		sandboxImport:    sandboxImport,
		sandboxSetup:     sandboxSetup,
		connectorsImport: connectorsImport,
		channelsImport:   channelsImport,
		identityImport:   identityImport,
		memoryImport:     memoryImport,
		asyncSubagents:   asyncSubagents,
		browserImport:    browserImport,
	})
	if err := writeModule(filepath.Join(outDir, entryFile), entrySource, "managed entry module"); err != nil {
		return nil, err
	}

	// When the project declares identity, generate the thin custom-auth module
	// and point `langgraph.json` at it. Identity is opt-in: with no
	// `identity.{ts,py}` there is no auth block and behavior is unchanged.
	authPath := ""
	if identityImport != "" {
		authFile := authStem + "." + extension
		source := generateAuthModule(p.Language, identityImport, connectorsImport, channelsImport)
		if err := writeModule(filepath.Join(outDir, authFile), source, "managed auth module"); err != nil {
			return nil, err
		}
		authPath = "./" + authFile + ":auth"
	}

	// The single managed HTTP app mounted via `langgraph.json` `http.app`
	// serves connector routes and channel Events routes. Routes use live
	// plugin definitions (no compile-time JSON embed).
	managedHTTPPath := ""
	if identityImport != "" || connectorsImport != "" || channelsImport != "" {
		httpFile := managedHTTPStem + "." + extension
		source := generateManagedHTTPModule(p.Language, identityImport, connectorsImport, channelsImport)
		if err := writeModule(filepath.Join(outDir, httpFile), source, "managed HTTP module"); err != nil {
			return nil, err
		}
		managedHTTPPath = "./" + httpFile + ":app"
	}

	graph := "./" + entryFile + ":agent"
	var extraGraphs [][2]string
	for _, subagent := range asyncSubagents {
		extraGraphs = append(extraGraphs, [2]string{subagent.GraphID, "./" + entryFile + ":" + subagent.Export})
	}
	// `browser_handoff` starts a run of this graph to wait for the user to
	// hand the browser back, then wake the agent.
	if browserImport != "" {
		if assistantID == browserHandoffGraph {
			return nil, fmt.Errorf("agent name `%s` is reserved for browser use", browserHandoffGraph)
		}
		extraGraphs = append(extraGraphs, [2]string{browserHandoffGraph, "./" + entryFile + ":" + browserHandoffExport})
	}
	var channelNames []string
	for _, rel := range channelEntries {
		name, err := introspect.EntryStemName(rel, "channel")
		if err != nil {
			return nil, err
		}
		channelNames = append(channelNames, name)
	}
	scheduleCrons, err := readScheduleCrons(ctx, p, scheduleReadContext{assistantID: assistantID, channels: channelNames})
	if err != nil {
		return nil, err
	}
	if identityImport == "" && len(scheduleCrons) > 0 {
		return nil, errors.New("schedules require a root identity.ts or identity.py declaration for managed execution")
	}
	if apiVersion != LanggraphAPIVersion {
		fmt.Fprintf(os.Stderr, "warning: using langgraph-api version `%s` from %s (default is `%s`)\n",
			apiVersion, LanggraphAPIVersionEnv, LanggraphAPIVersion)
	}
	// TypeScript keys the runtime on `node_version`, so Python configuration
	// is neither read nor validated for those builds: an exported
	// `MDA_PYTHON_VERSION` or an unrelated `pyproject.toml` at the project
	// root must not fail a build that ignores both.
	pythonVersion := formatPythonVersion(defaultPythonVersion)
	if p.Language == project.Python {
		python, err := ResolvedPythonVersion(p.Root)
		if err != nil {
			return nil, err
		}
		if python.Warning != "" {
			warnings = append(warnings, python.Warning)
		}
		pythonVersion = python.Version
	}
	var deployLocalPackages []string
	if context.kind == runtimeContextContextHub {
		for _, pkg := range localPackages {
			deployLocalPackages = append(deployLocalPackages, pkg.BuildDir())
		}
	}
	langgraph := generateLanggraphJSON(langgraphManifestInputs{
		graph:                  graph,
		language:               p.Language,
		apiVersion:             apiVersion,
		pythonVersion:          pythonVersion,
		authPath:               authPath,
		managedHTTPPath:        managedHTTPPath,
		assistantID:            assistantID,
		includeVendoredRuntime: context.kind == runtimeContextContextHub,
		localPackages:          deployLocalPackages,
		// Local builds turn off the Studio auth scheme: the dev server hands
		// out Studio identity on `x-auth-scheme: langsmith` alone, with no
		// credential check, and `MDA_LOCAL_DEV` already unscopes local Studio.
		disableStudioAuth: context.kind == runtimeContextLocal,
		extraGraphs:       extraGraphs,
	})
	// Avoid bumping mtime when the manifest is unchanged.
	if err := writeModule(filepath.Join(outDir, "langgraph.json"), langgraph, "langgraph.json"); err != nil {
		return nil, err
	}

	if p.Language == project.Python {
		if err := configurePythonPackaging(outDir); err != nil {
			return nil, err
		}
	}

	isDev := context.kind == runtimeContextLocal
	mode := runtimePathDeploy
	if isDev {
		mode = runtimePathAbsoluteDev
	}

	// Inject the inferred dependency stack (the LangChain/LangGraph runtime
	// plus the chat-model provider implied by the agent's `model`) into the
	// build manifest. Authors only declare `managed-deepagents` and
	// `deepagents`; everything else is managed and never written into their
	// source.
	runtimeDeps := installation.RuntimeManifest().Dependencies
	if err := injectManagedDependencies(p, outDir, isDev, agent.Model, browserImport != "", runtimeDeps); err != nil {
		return nil, err
	}

	if err := vendorRuntime(p.Language, installation, outDir); err != nil {
		return nil, err
	}
	if err := rewriteRuntimeDependency(p.Language, outDir, mode); err != nil {
		return nil, err
	}
	if err := vendorLocalPackages(p.Language, localPackages, outDir, mode); err != nil {
		return nil, err
	}
	if err := rewriteLocalPackageDependencies(p.Language, outDir, localPackages, mode); err != nil {
		return nil, err
	}

	if p.Language == project.TypeScript {
		// Deploy installs every lockfile entry (including author UI tooling).
		// Drop devDependencies before materializing the lockfile so remote
		// `npm ci` does not pull platform-skewed optional peers (e.g.
		// @emnapi/*).
		if !isDev {
			if err := stripPackageJSONDevDependencies(outDir); err != nil {
				return nil, err
			}
			for _, pkg := range localPackages {
				if err := stripPackageJSONDevDependencies(filepath.Join(outDir, filepath.FromSlash(pkg.BuildDir()))); err != nil {
					return nil, err
				}
			}
		}
		if err := generateLockfile(ctx, outDir); err != nil {
			return nil, err
		}
	}

	return &CompileSummary{
		OutDir:           outDir,
		Copied:           copied,
		AssistantID:      assistantID,
		Connectors:       connectorEntries,
		Channels:         channelEntries,
		ChannelProviders: channelScan.Providers,
		Identity:         identity,
		Memory:           memory,
		Sandbox:          hasSandbox || recipe != nil,
		HasSkills:        context.hasSkills,
		ScheduleCrons:    scheduleCrons,
		Warnings:         warnings,
	}, nil
}

// moduleImports pairs each discovered module with the import specifier the
// generated aggregator uses for it.
func moduleImports(entries []string, language project.Language) []moduleImport {
	modules := make([]moduleImport, len(entries))
	for i, entry := range entries {
		modules[i] = moduleImport{source: proc.ToUnixPath(entry), spec: agentImportSpecifier(entry, language)}
	}
	return modules
}

// copyProjectFiles carries the project's files into outDir, returning the
// files copied and a warning for each import that leaves the project.
//
// TypeScript sources resolve `with { type: "text" }` imports into inline
// constants so the build does not depend on runtime text-module support; all
// of them are rewritten in one introspection helper run.
func copyProjectFiles(ctx context.Context, p *project.Project, outDir string) ([]string, []string, error) {
	var files []string
	var sources []introspect.TypescriptSource
	for _, rel := range p.Files {
		if isBuildOwnedLockfile(rel, p) {
			continue
		}
		files = append(files, rel)
		if !isTypescriptSource(rel) {
			continue
		}
		src := filepath.Join(p.Root, rel)
		source, err := os.ReadFile(src)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read %s: %w", src, err)
		}
		if !utf8.Valid(source) {
			return nil, nil, fmt.Errorf("failed to read %s: stream did not contain valid UTF-8", src)
		}
		sources = append(sources, introspect.TypescriptSource{
			Source:     string(source),
			Rel:        rel,
			ResolveDir: filepath.Dir(src),
		})
	}
	transforms, err := introspect.TransformTypescriptSources(ctx, sources)
	if err != nil {
		return nil, nil, err
	}

	var copied, outsideImports []string
	next := 0
	for _, rel := range files {
		src := filepath.Join(p.Root, rel)
		dst := filepath.Join(outDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create %s: %w", filepath.Dir(dst), err)
		}
		var contents []byte
		if isTypescriptSource(rel) {
			transform := transforms[next]
			next++
			for _, specifier := range transform.OutsideImports {
				outsideImports = append(outsideImports, fmt.Sprintf(
					"%s imports \"%s\" from outside the project, so the build does not "+
						"include it. Make it a local package (`npm install <path>`) or move it into "+
						"the project.", proc.ToUnixPath(rel), specifier))
			}
			contents = []byte(transform.Source)
		} else {
			contents, err = os.ReadFile(src)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read %s: %w", src, err)
			}
		}
		if _, err := writeIfChanged(dst, contents); err != nil {
			return nil, nil, err
		}
		// writeIfChanged does not carry mode bits the way a copy does — sync
		// them after every write (and when bytes were already unchanged) so
		// executable helpers under the project tree stay executable.
		if err := syncFilePermissions(src, dst); err != nil {
			return nil, nil, err
		}
		copied = append(copied, rel)
	}
	return copied, outsideImports, nil
}

// prepareOutDir empties outDir so it can receive a fresh build.
//
// The wipe is gated: the target has to be missing, empty, or a directory a
// previous build wrote, and it may never be the project directory or an
// ancestor of it. Without those checks a build into `.` deletes the user's
// sources, `.env` included. The dependency caches in [fullPreserve] are kept.
func prepareOutDir(projectRoot, outDir string) error {
	if _, err := os.Stat(outDir); err == nil {
		if !isDir(outDir) {
			return fmt.Errorf("build output path %s exists and is not a directory", outDir)
		}
		if err := rejectProjectAncestor(projectRoot, outDir); err != nil {
			return err
		}
		empty, err := isEmptyDir(outDir)
		if err != nil {
			return err
		}
		if !empty {
			managed, err := isManagedBuildDir(outDir)
			if err != nil {
				return err
			}
			if !managed {
				return fmt.Errorf("refusing to clean build directory %s: it is not empty and was not written "+
					"by `mda`. Point --out at an empty or previously built directory, or remove "+
					"this one yourself.", outDir)
			}
			// Clear the contents rather than the directory itself: removing
			// outDir fails when it is the working directory, and only after
			// the removal has already run.
			if err := clearDirPreserving(outDir, fullPreserve); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create build directory %s: %w", outDir, err)
	}
	// Written before any artifact so a build that fails partway still leaves
	// a directory the next build recognizes as its own.
	if err := os.WriteFile(filepath.Join(outDir, buildMarker), []byte(buildMarkerContents), 0o666); err != nil {
		return fmt.Errorf("failed to stamp build directory %s: %w", outDir, err)
	}
	return nil
}

// rejectProjectAncestor rejects an outDir that is the project directory or
// contains it.
//
// A non-existent outDir cannot be an ancestor of the (existing) project root,
// so this only has to consider directories that are already there.
func rejectProjectAncestor(projectRoot, outDir string) error {
	out, err := canonicalize(outDir)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", outDir, err)
	}
	root, err := canonicalize(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", projectRoot, err)
	}
	if root == out {
		return fmt.Errorf("refusing to build into the project directory %s: the build directory is emptied "+
			"first, which would delete the project. Pass --out a separate directory.", outDir)
	}
	if _, ok := stripPathPrefix(root, out); ok {
		return fmt.Errorf("refusing to build into %s: it contains the project at %s, and the build directory "+
			"is emptied first. Pass --out a separate directory.", outDir, projectRoot)
	}
	return nil
}

// isManagedBuildDir reports whether dir holds build output this CLI wrote.
//
// Current builds are stamped with [buildMarker]. Builds from CLI versions that
// predate the stamp are recognized by their generated artifacts, so upgrading
// does not force users to delete an existing build directory.
func isManagedBuildDir(dir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, buildMarker)); err == nil {
		return true, nil
	}
	if !isFile(filepath.Join(dir, "langgraph.json")) {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	return slices.ContainsFunc(entries, func(entry fs.DirEntry) bool {
		return strings.HasPrefix(entry.Name(), generatedPrefix)
	}), nil
}

func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", dir, err)
	}
	return len(entries) == 0, nil
}

// clearDirPreserving removes everything inside dir except top-level names in
// preserve, leaving dir itself in place.
func clearDirPreserving(dir string, preserve []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", dir, err)
	}
	for _, entry := range entries {
		if slices.Contains(preserve, entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var removeErr error
		if entry.IsDir() {
			removeErr = os.RemoveAll(path)
		} else {
			removeErr = os.Remove(path)
		}
		if removeErr != nil {
			return fmt.Errorf("failed to clean build directory %s: %w", path, removeErr)
		}
	}
	return nil
}

// writeIfChanged writes contents to path only when the on-disk bytes differ,
// reporting whether a write occurred.
//
// Used for generated artifacts (`langgraph.json`, manifests) so identical
// rebuilds do not bump mtime.
func writeIfChanged(path string, contents []byte) (bool, error) {
	if isFile(path) {
		if existing, err := os.ReadFile(path); err == nil && slices.Equal(existing, contents) {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, contents, 0o666); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", path, err)
	}
	return true, nil
}

// permissionBits are the mode bits a copy carries over.
const permissionBits = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// syncFilePermissions copies permission bits from src onto dst, including
// when content was left unchanged by [writeIfChanged] (mode-only source
// edits).
func syncFilePermissions(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to read permissions for %s: %w", src, err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("failed to read permissions for %s: %w", dst, err)
	}
	if srcInfo.Mode()&permissionBits == dstInfo.Mode()&permissionBits {
		return nil
	}
	if err := os.Chmod(dst, srcInfo.Mode()&permissionBits); err != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", dst, err)
	}
	return nil
}

// isBuildOwnedLockfile reports whether the build owns rel, so the author's
// copy must not overwrite it.
//
// `package-lock.json` is compile-managed ([generateLockfile]). Copying the
// author's stale lock over a prior build would make the lock input marker
// vouch for a lock the build did not generate.
func isBuildOwnedLockfile(rel string, p *project.Project) bool {
	return p.Language == project.TypeScript && rel == "package-lock.json"
}

// recordPythonLockInput remembers the author's `uv.lock` bytes in the build
// ([lockInputMarker]).
func recordPythonLockInput(p *project.Project, outDir string) error {
	if p.Language != project.Python {
		return nil
	}
	author := filepath.Join(p.Root, "uv.lock")
	if !isFile(author) {
		return nil
	}
	contents, err := os.ReadFile(author)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", author, err)
	}
	_, err = writeIfChanged(filepath.Join(outDir, lockInputMarker), contents)
	return err
}

// isTypescriptSource reports whether rel is a TypeScript source file that
// should be run through the text-module inliner instead of copied verbatim.
func isTypescriptSource(rel string) bool {
	switch filepath.Ext(rel) {
	case ".ts", ".tsx", ".mts", ".cts":
		return true
	}
	return false
}

// canonicalize is Rust's `fs::canonicalize`: the absolute path with every
// symlink resolved; the path must exist.
func canonicalize(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// stripPathPrefix is Rust's `Path::strip_prefix`: path relative to base when
// base is a whole-component prefix of path ("" when they are equal).
func stripPathPrefix(path, base string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || filepath.IsAbs(rel) {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// comparePathComponents orders relative paths component by component, as
// Rust's `PathBuf` does.
func comparePathComponents(a, b string) int {
	return slices.Compare(strings.Split(filepath.ToSlash(a), "/"), strings.Split(filepath.ToSlash(b), "/"))
}
