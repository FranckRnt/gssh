package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"gssh/internal/completion"
	cfgfile "gssh/internal/config"
	"gssh/internal/report"
	"gssh/internal/runner"
	internalssh "gssh/internal/ssh"
	"gssh/internal/transfer"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// stringSlice implements flag.Value for repeated -c flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// config holds parsed CLI flags for the "run" subcommand.
type config struct {
	serversFile    string
	user           string
	commands       []string
	scriptFile     string
	keyPath        string
	knownHostsFile string
	insecure       bool
	confirm        bool
	port           string
	timeout        time.Duration
	maxWorkers     int
	retries        int
	verbose        bool
	dryRun         bool
	outputFormat   string
	bastionSpec    string
	tags           []string
	sudo           bool
	logDir         string
	stream         bool
	diff           bool
	group          bool
	reportFormat   string
}

// transferConfig holds parsed CLI flags for "push" and "pull" subcommands.
type transferConfig struct {
	serversFile    string
	user           string
	source         string
	dest           string
	keyPath        string
	knownHostsFile string
	insecure       bool
	confirm        bool
	port           string
	maxWorkers     int
	verbose        bool
	dryRun         bool
	outputFormat   string
	bastionSpec    string
	tags           []string
	logDir         string
}

// subcommands is the set of known subcommands.
var subcommands = map[string]bool{
	"run":  true,
	"push": true,
	"pull": true,
}

