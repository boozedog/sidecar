package conversations

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

	previewDebounce     = 150 * time.Millisecond
	watchReloadDebounce = 200 * time.Millisecond

	// Divider width for pane separator
	dividerWidth = 1

	// Hybrid content display thresholds
	ShortMessageCharLimit = 500 // Messages shorter than this display inline
	ShortMessageLineLimit = 13  // Messages with fewer lines display inline
	CollapsedPreviewChars = 300 // Preview length for collapsed messages
)

// Mouse hit region identifiers
const (
	regionSidebar     = "sidebar"
	regionMainPane    = "main-pane"
	regionPaneDivider = "pane-divider"
	regionSessionItem = "session-item" // Individual session row (Data: session index)
	regionTurnItem    = "turn-item"    // Individual turn row (Data: turn index)
	regionMessageItem = "message-item" // Conversation flow: click to select (Data: msg index)
	regionToolExpand  = "tool-expand"  // Conversation flow: toggle tool output (Data: tool_use_id)
	regionShowMore    = "show-more"    // Conversation flow: expand long message (Data: msg ID)
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

// renderCacheKey is used to cache rendered message content (td-8910b218).
type renderCacheKey struct {
	messageID string
	width     int
	expanded  bool // whether content is expanded (affects render)
}

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
	selectedSession string
	loadedSession   string // sessionID that p.messages currently represent
	messages        []adapter.Message
	turns           []Turn // messages grouped into turns
	turnCursor      int    // cursor for turn selection in list view
	turnScrollOff   int    // scroll offset for turns
	msgCursor       int
	msgScrollOff    int
	pageSize        int
	hasMore         bool

	// Pagination state (td-313ea851)
	messageOffset      int             // Start index in full message list (0 = most recent)
	totalMessages      int             // Total message count from adapter
	hasOlderMsgs       bool            // True if there are older messages to load
	expandedThinking   map[string]bool // message ID -> thinking expanded
	sessionSummary     *SessionSummary // computed summary for current session
	summaryModelCounts map[string]int  // model usage counts for incremental summary updates
	summaryFileSet     map[string]bool // unique files for incremental summary updates
	showToolSummary    bool            // toggle for tool impact view
	turnViewMode       bool            // false = conversation flow (default), true = turn view

	// Message detail view state
	detailMode   bool  // true when showing detail in right pane (two-pane mode)
	detailTurn   *Turn // turn being viewed in detail
	detailScroll int

	// Analytics view state
	analyticsScrollOff int
	analyticsLines     []string // pre-rendered lines for scrolling

	// Layout state
	activePane     FocusPane // Which pane is focused
	sidebarRestore FocusPane // Tracks pane focused before collapse; restored on expand via toggleSidebar()
	sidebarWidth   int       // Calculated width (~30%)
	sidebarVisible bool      // Toggle sidebar visibility with \
	previewToken       int       // monotonically increasing token for debounced preview loads
	messageReloadToken int       // monotonically increasing token for debounced watch reloads

	// View dimensions
	width  int
	height int

	// Watcher channel
	watchChan <-chan adapter.Event

	// Event coalescing for watch events
	coalescer    *EventCoalescer
	coalesceChan chan CoalescedRefreshMsg

	// Search state
	searchMode    bool
	searchQuery   string
	searchResults []adapter.Session

	// Filter state
	filterMode   bool
	filters      SearchFilters
	filterActive bool // true when any filter is active

	// Markdown rendering
	contentRenderer *GlamourRenderer

	// Conversation flow view state (Claude Code web UI style)
	expandedMessages    map[string]bool // message ID -> content expanded (for long messages)
	expandedToolResults map[string]bool // tool_use_id -> result expanded
	messageScroll       int             // global scroll offset for conversation view
	messageCursor       int             // selected message index in conversation view

	// Visible message line tracking (populated during render for accurate hit regions)
	visibleMsgRanges []msgLineRange // message index -> visible line range (populated each render)

	// Full message line positions (all rendered messages, before scroll window)
	// Used for accurate scroll calculations in ensureMessageCursorVisible
	msgLinePositions []msgLinePos

	// Render cache for message content (td-8910b218)
	renderCache      map[renderCacheKey]string
	renderCacheMutex sync.RWMutex

	// Hit region optimization (td-ea784b03)
	hitRegionsDirty bool
	prevWidth       int
	prevHeight      int
	prevScrollOff   int
	prevMsgScroll   int
	prevTurnScroll  int
}

// msgLineRange tracks which screen lines a message occupies (after scroll).
type msgLineRange struct {
	MsgIdx    int // index in p.messages
	StartLine int // first visible line (relative to content area)
	LineCount int // number of visible lines
}

// msgLinePos tracks actual line position for each rendered message (before scroll).
type msgLinePos struct {
	MsgIdx    int // index in p.messages
	StartLine int // starting line in full content (0 = first line)
	LineCount int // number of lines this message takes
}

