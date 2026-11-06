package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	toolPipx = "pipx"
	toolUV   = "uv-tool"
)

// wheelInstall describes the Python wheel that owns the running binary. The
// zero value means no wheel owns it.
type wheelInstall struct {
	installer    string // INSTALLER contents ("pip", "uv", or "unknown")
	sitePackages string // site-packages directory holding the wheel's dist-info
	tool         string // toolPipx or toolUV when found under that tool's venv root
	python       string // the venv's interpreter, when sitePackages is in a venv
	uvProject    string // project directory whose uv.lock owns the venv
}

// toolDirs is where one tool (pipx or `uv tool`) keeps its per-package venvs
// and where it puts their entry points.
type toolDirs struct {
	roots []string // directories holding one venv per package
	bins  []string // directories the tool installs entry points into
}

// toolLocations lists pipx's and `uv tool`'s venv roots and bin dirs, honoring
// PIPX_HOME, PIPX_BIN_DIR, UV_TOOL_DIR, UV_TOOL_BIN_DIR, XDG_BIN_HOME and
// XDG_DATA_HOME, and falling back to each tool's defaults on every OS. Both
// tools default to ~/.local/bin for entry points, Windows included.
func toolLocations(home string, getenv func(string) string) (pipx, uv toolDirs) {
	add := func(dst *[]string, parts ...string) {
		for _, p := range parts {
			if p == "" {
				return
			}
		}
		*dst = append(*dst, filepath.Join(parts...))
	}
	dataHome := getenv("XDG_DATA_HOME")
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}

	add(&pipx.roots, getenv("PIPX_HOME"), "venvs")
	add(&pipx.roots, dataHome, "pipx", "venvs")
	add(&pipx.roots, home, ".local", "pipx", "venvs")
	add(&pipx.roots, home, "Library", "Application Support", "pipx", "venvs")
	add(&pipx.roots, home, "pipx", "venvs")
	add(&pipx.roots, getenv("LOCALAPPDATA"), "pipx", "pipx", "venvs")
	if dir := getenv("PIPX_BIN_DIR"); dir != "" {
		pipx.bins = []string{dir}
	} else {
		add(&pipx.bins, home, ".local", "bin")
	}

	add(&uv.roots, getenv("UV_TOOL_DIR"))
	add(&uv.roots, dataHome, "uv", "tools")
	add(&uv.roots, getenv("APPDATA"), "uv", "data", "tools")
	switch {
	case getenv("UV_TOOL_BIN_DIR") != "":
		uv.bins = []string{getenv("UV_TOOL_BIN_DIR")}
	case getenv("XDG_BIN_HOME") != "":
		uv.bins = []string{getenv("XDG_BIN_HOME")}
	case getenv("XDG_DATA_HOME") != "":
		uv.bins = []string{filepath.Join(getenv("XDG_DATA_HOME"), "..", "bin")}
	default:
		add(&uv.bins, home, ".local", "bin")
	}
	return pipx, uv
}

// findWheelInstall reports which wheel of pypiPackageName owns execPath. It
// first checks the site-packages directories next to the binary, which covers
// venvs, the user site and system prefixes, plus pipx and `uv tool` on systems
// where their bin entry is a symlink. Then, when the binary sits in pipx's or
// uv's bin dir, it checks the package's venv under that tool's roots, for
// Windows, where both copy the exe there instead of linking it. A RECORD entry
// only counts when its sha256 matches the running binary, so a binary that
// install.sh later overwrote at the same path is not claimed by a stale RECORD.
//
// It only reads the local filesystem, and any error means "not a wheel
// install" so self-update falls back to its other checks. execPath must
// already be resolved through filepath.EvalSymlinks.
func findWheelInstall(execPath string, pipx, uv toolDirs) wheelInstall {
	bin := &binaryHash{path: execPath}
	for _, sp := range candidateSitePackages(execPath) {
		if w, ok := matchWheel(sp, execPath, bin, false); ok {
			return w
		}
	}
	for _, t := range []struct {
		tool string
		dirs toolDirs
	}{{toolPipx, pipx}, {toolUV, uv}} {
		inBin := inAnyDir(execPath, t.dirs.bins)
		for _, root := range t.dirs.roots {
			// An identical copy elsewhere, such as a release binary dropped into
			// C:\tools, is not the tool's to update. uv records where it put each
			// entry point, which also covers a one-off UV_TOOL_BIN_DIR; pipx
			// keeps no such record.
			if !inBin && (t.tool != toolUV || !uvReceiptLists(root, execPath)) {
				continue
			}
			for _, sp := range toolSitePackages(root) {
				if w, ok := matchWheel(sp, execPath, bin, true); ok {
					w.tool = t.tool
					return w
				}
			}
		}
	}
	return wheelInstall{}
}

