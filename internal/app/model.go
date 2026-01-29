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
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/palette"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/state"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/theme"
	"github.com/marcus/sidecar/internal/version"
)

// ModalKind identifies an app-level modal with explicit priority ordering.
// Lower values = higher priority (checked first for rendering and input routing).
type ModalKind int

const (
	ModalNone             ModalKind = iota // No modal open
	ModalPalette                           // Command palette (highest priority)
	ModalHelp                              // Help overlay
	ModalUpdate                            // Update modal
	ModalDiagnostics                       // Diagnostics/version info
	ModalQuitConfirm                       // Quit confirmation dialog
	ModalProjectSwitcher                   // Project switcher
	ModalWorktreeSwitcher                  // Worktree switcher
	ModalThemeSwitcher                     // Theme switcher
	ModalIssueInput                        // Issue ID text input
	ModalIssuePreview                      // Issue preview display (lowest priority)
)

// activeModal returns the highest-priority open modal.
// This is the single source of truth for which modal is currently active.
func (m *Model) activeModal() ModalKind {
	switch {
	case m.showPalette:
		return ModalPalette
	case m.showHelp:
		return ModalHelp
	case m.updateModalState != UpdateModalClosed:
		return ModalUpdate
	case m.showDiagnostics:
		return ModalDiagnostics
	case m.showQuitConfirm:
		return ModalQuitConfirm
	case m.showProjectSwitcher:
		return ModalProjectSwitcher
	case m.showWorktreeSwitcher:
		return ModalWorktreeSwitcher
	case m.showThemeSwitcher:
		return ModalThemeSwitcher
	case m.showIssueInput:
		return ModalIssueInput
	case m.showIssuePreview:
		return ModalIssuePreview
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
	width, height    int
	showHelp         bool
	helpModal        *modal.Modal
	helpModalWidth   int
	helpMouseHandler *mouse.Handler
	showDiagnostics         bool
	diagnosticsModal        *modal.Modal
	diagnosticsModalWidth   int
	diagnosticsMouseHandler *mouse.Handler
	showFooter              bool
	showPalette      bool
	showQuitConfirm  bool
	quitModal        *modal.Modal
	quitMouseHandler *mouse.Handler
	palette          palette.Model

	// Project switcher modal
	showProjectSwitcher         bool
	projectSwitcherCursor       int
	projectSwitcherScroll       int // scroll offset for list
	projectSwitcherInput        textinput.Model
	projectSwitcherFiltered     []config.ProjectConfig
	projectSwitcherModal        *modal.Modal
	projectSwitcherModalWidth   int
	projectSwitcherMouseHandler *mouse.Handler

	// Project add sub-mode (within project switcher)
	projectAddMode         bool
	projectAddNameInput    textinput.Model
	projectAddPathInput    textinput.Model
	projectAddError        string
	projectAddModal        *modal.Modal
	projectAddModalWidth   int
	projectAddMouseHandler *mouse.Handler

	// Theme picker within add-project flow
	projectAddThemeMode       bool // is theme picker sub-modal open?
	projectAddThemeCursor     int
	projectAddThemeScroll     int
	projectAddThemeInput      textinput.Model
	projectAddThemeFiltered   []string // filtered built-in theme list
	projectAddThemeSelected   string   // selected theme name (empty = use global)
	projectAddCommunityMode   bool     // in community sub-browser?
	projectAddCommunityList   []string // filtered community scheme names
	projectAddCommunityCursor int
	projectAddCommunityScroll int

	// Worktree switcher modal
	showWorktreeSwitcher         bool
	worktreeSwitcherCursor       int
	worktreeSwitcherScroll       int
	worktreeSwitcherInput        textinput.Model
	worktreeSwitcherFiltered     []WorktreeInfo
	worktreeSwitcherAll          []WorktreeInfo
	worktreeSwitcherModal        *modal.Modal
	worktreeSwitcherModalWidth   int
	worktreeSwitcherMouseHandler *mouse.Handler
	activeWorktreePath           string // Currently active worktree path (empty = main repo)
	worktreeCheckCounter         int    // Counter for periodic worktree existence check

	// Theme switcher modal
	showThemeSwitcher          bool
	themeSwitcherModal         *modal.Modal
	themeSwitcherModalWidth    int
	themeSwitcherMouseHandler  *mouse.Handler
	themeSwitcherCursor        int // cursor position in theme list
	themeSwitcherScroll        int // scroll offset in theme list
	themeSwitcherHover         int // hovered theme index (-1 = none)
	themeSwitcherSelectedIdx   int
	themeSwitcherInput         textinput.Model
	themeSwitcherFiltered      []string
	themeSwitcherOriginal      string // original theme to restore on cancel
	themeSwitcherCommunityName string
	themeSwitcherScope         string // "global" or "project"

	// Issue preview - input phase
	showIssueInput         bool
	issueInputInput        textinput.Model
	issueInputModal        *modal.Modal
	issueInputModalWidth   int
	issueInputMouseHandler *mouse.Handler

	// Issue input auto-complete
	issueSearchResults []IssueSearchResult
	issueSearchQuery   string // last query sent to td search
	issueSearchLoading bool
	issueSearchCursor  int // selected result index (-1 = none/input focused)

	// Issue preview - preview phase
	showIssuePreview         bool
	issuePreviewData         *IssuePreviewData
	issuePreviewLoading      bool
	issuePreviewError        error
	issuePreviewModal        *modal.Modal
	issuePreviewModalWidth   int
	issuePreviewMouseHandler *mouse.Handler

	// Community theme browser (sub-mode of theme switcher)
	showCommunityBrowser     bool
	communityBrowserCursor   int
	communityBrowserScroll   int
	communityBrowserHover    int
	communityBrowserInput    textinput.Model
	communityBrowserFiltered []string
	communityBrowserOriginal     string // theme state to restore on cancel
	communityBrowserModal        *modal.Modal
	communityBrowserModalWidth   int
	communityBrowserMouseHandler *mouse.Handler

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
	updateInProgress   bool
	updateError        string
	needsRestart       bool
	updateSpinnerFrame int

	// Update modal state
	updateModalState      UpdateModalState
	updatePhase           UpdatePhase
	updatePhaseStatus     map[UpdatePhase]string
	updateStartTime       time.Time
	updateReleaseNotes    string // Release notes for current update
	updateChangelog       string // Full changelog content
	changelogVisible      bool
	changelogScrollOffset int
	changelogScrollState  *changelogViewState // Shared state for modal closure

	// Update modal (declarative)
	updatePreviewModal        *modal.Modal
	updatePreviewModalWidth   int
	updatePreviewMouseHandler *mouse.Handler
	updateCompleteModal       *modal.Modal
	updateCompleteModalWidth  int
	updateCompleteMouseHandler *mouse.Handler
	updateErrorModal          *modal.Modal
	updateErrorModalWidth     int
	updateErrorMouseHandler   *mouse.Handler
	changelogModal            *modal.Modal
	changelogModalWidth       int
	changelogMouseHandler     *mouse.Handler
	changelogRenderedLines    []string // Cached rendered changelog lines
	changelogMaxVisibleLines  int      // Max lines visible in viewport

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
		communityBrowserHover: -1,
		updatePhaseStatus:     make(map[UpdatePhase]string),
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

// initQuitModal initializes the quit confirmation modal.
func (m *Model) initQuitModal() {
	m.quitModal = modal.New("Quit Sidecar?",
		modal.WithWidth(50),
		modal.WithVariant(modal.VariantDefault),
		modal.WithPrimaryAction("quit"),
	).
		AddSection(modal.Text("Are you sure you want to quit?")).
		AddSection(modal.Spacer()).
		AddSection(modal.Buttons(
			modal.Btn(" Quit ", "quit"),
			modal.Btn(" Cancel ", "cancel"),
		))
	m.quitMouseHandler = mouse.NewHandler()
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

// initPhaseStatus initializes all update phases to pending status.
func (m *Model) initPhaseStatus() {
	m.updatePhaseStatus = map[UpdatePhase]string{
		PhaseCheckPrereqs: "pending",
		PhaseInstalling:   "pending",
		PhaseVerifying:    "pending",
	}
}

// startUpdateWithPhases starts the update process with phase tracking.
// This replaces the old doUpdate for the new modal-based update flow.
func (m *Model) startUpdateWithPhases() tea.Cmd {
	return tea.Batch(
		// Start elapsed timer
		m.startElapsedTimer(),
		// Start spinner animation
		updateSpinnerTick(),
		// Start the first phase (check prerequisites)
		m.runCheckPrerequisites(),
	)
}

// startElapsedTimer starts the elapsed time ticker.
func (m *Model) startElapsedTimer() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return UpdateElapsedTickMsg{}
	})
}

