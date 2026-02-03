// Package tieredwatcher implements tiered file watching to reduce file descriptor count.
// HOT tier: 1-3 most recently active sessions use real-time fsnotify
// COLD tier: All other sessions use periodic polling (every 30s)
// Reduces FD count from ~9K to <500 (td-dca6fe).
package tieredwatcher

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/marcus/sidecar/internal/adapter"
)

const (
	// MaxHotSessions is the maximum number of sessions in the HOT tier with fsnotify.
	MaxHotSessions = 3
	// ColdPollInterval is how often COLD tier sessions are polled for changes.
	ColdPollInterval = 30 * time.Second
	// HotInactivityTimeout demotes sessions to COLD after this period without activity.
	HotInactivityTimeout = 5 * time.Minute
)

// SessionInfo tracks a watched session's path and modification time.
type SessionInfo struct {
	ID       string    // Session ID (e.g., filename without extension)
	Path     string    // Full path to session file
	ModTime  time.Time // Last known modification time
	LastHot  time.Time // When this session was last in HOT tier or accessed
	FileSize int64     // Last known file size
}

// TieredWatcher manages tiered watching for a single adapter's sessions.
type TieredWatcher struct {
	mu sync.Mutex

	// Session tracking
	sessions map[string]*SessionInfo // session ID -> info
	hotIDs   []string                // session IDs currently in HOT tier

	// fsnotify watcher for HOT tier (watches directory, not individual files)
	watcher   *fsnotify.Watcher
	watchDirs map[string]bool // directories being watched

	// Polling for COLD tier
	pollTicker *time.Ticker
	pollDone   chan struct{}

	// Output channel
	events chan adapter.Event
	closed bool

	// Configuration
	rootDir     string                              // Root directory to watch
	filePattern string                              // File extension pattern (e.g., ".jsonl")
	extractID   func(path string) string            // Extract session ID from path
	scanDir     func(dir string) ([]SessionInfo, error) // Scan directory for sessions
}

// Config holds configuration for creating a TieredWatcher.
type Config struct {
	// RootDir is the root directory to watch
	RootDir string
	// FilePattern is the file extension to watch (e.g., ".jsonl")
	FilePattern string
	// ExtractID extracts session ID from a file path
	ExtractID func(path string) string
	// ScanDir scans a directory and returns session info (optional, for COLD tier)
	ScanDir func(dir string) ([]SessionInfo, error)
}

// New creates a new TieredWatcher.
func New(cfg Config) (*TieredWatcher, <-chan adapter.Event, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	tw := &TieredWatcher{
		sessions:    make(map[string]*SessionInfo),
		hotIDs:      make([]string, 0, MaxHotSessions),
		watcher:     watcher,
		watchDirs:   make(map[string]bool),
		events:      make(chan adapter.Event, 32),
		rootDir:     cfg.RootDir,
		filePattern: cfg.FilePattern,
		extractID:   cfg.ExtractID,
		scanDir:     cfg.ScanDir,
	}

	// Watch the root directory
	if err := watcher.Add(cfg.RootDir); err != nil {
		watcher.Close()
		return nil, nil, err
	}
	tw.watchDirs[cfg.RootDir] = true

	// Start background goroutines
	tw.pollDone = make(chan struct{})
	tw.pollTicker = time.NewTicker(ColdPollInterval)

	go tw.watchLoop()
	go tw.pollLoop()
	go tw.demotionLoop()

	return tw, tw.events, nil
}

// PromoteToHot promotes a session to the HOT tier (e.g., when user selects it).
func (tw *TieredWatcher) PromoteToHot(sessionID string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	info, ok := tw.sessions[sessionID]
	if !ok {
		return
	}
	info.LastHot = time.Now()

	// Check if already in HOT tier
	for _, id := range tw.hotIDs {
		if id == sessionID {
			return
		}
	}

	// Add to HOT tier
	tw.hotIDs = append(tw.hotIDs, sessionID)

	// If we have too many HOT sessions, demote the oldest
	if len(tw.hotIDs) > MaxHotSessions {
		tw.demoteOldestLocked()
	}

	// Watch the session's directory if not already watched
	dir := filepath.Dir(info.Path)
	if !tw.watchDirs[dir] {
		if err := tw.watcher.Add(dir); err == nil {
			tw.watchDirs[dir] = true
		}
	}
}

// RegisterSession adds a session to tracking (starts in COLD tier).
func (tw *TieredWatcher) RegisterSession(id, path string) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.sessions[id] != nil {
		return // Already registered
	}

	info := &SessionInfo{
		ID:   id,
		Path: path,
	}

	// Get initial file info
	if stat, err := os.Stat(path); err == nil {
		info.ModTime = stat.ModTime()
		info.FileSize = stat.Size()
	}

	tw.sessions[id] = info
}

