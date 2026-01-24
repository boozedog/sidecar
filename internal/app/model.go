package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/community"
	"github.com/marcus/sidecar/internal/config"
	"github.com/marcus/sidecar/internal/keymap"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/palette"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/state"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/version"
)

// ModalKind identifies an app-level modal with explicit priority ordering.
// Lower values = higher priority (checked first for rendering and input routing).
type ModalKind int

const (
	ModalNone            ModalKind = iota // No modal open
	ModalPalette                          // Command palette (highest priority)
	ModalHelp                             // Help overlay
	ModalDiagnostics                      // Diagnostics/version info
	ModalQuitConfirm                      // Quit confirmation dialog
	ModalProjectSwitcher                  // Project switcher
	ModalThemeSwitcher                    // Theme switcher (lowest priority)
)

// activeModal returns the highest-priority open modal.
// This is the single source of truth for which modal is currently active.
func (m *Model) activeModal() ModalKind {
	switch {
	case m.showPalette:
		return ModalPalette
	case m.showHelp:
		return ModalHelp
	case m.showDiagnostics:
		return ModalDiagnostics
	case m.showQuitConfirm:
		return ModalQuitConfirm
	case m.showProjectSwitcher:
		return ModalProjectSwitcher
	case m.showThemeSwitcher:
		return ModalThemeSwitcher
	default:
		return ModalNone
	}
}

// hasModal returns true if any app-level modal is open.
func (m *Model) hasModal() bool {
	return m.activeModal() != ModalNone
}

// TabBounds represents the X position range of a tab for mouse hit testing.
type TabBounds struct {
	Start, End int
}

// Model is the root Bubble Tea model for the sidecar application.
type Model struct {
	// Configuration
	cfg *config.Config

	// Plugin management
	registry     *plugin.Registry
	activePlugin int

	// Keymap
	keymap        *keymap.Registry
	activeContext string

	// UI state
	width, height   int
	showHelp        bool
	showDiagnostics bool
	showFooter      bool
	showPalette     bool
	showQuitConfirm bool
	quitButtonFocus int // 0=quit, 1=cancel
	quitButtonHover int // 0=none, 1=quit, 2=cancel
	palette         palette.Model

	// Project switcher modal
	showProjectSwitcher     bool
	projectSwitcherCursor   int
	projectSwitcherScroll   int
	projectSwitcherHover    int // -1 = no hover, 0+ = hovered project index
	projectSwitcherInput    textinput.Model
	projectSwitcherFiltered []config.ProjectConfig

	// Project add sub-mode (within project switcher)
	projectAddMode        bool
	projectAddNameInput   textinput.Model
	projectAddPathInput   textinput.Model
	projectAddFocus       int // 0=name, 1=path, 2=add button, 3=cancel button
	projectAddButtonHover int // 0=none, 1=add, 2=cancel
	projectAddError       string

	// Theme switcher modal
	showThemeSwitcher          bool
	themeSwitcherCursor        int
	themeSwitcherScroll        int
	themeSwitcherHover         int // -1 = no hover, 0+ = hovered theme index
	themeSwitcherInput         textinput.Model
	themeSwitcherFiltered      []string
	themeSwitcherOriginal      string // original theme to restore on cancel
	themeSwitcherCommunityName string

	// Community theme browser (sub-mode of theme switcher)
	showCommunityBrowser     bool
	communityBrowserCursor   int
	communityBrowserScroll   int
	communityBrowserHover    int
	communityBrowserInput    textinput.Model
	communityBrowserFiltered []string
	communityBrowserOriginal string // theme state to restore on cancel

	// Header/footer
	ui *UIState

	// Status/toast messages
	statusMsg     string
	statusExpiry  time.Time
	statusIsError bool

	// Error handling
	lastError error

	// Ready state
	ready bool

	// Version info
	currentVersion  string
	updateAvailable *version.UpdateAvailableMsg
	tdVersionInfo   *version.TdVersionMsg

	// Update feature state
	updateButtonFocus  bool
	updateInProgress   bool
	updateError        string
	needsRestart       bool
	updateButtonBounds mouse.Rect
	updateSpinnerFrame int

	// Intro animation
	intro IntroModel
}

