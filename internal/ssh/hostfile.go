package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Server represents a target host with optional tags.
type Server struct {
	Host string   // "hostname" or "hostname:port"
	Tags []string // e.g. ["web", "prod", "paris"]
}

// Hosts extracts the host strings from a slice of servers.
func Hosts(servers []Server) []string {
	hosts := make([]string, len(servers))
	for i, s := range servers {
		hosts[i] = s.Host
	}
	return hosts
}

// FilterByTags returns servers that have ALL the specified tags (intersection).
// If tags is empty, all servers are returned.
func FilterByTags(servers []Server, tags []string) []Server {
	if len(tags) == 0 {
		return servers
	}

	var filtered []Server
	for _, s := range servers {
		if hasAllTags(s.Tags, tags) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func hasAllTags(serverTags, required []string) bool {
	set := make(map[string]bool, len(serverTags))
	for _, t := range serverTags {
		set[t] = true
	}
	for _, t := range required {
		if !set[t] {
			return false
		}
	}
	return true
}

// LoadServers reads a list of servers from a source.
// The source can be:
//   - A regular file path
//   - An executable file (its stdout is parsed)
//   - A URL starting with http:// or https://
//
// Format: one server per line, optional tags prefixed with #.
//
//	web01.example.com #web #prod
//	db01.example.com:5432 #db #prod #paris
func LoadServers(source string) ([]Server, error) {
	// URL source.
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadServersFromURL(source)
	}

	// Check if source is executable (dynamic inventory).
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat servers source: %w", err)
	}
	if !info.IsDir() && info.Mode()&0111 != 0 {
		return loadServersFromExecutable(source)
	}

	// Regular file.
	return loadServersFromFile(source)
}

// loadServersFromFile reads servers from a regular file with permission checks.
func loadServersFromFile(filename string) ([]Server, error) {
	if err := validateFilePermissions(filename); err != nil {
		return nil, err
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open servers file: %w", err)
	}
	defer file.Close()

	return parseServers(file)
}

// loadServersFromExecutable runs an executable and parses its stdout.
func loadServersFromExecutable(path string) ([]Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("run inventory script %s: %w", path, err)
	}

	return parseServers(strings.NewReader(string(out)))
}

// loadServersFromURL fetches a URL and parses the response body.
func loadServersFromURL(url string) ([]Server, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch inventory URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inventory URL %s returned status %d", url, resp.StatusCode)
	}

	return parseServers(resp.Body)
}

// parseServers parses server lines from a reader.
// Format: host[:port] [#tag1 #tag2 ...]
func parseServers(r io.Reader) ([]Server, error) {
	var servers []Server
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		srv := parseLine(line)
		servers = append(servers, srv)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read servers: %w", err)
	}
	return servers, nil
}

// parseLine parses "host[:port] #tag1 #tag2" into a Server.
func parseLine(line string) Server {
	// Split on whitespace. First token is host, rest are tags (prefixed with #).
	fields := strings.Fields(line)
	srv := Server{Host: fields[0]}

	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "#") {
			tag := strings.TrimPrefix(f, "#")
			if tag != "" {
				srv.Tags = append(srv.Tags, tag)
			}
		}
	}
	return srv
}

// validateFilePermissions checks that the servers file is not world-writable.
// This prevents an attacker from injecting hosts via a loosely-permissioned file.
func validateFilePermissions(filename string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Lstat(filename)
	if err != nil {
		return fmt.Errorf("stat servers file: %w", err)
	}

	// Reject symlinks to prevent redirection attacks.
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("servers file %s is a symlink, refusing to read", filename)
	}

	// Reject world-writable files.
	if info.Mode().Perm()&0002 != 0 {
		return fmt.Errorf("servers file %s is world-writable (mode %o), refusing to read", filename, info.Mode().Perm())
	}

	return nil
}
