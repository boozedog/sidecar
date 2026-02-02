package workspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

// ensureFetchPRModal builds/rebuilds the fetch PR modal when needed.
func (p *Plugin) ensureFetchPRModal() {
	modalW := 70
	maxW := p.width - 4
	if maxW < 1 {
		maxW = 1
	}
	if modalW > maxW {
		modalW = maxW
	}

	if p.fetchPRModal != nil && p.fetchPRModalWidth == modalW {
		return
	}
	p.fetchPRModalWidth = modalW

	p.fetchPRModal = modal.New("Fetch PR",
		modal.WithWidth(modalW),
		modal.WithHints(false),
	).
		AddSection(p.fetchPRContentSection())
}

// clearFetchPRModal invalidates the cached modal so it rebuilds next frame.
func (p *Plugin) clearFetchPRModal() {
	p.fetchPRModal = nil
	p.fetchPRModalWidth = 0
}

// fetchPRContentSection returns a custom section that renders the PR list content.
func (p *Plugin) fetchPRContentSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		var lines []string

		if p.fetchPRLoading {
			lines = append(lines, dimText("Loading PRs..."))
			return modal.RenderedSection{Content: strings.Join(lines, "\n")}
		}

		if p.fetchPRError != "" {
			errStyle := lipgloss.NewStyle().Foreground(styles.Error)
			lines = append(lines, errStyle.Render(p.fetchPRError))
			return modal.RenderedSection{Content: strings.Join(lines, "\n")}
		}

		// Filter field
		lines = append(lines, "Filter:")
		filterStyle := inputFocusedStyle()
		inputW := contentWidth - 4
		if inputW < 20 {
			inputW = 20
		}
		filterStyle = filterStyle.Width(inputW)
		filterDisplay := p.fetchPRFilter
		if filterDisplay == "" {
			filterDisplay = lipgloss.NewStyle().Foreground(styles.Muted.GetForeground()).Render("type to filter...")
		}
		lines = append(lines, filterStyle.Render(filterDisplay))

		// PR list
		filtered := p.filteredFetchPRItems()
		if len(filtered) == 0 && len(p.fetchPRItems) == 0 {
			lines = append(lines, "")
			lines = append(lines, dimText("No open PRs found"))
		} else if len(filtered) == 0 {
			lines = append(lines, "")
			lines = append(lines, dimText("No matching PRs"))
		} else {
			maxVisible := 10

			// Scroll offset is adjusted in Update (handleFetchPRKeys);
			// here we just clamp to valid range for rendering.
			offset := p.fetchPRScrollOffset
			if offset < 0 {
				offset = 0
			}
			if offset > len(filtered)-maxVisible && len(filtered) > maxVisible {
				offset = len(filtered) - maxVisible
			}

			endIdx := offset + maxVisible
			if endIdx > len(filtered) {
				endIdx = len(filtered)
			}

			for i := offset; i < endIdx; i++ {
				pr := filtered[i]
				prefix := "  "
				if i == p.fetchPRCursor {
					prefix = "> "
				}

				// Format: #42  fix-auth-flow  @claude  2h ago  [draft]
				num := fmt.Sprintf("#%d", pr.Number)
				branch := pr.Branch
				author := "@" + pr.Author.Login
				age := formatPRAge(pr.CreatedAt)

				// Truncate branch if needed
				maxBranch := contentWidth - len(num) - len(author) - len(age) - 14
				if maxBranch < 10 {
					maxBranch = 10
				}
				if len(branch) > maxBranch {
					branch = branch[:maxBranch-3] + "..."
				}

				line := fmt.Sprintf("%s%-5s %-*s  %s  %s", prefix, num, maxBranch, branch, author, age)
				if pr.IsDraft {
					line += "  [draft]"
				}

				if i == p.fetchPRCursor {
					lines = append(lines, lipgloss.NewStyle().Foreground(styles.Primary).Render(line))
				} else {
					lines = append(lines, dimText(line))
				}
			}

			// Show scroll indicators
			if offset > 0 {
				lines = append(lines, dimText(fmt.Sprintf("  ... %d more above", offset)))
			}
			remaining := len(filtered) - endIdx
			if remaining > 0 {
				lines = append(lines, dimText(fmt.Sprintf("  ... %d more below", remaining)))
			}

			// Show title of selected PR below the list
			if p.fetchPRCursor >= 0 && p.fetchPRCursor < len(filtered) {
				lines = append(lines, "")
				title := filtered[p.fetchPRCursor].Title
				maxTitle := contentWidth - 2
				if maxTitle < 4 {
					maxTitle = 4
				}
				if len(title) > maxTitle && maxTitle >= 4 {
					title = title[:maxTitle-3] + "..."
				}
				lines = append(lines, dimText("  "+title))
			}
		}

		return modal.RenderedSection{Content: strings.Join(lines, "\n")}
	}, nil)
}

// renderFetchPRModal renders the fetch PR modal with dimmed background.
func (p *Plugin) renderFetchPRModal(width, height int) string {
	background := p.renderListView(width, height)

	p.ensureFetchPRModal()
	if p.fetchPRModal == nil {
		return background
	}

	modalContent := p.fetchPRModal.Render(width, height, p.mouseHandler)
	return ui.OverlayModal(background, modalContent, width, height)
}

// formatPRAge parses a GitHub timestamp and returns relative age.
func formatPRAge(createdAt string) string {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return ""
	}
	return formatRelativeTime(t)
}
