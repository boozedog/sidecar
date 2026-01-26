package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"log/slog"
)

// ShellManifest stores persistent shell definitions for cross-instance sync
// and reboot survival. Stored in {project}/.sidecar/shells.json.
type ShellManifest struct {
	Version int               `json:"version"`
	Shells  []ShellDefinition `json:"shells"`

	path string     // not serialized - file path
	mu   sync.Mutex // protects concurrent access
}

// ShellDefinition contains all info needed to recreate a shell session.
type ShellDefinition struct {
	TmuxName    string    `json:"tmuxName"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	AgentType   string    `json:"agentType,omitempty"`
	SkipPerms   bool      `json:"skipPerms,omitempty"`
}

// manifestVersion is the current manifest format version.
const manifestVersion = 1

// LoadShellManifest loads the shell manifest from disk.
// Returns an empty manifest (not error) if file doesn't exist or is corrupted.
func LoadShellManifest(path string) (*ShellManifest, error) {
	m := &ShellManifest{
		Version: manifestVersion,
		Shells:  []ShellDefinition{},
		path:    path,
	}

	// Acquire shared lock for reading
	lockFile, err := acquireManifestLock(path, false)
	if err != nil {
		slog.Debug("manifest: lock failed, returning empty", "err", err)
		return m, nil
	}
	defer releaseManifestLock(lockFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil // Empty manifest is fine
		}
		slog.Warn("manifest: read failed", "err", err)
		return m, nil
	}

	if err := json.Unmarshal(data, m); err != nil {
		slog.Warn("manifest: parse failed, returning empty", "err", err)
		m.Shells = []ShellDefinition{}
	}
	m.path = path

	return m, nil
}

// Save writes the manifest to disk atomically with file locking.
func (m *ShellManifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveLocked()
}

// saveLocked writes the manifest to disk. Caller must hold m.mu.
func (m *ShellManifest) saveLocked() error {
	// Ensure .sidecar directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Acquire exclusive lock
	lockFile, err := acquireManifestLock(m.path, true)
	if err != nil {
		return err
	}
	defer releaseManifestLock(lockFile)

	// Ensure version is set
	m.Version = manifestVersion

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file + rename
	tmpPath := m.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, m.path)
}

// AddShell adds a shell definition and saves.
func (m *ShellManifest) AddShell(def ShellDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Check for duplicate
	for i, s := range m.Shells {
		if s.TmuxName == def.TmuxName {
			// Update existing
			m.Shells[i] = def
			return m.saveLocked()
		}
	}
	m.Shells = append(m.Shells, def)
	return m.saveLocked()
}

// RemoveShell removes a shell by tmuxName and saves.
func (m *ShellManifest) RemoveShell(tmuxName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.Shells {
		if s.TmuxName == tmuxName {
			m.Shells = append(m.Shells[:i], m.Shells[i+1:]...)
			return m.saveLocked()
		}
	}
	return nil // Not found, nothing to remove
}

// FindShell returns a shell definition by tmuxName, or nil if not found.
func (m *ShellManifest) FindShell(tmuxName string) *ShellDefinition {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.Shells {
		if m.Shells[i].TmuxName == tmuxName {
			return &m.Shells[i]
		}
	}
	return nil
}

// UpdateShell updates an existing shell definition and saves.
func (m *ShellManifest) UpdateShell(def ShellDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.Shells {
		if s.TmuxName == def.TmuxName {
			m.Shells[i] = def
			return m.saveLocked()
		}
	}
	// Not found - add it
	m.Shells = append(m.Shells, def)
	return m.saveLocked()
}

// Path returns the manifest file path.
func (m *ShellManifest) Path() string {
	return m.path
}

// lockTimeout is the maximum time to wait for file lock acquisition (td-984ead)
const lockTimeout = 5 * time.Second

// lockRetryInterval is how often to retry lock acquisition
const lockRetryInterval = 10 * time.Millisecond

// acquireManifestLock acquires an advisory lock on the manifest file with timeout.
// exclusive=true for writes, false for reads.
func acquireManifestLock(path string, exclusive bool) (*os.File, error) {
	lockPath := path + ".lock"

	// Ensure directory exists for lock file
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	lockType := syscall.LOCK_SH | syscall.LOCK_NB
	if exclusive {
		lockType = syscall.LOCK_EX | syscall.LOCK_NB
	}

	// Try non-blocking lock with timeout (td-984ead)
	deadline := time.Now().Add(lockTimeout)
	for {
		err := syscall.Flock(int(lockFile.Fd()), lockType)
		if err == nil {
			return lockFile, nil
		}
		// EWOULDBLOCK means lock is held by another process
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			lockFile.Close()
			return nil, err
		}
		if time.Now().After(deadline) {
			lockFile.Close()
			return nil, fmt.Errorf("lock acquisition timeout after %v", lockTimeout)
		}
		time.Sleep(lockRetryInterval)
	}
}

// releaseManifestLock releases the advisory lock.
func releaseManifestLock(lockFile *os.File) {
	if lockFile == nil {
		return
	}
	syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	lockFile.Close()
}

// shellToDefinition converts a ShellSession to a ShellDefinition for storage.
func shellToDefinition(shell *ShellSession) ShellDefinition {
	agentType := ""
	if shell.ChosenAgent != AgentNone {
		agentType = string(shell.ChosenAgent)
	}
	return ShellDefinition{
		TmuxName:    shell.TmuxName,
		DisplayName: shell.Name,
		CreatedAt:   shell.CreatedAt,
		AgentType:   agentType,
		SkipPerms:   shell.SkipPerms,
	}
}

// definitionToAgentType converts a string agent type to AgentType.
func definitionToAgentType(s string) AgentType {
	if s == "" {
		return AgentNone
	}
	return AgentType(s)
}
