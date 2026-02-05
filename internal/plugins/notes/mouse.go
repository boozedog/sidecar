package notes

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/marcus/sidecar/internal/app"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/state"
)

// dragForwardThrottle is the minimum interval between forwarding mouse drag
// events to the tmux session for inline editor text selection.
// ~60fps to prevent subprocess spam (each forward spawns tmux send-keys).
const dragForwardThrottle = 16 * time.Millisecond

// Mouse region identifiers
const (
	regionListPane   = "list-pane"   // Overall list pane for scroll targeting
	regionEditorPane = "editor-pane" // Overall editor pane for scroll targeting
	regionDivider    = "divider"     // Border between list and editor
	regionNoteItem   = "note-item"   // Individual note in list (Data: visible index)
	regionEditorLine = "editor-line" // Individual editor line (Data: line index)
)

// handleMouse processes mouse events and dispatches to appropriate handlers.
func (p *Plugin) handleMouse(msg tea.MouseMsg) (*Plugin, tea.Cmd) {
	// Handle exit confirmation dialog if active
	if p.showExitConfirmation {
		return p.handleExitConfirmationMouse(msg)
	}

	// Handle inline edit mode - detect click-away and forward mouse events for text selection
	if p.inlineEditMode && p.inlineEditor != nil && p.inlineEditor.IsActive() {
		action := p.mouseHandler.HandleMouse(msg)

		// Helper to handle click-away: auto-save and switch to clicked note
		handleClickAway := func(regionID string, regionData interface{}) (*Plugin, tea.Cmd) {
			p.inlineEditorDragging = false // Cancel any drag in progress

			// Check if the editor session is still alive
			if !p.isInlineEditSessionAlive() {
				// Session is dead - just clean up and process click
				p.exitInlineEditMode()
				p.pendingClickRegion = regionID
				p.pendingClickData = regionData
				return p.processPendingClickAction()
			}

			// Session is alive - auto-save and exit (no confirmation needed)
			// Store pending click info to process after save
			p.pendingClickRegion = regionID
			p.pendingClickData = regionData

			// Save current content and exit
			saveCmd := p.saveAndExitInlineEditMode()

			// Process the click action immediately
			p2, _ := p.processPendingClickAction()

			return p2, saveCmd
		}

		// Handle click (mouse press) - start potential drag
		if action.Type == mouse.ActionClick {
			if action.Region != nil {
				switch action.Region.ID {
				case regionNoteItem, regionListPane:
					// Click in list pane - auto-save and switch
					return handleClickAway(action.Region.ID, action.Region.Data)
				case regionEditorPane, regionEditorLine:
					// Forward mouse press to vim and start tracking drag
					col, row, ok := p.calculateInlineEditorMouseCoords(action.X, action.Y)
					if ok {
						p.inlineEditorDragging = true
						p.lastDragForwardTime = time.Time{}
						return p, p.forwardMousePressToInlineEditor(col, row)
					}
					cmd := p.inlineEditor.Update(msg)
					return p, cmd
				}
			}

			// Fallback: use X position to detect list pane clicks
			if action.X < p.listWidth {
				return handleClickAway(regionListPane, nil)
			}
		}

		// Handle mouse motion/hover - forward drag events to vim for text selection.
		// Throttled to ~60fps to prevent subprocess spam (each forward spawns tmux send-keys).
		if action.Type == mouse.ActionHover && p.inlineEditorDragging {
			now := time.Now()
			if now.Sub(p.lastDragForwardTime) < dragForwardThrottle {
				return p, nil
			}
			col, row, ok := p.calculateInlineEditorMouseCoords(action.X, action.Y)
			if ok {
				p.lastDragForwardTime = now
				return p, p.forwardMouseDragToInlineEditor(col, row)
			}
		}

		// Handle mouse release - end drag
		if msg.Action == tea.MouseActionRelease {
			if p.inlineEditorDragging {
				p.inlineEditorDragging = false
				p.lastDragForwardTime = time.Time{}
				col, row, ok := p.calculateInlineEditorMouseCoords(msg.X, msg.Y)
				if ok {
					return p, p.forwardMouseReleaseToInlineEditor(col, row)
				}
			}
			return p, nil
		}

		// Forward other mouse events to tty model
		cmd := p.inlineEditor.Update(msg)
		return p, cmd
	}

	action := p.mouseHandler.HandleMouse(msg)

	switch action.Type {
	case mouse.ActionClick:
		return p.handleMouseClick(action)
	case mouse.ActionDoubleClick:
		return p.handleMouseDoubleClick(action)
	case mouse.ActionScrollUp, mouse.ActionScrollDown:
		return p.handleMouseScroll(action)
	case mouse.ActionDrag:
		return p.handleMouseDrag(action)
	case mouse.ActionDragEnd:
		return p.handleMouseDragEnd()
	case mouse.ActionHover:
		return p.handleMouseHover(action)
	}
	return p, nil
}

