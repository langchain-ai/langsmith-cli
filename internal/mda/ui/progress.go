package ui

import (
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// tickInterval is how often a running spinner advances a frame.
var tickInterval = 100 * time.Millisecond

// Spinner is a line that animates while work runs and is replaced by a step
// line when it ends, like cliclack's.
//
// Without an animating terminal it prints its message once when it starts and
// again each time the message changes, then the step line.
type Spinner struct {
	t *Terminal

	// Guarded by t.mu.
	message  string
	frame    int
	started  bool
	finished bool
	// rows is how many rows of the terminal the drawn frame takes.
	rows int
	// printed is the last message a non-animating spinner printed.
	printed string

	quit chan struct{}
	done chan struct{}
}

// Spinner returns a spinner on t. Nothing shows until [Spinner.Start].
func (t *Terminal) Spinner() *Spinner {
	return &Spinner{t: t}
}

// NewSpinner returns a spinner on the default terminal.
func NewSpinner() *Spinner {
	return Default().Spinner()
}

// Start shows the spinner with message.
func (p *Spinner) Start(message string) {
	t := p.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if p.started || p.finished {
		return
	}
	p.started = true
	p.message = t.theme().formatProgressMessage(message)
	if !t.animates() {
		p.printLocked()
		return
	}
	if t.spinner != nil {
		t.spinner.eraseLocked()
	}
	t.spinner = p
	p.drawLocked()
	p.quit, p.done = make(chan struct{}), make(chan struct{})
	go p.tick()
}

// SetMessage replaces the spinner's message.
func (p *Spinner) SetMessage(message string) {
	t := p.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if p.finished {
		return
	}
	p.message = message
	if !p.started {
		return
	}
	if !t.animates() {
		p.printLocked()
		return
	}
	p.eraseLocked()
	p.drawLocked()
}

// Stop ends the spinner with a submitted step line.
func (p *Spinner) Stop(message string) {
	p.finish(message, submitState)
}

// Cancel ends the spinner with a cancelled step line.
func (p *Spinner) Cancel(message string) {
	p.finish(message, cancelState)
}

// Error ends the spinner with an error step line.
func (p *Spinner) Error(message string) {
	p.finish(message, errorState(""))
}

// Clear ends the spinner, leaving nothing behind.
func (p *Spinner) Clear() {
	p.finish("", submitState)
}

func (p *Spinner) finish(message string, state themeState) {
	t := p.t
	t.mu.Lock()
	if p.finished {
		t.mu.Unlock()
		return
	}
	p.finished = true
	quit, done := p.quit, p.done
	t.mu.Unlock()
	if quit != nil {
		close(quit)
		<-done
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	p.eraseLocked()
	if t.spinner == p {
		t.spinner = nil
	}
	if message != "" {
		_ = t.writeLocked(t.theme().formatProgressWithState(message, state) + "\n")
	}
}

func (p *Spinner) tick() {
	defer close(p.done)
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.quit:
			return
		case <-ticker.C:
			p.t.mu.Lock()
			if !p.finished {
				p.frame++
				p.eraseLocked()
				p.drawLocked()
			}
			p.t.mu.Unlock()
		}
	}
}

// drawLocked draws the current frame, each line cut to the terminal's width
// so the frame takes one row per line, and leaves the cursor at its end.
func (p *Spinner) drawLocked() {
	if p.finished {
		return
	}
	t := p.t
	width := t.Width()
	frame := lines(t.theme().formatProgressFrame(p.frame, p.message))
	for index, line := range frame {
		frame[index] = ansi.Truncate(line, width-1, "")
	}
	_, _ = io.WriteString(t.out, strings.Join(frame, "\n"))
	p.rows = len(frame)
}

// eraseLocked removes the drawn frame, leaving the cursor where it started.
func (p *Spinner) eraseLocked() {
	if p.rows == 0 {
		return
	}
	erase := "\r"
	if p.rows > 1 {
		erase += ansi.CursorUp(p.rows - 1)
	}
	_, _ = io.WriteString(p.t.out, erase+ansi.EraseScreenBelow)
	p.rows = 0
}

// printLocked prints the message of a spinner that cannot animate, unless it
// already printed it.
func (p *Spinner) printLocked() {
	if p.message == p.printed {
		return
	}
	p.printed = p.message
	_, _ = io.WriteString(p.t.out, p.t.theme().formatProgressFrame(0, p.message)+"\n")
}
