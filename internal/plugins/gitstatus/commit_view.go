package gitstatus

import (
	"fmt"
	"strings"

	"github.com/sst/sidecar/internal/styles"
)

// renderCommit renders the commit message view.
func (p *Plugin) renderCommit() string {
	var sb strings.Builder

	// Calculate stats
	additions, deletions := p.tree.StagedStats()
	fileCount := len(p.tree.Staged)

	// Header with stats
	statsStr := fmt.Sprintf("[%d files: +%d -%d]", fileCount, additions, deletions)
	header := fmt.Sprintf(" Commit                          %s", statsStr)
	sb.WriteString(styles.PanelHeader.Render(header))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("━", p.width-2)))
	sb.WriteString("\n")

	// Staged files section
	sb.WriteString(styles.StatusStaged.Render(fmt.Sprintf(" Staged Files (%d)", fileCount)))
	sb.WriteString("\n")

	// Show staged files (limit to a few to leave room for textarea)
	maxFiles := 5
	if p.height < 20 {
		maxFiles = 3
	}
	for i, entry := range p.tree.Staged {
		if i >= maxFiles {
			remaining := len(p.tree.Staged) - maxFiles
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("   ... and %d more files", remaining)))
			sb.WriteString("\n")
			break
		}

		// Status indicator
		status := styles.StatusStaged.Render(string(entry.Status))

		// Path with diff stats
		path := entry.Path
		maxPathWidth := p.width - 30
		if len(path) > maxPathWidth && maxPathWidth > 3 {
			path = "..." + path[len(path)-maxPathWidth+3:]
		}

		// Diff stats
		stats := ""
		if entry.DiffStats.Additions > 0 || entry.DiffStats.Deletions > 0 {
			addStr := styles.DiffAdd.Render(fmt.Sprintf("+%d", entry.DiffStats.Additions))
			delStr := styles.DiffRemove.Render(fmt.Sprintf("-%d", entry.DiffStats.Deletions))
			stats = fmt.Sprintf(" %s %s", addStr, delStr)
		}

		sb.WriteString(fmt.Sprintf("   %s %s%s\n", status, path, stats))
	}

	// Separator
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", p.width-2)))
	sb.WriteString("\n")

	// Commit message section
	sb.WriteString(styles.Subtitle.Render(" Commit Message"))
	sb.WriteString("\n")

	// Textarea
	sb.WriteString(p.commitMessage.View())
	sb.WriteString("\n\n")

	// Commit button
	buttonStyle := styles.Button
	if p.commitButtonFocus {
		buttonStyle = styles.ButtonFocused
	}
	sb.WriteString("  ")
	sb.WriteString(buttonStyle.Render(" Commit "))
	sb.WriteString("  ")
	sb.WriteString(styles.Muted.Render("Tab to select, Enter to confirm"))
	sb.WriteString("\n")

	// Error message if any
	if p.commitError != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.StatusDeleted.Render(" ✗ " + p.commitError))
		sb.WriteString("\n")
	}

	// Progress indicator
	if p.commitInProgress {
		sb.WriteString("\n")
		sb.WriteString(styles.Muted.Render(" Committing..."))
		sb.WriteString("\n")
	}

	// Separator
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", p.width-2)))
	sb.WriteString("\n")

	// Footer with keybindings
	escKey := styles.KeyHint.Render(" Esc ")
	commitKey := styles.KeyHint.Render(" ^S ")
	sb.WriteString(fmt.Sprintf(" %s Cancel   %s Commit", escKey, commitKey))

	return sb.String()
}
