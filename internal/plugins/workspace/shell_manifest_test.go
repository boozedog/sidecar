package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestShellManifest_LoadMissing(t *testing.T) {
	// Load from non-existent file should return empty manifest
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, err := LoadShellManifest(path)
	if err != nil {
		t.Fatalf("LoadShellManifest() error = %v", err)
	}
	if m == nil {
		t.Fatal("LoadShellManifest() returned nil")
	}
	if len(m.Shells) != 0 {
		t.Errorf("expected empty shells, got %d", len(m.Shells))
	}
}

func TestShellManifest_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".sidecar", "shells.json")

	// Create and save manifest
	m := &ShellManifest{
		Version: manifestVersion,
		Shells:  []ShellDefinition{},
		path:    path,
	}

	def := ShellDefinition{
		TmuxName:    "sidecar-sh-test-1",
		DisplayName: "Test Shell",
		CreatedAt:   time.Now().Truncate(time.Second),
		AgentType:   "claude",
		SkipPerms:   true,
	}

	if err := m.AddShell(def); err != nil {
		t.Fatalf("AddShell() error = %v", err)
	}

	// Load and verify
	m2, err := LoadShellManifest(path)
	if err != nil {
		t.Fatalf("LoadShellManifest() error = %v", err)
	}
	if len(m2.Shells) != 1 {
		t.Fatalf("expected 1 shell, got %d", len(m2.Shells))
	}
	if m2.Shells[0].TmuxName != def.TmuxName {
		t.Errorf("TmuxName = %q, want %q", m2.Shells[0].TmuxName, def.TmuxName)
	}
	if m2.Shells[0].DisplayName != def.DisplayName {
		t.Errorf("DisplayName = %q, want %q", m2.Shells[0].DisplayName, def.DisplayName)
	}
	if m2.Shells[0].AgentType != def.AgentType {
		t.Errorf("AgentType = %q, want %q", m2.Shells[0].AgentType, def.AgentType)
	}
	if m2.Shells[0].SkipPerms != def.SkipPerms {
		t.Errorf("SkipPerms = %v, want %v", m2.Shells[0].SkipPerms, def.SkipPerms)
	}
}

func TestShellManifest_AddRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	// Add two shells
	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Shell 1"})
	m.AddShell(ShellDefinition{TmuxName: "shell-2", DisplayName: "Shell 2"})

	if len(m.Shells) != 2 {
		t.Fatalf("expected 2 shells, got %d", len(m.Shells))
	}

	// Remove first
	if err := m.RemoveShell("shell-1"); err != nil {
		t.Fatalf("RemoveShell() error = %v", err)
	}

	if len(m.Shells) != 1 {
		t.Fatalf("expected 1 shell after remove, got %d", len(m.Shells))
	}
	if m.Shells[0].TmuxName != "shell-2" {
		t.Errorf("wrong shell remaining: %q", m.Shells[0].TmuxName)
	}
}

func TestShellManifest_FindShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Shell 1"})

	// Find existing
	found := m.FindShell("shell-1")
	if found == nil {
		t.Fatal("FindShell() returned nil for existing shell")
	}
	if found.DisplayName != "Shell 1" {
		t.Errorf("DisplayName = %q, want %q", found.DisplayName, "Shell 1")
	}

	// Find non-existing
	if m.FindShell("shell-999") != nil {
		t.Error("FindShell() should return nil for non-existing shell")
	}
}

func TestShellManifest_UpdateShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Original"})

	// Update
	if err := m.UpdateShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Updated"}); err != nil {
		t.Fatalf("UpdateShell() error = %v", err)
	}

	found := m.FindShell("shell-1")
	if found.DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want %q", found.DisplayName, "Updated")
	}
}

func TestShellManifest_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	sidecarDir := filepath.Join(dir, ".sidecar")
	os.MkdirAll(sidecarDir, 0755)
	path := filepath.Join(sidecarDir, "shells.json")

	// Write corrupted JSON
	os.WriteFile(path, []byte("{invalid json"), 0644)

	// Should return empty manifest, not error
	m, err := LoadShellManifest(path)
	if err != nil {
		t.Fatalf("LoadShellManifest() error = %v", err)
	}
	if len(m.Shells) != 0 {
		t.Errorf("expected empty shells for corrupted file, got %d", len(m.Shells))
	}
}

