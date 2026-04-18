package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath returns ~/.gssh/config.yaml if it exists, empty string otherwise.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".gssh", "config.yaml")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// File represents the YAML configuration file structure.
type File struct {
	User       string `yaml:"user"`
	KeyPath    string `yaml:"key"`
	Port       string `yaml:"port"`
	Timeout    string `yaml:"timeout"`
	MaxWorkers int    `yaml:"workers"`
	Retries    int    `yaml:"retries"`
	KnownHosts string `yaml:"known_hosts"`
	Verbose    bool   `yaml:"verbose"`
	Output     string `yaml:"output"`
	LogDir     string `yaml:"log_dir"`
}

// DefaultLogDir returns the default log directory (~/.gssh/logs/).
func DefaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gssh", "logs")
}

// Load reads and parses a YAML config file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg File
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// ParseTimeout parses the timeout string from config (e.g. "30s", "1m").
func (f *File) ParseTimeout() (time.Duration, error) {
	if f.Timeout == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(f.Timeout)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: %w", f.Timeout, err)
	}
	return d, nil
}
