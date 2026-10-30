package proc

// Resolve host toolchain binaries for subprocesses.
//
// Windows `CreateProcess` does not honor `PATHEXT` the way shells do: a bare
// `npm` looks for `npm.exe` only, while Node ships `npm.cmd`. The same applies
// to `npx` and npm `.bin` shims (`tsx.cmd`). Python 3 is usually installed as
// `python.exe` with no `python3.exe` (and a Windows Store `python3` alias may
// exist but fail). These helpers pick working program names so compile and
// deploy spawn sites stay OS-agnostic.
//
// The resolved program names are probed once per process, the first time they
// are asked for, and not under any context: a `--version` probe is short.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const isWindows = runtime.GOOS == "windows"

// Npm returns a command that runs the host `npm` (or `npm.cmd` on Windows).
func Npm(args ...string) *exec.Cmd {
	return exec.Command(NpmProgram(), args...)
}

// Npx returns a command that runs the host `npx` (or `npx.cmd` on Windows).
func Npx(args ...string) *exec.Cmd {
	return exec.Command(NpxProgram(), args...)
}

// Node returns a command that runs the host `node` binary.
func Node(args ...string) *exec.Cmd {
	return exec.Command(NodeProgram(), args...)
}

// Uv returns a command that runs the host `uv`.
func Uv(args ...string) *exec.Cmd {
	return exec.Command("uv", args...)
}

// NpmAvailable reports whether a working `npm` is on PATH.
func NpmAvailable(ctx context.Context) bool {
	return probeVersion(ctx, NpmProgram())
}

// UvAvailable reports whether a working `uv` is on PATH.
func UvAvailable(ctx context.Context) bool {
	return probeVersion(ctx, "uv")
}

// PythonPipAvailableFor reports whether `python -m pip` works for the given
// interpreter program or path.
//
// `uv`-created environments often omit pip; prefer probing the module over
// assuming a host `pip` binary exists.
func PythonPipAvailableFor(ctx context.Context, python string) bool {
	return exec.CommandContext(ctx, python, "-m", "pip", "--version").Run() == nil
}

// ProjectPython returns the Python interpreter for subprocesses that import
// project or runtime packages.
//
// Prefers `<root>/.venv`, then `$VIRTUAL_ENV`, then the active conda
// environment (`$CONDA_PREFIX`), which a conda user installs the project into
// instead of a virtualenv. A candidate must actually run (`--version`); a stale
// `.venv` file does not block a working active env. Does not fall back to
// `python3` on PATH: a global install can shadow a valid project environment
// (see langchain-ai/managed-deepagents-sdk#312).
func ProjectPython(ctx context.Context, root string) (string, error) {
	return resolveProjectPython(ctx, root, os.Getenv("VIRTUAL_ENV"), os.Getenv("CONDA_PREFIX"))
}

// PythonForEmit returns the interpreter for connector and channel emit.
//
// Uses the project environment when it exists. Otherwise creates an isolated
// venv next to the build directory (not inside it) so deploy never archives a
// local interpreter.
func PythonForEmit(ctx context.Context, projectRoot, outDir string) (string, error) {
	if python, err := ProjectPython(ctx, projectRoot); err == nil {
		return python, nil
	}
	return isolatedEmitPython(ctx, outDir)
}

// PythonEmitVenvDir is the directory name of the isolated emit venv.
const PythonEmitVenvDir = ".mda-emit-venv"

// EmitVenvDir returns the isolated emit venv path: a sibling of outDir.
func EmitVenvDir(outDir string) string {
	return filepath.Join(filepath.Dir(outDir), PythonEmitVenvDir)
}

func isolatedEmitPython(ctx context.Context, outDir string) (string, error) {
	venv := EmitVenvDir(outDir)
	if python, ok := venvInterpreter(ctx, venv); ok {
		return python, nil
	}
	if err := createIsolatedVenv(ctx, venv); err != nil {
		return "", err
	}
	python, ok := venvInterpreter(ctx, venv)
	if !ok {
		return "", fmt.Errorf("created isolated emit venv at %s but found no usable interpreter", venv)
	}
	return python, nil
}

const createVenvHint = "Run `uv sync` so `.venv` exists, or invoke `langsmith` via `uv run`."

func createIsolatedVenv(ctx context.Context, venv string) error {
	if err := os.RemoveAll(venv); err != nil {
		return fmt.Errorf("failed to replace %s: %w", venv, err)
	}
	parent := filepath.Dir(venv)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", parent, err)
	}

	if UvAvailable(ctx) {
		output, err := Output(ctx, Uv("venv", "--python", Python3Program(), venv))
		if err != nil {
			return fmt.Errorf("failed to run uv venv for connector emit: %w", err)
		}
		if output.Status.Success() {
			return nil
		}
	}

	output, err := Output(ctx, Python3("-m", "venv", venv))
	if err != nil {
		return fmt.Errorf("failed to create isolated emit venv at %s\n%s: %w", venv, createVenvHint, err)
	}
	if !output.Status.Success() {
		stderr := strings.ToValidUTF8(string(output.Stderr), "�")
		return fmt.Errorf("failed to create isolated emit venv at %s\n%s\n%s", venv, stderr, createVenvHint)
	}
	return nil
}

