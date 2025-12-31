package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sst/sidecar/internal/styles"
)

// calculatePaneWidths sets the sidebar and diff pane widths.
func (p *Plugin) calculatePaneWidths() {
	// Account for borders: each pane has 2 (left+right border)
	// Two panes = 4 border chars total, plus 1 gap between panes
	available := p.width - 5

	if !p.sidebarVisible {
		p.sidebarWidth = 0
		p.diffPaneWidth = available - 2 // Single pane border
		return
	}

	// 30% sidebar, 70% diff
	p.sidebarWidth = available * 30 / 100
	if p.sidebarWidth < 25 {
		p.sidebarWidth = 25
	}
	p.diffPaneWidth = available - p.sidebarWidth
	if p.diffPaneWidth < 40 {
		p.diffPaneWidth = 40
	}
}

// renderThreePaneView creates the three-pane layout for git status.
func (p *Plugin) renderThreePaneView() string {
	p.calculatePaneWidths()

	// Calculate pane height: total - pane border (2 lines)
	// Note: App footer is rendered by the app, not the plugin
	paneHeight := p.height - 2
	if paneHeight < 4 {
		paneHeight = 4
	}

	// Inner content height = pane height - header lines (2)
	innerHeight := paneHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	if p.sidebarVisible {
		// Determine border styles based on focus
		sidebarBorder := styles.PanelInactive
		diffBorder := styles.PanelInactive
		if p.activePane == PaneSidebar {
			sidebarBorder = styles.PanelActive
		} else {
			diffBorder = styles.PanelActive
		}

		sidebarContent := p.renderSidebar(innerHeight)
		diffContent := p.renderDiffPane(innerHeight)

		leftPane := sidebarBorder.
			Width(p.sidebarWidth).
			Height(paneHeight).
			Render(sidebarContent)

		rightPane := diffBorder.
			Width(p.diffPaneWidth).
			Height(paneHeight).
			Render(diffContent)

		return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	// Full-width diff pane when sidebar is hidden
	diffBorder := styles.PanelActive
	diffContent := p.renderDiffPane(innerHeight)

	return diffBorder.
		Width(p.diffPaneWidth).
		Height(paneHeight).
		Render(diffContent)
}

// renderSidebar renders the left sidebar with files and commits.
func (p *Plugin) renderSidebar(visibleHeight int) string {
	var sb strings.Builder

	// Header
	header := styles.Title.Render("Files")
	sb.WriteString(header)
	sb.WriteString("\n\n")

	entries := p.tree.AllEntries()
	if len(entries) == 0 {
		sb.WriteString(styles.Muted.Render("Working tree clean"))
	} else {
		// Calculate space for files vs commits
		// Reserve ~30% for commits section (min 4 lines for header + 2-3 commits)
		commitsReserve := 5
		if len(p.recentCommits) > 3 {
			commitsReserve = 6
		}
		filesHeight := visibleHeight - commitsReserve - 2 // -2 for section headers
		if filesHeight < 3 {
			filesHeight = 3
		}

		// Render file sections
		lineNum := 0
		globalIdx := 0

		// Staged section
		if len(p.tree.Staged) > 0 && lineNum < filesHeight {
			sb.WriteString(p.renderSidebarSection("Staged", p.tree.Staged, &lineNum, &globalIdx, filesHeight))
		}

		// Modified section
		if len(p.tree.Modified) > 0 && lineNum < filesHeight {
			if len(p.tree.Staged) > 0 {
				sb.WriteString("\n")
				lineNum++
			}
			sb.WriteString(p.renderSidebarSection("Modified", p.tree.Modified, &lineNum, &globalIdx, filesHeight))
		}

		// Untracked section
		if len(p.tree.Untracked) > 0 && lineNum < filesHeight {
			if len(p.tree.Staged) > 0 || len(p.tree.Modified) > 0 {
				sb.WriteString("\n")
				lineNum++
			}
			sb.WriteString(p.renderSidebarSection("Untracked", p.tree.Untracked, &lineNum, &globalIdx, filesHeight))
		}
	}

	// Separator
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", p.sidebarWidth-4)))
	sb.WriteString("\n")

	// Push status/error message
	if p.pushInProgress {
		sb.WriteString(styles.StatusInProgress.Render("Pushing..."))
		sb.WriteString("\n")
	} else if p.pushSuccess {
		sb.WriteString(styles.StatusStaged.Render("✓ Pushed"))
		sb.WriteString("\n")
	} else if p.pushError != "" {
		// Truncate error if too long (account for "✗ " prefix)
		errMsg := p.pushError
		maxLen := p.sidebarWidth - 8 // 2 for "✗ " prefix + 6 for padding
		if len(errMsg) > maxLen && maxLen > 3 {
			errMsg = errMsg[:maxLen-3] + "..."
		}
		sb.WriteString(styles.StatusDeleted.Render("✗ " + errMsg))
		sb.WriteString("\n")
	}

	// Recent commits section
	sb.WriteString(p.renderRecentCommits())

	return sb.String()
}

// renderSidebarSection renders a file section in the sidebar.
func (p *Plugin) renderSidebarSection(title string, entries []*FileEntry, lineNum, globalIdx *int, maxLines int) string {
	var sb strings.Builder

	// Section header with color based on type
	headerStyle := styles.Subtitle
	if title == "Staged" {
		headerStyle = styles.StatusStaged
	} else if title == "Modified" {
		headerStyle = styles.StatusModified
	}

	sb.WriteString(headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(entries))))
	sb.WriteString("\n")
	*lineNum++

	// Available width for file names
	maxWidth := p.sidebarWidth - 6 // Account for padding and cursor

	for _, entry := range entries {
		if *lineNum >= maxLines {
			break
		}

		selected := *globalIdx == p.cursor
		line := p.renderSidebarEntry(entry, selected, maxWidth)
		sb.WriteString(line)
		sb.WriteString("\n")
		*lineNum++
		*globalIdx++
	}

	return sb.String()
}

