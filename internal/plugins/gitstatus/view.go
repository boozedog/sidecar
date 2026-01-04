package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/styles"
)

// renderMain renders the main git status view.
func (p *Plugin) renderMain() string {
	if p.tree == nil {
		return styles.Muted.Render("Loading git status...")
	}

	var sb strings.Builder

	// Header
	header := fmt.Sprintf(" Git Status                          [%s]", p.tree.Summary())
	sb.WriteString(styles.PanelHeader.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")

	// Calculate visible area
	contentHeight := p.height - 2 // header
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Render sections
	entries := p.tree.AllEntries()
	if len(entries) == 0 {
		sb.WriteString(styles.Muted.Render(" Working tree clean"))
	} else {
		lineNum := 0
		globalIdx := 0

		// Staged section
		if len(p.tree.Staged) > 0 {
			sb.WriteString(p.renderSection("Staged", p.tree.Staged, &lineNum, &globalIdx, contentHeight))
		}

		// Modified section
		if len(p.tree.Modified) > 0 {
			if len(p.tree.Staged) > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.renderSection("Modified", p.tree.Modified, &lineNum, &globalIdx, contentHeight))
		}

		// Untracked section
		if len(p.tree.Untracked) > 0 {
			if len(p.tree.Staged) > 0 || len(p.tree.Modified) > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(p.renderSection("Untracked", p.tree.Untracked, &lineNum, &globalIdx, contentHeight))
		}
	}

	return sb.String()
}

// renderSection renders a single section (Staged/Modified/Untracked).
func (p *Plugin) renderSection(title string, entries []*FileEntry, lineNum, globalIdx *int, maxLines int) string {
	var sb strings.Builder

	// Section header
	headerStyle := styles.Subtitle
	if title == "Staged" {
		headerStyle = styles.StatusStaged
	} else if title == "Modified" {
		headerStyle = styles.StatusModified
	}

	sb.WriteString(headerStyle.Render(fmt.Sprintf(" %s (%d)", title, len(entries))))
	sb.WriteString("\n")
	*lineNum++

	// Entries
	for _, entry := range entries {
		if *lineNum >= maxLines {
			break
		}

		line := p.renderEntry(entry, *globalIdx == p.cursor)
		sb.WriteString(line)
		sb.WriteString("\n")
		*lineNum++
		*globalIdx++
	}

	return sb.String()
}

