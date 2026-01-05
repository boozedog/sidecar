package conversations

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/adapter"
	"github.com/marcus/sidecar/internal/app"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/state"
)

const (
	pluginID   = "conversations"
	pluginName = "conversations"
	pluginIcon = "C"

	// Default page size for messages
	defaultPageSize     = 50
	maxMessagesInMemory = 500

	previewDebounce = 150 * time.Millisecond

	// Divider width for pane separator
	dividerWidth = 1
)

// Mouse hit region identifiers
const (
	regionSidebar     = "sidebar"
	regionMainPane    = "main-pane"
	regionPaneDivider = "pane-divider"
	regionSessionItem = "session-item" // Individual session row (Data: session index)
	regionTurnItem    = "turn-item"    // Individual turn row (Data: turn index)
)

// View represents the current view mode.
type View int

const (
	ViewSessions View = iota
	ViewMessages
	ViewAnalytics
	ViewMessageDetail
)

// FocusPane represents which pane is active in two-pane mode.
type FocusPane int

const (
	PaneSidebar FocusPane = iota
	PaneMessages
)

// Plugin implements the conversations plugin.
type Plugin struct {
	ctx          *plugin.Context
	adapters     map[string]adapter.Adapter
	focused      bool
	mouseHandler *mouse.Handler

	// Current view
	view View

	// Session list state
	sessions  []adapter.Session
	cursor    int
	scrollOff int

	// Message view state
	selectedSession  string
	loadedSession    string // sessionID that p.messages currently represent
	messages         []adapter.Message
	turns            []Turn // messages grouped into turns
	turnCursor       int    // cursor for turn selection in list view
	turnScrollOff    int    // scroll offset for turns
	msgCursor        int
	msgScrollOff     int
	pageSize         int
	hasMore          bool
	expandedThinking map[string]bool // message ID -> thinking expanded
	sessionSummary   *SessionSummary // computed summary for current session
	showToolSummary  bool            // toggle for tool impact view

	// Message detail view state
	detailMode   bool  // true when showing detail in right pane (two-pane mode)
	detailTurn   *Turn // turn being viewed in detail
	detailScroll int

	// Analytics view state
	analyticsScrollOff int
	analyticsLines     []string // pre-rendered lines for scrolling

	// Two-pane layout state
	twoPane      bool      // Enable when width >= 120
	activePane   FocusPane // Which pane is focused
	sidebarWidth int       // Calculated width (~30%)
	previewToken int       // monotonically increasing token for debounced preview loads

	// View dimensions
	width  int
	height int

	// Watcher channel
	watchChan <-chan adapter.Event

	// Search state
	searchMode    bool
	searchQuery   string
	searchResults []adapter.Session

	// Filter state
	filterMode   bool
	filters      SearchFilters
	filterActive bool // true when any filter is active
}

// New creates a new conversations plugin.
func New() *Plugin {
	return &Plugin{
		pageSize:         defaultPageSize,
		expandedThinking: make(map[string]bool),
		mouseHandler:     mouse.NewHandler(),
	}
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return pluginID }

// Name returns the plugin display name.
func (p *Plugin) Name() string { return pluginName }

// Icon returns the plugin icon character.
func (p *Plugin) Icon() string { return pluginIcon }

// Init initializes the plugin with context.
func (p *Plugin) Init(ctx *plugin.Context) error {
	p.ctx = ctx

	// Load persisted sidebar width
	if savedWidth := state.GetConversationsSideWidth(); savedWidth > 0 {
		p.sidebarWidth = savedWidth
	}

	p.adapters = make(map[string]adapter.Adapter)
	for id, a := range ctx.Adapters {
		found, err := a.Detect(ctx.WorkDir)
		if err != nil || !found {
			continue
		}
		p.adapters[id] = a
	}
	if len(p.adapters) == 0 {
		return nil
	}

	return nil
}

// Start begins plugin operation.
func (p *Plugin) Start() tea.Cmd {
	if len(p.adapters) == 0 {
		return nil
	}

	return tea.Batch(
		p.loadSessions(),
		p.startWatcher(),
	)
}

// Stop cleans up plugin resources.
func (p *Plugin) Stop() {
	// Watcher cleanup handled by adapter
}

