package gitstatus

import (
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/msg"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/ui"
)

// showErrorModal sets up the error modal state and switches to the error view.
func (p *Plugin) showErrorModal(title string, err error) {
	var detail string
	switch e := err.(type) {
	case *PushError:
		detail = e.Output
	case *RemoteError:
		detail = e.Output
	default:
		detail = err.Error()
	}
	p.errorTitle = title
	p.errorDetail = detail
	p.clearErrorModal()
	p.viewMode = ViewModeError
}

// ensureErrorModal builds or rebuilds the error modal when needed.
func (p *Plugin) ensureErrorModal() {
	if p.errorDetail == "" {
		return
	}
	modalW := ui.ModalWidthLarge
	if modalW > p.width-4 {
		modalW = p.width - 4
	}
	if modalW < 30 {
		modalW = 30
	}
	if p.errorModal != nil && p.errorModalWidth == modalW && p.errorModalHeight == p.height {
		return
	}
	p.errorModalWidth = modalW
	p.errorModalHeight = p.height
	p.errorModal = modal.New(p.errorTitle,
		modal.WithWidth(modalW),
		modal.WithVariant(modal.VariantDanger),
	).
		AddSection(modal.Text(p.errorDetail)).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Dismiss ", "dismiss"),
		))
}

// renderErrorModal renders the error modal overlaid on the status view.
func (p *Plugin) renderErrorModal() string {
	background := p.renderThreePaneView()

	p.ensureErrorModal()
	if p.errorModal == nil {
		return background
	}

	modalContent := p.errorModal.Render(p.width, p.height, p.mouseHandler)
	return ui.OverlayModal(background, modalContent, p.width, p.height)
}

// updateErrorModal handles keyboard input for the error modal.
func (p *Plugin) updateErrorModal(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	p.ensureErrorModal()
	if p.errorModal == nil {
		return p, nil
	}

	// Intercept yank before delegating to modal key handler
	if msg.String() == "y" {
		return p, p.yankErrorToClipboard()
	}

	action, cmd := p.errorModal.HandleKey(msg)
	if action == "dismiss" || action == "cancel" {
		return p.dismissErrorModal()
	}
	return p, cmd
}

// handleErrorModalMouse handles mouse input for the error modal.
func (p *Plugin) handleErrorModalMouse(m tea.MouseMsg) (plugin.Plugin, tea.Cmd) {
	if p.errorModal == nil {
		return p, nil
	}

	action := p.errorModal.HandleMouse(m, p.mouseHandler)
	if action == "dismiss" || action == "cancel" {
		return p.dismissErrorModal()
	}
	return p, nil
}

// dismissErrorModal closes the error modal and clears error state.
func (p *Plugin) dismissErrorModal() (plugin.Plugin, tea.Cmd) {
	p.viewMode = ViewModeStatus
	p.errorTitle = ""
	p.errorDetail = ""
	p.errorModal = nil
	p.errorModalWidth = 0
	p.errorModalHeight = 0
	p.pushError = ""
	p.fetchError = ""
	p.pullError = ""
	return p, nil
}

// yankErrorToClipboard copies the error detail text to the system clipboard.
func (p *Plugin) yankErrorToClipboard() tea.Cmd {
	if p.errorDetail == "" {
		return nil
	}
	if err := clipboard.WriteAll(p.errorDetail); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Yanked error output", 2*time.Second)
}
