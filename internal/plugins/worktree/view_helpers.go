package worktree

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/marcus/sidecar/internal/styles"
)

// expandTabs replaces tabs with spaces, preserving ANSI sequences and column widths.
func expandTabs(line string, tabWidth int) string {
	if tabWidth <= 0 || !strings.Contains(line, "\t") {
		return line
	}

	var sb strings.Builder
	sb.Grow(len(line))

	state := ansi.NormalState
	column := 0
	for len(line) > 0 {
		seq, width, n, newState := ansi.GraphemeWidth.DecodeSequenceInString(line, state, nil)
		if n <= 0 {
			sb.WriteString(line)
			break
		}
		if seq == "\t" && width == 0 {
			spaces := tabWidth - (column % tabWidth)
			if spaces == 0 {
				spaces = tabWidth
			}
			sb.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		} else {
			sb.WriteString(seq)
			column += width
		}
		state = newState
		line = line[n:]
	}

	return sb.String()
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var lines []string
	for _, para := range strings.Split(text, "\n") {
		if len(para) <= width {
			lines = append(lines, para)
			continue
		}

		// Simple word wrapping
		words := strings.Fields(para)
		var currentLine string
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= width {
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
	return strings.Join(lines, "\n")
}

// dimText renders dim placeholder text using theme style.
func dimText(s string) string {
	return styles.Muted.Render(s)
}

// formatRelativeTime formats a time as relative (e.g., "3m", "2h").
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
