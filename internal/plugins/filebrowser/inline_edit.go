package filebrowser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/features"
	"github.com/marcus/sidecar/internal/msg"
)

// InlineEditStartedMsg is sent when inline edit mode starts successfully.
type InlineEditStartedMsg struct {
	SessionName string
	FilePath    string
}

// InlineEditExitedMsg is sent when inline edit mode exits.
type InlineEditExitedMsg struct {
	FilePath string
}

// enterInlineEditMode starts inline editing for the specified file.
// Creates a tmux session running the user's editor and delegates to tty.Model.
func (p *Plugin) enterInlineEditMode(path string) tea.Cmd {
	// Check feature flag
	if !features.IsEnabled(features.TmuxInlineEdit.Name) {
		return p.openFile(path)
	}

	fullPath := filepath.Join(p.ctx.WorkDir, path)

	// Get user's editor preference
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vim"
	}

	// Generate a unique session name
	sessionName := fmt.Sprintf("sidecar-edit-%d", time.Now().UnixNano())

	return func() tea.Msg {
		// Check if tmux is available
		if _, err := exec.LookPath("tmux"); err != nil {
			// Fall back to external editor
			return nil
		}

		// Create a detached tmux session with the editor
		// Use -x and -y to set initial size (will be resized later)
		cmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName,
			"-x", "80", "-y", "24", editor, fullPath)
		if err := cmd.Run(); err != nil {
			return msg.ToastMsg{
				Message:  fmt.Sprintf("Failed to start editor: %v", err),
				Duration: 3 * time.Second,
				IsError:  true,
			}
		}

		// Get the pane ID for the new session (for future use)
		paneCmd := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_id}")
		output, _ := paneCmd.Output()
		_ = strings.TrimSpace(string(output)) // paneID for future use

		return InlineEditStartedMsg{
			SessionName: sessionName,
			FilePath:    path,
		}
	}
}

// handleInlineEditStarted processes the InlineEditStartedMsg and activates the tty model.
func (p *Plugin) handleInlineEditStarted(msg InlineEditStartedMsg) tea.Cmd {
	p.inlineEditMode = true
	p.inlineEditSession = msg.SessionName
	p.inlineEditFile = msg.FilePath

	// Configure the tty model callbacks
	p.inlineEditor.OnExit = func() tea.Cmd {
		return func() tea.Msg {
			return InlineEditExitedMsg{FilePath: p.inlineEditFile}
		}
	}
	p.inlineEditor.OnAttach = func() tea.Cmd {
		// Attach to full tmux session
		return p.attachToInlineEditSession()
	}

	// Enter interactive mode on the tty model
	width := p.calculateInlineEditorWidth()
	height := p.calculateInlineEditorHeight()
	p.inlineEditor.SetDimensions(width, height)

	return p.inlineEditor.Enter(msg.SessionName, "")
}

// exitInlineEditMode cleans up inline edit state and kills the tmux session.
func (p *Plugin) exitInlineEditMode() {
	if p.inlineEditSession != "" {
		// Kill the tmux session
		_ = exec.Command("tmux", "kill-session", "-t", p.inlineEditSession).Run()
	}
	p.inlineEditMode = false
	p.inlineEditSession = ""
	p.inlineEditFile = ""
	p.inlineEditor.Exit()
}

// attachToInlineEditSession attaches to the inline edit tmux session in full-screen mode.
func (p *Plugin) attachToInlineEditSession() tea.Cmd {
	if p.inlineEditSession == "" {
		return nil
	}

	sessionName := p.inlineEditSession
	p.exitInlineEditMode()

	return func() tea.Msg {
		// Suspend the TUI and attach to tmux
		return AttachToTmuxMsg{SessionName: sessionName}
	}
}

// AttachToTmuxMsg requests the app to suspend and attach to a tmux session.
type AttachToTmuxMsg struct {
	SessionName string
}

// calculateInlineEditorWidth returns the width for the inline editor.
func (p *Plugin) calculateInlineEditorWidth() int {
	// Use the preview pane width
	if !p.treeVisible {
		return p.width - 2 // Account for borders
	}
	// Account for tree pane and divider
	available := p.width - 1 // divider
	treeW := p.treeWidth
	if treeW <= 0 {
		treeW = 30 // default
	}
	if treeW > available-40 {
		treeW = available - 40
	}
	return available - treeW - 2 // Account for borders
}

// calculateInlineEditorHeight returns the height for the inline editor.
func (p *Plugin) calculateInlineEditorHeight() int {
	// Account for header, footer, tabs, and borders
	h := p.height - 5
	if h < 10 {
		h = 10
	}
	return h
}

// isInlineEditSupported checks if inline editing can be used for the given file.
func (p *Plugin) isInlineEditSupported(path string) bool {
	// Check feature flag
	if !features.IsEnabled(features.TmuxInlineEdit.Name) {
		return false
	}

	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return false
	}

	// Don't support inline editing for binary files
	if p.isBinary {
		return false
	}

	return true
}

// renderInlineEditView renders the inline editor view.
func (p *Plugin) renderInlineEditView() string {
	var sb strings.Builder

	// Render a simple header with the file being edited
	fileName := filepath.Base(p.inlineEditFile)
	header := fmt.Sprintf(" Editing: %s (Ctrl+\\ or double-ESC to exit)", fileName)
	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Width(p.width)
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	// Get the terminal content from the tty model
	if p.inlineEditor != nil {
		content := p.inlineEditor.View()
		// Render within available height
		contentHeight := p.height - 2 // Account for header
		lines := strings.Split(content, "\n")
		if len(lines) > contentHeight {
			lines = lines[:contentHeight]
		}
		// Pad with empty lines if needed
		for len(lines) < contentHeight {
			lines = append(lines, "")
		}
		sb.WriteString(strings.Join(lines, "\n"))
	}

	return sb.String()
}