// Update handles messages.
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if p.twoPane {
			return p.handleMouse(msg)
		}
		return p, nil

	case tea.KeyMsg:
		switch p.view {
		case ViewMessageDetail:
			return p.updateMessageDetail(msg)
		case ViewMessages:
			// In two-pane mode, route based on active pane
			if p.twoPane && p.activePane == PaneSidebar {
				return p.updateSessions(msg)
			}
			return p.updateMessages(msg)
		case ViewAnalytics:
			return p.updateAnalytics(msg)
		default:
			// In two-pane mode, route based on active pane
			if p.twoPane && p.activePane == PaneMessages {
				return p.updateMessages(msg)
			}
			return p.updateSessions(msg)
		}

	case SessionsLoadedMsg:
		p.sessions = msg.Sessions
		// Keep selection valid when sessions refresh.
		if p.selectedSession != "" {
			found := false
			for i := range p.sessions {
				if p.sessions[i].ID == p.selectedSession {
					found = true
					break
				}
			}
			if !found {
				p.selectedSession = ""
				p.loadedSession = ""
				p.messages = nil
				p.turns = nil
				p.sessionSummary = nil
			}
		}

		// In two-pane mode, ensure a selection so the right pane can render.
		if p.twoPane && p.selectedSession == "" && len(p.sessions) > 0 {
			if p.cursor >= len(p.sessions) {
				p.cursor = len(p.sessions) - 1
			}
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.setSelectedSession(p.sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}
		return p, nil

	case PreviewLoadMsg:
		if msg.Token != p.previewToken {
			return p, nil
		}
		if msg.SessionID == "" || msg.SessionID != p.selectedSession {
			return p, nil
		}
		if p.loadedSession == msg.SessionID && len(p.messages) > 0 {
			return p, nil
		}
		return p, tea.Batch(
			p.loadMessages(msg.SessionID),
			p.loadUsage(msg.SessionID),
		)

	case MessagesLoadedMsg:
		if msg.SessionID == "" || msg.SessionID != p.selectedSession {
			// Ignore out-of-order loads when cursor moves quickly.
			return p, nil
		}
		p.loadedSession = msg.SessionID
		p.messages = msg.Messages
		p.turns = GroupMessagesIntoTurns(msg.Messages)
		p.turnCursor = 0
		p.turnScrollOff = 0
		p.hasMore = len(msg.Messages) >= p.pageSize
		// Compute session summary
		var duration time.Duration
		for _, s := range p.sessions {
			if s.ID == p.selectedSession {
				duration = s.Duration
				break
			}
		}
		summary := ComputeSessionSummary(msg.Messages, duration)
		p.sessionSummary = &summary
		return p, nil

	case WatchStartedMsg:
		// Watcher started, store channel and start listening
		if msg.Channel == nil {
			return p, nil // Watcher failed
		}
		p.watchChan = msg.Channel
		return p, p.listenForWatchEvents()

	case WatchEventMsg:
		// Session data changed, refresh and continue listening
		return p, tea.Batch(
			p.loadSessions(),
			p.listenForWatchEvents(),
		)

	case tea.WindowSizeMsg:
		prevTwoPane := p.twoPane
		p.width = msg.Width
		p.height = msg.Height
		p.twoPane = msg.Width >= 120
		if p.twoPane && (!prevTwoPane || p.selectedSession == "") && len(p.sessions) > 0 {
			if p.cursor >= len(p.sessions) {
				p.cursor = len(p.sessions) - 1
			}
			if p.cursor < 0 {
				p.cursor = 0
			}
			p.setSelectedSession(p.sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}
		return p, nil
	}

	return p, nil
}

func (p *Plugin) setSelectedSession(sessionID string) {
	if sessionID == "" || sessionID == p.selectedSession {
		return
	}
	p.selectedSession = sessionID
	p.loadedSession = ""
	p.messages = nil
	p.turns = nil
	p.turnCursor = 0
	p.turnScrollOff = 0
	p.sessionSummary = nil
	p.showToolSummary = false
	p.detailMode = false
	p.detailTurn = nil
	p.detailScroll = 0
	p.expandedThinking = make(map[string]bool)
}

func (p *Plugin) schedulePreviewLoad(sessionID string) tea.Cmd {
	if sessionID == "" {
		return nil
	}
	p.previewToken++
	token := p.previewToken
	return tea.Tick(previewDebounce, func(time.Time) tea.Msg {
		return PreviewLoadMsg{Token: token, SessionID: sessionID}
	})
}

