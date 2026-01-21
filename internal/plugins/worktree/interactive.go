package worktree

import (
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/marcus/sidecar/internal/features"
)

// Interactive mode constants
const (
	// doubleEscapeDelay is the max time between Escape presses for double-escape exit.
	// Single Escape is delayed by this amount to detect double-press.
	doubleEscapeDelay = 150 * time.Millisecond

	// pollingDecayFast is the polling interval during active typing.
	pollingDecayFast = 50 * time.Millisecond

	// pollingDecayMedium is the polling interval after brief inactivity.
	pollingDecayMedium = 200 * time.Millisecond

	// pollingDecaySlow is the polling interval after extended inactivity.
	pollingDecaySlow = 500 * time.Millisecond

	// inactivityMediumThreshold triggers medium polling.
	inactivityMediumThreshold = 2 * time.Second

	// inactivitySlowThreshold triggers slow polling.
	inactivitySlowThreshold = 10 * time.Second

	// defaultExitKey is the default keybinding to exit interactive mode.
	defaultExitKey = "ctrl+\\"
)

// escapeTimerMsg is sent when the escape delay timer fires.
// If pendingEscape is still true, we forward the single Escape to tmux.
type escapeTimerMsg struct{}

// InteractiveSessionDeadMsg indicates the tmux session has ended.
// Sent when send-keys or capture fails with a session/pane not found error.
type InteractiveSessionDeadMsg struct{}

// getInteractiveExitKey returns the configured exit keybinding for interactive mode.
// Falls back to defaultExitKey ("ctrl+\") if not configured.
func (p *Plugin) getInteractiveExitKey() string {
	if p.ctx != nil && p.ctx.Config != nil {
		if key := p.ctx.Config.Plugins.Worktree.InteractiveExitKey; key != "" {
			return key
		}
	}
	return defaultExitKey
}

// isSessionDeadError checks if an error indicates the tmux session/pane is gone.
func isSessionDeadError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "can't find pane") ||
		strings.Contains(errStr, "no such session") ||
		strings.Contains(errStr, "session not found") ||
		strings.Contains(errStr, "pane not found")
}

// MapKeyToTmux translates a Bubble Tea key message to a tmux send-keys argument.
// Returns the tmux key name and whether to use literal mode (-l).
// For modified keys and special keys, returns the tmux key name.
// For literal characters, returns the character with useLiteral=true.
func MapKeyToTmux(msg tea.KeyMsg) (key string, useLiteral bool) {
	// Handle special keys
	// Note: KeyCtrlI == KeyTab and KeyCtrlM == KeyEnter in BubbleTea,
	// so we handle Tab and Enter first, then other Ctrl keys.
	switch msg.Type {
	case tea.KeyEnter: // Also KeyCtrlM
		return "Enter", false
	case tea.KeyBackspace:
		return "BSpace", false
	case tea.KeyDelete:
		return "DC", false
	case tea.KeyTab: // Also KeyCtrlI
		return "Tab", false
	case tea.KeySpace:
		return "Space", false
	case tea.KeyUp:
		return "Up", false
	case tea.KeyDown:
		return "Down", false
	case tea.KeyLeft:
		return "Left", false
	case tea.KeyRight:
		return "Right", false
	case tea.KeyHome:
		return "Home", false
	case tea.KeyEnd:
		return "End", false
	case tea.KeyPgUp:
		return "PPage", false
	case tea.KeyPgDown:
		return "NPage", false
	case tea.KeyInsert:
		return "IC", false
	case tea.KeyEscape:
		return "Escape", false

	// Ctrl combinations (excluding KeyCtrlI/Tab and KeyCtrlM/Enter handled above)
	case tea.KeyCtrlA:
		return "C-a", false
	case tea.KeyCtrlB:
		return "C-b", false
	case tea.KeyCtrlC:
		return "C-c", false
	case tea.KeyCtrlD:
		return "C-d", false
	case tea.KeyCtrlE:
		return "C-e", false
	case tea.KeyCtrlF:
		return "C-f", false
	case tea.KeyCtrlG:
		return "C-g", false
	case tea.KeyCtrlH:
		return "C-h", false
	case tea.KeyCtrlJ:
		return "C-j", false
	case tea.KeyCtrlK:
		return "C-k", false
	case tea.KeyCtrlL:
		return "C-l", false
	case tea.KeyCtrlN:
		return "C-n", false
	case tea.KeyCtrlO:
		return "C-o", false
	case tea.KeyCtrlP:
		return "C-p", false
	case tea.KeyCtrlQ:
		return "C-q", false
	case tea.KeyCtrlR:
		return "C-r", false
	case tea.KeyCtrlS:
		return "C-s", false
	case tea.KeyCtrlT:
		return "C-t", false
	case tea.KeyCtrlU:
		return "C-u", false
	case tea.KeyCtrlV:
		return "C-v", false
	case tea.KeyCtrlW:
		return "C-w", false
	case tea.KeyCtrlX:
		return "C-x", false
	case tea.KeyCtrlY:
		return "C-y", false
	case tea.KeyCtrlZ:
		return "C-z", false

	// Function keys (F1-F12)
	case tea.KeyF1:
		return "F1", false
	case tea.KeyF2:
		return "F2", false
	case tea.KeyF3:
		return "F3", false
	case tea.KeyF4:
		return "F4", false
	case tea.KeyF5:
		return "F5", false
	case tea.KeyF6:
		return "F6", false
	case tea.KeyF7:
		return "F7", false
	case tea.KeyF8:
		return "F8", false
	case tea.KeyF9:
		return "F9", false
	case tea.KeyF10:
		return "F10", false
	case tea.KeyF11:
		return "F11", false
	case tea.KeyF12:
		return "F12", false

	case tea.KeyRunes:
		// Regular character input
		if len(msg.Runes) > 0 {
			return string(msg.Runes), true
		}
		return "", true
	}

	// Fallback for any unhandled key types
	if msg.String() != "" {
		return msg.String(), true
	}
	return "", true
}

