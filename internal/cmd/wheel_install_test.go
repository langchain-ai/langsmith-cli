package cmd

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testBinary = []byte("langsmith binary v0.3.9")

func recordHash(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256=" + base64.RawURLEncoding.EncodeToString(sum[:])
}

// writeWheel lays out a dist-info for dist in sitePackages whose RECORD lists
// each path (relative to sitePackages, as pip and uv write them) with the hash
// of testBinary.
func writeWheel(t *testing.T, sitePackages, dist, installer string, recordPaths ...string) {
	t.Helper()
	distInfo := filepath.Join(sitePackages, normalizeDistName(dist)+"-0.3.9.dist-info")
	if err := os.MkdirAll(distInfo, 0o755); err != nil {
		t.Fatal(err)
	}
	var record strings.Builder
	for _, p := range recordPaths {
		record.WriteString(p + "," + recordHash(testBinary) + "," + "23\n")
	}
	record.WriteString(normalizeDistName(dist) + "-0.3.9.dist-info/RECORD,,\n")
	if err := os.WriteFile(filepath.Join(distInfo, "RECORD"), []byte(record.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if installer != "" {
		if err := os.WriteFile(filepath.Join(distInfo, "INSTALLER"), []byte(installer+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeVenv creates a venv at dir with an interpreter, and returns its
// site-packages directory.
func writeVenv(t *testing.T, dir string) string {
	t.Helper()
	sp := filepath.Join(dir, "lib", "python3.12", "site-packages")
	writeWheelFile(t, filepath.Join(dir, "pyvenv.cfg"), []byte("home = /usr/bin\n"))
	writeWheelFile(t, filepath.Join(dir, "bin", "python"), []byte("python"))
	if err := os.MkdirAll(sp, 0o755); err != nil {
		t.Fatal(err)
	}
	return sp
}

func writeWheelFile(t *testing.T, path string, content []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func detectWheel(exe, home string, w wheelInstall) installMethod {
	return detectInstallMethod(exe, "0.3.9", "darwin", "", "", home, w)
}

func TestFindWheelInstall(t *testing.T) {
	t.Run("venv run by path gets that venv's pip", func(t *testing.T) {
		venv := t.TempDir()
		exe := writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "pip", "../../../bin/langsmith")

		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		if w.installer != "pip" {
			t.Fatalf("installer = %q, want pip", w.installer)
		}
		method := detectWheel(exe, "", w)
		if method != methodPip {
			t.Fatalf("method = %v, want methodPip", method)
		}
		want := filepath.Join(filepath.Dir(exe), "python") + " -m pip install --upgrade " + pypiPackageName
		if got := wheelUpdateCommand(method, w, runtime.GOOS); got != want {
			t.Errorf("command = %q, want %q", got, want)
		}
	})

	t.Run("uv pip venv gets --python", func(t *testing.T) {
		venv := filepath.Join(t.TempDir(), "my env")
		exe := writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "uv", "../../../bin/langsmith")

		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		method := detectWheel(exe, "", w)
		if method != methodUVPip {
			t.Fatalf("method = %v, want methodUVPip", method)
		}
		want := `uv pip install --python '` + filepath.Join(filepath.Dir(exe), "python") + `' --upgrade ` + pypiPackageName
		if got := wheelUpdateCommand(method, w, "darwin"); got != want {
			t.Errorf("command = %q, want %q", got, want)
		}
	})

	t.Run("uv project venv upgrades through the lockfile", func(t *testing.T) {
		project := t.TempDir()
		writeWheelFile(t, filepath.Join(project, "uv.lock"), []byte("version = 1\n"))
		venv := filepath.Join(project, ".venv")
		exe := writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "uv", "../../../bin/langsmith")

		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		method := detectWheel(exe, "", w)
		if method != methodUVProject {
			t.Fatalf("method = %v, want methodUVProject", method)
		}
		got := wheelUpdateCommand(method, w, runtime.GOOS)
		if !strings.Contains(got, "uv lock --project "+w.uvProject+" --upgrade-package "+pypiPackageName) ||
			!strings.Contains(got, "uv sync --project "+w.uvProject) {
			t.Errorf("command = %q, want uv lock/sync for %s", got, w.uvProject)
		}
	})

	t.Run("pip --user in ~/.local/bin", func(t *testing.T) {
		local := filepath.Join(t.TempDir(), ".local")
		exe := writeWheelFile(t, filepath.Join(local, "bin", "langsmith"), testBinary)
		writeWheel(t, filepath.Join(local, "lib", "python3.11", "site-packages"), pypiPackageName, "pip", "../../../bin/langsmith")

		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		method := detectWheel(exe, "", w)
		if method != methodPip {
			t.Fatalf("method = %v, want methodPip", method)
		}
		// Not a venv, so there's no interpreter to point at.
		if got := wheelUpdateCommand(method, w, "darwin"); got != "pip install --upgrade "+pypiPackageName {
			t.Errorf("command = %q", got)
		}
	})

	t.Run("uv tool reached through a ~/.local/bin symlink", func(t *testing.T) {
		home := t.TempDir()
		tool := filepath.Join(home, ".local", "share", "uv", "tools", pypiPackageName)
		target := writeWheelFile(t, filepath.Join(tool, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, tool), pypiPackageName, "uv", "../../../bin/langsmith")
		link := filepath.Join(home, ".local", "bin", "langsmith")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		exe, err := filepath.EvalSymlinks(link)
		if err != nil {
			t.Fatal(err)
		}
		if got := detectWheel(exe, home, findWheelInstall(exe, toolDirs{}, toolDirs{})); got != methodUVTool {
			t.Errorf("method = %v, want methodUVTool", got)
		}
	})

	t.Run("copied exe found under the pipx and uv tool roots", func(t *testing.T) {
		for _, tc := range []struct {
			tool string
			want installMethod
		}{{toolPipx, methodPipx}, {toolUV, methodUVTool}} {
			home := t.TempDir()
			// A custom root, so only the root list (not the path) can identify the tool.
			root := filepath.Join(home, "custom-root")
			venv := filepath.Join(root, pypiPackageName)
			writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
			writeWheel(t, writeVenv(t, venv), pypiPackageName, "pip", "../../../bin/langsmith")
			exe := writeWheelFile(t, filepath.Join(home, ".local", "bin", "langsmith"), testBinary)

			dirs := toolDirs{roots: []string{root}, bins: []string{filepath.Join(home, ".local", "bin")}}
			var pipx, uv toolDirs
			if tc.tool == toolPipx {
				pipx = dirs
			} else {
				uv = dirs
			}
			w := findWheelInstall(exe, pipx, uv)
			if w.tool != tc.tool {
				t.Fatalf("%s: tool = %q", tc.tool, w.tool)
			}
			if got := detectWheel(exe, home, w); got != tc.want {
				t.Errorf("%s: method = %v, want %v", tc.tool, got, tc.want)
			}
		}
	})

	t.Run("an older copy is not claimed by the tool venv", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, "pipx", "venvs")
		venv := filepath.Join(root, pypiPackageName)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "pip", "../../../bin/langsmith")
		exe := writeWheelFile(t, filepath.Join(home, ".local", "bin", "langsmith"), []byte("install.sh build"))
		pipx := toolDirs{roots: []string{root}, bins: []string{filepath.Join(home, ".local", "bin")}}
		if w := findWheelInstall(exe, pipx, toolDirs{}); w.installer != "" {
			t.Errorf("installer = %q, want empty", w.installer)
		}
	})

	t.Run("an identical copy outside the tool's bin dir self-updates", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, "pipx", "venvs")
		venv := filepath.Join(root, pypiPackageName)
		writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "pip", "../../../bin/langsmith")
		exe := writeWheelFile(t, filepath.Join(home, "tools", "langsmith"), testBinary)
		dirs := toolDirs{roots: []string{root}, bins: []string{filepath.Join(home, ".local", "bin")}}
		w := findWheelInstall(exe, dirs, dirs)
		if w.installer != "" {
			t.Fatalf("installer = %q, want empty", w.installer)
		}
		if got := detectWheel(exe, home, w); got != methodManaged {
			t.Errorf("method = %v, want methodManaged", got)
		}
	})

	t.Run("uv receipt claims a copy in a one-off UV_TOOL_BIN_DIR", func(t *testing.T) {
		home := t.TempDir()
		root := filepath.Join(home, "uv", "tools")
		venv := filepath.Join(root, pypiPackageName)
		writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "uv", "../../../bin/langsmith")
		exe := writeWheelFile(t, filepath.Join(home, "one-off", "langsmith"), testBinary)
		receipt := "[tool]\nentrypoints = [{ name = \"langsmith\", install-path = '" + exe + "' }]\n"
		writeWheelFile(t, filepath.Join(venv, "uv-receipt.toml"), []byte(receipt))
		dirs := toolDirs{roots: []string{root}, bins: []string{filepath.Join(home, ".local", "bin")}}

		if got := detectWheel(exe, home, findWheelInstall(exe, toolDirs{}, dirs)); got != methodUVTool {
			t.Errorf("method = %v, want methodUVTool", got)
		}
		// pipx keeps no receipt, so the same layout under pipx stays self-managed.
		if got := detectWheel(exe, home, findWheelInstall(exe, dirs, toolDirs{})); got != methodManaged {
			t.Errorf("pipx: method = %v, want methodManaged", got)
		}
	})

	t.Run("binary inside site-packages", func(t *testing.T) {
		sp := filepath.Join(t.TempDir(), "lib", "python3.12", "site-packages")
		exe := writeWheelFile(t, filepath.Join(sp, "langsmith_cli", "bin", "langsmith"), testBinary)
		writeWheel(t, sp, pypiPackageName, "pip", "langsmith_cli/bin/langsmith")
		if w := findWheelInstall(exe, toolDirs{}, toolDirs{}); w.installer != "pip" {
			t.Errorf("installer = %q, want pip", w.installer)
		}
	})

	t.Run("install.sh binary in ~/.local/bin next to unrelated wheels", func(t *testing.T) {
		home := t.TempDir()
		local := filepath.Join(home, ".local")
		exe := writeWheelFile(t, filepath.Join(local, "bin", "langsmith"), testBinary)
		// Another package's wheel with its own script in the same bin dir.
		writeWheel(t, filepath.Join(local, "lib", "python3.11", "site-packages"), "ruff", "pip", "../../../bin/ruff")
		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		if w.installer != "" {
			t.Fatalf("installer = %q, want empty", w.installer)
		}
		if got := detectInstallMethod(exe, "0.3.9", "linux", "", "", home, w); got != methodManaged {
			t.Errorf("method = %v, want methodManaged", got)
		}
	})

	t.Run("install.sh overwrote a pip --user binary at the same path", func(t *testing.T) {
		home := t.TempDir()
		local := filepath.Join(home, ".local")
		writeWheel(t, filepath.Join(local, "lib", "python3.11", "site-packages"), pypiPackageName, "pip", "../../../bin/langsmith")
		exe := writeWheelFile(t, filepath.Join(local, "bin", "langsmith"), []byte("install.sh build"))
		w := findWheelInstall(exe, toolDirs{}, toolDirs{})
		if w.installer != "" {
			t.Fatalf("installer = %q, want empty", w.installer)
		}
		if got := detectInstallMethod(exe, "0.3.9", "linux", "", "", home, w); got != methodManaged {
			t.Errorf("method = %v, want methodManaged", got)
		}
	})

	t.Run("RECORD lists a different file", func(t *testing.T) {
		venv := t.TempDir()
		exe := writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "pip", "../../../bin/other")
		if w := findWheelInstall(exe, toolDirs{}, toolDirs{}); w.installer != "" {
			t.Errorf("installer = %q, want empty", w.installer)
		}
	})

	t.Run("missing INSTALLER still counts as a wheel", func(t *testing.T) {
		venv := t.TempDir()
		exe := writeWheelFile(t, filepath.Join(venv, "bin", "langsmith"), testBinary)
		writeWheel(t, writeVenv(t, venv), pypiPackageName, "", "../../../bin/langsmith")
		if w := findWheelInstall(exe, toolDirs{}, toolDirs{}); w.installer != "unknown" {
			t.Errorf("installer = %q, want unknown", w.installer)
		}
	})
}