// UpdatePrereqsPassedMsg signals prerequisites check passed, triggers install phase.
type UpdatePrereqsPassedMsg struct{}

// UpdateInstallDoneMsg signals install completed, triggers verify phase.
type UpdateInstallDoneMsg struct {
	SidecarUpdated    bool
	TdUpdated         bool
	NewSidecarVersion string
	NewTdVersion      string
}

// runCheckPrerequisites runs the prerequisites check phase.
func (m *Model) runCheckPrerequisites() tea.Cmd {
	return func() tea.Msg {
		if _, err := exec.LookPath("go"); err != nil {
			return UpdateErrorMsg{Step: "check", Err: fmt.Errorf("go not found in PATH")}
		}
		return UpdatePrereqsPassedMsg{}
	}
}

// runInstallPhase runs the install phase (go install commands).
func (m *Model) runInstallPhase() tea.Cmd {
	sidecarUpdate := m.updateAvailable
	tdUpdate := m.tdVersionInfo

	return func() tea.Msg {
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

		return UpdateInstallDoneMsg{
			SidecarUpdated:    sidecarUpdated,
			TdUpdated:         tdUpdated,
			NewSidecarVersion: newSidecarVersion,
			NewTdVersion:      newTdVersion,
		}
	}
}

// runVerifyPhase runs the verification phase (check installed binaries).
func (m *Model) runVerifyPhase(installResult UpdateInstallDoneMsg) tea.Cmd {
	return func() tea.Msg {
		// Verify sidecar binary if it was updated
		if installResult.SidecarUpdated {
			sidecarPath, err := exec.LookPath("sidecar")
			if err != nil {
				return UpdateErrorMsg{Step: "verify", Err: fmt.Errorf("sidecar not found in PATH after install")}
			}
			// Verify the binary is executable by running --version
			cmd := exec.Command(sidecarPath, "--version")
			if err := cmd.Run(); err != nil {
				return UpdateErrorMsg{Step: "verify", Err: fmt.Errorf("sidecar binary not executable: %v", err)}
			}
		}

		// Verify td binary if it was updated
		if installResult.TdUpdated {
			tdPath, err := exec.LookPath("td")
			if err != nil {
				return UpdateErrorMsg{Step: "verify", Err: fmt.Errorf("td not found in PATH after install")}
			}
			// Verify the binary is executable
			cmd := exec.Command(tdPath, "version", "--short")
			if err := cmd.Run(); err != nil {
				return UpdateErrorMsg{Step: "verify", Err: fmt.Errorf("td binary not executable: %v", err)}
			}
		}

		return UpdateSuccessMsg{
			SidecarUpdated:    installResult.SidecarUpdated,
			TdUpdated:         installResult.TdUpdated,
			NewSidecarVersion: installResult.NewSidecarVersion,
			NewTdVersion:      installResult.NewTdVersion,
		}
	}
}