func resolveProjectPython(ctx context.Context, root, virtualEnv, condaPrefix string) (string, error) {
	dotVenv := filepath.Join(root, ".venv")
	if python, ok := venvInterpreter(ctx, dotVenv); ok {
		return python, nil
	}
	if virtualEnv != "" {
		if python, ok := venvInterpreter(ctx, virtualEnv); ok {
			return python, nil
		}
	}
	if condaPrefix != "" {
		if python, ok := firstUsable(ctx, condaInterpreterCandidates(condaPrefix, isWindows)); ok {
			return python, nil
		}
	}
	return "", fmt.Errorf("no usable Python virtualenv for connector discovery\n"+
		"Looked for %s, $VIRTUAL_ENV and $CONDA_PREFIX.\n"+
		"%s", dotVenv, createVenvHint)
}

func venvInterpreter(ctx context.Context, venv string) (string, bool) {
	return firstUsable(ctx, venvInterpreterCandidates(venv, isWindows))
}

func firstUsable(ctx context.Context, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if isFile(candidate) && probeVersion(ctx, candidate) {
			return candidate, true
		}
	}
	return "", false
}

// condaInterpreterCandidates are where a conda environment keeps its
// interpreter: like a virtualenv's on Unix, at the prefix's root on Windows.
func condaInterpreterCandidates(prefix string, windows bool) []string {
	if windows {
		return []string{filepath.Join(prefix, "python.exe")}
	}
	return venvInterpreterCandidates(prefix, false)
}

func venvInterpreterCandidates(venv string, windows bool) []string {
	if windows {
		return []string{
			filepath.Join(venv, "Scripts", "python.exe"),
			filepath.Join(venv, "Scripts", "python3.exe"),
		}
	}
	return []string{
		filepath.Join(venv, "bin", "python"),
		filepath.Join(venv, "bin", "python3"),
	}
}

var (
	npmProgram     = sync.OnceValue(func() string { return firstWorking(npmCandidates(isWindows)) })
	npxProgram     = sync.OnceValue(func() string { return firstWorking(npxCandidates(isWindows)) })
	nodeProgram    = sync.OnceValue(func() string { return firstWorking(nodeCandidates(isWindows)) })
	python3Program = sync.OnceValue(func() string { return firstWorking(python3Candidates(isWindows)) })
)

// NpmProgram returns the program name used for `npm` spawns (also useful in
// error messages).
func NpmProgram() string {
	return npmProgram()
}

// NpxProgram returns the program name used for `npx` spawns (also useful in
// error messages).
func NpxProgram() string {
	return npxProgram()
}

// NodeProgram returns the program name used for `node` spawns (also useful in
// error messages).
func NodeProgram() string {
	return nodeProgram()
}

// Python3 returns a command that runs Python 3 (`python3`, or `python` on
// Windows when needed).
func Python3(args ...string) *exec.Cmd {
	return exec.Command(Python3Program(), args...)
}

// Python3Available reports whether a working Python 3 interpreter is on PATH.
func Python3Available(ctx context.Context) bool {
	return probeVersion(ctx, Python3Program())
}

// Python3Program returns the resolved Python 3 program name (also useful in
// error messages).
func Python3Program() string {
	return python3Program()
}

// NpmBin resolves an npm-installed `.bin` shim, preferring the Windows `.cmd`
// form.
func NpmBin(binDir, name string) (string, bool) {
	for _, candidate := range npmBinCandidates(binDir, name, isWindows) {
		if isFile(candidate) {
			return candidate, true
		}
	}
	return "", false
}

// firstWorking returns the first candidate whose `--version` succeeds, else
// the first candidate.
func firstWorking(candidates []string) string {
	for _, candidate := range candidates {
		if exec.Command(candidate, "--version").Run() == nil {
			return candidate
		}
	}
	return candidates[0]
}

func nodeCandidates(windows bool) []string {
	if windows {
		return []string{"node.exe", "node"}
	}
	return []string{"node"}
}

func npmCandidates(windows bool) []string {
	if windows {
		// Node's Windows install ships `npm.cmd`. Custom shims may be
		// `npm.exe` (resolved via the bare `npm` name).
		return []string{"npm.cmd", "npm"}
	}
	return []string{"npm"}
}

func npxCandidates(windows bool) []string {
	if windows {
		return []string{"npx.cmd", "npx"}
	}
	return []string{"npx"}
}

func npmBinCandidates(binDir, name string, windows bool) []string {
	if windows {
		return []string{
			filepath.Join(binDir, name+".cmd"),
			filepath.Join(binDir, name+".exe"),
			filepath.Join(binDir, name),
		}
	}
	return []string{filepath.Join(binDir, name)}
}

func python3Candidates(windows bool) []string {
	if windows {
		return []string{"python3", "python"}
	}
	return []string{"python3"}
}

func probeVersion(ctx context.Context, program string) bool {
	return exec.CommandContext(ctx, program, "--version").Run() == nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
