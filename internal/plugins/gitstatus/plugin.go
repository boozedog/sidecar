package gitstatus

import (
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sst/sidecar/internal/plugin"
)

const (
	pluginID   = "git-status"
	pluginName = "Git Status"
	pluginIcon = "G"
)

// Plugin implements the git status plugin.
type Plugin struct {
	ctx       *plugin.Context
	tree      *FileTree
	focused   bool
	cursor    int
	scrollOff int

	// Diff modal state
	showDiff    bool
	diffContent string
	diffFile    string
	diffScroll  int

	// View dimensions
	width  int
	height int

	// Watcher
	watcher *Watcher
}

// New creates a new git status plugin.
func New() *Plugin {
	return &Plugin{}
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return pluginID }

// Name returns the plugin display name.
func (p *Plugin) Name() string { return pluginName }

// Icon returns the plugin icon character.
func (p *Plugin) Icon() string { return pluginIcon }

// Init initializes the plugin with context.
func (p *Plugin) Init(ctx *plugin.Context) error {
	// Check if we're in a git repository
	gitDir := filepath.Join(ctx.WorkDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return err // Not a git repo, silently degrade
	}

	p.ctx = ctx
	p.tree = NewFileTree(ctx.WorkDir)

	return nil
}

// Start begins plugin operation.
func (p *Plugin) Start() tea.Cmd {
	return tea.Batch(
		p.refresh(),
		p.startWatcher(),
	)
}

// Stop cleans up plugin resources.
func (p *Plugin) Stop() {
	if p.watcher != nil {
		p.watcher.Stop()
	}
}

// Update handles messages.
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.showDiff {
			return p.updateDiffModal(msg)
		}
		return p.updateMain(msg)

	case RefreshMsg:
		return p, p.refresh()

	case WatchEventMsg:
		return p, p.refresh()

	case DiffLoadedMsg:
		p.diffContent = msg.Content
		return p, nil

	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
	}

	return p, nil
}

// updateMain handles key events in the main view.
func (p *Plugin) updateMain(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	entries := p.tree.AllEntries()

	switch msg.String() {
	case "j", "down":
		if p.cursor < len(entries)-1 {
			p.cursor++
			p.ensureCursorVisible()
		}

	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
		}

	case "g":
		p.cursor = 0
		p.scrollOff = 0

	case "G":
		if len(entries) > 0 {
			p.cursor = len(entries) - 1
			p.ensureCursorVisible()
		}

	case "s":
		if len(entries) > 0 && p.cursor < len(entries) {
			entry := entries[p.cursor]
			if !entry.Staged {
				if err := p.tree.StageFile(entry.Path); err == nil {
					return p, p.refresh()
				}
			}
		}

	case "u":
		if len(entries) > 0 && p.cursor < len(entries) {
			entry := entries[p.cursor]
			if entry.Staged {
				if err := p.tree.UnstageFile(entry.Path); err == nil {
					return p, p.refresh()
				}
			}
		}

	case "d":
		if len(entries) > 0 && p.cursor < len(entries) {
			entry := entries[p.cursor]
			p.showDiff = true
			p.diffFile = entry.Path
			p.diffScroll = 0
			return p, p.loadDiff(entry.Path, entry.Staged)
		}

	case "enter":
		if len(entries) > 0 && p.cursor < len(entries) {
			entry := entries[p.cursor]
			return p, p.openFile(entry.Path)
		}

	case "r":
		return p, p.refresh()
	}

	return p, nil
}

// updateDiffModal handles key events in the diff modal.
func (p *Plugin) updateDiffModal(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		p.showDiff = false
		p.diffContent = ""
		p.diffFile = ""

	case "j", "down":
		p.diffScroll++

	case "k", "up":
		if p.diffScroll > 0 {
			p.diffScroll--
		}

	case "g":
		p.diffScroll = 0

	case "G":
		lines := countLines(p.diffContent)
		maxScroll := lines - (p.height - 2)
		if maxScroll > 0 {
			p.diffScroll = maxScroll
		}
	}

	return p, nil
}

// View renders the plugin.
func (p *Plugin) View(width, height int) string {
	p.width = width
	p.height = height

	if p.showDiff {
		return p.renderDiffModal()
	}

	return p.renderMain()
}

// IsFocused returns whether the plugin is focused.
func (p *Plugin) IsFocused() bool { return p.focused }

// SetFocused sets the focus state.
func (p *Plugin) SetFocused(f bool) { p.focused = f }

// Commands returns the available commands.
func (p *Plugin) Commands() []plugin.Command {
	return []plugin.Command{
		{ID: "stage-file", Name: "Stage file", Context: "git-status"},
		{ID: "unstage-file", Name: "Unstage file", Context: "git-status"},
		{ID: "show-diff", Name: "Show diff", Context: "git-status"},
		{ID: "open-file", Name: "Open file", Context: "git-status"},
		{ID: "close-diff", Name: "Close diff", Context: "git-diff"},
		{ID: "scroll", Name: "Scroll", Context: "git-diff"},
	}
}

// FocusContext returns the current focus context.
func (p *Plugin) FocusContext() string {
	if p.showDiff {
		return "git-diff"
	}
	return "git-status"
}

// Diagnostics returns plugin health info.
func (p *Plugin) Diagnostics() []plugin.Diagnostic {
	status := "ok"
	detail := p.tree.Summary()
	if p.tree.TotalCount() == 0 {
		status = "clean"
	}
	return []plugin.Diagnostic{
		{ID: "git-status", Status: status, Detail: detail},
	}
}

// refresh reloads the git status.
func (p *Plugin) refresh() tea.Cmd {
	return func() tea.Msg {
		if err := p.tree.Refresh(); err != nil {
			return ErrorMsg{Err: err}
		}
		return RefreshDoneMsg{}
	}
}

// startWatcher starts the file system watcher.
func (p *Plugin) startWatcher() tea.Cmd {
	return func() tea.Msg {
		watcher, err := NewWatcher(p.ctx.WorkDir)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		p.watcher = watcher
		return WatchStartedMsg{}
	}
}

// loadDiff loads the diff for a file.
func (p *Plugin) loadDiff(path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		content, err := GetDiff(p.ctx.WorkDir, path, staged)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return DiffLoadedMsg{Content: content}
	}
}

// openFile opens a file in the default editor.
func (p *Plugin) openFile(path string) tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		fullPath := filepath.Join(p.ctx.WorkDir, path)
		return OpenFileMsg{Editor: editor, Path: fullPath}
	}
}

// ensureCursorVisible adjusts scroll to keep cursor visible.
func (p *Plugin) ensureCursorVisible() {
	visibleRows := p.height - 4 // Account for header and section spacing
	if visibleRows < 1 {
		visibleRows = 1
	}

	if p.cursor < p.scrollOff {
		p.scrollOff = p.cursor
	} else if p.cursor >= p.scrollOff+visibleRows {
		p.scrollOff = p.cursor - visibleRows + 1
	}
}

// countLines counts newlines in a string.
func countLines(s string) int {
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

// Message types
type RefreshMsg struct{}
type RefreshDoneMsg struct{}
type WatchEventMsg struct{}
type WatchStartedMsg struct{}
type ErrorMsg struct{ Err error }
type DiffLoadedMsg struct{ Content string }
type OpenFileMsg struct {
	Editor string
	Path   string
}

// TickCmd returns a command that triggers a refresh every second.
func TickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return RefreshMsg{}
	})
}
