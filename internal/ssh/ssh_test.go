package ssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		defaultPort string
		wantHost    string
		wantPort    string
	}{
		{"bare host", "example.com", "22", "example.com", "22"},
		{"host with port", "example.com:2222", "22", "example.com", "2222"},
		{"ip bare", "10.0.0.1", "22", "10.0.0.1", "22"},
		{"ip with port", "10.0.0.1:8022", "22", "10.0.0.1", "8022"},
		{"different default", "myhost", "443", "myhost", "443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := ParseHostPort(tt.server, tt.defaultPort)
			if host != tt.wantHost || port != tt.wantPort {
				t.Errorf("ParseHostPort(%q, %q) = (%q, %q), want (%q, %q)",
					tt.server, tt.defaultPort, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestLoadServers(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		content := "server1.example.com\n# comment\n\nserver2.example.com:2222\n  server3.example.com  \n"
		path := writeTempFile(t, content, 0644)

		servers, err := LoadServers(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []Server{
			{Host: "server1.example.com"},
			{Host: "server2.example.com:2222"},
			{Host: "server3.example.com"},
		}
		if len(servers) != len(want) {
			t.Fatalf("got %d servers, want %d", len(servers), len(want))
		}
		for i, s := range servers {
			if s.Host != want[i].Host {
				t.Errorf("server[%d].Host = %q, want %q", i, s.Host, want[i].Host)
			}
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTempFile(t, "# only comments\n\n", 0644)

		servers, err := LoadServers(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(servers) != 0 {
			t.Errorf("got %d servers, want 0", len(servers))
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := LoadServers("/nonexistent/file")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})

	t.Run("world-writable file rejected", func(t *testing.T) {
		path := writeTempFile(t, "server1\n", 0644)
		if err := os.Chmod(path, 0666); err != nil {
			t.Fatalf("chmod: %v", err)
		}

		_, err := LoadServers(path)
		if err == nil {
			t.Fatal("expected error for world-writable file")
		}
	})
}

func TestResultIsSuccess(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{"success", Result{ReturnCode: 0}, true},
		{"non-zero exit", Result{ReturnCode: 1}, false},
		{"error string", Result{ReturnCode: 0, Error: "fail"}, false},
		{"both", Result{ReturnCode: 1, Error: "fail"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeSummary(t *testing.T) {
	results := []Result{
		{ReturnCode: 0},
		{ReturnCode: 1, Error: "fail"},
		{ReturnCode: 0},
		{ReturnCode: 0, Error: "partial"},
	}

	s := ComputeSummary(results, 5*time.Second)
	if s.Total != 4 {
		t.Errorf("total = %d, want 4", s.Total)
	}
	if s.Success != 2 {
		t.Errorf("success = %d, want 2", s.Success)
	}
	if s.Failed != 2 {
		t.Errorf("failed = %d, want 2", s.Failed)
	}
	if s.Duration != 5*time.Second {
		t.Errorf("duration = %v, want 5s", s.Duration)
	}
}

func TestLimitedBuffer(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		buf := NewLimitedBuffer(100)
		buf.Write([]byte("hello"))
		if buf.String() != "hello" {
			t.Errorf("got %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		buf := NewLimitedBuffer(10)
		n, err := buf.Write([]byte("0123456789extra"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 15 {
			t.Errorf("n = %d, want 15", n)
		}
		s := buf.String()
		if len(s) < 10 {
			t.Errorf("expected at least 10 bytes captured")
		}
		if !strings.Contains(s, "truncated") {
			t.Errorf("expected truncation notice, got %q", s)
		}
	})

	t.Run("multiple writes", func(t *testing.T) {
		buf := NewLimitedBuffer(10)
		buf.Write([]byte("12345"))
		buf.Write([]byte("67890"))
		buf.Write([]byte("overflow"))
		s := buf.String()
		if !strings.Contains(s, "1234567890") {
			t.Errorf("expected full 10 bytes, got %q", s)
		}
		if !strings.Contains(s, "truncated") {
			t.Errorf("expected truncation notice")
		}
	})
}

func TestWriteResults(t *testing.T) {
	t.Run("empty results", func(t *testing.T) {
		name, err := WriteResults(nil, 0, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "" {
			t.Errorf("expected empty filename, got %q", name)
		}
	})

	t.Run("writes file to logDir", func(t *testing.T) {
		dir := t.TempDir()
		logDir := filepath.Join(dir, "logs")

		results := []Result{{Hostname: "h1", ReturnCode: 0, Command: "echo"}}
		name, err := WriteResults(results, time.Second, logDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name == "" {
			t.Fatal("expected non-empty filename")
		}

		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("expected non-empty file")
		}
	})

	t.Run("creates logDir if missing", func(t *testing.T) {
		dir := t.TempDir()
		logDir := filepath.Join(dir, "deep", "nested", "logs")

		results := []Result{{Hostname: "h1", ReturnCode: 0, Command: "echo"}}
		name, err := WriteResults(results, time.Second, logDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name == "" {
			t.Fatal("expected non-empty filename")
		}

		// Verify the directory was created.
		info, err := os.Stat(logDir)
		if err != nil {
			t.Fatalf("logDir not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("logDir is not a directory")
		}
	})
}

func writeTempFile(t *testing.T, content string, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.txt")
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantHost string
		wantTags []string
	}{
		{"host only", "web01.example.com", "web01.example.com", nil},
		{"host with port", "db01:5432", "db01:5432", nil},
		{"host with tags", "web01 #web #prod", "web01", []string{"web", "prod"}},
		{"host with port and tags", "web01:2222 #web #staging", "web01:2222", []string{"web", "staging"}},
		{"empty hash ignored", "web01 #", "web01", nil},
		{"mixed spacing", "web01   #web   #prod", "web01", []string{"web", "prod"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := parseLine(tt.line)
			if srv.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", srv.Host, tt.wantHost)
			}
			if len(srv.Tags) != len(tt.wantTags) {
				t.Fatalf("Tags = %v, want %v", srv.Tags, tt.wantTags)
			}
			for i, tag := range srv.Tags {
				if tag != tt.wantTags[i] {
					t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
				}
			}
		})
	}
}

func TestFilterByTags(t *testing.T) {
	servers := []Server{
		{Host: "web01", Tags: []string{"web", "prod"}},
		{Host: "web02", Tags: []string{"web", "staging"}},
		{Host: "db01", Tags: []string{"db", "prod"}},
		{Host: "db02", Tags: []string{"db", "staging"}},
		{Host: "plain", Tags: nil},
	}

	tests := []struct {
		name      string
		tags      []string
		wantHosts []string
	}{
		{"no filter", nil, []string{"web01", "web02", "db01", "db02", "plain"}},
		{"single tag web", []string{"web"}, []string{"web01", "web02"}},
		{"single tag prod", []string{"prod"}, []string{"web01", "db01"}},
		{"intersection web+prod", []string{"web", "prod"}, []string{"web01"}},
		{"no match", []string{"nonexistent"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterByTags(servers, tt.tags)
			hosts := Hosts(result)
			if len(hosts) != len(tt.wantHosts) {
				t.Fatalf("got %v, want %v", hosts, tt.wantHosts)
			}
			for i, h := range hosts {
				if h != tt.wantHosts[i] {
					t.Errorf("hosts[%d] = %q, want %q", i, h, tt.wantHosts[i])
				}
			}
		})
	}
}

func TestHosts(t *testing.T) {
	servers := []Server{
		{Host: "a"},
		{Host: "b:2222"},
		{Host: "c"},
	}
	got := Hosts(servers)
	want := []string{"a", "b:2222", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Hosts()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadServersWithTags(t *testing.T) {
	content := "web01 #web #prod\ndb01:5432 #db #prod\n# comment\nplain-host\n"
	path := writeTempFile(t, content, 0644)

	servers, err := LoadServers(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(servers) != 3 {
		t.Fatalf("got %d servers, want 3", len(servers))
	}

	if servers[0].Host != "web01" || len(servers[0].Tags) != 2 {
		t.Errorf("server[0] = %+v, want Host=web01 Tags=[web prod]", servers[0])
	}
	if servers[1].Host != "db01:5432" || len(servers[1].Tags) != 2 {
		t.Errorf("server[1] = %+v, want Host=db01:5432 Tags=[db prod]", servers[1])
	}
	if servers[2].Host != "plain-host" || len(servers[2].Tags) != 0 {
		t.Errorf("server[2] = %+v, want Host=plain-host Tags=[]", servers[2])
	}
}

func TestParseBastionSpec(t *testing.T) {
	tests := []struct {
		name     string
		spec     string
		wantUser string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{"full spec", "admin@bastion.example.com:2222", "admin", "bastion.example.com", "2222", false},
		{"no port", "admin@bastion.example.com", "admin", "bastion.example.com", "22", false},
		{"ip address", "root@10.0.0.1:22", "root", "10.0.0.1", "22", false},
		{"ip no port", "root@10.0.0.1", "root", "10.0.0.1", "22", false},
		{"empty spec", "", "", "", "", true},
		{"no user", "bastion.example.com", "", "", "", true},
		{"no user with port", "bastion.example.com:22", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, port, err := ParseBastionSpec(tt.spec, "22")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user != tt.wantUser {
				t.Errorf("user = %q, want %q", user, tt.wantUser)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("port = %q, want %q", port, tt.wantPort)
			}
		})
	}
}