// renderSidebarEntry renders a single file entry in the sidebar.
func (p *Plugin) renderSidebarEntry(entry *FileEntry, selected bool, maxWidth int) string {
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = styles.ListCursor.Render("> ")
	}

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

	// Handle folder entries specially
	if entry.IsFolder {
		// Show expand/collapse indicator and file count
		indicator := "▶"
		if entry.IsExpanded {
			indicator = "▼"
		}
		folderName := entry.Path
		fileCount := len(entry.Children)
		countStr := fmt.Sprintf("(%d)", fileCount)

		// Calculate available width
		availableWidth := maxWidth - 6 // cursor + status + indicator + spacing
		displayName := folderName
		if len(folderName)+len(countStr)+1 > availableWidth && availableWidth > 10 {
			displayName = folderName[:availableWidth-len(countStr)-4] + "…/"
		}

		lineStyle := styles.ListItemNormal
		if selected {
			lineStyle = styles.ListItemSelected
		}

		return lineStyle.Render(fmt.Sprintf("%s%s %s %s %s", cursor, status, indicator, displayName, styles.Muted.Render(countStr)))
	}

	// Path - truncate if needed
	path := entry.Path
	availableWidth := maxWidth - 4 // cursor + status + space
	if len(path) > availableWidth && availableWidth > 3 {
		path = "…" + path[len(path)-availableWidth+1:]
	}

	// Compose line
	lineStyle := styles.ListItemNormal
	if selected {
		lineStyle = styles.ListItemSelected
	}

	return lineStyle.Render(fmt.Sprintf("%s%s %s", cursor, status, path))
}

