# Modal Overlay Implementation Guide

This guide covers how to implement modals with dimmed backgrounds in Sidecar.

## Overview

Modals should dim the background to:
- Draw user focus to the modal content
- Provide visual separation between modal and underlying content
- Create a consistent, polished UX across the application

## Two Approaches

### 1. Solid Black Overlay (Hides Background)

Use `lipgloss.Place()` with whitespace options when you want to **completely hide** the background:

```go
func (m Model) renderMyModal(content string) string {
    modal := styles.ModalBox.Render(content)

    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        modal,
        lipgloss.WithWhitespaceChars(" "),
        lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
    )
}
```

**How it works:**
- `lipgloss.Place()` centers the modal and fills surrounding space with spaces
- The spaces use the terminal's default background color
- Background content is **hidden**, not dimmed

**Note:** `WithWhitespaceForeground()` sets the foreground color of space characters, which are invisible. This does NOT create visible dimming.

**Examples:** None currently - all modals use dimmed background overlay.

### 2. Dimmed Background Overlay (Shows Background)

Use `ui.OverlayModal()` when you want to show **dimmed background content** behind the modal. This works for both app-level and plugin-level modals:

```go
// App-level modal (internal/app/view.go):
func (m Model) renderMyModal(background string) string {
    modal := styles.ModalBox.Render(content)
    return ui.OverlayModal(background, modal, m.width, m.height)
}

// Plugin-level modal:
func (p *Plugin) renderMyModal() string {
    background := p.renderNormalView()
    modalContent := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(styles.Primary).
        Padding(1, 2).
        Width(modalWidth).
        Render(content)
    return ui.OverlayModal(background, modalContent, p.width, p.height)
}
```

**How `ui.OverlayModal()` works:**
1. Calculates modal position (centered horizontally and vertically)
2. Strips ANSI codes from background and applies dim gray styling (color 242)
3. Composites each row: `dimmed-left + modal + dimmed-right`
4. Shows dimmed background on all four sides of the modal

Note: Background colors are not preserved because ANSI SGR 2 (faint) doesn't reliably combine with existing color codes in most terminals. The gray overlay provides consistent dimming.

**Visual result:**
```
╔════════════════════════════════════════════════╗
║  [dimmed gray background text]                 ║
║  [gray left]  ┌─Modal─┐  [gray right]          ║
║  [gray left]  │ text  │  [gray right]          ║
║  [gray left]  └───────┘  [gray right]          ║
║  [dimmed gray background text]                 ║
╚════════════════════════════════════════════════╝
```

**Examples:** Command palette, Help modal, Diagnostics modal, Quit modal, Git commit modal, Push menu, Branch picker

## Implementation Checklist

When adding a modal:

1. **Decide on the visual effect:**
   - Hide background completely → Use `lipgloss.Place()` with whitespace options
   - Show dimmed background → Use `ui.OverlayModal()`

