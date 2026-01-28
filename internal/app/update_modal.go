package app

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/markdown"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

const changelogURL = "https://raw.githubusercontent.com/marcus/sidecar/main/CHANGELOG.md"

// updateModalWidth returns the appropriate modal width based on screen size.
func (m *Model) updateModalWidth() int {
	modalW := 60
	maxW := m.width - 4
	if maxW < 20 {
		maxW = 20 // Absolute minimum for very small screens
	}
	if modalW > maxW {
		modalW = maxW
	}
	if modalW < 30 {
		modalW = 30
	}
	// Final cap: never exceed available space
	if modalW > maxW {
		modalW = maxW
	}
	return modalW
}

// renderUpdateModalOverlay renders the update modal as an overlay on top of background.
func (m *Model) renderUpdateModalOverlay(background string) string {
	// Render modal content based on state
	var modalContent string
	switch m.updateModalState {
	case UpdateModalPreview:
		modalContent = m.renderUpdatePreviewModal()
	case UpdateModalProgress:
		modalContent = m.renderUpdateProgressModal()
	case UpdateModalComplete:
		modalContent = m.renderUpdateCompleteModal()
	case UpdateModalError:
		modalContent = m.renderUpdateErrorModal()
	default:
		return background
	}

	return ui.OverlayModal(background, modalContent, m.width, m.height)
}

// ensureUpdatePreviewModal creates/updates the preview modal with caching.
func (m *Model) ensureUpdatePreviewModal() {
	if m.updateAvailable == nil {
		return
	}
	modalW := m.updateModalWidth()
	if m.updatePreviewModal != nil && m.updatePreviewModalWidth == modalW {
		return
	}
	m.updatePreviewModalWidth = modalW
	contentW := modalW - 6 // borders + padding

	// Version line
	currentV := m.updateAvailable.CurrentVersion
	latestV := m.updateAvailable.LatestVersion
	arrow := lipgloss.NewStyle().Foreground(styles.Success).Render(" → ")
	versionLine := fmt.Sprintf("%s%s%s", currentV, arrow, latestV)

	// Release notes
	releaseNotes := m.updateReleaseNotes
	if releaseNotes == "" {
		releaseNotes = m.updateAvailable.ReleaseNotes
	}
	if releaseNotes == "" {
		releaseNotes = "No release notes available."
	}
	releaseNotes = parseReleaseNotes(releaseNotes)
	renderedNotes := m.renderReleaseNotes(releaseNotes, contentW)

	// Limit height
	lines := strings.Split(renderedNotes, "\n")
	maxLines := 15
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, styles.Muted.Render("... (truncated)"))
	}
	notesContent := strings.Join(lines, "\n")

	changelogHint := styles.Muted.Render("[c] View Full Changelog")

	m.updatePreviewModal = modal.New("Sidecar Update",
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantDefault),
		modal.WithPrimaryAction("update"),
	).
		AddSection(modal.Text(versionLine)).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(lipgloss.NewStyle().Bold(true).Render("What's New"))).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(notesContent)).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(changelogHint)).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Update Now ", "update"),
			modal.Btn(" Later ", "cancel"),
		))
}

// renderUpdatePreviewModal renders the preview state showing release notes before update.
func (m *Model) renderUpdatePreviewModal() string {
	m.ensureUpdatePreviewModal()
	if m.updatePreviewModal == nil {
		return ""
	}
	if m.updatePreviewMouseHandler == nil {
		m.updatePreviewMouseHandler = mouse.NewHandler()
	}
	return m.updatePreviewModal.Render(m.width, m.height, m.updatePreviewMouseHandler)
}

// clearUpdatePreviewModal clears the preview modal cache.
func (m *Model) clearUpdatePreviewModal() {
	m.updatePreviewModal = nil
	m.updatePreviewModalWidth = 0
	m.updatePreviewMouseHandler = nil
}

