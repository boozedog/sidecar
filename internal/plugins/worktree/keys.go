package worktree

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/state"
)

// handleKeyPress processes key input based on current view mode.
func (p *Plugin) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	switch p.viewMode {
	case ViewModeList, ViewModeKanban:
		return p.handleListKeys(msg)
	case ViewModeCreate:
		return p.handleCreateKeys(msg)
	case ViewModeTaskLink:
		return p.handleTaskLinkKeys(msg)
	case ViewModeMerge:
		return p.handleMergeKeys(msg)
	case ViewModeAgentChoice:
		return p.handleAgentChoiceKeys(msg)
	case ViewModeConfirmDelete:
		return p.handleConfirmDeleteKeys(msg)
	case ViewModeCommitForMerge:
		return p.handleCommitForMergeKeys(msg)
	case ViewModePromptPicker:
		return p.handlePromptPickerKeys(msg)
	}
	return nil
}

// handlePromptPickerKeys handles keys in the prompt picker modal.
func (p *Plugin) handlePromptPickerKeys(msg tea.KeyMsg) tea.Cmd {
	if p.promptPicker == nil {
		return nil
	}
	_, cmd := p.promptPicker.Update(msg)
	return cmd
}

// handleAgentChoiceKeys handles keys in agent choice modal.
func (p *Plugin) handleAgentChoiceKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		// Cycle focus: options(0) -> confirm(1) -> cancel(2) -> options(0)
		p.agentChoiceButtonFocus = (p.agentChoiceButtonFocus + 1) % 3
	case "shift+tab":
		// Reverse cycle
		p.agentChoiceButtonFocus = (p.agentChoiceButtonFocus + 2) % 3
	case "j", "down":
		if p.agentChoiceButtonFocus == 0 && p.agentChoiceIdx < 1 {
			p.agentChoiceIdx++
		}
	case "k", "up":
		if p.agentChoiceButtonFocus == 0 && p.agentChoiceIdx > 0 {
			p.agentChoiceIdx--
		}
	case "enter":
		// If focused on cancel button, cancel
		if p.agentChoiceButtonFocus == 2 {
			p.viewMode = ViewModeList
			p.agentChoiceWorktree = nil
			p.agentChoiceButtonFocus = 0
			return nil
		}
		// Confirm action
		return p.executeAgentChoice()
	case "esc", "q":
		p.viewMode = ViewModeList
		p.agentChoiceWorktree = nil
		p.agentChoiceButtonFocus = 0
	}
	return nil
}

// executeAgentChoice executes the selected agent choice action.
func (p *Plugin) executeAgentChoice() tea.Cmd {
	wt := p.agentChoiceWorktree
	p.viewMode = ViewModeList
	p.agentChoiceWorktree = nil
	p.agentChoiceButtonFocus = 0
	if wt == nil {
		return nil
	}
	if p.agentChoiceIdx == 0 {
		// Attach to existing session
		return p.AttachToSession(wt)
	}
	// Restart agent: stop first, then start
	return tea.Sequence(
		p.StopAgent(wt),
		func() tea.Msg {
			return restartAgentMsg{worktree: wt}
		},
	)
}

// handleConfirmDeleteKeys handles keys in delete confirmation modal.
func (p *Plugin) handleConfirmDeleteKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		// Cycle focus: delete(0) -> cancel(1) -> delete(0)
		p.deleteConfirmButtonFocus = (p.deleteConfirmButtonFocus + 1) % 2
	case "shift+tab":
		// Reverse cycle
		p.deleteConfirmButtonFocus = (p.deleteConfirmButtonFocus + 1) % 2
	case "enter":
		if p.deleteConfirmButtonFocus == 1 {
			// Cancel button focused
			return p.cancelDelete()
		}
		// Delete button focused - execute delete
		return p.executeDelete()
	case "D":
		// Power user shortcut - immediate confirm
		return p.executeDelete()
	case "esc", "q":
		return p.cancelDelete()
	case "h", "left":
		if p.deleteConfirmButtonFocus > 0 {
			p.deleteConfirmButtonFocus--
		}
	case "l", "right":
		if p.deleteConfirmButtonFocus < 1 {
			p.deleteConfirmButtonFocus++
		}
	}
	return nil
}

