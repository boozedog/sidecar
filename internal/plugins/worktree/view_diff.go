package worktree

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/plugins/gitstatus"
	"github.com/marcus/sidecar/internal/styles"
)

// renderDiffContent renders git diff using the shared diff renderer.
func (p *Plugin) renderDiffContent(width, height int) string {
	wt := p.selectedWorktree()
	if wt == nil {
		return dimText("No worktree selected")
	}

	// Render commit status header if it belongs to current worktree
	header := ""
	if p.commitStatusWorktree == wt.Name {
		header = p.renderCommitStatusHeader(width)
	}

	headerHeight := 0
	if header != "" {
		headerHeight = lipgloss.Height(header) + 1 // +1 for blank line
	}

	if p.diffRaw == "" {
		if header != "" {
			return header + "\n" + dimText("No uncommitted changes")
		}
		return dimText("No changes")
	}

	// Adjust available height for diff content
	contentHeight := height - headerHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Parse the raw diff into structured format
	parsed, err := gitstatus.ParseUnifiedDiff(p.diffRaw)
	if err != nil || parsed == nil {
		// Fallback to basic rendering
		diffContent := p.renderDiffContentBasicWithHeight(width, contentHeight)
		if header != "" {
			return header + "\n" + diffContent
		}
		return diffContent
	}

	// Create syntax highlighter if we have file info
	var highlighter *gitstatus.SyntaxHighlighter
	if parsed.NewFile != "" {
		highlighter = gitstatus.NewSyntaxHighlighter(parsed.NewFile)
	}

	// Render based on view mode
	var diffContent string
	if p.diffViewMode == DiffViewSideBySide {
		diffContent = gitstatus.RenderSideBySide(parsed, width, p.previewOffset, contentHeight, p.previewHorizOffset, highlighter)
	} else {
		diffContent = gitstatus.RenderLineDiff(parsed, width, p.previewOffset, contentHeight, p.previewHorizOffset, highlighter)
	}

	if header != "" {
		return header + "\n" + diffContent
	}
	return diffContent
}

// renderDiffContentBasic renders git diff with basic highlighting (fallback).
func (p *Plugin) renderDiffContentBasic(width, height int) string {
	return p.renderDiffContentBasicWithHeight(width, height)
}

// renderDiffContentBasicWithHeight renders git diff with basic highlighting with explicit height.
func (p *Plugin) renderDiffContentBasicWithHeight(width, height int) string {
	lines := splitLines(p.diffContent)

	// Apply scroll offset
	start := p.previewOffset
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}

	// Diff highlighting with horizontal scroll support
	var rendered []string
	for _, line := range lines[start:end] {
		line = expandTabs(line, tabStopWidth)
		var styledLine string
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			styledLine = styles.DiffHeader.Render(line)
		case strings.HasPrefix(line, "@@"):
			styledLine = lipgloss.NewStyle().Foreground(styles.Info).Render(line)
		case strings.HasPrefix(line, "+"):
			styledLine = styles.DiffAdd.Render(line)
		case strings.HasPrefix(line, "-"):
			styledLine = styles.DiffRemove.Render(line)
		default:
			styledLine = line
		}

		if p.previewHorizOffset > 0 {
			styledLine = p.truncateCache.TruncateLeft(styledLine, p.previewHorizOffset, "")
		}
		if lipgloss.Width(styledLine) > width {
			styledLine = p.truncateCache.Truncate(styledLine, width, "")
		}
		rendered = append(rendered, styledLine)
	}

	return strings.Join(rendered, "\n")
}

// colorDiffLine applies basic diff coloring using theme styles.
func (p *Plugin) colorDiffLine(line string, width int) string {
	line = expandTabs(line, tabStopWidth)
	if len(line) == 0 {
		return line
	}

	// Truncate if needed
	if lipgloss.Width(line) > width {
		line = p.truncateCache.Truncate(line, width, "")
	}

	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "@@"):
		return lipgloss.NewStyle().Foreground(styles.Info).Render(line)
	case strings.HasPrefix(line, "+"):
		return styles.DiffAdd.Render(line)
	case strings.HasPrefix(line, "-"):
		return styles.DiffRemove.Render(line)
	default:
		return line
	}
}
