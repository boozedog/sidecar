package filebrowser

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the file system for changes.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	rootDir   string
	events    chan struct{}
	stop      chan struct{}
	debounce  *time.Timer
}

// NewWatcher creates a file system watcher for the given directory.
func NewWatcher(rootDir string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher: fsw,
		rootDir:   rootDir,
		events:    make(chan struct{}, 1),
		stop:      make(chan struct{}),
	}

	// Recursively add all directories (fsnotify doesn't watch subdirs automatically)
	if err := w.addRecursive(rootDir); err != nil {
		fsw.Close()
		return nil, err
	}

	go w.run()
	return w, nil
}

// addRecursive adds a directory and all its subdirectories to the watcher.
func (w *Watcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Return error for root dir, skip errors for subdirs
			if path == dir {
				return err
			}
			return nil // Skip unreadable subdirectories
		}
		if !d.IsDir() {
			return nil
		}
		// Skip common large/irrelevant directories
		name := d.Name()
		if name == ".git" || name == "node_modules" || name == "vendor" ||
			name == ".next" || name == "dist" || name == "build" ||
			name == "__pycache__" || name == ".venv" || name == "venv" ||
			name == ".idea" || name == ".vscode" {
			return filepath.SkipDir
		}
		// Skip hidden directories (except root)
		if path != dir && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		return w.fsWatcher.Add(path)
	})
}

// run processes file system events.
func (w *Watcher) run() {
	for {
		select {
		case <-w.stop:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			// Debounce: wait 100ms for more events before signaling
			if w.debounce != nil {
				w.debounce.Stop()
			}
			w.debounce = time.AfterFunc(100*time.Millisecond, func() {
				select {
				case w.events <- struct{}{}:
				default: // Channel full, skip
				}
			})

			// Watch newly created directories (recursively in case of mkdir -p)
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(event.Name)
				}
			}
		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Ignore errors, continue watching
		}
	}
}

// Events returns a channel that signals when files change.
func (w *Watcher) Events() <-chan struct{} {
	return w.events
}

// Stop shuts down the watcher.
func (w *Watcher) Stop() {
	close(w.stop)
	w.fsWatcher.Close()
}
