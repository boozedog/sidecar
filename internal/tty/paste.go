package tty

import (
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

// IsPasteInput detects if the input is a paste operation.
// Returns true if the input contains newlines or is longer than a typical typed sequence.
func IsPasteInput(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes {
		return false
	}
	if msg.Paste {
		return true
	}
	if len(msg.Runes) <= 1 {
		return false
	}
	text := string(msg.Runes)
	// Treat as paste if contains newline or is suspiciously long for typing
	return strings.Contains(text, "\n") || len(msg.Runes) > 10
}

// SendPasteToTmux pastes multi-line text via tmux buffer.
// Uses load-buffer + paste-buffer which works regardless of app paste mode state.
func SendPasteToTmux(sessionName, text string) error {
	// Load text into tmux default buffer via stdin
	loadCmd := exec.Command("tmux", "load-buffer", "-")
	loadCmd.Stdin = strings.NewReader(text)
	if err := loadCmd.Run(); err != nil {
		return err
	}

	// Paste buffer into target pane
	pasteCmd := exec.Command("tmux", "paste-buffer", "-t", sessionName)
	return pasteCmd.Run()
}

// SendBracketedPasteToTmux sends text wrapped in bracketed paste sequences.
// Used when the target app has enabled bracketed paste mode.
func SendBracketedPasteToTmux(sessionName, text string) error {
	// Send bracketed paste start sequence
	if err := SendLiteralToTmux(sessionName, BracketedPasteStart); err != nil {
		return err
	}

	// Send the actual text
	if err := SendLiteralToTmux(sessionName, text); err != nil {
		return err
	}

	// Send bracketed paste end sequence
	return SendLiteralToTmux(sessionName, BracketedPasteEnd)
}

// PasteClipboardToTmuxCmd returns a tea.Cmd that pastes clipboard content to a tmux session.
// The bracketed parameter determines whether to use bracketed paste mode.
// Returns a PasteResultMsg with the result.
func PasteClipboardToTmuxCmd(sessionName string, bracketed bool) tea.Cmd {
	return func() tea.Msg {
		text, err := clipboard.ReadAll()
		if err != nil {
			return PasteResultMsg{Err: err}
		}
		if text == "" {
			return PasteResultMsg{Empty: true}
		}

		if bracketed {
			err = SendBracketedPasteToTmux(sessionName, text)
		} else {
			err = SendPasteToTmux(sessionName, text)
		}
		if err != nil {
			return PasteResultMsg{Err: err, SessionDead: IsSessionDeadError(err)}
		}

		return PasteResultMsg{}
	}
}

// SendPasteInputCmd sends paste text to tmux asynchronously.
// Used for multi-character terminal input (not clipboard paste which is already async).
func SendPasteInputCmd(sessionName, text string, bracketed bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if bracketed {
			err = SendBracketedPasteToTmux(sessionName, text)
		} else {
			err = SendPasteToTmux(sessionName, text)
		}
		if err != nil && IsSessionDeadError(err) {
			return SessionDeadMsg{}
		}
		return nil
	}
}
