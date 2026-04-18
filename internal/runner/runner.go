package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	internalssh "gssh/internal/ssh"
	tmpl "gssh/internal/template"
)

// Config holds the parameters for a batch SSH execution.
type Config struct {
	SSHConfig     *ssh.ClientConfig
	Servers       []string
	Port          string
	Commands      []string
	Timeout       time.Duration
	MaxWorkers    int
	Retries       int
	Verbose       bool
	BastionClient *ssh.Client                // optional, for jump host tunneling
	SudoPassword  string                     // optional, for sudo mode
	ServerTags    map[string][]string        // optional, host -> tags for template expansion
	StreamCB      internalssh.StreamCallback // optional, for real-time line streaming
}

// ResultCallback is called after each server completes, for real-time output.
// It receives all results for that server (one per command).
type ResultCallback func([]internalssh.Result)

// Run executes the command on all servers using a fixed-size worker pool.
// It respects context cancellation for graceful shutdown.
// If cb is non-nil, it is called for each result as it completes.
func Run(ctx context.Context, cfg Config, cb ResultCallback) ([]internalssh.Result, time.Duration) {
	// Pre-resolve DNS for all servers.
	resolved := internalssh.ResolveServers(cfg.Servers, cfg.Port)
	resolvedMap := internalssh.ResolvedServerMap(resolved)

	bar := newProgress(os.Stderr, len(cfg.Servers), cfg.Verbose)

	results := runBatch(ctx, cfg, cfg.Servers, resolvedMap, bar, cb)

	// Retry failed servers.
	for attempt := 1; attempt <= cfg.Retries; attempt++ {
		// Identify servers with any failed command.
		failedHostSet := make(map[string]bool)
		for _, r := range results {
			if !r.IsSuccess() && r.Error != "shutdown before execution" && r.Error != "shutdown requested" {
				failedHostSet[r.Hostname] = true
			}
		}

		if len(failedHostSet) == 0 {
			break
		}

		// Map hostnames back to original server strings.
		var failedServers []string
		for _, srv := range cfg.Servers {
			host, _ := internalssh.ParseHostPort(srv, cfg.Port)
			if failedHostSet[host] {
				failedServers = append(failedServers, srv)
			}
		}

		slog.Info("retrying failed servers",
			"attempt", attempt,
			"count", len(failedServers),
		)

		// Remove old results for failed servers.
		var kept []internalssh.Result
		for _, r := range results {
			if !failedHostSet[r.Hostname] {
				kept = append(kept, r)
			}
		}

		// Update progress bar total for retry batch.
		retryBar := newProgress(os.Stderr, len(failedServers), cfg.Verbose)
		retryResults := runBatch(ctx, cfg, failedServers, resolvedMap, retryBar, cb)
		results = append(kept, retryResults...)
	}

	elapsed := bar.elapsed()
	bar.finish()

	summary := internalssh.ComputeSummary(results, elapsed)
	slog.Info("execution complete",
		"success", summary.Success,
		"failed", summary.Failed,
		"total", summary.Total,
		"duration", elapsed.Round(time.Millisecond),
	)

	return results, elapsed
}

// runBatch runs commands on a list of servers and returns results.
func runBatch(ctx context.Context, cfg Config, servers []string, resolvedMap map[string]internalssh.ResolvedServer, bar *progress, cb ResultCallback) []internalssh.Result {
	work := make(chan string, len(servers))
	for _, s := range servers {
		work <- s
	}
	close(work)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make([]internalssh.Result, 0, len(servers))
	)

	workerCount := cfg.MaxWorkers
	if workerCount > len(servers) {
		workerCount = len(servers)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for srv := range work {
				var serverResults []internalssh.Result

				if ctx.Err() != nil {
					host, _ := internalssh.ParseHostPort(srv, cfg.Port)
					for _, cmd := range cfg.Commands {
						serverResults = append(serverResults, internalssh.Result{
							Hostname:    host,
							Command:     cmd,
							DateCommand: time.Now().Format(time.RFC3339),
							ReturnCode:  1,
							Error:       "shutdown before execution",
						})
					}
				} else {
					rs := resolvedMap[srv]
					host, srvPort := internalssh.ParseHostPort(srv, cfg.Port)

					// Expand command templates if needed.
					commands := cfg.Commands
					if tmpl.NeedsExpansion(commands) {
						var tags []string
						if cfg.ServerTags != nil {
							tags = cfg.ServerTags[srv]
						}
						data := tmpl.ServerData{
							Hostname: host,
							Host:     srv,
							IP:       host,
							Port:     srvPort,
							Tags:     tags,
							TagsCSV:  strings.Join(tags, ","),
						}
						expanded, err := tmpl.ExpandCommands(commands, data)
						if err != nil {
							slog.Warn("template expansion failed", "server", host, "error", err)
							for _, cmd := range commands {
								serverResults = append(serverResults, internalssh.Result{
									Hostname:    host,
									Command:     cmd,
									DateCommand: time.Now().Format(time.RFC3339),
									ReturnCode:  1,
									Error:       fmt.Sprintf("template error: %v", err),
								})
							}
							goto done
						}
						commands = expanded
					}

					serverResults = internalssh.ProcessServerViaBastion(ctx, cfg.BastionClient, cfg.SSHConfig, srv, cfg.Port, commands, cfg.Timeout, rs.Addr, cfg.SudoPassword, cfg.StreamCB)
				}
			done:

				mu.Lock()
				results = append(results, serverResults...)
				mu.Unlock()

				// Count this server as one unit of progress.
				allOK := true
				for _, r := range serverResults {
					if !r.IsSuccess() {
						allOK = false
						break
					}
				}
				bar.complete(allOK)

				if cb != nil {
					bar.ClearAndRender(func() {
						cb(serverResults)
					})
				}
			}
		}()
	}

	wg.Wait()
	return results
}