// resetProjectSwitcher resets the project switcher modal state.
func (m *Model) resetProjectSwitcher() {
	m.showProjectSwitcher = false
	m.projectSwitcherCursor = 0
	m.projectSwitcherScroll = 0
	m.projectSwitcherFiltered = nil
	m.clearProjectSwitcherModal()
	m.resetProjectAdd()
	// Restore current project's theme (undo any live preview)
	resolved := theme.ResolveTheme(m.cfg, m.ui.WorkDir)
	theme.ApplyResolved(resolved)
}

// clearProjectSwitcherModal clears the modal cache.
func (m *Model) clearProjectSwitcherModal() {
	m.projectSwitcherModal = nil
	m.projectSwitcherModalWidth = 0
	m.projectSwitcherMouseHandler = nil
}

// initProjectSwitcher initializes the project switcher modal.
func (m *Model) initProjectSwitcher() {
	m.clearProjectSwitcherModal()
	ti := textinput.New()
	ti.Placeholder = "Filter projects..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	m.projectSwitcherInput = ti
	m.projectSwitcherFiltered = m.cfg.Projects.List
	m.projectSwitcherCursor = 0
	m.projectSwitcherScroll = 0

	// Set cursor to current project if found
	for i, proj := range m.projectSwitcherFiltered {
		if proj.Path == m.ui.WorkDir {
			m.projectSwitcherCursor = i
			break
		}
	}
	// Preview the initially-selected project's theme
	m.previewProjectTheme()
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

	// Normalize old workdir for comparisons
	normalizedOldWorkDir, _ := normalizePath(oldWorkDir)

	// Check if target project has a saved worktree we should restore.
	// Only restore if projectPath is the main repo - if user explicitly chose a
	// specific worktree path (via worktree switcher), respect that choice.
	targetPath := projectPath
	if targetMainRepo := GetMainWorktreePath(projectPath); targetMainRepo != "" {
		normalizedProject, _ := normalizePath(projectPath)
		normalizedTargetMain, _ := normalizePath(targetMainRepo)

		// Only restore saved worktree if switching to the main repo path
		if normalizedProject == normalizedTargetMain {
			if savedWorktree := state.GetLastWorktreePath(normalizedTargetMain); savedWorktree != "" {
				// Don't restore if the saved worktree is where we're coming FROM
				// (user is explicitly leaving that worktree)
				if savedWorktree != normalizedOldWorkDir {
					// Verify saved worktree still exists
					if WorktreeExists(savedWorktree) {
						targetPath = savedWorktree
					} else {
						// Stale entry - clear it
						state.ClearLastWorktreePath(normalizedTargetMain)
					}
				}
			}
		}

		// Save the final target as last active worktree for this repo
		normalizedTarget, _ := normalizePath(targetPath)
		state.SetLastWorktreePath(normalizedTargetMain, normalizedTarget)
	}

	// Update the UI state
	m.ui.WorkDir = targetPath
	m.intro.RepoName = GetRepoName(targetPath)

	// Apply project-specific theme (or global fallback)
	resolved := theme.ResolveTheme(m.cfg, targetPath)
	theme.ApplyResolved(resolved)

	// Reinitialize all plugins with the new working directory
	// This stops all plugins, updates the context, and starts them again
	startCmds := m.registry.Reinit(targetPath)

	// Send WindowSizeMsg to all plugins so they recalculate layout/bounds.
	// Without this, plugins like td-monitor lose mouse interactivity because
	// their panel bounds are only calculated on WindowSizeMsg receipt.
	adjustedHeight := m.height - headerHeight
	if m.showFooter {
		adjustedHeight -= footerHeight
	}
	sizeMsg := tea.WindowSizeMsg{Width: m.width, Height: adjustedHeight}
	plugins := m.registry.Plugins()
	for i, p := range plugins {
		newPlugin, cmd := p.Update(sizeMsg)
		plugins[i] = newPlugin
		if cmd != nil {
			startCmds = append(startCmds, cmd)
		}
	}

	// Restore active plugin for the new workdir if saved, otherwise keep current
	newActivePluginID := state.GetActivePlugin(targetPath)
	if newActivePluginID != "" {
		m.FocusPluginByID(newActivePluginID)
	}

	// Return batch of start commands plus a toast notification
	return tea.Batch(
		tea.Batch(startCmds...),
		func() tea.Msg {
			return ToastMsg{
				Message:  fmt.Sprintf("Switched to %s", GetRepoName(targetPath)),
				Duration: 3 * time.Second,
			}
		},
	)
}

