package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/styles"
)

// RenderDivider renders a vertical divider for separating panes.
// The divider uses BorderNormal color and is shifted down by 1 line
// to align with bordered pane content (below top border).
// Height should be the full pane height; divider renders height-2 lines
// to stop above the bottom border.
func RenderDivider(height int) string {
	dividerStyle := lipgloss.NewStyle().
		Foreground(styles.BorderNormal).
		MarginTop(1)

	// Build vertical bar (height-2 to stop above bottom border)
	var sb strings.Builder
	for i := 0; i < height-2; i++ {
		sb.WriteString("│")
		if i < height-3 {
			sb.WriteString("\n")
		}
	}

	return dividerStyle.Render(sb.String())
}
