package worktree

// RefreshMsg triggers a worktree list refresh.
type RefreshMsg struct{}

// RefreshDoneMsg signals that refresh has completed.
type RefreshDoneMsg struct {
	Worktrees []*Worktree
	Err       error
}

// WatchEventMsg signals a filesystem change was detected.
type WatchEventMsg struct {
	Path string
}

// WatcherStartedMsg signals the file watcher is running.
type WatcherStartedMsg struct{}

// WatcherErrorMsg signals a file watcher error.
type WatcherErrorMsg struct {
	Err error
}

// AgentOutputMsg delivers new agent output.
type AgentOutputMsg struct {
	WorktreeName string
	Output       string
	Status       WorktreeStatus
	WaitingFor   string
	// Cursor position captured atomically with output (only set in interactive mode)
	CursorRow     int
	CursorCol     int
	CursorVisible bool
	HasCursor     bool // True if cursor position was captured
	PaneHeight    int  // Tmux pane height for cursor offset calculation
	PaneWidth     int  // Tmux pane width for display alignment
}

// AgentStoppedMsg signals an agent has stopped.
type AgentStoppedMsg struct {
	WorktreeName string
	Err          error
}

// TmuxAttachFinishedMsg signals return from tmux attach.
type TmuxAttachFinishedMsg struct {
	WorktreeName string
	Err          error
}

// DiffLoadedMsg delivers diff content for a worktree.
type DiffLoadedMsg struct {
	WorktreeName string
	Content      string
	Raw          string
}

// DiffErrorMsg signals diff loading failed.
type DiffErrorMsg struct {
	WorktreeName string
	Err          error
}

// StatsLoadedMsg delivers git stats for a worktree.
type StatsLoadedMsg struct {
	WorktreeName string
	Stats        *GitStats
}

// StatsErrorMsg signals stats loading failed.
type StatsErrorMsg struct {
	WorktreeName string
	Err          error
}

// CreateWorktreeMsg requests worktree creation.
type CreateWorktreeMsg struct {
	Name       string
	BaseBranch string
	TaskID     string
}

// CreateDoneMsg signals worktree creation completed.
type CreateDoneMsg struct {
	Worktree  *Worktree
	AgentType AgentType // Agent selected at creation
	SkipPerms bool      // Whether to skip permissions
	Prompt    *Prompt   // Selected prompt template (nil if none)
	Err       error
}

// DeleteWorktreeMsg requests worktree deletion.
type DeleteWorktreeMsg struct {
	Name  string
	Force bool
}

// DeleteDoneMsg signals worktree deletion completed.
type DeleteDoneMsg struct {
	Name     string
	Err      error
	Warnings []string // Non-fatal warnings (e.g., branch deletion failures)
}

// RemoteCheckDoneMsg signals remote branch existence check completed.
type RemoteCheckDoneMsg struct {
	WorktreeName string
	Branch       string
	Exists       bool
}

// PushMsg requests pushing a worktree branch.
type PushMsg struct {
	WorktreeName string
	Force        bool
	SetUpstream  bool
}

// PushDoneMsg signals push operation completed.
type PushDoneMsg struct {
	WorktreeName string
	Err          error
}

// TaskSearchResultsMsg delivers task search results.
type TaskSearchResultsMsg struct {
	Tasks []Task
	Err   error
}

// BranchListMsg delivers available branches.
type BranchListMsg struct {
	Branches []string
	Err      error
}

// TaskLinkedMsg signals a task was linked to a worktree.
type TaskLinkedMsg struct {
	WorktreeName string
	TaskID       string
	Err          error
}

// Task represents a TD task for linking.
type Task struct {
	ID          string
	Title       string
	Status      string
	Description string
	EpicTitle   string // Parent epic title for search
}

// TaskDetails contains full task information for preview pane.
type TaskDetails struct {
	ID          string
	Title       string
	Status      string
	Priority    string
	Type        string
	Description string
	Acceptance  string
	CreatedAt   string
	UpdatedAt   string
}

// TaskDetailsLoadedMsg delivers task details for the preview pane.
type TaskDetailsLoadedMsg struct {
	TaskID  string
	Details *TaskDetails
	Err     error
}

// restartAgentMsg signals that an agent should be restarted after stopping.
type restartAgentMsg struct {
	worktree *Worktree
}

// CommitStatusLoadedMsg delivers commit status info for the diff view header.
type CommitStatusLoadedMsg struct {
	WorktreeName string
	Commits      []CommitStatusInfo
	Err          error
}

// OpenCreateModalWithTaskMsg opens create modal pre-filled with task data.
// Sent from td-monitor plugin when user presses send-to-worktree hotkey.
type OpenCreateModalWithTaskMsg struct {
	TaskID    string
	TaskTitle string
}

// cursorPositionMsg delivers async cursor position updates for interactive mode (td-648af4).
// Queried during poll handler when output changes, not during View() rendering.
type cursorPositionMsg struct {
	Row     int  // 0-indexed row in visible pane
	Col     int  // 0-indexed column
	Visible bool // Whether cursor should be rendered
}
