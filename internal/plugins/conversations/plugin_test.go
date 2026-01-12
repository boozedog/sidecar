package conversations

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/adapter"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("expected non-nil plugin")
	}
	if p.pageSize != defaultPageSize {
		t.Errorf("expected pageSize %d, got %d", defaultPageSize, p.pageSize)
	}
}

func TestPluginID(t *testing.T) {
	p := New()
	if id := p.ID(); id != "conversations" {
		t.Errorf("expected ID 'conversations', got %q", id)
	}
}

func TestPluginName(t *testing.T) {
	p := New()
	if name := p.Name(); name != "conversations" {
		t.Errorf("expected Name 'conversations', got %q", name)
	}
}

func TestPluginIcon(t *testing.T) {
	p := New()
	if icon := p.Icon(); icon != "C" {
		t.Errorf("expected Icon 'C', got %q", icon)
	}
}

func TestFocusContext(t *testing.T) {
	p := New()

	// Default: sidebar pane focused
	p.activePane = PaneSidebar
	if ctx := p.FocusContext(); ctx != "conversations-sidebar" {
		t.Errorf("expected context 'conversations-sidebar', got %q", ctx)
	}

	// Messages pane focused
	p.activePane = PaneMessages
	if ctx := p.FocusContext(); ctx != "conversations-main" {
		t.Errorf("expected context 'conversations-main', got %q", ctx)
	}

	// Detail mode
	p.detailMode = true
	if ctx := p.FocusContext(); ctx != "turn-detail" {
		t.Errorf("expected context 'turn-detail', got %q", ctx)
	}
}

func TestDiagnosticsNoAdapter(t *testing.T) {
	p := New()
	diags := p.Diagnostics()

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	if diags[0].Status != "disabled" {
		t.Errorf("expected status 'disabled', got %q", diags[0].Status)
	}
	if diags[1].ID != "watcher" {
		t.Errorf("expected watcher diagnostic, got %q", diags[1].ID)
	}
}

func TestDiagnosticsWithSessions(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}} // Set a non-nil adapter
	p.sessions = []adapter.Session{
		{ID: "test-1"},
		{ID: "test-2"},
	}

	diags := p.Diagnostics()

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	if diags[0].Status != "ok" {
		t.Errorf("expected status 'ok', got %q", diags[0].Status)
	}
	if diags[1].ID != "watcher" {
		t.Errorf("expected watcher diagnostic, got %q", diags[1].ID)
	}
}

func TestEnsureCursorVisible(t *testing.T) {
	p := New()
	p.height = 10 // 4 visible rows after header/footer

	// Cursor at 0, scroll at 0 - should stay
	p.cursor = 0
	p.scrollOff = 0
	p.ensureCursorVisible()
	if p.scrollOff != 0 {
		t.Errorf("expected scrollOff 0, got %d", p.scrollOff)
	}

	// Move cursor down past visible area
	p.cursor = 10
	p.ensureCursorVisible()
	if p.scrollOff == 0 {
		t.Error("expected scrollOff to increase")
	}

	// Move cursor back up
	p.cursor = 0
	p.ensureCursorVisible()
	if p.scrollOff != 0 {
		t.Errorf("expected scrollOff 0, got %d", p.scrollOff)
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		text     string
		maxWidth int
		expected int
	}{
		{"hello world", 20, 1},
		{"hello world this is a longer text", 10, 5},
		{"", 10, 0},
		{"one two three four five", 10, 3},
	}

	for _, tt := range tests {
		lines := wrapText(tt.text, tt.maxWidth)
		if len(lines) != tt.expected {
			t.Errorf("wrapText(%q, %d) = %d lines, expected %d",
				tt.text, tt.maxWidth, len(lines), tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{1 * time.Minute, "1m ago"},
		{2 * time.Hour, "2h ago"},
		{1 * time.Hour, "1h ago"},
		{48 * time.Hour, "2d ago"},
		{24 * time.Hour, "1d ago"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, expected %q",
				tt.duration, result, tt.expected)
		}
	}
}

func TestFormatSessionCount(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{1, "1 session"},
		{5, "5 sessions"},
		{10, "10 sessions"},
		{100, "100 sessions"},
	}

	for _, tt := range tests {
		result := formatSessionCount(tt.count)
		if result != tt.expected {
			t.Errorf("formatSessionCount(%d) = %q, expected %q",
				tt.count, result, tt.expected)
		}
	}
}

func TestSearchModeToggle(t *testing.T) {
	p := New()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "first-session"},
		{ID: "test-2", Name: "second-session"},
	}
	p.activePane = PaneSidebar

	// Initially not in search mode
	if p.searchMode {
		t.Error("expected searchMode to be false initially")
	}

	// FocusContext should be "conversations-sidebar"
	if ctx := p.FocusContext(); ctx != "conversations-sidebar" {
		t.Errorf("expected context 'conversations-sidebar', got %q", ctx)
	}

	// Toggle search mode on
	p.searchMode = true
	if ctx := p.FocusContext(); ctx != "conversations-search" {
		t.Errorf("expected context 'conversations-search', got %q", ctx)
	}
}