// sendKeyToTmux sends a key to a tmux pane using send-keys.
// Uses the tmux key name syntax (e.g., "Enter", "C-c", "Up").
func sendKeyToTmux(sessionName, key string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, key)
	return cmd.Run()
}

// sendLiteralToTmux sends literal text to a tmux pane using send-keys -l.
// This prevents tmux from interpreting special key names.
func sendLiteralToTmux(sessionName, text string) error {
	cmd := exec.Command("tmux", "send-keys", "-l", "-t", sessionName, text)
	return cmd.Run()
}

// sendPasteToTmux pastes multi-line text via tmux buffer.
// Uses load-buffer + paste-buffer which works regardless of app paste mode state.
func sendPasteToTmux(sessionName, text string) error {
	// Load text into tmux default buffer via stdin
	loadCmd := exec.Command("tmux", "load-buffer", "-")
	loadCmd.Stdin = strings.NewReader(text)
	if err := loadCmd.Run(); err != nil {
		return err
	}

	// Paste buffer into target pane
	pasteCmd := exec.Command("tmux", "paste-buffer", "-t", sessionName)
	return pasteCmd.Run()
}

// Bracketed paste escape sequences
const (
	bracketedPasteEnable  = "\x1b[?2004h" // ESC[?2004h - app enables bracketed paste
	bracketedPasteDisable = "\x1b[?2004l" // ESC[?2004l - app disables bracketed paste
	bracketedPasteStart   = "\x1b[200~"   // ESC[200~ - start of pasted content
	bracketedPasteEnd     = "\x1b[201~"   // ESC[201~ - end of pasted content
)

// detectBracketedPasteMode checks captured output to determine if the app has
// enabled bracketed paste mode. Looks for the most recent occurrence of either
// the enable (ESC[?2004h) or disable (ESC[?2004l) sequence.
func detectBracketedPasteMode(output string) bool {
	enableIdx := strings.LastIndex(output, bracketedPasteEnable)
	disableIdx := strings.LastIndex(output, bracketedPasteDisable)
	// If enable was found more recently than disable, bracketed paste is enabled
	return enableIdx > disableIdx
}

