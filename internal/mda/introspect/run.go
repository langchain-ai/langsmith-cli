package introspect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
	"github.com/langchain-ai/langsmith-cli/internal/mda/proc"
	"github.com/langchain-ai/langsmith-cli/internal/mda/project"
)

//go:embed helper/package.json helper/package-lock.json helper/*.mjs
var typescriptHelperFiles embed.FS

//go:embed helper/introspect.py
var pythonHelper string

// helperCacheRoot is where the TypeScript helper and its pinned dependencies
// are installed: a CLI-owned cache, never the project. Tests point it at a
// temporary directory.
var helperCacheRoot = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate the user cache directory for the introspection helper: %w", err)
	}
	return filepath.Join(dir, "langsmith", "mda-helper"), nil
}

// ImportStage is what [ImportProject] is doing.
type ImportStage int

const (
	// ImportStageInstallingHelper is installing the TypeScript helper's
	// packages into the CLI's cache, which happens once per Node.js platform.
	ImportStageInstallingHelper ImportStage = iota
	// ImportStageImporting is importing the project's modules.
	ImportStageImporting
)

// Message is how a progress line names the stage.
func (s ImportStage) Message() string {
	if s == ImportStageInstallingHelper {
		return "Installing the introspection helper…"
	}
	return "Importing project modules…"
}

// ImportOptions is how [ImportProject] imports a project's modules.
type ImportOptions struct {
	// Env are the project's `.env` values. Modules may read the environment
	// while they are imported, as they will when deployed, so the helper runs
	// with these values under the process environment, which wins as it does
	// everywhere in the CLI.
	Env map[string]string
	// Progress, when set, is told each stage before it starts.
	Progress func(ImportStage)
}

func (o ImportOptions) report(stage ImportStage) {
	if o.Progress != nil {
		o.Progress(stage)
	}
}

// ImportProject imports p's modules, once per process: every question asked
// about p afterwards is answered from what this import read. A question asked
// before it imports the modules itself, with the project's `.env`.
//
// Deploy calls it first, to show progress while the helper installs and the
// modules import, and to hand over the `.env` values it read.
func ImportProject(ctx context.Context, p *project.Project, opts ImportOptions) error {
	_, err := importDocument(ctx, p, &opts)
	return err
}

// documents caches one helper document per project per process: deploy asks
// many questions of the same project, and each answer comes from one import.
var documents = struct {
	sync.Mutex
	entries map[string]*documentEntry
}{entries: map[string]*documentEntry{}}

type documentEntry struct {
	mu       sync.Mutex
	done     bool
	document *document
	err      error
}

// projectDocument returns the helper's document for p, importing its modules
// the first time p is asked about.
func projectDocument(ctx context.Context, p *project.Project) (*document, error) {
	return importDocument(ctx, p, nil)
}

// importDocument is [projectDocument], importing with opts, or with the
// project's `.env` when opts is nil.
func importDocument(ctx context.Context, p *project.Project, opts *ImportOptions) (*document, error) {
	key := strings.Join(append([]string{p.Root, p.Language.Label(), p.AgentFile}, p.Files...), "\x00")
	documents.Lock()
	entry, ok := documents.entries[key]
	if !ok {
		entry = &documentEntry{}
		documents.entries[key] = entry
	}
	documents.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.done {
		return entry.document, entry.err
	}
	if opts == nil {
		opts = &ImportOptions{Env: project.ParseEnvFile(filepath.Join(p.Root, ".env"))}
	}
	doc, err := runProjectHelper(ctx, p, *opts)
	// A cancelled run says nothing about the project: the next caller retries.
	if err != nil && (errs.IsCancellation(err) || ctx.Err() != nil) {
		return nil, err
	}
	entry.done, entry.document, entry.err = true, doc, err
	return doc, err
}

