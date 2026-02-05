package notes

// NotesLoadedMsg is sent when notes are loaded from the database.
type NotesLoadedMsg struct {
	Notes []Note
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NotesLoadedMsg) GetEpoch() uint64 {
	return m.Epoch
}

// NoteSavedMsg is sent when a note is created or updated.
type NoteSavedMsg struct {
	Note  *Note
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NoteSavedMsg) GetEpoch() uint64 {
	return m.Epoch
}

// NoteDeletedMsg is sent when a note is deleted.
type NoteDeletedMsg struct {
	ID    string
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NoteDeletedMsg) GetEpoch() uint64 {
	return m.Epoch
}

// NotePinToggledMsg is sent when a note's pinned state is toggled.
type NotePinToggledMsg struct {
	ID    string
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NotePinToggledMsg) GetEpoch() uint64 {
	return m.Epoch
}

// NoteArchiveToggledMsg is sent when a note's archived state is toggled.
type NoteArchiveToggledMsg struct {
	ID    string
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NoteArchiveToggledMsg) GetEpoch() uint64 {
	return m.Epoch
}

// NoteContentSavedMsg is sent when a note's content is saved from the editor.
type NoteContentSavedMsg struct {
	ID    string
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NoteContentSavedMsg) GetEpoch() uint64 {
	return m.Epoch
}

// AutoSaveTickMsg is sent when the auto-save debounce timer fires.
type AutoSaveTickMsg struct {
	// ID identifies which auto-save timer this is (for debounce check)
	ID int
}

// NoteRestoredMsg is sent when a note is restored (undo delete/archive).
type NoteRestoredMsg struct {
	ID    string
	Title string
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m NoteRestoredMsg) GetEpoch() uint64 {
	return m.Epoch
}

// InlineAutoSaveTickMsg is sent periodically during inline edit mode for auto-save.
type InlineAutoSaveTickMsg struct {
	// Generation identifies which auto-save timer this is (for staleness check)
	Generation int
}

// InlineAutoSaveResultMsg is sent after an inline auto-save completes.
type InlineAutoSaveResultMsg struct {
	Err   error
	Epoch uint64
}

// GetEpoch returns the epoch for staleness detection.
func (m InlineAutoSaveResultMsg) GetEpoch() uint64 {
	return m.Epoch
}