// RegisterSessions adds multiple sessions to tracking.
func (tw *TieredWatcher) RegisterSessions(sessions []SessionInfo) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	for _, s := range sessions {
		if tw.sessions[s.ID] != nil {
			continue
		}
		info := &SessionInfo{
			ID:       s.ID,
			Path:     s.Path,
			ModTime:  s.ModTime,
			FileSize: s.FileSize,
		}
		tw.sessions[s.ID] = info
	}

	// Auto-promote the N most recently modified sessions to HOT
	tw.autoPromoteRecentLocked()
}

// autoPromoteRecentLocked promotes the most recently modified sessions to HOT tier.
// Must be called with tw.mu held.
func (tw *TieredWatcher) autoPromoteRecentLocked() {
	if len(tw.sessions) == 0 {
		return
	}

	// Sort sessions by ModTime descending
	type sessionTime struct {
		id      string
		modTime time.Time
	}
	sorted := make([]sessionTime, 0, len(tw.sessions))
	for id, info := range tw.sessions {
		sorted = append(sorted, sessionTime{id: id, modTime: info.ModTime})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].modTime.After(sorted[j].modTime)
	})

	// Promote top N to HOT
	for i := 0; i < MaxHotSessions && i < len(sorted); i++ {
		id := sorted[i].id
		info := tw.sessions[id]
		info.LastHot = time.Now()

		// Add to HOT tier if not already there
		found := false
		for _, hotID := range tw.hotIDs {
			if hotID == id {
				found = true
				break
			}
		}
		if !found {
			tw.hotIDs = append(tw.hotIDs, id)
		}

		// Watch the session's directory
		dir := filepath.Dir(info.Path)
		if !tw.watchDirs[dir] {
			if err := tw.watcher.Add(dir); err == nil {
				tw.watchDirs[dir] = true
			}
		}
	}
}

// demoteOldestLocked removes the oldest session from HOT tier.
// Must be called with tw.mu held.
func (tw *TieredWatcher) demoteOldestLocked() {
	if len(tw.hotIDs) == 0 {
		return
	}

	// Find oldest by LastHot time
	oldestIdx := 0
	oldestTime := time.Now()
	for i, id := range tw.hotIDs {
		if info, ok := tw.sessions[id]; ok && info.LastHot.Before(oldestTime) {
			oldestTime = info.LastHot
			oldestIdx = i
		}
	}

	// Remove from HOT tier
	tw.hotIDs = append(tw.hotIDs[:oldestIdx], tw.hotIDs[oldestIdx+1:]...)
}

// watchLoop handles fsnotify events for HOT tier sessions.
func (tw *TieredWatcher) watchLoop() {
	var debounceTimer *time.Timer
	var lastPath string
	debounceDelay := 100 * time.Millisecond

	var closed bool
	var mu sync.Mutex

	defer func() {
		mu.Lock()
		closed = true
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		mu.Unlock()
	}()

	for {
		select {
		case event, ok := <-tw.watcher.Events:
			if !ok {
				return
			}

			// Check if this is a file we care about
			if tw.filePattern != "" && filepath.Ext(event.Name) != tw.filePattern {
				continue
			}

			mu.Lock()
			lastPath = event.Name
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			capturedEvent := event
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				mu.Lock()
				defer mu.Unlock()
				if closed {
					return
				}

				tw.mu.Lock()
				sessionID := tw.extractID(lastPath)
				info := tw.sessions[sessionID]

				// Update mod time if this is a known session
				if info != nil {
					if stat, err := os.Stat(lastPath); err == nil {
						info.ModTime = stat.ModTime()
						info.FileSize = stat.Size()
					}
				}
				tw.mu.Unlock()

				var eventType adapter.EventType
				switch {
				case capturedEvent.Op&fsnotify.Create != 0:
					eventType = adapter.EventSessionCreated
				case capturedEvent.Op&fsnotify.Write != 0:
					eventType = adapter.EventMessageAdded
				case capturedEvent.Op&fsnotify.Remove != 0:
					return // Skip delete events
				default:
					eventType = adapter.EventSessionUpdated
				}

				select {
				case tw.events <- adapter.Event{
					Type:      eventType,
					SessionID: sessionID,
				}:
				default:
					// Channel full
				}
			})
			mu.Unlock()

		case _, ok := <-tw.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// pollLoop periodically checks COLD tier sessions for changes.
func (tw *TieredWatcher) pollLoop() {
	for {
		select {
		case <-tw.pollTicker.C:
			tw.pollColdSessions()
		case <-tw.pollDone:
			return
		}
	}
}

// pollColdSessions checks all COLD tier sessions for changes.
func (tw *TieredWatcher) pollColdSessions() {
	tw.mu.Lock()
	hotSet := make(map[string]bool, len(tw.hotIDs))
	for _, id := range tw.hotIDs {
		hotSet[id] = true
	}

	// Collect COLD sessions to check
	type checkInfo struct {
		id   string
		path string
		prev time.Time
		size int64
	}
	var toCheck []checkInfo
	for id, info := range tw.sessions {
		if !hotSet[id] {
			toCheck = append(toCheck, checkInfo{
				id:   id,
				path: info.Path,
				prev: info.ModTime,
				size: info.FileSize,
			})
		}
	}
	tw.mu.Unlock()

	// Check each COLD session outside the lock
	for _, c := range toCheck {
		stat, err := os.Stat(c.path)
		if err != nil {
			continue
		}

		// Check if file changed
		if stat.ModTime().After(c.prev) || stat.Size() != c.size {
			tw.mu.Lock()
			if info := tw.sessions[c.id]; info != nil {
				info.ModTime = stat.ModTime()
				info.FileSize = stat.Size()
			}
			tw.mu.Unlock()

			select {
			case tw.events <- adapter.Event{
				Type:      adapter.EventSessionUpdated,
				SessionID: c.id,
			}:
			default:
			}
		}
	}
}

// demotionLoop periodically demotes inactive HOT sessions to COLD.
func (tw *TieredWatcher) demotionLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tw.demoteInactive()
		case <-tw.pollDone:
			return
		}
	}
}

