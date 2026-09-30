//go:build integration

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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

func decodeLive[T any](t *testing.T, stdout string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(stdout), &v); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	return v
}
