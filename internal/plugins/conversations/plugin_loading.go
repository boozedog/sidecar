package conversations

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/adapter"
	"github.com/marcus/sidecar/internal/adapter/tieredwatcher"
	"github.com/marcus/sidecar/internal/app"
	"github.com/marcus/sidecar/internal/fdmonitor"
)

// Data loading and file watching methods

// loadSessions loads sessions from the adapter.
// Queries sessions from all related worktree paths to show cross-worktree conversations.
// Sessions from deleted worktrees are marked with "(deleted)" in their worktree name.
// Caches worktree paths and names to avoid git commands on every refresh (td-e74a4aaa).
// Serialized to prevent concurrent goroutines from accumulating file descriptors (td-023577).
func (p *Plugin) loadSessions() tea.Cmd {
	// Capture epoch for stale detection on project switch
	var epoch uint64
	if p.ctx != nil {
		epoch = p.ctx.Epoch
	}

	// Capture current cache state for goroutine (td-0e43c080: avoid race)
	cachedPaths := p.cachedWorktreePaths
	cachedNames := p.cachedWorktreeNames
	cacheTime := p.worktreeCacheTime
	adapters := p.adapters

	// Handle nil context (e.g., in tests)
	var workDir string
	if p.ctx != nil {
		workDir = p.ctx.WorkDir
	}

	return func() tea.Msg {
		// Serialize session loading to prevent FD accumulation (td-023577).
		// Multiple concurrent loadSessions() goroutines each opening session files
		// caused FD count to grow unbounded. Only allow one at a time.
		p.loadingMu.Lock()
		if p.loadingSessions {
			p.loadingMu.Unlock()
			return nil // Skip if another load is in progress
		}
		p.loadingSessions = true
		p.loadingMu.Unlock()

		if len(adapters) == 0 {
			p.loadingMu.Lock()
			p.loadingSessions = false
			p.loadingMu.Unlock()
			return SessionsLoadedMsg{Epoch: epoch}
		}

		// Check worktree cache (td-e74a4aaa)
		var worktreePaths []string
		var worktreeNames map[string]string
		var cacheUpdated bool
		cacheValid := cachedPaths != nil && time.Since(cacheTime) < worktreeCacheTTL

		if cacheValid {
			// Use cached data
			worktreePaths = cachedPaths
			worktreeNames = cachedNames
		} else {
			// Refresh cache - get all related worktree paths (main repo + all worktrees)
			worktreePaths = app.GetAllRelatedPaths(workDir)
			if len(worktreePaths) == 0 {
				// Not a git repo or no worktrees - just use current workdir
				worktreePaths = []string{workDir}
			}

			// Discover additional paths from adapters (finds deleted worktree conversations)
			mainPath := app.GetMainWorktreePath(workDir)
			if mainPath == "" {
				mainPath = workDir
			}
			pathSet := make(map[string]bool, len(worktreePaths))
			for _, path := range worktreePaths {
				pathSet[path] = true
			}
			for _, a := range adapters {
				if discoverer, ok := a.(adapter.ProjectDiscoverer); ok {
					discovered, _ := discoverer.DiscoverRelatedProjectDirs(mainPath)
					for _, path := range discovered {
						if !pathSet[path] {
							worktreePaths = append(worktreePaths, path)
							pathSet[path] = true
						}
					}
				}
			}

			// Compute worktree names
			worktreeNames = make(map[string]string)
			currentPath := workDir
			if absPath, err := filepath.Abs(currentPath); err == nil {
				currentPath = absPath
			}
			for _, wtPath := range worktreePaths {
				wtName := app.WorktreeNameForPath(workDir, wtPath)
				if wtName == "" && wtPath != currentPath {
					wtName = deriveWorktreeNameFromPath(wtPath, mainPath)
				}
				worktreeNames[wtPath] = wtName
			}

			// Mark cache as updated (td-0e43c080: Update() will store)
			cacheUpdated = true
		}

		// Get current working directory for worktree name comparison
		currentPath := workDir
		if absPath, err := filepath.Abs(currentPath); err == nil {
			currentPath = absPath
		}

		// Launch per-adapter goroutines that send directly to channel (td-7198a5)
		var wg sync.WaitGroup
		for id, a := range adapters {
			adapterID := id
			adpt := a
			wg.Add(1)
			go func() {
				defer wg.Done()
				var adapterSess []adapter.Session
				for _, wtPath := range worktreePaths {
					wtSessions, err := adpt.Sessions(wtPath)
					if err != nil {
						continue
					}
					wtName := worktreeNames[wtPath]
					for i := range wtSessions {
						if wtSessions[i].AdapterID == "" {
							wtSessions[i].AdapterID = adapterID
						}
						if wtSessions[i].AdapterName == "" {
							wtSessions[i].AdapterName = adpt.Name()
						}
						if wtSessions[i].AdapterIcon == "" {
							wtSessions[i].AdapterIcon = adpt.Icon()
						}
						absWtPath := wtPath
						if abs, err := filepath.Abs(wtPath); err == nil {
							absWtPath = abs
						}
						if absWtPath != currentPath {
							wtSessions[i].WorktreeName = wtName
							wtSessions[i].WorktreePath = absWtPath
						}
						adapterSess = append(adapterSess, wtSessions[i])
					}
				}
				// Mark sessions from deleted worktrees
				for i := range adapterSess {
					if adapterSess[i].WorktreePath != "" {
						if _, err := os.Stat(adapterSess[i].WorktreePath); os.IsNotExist(err) {
							adapterSess[i].WorktreeName = adapterSess[i].WorktreeName + " (deleted)"
						}
					}
				}
				p.adapterBatchChan <- AdapterBatchMsg{
					Epoch:    epoch,
					Sessions: adapterSess,
				}
			}()
		}

		// Coordinator goroutine: wait for all, send final signal, release lock
		go func() {
			wg.Wait()
			fdmonitor.Check(nil)

			finalMsg := AdapterBatchMsg{Epoch: epoch, Final: true}
			if cacheUpdated {
				finalMsg.WorktreePaths = worktreePaths
				finalMsg.WorktreeNames = worktreeNames
			}
			p.adapterBatchChan <- finalMsg

			p.loadingMu.Lock()
			p.loadingSessions = false
			p.loadingMu.Unlock()
		}()

		// Return immediately — adapter goroutines will send results to channel
		return LoadingStartedMsg{Epoch: epoch}
	}
}

