// Package output renders CLI output. Printf, Println, Fprintf, Fprintln, Errorf
// and Errorln sanitize every string-like argument, so text from the API cannot
// act on the terminal or forge lines; format strings are kept as written. The
// table and tree renderers sanitize the values they are given.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func Printf(format string, args ...any) {
	writeRaw(os.Stdout, fmt.Sprintf(format, sanitizeArgs(args)...))
}

func Println(args ...any) {
	writeRaw(os.Stdout, fmt.Sprintln(sanitizeArgs(args)...))
}

func Fprintf(w io.Writer, format string, args ...any) {
	writeRaw(w, fmt.Sprintf(format, sanitizeArgs(args)...))
}

func Fprintln(w io.Writer, args ...any) {
	writeRaw(w, fmt.Sprintln(sanitizeArgs(args)...))
}

// PrintLines writes multi-line text to stdout, keeping its line breaks and
// sanitizing everything else. The newline is the only control character it keeps.
func PrintLines(s string) {
	FprintLines(os.Stdout, s)
}

// FprintLines writes multi-line text to w, keeping its line breaks.
func FprintLines(w io.Writer, s string) {
	writeRaw(w, sanitizeLines(s))
}

// PrintProxied writes s to stdout unsanitized. Only for output another program
// produced, where the escape sequences are that program's own. Never for API text.
func PrintProxied(s string) {
	writeRaw(os.Stdout, s)
}

// FprintProxied writes s to w unsanitized. Same contract as PrintProxied.
func FprintProxied(w io.Writer, s string) {
	writeRaw(w, s)
}

func Errorf(format string, args ...any) {
	writeRaw(os.Stderr, fmt.Sprintf(format, sanitizeArgs(args)...))
}

func Errorln(args ...any) {
	writeRaw(os.Stderr, fmt.Sprintln(sanitizeArgs(args)...))
}

func sanitizeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case string:
			out[i] = SanitizeTerminal(v)
		case error:
			out[i] = SanitizeTerminal(v.Error())
		case fmt.Stringer:
			out[i] = SanitizeTerminal(v.String())
		default:
			out[i] = a
		}
	}
	return out
}

func writeRaw(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

const replacementChar = '\ufffd'

// SanitizeTerminal replaces every control character a terminal would act on,
// tabs and newlines included, with a visible placeholder. Idempotent.
func SanitizeTerminal(s string) string {
	if !strings.ContainsFunc(s, isTerminalControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isTerminalControl(r) {
			b.WriteRune(replacementChar)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isTerminalControl(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

func sanitizeLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = SanitizeTerminal(line)
	}
	return strings.Join(lines, "\n")
}

func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == max {
			return s[:i]
		}
		count++
	}
	return s
}
