package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// BuildHostKeyCallback returns a host key callback based on the provided
// known_hosts path. If insecure is true, host key verification is disabled.
func BuildHostKeyCallback(knownHostsPath string, insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		slog.Warn("host key verification disabled — vulnerable to MITM attacks")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if knownHostsPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		knownHostsPath = filepath.Join(homeDir, ".ssh", "known_hosts")
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %s: %w", knownHostsPath, err)
	}
	return callback, nil
}

// ParseHostPort extracts host and port from a server entry.
// Supports "host", "host:port", or bare host with a default port.
func ParseHostPort(server string, defaultPort string) (string, string) {
	if strings.Contains(server, ":") {
		host, port, err := net.SplitHostPort(server)
		if err == nil {
			return host, port
		}
	}
	return server, defaultPort
}

// RunCommand executes a command on an SSH client with context-aware timeout.
func RunCommand(ctx context.Context, client *ssh.Client, command string, timeout time.Duration, hostname string, streamCB StreamCallback) (stdout, stderr string, exitCode int, err error) {
	return runSSHCommand(ctx, client, command, timeout, "", hostname, streamCB)
}

// RunCommandWithSudo wraps the command in "sudo -S -- <command>" and feeds
// the password to stdin. The password is written followed by a newline,
// matching sudo's -S (read from stdin) behavior.
func RunCommandWithSudo(ctx context.Context, client *ssh.Client, command string, timeout time.Duration, password, hostname string, streamCB StreamCallback) (stdout, stderr string, exitCode int, err error) {
	sudoCmd := fmt.Sprintf("sudo -S -- %s", command)
	return runSSHCommand(ctx, client, sudoCmd, timeout, password, hostname, streamCB)
}

func runSSHCommand(ctx context.Context, client *ssh.Client, command string, timeout time.Duration, stdinData, hostname string, streamCB StreamCallback) (stdout, stderr string, exitCode int, err error) {
	// Create a deadline that covers the entire operation: session creation,
	// command start, and command execution. This prevents hanging when the
	// remote server is overloaded (100% CPU) and cannot allocate a session
	// or start a process in time.
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type sessionResult struct {
		session *ssh.Session
		err     error
	}
	sessCh := make(chan sessionResult, 1)
	go func() {
		s, e := client.NewSession()
		sessCh <- sessionResult{s, e}
	}()

	var session *ssh.Session
	select {
	case res := <-sessCh:
		if res.err != nil {
			return "", "", 1, fmt.Errorf("create session: %w", res.err)
		}
		session = res.session
	case <-cmdCtx.Done():
		return "", "", 1, fmt.Errorf("timed out creating session after %v", timeout)
	}
	defer session.Close()

	stdoutBuf := NewLineWriter(DefaultMaxOutputBytes, hostname, "stdout", streamCB)
	stderrBuf := NewLineWriter(DefaultMaxOutputBytes, hostname, "stderr", streamCB)
	session.Stdout = stdoutBuf
	session.Stderr = stderrBuf

	// Feed stdin if provided (used for sudo password).
	if stdinData != "" {
		pipe, err := session.StdinPipe()
		if err != nil {
			return "", "", 1, fmt.Errorf("stdin pipe: %w", err)
		}
		go func() {
			defer pipe.Close()
			pipe.Write([]byte(stdinData + "\n"))
		}()
	}

	startCh := make(chan error, 1)
	go func() {
		startCh <- session.Start(command)
	}()

	select {
	case startErr := <-startCh:
		if startErr != nil {
			return "", "", 1, fmt.Errorf("start command: %w", startErr)
		}
	case <-cmdCtx.Done():
		return "", "", 1, fmt.Errorf("timed out starting command after %v", timeout)
	}

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case cmdErr := <-done:
		stdoutBuf.Flush()
		stderrBuf.Flush()
		if cmdErr != nil {
			if exitErr, ok := cmdErr.(*ssh.ExitError); ok {
				return stdoutBuf.String(), stderrBuf.String(), exitErr.ExitStatus(), nil
			}
			return stdoutBuf.String(), stderrBuf.String(), 1, fmt.Errorf("run command: %w", cmdErr)
		}
		return stdoutBuf.String(), stderrBuf.String(), 0, nil

	case <-cmdCtx.Done():
		stdoutBuf.Flush()
		stderrBuf.Flush()
		return "", "", 1, fmt.Errorf("command timed out after %v", timeout)
	}
}

// DialSSH establishes an SSH connection to the target, optionally through a bastion host.
// If bastionClient is non-nil, the connection is tunneled through it.
// The caller must close the returned client.
func DialSSH(ctx context.Context, bastionClient *ssh.Client, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if bastionClient == nil {
		return ssh.Dial("tcp", addr, config)
	}

	// Dial through the bastion.
	conn, err := bastionClient.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bastion tunnel to %s: %w", addr, err)
	}

	ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake via bastion to %s: %w", addr, err)
	}

	return ssh.NewClient(ncc, chans, reqs), nil
}

