package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

// renderPushMenu renders the push options popup menu.
func (p *Plugin) renderPushMenu() string {
	// Render the background (current view dimmed)
	background := p.renderThreePaneView()

	// Build menu content
	var sb strings.Builder

	// Menu options
	options := []struct{ key, label string }{
		{"p", "Push to origin"},
		{"f", "Force push (--force-with-lease)"},
		{"u", "Push & set upstream (-u)"},
	}

	for i, opt := range options {
		key := styles.KeyHint.Render(" " + opt.key + " ")
		sb.WriteString(fmt.Sprintf("  %s  %s", key, opt.label))
		if i < len(options)-1 {
			sb.WriteString("\n\n") // Spacing between options
		} else {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render("  Esc to cancel"))

	// Create menu box - wide enough for longest option
	menuWidth := 44
	title := styles.Title.Render(" Push ")

	menuContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2).
		Width(menuWidth).
		Render(title + "\n\n" + sb.String())

	// Overlay menu on dimmed background
	return ui.OverlayModal(background, menuContent, p.width, p.height)
}
