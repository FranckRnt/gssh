package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
user: deploy
key: /home/deploy/.ssh/id_ed25519
port: "2222"
timeout: "1m30s"
workers: 50
retries: 2
known_hosts: /etc/ssh/known_hosts
verbose: true
output: json
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.User != "deploy" {
		t.Errorf("user = %q, want 'deploy'", cfg.User)
	}
	if cfg.KeyPath != "/home/deploy/.ssh/id_ed25519" {
		t.Errorf("key = %q", cfg.KeyPath)
	}
	if cfg.Port != "2222" {
		t.Errorf("port = %q, want '2222'", cfg.Port)
	}
	if cfg.MaxWorkers != 50 {
		t.Errorf("workers = %d, want 50", cfg.MaxWorkers)
	}
	if cfg.Retries != 2 {
		t.Errorf("retries = %d, want 2", cfg.Retries)
	}
	if !cfg.Verbose {
		t.Error("expected verbose=true")
	}
	if cfg.Output != "json" {
		t.Errorf("output = %q, want 'json'", cfg.Output)
	}

	d, err := cfg.ParseTimeout()
	if err != nil {
		t.Fatalf("parse timeout: %v", err)
	}
	if d != 90*time.Second {
		t.Errorf("timeout = %v, want 1m30s", d)
	}
}

func TestLoad_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(":::invalid"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}