// renderRecentCommits renders the recent commits section in the sidebar.
func (p *Plugin) renderRecentCommits() string {
	var sb strings.Builder

	// Section header with push status
	header := "Recent Commits"
	if p.pushStatus != nil {
		status := p.pushStatus.FormatAheadBehind()
		if status != "" {
			header = fmt.Sprintf("Recent Commits %s", styles.StatusModified.Render(status))
		}
	}
	sb.WriteString(styles.Subtitle.Render(header))
	sb.WriteString("\n")

	if len(p.recentCommits) == 0 {
		sb.WriteString(styles.Muted.Render("No commits"))
		return sb.String()
	}

	// Cursor selection: cursor indexes files first, then commits
	fileCount := len(p.tree.AllEntries())
	maxWidth := p.sidebarWidth - 4
	maxCommits := 5
	if len(p.recentCommits) < maxCommits {
		maxCommits = len(p.recentCommits)
	}

	for i := 0; i < maxCommits; i++ {
		commit := p.recentCommits[i]
		selected := p.cursor == fileCount+i

		// Cursor indicator
		var cursor string
		if selected {
			cursor = styles.ListCursor.Render("> ")
		} else {
			cursor = "  "
		}

		// Push indicator: ↑ for unpushed, nothing for pushed
		var indicator string
		if !commit.Pushed {
			indicator = styles.StatusModified.Render("↑") + " "
		} else {
			indicator = "  " // Two spaces to align with indicator
		}

		// Format: "> ↑ abc1234 commit message..."
		hash := styles.Code.Render(commit.Hash[:7])
		msgWidth := maxWidth - 14 // cursor + indicator + hash + space
		msg := commit.Subject
		if len(msg) > msgWidth && msgWidth > 3 {
			msg = msg[:msgWidth-1] + "…"
		}

		// Compose line with selection styling
		lineStyle := styles.ListItemNormal
		if selected {
			lineStyle = styles.ListItemSelected
		}

		sb.WriteString(lineStyle.Render(fmt.Sprintf("%s%s%s %s", cursor, indicator, hash, msg)))
		if i < maxCommits-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderDiffPane renders the right diff pane.
func (p *Plugin) renderDiffPane(visibleHeight int) string {
	// If previewing a commit, render commit preview instead of diff
	if p.previewCommit != nil && p.cursorOnCommit() {
		return p.renderCommitPreview(visibleHeight)
	}

	var sb strings.Builder

	// Header
	header := "Diff"
	if p.selectedDiffFile != "" {
		header = truncateDiffPath(p.selectedDiffFile, p.diffPaneWidth-6)
	}
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("\n\n")

	if p.selectedDiffFile == "" {
		sb.WriteString(styles.Muted.Render("Select a file to view diff"))
		return sb.String()
	}

	if p.diffPaneParsedDiff == nil {
		sb.WriteString(styles.Muted.Render("Loading diff..."))
		return sb.String()
	}

	// Render the diff content
	contentHeight := visibleHeight - 2 // Account for header
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Width: pane content width - padding (2) - extra buffer (2)
	// The pane style applies Padding(0,1) which takes 2 chars from content area
	diffWidth := p.diffPaneWidth - 4
	if diffWidth < 40 {
		diffWidth = 40
	}

	// Render diff and apply MaxWidth to prevent any line wrapping
	diffContent := RenderLineDiff(p.diffPaneParsedDiff, diffWidth, p.diffPaneScroll, contentHeight, p.diffPaneHorizScroll)
	// Force truncate each line to prevent wrapping
	lines := strings.Split(diffContent, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > diffWidth {
			// Truncate the line to fit
			lines[i] = truncateStyledLine(line, diffWidth-3) + "..."
		}
	}
	sb.WriteString(strings.Join(lines, "\n"))

	return sb.String()
}

// renderCommitPreview renders commit detail in the right pane.
func (p *Plugin) renderCommitPreview(visibleHeight int) string {
	var sb strings.Builder

	c := p.previewCommit
	if c == nil {
		sb.WriteString(styles.Muted.Render("Loading commit..."))
		return sb.String()
	}

	maxWidth := p.diffPaneWidth - 4

	// Header with commit hash
	header := fmt.Sprintf("Commit %s", c.ShortHash)
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString(styles.Subtitle.Render("Author: "))
	authorStr := c.Author
	if len(authorStr) > maxWidth-10 {
		authorStr = authorStr[:maxWidth-13] + "..."
	}
	sb.WriteString(styles.Body.Render(authorStr))
	sb.WriteString("\n")

	sb.WriteString(styles.Subtitle.Render("Date:   "))
	sb.WriteString(styles.Muted.Render(RelativeTime(c.Date)))
	sb.WriteString("\n\n")

	// Subject
	subject := c.Subject
	if len(subject) > maxWidth-2 {
		subject = subject[:maxWidth-5] + "..."
	}
	sb.WriteString(styles.Body.Render(subject))
	sb.WriteString("\n")

	// Body (if present, truncated)
	if c.Body != "" {
		sb.WriteString("\n")
		bodyLines := strings.Split(strings.TrimSpace(c.Body), "\n")
		maxBodyLines := 3
		for i, line := range bodyLines {
			if i >= maxBodyLines {
				sb.WriteString(styles.Muted.Render("..."))
				sb.WriteString("\n")
				break
			}
			if len(line) > maxWidth-2 {
				line = line[:maxWidth-5] + "..."
			}
			sb.WriteString(styles.Muted.Render(line))
			sb.WriteString("\n")
		}
	}

	// Separator
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", maxWidth)))
	sb.WriteString("\n")

	// Files header with stats
	statsLine := fmt.Sprintf("Files (%d)", len(c.Files))
	if c.Stats.Additions > 0 || c.Stats.Deletions > 0 {
		addStr := styles.DiffAdd.Render(fmt.Sprintf("+%d", c.Stats.Additions))
		delStr := styles.DiffRemove.Render(fmt.Sprintf("-%d", c.Stats.Deletions))
		statsLine = fmt.Sprintf("Files (%d)  %s %s", len(c.Files), addStr, delStr)
	}
	sb.WriteString(styles.Subtitle.Render(statsLine))
	sb.WriteString("\n")

	// Calculate remaining height for file list
	linesUsed := 10 // header, metadata, subject, separator, files header
	if c.Body != "" {
		bodyLineCount := len(strings.Split(strings.TrimSpace(c.Body), "\n"))
		if bodyLineCount > 3 {
			bodyLineCount = 4 // includes "..."
		}
		linesUsed += bodyLineCount + 1
	}
	fileListHeight := visibleHeight - linesUsed
	if fileListHeight < 3 {
		fileListHeight = 3
	}

	// Files list with cursor
	if len(c.Files) == 0 {
		sb.WriteString(styles.Muted.Render("No files changed"))
	} else {
		start := p.previewCommitScroll
		if start >= len(c.Files) {
			start = 0
		}
		end := start + fileListHeight
		if end > len(c.Files) {
			end = len(c.Files)
		}

		for i := start; i < end; i++ {
			file := c.Files[i]
			selected := i == p.previewCommitCursor && p.activePane == PaneDiff

			line := p.renderCommitPreviewFile(file, selected, maxWidth-4)
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderCommitPreviewFile renders a single file in the commit preview.
func (p *Plugin) renderCommitPreviewFile(file CommitFile, selected bool, maxWidth int) string {
	// Cursor indicator
	cursor := "  "
	if selected {
		cursor = styles.ListCursor.Render("> ")
	}

	// Status indicator with color
	var statusStyle lipgloss.Style
	switch file.Status {
	case StatusModified:
		statusStyle = styles.StatusModified
	case StatusAdded:
		statusStyle = styles.StatusStaged
	case StatusDeleted:
		statusStyle = styles.StatusDeleted
	case StatusRenamed:
		statusStyle = styles.StatusStaged
	default:
		statusStyle = styles.Muted
	}
	status := statusStyle.Render(string(file.Status))

	// Path - truncate if needed
	path := file.Path
	pathWidth := maxWidth - 8 // cursor + status + spacing
	if len(path) > pathWidth && pathWidth > 3 {
		path = "…" + path[len(path)-pathWidth+1:]
	}

	// Compose line
	lineStyle := styles.ListItemNormal
	if selected {
		lineStyle = styles.ListItemSelected
	}

	return lineStyle.Render(fmt.Sprintf("%s%s %s", cursor, status, path))
}

// truncateStyledLine truncates a line that may contain ANSI codes to a visual width.
func truncateStyledLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	// Use lipgloss to measure and truncate
	style := lipgloss.NewStyle().MaxWidth(maxWidth)
	return style.Render(s)
}

// truncateDiffPath shortens a path to fit width.
func truncateDiffPath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}
	if maxWidth < 10 {
		return path[:maxWidth]
	}
	return "…" + path[len(path)-maxWidth+1:]
}
