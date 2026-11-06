package ui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// ErrNotInteractive is what a prompt returns without an interactive terminal
// to ask on. Callers turn it into directions to run interactively or pass the
// answer as a flag.
var ErrNotInteractive = errs.Categorized(errs.UserInput, "no interactive terminal to prompt on")

// interaction is a prompt that renders itself for a state and handles keys,
// as cliclack's `PromptInteraction` is.
type interaction interface {
	// render draws the prompt in state.
	render(theme theme, state themeState) string
	// on handles a key the prompt's text input, if any, has already applied.
	on(key tea.KeyMsg) themeState
	// input is the text the keys edit, or nil for a prompt without one.
	input() *stringCursor
	// wordEditing reports whether keys may edit the input a word at a time.
	wordEditing() bool
}

// promptModel runs an interaction as a Bubble Tea program.
type promptModel struct {
	interaction interaction
	theme       theme
	state       themeState
}

func (m *promptModel) Init() tea.Cmd {
	return nil
}

func (m *promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.state = cancelState
		return m, tea.Quit
	case tea.KeyCtrlA:
		key = tea.KeyMsg{Type: tea.KeyHome}
	case tea.KeyCtrlE:
		key = tea.KeyMsg{Type: tea.KeyEnd}
	case tea.KeyCtrlP:
		key = tea.KeyMsg{Type: tea.KeyUp}
	case tea.KeyCtrlN:
		key = tea.KeyMsg{Type: tea.KeyDown}
	}
	if cursor := m.interaction.input(); cursor != nil {
		edit(cursor, key, m.interaction.wordEditing())
	}
	m.state = m.interaction.on(key)
	if m.state.kind == stateSubmit {
		return m, tea.Quit
	}
	return m, nil
}

func (m *promptModel) View() string {
	return m.interaction.render(m.theme, m.state)
}

// edit applies an editing key to cursor.
func edit(cursor *stringCursor, key tea.KeyMsg, wordEditing bool) {
	switch key.Type {
	case tea.KeyRunes, tea.KeySpace:
		if key.Alt {
			if wordEditing {
				switch string(key.Runes) {
				case "b":
					cursor.moveLeftByWord()
				case "f":
					cursor.moveRightByWord()
				}
			}
			return
		}
		for _, r := range key.Runes {
			cursor.insert(r)
		}
	case tea.KeyBackspace:
		if key.Alt && wordEditing {
			cursor.deleteWordToTheLeft()
		} else {
			cursor.deleteLeft()
		}
	case tea.KeyCtrlW:
		if wordEditing {
			cursor.deleteWordToTheLeft()
		}
	case tea.KeyDelete:
		cursor.deleteRight()
	case tea.KeyLeft:
		cursor.moveLeft()
	case tea.KeyRight:
		cursor.moveRight()
	case tea.KeyCtrlLeft:
		if wordEditing {
			cursor.moveLeftByWord()
		}
	case tea.KeyCtrlRight:
		if wordEditing {
			cursor.moveRightByWord()
		}
	case tea.KeyHome:
		cursor.moveHome()
	case tea.KeyEnd:
		cursor.moveEnd()
	}
}

// interact runs prompt on t until it is submitted or cancelled.
func (t *Terminal) interact(ctx context.Context, prompt interaction) error {
	if !t.Interactive() {
		return ErrNotInteractive
	}
	model := &promptModel{interaction: prompt, theme: t.theme(), state: activeState}
	program := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(t.in),
		tea.WithOutput(t.out),
		tea.WithoutSignalHandler(),
	)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, err := program.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	if model.state.kind == stateCancel {
		return errs.ErrCancelled
	}
	return nil
}

// Validator checks an answer; its error is shown under the prompt.
type Validator func(value string) error

func validate(validator Validator, value string) (themeState, bool) {
	if validator == nil {
		return activeState, true
	}
	if err := validator(value); err != nil {
		return errorState(err.Error()), false
	}
	return activeState, true
}