// inAnyDir reports whether path sits directly in one of dirs.
func inAnyDir(path string, dirs []string) bool {
	parent := cleanForCompare(filepath.Dir(path))
	for _, dir := range dirs {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		if cleanForCompare(dir) == parent {
			return true
		}
	}
	return false
}

// matchWheel checks the pypiPackageName dist-info in sitePackages. With
// byName, a RECORD entry matches on file name and hash alone, for a copy of
// the entry that lives outside the venv.
func matchWheel(sitePackages, execPath string, bin *binaryHash, byName bool) (wheelInstall, bool) {
	distPrefix := normalizeDistName(pypiPackageName) + "-"
	records, _ := filepath.Glob(filepath.Join(sitePackages, "*.dist-info", "RECORD"))
	for _, record := range records {
		distInfo := filepath.Dir(record)
		if !strings.HasPrefix(strings.ToLower(filepath.Base(distInfo)), distPrefix) {
			continue
		}
		if !recordListsFile(record, sitePackages, execPath, bin, byName) {
			continue
		}
		w := wheelInstall{installer: "unknown", sitePackages: sitePackages}
		if b, err := os.ReadFile(filepath.Join(distInfo, "INSTALLER")); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				w.installer = s
			}
		}
		if venv := venvPrefix(sitePackages); venv != "" {
			w.python = venvPython(venv)
			if project := filepath.Dir(venv); fileExists(filepath.Join(project, "uv.lock")) {
				w.uvProject = project
			}
		}
		return w, true
	}
	return wheelInstall{}, false
}

