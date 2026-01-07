package cursor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/marcus/sidecar/internal/adapter"
)

// NewWatcher creates a watcher for Cursor CLI session changes.
// It watches the workspace directory for changes to store.db files.
func NewWatcher(workspaceDir string) (<-chan adapter.Event, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the workspace directory and all session subdirectories
	if err := watcher.Add(workspaceDir); err != nil {
		watcher.Close()
		return nil, err
	}

	// Add existing session directories
	entries, err := os.ReadDir(workspaceDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				sessionDir := filepath.Join(workspaceDir, e.Name())
				_ = watcher.Add(sessionDir)
			}
		}
	}

	events := make(chan adapter.Event, 32)

	go func() {
		defer watcher.Close()
		defer close(events)

		// Debounce timer
		var debounceTimer *time.Timer
		debounceDelay := 100 * time.Millisecond

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Watch for store.db changes or new session directories
				if strings.HasSuffix(event.Name, "store.db") ||
					strings.HasSuffix(event.Name, "store.db-wal") {
					// Capture event for closure to avoid race condition
					capturedEvent := event

					// Debounce rapid events
					if debounceTimer != nil {
						debounceTimer.Stop()
					}
					debounceTimer = time.AfterFunc(debounceDelay, func() {
						// Extract session ID from path (use capturedEvent to avoid race)
						sessionID := filepath.Base(filepath.Dir(capturedEvent.Name))

						var eventType adapter.EventType
						switch {
						case capturedEvent.Op&fsnotify.Create != 0:
							eventType = adapter.EventSessionCreated
						case capturedEvent.Op&fsnotify.Write != 0:
							eventType = adapter.EventMessageAdded
						case capturedEvent.Op&fsnotify.Remove != 0:
							return
						default:
							eventType = adapter.EventSessionUpdated
						}

						select {
						case events <- adapter.Event{
							Type:      eventType,
							SessionID: sessionID,
						}:
						default:
							// Channel full, drop event
						}
					})
				} else if event.Op&fsnotify.Create != 0 {
					// New session directory created, add to watcher
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						_ = watcher.Add(event.Name)
					}
				}

			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Log error but continue watching
			}
		}
	}()

	return events, nil
}
