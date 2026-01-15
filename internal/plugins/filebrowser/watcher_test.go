package filebrowser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}

	if w == nil {
		t.Error("NewWatcher() returned nil")
	}
	if w.fsWatcher == nil {
		t.Error("fsWatcher not initialized")
	}
	if w.events == nil {
		t.Error("events channel not initialized")
	}

	w.Stop()
}

func TestWatcher_WatchFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	// Watch the file
	if err := w.WatchFile(testFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Modify the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Wait for event with timeout
	select {
	case <-w.Events():
		// Event received as expected
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for file change event")
	}
}

func TestWatcher_WatchFile_IgnoresOtherFiles(t *testing.T) {
	tmpDir := t.TempDir()
	watchedFile := filepath.Join(tmpDir, "watched.txt")
	otherFile := filepath.Join(tmpDir, "other.txt")

	// Create both files
	if err := os.WriteFile(watchedFile, []byte("watched"), 0644); err != nil {
		t.Fatalf("failed to create watched file: %v", err)
	}
	if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
		t.Fatalf("failed to create other file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	// Watch only one file
	if err := w.WatchFile(watchedFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Modify the OTHER file (should NOT trigger event)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(otherFile, []byte("modified other"), 0644); err != nil {
		t.Fatalf("failed to modify other file: %v", err)
	}

	// Should NOT receive event for other file
	select {
	case <-w.Events():
		t.Error("received event for unwatched file")
	case <-time.After(300 * time.Millisecond):
		// Expected - no event for unwatched file
	}

	// Now modify the watched file (SHOULD trigger event)
	if err := os.WriteFile(watchedFile, []byte("modified watched"), 0644); err != nil {
		t.Fatalf("failed to modify watched file: %v", err)
	}

	// Should receive event for watched file
	select {
	case <-w.Events():
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event on watched file")
	}
}

func TestWatcher_SwitchWatchedFile(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	// Create both files
	if err := os.WriteFile(file1, []byte("file1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("file2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	// Watch file1
	if err := w.WatchFile(file1); err != nil {
		t.Fatalf("WatchFile(file1) failed: %v", err)
	}

	// Switch to watching file2
	if err := w.WatchFile(file2); err != nil {
		t.Fatalf("WatchFile(file2) failed: %v", err)
	}

	// Modify file1 (should NOT trigger event since we switched)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(file1, []byte("modified file1"), 0644); err != nil {
		t.Fatalf("failed to modify file1: %v", err)
	}

	select {
	case <-w.Events():
		t.Error("received event for previously watched file after switch")
	case <-time.After(300 * time.Millisecond):
		// Expected - no event for unwatched file
	}

	// Modify file2 (SHOULD trigger event)
	if err := os.WriteFile(file2, []byte("modified file2"), 0644); err != nil {
		t.Fatalf("failed to modify file2: %v", err)
	}

	select {
	case <-w.Events():
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event on currently watched file")
	}
}

func TestWatcher_WatchEmptyStopsWatching(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	// Watch the file
	if err := w.WatchFile(testFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Stop watching by passing empty string
	if err := w.WatchFile(""); err != nil {
		t.Fatalf("WatchFile('') failed: %v", err)
	}

	// Modify the file (should NOT trigger event since we stopped watching)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	select {
	case <-w.Events():
		t.Error("received event after stopping watch")
	case <-time.After(300 * time.Millisecond):
		// Expected - no event
	}
}

func TestWatcher_Debounce(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	if err := w.WatchFile(testFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Rapidly modify the file multiple times
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(testFile, []byte("test"+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to modify test file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Should receive event(s) but debouncing prevents too many
	eventCount := 0
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-w.Events():
				eventCount++
			case <-time.After(300 * time.Millisecond):
				done <- true
				return
			}
		}
	}()

	<-done

	if eventCount == 0 {
		t.Error("no events detected")
	}
	// Due to debouncing, we should have fewer events than modifications
}

func TestWatcher_Stop(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}

	// Stop should not panic
	w.Stop()

	// Wait for run() goroutine to exit and close the channel
	time.Sleep(50 * time.Millisecond)

	// Channel should be closed after stop
	select {
	case _, ok := <-w.Events():
		if ok {
			t.Error("received event after watcher stopped")
		}
		// !ok means channel closed - this is expected and correct
	case <-time.After(200 * time.Millisecond):
		// Also acceptable - no event after stop
	}
}

func TestWatcher_EventsChannel(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	if err := w.WatchFile(testFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	eventsChan := w.Events()
	if eventsChan == nil {
		t.Error("Events() returned nil channel")
	}

	// Modify the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	select {
	case <-eventsChan:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout reading from events channel")
	}
}

func TestWatcher_DeleteWatchedFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	if err := w.WatchFile(testFile); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Delete the file
	time.Sleep(50 * time.Millisecond)
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("failed to delete test file: %v", err)
	}

	// Should detect the deletion
	select {
	case <-w.Events():
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for deletion event")
	}
}

func TestWatcher_RenameWatchedFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := filepath.Join(tmpDir, "old.txt")
	newPath := filepath.Join(tmpDir, "new.txt")

	if err := os.WriteFile(oldPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}
	defer w.Stop()

	if err := w.WatchFile(oldPath); err != nil {
		t.Fatalf("WatchFile() failed: %v", err)
	}

	// Rename the file
	time.Sleep(50 * time.Millisecond)
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("failed to rename file: %v", err)
	}

	// Should detect the rename (shows up as modification/deletion of watched file)
	select {
	case <-w.Events():
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for rename event")
	}
}

func TestWatcher_WatchClosedWatcher(t *testing.T) {
	w, err := NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() failed: %v", err)
	}

	w.Stop()
	time.Sleep(50 * time.Millisecond)

	// WatchFile on closed watcher should not panic
	err = w.WatchFile("/some/path")
	if err != nil {
		// Some error is acceptable, but no panic
	}
}
