// Package ui draws `langsmith deploy`'s terminal session the way the mda CLI
// does with the `cliclack` crate: the `┌ │ ◇ ◆ ▲ ■ └` bar layout, spinners and
// prompts. It also ports the link notes of `cli_output.rs` and the file
// reading of `secret_input.rs`. Only what deploy draws is ported.
//
// cliclack's free functions are package-level functions here that draw on the
// [Default] terminal, and methods of [Terminal] for any other one:
//
//	cliclack::intro(t)            Terminal.Intro(t), drawn by ShowWordmarkIntro()
//	cliclack::outro(m)            Outro(m)
//	cliclack::outro_cancel(m)     OutroCancel(m)
//	cliclack::note(p, m)          Note(p, m)
//	cliclack::log::info(m)        LogInfo(m), and LogWarning, LogSuccess, LogStep
//	cliclack::spinner()           NewSpinner(), a *Spinner
//	cliclack::confirm(p)          Confirm(p).InitialValue(v).Interact(ctx)
//	cliclack::input(p)            Input(p).Placeholder(s).Validate(f).Interact(ctx)
//	cliclack::password(p)         Password(p).Mask('•').Validate(f).Interact(ctx)
//	cliclack::select(p)           Select[T](p).Item(v, label, hint).Interact(ctx)
//
// cliclack's themes are not ported.
//
// All output goes to the terminal's writer, stderr by default, as cliclack
// writes to stderr. Without an interactive terminal (stdin or stderr not a
// terminal, or running under CI) nothing moves the cursor: a spinner prints
// its start and stop lines, and a prompt returns [ErrNotInteractive] for the
// caller to turn into "run interactively or pass a flag". Ctrl-C or Esc in a
// prompt returns [errs.ErrCancelled].
package ui

import (
	"io"
	"os"
	"sync"
	"sync/atomic"

	"golang.org/x/term"
)

// defaultWidth is the width assumed for a terminal whose size is unknown.
const defaultWidth = 80

// Terminal is where a session's output goes and its answers come from.
type Terminal struct {
	in  io.Reader
	out io.Writer

	inTerminal, outTerminal bool
	ci                      bool
	colors                  bool
	width                   int

	// mu serializes writes to out with the spinner drawn on it.
	mu      sync.Mutex
	spinner *Spinner
}

// Option configures a [Terminal] beyond what [NewTerminal] detects.
type Option func(*Terminal)

// WithColors turns colors on or off.
func WithColors(colors bool) Option {
	return func(t *Terminal) {
		t.colors = colors
	}
}

// WithInteractive treats the terminal as interactive, or not, whatever its
// streams are.
func WithInteractive(interactive bool) Option {
	return WithTerminals(interactive, interactive)
}

// WithTerminals treats input and output as terminals, or not, each whatever
// its stream is, outside CI.
func WithTerminals(input, output bool) Option {
	return func(t *Terminal) {
		t.inTerminal, t.outTerminal, t.ci = input, output, false
	}
}

// WithWidth fixes the terminal's width in columns.
func WithWidth(width int) Option {
	return func(t *Terminal) {
		t.width = width
	}
}

// NewTerminal returns a terminal that reads answers from in and writes to out,
// detecting whether they are terminals, whether CI runs it, and whether out
// takes colors the way the `console` crate does (`NO_COLOR`, `CLICOLOR`,
// `CLICOLOR_FORCE`).
func NewTerminal(in io.Reader, out io.Writer, options ...Option) *Terminal {
	t := &Terminal{
		in:          in,
		out:         out,
		inTerminal:  isTerminal(in),
		outTerminal: isTerminal(out),
		ci:          isCI(),
	}
	t.colors = colorsEnabled(t.outTerminal)
	for _, option := range options {
		option(t)
	}
	return t
}

var defaultTerminal atomic.Pointer[Terminal]

func init() {
	defaultTerminal.Store(NewTerminal(os.Stdin, os.Stderr))
}

// Default returns the terminal the package-level functions draw on: stdin and
// stderr unless [SetDefault] replaced it.
func Default() *Terminal {
	return defaultTerminal.Load()
}

// SetDefault makes t the terminal the package-level functions draw on, until
// restore puts the previous one back.
func SetDefault(t *Terminal) (restore func()) {
	previous := defaultTerminal.Swap(t)
	return func() {
		defaultTerminal.Store(previous)
	}
}

// IsOutputTerminal reports whether output goes to a terminal.
func (t *Terminal) IsOutputTerminal() bool {
	return t.outTerminal
}

// Interactive reports whether prompts can run: input and output are both a
// terminal, and CI is not running the command.
func (t *Terminal) Interactive() bool {
	return t.inTerminal && t.outTerminal && !t.ci
}

// ColorsEnabled reports whether output is styled.
func (t *Terminal) ColorsEnabled() bool {
	return t.colors
}

// Width returns the terminal's width in columns, 80 when it is unknown.
func (t *Terminal) Width() int {
	if t.width > 0 {
		return t.width
	}
	if file, ok := t.out.(*os.File); ok && t.outTerminal {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return defaultWidth
}

// animates reports whether a spinner may redraw itself in place.
func (t *Terminal) animates() bool {
	return t.outTerminal && !t.ci
}

func (t *Terminal) theme() theme {
	return theme{colors: t.colors, width: t.Width()}
}

// write writes text, above the running spinner if there is one.
func (t *Terminal) write(text string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.writeLocked(text)
}

func (t *Terminal) writeLocked(text string) error {
	if t.spinner != nil {
		t.spinner.eraseLocked()
	}
	_, err := io.WriteString(t.out, text)
	if t.spinner != nil {
		t.spinner.drawLocked()
	}
	return err
}

// IsOutputTerminal reports whether the default terminal's output goes to a
// terminal.
func IsOutputTerminal() bool {
	return Default().IsOutputTerminal()
}

// Interactive reports whether prompts can run on the default terminal.
func Interactive() bool {
	return Default().Interactive()
}

// ColorsEnabled reports whether the default terminal's output is styled.
func ColorsEnabled() bool {
	return Default().ColorsEnabled()
}

func isTerminal(stream any) bool {
	file, ok := stream.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func isCI() bool {
	switch os.Getenv("CI") {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

func colorsEnabled(outTerminal bool) bool {
	if force := os.Getenv("CLICOLOR_FORCE"); force != "" && force != "0" {
		return true
	}
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	return outTerminal && os.Getenv("TERM") != "dumb" && os.Getenv("CLICOLOR") != "0"
}