// updateSessions handles key events in session list view.
func (p *Plugin) updateSessions(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	// Handle search mode input
	if p.searchMode {
		return p.updateSearch(msg)
	}

	// Handle filter mode input
	if p.filterMode {
		return p.updateFilter(msg)
	}

	sessions := p.visibleSessions()

	switch msg.String() {
	case "j", "down":
		if p.cursor < len(sessions)-1 {
			p.cursor++
			p.ensureCursorVisible()
			// In two-pane mode, auto-load messages when cursor moves
			if p.twoPane && p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
			// In two-pane mode, auto-load messages when cursor moves
			if p.twoPane && p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "g":
		p.cursor = 0
		p.scrollOff = 0
		// In two-pane mode, auto-load messages when jumping
		if p.twoPane && len(sessions) > 0 {
			p.setSelectedSession(sessions[0].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "G":
		if len(sessions) > 0 {
			p.cursor = len(sessions) - 1
			p.ensureCursorVisible()
			// In two-pane mode, auto-load messages when jumping
			if p.twoPane {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "ctrl+d":
		// Page down
		pageSize := 10
		if p.cursor+pageSize < len(sessions) {
			p.cursor += pageSize
		} else {
			p.cursor = len(sessions) - 1
		}
		p.ensureCursorVisible()
		if p.twoPane && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "ctrl+u":
		// Page up
		pageSize := 10
		if p.cursor-pageSize >= 0 {
			p.cursor -= pageSize
		} else {
			p.cursor = 0
		}
		p.ensureCursorVisible()
		if p.twoPane && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "tab":
		// In two-pane mode, toggle focus between sidebar and messages
		if p.twoPane && p.selectedSession != "" {
			p.activePane = PaneMessages
		}

	case "l", "right":
		// In two-pane mode, switch focus to messages pane
		if p.twoPane && p.selectedSession != "" {
			p.activePane = PaneMessages
		}

	case "enter":
		if len(sessions) > 0 && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			// In two-pane mode, switch focus to messages pane
			if p.twoPane {
				p.activePane = PaneMessages
				return p, tea.Batch(
					p.loadMessages(p.selectedSession),
					p.loadUsage(p.selectedSession),
				)
			}
			// In single-pane mode, switch view
			p.view = ViewMessages
			p.msgCursor = 0
			p.msgScrollOff = 0
			return p, p.loadMessages(p.selectedSession)
		}

	case "/":
		p.searchMode = true
		p.searchQuery = ""
		p.cursor = 0
		p.scrollOff = 0

	case "f":
		// Open filter menu
		p.filterMode = true

	case "r":
		return p, p.loadSessions()

	case "U":
		// Toggle global analytics view
		p.view = ViewAnalytics
		return p, nil

	case "y":
		// Yank session details to clipboard
		return p, p.yankSessionDetails()

	case "Y":
		// Yank resume command to clipboard
		return p, p.yankResumeCommand()
	}

	return p, nil
}

// updateSearch handles key events in search mode.
func (p *Plugin) updateSearch(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.searchMode = false
		p.searchQuery = ""
		p.searchResults = nil
		p.cursor = 0
		p.scrollOff = 0
		if p.twoPane && len(p.sessions) > 0 {
			p.setSelectedSession(p.sessions[0].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "enter":
		sessions := p.visibleSessions()
		if len(sessions) > 0 && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			if p.twoPane {
				p.activePane = PaneMessages
			} else {
				p.view = ViewMessages
			}
			p.msgCursor = 0
			p.msgScrollOff = 0
			p.searchMode = false
			return p, tea.Batch(
				p.loadMessages(p.selectedSession),
				p.loadUsage(p.selectedSession),
			)
		}

	case "backspace":
		if len(p.searchQuery) > 0 {
			p.searchQuery = p.searchQuery[:len(p.searchQuery)-1]
			p.filterSessions()
			p.cursor = 0
			p.scrollOff = 0
		}

	case "up", "ctrl+p":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
			if p.twoPane {
				sessions := p.visibleSessions()
				if p.cursor < len(sessions) {
					p.setSelectedSession(sessions[p.cursor].ID)
					return p, p.schedulePreviewLoad(p.selectedSession)
				}
			}
		}

	case "down", "ctrl+n", "j":
		sessions := p.visibleSessions()
		if p.cursor < len(sessions)-1 {
			p.cursor++
			p.ensureCursorVisible()
			if p.twoPane && p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "k":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
			if p.twoPane {
				sessions := p.visibleSessions()
				if p.cursor < len(sessions) {
					p.setSelectedSession(sessions[p.cursor].ID)
					return p, p.schedulePreviewLoad(p.selectedSession)
				}
			}
		}

	case "ctrl+d":
		sessions := p.visibleSessions()
		if len(sessions) == 0 {
			return p, nil
		}
		pageSize := 10
		if p.cursor+pageSize < len(sessions) {
			p.cursor += pageSize
		} else {
			p.cursor = len(sessions) - 1
		}
		p.ensureCursorVisible()
		if p.twoPane && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "ctrl+u":
		sessions := p.visibleSessions()
		if len(sessions) == 0 {
			return p, nil
		}
		pageSize := 10
		if p.cursor-pageSize >= 0 {
			p.cursor -= pageSize
		} else {
			p.cursor = 0
		}
		p.ensureCursorVisible()
		if p.twoPane && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "g":
		p.cursor = 0
		p.scrollOff = 0
		if p.twoPane {
			sessions := p.visibleSessions()
			if len(sessions) > 0 {
				p.setSelectedSession(sessions[0].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "G":
		sessions := p.visibleSessions()
		if len(sessions) > 0 {
			p.cursor = len(sessions) - 1
			p.ensureCursorVisible()
			if p.twoPane {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	default:
		// Add character to search query
		if len(msg.String()) == 1 {
			p.searchQuery += msg.String()
			p.filterSessions()
			p.cursor = 0
			p.scrollOff = 0
			if p.twoPane {
				sessions := p.visibleSessions()
				if len(sessions) > 0 {
					p.setSelectedSession(sessions[0].ID)
					return p, p.schedulePreviewLoad(p.selectedSession)
				}
			}
		}
	}

	return p, nil
}

// filterSessions filters sessions based on search query.
func (p *Plugin) filterSessions() {
	if p.searchQuery == "" {
		p.searchResults = nil
		return
	}

	query := strings.ToLower(p.searchQuery)
	var results []adapter.Session
	for _, s := range p.sessions {
		if strings.Contains(strings.ToLower(s.Name), query) ||
			strings.Contains(strings.ToLower(s.Slug), query) ||
			strings.Contains(s.ID, query) ||
			strings.Contains(strings.ToLower(s.AdapterName), query) {
			results = append(results, s)
		}
	}
	p.searchResults = results
}

// visibleSessions returns sessions to display (filtered or all).
func (p *Plugin) visibleSessions() []adapter.Session {
	if p.searchMode && p.searchQuery != "" {
		return p.searchResults
	}

	// Apply filters if active
	if p.filterActive && p.filters.IsActive() {
		var filtered []adapter.Session
		for _, s := range p.sessions {
			if p.filters.Matches(s) {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}

	return p.sessions
}

// updateAnalytics handles key events in analytics view.
func (p *Plugin) updateAnalytics(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	// Calculate max scroll based on content
	maxScroll := len(p.analyticsLines) - (p.height - 2)
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "esc", "q", "U":
		p.view = ViewSessions
		p.analyticsScrollOff = 0

	case "j", "down":
		if p.analyticsScrollOff < maxScroll {
			p.analyticsScrollOff++
		}

	case "k", "up":
		if p.analyticsScrollOff > 0 {
			p.analyticsScrollOff--
		}

	case "g":
		p.analyticsScrollOff = 0

	case "G":
		p.analyticsScrollOff = maxScroll

	case "ctrl+d":
		p.analyticsScrollOff += 10
		if p.analyticsScrollOff > maxScroll {
			p.analyticsScrollOff = maxScroll
		}

	case "ctrl+u":
		p.analyticsScrollOff -= 10
		if p.analyticsScrollOff < 0 {
			p.analyticsScrollOff = 0
		}
	}
	return p, nil
}

// updateMessages handles key events in message view (now uses turns).
func (p *Plugin) updateMessages(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	// In detail mode, handle detail-specific navigation
	if p.detailMode {
		return p.updateDetailMode(msg)
	}

	switch msg.String() {
	case "esc":
		// In two-pane mode, ESC returns focus to sidebar
		if p.twoPane {
			p.activePane = PaneSidebar
			return p, nil
		}
		// In single-pane mode, return to sessions view
		p.view = ViewSessions
		p.messages = nil
		p.turns = nil
		p.selectedSession = ""
		p.expandedThinking = make(map[string]bool) // reset thinking state
		p.sessionSummary = nil
		p.showToolSummary = false

	case "q":
		// In single-pane mode only, 'q' returns to sessions view
		// In two-pane mode, 'q' falls through to app for quit confirmation
		if !p.twoPane {
			p.view = ViewSessions
			p.messages = nil
			p.turns = nil
			p.selectedSession = ""
			p.expandedThinking = make(map[string]bool)
			p.sessionSummary = nil
			p.showToolSummary = false
		}

	case "h", "left":
		// In two-pane mode, return focus to sidebar
		if p.twoPane {
			p.activePane = PaneSidebar
			return p, nil
		}

	case "tab":
		// In two-pane mode, toggle focus between sidebar and messages
		if p.twoPane {
			p.activePane = PaneSidebar
			return p, nil
		}

	case "j", "down":
		if p.turnCursor < len(p.turns)-1 {
			p.turnCursor++
			p.ensureTurnCursorVisible()
		}

	case "k", "up":
		if p.turnCursor > 0 {
			p.turnCursor--
			p.ensureTurnCursorVisible()
		}

	case "g":
		p.turnCursor = 0
		p.turnScrollOff = 0

	case "G":
		if len(p.turns) > 0 {
			p.turnCursor = len(p.turns) - 1
			p.ensureTurnCursorVisible()
		}

	case "ctrl+d":
		pageSize := 10
		if p.turnCursor+pageSize < len(p.turns) {
			p.turnCursor += pageSize
		} else if len(p.turns) > 0 {
			p.turnCursor = len(p.turns) - 1
		}
		p.ensureTurnCursorVisible()

	case "ctrl+u":
		pageSize := 10
		if p.turnCursor-pageSize >= 0 {
			p.turnCursor -= pageSize
		} else {
			p.turnCursor = 0
		}
		p.ensureTurnCursorVisible()

	case "T":
		// Toggle thinking block expansion for current turn's messages
		if p.turnCursor < len(p.turns) {
			turn := &p.turns[p.turnCursor]
			for _, m := range turn.Messages {
				if len(m.ThinkingBlocks) > 0 {
					p.expandedThinking[m.ID] = !p.expandedThinking[m.ID]
				}
			}
		}

	case "t":
		// Toggle tool impact summary
		p.showToolSummary = !p.showToolSummary

	case "enter":
		// Open turn detail view
		if p.turnCursor < len(p.turns) {
			p.detailTurn = &p.turns[p.turnCursor]
			p.detailScroll = 0
			if p.twoPane {
				// In two-pane mode, set detail mode (renders in right pane)
				p.detailMode = true
			} else {
				// In single-pane mode, switch to full-screen detail view
				p.view = ViewMessageDetail
			}
		}

	case "c":
		// Copy session to clipboard as markdown
		if p.selectedSession != "" {
			return p, p.copySessionToClipboard()
		}

	case "e":
		// Export session to file
		if p.selectedSession != "" {
			return p, p.exportSessionToFile()
		}

	case " ":
		// Load more messages (would need to implement paging in adapter)
		return p, nil

	case "y":
		// Yank current turn content to clipboard
		return p, p.yankTurnContent()

	case "Y":
		// Yank resume command to clipboard
		return p, p.yankResumeCommand()
	}

	return p, nil
}

// updateDetailMode handles key events when in detail mode (two-pane).
func (p *Plugin) updateDetailMode(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		// Exit detail mode, back to turn list
		p.detailMode = false
		p.detailTurn = nil
		p.detailScroll = 0

	case "h", "left":
		// In detail mode, h/left goes back to turn list (same as esc)
		p.detailMode = false
		p.detailTurn = nil
		p.detailScroll = 0

	case "j", "down":
		p.detailScroll++

	case "k", "up":
		if p.detailScroll > 0 {
			p.detailScroll--
		}

	case "g":
		p.detailScroll = 0

	case "G":
		// Scroll to bottom - will be clamped by renderer
		p.detailScroll = 9999

	case "ctrl+d":
		p.detailScroll += 10

	case "ctrl+u":
		p.detailScroll -= 10
		if p.detailScroll < 0 {
			p.detailScroll = 0
		}

	case "y":
		// Yank current turn content to clipboard
		return p, p.yankTurnContent()

	case "Y":
		// Yank resume command to clipboard
		return p, p.yankResumeCommand()
	}

	return p, nil
}

// View renders the plugin.
func (p *Plugin) View(width, height int) string {
	p.width = width
	p.height = height

	// Enable two-pane for wide terminals (>= 102 columns)
	p.twoPane = width >= 102
	// Note: sidebarWidth is calculated in renderTwoPane, not here,
	// to avoid resetting drag-adjusted widths on every render

	var content string
	if len(p.adapters) == 0 {
		content = renderNoAdapter()
	} else {
		switch p.view {
		case ViewMessages:
			if p.twoPane {
				content = p.renderTwoPane()
			} else {
				content = p.renderMessages()
			}
		case ViewMessageDetail:
			// In two-pane mode, use renderTwoPane (detailMode handles right pane)
			// In single-pane mode, use full-screen detail view
			if p.twoPane {
				content = p.renderTwoPane()
			} else {
				content = p.renderMessageDetail()
			}
		case ViewAnalytics:
			content = p.renderAnalytics()
		default:
			if p.twoPane {
				content = p.renderTwoPane()
			} else {
				content = p.renderSessions()
			}
		}
	}

	// Constrain output to allocated height to prevent header scrolling off-screen.
	// MaxHeight truncates content that exceeds the allocated space.
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
}

// IsFocused returns whether the plugin is focused.
func (p *Plugin) IsFocused() bool { return p.focused }

// SetFocused sets the focus state.
func (p *Plugin) SetFocused(f bool) { p.focused = f }

// Commands returns the available commands.
func (p *Plugin) Commands() []plugin.Command {
	if p.searchMode {
		return []plugin.Command{
			{ID: "select", Name: "Select", Description: "Select search result", Category: plugin.CategoryActions, Context: "conversations-search", Priority: 1},
			{ID: "cancel", Name: "Cancel", Description: "Cancel search", Category: plugin.CategoryActions, Context: "conversations-search", Priority: 1},
		}
	}
	if p.filterMode {
		return []plugin.Command{
			{ID: "select", Name: "Select", Description: "Apply filter", Category: plugin.CategoryActions, Context: "conversations-filter", Priority: 1},
			{ID: "cancel", Name: "Cancel", Description: "Cancel filter", Category: plugin.CategoryActions, Context: "conversations-filter", Priority: 1},
		}
	}
	// Detail mode in two-pane (right pane shows turn detail)
	if p.detailMode && p.twoPane {
		return []plugin.Command{
			{ID: "back", Name: "Back", Description: "Return to turn list", Category: plugin.CategoryNavigation, Context: "turn-detail", Priority: 1},
			{ID: "scroll", Name: "Scroll", Description: "Scroll detail", Category: plugin.CategoryNavigation, Context: "turn-detail", Priority: 2},
			{ID: "yank", Name: "Yank", Description: "Yank turn content", Category: plugin.CategoryActions, Context: "turn-detail", Priority: 3},
		}
	}
	// Full-screen detail view (single-pane mode)
	if p.view == ViewMessageDetail {
		return []plugin.Command{
			{ID: "back", Name: "Back", Description: "Return to messages", Category: plugin.CategoryNavigation, Context: "message-detail", Priority: 1},
			{ID: "scroll", Name: "Scroll", Description: "Scroll message", Category: plugin.CategoryNavigation, Context: "message-detail", Priority: 2},
			{ID: "yank", Name: "Yank", Description: "Yank turn content", Category: plugin.CategoryActions, Context: "message-detail", Priority: 3},
			{ID: "yank-resume", Name: "Resume", Description: "Yank resume command", Category: plugin.CategoryActions, Context: "message-detail", Priority: 3},
		}
	}
	if p.view == ViewMessages || (p.twoPane && p.activePane == PaneMessages) {
		return []plugin.Command{
			{ID: "detail", Name: "Detail", Description: "View turn details", Category: plugin.CategoryView, Context: "conversations-main", Priority: 1},
			{ID: "back", Name: "Back", Description: "Return to sidebar", Category: plugin.CategoryNavigation, Context: "conversations-main", Priority: 2},
			{ID: "yank", Name: "Yank", Description: "Yank turn content", Category: plugin.CategoryActions, Context: "conversations-main", Priority: 3},
		}
	}
	if p.view == ViewAnalytics {
		return []plugin.Command{
			{ID: "back", Name: "Back", Description: "Return to conversations", Category: plugin.CategoryNavigation, Context: "analytics", Priority: 1},
		}
	}
	return []plugin.Command{
		{ID: "view-session", Name: "View", Description: "View session messages", Category: plugin.CategoryView, Context: "conversations", Priority: 1},
		{ID: "search", Name: "Search", Description: "Search conversations", Category: plugin.CategorySearch, Context: "conversations", Priority: 2},
		{ID: "filter", Name: "Filter", Description: "Filter by project", Category: plugin.CategorySearch, Context: "conversations", Priority: 2},
		{ID: "yank", Name: "Yank", Description: "Yank session details", Category: plugin.CategoryActions, Context: "conversations", Priority: 3},
		{ID: "yank-resume", Name: "Resume", Description: "Yank resume command", Category: plugin.CategoryActions, Context: "conversations", Priority: 3},
	}
}

// FocusContext returns the current focus context.
func (p *Plugin) FocusContext() string {
	if p.searchMode {
		return "conversations-search"
	}
	if p.filterMode {
		return "conversations-filter"
	}
	// Detail mode in two-pane (right pane shows turn detail)
	if p.detailMode && p.twoPane {
		return "turn-detail"
	}
	switch p.view {
	case ViewMessageDetail:
		return "message-detail"
	case ViewMessages:
		// In two-pane mode, return context based on active pane
		// so 'q' triggers quit from root contexts (sidebar/main)
		if p.twoPane {
			if p.activePane == PaneSidebar {
				return "conversations-sidebar"
			}
			return "conversations-main"
		}
		return "conversation-detail"
	case ViewAnalytics:
		return "analytics"
	default:
		if p.twoPane {
			if p.activePane == PaneSidebar {
				return "conversations-sidebar"
			}
			return "conversations-main"
		}
		return "conversations"
	}
}

// Diagnostics returns plugin health info.
func (p *Plugin) Diagnostics() []plugin.Diagnostic {
	status := "ok"
	detail := ""
	if len(p.adapters) == 0 {
		status = "disabled"
		detail = "no adapters"
	} else if len(p.sessions) == 0 {
		status = "empty"
		detail = "no sessions"
	} else {
		detail = formatSessionCount(len(p.sessions))
		// Add active session count
		active := 0
		for _, s := range p.sessions {
			if s.IsActive {
				active++
			}
		}
		if active > 0 {
			detail = fmt.Sprintf("%s (%d active)", detail, active)
		}
	}

	// Add watcher status
	watchStatus := "off"
	if p.watchChan != nil {
		watchStatus = "on"
	}

	return []plugin.Diagnostic{
		{ID: "conversations", Status: status, Detail: detail},
		{ID: "watcher", Status: watchStatus, Detail: "fsnotify"},
	}
}

// loadSessions loads sessions from the adapter.
func (p *Plugin) loadSessions() tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return SessionsLoadedMsg{}
		}

		var sessions []adapter.Session
		for id, a := range p.adapters {
			adapterSessions, err := a.Sessions(p.ctx.WorkDir)
			if err != nil {
				continue
			}
			for i := range adapterSessions {
				if adapterSessions[i].AdapterID == "" {
					adapterSessions[i].AdapterID = id
				}
				if adapterSessions[i].AdapterName == "" {
					adapterSessions[i].AdapterName = a.Name()
				}
				if adapterSessions[i].AdapterIcon == "" {
					adapterSessions[i].AdapterIcon = a.Icon()
				}
			}
			sessions = append(sessions, adapterSessions...)
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
		})

		return SessionsLoadedMsg{Sessions: sessions}
	}
}

// loadMessages loads messages for a session.
func (p *Plugin) loadMessages(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return MessagesLoadedMsg{}
		}
		adapter := p.adapterForSession(sessionID)
		if adapter == nil {
			return MessagesLoadedMsg{}
		}
		messages, err := adapter.Messages(sessionID)
		if err != nil {
			return ErrorMsg{Err: err}
		}

		// Limit to last N messages
		if len(messages) > maxMessagesInMemory {
			messages = messages[len(messages)-maxMessagesInMemory:]
		}

		return MessagesLoadedMsg{SessionID: sessionID, Messages: messages}
	}
}

// startWatcher starts watching for session changes.
func (p *Plugin) startWatcher() tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return WatchStartedMsg{Channel: nil}
		}

		merged := make(chan adapter.Event, 32)
		watchCount := 0
		for _, a := range p.adapters {
			ch, err := a.Watch(p.ctx.WorkDir)
			if err != nil || ch == nil {
				continue
			}
			watchCount++
			go func(c <-chan adapter.Event) {
				for evt := range c {
					select {
					case merged <- evt:
					default:
					}
				}
			}(ch)
		}
		if watchCount == 0 {
			return WatchStartedMsg{Channel: nil}
		}
		return WatchStartedMsg{Channel: merged}
	}
}

