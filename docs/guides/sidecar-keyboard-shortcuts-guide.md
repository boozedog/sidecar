# Sidecar Keyboard Shortcuts Guide

How to implement keyboard shortcuts for plugins.

## Quick Start: Adding a New Shortcut

**Three things must match for a shortcut to work:**

1. **Command ID** in `Commands()` → e.g., `"stage-file"`
2. **Binding command** in `bindings.go` → e.g., `Command: "stage-file"`
3. **Context** in both places → e.g., `"git-status"`

```go
// 1. In your plugin's Commands() method:
func (p *Plugin) Commands() []plugin.Command {
    return []plugin.Command{
        {ID: "stage-file", Name: "Stage", Context: "git-status"},
    }
}

// 2. In your plugin's FocusContext() method:
func (p *Plugin) FocusContext() string {
    return "git-status"  // Must match the Context above
}

// 3. In internal/keymap/bindings.go:
{Key: "s", Command: "stage-file", Context: "git-status"},
```

## How It Works

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  FocusContext() │───▶│   bindings.go   │───▶│   Commands()    │
│  returns context│    │  key→command    │    │  shows in footer│
└─────────────────┘    └─────────────────┘    └─────────────────┘
        │                      │                      │
        │                      │                      │
        ▼                      ▼                      ▼
   "git-status"          Key: "s"              Name: "Stage"
                         Command: "stage-file"  ID: "stage-file"
                         Context: "git-status"  Context: "git-status"
```

1. User presses a key
2. App calls `FocusContext()` on active plugin → gets `"git-status"`
3. App looks up key in `bindings.go` for that context
4. App finds `{Key: "s", Command: "stage-file", Context: "git-status"}`
5. App dispatches the command to the plugin
6. Footer shows commands from `Commands()` that match the current context

## Critical Rule: Plugins Must NOT Render Footers

The app renders a unified footer bar using `Commands()`. Plugins must:
- **Never render their own footer/hint line in `View()`**
- Define commands with short names in `Commands()` method

Keep command names short (1 word preferred) to prevent footer wrapping:
- ✅ `"Stage"` not `"Stage file"`
- ✅ `"Diff"` not `"Show diff"`
- ✅ `"History"` not `"Show history"`

## Complete Example: Adding "edit" to File Browser

### Step 1: Add to Commands()

```go
// internal/plugins/filebrowser/plugin.go
func (p *Plugin) Commands() []plugin.Command {
    return []plugin.Command{
        {ID: "refresh", Name: "Refresh", Context: "file-browser-tree"},
        {ID: "expand", Name: "Open", Context: "file-browser-tree"},
        {ID: "edit", Name: "Edit", Context: "file-browser-tree"},  // NEW
    }
}
```

### Step 2: Add to bindings.go

```go
// internal/keymap/bindings.go
{Key: "e", Command: "edit", Context: "file-browser-tree"},
```

### Step 3: Handle in Update()

```go
// internal/plugins/filebrowser/plugin.go
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
    switch msg := msg.(type) {
    case keymap.CommandMsg:
        switch msg.Command {
        case "edit":
            return p, p.openInEditor()
        }
    }
    return p, nil
}
```

## Multiple Contexts (View Modes)

When your plugin has different modes, use different contexts:

```go
func (p *Plugin) FocusContext() string {
    switch p.viewMode {
    case ViewDiff:
        return "git-diff"      // Different bindings active
    case ViewCommit:
        return "git-commit"    // Different bindings active
    default:
        return "git-status"    // Default bindings
    }
}

func (p *Plugin) Commands() []plugin.Command {
    return []plugin.Command{
        // Main view commands
        {ID: "stage-file", Name: "Stage", Context: "git-status"},
        {ID: "show-diff", Name: "Diff", Context: "git-status"},

        // Diff view commands
        {ID: "close-diff", Name: "Close", Context: "git-diff"},
        {ID: "scroll", Name: "Scroll", Context: "git-diff"},

        // Commit view commands
        {ID: "cancel", Name: "Cancel", Context: "git-commit"},
        {ID: "execute-commit", Name: "Commit", Context: "git-commit"},
    }
}
```

## Core Files

| File | Purpose |
|------|---------|
| `internal/plugin/plugin.go` | `Command` struct, `Commands()`, `FocusContext()` interface |
| `internal/keymap/bindings.go` | Default key→command mappings |
| `internal/keymap/registry.go` | Runtime binding lookup |
| `internal/app/view.go` | Footer rendering from `Commands()` |

## Common Mistakes

| Symptom | Cause | Fix |
|---------|-------|-----|
| Shortcut doesn't work | Command ID mismatch | Ensure ID in `Commands()` matches `Command` in `bindings.go` |
| Shortcut doesn't work | Context mismatch | Ensure `FocusContext()` returns same context as binding |
| Double footer | Plugin renders own footer | Remove footer rendering from plugin's `View()` |
| Wrong hints shown | `FocusContext()` not updated | Return correct context for current view mode |
| Footer too long | Command names too verbose | Use 1-word names: "Stage" not "Stage file" |

## Checklist for New Shortcuts

- [ ] Command added to `Commands()` with ID, Name, Context
- [ ] `FocusContext()` returns matching context for current view
- [ ] Binding added to `bindings.go` with Key, Command, Context
- [ ] Key handled in `Update()` via `keymap.CommandMsg`
- [ ] No duplicate/conflicting keys in same context
- [ ] Command name is short (1-2 words max)
- [ ] Plugin does NOT render its own footer

## Key Format Reference

```go
// Letters (lowercase)
{Key: "j", Command: "cursor-down", Context: "global"}

// Shifted letters (uppercase)
{Key: "G", Command: "cursor-bottom", Context: "global"}

// Control combos
{Key: "ctrl+d", Command: "page-down", Context: "global"}
{Key: "ctrl+c", Command: "quit", Context: "global"}

// Alt combos
{Key: "alt+enter", Command: "execute-commit", Context: "git-commit"}

// Special keys
{Key: "enter", Command: "select", Context: "global"}
{Key: "esc", Command: "back", Context: "global"}
{Key: "tab", Command: "next-plugin", Context: "global"}
{Key: "shift+tab", Command: "prev-plugin", Context: "global"}
{Key: "up", Command: "cursor-up", Context: "global"}
{Key: "down", Command: "cursor-down", Context: "global"}

// Sequences (space-separated, 500ms timeout)
{Key: "g g", Command: "cursor-top", Context: "global"}
```

## Context Precedence

1. Plugin-specific context checked first (e.g., `git-status`)
2. Falls back to `global` context if no match

This means `c` in `git-status` context triggers `commit`, but `c` in `global` context triggers `focus-conversations`.

## Testing

1. Run `sidecar --debug` to see key handling logs
2. Press `?` to verify help overlay shows your bindings
3. Check footer shows your command names
4. Test that keys trigger correct actions
5. Test context switches (enter subview, verify new bindings active)
