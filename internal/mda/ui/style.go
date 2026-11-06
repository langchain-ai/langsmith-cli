package ui

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Style is a console style applied to whole strings, the way the mda CLI's
// `console::Style::apply_to` is. A style made for a terminal without colors
// leaves text as it is.
type Style struct {
	colors        bool
	foreground    string
	faint         bool
	strikethrough bool
	reverse       bool
}

// ANSI color numbers of the palette cliclack draws with.
const (
	red     = "1"
	green   = "2"
	yellow  = "3"
	blue    = "4"
	magenta = "5"
	cyan    = "6"
	white   = "7"
	gray    = "8"
)

// colorRenderer renders styles whether or not the process's own streams are
// terminals: each [Style] carries that decision itself.
var colorRenderer = func() *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.ANSI256)
	return renderer
}()

func newStyle(colors bool) Style {
	return Style{colors: colors}
}

func (s Style) fg(color string) Style {
	s.foreground = color
	return s
}

func (s Style) dim() Style {
	s.faint = true
	return s
}

func (s Style) strike() Style {
	s.strikethrough = true
	return s
}

func (s Style) reversed() Style {
	s.reverse = true
	return s
}

// Apply styles text. Each line is styled on its own, so a multi-line string
// keeps its layout.
func (s Style) Apply(text string) string {
	if !s.colors || text == "" {
		return text
	}
	style := colorRenderer.NewStyle().
		TabWidth(lipgloss.NoTabConversion).
		Faint(s.faint).
		Strikethrough(s.strikethrough).
		Reverse(s.reverse)
	if s.foreground != "" {
		style = style.Foreground(lipgloss.Color(s.foreground))
	}
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if line != "" {
			lines[index] = style.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

// SupportingStyle is the gray of guidance's supporting lines and bars.
func SupportingStyle(colors bool) Style {
	return newStyle(colors).fg(gray)
}

// DimStyle is the faint text of a sub-line under a spinner, the mda CLI's
// `console::style(…).dim()`.
func DimStyle(colors bool) Style {
	return newStyle(colors).dim()
}

// HeadingStyle is the white of guidance's headings.
func HeadingStyle(colors bool) Style {
	return newStyle(colors).fg(white)
}
