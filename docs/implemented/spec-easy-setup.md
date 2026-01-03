# Plan: Unified Setup & Update System for td and sidecar

## Goal

Create a failsafe way for users with basic terminal familiarity to install, configure, and keep sidecar (and optionally td) updated.

## Core Principles

1. **Transparency** - Show every command before running it. Users should feel confident the script is not harming their system.
2. **td is optional** - sidecar works standalone. td is recommended but not required.

## Summary

1. **td version --short** - Add flag to td for clean version output (needed if td is installed)
2. **Gum-based setup script** - Interactive installer with plain fallback
3. **Unified version checking** - sidecar checks for updates to both td and sidecar
4. **Documentation** - GETTING_STARTED.md for open source users

---

## Part 0: Prerequisite - td version --short

### File: `~/code/td/cmd/system.go`

Add `--short` flag to the version command:
- Output only the version string (e.g., `v0.4.12`)
- Skip update check when --short is used
- No extra whitespace or newlines

```bash
$ td version --short
v0.4.12
```

This is required before the setup script can work, as it needs to parse td's version.

---

## Part 1: Gum-Based Setup Script

### New File: `scripts/setup.sh` (sidecar repo only)

Hosted at: `https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh`

### Platform Support

- macOS (primary)
- Linux (primary)
- WSL (experimental - show warning)

```bash
detect_platform() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux)
      if grep -qi microsoft /proc/version 2>/dev/null; then
        echo "wsl"
      else
        echo "linux"
      fi ;;
    *) echo "unsupported" ;;
  esac
}
```

### Script Flow

1. **Bootstraps gum** - Installs gum if missing (via brew or curl binary)
   - **Plain fallback**: If gum install fails, continue with read -p and echo
   - All prompts must work in both gum and plain mode

2. **Shows status table** - Display current state before any prompts:
   ```
   ┌─────────────────────────────────────┐
   │ Current Status                      │
   ├─────────────────────────────────────┤
   │ Go:      ✓ 1.25.5                   │
   │ td:      ✓ v0.4.12 → v0.4.13 avail  │
   │ sidecar: ✗ not installed            │
   └─────────────────────────────────────┘
   ```

3. **Tool selection** - Ask what to install (td is optional):
   ```
   What would you like to install?

   > [Recommended] Both td and sidecar
     sidecar only
     td only
   ```

4. **Detects Go** - Checks if Go 1.21+ is installed
   - If missing, **show the command before running**:
     ```
     Go is required. Will run:
       brew install go

     [Run it] [Skip, I'll install manually]
     ```

5. **Checks PATH** - Verifies `~/go/bin` is in PATH
   - If missing: **preview the change** before writing:
     ```
     Will add to ~/.zshrc:
       export PATH="$HOME/go/bin:$PATH"

     [Add it] [Skip, I'll do it myself]
     ```
   - After adding, show: `Run: source ~/.zshrc`

6. **Compares versions** - Skip reinstall if already up-to-date
   ```bash
   if [[ "$LOCAL_VERSION" == "$LATEST_VERSION" ]]; then
     echo "sidecar is up to date ($LOCAL_VERSION)"
   else
     # SHOW command before running
     echo "Will run:"
     echo "  go install -ldflags \"-X main.Version=$LATEST\" github.com/marcus/sidecar/cmd/sidecar@$LATEST"
     # prompt, then run
   fi
   ```

7. **Installs/Updates** - Shows each command, waits for confirmation, then runs
   - If both selected: td first, then sidecar
   - If sidecar only: just sidecar

8. **Verifies** - Confirms installed tools work:
   ```
   ✓ sidecar v0.1.6
   ✓ td v0.4.13  (if installed)

   Run 'sidecar' in any project directory to start!
   ```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--yes`, `-y` | Skip all prompts (for CI/headless installs) - still shows commands |
| `--force`, `-f` | Reinstall even if versions are up-to-date |
| `--sidecar-only` | Install only sidecar, skip td |
| `--help`, `-h` | Show usage |

### Key Features

- **Transparent** - Shows every command before running it
- **td optional** - sidecar works standalone; td is recommended but not required
- **Idempotent** - Safe to run multiple times; skips work if up-to-date
- **Error handling** - Specific recovery messages (see table below)
- **Plain fallback** - Works without gum using read -p and echo

### Error Recovery

| Error | Recovery Message |
|-------|------------------|
| Network timeout | "Network error. Check connection and retry." |
| GitHub rate limit | "Rate limited. Set GITHUB_TOKEN or wait 1 hour." |
| Permission denied | "Permission error. Try: sudo chown -R $USER ~/go" |
| Go install fails | Show exact error and link to manual install docs |
| Shell config not writable | Show manual PATH instructions |

---

## Part 2: Unified Version Checking in sidecar

### Changes Required

**1. Add TdUpdateInfo type**

File: `internal/version/version.go`

```go
type TdUpdateInfo struct {
    CurrentVersion string
    LatestVersion  string
    UpdateCommand  string
}

