package worktree

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/plugins/gitstatus"
)

// MergeWorkflowStep represents the current step in the merge workflow.
type MergeWorkflowStep int

const (
	MergeStepReviewDiff MergeWorkflowStep = iota
	MergeStepMergeMethod                  // Choose: PR workflow or direct merge
	MergeStepPush
	MergeStepCreatePR
	MergeStepWaitingMerge
	MergeStepDirectMerge                  // Performing direct merge (no PR)
	MergeStepPostMergeConfirmation        // User confirms cleanup options after PR merge
	MergeStepCleanup
	MergeStepDone
)

// String returns a display name for the merge step.
func (s MergeWorkflowStep) String() string {
	switch s {
	case MergeStepReviewDiff:
		return "Review Diff"
	case MergeStepMergeMethod:
		return "Merge Method"
	case MergeStepPush:
		return "Push Branch"
	case MergeStepCreatePR:
		return "Create PR"
	case MergeStepWaitingMerge:
		return "Waiting for Merge"
	case MergeStepDirectMerge:
		return "Direct Merge"
	case MergeStepPostMergeConfirmation:
		return "Confirm Cleanup"
	case MergeStepCleanup:
		return "Cleanup"
	case MergeStepDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// MergeWorkflowState holds the state for the merge workflow modal.
type MergeWorkflowState struct {
	Worktree         *Worktree
	Step             MergeWorkflowStep
	DiffSummary      string
	PRTitle          string
	PRBody           string
	PRURL            string
	ExistingPR       bool   // True if using an existing PR (vs newly created)
	Error            error
	StepStatus       map[MergeWorkflowStep]string // "pending", "running", "done", "error", "skipped"
	DeleteAfterMerge bool                         // true = delete worktree after merge (default)

	// Merge method selection
	UseDirectMerge    bool // true = direct merge to base, false = PR workflow
	MergeMethodOption int  // 0 = Create PR (default), 1 = Direct merge

	// Post-merge confirmation options
	DeleteLocalWorktree bool // Checkbox: delete local worktree (default: true)
	DeleteLocalBranch   bool // Checkbox: delete local branch (default: true)
	DeleteRemoteBranch  bool // Checkbox: delete remote branch (default: false)
	PullAfterMerge      bool // Checkbox: pull changes to current branch after merge
	CurrentBranch       string // Branch user was on before merge (for pull)
	ConfirmationFocus   int  // 0-3=checkboxes, 4=confirm btn, 5=skip btn
	ConfirmationHover   int  // Mouse hover state

	// Cleanup results for summary display
	CleanupResults     *CleanupResults
	PendingCleanupOps  int // Counter for parallel cleanup operations in flight
}

// CleanupResults holds the results of cleanup operations for display in summary.
type CleanupResults struct {
	LocalWorktreeDeleted bool
	LocalBranchDeleted   bool
	RemoteBranchDeleted  bool
	PullAttempted        bool
	PullSuccess          bool
	PullError            error
	Errors               []string
}

// MergeStepCompleteMsg signals a merge workflow step completed.
type MergeStepCompleteMsg struct {
	WorktreeName    string
	Step            MergeWorkflowStep
	Data            string // Step-specific data (e.g., PR URL)
	Err             error
	ExistingPRFound bool // True if PR already existed (vs newly created)
}

// CheckPRMergedMsg signals the result of checking if a PR was merged.
type CheckPRMergedMsg struct {
	WorktreeName string
	Merged       bool
	Err          error
}

// UncommittedChangesCheckMsg signals the result of checking for uncommitted changes.
type UncommittedChangesCheckMsg struct {
	WorktreeName     string
	HasChanges       bool
	StagedCount      int
	ModifiedCount    int
	UntrackedCount   int
	Err              error
}

// MergeCommitDoneMsg signals that the commit before merge completed.
type MergeCommitDoneMsg struct {
	WorktreeName string
	CommitHash   string
	Err          error
}

// MergeCommitState holds state for the commit-before-merge modal.
type MergeCommitState struct {
	Worktree       *Worktree
	StagedCount    int
	ModifiedCount  int
	UntrackedCount int
	CommitMessage  string
	Error          string
}

// RemoteBranchDeleteMsg signals the result of deleting a remote branch.
type RemoteBranchDeleteMsg struct {
	WorktreeName string
	BranchName   string
	Err          error
}

// CleanupDoneMsg signals that cleanup operations completed.
type CleanupDoneMsg struct {
	WorktreeName string
	Results      *CleanupResults
}

// DirectMergeDoneMsg signals that direct merge completed.
type DirectMergeDoneMsg struct {
	WorktreeName string
	BaseBranch   string
	Err          error
}

// PullAfterMergeMsg signals that pull after merge completed.
type PullAfterMergeMsg struct {
	WorktreeName string
	Branch       string
	Success      bool
	Err          error
}

// checkUncommittedChanges checks if a worktree has uncommitted changes.
func (p *Plugin) checkUncommittedChanges(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		tree := gitstatus.NewFileTree(wt.Path)
		if err := tree.Refresh(); err != nil {
			return UncommittedChangesCheckMsg{
				WorktreeName: wt.Name,
				HasChanges:   false,
				Err:          err,
			}
		}

		stagedCount := len(tree.Staged)
		modifiedCount := len(tree.Modified)
		untrackedCount := len(tree.Untracked)
		hasChanges := stagedCount > 0 || modifiedCount > 0 || untrackedCount > 0

		return UncommittedChangesCheckMsg{
			WorktreeName:   wt.Name,
			HasChanges:     hasChanges,
			StagedCount:    stagedCount,
			ModifiedCount:  modifiedCount,
			UntrackedCount: untrackedCount,
		}
	}
}

// stageAllAndCommit stages all changes and commits with the given message.
func (p *Plugin) stageAllAndCommit(wt *Worktree, message string) tea.Cmd {
	return func() tea.Msg {
		tree := gitstatus.NewFileTree(wt.Path)
		if tree == nil {
			return MergeCommitDoneMsg{
				WorktreeName: wt.Name,
				Err:          fmt.Errorf("failed to initialize git tree for %s", wt.Path),
			}
		}

		// Stage all changes
		if err := tree.StageAll(); err != nil {
			return MergeCommitDoneMsg{
				WorktreeName: wt.Name,
				Err:          fmt.Errorf("failed to stage: %w", err),
			}
		}

		// Execute commit
		hash, err := gitstatus.ExecuteCommit(wt.Path, message)
		if err != nil {
			return MergeCommitDoneMsg{
				WorktreeName: wt.Name,
				Err:          err,
			}
		}

		return MergeCommitDoneMsg{
			WorktreeName: wt.Name,
			CommitHash:   hash,
		}
	}
}

// startMergeWorkflow initializes the merge workflow for a worktree.
// It first checks for uncommitted changes and shows a commit modal if needed.
func (p *Plugin) startMergeWorkflow(wt *Worktree) tea.Cmd {
	if wt == nil {
		return nil
	}

	// Check for uncommitted changes before proceeding
	return p.checkUncommittedChanges(wt)
}

// proceedToMergeWorkflow initializes the actual merge workflow (after commit check passes).
func (p *Plugin) proceedToMergeWorkflow(wt *Worktree) tea.Cmd {
	// Capture current branch in main repo for pull option later
	currentBranch, _ := getCurrentBranch(p.ctx.WorkDir)

	// Initialize merge state
	p.mergeState = &MergeWorkflowState{
		Worktree:         wt,
		Step:             MergeStepReviewDiff,
		PRTitle:          wt.Branch, // Default title to branch name
		PRBody:           "",
		StepStatus:       make(map[MergeWorkflowStep]string),
		DeleteAfterMerge: true, // default to delete worktree after merge
		CurrentBranch:    currentBranch,
	}
	p.mergeState.StepStatus[MergeStepReviewDiff] = "running"

	p.viewMode = ViewModeMerge

	// Load diff summary for review
	return p.loadMergeDiff(wt)
}

// loadMergeDiff loads the diff summary for the merge workflow.
func (p *Plugin) loadMergeDiff(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		// Get diff against base branch
		baseBranch := wt.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}

		diff, err := getDiffFromBase(wt.Path, baseBranch)
		if err != nil {
			return MergeStepCompleteMsg{
				WorktreeName: wt.Name,
				Step:         MergeStepReviewDiff,
				Data:         "",
				Err:          err,
			}
		}

		// Get a summary (stat output)
		summary, _ := getDiffSummary(wt.Path)

		return MergeStepCompleteMsg{
			WorktreeName: wt.Name,
			Step:         MergeStepReviewDiff,
			Data:         summary + "\n\n" + truncateDiff(diff, 50),
		}
	}
}

