package output

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner draws a live progress line for tests with a known duration window.
// It writes to stderr and is automatically suppressed in non-TTY environments.
type Spinner struct {
	label    string
	totalSec int
	active   bool
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	suppress bool
}

// NewSpinner creates a spinner. suppress=true when in --json mode or non-TTY.
func NewSpinner(suppress bool) *Spinner {
	if !isatty.IsTerminal(os.Stderr.Fd()) {
		suppress = true
	}
	return &Spinner{suppress: suppress}
}

// Start begins the live progress line for label over totalSec seconds.
func (s *Spinner) Start(label string, totalSec int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.label = label
	s.totalSec = totalSec
	if s.suppress {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.active = true
	start := time.Now()
	go func() {
		defer close(s.done)
		frame := 0
		for {
			select {
			case <-s.stop:
				return
			case <-time.After(100 * time.Millisecond):
				elapsed := int(time.Since(start).Seconds())
				fmt.Fprintf(os.Stderr, "\r  %-6s %s  %ds / %ds…  ",
					label, spinFrames[frame%len(spinFrames)], elapsed, totalSec)
				frame++
			}
		}
	}()
}

// Stop halts the spinner and writes finalLine to stderr, clearing the progress line.
func (s *Spinner) Stop(finalLine string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		close(s.stop)
		<-s.done
		s.active = false
		// Clear the progress line.
		fmt.Fprintf(os.Stderr, "\r%s\r", spaces(80))
	}
	_ = finalLine // final result printed by output.text via stdout
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
