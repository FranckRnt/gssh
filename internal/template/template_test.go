package template

import (
	"testing"
)

func TestNeedsExpansion(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		want     bool
	}{
		{"no templates", []string{"uptime", "df -h"}, false},
		{"with template", []string{"echo {{.Hostname}}"}, true},
		{"mixed", []string{"uptime", "echo {{.Port}}"}, true},
		{"empty", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsExpansion(tt.commands); got != tt.want {
				t.Errorf("NeedsExpansion(%v) = %v, want %v", tt.commands, got, tt.want)
			}
		})
	}
}

func TestExpandCommands(t *testing.T) {
	data := ServerData{
		Hostname: "web01.example.com",
		Host:     "web01.example.com:2222",
		IP:       "web01.example.com",
		Port:     "2222",
		Tags:     []string{"web", "prod"},
		TagsCSV:  "web,prod",
	}

	tests := []struct {
		name     string
		commands []string
		want     []string
	}{
		{
			"no templates",
			[]string{"uptime"},
			[]string{"uptime"},
		},
		{
			"hostname",
			[]string{"echo {{.Hostname}}"},
			[]string{"echo web01.example.com"},
		},
		{
			"port",
			[]string{"nc -zv localhost {{.Port}}"},
			[]string{"nc -zv localhost 2222"},
		},
		{
			"tags csv",
			[]string{"echo {{.TagsCSV}}"},
			[]string{"echo web,prod"},
		},
		{
			"multiple vars",
			[]string{"echo {{.Hostname}}:{{.Port}} tags={{.TagsCSV}}"},
			[]string{"echo web01.example.com:2222 tags=web,prod"},
		},
		{
			"mixed commands",
			[]string{"uptime", "echo {{.Hostname}}"},
			[]string{"uptime", "echo web01.example.com"},
		},
		{
			"host field",
			[]string{"echo {{.Host}}"},
			[]string{"echo web01.example.com:2222"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandCommands(tt.commands, data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d commands, want %d", len(got), len(tt.want))
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("command[%d] = %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}

func TestExpandCommands_InvalidTemplate(t *testing.T) {
	_, err := ExpandCommands([]string{"echo {{.Invalid"}, ServerData{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestExpandCommands_MissingField(t *testing.T) {
	// Go templates treat missing fields as zero values by default with struct,
	// but accessing a nonexistent field on a struct is an error.
	_, err := ExpandCommands([]string{"echo {{.Nonexistent}}"}, ServerData{})
	if err == nil {
		t.Fatal("expected error for nonexistent field")
	}
}
