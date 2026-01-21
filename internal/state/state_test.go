package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestInit(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	// Use InitWithDir to avoid reading real user state
	err := InitWithDir(filepath.Join(tmpDir, ".config", "sidecar"))
	if err != nil {
		t.Fatalf("InitWithDir() failed: %v", err)
	}

	if current == nil {
		t.Error("current state should be initialized")
	}
	if current.GitDiffMode != "unified" {
		t.Errorf("default GitDiffMode = %q, want unified", current.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestLoad_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	path = filepath.Join(tmpDir, "nonexistent", "state.json")

	err := Load()
	if err != nil {
		t.Fatalf("Load() for non-existent file should return nil, got %v", err)
	}

	if current == nil {
		t.Error("current should be initialized with defaults")
	}
	if current.GitDiffMode != "unified" {
		t.Errorf("default GitDiffMode = %q, want unified", current.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestLoad_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	// Create a state file
	testState := State{GitDiffMode: "side-by-side"}
	data, _ := json.Marshal(testState)
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if current.GitDiffMode != "side-by-side" {
		t.Errorf("GitDiffMode = %q, want side-by-side", current.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	// Create invalid JSON file
	if err := os.WriteFile(stateFile, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	err := Load()
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "config", "sidecar", "state.json")
	path = stateFile

	current = &State{GitDiffMode: "side-by-side"}

	err := Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Verify contents
	data, _ := os.ReadFile(stateFile)
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal saved state: %v", err)
	}

	if loaded.GitDiffMode != "side-by-side" {
		t.Errorf("saved GitDiffMode = %q, want side-by-side", loaded.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestSave_CreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "deep", "nested", "config", "sidecar", "state.json")
	path = stateFile

	current = &State{GitDiffMode: "unified"}

	err := Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestSave_NilCurrent(t *testing.T) {
	originalPath := path
	originalCurrent := current

	current = nil
	path = "/tmp/nonexistent/state.json"

	// Should not error when current is nil
	err := Save()
	if err != nil {
		t.Fatalf("Save() with nil current should not error, got %v", err)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestGetGitDiffMode_Default(t *testing.T) {
	originalCurrent := current

	current = nil
	mode := GetGitDiffMode()
	if mode != "unified" {
		t.Errorf("GetGitDiffMode() with nil current = %q, want unified", mode)
	}

	// Cleanup
	current = originalCurrent
}

func TestGetGitDiffMode_Set(t *testing.T) {
	originalCurrent := current

	current = &State{GitDiffMode: "side-by-side"}
	mode := GetGitDiffMode()
	if mode != "side-by-side" {
		t.Errorf("GetGitDiffMode() = %q, want side-by-side", mode)
	}

	// Cleanup
	current = originalCurrent
}

func TestSetGitDiffMode(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	current = &State{GitDiffMode: "unified"}

	err := SetGitDiffMode("side-by-side")
	if err != nil {
		t.Fatalf("SetGitDiffMode() failed: %v", err)
	}

	// Verify in-memory value
	if current.GitDiffMode != "side-by-side" {
		t.Errorf("current.GitDiffMode = %q, want side-by-side", current.GitDiffMode)
	}

	// Verify saved to disk
	data, _ := os.ReadFile(stateFile)
	var loaded State
	json.Unmarshal(data, &loaded)
	if loaded.GitDiffMode != "side-by-side" {
		t.Errorf("saved GitDiffMode = %q, want side-by-side", loaded.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestSetGitDiffMode_InitializesNilState(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	current = nil

	err := SetGitDiffMode("side-by-side")
	if err != nil {
		t.Fatalf("SetGitDiffMode() failed: %v", err)
	}

	if current == nil {
		t.Error("SetGitDiffMode() should initialize current state")
	}
	if current.GitDiffMode != "side-by-side" {
		t.Errorf("GitDiffMode = %q, want side-by-side", current.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestGetGitGraphEnabled_Default(t *testing.T) {
	current = nil
	enabled := GetGitGraphEnabled()
	if enabled {
		t.Errorf("GetGitGraphEnabled() with nil current = %v, want false", enabled)
	}
}

func TestGetGitGraphEnabled_Set(t *testing.T) {
	current = &State{GitGraphEnabled: true}
	enabled := GetGitGraphEnabled()
	if !enabled {
		t.Errorf("GetGitGraphEnabled() = %v, want true", enabled)
	}
}

func TestSetGitGraphEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile
	current = &State{GitGraphEnabled: false}

	err := SetGitGraphEnabled(true)
	if err != nil {
		t.Fatalf("SetGitGraphEnabled() failed: %v", err)
	}

	if !current.GitGraphEnabled {
		t.Errorf("current.GitGraphEnabled = %v, want true", current.GitGraphEnabled)
	}

	// Verify saved to disk
	data, _ := os.ReadFile(stateFile)
	var loaded State
	_ = json.Unmarshal(data, &loaded)
	if !loaded.GitGraphEnabled {
		t.Errorf("saved GitGraphEnabled = %v, want true", loaded.GitGraphEnabled)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	current = &State{GitDiffMode: "unified"}

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			mode := "unified"
			if n%2 == 0 {
				mode = "side-by-side"
			}
			if err := SetGitDiffMode(mode); err != nil {
				errors <- err
			}
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = GetGitDiffMode()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Errorf("concurrent access error: %v", err)
		}
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile

	// Set and save
	current = &State{GitDiffMode: "side-by-side"}
	if err := Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load into fresh state
	current = nil
	if err := Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if current.GitDiffMode != "side-by-side" {
		t.Errorf("round-trip GitDiffMode = %q, want side-by-side", current.GitDiffMode)
	}

	// Cleanup
	path = originalPath
	current = originalCurrent
}

func TestGetWorktreeState_Default(t *testing.T) {
	originalCurrent := current
	defer func() { current = originalCurrent }()

	current = nil
	state := GetWorktreeState("/path/to/project")
	if state.WorktreeName != "" || state.ShellTmuxName != "" || len(state.ShellDisplayNames) > 0 {
		t.Errorf("GetWorktreeState() with nil current should return empty state")
	}
}

func TestGetWorktreeState_EmptyMap(t *testing.T) {
	originalCurrent := current
	defer func() { current = originalCurrent }()

	current = &State{Worktree: nil}
	state := GetWorktreeState("/path/to/project")
	if state.WorktreeName != "" || state.ShellTmuxName != "" || len(state.ShellDisplayNames) > 0 {
		t.Errorf("GetWorktreeState() with nil map should return empty state")
	}
}

func TestGetWorktreeState_Found(t *testing.T) {
	originalCurrent := current
	defer func() { current = originalCurrent }()

	current = &State{
		Worktree: map[string]WorktreeState{
			"/path/to/project": {
				WorktreeName:  "feature-branch",
				ShellTmuxName: "sidecar-sh-project-1",
				ShellDisplayNames: map[string]string{
					"sidecar-sh-project-1": "Backend",
				},
			},
		},
	}
	state := GetWorktreeState("/path/to/project")
	if state.WorktreeName != "feature-branch" {
		t.Errorf("WorktreeName = %q, want feature-branch", state.WorktreeName)
	}
	if state.ShellTmuxName != "sidecar-sh-project-1" {
		t.Errorf("ShellTmuxName = %q, want sidecar-sh-project-1", state.ShellTmuxName)
	}
	if state.ShellDisplayNames["sidecar-sh-project-1"] != "Backend" {
		t.Errorf("ShellDisplayNames[sidecar-sh-project-1] = %q, want Backend", state.ShellDisplayNames["sidecar-sh-project-1"])
	}
}

func TestSetWorktreeState(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current
	defer func() {
		path = originalPath
		current = originalCurrent
	}()

	stateFile := filepath.Join(tmpDir, "state.json")
	path = stateFile
	current = &State{}

	wtState := WorktreeState{
		WorktreeName:  "my-worktree",
		ShellTmuxName: "",
		ShellDisplayNames: map[string]string{
			"sidecar-sh-project-1": "Backend",
		},
	}

	err := SetWorktreeState("/projects/sidecar", wtState)
	if err != nil {
		t.Fatalf("SetWorktreeState() failed: %v", err)
	}

	// Verify in memory
	stored := current.Worktree["/projects/sidecar"]
	if stored.WorktreeName != "my-worktree" {
		t.Errorf("stored WorktreeName = %q, want my-worktree", stored.WorktreeName)
	}
	if stored.ShellDisplayNames["sidecar-sh-project-1"] != "Backend" {
		t.Errorf("stored ShellDisplayNames[sidecar-sh-project-1] = %q, want Backend", stored.ShellDisplayNames["sidecar-sh-project-1"])
	}

	// Verify saved to disk
	data, _ := os.ReadFile(stateFile)
	var loaded State
	_ = json.Unmarshal(data, &loaded)
	if loaded.Worktree["/projects/sidecar"].WorktreeName != "my-worktree" {
		t.Errorf("persisted WorktreeName = %q, want my-worktree", loaded.Worktree["/projects/sidecar"].WorktreeName)
	}
	if loaded.Worktree["/projects/sidecar"].ShellDisplayNames["sidecar-sh-project-1"] != "Backend" {
		t.Errorf("persisted ShellDisplayNames[sidecar-sh-project-1] = %q, want Backend", loaded.Worktree["/projects/sidecar"].ShellDisplayNames["sidecar-sh-project-1"])
	}
}

func TestSetWorktreeState_ShellSelection(t *testing.T) {
	tmpDir := t.TempDir()
	originalPath := path
	originalCurrent := current
	defer func() {
		path = originalPath
		current = originalCurrent
	}()

	path = filepath.Join(tmpDir, "state.json")
	current = &State{}

	// Save shell selection
	wtState := WorktreeState{
		WorktreeName:  "",
		ShellTmuxName: "sidecar-sh-project-2",
	}

	err := SetWorktreeState("/projects/myapp", wtState)
	if err != nil {
		t.Fatalf("SetWorktreeState() failed: %v", err)
	}

	// Verify
	stored := current.Worktree["/projects/myapp"]
	if stored.ShellTmuxName != "sidecar-sh-project-2" {
		t.Errorf("stored ShellTmuxName = %q, want sidecar-sh-project-2", stored.ShellTmuxName)
	}
	if stored.WorktreeName != "" {
		t.Errorf("stored WorktreeName = %q, want empty", stored.WorktreeName)
	}
}
