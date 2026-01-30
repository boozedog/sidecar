# Getting Started with Sidecar

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash
```

Or install via Homebrew:

```bash
brew install marcus/tap/sidecar
```

The script will ask what you want to install:
- **Both td and sidecar** (recommended) - td provides task management for AI workflows
- **sidecar only** - works standalone without td

## Prerequisites

- macOS, Linux, or Windows (WSL)
- Terminal access
- Go 1.21+ (only if building from source — Homebrew and binary installs don't require Go)

## What the Setup Script Does

1. Shows you the current status of Go, td, and sidecar
2. Asks what you want to install
3. Shows every command before running it (you approve each one)
4. Installs Go if missing
5. Configures PATH
6. Installs your selected tools
7. Verifies installation

## Updating

Run the same command - the script detects installed versions and only updates what's needed.

```bash
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash
```

## Headless/CI Installation

```bash
# Install both (default)
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash -s -- --yes

# Install sidecar only
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash -s -- --yes --sidecar-only

# Force reinstall even if up-to-date
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash -s -- --yes --force
```

## Binary Download

Download pre-built binaries directly from [GitHub Releases](https://github.com/marcus/sidecar/releases). Available for macOS and Linux (amd64 and arm64).

1. Download the archive for your platform
2. Extract: `tar -xzf sidecar_*.tar.gz`
3. Move to PATH: `mv sidecar /usr/local/bin/` (or `~/go/bin/`)

## Manual Installation

If you prefer to install manually:

### 1. Install Go

```bash
# macOS
brew install go

# Ubuntu/Debian
sudo apt install golang

# Other
# Download from https://go.dev/dl/
```

### 2. Configure PATH

Add to ~/.zshrc or ~/.bashrc:

```bash
export PATH="$HOME/go/bin:$PATH"
```

### 3. Install sidecar

```bash
go install github.com/marcus/sidecar/cmd/sidecar@latest
```

### 4. (Optional) Install td

```bash
go install github.com/marcus/td@latest
```

## Quick Start

After installation, run from any project directory:

```bash
sidecar
```

**Tip:** You can run two sidecar instances side-by-side (e.g. in a split terminal) to create a dashboard view. This allows you to monitor tasks (TD Monitor) in one pane while reviewing code changes (Git Status) or managing parallel work (Workspaces) in another.

## Checking for Updates

In sidecar, press `!` to open diagnostics. You'll see version info for installed tools and update commands if updates are available.

## Troubleshooting

### "command not found: sidecar"

Your PATH may not include ~/go/bin. Run:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

### "permission denied"

Fix ownership of Go directory:

```bash
sudo chown -R $USER ~/go
```

### Network issues

The setup script requires internet access to download from GitHub. If behind a proxy, set HTTPS_PROXY environment variable.

### Go version too old

The setup script requires Go 1.21+. Update Go:

```bash
# macOS
brew upgrade go

# Linux - download latest from https://go.dev/dl/
```
