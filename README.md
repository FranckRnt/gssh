# gssh — SSH at scale

Run SSH commands in parallel across multiple servers. Fast and lightweight, written in Go.

## Features

- ⚡ Parallel execution with configurable worker pool
- 🔗 Multiple commands over a single SSH connection per server (`-c cmd1 -c cmd2`)
- 📄 Load commands from a script file (`-f`)
- 📦 **File transfer**: upload (`push`) and download (`pull`) via SFTP
- 🏰 **Bastion/jump host**: `-J user@host[:port]` flag to tunnel through an SSH proxy
- 🏷️ **Tags and filtering**: tag servers with `#tag` and filter with `-g`
- 🔄 **Dynamic inventory**: server source from a file, an executable script, or a URL
- 🔐 **Sudo**: `-S` flag to run commands via `sudo` (password prompted once)
- 🧩 **Command templates**: `{{.Hostname}}`, `{{.Port}}`, `{{.Tags}}`, etc. variables in commands
- 📡 **Real-time streaming**: `-s` flag to display output line by line as it arrives (`tail -f` style)
- 🔍 **Diff mode**: `--diff` flag to compare outputs across servers and detect drift
- 📊 **Grouped output**: `--group` flag to group servers by identical output
- 📝 **HTML report**: `--report html` flag to generate a static HTML report with sortable table, dark/light mode
- 🔁 Automatic retry of failed servers
- 📈 Real-time progress bar
- 🎨 Colored text or JSON output
- 🔑 Automatic SSH key detection (ed25519, ecdsa, rsa)
- 🛡️ SSH agent support and passphrase-protected keys
- 🌐 Upfront DNS resolution (cache)
- 💾 Memory protection (captured output limited to 1 MB per stream)
- ⚙️ Configuration file `~/.gssh/config.yaml`
- 🐚 Shell completion (bash, zsh, fish)
- ✅ Host key verification via `known_hosts`

## Installation

### From GitHub releases

Download the binary for your OS from the [Releases](https://github.com/FranckRnt/gssh/releases) page.

### From source

```bash
go install github.com/FranckRnt/gssh/cmd/gssh@latest
```

Or clone and build:

```bash
git clone git@github.com:FranckRnt/gssh.git
cd gssh
go build -o gssh ./cmd/gssh/
```

## Commands

gssh uses subcommands:

| Command | Description |
|---------|-------------|
| `gssh run` | Execute SSH commands (default, `run` is optional) |
| `gssh push` | Upload a file to N servers via SFTP |
| `gssh pull` | Download a file from N servers via SFTP |

## Quick start

### Running commands

```bash
# Run a command on all servers
gssh -l servers.txt -u root -c "uptime"

# Equivalent with explicit subcommand
gssh run -l servers.txt -u root -c "uptime"

# Multiple commands (reuses the SSH connection)
gssh -l servers.txt -u deploy -c "uptime" -c "df -h" -c "free -m"

# Load commands from a file
gssh -l servers.txt -u root -f commands.txt

# Dry-run mode: see targeted servers without executing anything
gssh -l servers.txt -u root -c "uptime" -n
```

### Bastion / jump host

```bash
# Execute through a bastion
gssh -l servers.txt -u root -c "uptime" -J admin@bastion.example.com

# Bastion with custom port
gssh -l servers.txt -u root -c "uptime" -J admin@bastion.example.com:2222

# Upload through bastion
gssh push -l servers.txt -u root -s ./config.yml -d /etc/app/config.yml \
     -J admin@bastion.example.com

# Download through bastion
gssh pull -l servers.txt -u root -s /var/log/app.log -d ./logs/ \
     -J admin@bastion.example.com
```

The bastion uses the same authentication methods (SSH agent, private key) for the initial connection. For target servers, gssh automatically fetches the bastion's private key and uses it for authentication — no need to have your local key authorized on targets.

### Sudo

```bash
# Run a command with sudo (password prompted once)
gssh -l servers.txt -u deploy -c "apt update" -S

# Sudo + multiple commands
gssh -l servers.txt -u deploy -c "apt update" -c "apt upgrade -y" -S

# Sudo + bastion
gssh -l servers.txt -u deploy -c "systemctl restart nginx" -S -J admin@bastion
```

The password is entered securely (no echo) and sent to each command via `sudo -S`.

### Command templates

Commands can contain Go template variables that are resolved per server:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Hostname}}` | Hostname (without port) | `web01.example.com` |
| `{{.Host}}` | Full entry (with port if present) | `web01.example.com:2222` |
| `{{.IP}}` | Alias for Hostname | `web01.example.com` |
| `{{.Port}}` | SSH port | `2222` |
| `{{.Tags}}` | Tag array | `[web prod]` |
| `{{.TagsCSV}}` | Comma-separated tags | `web,prod` |

```bash
# Create a file with the server name
gssh -l servers.txt -u root -c "echo {{.Hostname}} > /etc/hostname"