// parseReleaseNotes cleans up release notes by removing duplicate headers
// and excessive whitespace. The modal already shows "What's New" as a header,
// so we strip any leading "What's New" headers from the content.
func parseReleaseNotes(notes string) string {
	if notes == "" {
		return notes
	}

	// Patterns to strip from the beginning of release notes
	// Match: ## What's New, ### What's New, # What's New (case-insensitive)
	// Also match: # Release Notes, ## Release Notes
	headerPatterns := regexp.MustCompile(`(?im)^#+\s*(what'?s?\s*new|release\s*notes)\s*\n*`)

	result := notes

	// Strip leading whitespace and newlines first
	result = strings.TrimSpace(result)

	// Repeatedly strip matching headers from the beginning
	// (in case there are multiple, e.g., "## What's New\n### What's New\n")
	for {
		loc := headerPatterns.FindStringIndex(result)
		if loc == nil || loc[0] != 0 {
			break
		}
		result = result[loc[1]:]
		result = strings.TrimSpace(result)
	}

	// Collapse multiple consecutive newlines to at most 2
	multiNewlines := regexp.MustCompile(`\n{3,}`)
	result = multiNewlines.ReplaceAllString(result, "\n\n")

	// If parsing emptied the content, return original
	if strings.TrimSpace(result) == "" {
		return strings.TrimSpace(notes)
	}

	return result
}

// renderReleaseNotes renders markdown release notes.
func (m *Model) renderReleaseNotes(notes string, width int) string {
	// Try to use markdown renderer
	renderer, err := markdown.NewRenderer()
	if err != nil {
		return notes
	}

	lines := renderer.RenderContent(notes, width)
	return strings.Join(lines, "\n")
}

// centerText centers text within a given width.
func centerText(text string, width int) string {
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return text
	}
	padding := (width - textWidth) / 2
	return strings.Repeat(" ", padding) + text
}

// renderUpdateProgressModal renders the progress state during update.
func (m *Model) renderUpdateProgressModal() string {
	modalW := m.updateModalWidth()
	contentW := modalW - 4

	var sb strings.Builder

	// Title
	title := lipgloss.NewStyle().Bold(true).Foreground(styles.Warning).Render("Updating Sidecar")
	sb.WriteString(centerText(title, contentW))
	sb.WriteString("\n\n")

	// Version being installed
	if m.updateAvailable != nil {
		version := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(
			fmt.Sprintf("Installing %s", m.updateAvailable.LatestVersion))
		sb.WriteString(centerText(version, contentW))
		sb.WriteString("\n\n")
	}

	// Phase indicators - 3 real, observable phases
	phases := []UpdatePhase{PhaseCheckPrereqs, PhaseInstalling, PhaseVerifying}
	for _, phase := range phases {
		status := m.updatePhaseStatus[phase]
		icon := "○" // pending
		color := styles.TextMuted

		switch status {
		case "running":
			icon = "●"
			color = styles.Warning
		case "done":
			icon = "✓"
			color = styles.Success
		case "error":
			icon = "✗"
			color = styles.Error
		}

		phaseName := phase.String()
		if phase == m.updatePhase && status == "running" {
			phaseName = lipgloss.NewStyle().Bold(true).Render(phaseName)
		}

		phaseLine := fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Foreground(color).Render(icon),
			phaseName,
		)
		sb.WriteString(phaseLine)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Elapsed time
	elapsed := m.getUpdateElapsed()
	elapsedStr := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(
		fmt.Sprintf("Elapsed: %s", formatElapsed(elapsed)))
	sb.WriteString(centerText(elapsedStr, contentW))
	sb.WriteString("\n\n")

	// Divider
	sb.WriteString(lipgloss.NewStyle().Foreground(styles.TextMuted).Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n\n")

	// Cancel hint
	cancelHint := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Esc: cancel")
	sb.WriteString(centerText(cancelHint, contentW))

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.TextMuted).
		Padding(1, 2).
		Width(modalW)

	return modalStyle.Render(sb.String())
}