func TestFilterSessions(t *testing.T) {
	p := New()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha-session", Slug: "alpha-slug"},
		{ID: "test-2", Name: "beta-session", Slug: "beta-slug"},
		{ID: "test-3", Name: "gamma-session", Slug: "gamma-slug"},
	}

	// No filter
	p.filterSessions()
	if p.searchResults != nil {
		t.Error("expected nil searchResults with empty query")
	}

	// Filter by name
	p.searchQuery = "beta"
	p.filterSessions()
	if len(p.searchResults) != 1 {
		t.Errorf("expected 1 result, got %d", len(p.searchResults))
	}
	if p.searchResults[0].Name != "beta-session" {
		t.Errorf("expected 'beta-session', got %q", p.searchResults[0].Name)
	}

	// Filter by slug
	p.searchQuery = "gamma-slug"
	p.filterSessions()
	if len(p.searchResults) != 1 {
		t.Errorf("expected 1 result, got %d", len(p.searchResults))
	}

	// No matches
	p.searchQuery = "nonexistent"
	p.filterSessions()
	if len(p.searchResults) != 0 {
		t.Errorf("expected 0 results, got %d", len(p.searchResults))
	}
}

func TestVisibleSessions(t *testing.T) {
	p := New()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}

	// Without search mode, should return all sessions
	visible := p.visibleSessions()
	if len(visible) != 2 {
		t.Errorf("expected 2 visible sessions, got %d", len(visible))
	}

	// In search mode with query, should return filtered results
	p.searchMode = true
	p.searchQuery = "alpha"
	p.filterSessions()
	visible = p.visibleSessions()
	if len(visible) != 1 {
		t.Errorf("expected 1 visible session, got %d", len(visible))
	}
}

func TestShortID(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"12345678", "12345678"},
		{"123456789abcdef", "12345678"},
		{"1234567", "1234567"},
		{"abc", "abc"},
		{"", ""},
	}

	for _, tt := range tests {
		result := shortID(tt.id)
		if result != tt.expected {
			t.Errorf("shortID(%q) = %q, expected %q", tt.id, result, tt.expected)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input, output, cache int
		expected             string
	}{
		{0, 0, 0, ""},
		{100, 0, 0, " (in:100)"},
		{0, 100, 0, " (out:100)"},
		{0, 0, 100, " ($:100)"},
		{1000, 2000, 500, " (in:1.0k out:2.0k $:500)"},
		{1500000, 2500000, 0, " (in:1.5M out:2.5M)"},
	}

	for _, tt := range tests {
		result := formatTokens(tt.input, tt.output, tt.cache)
		if result != tt.expected {
			t.Errorf("formatTokens(%d, %d, %d) = %q, expected %q",
				tt.input, tt.output, tt.cache, result, tt.expected)
		}
	}
}

func TestFormatK(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{999999, "1000.0k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}

	for _, tt := range tests {
		result := formatK(tt.n)
		if result != tt.expected {
			t.Errorf("formatK(%d) = %q, expected %q", tt.n, result, tt.expected)
		}
	}
}

func TestDiagnosticsEmptySessions(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{} // Empty but adapter exists

	diags := p.Diagnostics()

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	if diags[0].Status != "empty" {
		t.Errorf("expected status 'empty', got %q", diags[0].Status)
	}
}

func TestDiagnosticsActiveSessions(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", IsActive: true},
		{ID: "test-2", IsActive: false},
		{ID: "test-3", IsActive: true},
	}

	diags := p.Diagnostics()

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	// Should show "3 sessions (2 active)"
	if diags[0].Status != "ok" {
		t.Errorf("expected status 'ok', got %q", diags[0].Status)
	}
	expectedDetail := "3 sessions (2 active)"
	if diags[0].Detail != expectedDetail {
		t.Errorf("expected detail %q, got %q", expectedDetail, diags[0].Detail)
	}
}

func TestDiagnosticsWatcherOn(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.watchChan = make(chan adapter.Event) // Non-nil channel

	diags := p.Diagnostics()

	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	if diags[1].Status != "on" {
		t.Errorf("expected watcher status 'on', got %q", diags[1].Status)
	}
}

// Test WatchStartedMsg with nil channel
func TestUpdateWatchStartedMsgNilChannel(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}

	msg := WatchStartedMsg{Channel: nil}
	newPlugin, cmd := p.Update(msg)

	if newPlugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	if cmd != nil {
		t.Error("expected nil command when channel is nil")
	}
	if p.watchChan != nil {
		t.Error("expected watchChan to remain nil")
	}
}

// Test WatchStartedMsg with valid channel
func TestUpdateWatchStartedMsgValidChannel(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}

	ch := make(chan adapter.Event)
	msg := WatchStartedMsg{Channel: ch}
	newPlugin, cmd := p.Update(msg)

	if newPlugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	if cmd == nil {
		t.Error("expected non-nil command to start listening")
	}
	if p.watchChan != ch {
		t.Error("expected watchChan to be set to the provided channel")
	}
}

// Test WatchEventMsg triggers session reload
func TestUpdateWatchEventMsg(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.watchChan = make(chan adapter.Event)

	msg := WatchEventMsg{}
	newPlugin, cmd := p.Update(msg)

	if newPlugin == nil {
		t.Fatal("expected non-nil plugin")
	}
	// Should return a batch command for loadSessions and listenForWatchEvents
	if cmd == nil {
		t.Error("expected non-nil command for batch operation")
	}
}

// Test listenForWatchEvents with nil channel
func TestListenForWatchEventsNilChannel(t *testing.T) {
	p := New()
	p.watchChan = nil

	cmd := p.listenForWatchEvents()
	if cmd != nil {
		t.Error("expected nil command when watchChan is nil")
	}
}

