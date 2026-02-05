package notes

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/marcus/sidecar/internal/tdroot"
)

// Note represents a single note.
type Note struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Pinned    bool       `json:"pinned"`
	Archived  bool       `json:"archived"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// ActionType represents the type of action performed.
type ActionType string

const (
	ActionCreate ActionType = "create"
	ActionUpdate ActionType = "update"
	ActionDelete ActionType = "delete"

	// maxTitleLength is the maximum length for note titles (truncated when displaying)
	maxTitleLength = 80
)

// Store handles SQLite operations for notes.
type Store struct {
	db        *sql.DB
	sessionID string
}

// NewStore creates a new Store with the given database path and session ID.
// If sessionID is empty, it checks TD_SESSION_ID env var, then falls back to "sidecar".
func NewStore(dbPath, sessionID string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Resolve session ID: param > TD_SESSION_ID env > "sidecar" default
	if sessionID == "" {
		sessionID = os.Getenv("TD_SESSION_ID")
	}
	if sessionID == "" {
		sessionID = "sidecar"
	}

	store := &Store{
		db:        db,
		sessionID: sessionID,
	}

	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// DefaultDBPath returns the default database path for a given workdir.
// It checks for .td-root link file first, which points to a shared td root.
func DefaultDBPath(workDir string) string {
	return tdroot.ResolveDBPath(workDir)
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// initSchema creates the notes table and indexes if they don't exist.
// NOTE: The notes table is created in td's database (.todos/issues.db) intentionally.
// This co-location enables notes to sync via td's existing sync infrastructure
// (action_log replication). The tradeoff is schema coupling - notes schema
// changes require coordination with td version compatibility.
func (s *Store) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS notes (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    pinned INTEGER DEFAULT 0,
    archived INTEGER DEFAULT 0,
    deleted_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_notes_updated ON notes(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_notes_deleted ON notes(deleted_at);
`
	_, err := s.db.Exec(schema)
	return err
}

// generateID creates a new note ID with "nt-" prefix and 8 hex chars.
func generateID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "nt-" + hex.EncodeToString(b), nil
}

// Create inserts a new note and logs the action.
func (s *Store) Create(title, content string) (*Note, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate ID: %w", err)
	}

	now := time.Now().UTC()
	note := &Note{
		ID:        id,
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
		Pinned:    false,
		Archived:  false,
	}

	_, err = s.db.Exec(`
		INSERT INTO notes (id, title, content, created_at, updated_at, pinned, archived)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, note.ID, note.Title, note.Content,
		note.CreatedAt.Format(time.RFC3339),
		note.UpdatedAt.Format(time.RFC3339),
		boolToInt(note.Pinned),
		boolToInt(note.Archived))
	if err != nil {
		return nil, fmt.Errorf("insert note: %w", err)
	}

	// Log action for sync - propagate errors
	if err := s.logAction(ActionCreate, note.ID, nil, note); err != nil {
		return nil, fmt.Errorf("log action: %w", err)
	}

	return note, nil
}

// Update modifies an existing note and logs the action.
func (s *Store) Update(note *Note) error {
	// Get previous state for action log
	prev, err := s.Get(note.ID)
	if err != nil {
		return fmt.Errorf("get previous state: %w", err)
	}
	if prev == nil {
		return fmt.Errorf("note not found: %s", note.ID)
	}

	note.UpdatedAt = time.Now().UTC()

	_, err = s.db.Exec(`
		UPDATE notes SET title = ?, content = ?, updated_at = ?, pinned = ?, archived = ?
		WHERE id = ? AND deleted_at IS NULL
	`, note.Title, note.Content,
		note.UpdatedAt.Format(time.RFC3339),
		boolToInt(note.Pinned),
		boolToInt(note.Archived),
		note.ID)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}

	// Log action for sync - propagate errors
	if err := s.logAction(ActionUpdate, note.ID, prev, note); err != nil {
		return fmt.Errorf("log action: %w", err)
	}

	return nil
}

// Delete performs a soft delete and logs the action.
func (s *Store) Delete(id string) error {
	// Get previous state for action log
	prev, err := s.Get(id)
	if err != nil {
		return fmt.Errorf("get previous state: %w", err)
	}
	if prev == nil {
		return fmt.Errorf("note not found: %s", id)
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE notes SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL
	`, now.Format(time.RFC3339), now.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("soft delete note: %w", err)
	}

	// Create new state with deleted_at set
	newNote := *prev
	newNote.DeletedAt = &now
	newNote.UpdatedAt = now

	// Log action for sync - propagate errors
	if err := s.logAction(ActionDelete, id, prev, &newNote); err != nil {
		return fmt.Errorf("log action: %w", err)
	}

	return nil
}

