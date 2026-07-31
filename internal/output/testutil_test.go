package output

import (
	"bytes"
	"io"
	"os"
	"testing"
	"unicode/utf8"
)

// captureStdout redirects os.Stdout during fn and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stdout, fn)
}

// captureStderr redirects os.Stderr during fn and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return capture(t, &os.Stderr, fn)
}

func capture(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	old := *target
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	*target = w

	fn()

	w.Close()
	*target = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading captured output: %v", err)
	}
	return buf.String()
}

func assertNoControlChars(t *testing.T, out string) {
	t.Helper()
	for _, r := range out {
		if r == '\n' {
			continue
		}
		if isTerminalControl(r) {
			t.Fatalf("output contains control char %U: %q", r, out)
		}
	}
	if !utf8.ValidString(out) {
		t.Fatalf("output is not valid UTF-8: %q", out)
	}
}
