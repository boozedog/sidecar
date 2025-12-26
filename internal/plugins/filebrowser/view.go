package filebrowser

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sst/sidecar/internal/styles"
)

// FocusPane represents which pane is active.
type FocusPane int

const (
	PaneTree FocusPane = iota
	PanePreview
)

// calculatePaneWidths sets the tree and preview pane widths.
func (p *Plugin) calculatePaneWidths() {
	// Account for borders (2 chars each pane) and separator
	available := p.width - 6
	p.treeWidth = available * 30 / 100
	if p.treeWidth < 20 {
		p.treeWidth = 20
	}
	p.previewWidth = available - p.treeWidth
	if p.previewWidth < 40 {
		p.previewWidth = 40
	}
}

// renderView creates the 2-pane layout.
func (p *Plugin) renderView() string {
	p.calculatePaneWidths()

	// Determine border styles based on focus
	treeBorder := styles.PanelInactive
	previewBorder := styles.PanelInactive
	if p.activePane == PaneTree && !p.searchMode {
		treeBorder = styles.PanelActive
	} else if p.activePane == PanePreview && !p.searchMode {
		previewBorder = styles.PanelActive
	}

	// Account for search bar if active
	searchBarHeight := 0
	if p.searchMode {
		searchBarHeight = 1
	}

	// Calculate pane height: total - footer (1 line) - search bar - pane border (2 lines)
	paneHeight := p.height - 1 - searchBarHeight - 2
	if paneHeight < 4 {
		paneHeight = 4
	}

	// Inner content height = pane height - header lines (2)
	innerHeight := paneHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	treeContent := p.renderTreePane(innerHeight)
	previewContent := p.renderPreviewPane(innerHeight)

	// Apply styles
	leftPane := treeBorder.
		Width(p.treeWidth).
		Height(paneHeight).
		Render(treeContent)

	rightPane := previewBorder.
		Width(p.previewWidth).
		Height(paneHeight).
		Render(previewContent)

	// Join panes horizontally
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Build final layout
	var parts []string

	// Add search bar if in search mode
	if p.searchMode {
		parts = append(parts, p.renderSearchBar())
	}

	parts = append(parts, panes)

	// Add footer
	footer := p.renderFooter()
	parts = append(parts, footer)

	return lipgloss.JoinVertical(lipgloss.Top, parts...)
}

// renderSearchBar renders the search input bar.
func (p *Plugin) renderSearchBar() string {
	cursor := "█"
	matchInfo := ""
	if len(p.searchMatches) > 0 {
		matchInfo = fmt.Sprintf(" (%d/%d)", p.searchCursor+1, len(p.searchMatches))
	} else if p.searchQuery != "" {
		matchInfo = " (no matches)"
	}

	searchLine := fmt.Sprintf(" / %s%s%s", p.searchQuery, cursor, matchInfo)
	return styles.ModalTitle.Render(searchLine)
}

