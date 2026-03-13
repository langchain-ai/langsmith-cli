package deployment

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Progress displays a CLI spinner with an optional elapsed time indicator.
type Progress struct {
	message     string
	baseMessage string
	showElapsed bool
	stop        chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	delay       time.Duration
}

// NewProgress creates a new Progress spinner.
func NewProgress(message string, showElapsed bool) *Progress {
	return &Progress{
		message:     message,
		baseMessage: message,
		showElapsed: showElapsed,
		stop:        make(chan struct{}),
		delay:       100 * time.Millisecond,
	}
}

// Start begins the spinner in a background goroutine.
// Returns a function that can be used to update the message.
// If stdout is not a terminal, messages are printed to stderr instead.
func (p *Progress) Start() func(string) {
	if !isTerminal() {
		return func(msg string) {
			if msg != "" {
				fmt.Fprintln(os.Stderr, msg)
			}
		}
	}

	p.wg.Add(1)
	go p.spin()

	return func(msg string) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.message = msg
		if msg != "" {
			p.baseMessage = msg
		}
	}
}

// Stop halts the spinner.
func (p *Progress) Stop() {
	close(p.stop)
	p.wg.Wait()
}

func (p *Progress) spin() {
	defer p.wg.Done()
	chars := []rune{'|', '/', '-', '\\'}
	idx := 0
	start := time.Now()

	for {
		select {
		case <-p.stop:
			return
		default:
		}

		p.mu.Lock()
		msg := p.message
		if p.showElapsed && msg != "" {
			elapsed := time.Since(start)
			secs := int(elapsed.Seconds())
			mins := secs / 60
			secs = secs % 60
			if mins > 0 {
				msg = fmt.Sprintf("%s (%dm %02ds)", p.baseMessage, mins, secs)
			} else {
				msg = fmt.Sprintf("%s (%ds)", p.baseMessage, secs)
			}
			p.message = msg
		}
		p.mu.Unlock()

		if msg != "" {
			display := fmt.Sprintf("%c %s", chars[idx%len(chars)], msg)
			fmt.Fprint(os.Stdout, display)
			time.Sleep(p.delay)
			// Clear the line
			clear := fmt.Sprintf("\r%s\r", spacePad(len(display)))
			fmt.Fprint(os.Stdout, clear)
		} else {
			time.Sleep(p.delay)
		}
		idx++
	}
}

func spacePad(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