// sendBracketedPasteToTmux sends text wrapped in bracketed paste sequences.
// Used when the target app has enabled bracketed paste mode.
func sendBracketedPasteToTmux(sessionName, text string) error {
	// Send bracketed paste start sequence
	if err := sendLiteralToTmux(sessionName, bracketedPasteStart); err != nil {
		return err
	}

	// Send the actual text
	if err := sendLiteralToTmux(sessionName, text); err != nil {
		return err
	}

	// Send bracketed paste end sequence
	return sendLiteralToTmux(sessionName, bracketedPasteEnd)
}

// updateBracketedPasteMode updates the BracketedPasteEnabled state from captured output.
// Should be called whenever new output is received for the interactive pane.
func (p *Plugin) updateBracketedPasteMode(output string) {
	if p.interactiveState == nil || !p.interactiveState.Active {
		return
	}
	p.interactiveState.BracketedPasteEnabled = detectBracketedPasteMode(output)
}

// isPasteInput detects if the input is a paste operation.
// Returns true if the input contains newlines or is longer than a typical typed sequence.
func isPasteInput(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) <= 1 {
		return false
	}
	text := string(msg.Runes)
	// Treat as paste if contains newline or is suspiciously long for typing
	return strings.Contains(text, "\n") || len(msg.Runes) > 10
}

// enterInteractiveMode enters interactive mode for the current selection.
// Returns a tea.Cmd if mode entry succeeded, nil otherwise.
// Requires tmux_interactive_input feature flag to be enabled.
func (p *Plugin) enterInteractiveMode() tea.Cmd {
	// Check feature flag
	if !features.IsEnabled(features.TmuxInteractiveInput.Name) {
		return nil
	}

	// Determine target based on current selection
	var sessionName, paneID string

	if p.shellSelected {
		// Shell session
		if p.selectedShellIdx < 0 || p.selectedShellIdx >= len(p.shells) {
			return nil
		}
		shell := p.shells[p.selectedShellIdx]
		if shell.Agent == nil {
			return nil
		}
		sessionName = shell.TmuxName
		paneID = shell.Agent.TmuxPane
	} else {
		// Worktree
		wt := p.selectedWorktree()
		if wt == nil || wt.Agent == nil {
			return nil
		}
		sessionName = wt.Agent.TmuxSession
		paneID = wt.Agent.TmuxPane
	}

	// Initialize interactive state
	p.interactiveState = &InteractiveState{
		Active:        true,
		TargetPane:    paneID,
		TargetSession: sessionName,
		LastKeyTime:   time.Now(),
	}

	p.viewMode = ViewModeInteractive

	// Trigger immediate poll for fresh content
	return p.pollInteractivePane()
}

// exitInteractiveMode exits interactive mode and returns to list view.
func (p *Plugin) exitInteractiveMode() {
	if p.interactiveState != nil {
		p.interactiveState.Active = false
	}
	p.interactiveState = nil
	p.viewMode = ViewModeList
}