// executeDelete performs the actual worktree deletion and cleans up state.
func (p *Plugin) executeDelete() tea.Cmd {
	wt := p.deleteConfirmWorktree
	if wt == nil {
		p.viewMode = ViewModeList
		return nil
	}

	name := wt.Name
	path := wt.Path

	// Clear modal state
	p.viewMode = ViewModeList
	p.deleteConfirmWorktree = nil
	p.deleteConfirmButtonFocus = 0
	p.deleteConfirmButtonHover = 0

	// Clear preview pane content
	p.diffContent = ""
	p.diffRaw = ""
	p.cachedTaskID = ""
	p.cachedTask = nil

	return func() tea.Msg {
		err := doDeleteWorktree(path)
		return DeleteDoneMsg{Name: name, Err: err}
	}
}

// cancelDelete closes the delete confirmation modal without deleting.
func (p *Plugin) cancelDelete() tea.Cmd {
	p.viewMode = ViewModeList
	p.deleteConfirmWorktree = nil
	p.deleteConfirmButtonFocus = 0
	p.deleteConfirmButtonHover = 0
	return nil
}

// handleListKeys handles keys in list view (and kanban view).
func (p *Plugin) handleListKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		if p.viewMode == ViewModeKanban {
			// Kanban mode: move down within column
			p.moveKanbanRow(1)
			return p.loadSelectedContent()
		}
		if p.activePane == PaneSidebar {
			p.moveCursor(1)
			return p.loadSelectedContent()
		}
		// Scroll down toward newer content (decrease offset from bottom)
		if p.previewOffset > 0 {
			p.previewOffset--
			if p.previewOffset == 0 {
				p.autoScrollOutput = true // Resume auto-scroll when at bottom
			}
		}
	case "k", "up":
		if p.viewMode == ViewModeKanban {
			// Kanban mode: move up within column
			p.moveKanbanRow(-1)
			return p.loadSelectedContent()
		}
		if p.activePane == PaneSidebar {
			p.moveCursor(-1)
			return p.loadSelectedContent()
		}
		// Scroll up toward older content (increase offset from bottom)
		p.autoScrollOutput = false
		p.previewOffset++
	case "g":
		if p.activePane == PaneSidebar {
			p.selectedIdx = 0
			p.scrollOffset = 0
			return p.loadSelectedContent()
		}
		// Go to top (oldest content) - pause auto-scroll
		p.autoScrollOutput = false
		p.previewOffset = 10000 // Large offset, will be clamped in render
	case "G":
		if p.activePane == PaneSidebar {
			p.selectedIdx = len(p.worktrees) - 1
			p.ensureVisible()
			return p.loadSelectedContent()
		}
		// Go to bottom (newest content) - resume auto-scroll
		p.previewOffset = 0
		p.autoScrollOutput = true
	case "n":
		p.viewMode = ViewModeCreate
		// Initialize textinputs for create modal
		p.createNameInput = textinput.New()
		p.createNameInput.Placeholder = "feature-name"
		p.createNameInput.Focus()
		p.createNameInput.CharLimit = 100
		p.createBaseBranchInput = textinput.New()
		p.createBaseBranchInput.Placeholder = "main"
		p.createBaseBranchInput.CharLimit = 100
		p.taskSearchInput = textinput.New()
		p.taskSearchInput.Placeholder = "Search tasks..."
		p.taskSearchInput.CharLimit = 100
		p.createAgentType = AgentClaude // Default to Claude
		p.createSkipPermissions = false
		p.createFocus = 0
		p.taskSearchLoading = true
		// Load prompts from global and project config
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".config", "sidecar")
		p.createPrompts = LoadPrompts(configDir, p.ctx.WorkDir)
		p.createPromptIdx = -1 // No prompt selected by default
		p.promptPicker = nil
		p.branchAll = nil
		p.branchFiltered = nil
		p.branchIdx = 0
		return tea.Batch(p.loadOpenTasks(), p.loadBranches())
	case "D":
		wt := p.selectedWorktree()
		if wt == nil {
			return nil
		}
		p.viewMode = ViewModeConfirmDelete
		p.deleteConfirmWorktree = wt
		p.deleteConfirmButtonFocus = 0 // Focus delete button
		p.deleteConfirmButtonHover = 0
		return nil
	case "p":
		return p.pushSelected()
	case "l", "right":
		if p.viewMode == ViewModeKanban {
			// Kanban mode: move to next column
			p.moveKanbanColumn(1)
			return p.loadSelectedContent()
		}
		if p.activePane == PaneSidebar {
			p.activePane = PanePreview
		} else {
			// Horizontal scroll right in preview pane
			p.previewHorizOffset += 10
		}
	case "enter":
		// Attach to tmux session if agent running, otherwise focus preview
		wt := p.selectedWorktree()
		if wt != nil && wt.Agent != nil {
			p.attachedSession = wt.Name
			return p.AttachToSession(wt)
		}
		if p.activePane == PaneSidebar {
			p.activePane = PanePreview
		}
	case "h", "left":
		if p.viewMode == ViewModeKanban {
			// Kanban mode: move to previous column
			p.moveKanbanColumn(-1)
			return p.loadSelectedContent()
		}
		if p.activePane == PanePreview {
			// Horizontal scroll left in preview pane
			if p.previewHorizOffset > 0 {
				p.previewHorizOffset -= 10
				if p.previewHorizOffset < 0 {
					p.previewHorizOffset = 0
				}
			}
		}
	case "esc":
		if p.activePane == PanePreview {
			p.activePane = PaneSidebar
		}
	case "\\":
		p.toggleSidebar()
	case "tab", "shift+tab":
		// Switch focus between panes (consistent with other plugins)
		if p.activePane == PaneSidebar && p.sidebarVisible {
			p.activePane = PanePreview
		} else if p.activePane == PanePreview && p.sidebarVisible {
			p.activePane = PaneSidebar
		}
	case "[":
		return p.cyclePreviewTab(-1)
	case "]":
		return p.cyclePreviewTab(1)
	case "r":
		return func() tea.Msg { return RefreshMsg{} }
	case "v":
		// In sidebar: toggle between list and kanban view
		// In preview pane on diff tab: toggle unified/side-by-side diff view
		if p.activePane == PanePreview && p.previewTab == PreviewTabDiff {
			if p.diffViewMode == DiffViewUnified {
				p.diffViewMode = DiffViewSideBySide
				_ = state.SetWorktreeDiffMode("side-by-side")
			} else {
				p.diffViewMode = DiffViewUnified
				_ = state.SetWorktreeDiffMode("unified")
			}
		} else if p.activePane == PaneSidebar || p.viewMode == ViewModeKanban {
			if p.viewMode == ViewModeList {
				p.viewMode = ViewModeKanban
				p.syncListToKanban()
			} else if p.viewMode == ViewModeKanban {
				p.viewMode = ViewModeList
			}
		}
	case "ctrl+d":
		// Page down in preview pane
		if p.activePane == PanePreview {
			pageSize := p.height / 2
			if pageSize < 5 {
				pageSize = 5
			}
			if p.previewTab == PreviewTabOutput {
				// For output, offset is from bottom
				if p.previewOffset > pageSize {
					p.previewOffset -= pageSize
				} else {
					p.previewOffset = 0
					p.autoScrollOutput = true
				}
			} else {
				p.previewOffset += pageSize
			}
		}
	case "ctrl+u":
		// Page up in preview pane
		if p.activePane == PanePreview {
			pageSize := p.height / 2
			if pageSize < 5 {
				pageSize = 5
			}
			if p.previewTab == PreviewTabOutput {
				// For output, offset is from bottom
				p.autoScrollOutput = false
				p.previewOffset += pageSize
			} else {
				if p.previewOffset > pageSize {
					p.previewOffset -= pageSize
				} else {
					p.previewOffset = 0
				}
			}
		}
	case "0":
		// Reset horizontal scroll
		if p.activePane == PanePreview {
			p.previewHorizOffset = 0
		}

	// Agent control keys
	case "s":
		// Start agent on selected worktree
		wt := p.selectedWorktree()
		if wt == nil {
			return nil
		}
		if wt.Agent == nil {
			// No agent running - start new one
			return p.StartAgent(wt, AgentClaude)
		}
		// Agent exists - show choice modal (attach or restart)
		p.agentChoiceWorktree = wt
		p.agentChoiceIdx = 0           // Default to attach
		p.agentChoiceButtonFocus = 0   // Start with options focused
		p.agentChoiceButtonHover = 0   // Clear hover state
		p.viewMode = ViewModeAgentChoice
		return nil
	case "S":
		// Stop agent on selected worktree
		wt := p.selectedWorktree()
		if wt != nil && wt.Agent != nil {
			return p.StopAgent(wt)
		}
	case "y":
		// Approve pending prompt on selected worktree
		wt := p.selectedWorktree()
		if wt != nil && wt.Status == StatusWaiting && wt.Agent != nil {
			return p.Approve(wt)
		}
	case "Y":
		// Approve all pending prompts
		return p.ApproveAll()
	case "N":
		// Reject pending prompt on selected worktree
		wt := p.selectedWorktree()
		if wt != nil && wt.Status == StatusWaiting && wt.Agent != nil {
			return p.Reject(wt)
		}
	case "t":
		// Link/unlink td task
		wt := p.selectedWorktree()
		if wt != nil {
			if wt.TaskID != "" {
				// Already linked - unlink
				return p.unlinkTask(wt)
			}
			// No task linked - show task link modal
			p.viewMode = ViewModeTaskLink
			p.linkingWorktree = wt
			p.taskSearchInput = textinput.New()
			p.taskSearchInput.Placeholder = "Search tasks..."
			p.taskSearchInput.Focus()
			p.taskSearchInput.CharLimit = 100
			p.taskSearchIdx = 0
			p.taskSearchLoading = true
			return p.loadOpenTasks()
		}
	case "m":
		// Start merge workflow
		wt := p.selectedWorktree()
		if wt != nil {
			return p.startMergeWorkflow(wt)
		}
	}
	return nil
}