// truncateDiff truncates a diff to a maximum number of lines.
func truncateDiff(diff string, maxLines int) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= maxLines {
		return diff
	}
	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines)
}

// pushForMerge pushes the branch for the merge workflow.
func (p *Plugin) pushForMerge(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		err := doPush(wt.Path, wt.Branch, false, true)
		return MergeStepCompleteMsg{
			WorktreeName: wt.Name,
			Step:         MergeStepPush,
			Err:          err,
		}
	}
}

// parseExistingPRURL extracts the PR URL from a "PR already exists" error message.
// Returns the URL and true if found, empty string and false otherwise.
func parseExistingPRURL(output string) (string, bool) {
	// Error format: "a pull request for branch X into branch Y already exists: <URL>: exit status 1"
	const marker = "already exists:"
	idx := strings.Index(output, marker)
	if idx == -1 {
		return "", false
	}

	// Extract URL after marker
	rest := strings.TrimSpace(output[idx+len(marker):])

	// Find the URL - it starts with http and ends before ": exit" or end of string
	if !strings.HasPrefix(rest, "http") {
		return "", false
	}

	// Find where URL ends - look for ": exit" pattern which follows the URL
	endIdx := strings.Index(rest, ": exit")
	if endIdx == -1 {
		// No ": exit" suffix, URL goes to end (trim whitespace)
		endIdx = strings.IndexAny(rest, " \t\n")
		if endIdx == -1 {
			endIdx = len(rest)
		}
	}

	url := strings.TrimSpace(rest[:endIdx])
	if url == "" {
		return "", false
	}
	return url, true
}

