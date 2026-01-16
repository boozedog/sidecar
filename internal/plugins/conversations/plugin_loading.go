package conversations

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/adapter"
	"github.com/marcus/sidecar/internal/app"
)

// Data loading and file watching methods

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

		// Discover additional paths from adapters (finds deleted worktree conversations)
		mainPath := app.GetMainWorktreePath(p.ctx.WorkDir)
		if mainPath == "" {
			mainPath = p.ctx.WorkDir
		}
		pathSet := make(map[string]bool, len(worktreePaths))
		for _, path := range worktreePaths {
			pathSet[path] = true
		}
		for _, a := range p.adapters {
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
