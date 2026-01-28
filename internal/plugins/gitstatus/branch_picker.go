package gitstatus

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

const (
	branchPickerItemPrefix = "branch-picker-item-"
)

func branchPickerItemID(idx int) string {
	return fmt.Sprintf("%s%d", branchPickerItemPrefix, idx)
}

func parseBranchPickerItem(id string) (int, bool) {
	if !strings.HasPrefix(id, branchPickerItemPrefix) {
		return 0, false
	}
	idx, err := strconv.Atoi(strings.TrimPrefix(id, branchPickerItemPrefix))
	if err != nil {
		return 0, false
	}
	return idx, true
}

// updateBranchPicker handles key events in the branch picker modal.
func (p *Plugin) updateBranchPicker(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	p.ensureBranchPickerModal()
	if p.branchPickerModal == nil {
		return p, nil
	}

	switch msg.String() {
	case "esc", "q":
		// Close picker
		p.closeBranchPicker()
		return p, nil

	case "j", "down":
		p.moveBranchCursor(1)
		return p, nil

	case "k", "up":
		p.moveBranchCursor(-1)
		return p, nil

	case "g":
		p.branchCursor = 0
		return p, nil

	case "G":
		if len(p.branches) > 0 {
			p.branchCursor = len(p.branches) - 1
		}
		return p, nil

	case "enter":
		// Switch to selected branch
		return p, p.switchSelectedBranch()
	}

	action, cmd := p.branchPickerModal.HandleKey(msg)
	if action == "cancel" {
		p.closeBranchPicker()
		return p, nil
	}
	if idx, ok := parseBranchPickerItem(action); ok {
		return p, p.switchBranchByIndex(idx)
	}

	return p, cmd
}

// doSwitchBranch switches to a different branch.
func (p *Plugin) doSwitchBranch(branchName string) tea.Cmd {
	workDir := p.repoRoot
	return func() tea.Msg {
		err := CheckoutBranch(workDir, branchName)
		if err != nil {
			return BranchErrorMsg{Err: err}
		}
		return BranchSwitchSuccessMsg{Branch: branchName}
	}
}

// loadBranches loads the branch list.
func (p *Plugin) loadBranches() tea.Cmd {
	epoch := p.ctx.Epoch
	workDir := p.repoRoot
	return func() tea.Msg {
		branches, err := GetBranches(workDir)
		if err != nil {
			return BranchErrorMsg{Err: err}
		}
		return BranchListLoadedMsg{Epoch: epoch, Branches: branches}
	}
}

// ensureBranchPickerModal builds/rebuilds the branch picker modal.
func (p *Plugin) ensureBranchPickerModal() {
	modalW := p.branchPickerWidthForContent()
	if p.branchPickerModal != nil && p.branchPickerWidth == modalW {
		return
	}
	p.branchPickerWidth = modalW

	p.branchPickerModal = modal.New("Branches",
		modal.WithWidth(modalW),
		modal.WithHints(false),
	).
		AddSection(p.branchPickerListSection()).
		AddSection(modal.Spacer()).
		AddSection(p.branchPickerHintsSection())
}

func (p *Plugin) branchPickerWidthForContent() int {
	modalW := 50
	for _, b := range p.branches {
		lineLen := len(b.Name) + len(b.FormatTrackingInfo()) + len(b.Upstream) + 10
		if lineLen > modalW {
			modalW = lineLen
		}
	}
	if modalW > p.width-10 {
		modalW = p.width - 10
	}
	if modalW < 20 {
		modalW = 20
	}
	return modalW
}

func (p *Plugin) branchPickerListSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		if len(p.branches) == 0 {
			return modal.RenderedSection{Content: styles.Muted.Render("  Loading branches...")}
		}

		maxVisible := p.branchPickerMaxVisible()
		start := 0
		if p.branchCursor >= maxVisible {
			start = p.branchCursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(p.branches) {
			end = len(p.branches)
		}

		var sb strings.Builder
		focusables := make([]modal.FocusableInfo, 0, end-start)

		for i := start; i < end; i++ {
			branch := p.branches[i]
			itemID := branchPickerItemID(i)
			selected := i == p.branchCursor
			hovered := itemID == hoverID

			line := p.renderBranchLine(branch, selected, hovered)
			if i > start {
				sb.WriteString("\n")
			}
			sb.WriteString(line)

			focusables = append(focusables, modal.FocusableInfo{
				ID:      itemID,
				OffsetX: 0,
				OffsetY: i - start,
				Width:   ansi.StringWidth(line),
				Height:  1,
			})
		}

		content := sb.String()
		if len(p.branches) > maxVisible {
			content += "\n\n" + styles.Muted.Render(fmt.Sprintf("  %d/%d branches", p.branchCursor+1, len(p.branches)))
		}

		return modal.RenderedSection{
			Content:    content,
			Focusables: focusables,
		}
	}, p.branchPickerListUpdate)
}