// handleCreateKeys handles keys in create modal.
// createFocus: 0=name, 1=base, 2=prompt, 3=task, 4=agent, 5=skipPerms, 6=create button, 7=cancel button
func (p *Plugin) handleCreateKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.viewMode = ViewModeList
		p.clearCreateModal()
		return nil
	case "tab":
		// Blur current, move focus, focus new
		p.blurCreateInputs()
		p.createFocus = (p.createFocus + 1) % 8
		// Skip task field (3) if prompt has ticketMode=none
		if p.createFocus == 3 {
			prompt := p.getSelectedPrompt()
			if prompt != nil && prompt.TicketMode == TicketNone {
				p.createFocus = 4 // Skip to agent
			}
		}
		// Skip state 5 (skipPerms) if checkbox is hidden
		if p.createFocus == 5 && !p.shouldShowSkipPermissions() {
			p.createFocus = 6
		}
		p.focusCreateInput()
		return nil
	case "shift+tab":
		p.blurCreateInputs()
		p.createFocus = (p.createFocus + 7) % 8
		// Skip state 5 (skipPerms) if checkbox is hidden
		if p.createFocus == 5 && !p.shouldShowSkipPermissions() {
			p.createFocus = 4
		}
		// Skip task field (3) if prompt has ticketMode=none
		if p.createFocus == 3 {
			prompt := p.getSelectedPrompt()
			if prompt != nil && prompt.TicketMode == TicketNone {
				p.createFocus = 2 // Back to prompt
			}
		}
		p.focusCreateInput()
		return nil
	case "backspace":
		// Clear selected task and allow searching again (now focus 3)
		if p.createFocus == 3 && p.createTaskID != "" {
			p.createTaskID = ""
			p.createTaskTitle = ""
			p.taskSearchInput.SetValue("")
			p.taskSearchInput.Focus()
			p.taskSearchFiltered = filterTasks("", p.taskSearchAll)
			p.taskSearchIdx = 0
			return nil
		}
	case " ":
		// Toggle skip permissions checkbox (now focus 5)
		if p.createFocus == 5 {
			p.createSkipPermissions = !p.createSkipPermissions
			return nil
		}
	case "up":
		// Navigate branch dropdown
		if p.createFocus == 1 && len(p.branchFiltered) > 0 {
			if p.branchIdx > 0 {
				p.branchIdx--
			}
			return nil
		}
		// Navigate task dropdown (now focus 3)
		if p.createFocus == 3 && len(p.taskSearchFiltered) > 0 {
			if p.taskSearchIdx > 0 {
				p.taskSearchIdx--
			}
			return nil
		}
		// Navigate agent selection (now focus 4)
		if p.createFocus == 4 {
			p.cycleAgentType(false)
			return nil
		}
	case "down":
		// Navigate branch dropdown
		if p.createFocus == 1 && len(p.branchFiltered) > 0 {
			if p.branchIdx < len(p.branchFiltered)-1 {
				p.branchIdx++
			}
			return nil
		}
		// Navigate task dropdown (now focus 3)
		if p.createFocus == 3 && len(p.taskSearchFiltered) > 0 {
			if p.taskSearchIdx < len(p.taskSearchFiltered)-1 {
				p.taskSearchIdx++
			}
			return nil
		}
		// Navigate agent selection (now focus 4)
		if p.createFocus == 4 {
			p.cycleAgentType(true)
			return nil
		}
	case "enter":
		// Select branch from dropdown if in branch field
		if p.createFocus == 1 && len(p.branchFiltered) > 0 {
			selectedBranch := p.branchFiltered[p.branchIdx]
			p.createBaseBranchInput.SetValue(selectedBranch)
			p.createBaseBranchInput.Blur()
			p.createFocus = 2 // Move to prompt field
			p.focusCreateInput()
			return nil
		}
		// Open prompt picker if in prompt field
		if p.createFocus == 2 {
			p.promptPicker = NewPromptPicker(p.createPrompts, p.width, p.height)
			p.viewMode = ViewModePromptPicker
			return nil
		}
		// Select task from dropdown if in task field (now focus 3)
		if p.createFocus == 3 && len(p.taskSearchFiltered) > 0 {
			// Select task and move to next field
			selectedTask := p.taskSearchFiltered[p.taskSearchIdx]
			p.createTaskID = selectedTask.ID
			p.createTaskTitle = selectedTask.Title
			p.taskSearchInput.Blur()
			p.createFocus = 4 // Move to agent field
			return nil
		}
		// Create button (now focus 6)
		if p.createFocus == 6 {
			return p.createWorktree()
		}
		// Cancel button (now focus 7)
		if p.createFocus == 7 {
			p.viewMode = ViewModeList
			p.clearCreateModal()
			return nil
		}
		// From input fields (0-1), move to next field
		if p.createFocus < 2 {
			p.blurCreateInputs()
			p.createFocus++
			p.focusCreateInput()
		}
		return nil
	}

	// Delegate to focused textinput for all other keys
	// Clear error when user types (they're correcting the issue)
	p.createError = ""
	var cmd tea.Cmd
	switch p.createFocus {
	case 0:
		p.createNameInput, cmd = p.createNameInput.Update(msg)
	case 1:
		p.createBaseBranchInput, cmd = p.createBaseBranchInput.Update(msg)
		// Update filtered branches on input change
		p.branchFiltered = filterBranches(p.createBaseBranchInput.Value(), p.branchAll)
		p.branchIdx = 0
	case 3:
		p.taskSearchInput, cmd = p.taskSearchInput.Update(msg)
		// Update filtered results on input change
		p.taskSearchFiltered = filterTasks(p.taskSearchInput.Value(), p.taskSearchAll)
		p.taskSearchIdx = 0
	}
	return cmd
}

