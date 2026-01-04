package gitstatus

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/msg"
)

// copyCommitIDToClipboard copies just the short commit hash to clipboard.
func (p *Plugin) copyCommitIDToClipboard() tea.Cmd {
	commit := p.getCurrentCommit()
	if commit == nil {
		return nil
	}

	if err := clipboard.WriteAll(commit.ShortHash); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Yanked: "+commit.ShortHash, 2*time.Second)
}

// copyCommitToClipboard copies full commit details as markdown to clipboard.
func (p *Plugin) copyCommitToClipboard() tea.Cmd {
	commit := p.getCurrentCommit()
	if commit == nil {
		return nil
	}

	markdown := formatCommitAsMarkdown(commit)
	if err := clipboard.WriteAll(markdown); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Yanked commit details", 2*time.Second)
}

// getCurrentCommit returns the commit under cursor based on current view mode.
func (p *Plugin) getCurrentCommit() *Commit {
	switch p.viewMode {
	case ViewModeStatus:
		// In status view, check sidebar commits or preview commit
		if p.activePane == PaneDiff && p.previewCommit != nil {
			return p.previewCommit
		}
		if p.cursorOnCommit() && p.recentCommits != nil {
			commitIdx := p.cursor - len(p.tree.AllEntries())
			if commitIdx >= 0 && commitIdx < len(p.recentCommits) {
				return p.recentCommits[commitIdx]
			}
		}
	}
	return nil
}

// formatCommitAsMarkdown formats a commit as markdown for clipboard.
func formatCommitAsMarkdown(commit *Commit) string {
	var sb strings.Builder

	// Subject as heading
	sb.WriteString(fmt.Sprintf("# %s\n\n", commit.Subject))

	// Metadata
	sb.WriteString(fmt.Sprintf("**Commit:** `%s`\n", commit.ShortHash))
	sb.WriteString(fmt.Sprintf("**Author:** %s <%s>\n", commit.Author, commit.AuthorEmail))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", commit.Date.Format("2006-01-02 15:04:05")))

	// Stats if available
	if commit.Stats.FilesChanged > 0 {
		sb.WriteString(fmt.Sprintf("**Stats:** %d file(s), +%d/-%d\n",
			commit.Stats.FilesChanged, commit.Stats.Additions, commit.Stats.Deletions))
	}

	// Body if present
	if commit.Body != "" {
		sb.WriteString("\n## Message\n\n")
		sb.WriteString(commit.Body)
		sb.WriteString("\n")
	}

	// Files if present
	if len(commit.Files) > 0 {
		sb.WriteString("\n## Files Changed\n\n")
		for _, f := range commit.Files {
			status := fileStatusIcon(f.Status)
			sb.WriteString(fmt.Sprintf("- %s `%s`", status, f.Path))
			if f.Additions > 0 || f.Deletions > 0 {
				sb.WriteString(fmt.Sprintf(" (+%d/-%d)", f.Additions, f.Deletions))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// fileStatusIcon returns a short icon for file status.
func fileStatusIcon(status FileStatus) string {
	switch status {
	case StatusAdded:
		return "[A]"
	case StatusModified:
		return "[M]"
	case StatusDeleted:
		return "[D]"
	case StatusRenamed:
		return "[R]"
	case StatusCopied:
		return "[C]"
	default:
		return "[?]"
	}
}