func runProjectHelper(ctx context.Context, p *project.Project, opts ImportOptions) (*document, error) {
	request := newProjectRequest(p)
	env := helperEnvironment(opts.Env)
	doc := &document{}
	var err error
	switch p.Language {
	case project.Python:
		err = runPythonHelper(ctx, p, request, env, opts, doc)
	default:
		err = runTypescriptHelper(ctx, request, p.Root, env, p.Root, opts, doc)
	}
	if failure, ok := errors.AsType[*helperFailure](err); ok && failure.kind == failureHelper {
		return nil, errs.Categorized(errs.Toolchain, fmt.Sprintf(
			"cannot import the modules of %s to read its configuration: %s. "+
				"Install the project's dependencies (`%s`).",
			p.Root, failure.message, p.Language.Ident("npm install", "uv sync")))
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// helperEnvironment is the environment a helper imports a project's modules
// in: dotenv under the process environment, which wins.
func helperEnvironment(dotenv map[string]string) []string {
	env := os.Environ()
	keys := make([]string, 0, len(dotenv))
	for key := range dotenv {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if _, set := os.LookupEnv(key); !set {
			env = append(env, key+"="+dotenv[key])
		}
	}
	return env
}

// newProjectRequest names the modules of p the helper imports.
func newProjectRequest(p *project.Project) *projectRequest {
	optional := func(rel string, ok bool) *string {
		if !ok {
			return nil
		}
		slash := filepath.ToSlash(rel)
		return &slash
	}
	request := &projectRequest{
		requestHeader:    requestHeader{Mode: "project"},
		Root:             p.Root,
		Agent:            filepath.ToSlash(p.AgentFile),
		LegacyConnectors: slashPaths(FindLegacyConnectorEntries(p)),
		Channels:         slashPaths(FindChannelEntries(p)),
		Schedules:        slashPaths(LanguageDirEntries(p, "schedules")),
	}
	request.Identity = optional(FindIdentityEntry(p))
	request.Memory = optional(FindMemoryEntry(p))
	request.Sandbox = optional(FindSandboxEntry(p))
	request.Mcp = optional(FindMcpEntry(p))
	return request
}

func slashPaths(paths []string) []string {
	out := make([]string, len(paths))
	for i, path := range paths {
		out[i] = filepath.ToSlash(path)
	}
	return out
}

// runPythonHelper runs the Python helper with the project interpreter,
// isolated (`-I`: no user site-packages, no `PYTHON*` variables, no script
// directory on `sys.path`; the helper puts the project root there itself) and
// without writing bytecode into the project (`-B`).
func runPythonHelper(
	ctx context.Context,
	p *project.Project,
	request helperRequest,
	env []string,
	opts ImportOptions,
	out any,
) error {
	python, err := proc.ProjectPython(ctx, p.Root)
	if err != nil {
		return errs.WithCategory(errs.Toolchain,
			fmt.Errorf("cannot import the project's modules to read its configuration: %w", err))
	}
	opts.report(ImportStageImporting)
	cmd := exec.Command(python, "-I", "-B", "-c", pythonHelper)
	cmd.Dir = p.Root
	cmd.Env = env
	return runHelper(ctx, cmd, request, p.Root, out)
}

// runTypescriptHelper runs the TypeScript helper from dir with env, installing
// it first when needed. A helper whose toolchain does not load is installed
// again, once.
func runTypescriptHelper(
	ctx context.Context,
	request helperRequest,
	dir string,
	env []string,
	subject string,
	opts ImportOptions,
	out any,
) error {
	toolchain, err := openTypescriptToolchain(ctx)
	if err != nil {
		return err
	}
	installing := func() { opts.report(ImportStageInstallingHelper) }
	if err := toolchain.ensure(ctx, installing); err != nil {
		return err
	}
	run := func() error {
		opts.report(ImportStageImporting)
		cmd := toolchain.command()
		cmd.Dir = dir
		cmd.Env = env
		return runHelper(ctx, cmd, request, subject, out)
	}
	err = run()
	if failure, ok := errors.AsType[*helperFailure](err); ok && failure.kind == failureToolchain {
		slog.Debug("reinstalling the introspection helper, whose toolchain does not load",
			"cache", toolchain.deps, "error", failure.message)
		if err := toolchain.repair(ctx, installing); err != nil {
			return err
		}
		err = run()
	}
	return toolchain.explain(err)
}

// helperFailure is a helper's report that it could not answer at all.
type helperFailure struct {
	kind    failureKind
	message string
}

func (f *helperFailure) Error() string {
	return f.message
}

// failureKind says whose a [helperFailure] is.
type failureKind string

const (
	// failureHelper is the project's environment: its `managed-deepagents`
	// does not import, for instance.
	failureHelper failureKind = "helper"
	// failureToolchain is the TypeScript helper's cached packages, which do
	// not load.
	failureToolchain failureKind = "toolchain"
	// failureNode is a Node.js too old for the TypeScript helper.
	failureNode failureKind = "node"
)

// runHelper runs a helper with request on stdin and decodes the document it
// writes into out. subject names what it was asked about.
func runHelper(ctx context.Context, cmd *exec.Cmd, request helperRequest, subject string, out any) error {
	program := filepath.Base(cmd.Path)
	scratch, err := os.MkdirTemp("", "langsmith-introspect-*")
	if err != nil {
		return fmt.Errorf("failed to create a directory for the introspection document: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(scratch); err != nil {
			slog.Debug("failed to remove the introspection document", "dir", scratch, "error", err)
		}
	}()
	header := request.header()
	header.Output = filepath.Join(scratch, "document.json")
	input, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode the introspection request: %w", err)
	}

	output, err := proc.OutputWithStdin(ctx, cmd, input)
	if err != nil {
		if errs.IsCancellation(err) {
			return err
		}
		return errs.WithCategory(errs.Toolchain,
			fmt.Errorf("failed to run %s to introspect %s: %w", program, subject, err))
	}
	if !output.Status.Success() {
		return errs.WithCategory(errs.Toolchain,
			errs.ChildFailed(output, fmt.Sprintf("the introspection helper failed for %s (%s)", subject, output.Status)))
	}
	contents, err := os.ReadFile(header.Output)
	if err != nil {
		return errs.WithCategory(errs.Toolchain, errs.ChildFailed(output,
			fmt.Sprintf("the introspection helper wrote no document for %s", subject)))
	}
	var envelope struct {
		Version int          `json:"version"`
		Error   *helperError `json:"error"`
	}
	if err := json.Unmarshal(contents, &envelope); err != nil {
		return errs.WithCategory(errs.Toolchain, errs.ChildFailed(output,
			fmt.Sprintf("the introspection helper wrote no readable document for %s", subject)))
	}
	if envelope.Version != DocumentVersion {
		return errs.Categorized(errs.Toolchain, fmt.Sprintf(
			"the introspection helper wrote document version %d for %s, expected %d",
			envelope.Version, subject, DocumentVersion))
	}
	if envelope.Error != nil {
		return &helperFailure{kind: failureKind(envelope.Error.Kind), message: envelope.Error.Message}
	}
	if err := decodeDocument(contents, out); err != nil {
		return errs.WithCategory(errs.Toolchain,
			fmt.Errorf("failed to decode the introspection document for %s: %w", subject, err))
	}
	return nil
}

// installMu serializes installs within this process; installs racing from
// other processes each finish in a private directory and rename it into place.
var installMu sync.Mutex

// installedMarker is written into a dependency directory once its packages
// installed and loaded.
const installedMarker = ".installed"

// typescriptToolchain is the TypeScript helper in the CLI's cache: its scripts,
// and tsx, esbuild and es-module-lexer installed next to them from the embedded
// lockfile.
//
// The layout is `<cache>/deps-<key>/node_modules` with the scripts in
// `<deps>/helper-<hash of the scripts>/`, so the scripts resolve the packages by
// walking up, and two CLI versions never rewrite each other's scripts. The key
// hashes the manifest and lockfile, and the platform the packages were
// installed for: this binary's GOOS/GOARCH, and the arch and major version of
// the Node.js that runs them, since esbuild's native package and the loaders
// tsx registers differ between them.
type typescriptToolchain struct {
	root     string
	deps     string
	dir      string
	manifest []byte
	lockfile []byte
	scripts  map[string][]byte
}

func openTypescriptToolchain(ctx context.Context) (*typescriptToolchain, error) {
	root, err := helperCacheRoot()
	if err != nil {
		return nil, err
	}
	manifest, err := typescriptHelperFiles.ReadFile("helper/package.json")
	if err != nil {
		return nil, err
	}
	lockfile, err := typescriptHelperFiles.ReadFile("helper/package-lock.json")
	if err != nil {
		return nil, err
	}
	scripts, err := helperScripts()
	if err != nil {
		return nil, err
	}
	node, err := nodePlatform(ctx)
	if err != nil {
		return nil, err
	}
	key := digest(bytes.Join([][]byte{
		manifest, lockfile, []byte(runtime.GOOS + "/" + runtime.GOARCH), []byte(node),
	}, []byte{0}))
	deps := filepath.Join(root, "deps-"+key)
	return &typescriptToolchain{
		root:     root,
		deps:     deps,
		dir:      filepath.Join(deps, helperDirName(scripts)),
		manifest: manifest,
		lockfile: lockfile,
		scripts:  scripts,
	}, nil
}

// command runs the helper's entry script.
func (t *typescriptToolchain) command() *exec.Cmd {
	return proc.Node(filepath.Join(t.dir, "introspect.mjs"))
}

// ensure installs the packages and publishes the scripts when they are
// missing, calling installing first when it installs.
func (t *typescriptToolchain) ensure(ctx context.Context, installing func()) error {
	installMu.Lock()
	defer installMu.Unlock()
	if !isFile(filepath.Join(t.deps, installedMarker)) {
		installing()
		return t.install(ctx)
	}
	if isFile(filepath.Join(t.dir, "introspect.mjs")) {
		return nil
	}
	staging, err := os.MkdirTemp(t.deps, "helper-*.tmp")
	if err != nil {
		return t.cacheError(fmt.Errorf("failed to create a staging directory: %w", err))
	}
	defer removeStaging(staging)
	if err := writeHelperFiles(staging, t.scripts); err != nil {
		return t.cacheError(err)
	}
	return t.cacheError(renameIntoPlace(ctx, staging, t.dir))
}

// repair deletes the installed packages and installs them again.
func (t *typescriptToolchain) repair(ctx context.Context, installing func()) error {
	installMu.Lock()
	err := os.RemoveAll(t.deps)
	installMu.Unlock()
	if err != nil {
		return t.cacheError(fmt.Errorf("failed to delete the broken install: %w", err))
	}
	return t.ensure(ctx, installing)
}

// install runs `npm ci` from the embedded lockfile in a staging directory,
// checks that the packages load there, and renames it into place.
func (t *typescriptToolchain) install(ctx context.Context) error {
	if err := os.MkdirAll(t.root, 0o755); err != nil {
		return t.cacheError(fmt.Errorf("failed to create %s: %w", t.root, err))
	}
	staging, err := os.MkdirTemp(t.root, "install-*")
	if err != nil {
		return t.cacheError(fmt.Errorf("failed to create a staging directory: %w", err))
	}
	defer removeStaging(staging)
	files := map[string][]byte{"package.json": t.manifest, "package-lock.json": t.lockfile}
	if err := writeHelperFiles(staging, files); err != nil {
		return t.cacheError(err)
	}
	args := []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--loglevel=error"}
	command := proc.NpmProgram() + " " + strings.Join(args, " ")
	cmd := proc.Npm(args...)
	cmd.Dir = staging
	output, err := proc.Output(ctx, cmd)
	if err != nil {
		if errs.IsCancellation(err) {
			return err
		}
		return errs.WithCategory(errs.Toolchain, fmt.Errorf(
			"failed to run %s to install the TypeScript introspection helper into %s (is Node.js installed?): %w",
			command, t.deps, err))
	}
	if !output.Status.Success() {
		return errs.WithCategory(errs.Toolchain, errs.ChildFailed(output, fmt.Sprintf(
			"%s failed to install the TypeScript introspection helper into %s: it needs tsx, esbuild and "+
				"es-module-lexer from the npm registry", command, t.deps)))
	}

	// Check the packages load before marking the install complete.
	helper := filepath.Join(staging, filepath.Base(t.dir))
	if err := os.Mkdir(helper, 0o755); err != nil {
		return t.cacheError(fmt.Errorf("failed to create %s: %w", helper, err))
	}
	if err := writeHelperFiles(helper, t.scripts); err != nil {
		return t.cacheError(err)
	}
	check := proc.Node(filepath.Join(helper, "introspect.mjs"))
	check.Dir = staging
	var checked struct {
		Version int `json:"version"`
	}
	request := &requestHeader{Mode: "check"}
	if err := runHelper(ctx, check, request, "the helper installed into "+t.deps, &checked); err != nil {
		if failure, ok := errors.AsType[*helperFailure](err); ok && failure.kind == failureToolchain {
			return errs.Categorized(errs.Toolchain, fmt.Sprintf(
				"the TypeScript introspection helper installed into %s does not load: %s", t.deps, failure.message))
		}
		return t.explain(err)
	}

	if err := os.WriteFile(filepath.Join(staging, installedMarker), nil, 0o644); err != nil {
		return t.cacheError(fmt.Errorf("failed to mark the helper install complete: %w", err))
	}
	return t.cacheError(renameIntoPlace(ctx, staging, t.deps))
}

// explain turns the failures of the toolchain itself into errors that say what
// to do about them.
func (t *typescriptToolchain) explain(err error) error {
	failure, ok := errors.AsType[*helperFailure](err)
	if !ok {
		return err
	}
	switch failure.kind {
	case failureNode:
		return errs.Categorized(errs.Toolchain, failure.message+". Install a newer Node.js and try again.")
	case failureToolchain:
		return errs.Categorized(errs.Toolchain, fmt.Sprintf(
			"the TypeScript introspection helper in %s does not load, even after reinstalling it: %s",
			t.deps, failure.message))
	}
	return err
}

// cacheError names the cache directory in a failure to write it.
func (t *typescriptToolchain) cacheError(err error) error {
	if err == nil {
		return nil
	}
	return errs.WithCategory(errs.Toolchain,
		fmt.Errorf("failed to install the TypeScript introspection helper into %s: %w", t.deps, err))
}

// nodeRuntime caches [nodePlatform]: one `node` per process.
var nodeRuntime struct {
	sync.Mutex
	platform string
}

// nodePlatform is the arch and major version of the `node` the helper runs
// on, as `arm64/v24`.
func nodePlatform(ctx context.Context) (string, error) {
	nodeRuntime.Lock()
	defer nodeRuntime.Unlock()
	if nodeRuntime.platform != "" {
		return nodeRuntime.platform, nil
	}
	output, err := proc.Output(ctx, proc.Node("-p", `process.arch + "/v" + process.versions.node.split(".")[0]`))
	if err != nil {
		if errs.IsCancellation(err) {
			return "", err
		}
		return "", errs.WithCategory(errs.Toolchain, fmt.Errorf(
			"failed to run %s for the TypeScript introspection helper (is Node.js installed?): %w",
			proc.NodeProgram(), err))
	}
	if !output.Status.Success() {
		return "", errs.WithCategory(errs.Toolchain, errs.ChildFailed(output,
			fmt.Sprintf("%s failed to report its version", proc.NodeProgram())))
	}
	nodeRuntime.platform = strings.TrimSpace(string(output.Stdout))
	return nodeRuntime.platform, nil
}

func helperScripts() (map[string][]byte, error) {
	entries, err := fs.ReadDir(typescriptHelperFiles, "helper")
	if err != nil {
		return nil, err
	}
	scripts := map[string][]byte{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".mjs" {
			continue
		}
		data, err := typescriptHelperFiles.ReadFile("helper/" + entry.Name())
		if err != nil {
			return nil, err
		}
		scripts[entry.Name()] = data
	}
	return scripts, nil
}

// helperDirName is the directory the scripts are published to, named after
// their contents.
func helperDirName(scripts map[string][]byte) string {
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	slices.Sort(names)
	var buf bytes.Buffer
	for _, name := range names {
		buf.WriteString(name)
		buf.WriteByte(0)
		buf.Write(scripts[name])
		buf.WriteByte(0)
	}
	return "helper-" + digest(buf.Bytes())
}

func writeHelperFiles(dir string, files map[string][]byte) error {
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}
	return nil
}

