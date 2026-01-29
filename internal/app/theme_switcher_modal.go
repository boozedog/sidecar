package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

const (
	themeSwitcherFilterID   = "theme-switcher-filter"
	themeSwitcherItemPrefix = "theme-switcher-item-"
)

// themeSwitcherItemID returns the ID for a theme item at the given index.
func themeSwitcherItemID(idx int) string {
	return fmt.Sprintf("%s%d", themeSwitcherItemPrefix, idx)
}

// ensureThemeSwitcherModal builds/rebuilds the theme switcher modal.
func (m *Model) ensureThemeSwitcherModal() {
	modalW := 50
	if modalW > m.width-4 {
		modalW = m.width - 4
	}
	if modalW < 30 {
		modalW = 30
	}

	// Only rebuild if modal doesn't exist or width changed
	if m.themeSwitcherModal != nil && m.themeSwitcherModalWidth == modalW {
		return
	}
	m.themeSwitcherModalWidth = modalW

	m.themeSwitcherModal = modal.New("Switch Theme",
		modal.WithWidth(modalW),
		modal.WithHints(false),
	).
		AddSection(modal.When(m.themeSwitcherHasProject, m.themeSwitcherScopeSection())).
		AddSection(modal.Input(themeSwitcherFilterID, &m.themeSwitcherInput, modal.WithSubmitOnEnter(false))).
		AddSection(m.themeSwitcherCountSection()).
		AddSection(modal.Spacer()).
		AddSection(m.themeSwitcherListSection()).
		AddSection(modal.Spacer()).
		AddSection(m.themeSwitcherHintsSection())
}

// themeSwitcherHasProject returns true if the current project is in the project list.
func (m *Model) themeSwitcherHasProject() bool {
	return m.currentProjectConfig() != nil
}

// themeSwitcherCountSection renders the theme count/community info.
func (m *Model) themeSwitcherCountSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		allThemes := styles.ListThemes()
		themes := m.themeSwitcherFiltered

		var text string
		if m.themeSwitcherInput.Value() != "" {
			text = fmt.Sprintf("%d of %d themes", len(themes), len(allThemes))
		} else if m.themeSwitcherCommunityName != "" {
			text = fmt.Sprintf("Community theme: %s", m.themeSwitcherCommunityName)
		}

		if text == "" {
			return modal.RenderedSection{Content: " "}
		}
		return modal.RenderedSection{Content: styles.Muted.Render(text)}
	}, nil)
}

// themeSwitcherListSection renders the theme list with selection.
func (m *Model) themeSwitcherListSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		themes := m.themeSwitcherFiltered

		if len(themes) == 0 {
			return modal.RenderedSection{Content: styles.Muted.Render("No matches")}
		}

		// Styles for theme items
		cursorStyle := lipgloss.NewStyle().Foreground(styles.Primary)
		nameNormalStyle := lipgloss.NewStyle().Foreground(styles.Secondary)
		nameSelectedStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
		nameCurrentStyle := lipgloss.NewStyle().Foreground(styles.Success).Bold(true)
		nameCurrentSelectedStyle := lipgloss.NewStyle().Foreground(styles.Success).Bold(true)

		currentTheme := m.themeSwitcherOriginal
		if m.themeSwitcherCommunityName != "" {
			currentTheme = ""
		}

		// Calculate visible window for scrolling
		maxVisible := 8
		visibleCount := min(maxVisible, len(themes))

		// Ensure selected index is valid
		selectedIdx := m.themeSwitcherSelectedIdx
		if selectedIdx < 0 {
			selectedIdx = 0
		}
		if selectedIdx >= len(themes) {
			selectedIdx = len(themes) - 1
		}

		// Calculate scroll offset to keep selection visible
		scrollOffset := 0
		if selectedIdx >= maxVisible {
			scrollOffset = selectedIdx - maxVisible + 1
		}
		if scrollOffset > len(themes)-visibleCount {
			scrollOffset = len(themes) - visibleCount
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}

		var sb strings.Builder
		focusables := make([]modal.FocusableInfo, 0, visibleCount)
		lineOffset := 0

		// Scroll indicator (top)
		if scrollOffset > 0 {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("  ↑ %d more above", scrollOffset)))
			sb.WriteString("\n")
			lineOffset++
		}

		// Render theme list
		for i := scrollOffset; i < scrollOffset+visibleCount && i < len(themes); i++ {
			themeName := themes[i]
			isSelected := i == selectedIdx
			itemID := themeSwitcherItemID(i)
			isHovered := itemID == hoverID
			isCurrent := themeName == currentTheme

			// Cursor indicator
			if isSelected {
				sb.WriteString(cursorStyle.Render("> "))
			} else {
				sb.WriteString("  ")
			}

			// Theme name styling
			var nameStyle lipgloss.Style
			if isCurrent {
				if isSelected || isHovered {
					nameStyle = nameCurrentSelectedStyle
				} else {
					nameStyle = nameCurrentStyle
				}
			} else if isSelected || isHovered {
				nameStyle = nameSelectedStyle
			} else {
				nameStyle = nameNormalStyle
			}

			// Get display name from theme
			theme := styles.GetTheme(themeName)
			displayName := theme.DisplayName
			if displayName == "" {
				displayName = themeName
			}
			sb.WriteString(nameStyle.Render(displayName))

			// Current indicator
			if isCurrent {
				sb.WriteString(styles.Muted.Render(" (current)"))
			}
			sb.WriteString("\n")

			focusables = append(focusables, modal.FocusableInfo{
				ID:      itemID,
				OffsetX: 0,
				OffsetY: lineOffset + (i - scrollOffset),
				Width:   contentWidth,
				Height:  1,
			})
		}

		// Scroll indicator (bottom)
		remaining := len(themes) - (scrollOffset + visibleCount)
		if remaining > 0 {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
		}

		return modal.RenderedSection{Content: strings.TrimRight(sb.String(), "\n"), Focusables: focusables}
	}, m.themeSwitcherListUpdate)
}

