package runner

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	internalssh "gssh/internal/ssh"
)

func TestRunShutdownBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := Config{
		SSHConfig: &ssh.ClientConfig{
			Timeout: 1 * time.Second,
		},
		Servers:    []string{"host1", "host2", "host3"},
		Port:       "22",
		Commands:   []string{"echo test"},
		Timeout:    5 * time.Second,
		MaxWorkers: 2,
	}

	results, elapsed := Run(ctx, cfg, nil)

	if elapsed == 0 {
		t.Error("expected non-zero elapsed time")
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.Error != "shutdown before execution" {
			t.Errorf("result for %s: error = %q, want 'shutdown before execution'", r.Hostname, r.Error)
		}
	}
}

func TestRunWorkerCountCapping(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		SSHConfig: &ssh.ClientConfig{
			Timeout: 1 * time.Second,
		},
		Servers:    []string{"a", "b"},
		Port:       "22",
		Commands:   []string{"test"},
		Timeout:    1 * time.Second,
		MaxWorkers: 1000,
	}

	results, _ := Run(ctx, cfg, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	summary := internalssh.ComputeSummary(results, 0)
	if summary.Failed != 2 {
		t.Errorf("expected all failed, got %d failed", summary.Failed)
	}
}

func TestRunVerboseCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := Config{
		SSHConfig: &ssh.ClientConfig{
			Timeout: 1 * time.Second,
		},
		Servers:    []string{"host1", "host2"},
		Port:       "22",
		Commands:   []string{"test"},
		Timeout:    1 * time.Second,
		MaxWorkers: 2,
		Verbose:    true,
	}

	var callbackCount int
	cb := func(results []internalssh.Result) {
		callbackCount++
	}

	results, _ := Run(ctx, cfg, cb)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if callbackCount != 2 {
		t.Errorf("callback called %d times, want 2", callbackCount)
	}
}
