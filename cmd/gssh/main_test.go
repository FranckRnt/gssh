package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Use a temp HOME so tests don't load the real ~/.gssh/config.yaml.
	tmp, _ := os.MkdirTemp("", "gssh-test-home")
	defer os.RemoveAll(tmp)
	os.Setenv("HOME", tmp)
	os.Exit(m.Run())
}

func TestParseFlags_MissingRequired(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no flags", []string{}, "missing required flags"},
		{"missing user", []string{"-l", "servers.txt", "-c", "uptime"}, "missing required flags"},
		{"missing command", []string{"-l", "servers.txt", "-u", "root"}, "missing required flags"},
		{"missing servers", []string{"-u", "root", "-c", "uptime"}, "missing required flags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			_, err := parseFlags(tt.args, &stderr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func TestParseFlags_InsecureWithConfirm(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-insecure", "-y"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.insecure {
		t.Error("expected insecure=true")
	}
	if !cfg.confirm {
		t.Error("expected confirm=true")
	}
}

func TestParseFlags_Defaults(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "admin", "-c", "hostname"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.port != "22" {
		t.Errorf("port = %q, want '22'", cfg.port)
	}
	if cfg.maxWorkers != 100 {
		t.Errorf("maxWorkers = %d, want 100", cfg.maxWorkers)
	}
	if cfg.timeout.Seconds() != 30 {
		t.Errorf("timeout = %v, want 30s", cfg.timeout)
	}
	if cfg.insecure {
		t.Error("expected insecure=false by default")
	}
	if cfg.retries != 0 {
		t.Errorf("retries = %d, want 0", cfg.retries)
	}
	if cfg.verbose {
		t.Error("expected verbose=false by default")
	}
	if cfg.dryRun {
		t.Error("expected dryRun=false by default")
	}
	if cfg.outputFormat != "text" {
		t.Errorf("outputFormat = %q, want 'text'", cfg.outputFormat)
	}
}

func TestParseFlags_CustomValues(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{
		"-l", "hosts.txt",
		"-u", "deploy",
		"-c", "df -h",
		"-k", "/tmp/mykey",
		"-p", "2222",
		"-t", "10s",
		"-w", "50",
		"-r", "3",
		"-v",
		"-o", "json",
		"-known-hosts", "/tmp/known",
	}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.serversFile != "hosts.txt" {
		t.Errorf("serversFile = %q, want 'hosts.txt'", cfg.serversFile)
	}
	if cfg.user != "deploy" {
		t.Errorf("user = %q, want 'deploy'", cfg.user)
	}
	if len(cfg.commands) != 1 || cfg.commands[0] != "df -h" {
		t.Errorf("commands = %v, want [\"df -h\"]", cfg.commands)
	}
	if cfg.keyPath != "/tmp/mykey" {
		t.Errorf("keyPath = %q, want '/tmp/mykey'", cfg.keyPath)
	}
	if cfg.port != "2222" {
		t.Errorf("port = %q, want '2222'", cfg.port)
	}
	if cfg.maxWorkers != 50 {
		t.Errorf("maxWorkers = %d, want 50", cfg.maxWorkers)
	}
	if cfg.retries != 3 {
		t.Errorf("retries = %d, want 3", cfg.retries)
	}
	if !cfg.verbose {
		t.Error("expected verbose=true")
	}
	if cfg.outputFormat != "json" {
		t.Errorf("outputFormat = %q, want 'json'", cfg.outputFormat)
	}
	if cfg.knownHostsFile != "/tmp/known" {
		t.Errorf("knownHostsFile = %q, want '/tmp/known'", cfg.knownHostsFile)
	}
}

func TestParseFlags_InvalidOutputFormat(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-o", "yaml"}

	_, err := parseFlags(args, &stderr)
	if err == nil {
		t.Fatal("expected error for invalid output format")
	}
	if !strings.Contains(err.Error(), "invalid output format") {
		t.Errorf("error = %q, want containing 'invalid output format'", err.Error())
	}
}

func TestParseFlags_DryRun(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-n"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.dryRun {
		t.Error("expected dryRun=true")
	}
}

func TestParseFlags_InvalidFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-bogus"}

	_, err := parseFlags(args, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseFlags_NegativeRetries(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-r", "-1"}

	_, err := parseFlags(args, &stderr)
	if err == nil {
		t.Fatal("expected error for negative retries")
	}
	if !strings.Contains(err.Error(), "retries must be >= 0") {
		t.Errorf("error = %q, want containing 'retries must be >= 0'", err.Error())
	}
}

func TestParseFlags_MultipleCommands(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-c", "df -h", "-c", "whoami"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(cfg.commands))
	}
	want := []string{"uptime", "df -h", "whoami"}
	for i, cmd := range cfg.commands {
		if cmd != want[i] {
			t.Errorf("command[%d] = %q, want %q", i, cmd, want[i])
		}
	}
}

func TestParseFlags_ScriptFile(t *testing.T) {
	dir := t.TempDir()
	scriptPath := dir + "/cmds.txt"
	os.WriteFile(scriptPath, []byte("uptime\n# comment\n\ndf -h\nwhoami\n"), 0644)

	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-f", scriptPath}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(cfg.commands))
	}
}

func TestParseFlags_ScriptFileAndCommands_MutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	scriptPath := dir + "/cmds.txt"
	os.WriteFile(scriptPath, []byte("df -h\n"), 0644)

	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-f", scriptPath}

	_, err := parseFlags(args, &stderr)
	if err == nil {
		t.Fatal("expected error when both -c and -f are provided")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want 'mutually exclusive' message", err)
	}
}

// --- Transfer flags tests ---