// DialBastion establishes a connection to the bastion/jump host.
// bastionAddr is "host:port", bastionConfig is the SSH config for the bastion.
func DialBastion(bastionAddr string, bastionConfig *ssh.ClientConfig) (*ssh.Client, error) {
	client, err := ssh.Dial("tcp", bastionAddr, bastionConfig)
	if err != nil {
		return nil, fmt.Errorf("dial bastion %s: %w", bastionAddr, err)
	}
	return client, nil
}

// ParseBastionSpec parses a "-J user@host[:port]" spec into user, host, port.
// If no port is specified, defaultPort is used.
func ParseBastionSpec(spec, defaultPort string) (user, host, port string, err error) {
	if spec == "" {
		return "", "", "", fmt.Errorf("empty bastion spec")
	}

	userHost := spec
	if at := strings.Index(spec, "@"); at >= 0 {
		user = spec[:at]
		userHost = spec[at+1:]
	}

	if user == "" {
		return "", "", "", fmt.Errorf("bastion spec requires user: user@host[:port]")
	}

	host, port = ParseHostPort(userHost, defaultPort)
	return user, host, port, nil
}

// ProcessServer connects to a server via SSH and executes one or more commands
// over a single TCP connection (SSH multiplexing).
// If resolvedAddr is non-empty, it is used instead of resolving host:port.
func ProcessServer(ctx context.Context, sshConfig *ssh.ClientConfig, server, port string, commands []string, timeout time.Duration, resolvedAddr string) []Result {
	return ProcessServerViaBastion(ctx, nil, sshConfig, server, port, commands, timeout, resolvedAddr, "", nil)
}

// ProcessServerViaBastion is like ProcessServer but optionally tunnels through a bastion.
// If sudoPassword is non-empty, commands are wrapped with sudo -S.
// If streamCB is non-nil, output lines are streamed in real-time.
func ProcessServerViaBastion(ctx context.Context, bastionClient *ssh.Client, sshConfig *ssh.ClientConfig, server, port string, commands []string, timeout time.Duration, resolvedAddr, sudoPassword string, streamCB StreamCallback) []Result {
	start := time.Now()
	dateCommand := start.Format(time.RFC3339)
	host, srvPort := ParseHostPort(server, port)
	addr := resolvedAddr
	if addr == "" {
		addr = net.JoinHostPort(host, srvPort)
	}
	log := slog.With("server", host)

	if ctx.Err() != nil {
		var results []Result
		for _, cmd := range commands {
			results = append(results, Result{
				Hostname: host, Command: cmd, DateCommand: dateCommand,
				Duration: time.Since(start), ReturnCode: 1, Error: "shutdown requested",
			})
		}
		return results
	}

	client, err := DialSSH(ctx, bastionClient, addr, sshConfig)
	if err != nil {
		log.Warn("SSH connection failed", "addr", addr, "error", err)
		var results []Result
		for _, cmd := range commands {
			results = append(results, Result{
				Hostname: host, Command: cmd, DateCommand: dateCommand,
				Duration: time.Since(start), ReturnCode: 1, Error: fmt.Sprintf("SSH dial: %v", err),
			})
		}
		return results
	}
	defer client.Close()

	results := make([]Result, 0, len(commands))
	for _, command := range commands {
		cmdStart := time.Now()
		var stdout, stderr string
		var exitCode int
		var err error
		if sudoPassword != "" {
			stdout, stderr, exitCode, err = RunCommandWithSudo(ctx, client, command, timeout, sudoPassword, host, streamCB)
		} else {
			stdout, stderr, exitCode, err = RunCommand(ctx, client, command, timeout, host, streamCB)
		}
		cmdDuration := time.Since(cmdStart)

		if err != nil {
			log.Warn("command execution failed", "command", command, "error", err)
			results = append(results, Result{
				Hostname: host, Command: command, DateCommand: dateCommand,
				Duration: cmdDuration, Output: stdout, Stderr: stderr,
				ReturnCode: exitCode, Error: err.Error(),
			})
			// Stop executing further commands on this server if one fails.
			for _, remaining := range commands[len(results):] {
				results = append(results, Result{
					Hostname: host, Command: remaining, DateCommand: dateCommand,
					Duration: 0, ReturnCode: 1, Error: fmt.Sprintf("skipped: previous command failed"),
				})
			}
			break
		}

		log.Debug("command completed", "command", command, "exit_code", exitCode, "duration", cmdDuration)
		results = append(results, Result{
			Hostname: host, Command: command, DateCommand: dateCommand,
			Duration: cmdDuration, Output: stdout, Stderr: stderr, ReturnCode: exitCode,
		})
	}

	return results
}
