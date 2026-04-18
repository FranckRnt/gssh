package runner

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const barWidth = 30

// progress tracks and renders an inline progress bar on a terminal.
type progress struct {
	mu      sync.Mutex
	w       io.Writer
	total   int
	done    int
	success int
	failed  int
	start   time.Time
	verbose bool
}

func newProgress(w io.Writer, total int, verbose bool) *progress {
	return &progress{
		w:       w,
		total:   total,
		start:   time.Now(),
		verbose: verbose,
	}
}

// complete records one completed result and redraws the bar.
func (p *progress) complete(ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.done++
	if ok {
		p.success++
	} else {
		p.failed++
	}

	if !p.verbose {
		p.render()
	}
}

// finish prints the final state and moves to a new line.
func (p *progress) finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.verbose {
		p.render()
		fmt.Fprintln(p.w)
	}
}

// elapsed returns the total elapsed time.
func (p *progress) elapsed() time.Duration {
	return time.Since(p.start)
}

// clear erases the progress bar line. Must be called with mu held.
func (p *progress) clear() {
	if !p.verbose {
		// Overwrite the line with spaces and return to start.
		fmt.Fprintf(p.w, "\r%s\r", strings.Repeat(" ", barWidth+40))
	}
}

// ClearAndRender temporarily clears the bar, calls fn, then redraws.
// This prevents output from mixing with the progress bar.
func (p *progress) ClearAndRender(fn func()) {
	p.mu.Lock()
	p.clear()
	p.mu.Unlock()

	fn()

	p.mu.Lock()
	if !p.verbose {
		p.render()
	}
	p.mu.Unlock()
}

// render draws the bar. Must be called with mu held.
func (p *progress) render() {
	pct := float64(p.done) / float64(p.total)
	filled := int(pct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	elapsed := time.Since(p.start).Round(time.Second)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	fmt.Fprintf(p.w, "\r[%s] %d/%d  ✓ %d  ✗ %d  %s",
		bar, p.done, p.total, p.success, p.failed, elapsed)
}