// New creates a new application model.
// initialPluginID optionally specifies which plugin to focus on startup (empty = first plugin).
func New(reg *plugin.Registry, km *keymap.Registry, cfg *config.Config, currentVersion, workDir, initialPluginID string) Model {
	repoName := GetRepoName(workDir)
	ui := NewUIState()
	ui.WorkDir = workDir

	// Determine initial active plugin index
	activeIdx := 0
	if initialPluginID != "" {
		for i, p := range reg.Plugins() {
			if p.ID() == initialPluginID {
				activeIdx = i
				break
			}
		}
	}

	return Model{
		cfg:                   cfg,
		registry:              reg,
		keymap:                km,
		activePlugin:          activeIdx,
		activeContext:         "global",
		showFooter:            true,
		palette:               palette.New(),
		ui:                    ui,
		ready:                 false,
		intro:                 NewIntroModel(repoName),
		currentVersion:        currentVersion,
		projectSwitcherHover:  -1, // No hover initially
		themeSwitcherHover:    -1, // No hover initially
		communityBrowserHover: -1,
	}
}

// Init initializes the model and returns initial commands.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tickCmd(),
		IntroTick(),
		version.CheckAsync(m.currentVersion),
		version.CheckTdAsync(),
	}

	// Start all registered plugins
	for _, cmd := range m.registry.Start() {
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// ActivePlugin returns the currently active plugin.
func (m Model) ActivePlugin() plugin.Plugin {
	plugins := m.registry.Plugins()
	if len(plugins) == 0 {
		return nil
	}
	if m.activePlugin >= len(plugins) {
		m.activePlugin = 0
	}
	return plugins[m.activePlugin]
}

// SetActivePlugin sets the active plugin by index and returns a command
// to notify the plugin it has been focused.
func (m *Model) SetActivePlugin(idx int) tea.Cmd {
	plugins := m.registry.Plugins()
	if idx >= 0 && idx < len(plugins) {
		// Unfocus current
		if current := m.ActivePlugin(); current != nil {
			current.SetFocused(false)
		}
		m.activePlugin = idx
		// Focus new
		if next := m.ActivePlugin(); next != nil {
			next.SetFocused(true)
			m.activeContext = next.FocusContext()
			return PluginFocused()
		}
	}
	return nil
}

// NextPlugin switches to the next plugin.
func (m *Model) NextPlugin() tea.Cmd {
	plugins := m.registry.Plugins()
	if len(plugins) == 0 {
		return nil
	}
	return m.SetActivePlugin((m.activePlugin + 1) % len(plugins))
}

// PrevPlugin switches to the previous plugin.
func (m *Model) PrevPlugin() tea.Cmd {
	plugins := m.registry.Plugins()
	if len(plugins) == 0 {
		return nil
	}
	idx := m.activePlugin - 1
	if idx < 0 {
		idx = len(plugins) - 1
	}
	return m.SetActivePlugin(idx)
}

// FocusPluginByID switches to a plugin by its ID.
func (m *Model) FocusPluginByID(id string) tea.Cmd {
	plugins := m.registry.Plugins()
	for i, p := range plugins {
		if p.ID() == id {
			return m.SetActivePlugin(i)
		}
	}
	return nil
}

// ShowToast displays a temporary status message.
func (m *Model) ShowToast(msg string, duration time.Duration) {
	m.statusMsg = msg
	m.statusExpiry = time.Now().Add(duration)
}

// ClearToast clears any expired toast message.
func (m *Model) ClearToast() {
	if m.statusMsg != "" && time.Now().After(m.statusExpiry) {
		m.statusMsg = ""
		m.statusIsError = false
	}
}

// hasUpdatesAvailable returns true if either sidecar or td has an update available.
func (m *Model) hasUpdatesAvailable() bool {
	if m.updateAvailable != nil {
		return true
	}
	if m.tdVersionInfo != nil && m.tdVersionInfo.HasUpdate && m.tdVersionInfo.Installed {
		return true
	}
	return false
}

// doUpdate executes go install commands for available updates.
func (m *Model) doUpdate() tea.Cmd {
	sidecarUpdate := m.updateAvailable
	tdUpdate := m.tdVersionInfo

	return func() tea.Msg {
		// Check Go is available
		if _, err := exec.LookPath("go"); err != nil {
			return UpdateErrorMsg{Step: "check", Err: fmt.Errorf("go not found in PATH")}
		}

		var sidecarUpdated, tdUpdated bool
		var newSidecarVersion, newTdVersion string

		// Update sidecar
		if sidecarUpdate != nil {
			args := []string{
				"install",
				"-ldflags", fmt.Sprintf("-X main.Version=%s", sidecarUpdate.LatestVersion),
				fmt.Sprintf("github.com/marcus/sidecar/cmd/sidecar@%s", sidecarUpdate.LatestVersion),
			}
			cmd := exec.Command("go", args...)
			if output, err := cmd.CombinedOutput(); err != nil {
				return UpdateErrorMsg{Step: "sidecar", Err: fmt.Errorf("%v: %s", err, output)}
			}
			sidecarUpdated = true
			newSidecarVersion = sidecarUpdate.LatestVersion
		}

		// Update td
		if tdUpdate != nil && tdUpdate.HasUpdate && tdUpdate.Installed {
			cmd := exec.Command("go", "install",
				fmt.Sprintf("github.com/marcus/td@%s", tdUpdate.LatestVersion))
			if output, err := cmd.CombinedOutput(); err != nil {
				return UpdateErrorMsg{Step: "td", Err: fmt.Errorf("%v: %s", err, output)}
			}
			tdUpdated = true
			newTdVersion = tdUpdate.LatestVersion
		}

		return UpdateSuccessMsg{
			SidecarUpdated:    sidecarUpdated,
			TdUpdated:         tdUpdated,
			NewSidecarVersion: newSidecarVersion,
			NewTdVersion:      newTdVersion,
		}
	}
}

// updateDiagnosticsButtonBounds calculates the button bounds for mouse clicks.
// Call this when diagnostics modal is shown or window is resized.
func (m *Model) updateDiagnosticsButtonBounds() {
	if !m.hasUpdatesAvailable() || m.updateInProgress || m.needsRestart {
		m.updateButtonBounds = mouse.Rect{} // No clickable button
		return
	}

	// The modal content has a known structure:
	// - Logo: 7 lines
	// - Blank: 1
	// - Plugins section: 1 (title) + N (one per plugin with potential diagnostics)
	// - Blank: 1
	// - System section: 1 (title) + 2 (workdir, refresh)
	// - Blank: 1
	// - Version section: 1 (title) + 2-3 (sidecar, td)
	// - Blank: 1
	// - Button line (this is what we need)

	// Count lines dynamically
	lineCount := 7 + 1 // logo + blank
	lineCount++        // plugins title
	for _, p := range m.registry.Plugins() {
		lineCount++
		if dp, ok := p.(plugin.DiagnosticProvider); ok {
			lineCount += len(dp.Diagnostics())
		}
	}
	lineCount++    // blank after plugins
	lineCount += 3 // system section (title + 2 lines)
	lineCount++    // blank
	lineCount++    // version title
	lineCount++    // sidecar version line
	if m.tdVersionInfo != nil {
		lineCount++ // td version line
	}
	lineCount++ // blank before button
	// Now we're at the button line

	buttonLineInModal := lineCount

	// ModalBox has 1 cell padding all around, plus 1 cell border
	modalPadding := 1
	modalBorder := 1
	buttonIndent := 2 // "  " before button

	// Estimate modal dimensions (will be close enough for click detection)
	// Logo width is approximately 45 chars
	modalWidth := 50 + (modalPadding * 2) + (modalBorder * 2)
	modalHeight := lineCount + 4 + (modalPadding * 2) + (modalBorder * 2) // +4 for lines after button

	// Calculate modal position (centered)
	modalX := (m.width - modalWidth) / 2
	modalY := (m.height - modalHeight) / 2
	if modalX < 0 {
		modalX = 0
	}
	if modalY < 0 {
		modalY = 0
	}

	// Calculate button position
	buttonX := modalX + modalBorder + modalPadding + buttonIndent
	buttonY := modalY + modalBorder + modalPadding + buttonLineInModal
	buttonWidth := 8 // " Update "

	m.updateButtonBounds = mouse.Rect{X: buttonX, Y: buttonY, W: buttonWidth, H: 1}
}

// resetProjectSwitcher resets the project switcher modal state.
func (m *Model) resetProjectSwitcher() {
	m.showProjectSwitcher = false
	m.projectSwitcherCursor = 0
	m.projectSwitcherScroll = 0
	m.projectSwitcherHover = -1
	m.projectSwitcherFiltered = nil
}

// initProjectSwitcher initializes the project switcher modal.
func (m *Model) initProjectSwitcher() {
	ti := textinput.New()
	ti.Placeholder = "Filter projects..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	m.projectSwitcherInput = ti
	m.projectSwitcherFiltered = m.cfg.Projects.List
	m.projectSwitcherCursor = 0
	m.projectSwitcherScroll = 0
	m.projectSwitcherHover = -1

	// Set cursor to current project if found
	for i, proj := range m.projectSwitcherFiltered {
		if proj.Path == m.ui.WorkDir {
			m.projectSwitcherCursor = i
			break
		}
	}
}

// filterProjects filters projects by name or path using a case-insensitive substring match.
func filterProjects(all []config.ProjectConfig, query string) []config.ProjectConfig {
	if query == "" {
		return all
	}
	q := strings.ToLower(query)
	var matches []config.ProjectConfig
	for _, p := range all {
		if strings.Contains(strings.ToLower(p.Name), q) ||
			strings.Contains(strings.ToLower(p.Path), q) {
			matches = append(matches, p)
		}
	}
	return matches
}

// projectSwitcherEnsureCursorVisible adjusts scroll to keep cursor in view.
// Returns the new scroll offset.
func projectSwitcherEnsureCursorVisible(cursor, scroll, maxVisible int) int {
	if cursor < scroll {
		return cursor
	}
	if cursor >= scroll+maxVisible {
		return cursor - maxVisible + 1
	}
	return scroll
}

// switchProject switches all plugins to a new project directory.
func (m *Model) switchProject(projectPath string) tea.Cmd {
	// Skip if already on this project
	if projectPath == m.ui.WorkDir {
		return func() tea.Msg {
			return ToastMsg{Message: "Already on this project", Duration: 2 * time.Second}
		}
	}

	// Save the active plugin state for the old workdir
	oldWorkDir := m.ui.WorkDir
	if activePlugin := m.ActivePlugin(); activePlugin != nil {
		state.SetActivePlugin(oldWorkDir, activePlugin.ID())
	}

	// Update the UI state
	m.ui.WorkDir = projectPath
	m.intro.RepoName = GetRepoName(projectPath)

	// Reinitialize all plugins with the new working directory
	// This stops all plugins, updates the context, and starts them again
	startCmds := m.registry.Reinit(projectPath)

	// Restore active plugin for the new workdir if saved, otherwise keep current
	newActivePluginID := state.GetActivePlugin(projectPath)
	if newActivePluginID != "" {
		m.FocusPluginByID(newActivePluginID)
	}

	// Return batch of start commands plus a toast notification
	return tea.Batch(
		tea.Batch(startCmds...),
		func() tea.Msg {
			return ToastMsg{
				Message:  fmt.Sprintf("Switched to %s", GetRepoName(projectPath)),
				Duration: 3 * time.Second,
			}
		},
	)
}

// copyProjectSetupPrompt copies an LLM-friendly prompt for configuring projects.
func (m *Model) copyProjectSetupPrompt() tea.Cmd {
	prompt := `Configure sidecar projects for me.

Add my code projects to ~/.config/sidecar/config.json using this format:

{
  "projects": {
    "list": [
      {"name": "short-name", "path": "~/code/project-path"}
    ]
  }
}

Rules:
- Use short, memorable names (1-2 words, lowercase, hyphens ok)
- Expand ~ to full home path if needed
- Only add directories that exist and contain code
- Merge with existing config if present

My code is located at: [TELL ME WHERE YOUR CODE DIRECTORIES ARE]`

	if err := clipboard.WriteAll(prompt); err != nil {
		return func() tea.Msg {
			return ToastMsg{Message: "Copy failed: " + err.Error(), Duration: 2 * time.Second}
		}
	}
	return func() tea.Msg {
		return ToastMsg{Message: "Copied LLM setup prompt", Duration: 2 * time.Second}
	}
}

// initProjectAdd initializes the project add sub-mode.
func (m *Model) initProjectAdd() {
	m.projectAddMode = true
	m.projectAddError = ""
	m.projectAddFocus = 0
	m.projectAddButtonHover = 0

	nameInput := textinput.New()
	nameInput.Placeholder = "project-name"
	nameInput.CharLimit = 40
	nameInput.Width = 36
	nameInput.Focus()
	m.projectAddNameInput = nameInput

	pathInput := textinput.New()
	pathInput.Placeholder = "~/code/project-path"
	pathInput.CharLimit = 200
	pathInput.Width = 36
	m.projectAddPathInput = pathInput
}

// resetProjectAdd resets the project add sub-mode state.
func (m *Model) resetProjectAdd() {
	m.projectAddMode = false
	m.projectAddError = ""
	m.projectAddFocus = 0
	m.projectAddButtonHover = 0
}

// validateProjectAdd validates the project add form inputs.
// Returns an error message or empty string if valid.
func (m *Model) validateProjectAdd() string {
	name := strings.TrimSpace(m.projectAddNameInput.Value())
	path := strings.TrimSpace(m.projectAddPathInput.Value())

	if name == "" {
		return "Name is required"
	}
	if path == "" {
		return "Path is required"
	}

	// Expand path for validation
	expanded := config.ExpandPath(path)

	// Check path exists and is a directory
	info, err := os.Stat(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return "Path does not exist"
		}
		return "Cannot access path"
	}
	if !info.IsDir() {
		return "Path is not a directory"
	}

	// Check for duplicate name or path
	for _, proj := range m.cfg.Projects.List {
		if strings.EqualFold(proj.Name, name) {
			return "Project name already exists"
		}
		if proj.Path == expanded {
			return "Project path already configured"
		}
	}

	return ""
}