// demoteInactive demotes HOT sessions that have been inactive too long.
func (tw *TieredWatcher) demoteInactive() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	cutoff := time.Now().Add(-HotInactivityTimeout)
	var remaining []string
	for _, id := range tw.hotIDs {
		if info, ok := tw.sessions[id]; ok && info.LastHot.After(cutoff) {
			remaining = append(remaining, id)
		}
	}
	tw.hotIDs = remaining
}

// Close shuts down the watcher.
func (tw *TieredWatcher) Close() error {
	tw.mu.Lock()
	if tw.closed {
		tw.mu.Unlock()
		return nil
	}
	tw.closed = true
	tw.mu.Unlock()

	// Stop polling
	if tw.pollTicker != nil {
		tw.pollTicker.Stop()
	}
	close(tw.pollDone)

	// Close fsnotify watcher
	if tw.watcher != nil {
		tw.watcher.Close()
	}

	// Close events channel
	close(tw.events)
	return nil
}

// Stats returns current watcher statistics.
func (tw *TieredWatcher) Stats() (hotCount, coldCount, watchedDirs int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	hotCount = len(tw.hotIDs)
	coldCount = len(tw.sessions) - len(tw.hotIDs)
	watchedDirs = len(tw.watchDirs)
	return
}

// TieredCloser wraps TieredWatcher to implement io.Closer.
type TieredCloser struct {
	tw *TieredWatcher
}

// Close implements io.Closer.
func (tc *TieredCloser) Close() error {
	return tc.tw.Close()
}

// NewCloser returns an io.Closer for the TieredWatcher.
func (tw *TieredWatcher) NewCloser() io.Closer {
	return &TieredCloser{tw: tw}
}

// Manager coordinates tiered watching across multiple adapters.
// It merges events from all adapter watchers into a single channel.
type Manager struct {
	mu       sync.Mutex
	watchers map[string]*TieredWatcher // adapter ID -> watcher
	events   chan adapter.Event
	closers  []io.Closer
	closed   bool
}

// NewManager creates a new tiered watcher manager.
func NewManager() *Manager {
	return &Manager{
		watchers: make(map[string]*TieredWatcher),
		events:   make(chan adapter.Event, 64),
	}
}

// AddWatcher adds a tiered watcher for an adapter and starts forwarding its events.
func (m *Manager) AddWatcher(adapterID string, tw *TieredWatcher, ch <-chan adapter.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.watchers[adapterID] = tw
	m.closers = append(m.closers, tw.NewCloser())

	// Forward events from this watcher to the merged channel
	go func() {
		for evt := range ch {
			m.mu.Lock()
			closed := m.closed
			m.mu.Unlock()
			if closed {
				return
			}
			select {
			case m.events <- evt:
			default:
			}
		}
	}()
}

// Events returns the merged event channel.
func (m *Manager) Events() <-chan adapter.Event {
	return m.events
}

// PromoteSession promotes a session to HOT tier across all watchers.
func (m *Manager) PromoteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tw := range m.watchers {
		tw.PromoteToHot(sessionID)
	}
}

// RegisterSession registers a session with the appropriate watcher.
func (m *Manager) RegisterSession(adapterID, sessionID, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tw, ok := m.watchers[adapterID]; ok {
		tw.RegisterSession(sessionID, path)
	}
}

// Stats returns aggregate statistics across all watchers.
func (m *Manager) Stats() (hotCount, coldCount, watchedDirs int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, tw := range m.watchers {
		h, c, w := tw.Stats()
		hotCount += h
		coldCount += c
		watchedDirs += w
	}
	return
}

// Close shuts down all watchers.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	closers := m.closers
	m.mu.Unlock()

	for _, c := range closers {
		c.Close()
	}
	close(m.events)
	return nil
}

// Closers returns all io.Closers for the manager's watchers.
func (m *Manager) Closers() []io.Closer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closers
}
