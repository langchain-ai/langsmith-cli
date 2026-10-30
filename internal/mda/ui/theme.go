package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

// The symbols of cliclack's default @clack/prompts theme.
const (
	symbolStepActive = "◆"
	symbolStepCancel = "■"
	symbolStepError  = "▲"
	symbolStepSubmit = "◇"

	symbolBarStart = "┌"
	symbolBar      = "│"
	symbolBarEnd   = "└"

	symbolRadioActive   = "●"
	symbolRadioInactive = "○"
	symbolPasswordMask  = '▪'

	symbolBarH              = "─"
	symbolCornerTopRight    = "╮"
	symbolConnectLeft       = "├"
	symbolCornerBottomRight = "╯"

	symbolInfo  = "●"
	symbolWarn  = "▲"
	symbolError = "■"
)

// spinnerFrames are the frames a spinner cycles through. cliclack's tick
// characters are "◒◐◓◑", whose last one indicatif keeps for a finished bar.
var spinnerFrames = []string{"◒", "◐", "◓"}

// stateKind is where a prompt's rendering is.
type stateKind int

const (
	// stateActive is a prompt waiting for input.
	stateActive stateKind = iota
	// stateCancel is a prompt the user aborted with Esc or Ctrl-C.
	stateCancel
	// stateSubmit is a prompt the user answered with Enter.
	stateSubmit
	// stateError is an answer that failed validation.
	stateError
)

// themeState is the state of a prompt's rendering, with the validation
// message of an error.
type themeState struct {
	kind stateKind
	err  string
}

var (
	activeState = themeState{kind: stateActive}
	cancelState = themeState{kind: stateCancel}
	submitState = themeState{kind: stateSubmit}
)

func errorState(message string) themeState {
	return themeState{kind: stateError, err: message}
}

// theme renders cliclack's default @clack/prompts theme.
type theme struct {
	colors bool
	// width is the terminal's width in columns, for wrapping notes.
	width int
}

func (t theme) style() Style {
	return newStyle(t.colors)
}

// barColor is the color of the vertical side bar.
func (t theme) barColor(state themeState) Style {
	switch state.kind {
	case stateActive:
		return t.style().fg(cyan)
	case stateCancel:
		return t.style().fg(red)
	case stateError:
		return t.style().fg(yellow)
	default:
		return t.style().fg(gray)
	}
}

// stateSymbolColor is the color of the symbol of state.
func (t theme) stateSymbolColor(state themeState) Style {
	if state.kind == stateSubmit {
		return t.style().fg(green)
	}
	return t.barColor(state)
}

// stateSymbol is the symbol of state.
func (t theme) stateSymbol(state themeState) string {
	symbol := symbolStepActive
	switch state.kind {
	case stateCancel:
		symbol = symbolStepCancel
	case stateSubmit:
		symbol = symbolStepSubmit
	case stateError:
		symbol = symbolStepError
	}
	return t.stateSymbolColor(state).Apply(symbol)
}

// radioSymbol is the symbol of a select list's item.
func (t theme) radioSymbol(state themeState, selected bool) string {
	if state.kind != stateActive {
		return ""
	}
	if selected {
		return t.style().fg(green).Apply(symbolRadioActive)
	}
	return t.style().dim().Apply(symbolRadioInactive)
}

func (t theme) remarkSymbol() string {
	return t.barColor(submitState).Apply(symbolConnectLeft)
}

func (t theme) infoSymbol() string {
	return t.style().fg(blue).Apply(symbolInfo)
}

func (t theme) warningSymbol() string {
	return t.style().fg(yellow).Apply(symbolWarn)
}

func (t theme) errorSymbol() string {
	return t.style().fg(red).Apply(symbolError)
}

func (t theme) activeSymbol() string {
	return t.style().fg(green).Apply(symbolStepActive)
}

func (t theme) submitSymbol() string {
	return t.style().fg(green).Apply(symbolStepSubmit)
}

// inputStyle is the style of a prompt's input text.
func (t theme) inputStyle(state themeState) Style {
	switch state.kind {
	case stateCancel:
		return t.style().dim().strike()
	case stateSubmit:
		return t.style().dim()
	default:
		return t.style()
	}
}

// placeholderStyle is the style of a placeholder; a cancelled prompt hides it.
func (t theme) placeholderStyle(state themeState) Style {
	return t.style().dim()
}

// cursorWithStyle highlights the cursor character, styling the rest of the
// text with style.
func (t theme) cursorWithStyle(cursor *stringCursor, style Style) string {
	left, at, right := cursor.split()
	return style.Apply(left) + t.style().reversed().Apply(at) + style.Apply(right)
}

// formatIntro formats the intro message (like `┌  title`).
func (t theme) formatIntro(title string) string {
	color := t.barColor(submitState)
	return color.Apply(symbolBarStart) + "  " + title + "\n" + color.Apply(symbolBar) + "\n"
}

// formatOutro formats the outro message (like `└  message`).
func (t theme) formatOutro(message string) string {
	return t.barColor(submitState).Apply(symbolBarEnd) + "  " + message + "\n"
}