func TestShellManifest_AddDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Original"})
	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Updated"})

	// Should update, not duplicate
	if len(m.Shells) != 1 {
		t.Fatalf("expected 1 shell, got %d", len(m.Shells))
	}
	if m.Shells[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q, want %q", m.Shells[0].DisplayName, "Updated")
	}
}

// TestShellManifest_ConcurrentAdd tests concurrent AddShell calls (td-6db032)
func TestShellManifest_ConcurrentAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			err := m.AddShell(ShellDefinition{
				TmuxName:    fmt.Sprintf("shell-%d", idx),
				DisplayName: fmt.Sprintf("Shell %d", idx),
			})
			if err != nil {
				t.Errorf("AddShell(%d) error = %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	if len(m.Shells) != numGoroutines {
		t.Errorf("expected %d shells, got %d", numGoroutines, len(m.Shells))
	}
}

// TestShellManifest_ConcurrentAddRemove tests concurrent Add and Remove (td-6db032)
func TestShellManifest_ConcurrentAddRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	// Add initial shells
	for i := 0; i < 5; i++ {
		m.AddShell(ShellDefinition{TmuxName: fmt.Sprintf("shell-%d", i)})
	}

	var wg sync.WaitGroup
	// Concurrent adds
	for i := 5; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.AddShell(ShellDefinition{TmuxName: fmt.Sprintf("shell-%d", idx)})
		}(i)
	}
	// Concurrent removes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.RemoveShell(fmt.Sprintf("shell-%d", idx))
		}(i)
	}

	wg.Wait()

	// Should have exactly 5 shells (0-4 removed, 5-9 added)
	if len(m.Shells) != 5 {
		t.Errorf("expected 5 shells, got %d", len(m.Shells))
	}
}

// TestShellManifest_ConcurrentUpdate tests concurrent UpdateShell calls (td-6db032)
func TestShellManifest_ConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Original"})

	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			m.UpdateShell(ShellDefinition{
				TmuxName:    "shell-1",
				DisplayName: fmt.Sprintf("Update %d", idx),
			})
		}(i)
	}

	wg.Wait()

	// Should still have exactly 1 shell
	if len(m.Shells) != 1 {
		t.Errorf("expected 1 shell, got %d", len(m.Shells))
	}
}

// TestShellManifest_ConcurrentFind tests concurrent FindShell with modifications (td-6db032)
func TestShellManifest_ConcurrentFind(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")
	m, _ := LoadShellManifest(path)

	m.AddShell(ShellDefinition{TmuxName: "shell-1", DisplayName: "Test"})

	var wg sync.WaitGroup
	// Concurrent finds
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.FindShell("shell-1")
		}()
	}
	// Concurrent updates
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m.UpdateShell(ShellDefinition{
				TmuxName:    "shell-1",
				DisplayName: fmt.Sprintf("Update %d", idx),
			})
		}(i)
	}

	wg.Wait()
	// Test passes if no race detected (run with -race flag)
}

// TestShellManifest_MigrationFromEmptyManifest tests migrating display names to new manifest (td-e1b7ef)
func TestShellManifest_MigrationFromEmptyManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")

	// Simulate migration: manifest exists but has no display name, need to update it
	m, _ := LoadShellManifest(path)
	m.AddShell(ShellDefinition{
		TmuxName:    "sidecar-sh-test-1",
		DisplayName: "", // Empty initially (like after tmux session discovery)
		CreatedAt:   time.Now(),
	})

	// Verify shell has no display name
	found := m.FindShell("sidecar-sh-test-1")
	if found == nil {
		t.Fatal("shell not found")
	}
	if found.DisplayName != "" {
		t.Errorf("initial DisplayName = %q, want empty", found.DisplayName)
	}

	// Simulate migration: update with display name from state.json
	err := m.UpdateShell(ShellDefinition{
		TmuxName:    "sidecar-sh-test-1",
		DisplayName: "Backend", // Migrated from state.json
		CreatedAt:   found.CreatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateShell() error = %v", err)
	}

	// Verify migration worked
	found = m.FindShell("sidecar-sh-test-1")
	if found.DisplayName != "Backend" {
		t.Errorf("migrated DisplayName = %q, want Backend", found.DisplayName)
	}

	// Verify persisted correctly
	m2, _ := LoadShellManifest(path)
	found2 := m2.FindShell("sidecar-sh-test-1")
	if found2 == nil {
		t.Fatal("shell not found after reload")
	}
	if found2.DisplayName != "Backend" {
		t.Errorf("persisted DisplayName = %q, want Backend", found2.DisplayName)
	}
}

