package filebrowser

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/state"
)

// Mouse region identifiers
const (
	regionTreePane    = "tree-pane"    // Overall tree pane for scroll targeting
	regionPreviewPane = "preview-pane" // Overall preview pane for scroll targeting
	regionPaneDivider = "pane-divider" // Border between tree and preview
	regionTreeItem    = "tree-item"    // Individual file/folder (Data: visible index)
	regionQuickOpen   = "quick-open"   // Quick open modal item (Data: match index)
	regionPreviewLine = "preview-line" // Individual preview line (Data: line index)

	// Project search regions
	regionSearchToggleRegex = "search-toggle-regex" // Regex toggle button
	regionSearchToggleCase  = "search-toggle-case"  // Case sensitivity toggle
	regionSearchToggleWord  = "search-toggle-word"  // Whole word toggle
	regionSearchInput       = "search-input"        // Search input area
	regionSearchFile        = "search-file"         // File header (Data: file index)
	regionSearchMatch       = "search-match"        // Match line (Data: searchMatchData)
	regionSearchResults     = "search-results"      // Results pane for scrolling

	// File operation modal buttons
	regionFileOpConfirm     = "file-op-confirm"     // Confirm/Create/Delete/Yes button
	regionFileOpCancel      = "file-op-cancel"      // Cancel/No button
	regionFileOpSuggestion  = "file-op-suggestion"  // Path suggestion item (Data: index)
)

// searchMatchData holds indices for a search match region.
type searchMatchData struct {
	FileIdx  int
	MatchIdx int
}

// handleMouse processes mouse events and dispatches to appropriate handlers.
func (p *Plugin) handleMouse(msg tea.MouseMsg) (*Plugin, tea.Cmd) {
	// Handle project search modal first if active
	if p.projectSearchMode {
		return p.handleProjectSearchMouse(msg)
	}

	// Handle quick open modal if active
	if p.quickOpenMode {
		return p.handleQuickOpenMouse(msg)
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

// handleMouseHover handles mouse hover for visual feedback.
func (p *Plugin) handleMouseHover(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	// Only track hover for file operation modal buttons
	if p.fileOpMode == FileOpNone {
		p.fileOpButtonHover = 0
		return p, nil
	}

	if action.Region == nil {
		p.fileOpButtonHover = 0
		return p, nil
	}

	switch action.Region.ID {
	case regionFileOpConfirm:
		p.fileOpButtonHover = 1
	case regionFileOpCancel:
		p.fileOpButtonHover = 2
	case regionFileOpSuggestion:
		// Highlight suggestion on hover
		if idx, ok := action.Region.Data.(int); ok {
			p.fileOpSuggestionIdx = idx
		}
		p.fileOpButtonHover = 0
	default:
		p.fileOpButtonHover = 0
	}
	return p, nil
}

// handleMouseClick handles single click actions.
func (p *Plugin) handleMouseClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	if action.Region == nil {
		return p, nil
	}

	switch action.Region.ID {
	case regionTreeItem:
		idx, ok := action.Region.Data.(int)
		if !ok {
			return p, nil
		}
		p.treeCursor = idx
		p.activePane = PaneTree
		p.ensureTreeCursorVisible()
		return p, p.loadPreviewForCursor()

	case regionTreePane:
		p.activePane = PaneTree
		return p, nil

	case regionPreviewPane:
		p.activePane = PanePreview
		p.clearTextSelection() // Clear selection when clicking empty area
		return p, nil

	case regionPreviewLine:
		if lineIdx, ok := action.Region.Data.(int); ok {
			p.activePane = PanePreview
			// Initialize selection on this single line
			p.textSelectionActive = false
			p.textSelectionStart = lineIdx
			p.textSelectionEnd = lineIdx
			p.textSelectionAnchor = lineIdx
			// Start drag tracking for potential drag-select
			p.mouseHandler.StartDrag(action.X, action.Y, regionPreviewLine, lineIdx)
		}
		return p, nil

	case regionPaneDivider:
		// Start drag with current tree width
		p.mouseHandler.StartDrag(action.X, action.Y, regionPaneDivider, p.treeWidth)
		return p, nil

	case regionFileOpConfirm:
		// Click on confirm button in file op modal
		if p.fileOpMode != FileOpNone {
			plug, cmd := p.executeFileOp()
			return plug.(*Plugin), cmd
		}
		return p, nil

	case regionFileOpCancel:
		// Click on cancel button in file op modal
		if p.fileOpMode != FileOpNone {
			p.fileOpMode = FileOpNone
			p.fileOpTarget = nil
			p.fileOpError = ""
			p.fileOpShowSuggestions = false
			p.fileOpConfirmDelete = false
			p.fileOpConfirmCreate = false
			return p, nil
		}
		return p, nil

	case regionFileOpSuggestion:
		// Click on a path suggestion item
		if idx, ok := action.Region.Data.(int); ok {
			if idx >= 0 && idx < len(p.fileOpSuggestions) {
				p.fileOpTextInput.SetValue(p.fileOpSuggestions[idx])
				p.fileOpShowSuggestions = false
				p.fileOpSuggestionIdx = -1
			}
		}
		return p, nil
	}

	return p, nil
}

// handleMouseDoubleClick handles double click actions.
func (p *Plugin) handleMouseDoubleClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	if action.Region == nil || action.Region.ID != regionTreeItem {
		return p, nil
	}

	idx, ok := action.Region.Data.(int)
	if !ok {
		return p, nil
	}

	node := p.tree.GetNode(idx)
	if node == nil {
		return p, nil
	}

	if node.IsDir {
		// Toggle folder expand/collapse
		_ = p.tree.Toggle(node)
		p.treeCursor = idx
		p.ensureTreeCursorVisible()
		return p, nil
	}

	// Open file in editor (same as 'e' key)
	return p, p.openFile(node.Path)
}