// noInput is the part of interaction for prompts without a text input.
type noInput struct{}

func (noInput) input() *stringCursor { return nil }
func (noInput) wordEditing() bool    { return false }

// ConfirmPrompt asks for a yes or no confirmation.
//
//   - Arrows switch the answer.
//   - Enter confirms it.
//   - `Y`/`y` answers yes at once, `N`/`n` no.
type ConfirmPrompt struct {
	noInput
	prompt       string
	initialValue bool
	value        bool
}

// Confirm returns a confirmation prompt.
func Confirm(prompt string) *ConfirmPrompt {
	return &ConfirmPrompt{prompt: prompt}
}

// InitialValue sets the initially selected answer.
func (c *ConfirmPrompt) InitialValue(value bool) *ConfirmPrompt {
	c.initialValue = value
	return c
}

// Interact asks on the default terminal.
func (c *ConfirmPrompt) Interact(ctx context.Context) (bool, error) {
	return c.InteractOn(ctx, Default())
}

// InteractOn asks on t.
func (c *ConfirmPrompt) InteractOn(ctx context.Context, t *Terminal) (bool, error) {
	c.value = c.initialValue
	if err := t.interact(ctx, c); err != nil {
		return false, err
	}
	return c.value, nil
}

func (c *ConfirmPrompt) on(key tea.KeyMsg) themeState {
	switch key.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight:
		c.value = !c.value
	case tea.KeyEnter:
		return submitState
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "h", "j", "k", "l":
			c.value = !c.value
		case "y", "Y":
			c.value = true
			return submitState
		case "n", "N":
			c.value = false
			return submitState
		}
	}
	return activeState
}

func (c *ConfirmPrompt) render(theme theme, state themeState) string {
	return theme.formatHeader(state, c.prompt) +
		theme.formatConfirm(state, c.value) +
		theme.formatFooter(state)
}

// InputPrompt asks for a line of text.
type InputPrompt struct {
	prompt      string
	text        *stringCursor
	placeholder *stringCursor
	validator   Validator
}

// Input returns a text prompt. An empty answer is refused.
func Input(prompt string) *InputPrompt {
	return &InputPrompt{
		prompt:      prompt,
		text:        newStringCursor(""),
		placeholder: newStringCursor(""),
	}
}

// Placeholder sets the dimmed hint shown while the input is empty.
func (i *InputPrompt) Placeholder(placeholder string) *InputPrompt {
	i.placeholder = newStringCursor(placeholder)
	return i
}

// Validate checks the answer when it is submitted.
func (i *InputPrompt) Validate(validator Validator) *InputPrompt {
	i.validator = validator
	return i
}

// Interact asks on the default terminal.
func (i *InputPrompt) Interact(ctx context.Context) (string, error) {
	return i.InteractOn(ctx, Default())
}

// InteractOn asks on t.
func (i *InputPrompt) InteractOn(ctx context.Context, t *Terminal) (string, error) {
	if err := t.interact(ctx, i); err != nil {
		return "", err
	}
	return i.text.String(), nil
}

func (i *InputPrompt) input() *stringCursor { return i.text }
func (i *InputPrompt) wordEditing() bool    { return true }

func (i *InputPrompt) on(key tea.KeyMsg) themeState {
	if key.Type != tea.KeyEnter {
		return activeState
	}
	if i.text.isEmpty() {
		return errorState("Input required")
	}
	if state, ok := validate(i.validator, i.text.String()); !ok {
		return state
	}
	return submitState
}

func (i *InputPrompt) render(theme theme, state themeState) string {
	body := theme.formatInput(state, i.text)
	if i.text.isEmpty() {
		body = theme.formatPlaceholder(state, i.placeholder)
	}
	return theme.formatHeader(state, i.prompt) + body + theme.formatFooterWithMessage(state, "")
}

// PasswordPrompt asks for a secret, masking what is typed.
type PasswordPrompt struct {
	prompt    string
	mask      rune
	text      *stringCursor
	validator Validator
}

