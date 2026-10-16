//go:build integration

package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tidwall/gjson"
)

var (
	liveBinaryOnce sync.Once
	liveBinaryPath string
	liveBinaryErr  error
)

// liveBinary builds the langsmith binary once per test run. Live tests run
// each command in its own process, as users and agents do: the generated
// command tree keeps flag state from one run to the next within a process.
func liveBinary(t *testing.T) string {
	t.Helper()
	liveBinaryOnce.Do(func() {
		dir, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			liveBinaryErr = err
			return
		}
		liveBinaryPath = filepath.Join(os.TempDir(), "langsmith-live-test")
		build := exec.Command("go", "build", "-o", liveBinaryPath, "./cmd/langsmith")
		build.Dir = dir
		if out, err := build.CombinedOutput(); err != nil {
			liveBinaryErr = errors.New(string(out))
		}
	})
	if liveBinaryErr != nil {
		t.Fatalf("building langsmith: %v", liveBinaryErr)
	}
	return liveBinaryPath
}

// runGeneratedLive runs `langsmith --format json <args>` against the API
// configured by LANGSMITH_API_KEY and LANGSMITH_ENDPOINT and returns stdout and
// the exit code.
func runGeneratedLive(t *testing.T, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"--format", "json"}, args...)
	cmd := exec.Command(liveBinary(t), full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running langsmith %s: %v", strings.Join(full, " "), err)
	}
	if code != 0 {
		t.Logf("langsmith %s exited %d\nstderr: %s", strings.Join(full, " "), code, stderr.String())
	}
	return stdout.String(), code
}

// TestGeneratedLifecycleIntegration runs each generated resource's lifecycle
// steps against the live API.
func TestGeneratedLifecycleIntegration(t *testing.T) {
	requireIntegrationEnv(t)
	for name, file := range loadGeneratedTests(t) {
		if len(file.Lifecycle.Steps) == 0 {
			continue
		}
		t.Run(name, func(t *testing.T) {
			runGeneratedLifecycle(t, name, file.Lifecycle)
		})
	}
}

func runGeneratedLifecycle(t *testing.T, resource string, lifecycle generatedLifecycle) {
	run := make([]byte, 4)
	_, _ = rand.Read(run)
	vars := map[string]string{"run": hex.EncodeToString(run)}
	live := map[string]bool{}
	expand := func(s string) string {
		for name, value := range vars {
			s = strings.ReplaceAll(s, "{{"+name+"}}", value)
		}
		return s
	}
	command := func(step generatedLifecycleStep) []string {
		args := []string{resource}
		for _, arg := range step.Args {
			args = append(args, expand(arg))
		}
		return args
	}

	t.Cleanup(func() {
		for _, step := range lifecycle.Cleanup {
			ready := true
			for _, name := range step.Needs {
				ready = ready && live[name]
			}
			if ready {
				runGeneratedLive(t, command(step)...)
			}
		}
	})

	for _, step := range lifecycle.Steps {
		args := command(step)
		stdout, code := runGeneratedLive(t, args...)
		if code != step.ExitCode {
			t.Fatalf("%s: langsmith %s exited %d, want %d", step.Name, strings.Join(args, " "), code, step.ExitCode)
		}
		for name, path := range step.Capture {
			value := gjson.Get(stdout, path)
			if !value.Exists() {
				t.Fatalf("%s: output has no %q to capture\n%s", step.Name, path, stdout)
			}
			vars[name] = value.String()
			live[name] = true
		}
		for path, want := range step.Expect {
			if got := gjson.Get(stdout, expand(path)).String(); got != expand(want) {
				t.Fatalf("%s: %s = %q, want %q", step.Name, path, got, expand(want))
			}
		}
		for _, path := range step.Exists {
			if !gjson.Get(stdout, expand(path)).Exists() {
				t.Fatalf("%s: output has nothing at %q\n%s", step.Name, expand(path), stdout)
			}
		}
		for _, name := range step.Forget {
			live[name] = false
		}
	}
}