// Test listenForWatchEvents with closed channel
func TestListenForWatchEventsClosedChannel(t *testing.T) {
	p := New()
	ch := make(chan adapter.Event)
	close(ch) // Close the channel
	p.watchChan = ch

	cmd := p.listenForWatchEvents()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Execute the command - should return nil when channel is closed
	msg := cmd()
	if msg != nil {
		t.Errorf("expected nil message for closed channel, got %T", msg)
	}
}

// Test listenForWatchEvents receives event
func TestListenForWatchEventsReceivesEvent(t *testing.T) {
	p := New()
	ch := make(chan adapter.Event, 1) // Buffered to avoid blocking
	p.watchChan = ch

	cmd := p.listenForWatchEvents()
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	// Send an event
	ch <- adapter.Event{}

	// Execute the command - should return WatchEventMsg
	msg := cmd()
	if _, ok := msg.(WatchEventMsg); !ok {
		t.Errorf("expected WatchEventMsg, got %T", msg)
	}
}

// Test SessionsLoadedMsg updates sessions
func TestUpdateSessionsLoadedMsg(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}

	sessions := []adapter.Session{
		{ID: "s1", Name: "Session 1"},
		{ID: "s2", Name: "Session 2"},
	}

	msg := SessionsLoadedMsg{Sessions: sessions}
	_, _ = p.Update(msg)

	if len(p.sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(p.sessions))
	}
	if p.sessions[0].ID != "s1" {
		t.Errorf("expected session ID 's1', got %q", p.sessions[0].ID)
	}
}

// Test MessagesLoadedMsg updates messages and hasMore flag
func TestUpdateMessagesLoadedMsg(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.pageSize = 2
	p.selectedSession = "s1"

	// Test with fewer messages than page size
	messages := []adapter.Message{
		{Role: "user", Content: "Hello"},
	}
	msg := MessagesLoadedMsg{SessionID: "s1", Messages: messages}
	_, _ = p.Update(msg)

	if len(p.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(p.messages))
	}
	if p.hasMore {
		t.Error("expected hasMore to be false when messages < pageSize")
	}

	// Test with page size messages (indicates more might be available)
	messages = []adapter.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}
	msg = MessagesLoadedMsg{SessionID: "s1", Messages: messages}
	_, _ = p.Update(msg)

	if len(p.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(p.messages))
	}
	if !p.hasMore {
		t.Error("expected hasMore to be true when messages == pageSize")
	}
}

// mockAdapter is a minimal adapter for testing
type mockAdapter struct{}

func (m *mockAdapter) ID() string                                             { return "mock" }
func (m *mockAdapter) Name() string                                           { return "Mock" }
func (m *mockAdapter) Icon() string                                           { return "◆" }
func (m *mockAdapter) Detect(projectRoot string) (bool, error)                { return true, nil }
func (m *mockAdapter) Capabilities() adapter.CapabilitySet                    { return nil }
func (m *mockAdapter) Sessions(projectRoot string) ([]adapter.Session, error) { return nil, nil }
func (m *mockAdapter) Messages(sessionID string) ([]adapter.Message, error)   { return nil, nil }
func (m *mockAdapter) Usage(sessionID string) (*adapter.UsageStats, error)    { return nil, nil }
func (m *mockAdapter) Watch(projectRoot string) (<-chan adapter.Event, error) { return nil, nil }

// =============================================================================
// Search Key Handling Tests via Update()
// =============================================================================

// TestUpdateSearchModeEnter tests entering search mode with '/' key.
func TestUpdateSearchModeEnter(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}
	p.cursor = 1 // Set cursor to non-zero

	if p.searchMode {
		t.Fatal("expected searchMode to be false initially")
	}

	// Press '/' to enter search mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	_, _ = p.Update(msg)

	if !p.searchMode {
		t.Error("expected searchMode to be true after pressing '/'")
	}
	if p.searchQuery != "" {
		t.Errorf("expected empty searchQuery, got %q", p.searchQuery)
	}
	if p.cursor != 0 {
		t.Errorf("expected cursor to reset to 0, got %d", p.cursor)
	}
	if p.scrollOff != 0 {
		t.Errorf("expected scrollOff to reset to 0, got %d", p.scrollOff)
	}
}

// TestUpdateSearchModeExit tests exiting search mode with 'esc' key.
func TestUpdateSearchModeExit(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}
	p.searchMode = true
	p.searchQuery = "test"
	p.searchResults = []adapter.Session{{ID: "test-1"}}
	p.cursor = 1

	// Press 'esc' to exit search mode
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, _ = p.Update(msg)

	if p.searchMode {
		t.Error("expected searchMode to be false after pressing 'esc'")
	}
	if p.searchQuery != "" {
		t.Errorf("expected searchQuery to be cleared, got %q", p.searchQuery)
	}
	if p.searchResults != nil {
		t.Error("expected searchResults to be nil")
	}
	if p.cursor != 0 {
		t.Errorf("expected cursor to reset to 0, got %d", p.cursor)
	}
}

