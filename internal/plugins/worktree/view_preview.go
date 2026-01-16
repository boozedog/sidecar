package worktree

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/marcus/sidecar/internal/styles"
)

// renderPreviewContent renders the preview pane content (no borders).
func (p *Plugin) renderPreviewContent(width, height int) string {
	var lines []string

	// Hide tabs when no worktree is selected - show welcome guide instead
	wt := p.selectedWorktree()
	if wt == nil {
		return truncateAllLines(p.renderWelcomeGuide(width, height), width)
	}

	// Tab header
	tabs := p.renderTabs(width)
	lines = append(lines, tabs)
	lines = append(lines, "") // Empty line after header

	contentHeight := height - 2 // header + empty line

	// Render content based on active tab
	var content string
	switch p.previewTab {
	case PreviewTabOutput:
		content = p.renderOutputContent(width, contentHeight)
	case PreviewTabDiff:
		content = p.renderDiffContent(width, contentHeight)
	case PreviewTabTask:
		content = p.renderTaskContent(width, contentHeight)
	}

	lines = append(lines, content)

	// Final safety: ensure ALL lines are truncated to width
	// This catches any content that wasn't properly truncated
	result := strings.Join(lines, "\n")
	return truncateAllLines(result, width)
}

// renderWelcomeGuide renders a helpful guide when no worktree is selected.
func (p *Plugin) renderWelcomeGuide(width, height int) string {
	var lines []string

	// Section Style
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

	// Git Worktree Explanation
	lines = append(lines, sectionStyle.Render("Git Worktrees: A Better Workflow"))
	lines = append(lines, dimText("  • Parallel Development: Work on multiple branches simultaneously"))
	lines = append(lines, dimText("    in separate directories."))
	lines = append(lines, dimText("  • No Context Switching: Keep your editor/server running while"))
	lines = append(lines, dimText("    reviewing a PR or fixing a bug."))
	lines = append(lines, dimText("  • Isolated Environments: Each worktree has its own clean state,"))
	lines = append(lines, dimText("    unaffected by other changes."))
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("─", min(width-4, 60)))
	lines = append(lines, "")

	// Title
	title := lipgloss.NewStyle().Bold(true).Render("tmux Quick Reference")
	lines = append(lines, title)
	lines = append(lines, "")

	// Section: Attaching to agent sessions
	lines = append(lines, sectionStyle.Render("Agent Sessions"))
	lines = append(lines, dimText("  Enter      Attach to selected worktree session"))
	lines = append(lines, dimText("  Ctrl-b d   Detach from session (return here)"))
	lines = append(lines, "")

	// Section: Navigation inside tmux
	lines = append(lines, sectionStyle.Render("Scrolling (in attached session)"))
	lines = append(lines, dimText("  Ctrl-b [        Enter scroll mode"))
	lines = append(lines, dimText("  PgUp/PgDn       Scroll page (fn+↑/↓ on Mac)"))
	lines = append(lines, dimText("  ↑/↓             Scroll line by line"))
	lines = append(lines, dimText("  q               Exit scroll mode"))
	lines = append(lines, "")

	// Section: Interacting with editors
	lines = append(lines, sectionStyle.Render("Editor Navigation"))
	lines = append(lines, dimText("  When agent opens vim/nano:"))
	lines = append(lines, dimText("    :q!      Quit vim without saving"))
	lines = append(lines, dimText("    :wq      Save and quit vim"))
	lines = append(lines, dimText("    Ctrl-x   Exit nano"))
	lines = append(lines, "")

	// Section: Common tasks
	lines = append(lines, sectionStyle.Render("Tips"))
	lines = append(lines, dimText("  • Create a worktree with 'n' to start"))
	lines = append(lines, dimText("  • Agent output streams in the Output tab"))
	lines = append(lines, dimText("  • Attach to interact with the agent directly"))
	lines = append(lines, "")
	lines = append(lines, dimText("Customize tmux: ~/.tmux.conf (man tmux for options)"))

	return strings.Join(lines, "\n")
}

// truncateAllLines ensures every line in the content is truncated to maxWidth.
func truncateAllLines(content string, maxWidth int) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = expandTabs(line, tabStopWidth)
		if lipgloss.Width(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "")
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// renderTabs renders the preview pane tab header.
func (p *Plugin) renderTabs(width int) string {
	tabs := []string{"Output", "Diff", "Task"}
	var rendered []string

	for i, tab := range tabs {
		if PreviewTab(i) == p.previewTab {
			rendered = append(rendered, styles.BarChipActive.Render(" "+tab+" "))
		} else {
			rendered = append(rendered, styles.BarChip.Render(" "+tab+" "))
		}
	}

	return strings.Join(rendered, " ")
}

