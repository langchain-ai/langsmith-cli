package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func plainTheme() theme {
	return theme{colors: false, width: defaultWidth}
}

func captured(options ...Option) (*Terminal, *bytes.Buffer) {
	var out bytes.Buffer
	options = append([]Option{WithColors(false), WithWidth(defaultWidth)}, options...)
	return NewTerminal(strings.NewReader(""), &out, options...), &out
}

func TestSetupFrameClosesBeforePlainChat(t *testing.T) {
	instruction := "Type a message, /new for a fresh thread, /exit to leave."

	output, err := renderSequence([]SequenceItem{NewSection("Conversation", "Thread: test", instruction)}, false, plainTheme())

	require.NoError(t, err)
	assert.Equal(t, "◇  Conversation\n│  Thread: test\n└  "+instruction+"\n\n", output)
}

func TestEmptyLinkSectionKeepsExistingOutput(t *testing.T) {
	links := []Link{{Label: "Agent Server", URL: "https://agent.example"}}

	assert.Equal(t, linkLines(links), sectionedLinkLines(links, "HTTP channels", nil))
}

func TestChannelLinksAreGroupedInsideDeploymentWithoutSplittingLongURLs(t *testing.T) {
	url := "https://agent.example/channels/orders/events"

	lines := sectionedLinkLines(
		[]Link{{Label: "Agent Server", URL: "https://agent.example"}},
		"HTTP channels",
		[]Link{{Label: "orders", URL: url}},
	)

	assert.Equal(t, []string{
		"Agent Server  https://agent.example",
		"",
		"HTTP channels",
		"  orders  https://agent.example/channels/orders/events",
	}, lines)
	assert.False(t, linksFit(lines, 40))
	assert.True(t, linksFit(lines, 120))
	assert.Contains(t, renderLinkLines("Deployment", lines, false), url)
}

func TestLinkLinesAlignURLsUnderPaddedLabels(t *testing.T) {
	assert.Equal(t, []string{
		"Agent Server  https://agent.us.langgraph.app",
		"Dashboard     https://smith.langchain.com/o/t-1",
	}, linkLines([]Link{
		{Label: "Agent Server", URL: "https://agent.us.langgraph.app"},
		{Label: "Dashboard", URL: "https://smith.langchain.com/o/t-1"},
	}))
}

func TestLinkLinesDropTheLabelColumnWhenNoLinkIsLabeled(t *testing.T) {
	assert.Equal(t, []string{"https://smith.langchain.com/o/t-1"},
		linkLines([]Link{{URL: "https://smith.langchain.com/o/t-1"}}))
}

func TestLinksFitOnlyWhileTheNoteBoxLeavesRoom(t *testing.T) {
	lines := []string{"Dashboard  https://smith.langchain.com"}

	assert.True(t, linksFit(lines, 44))
	assert.False(t, linksFit(lines, 43))
}

func TestUnboxedLinksStartAtColumnZeroUnderAStepHeading(t *testing.T) {
	lines := []string{"Agent Server  https://agent.us.langgraph.app"}

	assert.Equal(t,
		"◇  Deployment\n"+
			"Agent Server  https://agent.us.langgraph.app\n"+
			"│\n",
		renderLinkLines("Deployment", lines, false))
}

func TestLinksThatFitAreBoxedInANote(t *testing.T) {
	terminal, out := captured()

	require.NoError(t, terminal.NoteLinks("Deployment", []Link{
		{Label: "Agent Server", URL: "https://agent.us.langgraph.app"},
		{Label: "Dashboard", URL: "https://smith.langchain.com/o/t-1"},
	}))

	assert.Equal(t,
		"◇  Deployment ──────────────────────────────────────╮\n"+
			"│                                                   │\n"+
			"│  Agent Server  https://agent.us.langgraph.app     │\n"+
			"│  Dashboard     https://smith.langchain.com/o/t-1  │\n"+
			"├───────────────────────────────────────────────────╯\n"+
			"│\n",
		out.String())
}

