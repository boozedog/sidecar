package plugin

import tea "github.com/charmbracelet/bubbletea"

// Plugin defines the interface for all sidecar plugins.
type Plugin interface {
	ID() string
	Name() string
	Icon() string
	Init(ctx *Context) error
	Start() tea.Cmd
	Stop()
	Update(msg tea.Msg) (Plugin, tea.Cmd)
	View(width, height int) string
	IsFocused() bool
	SetFocused(bool)
	Commands() []Command
	FocusContext() string
}

// Command represents a keybinding command exposed by a plugin.
type Command struct {
	ID      string
	Name    string
	Handler func() tea.Cmd
	Context string
}

// DiagnosticProvider is implemented by plugins that expose diagnostics.
type DiagnosticProvider interface {
	Diagnostics() []Diagnostic
}

// Diagnostic represents a health/status check result.
type Diagnostic struct {
	ID     string
	Status string
	Detail string
}