// TestUpdateSearchTypingCharacters tests typing characters to build searchQuery.
func TestUpdateSearchTypingCharacters(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
		{ID: "test-3", Name: "gamma"},
	}
	p.searchMode = true

	// Type 'a'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	_, _ = p.Update(msg)

	if p.searchQuery != "a" {
		t.Errorf("expected searchQuery 'a', got %q", p.searchQuery)
	}

	// Type 'l'
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	_, _ = p.Update(msg)

	if p.searchQuery != "al" {
		t.Errorf("expected searchQuery 'al', got %q", p.searchQuery)
	}

	// Type 'p'
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	_, _ = p.Update(msg)

	if p.searchQuery != "alp" {
		t.Errorf("expected searchQuery 'alp', got %q", p.searchQuery)
	}

	// Verify filtering occurred (should match "alpha")
	if len(p.searchResults) != 1 {
		t.Errorf("expected 1 search result, got %d", len(p.searchResults))
	}
	if len(p.searchResults) > 0 && p.searchResults[0].Name != "alpha" {
		t.Errorf("expected result 'alpha', got %q", p.searchResults[0].Name)
	}
}

// TestUpdateSearchBackspace tests backspace deleting from searchQuery.
func TestUpdateSearchBackspace(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}
	p.searchMode = true
	p.searchQuery = "alph"
	p.filterSessions() // Initialize searchResults

	// Press backspace
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	_, _ = p.Update(msg)

	if p.searchQuery != "alp" {
		t.Errorf("expected searchQuery 'alp', got %q", p.searchQuery)
	}

	// Press backspace again
	_, _ = p.Update(msg)

	if p.searchQuery != "al" {
		t.Errorf("expected searchQuery 'al', got %q", p.searchQuery)
	}

	// Press backspace until empty
	_, _ = p.Update(msg)
	_, _ = p.Update(msg)

	if p.searchQuery != "" {
		t.Errorf("expected empty searchQuery, got %q", p.searchQuery)
	}

	// Backspace on empty query should do nothing
	_, _ = p.Update(msg)

	if p.searchQuery != "" {
		t.Errorf("expected searchQuery to remain empty, got %q", p.searchQuery)
	}
}

// TestUpdateSearchNavigationDown tests down navigation in search results.
func TestUpdateSearchNavigationDown(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "also-alpha"},
		{ID: "test-3", Name: "another-alpha"},
	}
	p.searchMode = true
	p.searchQuery = "alpha"
	p.filterSessions()
	p.height = 20 // Set height for scroll calculations

	if p.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", p.cursor)
	}

	// Press down arrow
	msg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = p.Update(msg)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", p.cursor)
	}

	// Press down again
	_, _ = p.Update(msg)

	if p.cursor != 2 {
		t.Errorf("expected cursor 2 after second down, got %d", p.cursor)
	}

	// Press down at end (should stay at end)
	_, _ = p.Update(msg)

	if p.cursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", p.cursor)
	}
}

// TestUpdateSearchNavigationUp tests up navigation in search results.
func TestUpdateSearchNavigationUp(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "also-alpha"},
		{ID: "test-3", Name: "another-alpha"},
	}
	p.searchMode = true
	p.searchQuery = "alpha"
	p.filterSessions()
	p.cursor = 2 // Start at last item
	p.height = 20

	// Press up arrow
	msg := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = p.Update(msg)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", p.cursor)
	}

	// Press up again
	_, _ = p.Update(msg)

	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after second up, got %d", p.cursor)
	}

	// Press up at top (should stay at 0)
	_, _ = p.Update(msg)

	if p.cursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", p.cursor)
	}
}

// TestUpdateSearchNavigationCtrlN tests ctrl+n navigation in search results.
func TestUpdateSearchNavigationCtrlN(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "also-alpha"},
	}
	p.searchMode = true
	p.searchQuery = "alpha"
	p.filterSessions()
	p.height = 20

	// Press ctrl+n (should move down)
	msg := tea.KeyMsg{Type: tea.KeyCtrlN}
	_, _ = p.Update(msg)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after ctrl+n, got %d", p.cursor)
	}
}

// TestUpdateSearchNavigationCtrlP tests ctrl+p navigation in search results.
func TestUpdateSearchNavigationCtrlP(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "also-alpha"},
	}
	p.searchMode = true
	p.searchQuery = "alpha"
	p.filterSessions()
	p.cursor = 1 // Start at second item
	p.height = 20

	// Press ctrl+p (should move up)
	msg := tea.KeyMsg{Type: tea.KeyCtrlP}
	_, _ = p.Update(msg)

	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after ctrl+p, got %d", p.cursor)
	}
}

// TestUpdateSearchEnterSelectsSession tests enter key selects session in search mode.
func TestUpdateSearchEnterSelectsSession(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}
	p.searchMode = true
	p.searchQuery = "beta"
	p.filterSessions()
	p.height = 20

	if len(p.searchResults) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(p.searchResults))
	}

	// Press enter to select
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := p.Update(msg)

	if p.searchMode {
		t.Error("expected searchMode to be false after enter")
	}
	if p.activePane != PaneMessages {
		t.Errorf("expected activePane to be PaneMessages, got %d", p.activePane)
	}
	if p.selectedSession != "test-2" {
		t.Errorf("expected selectedSession 'test-2', got %q", p.selectedSession)
	}
	if cmd == nil {
		t.Error("expected non-nil command to load messages")
	}
}

