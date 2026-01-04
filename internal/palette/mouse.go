package palette

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/mouse"
)

// Mouse region identifiers
const (
	regionPaletteEntry = "palette-entry" // Individual command entry (Data: entry index int)
)

// handleMouse processes mouse events for the command palette.
func (m *Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	// Calculate modal position (same logic as View)
	modalWidth := min(80, m.width-4)
	if modalWidth < 40 {
		modalWidth = 40
	}

	// Modal height = header (3 lines) + visible entries + scroll indicators + borders/padding
	// Approximate: header(3) + maxVisible + scroll hints(2) + borders(4)
	modalHeight := 3 + m.maxVisible + 6

	modalX := (m.width - modalWidth) / 2
	modalY := (m.height - modalHeight) / 2

	// Translate to modal-relative coordinates
	relX := msg.X - modalX
	relY := msg.Y - modalY

	// Ignore clicks outside modal bounds
	if relX < 0 || relY < 0 || relX >= modalWidth || relY >= modalHeight {
		return *m, nil
	}

	// Create adjusted message for hit testing
	adjusted := tea.MouseMsg{
		X:      relX,
		Y:      relY,
		Button: msg.Button,
		Action: msg.Action,
	}

	action := m.mouseHandler.HandleMouse(adjusted)

	switch action.Type {
	case mouse.ActionClick:
		return m.handleMouseClick(action)
	case mouse.ActionDoubleClick:
		return m.handleMouseDoubleClick(action)
	case mouse.ActionScrollUp, mouse.ActionScrollDown:
		return m.handleMouseScroll(action)
	}

	return *m, nil
}

// handleMouseClick handles single click on palette entries.
func (m *Model) handleMouseClick(action mouse.MouseAction) (Model, tea.Cmd) {
	if action.Region == nil || action.Region.ID != regionPaletteEntry {
		return *m, nil
	}

	if idx, ok := action.Region.Data.(int); ok {
		m.cursor = idx
		m.ensureCursorVisible()
	}

	return *m, nil
}

// handleMouseDoubleClick handles double click to execute command.
func (m *Model) handleMouseDoubleClick(action mouse.MouseAction) (Model, tea.Cmd) {
	if action.Region == nil || action.Region.ID != regionPaletteEntry {
		return *m, nil
	}

	if idx, ok := action.Region.Data.(int); ok {
		m.cursor = idx
		// Execute the selected command
		if entry := m.SelectedEntry(); entry != nil {
			return *m, func() tea.Msg {
				return CommandSelectedMsg{
					CommandID: entry.CommandID,
					Context:   entry.Context,
				}
			}
		}
	}

	return *m, nil
}

// handleMouseScroll handles scroll wheel for navigation.
func (m *Model) handleMouseScroll(action mouse.MouseAction) (Model, tea.Cmd) {
	delta := 3
	if action.Type == mouse.ActionScrollUp {
		delta = -3
	}
	m.moveCursor(delta)
	return *m, nil
}

// ensureCursorVisible adjusts offset to keep cursor in view.
func (m *Model) ensureCursorVisible() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.maxVisible {
		m.offset = m.cursor - m.maxVisible + 1
	}
}
