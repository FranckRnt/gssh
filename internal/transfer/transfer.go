// Package transfer provides SFTP-based file upload and download operations
// over existing SSH connections.
package transfer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	internalssh "gssh/internal/ssh"
)

// Direction indicates whether the transfer is an upload or download.
type Direction int

const (
	Upload Direction = iota
	Download
)

// Result holds the outcome of a file transfer on a single host.
type Result struct {
	Hostname  string        `json:"hostname"`
	Direction string        `json:"direction"` // "push" or "pull"
	Source    string        `json:"source"`
	Dest      string        `json:"dest"`
	Bytes     int64         `json:"bytes"`
	Duration  time.Duration `json:"duration_ms"`
	Error     string        `json:"error,omitempty"`
}

// IsSuccess returns true if the transfer completed without error.
func (r Result) IsSuccess() bool {
	return r.Error == ""
}

// UploadFile uploads a local file to a remote path via SFTP over the given SSH client.
// If remotePath is an existing remote directory (or ends with /), the source
// filename is appended automatically (e.g. /root + test.txt -> /root/test.txt).
func UploadFile(ctx context.Context, client *ssh.Client, localPath, remotePath string) (int64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("create sftp client: %w", err)
	}
	defer sftpClient.Close()

	local, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("open local file: %w", err)
	}
	defer local.Close()

	// Get local file info for permission.
	info, err := local.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat local file: %w", err)
	}

	// If remotePath is a directory (or ends with /), append source filename.
	remotePath = resolveRemotePath(sftpClient, localPath, remotePath)

	// Ensure remote parent directory exists.
	remoteDir := path.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			slog.Debug("mkdir remote dir (may already exist)", "dir", remoteDir, "error", err)
		}
	}

	remote, err := sftpClient.Create(remotePath)
	if err != nil {
		return 0, fmt.Errorf("create remote file %s: %w", remotePath, err)
	}
	defer remote.Close()

	n, err := io.Copy(remote, local)
	if err != nil {
		return n, fmt.Errorf("copy to remote: %w", err)
	}

	// Preserve permissions.
	if err := sftpClient.Chmod(remotePath, info.Mode()); err != nil {
		slog.Debug("chmod remote file failed (non-fatal)", "path", remotePath, "error", err)
	}

	return n, nil
}

// DownloadFile downloads a remote file to a local path via SFTP over the given SSH client.
func DownloadFile(ctx context.Context, client *ssh.Client, remotePath, localPath string) (int64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return 0, fmt.Errorf("create sftp client: %w", err)
	}
	defer sftpClient.Close()

	remote, err := sftpClient.Open(remotePath)
	if err != nil {
		return 0, fmt.Errorf("open remote file %s: %w", remotePath, err)
	}
	defer remote.Close()

	// Get remote file info for permission.
	info, err := remote.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat remote file: %w", err)
	}

	// Ensure local parent directory exists.
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return 0, fmt.Errorf("create local directory %s: %w", localDir, err)
	}

	local, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local file %s: %w", localPath, err)
	}
	defer local.Close()

	n, err := io.Copy(local, remote)
	if err != nil {
		return n, fmt.Errorf("copy from remote: %w", err)
	}

	// Preserve permissions.
	if err := os.Chmod(localPath, info.Mode()); err != nil {
		slog.Debug("chmod local file failed (non-fatal)", "path", localPath, "error", err)
	}

	return n, nil
}

// ProcessServerUpload connects to a server and uploads a file.
func ProcessServerUpload(ctx context.Context, bastionClient *ssh.Client, sshConfig *ssh.ClientConfig, addr, hostname, localPath, remotePath string) Result {
	start := time.Now()

	if ctx.Err() != nil {
		return Result{
			Hostname:  hostname,
			Direction: "push",
			Source:    localPath,
			Dest:      remotePath,
			Duration:  time.Since(start),
			Error:     "shutdown requested",
		}
	}

	client, err := internalssh.DialSSH(ctx, bastionClient, addr, sshConfig)
	if err != nil {
		return Result{
			Hostname:  hostname,
			Direction: "push",
			Source:    localPath,
			Dest:      remotePath,
			Duration:  time.Since(start),
			Error:     fmt.Sprintf("SSH dial: %v", err),
		}
	}
	defer client.Close()

	n, err := UploadFile(ctx, client, localPath, remotePath)
	r := Result{
		Hostname:  hostname,
		Direction: "push",
		Source:    localPath,
		Dest:      remotePath,
		Bytes:     n,
		Duration:  time.Since(start),
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

// ProcessServerDownload connects to a server and downloads a file.
// The local path is placed under outputDir/<hostname>/<filename>.
func ProcessServerDownload(ctx context.Context, bastionClient *ssh.Client, sshConfig *ssh.ClientConfig, addr, hostname, remotePath, outputDir string) Result {
	start := time.Now()
	localPath := filepath.Join(outputDir, hostname, filepath.Base(remotePath))

	if ctx.Err() != nil {
		return Result{
			Hostname:  hostname,
			Direction: "pull",
			Source:    remotePath,
			Dest:      localPath,
			Duration:  time.Since(start),
			Error:     "shutdown requested",
		}
	}

	client, err := internalssh.DialSSH(ctx, bastionClient, addr, sshConfig)
	if err != nil {
		return Result{
			Hostname:  hostname,
			Direction: "pull",
			Source:    remotePath,
			Dest:      localPath,
			Duration:  time.Since(start),
			Error:     fmt.Sprintf("SSH dial: %v", err),
		}
	}
	defer client.Close()

	n, err := DownloadFile(ctx, client, remotePath, localPath)
	r := Result{
		Hostname:  hostname,
		Direction: "pull",
		Source:    remotePath,
		Dest:      localPath,
		Bytes:     n,
		Duration:  time.Since(start),
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

// resolveRemotePath checks if remotePath is a directory (existing on remote, or
// ending with "/") and appends the source filename if so.
// Uses POSIX path semantics (path.Base) for remote paths.
func resolveRemotePath(sftpClient *sftp.Client, localPath, remotePath string) string {
	// Explicit directory indicator.
	if strings.HasSuffix(remotePath, "/") {
		return path.Join(remotePath, filepath.Base(localPath))
	}

	// Check if it's an existing remote directory.
	if info, err := sftpClient.Stat(remotePath); err == nil && info.IsDir() {
		return path.Join(remotePath, filepath.Base(localPath))
	}

	return remotePath
}