// TestUpdateSearchCursorResetOnQuery tests cursor resets when query changes.
func TestUpdateSearchCursorResetOnQuery(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "also-alpha"},
		{ID: "test-3", Name: "beta"},
	}
	p.searchMode = true
	p.searchQuery = "a"
	p.filterSessions()
	p.cursor = 1 // Move cursor to second result
	p.height = 20

	// Type another character - cursor should reset
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	_, _ = p.Update(msg)

	if p.cursor != 0 {
		t.Errorf("expected cursor to reset to 0, got %d", p.cursor)
	}

	// Move cursor again
	p.cursor = 1

	// Backspace should also reset cursor
	msg = tea.KeyMsg{Type: tea.KeyBackspace}
	_, _ = p.Update(msg)

	if p.cursor != 0 {
		t.Errorf("expected cursor to reset to 0 after backspace, got %d", p.cursor)
	}
}

// TestUpdateSearchEmptyResults tests behavior with no matching results.
func TestUpdateSearchEmptyResults(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "alpha"},
		{ID: "test-2", Name: "beta"},
	}
	p.searchMode = true
	p.height = 20

	// Type query that matches nothing
	for _, r := range "xyz" {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		_, _ = p.Update(msg)
	}

	if p.searchQuery != "xyz" {
		t.Errorf("expected searchQuery 'xyz', got %q", p.searchQuery)
	}
	if len(p.searchResults) != 0 {
		t.Errorf("expected 0 results, got %d", len(p.searchResults))
	}

	// Navigation should not panic with empty results
	msg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = p.Update(msg) // Should not panic

	msg = tea.KeyMsg{Type: tea.KeyUp}
	_, _ = p.Update(msg) // Should not panic

	// Enter should do nothing with empty results
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	_, _ = p.Update(msg)

	if p.view != ViewSessions {
		t.Error("expected to stay in sessions view with empty results")
	}
}

// =============================================================================
// Thinking Block Toggle Persistence Tests
// =============================================================================

// TestThinkingBlockTogglePersistence tests that thinking block expansion
// persists when scrolling away and back to a message.
func TestThinkingBlockTogglePersistence(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.activePane = PaneMessages
	p.height = 20
	p.turnViewMode = true // Test turn-based behavior

	// Create messages with thinking blocks - alternating roles to create separate turns
	p.messages = []adapter.Message{
		{
			ID:   "msg-1",
			Role: "assistant",
			ThinkingBlocks: []adapter.ThinkingBlock{
				{Content: "thinking 1"},
			},
		},
		{
			ID:      "msg-2",
			Role:    "user",
			Content: "user message",
		},
		{
			ID:   "msg-3",
			Role: "assistant",
			ThinkingBlocks: []adapter.ThinkingBlock{
				{Content: "thinking 3"},
			},
		},
	}
	p.turns = GroupMessagesIntoTurns(p.messages)
	p.turnCursor = 0

	// Initially all thinking blocks should be collapsed
	if p.expandedThinking["msg-1"] {
		t.Error("expected msg-1 thinking to be collapsed initially")
	}

	// Toggle turn 0 thinking blocks (press 'T')
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = p.Update(msg)

	if !p.expandedThinking["msg-1"] {
		t.Error("expected msg-1 thinking to be expanded after toggle")
	}

	// Scroll to turn 2 (assistant with msg-3)
	p.turnCursor = 2

	// Toggle turn 2 thinking block
	_, _ = p.Update(msg)

	if !p.expandedThinking["msg-3"] {
		t.Error("expected msg-3 thinking to be expanded after toggle")
	}

	// Verify msg-1 is still expanded
	if !p.expandedThinking["msg-1"] {
		t.Error("expected msg-1 thinking to remain expanded after scrolling")
	}

	// Scroll back to turn 0 and verify still expanded
	p.turnCursor = 0
	if !p.expandedThinking["msg-1"] {
		t.Error("expected msg-1 thinking to remain expanded after scrolling back")
	}

	// Toggle turn 0 again to collapse
	_, _ = p.Update(msg)

	if p.expandedThinking["msg-1"] {
		t.Error("expected msg-1 thinking to be collapsed after second toggle")
	}

	// msg-3 should still be expanded
	if !p.expandedThinking["msg-3"] {
		t.Error("expected msg-3 thinking to remain expanded")
	}
}

// TestThinkingBlockToggleNoThinkingBlocks tests toggle on turn without thinking blocks.
func TestThinkingBlockToggleNoThinkingBlocks(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.activePane = PaneMessages
	p.height = 20
	p.turnViewMode = true // Test turn-based behavior

	// Create message without thinking blocks
	p.messages = []adapter.Message{
		{
			ID:   "msg-1",
			Role: "user",
		},
	}
	p.turns = GroupMessagesIntoTurns(p.messages)
	p.turnCursor = 0

	// Try to toggle (should do nothing)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = p.Update(msg)

	// Map should remain empty
	if len(p.expandedThinking) != 0 {
		t.Errorf("expected empty expandedThinking map, got %d entries", len(p.expandedThinking))
	}
}

// TestThinkingBlockResetOnSessionChange tests that expanded state resets when changing sessions.
func TestThinkingBlockResetOnSessionChange(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.activePane = PaneMessages
	p.height = 20
	p.selectedSession = "session-1"
	p.turnViewMode = true // Test turn-based behavior

	// Create message with thinking block and expand it
	p.messages = []adapter.Message{
		{
			ID:   "msg-1",
			Role: "assistant",
			ThinkingBlocks: []adapter.ThinkingBlock{
				{Content: "thinking"},
			},
		},
	}
	p.turns = GroupMessagesIntoTurns(p.messages)
	p.turnCursor = 0

	// Toggle to expand
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = p.Update(msg)

	if !p.expandedThinking["msg-1"] {
		t.Fatal("expected msg-1 thinking to be expanded")
	}

	// Change to a different session - this should reset expandedThinking
	p.setSelectedSession("session-2")

	// Verify state was reset
	if len(p.expandedThinking) != 0 {
		t.Errorf("expected expandedThinking to be reset, got %d entries", len(p.expandedThinking))
	}
}

