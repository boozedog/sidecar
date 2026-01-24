package gitstatus

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

// renderPullMenu renders the pull options popup menu.
func (p *Plugin) renderPullMenu() string {
	background := p.renderThreePaneView()

	var sb strings.Builder

	sb.WriteString(styles.Title.Render(" Pull "))
	sb.WriteString("\n\n")

	options := []struct{ key, label string }{
		{"p", " Pull (merge) "},
		{"r", " Pull (rebase) "},
		{"f", " Pull (fast-forward only) "},
		{"a", " Pull (rebase + autostash) "},
	}

	for i, opt := range options {
		style := ui.ResolveButtonStyle(p.pullMenuFocus, p.pullMenuHover, i)
		keyHint := styles.KeyHint.Render(" " + opt.key + " ")
		sb.WriteString(keyHint)
		sb.WriteString(" ")
		sb.WriteString(style.Render(opt.label))
		if i < len(options)-1 {
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(styles.Muted.Render("Tab/↑↓ to navigate, Enter to select, Esc to cancel"))

	menuWidth := ui.ModalWidthMedium

	menuContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Padding(1, 2).
		Width(menuWidth).
		Render(sb.String())

	return ui.OverlayModal(background, menuContent, p.width, p.height)
}

// renderPullConflict renders the pull conflict resolution modal.
func (p *Plugin) renderPullConflict() string {
	background := p.renderThreePaneView()

	var sb strings.Builder

	sb.WriteString(styles.StatusDeleted.Render(" Conflicts "))
	sb.WriteString("\n\n")

	// Show conflict type
	conflictLabel := "Merge"
	if p.pullConflictType == "rebase" {
		conflictLabel = "Rebase"
	}
	sb.WriteString(styles.Muted.Render(fmt.Sprintf("%s produced conflicts in %d file(s):", conflictLabel, len(p.pullConflictFiles))))
	sb.WriteString("\n\n")

	// Show conflicted files (max 8)
	maxFiles := 8
	for i, f := range p.pullConflictFiles {
		if i >= maxFiles {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("  ... and %d more", len(p.pullConflictFiles)-maxFiles)))
			sb.WriteString("\n")
			break
		}
		sb.WriteString(styles.StatusModified.Render("  U " + f))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Action options
	abortStyle := ui.ResolveButtonStyle(p.pullMenuFocus, p.pullMenuHover, 0)
	sb.WriteString(styles.KeyHint.Render(" a "))
	sb.WriteString(" ")
	sb.WriteString(abortStyle.Render(" Abort "))

	sb.WriteString("\n\n")
	sb.WriteString(styles.Muted.Render("Resolve conflicts in your editor, then commit."))

	menuWidth := ui.ModalWidthMedium

	menuContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Error).
		Padding(1, 2).
		Width(menuWidth).
		Render(sb.String())

	return ui.OverlayModal(background, menuContent, p.width, p.height)
}