// Get retrieves a note by ID (excluding soft-deleted).
func (s *Store) Get(id string) (*Note, error) {
	var note Note
	var createdAt, updatedAt string
	var deletedAt sql.NullString
	var pinned, archived int

	err := s.db.QueryRow(`
		SELECT id, title, content, created_at, updated_at, pinned, archived, deleted_at
		FROM notes WHERE id = ?
	`, id).Scan(&note.ID, &note.Title, &note.Content,
		&createdAt, &updatedAt, &pinned, &archived, &deletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query note: %w", err)
	}

	note.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	note.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	note.Pinned = pinned == 1
	note.Archived = archived == 1
	if deletedAt.Valid {
		t, _ := time.Parse(time.RFC3339, deletedAt.String)
		note.DeletedAt = &t
	}

	return &note, nil
}

// List retrieves all non-deleted notes, ordered by pinned then updated_at.
func (s *Store) List(includeArchived bool) ([]Note, error) {
	query := `
		SELECT id, title, content, created_at, updated_at, pinned, archived, deleted_at
		FROM notes
		WHERE deleted_at IS NULL`
	if !includeArchived {
		query += ` AND archived = 0`
	}
	query += ` ORDER BY pinned DESC, updated_at DESC`

	return s.queryNotes(query)
}

// ListArchived retrieves only archived notes (not deleted), ordered by updated_at.
func (s *Store) ListArchived() ([]Note, error) {
	query := `
		SELECT id, title, content, created_at, updated_at, pinned, archived, deleted_at
		FROM notes
		WHERE deleted_at IS NULL AND archived = 1
		ORDER BY pinned DESC, updated_at DESC`

	return s.queryNotes(query)
}

// ListDeleted retrieves only soft-deleted notes, ordered by deleted_at (most recent first).
func (s *Store) ListDeleted() ([]Note, error) {
	query := `
		SELECT id, title, content, created_at, updated_at, pinned, archived, deleted_at
		FROM notes
		WHERE deleted_at IS NOT NULL
		ORDER BY deleted_at DESC`

	return s.queryNotes(query)
}

// queryNotes executes a query and returns notes.
func (s *Store) queryNotes(query string) ([]Note, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var note Note
		var createdAt, updatedAt string
		var deletedAt sql.NullString
		var pinned, archived int

		err := rows.Scan(&note.ID, &note.Title, &note.Content,
			&createdAt, &updatedAt, &pinned, &archived, &deletedAt)
		if err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}

		note.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		note.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		note.Pinned = pinned == 1
		note.Archived = archived == 1
		if deletedAt.Valid {
			t, _ := time.Parse(time.RFC3339, deletedAt.String)
			note.DeletedAt = &t
		}

		notes = append(notes, note)
	}

	return notes, rows.Err()
}

// TogglePin toggles the pinned state of a note.
func (s *Store) TogglePin(id string) error {
	note, err := s.Get(id)
	if err != nil {
		return err
	}
	if note == nil || note.DeletedAt != nil {
		return fmt.Errorf("note not found: %s", id)
	}

	note.Pinned = !note.Pinned
	return s.Update(note)
}

