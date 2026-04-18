package runner

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	internalssh "gssh/internal/ssh"
	"gssh/internal/transfer"
)

// color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// Formatter formats results for display.
type Formatter struct {
	w     io.Writer
	color bool
}

// NewFormatter creates a formatter. color enables ANSI colors.
func NewFormatter(w io.Writer, color bool) *Formatter {
	return &Formatter{w: w, color: color}
}

// IsTerminal returns true if the given file is a terminal.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// PrintCompactServer prints a single compact line per server showing the status
// of each command. Failed commands get their output/error printed below.
func (f *Formatter) PrintCompactServer(results []internalssh.Result) {
	if len(results) == 0 {
		return
	}

	host := results[0].Hostname

	// Build compact status line: host: cmd1 OK | cmd2 OK | cmd3 FAIL (exit=100)
	var parts []string
	var failures []internalssh.Result
	for _, r := range results {
		// Use a short command label (first 30 chars).
		label := r.Command
		if len(label) > 30 {
			label = label[:27] + "..."
		}

		if r.IsSuccess() {
			parts = append(parts, fmt.Sprintf("%s %s", label, f.green("OK")))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s (exit=%d)", label, f.red("FAIL"), r.ReturnCode))
			failures = append(failures, r)
		}
	}

	fmt.Fprintf(f.w, "%s  %s\n", f.bold(f.cyan(host)), strings.Join(parts, " | "))

	// Print detail for failures only.
	for _, r := range failures {
		if r.Error != "" {
			fmt.Fprintf(f.w, "    %s [%s] %s\n", f.red("error:"), r.Command, r.Error)
		}
		if r.Stderr != "" {
			for _, line := range strings.Split(strings.TrimRight(r.Stderr, "\n"), "\n") {
				fmt.Fprintf(f.w, "    %s %s\n", f.yellow("stderr:"), line)
			}
		}
	}
}

// PrintResult prints a single result in text format (verbose mode).
func (f *Formatter) PrintResult(r internalssh.Result) {
	status := f.green("OK")
	if !r.IsSuccess() {
		status = f.red("FAIL")
	}

	fmt.Fprintf(f.w, "\n%s %s %s (exit=%d, %s)\n",
		f.bold(">>>"),
		f.cyan(r.Hostname),
		status,
		r.ReturnCode,
		r.Duration.Round(1e6), // round to milliseconds
	)

	if r.Error != "" {
		fmt.Fprintf(f.w, "    %s %s\n", f.red("error:"), r.Error)
	}

	if r.Output != "" {
		for _, line := range strings.Split(strings.TrimRight(r.Output, "\n"), "\n") {
			fmt.Fprintf(f.w, "    %s\n", line)
		}
	}

	if r.Stderr != "" {
		for _, line := range strings.Split(strings.TrimRight(r.Stderr, "\n"), "\n") {
			fmt.Fprintf(f.w, "    %s %s\n", f.yellow("stderr:"), line)
		}
	}
}

// PrintSummary prints the final summary in text format.
func (f *Formatter) PrintSummary(s internalssh.Summary) {
	fmt.Fprintf(f.w, "\n%s\n", f.bold("── Summary ──────────────────────────"))
	fmt.Fprintf(f.w, "  Total:    %d\n", s.Total)
	fmt.Fprintf(f.w, "  Success:  %s\n", f.green(fmt.Sprintf("%d", s.Success)))
	fmt.Fprintf(f.w, "  Failed:   %s\n", f.red(fmt.Sprintf("%d", s.Failed)))
	fmt.Fprintf(f.w, "  Duration: %s\n", s.Duration.Round(1e6))
}

// PrintDryRun prints the list of servers that would be targeted.
func (f *Formatter) PrintDryRun(servers []string, port string) {
	fmt.Fprintf(f.w, "%s\n\n", f.bold("── Dry run ── (no commands will be executed)"))
	for i, srv := range servers {
		host, srvPort := internalssh.ParseHostPort(srv, port)
		fmt.Fprintf(f.w, "  %3d. %s:%s\n", i+1, host, srvPort)
	}
	fmt.Fprintf(f.w, "\n  %s %d server(s)\n", f.bold("Total:"), len(servers))
}

func (f *Formatter) bold(s string) string {
	if !f.color {
		return s
	}
	return colorBold + s + colorReset
}

func (f *Formatter) red(s string) string {
	if !f.color {
		return s
	}
	return colorRed + s + colorReset
}

func (f *Formatter) green(s string) string {
	if !f.color {
		return s
	}
	return colorGreen + s + colorReset
}

func (f *Formatter) yellow(s string) string {
	if !f.color {
		return s
	}
	return colorYellow + s + colorReset
}

func (f *Formatter) cyan(s string) string {
	if !f.color {
		return s
	}
	return colorCyan + s + colorReset
}

// PrintStreamLine prints a single streamed line with hostname prefix.
// stream is "stdout" or "stderr".
func (f *Formatter) PrintStreamLine(hostname, stream, line string) {
	prefix := f.bold(f.cyan(hostname))
	if stream == "stderr" {
		fmt.Fprintf(f.w, "%s %s %s\n", prefix, f.yellow("err|"), line)
	} else {
		fmt.Fprintf(f.w, "%s %s\n", prefix, line)
	}
}

// PrintTransferResult prints a compact line for a file transfer result.
func (f *Formatter) PrintTransferResult(r transfer.Result) {
	status := f.green("OK")
	detail := formatBytes(r.Bytes)
	if !r.IsSuccess() {
		status = f.red("FAIL")
		detail = r.Error
	}

	fmt.Fprintf(f.w, "%s  %s %s  %s  %s\n",
		f.bold(f.cyan(r.Hostname)),
		r.Direction,
		status,
		detail,
		r.Duration.Round(time.Millisecond),
	)
}

// PrintTransferSummary prints the final summary for a batch transfer.
func (f *Formatter) PrintTransferSummary(s transfer.Summary) {
	fmt.Fprintf(f.w, "\n%s\n", f.bold("── Transfer Summary ─────────────────"))
	fmt.Fprintf(f.w, "  Total:      %d\n", s.Total)
	fmt.Fprintf(f.w, "  Success:    %s\n", f.green(fmt.Sprintf("%d", s.Success)))
	fmt.Fprintf(f.w, "  Failed:     %s\n", f.red(fmt.Sprintf("%d", s.Failed)))
	fmt.Fprintf(f.w, "  Bytes:      %s\n", formatBytes(s.TotalBytes))
	fmt.Fprintf(f.w, "  Duration:   %s\n", s.Duration.Round(time.Millisecond))
}

// formatBytes formats a byte count in human-readable form.
func formatBytes(b int64) string {
	const (
		kB = 1024
		mB = 1024 * kB
		gB = 1024 * mB
	)
	switch {
	case b >= gB:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gB))
	case b >= mB:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mB))
	case b >= kB:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