// getUpdateElapsed returns the elapsed time since update started.
func (m *Model) getUpdateElapsed() time.Duration {
	if m.updateStartTime.IsZero() {
		return 0
	}
	return time.Since(m.updateStartTime)
}

// formatElapsed formats a duration as M:SS.
func formatElapsed(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// ensureUpdateCompleteModal creates/updates the complete modal with caching.
func (m *Model) ensureUpdateCompleteModal() {
	modalW := m.updateModalWidth()
	if m.updateCompleteModal != nil && m.updateCompleteModalWidth == modalW {
		return
	}
	m.updateCompleteModalWidth = modalW

	checkmark := lipgloss.NewStyle().Foreground(styles.Success).Render("✓")

	var updatesText string
	if m.updateAvailable != nil {
		updatesText = fmt.Sprintf("  %s Sidecar updated to %s", checkmark, m.updateAvailable.LatestVersion)
	} else {
		updatesText = fmt.Sprintf("  %s Sidecar updated", checkmark)
	}
	if m.tdVersionInfo != nil && m.tdVersionInfo.HasUpdate {
		updatesText += fmt.Sprintf("\n  %s td updated to %s", checkmark, m.tdVersionInfo.LatestVersion)
	}

	restartMsg := styles.Muted.Render("Restart sidecar to use the new version.")
	tip := styles.Muted.Render("Tip: Press q to quit, then run 'sidecar' again.")

	m.updateCompleteModal = modal.New("Update Complete!",
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantInfo),
		modal.WithPrimaryAction("quit"),
	).
		AddSection(modal.Text(updatesText)).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(restartMsg)).
		AddSection(modal.Text(tip)).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Quit & Restart ", "quit"),
			modal.Btn(" Later ", "cancel"),
		))
}

// renderUpdateCompleteModal renders the completion state.
func (m *Model) renderUpdateCompleteModal() string {
	m.ensureUpdateCompleteModal()
	if m.updateCompleteModal == nil {
		return ""
	}
	if m.updateCompleteMouseHandler == nil {
		m.updateCompleteMouseHandler = mouse.NewHandler()
	}
	return m.updateCompleteModal.Render(m.width, m.height, m.updateCompleteMouseHandler)
}

// clearUpdateCompleteModal clears the complete modal cache.
func (m *Model) clearUpdateCompleteModal() {
	m.updateCompleteModal = nil
	m.updateCompleteModalWidth = 0
	m.updateCompleteMouseHandler = nil
}

// ensureUpdateErrorModal creates/updates the error modal with caching.
func (m *Model) ensureUpdateErrorModal() {
	modalW := m.updateModalWidth()
	if m.updateErrorModal != nil && m.updateErrorModalWidth == modalW {
		return
	}
	m.updateErrorModalWidth = modalW

	errorIcon := lipgloss.NewStyle().Foreground(styles.Error).Render("✗")
	phaseName := m.updatePhase.String()
	errorLine := fmt.Sprintf("  %s Error during: %s", errorIcon, phaseName)

	errorMsg := m.updateError
	if errorMsg == "" {
		errorMsg = "An unknown error occurred."
	}

	m.updateErrorModal = modal.New("Update Failed",
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantDanger),
		modal.WithPrimaryAction("retry"),
	).
		AddSection(modal.Text(errorLine)).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(styles.Muted.Render("  "+errorMsg))).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Retry ", "retry"),
			modal.Btn(" Close ", "cancel"),
		))
}

// renderUpdateErrorModal renders the error state.
func (m *Model) renderUpdateErrorModal() string {
	m.ensureUpdateErrorModal()
	if m.updateErrorModal == nil {
		return ""
	}
	if m.updateErrorMouseHandler == nil {
		m.updateErrorMouseHandler = mouse.NewHandler()
	}
	return m.updateErrorModal.Render(m.width, m.height, m.updateErrorMouseHandler)
}

// clearUpdateErrorModal clears the error modal cache.
func (m *Model) clearUpdateErrorModal() {
	m.updateErrorModal = nil
	m.updateErrorModalWidth = 0
	m.updateErrorMouseHandler = nil
}

