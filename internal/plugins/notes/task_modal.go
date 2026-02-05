package notes

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	appmsg "github.com/marcus/sidecar/internal/msg"
	"github.com/marcus/sidecar/internal/ui"
)

// Task types for the dropdown
var taskTypes = []string{"task", "feature", "bug", "chore"}

// Priority levels for the dropdown
var priorityLevels = []string{"P0", "P1", "P2", "P3"}

// TaskCreatedMsg is sent when a task is created from a note.
type TaskCreatedMsg struct {
	TaskID string
	NoteID string
	Err    error
	Epoch  uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m TaskCreatedMsg) GetEpoch() uint64 {
	return m.Epoch
}

// ensureTaskModal builds the task modal if needed.
// Must be called in both View() and Update() handlers.
func (p *Plugin) ensureTaskModal() {
	if p.taskModalNote == nil {
		return
	}

	// Calculate modal width
	modalW := ui.ModalWidthLarge
	if modalW > p.width-4 {
		modalW = p.width - 4
	}
	if modalW < 40 {
		modalW = 40
	}

	// Only rebuild if modal doesn't exist or width changed
	if p.taskModal != nil && p.taskModalWidth == modalW {
		return
	}

	p.taskModalWidth = modalW

	// Build type dropdown items
	typeItems := make([]modal.ListItem, len(taskTypes))
	for i, t := range taskTypes {
		typeItems[i] = modal.ListItem{ID: "type-" + t, Label: t}
	}

	// Build priority dropdown items
	priorityItems := make([]modal.ListItem, len(priorityLevels))
	for i, p := range priorityLevels {
		priorityItems[i] = modal.ListItem{ID: "priority-" + p, Label: p}
	}

	p.taskModal = modal.New("Convert to Task",
		modal.WithWidth(modalW),
		modal.WithPrimaryAction("create"),
	).
		AddSection(modal.InputWithLabel("title", "Title", &p.taskModalTitleInput)).
		AddSection(modal.Spacer()).
		AddSection(modal.Text("Type")).
		AddSection(modal.List("type-list", typeItems, &p.taskModalTypeIdx, modal.WithMaxVisible(4))).
		AddSection(modal.Spacer()).
		AddSection(modal.Text("Priority")).
		AddSection(modal.List("priority-list", priorityItems, &p.taskModalPriorityIdx, modal.WithMaxVisible(4))).
		AddSection(modal.Spacer()).
		AddSection(modal.Checkbox("archive-note", "Archive note after creating task", &p.taskModalArchiveNote)).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Create ", "create"),
			modal.Btn(" Cancel ", "cancel"),
		))
}

// clearTaskModal clears the task modal cache, forcing rebuild.
func (p *Plugin) clearTaskModal() {
	p.taskModal = nil
	p.taskModalWidth = 0
}

// openTaskModal opens the task modal for the selected note.
func (p *Plugin) openTaskModal() tea.Cmd {
	note := p.getSelectedNote()
	if note == nil {
		return nil
	}

	// Store reference to note being converted
	p.taskModalNote = note
	p.showTaskModal = true

	// Initialize title input with note title (or first line of content)
	p.taskModalTitleInput = textinput.New()
	p.taskModalTitleInput.Placeholder = "Task title"
	p.taskModalTitleInput.Width = 40
	title := note.Title
	if title == "" && note.Content != "" {
		lines := strings.SplitN(note.Content, "\n", 2)
		if len(lines) > 0 {
			title = strings.TrimSpace(lines[0])
		}
	}
	p.taskModalTitleInput.SetValue(title)
	p.taskModalTitleInput.Focus()

	// Initialize type and priority to defaults
	p.taskModalTypeIdx = 0     // task
	p.taskModalPriorityIdx = 2 // P2

	// Initialize archive option
	p.taskModalArchiveNote = false

	// Initialize mouse handler
	if p.taskModalMouseHandler == nil {
		p.taskModalMouseHandler = mouse.NewHandler()
	}

	// Clear modal cache to rebuild with new data
	p.clearTaskModal()

	return nil
}

