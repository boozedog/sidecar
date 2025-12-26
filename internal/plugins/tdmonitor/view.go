package tdmonitor

import (
	"fmt"
	"strings"

	"github.com/sst/sidecar/internal/styles"
)

// renderNoDatabase renders the view when no database is available.
func renderNoDatabase() string {
	return styles.Muted.Render(" td database not found (.todos/issues.db)")
}

// renderList renders the main list view.
func (p *Plugin) renderList() string {
	var sb strings.Builder

	// Header
	sessionInfo := ""
	if p.session != nil {
		sessionInfo = fmt.Sprintf("session: %s", p.session.ID[:8])
	}
	header := fmt.Sprintf(" td monitor                               %s", sessionInfo)
	sb.WriteString(styles.PanelHeader.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")

	// In Progress section
	if len(p.inProgress) > 0 {
		sb.WriteString(p.renderSection("In Progress", p.inProgress, p.activeList == "in_progress"))
		sb.WriteString("\n")
	}

	// Ready section
	sb.WriteString(p.renderSection("Ready", p.ready, p.activeList == "ready"))
	sb.WriteString("\n")

	// Reviewable section
	if len(p.reviewable) > 0 {
		sb.WriteString(p.renderSection("Reviewable", p.reviewable, p.activeList == "reviewable"))
	}

	// Footer
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")
	sb.WriteString(p.renderFooter())

	return sb.String()
}

// renderSection renders a section of issues.
func (p *Plugin) renderSection(title string, issues []Issue, active bool) string {
	var sb strings.Builder

	// Section header with count
	headerStyle := styles.Subtitle
	if active {
		headerStyle = styles.StatusInProgress
	}
	sb.WriteString(headerStyle.Render(fmt.Sprintf(" %s (%d)", title, len(issues))))
	sb.WriteString("\n")

	if len(issues) == 0 {
		sb.WriteString(styles.Muted.Render("   (none)\n"))
		return sb.String()
	}

	// Render issues
	activeList := p.activeListData()
	for i, issue := range issues {
		// Check if this is the selected item
		selected := active && i == p.cursor
		sb.WriteString(p.renderIssueRow(issue, selected))
		sb.WriteString("\n")

		// Check if we're at max visible
		if active && i >= p.scrollOff+p.height-10 && i < len(activeList)-1 {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("   ... and %d more\n", len(activeList)-i-1)))
			break
		}
	}

	return sb.String()
}

// renderIssueRow renders a single issue row.
func (p *Plugin) renderIssueRow(issue Issue, selected bool) string {
	// Cursor
	cursor := "  "
	if selected {
		cursor = styles.ListCursor.Render("> ")
	}

	// Priority badge
	prioStyle := styles.Muted
	switch issue.Priority {
	case "P0":
		prioStyle = styles.StatusBlocked
	case "P1":
		prioStyle = styles.StatusModified
	case "P2":
		prioStyle = styles.StatusInProgress
	}
	priority := prioStyle.Render(fmt.Sprintf("[%s]", issue.Priority))

	// Type badge
	typeStr := styles.Muted.Render(issue.Type)

	// ID (short)
	id := issue.ID
	if len(id) > 8 {
		id = id[:8]
	}
	idStr := styles.Code.Render(id)

	// Title (truncated)
	title := issue.Title
	maxTitleWidth := p.width - 35
	if len(title) > maxTitleWidth && maxTitleWidth > 3 {
		title = title[:maxTitleWidth-3] + "..."
	}

	// Compose line
	lineStyle := styles.ListItemNormal
	if selected {
		lineStyle = styles.ListItemSelected
	}

	return lineStyle.Render(fmt.Sprintf("%s%s %s  %s  %s", cursor, idStr, priority, typeStr, title))
}

// renderFooter renders the key hints footer.
func (p *Plugin) renderFooter() string {
	hints := []string{
		styles.KeyHint.Render("enter") + " details",
		styles.KeyHint.Render("a") + " approve",
		styles.KeyHint.Render("x") + " delete",
		styles.KeyHint.Render("tab") + " switch",
		styles.KeyHint.Render("?") + " help",
	}
	return styles.Muted.Render(" " + strings.Join(hints, "  "))
}

// renderDetail renders the issue detail view.
func (p *Plugin) renderDetail() string {
	if p.detailIssue == nil {
		return ""
	}

	issue := p.detailIssue
	var sb strings.Builder

	// Header
	header := fmt.Sprintf(" %s: %s", issue.ID[:8], issue.Title)
	sb.WriteString(styles.PanelHeader.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString(fmt.Sprintf(" Status:   %s\n", statusBadge(issue.Status)))
	sb.WriteString(fmt.Sprintf(" Priority: %s\n", issue.Priority))
	sb.WriteString(fmt.Sprintf(" Type:     %s\n", issue.Type))
	if issue.Labels != "" {
		sb.WriteString(fmt.Sprintf(" Labels:   %s\n", issue.Labels))
	}
	sb.WriteString("\n")

	// Description
	if issue.Description != "" {
		sb.WriteString(styles.Title.Render(" Description"))
		sb.WriteString("\n")
		// Word wrap description
		lines := wrapText(issue.Description, p.width-4)
		for _, line := range lines {
			sb.WriteString(" " + line + "\n")
		}
	}

	// Footer
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")
	hints := []string{
		styles.KeyHint.Render("esc") + " back",
		styles.KeyHint.Render("a") + " approve",
		styles.KeyHint.Render("x") + " delete",
	}
	sb.WriteString(styles.Muted.Render(" " + strings.Join(hints, "  ")))

	return sb.String()
}

// statusBadge returns a styled status string.
func statusBadge(status string) string {
	switch status {
	case "open":
		return styles.StatusPending.Render("open")
	case "in_progress":
		return styles.StatusInProgress.Render("in_progress")
	case "in_review":
		return styles.StatusModified.Render("in_review")
	case "done":
		return styles.StatusCompleted.Render("done")
	default:
		return styles.Muted.Render(status)
	}
}

// wrapText wraps text to fit within maxWidth.
func wrapText(text string, maxWidth int) []string {
	if text == "" {
		return nil
	}

	if maxWidth <= 0 {
		return []string{text}
	}

	var lines []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= maxWidth {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}