// clearAllUpdateModals clears all update modal caches.
func (m *Model) clearAllUpdateModals() {
	m.clearUpdatePreviewModal()
	m.clearUpdateCompleteModal()
	m.clearUpdateErrorModal()
	m.clearChangelogModal()
}

// getChangelogModalWidth returns the width for the changelog modal.
func (m *Model) getChangelogModalWidth() int {
	modalW := m.updateModalWidth() + 10
	maxW := m.width - 4
	if modalW > maxW {
		modalW = maxW
	}
	if modalW < 30 {
		modalW = 30
	}
	return modalW
}

// ensureChangelogModal creates/updates the changelog modal with caching.
func (m *Model) ensureChangelogModal() {
	modalW := m.getChangelogModalWidth()
	if m.changelogModal != nil && m.changelogModalWidth == modalW {
		return
	}
	m.changelogModalWidth = modalW
	contentW := modalW - 6 // borders + padding

	// Calculate max visible lines
	modalMaxHeight := m.height - 6
	if modalMaxHeight < 10 {
		modalMaxHeight = 10
	}
	maxContentLines := modalMaxHeight - 8
	if maxContentLines < 5 {
		maxContentLines = 5
	}

	// Get and render changelog content
	content := m.updateChangelog
	if content == "" {
		content = "Loading changelog..."
	}
	renderedContent := m.renderReleaseNotes(content, contentW)
	lines := strings.Split(renderedContent, "\n")

	// Create a custom section that handles scrolling
	scrollSection := modal.Custom(func(cw int, focusID, hoverID string) modal.RenderedSection {
		// Apply scroll offset
		startLine := m.changelogScrollOffset
		maxStart := len(lines) - maxContentLines
		if maxStart < 0 {
			maxStart = 0
		}
		if startLine > maxStart {
			startLine = maxStart
		}
		if startLine < 0 {
			startLine = 0
		}

		endLine := startLine + maxContentLines
		if endLine > len(lines) {
			endLine = len(lines)
		}

		visibleLines := lines[startLine:endLine]
		visibleContent := strings.Join(visibleLines, "\n")

		// Add scroll indicator if needed
		if len(lines) > maxContentLines {
			scrollInfo := styles.Muted.Render(fmt.Sprintf("Lines %d-%d of %d", startLine+1, endLine, len(lines)))
			visibleContent += "\n\n" + scrollInfo
		}

		return modal.RenderedSection{Content: visibleContent}
	}, nil)

	m.changelogModal = modal.New("Changelog",
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantDefault),
		modal.WithHints(false), // We show custom hints
	).
		AddSection(scrollSection).
		AddSection(modal.Spacer()).
		AddSection(modal.Text(styles.Muted.Render("j/k scroll   Esc: close"))).
		AddSection(modal.Buttons(
			modal.Btn(" Close ", "cancel"),
		))
}

// clearChangelogModal clears the changelog modal cache.
func (m *Model) clearChangelogModal() {
	m.changelogModal = nil
	m.changelogModalWidth = 0
	m.changelogMouseHandler = nil
}

// fetchChangelog fetches the CHANGELOG.md from GitHub.
func fetchChangelog() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(changelogURL)
		if err != nil {
			return ChangelogLoadedMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ChangelogLoadedMsg{Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ChangelogLoadedMsg{Err: err}
		}

		return ChangelogLoadedMsg{Content: string(body)}
	}
}

// renderChangelogOverlay renders the changelog as an overlay on the update preview modal.
func (m *Model) renderChangelogOverlay(background string) string {
	m.ensureChangelogModal()
	if m.changelogModal == nil {
		return background
	}
	if m.changelogMouseHandler == nil {
		m.changelogMouseHandler = mouse.NewHandler()
	}
	modalContent := m.changelogModal.Render(m.width, m.height, m.changelogMouseHandler)
	return ui.OverlayModal(background, modalContent, m.width, m.height)
}