// closeTaskModal closes the task modal and resets state.
func (p *Plugin) closeTaskModal() {
	p.showTaskModal = false
	p.taskModalNote = nil
	p.taskModal = nil
	p.taskModalWidth = 0
}

// renderTaskModal renders the task modal overlaid on the main view.
func (p *Plugin) renderTaskModal() string {
	background := p.renderTwoPaneLayout(p.height)

	p.ensureTaskModal()
	if p.taskModal == nil {
		return background
	}

	modalContent := p.taskModal.Render(p.width, p.height, p.taskModalMouseHandler)
	return ui.OverlayModal(background, modalContent, p.width, p.height)
}

// handleTaskModalKey handles keyboard input for the task modal.
func (p *Plugin) handleTaskModalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	p.ensureTaskModal()
	if p.taskModal == nil {
		return nil, false
	}

	action, cmd := p.taskModal.HandleKey(msg)
	switch action {
	case "create":
		return p.createTaskFromNote(), true
	case "cancel":
		p.closeTaskModal()
		return nil, true
	}

	return cmd, true
}

// handleTaskModalMouse handles mouse input for the task modal.
func (p *Plugin) handleTaskModalMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	p.ensureTaskModal()
	if p.taskModal == nil {
		return nil, false
	}

	action := p.taskModal.HandleMouse(msg, p.taskModalMouseHandler)
	switch action {
	case "create":
		return p.createTaskFromNote(), true
	case "cancel":
		p.closeTaskModal()
		return nil, true
	}

	return nil, true
}

// createTaskFromNote creates a td task from the current note.
func (p *Plugin) createTaskFromNote() tea.Cmd {
	if p.taskModalNote == nil {
		return nil
	}

	// Get values from form
	title := p.taskModalTitleInput.Value()
	if title == "" {
		title = "Untitled task"
	}

	taskType := taskTypes[p.taskModalTypeIdx]
	priority := strings.ToLower(priorityLevels[p.taskModalPriorityIdx])

	// Build description from note content
	desc := p.taskModalNote.Content

	// Store note ID for archive option
	noteID := p.taskModalNote.ID
	shouldArchive := p.taskModalArchiveNote
	epoch := p.ctx.Epoch

	// Close modal
	p.closeTaskModal()

	return func() tea.Msg {
		// Execute td create command
		args := []string{"create", title,
			"--type", taskType,
			"--priority", priority,
		}
		if desc != "" {
			args = append(args, "--description", desc)
		}

		cmd := exec.Command("td", args...)
		cmd.Dir = p.ctx.WorkDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return TaskCreatedMsg{TaskID: "", NoteID: noteID, Err: fmt.Errorf("td create failed: %s", string(output))}
		}

		// Parse task ID from output (format: "Created td-xxxxxxxx")
		taskID := parseTaskID(string(output))

		// Archive note if requested
		if shouldArchive && p.store != nil {
			_ = p.store.ToggleArchive(noteID)
		}

		return TaskCreatedMsg{TaskID: taskID, NoteID: noteID, Err: nil, Epoch: epoch}
	}
}

// parseTaskID extracts the task ID from td create output.
func parseTaskID(output string) string {
	// Look for pattern "Created td-xxxxxxxx" or just "td-xxxxxxxx"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Created ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
		// Also check for just the task ID
		if strings.HasPrefix(line, "td-") {
			return strings.Fields(line)[0]
		}
	}
	return ""
}

// showTaskCreatedToast shows a toast notification for task creation.
func showTaskCreatedToast(taskID string) tea.Cmd {
	msg := "Task created"
	if taskID != "" {
		msg = fmt.Sprintf("Created %s", taskID)
	}
	return appmsg.ShowToast(msg, 3*time.Second)
}