// handleExitConfirmationMouse handles mouse events in the exit confirmation dialog.
func (p *Plugin) handleExitConfirmationMouse(msg tea.MouseMsg) (*Plugin, tea.Cmd) {
	// For now, clicks anywhere in the confirmation just select the option under cursor
	// The keyboard handling does the main interaction
	return p, nil
}

// handleMouseClick handles single click actions.
func (p *Plugin) handleMouseClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	if action.Region == nil {
		return p, nil
	}

	switch action.Region.ID {
	case regionNoteItem:
		idx, ok := action.Region.Data.(int)
		if !ok {
			return p, nil
		}
		p.cursor = idx
		p.activePane = PaneList
		p.loadNoteIntoEditor()
		return p, nil

	case regionListPane:
		p.activePane = PaneList
		p.selection.Clear()
		return p, nil

	case regionEditorPane:
		p.activePane = PaneEditor
		p.selection.Clear()
		// Enter edit mode when clicking editor pane (only for Active notes)
		if p.viewFilter == FilterActive {
			// Check if default editor is vim/nvim - use tty.Model inline editor
			if p.isDefaultEditorVim() {
				note := p.getSelectedNote()
				if note != nil && p.isInlineEditSupported() {
					return p, p.enterInlineEditMode(note.ID)
				}
			}
			wasPreview := p.previewMode
			p.previewMode = false
			var cmd tea.Cmd
			if wasPreview {
				cmd = p.editorTextarea.Focus()
			}
			// Position cursor at click location
			clickedRow := p.screenYToEditorLine(action.Y)
			clickedCol := p.screenXToEditorCol(action.X)
			p.setTextareaCursorPosition(clickedRow, clickedCol)
			p.trackTextareaScroll()
			// Prepare drag-to-select (use regionEditorLine for drag dispatch)
			p.selection.PrepareDrag(clickedRow, clickedCol, action.Region.Rect)
			p.mouseHandler.StartDrag(action.X, action.Y, regionEditorLine, 0)
			return p, cmd
		}
		return p, nil

	case regionEditorLine:
		if lineIdx, ok := action.Region.Data.(int); ok {
			p.activePane = PaneEditor
			if p.viewFilter == FilterActive {
				if p.isDefaultEditorVim() {
					note := p.getSelectedNote()
					if note != nil && p.isInlineEditSupported() {
						return p, p.enterInlineEditMode(note.ID)
					}
				}
				wasPreview := p.previewMode
				p.previewMode = false
				var cmd tea.Cmd
				if wasPreview {
					cmd = p.editorTextarea.Focus()
				}
				// Position cursor at clicked line and column
				col := p.screenXToEditorCol(action.X)
				p.setTextareaCursorPosition(lineIdx, col)
				p.trackTextareaScroll()
				// Prepare drag-to-select (same as preview mode)
				p.selection.PrepareDrag(lineIdx, col, action.Region.Rect)
				p.mouseHandler.StartDrag(action.X, action.Y, regionEditorLine, lineIdx)
				return p, cmd
			}
			// Preview mode: position cursor and prepare selection
			p.previewCursorLine = lineIdx
			col := p.editorColAtScreenX(action.X, lineIdx)
			p.selection.PrepareDrag(lineIdx, col, action.Region.Rect)
			p.mouseHandler.StartDrag(action.X, action.Y, regionEditorLine, lineIdx)
		}
		return p, nil

	case regionDivider:
		// Start drag with current list width
		p.mouseHandler.StartDrag(action.X, action.Y, regionDivider, p.listWidth)
		return p, nil
	}

	return p, nil
}