type TdUpdateAvailableMsg struct {
    Info TdUpdateInfo
}
```

**2. Add CheckTdAsync function**

File: `internal/version/checker.go`

- New function to check td's repo (marcus/td) for updates
- Uses same cache pattern but separate cache file (~/.config/sidecar/td_version_cache.json)
- 6-hour cache TTL (same as sidecar)
- Skip check for dev versions
- Silent failure on network errors

**3. Add helper to get td version**

File: `internal/app/model.go` (or util file)

```go
func getTdVersion() string {
    out, err := exec.Command("td", "version", "--short").Output()
    if err != nil { return "" }
    return strings.TrimSpace(string(out))
}
```

**4. Add td update tracking to app Model**

File: `internal/app/model.go`

```go
// Add field:
tdUpdateAvailable *version.TdUpdateInfo
```

**5. Check td version in Init()**

File: `internal/app/model.go`

- Call `version.CheckTdAsync(getTdVersion())` in Init() batch

**6. Handle TdUpdateAvailableMsg**

File: `internal/app/update.go`

- Store in `m.tdUpdateAvailable`
- Toast options:
  - td only: "td update v0.4.13 available! Press ! for details"
  - both: "Updates: td v0.4.13, sidecar v0.1.6. Press ! for details"

**7. Display both versions in diagnostics modal**

File: `internal/app/view.go` (buildDiagnosticsContent)

Simplified format - single curl command instead of two go install commands:

```
Version
  sidecar: v0.1.5 → v0.1.6 available
  td:      v0.4.12 ✓

Update: curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash
```

When both up to date:
```
Version
  sidecar: v0.1.5 ✓
  td:      v0.4.12 ✓
```

When td not installed:
```
Version
  sidecar: v0.1.5 ✓
  td:      not installed
```

---

## Part 3: Documentation

### New File: `docs/GETTING_STARTED.md`

```markdown
# Getting Started with Sidecar

## Quick Install

curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash

The script will ask what you want to install:
- **Both td and sidecar** (recommended) - td provides task management for AI workflows
- **sidecar only** - works standalone without td

## Prerequisites

- macOS, Linux, or Windows (WSL)
- Terminal access

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

## Headless/CI Installation

# Install both (default)
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash -s -- --yes

# Install sidecar only
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash -s -- --yes --sidecar-only

## Manual Installation

If you prefer to install manually:

### 1. Install Go
macOS: brew install go
Ubuntu/Debian: sudo apt install golang

### 2. Configure PATH
Add to ~/.zshrc or ~/.bashrc:
export PATH="$HOME/go/bin:$PATH"

### 3. Install sidecar
go install github.com/marcus/sidecar/cmd/sidecar@latest

### 4. (Optional) Install td
go install github.com/marcus/td@latest

## Checking for Updates

In sidecar, press `!` to open diagnostics. You'll see version info for installed tools.

## Troubleshooting

### "command not found: sidecar"
Your PATH may not include ~/go/bin. Run:
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc

### "permission denied"
Fix ownership of Go directory:
sudo chown -R $USER ~/go

### Network issues
The setup script requires internet access to download from GitHub.
If behind a proxy, set HTTPS_PROXY environment variable.
```

### Update `README.md`

Add "Quick Install" section at top pointing to setup script:

```markdown
## Quick Install

curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash

Or see [Getting Started](docs/GETTING_STARTED.md) for manual installation.
```

---

## Files to Create/Modify

| File                                   | Action                                     |
| -------------------------------------- | ------------------------------------------ |
| `~/code/td/cmd/system.go`              | Modify - add --short flag to version cmd   |
| `scripts/setup.sh`                     | Create - gum-based interactive installer   |
| `internal/version/version.go`          | Modify - add TdUpdateInfo type             |
| `internal/version/checker.go`          | Modify - add CheckTdAsync function         |
| `internal/app/model.go`                | Modify - add tdUpdateAvailable field       |
| `internal/app/update.go`               | Modify - handle TdUpdateAvailableMsg       |
| `internal/app/view.go`                 | Modify - show both versions in diagnostics |
| `docs/GETTING_STARTED.md`              | Create - user documentation                |
| `README.md`                            | Modify - add quick install section         |

---

## Implementation Order

1. **td version --short** - Add flag to td repo (prerequisite for setup script)
2. **Setup script** (`scripts/setup.sh`) - Create installer with gum + plain fallback
3. **Version checking** - Add TdUpdateInfo and CheckTdAsync to version package
4. **App integration** - Wire up td version checking to app model
5. **Diagnostics display** - Update view to show both versions
6. **Documentation** - GETTING_STARTED.md and README updates

---

## Testing Scenarios

- Fresh install (no Go)
- Fresh install (Go present, no PATH)
- Fresh install (Go ready)
- Install sidecar only (user selects sidecar-only option)
- Install both td and sidecar (recommended option)
- Existing td user (td installed, sidecar missing)
- Existing sidecar user (sidecar installed, td missing)
- Update td only
- Update sidecar only
- Update both
- Both up to date (should be fast, skip reinstall)
- Offline/network timeout
- macOS
- Linux
- WSL (experimental)
- Headless/CI with --yes flag
- Headless with --yes --sidecar-only
- Force reinstall with --force flag
- Gum unavailable (plain mode fallback)
- User skips a command (selects "Skip" instead of "Run it")