// handleMouseScroll handles scroll wheel actions.
func (p *Plugin) handleMouseScroll(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	// Determine which pane to scroll based on region or X position
	inTreePane := false
	if action.Region != nil {
		inTreePane = action.Region.ID == regionTreePane || action.Region.ID == regionTreeItem
	} else {
		inTreePane = action.X < p.treeWidth
	}

	delta := 3
	if action.Type == mouse.ActionScrollUp {
		delta = -3
	}

	if inTreePane {
		// Scroll tree by moving cursor
		p.treeCursor += delta
		if p.treeCursor < 0 {
			p.treeCursor = 0
		} else if p.treeCursor >= p.tree.Len() {
			p.treeCursor = p.tree.Len() - 1
		}
		p.ensureTreeCursorVisible()
		return p, p.loadPreviewForCursor()
	}

	// Scroll preview pane
	lines := p.getPreviewLines()
	visibleHeight := p.visibleContentHeight()
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	p.previewScroll += delta
	if p.previewScroll < 0 {
		p.previewScroll = 0
	} else if p.previewScroll > maxScroll {
		p.previewScroll = maxScroll
	}

	return p, nil
}

// handleMouseDrag handles drag actions (pane resizing and text selection).
func (p *Plugin) handleMouseDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	switch p.mouseHandler.DragRegion() {
	case regionPaneDivider:
		return p.handlePaneDividerDrag(action)
	case regionPreviewLine:
		return p.handlePreviewSelectionDrag(action)
	}
	return p, nil
}

// handlePaneDividerDrag handles dragging the pane divider to resize.
func (p *Plugin) handlePaneDividerDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	startValue := p.mouseHandler.DragStartValue()
	newWidth := startValue + action.DragDX

	// Clamp to reasonable bounds (match calculatePaneWidths logic)
	available := p.width - 6 - dividerWidth
	minWidth := 20
	maxWidth := available - 40 // Leave at least 40 for preview
	if maxWidth < minWidth {
		maxWidth = minWidth
	}
	if newWidth < minWidth {
		newWidth = minWidth
	} else if newWidth > maxWidth {
		newWidth = maxWidth
	}

	p.treeWidth = newWidth
	p.previewWidth = available - p.treeWidth

	return p, nil
}

