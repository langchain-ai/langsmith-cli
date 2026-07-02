package cmd

import (
	"fmt"
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

// updateCommand returns the command a user should run to update an
// externally-managed install. Returns "" for managed/dev methods.
func (m installMethod) updateCommand() string {
	switch m {
	case methodHomebrew:
		return "brew upgrade langchain-ai/tap/langsmith-cli"
	case methodScoop:
		return "scoop update langsmith-cli"
	case methodGo:
		return "go install github.com/langchain-ai/langsmith-cli/cmd/langsmith@latest"
	default:
		return ""
	}
}

// label returns a stable machine-readable name for the install method, used in
// JSON output.
func (m installMethod) label() string {
	switch m {
	case methodHomebrew:
		return "homebrew"
	case methodScoop:
		return "scoop"
	case methodGo:
		return "go"
	case methodDev:
		return "dev"
	default:
		return "managed"
	}
}

// displayName returns a human-readable name of the package manager for messages.
func (m installMethod) displayName() string {
	switch m {
	case methodHomebrew:
		return "Homebrew"
	case methodScoop:
		return "Scoop"
	case methodGo:
		return "`go install`"
	default:
		return ""
	}
}

// managedExternallyMessage returns the pretty-format message shown when
// self-update declines to replace a package-manager-owned binary.
func (m installMethod) managedExternallyMessage() string {
	return fmt.Sprintf(
		"langsmith was installed with %s, which manages updates for you.\nTo update, run:\n\n    %s\n\n(Use --force to update in place anyway.)",
		m.displayName(),
		m.updateCommand(),
	)
}