# Configure based on tags
gssh -l servers.txt -u root -c "echo 'role={{.TagsCSV}}' >> /etc/environment"

# Use port in a command
gssh -l servers.txt -u root -c "echo 'SSH port: {{.Port}}'"
```

Templates are only evaluated if at least one command contains `{{`. If no command uses templates, there is zero overhead.

### File transfer

```bash
# Upload a file to all servers
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf

# Upload with custom port and key
gssh push -l servers.txt -u deploy -s ./app.tar.gz -d /opt/app.tar.gz \
     -p 2222 -k ~/.ssh/deploy_key

# Download a file from all servers
# Files are saved to ./logs/<hostname>/syslog
gssh pull -l servers.txt -u root -s /var/log/syslog -d ./logs/

# Download nginx configs
gssh pull -l servers.txt -u deploy -s /etc/nginx/nginx.conf -d ./configs/
```

## Server file

A text file with one server per line. Empty lines and comments (`#` at the beginning of a line) are ignored. The port can be specified with `:port`. Tags are optional, prefixed with `#` after the hostname.

```
# servers.txt
web01.example.com #web #prod
web02.example.com:2222 #web #staging
10.0.1.50 #db #prod
db01.example.com #db #prod #paris
plain-host
```

> The file must not be world-writable (permissions `0644` or more restrictive).

### Tag filtering

```bash
# Target only web servers
gssh -l servers.txt -u root -c "uptime" -g web

# Target web AND prod servers (intersection)
gssh -l servers.txt -u root -c "uptime" -g web,prod

# Also works with push/pull
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf -g web
```

The `-g` flag takes a comma-separated list of tags. Servers must have **all** specified tags to be selected.

### Dynamic inventory

Instead of a static file, the server source (`-l`) can be:

**An executable script** — gssh detects files with the executable bit and runs the script. The stdout output is parsed in the same format (one server per line, with optional tags).

```bash
# inventory.sh must be executable (chmod +x)
gssh -l ./inventory.sh -u root -c "uptime"

# Example script
#!/bin/bash
echo "web01.example.com #web #prod"
echo "web02.example.com #web #staging"
# Can query an API, database, AWS, etc.
```

**An HTTP/HTTPS URL** — gssh fetches the URL and parses the body in the same format.

```bash
gssh -l https://inventory.example.com/servers -u root -c "uptime"
gssh -l https://inventory.example.com/servers -u root -c "uptime" -g prod
```

## Options

### `run` command (or without subcommand)

| Flag | Description | Default |
|------|-------------|---------|
| `-l <file>` | Server list file **(required)** | — |
| `-u <user>` | SSH user **(required)** | — |
| `-c <command>` | Command to execute (repeatable) **(required, exclusive with `-f`)** | — |
| `-f <file>` | Command file, one per line **(required, exclusive with `-c`)** | — |
| `-k <path>` | SSH private key | auto-detection |
| `-p <port>` | Default SSH port | `22` |
| `-t <duration>` | Timeout per server | `30s` |
| `-w <count>` | Max concurrent SSH workers | `100` |
| `-r <count>` | Retry attempts on failure | `0` |
| `-v` | Real-time output (verbose) | `false` |
| `-n` | Dry run | `false` |
| `-o <format>` | Output format: `text` or `json` | `text` |
| `-known-hosts <path>` | known_hosts file | `~/.ssh/known_hosts` |
| `-insecure` | Disable host key verification | `false` |
| `-J <user@host[:port]>` | Bastion/jump host to tunnel connections | — |
| `-g <tags>` | Filter by tags (comma-separated, intersection) | — |
| `-S` | Run commands via sudo (prompts password once) | `false` |
| `-L <path>` | Directory for JSON log files | `~/.gssh/logs/` |
| `-s` | Real-time streaming, line by line (implies `-v`) | `false` |
| `--diff` | Compare outputs across servers after execution | `false` |
| `--group` | Group servers by identical output | `false` |
| `--report <format>` | Generate a report: `html` | — |

> **Note**: `-c` and `-f` are mutually exclusive. Use one or the other, not both.

### `push` and `pull` commands

| Flag | Description | Default |
|------|-------------|---------|
| `-l <file>` | Server list file **(required)** | — |
| `-u <user>` | SSH user **(required)** | — |
| `-s <path>` | Source path **(required)** | — |
| `-d <path>` | Destination path **(required)** | — |
| `-k <path>` | SSH private key | auto-detection |
| `-p <port>` | Default SSH port | `22` |
| `-w <count>` | Max concurrent SSH workers | `100` |
| `-v` | Verbose output | `false` |
| `-n` | Dry run | `false` |
| `-o <format>` | Output format: `text` or `json` | `text` |
| `-known-hosts <path>` | known_hosts file | `~/.ssh/known_hosts` |
| `-insecure` | Disable host key verification | `false` |
| `-J <user@host[:port]>` | Bastion/jump host to tunnel connections | — |
| `-g <tags>` | Filter by tags (comma-separated, intersection) | — |
| `-L <path>` | Directory for JSON log files | `~/.gssh/logs/` |

For `push`, `-s` is the local path and `-d` is the remote path. If `-d` is an existing directory on the server (or ends with `/`), the source filename is appended automatically (e.g., `-s ./nginx.conf -d /etc/nginx/` → `/etc/nginx/nginx.conf`).
For `pull`, `-s` is the remote path and `-d` is the local output directory (files are saved to `<dest>/<hostname>/<filename>`).

## Examples

### Verbose output with retries and JSON format

```bash
gssh -l servers.txt -u root -c "systemctl status nginx" -v -r 3 -o json
```

### Custom SSH key, port, and timeout

```bash
gssh -l servers.txt -u deploy -c "systemctl restart app" \
     -k ~/.ssh/deploy_key -p 2222 -t 60s -w 50
```

### Command file

```bash
# commands.txt
uptime
df -h
free -m
systemctl status nginx
```

```bash
gssh -l servers.txt -u root -f commands.txt
```

Commands are executed in order on each server. If a command fails, subsequent commands are skipped for that server.

### Upload a configuration file

```bash
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf
```

### Collect logs from all servers

```bash
gssh pull -l servers.txt -u root -s /var/log/syslog -d ./collected-logs/
# Result: ./collected-logs/web01/syslog, ./collected-logs/web02/syslog, etc.
```

## Output modes

### Normal mode (default)

One compact line per server with the status of each command. Details (error, stderr) are only shown for failed commands.

```
192.168.1.12  hostname OK | uptime OK | yum check-update FAIL (exit=100)
    error: [yum check-update] exit status 100
192.168.1.13  hostname OK | uptime OK | yum check-update OK

── Summary ──────────────────────────
  Total:    6
  Success:  5
  Failed:   1
  Duration: 1.5s
```

For transfers:

```
web01  push OK  4.2 KiB  230ms
web02  push OK  4.2 KiB  245ms
web03  push FAIL  connection refused  50ms

── Transfer Summary ─────────────────
  Total:      3
  Success:    2
  Failed:     1
  Bytes:      8.4 KiB
  Duration:   280ms
```

### Verbose mode (`-v`)

Full output of each command in real time, with stdout and stderr displayed in full.

```bash
gssh -l servers.txt -u root -c "uptime" -c "df -h" -v
```

### Streaming mode (`-s`)

Displays output line by line as execution progresses, like `tail -f`. Each line is prefixed with the hostname. Useful for long-running commands (`apt upgrade`, `docker pull`).

```bash
gssh -l servers.txt -u root -c "apt update && apt upgrade -y" -s
```

```
web01 Hit:1 http://deb.debian.org/debian bookworm InRelease
web02 Hit:1 http://deb.debian.org/debian bookworm InRelease
web01 Reading package lists...
web02 err| W: Some warning here
```

Stderr lines are marked with the `err|` prefix in yellow.

### Diff mode (`--diff`)

Compares the output of each command across all servers. Useful for detecting configuration drift.

```bash
gssh -l servers.txt -u root -c "cat /etc/hostname" --diff
```

```
── Diff: cat /etc/hostname
  ⚠ 2 different outputs across 5 server(s)

  MAJORITY OK (3 server(s)) [web01, web02, web03]
    myhost

  GROUP OK (2 server(s)) [db01, db02]
    dbhost
```

### Grouped mode (`--group`)

Groups servers by identical output instead of per-server display. Shows directly how many servers produced the same result.

```bash
gssh -l servers.txt -u root -c "systemctl is-active nginx" --group
```

```
▸ OK  42 server(s): [web01, web02, web03, ... +39 more]
  active

▸ FAIL (exit=3)  3 server(s): [db01, db02, db03]
  inactive
```

### HTML report (`--report html`)

Generates a self-contained static HTML report with a sortable table, colors, and a visual summary. No external dependencies. The report includes a **dark/light mode** toggle (automatic system preference detection, persisted via `localStorage`).

```bash
gssh -l servers.txt -u root -c "uptime" -c "df -h" --report html
```

The report is written to the log directory (`~/.gssh/logs/` by default, or `-L`).

### JSON format (`-o json`)

The JSON log file is always written. With `-o json`, the summary is also in JSON.

## Configuration file

Create `~/.gssh/config.yaml` to set default values. Command-line flags always take priority.

```yaml
# ~/.gssh/config.yaml
user: deploy
key: ~/.ssh/deploy_key
port: "22"
timeout: "45s"
workers: 50
retries: 1
known_hosts: ~/.ssh/known_hosts
verbose: false
output: text
log_dir: ~/.gssh/logs/
```

## Shell completion

```bash
# Bash (add to ~/.bashrc)
eval "$(gssh --completion=bash)"

# Zsh (add to ~/.zshrc)
eval "$(gssh --completion=zsh)"

# Fish
gssh --completion=fish | source
```

## SSH authentication

gssh attempts authentication in this order:

1. **SSH agent** (`SSH_AUTH_SOCK`) — if the agent is active and contains keys
2. **Private key file** — specified with `-k` or auto-detected in `~/.ssh/` (ed25519, ecdsa, rsa)

If the key is protected by a passphrase, gssh prompts for it interactively.

## Log file

After each execution, gssh writes a JSON file `gssh-YYYY-MM-DD_HH-MM-SS.log` to the log directory. By default, this directory is `~/.gssh/logs/`. It can be changed via the `-L` flag or the `log_dir` key in `~/.gssh/config.yaml`. The directory is created automatically if it does not exist.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All servers succeeded |
| `1` | Configuration or initialization error |
| `2` | At least one server failed |

## License

MIT