// handleMouseDoubleClick handles double click actions.
func (p *Plugin) handleMouseDoubleClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	if action.Region == nil {
		return p, nil
	}

	switch action.Region.ID {
	case regionNoteItem:
		idx, ok := action.Region.Data.(int)
		if !ok {
			return p, nil
		}
		p.cursor = idx
		p.loadNoteIntoEditor()
		p.activePane = PaneEditor
		return p, nil

	case regionEditorLine:
		if lineIdx, ok := action.Region.Data.(int); ok {
			p.activePane = PaneEditor
			if !p.previewMode {
				// Edit mode: position textarea cursor at double-click location
				col := p.screenXToEditorCol(action.X)
				p.setTextareaCursorPosition(lineIdx, col)
				p.trackTextareaScroll()
			} else {
				// Preview mode: position preview cursor
				p.previewCursorLine = lineIdx
			}
		}
		return p, nil
	}

	return p, nil
}

// handleMouseScroll handles scroll wheel actions.
func (p *Plugin) handleMouseScroll(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	inListPane := false
	if action.Region != nil {
		inListPane = action.Region.ID == regionListPane || action.Region.ID == regionNoteItem
	} else {
		inListPane = action.X < p.listWidth
	}

	delta := 3
	if action.Type == mouse.ActionScrollUp {
		delta = -3
	}

	if inListPane {
		// Scroll list by moving cursor
		notesList := p.getDisplayNotes()
		p.cursor += delta
		if p.cursor < 0 {
			p.cursor = 0
		} else if p.cursor >= len(notesList) {
			p.cursor = len(notesList) - 1
		}
		p.loadNoteIntoEditor()
		return p, nil
	}

	// Scroll editor pane
	if p.editorNote == nil {
		return p, nil
	}

	if p.previewMode {
		if len(p.previewLines) == 0 {
			return p, nil
		}
		p.previewScrollOff += delta
		if p.previewScrollOff < 0 {
			p.previewScrollOff = 0
		}
	} else {
		// In edit mode, forward scroll as cursor movement to textarea
		var cmd tea.Cmd
		for i := 0; i < 3; i++ {
			if delta > 0 {
				p.editorTextarea, cmd = p.editorTextarea.Update(tea.KeyMsg{Type: tea.KeyDown})
			} else {
				p.editorTextarea, cmd = p.editorTextarea.Update(tea.KeyMsg{Type: tea.KeyUp})
			}
		}
		p.trackTextareaScroll()
		return p, cmd
	}
	return p, nil
}

// handleMouseDrag handles drag actions (pane resizing and text selection).
func (p *Plugin) handleMouseDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	switch p.mouseHandler.DragRegion() {
	case regionDivider:
		return p.handleDividerDrag(action)
	case regionEditorLine:
		return p.handleEditorSelectionDrag(action)
	}
	return p, nil
}

// handleDividerDrag handles dragging the pane divider to resize.
func (p *Plugin) handleDividerDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	startValue := p.mouseHandler.DragStartValue()
	newWidth := startValue + action.DragDX

	// Clamp to reasonable bounds
	available := p.width - dividerWidth
	minWidth := 20
	maxWidth := available - 40 // Leave at least 40 for editor
	if maxWidth < minWidth {
		maxWidth = minWidth
	}
	if newWidth < minWidth {
		newWidth = minWidth
	} else if newWidth > maxWidth {
		newWidth = maxWidth
	}

	p.listWidth = newWidth
	return p, nil
}

// handleEditorSelectionDrag handles drag-to-select in the editor pane.
func (p *Plugin) handleEditorSelectionDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	if p.editorNote == nil {
		return p, nil
	}

	// Get line count based on mode
	var lineCount int
	if p.previewMode {
		lineCount = len(p.previewLines)
	} else {
		lineCount = p.editorTextarea.LineCount()
		// Keep previewLines synced for getSelectedText
		p.syncPreviewFromTextarea()
	}
	if lineCount == 0 {
		return p, nil
	}

	// Calculate Y offset to editor content
	editorContentStartY := p.editorContentStartY()
	currentLine := (action.Y - editorContentStartY) + p.previewScrollOff

	// Clamp to valid range
	if currentLine < 0 {
		currentLine = 0
	}
	maxLine := lineCount - 1
	if currentLine > maxLine {
		currentLine = maxLine
	}

	// Map X to character column (mode-aware)
	var col int
	if p.previewMode {
		col = p.editorColAtScreenX(action.X, currentLine)
	} else {
		col = p.screenXToEditorCol(action.X)
	}

	// Update selection
	p.selection.HandleDrag(currentLine, col)

	return p, nil
}