// themeSwitcherListUpdate handles key events for the theme list.
func (m *Model) themeSwitcherListUpdate(msg tea.Msg, focusID string) (string, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return "", nil
	}

	themes := m.themeSwitcherFiltered
	if len(themes) == 0 {
		return "", nil
	}

	switch keyMsg.String() {
	case "up", "k", "ctrl+p":
		if m.themeSwitcherSelectedIdx > 0 {
			m.themeSwitcherSelectedIdx--
			m.themeSwitcherModalWidth = 0 // Force modal rebuild for scroll
			// Live preview
			if m.themeSwitcherSelectedIdx < len(themes) {
				m.applyThemeFromConfig(themes[m.themeSwitcherSelectedIdx])
			}
		}
		return "", nil

	case "down", "j", "ctrl+n":
		if m.themeSwitcherSelectedIdx < len(themes)-1 {
			m.themeSwitcherSelectedIdx++
			m.themeSwitcherModalWidth = 0 // Force modal rebuild for scroll
			// Live preview
			if m.themeSwitcherSelectedIdx < len(themes) {
				m.applyThemeFromConfig(themes[m.themeSwitcherSelectedIdx])
			}
		}
		return "", nil

	case "enter":
		if m.themeSwitcherSelectedIdx >= 0 && m.themeSwitcherSelectedIdx < len(themes) {
			return "select", nil
		}
		return "", nil
	}

	return "", nil
}

// themeSwitcherScopeSection renders the scope selector.
func (m *Model) themeSwitcherScopeSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		var sb strings.Builder

		activeStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)

		scopeGlobal := "Global"
		scopeProject := "This project"
		if m.themeSwitcherScope == "project" {
			sb.WriteString(styles.Muted.Render(scopeGlobal))
			sb.WriteString(styles.Muted.Render("  "))
			sb.WriteString(activeStyle.Render(scopeProject))
		} else {
			sb.WriteString(activeStyle.Render(scopeGlobal))
			sb.WriteString(styles.Muted.Render("  "))
			sb.WriteString(styles.Muted.Render(scopeProject))
		}
		sb.WriteString(styles.Muted.Render("    "))
		sb.WriteString(styles.KeyHint.Render("←/→"))
		sb.WriteString(styles.Muted.Render(" switch"))

		return modal.RenderedSection{Content: sb.String()}
	}, nil)
}

// themeSwitcherHintsSection renders the help text.
func (m *Model) themeSwitcherHintsSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		var sb strings.Builder

		if len(m.themeSwitcherFiltered) == 0 {
			sb.WriteString(styles.KeyHint.Render("esc"))
			sb.WriteString(styles.Muted.Render(" clear filter  "))
			sb.WriteString(styles.KeyHint.Render("#"))
			sb.WriteString(styles.Muted.Render(" close"))
		} else {
			sb.WriteString(styles.KeyHint.Render("enter"))
			sb.WriteString(styles.Muted.Render(" select  "))
			sb.WriteString(styles.KeyHint.Render("↑/↓"))
			sb.WriteString(styles.Muted.Render(" navigate  "))
			sb.WriteString(styles.KeyHint.Render("tab"))
			sb.WriteString(styles.Muted.Render(" community  "))
			sb.WriteString(styles.KeyHint.Render("esc"))
			sb.WriteString(styles.Muted.Render(" cancel"))
		}

		return modal.RenderedSection{Content: sb.String()}
	}, nil)
}

// renderThemeSwitcherModal renders the theme switcher modal using the modal library.
func (m *Model) renderThemeSwitcherModal(content string) string {
	if m.showCommunityBrowser {
		return m.renderCommunityBrowserOverlay(content)
	}

	m.ensureThemeSwitcherModal()
	if m.themeSwitcherModal == nil {
		return content
	}

	if m.themeSwitcherMouseHandler == nil {
		m.themeSwitcherMouseHandler = mouse.NewHandler()
	}
	modalContent := m.themeSwitcherModal.Render(m.width, m.height, m.themeSwitcherMouseHandler)
	return ui.OverlayModal(content, modalContent, m.width, m.height)
}