// handlePreviewSelectionDrag handles drag-to-select in the preview pane.
func (p *Plugin) handlePreviewSelectionDrag(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	p.textSelectionActive = true

	// Calculate Y offset to preview content
	inputBarHeight := 0
	if p.searchMode || p.contentSearchMode || p.fileOpMode != FileOpNone {
		inputBarHeight = 1
		if p.fileOpMode != FileOpNone && p.fileOpError != "" {
			inputBarHeight = 2
		}
	}
	previewContentStartY := inputBarHeight + 3 // border + header

	// Map Y coordinate to line index
	currentLine := (action.Y - previewContentStartY) + p.previewScroll

	// Clamp to valid range
	if currentLine < 0 {
		currentLine = 0
	}
	maxLine := len(p.previewLines) - 1
	if maxLine < 0 {
		return p, nil
	}
	if currentLine > maxLine {
		currentLine = maxLine
	}

	// Update selection based on anchor
	if currentLine < p.textSelectionAnchor {
		p.textSelectionStart = currentLine
		p.textSelectionEnd = p.textSelectionAnchor
	} else {
		p.textSelectionStart = p.textSelectionAnchor
		p.textSelectionEnd = currentLine
	}

	return p, nil
}

// handleMouseDragEnd handles the end of a drag operation.
func (p *Plugin) handleMouseDragEnd() (*Plugin, tea.Cmd) {
	switch p.mouseHandler.DragRegion() {
	case regionPaneDivider:
		// Save the current tree width to state
		_ = state.SetFileBrowserTreeWidth(p.treeWidth)
	case regionPreviewLine:
		// Selection complete - keep selection visible but mark as inactive
		p.textSelectionActive = false
	}
	return p, nil
}

// handleQuickOpenMouse handles mouse events in quick open modal.
func (p *Plugin) handleQuickOpenMouse(msg tea.MouseMsg) (*Plugin, tea.Cmd) {
	action := p.mouseHandler.HandleMouse(msg)

	switch action.Type {
	case mouse.ActionClick:
		if action.Region != nil && action.Region.ID == regionQuickOpen {
			if idx, ok := action.Region.Data.(int); ok {
				p.quickOpenCursor = idx
			}
		}
		return p, nil

	case mouse.ActionDoubleClick:
		if action.Region != nil && action.Region.ID == regionQuickOpen {
			if idx, ok := action.Region.Data.(int); ok {
				p.quickOpenCursor = idx
				plug, cmd := p.selectQuickOpenMatch()
				return plug.(*Plugin), cmd
			}
		}
		return p, nil

	case mouse.ActionScrollUp, mouse.ActionScrollDown:
		// Scroll quick open list
		delta := 3
		if action.Type == mouse.ActionScrollUp {
			delta = -3
		}
		p.quickOpenCursor += delta
		if p.quickOpenCursor < 0 {
			p.quickOpenCursor = 0
		} else if p.quickOpenCursor >= len(p.quickOpenMatches) {
			p.quickOpenCursor = len(p.quickOpenMatches) - 1
		}
		return p, nil
	}

	return p, nil
}

// handleProjectSearchMouse handles mouse events in project search modal.
func (p *Plugin) handleProjectSearchMouse(msg tea.MouseMsg) (*Plugin, tea.Cmd) {
	state := p.projectSearchState
	if state == nil {
		return p, nil
	}

	action := p.mouseHandler.HandleMouse(msg)

	switch action.Type {
	case mouse.ActionClick:
		return p.handleProjectSearchClick(action)

	case mouse.ActionDoubleClick:
		return p.handleProjectSearchDoubleClick(action)

	case mouse.ActionScrollUp, mouse.ActionScrollDown:
		// Scroll results list
		delta := 3
		if action.Type == mouse.ActionScrollUp {
			delta = -3
		}
		maxIdx := state.FlatLen() - 1
		state.Cursor += delta
		if state.Cursor < 0 {
			state.Cursor = 0
		} else if state.Cursor > maxIdx {
			state.Cursor = maxIdx
		}
		return p, nil
	}

	return p, nil
}

