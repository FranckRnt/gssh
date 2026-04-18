package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// ResolvedServer holds a server entry with its pre-resolved IP address.
type ResolvedServer struct {
	Original string // original server string from file
	Host     string // parsed hostname
	Port     string // parsed port
	Addr     string // resolved "ip:port" ready for Dial
}

// ResolveServers resolves DNS for all servers in parallel, returning resolved
// entries. Servers that fail DNS resolution are included with an empty Addr
// and will be reported as errors during execution.
func ResolveServers(servers []string, defaultPort string) []ResolvedServer {
	resolved := make([]ResolvedServer, len(servers))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	// Limit concurrent DNS lookups.
	sem := make(chan struct{}, 50)

	for i, srv := range servers {
		wg.Add(1)
		go func(i int, srv string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			host, port := ParseHostPort(srv, defaultPort)
			rs := ResolvedServer{
				Original: srv,
				Host:     host,
				Port:     port,
			}

			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				slog.Debug("DNS resolution failed, will use hostname directly", "host", host, "error", err)
				// Fall back to using hostname directly — SSH dial will resolve it.
				rs.Addr = net.JoinHostPort(host, port)
			} else {
				rs.Addr = net.JoinHostPort(ips[0], port)
				if ips[0] != host {
					slog.Debug("resolved", "host", host, "ip", ips[0])
				}
			}

			resolved[i] = rs
		}(i, srv)
	}

	wg.Wait()
	return resolved
}

// ResolvedServerMap builds a hostname->ResolvedServer lookup for quick access.
func ResolvedServerMap(resolved []ResolvedServer) map[string]ResolvedServer {
	m := make(map[string]ResolvedServer, len(resolved))
	for _, rs := range resolved {
		m[rs.Original] = rs
	}
	return m
}

// FormatResolutionErrors returns a human-readable string of any DNS failures.
func FormatResolutionErrors(resolved []ResolvedServer) string {
	var errs []string
	for _, rs := range resolved {
		if rs.Addr == "" {
			errs = append(errs, fmt.Sprintf("  %s: resolution failed", rs.Host))
		}
	}
	if len(errs) == 0 {
		return ""
	}
	return fmt.Sprintf("DNS resolution failures:\n%s", joinLines(errs))
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}
