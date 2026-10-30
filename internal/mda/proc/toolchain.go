package proc

// Resolve host toolchain binaries for subprocesses.
//
// Windows `CreateProcess` does not honor `PATHEXT` the way shells do: a bare
// `npm` looks for `npm.exe` only, while Node ships `npm.cmd`. The same applies
// to `npx`. Python 3 is usually installed as `python.exe` with no
// `python3.exe` (and a Windows Store `python3` alias may exist but fail).
// These helpers pick working program names so compile and deploy spawn sites
// stay OS-agnostic.
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

// PythonEmitVenvDir is the directory name of the isolated venv the mda CLI
// creates for the requirement emit when a project has no environment of its
// own. `langsmith deploy` emits in the project's environment, but the deploy
// archive still omits the directory.
const PythonEmitVenvDir = ".mda-emit-venv"

const createVenvHint = "Run `uv sync` so `.venv` exists, or invoke `langsmith` via `uv run`."

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

// Python3Program returns the resolved Python 3 program name. Deploy runs a
// project's own interpreter ([ProjectPython]); tests use this one to create
// the virtualenvs they run projects under.
func Python3Program() string {
	return python3Program()
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