// Password returns a masked prompt. An empty answer is refused.
func Password(prompt string) *PasswordPrompt {
	return &PasswordPrompt{prompt: prompt, mask: symbolPasswordMask, text: newStringCursor("")}
}

// Mask sets the character shown for each typed one, like `•`.
func (p *PasswordPrompt) Mask(mask rune) *PasswordPrompt {
	p.mask = mask
	return p
}

// Validate checks the answer when it is submitted.
func (p *PasswordPrompt) Validate(validator Validator) *PasswordPrompt {
	p.validator = validator
	return p
}

// Interact asks on the default terminal.
func (p *PasswordPrompt) Interact(ctx context.Context) (string, error) {
	return p.InteractOn(ctx, Default())
}

// InteractOn asks on t.
func (p *PasswordPrompt) InteractOn(ctx context.Context, t *Terminal) (string, error) {
	if err := t.interact(ctx, p); err != nil {
		return "", err
	}
	return p.text.String(), nil
}

func (p *PasswordPrompt) input() *stringCursor { return p.text }

// wordEditing is off so as not to reveal the secret's structure.
func (p *PasswordPrompt) wordEditing() bool { return false }

func (p *PasswordPrompt) on(key tea.KeyMsg) themeState {
	if key.Type != tea.KeyEnter {
		return activeState
	}
	if p.text.isEmpty() {
		return errorState("Input required")
	}
	if state, ok := validate(p.validator, p.text.String()); !ok {
		return state
	}
	return submitState
}

func (p *PasswordPrompt) render(theme theme, state themeState) string {
	return theme.formatHeader(state, p.prompt) +
		theme.formatInput(state, p.text.masked(p.mask)) +
		theme.formatFooter(state)
}

// SelectPrompt asks for one of a list of options.
type SelectPrompt[T comparable] struct {
	noInput
	prompt string
	items  []selectItem[T]
	cursor int
}

type selectItem[T comparable] struct {
	value T
	label string
	hint  string
}

// Select returns a prompt choosing one value of type T.
func Select[T comparable](prompt string) *SelectPrompt[T] {
	return &SelectPrompt[T]{prompt: prompt}
}

// Item adds an option: value is what choosing it returns, hint is shown
// beside the highlighted option.
func (s *SelectPrompt[T]) Item(value T, label, hint string) *SelectPrompt[T] {
	s.items = append(s.items, selectItem[T]{value: value, label: label, hint: hint})
	return s
}

// Interact asks on the default terminal.
func (s *SelectPrompt[T]) Interact(ctx context.Context) (T, error) {
	return s.InteractOn(ctx, Default())
}

// InteractOn asks on t.
func (s *SelectPrompt[T]) InteractOn(ctx context.Context, t *Terminal) (T, error) {
	var zero T
	if len(s.items) == 0 {
		return zero, errors.New("no items added to the list")
	}
	if err := t.interact(ctx, s); err != nil {
		return zero, err
	}
	return s.items[s.cursor].value, nil
}

func (s *SelectPrompt[T]) on(key tea.KeyMsg) themeState {
	up, down := key.Type == tea.KeyUp || key.Type == tea.KeyLeft, key.Type == tea.KeyDown || key.Type == tea.KeyRight
	if key.Type == tea.KeyRunes {
		switch string(key.Runes) {
		case "k", "h":
			up = true
		case "j", "l":
			down = true
		}
	}
	switch {
	case up && s.cursor > 0:
		s.cursor--
	case down && s.cursor < len(s.items)-1:
		s.cursor++
	case key.Type == tea.KeyEnter:
		return submitState
	}
	return activeState
}

func (s *SelectPrompt[T]) render(theme theme, state themeState) string {
	rendered := theme.formatHeader(state, s.prompt)
	for index, item := range s.items {
		rendered += theme.formatSelectItem(state, index == s.cursor, item.label, item.hint)
	}
	return rendered + theme.formatFooterWithMessage(state, "")
}