// previewProjectTheme applies the theme for the currently selected project in the switcher.
func (m *Model) previewProjectTheme() {
	projects := m.projectSwitcherFiltered
	if m.projectSwitcherCursor >= 0 && m.projectSwitcherCursor < len(projects) {
		resolved := theme.ResolveTheme(m.cfg, projects[m.projectSwitcherCursor].Path)
		theme.ApplyResolved(resolved)
	}
}

// currentProjectConfig returns the ProjectConfig for the current workdir, or nil.
// If the current workdir is a worktree, it also checks if the main worktree path
// matches a registered project (so theme scope selector works from worktrees).
func (m *Model) currentProjectConfig() *config.ProjectConfig {
	// First, check direct match
	for i := range m.cfg.Projects.List {
		if m.cfg.Projects.List[i].Path == m.ui.WorkDir {
			return &m.cfg.Projects.List[i]
		}
	}

	// If not found, check if we're in a worktree and the main repo is registered
	mainPath := GetMainWorktreePath(m.ui.WorkDir)
	if mainPath != "" && mainPath != m.ui.WorkDir {
		for i := range m.cfg.Projects.List {
			if m.cfg.Projects.List[i].Path == mainPath {
				return &m.cfg.Projects.List[i]
			}
		}
	}

	return nil
}

