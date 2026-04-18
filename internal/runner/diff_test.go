package runner

import (
	"testing"

	internalssh "gssh/internal/ssh"
)

func TestComputeDiff(t *testing.T) {
	t.Run("all identical", func(t *testing.T) {
		results := []internalssh.Result{
			{Hostname: "h1", Command: "uptime", Output: "up 5 days\n", ReturnCode: 0},
			{Hostname: "h2", Command: "uptime", Output: "up 5 days\n", ReturnCode: 0},
			{Hostname: "h3", Command: "uptime", Output: "up 5 days\n", ReturnCode: 0},
		}
		reports := ComputeDiff(results)
		if len(reports) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reports))
		}
		if len(reports[0].Groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(reports[0].Groups))
		}
		if len(reports[0].Groups[0].Hosts) != 3 {
			t.Errorf("expected 3 hosts, got %d", len(reports[0].Groups[0].Hosts))
		}
	})

	t.Run("two different outputs", func(t *testing.T) {
		results := []internalssh.Result{
			{Hostname: "h1", Command: "cat /etc/os", Output: "debian\n", ReturnCode: 0},
			{Hostname: "h2", Command: "cat /etc/os", Output: "debian\n", ReturnCode: 0},
			{Hostname: "h3", Command: "cat /etc/os", Output: "centos\n", ReturnCode: 0},
		}
		reports := ComputeDiff(results)
		if len(reports) != 1 {
			t.Fatalf("expected 1 report, got %d", len(reports))
		}
		if len(reports[0].Groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(reports[0].Groups))
		}
		// Majority group (debian) should be first.
		if len(reports[0].Groups[0].Hosts) != 2 {
			t.Errorf("majority group should have 2 hosts, got %d", len(reports[0].Groups[0].Hosts))
		}
		if reports[0].Groups[0].Output != "debian" {
			t.Errorf("majority output = %q, want %q", reports[0].Groups[0].Output, "debian")
		}
	})

	t.Run("multiple commands", func(t *testing.T) {
		results := []internalssh.Result{
			{Hostname: "h1", Command: "cmd1", Output: "a\n", ReturnCode: 0},
			{Hostname: "h2", Command: "cmd1", Output: "a\n", ReturnCode: 0},
			{Hostname: "h1", Command: "cmd2", Output: "x\n", ReturnCode: 0},
			{Hostname: "h2", Command: "cmd2", Output: "y\n", ReturnCode: 0},
		}
		reports := ComputeDiff(results)
		if len(reports) != 2 {
			t.Fatalf("expected 2 reports, got %d", len(reports))
		}
		// cmd1: 1 group (identical)
		if len(reports[0].Groups) != 1 {
			t.Errorf("cmd1: expected 1 group, got %d", len(reports[0].Groups))
		}
		// cmd2: 2 groups (different)
		if len(reports[1].Groups) != 2 {
			t.Errorf("cmd2: expected 2 groups, got %d", len(reports[1].Groups))
		}
	})

	t.Run("errors grouped separately", func(t *testing.T) {
		results := []internalssh.Result{
			{Hostname: "h1", Command: "cmd", Output: "ok\n", ReturnCode: 0},
			{Hostname: "h2", Command: "cmd", Output: "", ReturnCode: 1, Error: "SSH dial: connection refused"},
		}
		reports := ComputeDiff(results)
		if len(reports[0].Groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(reports[0].Groups))
		}
	})

	t.Run("empty results", func(t *testing.T) {
		reports := ComputeDiff(nil)
		if len(reports) != 0 {
			t.Errorf("expected 0 reports, got %d", len(reports))
		}
	})
}