// renderTreePane renders the file tree in the left pane.
func (p *Plugin) renderTreePane(visibleHeight int) string {
	var sb strings.Builder

	// Header
	header := styles.Title.Render("Files")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	if p.tree == nil || p.tree.Len() == 0 {
		sb.WriteString(styles.Muted.Render("No files"))
		return sb.String()
	}

	end := p.treeScrollOff + visibleHeight
	if end > p.tree.Len() {
		end = p.tree.Len()
	}

	for i := p.treeScrollOff; i < end; i++ {
		node := p.tree.GetNode(i)
		if node == nil {
			continue
		}

		selected := i == p.treeCursor
		maxWidth := p.treeWidth - 4 // Account for border padding
		line := p.renderTreeNode(node, selected, maxWidth)

		sb.WriteString(line)
		// Don't add newline after last line
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderTreeNode renders a single tree node.
func (p *Plugin) renderTreeNode(node *FileNode, selected bool, maxWidth int) string {
	// Indentation
	indent := strings.Repeat("  ", node.Depth)

	// Icon for directories
	icon := "  "
	if node.IsDir {
		if node.IsExpanded {
			icon = "> "
		} else {
			icon = "+ "
		}
	}

	// Calculate available width for name (after indent and icon)
	prefixLen := len(indent) + len(icon)
	availableWidth := maxWidth - prefixLen
	if availableWidth < 3 {
		availableWidth = 3
	}

	// Truncate name before styling to avoid cutting ANSI escape codes
	displayName := node.Name
	if len(displayName) > availableWidth {
		displayName = displayName[:availableWidth-1] + "…"
	}

	// Name styling
	var name string
	if node.IsDir {
		name = styles.FileBrowserDir.Render(displayName)
	} else if node.IsIgnored {
		name = styles.FileBrowserIgnored.Render(displayName)
	} else {
		name = styles.FileBrowserFile.Render(displayName)
	}

	line := fmt.Sprintf("%s%s%s", indent, styles.FileBrowserIcon.Render(icon), name)

	if selected {
		return styles.ListItemSelected.Render(line)
	}
	return line
}

// renderPreviewPane renders the file preview in the right pane.
func (p *Plugin) renderPreviewPane(visibleHeight int) string {
	var sb strings.Builder

	// Header
	header := "Preview"
	if p.previewFile != "" {
		header = truncatePath(p.previewFile, p.previewWidth-4)
	}
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("\n\n")

	if p.previewFile == "" {
		sb.WriteString(styles.Muted.Render("Select a file to preview"))
		return sb.String()
	}

	if p.previewError != nil {
		sb.WriteString(styles.StatusDeleted.Render(p.previewError.Error()))
		return sb.String()
	}

	if p.isBinary {
		sb.WriteString(styles.Muted.Render("Binary file"))
		return sb.String()
	}

	// Use highlighted lines if available
	lines := p.previewHighlighted
	if len(lines) == 0 {
		lines = p.previewLines
	}

	start := p.previewScroll
	end := start + visibleHeight
	if end > len(lines) {
		end = len(lines)
	}

	// Calculate max line width (pane width - line number - padding)
	lineNumWidth := 5 // "1234 " = 5 chars
	maxLineWidth := p.previewWidth - lineNumWidth - 4
	if maxLineWidth < 10 {
		maxLineWidth = 10
	}

	// Style for truncating lines with ANSI codes
	lineStyle := lipgloss.NewStyle().MaxWidth(maxLineWidth)

	// Reserve 1 line for truncation message if needed
	contentEnd := end
	if p.isTruncated && end-start > 1 {
		contentEnd = end - 1
	}

	for i := start; i < contentEnd; i++ {
		lineNum := styles.FileBrowserLineNumber.Render(fmt.Sprintf("%4d ", i+1))
		line := lineStyle.Render(lines[i]) // Truncates while preserving ANSI codes

		sb.WriteString(lineNum)
		sb.WriteString(line)
		// Don't add newline after last line
		if i < contentEnd-1 || p.isTruncated {
			sb.WriteString("\n")
		}
	}

	if p.isTruncated {
		sb.WriteString(styles.Muted.Render("... (file truncated)"))
	}

	return sb.String()
}

// renderFooter renders the keybinding hints.
func (p *Plugin) renderFooter() string {
	var hints string
	if p.searchMode {
		hints = "esc cancel  enter jump  up/down select match"
	} else if p.activePane == PaneTree {
		hints = "j/k nav  l open/preview  h close  e edit  / search  n/N match"
	} else {
		hints = "h back  e edit  j/k scroll  g top  G bottom  ctrl+d/u page"
	}
	return styles.Muted.Render(hints)
}

// truncatePath shortens a path to fit width.
func truncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	if maxWidth < 10 {
		return path[:maxWidth]
	}
	// Show ...end of path
	return "..." + path[len(path)-maxWidth+3:]
}
