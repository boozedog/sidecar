package notes

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

// ensureInfoModal builds the info modal if needed.
func (p *Plugin) ensureInfoModal() {
	if p.infoModalNote == nil {
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
	if p.infoModal != nil && p.infoModalWidth == modalW {
		return
	}

	p.infoModalWidth = modalW
	p.infoModal = modal.New("Note Info",
		modal.WithWidth(modalW),
		modal.WithHints(false),
	).
		AddSection(p.infoModalDetailsSection())
}

// clearInfoModal clears the info modal cache, forcing rebuild.
func (p *Plugin) clearInfoModal() {
	p.infoModal = nil
	p.infoModalWidth = 0
}

// openInfoModal opens the info modal for the selected note.
func (p *Plugin) openInfoModal() tea.Cmd {
	note := p.getSelectedNote()
	if note == nil {
		return nil
	}

	p.infoModalNote = note
	p.showInfoModal = true

	if p.infoModalMouseHandler == nil {
		p.infoModalMouseHandler = mouse.NewHandler()
	}

	p.clearInfoModal()
	return nil
}

// closeInfoModal closes the info modal and resets state.
func (p *Plugin) closeInfoModal() {
	p.showInfoModal = false
	p.infoModalNote = nil
	p.infoModal = nil
	p.infoModalWidth = 0
}

// renderInfoModal renders the info modal overlaid on the main view.
func (p *Plugin) renderInfoModal() string {
	background := p.renderTwoPaneLayout(p.height)

	p.ensureInfoModal()
	if p.infoModal == nil {
		return background
	}

	modalContent := p.infoModal.Render(p.width, p.height, p.infoModalMouseHandler)
	return ui.OverlayModal(background, modalContent, p.width, p.height)
}

// handleInfoModalKey handles keyboard input for the info modal.
func (p *Plugin) handleInfoModalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	p.ensureInfoModal()
	if p.infoModal == nil {
		return nil, false
	}

	action, cmd := p.infoModal.HandleKey(msg)
	switch action {
	case "close":
		p.closeInfoModal()
		return nil, true
	}

	// Also close on Esc or Enter
	key := msg.String()
	if key == "esc" || key == "enter" {
		p.closeInfoModal()
		return nil, true
	}

	return cmd, true
}

// handleInfoModalMouse handles mouse input for the info modal.
func (p *Plugin) handleInfoModalMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	p.ensureInfoModal()
	if p.infoModal == nil {
		return nil, false
	}

	action := p.infoModal.HandleMouse(msg, p.infoModalMouseHandler)
	switch action {
	case "close":
		p.closeInfoModal()
		return nil, true
	}

	return nil, true
}

// infoModalDetailsSection renders the note metadata fields.
func (p *Plugin) infoModalDetailsSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		note := p.infoModalNote
		if note == nil {
			return modal.RenderedSection{Content: styles.Muted.Render("No note selected")}
		}

		// Get note title
		title := note.Title
		if title == "" {
			lines := strings.SplitN(note.Content, "\n", 2)
			if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
				title = strings.TrimSpace(lines[0])
			} else {
				title = "untitled"
			}
		}

		// Truncate title if too long
		maxTitleLen := contentWidth - 10
		if maxTitleLen < 10 {
			maxTitleLen = 10
		}
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}

		// Format timestamps
		createdAt := note.CreatedAt.Format("Jan 2, 2006 at 15:04")
		updatedAt := note.UpdatedAt.Format("Jan 2, 2006 at 15:04")

		// Format boolean fields
		pinnedStr := "No"
		if note.Pinned {
			pinnedStr = "Yes"
		}
		archivedStr := "No"
		if note.Archived {
			archivedStr = "Yes"
		}

		labelStyle := styles.Muted.Width(10).Align(lipgloss.Right).MarginRight(2)
		valueStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)

		fields := []struct{ label, value string }{
			{"Title:", title},
			{"Created:", createdAt},
			{"Updated:", updatedAt},
			{"Pinned:", pinnedStr},
			{"Archived:", archivedStr},
		}

		var sb strings.Builder
		for i, f := range fields {
			line := lipgloss.JoinHorizontal(lipgloss.Top,
				labelStyle.Render(f.label),
				valueStyle.Render(f.value),
			)
			sb.WriteString(line)
			if i < len(fields)-1 {
				sb.WriteString("\n")
			}
		}

		return modal.RenderedSection{Content: sb.String()}
	}, nil)
}