// formatOutroCancel formats the outro message with a failure style (like
// `└  message` in red).
func (t theme) formatOutroCancel(message string) string {
	return t.barColor(submitState).Apply(symbolBarEnd) + "  " + t.style().fg(red).Apply(message) + "\n"
}

// formatHeader formats the header of a prompt (like `◇  Input data`).
func (t theme) formatHeader(state themeState, prompt string) string {
	var header strings.Builder
	for index, line := range lines(prompt) {
		if index == 0 {
			header.WriteString(t.stateSymbol(state) + "  " + line + "\n")
		} else {
			header.WriteString(t.barColor(state).Apply(symbolBar) + "  " + line + "\n")
		}
	}
	return header.String()
}

// formatFooter formats the footer of a prompt (like `└  Operation
// cancelled.`).
func (t theme) formatFooter(state themeState) string {
	return t.formatFooterWithMessage(state, "")
}

// formatFooterWithMessage formats the footer with a custom message (like
// `└  message`).
func (t theme) formatFooterWithMessage(state themeState, message string) string {
	var footer string
	switch state.kind {
	case stateActive:
		footer = symbolBarEnd + "  " + message
	case stateCancel:
		footer = symbolBarEnd + "  Operation cancelled."
	case stateSubmit:
		footer = symbolBar
	case stateError:
		footer = symbolBarEnd + "  " + state.err
	}
	return t.barColor(state).Apply(footer) + "\n"
}

// formatInput formats the input with frame bars around it, hiding the cursor
// when the input is not active.
func (t theme) formatInput(state themeState, cursor *stringCursor) string {
	style := t.inputStyle(state)
	var input string
	if state.kind == stateActive || state.kind == stateError {
		input = t.cursorWithStyle(cursor, style)
	} else {
		input = cursor.String()
	}
	if strings.HasSuffix(input, "\n") {
		input += "\n"
	}
	var framed strings.Builder
	for _, line := range lines(input) {
		framed.WriteString(t.barColor(state).Apply(symbolBar) + "  " + style.Apply(line) + "\n")
	}
	return framed.String()
}

// formatPlaceholder formats the placeholder in its dimmed style: hidden once
// the prompt is cancelled, without the cursor once it is submitted.
func (t theme) formatPlaceholder(state themeState, cursor *stringCursor) string {
	style := t.placeholderStyle(state)
	var placeholder string
	switch state.kind {
	case stateActive, stateError:
		placeholder = t.cursorWithStyle(cursor, style)
	case stateCancel:
		placeholder = ""
	default:
		placeholder = cursor.String()
	}
	var framed strings.Builder
	for _, line := range lines(placeholder) {
		framed.WriteString(t.barColor(state).Apply(symbolBar) + "  " + style.Apply(line) + "\n")
	}
	return framed.String()
}

// radioItem is a select list item without frame bars, hidden if not
// selected once the prompt is submitted or cancelled.
func (t theme) radioItem(state themeState, selected bool, label, hint string) string {
	if (state.kind == stateCancel || state.kind == stateSubmit) && !selected {
		return ""
	}
	radio := t.radioSymbol(state, selected)
	if selected {
		label = t.inputStyle(state).Apply(label)
	} else {
		label = t.placeholderStyle(state).Apply(label)
	}
	if (state.kind == stateActive || state.kind == stateError) && hint != "" && selected {
		hint = t.placeholderStyle(state).Apply("(" + hint + ")")
	} else {
		hint = ""
	}
	var item strings.Builder
	item.WriteString(radio)
	if radio != "" {
		item.WriteString(" ")
	}
	item.WriteString(label)
	if label != "" {
		item.WriteString(" ")
	}
	item.WriteString(hint)
	return item.String()
}

// formatSelectItem is a select list item with frame bars, hidden if not
// selected once the prompt is submitted or cancelled.
func (t theme) formatSelectItem(state themeState, selected bool, label, hint string) string {
	if (state.kind == stateCancel || state.kind == stateSubmit) && !selected {
		return ""
	}
	return t.barColor(state).Apply(symbolBar) + "  " + t.radioItem(state, selected, label, hint) + "\n"
}

// formatConfirm formats a confirmation prompt's choices.
func (t theme) formatConfirm(state themeState, confirm bool) string {
	yes := t.radioItem(state, confirm, "Yes", "")
	no := t.radioItem(state, !confirm, "No", "")
	divider := ""
	if state.kind == stateActive {
		divider = t.placeholderStyle(state).Apply(" / ")
	}
	return t.barColor(state).Apply(symbolBar) + "  " + yes + divider + no + "\n"
}

// formatProgressMessage gives a multi-line progress message the side bar on
// every line but the first, ending with the section end.
func (t theme) formatProgressMessage(text string) string {
	bar := t.barColor(submitState).Apply(symbolBar)
	end := t.barColor(submitState).Apply(symbolBarEnd)
	all := lines(text)
	parts := make([]string, len(all))
	for index, line := range all {
		switch {
		case index == 0:
			parts[index] = line
		case index < len(all)-1:
			parts[index] = bar + "  " + line
		default:
			parts[index] = end + "  " + line
		}
	}
	return strings.Join(parts, "\n")
}