func TestToolLocations(t *testing.T) {
	contains := func(list []string, want string) bool {
		for _, p := range list {
			if p == want {
				return true
			}
		}
		return false
	}

	env := map[string]string{
		"PIPX_HOME":   "/opt/pipx",
		"UV_TOOL_DIR": "/opt/uv-tools",
		"APPDATA":     `C:\Users\me\AppData\Roaming`,
	}
	pipx, uv := toolLocations("/home/me", func(k string) string { return env[k] })
	for _, want := range []string{
		filepath.Join("/opt/pipx", "venvs"),
		filepath.Join("/home/me", ".local", "share", "pipx", "venvs"),
		filepath.Join("/home/me", "pipx", "venvs"),
	} {
		if !contains(pipx.roots, want) {
			t.Errorf("pipx roots %v missing %s", pipx.roots, want)
		}
	}
	for _, want := range []string{
		filepath.Clean("/opt/uv-tools"),
		filepath.Join("/home/me", ".local", "share", "uv", "tools"),
		filepath.Join(`C:\Users\me\AppData\Roaming`, "uv", "data", "tools"),
	} {
		if !contains(uv.roots, want) {
			t.Errorf("uv roots %v missing %s", uv.roots, want)
		}
	}
	defaultBin := filepath.Join("/home/me", ".local", "bin")
	if !contains(pipx.bins, defaultBin) || !contains(uv.bins, defaultBin) {
		t.Errorf("default bins: pipx %v, uv %v, want %s", pipx.bins, uv.bins, defaultBin)
	}

	for _, tc := range []struct {
		env      map[string]string
		pipx, uv string
	}{
		{map[string]string{"PIPX_BIN_DIR": "/p/bin", "UV_TOOL_BIN_DIR": "/u/bin", "XDG_BIN_HOME": "/x/bin"}, "/p/bin", "/u/bin"},
		{map[string]string{"XDG_BIN_HOME": "/x/bin", "XDG_DATA_HOME": "/d/share"}, defaultBin, "/x/bin"},
		{map[string]string{"XDG_DATA_HOME": "/d/share"}, defaultBin, filepath.Join("/d/share", "..", "bin")},
	} {
		pipx, uv := toolLocations("/home/me", func(k string) string { return tc.env[k] })
		if len(pipx.bins) != 1 || pipx.bins[0] != tc.pipx {
			t.Errorf("env %v: pipx bins = %v, want %s", tc.env, pipx.bins, tc.pipx)
		}
		if len(uv.bins) != 1 || uv.bins[0] != tc.uv {
			t.Errorf("env %v: uv bins = %v, want %s", tc.env, uv.bins, tc.uv)
		}
	}
}