// candidateSitePackages lists the site-packages directories that may hold the
// wheel which installed execPath: any site-packages/dist-packages ancestor (a
// binary shipped inside the package), and the ones next to the scripts
// directory the binary sits in. That covers venvs, pipx and `uv tool` venvs,
// the user site (~/.local on Linux, ~/Library/Python/X.Y on macOS,
// %APPDATA%\Python\PythonXY on Windows), and system prefixes.
func candidateSitePackages(execPath string) []string {
	var dirs []string
	for dir := filepath.Dir(execPath); ; {
		base := strings.ToLower(filepath.Base(dir))
		if base == "site-packages" || base == "dist-packages" {
			dirs = append(dirs, dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return append(dirs, prefixSitePackages(filepath.Dir(filepath.Dir(execPath)))...)
}

// toolSitePackages lists site-packages of the venv named after pypiPackageName
// under a pipx or `uv tool` root.
func toolSitePackages(root string) []string {
	var dirs []string
	for _, venv := range toolVenvs(root) {
		dirs = append(dirs, prefixSitePackages(venv)...)
	}
	return dirs
}

// toolVenvs lists the venvs named after pypiPackageName under a pipx or
// `uv tool` root.
func toolVenvs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	want := normalizeDistName(pypiPackageName)
	var venvs []string
	for _, e := range entries {
		if e.IsDir() && normalizeDistName(e.Name()) == want {
			venvs = append(venvs, filepath.Join(root, e.Name()))
		}
	}
	return venvs
}

// uvReceiptLists reports whether the uv-receipt.toml of a pypiPackageName
// tool venv under root lists execPath as an entry point's install path.
func uvReceiptLists(root, execPath string) bool {
	want := cleanForCompare(execPath)
	for _, venv := range toolVenvs(root) {
		b, err := os.ReadFile(filepath.Join(venv, "uv-receipt.toml"))
		if err != nil {
			continue
		}
		var receipt struct {
			Tool struct {
				Entrypoints []struct {
					InstallPath string `toml:"install-path"`
				} `toml:"entrypoints"`
			} `toml:"tool"`
		}
		if toml.Unmarshal(b, &receipt) != nil {
			continue
		}
		for _, e := range receipt.Tool.Entrypoints {
			p := e.InstallPath
			if p == "" {
				continue
			}
			if resolved, err := filepath.EvalSymlinks(p); err == nil {
				p = resolved
			}
			if cleanForCompare(p) == want {
				return true
			}
		}
	}
	return false
}

func prefixSitePackages(prefix string) []string {
	var dirs []string
	for _, pattern := range []string{
		filepath.Join(prefix, "lib", "python*", "site-packages"),
		filepath.Join(prefix, "lib64", "python*", "site-packages"),
		filepath.Join(prefix, "lib", "python*", "dist-packages"),
		filepath.Join(prefix, "Lib", "site-packages"),
		filepath.Join(prefix, "site-packages"),
	} {
		matches, _ := filepath.Glob(pattern)
		dirs = append(dirs, matches...)
	}
	return dirs
}

// venvPrefix returns the virtual environment containing sitePackages
// (<venv>/lib/pythonX.Y/site-packages or <venv>\Lib\site-packages), or "".
func venvPrefix(sitePackages string) string {
	dir := sitePackages
	for range 3 {
		dir = filepath.Dir(dir)
		if fileExists(filepath.Join(dir, "pyvenv.cfg")) {
			return dir
		}
	}
	return ""
}

func venvPython(venv string) string {
	for _, p := range []string{
		filepath.Join(venv, "bin", "python"),
		filepath.Join(venv, "Scripts", "python.exe"),
	} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// wheelUpdateCommand returns the upgrade command for a wheel install, aimed at
// the environment that holds the wheel. A bare `pip` or `uv pip` would act on
// whatever Python the shell finds, not on a venv the CLI is run from by path.
// On Windows the command is written for PowerShell, the default shell there.
func wheelUpdateCommand(method installMethod, wheel wheelInstall, goos string) string {
	switch {
	case method == methodPip && wheel.python != "":
		python := quoteArg(wheel.python, goos)
		if goos == "windows" && python != wheel.python {
			// PowerShell treats a quoted string in command position as a value,
			// not a program, unless it's invoked with the call operator.
			python = "& " + python
		}
		return python + " -m pip install --upgrade " + pypiPackageName
	case method == methodUVPip && wheel.python != "":
		return "uv pip install --python " + quoteArg(wheel.python, goos) + " --upgrade " + pypiPackageName
	case method == methodUVProject:
		project := quoteArg(wheel.uvProject, goos)
		lock := "uv lock --project " + project + " --upgrade-package " + pypiPackageName
		sync := "uv sync --project " + project
		if goos == "windows" {
			// Windows PowerShell 5.1 has no &&; $? is false when uv lock fails.
			return lock + "; if ($?) { " + sync + " }"
		}
		return lock + " && " + sync
	}
	return externalManagers[method].command
}

// quoteArg single-quotes a path unless it only has characters no shell treats
// specially. Single quotes are literal in POSIX shells and PowerShell, so a
// directory name holding $(...), backticks or " can't run anything when the
// command is pasted. An embedded single quote is closed, escaped and reopened
// for POSIX shells, and doubled for PowerShell, the Windows default.
func quoteArg(s, goos string) string {
	safe := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return true
		case strings.ContainsRune("_-./:+=,@", r):
			return true
		case r == '\\':
			return goos == "windows"
		}
		return false
	}
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !safe(r) }) < 0 {
		return s
	}
	if goos == "windows" {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// recordListsFile reports whether a dist-info RECORD lists target with a hash
// that matches its current contents. RECORD paths are relative to the
// site-packages directory (scripts appear as ../../../bin/<name>) or absolute.
// With byName, an entry matches on base name instead of path.
func recordListsFile(record, sitePackages, target string, bin *binaryHash, byName bool) bool {
	f, err := os.Open(record)
	if err != nil {
		return false
	}
	defer f.Close()

	want := cleanForCompare(target)
	wantName := cleanForCompare(filepath.Base(target))
	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1
	for {
		row, err := r.Read()
		if err != nil {
			return false
		}
		if len(row) == 0 || row[0] == "" {
			continue
		}
		p := filepath.FromSlash(row[0])
		if byName {
			if cleanForCompare(filepath.Base(p)) != wantName {
				continue
			}
		} else {
			if !filepath.IsAbs(p) {
				p = filepath.Join(sitePackages, p)
			}
			if resolved, err := filepath.EvalSymlinks(p); err == nil {
				p = resolved
			}
			if cleanForCompare(p) != want {
				continue
			}
		}
		hash := ""
		if len(row) > 1 {
			hash = row[1]
		}
		// A copy outside the venv is only ours if its contents match.
		if hash == "" && !byName {
			return true
		}
		if hash != "" && bin.matches(hash) {
			return true
		}
	}
}

// binaryHash hashes the running binary at most once, and only when a RECORD
// entry needs comparing.
type binaryHash struct {
	path string
	sum  string
	done bool
}

// matches compares against a RECORD hash ("sha256=<urlsafe-b64, no padding>").
func (b *binaryHash) matches(recordHash string) bool {
	algo, digest, ok := strings.Cut(recordHash, "=")
	if !ok || algo != "sha256" {
		return false
	}
	if !b.done {
		b.done = true
		if f, err := os.Open(b.path); err == nil {
			h := sha256.New()
			if _, err := io.Copy(h, f); err == nil {
				b.sum = base64.RawURLEncoding.EncodeToString(h.Sum(nil))
			}
			f.Close()
		}
	}
	return b.sum != "" && b.sum == strings.TrimRight(digest, "=")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cleanForCompare(p string) string {
	p = filepath.Clean(p)
	if filepath.Separator == '\\' {
		p = strings.ToLower(p)
	}
	return p
}

// normalizeDistName applies the wheel filename escaping used for dist-info
// directory names: lowercase, with runs of "-", "_" and "." collapsed to "_".
func normalizeDistName(name string) string {
	var b strings.Builder
	sep := false
	for _, r := range strings.ToLower(name) {
		if r == '-' || r == '_' || r == '.' {
			sep = true
			continue
		}
		if sep {
			b.WriteByte('_')
			sep = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
