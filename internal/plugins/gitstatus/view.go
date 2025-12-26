package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sst/sidecar/internal/styles"
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

	// Compose line
	lineStyle := styles.ListItemNormal
	if selected {
		lineStyle = styles.ListItemSelected
	}

	// Calculate available width for path
	maxPathWidth := p.width - 20 // Reserve space for cursor, status, stats
	if len(path) > maxPathWidth && maxPathWidth > 3 {
		path = "..." + path[len(path)-maxPathWidth+3:]
	}

	return lineStyle.Render(fmt.Sprintf("%s%s %s%s", cursor, status, path, stats))
}

// renderDiffModal renders the diff modal.
func (p *Plugin) renderDiffModal() string {
	var sb strings.Builder

	// Header
	header := fmt.Sprintf(" Diff: %s", p.diffFile)
	sb.WriteString(styles.ModalTitle.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")

	// Content
	if p.diffContent == "" {
		sb.WriteString(styles.Muted.Render(" Loading diff..."))
	} else {
		lines := strings.Split(p.diffContent, "\n")
		visibleLines := p.height - 2
		if visibleLines < 1 {
			visibleLines = 1
		}

		start := p.diffScroll
		if start >= len(lines) {
			start = 0
		}
		end := start + visibleLines
		if end > len(lines) {
			end = len(lines)
		}

		for _, line := range lines[start:end] {
			sb.WriteString(p.renderDiffLine(line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderDiffLine renders a single diff line with appropriate styling.
func (p *Plugin) renderDiffLine(line string) string {
	if len(line) == 0 {
		return ""
	}

	// Truncate long lines
	maxWidth := p.width - 4
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