// listenForWatchEvents waits for the next file system event.
func (p *Plugin) listenForWatchEvents() tea.Cmd {
	if p.watchChan == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-p.watchChan
		if !ok {
			// Channel closed
			return nil
		}
		return WatchEventMsg{}
	}
}

// ensureCursorVisible adjusts scroll to keep cursor visible.
func (p *Plugin) ensureCursorVisible() {
	var visibleRows int
	if p.twoPane {
		// In two-pane mode: pane height - borders(2) - header(1-2)
		paneHeight := p.height - 2
		visibleRows = paneHeight - 3 // -2 for inner height calc, -1 for header
	} else {
		// Single pane: total height - header lines
		visibleRows = p.height - 4
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	sessions := p.visibleSessions()

	// When not in search mode and we have sessions, account for group headers
	if !p.searchMode && len(sessions) > 0 {
		// Calculate visual lines between scrollOff and cursor (including headers)
		headerLines := p.countHeaderLinesBetween(p.scrollOff, p.cursor)
		visualOffset := (p.cursor - p.scrollOff) + headerLines

		if p.cursor < p.scrollOff {
			p.scrollOff = p.cursor
		} else if visualOffset >= visibleRows {
			// Need to scroll - find scrollOff that puts cursor at bottom
			p.scrollOff = p.findScrollOffForCursor(p.cursor, visibleRows)
		}
	} else {
		// Search mode or no sessions: flat list, no headers
		if p.cursor < p.scrollOff {
			p.scrollOff = p.cursor
		} else if p.cursor >= p.scrollOff+visibleRows {
			p.scrollOff = p.cursor - visibleRows + 1
		}
	}
}

// countHeaderLinesBetween counts header lines (group headers + spacers) between two session indices.
func (p *Plugin) countHeaderLinesBetween(start, end int) int {
	if start >= end {
		return 0
	}
	sessions := p.visibleSessions()
	if len(sessions) == 0 {
		return 0
	}

	headerLines := 0
	currentGroup := ""
	if start > 0 && start < len(sessions) {
		currentGroup = getSessionGroup(sessions[start].UpdatedAt)
	}

	for i := start; i <= end && i < len(sessions); i++ {
		sessionGroup := getSessionGroup(sessions[i].UpdatedAt)
		if sessionGroup != currentGroup {
			// Group header line
			headerLines++
			// Spacer line for Yesterday/This Week (except first visible)
			if currentGroup != "" && (sessionGroup == "Yesterday" || sessionGroup == "This Week") {
				headerLines++
			}
			currentGroup = sessionGroup
		}
	}
	return headerLines
}

// findScrollOffForCursor finds the scrollOff that puts cursor at the bottom of visible area.
func (p *Plugin) findScrollOffForCursor(cursor, visibleRows int) int {
	sessions := p.visibleSessions()
	if len(sessions) == 0 {
		return 0
	}

	// Binary search or iterate backwards to find best scrollOff
	for scrollOff := cursor; scrollOff >= 0; scrollOff-- {
		headerLines := p.countHeaderLinesBetween(scrollOff, cursor)
		visualOffset := (cursor - scrollOff) + headerLines
		if visualOffset < visibleRows {
			return scrollOff
		}
	}
	return 0
}

// ensureMsgCursorVisible adjusts scroll to keep message cursor visible.
func (p *Plugin) ensureMsgCursorVisible() {
	var visibleRows int
	if p.twoPane {
		// In two-pane mode: pane height - borders(2) - header(4-5)
		paneHeight := p.height - 2
		visibleRows = paneHeight - 6 // Account for header, stats, resume cmd, separator
	} else {
		// Single pane: total height - header lines
		visibleRows = p.height - 4
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	if p.msgCursor < p.msgScrollOff {
		p.msgScrollOff = p.msgCursor
	} else if p.msgCursor >= p.msgScrollOff+visibleRows {
		p.msgScrollOff = p.msgCursor - visibleRows + 1
	}
}

// ensureTurnCursorVisible adjusts scroll to keep turn cursor visible.
func (p *Plugin) ensureTurnCursorVisible() {
	var visibleRows int
	if p.twoPane {
		// In two-pane mode: pane height - borders(2) - header(4-5)
		paneHeight := p.height - 2
		visibleRows = paneHeight - 6 // Account for header, stats, resume cmd, separator
	} else {
		// Single pane: total height - header lines
		visibleRows = p.height - 4
	}
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Each turn takes ~3 lines (header + content/thinking/tools)
	// so divide by 3 to get approximate visible turns
	visibleTurns := visibleRows / 3
	if visibleTurns < 1 {
		visibleTurns = 1
	}

	if p.turnCursor < p.turnScrollOff {
		p.turnScrollOff = p.turnCursor
	} else if p.turnCursor >= p.turnScrollOff+visibleTurns {
		p.turnScrollOff = p.turnCursor - visibleTurns + 1
	}
}

// formatSessionCount formats a session count.
func formatSessionCount(n int) string {
	if n == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

// shortID returns the first 8 characters of an ID, or the full ID if shorter.
func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// loadUsage loads usage stats for a session (placeholder for future implementation).
func (p *Plugin) loadUsage(sessionID string) tea.Cmd {
	// Usage is already computed from messages in MessagesLoadedMsg handler
	return nil
}

// updateMessageDetail handles key events in the message detail view.
func (p *Plugin) updateMessageDetail(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		p.view = ViewMessages
		p.detailTurn = nil
		p.detailScroll = 0

	case "j", "down":
		p.detailScroll++

	case "k", "up":
		if p.detailScroll > 0 {
			p.detailScroll--
		}

	case "g":
		p.detailScroll = 0

	case "G":
		// Scroll to bottom - calculate based on content
		p.detailScroll = 100 // Placeholder, will be clamped by renderer

	case "ctrl+d":
		p.detailScroll += 10

	case "ctrl+u":
		p.detailScroll -= 10
		if p.detailScroll < 0 {
			p.detailScroll = 0
		}

	case "y":
		// Yank current turn content to clipboard
		return p, p.yankTurnContent()

	case "Y":
		// Yank resume command to clipboard
		return p, p.yankResumeCommand()
	}
	return p, nil
}

// updateFilter handles key events in filter mode.
func (p *Plugin) updateFilter(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()
	for _, opt := range adapterFilterOptions(p.adapters) {
		if key == opt.key {
			p.filters.ToggleAdapter(opt.id)
			return p, nil
		}
	}

	switch key {
	case "esc":
		p.filterMode = false

	case "enter":
		p.filterMode = false
		p.filterActive = p.filters.IsActive()
		p.cursor = 0
		p.scrollOff = 0

	case "1":
		// Toggle model filter: opus
		p.filters.ToggleModel("opus")

	case "2":
		// Toggle model filter: sonnet
		p.filters.ToggleModel("sonnet")

	case "3":
		// Toggle model filter: haiku
		p.filters.ToggleModel("haiku")

	case "t":
		// Toggle date filter: today
		p.filters.SetDateRange("today")

	case "y":
		// Toggle date filter: yesterday
		p.filters.SetDateRange("yesterday")

	case "w":
		// Toggle date filter: week
		p.filters.SetDateRange("week")

	case "a":
		// Toggle active only
		p.filters.ActiveOnly = !p.filters.ActiveOnly

	case "x":
		// Clear all filters
		p.filters = SearchFilters{}
	}
	return p, nil
}

// copySessionToClipboard copies the current session as markdown to clipboard.
func (p *Plugin) copySessionToClipboard() tea.Cmd {
	session := p.findSelectedSession()
	messages := p.messages

	return func() tea.Msg {
		md := ExportSessionAsMarkdown(session, messages)
		if err := CopyToClipboard(md); err != nil {
			return app.ToastMsg{Message: "Copy failed: " + err.Error(), Duration: 2 * time.Second, IsError: true}
		}
		return app.ToastMsg{Message: "Session copied to clipboard", Duration: 2 * time.Second}
	}
}

// exportSessionToFile exports the current session to a markdown file.
func (p *Plugin) exportSessionToFile() tea.Cmd {
	session := p.findSelectedSession()
	messages := p.messages
	workDir := p.ctx.WorkDir

	return func() tea.Msg {
		filename, err := ExportSessionToFile(session, messages, workDir)
		if err != nil {
			return app.ToastMsg{Message: "Export failed: " + err.Error(), Duration: 2 * time.Second, IsError: true}
		}
		return app.ToastMsg{Message: "Exported to " + filename, Duration: 2 * time.Second}
	}
}

// findSelectedSession returns the currently selected session.
func (p *Plugin) findSelectedSession() *adapter.Session {
	for i := range p.sessions {
		if p.sessions[i].ID == p.selectedSession {
			return &p.sessions[i]
		}
	}
	return nil
}

func (p *Plugin) adapterForSession(sessionID string) adapter.Adapter {
	for i := range p.sessions {
		if p.sessions[i].ID == sessionID {
			if p.sessions[i].AdapterID == "" {
				return nil
			}
			return p.adapters[p.sessions[i].AdapterID]
		}
	}
	return nil
}

// Message types
type SessionsLoadedMsg struct {
	Sessions []adapter.Session
}

type MessagesLoadedMsg struct {
	SessionID string
	Messages []adapter.Message
}

type WatchEventMsg struct{}
type WatchStartedMsg struct {
	Channel <-chan adapter.Event
}
type ErrorMsg struct{ Err error }

type PreviewLoadMsg struct {
	Token     int
	SessionID string
}

// TickCmd returns a command that triggers periodic refresh.
func TickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return WatchEventMsg{}
	})
}
