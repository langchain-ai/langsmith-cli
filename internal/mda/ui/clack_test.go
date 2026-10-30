package ui

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASessionIsFramedByItsIntroAndOutro(t *testing.T) {
	terminal, out := captured()

	require.NoError(t, terminal.Intro("langsmith deploy"))
	require.NoError(t, terminal.LogInfo("Project: /private/project"))
	require.NoError(t, terminal.LogWarning("OPENAI_API_KEY is set in both your shell and .env\nusing the shell value"))
	require.NoError(t, terminal.LogStep("Compiled"))
	require.NoError(t, terminal.OutroCancel("Cancelled"))

	assert.Equal(t,
		"┌  langsmith deploy\n"+
			"│\n"+
			"●  Project: /private/project\n"+
			"│  \n"+
			"▲  OPENAI_API_KEY is set in both your shell and .env\n"+
			"│  using the shell value\n"+
			"│  \n"+
			"◇  Compiled\n"+
			"│  \n"+
			"└  Cancelled\n",
		out.String())
}

func TestTheOutroCancelIsRed(t *testing.T) {
	terminal, out := captured(WithColors(true))

	require.NoError(t, terminal.OutroCancel("Cancelled"))

	assert.Equal(t, "\x1b[90m└\x1b[0m  \x1b[31mCancelled\x1b[0m\n", out.String())
}

func TestTheDefaultTerminalCanBeReplaced(t *testing.T) {
	terminal, out := captured()
	restore := SetDefault(terminal)

	require.NoError(t, Outro("Deployed 'agent' to LangSmith."))
	restore()

	assert.Equal(t, "└  Deployed 'agent' to LangSmith.\n", out.String())
	assert.NotSame(t, terminal, Default())
}

func TestASpinnerWithoutATerminalPrintsItsStartAndStopLines(t *testing.T) {
	terminal, out := captured(WithInteractive(false))
	spinner := terminal.Spinner()

	spinner.Start("Waiting for deployment")
	spinner.SetMessage("Waiting for deployment")
	spinner.SetMessage("BUILDING…")
	spinner.Stop("Deployment live")
	spinner.Stop("again")

	assert.Equal(t,
		"◒  Waiting for deployment\n"+
			"◒  BUILDING…\n"+
			"◇  Deployment live\n"+
			"│\n",
		out.String())
	assert.True(t, spinner.IsFinished())
}

func TestASpinnerEndsInTheStateItFinishedWith(t *testing.T) {
	cases := map[string]func(*ProgressBar){
		"■  Sign-in cancelled\n│\n":   func(p *ProgressBar) { p.Cancel("Sign-in cancelled") },
		"▲  Could not reconcile\n│\n": func(p *ProgressBar) { p.Error("Could not reconcile") },
		"":                            func(p *ProgressBar) { p.Clear() },
	}
	for expected, finish := range cases {
		terminal, out := captured(WithInteractive(false))
		spinner := terminal.Spinner()
		spinner.Start("Reconciling schedules")
		out.Reset()

		finish(spinner)

		assert.Equal(t, expected, out.String())
	}
}

// lockedBuffer is a buffer a running spinner and the test can share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestAnAnimatedSpinnerIsReplacedByItsStepLine(t *testing.T) {
	interval := tickInterval
	tickInterval = time.Hour
	t.Cleanup(func() { tickInterval = interval })
	var out lockedBuffer
	terminal := NewTerminal(strings.NewReader(""), &out, WithInteractive(true), WithColors(false), WithWidth(80))
	spinner := terminal.Spinner()

	spinner.Start("Compiling project")
	require.NoError(t, terminal.LogInfo("Project: /private/project"))
	spinner.Stop("Compiled project")

	erase := "\r" + ansi.EraseScreenBelow
	assert.Equal(t,
		"◒  Compiling project"+erase+
			"●  Project: /private/project\n│  \n"+
			"◒  Compiling project"+erase+
			"◇  Compiled project\n│\n",
		out.String())
}

func TestAnAnimatedSpinnerAdvancesItsFrames(t *testing.T) {
	interval := tickInterval
	tickInterval = time.Millisecond
	t.Cleanup(func() { tickInterval = interval })
	var out lockedBuffer
	terminal := NewTerminal(strings.NewReader(""), &out, WithInteractive(true), WithColors(false), WithWidth(80))
	spinner := terminal.Spinner()

	spinner.Start("Uploading source")
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(out.String(), "◓  Uploading source") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	spinner.Clear()

	assert.Contains(t, out.String(), "◒  Uploading source\r"+ansi.EraseScreenBelow+"◐  Uploading source\r"+ansi.EraseScreenBelow+"◓  Uploading source")
	assert.True(t, strings.HasSuffix(out.String(), "Uploading source\r"+ansi.EraseScreenBelow), "%q", out.String())
}