// handleInteractiveKeys processes key input in interactive mode.
// Returns a tea.Cmd for any async operations needed.
func (p *Plugin) handleInteractiveKeys(msg tea.KeyMsg) tea.Cmd {
	if p.interactiveState == nil || !p.interactiveState.Active {
		p.exitInteractiveMode()
		return nil
	}

	// Check for exit keys

	// Primary exit: Configurable key (default: Ctrl+\)
	if msg.String() == p.getInteractiveExitKey() {
		p.exitInteractiveMode()
		return nil
	}

	// Secondary exit: Double-Escape with 150ms delay
	// Per spec: first Escape is delayed to detect double-press
	if msg.Type == tea.KeyEscape {
		if p.interactiveState.EscapePressed {
			// Second Escape within window: exit interactive mode
			p.interactiveState.EscapePressed = false
			p.exitInteractiveMode()
			return nil
		}
		// First Escape: mark pending and start delay timer
		// Do NOT forward to tmux yet - wait for timer or next key
		p.interactiveState.EscapePressed = true
		p.interactiveState.EscapeTime = time.Now()
		return tea.Tick(doubleEscapeDelay, func(t time.Time) tea.Msg {
			return escapeTimerMsg{}
		})
	}

	// Non-escape key: check if we have a pending Escape to forward first
	var cmds []tea.Cmd
	if p.interactiveState.EscapePressed {
		p.interactiveState.EscapePressed = false
		// Forward the pending Escape before this key
		if err := sendKeyToTmux(p.interactiveState.TargetSession, "Escape"); err != nil {
			p.exitInteractiveMode()
			if isSessionDeadError(err) {
				return func() tea.Msg { return InteractiveSessionDeadMsg{} }
			}
			return nil
		}
	}

	// Update last key time for polling decay
	p.interactiveState.LastKeyTime = time.Now()

	sessionName := p.interactiveState.TargetSession

	// Check for paste (multi-character input with newlines or long text)
	if isPasteInput(msg) {
		text := string(msg.Runes)
		var err error
		// Use bracketed paste if app has it enabled (td-79ab6163)
		if p.interactiveState.BracketedPasteEnabled {
			err = sendBracketedPasteToTmux(sessionName, text)
		} else {
			err = sendPasteToTmux(sessionName, text)
		}
		if err != nil {
			p.exitInteractiveMode()
			if isSessionDeadError(err) {
				return func() tea.Msg { return InteractiveSessionDeadMsg{} }
			}
			return nil
		}
		cmds = append(cmds, p.pollInteractivePane())
		return tea.Batch(cmds...)
	}

	// Map key to tmux format and send
	key, useLiteral := MapKeyToTmux(msg)
	if key == "" {
		return tea.Batch(cmds...)
	}

	var err error
	if useLiteral {
		err = sendLiteralToTmux(sessionName, key)
	} else {
		err = sendKeyToTmux(sessionName, key)
	}

	if err != nil {
		// Session may have died - exit interactive mode
		p.exitInteractiveMode()
		if isSessionDeadError(err) {
			return func() tea.Msg { return InteractiveSessionDeadMsg{} }
		}
		return nil
	}

	// Schedule fast poll to show updated output quickly
	cmds = append(cmds, p.pollInteractivePane())
	return tea.Batch(cmds...)
}

// handleEscapeTimer processes the escape delay timer firing.
// If a single Escape is still pending (no second Escape arrived), forward it to tmux.
func (p *Plugin) handleEscapeTimer() tea.Cmd {
	if p.interactiveState == nil || !p.interactiveState.Active {
		return nil
	}

	if !p.interactiveState.EscapePressed {
		// Escape was already handled (double-press or another key arrived)
		return nil
	}

	// Timer fired with pending Escape: forward the single Escape to tmux
	p.interactiveState.EscapePressed = false
	if err := sendKeyToTmux(p.interactiveState.TargetSession, "Escape"); err != nil {
		p.exitInteractiveMode()
		if isSessionDeadError(err) {
			return func() tea.Msg { return InteractiveSessionDeadMsg{} }
		}
		return nil
	}

	// Update last key time and poll
	p.interactiveState.LastKeyTime = time.Now()
	return p.pollInteractivePane()
}

// forwardScrollToTmux sends scroll wheel events to the tmux pane.
// Returns a tea.Cmd for any async operations needed.
// Scroll up (delta < 0) sends PPage (Page Up), scroll down (delta > 0) sends NPage (Page Down).
// For smoother scrolling, smaller scroll increments use arrow keys.
func (p *Plugin) forwardScrollToTmux(delta int) tea.Cmd {
	if p.interactiveState == nil || !p.interactiveState.Active {
		return nil
	}

	sessionName := p.interactiveState.TargetSession

	// Determine key based on scroll direction and magnitude
	var key string
	var count int
	if delta < 0 {
		// Scroll up
		count = -delta
		if count >= 3 {
			key = "PPage" // Page up for larger scrolls
			count = 1
		} else {
			key = "Up" // Arrow up for small scrolls
		}
	} else {
		// Scroll down
		count = delta
		if count >= 3 {
			key = "NPage" // Page down for larger scrolls
			count = 1
		} else {
			key = "Down" // Arrow down for small scrolls
		}
	}

	// Send keys to tmux
	for i := 0; i < count; i++ {
		if err := sendKeyToTmux(sessionName, key); err != nil {
			p.exitInteractiveMode()
			if isSessionDeadError(err) {
				return func() tea.Msg { return InteractiveSessionDeadMsg{} }
			}
			return nil
		}
	}

	// Update last key time for polling decay
	p.interactiveState.LastKeyTime = time.Now()

	return p.pollInteractivePane()
}