// createPR creates a pull request using gh CLI.
func (p *Plugin) createPR(wt *Worktree, title, body string) tea.Cmd {
	return func() tea.Msg {
		baseBranch := wt.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}

		// Build gh pr create command
		args := []string{"pr", "create",
			"--title", title,
			"--body", body,
			"--base", baseBranch,
		}

		cmd := exec.Command("gh", args...)
		cmd.Dir = wt.Path
		output, err := cmd.CombinedOutput()

		if err != nil {
			// Check if PR already exists
			outputStr := string(output)
			if existingURL, found := parseExistingPRURL(outputStr); found {
				return MergeStepCompleteMsg{
					WorktreeName:    wt.Name,
					Step:            MergeStepCreatePR,
					Data:            existingURL,
					ExistingPRFound: true,
				}
			}
			return MergeStepCompleteMsg{
				WorktreeName: wt.Name,
				Step:         MergeStepCreatePR,
				Err:          fmt.Errorf("gh pr create: %s: %w", strings.TrimSpace(outputStr), err),
			}
		}

		// Output should contain the PR URL
		prURL := strings.TrimSpace(string(output))

		return MergeStepCompleteMsg{
			WorktreeName: wt.Name,
			Step:         MergeStepCreatePR,
			Data:         prURL,
		}
	}
}

// checkPRMerged checks if a PR has been merged using gh CLI.
func (p *Plugin) checkPRMerged(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		// Use gh pr view to check status
		cmd := exec.Command("gh", "pr", "view", "--json", "state,mergedAt")
		cmd.Dir = wt.Path
		output, err := cmd.Output()

		if err != nil {
			return CheckPRMergedMsg{
				WorktreeName: wt.Name,
				Merged:       false,
				Err:          err,
			}
		}

		// Parse JSON response
		var prStatus struct {
			State    string `json:"state"`
			MergedAt string `json:"mergedAt"`
		}

		merged := false
		if err := json.Unmarshal(output, &prStatus); err == nil {
			merged = prStatus.MergedAt != "" || prStatus.State == "MERGED"
		}

		return CheckPRMergedMsg{
			WorktreeName: wt.Name,
			Merged:       merged,
		}
	}
}