func TestLinksThatDoNotFitAreLeftUnboxed(t *testing.T) {
	terminal, out := captured(WithWidth(40))

	require.NoError(t, terminal.NoteLinks("Deployment", []Link{
		{Label: "Agent Server", URL: "https://agent.us.langgraph.app"},
	}))

	assert.Equal(t, "◇  Deployment\nAgent Server  https://agent.us.langgraph.app\n│\n", out.String())
}

func TestALinkNoteNeedsALink(t *testing.T) {
	terminal, _ := captured()

	assert.EqualError(t, terminal.NoteLinks("Deployment", nil), "link notes require at least one link")
}

func TestSequenceOwnsItsShapeAndSpacing(t *testing.T) {
	items := []SequenceItem{
		NewSection("First heading", "first detail\nsecond detail"),
		NewSection("Second heading", "final detail"),
	}

	output, err := renderSequence(items, true, plainTheme())

	require.NoError(t, err)
	assert.Equal(t,
		"\n"+
			"◇  First heading\n"+
			"│  first detail\n"+
			"│  second detail\n"+
			"│  \n"+
			"◇  Second heading\n"+
			"└  final detail\n"+
			"\n",
		output)
}

func TestSequenceStylesHeadingsWhiteAndSupportingLinesGray(t *testing.T) {
	items := []SequenceItem{NewSection("Heading", "detail")}

	output, err := renderSequence(items, true, theme{colors: true, width: defaultWidth})

	require.NoError(t, err)
	assert.Equal(t,
		"\n"+
			"\x1b[32m◇\x1b[0m  \x1b[37mHeading\x1b[0m\n"+
			"\x1b[90m└\x1b[0m  \x1b[90mdetail\x1b[0m\n"+
			"\n",
		output)
}

func TestSequenceClosesAFinalAgentNote(t *testing.T) {
	items := []SequenceItem{AgentNote{Title: "Agent note", Body: "Follow these instructions"}}

	output, err := renderSequence(items, true, plainTheme())

	require.NoError(t, err)
	assert.Equal(t,
		"\n"+
			"├  Agent note ────────────────╮\n"+
			"│                             │\n"+
			"│  Follow these instructions  │\n"+
			"└─────────────────────────────╯\n"+
			"\n",
		output)
}

func TestAnOpenSequenceLeavesTheBarOpen(t *testing.T) {
	terminal, out := captured()

	require.NoError(t, terminal.ContinueOpenSequence([]SequenceItem{NewSection("Redirect URI", "https://example.com/callback")}))

	assert.Equal(t, "◇  Redirect URI\n│  https://example.com/callback\n│  \n", out.String())
}

func TestASequenceRejectsEmptyAndMultilineItems(t *testing.T) {
	_, empty := renderSequence(nil, true, plainTheme())
	_, multiline := renderSequence([]SequenceItem{NewSection("two\nlines", "detail")}, true, plainTheme())
	_, bodiless := renderSequence([]SequenceItem{AgentNote{Title: "Agent note"}}, true, plainTheme())

	assert.EqualError(t, empty, "CLI output sequences require at least one item")
	assert.EqualError(t, multiline, "CLI output sequence items require a single-line title and non-empty content")
	assert.EqualError(t, bodiless, "CLI output sequence items require a single-line title and non-empty content")
}

func TestWordmarkIntroOpensALangSmithDeploySession(t *testing.T) {
	terminal, out := captured()

	require.NoError(t, terminal.ShowWordmarkIntro())

	assert.Equal(t, "\n┌  langsmith deploy\n│\n", out.String())
}

func TestANoteWithAStyledEmptyTitleHasNoStepSymbol(t *testing.T) {
	rendered := ansi.Strip(plainTheme().formatNoteGeneric(false, "\x1b[34m\x1b[0m", "my message"))

	assert.Equal(t,
		"├──────────────╮\n"+
			"│  my message  │\n"+
			"├──────────────╯\n"+
			"│\n",
		rendered)
}
