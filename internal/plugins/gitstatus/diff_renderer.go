package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/styles"
)

// DiffViewMode specifies the diff rendering mode.
type DiffViewMode int

const (
	DiffViewUnified   DiffViewMode = iota // Line-by-line unified view
	DiffViewSideBySide                     // Side-by-side split view
)

// Additional styles for enhanced diff rendering
var (
	lineNumberStyle = lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Width(4).
			Align(lipgloss.Right)

	lineNumberSeparator = lipgloss.NewStyle().
				Foreground(styles.TextSubtle)

	wordDiffAddStyle = lipgloss.NewStyle().
				Foreground(styles.Success).
				Background(lipgloss.Color("#0D3320")).
				Bold(true)

	wordDiffRemoveStyle = lipgloss.NewStyle().
				Foreground(styles.Error).
				Background(lipgloss.Color("#3D1A1A")).
				Bold(true)

	hunkHeaderStyle = lipgloss.NewStyle().
			Foreground(styles.Info).
			Background(styles.BgSecondary).
			Bold(true)

	sideBySideBorder = lipgloss.NewStyle().
				Foreground(styles.BorderNormal)
)

// RenderLineDiff renders a parsed diff in unified line-by-line format with line numbers.
// horizontalOffset scrolls the content horizontally (0 = no scroll).
func RenderLineDiff(diff *ParsedDiff, width, startLine, maxLines, horizontalOffset int) string {
	if diff == nil || diff.Binary {
		if diff != nil && diff.Binary {
			return styles.Muted.Render(" Binary file differs")
		}
		return styles.Muted.Render(" No diff content")
	}

	var sb strings.Builder
	lineNum := 0
	rendered := 0

	// Calculate line number width based on max line number
	maxLineNo := diff.MaxLineNumber()
	lineNoWidth := len(fmt.Sprintf("%d", maxLineNo))
	if lineNoWidth < 4 {
		lineNoWidth = 4
	}

	lineNoStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Width(lineNoWidth).
		Align(lipgloss.Right)

	contentWidth := width - (lineNoWidth*2 + 4) // Two line numbers + separators

	for _, hunk := range diff.Hunks {
		// Skip until we reach the start line
		if lineNum < startLine {
			lineNum++
			if lineNum > startLine {
				// Render hunk header
				header := truncateLine(fmt.Sprintf("@@ -%d,%d +%d,%d @@%s",
					hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount, hunk.Header), contentWidth)
				sb.WriteString(hunkHeaderStyle.Render(header))
				sb.WriteString("\n")
				rendered++
			}
		} else {
			// Render hunk header
			header := truncateLine(fmt.Sprintf("@@ -%d,%d +%d,%d @@%s",
				hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount, hunk.Header), contentWidth)
			sb.WriteString(hunkHeaderStyle.Render(header))
			sb.WriteString("\n")
			rendered++
		}

		if rendered >= maxLines {
			break
		}

		for _, line := range hunk.Lines {
			lineNum++
			if lineNum <= startLine {
				continue
			}

			if rendered >= maxLines {
				break
			}

			// Format line numbers
			oldNo := " "
			newNo := " "
			if line.OldLineNo > 0 {
				oldNo = fmt.Sprintf("%d", line.OldLineNo)
			}
			if line.NewLineNo > 0 {
				newNo = fmt.Sprintf("%d", line.NewLineNo)
			}

			lineNos := fmt.Sprintf("%s %s │ ",
				lineNoStyle.Render(oldNo),
				lineNoStyle.Render(newNo))

			// Render content with appropriate style and horizontal offset
			content := renderDiffContentWithOffset(line, contentWidth, horizontalOffset)

			sb.WriteString(lineNos)
			sb.WriteString(content)
			sb.WriteString("\n")
			rendered++
		}

		if rendered >= maxLines {
			break
		}
	}

	return sb.String()
}

// RenderSideBySide renders a parsed diff in side-by-side format.
func RenderSideBySide(diff *ParsedDiff, width, startLine, maxLines, horizontalOffset int) string {
	if diff == nil || diff.Binary {
		if diff != nil && diff.Binary {
			return styles.Muted.Render(" Binary file differs")
		}
		return styles.Muted.Render(" No diff content")
	}

	var sb strings.Builder
	lineNum := 0
	rendered := 0

	// Calculate panel widths
	panelWidth := (width - 3) / 2 // -3 for center separator
	lineNoWidth := 5
	contentWidth := panelWidth - lineNoWidth - 2

	lineNoStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Width(lineNoWidth).
		Align(lipgloss.Right)

	for _, hunk := range diff.Hunks {
		if rendered >= maxLines {
			break
		}

		// Render hunk header across both panels
		if lineNum >= startLine {
			header := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
				hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
			sb.WriteString(hunkHeaderStyle.Render(padRight(header, width-1)))
			sb.WriteString("\n")
			rendered++
		}
		lineNum++

		// Group lines into pairs (remove/add or context)
		pairs := groupLinesForSideBySide(hunk.Lines)

		for _, pair := range pairs {
			if rendered >= maxLines {
				break
			}
			if lineNum <= startLine {
				lineNum++
				continue
			}

			// Left side (old)
			leftLineNo := " "
			leftContent := ""
			leftStyle := styles.DiffContext
			if pair.left != nil {
				if pair.left.OldLineNo > 0 {
					leftLineNo = fmt.Sprintf("%d", pair.left.OldLineNo)
				}
				leftContent = applyHorizontalOffset(pair.left.Content, horizontalOffset)
				leftContent = truncateLine(leftContent, contentWidth)
				if pair.left.Type == LineRemove {
					leftStyle = styles.DiffRemove
				}
			}

			// Right side (new)
			rightLineNo := " "
			rightContent := ""
			rightStyle := styles.DiffContext
			if pair.right != nil {
				if pair.right.NewLineNo > 0 {
					rightLineNo = fmt.Sprintf("%d", pair.right.NewLineNo)
				}
				rightContent = applyHorizontalOffset(pair.right.Content, horizontalOffset)
				rightContent = truncateLine(rightContent, contentWidth)
				if pair.right.Type == LineAdd {
					rightStyle = styles.DiffAdd
				}
			}

			// Compose line
			leftPanel := fmt.Sprintf("%s │%s",
				lineNoStyle.Render(leftLineNo),
				leftStyle.Render(padRight(leftContent, contentWidth)))

			rightPanel := fmt.Sprintf("%s │%s",
				lineNoStyle.Render(rightLineNo),
				rightStyle.Render(padRight(rightContent, contentWidth)))

			sb.WriteString(leftPanel)
			sb.WriteString(sideBySideBorder.Render(" │ "))
			sb.WriteString(rightPanel)
			sb.WriteString("\n")
			rendered++
			lineNum++
		}
	}

	return sb.String()
}