// renderOutputContent renders agent output.
func (p *Plugin) renderOutputContent(width, height int) string {
	wt := p.selectedWorktree()
	if wt == nil {
		return dimText("No worktree selected")
	}

	if wt.Agent == nil {
		return dimText("No agent running\nPress 's' to start an agent")
	}

	// Hint for tmux detach
	hint := dimText("enter to attach • Ctrl-b d to detach")
	height-- // Reserve line for hint

	if wt.Agent.OutputBuf == nil {
		return hint + "\n" + dimText("No output yet")
	}

	lines := wt.Agent.OutputBuf.Lines()
	if len(lines) == 0 {
		return hint + "\n" + dimText("No output yet")
	}

	var start, end int
	if p.autoScrollOutput {
		// Auto-scroll: show newest content (last height lines)
		start = len(lines) - height
		if start < 0 {
			start = 0
		}
		end = len(lines)
	} else {
		// Manual scroll: previewOffset is lines from bottom
		// offset=0 means bottom, offset=N means N lines up from bottom
		start = len(lines) - height - p.previewOffset
		if start < 0 {
			start = 0
		}
		end = start + height
		if end > len(lines) {
			end = len(lines)
		}
	}

	// Apply horizontal offset and truncate each line
	var displayLines []string
	for _, line := range lines[start:end] {
		displayLine := expandTabs(line, tabStopWidth)
		// Apply horizontal offset using ANSI-aware truncation
		if p.previewHorizOffset > 0 {
			displayLine = ansi.TruncateLeft(displayLine, p.previewHorizOffset, "")
		}
		// Truncate to width if needed
		if lipgloss.Width(displayLine) > width {
			displayLine = ansi.Truncate(displayLine, width, "")
		}
		displayLines = append(displayLines, displayLine)
	}

	return hint + "\n" + strings.Join(displayLines, "\n")
}

// renderCommitStatusHeader renders the commit status header for diff view.
func (p *Plugin) renderCommitStatusHeader(width int) string {
	if len(p.commitStatusList) == 0 {
		return ""
	}

	// Box style for header
	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1).
		Width(width - 2)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	hashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	pushedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	localStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(fmt.Sprintf("Commits (%d)", len(p.commitStatusList))))
	sb.WriteString("\n")

	// Show up to 5 commits
	maxCommits := 5
	displayCount := len(p.commitStatusList)
	if displayCount > maxCommits {
		displayCount = maxCommits
	}

	for i := 0; i < displayCount; i++ {
		commit := p.commitStatusList[i]

		// Status icon
		var statusIcon string
		if commit.Pushed {
			statusIcon = pushedStyle.Render("↑")
		} else {
			statusIcon = localStyle.Render("○")
		}

		// Truncate subject to fit
		subject := commit.Subject
		maxSubjectLen := width - 15 // hash(7) + icon(2) + spaces(6)
		if maxSubjectLen < 10 {
			maxSubjectLen = 10
		}
		if len(subject) > maxSubjectLen {
			subject = subject[:maxSubjectLen-3] + "..."
		}

		line := fmt.Sprintf("%s %s %s", statusIcon, hashStyle.Render(commit.Hash), subject)
		sb.WriteString(line)
		if i < displayCount-1 {
			sb.WriteString("\n")
		}
	}

	if len(p.commitStatusList) > maxCommits {
		sb.WriteString("\n")
		sb.WriteString(dimText(fmt.Sprintf("  ... and %d more", len(p.commitStatusList)-maxCommits)))
	}

	return headerStyle.Render(sb.String())
}

// renderTaskContent renders linked task info.
func (p *Plugin) renderTaskContent(width, height int) string {
	wt := p.selectedWorktree()
	if wt == nil {
		return dimText("No worktree selected")
	}

	if wt.TaskID == "" {
		return dimText("No linked task\nPress 't' to link a task")
	}

	// Check if we have cached details for this task
	if p.cachedTask == nil || p.cachedTaskID != wt.TaskID {
		return dimText(fmt.Sprintf("Loading task %s...", wt.TaskID))
	}

	task := p.cachedTask
	var lines []string

	// Mode indicator
	modeHint := dimText("[m] raw")
	if p.taskMarkdownMode {
		modeHint = dimText("[m] rendered")
	}

	// Header
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Task: %s", task.ID))+"  "+modeHint)

	// Status and priority
	statusLine := fmt.Sprintf("Status: %s", task.Status)
	if task.Priority != "" {
		statusLine += fmt.Sprintf("  Priority: %s", task.Priority)
	}
	if task.Type != "" {
		statusLine += fmt.Sprintf("  Type: %s", task.Type)
	}
	lines = append(lines, statusLine)
	lines = append(lines, strings.Repeat("─", min(width-4, 60)))
	lines = append(lines, "")

	// Title
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render(task.Title))
	lines = append(lines, "")

	// Markdown rendering for description and acceptance
	if p.taskMarkdownMode && p.markdownRenderer != nil {
		// Build markdown content
		var mdContent strings.Builder
		if task.Description != "" {
			mdContent.WriteString(task.Description)
			mdContent.WriteString("\n\n")
		}
		if task.Acceptance != "" {
			mdContent.WriteString("## Acceptance Criteria\n\n")
			mdContent.WriteString(task.Acceptance)
		}

		// Check if we need to re-render (width changed or cache empty)
		if p.taskMarkdownWidth != width || len(p.taskMarkdownRendered) == 0 {
			p.taskMarkdownRendered = p.markdownRenderer.RenderContent(mdContent.String(), width-4)
			p.taskMarkdownWidth = width
		}

		// Append rendered lines
		lines = append(lines, p.taskMarkdownRendered...)
	} else {
		// Plain text fallback
		if task.Description != "" {
			wrapped := wrapText(task.Description, width-4)
			lines = append(lines, wrapped)
			lines = append(lines, "")
		}

		if task.Acceptance != "" {
			lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Acceptance Criteria:"))
			wrapped := wrapText(task.Acceptance, width-4)
			lines = append(lines, wrapped)
			lines = append(lines, "")
		}
	}

	// Timestamps (dimmed)
	lines = append(lines, "")
	if task.CreatedAt != "" {
		lines = append(lines, dimText(fmt.Sprintf("Created: %s", task.CreatedAt)))
	}
	if task.UpdatedAt != "" {
		lines = append(lines, dimText(fmt.Sprintf("Updated: %s", task.UpdatedAt)))
	}

	return strings.Join(lines, "\n")
}