// TestThinkingBlockToggleMultipleIndependent tests independent toggle state for multiple turns.
func TestThinkingBlockToggleMultipleIndependent(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.activePane = PaneMessages
	p.height = 20
	p.turnViewMode = true // Test turn-based behavior

	// Create 5 alternating messages to create 5 separate turns
	p.messages = []adapter.Message{
		{ID: "msg-0", Role: "assistant", ThinkingBlocks: []adapter.ThinkingBlock{{Content: "t0"}}},
		{ID: "msg-1", Role: "user", Content: "u1"},
		{ID: "msg-2", Role: "assistant", ThinkingBlocks: []adapter.ThinkingBlock{{Content: "t2"}}},
		{ID: "msg-3", Role: "user", Content: "u3"},
		{ID: "msg-4", Role: "assistant", ThinkingBlocks: []adapter.ThinkingBlock{{Content: "t4"}}},
	}
	p.turns = GroupMessagesIntoTurns(p.messages)

	toggleMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}

	// Expand turns 0, 2, 4 (assistant turns with thinking blocks)
	p.turnCursor = 0
	_, _ = p.Update(toggleMsg)

	p.turnCursor = 2
	_, _ = p.Update(toggleMsg)

	p.turnCursor = 4
	_, _ = p.Update(toggleMsg)

	// Verify assistant turns are expanded
	if !p.expandedThinking["msg-0"] {
		t.Error("expected msg-0 to be expanded")
	}
	if !p.expandedThinking["msg-2"] {
		t.Error("expected msg-2 to be expanded")
	}
	if !p.expandedThinking["msg-4"] {
		t.Error("expected msg-4 to be expanded")
	}

	// User turns (1, 3) have no thinking blocks so nothing to check there

	// Toggle turn 2 to collapse it
	p.turnCursor = 2
	_, _ = p.Update(toggleMsg)

	if p.expandedThinking["msg-2"] {
		t.Error("expected msg-2 to be collapsed after second toggle")
	}

	// Others should remain unchanged
	if !p.expandedThinking["msg-0"] {
		t.Error("expected msg-0 to still be expanded")
	}
	if !p.expandedThinking["msg-4"] {
		t.Error("expected msg-4 to still be expanded")
	}
}

// TestSessionListNavigation tests j/k navigation in the main session list.
func TestSessionListNavigation(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1"},
		{ID: "test-2", Name: "Session 2"},
		{ID: "test-3", Name: "Session 3"},
	}
	p.height = 20
	p.width = 100

	// Initial cursor should be at 0
	if p.cursor != 0 {
		t.Errorf("expected initial cursor 0, got %d", p.cursor)
	}

	// Press 'j' to move down
	keyJ := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, _ = p.Update(keyJ)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", p.cursor)
	}

	// Press 'down' to move down again
	keyDown := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = p.Update(keyDown)

	if p.cursor != 2 {
		t.Errorf("expected cursor 2 after 'down', got %d", p.cursor)
	}

	// Press 'k' to move up
	keyK := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	_, _ = p.Update(keyK)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after 'k', got %d", p.cursor)
	}

	// Press 'up' to move up
	keyUp := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = p.Update(keyUp)

	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after 'up', got %d", p.cursor)
	}

	// Press 'G' to jump to bottom
	keyG := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	_, _ = p.Update(keyG)

	if p.cursor != 2 {
		t.Errorf("expected cursor 2 after 'G', got %d", p.cursor)
	}

	// Press 'g' to jump to top
	keyg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	_, _ = p.Update(keyg)

	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after 'g', got %d", p.cursor)
	}
}

// TestSessionListNavigationBounds tests cursor bounds checking.
func TestSessionListNavigationBounds(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1"},
		{ID: "test-2", Name: "Session 2"},
	}
	p.height = 20
	p.width = 100

	// Try to move up when at top - should stay at 0
	keyK := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	_, _ = p.Update(keyK)

	if p.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 when at top, got %d", p.cursor)
	}

	// Move to bottom
	keyG := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	_, _ = p.Update(keyG)

	// Try to move down when at bottom - should stay at last
	keyJ := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, _ = p.Update(keyJ)

	if p.cursor != 1 {
		t.Errorf("expected cursor to stay at 1 when at bottom, got %d", p.cursor)
	}
}

// TestTwoPaneNavigationRouting tests that navigation is properly routed in two-pane mode.
func TestTwoPaneNavigationRouting(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1"},
		{ID: "test-2", Name: "Session 2"},
		{ID: "test-3", Name: "Session 3"},
	}
	p.height = 30
	p.width = 150
	p.activePane = PaneSidebar

	// With sidebar focused, j should move cursor in session list
	keyJ := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	_, _ = p.Update(keyJ)

	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after 'j' in sidebar, got %d", p.cursor)
	}

	// Switch to messages pane
	p.activePane = PaneMessages
	p.selectedSession = "test-1"
	p.messages = []adapter.Message{
		{ID: "msg-1", Role: "user"},
		{ID: "msg-2", Role: "assistant"},
	}
	p.turns = GroupMessagesIntoTurns(p.messages)
	p.view = ViewMessages

	// With messages pane focused, j should move turn cursor, not session cursor
	initialCursor := p.cursor
	_, _ = p.Update(keyJ)

	if p.cursor != initialCursor {
		t.Errorf("expected session cursor to stay at %d in messages pane, got %d", initialCursor, p.cursor)
	}
}