// saveThemeForScope saves a ThemeConfig based on the current scope setting.
// When saving to project scope from a worktree, uses the main repo path.
func (m *Model) saveThemeForScope(tc config.ThemeConfig) error {
	if m.themeSwitcherScope == "project" {
		// Use the project path, which may be the main worktree if we're in a worktree
		projectPath := m.ui.WorkDir
		if pc := m.currentProjectConfig(); pc != nil {
			projectPath = pc.Path
		}
		return config.SaveProjectTheme(projectPath, &tc)
	}
	return config.SaveGlobalTheme(tc)
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
	m.clearProjectAddModal()

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
	m.projectAddThemeSelected = ""
	m.clearProjectAddModal()
	m.resetProjectAddThemePicker()
}

// initProjectAddThemePicker opens the theme picker sub-modal.
func (m *Model) initProjectAddThemePicker() {
	m.projectAddThemeMode = true
	ti := textinput.New()
	ti.Placeholder = "Filter themes..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 36
	m.projectAddThemeInput = ti
	m.projectAddThemeFiltered = append([]string{"(use global)"}, styles.ListThemes()...)
	m.projectAddThemeCursor = 0
	m.projectAddThemeScroll = 0
	m.projectAddCommunityMode = false
}

// resetProjectAddThemePicker closes the theme picker sub-modal.
func (m *Model) resetProjectAddThemePicker() {
	m.projectAddThemeMode = false
	m.projectAddCommunityMode = false
	m.projectAddThemeCursor = 0
	m.projectAddThemeScroll = 0
	m.projectAddCommunityCursor = 0
	m.projectAddCommunityScroll = 0
}

// previewProjectAddTheme previews the currently-selected built-in theme.
func (m *Model) previewProjectAddTheme() {
	if m.projectAddThemeCursor >= 0 && m.projectAddThemeCursor < len(m.projectAddThemeFiltered) {
		name := m.projectAddThemeFiltered[m.projectAddThemeCursor]
		if name == "(use global)" {
			resolved := theme.ResolveTheme(m.cfg, m.ui.WorkDir)
			theme.ApplyResolved(resolved)
		} else {
			theme.ApplyResolved(theme.ResolvedTheme{BaseName: name})
		}
	}
}

// previewProjectAddCommunity previews the currently-selected community theme.
func (m *Model) previewProjectAddCommunity() {
	if m.projectAddCommunityCursor >= 0 && m.projectAddCommunityCursor < len(m.projectAddCommunityList) {
		name := m.projectAddCommunityList[m.projectAddCommunityCursor]
		theme.ApplyResolved(theme.ResolvedTheme{BaseName: "default", CommunityName: name})
	}
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

	// Build project config
	proj := config.ProjectConfig{
		Name: name,
		Path: config.ExpandPath(path),
	}

	// Add theme if user selected one
	if m.projectAddThemeSelected != "" && m.projectAddThemeSelected != "(use global)" {
		if community.GetScheme(m.projectAddThemeSelected) != nil {
			proj.Theme = &config.ThemeConfig{
				Name:      "default",
				Community: m.projectAddThemeSelected,
			}
		} else {
			proj.Theme = &config.ThemeConfig{
				Name: m.projectAddThemeSelected,
			}
		}
	}

	// Reload config from disk to avoid overwriting external changes
	cfg, err := config.Load()
	if err != nil {
		return func() tea.Msg {
			return ToastMsg{Message: "Failed to load config: " + err.Error(), Duration: 3 * time.Second, IsError: true}
		}
	}

	// Add project to fresh config
	cfg.Projects.List = append(cfg.Projects.List, proj)

	// Save to disk
	if err := config.Save(cfg); err != nil {
		return func() tea.Msg {
			return ToastMsg{Message: "Added project (save failed: " + err.Error() + ")", Duration: 3 * time.Second, IsError: true}
		}
	}

	// Update in-memory config
	m.cfg.Projects.List = cfg.Projects.List

	// Refresh the filtered list
	m.projectSwitcherFiltered = m.cfg.Projects.List

	return func() tea.Msg {
		return ToastMsg{Message: fmt.Sprintf("Added project: %s", name), Duration: 3 * time.Second}
	}
}