func TestQuoteArg(t *testing.T) {
	tests := []struct {
		in, goos, want string
	}{
		{"/home/me/proj/.venv/bin/python", "linux", "/home/me/proj/.venv/bin/python"},
		{`C:\proj\.venv\Scripts\python.exe`, "windows", `C:\proj\.venv\Scripts\python.exe`},
		{"/home/me/my proj", "linux", "'/home/me/my proj'"},
		{"/home/me/$(id)", "linux", "'/home/me/$(id)'"},
		{"/home/me/a`id`b", "darwin", "'/home/me/a`id`b'"},
		{`/home/me/a"b`, "linux", `'/home/me/a"b'`},
		{"/home/me/it's", "linux", `'/home/me/it'\''s'`},
		{`C:\$(id)`, "windows", `'C:\$(id)'`},
		{"C:\\a`id`b", "windows", "'C:\\a`id`b'"},
		{`C:\a"b`, "windows", `'C:\a"b'`},
		{`C:\it's`, "windows", `'C:\it''s'`},
		// A backslash is a path separator on Windows but an escape in POSIX shells.
		{`/home/me/a\b`, "linux", `'/home/me/a\b'`},
	}
	for _, tt := range tests {
		if got := quoteArg(tt.in, tt.goos); got != tt.want {
			t.Errorf("quoteArg(%q, %s) = %s, want %s", tt.in, tt.goos, got, tt.want)
		}
	}
}