// shouldShowSkipPermissions returns true if the current agent type supports skip permissions.
func (p *Plugin) shouldShowSkipPermissions() bool {
	if p.createAgentType == AgentNone {
		return false
	}
	flag := SkipPermissionsFlags[p.createAgentType]
	return flag != ""
}

// cycleAgentType cycles through agent types in the selection.
func (p *Plugin) cycleAgentType(forward bool) {
	currentIdx := 0
	for i, at := range AgentTypeOrder {
		if at == p.createAgentType {
			currentIdx = i
			break
		}
	}

	if forward {
		currentIdx = (currentIdx + 1) % len(AgentTypeOrder)
	} else {
		currentIdx = (currentIdx + len(AgentTypeOrder) - 1) % len(AgentTypeOrder)
	}

	p.createAgentType = AgentTypeOrder[currentIdx]
}

// blurCreateInputs blurs all create modal textinputs.
func (p *Plugin) blurCreateInputs() {
	p.createNameInput.Blur()
	p.createBaseBranchInput.Blur()
	p.taskSearchInput.Blur()
}

// focusCreateInput focuses the appropriate textinput based on createFocus.
// createFocus: 0=name, 1=base, 2=prompt (no textinput), 3=task, 4+=non-inputs
func (p *Plugin) focusCreateInput() {
	switch p.createFocus {
	case 0:
		p.createNameInput.Focus()
	case 1:
		p.createBaseBranchInput.Focus()
	// case 2 is prompt field - no textinput to focus (opens picker on Enter)
	case 3:
		p.taskSearchInput.Focus()
	}
}

