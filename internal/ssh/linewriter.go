package ssh

import (
	"bytes"
	"sync"
)

// StreamCallback is called for each complete line of output during streaming.
// stream is "stdout" or "stderr".
type StreamCallback func(hostname, stream, line string)

// LineWriter is an io.Writer that both captures output (like LimitedBuffer)
// and calls a callback for each complete line as it arrives.
type LineWriter struct {
	mu        sync.Mutex
	buf       []byte // total captured output
	partial   []byte // incomplete line pending a newline
	maxBytes  int
	truncated int64
	hostname  string
	stream    string // "stdout" or "stderr"
	cb        StreamCallback
}

// NewLineWriter creates a writer that captures up to maxBytes and streams
// complete lines via cb. If cb is nil, it behaves like LimitedBuffer.
func NewLineWriter(maxBytes int, hostname, stream string, cb StreamCallback) *LineWriter {
	return &LineWriter{
		buf:      make([]byte, 0, min(maxBytes, 4096)),
		maxBytes: maxBytes,
		hostname: hostname,
		stream:   stream,
		cb:       cb,
	}
}

// Write implements io.Writer.
func (lw *LineWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	n := len(p)

	// Capture to buffer (with limit).
	remaining := lw.maxBytes - len(lw.buf)
	if remaining > 0 {
		capture := p
		if len(capture) > remaining {
			lw.truncated += int64(len(capture) - remaining)
			capture = capture[:remaining]
		}
		lw.buf = append(lw.buf, capture...)
	} else {
		lw.truncated += int64(n)
	}

	// Stream complete lines to callback.
	if lw.cb != nil {
		lw.partial = append(lw.partial, p...)
		for {
			idx := bytes.IndexByte(lw.partial, '\n')
			if idx < 0 {
				break
			}
			line := string(lw.partial[:idx])
			lw.partial = lw.partial[idx+1:]
			lw.cb(lw.hostname, lw.stream, line)
		}
	}

	return n, nil
}

// Flush sends any remaining partial line to the callback.
func (lw *LineWriter) Flush() {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	if lw.cb != nil && len(lw.partial) > 0 {
		lw.cb(lw.hostname, lw.stream, string(lw.partial))
		lw.partial = nil
	}
}

// String returns the captured output, with a truncation notice if applicable.
func (lw *LineWriter) String() string {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	if lw.truncated > 0 {
		return string(lw.buf) + "\n... [truncated]"
	}
	return string(lw.buf)
}