// handleProjectSearchClick handles single clicks in project search.
func (p *Plugin) handleProjectSearchClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	state := p.projectSearchState
	if action.Region == nil || state == nil {
		return p, nil
	}

	switch action.Region.ID {
	case regionSearchToggleRegex:
		state.UseRegex = !state.UseRegex
		if state.Query != "" {
			state.IsSearching = true
			return p, RunProjectSearch(p.ctx.WorkDir, state)
		}
		return p, nil

	case regionSearchToggleCase:
		state.CaseSensitive = !state.CaseSensitive
		if state.Query != "" {
			state.IsSearching = true
			return p, RunProjectSearch(p.ctx.WorkDir, state)
		}
		return p, nil

	case regionSearchToggleWord:
		state.WholeWord = !state.WholeWord
		if state.Query != "" {
			state.IsSearching = true
			return p, RunProjectSearch(p.ctx.WorkDir, state)
		}
		return p, nil

	case regionSearchFile:
		// Click on file header - move cursor to it
		if fileIdx, ok := action.Region.Data.(int); ok {
			// Find flat index for this file
			flatIdx := p.findFlatIndexForFile(fileIdx)
			if flatIdx >= 0 {
				state.Cursor = flatIdx
			}
		}
		return p, nil

	case regionSearchMatch:
		// Click on match line - move cursor to it
		if data, ok := action.Region.Data.(searchMatchData); ok {
			flatIdx := p.findFlatIndexForMatch(data.FileIdx, data.MatchIdx)
			if flatIdx >= 0 {
				state.Cursor = flatIdx
			}
		}
		return p, nil
	}

	return p, nil
}

// handleProjectSearchDoubleClick handles double clicks in project search.
func (p *Plugin) handleProjectSearchDoubleClick(action mouse.MouseAction) (*Plugin, tea.Cmd) {
	state := p.projectSearchState
	if action.Region == nil || state == nil {
		return p, nil
	}

	switch action.Region.ID {
	case regionSearchFile:
		// Double-click on file header - toggle expand/collapse
		if fileIdx, ok := action.Region.Data.(int); ok {
			// Move cursor to this file first
			flatIdx := p.findFlatIndexForFile(fileIdx)
			if flatIdx >= 0 {
				state.Cursor = flatIdx
			}
			// Toggle collapse
			if fileIdx >= 0 && fileIdx < len(state.Results) {
				state.Results[fileIdx].Collapsed = !state.Results[fileIdx].Collapsed
			}
		}
		return p, nil

	case regionSearchMatch:
		// Double-click on match - open in editor
		if data, ok := action.Region.Data.(searchMatchData); ok {
			if data.FileIdx >= 0 && data.FileIdx < len(state.Results) {
				file := state.Results[data.FileIdx]
				if data.MatchIdx >= 0 && data.MatchIdx < len(file.Matches) {
					match := file.Matches[data.MatchIdx]
					// Close project search and open file
					p.projectSearchMode = false
					p.projectSearchState = nil
					return p, p.openFileAtLine(file.Path, match.LineNo)
				}
			}
		}
		return p, nil
	}

	return p, nil
}

// findFlatIndexForFile finds the flat index for a file header.
func (p *Plugin) findFlatIndexForFile(fileIdx int) int {
	state := p.projectSearchState
	if state == nil || fileIdx < 0 || fileIdx >= len(state.Results) {
		return -1
	}

	flatIdx := 0
	for fi := range state.Results {
		if fi == fileIdx {
			return flatIdx
		}
		flatIdx++ // file header
		if !state.Results[fi].Collapsed {
			flatIdx += len(state.Results[fi].Matches)
		}
	}
	return -1
}

// findFlatIndexForMatch finds the flat index for a specific match.
func (p *Plugin) findFlatIndexForMatch(fileIdx, matchIdx int) int {
	state := p.projectSearchState
	if state == nil || fileIdx < 0 || fileIdx >= len(state.Results) {
		return -1
	}

	flatIdx := 0
	for fi, file := range state.Results {
		flatIdx++ // file header
		if fi == fileIdx {
			if file.Collapsed || matchIdx < 0 || matchIdx >= len(file.Matches) {
				return -1
			}
			return flatIdx + matchIdx
		}
		if !file.Collapsed {
			flatIdx += len(file.Matches)
		}
	}
	return -1
}