// performDirectMerge merges the branch directly to base without creating a PR.
func (p *Plugin) performDirectMerge(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		baseBranch := wt.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
		workDir := p.ctx.WorkDir
		branch := wt.Branch

		// 1. Fetch latest from origin
		fetchCmd := exec.Command("git", "fetch", "origin", baseBranch)
		fetchCmd.Dir = workDir
		if output, err := fetchCmd.CombinedOutput(); err != nil {
			return DirectMergeDoneMsg{
				WorktreeName: wt.Name,
				BaseBranch:   baseBranch,
				Err:          fmt.Errorf("fetch origin: %s: %w", strings.TrimSpace(string(output)), err),
			}
		}

		// 2. Checkout base branch
		checkoutCmd := exec.Command("git", "checkout", baseBranch)
		checkoutCmd.Dir = workDir
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			return DirectMergeDoneMsg{
				WorktreeName: wt.Name,
				BaseBranch:   baseBranch,
				Err:          fmt.Errorf("checkout %s: %s: %w", baseBranch, strings.TrimSpace(string(output)), err),
			}
		}

		// 3. Pull latest
		pullCmd := exec.Command("git", "pull", "origin", baseBranch)
		pullCmd.Dir = workDir
		if output, err := pullCmd.CombinedOutput(); err != nil {
			return DirectMergeDoneMsg{
				WorktreeName: wt.Name,
				BaseBranch:   baseBranch,
				Err:          fmt.Errorf("pull origin %s: %s: %w", baseBranch, strings.TrimSpace(string(output)), err),
			}
		}

		// 4. Merge the worktree branch
		mergeMsg := fmt.Sprintf("Merge branch '%s'", branch)
		mergeCmd := exec.Command("git", "merge", branch, "--no-ff", "-m", mergeMsg)
		mergeCmd.Dir = workDir
		if output, err := mergeCmd.CombinedOutput(); err != nil {
			return DirectMergeDoneMsg{
				WorktreeName: wt.Name,
				BaseBranch:   baseBranch,
				Err:          fmt.Errorf("merge %s: %s: %w", branch, strings.TrimSpace(string(output)), err),
			}
		}

		// 5. Push the merge
		pushCmd := exec.Command("git", "push", "origin", baseBranch)
		pushCmd.Dir = workDir
		if output, err := pushCmd.CombinedOutput(); err != nil {
			return DirectMergeDoneMsg{
				WorktreeName: wt.Name,
				BaseBranch:   baseBranch,
				Err:          fmt.Errorf("push origin %s: %s: %w", baseBranch, strings.TrimSpace(string(output)), err),
			}
		}

		return DirectMergeDoneMsg{
			WorktreeName: wt.Name,
			BaseBranch:   baseBranch,
		}
	}
}

// pullAfterMerge updates the local base branch to match remote after merge.
// Uses git update-ref to update the local branch without requiring checkout.
func (p *Plugin) pullAfterMerge(wt *Worktree, branch string) tea.Cmd {
	return func() tea.Msg {
		workDir := p.ctx.WorkDir

		// 1. Fetch latest for the specific branch
		fetchCmd := exec.Command("git", "fetch", "origin", branch)
		fetchCmd.Dir = workDir
		if output, err := fetchCmd.CombinedOutput(); err != nil {
			return PullAfterMergeMsg{
				WorktreeName: wt.Name,
				Branch:       branch,
				Success:      false,
				Err:          fmt.Errorf("fetch: %s: %w", strings.TrimSpace(string(output)), err),
			}
		}

		// 2. Update local branch ref to match remote (without checkout)
		updateCmd := exec.Command("git", "update-ref", "refs/heads/"+branch, "origin/"+branch)
		updateCmd.Dir = workDir
		if output, err := updateCmd.CombinedOutput(); err != nil {
			return PullAfterMergeMsg{
				WorktreeName: wt.Name,
				Branch:       branch,
				Success:      false,
				Err:          fmt.Errorf("update-ref: %s: %w", strings.TrimSpace(string(output)), err),
			}
		}

		return PullAfterMergeMsg{
			WorktreeName: wt.Name,
			Branch:       branch,
			Success:      true,
		}
	}
}