// parseFlags parses CLI arguments for the "run" subcommand.
func parseFlags(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("gssh run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg config
	var cmds stringSlice
	fs.StringVar(&cfg.serversFile, "l", "", "File with a list of servers (required)")
	fs.StringVar(&cfg.user, "u", "", "SSH user (required)")
	fs.Var(&cmds, "c", "Command to execute (repeatable for multi-command mode)")
	fs.StringVar(&cfg.scriptFile, "f", "", "File containing commands to execute (one per line)")
	fs.StringVar(&cfg.keyPath, "k", "", "Path to SSH private key (default: auto-detect)")
	fs.StringVar(&cfg.knownHostsFile, "known-hosts", "", "Path to known_hosts file (default: ~/.ssh/known_hosts)")
	fs.BoolVar(&cfg.insecure, "insecure", false, "Disable host key verification (NOT recommended)")
	fs.BoolVar(&cfg.confirm, "y", false, "Confirm dangerous operations (required with -insecure)")
	fs.StringVar(&cfg.port, "p", "22", "Default SSH port")
	fs.DurationVar(&cfg.timeout, "t", 30*time.Second, "Command timeout per server")
	fs.IntVar(&cfg.maxWorkers, "w", 100, "Maximum concurrent SSH connections")
	fs.IntVar(&cfg.retries, "r", 0, "Number of retries for failed servers")
	fs.BoolVar(&cfg.verbose, "v", false, "Show output in real-time for each server")
	fs.BoolVar(&cfg.dryRun, "n", false, "Dry run: show target servers without executing")
	fs.StringVar(&cfg.outputFormat, "o", "text", "Output format: text or json")
	fs.StringVar(&cfg.bastionSpec, "J", "", "Jump host: user@host[:port] (bastion/proxy)")

	var tags string
	fs.StringVar(&tags, "g", "", "Filter servers by tags (comma-separated, requires ALL)")
	fs.BoolVar(&cfg.sudo, "S", false, "Run commands via sudo (prompts for password once)")
	fs.StringVar(&cfg.logDir, "L", "", "Log directory for result files (default: ~/.gssh/logs/)")
	fs.BoolVar(&cfg.stream, "s", false, "Stream output line-by-line in real-time (implies -v, disables progress bar)")
	fs.BoolVar(&cfg.diff, "diff", false, "Compare outputs across servers and show differences")
	fs.BoolVar(&cfg.group, "group", false, "Group servers by identical output instead of per-server display")
	fs.StringVar(&cfg.reportFormat, "report", "", "Generate a report file: html")

	fs.Usage = func() {
		fmt.Fprintf(stderr, `gssh run — parallel SSH command execution

Usage:
  gssh [run] -l <servers_file> -u <user> -c <command> [options]
  gssh [run] -l <servers_file> -u <user> -c <cmd1> -c <cmd2> [options]
  gssh [run] -l <servers_file> -u <user> -f <script_file> [options]

Examples:
  # Run uptime on all servers
  gssh -l servers.txt -u root -c "uptime"

  # Run multiple commands (reuses SSH connection per server)
  gssh -l servers.txt -u deploy -c "uptime" -c "df -h" -c "free -m"

  # Load commands from a script file
  gssh -l servers.txt -u root -f commands.txt

  # Through a bastion/jump host
  gssh -l servers.txt -u root -c "uptime" -J admin@bastion.example.com

  # Dry run — see which servers would be targeted
  gssh -l servers.txt -u root -c "uptime" -n

  # Verbose output with 3 retries, JSON format
  gssh -l servers.txt -u root -c "uptime" -v -r 3 -o json

  # Custom key, port, timeout, workers
  gssh -l servers.txt -u deploy -c "systemctl status nginx" \
       -k ~/.ssh/deploy_key -p 2222 -t 60s -w 50

  # Skip host key verification (dangerous, requires -y)
  gssh -l servers.txt -u root -c "uptime" -insecure -y

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	cfg.commands = []string(cmds)
	if tags != "" {
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				cfg.tags = append(cfg.tags, t)
			}
		}
	}

	// Load config file defaults (CLI flags take precedence).
	applyConfigFileDefaults(fs, &cfg)

	// Load commands from script file if provided.
	if cfg.scriptFile != "" {
		if len(cfg.commands) > 0 {
			return config{}, fmt.Errorf("-c and -f are mutually exclusive: use one or the other")
		}
		scriptCmds, err := loadCommandsFromFile(cfg.scriptFile)
		if err != nil {
			return config{}, fmt.Errorf("load script file: %w", err)
		}
		cfg.commands = scriptCmds
	}

	if cfg.serversFile == "" || cfg.user == "" || len(cfg.commands) == 0 {
		fs.Usage()
		return config{}, fmt.Errorf("missing required flags")
	}

	if cfg.insecure && !cfg.confirm {
		return config{}, fmt.Errorf("-insecure requires -y to confirm you understand the risk")
	}

	if cfg.outputFormat != "text" && cfg.outputFormat != "json" {
		return config{}, fmt.Errorf("invalid output format %q: must be 'text' or 'json'", cfg.outputFormat)
	}

	if cfg.retries < 0 {
		return config{}, fmt.Errorf("retries must be >= 0")
	}

	if cfg.reportFormat != "" && cfg.reportFormat != "html" {
		return config{}, fmt.Errorf("invalid report format %q: must be 'html'", cfg.reportFormat)
	}

	return cfg, nil
}

// parseTransferFlags parses CLI arguments for "push" and "pull" subcommands.
func parseTransferFlags(args []string, stderr io.Writer, direction string) (transferConfig, error) {
	name := "gssh " + direction
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)

	var cfg transferConfig
	fs.StringVar(&cfg.serversFile, "l", "", "File with a list of servers (required)")
	fs.StringVar(&cfg.user, "u", "", "SSH user (required)")
	fs.StringVar(&cfg.source, "s", "", "Source path (required)")
	fs.StringVar(&cfg.dest, "d", "", "Destination path (required)")
	fs.StringVar(&cfg.keyPath, "k", "", "Path to SSH private key (default: auto-detect)")
	fs.StringVar(&cfg.knownHostsFile, "known-hosts", "", "Path to known_hosts file (default: ~/.ssh/known_hosts)")
	fs.BoolVar(&cfg.insecure, "insecure", false, "Disable host key verification (NOT recommended)")
	fs.BoolVar(&cfg.confirm, "y", false, "Confirm dangerous operations (required with -insecure)")
	fs.StringVar(&cfg.port, "p", "22", "Default SSH port")
	fs.IntVar(&cfg.maxWorkers, "w", 100, "Maximum concurrent SSH connections")
	fs.BoolVar(&cfg.verbose, "v", false, "Verbose output")
	fs.BoolVar(&cfg.dryRun, "n", false, "Dry run: show target servers without executing")
	fs.StringVar(&cfg.outputFormat, "o", "text", "Output format: text or json")
	fs.StringVar(&cfg.bastionSpec, "J", "", "Jump host: user@host[:port] (bastion/proxy)")

	var tags string
	fs.StringVar(&tags, "g", "", "Filter servers by tags (comma-separated, requires ALL)")
	fs.StringVar(&cfg.logDir, "L", "", "Log directory for result files (default: ~/.gssh/logs/)")

	fs.Usage = func() {
		if direction == "push" {
			fmt.Fprintf(stderr, `gssh push — upload files to multiple servers via SFTP

Usage:
  gssh push -l <servers_file> -u <user> -s <local_path> -d <remote_path> [options]

Examples:
  # Upload a config file to all servers
  gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf

  # Upload with custom port and key
  gssh push -l servers.txt -u deploy -s ./app.tar.gz -d /opt/app.tar.gz -p 2222 -k ~/.ssh/deploy_key

Options:
`)
		} else {
			fmt.Fprintf(stderr, `gssh pull — download files from multiple servers via SFTP

Usage:
  gssh pull -l <servers_file> -u <user> -s <remote_path> -d <output_dir> [options]

Files are saved to <output_dir>/<hostname>/<filename>.

Examples:
  # Download logs from all servers
  gssh pull -l servers.txt -u root -s /var/log/syslog -d ./logs/

  # Download config files
  gssh pull -l servers.txt -u deploy -s /etc/nginx/nginx.conf -d ./configs/

Options:
`)
		}
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return transferConfig{}, err
	}

	if tags != "" {
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				cfg.tags = append(cfg.tags, t)
			}
		}
	}

	// Apply config file defaults for common fields.
	applyTransferConfigFileDefaults(fs, &cfg)

	if cfg.serversFile == "" || cfg.user == "" || cfg.source == "" || cfg.dest == "" {
		fs.Usage()
		return transferConfig{}, fmt.Errorf("missing required flags: -l, -u, -s, -d are all required")
	}

	if cfg.insecure && !cfg.confirm {
		return transferConfig{}, fmt.Errorf("-insecure requires -y to confirm you understand the risk")
	}

	if cfg.outputFormat != "text" && cfg.outputFormat != "json" {
		return transferConfig{}, fmt.Errorf("invalid output format %q: must be 'text' or 'json'", cfg.outputFormat)
	}

	// For push, validate local source exists.
	if direction == "push" {
		if _, err := os.Stat(cfg.source); err != nil {
			return transferConfig{}, fmt.Errorf("source file %s: %w", cfg.source, err)
		}
	}

	return cfg, nil
}

// loadCommandsFromFile reads commands from a file, one per line.
func loadCommandsFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var cmds []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmds = append(cmds, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return cmds, nil
}

// applyConfigFileDefaults loads ~/.gssh/config.yaml and applies defaults where
// CLI flags were not explicitly set.
func applyConfigFileDefaults(fs *flag.FlagSet, cfg *config) {
	path := cfgfile.DefaultConfigPath()
	if path == "" {
		return
	}

	fileCfg, err := cfgfile.Load(path)
	if err != nil {
		slog.Warn("failed to load config file", "path", path, "error", err)
		return
	}

	// Track which flags were explicitly set on the command line.
	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	if !set["u"] && fileCfg.User != "" {
		cfg.user = fileCfg.User
	}
	if !set["k"] && fileCfg.KeyPath != "" {
		cfg.keyPath = expandTilde(fileCfg.KeyPath)
	}
	if !set["p"] && fileCfg.Port != "" {
		cfg.port = fileCfg.Port
	}
	if !set["t"] && fileCfg.Timeout != "" {
		if d, err := fileCfg.ParseTimeout(); err == nil {
			cfg.timeout = d
		}
	}
	if !set["w"] && fileCfg.MaxWorkers > 0 {
		cfg.maxWorkers = fileCfg.MaxWorkers
	}
	if !set["r"] && fileCfg.Retries > 0 {
		cfg.retries = fileCfg.Retries
	}
	if !set["known-hosts"] && fileCfg.KnownHosts != "" {
		cfg.knownHostsFile = expandTilde(fileCfg.KnownHosts)
	}
	if !set["v"] && fileCfg.Verbose {
		cfg.verbose = true
	}
	if !set["o"] && fileCfg.Output != "" {
		cfg.outputFormat = fileCfg.Output
	}
	if !set["L"] && fileCfg.LogDir != "" {
		cfg.logDir = expandTilde(fileCfg.LogDir)
	}
}

// expandTilde replaces ~ with the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	return path
}

// applyTransferConfigFileDefaults loads config file defaults for transfer commands.
func applyTransferConfigFileDefaults(fs *flag.FlagSet, cfg *transferConfig) {
	path := cfgfile.DefaultConfigPath()
	if path == "" {
		return
	}

	fileCfg, err := cfgfile.Load(path)
	if err != nil {
		slog.Warn("failed to load config file", "path", path, "error", err)
		return
	}

	set := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	if !set["u"] && fileCfg.User != "" {
		cfg.user = fileCfg.User
	}
	if !set["k"] && fileCfg.KeyPath != "" {
		cfg.keyPath = expandTilde(fileCfg.KeyPath)
	}
	if !set["p"] && fileCfg.Port != "" {
		cfg.port = fileCfg.Port
	}
	if !set["w"] && fileCfg.MaxWorkers > 0 {
		cfg.maxWorkers = fileCfg.MaxWorkers
	}
	if !set["known-hosts"] && fileCfg.KnownHosts != "" {
		cfg.knownHostsFile = expandTilde(fileCfg.KnownHosts)
	}
	if !set["v"] && fileCfg.Verbose {
		cfg.verbose = true
	}
	if !set["o"] && fileCfg.Output != "" {
		cfg.outputFormat = fileCfg.Output
	}
	if !set["L"] && fileCfg.LogDir != "" {
		cfg.logDir = expandTilde(fileCfg.LogDir)
	}
}

func run(args []string, stderr io.Writer) int {
	// Handle --completion before normal flag parsing.
	if len(args) == 1 {
		switch args[0] {
		case "--completion=bash":
			completion.Bash(os.Stdout)
			return 0
		case "--completion=zsh":
			completion.Zsh(os.Stdout)
			return 0
		case "--completion=fish":
			completion.Fish(os.Stdout)
			return 0
		case "-h", "--help":
			printGlobalUsage(stderr)
			return 0
		}
	}

	// Dispatch subcommands.
	if len(args) > 0 {
		switch args[0] {
		case "push":
			return runTransfer(args[1:], stderr, "push")
		case "pull":
			return runTransfer(args[1:], stderr, "pull")
		case "run":
			return runCommand(args[1:], stderr)
		}
	}

	// No subcommand or unknown first arg: treat as "run" (backward compat).
	return runCommand(args, stderr)
}

// dialBastion parses the bastion spec and establishes the jump host connection.
func dialBastion(spec, defaultPort string, authMethods []ssh.AuthMethod, hostKeyCallback ssh.HostKeyCallback) (*ssh.Client, error) {
	bastionUser, bastionHost, bastionPort, err := internalssh.ParseBastionSpec(spec, defaultPort)
	if err != nil {
		return nil, fmt.Errorf("parse bastion spec: %w", err)
	}

	bastionConfig := &ssh.ClientConfig{
		User:            bastionUser,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	bastionAddr := fmt.Sprintf("%s:%s", bastionHost, bastionPort)
	slog.Info("connecting to bastion", "addr", bastionAddr, "user", bastionUser)

	return internalssh.DialBastion(bastionAddr, bastionConfig)
}

func printGlobalUsage(w io.Writer) {
	fmt.Fprintf(w, `gssh — parallel SSH tool

Usage:
  gssh [command] [options]

Commands:
  run     Execute commands on multiple servers (default)
  push    Upload files to multiple servers via SFTP
  pull    Download files from multiple servers via SFTP

Shell Completion:
  gssh --completion=bash   # Bash: eval "$(gssh --completion=bash)"
  gssh --completion=zsh    # Zsh:  eval "$(gssh --completion=zsh)"
  gssh --completion=fish   # Fish: gssh --completion=fish | source

Run 'gssh <command> -h' for help on a specific command.
`)
}

func runCommand(args []string, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		return 1
	}

	// Load servers.
	allServers, err := internalssh.LoadServers(cfg.serversFile)
	if err != nil {
		slog.Error("failed to load servers", "error", err)
		return 1
	}

	// Filter by tags if specified.
	filtered := internalssh.FilterByTags(allServers, cfg.tags)
	if len(filtered) == 0 {
		slog.Error("no servers found after filtering", "file", cfg.serversFile, "tags", cfg.tags)
		return 1
	}
	servers := internalssh.Hosts(filtered)
	slog.Info("servers loaded", "count", len(servers), "tags", cfg.tags)

	// Setup formatter.
	useColor := runner.IsTerminal(os.Stdout)
	fmtr := runner.NewFormatter(os.Stdout, useColor)

	// Dry run: just list servers and exit.
	if cfg.dryRun {
		fmtr.PrintDryRun(servers, cfg.port)
		return 0
	}

	// Resolve key path default.
	if cfg.keyPath == "" {
		cfg.keyPath = internalssh.ResolveDefaultKeyPath()
	}

	// Build auth methods.
	authMethods, agentCloser, err := internalssh.BuildAuthMethods(cfg.keyPath)
	if err != nil {
		slog.Error("no authentication available", "error", err)
		return 1
	}
	if agentCloser != nil {
		defer agentCloser.Close()
	}

	// Build host key callback.
	hostKeyCallback, err := internalssh.BuildHostKeyCallback(cfg.knownHostsFile, cfg.insecure)
	if err != nil {
		slog.Error("host key verification setup failed", "error", err)
		return 1
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	// Prompt for sudo password if needed.
	var sudoPassword string
	if cfg.sudo {
		fmt.Fprint(stderr, "Sudo password: ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(stderr) // newline after hidden input
		if err != nil {
			slog.Error("failed to read sudo password", "error", err)
			return 1
		}
		sudoPassword = string(passBytes)
	}

	// Build server tags map for template expansion.
	serverTags := make(map[string][]string, len(filtered))
	for _, s := range filtered {
		serverTags[s.Host] = s.Tags
	}

	// Context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		slog.Warn("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Connect to bastion if specified.
	var bastionClient *ssh.Client
	if cfg.bastionSpec != "" {
		bastionClient, err = dialBastion(cfg.bastionSpec, cfg.port, authMethods, hostKeyCallback)
		if err != nil {
			slog.Error("bastion connection failed", "error", err)
			return 1
		}
		defer bastionClient.Close()
	}

	// Streaming mode implies verbose (disables compact output and progress bar).
	if cfg.stream {
		cfg.verbose = true
	}

	// Build stream callback for real-time line output.
	var streamCB internalssh.StreamCallback
	if cfg.stream {
		var streamMu sync.Mutex
		streamCB = func(hostname, stream, line string) {
			streamMu.Lock()
			defer streamMu.Unlock()
			fmtr.PrintStreamLine(hostname, stream, line)
		}
	}

	// Callback: print results per server as they complete.
	var cb runner.ResultCallback
	var mu sync.Mutex
	if !cfg.group {
		// In group mode, we suppress per-server output and print grouped after completion.
		cb = func(serverResults []internalssh.Result) {
			mu.Lock()
			defer mu.Unlock()
			if cfg.stream {
				for _, r := range serverResults {
					if !r.IsSuccess() && r.Error != "" {
						fmtr.PrintResult(r)
					}
				}
			} else if cfg.verbose {
				for _, r := range serverResults {
					fmtr.PrintResult(r)
				}
			} else if cfg.outputFormat == "text" {
				fmtr.PrintCompactServer(serverResults)
			}
		}
	}

	// Run commands on all servers.
	results, elapsed := runner.Run(ctx, runner.Config{
		SSHConfig:     sshConfig,
		Servers:       servers,
		Port:          cfg.port,
		Commands:      cfg.commands,
		Timeout:       cfg.timeout,
		MaxWorkers:    cfg.maxWorkers,
		Retries:       cfg.retries,
		Verbose:       cfg.verbose,
		BastionClient: bastionClient,
		SudoPassword:  sudoPassword,
		ServerTags:    serverTags,
		StreamCB:      streamCB,
	}, cb)

	summary := internalssh.ComputeSummary(results, elapsed)
	if cfg.outputFormat == "text" {
		// Grouped mode: display results grouped by identical output.
		if cfg.group {
			reports := runner.ComputeDiff(results)
			fmtr.PrintGrouped(reports)
			fmt.Fprintln(os.Stdout)
		}
		fmtr.PrintSummary(summary)
	}

	// Diff mode: compare outputs across servers.
	if cfg.diff && cfg.outputFormat == "text" {
		fmt.Fprintln(os.Stdout)
		reports := runner.ComputeDiff(results)
		fmtr.PrintDiff(reports)
	}

	// Write JSON log file.
	logDir := cfg.logDir
	if logDir == "" {
		logDir = cfgfile.DefaultLogDir()
	}
	fileName, err := internalssh.WriteResults(results, elapsed, logDir)
	if err != nil {
		slog.Error("failed to write results", "error", err)
		return 1
	}
	if fileName != "" {
		slog.Info("results written", "file", fileName, "count", len(results))
	}

	// Generate HTML report if requested.
	if cfg.reportFormat == "html" {
		htmlFile, err := report.WriteHTML(results, summary, elapsed, logDir)
		if err != nil {
			slog.Error("failed to write HTML report", "error", err)
			return 1
		}
		slog.Info("HTML report written", "file", htmlFile)
	}

	// Exit non-zero if any server failed.
	if summary.Failed > 0 {
		return 2
	}
	return 0
}

func runTransfer(args []string, stderr io.Writer, direction string) int {
	cfg, err := parseTransferFlags(args, stderr, direction)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "ERROR: %v\n", err)
		return 1
	}

	// Load servers.
	allServers, err := internalssh.LoadServers(cfg.serversFile)
	if err != nil {
		slog.Error("failed to load servers", "error", err)
		return 1
	}

	// Filter by tags if specified.
	filtered := internalssh.FilterByTags(allServers, cfg.tags)
	if len(filtered) == 0 {
		slog.Error("no servers found after filtering", "file", cfg.serversFile, "tags", cfg.tags)
		return 1
	}
	servers := internalssh.Hosts(filtered)
	slog.Info("servers loaded", "count", len(servers), "tags", cfg.tags)

	// Setup formatter.
	useColor := runner.IsTerminal(os.Stdout)
	fmtr := runner.NewFormatter(os.Stdout, useColor)

	// Dry run.
	if cfg.dryRun {
		fmtr.PrintDryRun(servers, cfg.port)
		fmt.Fprintf(os.Stdout, "\n  %s  %s -> %s\n",
			strings.ToUpper(direction), cfg.source, cfg.dest)
		return 0
	}

	// Resolve key path default.
	if cfg.keyPath == "" {
		cfg.keyPath = internalssh.ResolveDefaultKeyPath()
	}

	// Build auth methods.
	authMethods, agentCloser, err := internalssh.BuildAuthMethods(cfg.keyPath)
	if err != nil {
		slog.Error("no authentication available", "error", err)
		return 1
	}
	if agentCloser != nil {
		defer agentCloser.Close()
	}

	// Build host key callback.
	hostKeyCallback, err := internalssh.BuildHostKeyCallback(cfg.knownHostsFile, cfg.insecure)
	if err != nil {
		slog.Error("host key verification setup failed", "error", err)
		return 1
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	// Context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		slog.Warn("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Connect to bastion if specified.
	var bastionClient *ssh.Client
	if cfg.bastionSpec != "" {
		bastionClient, err = dialBastion(cfg.bastionSpec, cfg.port, authMethods, hostKeyCallback)
		if err != nil {
			slog.Error("bastion connection failed", "error", err)
			return 1
		}
		defer bastionClient.Close()
	}

	// Determine transfer direction.
	dir := transfer.Upload
	if direction == "pull" {
		dir = transfer.Download
	}

	// Callback: print result per server.
	var mu sync.Mutex
	cb := func(r transfer.Result) {
		mu.Lock()
		defer mu.Unlock()
		if cfg.outputFormat == "text" {
			fmtr.PrintTransferResult(r)
		}
	}

	results, elapsed := transfer.Run(ctx, transfer.Config{
		SSHConfig:     sshConfig,
		Servers:       servers,
		Port:          cfg.port,
		Source:        cfg.source,
		Dest:          cfg.dest,
		Direction:     dir,
		MaxWorkers:    cfg.maxWorkers,
		BastionClient: bastionClient,
	}, cb)

	summary := transfer.ComputeSummary(results, elapsed)
	if cfg.outputFormat == "text" {
		fmtr.PrintTransferSummary(summary)
	}

	// Write JSON log file.
	logDir := cfg.logDir
	if logDir == "" {
		logDir = cfgfile.DefaultLogDir()
	}
	fileName, err := transfer.WriteResults(results, elapsed, logDir)
	if err != nil {
		slog.Error("failed to write results", "error", err)
		return 1
	}
	if fileName != "" {
		slog.Info("results written", "file", fileName, "count", len(results))
	}

	if summary.Failed > 0 {
		return 2
	}
	return 0
}