// handleTaskLinkKeys handles keys in task link modal.
func (p *Plugin) handleTaskLinkKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		p.viewMode = ViewModeList
		p.linkingWorktree = nil
		p.taskSearchInput = textinput.Model{}
		p.taskSearchAll = nil
		p.taskSearchFiltered = nil
		p.taskSearchIdx = 0
		return nil
	case "up":
		if len(p.taskSearchFiltered) > 0 && p.taskSearchIdx > 0 {
			p.taskSearchIdx--
		}
		return nil
	case "down":
		if len(p.taskSearchFiltered) > 0 && p.taskSearchIdx < len(p.taskSearchFiltered)-1 {
			p.taskSearchIdx++
		}
		return nil
	case "enter":
		if len(p.taskSearchFiltered) > 0 && p.linkingWorktree != nil {
			selectedTask := p.taskSearchFiltered[p.taskSearchIdx]
			wt := p.linkingWorktree
			p.viewMode = ViewModeList
			p.linkingWorktree = nil
			p.taskSearchInput = textinput.Model{}
			p.taskSearchAll = nil
			p.taskSearchFiltered = nil
			p.taskSearchIdx = 0
			return p.linkTask(wt, selectedTask.ID)
		}
		return nil
	}

	// Delegate to textinput for all other keys (typing, backspace, paste, etc.)
	var cmd tea.Cmd
	p.taskSearchInput, cmd = p.taskSearchInput.Update(msg)
	// Update filtered results on input change
	p.taskSearchFiltered = filterTasks(p.taskSearchInput.Value(), p.taskSearchAll)
	p.taskSearchIdx = 0
	return cmd
}