// saveProjectAdd saves the new project to config and refreshes the list.
func (m *Model) saveProjectAdd() tea.Cmd {
	name := strings.TrimSpace(m.projectAddNameInput.Value())
	path := strings.TrimSpace(m.projectAddPathInput.Value())

	// Add to in-memory config
	m.cfg.Projects.List = append(m.cfg.Projects.List, config.ProjectConfig{
		Name: name,
		Path: config.ExpandPath(path),
	})

	// Save to disk
	if err := config.Save(m.cfg); err != nil {
		return func() tea.Msg {
			return ToastMsg{Message: "Added project (save failed: " + err.Error() + ")", Duration: 3 * time.Second, IsError: true}
		}
	}

	// Refresh the filtered list
	m.projectSwitcherFiltered = m.cfg.Projects.List

	return func() tea.Msg {
		return ToastMsg{Message: fmt.Sprintf("Added project: %s", name), Duration: 3 * time.Second}
	}
}

// resetThemeSwitcher resets the theme switcher modal state.
func (m *Model) resetThemeSwitcher() {
	m.showThemeSwitcher = false
	m.themeSwitcherCursor = 0
	m.themeSwitcherScroll = 0
	m.themeSwitcherHover = -1
	m.themeSwitcherFiltered = nil
	m.themeSwitcherOriginal = ""
	m.themeSwitcherCommunityName = ""
}

