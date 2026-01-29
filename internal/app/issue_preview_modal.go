package app

import (
	"fmt"
	"strings"

	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

func (m *Model) renderIssueInputOverlay(content string) string {
	m.ensureIssueInputModal()
	if m.issueInputModal == nil {
		return content
	}
	if m.issueInputMouseHandler == nil {
		m.issueInputMouseHandler = mouse.NewHandler()
	}
	rendered := m.issueInputModal.Render(m.width, m.height, m.issueInputMouseHandler)
	return ui.OverlayModal(content, rendered, m.width, m.height)
}

func (m *Model) ensureIssueInputModal() {
	modalW := 60
	if modalW > m.width-4 {
		modalW = m.width - 4
	}
	if modalW < 20 {
		modalW = 20
	}
	if m.issueInputModal != nil && m.issueInputModalWidth == modalW {
		return
	}
	m.issueInputModalWidth = modalW
	b := modal.New("Open Issue",
		modal.WithWidth(modalW),
		modal.WithHints(false),
	).
		AddSection(modal.Input("issue-id", &m.issueInputInput))

	// Show search results dropdown
	if m.issueSearchLoading {
		b = b.AddSection(modal.Text(styles.Muted.Render("Searching...")))
	} else if len(m.issueSearchResults) > 0 {
		searchResults := m.issueSearchResults
		searchCursor := m.issueSearchCursor
		b = b.AddSection(modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
			var sb strings.Builder
			for i, r := range searchResults {
				if i >= 10 {
					break
				}
				line := fmt.Sprintf("  %s  %s", r.ID, r.Title)
				if len(line) > contentWidth-2 {
					line = line[:contentWidth-5] + "..."
				}
				if i == searchCursor {
					sb.WriteString(styles.ListItemSelected.Render(line))
				} else {
					sb.WriteString(styles.ListItemNormal.Render(line))
				}
				if i < len(searchResults)-1 && i < 9 {
					sb.WriteString("\n")
				}
			}
			return modal.RenderedSection{Content: sb.String()}
		}, nil))
	}

	// Hint line
	hasResults := len(m.issueSearchResults) > 0
	b = b.AddSection(modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(styles.KeyHint.Render("enter"))
		sb.WriteString(styles.Muted.Render(" open  "))
		if hasResults {
			sb.WriteString(styles.KeyHint.Render("↑↓"))
			sb.WriteString(styles.Muted.Render(" select  "))
			sb.WriteString(styles.KeyHint.Render("tab"))
			sb.WriteString(styles.Muted.Render(" fill  "))
		}
		sb.WriteString(styles.KeyHint.Render("esc"))
		sb.WriteString(styles.Muted.Render(" cancel"))
		return modal.RenderedSection{Content: sb.String()}
	}, nil))

	m.issueInputModal = b
}

func (m *Model) renderIssuePreviewOverlay(content string) string {
	m.ensureIssuePreviewModal()
	if m.issuePreviewModal == nil {
		return content
	}
	if m.issuePreviewMouseHandler == nil {
		m.issuePreviewMouseHandler = mouse.NewHandler()
	}
	rendered := m.issuePreviewModal.Render(m.width, m.height, m.issuePreviewMouseHandler)
	return ui.OverlayModal(content, rendered, m.width, m.height)
}

func (m *Model) ensureIssuePreviewModal() {
	modalW := 60
	if modalW > m.width-4 {
		modalW = m.width - 4
	}
	if modalW < 20 {
		modalW = 20
	}

	// Cache check -- also invalidate when data/error/loading changes
	cacheKey := modalW
	if m.issuePreviewModal != nil && m.issuePreviewModalWidth == cacheKey {
		return
	}
	m.issuePreviewModalWidth = cacheKey

	if m.issuePreviewLoading {
		m.issuePreviewModal = modal.New("Loading...",
			modal.WithWidth(modalW),
			modal.WithHints(false),
		).
			AddSection(modal.Text("Fetching issue data..."))
		return
	}

	if m.issuePreviewError != nil {
		m.issuePreviewModal = modal.New("Error",
			modal.WithWidth(modalW),
			modal.WithVariant(modal.VariantDanger),
			modal.WithHints(false),
		).
			AddSection(modal.Text(m.issuePreviewError.Error())).
			AddSection(modal.Spacer()).
			AddSection(modal.Buttons(
				modal.Btn(" Close ", "cancel"),
			))
		return
	}

	if m.issuePreviewData == nil {
		m.issuePreviewModal = nil
		return
	}

	data := m.issuePreviewData

	// Build title
	title := data.ID
	if data.Title != "" {
		title += ": " + data.Title
	}

	// Build status line
	var metaParts []string
	if data.Status != "" {
		metaParts = append(metaParts, "["+data.Status+"]")
	}
	if data.Type != "" {
		metaParts = append(metaParts, data.Type)
	}
	if data.Priority != "" {
		metaParts = append(metaParts, data.Priority)
	}
	if data.Points > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%dp", data.Points))
	}
	statusLine := strings.Join(metaParts, "  ")

	// Build modal
	b := modal.New(title,
		modal.WithWidth(modalW),
		modal.WithHints(false),
	)

	if statusLine != "" {
		b = b.AddSection(modal.Text(statusLine))
	}

	if data.ParentID != "" {
		b = b.AddSection(modal.Text("Parent: " + data.ParentID))
	}

	if len(data.Labels) > 0 {
		b = b.AddSection(modal.Text("Labels: " + strings.Join(data.Labels, ", ")))
	}

	// Description snippet (first ~10 lines)
	if data.Description != "" {
		b = b.AddSection(modal.Spacer())
		desc := data.Description
		lines := strings.Split(desc, "\n")
		if len(lines) > 10 {
			lines = lines[:10]
			lines = append(lines, "...")
		}
		b = b.AddSection(modal.Text(strings.Join(lines, "\n")))
	}

	b = b.AddSection(modal.Spacer())
	b = b.AddSection(modal.Buttons(
		modal.Btn(" Open in TD ", "open-in-td", modal.BtnPrimary()),
		modal.Btn(" Close ", "cancel"),
	))

	// Hint line
	b = b.AddSection(modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(styles.KeyHint.Render("o"))
		sb.WriteString(styles.Muted.Render(" open  "))
		sb.WriteString(styles.KeyHint.Render("esc"))
		sb.WriteString(styles.Muted.Render(" close"))
		return modal.RenderedSection{Content: sb.String()}
	}, nil))

	m.issuePreviewModal = b
}
