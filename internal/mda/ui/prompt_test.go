package ui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/langchain-ai/langsmith-cli/internal/mda/errs"
)

// press feeds keys to prompt the way a running program would, and returns
// the model with the frame it shows last.
func press(prompt interaction, keys ...tea.KeyMsg) (*promptModel, string) {
	model := &promptModel{interaction: prompt, theme: plainTheme(), state: activeState}
	for _, key := range keys {
		model.Update(key)
	}
	return model, model.View()
}

func runes(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

var (
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	esc   = tea.KeyMsg{Type: tea.KeyEsc}
)

func TestAConfirmShowsBothAnswersUntilOneIsChosen(t *testing.T) {
	prompt := Confirm("Deploy anyway?")
	prompt.value = true

	_, active := press(prompt)
	model, submitted := press(prompt, tea.KeyMsg{Type: tea.KeyRight}, enter)

	assert.Equal(t, "◆  Deploy anyway?\n│  ● Yes  / ○ No \n└  \n", active)
	assert.Equal(t, stateSubmit, model.state.kind)
	assert.False(t, prompt.value)
	assert.Equal(t, "◇  Deploy anyway?\n│  No \n│\n", submitted)
}

func TestEscapeCancelsAPrompt(t *testing.T) {
	model, frame := press(Confirm("Deploy anyway?"), esc)

	assert.Equal(t, stateCancel, model.state.kind)
	assert.Equal(t, "■  Deploy anyway?\n│  No \n└  Operation cancelled.\n", frame)
}

func TestAnInputRequiresAnAnswer(t *testing.T) {
	_, required := press(Input("Organization name").Placeholder("e.g. acme"), enter)

	assert.Equal(t, "▲  Organization name\n│  e.g. acme\n└  Input required\n", required)
}

func TestAnInputShowsItsValidationErrorUntilTheNextKey(t *testing.T) {
	prompt := Input("GitHub App ID").Validate(func(value string) error {
		if value != "42" {
			return assert.AnError
		}
		return nil
	})

	_, invalid := press(prompt, runes("4"), runes("1"), enter)
	_, editing := press(prompt, tea.KeyMsg{Type: tea.KeyBackspace})
	model, _ := press(prompt, runes("2"), enter)

	assert.Equal(t, "▲  GitHub App ID\n│  41 \n└  "+assert.AnError.Error()+"\n", invalid)
	assert.Equal(t, "◆  GitHub App ID\n│  4 \n└  \n", editing)
	assert.Equal(t, stateSubmit, model.state.kind)
	assert.Equal(t, "42", prompt.text.String())
}

func TestAPasswordMasksWhatIsTyped(t *testing.T) {
	prompt := Password("Signing Secret").Mask('•')

	_, typing := press(prompt, runes("s3"), tea.KeyMsg{Type: tea.KeyLeft})
	_, submitted := press(prompt, enter)

	assert.Equal(t, "◆  Signing Secret\n│  ••\n└  \n", typing)
	assert.Equal(t, "◇  Signing Secret\n│  ••\n│\n", submitted)
	assert.Equal(t, "s3", prompt.text.String())
}

func TestAPasswordKeepsItsStructureFromWordEditing(t *testing.T) {
	prompt := Password("Client secret")

	press(prompt, runes("two words"), tea.KeyMsg{Type: tea.KeyCtrlW})

	assert.Equal(t, "two words", prompt.text.String())
}

func TestASelectMovesBetweenItemsAndShowsTheHighlightedHint(t *testing.T) {
	prompt := Select[string]("This agent's GitHub channel needs a GitHub App").
		Item("new", "Create a new GitHub App", "recommended").
		Item("existing", "Use an existing GitHub App", "")

	_, active := press(prompt)
	model, submitted := press(prompt, runes("j"), enter)

	assert.Equal(t,
		"◆  This agent's GitHub channel needs a GitHub App\n"+
			"│  ● Create a new GitHub App (recommended)\n"+
			"│  ○ Use an existing GitHub App \n"+
			"└  \n",
		active)
	assert.Equal(t, stateSubmit, model.state.kind)
	assert.Equal(t, "◇  This agent's GitHub channel needs a GitHub App\n│  Use an existing GitHub App \n│\n", submitted)
}

func TestAPromptAnswersThroughTheTerminal(t *testing.T) {
	var out bytes.Buffer
	terminal := NewTerminal(strings.NewReader("y"), &out, WithInteractive(true), WithColors(false))

	answer, err := Confirm("Deploy anyway?").InteractOn(t.Context(), terminal)

	require.NoError(t, err)
	assert.True(t, answer)
	assert.Contains(t, out.String(), "◇  Deploy anyway?")
}

func TestCtrlCInAPromptIsACancellation(t *testing.T) {
	terminal := NewTerminal(strings.NewReader("\x03"), &bytes.Buffer{}, WithInteractive(true))

	_, err := Select[int]("Choose").Item(1, "one", "").InteractOn(t.Context(), terminal)

	assert.ErrorIs(t, err, errs.ErrCancelled)
	assert.True(t, errs.IsCancellation(err))
}

func TestACancelledContextEndsAPrompt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	input, typist := io.Pipe()
	t.Cleanup(func() { _ = typist.Close() })
	terminal := NewTerminal(input, &bytes.Buffer{}, WithInteractive(true))

	_, err := Input("Organization name").InteractOn(ctx, terminal)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestAPromptWithoutATerminalAsksForAFlag(t *testing.T) {
	terminal := NewTerminal(strings.NewReader("y"), &bytes.Buffer{})

	_, err := Confirm("Deploy anyway?").InteractOn(t.Context(), terminal)

	assert.ErrorIs(t, err, ErrNotInteractive)
	assert.Equal(t, errs.UserInput, errs.CategoryOf(err))
}

func TestASelectWithoutItemsFails(t *testing.T) {
	terminal, _ := captured(WithInteractive(true))

	_, err := Select[string]("Select an item").InteractOn(t.Context(), terminal)

	assert.EqualError(t, err, "no items added to the list")
}

func TestAnInputEditsAWordAtATime(t *testing.T) {
	prompt := Input("Organization name")

	press(prompt, runes("acme corp"), tea.KeyMsg{Type: tea.KeyCtrlW}, runes("inc"),
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}, runes("x"))

	assert.Equal(t, "acme xinc", prompt.text.String())
}