func TestWheelUpdateCommandWindows(t *testing.T) {
	tests := []struct {
		name   string
		method installMethod
		wheel  wheelInstall
		want   string
	}{
		{
			name:   "quoted venv python uses the call operator",
			method: methodPip,
			wheel:  wheelInstall{python: `C:\Users\me\my proj\.venv\Scripts\python.exe`},
			want:   `& 'C:\Users\me\my proj\.venv\Scripts\python.exe' -m pip install --upgrade ` + pypiPackageName,
		},
		{
			name:   "unquoted venv python runs as is",
			method: methodPip,
			wheel:  wheelInstall{python: `C:\proj\.venv\Scripts\python.exe`},
			want:   `C:\proj\.venv\Scripts\python.exe -m pip install --upgrade ` + pypiPackageName,
		},
		{
			name:   "uv pip keeps the quoted path as an argument",
			method: methodUVPip,
			wheel:  wheelInstall{python: `C:\my proj\.venv\Scripts\python.exe`},
			want:   `uv pip install --python 'C:\my proj\.venv\Scripts\python.exe' --upgrade ` + pypiPackageName,
		},
		{
			name:   "uv project syncs only after a successful lock in PowerShell",
			method: methodUVProject,
			wheel:  wheelInstall{uvProject: `C:\proj`},
			want:   `uv lock --project C:\proj --upgrade-package ` + pypiPackageName + `; if ($?) { uv sync --project C:\proj }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wheelUpdateCommand(tt.method, tt.wheel, "windows"); got != tt.want {
				t.Errorf("wheelUpdateCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