func TestParseTransferFlags_Push(t *testing.T) {
	// Create a temp file to use as source.
	dir := t.TempDir()
	srcPath := dir + "/test.txt"
	os.WriteFile(srcPath, []byte("hello"), 0644)

	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", srcPath, "-d", "/tmp/test.txt"}

	cfg, err := parseTransferFlags(args, &stderr, "push")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.serversFile != "servers.txt" {
		t.Errorf("serversFile = %q, want 'servers.txt'", cfg.serversFile)
	}
	if cfg.user != "root" {
		t.Errorf("user = %q, want 'root'", cfg.user)
	}
	if cfg.source != srcPath {
		t.Errorf("source = %q, want %q", cfg.source, srcPath)
	}
	if cfg.dest != "/tmp/test.txt" {
		t.Errorf("dest = %q, want '/tmp/test.txt'", cfg.dest)
	}
}

func TestParseTransferFlags_Pull(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", "/var/log/syslog", "-d", "./output/"}

	cfg, err := parseTransferFlags(args, &stderr, "pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.source != "/var/log/syslog" {
		t.Errorf("source = %q, want '/var/log/syslog'", cfg.source)
	}
	if cfg.dest != "./output/" {
		t.Errorf("dest = %q, want './output/'", cfg.dest)
	}
}

func TestParseTransferFlags_MissingRequired(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no flags", []string{}},
		{"missing source", []string{"-l", "servers.txt", "-u", "root", "-d", "/tmp/"}},
		{"missing dest", []string{"-l", "servers.txt", "-u", "root", "-s", "/tmp/file"}},
		{"missing user", []string{"-l", "servers.txt", "-s", "/tmp/file", "-d", "/tmp/"}},
		{"missing servers", []string{"-u", "root", "-s", "/tmp/file", "-d", "/tmp/"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			_, err := parseTransferFlags(tt.args, &stderr, "push")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseTransferFlags_PushSourceNotExist(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", "/nonexistent/file.txt", "-d", "/tmp/"}

	_, err := parseTransferFlags(args, &stderr, "push")
	if err == nil {
		t.Fatal("expected error for non-existent source file")
	}
	if !strings.Contains(err.Error(), "source file") {
		t.Errorf("error = %q, want containing 'source file'", err.Error())
	}
}

func TestParseTransferFlags_Defaults(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", "/remote/file", "-d", "./out/"}

	cfg, err := parseTransferFlags(args, &stderr, "pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.port != "22" {
		t.Errorf("port = %q, want '22'", cfg.port)
	}
	if cfg.maxWorkers != 100 {
		t.Errorf("maxWorkers = %d, want 100", cfg.maxWorkers)
	}
	if cfg.outputFormat != "text" {
		t.Errorf("outputFormat = %q, want 'text'", cfg.outputFormat)
	}
}

// --- Subcommand dispatch tests ---

func TestRun_SubcommandDispatch(t *testing.T) {
	// "run" with no required args should fail with exit 1
	var stderr bytes.Buffer
	code := run([]string{"run"}, &stderr)
	if code != 1 {
		t.Errorf("run(['run']) exit code = %d, want 1", code)
	}

	// "push" with no args should fail
	stderr.Reset()
	code = run([]string{"push"}, &stderr)
	if code != 1 {
		t.Errorf("run(['push']) exit code = %d, want 1", code)
	}

	// "pull" with no args should fail
	stderr.Reset()
	code = run([]string{"pull"}, &stderr)
	if code != 1 {
		t.Errorf("run(['pull']) exit code = %d, want 1", code)
	}
}

func TestRun_BackwardCompat(t *testing.T) {
	// No subcommand, just flags — should still work as "run" (fail for missing flags)
	var stderr bytes.Buffer
	code := run([]string{"-l", "servers.txt"}, &stderr)
	if code != 1 {
		t.Errorf("backward compat exit code = %d, want 1", code)
	}
}

// --- Bastion flag tests ---

func TestParseFlags_BastionFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-J", "admin@bastion.example.com:2222"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.bastionSpec != "admin@bastion.example.com:2222" {
		t.Errorf("bastionSpec = %q, want 'admin@bastion.example.com:2222'", cfg.bastionSpec)
	}
}

func TestParseTransferFlags_BastionFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", "/remote/file", "-d", "./out/", "-J", "admin@bastion:22"}

	cfg, err := parseTransferFlags(args, &stderr, "pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.bastionSpec != "admin@bastion:22" {
		t.Errorf("bastionSpec = %q, want 'admin@bastion:22'", cfg.bastionSpec)
	}
}

func TestParseFlags_TagsFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime", "-g", "web,prod"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.tags) != 2 || cfg.tags[0] != "web" || cfg.tags[1] != "prod" {
		t.Errorf("tags = %v, want [web prod]", cfg.tags)
	}
}

func TestParseFlags_NoTags(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.tags) != 0 {
		t.Errorf("tags = %v, want empty", cfg.tags)
	}
}

func TestParseTransferFlags_TagsFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-s", "/remote/file", "-d", "./out/", "-g", "db,staging"}

	cfg, err := parseTransferFlags(args, &stderr, "pull")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.tags) != 2 || cfg.tags[0] != "db" || cfg.tags[1] != "staging" {
		t.Errorf("tags = %v, want [db staging]", cfg.tags)
	}
}

func TestParseFlags_SudoFlag(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "apt update", "-S"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.sudo {
		t.Error("sudo = false, want true")
	}
}

func TestParseFlags_SudoDefault(t *testing.T) {
	var stderr bytes.Buffer
	args := []string{"-l", "servers.txt", "-u", "root", "-c", "uptime"}

	cfg, err := parseFlags(args, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.sudo {
		t.Error("sudo = true, want false")
	}
}