// TestShellManifest_MigrationPreservesExisting tests migration doesn't overwrite existing manifest data (td-e1b7ef)
func TestShellManifest_MigrationPreservesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")

	// Create manifest with existing data
	m, _ := LoadShellManifest(path)
	m.AddShell(ShellDefinition{
		TmuxName:    "sidecar-sh-test-1",
		DisplayName: "Original Name",
		AgentType:   "claude",
		SkipPerms:   true,
		CreatedAt:   time.Now(),
	})

	// Simulate partial migration update (only updates display name, preserves other fields)
	found := m.FindShell("sidecar-sh-test-1")
	err := m.UpdateShell(ShellDefinition{
		TmuxName:    "sidecar-sh-test-1",
		DisplayName: "Migrated Name",
		AgentType:   found.AgentType, // Preserve
		SkipPerms:   found.SkipPerms, // Preserve
		CreatedAt:   found.CreatedAt, // Preserve
	})
	if err != nil {
		t.Fatalf("UpdateShell() error = %v", err)
	}

	// Verify all fields preserved
	updated := m.FindShell("sidecar-sh-test-1")
	if updated.DisplayName != "Migrated Name" {
		t.Errorf("DisplayName = %q, want Migrated Name", updated.DisplayName)
	}
	if updated.AgentType != "claude" {
		t.Errorf("AgentType = %q, want claude", updated.AgentType)
	}
	if !updated.SkipPerms {
		t.Error("SkipPerms should be true")
	}
}

// TestShellManifest_MigrationNewShell tests migration handles shells not yet in manifest (td-e1b7ef)
func TestShellManifest_MigrationNewShell(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".sidecar", "shells.json")

	// Empty manifest
	m, _ := LoadShellManifest(path)
	if len(m.Shells) != 0 {
		t.Fatalf("expected empty manifest, got %d shells", len(m.Shells))
	}

	// UpdateShell on non-existent shell should add it (migration behavior)
	err := m.UpdateShell(ShellDefinition{
		TmuxName:    "sidecar-sh-new",
		DisplayName: "Newly Migrated",
		CreatedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("UpdateShell() error = %v", err)
	}

	// Verify shell was added
	if len(m.Shells) != 1 {
		t.Fatalf("expected 1 shell, got %d", len(m.Shells))
	}
	found := m.FindShell("sidecar-sh-new")
	if found == nil {
		t.Fatal("shell not found")
	}
	if found.DisplayName != "Newly Migrated" {
		t.Errorf("DisplayName = %q, want Newly Migrated", found.DisplayName)
	}
}

// TestShellManifest_LockAcquisitionNonBlocking tests that lock acquisition uses non-blocking retry (td-984ead)
func TestShellManifest_LockAcquisitionNonBlocking(t *testing.T) {
	dir := t.TempDir()
	sidecarDir := filepath.Join(dir, ".sidecar")
	os.MkdirAll(sidecarDir, 0755)
	path := filepath.Join(sidecarDir, "shells.json")

	// Create initial manifest
	m, _ := LoadShellManifest(path)
	m.AddShell(ShellDefinition{TmuxName: "shell-1"})

	// Verify concurrent operations don't deadlock (would timeout if blocking indefinitely)
	done := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			for j := 0; j < 5; j++ {
				m.UpdateShell(ShellDefinition{
					TmuxName:    "shell-1",
					DisplayName: fmt.Sprintf("Update %d-%d", idx, j),
				})
			}
			done <- true
		}(i)
	}

	// Wait for completion with timeout
	timeout := time.After(10 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
			// OK
		case <-timeout:
			t.Fatal("concurrent operations timed out - possible deadlock")
		}
	}
}
