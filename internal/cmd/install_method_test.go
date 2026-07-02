package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectInstallMethod(t *testing.T) {
	tests := []struct {
		name     string
		execPath string
		version  string
		goos     string
		gobin    string
		gopath   string
		home     string
		want     installMethod
	}{
		{
			name:     "homebrew apple silicon cellar",
			execPath: "/opt/homebrew/Cellar/langsmith-cli/0.2.38/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodHomebrew,
		},
		{
			name:     "homebrew intel cellar",
			execPath: "/usr/local/Cellar/langsmith-cli/0.2.38/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodHomebrew,
		},
		{
			name:     "homebrew linux cellar",
			execPath: "/home/linuxbrew/.linuxbrew/Cellar/langsmith-cli/0.2.38/bin/langsmith",
			version:  "0.2.38",
			goos:     "linux",
			home:     "/home/me",
			want:     methodHomebrew,
		},
		{
			name:     "scoop apps",
			execPath: `C:\Users\me\scoop\apps\langsmith-cli\current\langsmith.exe`,
			version:  "0.2.38",
			goos:     "windows",
			home:     `C:\Users\me`,
			want:     methodScoop,
		},
		{
			name:     "scoop shims",
			execPath: `C:\Users\me\scoop\shims\langsmith.exe`,
			version:  "0.2.38",
			goos:     "windows",
			home:     `C:\Users\me`,
			want:     methodScoop,
		},
		{
			name:     "go install via GOBIN",
			execPath: "/Users/me/dev/gobin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			gobin:    "/Users/me/dev/gobin",
			home:     "/Users/me",
			want:     methodGo,
		},
		{
			name:     "go install via GOPATH bin",
			execPath: "/Users/me/gopath/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			gopath:   "/Users/me/gopath",
			home:     "/Users/me",
			want:     methodGo,
		},
		{
			name:     "go install via default HOME/go/bin",
			execPath: "/Users/me/go/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodGo,
		},
		{
			name:     "go install with dev version still detected as go",
			execPath: "/Users/me/go/bin/langsmith",
			version:  "dev",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodGo,
		},
		{
			name:     "managed install.sh usr local bin",
			execPath: "/usr/local/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodManaged,
		},
		{
			name:     "managed install.sh local bin",
			execPath: "/Users/me/.local/bin/langsmith",
			version:  "0.2.38",
			goos:     "linux",
			home:     "/Users/me",
			want:     methodManaged,
		},
		{
			name:     "dev build outside package dirs",
			execPath: "/Users/me/repos/langsmith-cli/bin/langsmith",
			version:  "dev",
			goos:     "darwin",
			home:     "/Users/me",
			want:     methodDev,
		},
		{
			name:     "GOBIN takes precedence over GOPATH",
			execPath: "/Users/me/gopath/bin/langsmith",
			version:  "0.2.38",
			goos:     "darwin",
			gobin:    "/Users/me/gobin",
			gopath:   "/Users/me/gopath",
			home:     "/Users/me",
			// exec is in GOPATH/bin, but GOBIN is set and differs → not the go bin dir
			want: methodManaged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectInstallMethod(tt.execPath, tt.version, tt.goos, tt.gobin, tt.gopath, tt.home)
			if got != tt.want {
				t.Errorf("detectInstallMethod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstallMethodUpdateCommand(t *testing.T) {
	tests := map[installMethod]string{
		methodHomebrew: "brew upgrade langchain-ai/tap/langsmith-cli",
		methodScoop:    "scoop update langsmith-cli",
		methodGo:       "go install github.com/langchain-ai/langsmith-cli/cmd/langsmith@latest",
		methodManaged:  "",
		methodDev:      "",
	}
	for method, want := range tests {
		if got := method.updateCommand(); got != want {
			t.Errorf("updateCommand(%v) = %q, want %q", method, got, want)
		}
	}
}

func TestReportManagedExternally_JSON(t *testing.T) {
	oldFmt := flagOutputFormat
	flagOutputFormat = "json"
	defer func() { flagOutputFormat = oldFmt }()

	output := captureStdout(t, func() {
		if err := reportManagedExternally(methodHomebrew, "json"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON output: %v, output: %q", err, output)
	}
	if result["status"] != "managed-externally" {
		t.Errorf("expected status managed-externally, got %q", result["status"])
	}
	if result["install_method"] != "homebrew" {
		t.Errorf("expected install_method homebrew, got %q", result["install_method"])
	}
	if result["update_command"] != "brew upgrade langchain-ai/tap/langsmith-cli" {
		t.Errorf("unexpected update_command: %q", result["update_command"])
	}
}

func TestReportManagedExternally_Pretty(t *testing.T) {
	output := captureStdout(t, func() {
		if err := reportManagedExternally(methodScoop, "pretty"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "scoop update langsmith-cli") {
		t.Errorf("expected pretty output to contain the scoop command, got %q", output)
	}
	if !strings.Contains(output, "--force") {
		t.Errorf("expected pretty output to mention --force, got %q", output)
	}
}

func TestInstallMethodLabel(t *testing.T) {
	tests := map[installMethod]string{
		methodHomebrew: "homebrew",
		methodScoop:    "scoop",
		methodGo:       "go",
		methodDev:      "dev",
		methodManaged:  "managed",
	}
	for method, want := range tests {
		if got := method.label(); got != want {
			t.Errorf("label(%v) = %q, want %q", method, got, want)
		}
	}
}