// formatProgressFrame is a running spinner's line: its frame, then message.
func (t theme) formatProgressFrame(frame int, message string) string {
	return t.style().fg(magenta).Apply(spinnerFrames[frame%len(spinnerFrames)]) + "  " + message
}

// formatProgressWithState is a finished spinner's line, like `◇  Done`
// followed by a bar.
func (t theme) formatProgressWithState(message string, state themeState) string {
	prefix, suffix := "", ""
	if state.kind != stateActive {
		prefix = t.stateSymbol(state) + "  "
		suffix = "\n" + t.barColor(submitState).Apply(symbolBar)
	}
	if message == "" {
		return suffix
	}
	return prefix + message + suffix
}

// formatNoteWithSymbol renders a boxed note with symbol, inline or as an
// outro, with its body in text.
func (t theme) formatNoteWithSymbol(isOutro bool, symbol, prompt, message string, text Style) string {
	if displayWidth(prompt) == 0 {
		prompt = ""
	}
	prompt = t.termwrap(prompt, 7)
	message = t.termwrap(message, 6)

	barColor := t.barColor(submitState)
	if isOutro {
		symbol = barColor.Apply(symbolConnectLeft)
	}
	promptLines := lines(prompt)
	if prompt == "" {
		// Omit the empty line inside the box above the message.
		message += "\n"
	} else {
		// Add a line above the message, below the header.
		message = "\n" + message + "\n"
	}

	// The header line is the last prompt line.
	lastHeaderLine := ""
	if len(promptLines) > 0 {
		lastHeaderLine = "  " + promptLines[len(promptLines)-1] + " "
	}
	width := displayWidth(lastHeaderLine)
	for _, line := range strings.Split(message, "\n") {
		width = max(width, displayWidth(line)+2)
	}

	var note strings.Builder
	// Preamble lines (all but the last): symbol on the first, bar on the rest.
	for index := 0; index < len(promptLines)-1; index++ {
		if index == 0 {
			note.WriteString(symbol + "  " + promptLines[index] + "\n")
		} else {
			note.WriteString(barColor.Apply(symbolBar) + "  " + promptLines[index] + "\n")
		}
	}
	leftSymbol := symbol
	if len(promptLines) > 1 {
		leftSymbol = barColor.Apply(symbolBar)
	}
	note.WriteString(leftSymbol + lastHeaderLine +
		barColor.Apply(strings.Repeat(symbolBarH, 2+width-displayWidth(lastHeaderLine))) +
		barColor.Apply(symbolCornerTopRight) + "\n")

	for _, line := range lines(message) {
		note.WriteString(barColor.Apply(symbolBar) + "  " + text.Apply(line) +
			strings.Repeat(" ", width-displayWidth(line)) + barColor.Apply(symbolBar) + "\n")
	}

	horizontal := strings.Repeat(symbolBarH, width+2)
	if isOutro {
		note.WriteString(barColor.Apply(symbolBarEnd+horizontal+symbolCornerBottomRight) + "\n")
	} else {
		note.WriteString(barColor.Apply(symbolConnectLeft+horizontal+symbolCornerBottomRight) + "\n" +
			barColor.Apply(symbolBar) + "\n")
	}
	return note.String()
}

// formatNoteGeneric renders a note with the remark symbol when it has no
// title and the submitted step's otherwise.
func (t theme) formatNoteGeneric(isOutro bool, prompt, message string) string {
	symbol := t.stateSymbol(submitState)
	if displayWidth(prompt) == 0 {
		symbol = t.remarkSymbol()
	}
	return t.formatNoteWithSymbol(isOutro, symbol, prompt, message, t.inputStyle(submitState))
}

// formatLog renders a log message with symbol, followed by an empty bar line.
func (t theme) formatLog(text, symbol string) string {
	parts := lines(text)
	parts = append(parts, "")
	var log strings.Builder
	for index, line := range parts {
		if index == 0 {
			log.WriteString(symbol + "  " + line + "\n")
		} else {
			log.WriteString(t.barColor(submitState).Apply(symbolBar) + "  " + line + "\n")
		}
	}
	return log.String()
}

// termwrap wraps each line of text to the terminal's width minus padding.
func (t theme) termwrap(text string, padding int) string {
	limit := t.width - padding
	wrapped := lines(text)
	for index, line := range wrapped {
		if limit > 0 {
			wrapped[index] = wrap.String(wordwrap.String(line, limit), limit)
		}
	}
	return strings.Join(wrapped, "\n")
}

// lines splits text into lines the way Rust's `str::lines` does: on `\n` or
// `\r\n`, with no empty line after a final newline.
func lines(text string) []string {
	if text == "" {
		return nil
	}
	split := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for index, line := range split {
		split[index] = strings.TrimSuffix(line, "\r")
	}
	return split
}

// displayWidth is the width of text in terminal cells, ignoring ANSI escape
// sequences.
func displayWidth(text string) int {
	return lipgloss.Width(text)
}
