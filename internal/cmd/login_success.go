package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var loginLogo = []string{
	"▄█▄               ",
	"████▄             ",
	"██████▄ ▄▄▄▄▄▄    ",
	" ██████ ██████▀   ",
	"  ▀████  ▀██▀     ",
	"    ▀█ ▄███▄▄     ",
	"██████▀███████▄   ",
	" █████  ▀███████▄ ",
	"  ▀▀█▀    ▀▀███▀▀ ",
}

var outputIsTerminal = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// writeLoginSuccess celebrates with the pixel logo on interactive terminals and
// keeps the single-line message when output is piped or captured.
func writeLoginSuccess(w io.Writer, apiURL, profileName string) {
	if !outputIsTerminal(w) || getenv("TERM") == "dumb" {
		fmt.Fprintf(w, "Logged in to %s as profile %q\n", apiURL, profileName)
		return
	}
	style := func(code, s string) string { return s }
	if getenv("NO_COLOR") == "" {
		style = func(code, s string) string { return "\x1b[" + code + "m" + s + "\x1b[0m" }
	}
	captions := map[int]string{
		2: style("1;94", "✓ You're in."),
		4: fmt.Sprintf("Logged in as profile %q.", profileName),
		6: style("2", "Ready when you are."),
	}
	var b strings.Builder
	b.WriteString("\n")
	for i, row := range loginLogo {
		line := style("94", row)
		if caption, ok := captions[i]; ok {
			line += "  " + caption
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + style("2", "API") + "  " + apiURL + "\n")
	fmt.Fprint(w, b.String())
}
