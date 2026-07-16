package cmd

import (
	"path/filepath"
	"strings"
)

// installMethod identifies how the langsmith binary was installed, which
// determines whether self-update may replace the binary in place.
type installMethod int

const (
	// methodManaged means self-update owns the binary: install.sh, install.ps1,
	// or a manual download from GitHub Releases. In-place replacement is correct.
	methodManaged installMethod = iota
	// methodHomebrew means the binary lives in a Homebrew Cellar.
	methodHomebrew
	// methodScoop means the binary was installed by Scoop.
	methodScoop
	// methodGo means the binary was installed via `go install`.
	methodGo
	// methodDev means a local/development build with no release version.
	methodDev
)

// externalManagers describes installs owned by a package manager: how to name
// the manager in output and the command the user should run to update. Only the
// methods self-update must defer to appear here — methodManaged and methodDev are
// handled by self-update itself and intentionally have no entry, so membership in
// this map is the single source of truth for "must defer to a package manager".
var externalManagers = map[installMethod]struct {
	label   string // machine-readable name, for JSON output
	display string // human-readable name, for the pretty message
	command string // the command the user should run to update
}{
	methodHomebrew: {"homebrew", "Homebrew", "brew upgrade langchain-ai/tap/langsmith-cli"},
	methodScoop:    {"scoop", "Scoop", "scoop update langsmith-cli"},
	methodGo:       {"go", "`go install`", "go install github.com/langchain-ai/langsmith-cli/cmd/langsmith@latest"},
}

// detectInstallMethod classifies the install method from the resolved executable
// path, the build version, and the relevant environment values. It is pure: all
// I/O (resolving the path, reading env) happens in the caller.
//
//   - execPath must already be resolved through filepath.EvalSymlinks so that
//     Homebrew's bin symlinks point at the Cellar.
//   - goos is runtime.GOOS.
//   - gobin/gopath/home come from $GOBIN, $GOPATH, and the user's home dir.
func detectInstallMethod(execPath, version, goos, gobin, gopath, home string) installMethod {
	// Normalize both separators to "/" so a single "/segment/" check works for
	// Windows paths regardless of the OS this code runs on (matters for tests).
	lower := strings.ToLower(strings.ReplaceAll(execPath, `\`, "/"))

	if strings.Contains(lower, "/cellar/") {
		return methodHomebrew
	}
	if strings.Contains(lower, "/scoop/") {
		return methodScoop
	}

	dir := filepath.Dir(execPath)
	for _, bin := range goBinDirs(gobin, gopath, home) {
		if bin != "" && filepath.Clean(dir) == filepath.Clean(bin) {
			return methodGo
		}
	}

	if version == "dev" {
		return methodDev
	}
	return methodManaged
}

// goBinDirs returns the candidate directories where `go install` places binaries,
// in precedence order: $GOBIN, then $GOPATH/bin, then $HOME/go/bin (Go's default
// GOPATH when unset).
func goBinDirs(gobin, gopath, home string) []string {
	if gobin != "" {
		return []string{gobin}
	}
	if gopath != "" {
		// GOPATH may contain multiple entries; go install uses the first.
		first := filepath.SplitList(gopath)
		if len(first) > 0 && first[0] != "" {
			return []string{filepath.Join(first[0], "bin")}
		}
	}
	if home != "" {
		return []string{filepath.Join(home, "go", "bin")}
	}
	return nil
}

// shouldDeferToManager reports whether self-update must defer to a package
// manager instead of replacing the binary in place. --force overrides it.
// Membership in externalManagers is the source of truth; methodManaged and
// methodDev are not externally managed and so are never deferred.
func shouldDeferToManager(method installMethod, force bool) bool {
	_, external := externalManagers[method]
	return external && !force
}
