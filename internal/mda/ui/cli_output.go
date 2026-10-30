package ui

import (
	"errors"
	"fmt"
	"strings"
)

// The session intro and link notes.

// wordmarkTitle opens a `langsmith deploy` session.
const wordmarkTitle = "langsmith deploy"

// ShowWordmarkIntro opens the session on t with its intro.
func (t *Terminal) ShowWordmarkIntro() error {
	if err := t.write("\n"); err != nil {
		return fmt.Errorf("failed to show CLI wordmark: %w", err)
	}
	if err := t.Intro(wordmarkTitle); err != nil {
		return fmt.Errorf("failed to start CLI session: %w", err)
	}
	return nil
}

// ShowWordmarkIntro opens the session on the default terminal.
func ShowWordmarkIntro() error {
	return Default().ShowWordmarkIntro()
}

// noteBodyPadding is the horizontal room [Terminal.Note] spends on its box:
// the border columns plus the message indent. A body line wider than the
// terminal minus this padding is hard-wrapped inside the box.
const noteBodyPadding = 6

// Link is a labeled URL in a link note.
type Link struct {
	Label string
	URL   string
}

// NoteLinks prints a titled block of labeled URLs, keeping every URL on one
// line.
//
// [Terminal.Note] hard-wraps its body to the terminal width and indents each
// line inside a box, which splits a long URL across lines with border
// characters in between — the URL stops being clickable and copies back
// broken. So the box is used only while every link fits; otherwise the links
// are printed unindented, where the terminal soft-wraps them and each URL
// stays a single selectable, clickable run.
//
// A link with an empty label prints as the bare URL.
func (t *Terminal) NoteLinks(title string, links []Link) error {
	return t.NoteLinksWithSection(title, links, "", nil)
}

// NoteLinksWithSection adds an optional group inside the same link note,
// preserving URL wrapping behavior.
func (t *Terminal) NoteLinksWithSection(title string, links []Link, sectionTitle string, sectionLinks []Link) error {
	lines := sectionedLinkLines(links, sectionTitle, sectionLinks)
	if len(lines) == 0 {
		return errors.New("link notes require at least one link")
	}

	if linksFit(lines, t.Width()) {
		if err := t.Note(title, strings.Join(lines, "\n")); err != nil {
			return fmt.Errorf("failed to show CLI output: %w", err)
		}
		return nil
	}

	if err := t.write(renderLinkLines(title, lines, t.colors)); err != nil {
		return fmt.Errorf("failed to show CLI output: %w", err)
	}
	return nil
}

// NoteLinks prints a link note on the default terminal.
func NoteLinks(title string, links []Link) error {
	return Default().NoteLinks(title, links)
}

// NoteLinksWithSection prints a link note with a group on the default
// terminal.
func NoteLinksWithSection(title string, links []Link, sectionTitle string, sectionLinks []Link) error {
	return Default().NoteLinksWithSection(title, links, sectionTitle, sectionLinks)
}

func sectionedLinkLines(links []Link, sectionTitle string, sectionLinks []Link) []string {
	lines := linkLines(links)
	if len(sectionLinks) > 0 {
		lines = append(lines, "", sectionTitle)
		for _, line := range linkLines(sectionLinks) {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

// linkLines is one line per link, labels padded into a column so the URLs
// line up.
func linkLines(links []Link) []string {
	labelWidth := 0
	for _, link := range links {
		labelWidth = max(labelWidth, displayWidth(link.Label))
	}

	lines := make([]string, 0, len(links))
	for _, link := range links {
		if labelWidth == 0 {
			lines = append(lines, link.URL)
		} else {
			padding := strings.Repeat(" ", labelWidth-displayWidth(link.Label))
			lines = append(lines, link.Label+padding+"  "+link.URL)
		}
	}
	return lines
}

// linksFit reports whether every link line survives [Terminal.Note]
// unwrapped.
func linksFit(lines []string, terminalWidth int) bool {
	for _, line := range lines {
		if displayWidth(line)+noteBodyPadding > terminalWidth {
			return false
		}
	}
	return true
}

// renderLinkLines renders the unboxed form: the step heading, then the links
// at column zero so nothing but the terminal's own soft wrap touches them.
func renderLinkLines(title string, lines []string, colors bool) string {
	marker := newStyle(colors).fg(green)
	var rendered strings.Builder
	rendered.WriteString(marker.Apply("◇") + "  " + HeadingStyle(colors).Apply(title) + "\n")
	for _, line := range lines {
		rendered.WriteString(line + "\n")
	}
	rendered.WriteString(SupportingStyle(colors).Apply("│") + "\n")
	return rendered.String()
}
