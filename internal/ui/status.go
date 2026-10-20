package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// Status is the live line shown while a scan runs: a spinner, the elapsed
// time and what is being looked at. Finished steps stay on screen as ✓ lines.
// Off a terminal only the ✓ lines are printed. Safe for concurrent use.
type Status struct {
	w         io.Writer
	live      bool
	mu        sync.Mutex
	msg       string
	start     time.Time
	stepStart time.Time
	frame     int
	stop      chan struct{}
	stopped   chan struct{}
}

// NewStatus starts a status line on w; call Stop when the scan is over.
func NewStatus(w io.Writer) *Status {
	now := time.Now()
	s := &Status{w: w, start: now, stepStart: now, stop: make(chan struct{}), stopped: make(chan struct{})}
	s.live = w == io.Writer(os.Stdout) && IsTerminal(os.Stdout)
	if !s.live {
		close(s.stopped)
		return s
	}
	go func() {
		defer close(s.stopped)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				s.mu.Lock()
				s.draw()
				s.mu.Unlock()
			}
		}
	}()
	return s
}

// Set changes what the live line says is happening now.
func (s *Status) Set(format string, a ...any) {
	s.mu.Lock()
	s.msg = fmt.Sprintf(format, a...)
	s.mu.Unlock()
}

// Step prints a finished step with the time it took.
func (s *Status) Step(format string, a ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	fmt.Fprintf(s.w, "   %s %s %s\n", Green("✓"), fmt.Sprintf(format, a...), Dim(seconds(time.Since(s.stepStart))))
	s.stepStart = time.Now()
	s.msg = ""
	s.draw()
}

// Stop removes the live line.
func (s *Status) Stop() {
	if s.live {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
	<-s.stopped
	s.mu.Lock()
	s.clear()
	s.live = false
	s.mu.Unlock()
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (s *Status) draw() {
	if !s.live {
		return
	}
	s.frame++
	line := fmt.Sprintf(" %s %s %s", spinner[s.frame%len(spinner)], seconds(time.Since(s.start)), Safe(s.msg))
	width := 80
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil && ws.Col > 0 {
		width = int(ws.Col)
	}
	if utf8.RuneCountInString(line) >= width { // a wrapped line could not be cleared with \r
		r := []rune(line)
		line = string(r[:width-2]) + "…"
	}
	fmt.Fprint(s.w, "\r\x1b[K"+line)
}

func (s *Status) clear() {
	if s.live {
		fmt.Fprint(s.w, "\r\x1b[K")
	}
}

func seconds(d time.Duration) string {
	if d < 10*time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}