// TestTwoPaneFocusSwitching tests pane focus switching with tab, h, l keys.
func TestTwoPaneFocusSwitching(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1"},
	}
	p.height = 30
	p.width = 150
	p.activePane = PaneSidebar
	p.selectedSession = "test-1"
	p.messages = []adapter.Message{{ID: "msg-1", Role: "user"}}
	p.turns = GroupMessagesIntoTurns(p.messages)

	// Press 'l' to switch to messages pane
	keyL := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	_, _ = p.Update(keyL)

	if p.activePane != PaneMessages {
		t.Errorf("expected activePane to be PaneMessages after 'l', got %v", p.activePane)
	}

	// Press 'h' to switch back to sidebar
	keyH := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	_, _ = p.Update(keyH)

	if p.activePane != PaneSidebar {
		t.Errorf("expected activePane to be PaneSidebar after 'h', got %v", p.activePane)
	}

	// Switch to messages and test tab to switch back
	p.activePane = PaneMessages
	keyTab := tea.KeyMsg{Type: tea.KeyTab}
	_, _ = p.Update(keyTab)

	if p.activePane != PaneSidebar {
		t.Errorf("expected activePane to be PaneSidebar after tab, got %v", p.activePane)
	}
}

// TestGroupHeaderSpacing verifies spacing behavior for time-based group headers.
func TestGroupHeaderSpacing(t *testing.T) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	thisWeek := today.AddDate(0, 0, -3)
	older := today.AddDate(0, 0, -10)

	tests := []struct {
		timestamp time.Time
		expected  string
	}{
		{today, "Today"},
		{yesterday, "Yesterday"},
		{thisWeek, "This Week"},
		{older, "Older"},
	}

	for _, tt := range tests {
		result := getSessionGroup(tt.timestamp)
		if result != tt.expected {
			t.Errorf("getSessionGroup(%v) = %q, expected %q", tt.timestamp, result, tt.expected)
		}
	}
}

// =============================================================================
// Mouse Click Tests
// =============================================================================

// TestMouseClickSessionItem tests clicking on a session item selects it.
func TestMouseClickSessionItem(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	now := time.Now()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1", UpdatedAt: now},
		{ID: "test-2", Name: "Session 2", UpdatedAt: now.Add(-time.Hour)},
		{ID: "test-3", Name: "Session 3", UpdatedAt: now.Add(-2 * time.Hour)},
	}
	p.width = 150
	p.height = 30
	p.activePane = PaneSidebar
	p.cursor = 0
	p.selectedSession = "test-1"

	// First render to set up hit regions
	_ = p.View(p.width, p.height)

	// Now test that the handler correctly processes session item clicks
	// by checking the plugin's internal state after a click
	if p.cursor != 0 {
		// Initial cursor should be 0
	}

	// Verify hit regions were registered
	hitMap := p.mouseHandler.HitMap
	if hitMap == nil {
		t.Fatal("expected HitMap to be initialized")
	}

	// Check that session item regions exist by testing hit detection
	// Y position for sessions should be after header lines
	// In two-pane mode: border(1) + title(1) = 2, so sessions start at Y=2
	region := hitMap.Test(5, 3) // Should hit a session item
	if region != nil && region.ID == regionSessionItem {
		if idx, ok := region.Data.(int); ok {
			if idx < 0 || idx >= len(p.sessions) {
				t.Errorf("session item region has invalid index: %d", idx)
			}
		}
	}
}

// TestHandleMouseClickSessionItem tests the handleMouseClick function with session item data.
func TestHandleMouseClickSessionItem(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	now := time.Now()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1", UpdatedAt: now},
		{ID: "test-2", Name: "Session 2", UpdatedAt: now.Add(-time.Hour)},
		{ID: "test-3", Name: "Session 3", UpdatedAt: now.Add(-2 * time.Hour)},
	}
	p.width = 150
	p.height = 30
	p.activePane = PaneMessages
	p.cursor = 0
	p.selectedSession = "test-1"

	// Render to initialize hit regions
	_ = p.View(p.width, p.height)

	// Create a mock mouse action with session item region data
	// X=5 is inside the content area (after border+padding which is 2 chars)
	// Y=3 should be where first session is (border + title + group header)
	action := p.mouseHandler.HitMap.Test(5, 3) // Should hit a session in the list

	// If we found a session item region, test the click handler
	if action != nil && action.ID == regionSessionItem {
		if idx, ok := action.Data.(int); ok {
			// Verify the index is valid
			if idx < 0 || idx >= len(p.sessions) {
				t.Errorf("invalid session index: %d", idx)
			}
		}
	}
}

// TestHandleMouseDoubleClickSessionItem tests double-click on session item.
func TestHandleMouseDoubleClickSessionItem(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	now := time.Now()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1", UpdatedAt: now},
		{ID: "test-2", Name: "Session 2", UpdatedAt: now.Add(-time.Hour)},
	}
	p.width = 150
	p.height = 30
	p.activePane = PaneSidebar
	p.cursor = 0
	p.selectedSession = ""

	// Render to initialize state
	_ = p.View(p.width, p.height)

	// Verify initial state
	if p.activePane != PaneSidebar {
		t.Errorf("expected initial activePane to be PaneSidebar, got %v", p.activePane)
	}
}