// refreshSessions updates only specific sessions in-place (td-2b8ebe).
// Falls back to full loadSessions() if adapters don't support targeted refresh.
func (p *Plugin) refreshSessions(sessionIDs []string) tea.Cmd {
	// Capture epoch for stale detection on project switch
	var epoch uint64
	if p.ctx != nil {
		epoch = p.ctx.Epoch
	}

	adapters := p.adapters
	currentSessions := p.sessions

	return func() tea.Msg {
		if len(adapters) == 0 {
			return SessionsLoadedMsg{Epoch: epoch}
		}

		// Build lookup of current sessions by ID for in-place update
		sessionMap := make(map[string]int, len(currentSessions))
		for i, s := range currentSessions {
			sessionMap[s.ID] = i
		}

		updated := make([]adapter.Session, len(currentSessions))
		copy(updated, currentSessions)
		changed := false

		for _, sessionID := range sessionIDs {
			var refreshed *adapter.Session

			// Try each adapter's TargetedRefresher interface
			for _, a := range adapters {
				if tr, ok := a.(adapter.TargetedRefresher); ok {
					s, err := tr.SessionByID(sessionID)
					if err == nil && s != nil {
						refreshed = s
						break
					}
				}
			}

			if refreshed == nil {
				continue
			}

			if idx, exists := sessionMap[sessionID]; exists {
				// Preserve worktree fields from existing session
				refreshed.WorktreeName = updated[idx].WorktreeName
				refreshed.WorktreePath = updated[idx].WorktreePath
				updated[idx] = *refreshed
			} else {
				// New session - append
				updated = append(updated, *refreshed)
			}
			changed = true
		}

		if !changed {
			return SessionsLoadedMsg{Epoch: epoch, Sessions: updated}
		}

		// Re-sort by UpdatedAt descending
		sort.Slice(updated, func(i, j int) bool {
			return updated[i].UpdatedAt.After(updated[j].UpdatedAt)
		})

		return SessionsLoadedMsg{Epoch: epoch, Sessions: updated}
	}
}

// loadMessages loads messages for a session with pagination support (td-313ea851).
func (p *Plugin) loadMessages(sessionID string) tea.Cmd {
	// Capture epoch for stale detection on project switch
	var epoch uint64
	if p.ctx != nil {
		epoch = p.ctx.Epoch
	}

	offset := p.messageOffset
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return MessagesLoadedMsg{Epoch: epoch}
		}
		adapter := p.adapterForSession(sessionID)
		if adapter == nil {
			return MessagesLoadedMsg{Epoch: epoch}
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
			Epoch:      epoch,
			SessionID:  sessionID,
			Messages:   messages,
			TotalCount: totalCount,
			Offset:     resultOffset,
		}
	}
}

