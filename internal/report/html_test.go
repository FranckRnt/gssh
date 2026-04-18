package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	internalssh "gssh/internal/ssh"
)

func TestWriteHTML(t *testing.T) {
	t.Run("generates valid HTML file", func(t *testing.T) {
		dir := t.TempDir()
		results := []internalssh.Result{
			{Hostname: "web01", Command: "uptime", Output: "up 5 days\n", ReturnCode: 0, Duration: 150 * time.Millisecond},
			{Hostname: "web02", Command: "uptime", Output: "up 10 days\n", ReturnCode: 0, Duration: 200 * time.Millisecond},
			{Hostname: "web03", Command: "uptime", Output: "", ReturnCode: 1, Error: "connection refused", Duration: 50 * time.Millisecond},
		}
		summary := internalssh.ComputeSummary(results, time.Second)

		path, err := WriteHTML(results, summary, time.Second, dir)
		if err != nil {
			t.Fatalf("WriteHTML: %v", err)
		}
		if path == "" {
			t.Fatal("expected non-empty path")
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read HTML: %v", err)
		}

		html := string(data)
		if !strings.Contains(html, "<!DOCTYPE html>") {
			t.Error("missing DOCTYPE")
		}
		if !strings.Contains(html, "web01") {
			t.Error("missing hostname web01")
		}
		if !strings.Contains(html, "web03") {
			t.Error("missing hostname web03")
		}
		if !strings.Contains(html, "connection refused") {
			t.Error("missing error text")
		}
		if !strings.Contains(html, "uptime") {
			t.Error("missing command")
		}
	})

	t.Run("creates logDir if missing", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "deep", "nested")
		results := []internalssh.Result{
			{Hostname: "h1", Command: "echo", Output: "ok\n", ReturnCode: 0},
		}
		summary := internalssh.ComputeSummary(results, time.Second)

		path, err := WriteHTML(results, summary, time.Second, dir)
		if err != nil {
			t.Fatalf("WriteHTML: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file not created: %v", err)
		}
	})

	t.Run("empty logDir writes to current dir", func(t *testing.T) {
		// Use a temp dir as working directory.
		dir := t.TempDir()
		origDir, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(origDir)

		results := []internalssh.Result{
			{Hostname: "h1", Command: "echo", Output: "ok\n", ReturnCode: 0},
		}
		summary := internalssh.ComputeSummary(results, time.Second)

		path, err := WriteHTML(results, summary, time.Second, "")
		if err != nil {
			t.Fatalf("WriteHTML: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("file not in cwd: %v", err)
		}
	})
}
