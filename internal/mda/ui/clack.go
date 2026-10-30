package ui

// Intro prints the header of a prompt sequence (like `┌  title`).
func (t *Terminal) Intro(title string) error {
	return t.write(t.theme().formatIntro(title))
}

// Outro prints the footer of a prompt sequence (like `└  message`).
func (t *Terminal) Outro(message string) error {
	return t.write(t.theme().formatOutro(message))
}

// OutroCancel prints the footer of a prompt sequence in a failure style.
func (t *Terminal) OutroCancel(message string) error {
	return t.write(t.theme().formatOutroCancel(message))
}

// Note prints a boxed note titled prompt. With an empty prompt, the note has
// no title.
func (t *Terminal) Note(prompt, message string) error {
	return t.write(t.theme().formatNote(prompt, message))
}

// LogInfo prints an info message. The text can be multiline.
func (t *Terminal) LogInfo(text string) error {
	theme := t.theme()
	return t.write(theme.formatLog(text, theme.infoSymbol()))
}

// LogWarning prints a warning. The text can be multiline.
func (t *Terminal) LogWarning(text string) error {
	theme := t.theme()
	return t.write(theme.formatLog(text, theme.warningSymbol()))
}

// LogSuccess prints a success message. The text can be multiline.
func (t *Terminal) LogSuccess(text string) error {
	theme := t.theme()
	return t.write(theme.formatLog(text, theme.activeSymbol()))
}

// LogStep prints a submitted step. The text can be multiline.
func (t *Terminal) LogStep(text string) error {
	theme := t.theme()
	return t.write(theme.formatLog(text, theme.submitSymbol()))
}

// Outro prints the footer of a prompt sequence on the default terminal.
func Outro(message string) error {
	return Default().Outro(message)
}

// OutroCancel prints the footer of a prompt sequence in a failure style on
// the default terminal.
func OutroCancel(message string) error {
	return Default().OutroCancel(message)
}

// Note prints a boxed note on the default terminal.
func Note(prompt, message string) error {
	return Default().Note(prompt, message)
}

// LogInfo prints an info message on the default terminal.
func LogInfo(text string) error {
	return Default().LogInfo(text)
}

// LogWarning prints a warning on the default terminal.
func LogWarning(text string) error {
	return Default().LogWarning(text)
}

// LogSuccess prints a success message on the default terminal.
func LogSuccess(text string) error {
	return Default().LogSuccess(text)
}

// LogStep prints a submitted step on the default terminal.
func LogStep(text string) error {
	return Default().LogStep(text)
}
