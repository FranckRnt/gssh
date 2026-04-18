package ssh

import (
	"sync"
	"testing"
)

func TestLineWriter(t *testing.T) {
	t.Run("captures output like LimitedBuffer", func(t *testing.T) {
		lw := NewLineWriter(1024, "host1", "stdout", nil)
		lw.Write([]byte("hello world\n"))
		got := lw.String()
		if got != "hello world\n" {
			t.Errorf("got %q, want %q", got, "hello world\n")
		}
	})

	t.Run("streams complete lines", func(t *testing.T) {
		var mu sync.Mutex
		var lines []string
		cb := func(hostname, stream, line string) {
			mu.Lock()
			defer mu.Unlock()
			lines = append(lines, hostname+":"+stream+":"+line)
		}

		lw := NewLineWriter(1024, "web01", "stdout", cb)
		lw.Write([]byte("line1\nline2\n"))

		mu.Lock()
		defer mu.Unlock()
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2", len(lines))
		}
		if lines[0] != "web01:stdout:line1" {
			t.Errorf("line[0] = %q", lines[0])
		}
		if lines[1] != "web01:stdout:line2" {
			t.Errorf("line[1] = %q", lines[1])
		}
	})

	t.Run("handles partial lines across writes", func(t *testing.T) {
		var mu sync.Mutex
		var lines []string
		cb := func(hostname, stream, line string) {
			mu.Lock()
			defer mu.Unlock()
			lines = append(lines, line)
		}

		lw := NewLineWriter(1024, "h1", "stdout", cb)
		lw.Write([]byte("hel"))
		lw.Write([]byte("lo\nwor"))
		lw.Write([]byte("ld\n"))

		mu.Lock()
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2: %v", len(lines), lines)
		}
		if lines[0] != "hello" {
			t.Errorf("line[0] = %q", lines[0])
		}
		if lines[1] != "world" {
			t.Errorf("line[1] = %q", lines[1])
		}
		mu.Unlock()
	})

	t.Run("flush sends partial line", func(t *testing.T) {
		var lines []string
		cb := func(_, _, line string) {
			lines = append(lines, line)
		}

		lw := NewLineWriter(1024, "h1", "stdout", cb)
		lw.Write([]byte("no-newline"))
		if len(lines) != 0 {
			t.Fatalf("expected 0 lines before flush, got %d", len(lines))
		}

		lw.Flush()
		if len(lines) != 1 || lines[0] != "no-newline" {
			t.Errorf("after flush: lines = %v", lines)
		}
	})

	t.Run("truncates captured output", func(t *testing.T) {
		lw := NewLineWriter(10, "h1", "stdout", nil)
		lw.Write([]byte("12345678901234567890"))
		got := lw.String()
		if len(got) < 10 {
			t.Errorf("expected truncated output, got %q", got)
		}
		// Should contain truncation notice.
		if got == "12345678901234567890" {
			t.Error("expected truncation, got full output")
		}
	})

	t.Run("streams even beyond capture limit", func(t *testing.T) {
		var lines []string
		cb := func(_, _, line string) {
			lines = append(lines, line)
		}

		lw := NewLineWriter(5, "h1", "stdout", cb)
		lw.Write([]byte("abc\ndefgh\n"))

		// Both lines should be streamed even though capture limit is 5.
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2", len(lines))
		}
		if lines[0] != "abc" {
			t.Errorf("line[0] = %q", lines[0])
		}
		if lines[1] != "defgh" {
			t.Errorf("line[1] = %q", lines[1])
		}
	})
}