// New creates a new conversations plugin.
func New() *Plugin {
	renderer, err := NewGlamourRenderer()
	if err != nil {
		log.Printf("warn: glamour init failed: %v", err)
	}

	coalesceChan := make(chan CoalescedRefreshMsg, 8)
	p := &Plugin{
		pageSize:            defaultPageSize,
		expandedThinking:    make(map[string]bool),
		expandedMessages:    make(map[string]bool),
		expandedToolResults: make(map[string]bool),
		mouseHandler:        mouse.NewHandler(),
		contentRenderer:     renderer,
		coalesceChan:        coalesceChan,
		renderCache:         make(map[renderCacheKey]string),
		hitRegionsDirty:     true, // Start dirty to ensure first render builds regions
		sidebarVisible:      true, // Sidebar visible by default
		sidebarRestore:      PaneSidebar,
	}
	p.coalescer = NewEventCoalescer(0, coalesceChan)
	return p
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return pluginID }

// Name returns the plugin display name.
func (p *Plugin) Name() string { return pluginName }

// Icon returns the plugin icon character.
func (p *Plugin) Icon() string { return pluginIcon }

// renderContent renders markdown content to styled lines, falling back to plain text.
func (p *Plugin) renderContent(content string, width int) []string {
	if p.contentRenderer != nil {
		return p.contentRenderer.RenderContent(content, width)
	}
	return wrapText(content, width)
}

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
		p.listenForCoalescedRefresh(),
	)
}

// Stop cleans up plugin resources.
func (p *Plugin) Stop() {
	// Stop event coalescer
	if p.coalescer != nil {
		p.coalescer.Stop()
	}
	// Watcher cleanup handled by adapter
}