// initThemeSwitcher initializes the theme switcher modal.
func (m *Model) initThemeSwitcher() {
	m.resetCommunityBrowser()

	ti := textinput.New()
	ti.Placeholder = "Filter themes..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	m.themeSwitcherInput = ti
	m.themeSwitcherFiltered = styles.ListThemes()
	m.themeSwitcherCursor = 0
	m.themeSwitcherScroll = 0
	m.themeSwitcherHover = -1
	m.themeSwitcherOriginal = styles.GetCurrentThemeName()
	m.themeSwitcherCommunityName = ""

	if freshCfg, err := config.Load(); err == nil {
		if freshCfg.UI.Theme.Name != "" {
			m.themeSwitcherOriginal = freshCfg.UI.Theme.Name
		}
		m.themeSwitcherCommunityName = communityNameFromOverrides(freshCfg.UI.Theme.Overrides)
	}

	// Set cursor to current theme if found
	if m.themeSwitcherCommunityName == "" {
		for i, name := range m.themeSwitcherFiltered {
			if name == m.themeSwitcherOriginal {
				m.themeSwitcherCursor = i
				break
			}
		}
	} else {
		m.initCommunityBrowser()
		m.selectCommunityScheme(m.themeSwitcherCommunityName)
	}
}

// filterThemes filters themes by name using a case-insensitive substring match.
func filterThemes(all []string, query string) []string {
	if query == "" {
		return all
	}
	q := strings.ToLower(query)
	var matches []string
	for _, name := range all {
		if strings.Contains(strings.ToLower(name), q) {
			matches = append(matches, name)
		}
	}
	return matches
}

