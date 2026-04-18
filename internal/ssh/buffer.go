package ssh

import (
	"fmt"
	"sync"
)

// DefaultMaxOutputBytes is the default max capture size per stream (1 MiB).
const DefaultMaxOutputBytes = 1 << 20

// LimitedBuffer captures up to maxBytes of data. Excess bytes are discarded
// and a truncation notice is appended when the buffer is read.
type LimitedBuffer struct {
	mu        sync.Mutex
	buf       []byte
	maxBytes  int
	truncated int64
}

// NewLimitedBuffer creates a buffer that captures at most maxBytes.
func NewLimitedBuffer(maxBytes int) *LimitedBuffer {
	return &LimitedBuffer{
		buf:      make([]byte, 0, min(maxBytes, 4096)),
		maxBytes: maxBytes,
	}
}

// Write implements io.Writer.
func (lb *LimitedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	n := len(p)
	remaining := lb.maxBytes - len(lb.buf)
	if remaining <= 0 {
		lb.truncated += int64(n)
		return n, nil
	}
	if len(p) > remaining {
		lb.truncated += int64(len(p) - remaining)
		p = p[:remaining]
	}
	lb.buf = append(lb.buf, p...)
	return n, nil
}

// String returns the captured output, with a truncation notice if applicable.
func (lb *LimitedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.truncated > 0 {
		return string(lb.buf) + fmt.Sprintf("\n... [truncated: %d bytes dropped]", lb.truncated)
	}
	return string(lb.buf)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