func (p *Plugin) branchPickerListUpdate(msg tea.Msg, focusID string) (string, tea.Cmd) {
	if _, ok := parseBranchPickerItem(focusID); !ok {
		return "", nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return "", nil
	}

	switch keyMsg.String() {
	case "up", "k":
		p.moveBranchCursor(-1)
	case "down", "j":
		p.moveBranchCursor(1)
	case "enter":
		if len(p.branches) > 0 && p.branchCursor >= 0 && p.branchCursor < len(p.branches) {
			return branchPickerItemID(p.branchCursor), nil
		}
	}

	return "", nil
}

func (p *Plugin) branchPickerHintsSection() modal.Section {
	return modal.Custom(func(contentWidth int, focusID, hoverID string) modal.RenderedSection {
		return modal.RenderedSection{Content: styles.Muted.Render("  Enter to switch, j/k to navigate, Esc to cancel")}
	}, nil)
}

func (p *Plugin) branchPickerMaxVisible() int {
	maxVisible := 15
	if p.height-10 < maxVisible {
		maxVisible = p.height - 10
	}
	if maxVisible < 5 {
		maxVisible = 5
	}
	return maxVisible
}

func (p *Plugin) moveBranchCursor(delta int) {
	if len(p.branches) == 0 {
		return
	}
	newCursor := p.branchCursor + delta
	if newCursor < 0 {
		newCursor = 0
	}
	if newCursor >= len(p.branches) {
		newCursor = len(p.branches) - 1
	}
	p.branchCursor = newCursor
}

func (p *Plugin) switchSelectedBranch() tea.Cmd {
	return p.switchBranchByIndex(p.branchCursor)
}

func (p *Plugin) switchBranchByIndex(idx int) tea.Cmd {
	if idx < 0 || idx >= len(p.branches) {
		return nil
	}
	p.branchCursor = idx
	branch := p.branches[idx]
	if branch.IsCurrent {
		return nil
	}
	return p.doSwitchBranch(branch.Name)
}

func (p *Plugin) closeBranchPicker() {
	p.viewMode = p.branchReturnMode
	p.branches = nil
	p.clearBranchPickerModal()
}

func (p *Plugin) clearBranchPickerModal() {
	p.branchPickerModal = nil
	p.branchPickerWidth = 0
}

// renderBranchPicker renders the branch picker modal.
func (p *Plugin) renderBranchPicker() string {
	// Render the background (status view dimmed)
	background := p.renderThreePaneView()

	p.ensureBranchPickerModal()
	if p.branchPickerModal == nil {
		return background
	}

	modalContent := p.branchPickerModal.Render(p.width, p.height, p.mouseHandler)
	return ui.OverlayModal(background, modalContent, p.width, p.height)
}

// renderBranchLine renders a single branch line.
func (p *Plugin) renderBranchLine(branch *Branch, selected, hovered bool) string {
	// Current branch indicator
	indicator := "  "
	if branch.IsCurrent {
		indicator = "* "
	}

	// Branch name
	name := branch.Name

	// Tracking info
	trackingInfo := branch.FormatTrackingInfo()
	trackingInfoPlain := trackingInfo
	if trackingInfo != "" {
		trackingInfo = " " + styles.StatusModified.Render(trackingInfo)
	}

	// Upstream indicator
	upstream := ""
	if branch.Upstream != "" {
		upstream = styles.Muted.Render(" → " + branch.Upstream)
	}

	// Build plain line for selected/hovered states (need consistent width)
	buildPlainLine := func() string {
		line := fmt.Sprintf("%s%s", indicator, name)
		if trackingInfoPlain != "" {
			line += " " + trackingInfoPlain
		}
		maxWidth := 45
		if len(line) < maxWidth {
			line += strings.Repeat(" ", maxWidth-len(line))
		}
		return line
	}

	if selected {
		return styles.ListItemSelected.Render(buildPlainLine())
	}

	if hovered {
		// Use a hover style - slightly highlighted background
		return styles.ListItemSelected.Render(buildPlainLine())
	}

	// Style based on current branch
	nameStyle := styles.Body
	if branch.IsCurrent {
		nameStyle = styles.StatusStaged
	}

	return styles.ListItemNormal.Render(fmt.Sprintf("%s%s%s%s", indicator, nameStyle.Render(name), trackingInfo, upstream))
}