// linePair represents a pair of lines for side-by-side view.
type linePair struct {
	left  *DiffLine
	right *DiffLine
}

// groupLinesForSideBySide groups diff lines into pairs for side-by-side display.
func groupLinesForSideBySide(lines []DiffLine) []linePair {
	var pairs []linePair
	i := 0

	for i < len(lines) {
		line := &lines[i]

		switch line.Type {
		case LineContext:
			// Context lines appear on both sides
			pairs = append(pairs, linePair{left: line, right: line})
			i++

		case LineRemove:
			// Check if followed by add lines
			removeStart := i
			for i < len(lines) && lines[i].Type == LineRemove {
				i++
			}
			removeEnd := i

			addStart := i
			for i < len(lines) && lines[i].Type == LineAdd {
				i++
			}
			addEnd := i

			// Pair up removes with adds
			removeCount := removeEnd - removeStart
			addCount := addEnd - addStart
			maxPairs := removeCount
			if addCount > maxPairs {
				maxPairs = addCount
			}

			for j := 0; j < maxPairs; j++ {
				var left, right *DiffLine
				if j < removeCount {
					left = &lines[removeStart+j]
				}
				if j < addCount {
					right = &lines[addStart+j]
				}
				pairs = append(pairs, linePair{left: left, right: right})
			}

		case LineAdd:
			// Orphan add (shouldn't happen if grouping is correct)
			pairs = append(pairs, linePair{left: nil, right: line})
			i++
		}
	}

	return pairs
}

// renderDiffContentWithOffset renders line content with horizontal scroll and word-level highlighting.
func renderDiffContentWithOffset(line DiffLine, maxWidth, horizontalOffset int) string {
	// Apply horizontal offset first
	content := line.Content
	if horizontalOffset > 0 {
		runes := []rune(content)
		if horizontalOffset >= len(runes) {
			content = ""
		} else {
			content = string(runes[horizontalOffset:])
		}
		// Create a modified line with offset content
		line = DiffLine{
			Type:      line.Type,
			Content:   content,
			OldLineNo: line.OldLineNo,
			NewLineNo: line.NewLineNo,
			WordDiff:  nil, // Word diff doesn't work well with offset
		}
	}
	return renderDiffContent(line, maxWidth)
}

// renderDiffContent renders line content with word-level highlighting.
func renderDiffContent(line DiffLine, maxWidth int) string {
	var style lipgloss.Style
	switch line.Type {
	case LineAdd:
		style = styles.DiffAdd
	case LineRemove:
		style = styles.DiffRemove
	default:
		style = styles.DiffContext
	}

	// If we have word diff data, use it
	if len(line.WordDiff) > 0 {
		var sb strings.Builder
		for _, segment := range line.WordDiff {
			if segment.IsChange {
				if line.Type == LineAdd {
					sb.WriteString(wordDiffAddStyle.Render(segment.Text))
				} else {
					sb.WriteString(wordDiffRemoveStyle.Render(segment.Text))
				}
			} else {
				sb.WriteString(style.Render(segment.Text))
			}
		}
		content := sb.String()
		// Truncate if needed (accounting for ANSI codes is complex, so just truncate raw)
		runes := []rune(line.Content)
		if len(runes) > maxWidth && maxWidth > 3 {
			// Re-render truncated
			truncated := string(runes[:maxWidth-3]) + "..."
			return style.Render(truncated)
		}
		return content
	}

	content := line.Content
	runes := []rune(content)
	if len(runes) > maxWidth && maxWidth > 3 {
		content = string(runes[:maxWidth-3]) + "..."
	}
	return style.Render(content)
}

// truncateLine truncates a line to fit within maxWidth using visual width.
func truncateLine(s string, maxWidth int) string {
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

// padRight pads a string with spaces to reach the desired visual width.
func padRight(s string, width int) string {
	runeLen := len([]rune(s))
	if runeLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeLen)
}

// applyHorizontalOffset removes the first n runes from a string.
func applyHorizontalOffset(s string, offset int) string {
	if offset <= 0 {
		return s
	}
	runes := []rune(s)
	if offset >= len(runes) {
		return ""
	}
	return string(runes[offset:])
}