2. **For dimmed background modals** (preferred for most cases):
   - Import `github.com/marcus/sidecar/internal/ui`
   - Call `ui.OverlayModal(background, modalContent, width, height)`
   - Pass the full background content (the function handles dimming)
   - Pass raw modal content (don't pre-center with `lipgloss.Place()`)

3. **For solid overlay modals** (hides background):
   ```go
   lipgloss.WithWhitespaceChars(" "),
   lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
   ```

## Interactive Modal Buttons

All modals with user actions should use interactive buttons instead of key hints like `[Enter] Confirm [Esc] Cancel`.

### Button Rendering Pattern

```go
// In plugin struct, add:
fileOpButtonFocus int // 0=input, 1=confirm, 2=cancel

// In modal render function:
confirmStyle := styles.Button
cancelStyle := styles.Button
if p.fileOpButtonFocus == 1 {
    confirmStyle = styles.ButtonFocused
}
if p.fileOpButtonFocus == 2 {
    cancelStyle = styles.ButtonFocused
}

sb.WriteString("\n\n")
sb.WriteString(confirmStyle.Render(" Confirm "))
sb.WriteString("  ")
sb.WriteString(cancelStyle.Render(" Cancel "))
```

### Keyboard Navigation

- **Tab**: Cycle focus between input field and buttons (input → confirm → cancel → input)
- **Shift+Tab**: Reverse cycle
- **Enter**: Execute focused button (or confirm from input)
- **Esc**: Always cancels (global shortcut)

```go
case "tab":
    p.fileOpButtonFocus = (p.fileOpButtonFocus + 1) % 3
    if p.fileOpButtonFocus == 0 {
        p.fileOpTextInput.Focus()
    } else {
        p.fileOpTextInput.Blur()
    }
    return p, nil
```

### Mouse Support

Register hit regions for buttons during render:

```go
// In mouse.go, add region constants:
const (
    regionFileOpConfirm = "file-op-confirm"
    regionFileOpCancel  = "file-op-cancel"
)

// Register hit regions (calculate positions based on modal layout)
p.mouseHandler.HitMap.AddRect(regionFileOpConfirm, x, y, 10, 1, nil)
p.mouseHandler.HitMap.AddRect(regionFileOpCancel, x+15, y, 10, 1, nil)
```

Handle clicks in the mouse handler:

```go
case regionFileOpConfirm:
    return p.executeFileOp()
case regionFileOpCancel:
    return p.cancelFileOp()
```

### Hover State

Add hover state for visual feedback when mouse moves over buttons:

```go
// In plugin struct:
fileOpButtonHover int // 0=none, 1=confirm, 2=cancel

// Handle hover in mouse handler:
case mouse.ActionHover:
    return p.handleMouseHover(action)

func (p *Plugin) handleMouseHover(action mouse.MouseAction) (*Plugin, tea.Cmd) {
    if p.fileOpMode == FileOpNone || action.Region == nil {
        p.fileOpButtonHover = 0
        return p, nil
    }
    switch action.Region.ID {
    case regionFileOpConfirm:
        p.fileOpButtonHover = 1
    case regionFileOpCancel:
        p.fileOpButtonHover = 2
    default:
        p.fileOpButtonHover = 0
    }
    return p, nil
}

// In modal render, focus takes precedence over hover:
confirmStyle := styles.Button
if p.fileOpButtonFocus == 1 {
    confirmStyle = styles.ButtonFocused
} else if p.fileOpButtonHover == 1 {
    confirmStyle = styles.ButtonHover
}
```

## Path Auto-Complete (for path inputs)

For modals accepting directory paths (like move), show fuzzy-matched suggestions.

### State Fields

```go
// In plugin struct:
dirCache              []string // Cached directory paths
fileOpSuggestions     []string // Current filtered suggestions
fileOpSuggestionIdx   int      // Selected suggestion (-1 = none)
fileOpShowSuggestions bool     // Show suggestions dropdown
```

### Directory Cache

Build a cache of directories (not files) for path suggestions:

```go
func (p *Plugin) buildDirCache() {
    // Walk filesystem collecting only directories
    // Skip .git, node_modules, etc.
    // Respect gitignore
}

func (p *Plugin) getPathSuggestions(query string) []string {
    if len(p.dirCache) == 0 {
        p.buildDirCache()
    }
    matches := FuzzyFilter(p.dirCache, query, maxResults)
    // Return matched paths
}
```

### Suggestion Rendering

```go
// After input field in modal render:
if p.fileOpShowSuggestions && len(p.fileOpSuggestions) > 0 {
    sb.WriteString("\n")
    for i, suggestion := range p.fileOpSuggestions {
        if i == p.fileOpSuggestionIdx {
            sb.WriteString(styles.ListItemSelected.Render("  → " + suggestion))
        } else {
            sb.WriteString(styles.Muted.Render("    " + suggestion))
        }
        if i < len(p.fileOpSuggestions)-1 {
            sb.WriteString("\n")
        }
    }
}
```

### Suggestion Navigation

- **Up/Down** or **Ctrl+P/N**: Navigate suggestions (-1 = no selection)
- **Tab**: Accept top/selected suggestion
- **Enter** (with selection): Accept suggestion and stay in input
- Update suggestions on each keystroke

## Style Constants

```go
// Dimming style used by ui.OverlayModal() (strips ANSI and applies gray)
var DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
```

**Why not preserve colors?** ANSI SGR 2 (faint) doesn't reliably combine with existing color codes in most terminals. Stripping colors and applying a consistent gray provides reliable dimming across all terminal emulators.

## Common Pitfalls

1. **`WithWhitespaceForeground()` doesn't create visible dimming** - It sets the foreground color of space characters, which are invisible. Use `ui.OverlayModal()` if you want visible dimmed background.

2. **Don't use `lipgloss.Place()` with `ui.OverlayModal()`** - they both handle centering, which causes layout issues.

3. **Pass the full background** - `ui.OverlayModal()` needs the complete background content to composite correctly. Don't pre-truncate or pre-dim.

4. **Height constraints** - Ensure modal content respects available height to prevent overflow.

## File Locations

- App-level modals: `internal/app/view.go`
- Plugin modal helper: `internal/ui/overlay.go` (`OverlayModal()`)
- Modal styles: `internal/styles/styles.go` (`ModalBox`, `ModalTitle`, etc.)
