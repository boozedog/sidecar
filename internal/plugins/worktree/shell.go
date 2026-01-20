package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Shell session constants
const (
	shellSessionPrefix = "sidecar-sh-" // Distinct from worktree prefix "sidecar-wt-"
)

// Shell session messages
type (
	// ShellCreatedMsg signals shell session was created
	ShellCreatedMsg struct {
		SessionName string // Name of the created session
		Err         error  // Non-nil if creation failed
	}

	// ShellDetachedMsg signals user detached from shell session
	ShellDetachedMsg struct {
		Err error
	}

	// ShellKilledMsg signals shell session was terminated
	ShellKilledMsg struct{}

	// ShellOutputMsg signals shell output was captured (for polling)
	ShellOutputMsg struct {
		Output  string
		Changed bool
	}
)

// initShellSession initializes shell session tracking for the current project.
// Called from Init() to check for existing sessions from previous runs.
func (p *Plugin) initShellSession() {
	projectName := filepath.Base(p.ctx.WorkDir)
	p.shellSessionName = shellSessionPrefix + sanitizeName(projectName)

	// Check if session already exists from previous run
	if sessionExists(p.shellSessionName) {
		p.shellSession = &Agent{
			Type:        AgentShell,
			TmuxSession: p.shellSessionName,
			OutputBuf:   NewOutputBuffer(outputBufferCap),
			StartedAt:   time.Now(), // Approximate, we don't know actual start
			Status:      AgentStatusRunning,
		}
	}
}

// createShellSession creates a new tmux session in the project root directory.
func (p *Plugin) createShellSession() tea.Cmd {
	// Capture values to avoid race conditions in closure
	sessionName := p.shellSessionName
	workDir := p.ctx.WorkDir

	return func() tea.Msg {
		// Check if session already exists
		if sessionExists(sessionName) {
			return ShellCreatedMsg{SessionName: sessionName}
		}

		// Create new detached session in project directory
		args := []string{
			"new-session",
			"-d",            // Detached
			"-s", sessionName, // Session name
			"-c", workDir,     // Working directory
		}
		cmd := exec.Command("tmux", args...)
		if err := cmd.Run(); err != nil {
			return ShellCreatedMsg{SessionName: sessionName, Err: fmt.Errorf("create shell session: %w", err)}
		}

		return ShellCreatedMsg{SessionName: sessionName}
	}
}

// attachToShell attaches to the shell tmux session.
func (p *Plugin) attachToShell() tea.Cmd {
	if p.shellSessionName == "" {
		return nil
	}

	sessionName := p.shellSessionName
	projectName := filepath.Base(p.ctx.WorkDir)
	c := exec.Command("tmux", "attach-session", "-t", sessionName)
	return tea.Sequence(
		tea.Printf("\nAttaching to %s shell. Press Ctrl-b d to return to sidecar.\n", projectName),
		tea.ExecProcess(c, func(err error) tea.Msg {
			return ShellDetachedMsg{Err: err}
		}),
	)
}

// ensureShellAndAttach creates shell session if needed, then attaches.
// If session already exists (even without tracking state), it attaches directly.
func (p *Plugin) ensureShellAndAttach() tea.Cmd {
	if p.shellSessionName == "" {
		return nil
	}

	// If session already exists, ensure we have tracking state and attach
	if sessionExists(p.shellSessionName) {
		// Set up tracking state if not already present
		if p.shellSession == nil {
			p.shellSession = &Agent{
				Type:        AgentShell,
				TmuxSession: p.shellSessionName,
				OutputBuf:   NewOutputBuffer(outputBufferCap),
				StartedAt:   time.Now(),
				Status:      AgentStatusRunning,
			}
			p.managedSessions[p.shellSessionName] = true
		}
		return p.attachToShell()
	}

	// No session exists, create one then attach
	sessionName := p.shellSessionName
	return tea.Sequence(
		p.createShellSession(),
		func() tea.Msg {
			// Wait for session to be ready with exponential backoff
			waitForSession(sessionName)
			return shellAttachAfterCreateMsg{}
		},
	)
}

// waitForSession waits for a tmux session to become available using exponential backoff.
// Returns true if session exists, false if max attempts exceeded.
func waitForSession(sessionName string) bool {
	const maxAttempts = 10
	delay := 10 * time.Millisecond

	for range maxAttempts {
		if sessionExists(sessionName) {
			return true
		}
		time.Sleep(delay)
		delay *= 2 // Exponential backoff: 10, 20, 40, 80, 160, 320, 640ms...
		if delay > 200*time.Millisecond {
			delay = 200 * time.Millisecond // Cap at 200ms per attempt
		}
	}
	return false
}

// shellAttachAfterCreateMsg triggers attachment after shell creation.
type shellAttachAfterCreateMsg struct{}

// killShellSession terminates the shell tmux session.
func (p *Plugin) killShellSession() tea.Cmd {
	if p.shellSessionName == "" {
		return nil
	}

	sessionName := p.shellSessionName
	return func() tea.Msg {
		// Kill the session
		cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
		cmd.Run() // Ignore errors (session may already be dead)

		// Clean up pane cache
		globalPaneCache.remove(sessionName)

		return ShellKilledMsg{}
	}
}

// pollShellSession captures output from the shell tmux session.
func (p *Plugin) pollShellSession() tea.Cmd {
	if p.shellSession == nil || p.shellSessionName == "" {
		return nil
	}

	// Capture values to avoid race conditions in closure
	sessionName := p.shellSessionName
	outputBuf := p.shellSession.OutputBuf
	maxBytes := p.tmuxCaptureMaxBytes

	return func() tea.Msg {
		output, err := capturePaneDirect(sessionName)
		if err != nil {
			return ShellOutputMsg{Output: "", Changed: false}
		}

		// Trim to max bytes
		output = trimCapturedOutput(output, maxBytes)

		// Update buffer and check if content changed
		changed := outputBuf.Update(output)

		return ShellOutputMsg{Output: output, Changed: changed}
	}
}

// scheduleShellPoll schedules a poll for shell output after delay.
func (p *Plugin) scheduleShellPoll(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return pollShellMsg{}
	})
}

// pollShellMsg triggers a shell output poll.
type pollShellMsg struct{}

