package transfer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	internalssh "gssh/internal/ssh"
)

// Config holds parameters for a batch file transfer.
type Config struct {
	SSHConfig     *ssh.ClientConfig
	Servers       []string
	Port          string
	Source        string // local path (push) or remote path (pull)
	Dest          string // remote path (push) or local output dir (pull)
	Direction     Direction
	MaxWorkers    int
	BastionClient *ssh.Client // optional, for jump host tunneling
}

// ResultCallback is called after each server transfer completes.
type ResultCallback func(Result)

// Run executes file transfers to/from all servers using a worker pool.
func Run(ctx context.Context, cfg Config, cb ResultCallback) ([]Result, time.Duration) {
	// Pre-resolve DNS.
	resolved := internalssh.ResolveServers(cfg.Servers, cfg.Port)
	resolvedMap := internalssh.ResolvedServerMap(resolved)

	start := time.Now()

	work := make(chan string, len(cfg.Servers))
	for _, s := range cfg.Servers {
		work <- s
	}
	close(work)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]Result, 0, len(cfg.Servers))
	)

	workerCount := cfg.MaxWorkers
	if workerCount > len(cfg.Servers) {
		workerCount = len(cfg.Servers)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for srv := range work {
				host, srvPort := internalssh.ParseHostPort(srv, cfg.Port)
				rs := resolvedMap[srv]
				addr := rs.Addr
				if addr == "" {
					addr = net.JoinHostPort(host, srvPort)
				}

				var r Result
				switch cfg.Direction {
				case Upload:
					r = ProcessServerUpload(ctx, cfg.BastionClient, cfg.SSHConfig, addr, host, cfg.Source, cfg.Dest)
				case Download:
					r = ProcessServerDownload(ctx, cfg.BastionClient, cfg.SSHConfig, addr, host, cfg.Source, cfg.Dest)
				}

				mu.Lock()
				results = append(results, r)
				mu.Unlock()

				if cb != nil {
					cb(r)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Log summary.
	var success, failed int
	for _, r := range results {
		if r.IsSuccess() {
			success++
		} else {
			failed++
		}
	}
	slog.Info("transfer complete",
		"direction", dirLabel(cfg.Direction),
		"success", success,
		"failed", failed,
		"total", len(results),
		"duration", elapsed.Round(time.Millisecond),
	)

	return results, elapsed
}

func dirLabel(d Direction) string {
	if d == Upload {
		return "push"
	}
	return "pull"
}

// Report is the top-level JSON structure for transfer results.
type Report struct {
	Results []Result `json:"results"`
	Summary Summary  `json:"summary"`
}

// Summary holds aggregate counts for a batch transfer.
type Summary struct {
	Total      int           `json:"total"`
	Success    int           `json:"success"`
	Failed     int           `json:"failed"`
	TotalBytes int64         `json:"total_bytes"`
	Duration   time.Duration `json:"total_duration_ms"`
}

// ComputeSummary returns a Summary from transfer results.
func ComputeSummary(results []Result, elapsed time.Duration) Summary {
	s := Summary{Total: len(results), Duration: elapsed}
	for _, r := range results {
		if r.IsSuccess() {
			s.Success++
			s.TotalBytes += r.Bytes
		} else {
			s.Failed++
		}
	}
	return s
}

// WriteResults writes transfer results to a timestamped JSON log file
// in the specified logDir. If logDir is empty, the file is written in the
// current directory.
func WriteResults(results []Result, elapsed time.Duration, logDir string) (string, error) {
	if len(results) == 0 {
		return "", nil
	}

	report := Report{
		Results: results,
		Summary: ComputeSummary(results, elapsed),
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