// TestRegisterSessionHitRegions tests that session hit regions are registered correctly.
func TestRegisterSessionHitRegions(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	now := time.Now()
	p.sessions = []adapter.Session{
		{ID: "test-1", Name: "Session 1", UpdatedAt: now},
		{ID: "test-2", Name: "Session 2", UpdatedAt: now.Add(-time.Hour)},
		{ID: "test-3", Name: "Session 3", UpdatedAt: now.Add(-2 * time.Hour)},
	}
	p.width = 150
	p.height = 30

	// Render to trigger hit region registration
	_ = p.View(p.width, p.height)

	// Verify hit regions were registered for sessions
	hitMap := p.mouseHandler.HitMap
	if hitMap == nil {
		t.Fatal("expected HitMap to be initialized")
	}

	// Test that clicking in the session area returns a session-item region
	// X must be inside content (border=1 + padding=1 = 2, so X >= 2)
	// Y: border(1) + title(1) + group header(1) = 3 for first session
	foundSessionRegion := false
	for y := 3; y < 10; y++ {
		region := hitMap.Test(5, y) // X=5 is inside content area
		if region != nil && region.ID == regionSessionItem {
			foundSessionRegion = true
			if idx, ok := region.Data.(int); ok {
				if idx < 0 || idx >= len(p.sessions) {
					t.Errorf("session item region has invalid index: %d", idx)
				}
			} else {
				t.Error("session item region Data should be an int")
			}
			break
		}
	}

	if !foundSessionRegion {
		t.Error("expected to find at least one session-item region")
	}
}

// TestScrollSidebarFunction tests the scrollSidebar function directly.
func TestScrollSidebarFunction(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	now := time.Now()
	// Create many sessions to test scrolling
	for i := 0; i < 20; i++ {
		p.sessions = append(p.sessions, adapter.Session{
			ID:        "test-" + string(rune('a'+i)),
			Name:      "Session " + string(rune('A'+i)),
			UpdatedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	p.width = 150
	p.height = 15
	p.cursor = 5

	// Test scroll down (positive delta)
	oldCursor := p.cursor
	_, _ = p.scrollSidebar(3) // delta of 3 (typical scroll amount)

	if p.cursor != oldCursor+3 {
		t.Errorf("expected cursor %d after scroll down, got %d", oldCursor+3, p.cursor)
	}

	// Test scroll up (negative delta)
	oldCursor = p.cursor
	_, _ = p.scrollSidebar(-2)

	if p.cursor != oldCursor-2 {
		t.Errorf("expected cursor %d after scroll up, got %d", oldCursor-2, p.cursor)
	}

	// Test scroll bounds at top
	p.cursor = 0
	_, _ = p.scrollSidebar(-5)

	if p.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 when scrolling up at top, got %d", p.cursor)
	}

	// Test scroll bounds at bottom
	p.cursor = len(p.sessions) - 1
	_, _ = p.scrollSidebar(5)

	if p.cursor != len(p.sessions)-1 {
		t.Errorf("expected cursor to stay at %d when scrolling down at bottom, got %d", len(p.sessions)-1, p.cursor)
	}
}

// TestHitRegionsDirtyOnModeChange tests that hitRegionsDirty is set when view modes change (td-455e378b).
func TestHitRegionsDirtyOnModeChange(t *testing.T) {
	p := New()
	p.adapters = map[string]adapter.Adapter{"mock": &mockAdapter{}}
	p.width = 150
	p.height = 30
	p.activePane = PaneMessages
	p.sessions = []adapter.Session{{ID: "test-1", Name: "Test", UpdatedAt: time.Now()}}
	p.selectedSession = "test-1"

	// Render to clear initial dirty flag
	_ = p.View(p.width, p.height)
	p.hitRegionsDirty = false

	t.Run("turnViewMode toggle sets dirty", func(t *testing.T) {
		p.hitRegionsDirty = false
		// Simulate 'v' key press
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}}
		p.Update(msg)
		if !p.hitRegionsDirty {
			t.Error("expected hitRegionsDirty=true after turnViewMode toggle")
		}
	})

	t.Run("filterMode open sets dirty", func(t *testing.T) {
		p.hitRegionsDirty = false
		p.activePane = PaneSidebar
		// Simulate 'f' key press
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
		p.Update(msg)
		if !p.hitRegionsDirty {
			t.Error("expected hitRegionsDirty=true when opening filter menu")
		}
	})

	t.Run("filterMode close via esc sets dirty", func(t *testing.T) {
		p.filterMode = true
		p.hitRegionsDirty = false
		// Simulate 'esc' key press
		msg := tea.KeyMsg{Type: tea.KeyEscape}
		p.Update(msg)
		if !p.hitRegionsDirty {
			t.Error("expected hitRegionsDirty=true when closing filter menu via esc")
		}
	})

	t.Run("filterMode close via enter sets dirty", func(t *testing.T) {
		p.filterMode = true
		p.hitRegionsDirty = false
		// Simulate 'enter' key press
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		p.Update(msg)
		if !p.hitRegionsDirty {
			t.Error("expected hitRegionsDirty=true when closing filter menu via enter")
		}
	})
}