// themeSwitcherEnsureCursorVisible adjusts scroll to keep cursor in view.
func themeSwitcherEnsureCursorVisible(cursor, scroll, maxVisible int) int {
	if cursor < scroll {
		return cursor
	}
	if cursor >= scroll+maxVisible {
		return cursor - maxVisible + 1
	}
	return scroll
}

// initCommunityBrowser initializes the community theme browser sub-mode.
func (m *Model) initCommunityBrowser() {
	ti := textinput.New()
	ti.Placeholder = "Search community themes..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	m.communityBrowserInput = ti
	m.communityBrowserFiltered = community.ListSchemes()
	m.communityBrowserCursor = 0
	m.communityBrowserScroll = 0
	m.communityBrowserHover = -1
	m.communityBrowserOriginal = m.themeSwitcherOriginal
	m.showCommunityBrowser = true
}

// resetCommunityBrowser resets the community browser state.
func (m *Model) resetCommunityBrowser() {
	m.showCommunityBrowser = false
	m.communityBrowserCursor = 0
	m.communityBrowserScroll = 0
	m.communityBrowserHover = -1
	m.communityBrowserFiltered = nil
	m.communityBrowserOriginal = ""
}

// filterCommunitySchemes filters scheme names by substring match.
func filterCommunitySchemes(all []string, query string) []string {
	if query == "" {
		return all
	}
	q := strings.ToLower(query)
	var matches []string
	for _, name := range all {
		if strings.Contains(strings.ToLower(name), q) {
			matches = append(matches, name)
		}
	}
	return matches
}

func communityNameFromOverrides(overrides map[string]interface{}) string {
	if overrides == nil {
		return ""
	}
	if value, ok := overrides["communityName"]; ok {
		if name, ok := value.(string); ok {
			return name
		}
	}
	return ""
}

func (m *Model) selectCommunityScheme(name string) {
	if name == "" {
		return
	}
	for i, schemeName := range m.communityBrowserFiltered {
		if schemeName == name {
			m.communityBrowserCursor = i
			m.communityBrowserScroll = themeSwitcherEnsureCursorVisible(i, 0, 8)
			m.communityBrowserHover = -1
			return
		}
	}
}

// applyThemeFromConfig applies a theme, using config overrides only if the
// saved config has that theme selected. This means live preview of other themes
// won't include user customizations (which is intentional - you want to see the
// base theme, not your customizations for a different theme).
func (m *Model) applyThemeFromConfig(themeName string) {
	freshCfg, err := config.Load()
	if err == nil && freshCfg.UI.Theme.Name == themeName {
		styles.ApplyThemeWithGenericOverrides(themeName, freshCfg.UI.Theme.Overrides)
	} else {
		styles.ApplyTheme(themeName)
	}
}