// startWatcher starts watching for session changes.
// Uses tiered watching (td-dca6fe) to reduce FD count:
// - HOT tier: 1-3 most recently active sessions use real-time fsnotify
// - COLD tier: All other sessions use periodic polling (every 30s)
// Global-scoped adapters (codex, warp) only create one watcher to avoid duplicates (td-7a72b6f7).
func (p *Plugin) startWatcher() tea.Cmd {
	return func() tea.Msg {
		if len(p.adapters) == 0 {
			return WatchStartedMsg{Channel: nil}
		}

		// Create context for cancellation (td-eb2699b4)
		ctx, cancel := context.WithCancel(context.Background())
		p.watchCancel = cancel

		// Get all related worktree paths (main repo + all worktrees)
		worktreePaths := app.GetAllRelatedPaths(p.ctx.WorkDir)
		if len(worktreePaths) == 0 {
			// Not a git repo or no worktrees - just use current workdir
			worktreePaths = []string{p.ctx.WorkDir}
		}

		// Create tiered watcher manager (td-dca6fe)
		manager := tieredwatcher.NewManager()
		p.tieredManager = manager

		merged := make(chan adapter.Event, 32)
		var wg sync.WaitGroup
		watchCount := 0

		// Watch all worktree paths with each adapter
		// Global-scoped adapters only watch once to avoid duplicate events (td-7a72b6f7)
		for adapterID, a := range p.adapters {
			// Check if adapter has global watch scope
			isGlobal := false
			if scopeProvider, ok := a.(adapter.WatchScopeProvider); ok {
				isGlobal = scopeProvider.WatchScope() == adapter.WatchScopeGlobal
			}

			pathsToWatch := worktreePaths
			if isGlobal {
				// Global adapters only need one watch call (uses first path)
				pathsToWatch = worktreePaths[:1]
			}

			for _, wtPath := range pathsToWatch {
				ch, closer, err := a.Watch(wtPath)
				if err != nil || ch == nil || closer == nil {
					if closer != nil {
						_ = closer.Close()
					}
					continue
				}

				// Register sessions with tiered manager for COLD tier polling
				sessions, _ := a.Sessions(wtPath)
				for _, s := range sessions {
					// Session ID is already unique per adapter
					manager.RegisterSession(adapterID, s.ID, "")
				}

				watchCount++
				wg.Add(1)
				go func(c <-chan adapter.Event, cl io.Closer, aid string) {
					defer wg.Done()
					defer cl.Close()
					for {
						select {
						case <-ctx.Done():
							return
						case evt, ok := <-c:
							if !ok {
								return
							}
							// Promote session to HOT tier on activity (td-dca6fe)
							if evt.SessionID != "" {
								manager.PromoteSession(evt.SessionID)
							}
							select {
							case merged <- evt:
							default:
							}
						}
					}
				}(ch, closer, adapterID)
			}
		}

		if watchCount == 0 {
			manager.Close()
			p.tieredManager = nil
			return WatchStartedMsg{Channel: nil, Closers: nil}
		}

		// Close merged channel when all source channels are done
		go func() {
			wg.Wait()
			close(merged)
		}()

		return WatchStartedMsg{Channel: merged, Closers: nil}
	}
}

// listenForWatchEvents waits for the next file system event.
func (p *Plugin) listenForWatchEvents() tea.Cmd {
	if p.watchChan == nil {
		return nil
	}
	// Capture epoch for stale detection on project switch
	var epoch uint64
	if p.ctx != nil {
		epoch = p.ctx.Epoch
	}
	return func() tea.Msg {
		evt, ok := <-p.watchChan
		if !ok {
			// Channel closed
			return nil
		}
		return WatchEventMsg{Epoch: epoch, SessionID: evt.SessionID}
	}
}

// AdapterBatchMsg delivers sessions from a single adapter incrementally (td-7198a5).
type AdapterBatchMsg struct {
	Epoch         uint64
	Sessions      []adapter.Session
	Final         bool // true when all adapters are done
	WorktreePaths []string
	WorktreeNames map[string]string
}

// GetEpoch implements plugin.EpochMessage.
func (m AdapterBatchMsg) GetEpoch() uint64 { return m.Epoch }

// LoadingStartedMsg signals that adapter goroutines have been launched (td-7198a5).
type LoadingStartedMsg struct {
	Epoch uint64
}

// GetEpoch implements plugin.EpochMessage.
func (m LoadingStartedMsg) GetEpoch() uint64 { return m.Epoch }

// listenForAdapterBatch waits for incremental adapter session batches (td-7198a5).
func (p *Plugin) listenForAdapterBatch() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-p.adapterBatchChan
		if !ok {
			return nil
		}
		return msg
	}
}

// listenForCoalescedRefresh waits for coalesced refresh messages.
func (p *Plugin) listenForCoalescedRefresh() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-p.coalesceChan
		if !ok {
			return nil // Channel closed (td-e2791614)
		}
		return msg
	}
}

// loadUsage loads usage stats for a session (placeholder for future implementation).
func (p *Plugin) loadUsage(sessionID string) tea.Cmd {
	// Usage is already computed from messages in MessagesLoadedMsg handler
	return nil
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

// deriveWorktreeNameFromPath extracts the worktree name from a directory path.
// For paths like '/Users/foo/code/myrepo-feature-auth' with main repo 'myrepo',
// returns 'feature-auth'. This is used for deleted worktrees where git no longer
// has branch information.
func deriveWorktreeNameFromPath(wtPath, mainPath string) string {
	dirName := filepath.Base(wtPath)
	repoName := filepath.Base(mainPath)

	// If directory starts with repo name + hyphen, strip it
	// This handles the {repo}-{name} naming convention
	prefix := repoName + "-"
	if strings.HasPrefix(dirName, prefix) {
		return strings.TrimPrefix(dirName, prefix)
	}

	// Fallback: just use directory name
	return dirName
}
