package tty

// SessionDeadMsg indicates the tmux session has ended.
// Sent when send-keys or capture fails with a session/pane not found error.
type SessionDeadMsg struct{}

// PasteResultMsg is sent after a paste operation completes.
type PasteResultMsg struct {
	Err         error // Non-nil if paste failed
	Empty       bool  // True if clipboard was empty
	SessionDead bool  // True if session died during paste
}

// EscapeTimerMsg is sent when the escape delay timer fires.
// If pendingEscape is still true, we forward the single Escape to tmux.
type EscapeTimerMsg struct{}

// CursorPositionMsg delivers cursor position from async query.
type CursorPositionMsg struct {
	Row     int
	Col     int
	Visible bool
}

// PaneResizedMsg is sent when a pane resize operation completes.
// Triggers a fresh poll so captured content reflects the new width/wrapping.
type PaneResizedMsg struct{}

// CaptureResultMsg delivers async tmux capture results.
// Used to avoid blocking the UI thread on tmux subprocess calls.
type CaptureResultMsg struct {
	Target string // Pane or session this capture is for
	Output string // Captured output (empty on error)
	Err    error  // Non-nil if capture failed

	// Cursor state captured atomically with output
	CursorRow     int
	CursorCol     int
	CursorVisible bool
	PaneHeight    int
	PaneWidth     int
}

// PollTickMsg is sent to trigger a poll for output updates.
type PollTickMsg struct {
	Target     string // Which pane/session to poll
	Generation int    // For invalidating stale polls
}