// ToggleArchive toggles the archived state of a note.
func (s *Store) ToggleArchive(id string) error {
	note, err := s.Get(id)
	if err != nil {
		return err
	}
	if note == nil || note.DeletedAt != nil {
		return fmt.Errorf("note not found: %s", id)
	}

	note.Archived = !note.Archived
	return s.Update(note)
}

// Restore undoes a soft delete by clearing deleted_at.
func (s *Store) Restore(id string) error {
	// Get current state for action log
	prev, err := s.Get(id)
	if err != nil {
		return fmt.Errorf("get previous state: %w", err)
	}
	if prev == nil {
		return fmt.Errorf("note not found: %s", id)
	}
	if prev.DeletedAt == nil {
		return fmt.Errorf("note not deleted: %s", id)
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE notes SET deleted_at = NULL, updated_at = ?
		WHERE id = ?
	`, now.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("restore note: %w", err)
	}

	// Create new state with deleted_at cleared
	newNote := *prev
	newNote.DeletedAt = nil
	newNote.UpdatedAt = now

	// Log action for sync
	if err := s.logAction(ActionUpdate, id, prev, &newNote); err != nil {
		return fmt.Errorf("log action: %w", err)
	}

	return nil
}

// Unarchive sets archived=false for a note.
func (s *Store) Unarchive(id string) error {
	note, err := s.Get(id)
	if err != nil {
		return err
	}
	if note == nil || note.DeletedAt != nil {
		return fmt.Errorf("note not found: %s", id)
	}
	if !note.Archived {
		return nil // Already unarchived
	}

	note.Archived = false
	return s.Update(note)
}

// UpdateContent updates the content of a note.
func (s *Store) UpdateContent(id, content string) error {
	note, err := s.Get(id)
	if err != nil {
		return err
	}
	if note == nil || note.DeletedAt != nil {
		return fmt.Errorf("note not found: %s", id)
	}

	// Extract title from first line of content
	title := ""
	if lines := splitFirst(content, "\n"); len(lines) > 0 {
		title = lines[0]
	}

	note.Title = title
	note.Content = content
	return s.Update(note)
}

// NotePath returns the path to the note file for external editor.
// Since notes are stored in SQLite, this creates a temporary file.
// Returns empty string if note doesn't exist or on error.
func (s *Store) NotePath(id string) string {
	note, err := s.Get(id)
	if err != nil || note == nil {
		return ""
	}

	// Create temp file with note content
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "sidecar-note-"+id+".md")

	err = os.WriteFile(tmpFile, []byte(note.Content), 0644)
	if err != nil {
		return ""
	}

	return tmpFile
}

// splitFirst splits a string on the first occurrence of sep.
func splitFirst(s, sep string) []string {
	idx := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			idx = i
			break
		}
	}
	if idx == 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}

// logAction writes an entry to the action_log table for sync.
// Uses td's action_log schema (TEXT PRIMARY KEY with "al-" prefix IDs).
// Session ID is resolved via: explicit param > TD_SESSION_ID env > "sidecar" default.
func (s *Store) logAction(actionType ActionType, entityID string, prev, new interface{}) error {
	var prevData, newData string

	if prev != nil {
		b, err := json.Marshal(prev)
		if err != nil {
			return fmt.Errorf("marshal previous data: %w", err)
		}
		prevData = string(b)
	}

	if new != nil {
		b, err := json.Marshal(new)
		if err != nil {
			return fmt.Errorf("marshal new data: %w", err)
		}
		newData = string(b)
	}

	// Generate action ID with "al-" prefix (matches td's generateActionID format)
	// td's action_log.id is TEXT PRIMARY KEY after migration v15, requiring explicit IDs
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate action ID: %w", err)
	}
	actionID := "al-" + hex.EncodeToString(b)

	_, err := s.db.Exec(`
		INSERT INTO action_log (id, session_id, action_type, entity_type, entity_id, previous_data, new_data, timestamp, undone)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)
	`, actionID, s.sessionID, string(actionType), "notes", entityID, prevData, newData, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert action log: %w", err)
	}

	return nil
}

// boolToInt converts a bool to an int for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
