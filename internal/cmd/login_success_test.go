package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWriteLoginSuccessPlainWhenNotTerminal(t *testing.T) {
	var out bytes.Buffer
	writeLoginSuccess(&out, "https://api.smith.langchain.com", "default")
	want := "Logged in to https://api.smith.langchain.com as profile \"default\"\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q", out.String(), want)
	}
}

func TestWriteLoginSuccessShowsLogoOnTerminal(t *testing.T) {
	prev := outputIsTerminal
	outputIsTerminal = func(io.Writer) bool { return true }
	t.Cleanup(func() { outputIsTerminal = prev })
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "1")

	var out bytes.Buffer
	writeLoginSuccess(&out, "https://api.smith.langchain.com", "work")
	got := out.String()
	for _, want := range []string{loginLogo[0], "✓ You're in.", `Logged in as profile "work".`, "API  https://api.smith.langchain.com"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI escapes:\n%q", got)
	}
}

func TestLoginLogoRowsAlign(t *testing.T) {
	width := utf8.RuneCountInString(loginLogo[0])
	for i, row := range loginLogo {
		if n := utf8.RuneCountInString(row); n != width {
			t.Fatalf("row %d has width %d, want %d", i, n, width)
		}
	}
}
