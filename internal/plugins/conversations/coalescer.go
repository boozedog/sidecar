package conversations

import (
	"sync"
	"time"

	"github.com/marcus/sidecar/internal/adapter"
)

const (
	defaultCoalesceWindow = 250 * time.Millisecond
	maxCoalesceWindow     = 5 * time.Second
	maxPendingSessionIDs  = 10 // Above this, trigger full refresh

	// sizeScaleFactor determines how much each 100MB adds to the debounce window
	sizeScaleFactor = 100 * 1024 * 1024 // 100MB
	sizeScaleAmount = 500 * time.Millisecond
)

// EventCoalescer batches rapid watch events into single refreshes.
// When events arrive faster than the coalesce window, they are
// accumulated and a single refresh is triggered after the window closes.
// td-190095: Uses dynamic window based on largest pending session's size.
type EventCoalescer struct {
	mu             sync.Mutex
	pendingIDs     map[string]struct{} // SessionIDs to refresh
	refreshAll     bool                // true if we need full refresh (empty ID received)
	timer          *time.Timer
	coalesceWindow time.Duration
	msgChan        chan<- CoalescedRefreshMsg // channel to send messages
	closed         bool                       // true after Stop() called, prevents send on closed channel
	pendingEpoch   uint64                     // Epoch from first event in batch (for stale detection)

	// Session size tracking for dynamic debounce (td-190095)
	sessionSizes map[string]int64
	sizeMu       sync.RWMutex
}

// NewEventCoalescer creates a coalescer with the given window duration.
// msgChan receives CoalescedRefreshMsg when the coalesce window closes.
func NewEventCoalescer(window time.Duration, msgChan chan<- CoalescedRefreshMsg) *EventCoalescer {
	if window == 0 {
		window = defaultCoalesceWindow
	}
	return &EventCoalescer{
		pendingIDs:     make(map[string]struct{}),
		sessionSizes:   make(map[string]int64),
		coalesceWindow: window,
		msgChan:        msgChan,
	}
}

// Add queues a sessionID for refresh. Empty string triggers full refresh.
// Uses dynamic window based on largest pending session (td-190095).
// The epoch parameter tracks the project context for stale detection.
func (c *EventCoalescer) Add(sessionID string, epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store epoch from first event in batch (subsequent events in same batch use this)
	if len(c.pendingIDs) == 0 && !c.refreshAll {
		c.pendingEpoch = epoch
	}

	if sessionID == "" {
		c.refreshAll = true
	} else {
		// Skip auto-reload for huge sessions (td-190095)
		if !c.shouldAutoReloadLocked(sessionID) {
			return
		}
		c.pendingIDs[sessionID] = struct{}{}
	}

	// Compute dynamic window based on largest pending session
	window := c.maxWindowForPendingLocked()

	// Reset timer - we wait for a quiet period
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(window, c.flush)
}

// UpdateSessionSize records the file size for a session (td-190095).
// Call this after loading sessions to enable dynamic debounce.
func (c *EventCoalescer) UpdateSessionSize(sessionID string, size int64) {
	c.sizeMu.Lock()
	c.sessionSizes[sessionID] = size
	c.sizeMu.Unlock()
}

// UpdateSessionSizes updates sizes for multiple sessions at once.
func (c *EventCoalescer) UpdateSessionSizes(sessions []adapter.Session) {
	c.sizeMu.Lock()
	for i := range sessions {
		c.sessionSizes[sessions[i].ID] = sessions[i].FileSize
	}
	c.sizeMu.Unlock()
}

// ShouldAutoReload returns whether auto-reload is enabled for a session.
// Sessions larger than HugeSessionThreshold (500MB) disable auto-reload.
func (c *EventCoalescer) ShouldAutoReload(sessionID string) bool {
	c.sizeMu.RLock()
	defer c.sizeMu.RUnlock()
	return c.shouldAutoReloadLocked(sessionID)
}

// shouldAutoReloadLocked requires sizeMu NOT held (acquires read lock).
func (c *EventCoalescer) shouldAutoReloadLocked(sessionID string) bool {
	c.sizeMu.RLock()
	size := c.sessionSizes[sessionID]
	c.sizeMu.RUnlock()
	return size < adapter.HugeSessionThreshold
}

// maxWindowForPendingLocked computes the max window across all pending sessions.
// Returns base window if no size info available.
func (c *EventCoalescer) maxWindowForPendingLocked() time.Duration {
	c.sizeMu.RLock()
	defer c.sizeMu.RUnlock()

	maxWindow := c.coalesceWindow
	for id := range c.pendingIDs {
		size := c.sessionSizes[id]
		if size == 0 {
			continue
		}
		// Scale: base + 500ms per 100MB
		scale := time.Duration(size/sizeScaleFactor) * sizeScaleAmount
		window := c.coalesceWindow + scale
		if window > maxWindow {
			maxWindow = window
		}
	}

	if maxWindow > maxCoalesceWindow {
		return maxCoalesceWindow
	}
	return maxWindow
}

// flush sends the coalesced refresh message and resets state.
// Called by timer when coalesce window closes.
func (c *EventCoalescer) flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if stopped (channel may be closed)
	if c.closed {
		return
	}

	// Collect pending IDs
	sessionIDs := make([]string, 0, len(c.pendingIDs))
	for id := range c.pendingIDs {
		sessionIDs = append(sessionIDs, id)
	}

	refreshAll := c.refreshAll || len(sessionIDs) > maxPendingSessionIDs
	epoch := c.pendingEpoch

	// Reset state
	c.pendingIDs = make(map[string]struct{})
	c.refreshAll = false
	c.timer = nil
	c.pendingEpoch = 0

	// Send message with lock held - safe because select/default prevents blocking
	if c.msgChan != nil {
		select {
		case c.msgChan <- CoalescedRefreshMsg{
			Epoch:      epoch,
			SessionIDs: sessionIDs,
			RefreshAll: refreshAll,
		}:
		default:
			// Channel full, drop message (next event will trigger refresh)
		}
	}
}

// Stop cancels any pending flush. Call when plugin is shutting down.
func (c *EventCoalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true // Prevent flush() from sending on closed channel

	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
}

// CoalescedRefreshMsg is sent when the coalesce window closes.
type CoalescedRefreshMsg struct {
	Epoch      uint64   // Epoch when request was issued (for stale detection)
	SessionIDs []string // Specific sessions to refresh (if not RefreshAll)
	RefreshAll bool     // If true, ignore SessionIDs and do full refresh
}

// GetEpoch implements plugin.EpochMessage.
func (m CoalescedRefreshMsg) GetEpoch() uint64 { return m.Epoch }