// renderEntry renders a single file entry.
func (p *Plugin) renderEntry(entry *FileEntry, selected bool) string {
	// Status indicator
	var statusStyle lipgloss.Style
	switch entry.Status {
	case StatusModified:
		statusStyle = styles.StatusModified
	case StatusAdded:
		statusStyle = styles.StatusStaged
	case StatusDeleted:
		statusStyle = styles.StatusDeleted
	case StatusRenamed:
		statusStyle = styles.StatusStaged
	case StatusUntracked:
		statusStyle = styles.StatusUntracked
	default:
		statusStyle = styles.Muted
	}

	status := statusStyle.Render(string(entry.Status))

	// Path
	path := entry.Path
	if entry.OldPath != "" {
		path = fmt.Sprintf("%s → %s", entry.OldPath, entry.Path)
	}

	// Diff stats
	stats := ""
	if entry.DiffStats.Additions > 0 || entry.DiffStats.Deletions > 0 {
		addStr := ""
		delStr := ""
		if entry.DiffStats.Additions > 0 {
			addStr = styles.DiffAdd.Render(fmt.Sprintf("+%d", entry.DiffStats.Additions))
		}
		if entry.DiffStats.Deletions > 0 {
			delStr = styles.DiffRemove.Render(fmt.Sprintf("-%d", entry.DiffStats.Deletions))
		}
		stats = fmt.Sprintf(" %s %s", addStr, delStr)
	}

	// Calculate available width for path
	maxPathWidth := p.width - 20 // Reserve space for cursor, status, stats
	if len(path) > maxPathWidth && maxPathWidth > 3 {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	if selected {
		// Build plain text and pad to full width
		plainLine := fmt.Sprintf("%s %s%s", string(entry.Status), path, fmt.Sprintf(" +%d -%d", entry.DiffStats.Additions, entry.DiffStats.Deletions))
		if entry.DiffStats.Additions == 0 && entry.DiffStats.Deletions == 0 {
			plainLine = fmt.Sprintf("%s %s", string(entry.Status), path)
		}
		maxWidth := p.width - 4
		if len(plainLine) < maxWidth {
			plainLine += strings.Repeat(" ", maxWidth-len(plainLine))
		}
		return styles.ListItemSelected.Render(plainLine)
	}

	return styles.ListItemNormal.Render(fmt.Sprintf("%s %s%s", status, path, stats))
}

// renderDiffModal renders the diff modal with panel border.
func (p *Plugin) renderDiffModal() string {
	// Calculate dimensions accounting for panel border (2) + padding (2)
	paneHeight := p.height - 2
	contentWidth := p.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Register hit region for mouse scrolling
	p.mouseHandler.Clear()
	p.mouseHandler.HitMap.AddRect(regionDiffModal, 0, 0, p.width, p.height, nil)

	var sb strings.Builder

	// Header with view mode indicator
	viewModeStr := "unified"
	if p.diffViewMode == DiffViewSideBySide {
		viewModeStr = "side-by-side"
	}
	header := fmt.Sprintf("Diff: %s [%s]", p.diffFile, viewModeStr)
	sb.WriteString(styles.ModalTitle.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", contentWidth)))
	sb.WriteString("\n")

	// Show delta tip if not installed (one-time)
	if p.externalTool != nil && p.externalTool.ShouldShowTip() {
		tip := styles.Code.Render(p.externalTool.GetTipMessage())
		sb.WriteString(tip)
		sb.WriteString("\n\n")
	}

	// Content
	if p.diffContent == "" {
		sb.WriteString(styles.Muted.Render("Loading diff..."))
	} else {
		// Visible lines = pane height - header (2 lines)
		visibleLines := paneHeight - 2
		if visibleLines < 1 {
			visibleLines = 1
		}

		// Determine content to display based on view mode and available tools
		var displayContent string
		useDelta := p.externalTool != nil && p.externalTool.ShouldUseDelta()

		if p.diffViewMode == DiffViewSideBySide {
			// Prefer built-in side-by-side renderer for consistency
			parsed := p.parsedDiff
			if parsed == nil {
				parsed, _ = ParseUnifiedDiff(p.diffRaw)
			}
			if parsed != nil {
				sb.WriteString(RenderSideBySide(parsed, contentWidth, p.diffScroll, visibleLines, p.diffHorizOff))
				return p.wrapDiffContent(sb.String(), paneHeight)
			} else if useDelta {
				// Fall back to delta if parsing failed
				rendered, _ := p.externalTool.RenderWithDelta(p.diffRaw, true, contentWidth)
				displayContent = rendered
			} else {
				sb.WriteString(styles.Muted.Render("Unable to parse diff for side-by-side view"))
				return p.wrapDiffContent(sb.String(), paneHeight)
			}
		} else {
			// Unified view - prefer built-in renderer for consistency with inline diff pane
			if p.parsedDiff != nil {
				sb.WriteString(RenderLineDiff(p.parsedDiff, contentWidth, p.diffScroll, visibleLines, p.diffHorizOff))
				return p.wrapDiffContent(sb.String(), paneHeight)
			} else if useDelta && p.diffContent != p.diffRaw {
				// Fall back to delta if parsing failed
				displayContent = p.diffContent
			} else {
				displayContent = p.diffRaw
			}
		}

		// Render line-by-line content (delta output or raw)
		lines := strings.Split(displayContent, "\n")
		start := p.diffScroll
		if start >= len(lines) {
			start = 0
		}
		end := start + visibleLines
		if end > len(lines) {
			end = len(lines)
		}

		for _, line := range lines[start:end] {
			if useDelta {
				sb.WriteString(line)
			} else {
				sb.WriteString(p.renderDiffLine(line))
			}
			sb.WriteString("\n")
		}
	}

	return p.wrapDiffContent(sb.String(), paneHeight)
}

// wrapDiffContent wraps diff content with panel border.
func (p *Plugin) wrapDiffContent(content string, paneHeight int) string {
	return styles.PanelActive.
		Width(p.width - 2).
		Height(paneHeight).
		Render(content)
}

// renderDiffTwoPane renders the diff view with sidebar visible.
// This matches the three-pane layout from status view but for full-screen diff.
func (p *Plugin) renderDiffTwoPane() string {
	p.calculatePaneWidths()

	// Calculate pane height: total - pane border (2 lines)
	paneHeight := p.height - 2
	if paneHeight < 4 {
		paneHeight = 4
	}

	// Inner content height = pane height - header lines (2)
	innerHeight := paneHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Clear and rebuild hit regions for mouse support
	p.mouseHandler.Clear()

	// Register hit regions
	p.mouseHandler.HitMap.AddRect(regionSidebar, 0, 0, p.sidebarWidth, p.height, nil)
	diffX := p.sidebarWidth + dividerWidth
	p.mouseHandler.HitMap.AddRect(regionDiffPane, diffX, 0, p.diffPaneWidth, p.height, nil)

	// Sidebar is inactive (showing files), diff pane is active
	sidebarBorder := styles.PanelInactive
	diffBorder := styles.PanelActive

	sidebarContent := p.renderSidebar(innerHeight)
	diffContent := p.renderFullDiffContent(innerHeight)

	leftPane := sidebarBorder.
		Width(p.sidebarWidth).
		Height(paneHeight).
		Render(sidebarContent)

	divider := p.renderDivider(paneHeight)

	rightPane := diffBorder.
		Width(p.diffPaneWidth).
		Height(paneHeight).
		Render(diffContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)
}

// renderFullDiffContent renders the diff content for the two-pane diff view.
func (p *Plugin) renderFullDiffContent(visibleHeight int) string {
	var sb strings.Builder

	// Header with view mode indicator
	viewModeStr := "unified"
	if p.diffViewMode == DiffViewSideBySide {
		viewModeStr = "side-by-side"
	}
	header := "Diff"
	if p.diffFile != "" {
		header = truncateDiffPath(p.diffFile, p.diffPaneWidth-14)
	}
	header = fmt.Sprintf("%s [%s]", header, viewModeStr)
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("\n\n")

	if p.diffContent == "" && p.diffRaw == "" {
		sb.WriteString(styles.Muted.Render("Loading diff..."))
		return sb.String()
	}

	// Content height accounting for header
	contentHeight := visibleHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Width: pane content width - padding
	diffWidth := p.diffPaneWidth - 4
	if diffWidth < 40 {
		diffWidth = 40
	}

	// Render diff based on view mode
	var diffContent string
	if p.diffViewMode == DiffViewSideBySide {
		parsed := p.parsedDiff
		if parsed == nil {
			parsed, _ = ParseUnifiedDiff(p.diffRaw)
		}
		if parsed != nil {
			diffContent = RenderSideBySide(parsed, diffWidth, p.diffScroll, contentHeight, p.diffHorizOff)
		}
	} else {
		if p.parsedDiff != nil {
			diffContent = RenderLineDiff(p.parsedDiff, diffWidth, p.diffScroll, contentHeight, p.diffHorizOff)
		}
	}

	// Truncate lines to prevent wrapping
	lines := strings.Split(diffContent, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > diffWidth {
			lines[i] = truncateStyledLine(line, diffWidth-3) + "..."
		}
	}
	sb.WriteString(strings.Join(lines, "\n"))

	return sb.String()
}

// renderDiffLine renders a single diff line with appropriate styling.
func (p *Plugin) renderDiffLine(line string) string {
	if len(line) == 0 {
		return ""
	}

	// Truncate long lines (account for panel border+padding: 4 chars, plus margin: 4 chars)
	maxWidth := p.width - 8
	if len(line) > maxWidth && maxWidth > 3 {
		line = line[:maxWidth-3] + "..."
	}

	switch {
	case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
		return styles.DiffAdd.Render(line)
	case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		return styles.DiffRemove.Render(line)
	case strings.HasPrefix(line, "@@"):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		return styles.DiffHeader.Render(line)
	default:
		return styles.DiffContext.Render(line)
	}
}