// handleMouseDragEnd handles the end of a drag operation.
func (p *Plugin) handleMouseDragEnd() (*Plugin, tea.Cmd) {
	switch p.mouseHandler.DragRegion() {
	case regionDivider:
		// Save the current list width to state
		_ = state.SetNotesListWidth(p.listWidth)
	case regionEditorLine:
		// Selection complete - finalize drag and copy to clipboard
		p.selection.FinishDrag()
		if p.selection.HasSelection() {
			return p, p.copySelectionCmd()
		}
	}
	return p, nil
}

// handleMouseHover handles mouse hover for visual feedback.
func (p *Plugin) handleMouseHover(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	// No specific hover behavior needed for now
	return p, nil
}

// editorColAtScreenX maps a screen X coordinate to a visual column within
// the editor content for the given line index.
func (p *Plugin) editorColAtScreenX(x, lineIdx int) int {
	// Calculate the X offset where editor content starts:
	// list pane width + divider + editor border + line number width
	lineNumWidth := p.lineNumberWidth()
	editorContentX := p.listWidth + dividerWidth + 1 + lineNumWidth + 1 // +1 for border, +1 for space after line num

	relX := x - editorContentX
	if relX < 0 {
		relX = 0
	}

	// Get the raw line
	if lineIdx < 0 || lineIdx >= len(p.previewLines) {
		return 0
	}

	// Notes uses plain text, so just return relX clamped to line length
	line := p.previewLines[lineIdx]
	if relX > len(line) {
		return len(line)
	}
	return relX
}

// editorContentStartY returns the Y coordinate where editor content begins.
func (p *Plugin) editorContentStartY() int {
	// 1 for top border + 1 for header line
	return 2
}

// screenYToEditorLine converts a screen Y coordinate to an editor line index.
// Uses previewScrollOff which is kept in sync with the textarea viewport
// via trackTextareaScroll().
func (p *Plugin) screenYToEditorLine(y int) int {
	editorContentY := p.editorContentStartY()
	visualRow := y - editorContentY
	if visualRow < 0 {
		visualRow = 0
	}
	line := p.previewScrollOff + visualRow
	lineCount := p.editorTextarea.LineCount()
	if lineCount == 0 {
		return 0
	}
	if line >= lineCount {
		line = lineCount - 1
	}
	if line < 0 {
		line = 0
	}
	return line
}

// screenXToEditorCol converts a screen X coordinate to a column in editor content.
// In edit mode, the textarea renders its own line numbers with a hardcoded width of 4
// (see bubbles/textarea SetWidth: const lnWidth = 4). The panel adds 1 border + 1 padding.
// In preview mode, uses the same formula as editorColAtScreenX (proven working).
func (p *Plugin) screenXToEditorCol(x int) int {
	if p.previewMode {
		// Same formula as editorColAtScreenX (used for preview mode selection)
		lineNumWidth := p.lineNumberWidth()
		editorContentX := p.listWidth + dividerWidth + 1 + lineNumWidth + 1
		relX := x - editorContentX
		if relX < 0 {
			relX = 0
		}
		return relX
	}
	// Edit mode: panel border(1) + padding(1) + textarea line numbers(4)
	editorContentX := p.listWidth + dividerWidth + 2 + 4
	relX := x - editorContentX
	if relX < 0 {
		relX = 0
	}
	return relX
}

// lineNumberWidth returns the width used for line numbers in the editor.
func (p *Plugin) lineNumberWidth() int {
	var count int
	if p.previewMode {
		count = len(p.previewLines)
	} else {
		count = p.editorTextarea.LineCount()
	}
	if count == 0 {
		return 2
	}
	w := len(fmt.Sprintf("%d", count))
	if w < 2 {
		w = 2
	}
	return w
}

// copySelectionCmd returns a command that copies the selection to clipboard.
func (p *Plugin) copySelectionCmd() tea.Cmd {
	return func() tea.Msg {
		lines := p.getSelectedText()
		if len(lines) == 0 {
			return nil
		}

		// Strip ANSI codes and join
		stripped := make([]string, 0, len(lines))
		for _, line := range lines {
			stripped = append(stripped, ansi.Strip(line))
		}
		text := strings.Join(stripped, "\n")

		if err := clipboard.WriteAll(text); err != nil {
			return app.ToastMsg{Message: "Copy failed: " + err.Error(), Duration: 2 * time.Second, IsError: true}
		}

		return app.ToastMsg{Message: "Copied to clipboard", Duration: 2 * time.Second}
	}
}