// forwardClickToTmux sends a mouse click to the tmux pane.
// Currently a no-op as full mouse support requires knowing the terminal's mouse mode.
// This is provided for future extension.
func (p *Plugin) forwardClickToTmux(x, y int) tea.Cmd {
	// Full click forwarding requires:
	// 1. Knowing if app has mouse mode enabled
	// 2. Calculating position relative to pane
	// 3. Generating correct escape sequences
	// For M3, we skip this complexity and rely on keyboard navigation
	return nil
}

// pollInteractivePane schedules a poll for interactive mode with adaptive timing.
func (p *Plugin) pollInteractivePane() tea.Cmd {
	if p.interactiveState == nil || !p.interactiveState.Active {
		return nil
	}

	// Determine polling interval based on activity
	interval := pollingDecayFast
	inactivity := time.Since(p.interactiveState.LastKeyTime)

	if inactivity > inactivitySlowThreshold {
		interval = pollingDecaySlow
	} else if inactivity > inactivityMediumThreshold {
		interval = pollingDecayMedium
	}

	// Use existing shell or worktree polling mechanism
	if p.shellSelected && p.selectedShellIdx >= 0 && p.selectedShellIdx < len(p.shells) {
		return p.scheduleShellPollByName(p.shells[p.selectedShellIdx].TmuxName, interval)
	}
	if wt := p.selectedWorktree(); wt != nil {
		return p.scheduleAgentPoll(wt.Name, interval)
	}
	return nil
}

// cursorStyle defines the appearance of the cursor overlay (reverse video).
var cursorStyle = lipgloss.NewStyle().Reverse(true)

// getCursorPosition queries tmux for the current cursor position in the target pane.
// Returns the cursor row, column (0-indexed), and whether the cursor is visible.
func (p *Plugin) getCursorPosition() (row, col int, visible bool, err error) {
	if p.interactiveState == nil || !p.interactiveState.Active {
		return 0, 0, false, nil
	}

	paneID := p.interactiveState.TargetPane
	if paneID == "" {
		// Fall back to session name if pane ID not available
		paneID = p.interactiveState.TargetSession
	}

	// Query cursor position using tmux display-message
	// #{cursor_x},#{cursor_y} gives 0-indexed position
	// #{cursor_flag} is 0 if cursor hidden (e.g., alternate screen), 1 if visible
	cmd := exec.Command("tmux", "display-message", "-t", paneID,
		"-p", "#{cursor_x},#{cursor_y},#{cursor_flag}")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, false, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) < 2 {
		return 0, 0, false, nil
	}

	col, _ = strconv.Atoi(parts[0])
	row, _ = strconv.Atoi(parts[1])
	visible = len(parts) < 3 || parts[2] != "0"

	// Update cached position in state
	p.interactiveState.CursorCol = col
	p.interactiveState.CursorRow = row

	return row, col, visible, nil
}

// renderWithCursor overlays the cursor on content at the specified position.
// cursorRow is relative to the visible content (0 = first visible line).
// cursorCol is the column within the line (0-indexed).
// Preserves ANSI escape codes in surrounding content while rendering cursor.
func renderWithCursor(content string, cursorRow, cursorCol int, visible bool) string {
	if !visible || cursorRow < 0 || cursorCol < 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	if cursorRow >= len(lines) {
		return content
	}

	line := lines[cursorRow]

	// Use ANSI-aware width calculation for visual position
	lineWidth := ansi.StringWidth(line)

	if cursorCol >= lineWidth {
		// Cursor past end of line: append cursor block
		lines[cursorRow] = line + cursorStyle.Render(" ")
	} else {
		// Use ANSI-aware slicing to preserve escape codes in before/after
		before := ansi.Cut(line, 0, cursorCol)
		char := ansi.Cut(line, cursorCol, cursorCol+1)
		after := ansi.Cut(line, cursorCol+1, lineWidth)

		// Strip the cursor char to get clean reverse video styling
		charStripped := ansi.Strip(char)
		if charStripped == "" {
			charStripped = " "
		}
		lines[cursorRow] = before + cursorStyle.Render(charStripped) + after
	}

	return strings.Join(lines, "\n")
}
