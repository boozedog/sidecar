package tieredwatcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, ch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer tw.Close()

	if ch == nil {
		t.Fatal("events channel is nil")
	}
}

func TestRegisterSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test session file
	sessionPath := filepath.Join(tmpDir, "test-session.jsonl")
	if err := os.WriteFile(sessionPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer tw.Close()

	tw.RegisterSession("test-session", sessionPath)

	tw.mu.Lock()
	info, ok := tw.sessions["test-session"]
	tw.mu.Unlock()

	if !ok {
		t.Fatal("session not registered")
	}
	if info.Path != sessionPath {
		t.Errorf("session path = %q, want %q", info.Path, sessionPath)
	}
}

func TestPromoteToHot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test session files
	for i := 0; i < 5; i++ {
		path := filepath.Join(tmpDir, "session-"+string('a'+byte(i))+".jsonl")
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
	}

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer tw.Close()

	// Register sessions
	for i := 0; i < 5; i++ {
		id := "session-" + string('a'+byte(i))
		path := filepath.Join(tmpDir, id+".jsonl")
		tw.RegisterSession(id, path)
	}

	// Promote more than MaxHotSessions
	tw.PromoteToHot("session-a")
	tw.PromoteToHot("session-b")
	tw.PromoteToHot("session-c")
	tw.PromoteToHot("session-d") // This should demote the oldest

	tw.mu.Lock()
	hotCount := len(tw.hotIDs)
	tw.mu.Unlock()

	if hotCount > MaxHotSessions {
		t.Errorf("hot sessions = %d, want <= %d", hotCount, MaxHotSessions)
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer tw.Close()

	// Create and register sessions
	for i := 0; i < 5; i++ {
		id := "session-" + string('a'+byte(i))
		path := filepath.Join(tmpDir, id+".jsonl")
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
		tw.RegisterSession(id, path)
	}

	// Promote some to HOT
	tw.PromoteToHot("session-a")
	tw.PromoteToHot("session-b")

	hot, cold, dirs := tw.Stats()

	if hot != 2 {
		t.Errorf("hot = %d, want 2", hot)
	}
	if cold != 3 {
		t.Errorf("cold = %d, want 3", cold)
	}
	if dirs < 1 {
		t.Errorf("watchedDirs = %d, want >= 1", dirs)
	}
}

func TestManager(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewManager()
	defer manager.Close()

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, ch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	manager.AddWatcher("test-adapter", tw, ch)

	// Create and register a session
	sessionPath := filepath.Join(tmpDir, "test-session.jsonl")
	if err := os.WriteFile(sessionPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	manager.RegisterSession("test-adapter", "test-session", sessionPath)

	hot, cold, _ := manager.Stats()
	if hot+cold != 1 {
		t.Errorf("total sessions = %d, want 1", hot+cold)
	}
}

func TestManagerPromoteSession(t *testing.T) {
	tmpDir := t.TempDir()

	manager := NewManager()
	defer manager.Close()

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, ch, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	manager.AddWatcher("test-adapter", tw, ch)

	// Create and register sessions
	for i := 0; i < 3; i++ {
		id := "session-" + string('a'+byte(i))
		path := filepath.Join(tmpDir, id+".jsonl")
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
		tw.RegisterSession(id, path)
	}

	// Promote a session through the manager
	manager.PromoteSession("session-a")

	tw.mu.Lock()
	found := false
	for _, id := range tw.hotIDs {
		if id == "session-a" {
			found = true
			break
		}
	}
	tw.mu.Unlock()

	if !found {
		t.Error("session-a not found in HOT tier after promotion")
	}
}

func TestRegisterSessions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test session files with different modification times
	sessions := []SessionInfo{}
	for i := 0; i < 5; i++ {
		id := "session-" + string('a'+byte(i))
		path := filepath.Join(tmpDir, id+".jsonl")
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}
		// Set modification times to be different
		modTime := time.Now().Add(-time.Duration(5-i) * time.Hour)
		os.Chtimes(path, modTime, modTime)

		stat, _ := os.Stat(path)
		sessions = append(sessions, SessionInfo{
			ID:      id,
			Path:    path,
			ModTime: stat.ModTime(),
		})
	}

	cfg := Config{
		RootDir:     tmpDir,
		FilePattern: ".jsonl",
		ExtractID: func(path string) string {
			return strings.TrimSuffix(filepath.Base(path), ".jsonl")
		},
	}

	tw, _, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer tw.Close()

	// Register all sessions at once
	tw.RegisterSessions(sessions)

	hot, cold, _ := tw.Stats()

	// Should auto-promote most recent sessions to HOT
	if hot != MaxHotSessions {
		t.Errorf("hot = %d, want %d (auto-promoted)", hot, MaxHotSessions)
	}
	if cold != 5-MaxHotSessions {
		t.Errorf("cold = %d, want %d", cold, 5-MaxHotSessions)
	}
}