// cleanupAfterMerge removes the worktree and branch after a successful merge.
func (p *Plugin) cleanupAfterMerge(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		name := wt.Name
		path := wt.Path
		branch := wt.Branch

		// Stop agent if running
		if wt.Agent != nil {
			sessionName := wt.Agent.TmuxSession
			exec.Command("tmux", "kill-session", "-t", sessionName).Run()
		}

		// Remove worktree
		if err := doDeleteWorktree(path); err != nil {
			return MergeStepCompleteMsg{
				WorktreeName: name,
				Step:         MergeStepCleanup,
				Err:          fmt.Errorf("remove worktree: %w", err),
			}
		}

		// Delete the branch (it's been merged)
		cmd := exec.Command("git", "branch", "-d", branch)
		cmd.Dir = p.ctx.WorkDir
		if output, err := cmd.CombinedOutput(); err != nil {
			// Try force delete if regular delete fails
			cmd = exec.Command("git", "branch", "-D", branch)
			cmd.Dir = p.ctx.WorkDir
			if output, err = cmd.CombinedOutput(); err != nil {
				return MergeStepCompleteMsg{
					WorktreeName: name,
					Step:         MergeStepCleanup,
					Err:          fmt.Errorf("delete branch: %s: %w", strings.TrimSpace(string(output)), err),
				}
			}
		}

		return MergeStepCompleteMsg{
			WorktreeName: name,
			Step:         MergeStepCleanup,
		}
	}
}

// deleteRemoteBranch deletes the remote branch from origin.
func (p *Plugin) deleteRemoteBranch(wt *Worktree) tea.Cmd {
	return func() tea.Msg {
		branch := wt.Branch
		name := wt.Name

		// Delete remote branch: git push origin --delete <branch>
		cmd := exec.Command("git", "push", "origin", "--delete", branch)
		cmd.Dir = p.ctx.WorkDir
		output, err := cmd.CombinedOutput()

		if err != nil {
			outputStr := string(output)
			// Check if branch was already deleted (GitHub auto-delete)
			if strings.Contains(outputStr, "remote ref does not exist") ||
				strings.Contains(outputStr, "unable to delete") ||
				strings.Contains(outputStr, "couldn't find remote ref") {
				// Not an error - branch already gone
				return RemoteBranchDeleteMsg{
					WorktreeName: name,
					BranchName:   branch,
				}
			}
			return RemoteBranchDeleteMsg{
				WorktreeName: name,
				BranchName:   branch,
				Err:          fmt.Errorf("delete remote branch: %s", strings.TrimSpace(outputStr)),
			}
		}

		return RemoteBranchDeleteMsg{
			WorktreeName: name,
			BranchName:   branch,
		}
	}
}

// performSelectedCleanup executes only the user-selected cleanup actions.
func (p *Plugin) performSelectedCleanup(wt *Worktree, state *MergeWorkflowState) tea.Cmd {
	return func() tea.Msg {
		results := &CleanupResults{}
		name := wt.Name
		path := wt.Path
		branch := wt.Branch

		// Stop agent if running (always do this)
		if wt.Agent != nil {
			sessionName := wt.Agent.TmuxSession
			exec.Command("tmux", "kill-session", "-t", sessionName).Run()
		}

		// Delete local worktree if selected
		if state.DeleteLocalWorktree {
			if err := doDeleteWorktree(path); err != nil {
				results.Errors = append(results.Errors, fmt.Sprintf("Worktree: %v", err))
			} else {
				results.LocalWorktreeDeleted = true
			}
		}

		// Delete local branch if selected
		if state.DeleteLocalBranch {
			cmd := exec.Command("git", "branch", "-d", branch)
			cmd.Dir = p.ctx.WorkDir
			if output, err := cmd.CombinedOutput(); err != nil {
				// Try force delete if safe delete fails
				cmd = exec.Command("git", "branch", "-D", branch)
				cmd.Dir = p.ctx.WorkDir
				if output, err = cmd.CombinedOutput(); err != nil {
					results.Errors = append(results.Errors,
						fmt.Sprintf("Branch: %s", strings.TrimSpace(string(output))))
				} else {
					results.LocalBranchDeleted = true
				}
			} else {
				results.LocalBranchDeleted = true
			}
		}

		return CleanupDoneMsg{WorktreeName: name, Results: results}
	}
}

