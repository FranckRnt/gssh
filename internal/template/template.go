package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// ServerData holds the per-server variables available in command templates.
type ServerData struct {
	Hostname string   // hostname without port (e.g. "web01.example.com")
	Host     string   // original host entry (e.g. "web01.example.com:2222")
	IP       string   // same as Hostname (resolved at template time, alias for convenience)
	Port     string   // port number (e.g. "2222")
	Tags     []string // tags from inventory (e.g. ["web", "prod"])
	TagsCSV  string   // comma-separated tags (e.g. "web,prod")
}

// NeedsExpansion returns true if any command contains template syntax.
func NeedsExpansion(commands []string) bool {
	for _, c := range commands {
		if strings.Contains(c, "{{") {
			return true
		}
	}
	return false
}

// ExpandCommands expands Go templates in each command string using the given server data.
// If no templates are found, commands are returned as-is with no allocation.
func ExpandCommands(commands []string, data ServerData) ([]string, error) {
	if !NeedsExpansion(commands) {
		return commands, nil
	}

	expanded := make([]string, len(commands))
	for i, cmd := range commands {
		tmpl, err := template.New("cmd").Parse(cmd)
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", cmd, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("execute template %q: %w", cmd, err)
		}
		expanded[i] = buf.String()
	}
	return expanded, nil
}
