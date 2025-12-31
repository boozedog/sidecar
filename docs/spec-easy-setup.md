# Plan: Unified Setup & Update System for td and sidecar

## Goal

Create a failsafe way for users with basic terminal familiarity to install, configure, and keep both td and sidecar updated.

## Summary

1. **Gum-based setup script** - Interactive installer that handles Go, PATH, and both tools
2. **Unified version checking** - sidecar checks for updates to both td and sidecar
3. **Documentation** - GETTING_STARTED.md for open source users

---

## Part 1: Gum-Based Setup Script

### New File: `scripts/setup.sh` (sidecar repo only)

Hosted at: `https://raw.githubusercontent.com/sst/sidecar/main/scripts/setup.sh`

Interactive shell script that:

1. **Bootstraps gum** - Installs gum if missing (via brew or curl binary)
2. **Detects Go** - Checks if Go 1.21+ is installed
   - If missing: offers to install via brew, apt, or shows manual instructions
3. **Checks PATH** - Verifies `~/go/bin` is in PATH
   - If missing: offers to add to shell config (~/.zshrc, ~/.bashrc, etc.)
4. **Detects existing installs** - Checks current versions of td and sidecar
5. **Checks for updates** - Queries GitHub API for latest releases
6. **Installs/Updates** - Runs `go install` with proper ldflags (td first, then sidecar)
7. **Verifies** - Confirms both tools work

Key features:

- **Idempotent** - Safe to run multiple times
- **Dependency order** - Always updates td before sidecar
- **Error handling** - Clear messages on failure, recovery suggestions
- **Cross-platform** - Works on macOS and Linux

---

## Part 2: Unified Version Checking in sidecar

### Changes Required

**1. Pass td version to embedded monitor**

File: `internal/plugins/tdmonitor/plugin.go:56`

```go
// Currently: model, err := monitor.NewEmbedded(ctx.WorkDir, pollInterval, "")
// Change to: Get td version and pass it
model, err := monitor.NewEmbedded(ctx.WorkDir, pollInterval, tdVersion())
```

Add helper to get td version from binary:

```go
func tdVersion() string {
    out, err := exec.Command("td", "version", "--short").Output()
    if err != nil { return "" }
    return strings.TrimSpace(string(out))
}
```

**2. Add TdUpdateInfo type**

File: `internal/version/checker.go`

```go
type TdUpdateInfo struct {
    CurrentVersion string
    LatestVersion  string
    UpdateCommand  string
}
```

**3. Add CheckTdAsync function**

File: `internal/version/checker.go`

- New function to check td's repo (marcus/td) for updates
- Uses same cache pattern but separate cache file (~/.config/sidecar/td_version_cache.json)

**4. Add td update tracking to app Model**

File: `internal/app/model.go:52`

```go
// Add field:
tdUpdateAvailable *version.TdUpdateInfo
```

**5. Check td version in Init()**

File: `internal/app/model.go:79`

- Add `version.CheckTdAsync(tdVersion)` to cmds batch

**6. Handle TdUpdateAvailableMsg**

File: `internal/app/update.go`

- Store in `m.tdUpdateAvailable`
- Combined toast: "Updates: td v0.2.3, sidecar v0.1.6. Press ! for details"

**7. Display both versions in diagnostics modal**

File: `internal/app/view.go` (buildDiagnosticsContent)

```
Version
  sidecar: v0.1.5  → v0.1.6  update available
  td:      v0.2.1  → v0.2.3  update available

Update commands:
  1. go install -ldflags "-X main.Version=v0.2.3" github.com/marcus/td@v0.2.3
  2. go install -ldflags "-X main.Version=v0.1.6" github.com/sst/sidecar/cmd/sidecar@v0.1.6

Or run: curl -fsSL https://raw.githubusercontent.com/sst/sidecar/main/scripts/setup.sh | bash
```

---

## Part 3: Documentation

### New File: `docs/GETTING_STARTED.md`

```markdown
# Getting Started with Sidecar + td

## Quick Install (Recommended)

curl -fsSL https://raw.githubusercontent.com/sst/sidecar/main/scripts/setup.sh | bash

## Prerequisites

- macOS or Linux
- Terminal access

## What the Setup Script Does

1. Installs Go (via Homebrew) if missing
2. Adds ~/go/bin to your PATH
3. Installs td and sidecar
4. Verifies installation

## Updating

Run the same setup script - it detects installed versions and updates as needed.

## Manual Installation

[detailed steps for those who prefer manual setup]
```

### Update `README.md`

Add "Quick Install" section at top pointing to setup script.

---

## Files to Create/Modify

| File                                   | Action                                     |
| -------------------------------------- | ------------------------------------------ |
| `scripts/setup.sh`                     | Create - gum-based interactive installer   |
| `internal/version/checker.go`          | Modify - add TdUpdateInfo, CheckTdAsync    |
| `internal/app/model.go`                | Modify - add tdUpdateAvailable field       |
| `internal/app/update.go`               | Modify - handle TdUpdateAvailableMsg       |
| `internal/app/view.go`                 | Modify - show both versions in diagnostics |
| `internal/plugins/tdmonitor/plugin.go` | Modify - pass td version to monitor        |
| `docs/GETTING_STARTED.md`              | Create - user documentation                |
| `README.md`                            | Modify - add quick install section         |

---

## Implementation Order

1. **Setup script** (`scripts/setup.sh`) - standalone, can be tested immediately
2. **Version checking** - add TdUpdateInfo and CheckTdAsync to version package
3. **App integration** - wire up td version checking to app model
4. **Diagnostics display** - update view to show both versions
5. **Documentation** - GETTING_STARTED.md and README updates

---

## Testing Scenarios

- Fresh install (no Go)
- Fresh install (Go present, no PATH)
- Fresh install (Go ready)
- Update td only
- Update sidecar only
- Update both
- Offline/network timeout
- macOS and Linux