// Update handles messages.
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return p.handleMouse(msg)

	case tea.KeyMsg:
		switch p.view {
		case ViewAnalytics:
			return p.updateAnalytics(msg)
		default:
			// Route based on active pane
			if p.activePane == PaneMessages {
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

		// Ensure a selection so the right pane can render.
		if p.selectedSession == "" && len(p.sessions) > 0 {
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

	case MessageReloadMsg:
		if msg.Token != p.messageReloadToken {
			return p, nil
		}
		if msg.SessionID == "" || msg.SessionID != p.selectedSession {
			return p, nil
		}
		return p, p.loadMessages(msg.SessionID)

	case MessagesLoadedMsg:
		if msg.SessionID == "" || msg.SessionID != p.selectedSession {
			// Ignore out-of-order loads when cursor moves quickly.
			return p, nil
		}

		// Check if this is an incremental update (same session, more messages)
		isIncremental := p.loadedSession == msg.SessionID &&
			len(p.messages) > 0 &&
			len(msg.Messages) >= len(p.messages) &&
			p.messagesMatch(p.messages, msg.Messages[:len(p.messages)])

		if isIncremental && len(msg.Messages) == len(p.messages) {
			// No new messages, skip re-processing entirely
			return p, nil
		}

		// Get session duration for summary
		var duration time.Duration
		for _, s := range p.sessions {
			if s.ID == p.selectedSession {
				duration = s.Duration
				break
			}
		}

		if isIncremental {
			// Incremental update: only process new messages
			oldLen := len(p.messages)
			newMessages := msg.Messages[oldLen:]
			p.messages = msg.Messages

			// Incrementally update turns (handles extending last turn if same role)
			p.turns = AppendMessagesToTurns(p.turns, newMessages, oldLen)

			// Incrementally update summary
			if p.sessionSummary != nil {
				UpdateSessionSummary(p.sessionSummary, newMessages, p.summaryModelCounts, p.summaryFileSet)
			}
			// Mark hit regions dirty for new content (td-ea784b03)
			p.hitRegionsDirty = true
			// Don't reset cursors - user may be scrolled
		} else {
			// Full reload: different session or messages don't match
			p.loadedSession = msg.SessionID
			p.messages = msg.Messages
			p.turns = GroupMessagesIntoTurns(msg.Messages)
			p.turnCursor = 0
			p.turnScrollOff = 0
			// Snap messageCursor to first visible message (skip tool-result-only)
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				p.messageCursor = visibleIndices[0]
			}

			// Full summary computation - also initialize tracking maps for future incremental updates
			summary := ComputeSessionSummary(msg.Messages, duration)
			p.sessionSummary = &summary
			p.summaryModelCounts = make(map[string]int)
			p.summaryFileSet = make(map[string]bool)
			for _, m := range msg.Messages {
				if m.Model != "" {
					p.summaryModelCounts[m.Model]++
				}
				for _, tu := range m.ToolUses {
					if fp := extractFilePath(tu.Input); fp != "" {
						p.summaryFileSet[fp] = true
					}
				}
			}
			// Mark hit regions dirty for new content (td-ea784b03)
			p.hitRegionsDirty = true
		}

		p.hasMore = len(msg.Messages) >= p.pageSize

		// Update pagination state (td-313ea851)
		p.totalMessages = msg.TotalCount
		p.messageOffset = msg.Offset // Sync offset with actual loaded offset (td-39018be2)
		// hasOlderMsgs: true when there are messages beyond the current window (td-07fc795d)
		p.hasOlderMsgs = (msg.Offset + len(msg.Messages)) < msg.TotalCount

		return p, nil

	case WatchStartedMsg:
		// Watcher started, store channel and start listening
		if msg.Channel == nil {
			return p, nil // Watcher failed
		}
		p.watchChan = msg.Channel
		return p, p.listenForWatchEvents()

	case WatchEventMsg:
		// Queue event for coalescing instead of immediate reload
		p.coalescer.Add(msg.SessionID)

		cmds := []tea.Cmd{
			p.listenForWatchEvents(),
		}

		// Still reload messages immediately if selected session changed
		// (coalescer handles session list refresh)
		if msg.SessionID != "" && msg.SessionID == p.selectedSession {
			cmds = append(cmds, p.scheduleMessageReload(p.selectedSession))
		}

		return p, tea.Batch(cmds...)

	case CoalescedRefreshMsg:
		// Coalesced watch events - batch refresh
		cmds := []tea.Cmd{
			p.listenForCoalescedRefresh(), // Continue listening for more batches
		}

		if msg.RefreshAll || len(msg.SessionIDs) == 0 {
			// Full refresh needed
			cmds = append(cmds, p.loadSessions())
		} else {
			// Phase 1: still do full refresh (Phase 2 will add targeted refresh)
			cmds = append(cmds, p.loadSessions())
		}

		return p, tea.Batch(cmds...)

	case OpenSessionReturnedMsg:
		// After returning from external session, re-enable mouse and refresh
		cmds := []tea.Cmd{
			func() tea.Msg { return tea.EnableMouseAllMotion() },
			p.loadSessions(), // Refresh sessions in case something changed
		}
		if p.selectedSession != "" {
			cmds = append(cmds, p.loadMessages(p.selectedSession))
		}
		if msg.Err != nil {
			cmds = append(cmds, func() tea.Msg {
				return app.ToastMsg{Message: "Session exited: " + msg.Err.Error(), Duration: 2 * time.Second, IsError: true}
			})
		}
		return p, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		// Ensure a session is selected so the right pane can render
		if p.selectedSession == "" && len(p.sessions) > 0 {
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
	// Reset conversation flow view state
	p.expandedMessages = make(map[string]bool)
	p.expandedToolResults = make(map[string]bool)
	p.messageScroll = 0
	p.messageCursor = 0
	p.turnViewMode = false // Start in conversation flow mode
	// Reset pagination state (td-313ea851)
	p.messageOffset = 0
	p.totalMessages = 0
	p.hasOlderMsgs = false
	// Clear render cache on session change (td-8910b218)
	p.clearRenderCache()
	// Mark hit regions dirty (td-ea784b03)
	p.hitRegionsDirty = true
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

func (p *Plugin) scheduleMessageReload(sessionID string) tea.Cmd {
	if sessionID == "" {
		return nil
	}
	p.messageReloadToken++
	token := p.messageReloadToken
	return tea.Tick(watchReloadDebounce, func(time.Time) tea.Msg {
		return MessageReloadMsg{Token: token, SessionID: sessionID}
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
			if p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
			if p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "g":
		p.cursor = 0
		p.scrollOff = 0
		if len(sessions) > 0 {
			p.setSelectedSession(sessions[0].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "G":
		if len(sessions) > 0 {
			p.cursor = len(sessions) - 1
			p.ensureCursorVisible()
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
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
		if p.cursor < len(sessions) {
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
		if p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "tab", "shift+tab":
		// Switch focus to messages pane (if sidebar visible)
		if p.selectedSession != "" && p.sidebarVisible {
			p.activePane = PaneMessages
		}

	case "\\":
		// Toggle sidebar visibility
		p.toggleSidebar()

	case "l", "right":
		// Switch focus to messages pane
		if p.selectedSession != "" {
			p.activePane = PaneMessages
		}

	case "enter":
		if len(sessions) > 0 && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			p.activePane = PaneMessages
			return p, tea.Batch(
				p.loadMessages(p.selectedSession),
				p.loadUsage(p.selectedSession),
			)
		}

	case "/":
		p.searchMode = true
		p.searchQuery = ""
		p.cursor = 0
		p.scrollOff = 0

	case "f":
		// Open filter menu
		p.filterMode = true
		p.hitRegionsDirty = true // Filter menu replaces session list (td-455e378b)

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

	case "o":
		// Open session in native CLI tool
		return p, p.openSessionInCLI()
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
		if len(p.sessions) > 0 {
			p.setSelectedSession(p.sessions[0].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "enter":
		sessions := p.visibleSessions()
		if len(sessions) > 0 && p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			p.activePane = PaneMessages
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
			sessions := p.visibleSessions()
			if p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "down", "ctrl+n", "j":
		sessions := p.visibleSessions()
		if p.cursor < len(sessions)-1 {
			p.cursor++
			p.ensureCursorVisible()
			if p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
			}
		}

	case "k":
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisible()
			sessions := p.visibleSessions()
			if p.cursor < len(sessions) {
				p.setSelectedSession(sessions[p.cursor].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
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
		if p.cursor < len(sessions) {
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
		if p.cursor < len(sessions) {
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "g":
		p.cursor = 0
		p.scrollOff = 0
		sessions := p.visibleSessions()
		if len(sessions) > 0 {
			p.setSelectedSession(sessions[0].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	case "G":
		sessions := p.visibleSessions()
		if len(sessions) > 0 {
			p.cursor = len(sessions) - 1
			p.ensureCursorVisible()
			p.setSelectedSession(sessions[p.cursor].ID)
			return p, p.schedulePreviewLoad(p.selectedSession)
		}

	default:
		// Add character to search query
		if len(msg.String()) == 1 {
			p.searchQuery += msg.String()
			p.filterSessions()
			p.cursor = 0
			p.scrollOff = 0
			sessions := p.visibleSessions()
			if len(sessions) > 0 {
				p.setSelectedSession(sessions[0].ID)
				return p, p.schedulePreviewLoad(p.selectedSession)
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
		// ESC returns focus to sidebar
		p.activePane = PaneSidebar
		return p, nil

	case "h", "left":
		// Return focus to sidebar
		p.activePane = PaneSidebar
		return p, nil

	case "tab", "shift+tab":
		// Switch focus to sidebar (if visible)
		if p.sidebarVisible {
			p.activePane = PaneSidebar
		}
		return p, nil

	case "\\":
		// Toggle sidebar visibility
		p.toggleSidebar()
		return p, nil

	case "j", "down":
		if p.turnViewMode {
			// Turn view navigation
			if p.turnCursor < len(p.turns)-1 {
				p.turnCursor++
				p.ensureTurnCursorVisible()
			}
		} else {
			// Conversation flow cursor navigation
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				// Find current position and move to next
				found := false
				for i, idx := range visibleIndices {
					if idx == p.messageCursor {
						found = true
						if i < len(visibleIndices)-1 {
							p.messageCursor = visibleIndices[i+1]
							p.ensureMessageCursorVisible()
						}
						break
					}
				}
				// If cursor wasn't found (edge case), snap to first visible message
				if !found && len(visibleIndices) > 0 {
					p.messageCursor = visibleIndices[0]
					p.ensureMessageCursorVisible()
				}
			}
		}

	case "k", "up":
		if p.turnViewMode {
			if p.turnCursor > 0 {
				p.turnCursor--
				p.ensureTurnCursorVisible()
			}
		} else {
			// Conversation flow cursor navigation
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				// Find current position and move to previous
				found := false
				for i, idx := range visibleIndices {
					if idx == p.messageCursor {
						found = true
						if i > 0 {
							p.messageCursor = visibleIndices[i-1]
							p.ensureMessageCursorVisible()
						}
						break
					}
				}
				// If cursor wasn't found (edge case), snap to first visible message
				if !found && len(visibleIndices) > 0 {
					p.messageCursor = visibleIndices[0]
					p.ensureMessageCursorVisible()
				}
			}
		}

	case "g":
		if p.turnViewMode {
			p.turnCursor = 0
			p.turnScrollOff = 0
		} else {
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				p.messageCursor = visibleIndices[0]
				p.messageScroll = 0
			}
		}

	case "G":
		if p.turnViewMode {
			if len(p.turns) > 0 {
				p.turnCursor = len(p.turns) - 1
				p.ensureTurnCursorVisible()
			}
		} else {
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				p.messageCursor = visibleIndices[len(visibleIndices)-1]
				p.messageScroll = 999999 // Will be clamped in renderer
			}
		}

	case "ctrl+d":
		pageSize := 10
		if p.turnViewMode {
			if p.turnCursor+pageSize < len(p.turns) {
				p.turnCursor += pageSize
			} else if len(p.turns) > 0 {
				p.turnCursor = len(p.turns) - 1
			}
			p.ensureTurnCursorVisible()
		} else {
			// Page down in conversation flow - move cursor by pageSize messages
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				currentPos := 0
				for i, idx := range visibleIndices {
					if idx == p.messageCursor {
						currentPos = i
						break
					}
				}
				newPos := currentPos + pageSize
				if newPos >= len(visibleIndices) {
					newPos = len(visibleIndices) - 1
				}
				p.messageCursor = visibleIndices[newPos]
				p.ensureMessageCursorVisible()
			}
		}

	case "ctrl+u":
		pageSize := 10
		if p.turnViewMode {
			if p.turnCursor-pageSize >= 0 {
				p.turnCursor -= pageSize
			} else {
				p.turnCursor = 0
			}
			p.ensureTurnCursorVisible()
		} else {
			// Page up in conversation flow - move cursor by pageSize messages
			visibleIndices := p.visibleMessageIndices()
			if len(visibleIndices) > 0 {
				currentPos := 0
				for i, idx := range visibleIndices {
					if idx == p.messageCursor {
						currentPos = i
						break
					}
				}
				newPos := currentPos - pageSize
				if newPos < 0 {
					newPos = 0
				}
				p.messageCursor = visibleIndices[newPos]
				p.ensureMessageCursorVisible()
			}
		}

	case "t":
		// Toggle tool impact summary
		p.showToolSummary = !p.showToolSummary

	case "v":
		// Toggle between conversation flow and turn view
		p.turnViewMode = !p.turnViewMode
		p.hitRegionsDirty = true // Different hit regions per view mode (td-455e378b)
		return p, nil

	case "p":
		// Load older messages (td-313ea851)
		if p.hasOlderMsgs && p.totalMessages > maxMessagesInMemory {
			p.messageOffset += maxMessagesInMemory / 2 // Load half a page older
			if p.messageOffset > p.totalMessages-maxMessagesInMemory {
				p.messageOffset = p.totalMessages - maxMessagesInMemory
			}
			return p, p.loadMessages(p.selectedSession)
		}

	case "n":
		// Load newer messages (td-313ea851)
		if p.messageOffset > 0 {
			p.messageOffset -= maxMessagesInMemory / 2 // Load half a page newer
			if p.messageOffset < 0 {
				p.messageOffset = 0
			}
			return p, p.loadMessages(p.selectedSession)
		}

	case "e":
		// Toggle expand for selected message (content, tools, and thinking)
		if p.turnViewMode {
			if p.turnCursor < len(p.turns) {
				turn := &p.turns[p.turnCursor]
				for _, msg := range turn.Messages {
					// Toggle message content
					p.expandedMessages[msg.ID] = !p.expandedMessages[msg.ID]
					// Toggle thinking
					if len(msg.ThinkingBlocks) > 0 {
						p.expandedThinking[msg.ID] = !p.expandedThinking[msg.ID]
					}
					// Toggle tool outputs
					for _, tu := range msg.ToolUses {
						p.expandedToolResults[tu.ID] = !p.expandedToolResults[tu.ID]
					}
					// Invalidate render cache for this message (td-5445abd6)
					p.invalidateCacheForMessage(msg.ID)
				}
				// Mark hit regions dirty since expansion changes layout (td-ea784b03)
				p.hitRegionsDirty = true
			}
		} else {
			// Conversation flow: toggle for current message
			if msg := p.getSelectedMessage(); msg != nil {
				// Toggle message content
				p.expandedMessages[msg.ID] = !p.expandedMessages[msg.ID]
				// Toggle thinking blocks
				for _, block := range msg.ContentBlocks {
					if block.Type == "thinking" {
						p.expandedThinking[msg.ID] = !p.expandedThinking[msg.ID]
						break
					}
				}
				// Toggle tool outputs
				for _, block := range msg.ContentBlocks {
					if block.Type == "tool_use" && block.ToolUseID != "" {
						p.expandedToolResults[block.ToolUseID] = !p.expandedToolResults[block.ToolUseID]
					}
				}
				// Invalidate render cache for this message (td-5445abd6)
				p.invalidateCacheForMessage(msg.ID)
				// Mark hit regions dirty since expansion changes layout (td-ea784b03)
				p.hitRegionsDirty = true
			}
		}

	case "enter":
		// Open turn detail view in right pane
		if p.turnViewMode {
			// Turn view: use turnCursor
			if p.turnCursor < len(p.turns) {
				p.detailTurn = &p.turns[p.turnCursor]
				p.detailScroll = 0
				p.detailMode = true
			}
		} else {
			// Conversation flow: find turn containing selected message
			if msg := p.getSelectedMessage(); msg != nil {
				for i := range p.turns {
					for _, m := range p.turns[i].Messages {
						if m.ID == msg.ID {
							p.detailTurn = &p.turns[i]
							p.detailScroll = 0
							p.detailMode = true
							return p, nil
						}
					}
				}
			}
		}

	case "c":
		// Copy session to clipboard as markdown
		if p.selectedSession != "" {
			return p, p.copySessionToClipboard()
		}

	case "E":
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

	case "o":
		// Open session in native CLI tool
		return p, p.openSessionInCLI()
	}

	return p, nil
}

func (p *Plugin) toggleSidebar() {
	if p.sidebarVisible {
		p.sidebarRestore = p.activePane
		p.sidebarVisible = false
		if p.activePane == PaneSidebar {
			p.activePane = PaneMessages
		}
		return
	}

	p.sidebarVisible = true
	if p.sidebarRestore == PaneSidebar {
		p.activePane = PaneSidebar
	} else {
		p.activePane = PaneMessages
	}
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
	// Note: sidebarWidth is calculated in renderTwoPane, not here,
	// to avoid resetting drag-adjusted widths on every render

	var content string
	if len(p.adapters) == 0 {
		content = renderNoAdapter()
	} else {
		switch p.view {
		case ViewAnalytics:
			content = p.renderAnalytics()
		default:
			content = p.renderTwoPane()
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
	// Detail mode (right pane shows turn detail)
	if p.detailMode {
		return []plugin.Command{
			{ID: "back", Name: "Back", Description: "Return to turn list", Category: plugin.CategoryNavigation, Context: "turn-detail", Priority: 1},
			{ID: "scroll", Name: "Scroll", Description: "Scroll detail", Category: plugin.CategoryNavigation, Context: "turn-detail", Priority: 2},
			{ID: "yank", Name: "Yank", Description: "Yank turn content", Category: plugin.CategoryActions, Context: "turn-detail", Priority: 3},
		}
	}
	if p.activePane == PaneMessages {
		return []plugin.Command{
			{ID: "toggle-view", Name: "View", Description: "Toggle conversation/turn view", Category: plugin.CategoryView, Context: "conversations-main", Priority: 1},
			{ID: "detail", Name: "Detail", Description: "View turn details", Category: plugin.CategoryView, Context: "conversations-main", Priority: 2},
			{ID: "expand", Name: "Expand", Description: "Expand selected item", Category: plugin.CategoryView, Context: "conversations-main", Priority: 3},
			{ID: "back", Name: "Back", Description: "Return to sidebar", Category: plugin.CategoryNavigation, Context: "conversations-main", Priority: 4},
			{ID: "open", Name: "Open", Description: "Open in CLI", Category: plugin.CategoryActions, Context: "conversations-main", Priority: 5},
			{ID: "yank", Name: "Yank", Description: "Yank turn content", Category: plugin.CategoryActions, Context: "conversations-main", Priority: 6},
			{ID: "toggle-sidebar", Name: "Panel", Description: "Toggle sidebar visibility", Category: plugin.CategoryView, Context: "conversations-main", Priority: 7},
		}
	}
	if p.view == ViewAnalytics {
		return []plugin.Command{
			{ID: "back", Name: "Back", Description: "Return to conversations", Category: plugin.CategoryNavigation, Context: "analytics", Priority: 1},
		}
	}
	return []plugin.Command{
		{ID: "view-session", Name: "View", Description: "View session messages", Category: plugin.CategoryView, Context: "conversations-sidebar", Priority: 1},
		{ID: "search", Name: "Search", Description: "Search conversations", Category: plugin.CategorySearch, Context: "conversations-sidebar", Priority: 2},
		{ID: "filter", Name: "Filter", Description: "Filter by project", Category: plugin.CategorySearch, Context: "conversations-sidebar", Priority: 2},
		{ID: "open", Name: "Open", Description: "Open in CLI", Category: plugin.CategoryActions, Context: "conversations-sidebar", Priority: 3},
		{ID: "yank", Name: "Yank", Description: "Yank session details", Category: plugin.CategoryActions, Context: "conversations-sidebar", Priority: 4},
		{ID: "yank-resume", Name: "Resume", Description: "Yank resume command", Category: plugin.CategoryActions, Context: "conversations-sidebar", Priority: 4},
		{ID: "toggle-sidebar", Name: "Panel", Description: "Toggle sidebar visibility", Category: plugin.CategoryView, Context: "conversations-sidebar", Priority: 5},
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
	// Detail mode (right pane shows turn detail)
	if p.detailMode {
		return "turn-detail"
	}
	switch p.view {
	case ViewAnalytics:
		return "analytics"
	default:
		// Return context based on active pane
		if p.activePane == PaneSidebar {
			return "conversations-sidebar"
		}
		return "conversations-main"
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
// Queries sessions from all related worktree paths to show cross-worktree conversations.
// Sessions from deleted worktrees are marked with "(deleted)" in their worktree name.
func (p *Plugin) loadSessions() tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return SessionsLoadedMsg{}
		}

		// Get all related worktree paths (main repo + all worktrees)
		worktreePaths := app.GetAllRelatedPaths(p.ctx.WorkDir)
		if len(worktreePaths) == 0 {
			// Not a git repo or no worktrees - just use current workdir
			worktreePaths = []string{p.ctx.WorkDir}
		}

		// Track seen sessions to avoid duplicates (same session loaded from multiple paths)
		seenSessions := make(map[string]bool)
		var sessions []adapter.Session

		// Get current working directory for worktree name comparison
		currentPath := p.ctx.WorkDir
		if absPath, err := filepath.Abs(currentPath); err == nil {
			currentPath = absPath
		}

		for id, a := range p.adapters {
			for _, wtPath := range worktreePaths {
				adapterSessions, err := a.Sessions(wtPath)
				if err != nil {
					continue
				}

				// Get worktree name for sessions from this path
				wtName := app.WorktreeNameForPath(p.ctx.WorkDir, wtPath)

				for i := range adapterSessions {
					// Skip duplicates
					if seenSessions[adapterSessions[i].ID] {
						continue
					}
					seenSessions[adapterSessions[i].ID] = true

					if adapterSessions[i].AdapterID == "" {
						adapterSessions[i].AdapterID = id
					}
					if adapterSessions[i].AdapterName == "" {
						adapterSessions[i].AdapterName = a.Name()
					}
					if adapterSessions[i].AdapterIcon == "" {
						adapterSessions[i].AdapterIcon = a.Icon()
					}

					// Set worktree fields if session is from a different worktree
					absWtPath := wtPath
					if abs, err := filepath.Abs(wtPath); err == nil {
						absWtPath = abs
					}
					if absWtPath != currentPath {
						adapterSessions[i].WorktreeName = wtName
						adapterSessions[i].WorktreePath = absWtPath
					}

					sessions = append(sessions, adapterSessions[i])
				}
			}
		}

		// Mark sessions from deleted worktrees
		for i := range sessions {
			if sessions[i].WorktreePath != "" {
				if _, err := os.Stat(sessions[i].WorktreePath); os.IsNotExist(err) {
					sessions[i].WorktreeName = sessions[i].WorktreeName + " (deleted)"
				}
			}
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
		})

		return SessionsLoadedMsg{Sessions: sessions}
	}
}

// loadMessages loads messages for a session with pagination support (td-313ea851).
func (p *Plugin) loadMessages(sessionID string) tea.Cmd {
	offset := p.messageOffset
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

		totalCount := len(messages)
		resultOffset := 0

		// Apply pagination: load a window of maxMessagesInMemory messages
		if len(messages) > maxMessagesInMemory {
			// offset indicates how many messages to skip from the end (most recent)
			// offset=0 means show the most recent messages
			// offset=100 means skip the 100 most recent and show older ones
			endIdx := len(messages) - offset
			if endIdx < 0 {
				endIdx = 0
			}
			startIdx := endIdx - maxMessagesInMemory
			if startIdx < 0 {
				startIdx = 0
			}
			resultOffset = startIdx
			messages = messages[startIdx:endIdx]
		}

		return MessagesLoadedMsg{
			SessionID:  sessionID,
			Messages:   messages,
			TotalCount: totalCount,
			Offset:     resultOffset,
		}
	}
}

// startWatcher starts watching for session changes.
// Monitors all related worktree paths for live updates across worktrees.
func (p *Plugin) startWatcher() tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return WatchStartedMsg{Channel: nil}
		}

		// Get all related worktree paths (main repo + all worktrees)
		worktreePaths := app.GetAllRelatedPaths(p.ctx.WorkDir)
		if len(worktreePaths) == 0 {
			// Not a git repo or no worktrees - just use current workdir
			worktreePaths = []string{p.ctx.WorkDir}
		}

		merged := make(chan adapter.Event, 32)
		var wg sync.WaitGroup
		watchCount := 0

		// Watch all worktree paths with each adapter
		for _, a := range p.adapters {
			for _, wtPath := range worktreePaths {
				ch, err := a.Watch(wtPath)
				if err != nil || ch == nil {
					continue
				}
				watchCount++
				wg.Add(1)
				go func(c <-chan adapter.Event) {
					defer wg.Done()
					for evt := range c {
						select {
						case merged <- evt:
						default:
						}
					}
				}(ch)
			}
		}

		if watchCount == 0 {
			return WatchStartedMsg{Channel: nil}
		}

		// Close merged channel when all source channels are done
		go func() {
			wg.Wait()
			close(merged)
		}()

		return WatchStartedMsg{Channel: merged}
	}
}

// listenForWatchEvents waits for the next file system event.
func (p *Plugin) listenForWatchEvents() tea.Cmd {
	if p.watchChan == nil {
		return nil
	}
	return func() tea.Msg {
		evt, ok := <-p.watchChan
		if !ok {
			// Channel closed
			return nil
		}
		return WatchEventMsg{SessionID: evt.SessionID}
	}
}

// listenForCoalescedRefresh waits for coalesced refresh messages.
func (p *Plugin) listenForCoalescedRefresh() tea.Cmd {
	return func() tea.Msg {
		return <-p.coalesceChan
	}
}

// ensureCursorVisible adjusts scroll to keep cursor visible.
func (p *Plugin) ensureCursorVisible() {
	// Pane height - borders(2) - header(1-2)
	paneHeight := p.height - 2
	visibleRows := paneHeight - 3 // -2 for inner height calc, -1 for header
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
	// Pane height - borders(2) - header(4-5)
	paneHeight := p.height - 2
	visibleRows := paneHeight - 6 // Account for header, stats, resume cmd, separator
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
	// Pane height - borders(2) - header(4-5)
	paneHeight := p.height - 2
	visibleRows := paneHeight - 6 // Account for header, stats, resume cmd, separator
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
		p.hitRegionsDirty = true // Session list returns (td-455e378b)

	case "enter":
		p.filterMode = false
		p.hitRegionsDirty = true // Session list returns (td-455e378b)
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

// getSelectedMessage returns the message at the current messageCursor position.
func (p *Plugin) getSelectedMessage() *adapter.Message {
	if len(p.messages) == 0 {
		return nil
	}
	idx := p.messageCursor
	if idx < 0 {
		idx = 0
	}
	if idx >= len(p.messages) {
		idx = len(p.messages) - 1
	}
	return &p.messages[idx]
}

// visibleMessageIndices returns indices of messages shown in conversation flow
// (excludes tool_result-only messages which are shown inline).
func (p *Plugin) visibleMessageIndices() []int {
	var indices []int
	for i, msg := range p.messages {
		if !p.isToolResultOnlyMessage(msg) {
			indices = append(indices, i)
		}
	}
	return indices
}

// isToolResultOnlyMessage checks if a message contains only tool_result blocks.
func (p *Plugin) isToolResultOnlyMessage(msg adapter.Message) bool {
	if len(msg.ContentBlocks) == 0 {
		return false
	}
	for _, block := range msg.ContentBlocks {
		if block.Type != "tool_result" {
			return false
		}
	}
	return true
}

// ensureMessageCursorVisible scrolls to keep the message cursor in view.
func (p *Plugin) ensureMessageCursorVisible() {
	// Use actual line positions if available (populated during render)
	// Otherwise fall back to estimation for initial render

	// Get available height
	viewHeight := p.height - 10 // Account for headers/margins
	if viewHeight < 10 {
		viewHeight = 10
	}

	// Try to use actual line positions from last render
	if len(p.msgLinePositions) > 0 {
		// Find cursor position in msgLinePositions
		for _, pos := range p.msgLinePositions {
			if pos.MsgIdx == p.messageCursor {
				cursorLine := pos.StartLine
				cursorEnd := cursorLine + pos.LineCount

				// Scroll to keep cursor visible with margin
				margin := 2

				// If cursor is above viewport, scroll up
				if cursorLine < p.messageScroll+margin {
					p.messageScroll = cursorLine - margin
				}
				// If cursor is below viewport, scroll down
				if cursorEnd > p.messageScroll+viewHeight-margin {
					p.messageScroll = cursorEnd - viewHeight + margin
				}

				if p.messageScroll < 0 {
					p.messageScroll = 0
				}
				return
			}
		}
	}

	// Fallback: estimate based on message count (for initial render)
	visibleIndices := p.visibleMessageIndices()
	if len(visibleIndices) == 0 {
		return
	}

	cursorPos := 0
	for i, idx := range visibleIndices {
		if idx == p.messageCursor {
			cursorPos = i
			break
		}
	}

	// Rough estimate of 5 lines per message
	estimatedLine := cursorPos * 5

	margin := viewHeight / 4
	if estimatedLine < p.messageScroll+margin {
		p.messageScroll = estimatedLine - margin
	} else if estimatedLine > p.messageScroll+viewHeight-margin {
		p.messageScroll = estimatedLine - viewHeight + margin
	}

	if p.messageScroll < 0 {
		p.messageScroll = 0
	}
}

// Message types
type SessionsLoadedMsg struct {
	Sessions []adapter.Session
}

type MessagesLoadedMsg struct {
	SessionID  string
	Messages   []adapter.Message
	TotalCount int // Total message count before truncation (td-313ea851)
	Offset     int // Offset into the message list (td-313ea851)
}

type WatchEventMsg struct {
	SessionID string // ID of the session that changed (empty for periodic refresh)
}
type WatchStartedMsg struct {
	Channel <-chan adapter.Event
}
type ErrorMsg struct{ Err error }

type PreviewLoadMsg struct {
	Token     int
	SessionID string
}

type MessageReloadMsg struct {
	Token     int
	SessionID string
}

// TickCmd returns a command that triggers periodic refresh.
func TickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return WatchEventMsg{}
	})
}

// messagesMatch checks if old messages match the prefix of new messages.
// Used to detect incremental updates (new messages appended).
func (p *Plugin) messagesMatch(old, newPrefix []adapter.Message) bool {
	if len(old) != len(newPrefix) {
		return false
	}
	for i := range old {
		// Compare by ID and timestamp - content may have minor differences
		if old[i].ID != newPrefix[i].ID {
			return false
		}
	}
	return true
}

// Render cache methods (td-8910b218)

// clearRenderCache clears the entire render cache.
func (p *Plugin) clearRenderCache() {
	p.renderCacheMutex.Lock()
	defer p.renderCacheMutex.Unlock()
	p.renderCache = make(map[renderCacheKey]string)
}

// getCachedRender returns cached render content if available.
func (p *Plugin) getCachedRender(msgID string, width int, expanded bool) (string, bool) {
	p.renderCacheMutex.RLock()
	defer p.renderCacheMutex.RUnlock()
	key := renderCacheKey{messageID: msgID, width: width, expanded: expanded}
	cached, ok := p.renderCache[key]
	return cached, ok
}

// setCachedRender stores rendered content in cache with size-based eviction.
func (p *Plugin) setCachedRender(msgID string, width int, expanded bool, content string) {
	p.renderCacheMutex.Lock()
	defer p.renderCacheMutex.Unlock()

	// LRU eviction: limit cache to 100 entries
	const maxCacheSize = 100
	if len(p.renderCache) >= maxCacheSize {
		// Simple eviction: clear half the cache
		count := 0
		for k := range p.renderCache {
			delete(p.renderCache, k)
			count++
			if count >= maxCacheSize/2 {
				break
			}
		}
	}

	key := renderCacheKey{messageID: msgID, width: width, expanded: expanded}
	p.renderCache[key] = content
}

// invalidateCacheForMessage removes cache entries for a specific message.
func (p *Plugin) invalidateCacheForMessage(msgID string) {
	p.renderCacheMutex.Lock()
	defer p.renderCacheMutex.Unlock()
	for k := range p.renderCache {
		if k.messageID == msgID {
			delete(p.renderCache, k)
		}
	}
}
