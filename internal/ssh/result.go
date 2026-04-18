package ssh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Result holds the outcome of a command execution on a single host.
type Result struct {
	Hostname    string        `json:"hostname"`
	Command     string        `json:"command"`
	Output      string        `json:"output"`
	Stderr      string        `json:"stderr,omitempty"`
	ReturnCode  int           `json:"return_code"`
	DateCommand string        `json:"date_command"`
	Duration    time.Duration `json:"duration_ms"`
	Error       string        `json:"error,omitempty"`
}

// IsSuccess returns true if the command completed without error.
func (r Result) IsSuccess() bool {
	return r.ReturnCode == 0 && r.Error == ""
}

// Summary holds aggregate counts for a batch execution.
type Summary struct {
	Total    int           `json:"total"`
	Success  int           `json:"success"`
	Failed   int           `json:"failed"`
	Duration time.Duration `json:"total_duration_ms"`
}

// Report is the top-level JSON structure written to the log file.
type Report struct {
	Results []Result `json:"results"`
	Summary Summary  `json:"summary"`
}

// ComputeSummary returns a Summary from a slice of results.
func ComputeSummary(results []Result, totalDuration time.Duration) Summary {
	s := Summary{
		Total:    len(results),
		Duration: totalDuration,
	}
	for _, r := range results {
		if r.IsSuccess() {
			s.Success++
		} else {
			s.Failed++
		}
	}
	return s
}

// WriteResults writes the results and summary to a timestamped JSON log file
// in the specified logDir. If logDir is empty, the file is written in the
// current directory.
func WriteResults(results []Result, totalDuration time.Duration, logDir string) (string, error) {
	if len(results) == 0 {
		return "", nil
	}

	report := Report{
		Results: results,
		Summary: ComputeSummary(results, totalDuration),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}

	baseName := fmt.Sprintf("gssh-%s.log", time.Now().Format("2006-01-02_15-04-05"))

	filePath := baseName
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0750); err != nil {
			return "", fmt.Errorf("create log dir %s: %w", logDir, err)
		}
		filePath = filepath.Join(logDir, baseName)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("write results file: %w", err)
	}

	return filePath, nil
}
