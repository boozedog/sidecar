package notes

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/ui"
)

// ensureDeleteModal builds the delete modal if needed.
// Must be called in both View() and Update() handlers.
func (p *Plugin) ensureDeleteModal() {
	if p.deleteModalNote == nil {
		return
	}

	// Calculate modal width
	modalW := 50
	if modalW > p.width-4 {
		modalW = p.width - 4
	}
	if modalW < 30 {
		modalW = 30
	}

	// Only rebuild if modal doesn't exist or width changed
	if p.deleteModal != nil && p.deleteModalWidth == modalW {
		return
	}

	p.deleteModalWidth = modalW

	// Get note title for display
	title := p.deleteModalNote.Title
	if title == "" {
		// Use first line of content as title
		lines := strings.SplitN(p.deleteModalNote.Content, "\n", 2)
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			title = strings.TrimSpace(lines[0])
		} else {
			title = "untitled"
		}
	}

	// Truncate title if too long
	maxTitleLen := modalW - 10
	if len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	p.deleteModal = modal.New("Delete Note?",
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantDanger),
		modal.WithPrimaryAction("delete"),
	).
		AddSection(modal.Text("Delete '" + title + "'?")).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Delete ", "delete", modal.BtnDanger()),
			modal.Btn(" Cancel ", "cancel"),
		))
}

// clearDeleteModal clears the delete modal cache, forcing rebuild.
func (p *Plugin) clearDeleteModal() {
	p.deleteModal = nil
	p.deleteModalWidth = 0
}

// openDeleteModal opens the delete confirmation modal for the selected note.
func (p *Plugin) openDeleteModal() tea.Cmd {
	note := p.getSelectedNote()
	if note == nil {
		return nil
	}

	// Store reference to note being deleted
	p.deleteModalNote = note
	p.showDeleteModal = true

	// Initialize mouse handler
	if p.deleteModalMouseHandler == nil {
		p.deleteModalMouseHandler = mouse.NewHandler()
	}

	// Clear modal cache to rebuild with new data
	p.clearDeleteModal()

	return nil
}

// closeDeleteModal closes the delete modal and resets state.
func (p *Plugin) closeDeleteModal() {
	p.showDeleteModal = false
	p.deleteModalNote = nil
	p.deleteModal = nil
	p.deleteModalWidth = 0
}

// renderDeleteModal renders the delete modal overlaid on the main view.
func (p *Plugin) renderDeleteModal() string {
	background := p.renderTwoPaneLayout(p.height)

	p.ensureDeleteModal()
	if p.deleteModal == nil {
		return background
	}

	modalContent := p.deleteModal.Render(p.width, p.height, p.deleteModalMouseHandler)
	return ui.OverlayModal(background, modalContent, p.width, p.height)
}

// handleDeleteModalKey handles keyboard input for the delete modal.
func (p *Plugin) handleDeleteModalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	p.ensureDeleteModal()
	if p.deleteModal == nil {
		return nil, false
	}

	action, cmd := p.deleteModal.HandleKey(msg)
	switch action {
	case "delete":
		return p.confirmDeleteNote(), true
	case "cancel":
		p.closeDeleteModal()
		return nil, true
	}

	return cmd, true
}

// handleDeleteModalMouse handles mouse input for the delete modal.
func (p *Plugin) handleDeleteModalMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	p.ensureDeleteModal()
	if p.deleteModal == nil {
		return nil, false
	}

	action := p.deleteModal.HandleMouse(msg, p.deleteModalMouseHandler)
	switch action {
	case "delete":
		return p.confirmDeleteNote(), true
	case "cancel":
		p.closeDeleteModal()
		return nil, true
	}

	return nil, true
}

// confirmDeleteNote deletes the note and closes the modal.
func (p *Plugin) confirmDeleteNote() tea.Cmd {
	if p.deleteModalNote == nil || p.store == nil {
		p.closeDeleteModal()
		return nil
	}

	noteID := p.deleteModalNote.ID
	epoch := p.ctx.Epoch

	// Close modal
	p.closeDeleteModal()

	return func() tea.Msg {
		err := p.store.Delete(noteID)
		return NoteDeletedMsg{ID: noteID, Err: err, Epoch: epoch}
	}
}