// getSelectedText returns the selected text lines.
func (p *Plugin) getSelectedText() []string {
	if !p.selection.HasSelection() || p.editorNote == nil {
		return nil
	}

	startLine := p.selection.Start.Line
	endLine := p.selection.End.Line

	// Get lines based on mode
	var allLines []string
	if p.previewMode {
		allLines = p.previewLines
	} else {
		// In edit mode, split textarea content into lines
		allLines = strings.Split(p.editorTextarea.Value(), "\n")
	}

	if startLine < 0 || endLine >= len(allLines) {
		return nil
	}

	// Get the lines in selection range
	lines := allLines[startLine : endLine+1]
	if len(lines) == 0 {
		return nil
	}

	// Use shared selection API to extract with column precision
	return p.selection.SelectedText(lines, startLine, 8) // tabWidth=8
}

// registerMouseRegions registers hit regions for mouse interaction.
// Called during View() to update regions based on current layout.
func (p *Plugin) registerMouseRegions() {
	p.mouseHandler.Clear()

	// Skip if dimensions not set
	if p.width == 0 || p.height == 0 {
		return
	}

	// Calculate layout
	p.calculatePaneWidths()

	// IMPORTANT: Add general regions FIRST, specific regions LAST
	// (they get tested in reverse order, so last = highest priority)

	// General pane regions (lower priority)
	p.mouseHandler.HitMap.AddRect(regionListPane, 0, 0, p.listWidth, p.height, nil)
	editorX := p.listWidth + dividerWidth
	editorWidth := p.width - editorX
	p.mouseHandler.HitMap.AddRect(regionEditorPane, editorX, 0, editorWidth, p.height, nil)

	// Divider region
	p.mouseHandler.HitMap.AddRect(regionDivider, p.listWidth, 0, dividerWidth, p.height, nil)

	// Note items in list (higher priority)
	p.registerListItemRegions()

	// Editor line regions (higher priority)
	p.registerEditorLineRegions()
}

// registerListItemRegions registers click regions for visible note items.
func (p *Plugin) registerListItemRegions() {
	displayNotes := p.getDisplayNotes()
	if len(displayNotes) == 0 {
		return
	}

	// Calculate visible range
	headerLines := 1 // "Notes (count)"
	if p.searchMode || p.searchQuery != "" {
		headerLines = 2 // + search input
	}
	contentHeight := p.height - 2 - headerLines // -2 for borders
	if contentHeight < 1 {
		contentHeight = 1
	}

	start := p.scrollOff
	end := start + contentHeight
	if end > len(displayNotes) {
		end = len(displayNotes)
	}

	// Y offset: border + header lines
	yOffset := 1 + headerLines

	for i := start; i < end; i++ {
		row := i - start
		// Each note item takes 1 row
		p.mouseHandler.HitMap.AddRect(
			regionNoteItem,
			0,           // x
			yOffset+row, // y
			p.listWidth, // width
			1,           // height (1 row per item)
			i,           // data = note index
		)
	}
}

// registerEditorLineRegions registers click regions for visible editor lines.
func (p *Plugin) registerEditorLineRegions() {
	if p.editorNote == nil {
		return
	}

	// Determine line count based on mode
	var lineCount int
	if p.previewMode {
		lineCount = len(p.previewLines)
	} else {
		lineCount = p.editorTextarea.LineCount()
	}
	if lineCount == 0 {
		return
	}

	// Calculate editor pane position
	editorX := p.listWidth + dividerWidth

	// Calculate visible range
	headerLines := 1                            // Title line
	contentHeight := p.height - 2 - headerLines // -2 for borders
	editorWidth := p.width - editorX - 2        // -2 for borders

	if contentHeight < 1 {
		contentHeight = 1
	}

	start := p.previewScrollOff
	end := start + contentHeight
	if end > lineCount {
		end = lineCount
	}

	// Y offset: border + header
	yOffset := 1 + headerLines

	for i := start; i < end; i++ {
		row := i - start
		rect := mouse.Rect{
			X: editorX + 1, // +1 for border
			Y: yOffset + row,
			W: editorWidth,
			H: 1,
		}
		p.mouseHandler.HitMap.Add(regionEditorLine, rect, i)
	}
}