// removeStaging removes a staging directory, which is gone after a successful
// rename; a leftover is only cache litter.
func removeStaging(dir string) {
	if err := os.RemoveAll(dir); err != nil {
		slog.Debug("failed to remove an introspection helper staging directory", "dir", dir, "error", err)
	}
}

// renameRetryWindow is how long a refused rename is retried.
const renameRetryWindow = 5 * time.Second

// rename is [os.Rename]; tests replace it.
var rename = os.Rename

// renameIntoPlace moves staging to target, accepting a target another process
// published first.
//
// Windows refuses to rename a directory while anything holds a file in it
// open, typically an antivirus scanner reading what npm just wrote, so a
// refused rename is retried for a while ([renameRefused]).
func renameIntoPlace(ctx context.Context, staging, target string) error {
	deadline := time.Now().Add(renameRetryWindow)
	delay := 10 * time.Millisecond
	for {
		err := rename(staging, target)
		if err == nil {
			return nil
		}
		if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
			return nil
		}
		if !renameRefused(err) || time.Now().After(deadline) {
			return fmt.Errorf("failed to move the introspection helper into %s: %w", target, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(2*delay, 250*time.Millisecond)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// errNoHelperValue is returned when a document lacks a section a caller needs,
// which only a helper bug can cause.
var errNoHelperValue = errors.New("the introspection helper did not report this module")