// resetThemeSwitcher resets the theme switcher modal state.
func (m *Model) resetThemeSwitcher() {
	m.showThemeSwitcher = false
	m.themeSwitcherSelectedIdx = 0
	m.themeSwitcherFiltered = nil
	m.themeSwitcherScope = ""
	m.themeSwitcherOriginal = ""
	m.themeSwitcherCommunityName = ""
	m.clearThemeSwitcherModal()
}

// clearThemeSwitcherModal clears the theme switcher modal state.
func (m *Model) clearThemeSwitcherModal() {
	m.themeSwitcherModal = nil
	m.themeSwitcherModalWidth = 0
	m.themeSwitcherMouseHandler = nil
}

// initIssueInput initializes the issue input modal.
func (m *Model) initIssueInput() {
	ti := textinput.New()
	ti.Placeholder = "Issue ID or search text"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 50
	m.issueInputInput = ti
	m.issueInputModal = nil
	m.issueInputModalWidth = 0
	m.issueInputMouseHandler = mouse.NewHandler()
	m.issueSearchResults = nil
	m.issueSearchQuery = ""
	m.issueSearchLoading = false
	m.issueSearchCursor = -1
}

// resetIssueInput resets the issue input modal state.
func (m *Model) resetIssueInput() {
	m.showIssueInput = false
	m.issueInputModal = nil
	m.issueInputModalWidth = 0
	m.issueInputMouseHandler = nil
	m.issueSearchResults = nil
	m.issueSearchQuery = ""
	m.issueSearchLoading = false
	m.issueSearchCursor = -1
}

// resetIssuePreview resets the issue preview modal state.
func (m *Model) resetIssuePreview() {
	m.showIssuePreview = false
	m.issuePreviewData = nil
	m.issuePreviewLoading = false
	m.issuePreviewError = nil
	m.issuePreviewModal = nil
	m.issuePreviewModalWidth = 0
	m.issuePreviewMouseHandler = nil
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
	m.themeSwitcherSelectedIdx = 0
	m.themeSwitcherOriginal = styles.GetCurrentThemeName()
	m.themeSwitcherCommunityName = ""
	m.themeSwitcherScope = "global" // default scope
	m.clearThemeSwitcherModal()

	if freshCfg, err := config.Load(); err == nil {
		if freshCfg.UI.Theme.Name != "" {
			m.themeSwitcherOriginal = freshCfg.UI.Theme.Name
		}
		m.themeSwitcherCommunityName = freshCfg.UI.Theme.Community
	}

	// Set cursor to current theme if found
	if m.themeSwitcherCommunityName == "" {
		for i, name := range m.themeSwitcherFiltered {
			if name == m.themeSwitcherOriginal {
				m.themeSwitcherSelectedIdx = i
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
	m.clearCommunityBrowserModal()
}

// clearCommunityBrowserModal clears the modal to force rebuild on next render.
func (m *Model) clearCommunityBrowserModal() {
	m.communityBrowserModal = nil
	m.communityBrowserModalWidth = 0
	m.communityBrowserMouseHandler = nil
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
		// Apply the saved theme with its full config (community + overrides)
		theme.ApplyResolved(theme.ResolvedTheme{
			BaseName:      themeName,
			CommunityName: freshCfg.UI.Theme.Community,
			Overrides:     freshCfg.UI.Theme.Overrides,
		})
	} else {
		styles.ApplyTheme(themeName)
	}
}
