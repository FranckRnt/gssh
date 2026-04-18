package runner

import (
	"bytes"
	"strings"
	"testing"
	"time"

	internalssh "gssh/internal/ssh"
	"gssh/internal/transfer"
)

func TestPrintCompactServer_AllSuccess(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf, false)

	results := []internalssh.Result{
		{Hostname: "web01", Command: "hostname", ReturnCode: 0, Duration: 30 * time.Millisecond},
		{Hostname: "web01", Command: "uptime", ReturnCode: 0, Duration: 40 * time.Millisecond},
	}
	f.PrintCompactServer(results)

	out := buf.String()
	if !strings.Contains(out, "web01") {
		t.Errorf("expected hostname in output, got %q", out)
	}
	if !strings.Contains(out, "hostname OK") {
		t.Errorf("expected 'hostname OK' in output, got %q", out)
	}
	if !strings.Contains(out, "uptime OK") {
		t.Errorf("expected 'uptime OK' in output, got %q", out)
	}
	// Should be a single line (no failure detail).
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d: %q", len(lines), out)
	}
}

func TestPrintCompactServer_WithFailure(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf, false)

	results := []internalssh.Result{
		{Hostname: "web01", Command: "hostname", ReturnCode: 0, Duration: 30 * time.Millisecond},
		{Hostname: "web01", Command: "yum check-update", ReturnCode: 100, Duration: time.Second, Error: "exit status 100"},
	}
	f.PrintCompactServer(results)

	out := buf.String()
	if !strings.Contains(out, "hostname OK") {
		t.Errorf("expected 'hostname OK', got %q", out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected 'FAIL', got %q", out)
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("expected error detail for failure, got %q", out)
	}
}

func TestPrintCompactServer_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf, false)
	f.PrintCompactServer(nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty results, got %q", buf.String())
	}
}

func TestPrintTransferResult_Success(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf, false)

	r := transfer.Result{
		Hostname:  "web01",
		Direction: "push",
		Source:    "/tmp/file.txt",
		Dest:      "/opt/file.txt",
		Bytes:     2048,
		Duration:  150 * time.Millisecond,
	}
	f.PrintTransferResult(r)

	out := buf.String()
	if !strings.Contains(out, "web01") {
		t.Errorf("expected hostname, got %q", out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got %q", out)
	}
	if !strings.Contains(out, "KiB") {
		t.Errorf("expected KiB size, got %q", out)
	}
}

func TestPrintTransferResult_Failure(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf, false)

	r := transfer.Result{
		Hostname:  "web02",
		Direction: "pull",
		Error:     "connection refused",
		Duration:  50 * time.Millisecond,
	}
	f.PrintTransferResult(r)

	out := buf.String()
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected FAIL, got %q", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error message, got %q", out)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