// handleMergeKeys handles keys in merge workflow modal.
func (p *Plugin) handleMergeKeys(msg tea.KeyMsg) tea.Cmd {
	if p.mergeState == nil {
		p.viewMode = ViewModeList
		return nil
	}

	switch msg.String() {
	case "esc", "q":
		p.cancelMergeWorkflow()
		return nil

	case "enter":
		// Continue to next step based on current step
		switch p.mergeState.Step {
		case MergeStepReviewDiff:
			// User reviewed diff, proceed to push
			return p.advanceMergeStep()
		case MergeStepWaitingMerge:
			// Manual check for merge status
			return p.checkPRMerged(p.mergeState.Worktree)
		case MergeStepPostMergeConfirmation:
			// User confirmed cleanup options
			if p.mergeState.ConfirmationFocus == 4 {
				// Skip All button - uncheck everything
				p.mergeState.DeleteLocalWorktree = false
				p.mergeState.DeleteLocalBranch = false
				p.mergeState.DeleteRemoteBranch = false
			}
			return p.advanceMergeStep()
		case MergeStepDone:
			// Close modal
			p.cancelMergeWorkflow()
		}

	case "up", "k":
		if p.mergeState.Step == MergeStepWaitingMerge {
			// Select "Delete worktree after merge"
			p.mergeState.DeleteAfterMerge = true
		} else if p.mergeState.Step == MergeStepPostMergeConfirmation {
			// Navigate checkboxes/buttons
			if p.mergeState.ConfirmationFocus > 0 {
				p.mergeState.ConfirmationFocus--
			}
		}

	case "down", "j":
		if p.mergeState.Step == MergeStepWaitingMerge {
			// Select "Keep worktree"
			p.mergeState.DeleteAfterMerge = false
		} else if p.mergeState.Step == MergeStepPostMergeConfirmation {
			// Navigate checkboxes/buttons
			if p.mergeState.ConfirmationFocus < 4 {
				p.mergeState.ConfirmationFocus++
			}
		}

	case " ":
		// Space toggles checkboxes in confirmation step
		if p.mergeState.Step == MergeStepPostMergeConfirmation {
			switch p.mergeState.ConfirmationFocus {
			case 0:
				p.mergeState.DeleteLocalWorktree = !p.mergeState.DeleteLocalWorktree
			case 1:
				p.mergeState.DeleteLocalBranch = !p.mergeState.DeleteLocalBranch
			case 2:
				p.mergeState.DeleteRemoteBranch = !p.mergeState.DeleteRemoteBranch
			}
		}

	case "tab":
		// Tab cycles focus in confirmation step
		if p.mergeState.Step == MergeStepPostMergeConfirmation {
			p.mergeState.ConfirmationFocus = (p.mergeState.ConfirmationFocus + 1) % 5
		}

	case "shift+tab":
		// Shift+Tab reverse cycles focus
		if p.mergeState.Step == MergeStepPostMergeConfirmation {
			p.mergeState.ConfirmationFocus = (p.mergeState.ConfirmationFocus + 4) % 5
		}

	case "s":
		// Skip current step (for pushing, creating PR)
		switch p.mergeState.Step {
		case MergeStepReviewDiff:
			// Skip push step if already pushed
			p.mergeState.StepStatus[MergeStepReviewDiff] = "done"
			p.mergeState.Step = MergeStepPush
			return p.advanceMergeStep()
		}
	}
	return nil
}

// handleCommitForMergeKeys handles keys in the commit-before-merge modal.
func (p *Plugin) handleCommitForMergeKeys(msg tea.KeyMsg) tea.Cmd {
	if p.mergeCommitState == nil {
		p.viewMode = ViewModeList
		return nil
	}

	switch msg.String() {
	case "esc":
		// Cancel - return to list
		p.mergeCommitState = nil
		p.mergeCommitMessageInput = textinput.Model{}
		p.viewMode = ViewModeList
		return nil

	case "enter":
		// Commit and continue
		message := p.mergeCommitMessageInput.Value()
		if message == "" {
			p.mergeCommitState.Error = "Commit message cannot be empty"
			return nil
		}
		p.mergeCommitState.Error = ""
		return p.stageAllAndCommit(p.mergeCommitState.Worktree, message)
	}

	// Delegate to textinput for all other keys
	p.mergeCommitState.Error = "" // Clear error when user types
	var cmd tea.Cmd
	p.mergeCommitMessageInput, cmd = p.mergeCommitMessageInput.Update(msg)
	return cmd
}