// schedulePRCheck schedules a periodic check for PR merge status.
func (p *Plugin) schedulePRCheck(worktreeName string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return checkPRMergeMsg{WorktreeName: worktreeName}
	})
}

// checkPRMergeMsg triggers a PR merge status check.
type checkPRMergeMsg struct {
	WorktreeName string
}

// advanceMergeStep moves to the next step in the merge workflow.
// It marks the current step as "done" and advances to the next step.
func (p *Plugin) advanceMergeStep() tea.Cmd {
	if p.mergeState == nil {
		return nil
	}

	switch p.mergeState.Step {
	case MergeStepReviewDiff:
		// Move to merge method selection step
		p.mergeState.StepStatus[MergeStepReviewDiff] = "done"
		p.mergeState.Step = MergeStepMergeMethod
		p.mergeState.StepStatus[MergeStepMergeMethod] = "running"
		p.mergeState.MergeMethodOption = 0 // Default to PR workflow
		p.mergeState.UseDirectMerge = false
		return nil // Wait for user to select merge method

	case MergeStepMergeMethod:
		// User has selected merge method
		p.mergeState.StepStatus[MergeStepMergeMethod] = "done"
		p.mergeState.UseDirectMerge = p.mergeState.MergeMethodOption == 1

		if p.mergeState.UseDirectMerge {
			// Direct merge path - skip PR workflow
			p.mergeState.StepStatus[MergeStepPush] = "skipped"
			p.mergeState.StepStatus[MergeStepCreatePR] = "skipped"
			p.mergeState.StepStatus[MergeStepWaitingMerge] = "skipped"
			p.mergeState.Step = MergeStepDirectMerge
			p.mergeState.StepStatus[MergeStepDirectMerge] = "running"
			return p.performDirectMerge(p.mergeState.Worktree)
		}

		// PR workflow path - push first
		p.mergeState.Step = MergeStepPush
		p.mergeState.StepStatus[MergeStepPush] = "running"
		return p.pushForMerge(p.mergeState.Worktree)

	case MergeStepDirectMerge:
		// Direct merge completed, go to confirmation
		p.mergeState.StepStatus[MergeStepDirectMerge] = "done"
		p.mergeState.Step = MergeStepPostMergeConfirmation
		p.mergeState.StepStatus[MergeStepPostMergeConfirmation] = "running"
		// Initialize default checkbox values
		p.mergeState.DeleteLocalWorktree = true
		p.mergeState.DeleteLocalBranch = true
		p.mergeState.DeleteRemoteBranch = false // Don't delete remote - wasn't pushed for direct merge
		// Pull option: default checked if current branch matches base branch
		baseBranch := p.mergeState.Worktree.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
		p.mergeState.PullAfterMerge = p.mergeState.CurrentBranch == baseBranch
		p.mergeState.ConfirmationFocus = 0
		return nil

	case MergeStepPush:
		// Mark Push as done, move to create PR step
		p.mergeState.StepStatus[MergeStepPush] = "done"
		p.mergeState.Step = MergeStepCreatePR
		p.mergeState.StepStatus[MergeStepCreatePR] = "running"
		title := p.mergeState.PRTitle
		if title == "" {
			title = p.mergeState.Worktree.Branch
		}
		body := p.mergeState.PRBody
		if body == "" {
			body = "Created from worktree manager"
		}
		return p.createPR(p.mergeState.Worktree, title, body)

	case MergeStepCreatePR:
		// Mark CreatePR as done, move to waiting for merge
		p.mergeState.StepStatus[MergeStepCreatePR] = "done"
		p.mergeState.Step = MergeStepWaitingMerge
		p.mergeState.StepStatus[MergeStepWaitingMerge] = "running"
		// Schedule periodic checks
		return p.schedulePRCheck(p.mergeState.Worktree.Name, 10*time.Second)

	case MergeStepWaitingMerge:
		// Mark WaitingMerge as done, go to confirmation step
		p.mergeState.StepStatus[MergeStepWaitingMerge] = "done"
		p.mergeState.Step = MergeStepPostMergeConfirmation
		p.mergeState.StepStatus[MergeStepPostMergeConfirmation] = "running"

		// Initialize default checkbox values
		p.mergeState.DeleteLocalWorktree = true  // Default: checked
		p.mergeState.DeleteLocalBranch = true    // Default: checked
		p.mergeState.DeleteRemoteBranch = false  // Default: unchecked (safer)
		// Pull option: default checked if current branch matches base branch
		baseBranch := p.mergeState.Worktree.BaseBranch
		if baseBranch == "" {
			baseBranch = "main"
		}
		p.mergeState.PullAfterMerge = p.mergeState.CurrentBranch == baseBranch
		p.mergeState.ConfirmationFocus = 0
		return nil // Wait for user interaction

	case MergeStepPostMergeConfirmation:
		// Mark confirmation as done
		p.mergeState.StepStatus[MergeStepPostMergeConfirmation] = "done"

		// Check if any cleanup or pull is selected
		hasCleanup := p.mergeState.DeleteLocalWorktree ||
			p.mergeState.DeleteLocalBranch ||
			p.mergeState.DeleteRemoteBranch
		hasPull := p.mergeState.PullAfterMerge

		if !hasCleanup && !hasPull {
			// Skip cleanup, go directly to done
			p.mergeState.Step = MergeStepDone
			p.mergeState.StepStatus[MergeStepCleanup] = "skipped"
			p.mergeState.StepStatus[MergeStepDone] = "done"
			return nil
		}

		// Proceed to cleanup
		p.mergeState.Step = MergeStepCleanup
		p.mergeState.StepStatus[MergeStepCleanup] = "running"

		var cmds []tea.Cmd

		// Count pending operations for completion tracking
		pendingOps := 0

		// Local cleanup (worktree + branch)
		if p.mergeState.DeleteLocalWorktree || p.mergeState.DeleteLocalBranch {
			cmds = append(cmds, p.performSelectedCleanup(p.mergeState.Worktree, p.mergeState))
			pendingOps++
		}

		// Remote cleanup (in parallel)
		if p.mergeState.DeleteRemoteBranch {
			cmds = append(cmds, p.deleteRemoteBranch(p.mergeState.Worktree))
			pendingOps++
		}

		// Pull changes to current branch (in parallel)
		if p.mergeState.PullAfterMerge {
			baseBranch := p.mergeState.Worktree.BaseBranch
			if baseBranch == "" {
				baseBranch = "main"
			}
			cmds = append(cmds, p.pullAfterMerge(p.mergeState.Worktree, baseBranch))
			pendingOps++
		}

		p.mergeState.PendingCleanupOps = pendingOps

		return tea.Batch(cmds...)

	case MergeStepCleanup:
		// Done
		p.mergeState.Step = MergeStepDone
		p.mergeState.StepStatus[MergeStepDone] = "done"
		return nil
	}

	return nil
}

// cancelMergeWorkflow cancels the merge workflow and returns to list view.
func (p *Plugin) cancelMergeWorkflow() {
	p.mergeState = nil
	p.viewMode = ViewModeList
}

// checkCleanupComplete decrements pending ops counter and advances to done step when all complete.
// Returns true if all cleanup operations are now complete.
func (p *Plugin) checkCleanupComplete() bool {
	if p.mergeState == nil || p.mergeState.Step != MergeStepCleanup {
		return false
	}

	p.mergeState.PendingCleanupOps--

	if p.mergeState.PendingCleanupOps <= 0 {
		// All done - advance to done step
		p.mergeState.StepStatus[MergeStepCleanup] = "done"
		p.mergeState.Step = MergeStepDone
		p.mergeState.StepStatus[MergeStepDone] = "done"
		return true
	}

	return false
}
