package ssh

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// defaultKeyNames lists private key filenames to try, in priority order.
var defaultKeyNames = []string{
	"id_ed25519",
	"id_ecdsa",
	"id_rsa",
}

// AgentAuth connects to the SSH agent and returns the auth method along with
// a closer that the caller must invoke to release the agent connection.
func AgentAuth() (ssh.AuthMethod, io.Closer, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to ssh-agent: %w", err)
	}
	agentClient := agent.NewClient(conn)

	// Verify the agent actually has keys loaded.
	keys, err := agentClient.List()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("list agent keys: %w", err)
	}
	if len(keys) == 0 {
		conn.Close()
		return nil, nil, fmt.Errorf("ssh-agent has no keys loaded")
	}

	return ssh.PublicKeysCallback(agentClient.Signers), conn, nil
}

// LoadSSHKey reads and parses an SSH private key file.
// If the key is encrypted with a passphrase, it prompts the user interactively.
func LoadSSHKey(keyPath string) (ssh.Signer, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return signer, nil
	}

	// If parsing failed, try with passphrase (key might be encrypted).
	passphraseMissingErr, ok := err.(*ssh.PassphraseMissingError)
	if !ok {
		return nil, fmt.Errorf("parse private key %s: %w", keyPath, err)
	}
	_ = passphraseMissingErr

	passphrase, promptErr := readPassphrase(keyPath)
	if promptErr != nil {
		return nil, fmt.Errorf("read passphrase for %s: %w", keyPath, promptErr)
	}

	signer, err = ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("parse private key %s with passphrase: %w", keyPath, err)
	}
	return signer, nil
}

// readPassphrase prompts the user for a passphrase on the terminal.
func readPassphrase(keyPath string) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Enter passphrase for key '%s': ", keyPath)
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden input
	if err != nil {
		return nil, fmt.Errorf("read from terminal: %w", err)
	}
	return passphrase, nil
}

// ResolveDefaultKeyPath returns the first existing private key path from the
// user's ~/.ssh directory, trying ed25519, ecdsa, then rsa.
// Returns an empty string if none are found.
func ResolveDefaultKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	sshDir := filepath.Join(homeDir, ".ssh")
	for _, name := range defaultKeyNames {
		path := filepath.Join(sshDir, name)
		if _, err := os.Stat(path); err == nil {
			slog.Debug("using default key", "path", path)
			return path
		}
	}
	return ""
}

// BuildAuthMethods returns available SSH authentication methods.
// The returned closer (if non-nil) must be called to release the agent connection.
func BuildAuthMethods(keyPath string) ([]ssh.AuthMethod, io.Closer, error) {
	var methods []ssh.AuthMethod
	var agentCloser io.Closer

	// Try ssh-agent first.
	if authMethod, closer, err := AgentAuth(); err == nil {
		methods = append(methods, authMethod)
		agentCloser = closer
		slog.Debug("ssh-agent authentication available")
	} else {
		slog.Debug("ssh-agent not available", "error", err)
	}

	// Then try the explicit key (or resolved default).
	if keyPath != "" {
		signer, err := LoadSSHKey(keyPath)
		if err != nil {
			slog.Warn("failed to load SSH key", "path", keyPath, "error", err)
		} else {
			methods = append(methods, ssh.PublicKeys(signer))
			slog.Debug("key authentication available", "path", keyPath)
		}
	}

	if len(methods) == 0 {
		return nil, nil, fmt.Errorf("no SSH authentication method available (no agent, no usable key)")
	}
	return methods, agentCloser, nil
}
