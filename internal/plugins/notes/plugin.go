package notes

import (
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/app"
	"github.com/marcus/sidecar/internal/modal"
	"github.com/marcus/sidecar/internal/mouse"
	"github.com/marcus/sidecar/internal/msg"
	"github.com/marcus/sidecar/internal/plugin"
	"github.com/marcus/sidecar/internal/state"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/tty"
	"github.com/marcus/sidecar/internal/ui"
)

const (
	pluginID   = "notes"
	pluginName = "notes"
	pluginIcon = "N"

	// Pane layout
	dividerWidth = 1
)

// FocusPane represents which pane is active.
type FocusPane int

const (
	PaneList FocusPane = iota
	PaneEditor
)

// NoteFilter represents the current note filter view.
type NoteFilter int

const (
	FilterActive NoteFilter = iota
	FilterArchived
	FilterDeleted
)

// String returns the display name for the filter.
func (f NoteFilter) String() string {
	switch f {
	case FilterArchived:
		return "Archived"
	case FilterDeleted:
		return "Deleted"
	default:
		return "Active"
	}
}

// Plugin implements the notes plugin.
type Plugin struct {
	ctx     *plugin.Context
	focused bool
	store   *Store

	// View dimensions
	width  int
	height int

	// Pane state
	activePane FocusPane
	listWidth  int // width of list pane (calculated from ratio)

	// Filter state
	viewFilter NoteFilter // Active, Archived, or Deleted view

	// Note state
	notes     []Note
	cursor    int
	scrollOff int
	loading   bool
	loadErr   error

	// g key state for g g sequence
	pendingG bool

	// Search state (NV-style)
	searchMode    bool        // true when search input is focused
	searchQuery   string      // current search query
	filteredNotes []NoteMatch // filtered results

	// Editor state
	editorNote     *Note          // The note being edited (nil = no note open)
	editorTextarea textarea.Model // Bubbles textarea for edit mode
	editorDirty    bool           // Unsaved changes
	previewMode    bool           // true = read-only preview, false = editing

	// Preview mode state (read-only navigation)
	previewLines       []string // Lines for preview mode rendering
	previewCursorLine  int      // Cursor line for preview mode navigation
	previewScrollOff   int      // Scroll offset for preview mode
	previewWrapEnabled bool     // true = wrap long lines, false = truncate

	// Mouse state
	mouseHandler *mouse.Handler
	selection    ui.SelectionState

	// Task modal state
	showTaskModal         bool
	taskModal             *modal.Modal
	taskModalWidth        int
	taskModalNote         *Note
	taskModalTitleInput   textinput.Model
	taskModalTypeIdx      int
	taskModalPriorityIdx  int
	taskModalArchiveNote  bool
	taskModalMouseHandler *mouse.Handler

	// Delete modal state
	showDeleteModal         bool
	deleteModal             *modal.Modal
	deleteModalWidth        int
	deleteModalNote         *Note
	deleteModalMouseHandler *mouse.Handler

	// Info modal state
	showInfoModal         bool
	infoModal             *modal.Modal
	infoModalWidth        int
	infoModalNote         *Note
	infoModalMouseHandler *mouse.Handler

	// Pending edit state (for auto-edit on new note)
	pendingEditID string

	// Inline editor state (for reading back content after editor exits)
	pendingInlineEditID   string // Note ID being edited
	pendingInlineEditPath string // Temp file path

	// Inline tty editor state (for true inline editing)
	inlineEditor      *tty.Model
	inlineEditMode    bool
	inlineEditSession string
	inlineEditNoteID  string
	inlineEditPath    string
	inlineEditEditor  string

	// Inline editor mouse drag state (for text selection forwarding)
	inlineEditorDragging bool      // True when mouse is being dragged in editor (for text selection)
	lastDragForwardTime  time.Time // Throttle: last time a drag event was forwarded to tmux

	// Inline auto-save state (for periodic saving during inline edit)
	inlineAutoSaveGen      int    // Generation for staleness check
	inlineLastSavedContent string // Last saved content for change detection

	// Auto-save state
	autoSaveID int // Incremented on each edit to identify debounce timer

	// Undo state
	undoStack []UndoAction // Stack of undoable actions (most recent last)

	// Exit confirmation state (when clicking away from editor)
	showExitConfirmation bool        // True when confirmation dialog is shown
	exitConfirmSelection int         // 0=Save&Exit, 1=Exit without saving, 2=Cancel
	pendingClickRegion   string      // Region that was clicked
	pendingClickData     interface{} // Data associated with the click
}

// UndoActionType represents the type of undoable action.
type UndoActionType string

const (
	UndoDelete  UndoActionType = "delete"
	UndoArchive UndoActionType = "archive"
)

// UndoAction represents an undoable action.
type UndoAction struct {
	Type   UndoActionType
	NoteID string
	Title  string // For toast message
}

// New creates a new Notes plugin.
func New() *Plugin {
	return &Plugin{
		mouseHandler: mouse.NewHandler(),
		inlineEditor: tty.New(nil),
	}
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return pluginID }

// Name returns the plugin display name.
func (p *Plugin) Name() string { return pluginName }

// Icon returns the plugin icon character.
func (p *Plugin) Icon() string { return pluginIcon }

// Init initializes the plugin with context.
func (p *Plugin) Init(ctx *plugin.Context) error {
	p.ctx = ctx
	p.notes = nil
	p.cursor = 0
	p.scrollOff = 0
	p.loading = false
	p.loadErr = nil
	p.pendingG = false
	p.searchMode = false
	p.searchQuery = ""
	p.filteredNotes = nil

	// Pane state
	p.activePane = PaneList
	p.viewFilter = FilterActive
	// Load persisted list width
	notesState := state.GetNotesState(ctx.WorkDir)
	if notesState.ListWidth > 0 {
		p.listWidth = notesState.ListWidth
	} else {
		p.listWidth = 0 // calculated on render
	}

	// Mouse state
	if p.mouseHandler == nil {
		p.mouseHandler = mouse.NewHandler()
	}
	p.selection.Clear()

	// Editor state
	p.editorNote = nil
	p.editorDirty = false
	p.previewMode = true
	p.previewLines = nil
	p.previewCursorLine = 0
	p.previewScrollOff = 0
	p.previewWrapEnabled = state.GetLineWrapEnabled()

	// Initialize textarea
	ta := textarea.New()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.Prompt = ""
	ta.EndOfBufferCharacter = '~'
	ta.FocusedStyle = textarea.Style{
		Base:             lipgloss.NewStyle(),
		CursorLine:       lipgloss.NewStyle(),
		CursorLineNumber: styles.Muted,
		EndOfBuffer:      styles.Muted,
		LineNumber:       styles.Muted,
		Placeholder:      styles.Muted,
		Prompt:           lipgloss.NewStyle(),
		Text:             lipgloss.NewStyle(),
	}
	ta.BlurredStyle = ta.FocusedStyle
	// Unbind alt+c (CapitalizeWordForward) - we use it for clipboard copy
	ta.KeyMap.CapitalizeWordForward = key.NewBinding(key.WithDisabled())
	ta.Blur()
	p.editorTextarea = ta

	// Initialize store - session ID resolved by store from TD_SESSION_ID env var
	// or falls back to "sidecar" if not set
	dbPath := DefaultDBPath(ctx.WorkDir)
	store, err := NewStore(dbPath, "")
	if err != nil {
		// Store initialization may fail if .todos directory doesn't exist
		// This is OK - plugin will show appropriate message
		p.ctx.Logger.Debug("notes: store init failed", "error", err)
		p.store = nil
		return nil
	}

	p.store = store
	return nil
}

// Start begins plugin operation.
func (p *Plugin) Start() tea.Cmd {
	if p.store == nil {
		return nil
	}
	return p.loadNotes()
}

// Stop cleans up plugin resources.
func (p *Plugin) Stop() {
	if p.store != nil {
		p.store.Close()
		p.store = nil
	}
}

// Update handles messages.
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
	// Handle exit confirmation dialog first
	if p.showExitConfirmation {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "j", "down":
				p.exitConfirmSelection = (p.exitConfirmSelection + 1) % 3
				return p, nil
			case "k", "up":
				p.exitConfirmSelection = (p.exitConfirmSelection + 2) % 3
				return p, nil
			case "enter":
				return p.handleExitConfirmationChoice()
			case "esc", "q":
				// Cancel - return to editing
				p.showExitConfirmation = false
				p.pendingClickRegion = ""
				p.pendingClickData = nil
				return p, nil
			}
		}
		return p, nil
	}

	// Handle inline editor messages first when in inline edit mode
	if p.inlineEditMode && p.inlineEditor != nil {
		// Check if editor became inactive or tmux session died
		// This proactively handles :wq exit before SessionDeadMsg arrives
		if !p.inlineEditor.IsActive() || !p.isInlineEditSessionAlive() {
			noteID := p.inlineEditNoteID
			notePath := p.inlineEditPath
			p.exitInlineEditMode()
			return p, p.saveNoteAfterInlineExit(noteID, notePath)
		}

		if handled, cmd := p.handleTtyMessages(msg); handled {
			return p, cmd
		}
	}

	switch msg := msg.(type) {
	case InlineEditStartedMsg:
		return p, p.handleInlineEditStarted(msg)

	case InlineEditExitedMsg:
		return p, p.handleInlineEditExited(msg)

	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		// Update textarea dimensions
		p.updateTextareaDimensions()
		// Update inline editor dimensions if active - use ResizeAndPollImmediate
		// to bypass debounce and trigger immediate poll for smooth resize
		if p.inlineEditMode && p.inlineEditor != nil {
			width := p.calculateInlineEditorWidth()
			height := p.calculateInlineEditorHeight()
			if cmd := p.inlineEditor.ResizeAndPollImmediate(width, height); cmd != nil {
				return p, cmd
			}
		}

	case NotesLoadedMsg:
		// Check for stale message
		if plugin.IsStale(p.ctx, msg) {
			return p, nil
		}
		p.loading = false
		if msg.Err != nil {
			p.loadErr = msg.Err
			p.ctx.Logger.Error("notes: load failed", "error", msg.Err)
		} else {
			p.notes = msg.Notes
			p.loadErr = nil

			// Auto-edit mode: if we just created a note, select it and enter edit mode
			if p.pendingEditID != "" {
				for i, n := range p.notes {
					if n.ID == p.pendingEditID {
						p.cursor = i
						p.loadNoteIntoEditorAtEnd()
						p.activePane = PaneEditor
						p.previewMode = false // Enter edit mode for immediate typing
						break
					}
				}
				p.pendingEditID = ""
			} else if p.editorNote != nil {
				// Follow the edited note if it moved position (due to updated_at sort)
				for i, n := range p.notes {
					if n.ID == p.editorNote.ID {
						p.cursor = i
						// Update editorNote reference to get latest content
						p.editorNote = &p.notes[i]
						break
					}
				}
				// Ensure cursor is visible in list
				p.ensureCursorVisibleForList(p.height-2, len(p.notes))
			} else if len(p.notes) > 0 {
				// Initial load: show the first note in the editor pane
				if p.cursor >= len(p.notes) {
					p.cursor = 0
				}
				p.loadNoteIntoEditor()
			}
		}

	case NoteSavedMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("notes: save failed", "error", msg.Err)
		} else {
			// Track new note ID for auto-edit mode
			if msg.Note != nil && p.pendingEditID == "" {
				p.pendingEditID = msg.Note.ID
			}
			return p, p.loadNotes()
		}

	case NoteDeletedMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("notes: delete failed", "error", msg.Err)
		} else {
			return p, p.loadNotes()
		}

	case NotePinToggledMsg, NoteArchiveToggledMsg:
		return p, p.loadNotes()

	case NoteRestoredMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("notes: restore failed", "error", msg.Err)
		} else {
			p.ctx.Logger.Debug("notes: restored", "id", msg.ID)
			return p, tea.Batch(
				showRestoredToast(msg.Title),
				p.loadNotes(),
			)
		}

	case NoteContentSavedMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("notes: content save failed", "error", msg.Err)
		} else {
			p.editorDirty = false
			p.ctx.Logger.Debug("notes: content saved", "id", msg.ID)
			return p, tea.Batch(
				showSavedToast(),
				p.loadNotes(),
			)
		}

	case TaskCreatedMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("notes: task creation failed", "error", msg.Err)
		} else {
			p.ctx.Logger.Debug("notes: task created", "taskID", msg.TaskID, "noteID", msg.NoteID)
			// Reload notes (in case note was archived)
			return p, tea.Batch(showTaskCreatedToast(msg.TaskID), p.loadNotes())
		}

	case AutoSaveTickMsg:
		// Only auto-save if this tick matches current auto-save ID (debounce)
		if msg.ID == p.autoSaveID && p.editorDirty && p.activePane == PaneEditor {
			return p, p.saveEditorContent()
		}

	case InlineAutoSaveTickMsg:
		// Handle periodic auto-save during inline edit mode
		if p.inlineEditMode && msg.Generation == p.inlineAutoSaveGen {
			return p, p.performInlineAutoSave()
		}

	case InlineAutoSaveResultMsg:
		// Auto-save completed silently (no toast) - schedule next tick
		if p.inlineEditMode {
			return p, p.scheduleInlineAutoSave()
		}

	case app.RefreshMsg:
		// After inline editor exits, read back temp file content and update note
		if p.pendingInlineEditID != "" && p.pendingInlineEditPath != "" {
			return p, p.readBackInlineEdit()
		}
		// Normal refresh: reload notes
		return p, p.loadNotes()

	case tea.KeyMsg:
		// Handle inline editor first if in inline edit mode
		if p.inlineEditMode {
			handled, cmd := p.handleInlineEditorKey(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle info modal first if open
		if p.showInfoModal {
			p.ensureInfoModal()
			cmd, handled := p.handleInfoModalKey(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle delete modal if open
		if p.showDeleteModal {
			p.ensureDeleteModal()
			cmd, handled := p.handleDeleteModalKey(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle task modal if open
		if p.showTaskModal {
			p.ensureTaskModal()
			cmd, handled := p.handleTaskModalKey(msg)
			if handled {
				return p, cmd
			}
		}
		return p.handleKey(msg)

	case tea.MouseMsg:
		// Handle inline editor first if in inline edit mode
		if p.inlineEditMode {
			handled, cmd := p.handleInlineEditorMouse(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle info modal first if open
		if p.showInfoModal {
			p.ensureInfoModal()
			cmd, handled := p.handleInfoModalMouse(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle delete modal if open
		if p.showDeleteModal {
			p.ensureDeleteModal()
			cmd, handled := p.handleDeleteModalMouse(msg)
			if handled {
				return p, cmd
			}
		}
		// Handle task modal if open
		if p.showTaskModal {
			p.ensureTaskModal()
			cmd, handled := p.handleTaskModalMouse(msg)
			if handled {
				return p, cmd
			}
		}
		return p.handleMouse(msg)
	}

	// Pass through other messages to textarea (for cursor blink, etc.)
	if p.activePane == PaneEditor && !p.previewMode && p.editorNote != nil {
		var cmd tea.Cmd
		p.editorTextarea, cmd = p.editorTextarea.Update(msg)
		if cmd != nil {
			return p, cmd
		}
	}

	return p, nil
}

// handleKey processes keyboard input.
func (p *Plugin) handleKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	// Handle search mode input (only when in list pane)
	if p.searchMode {
		return p.handleSearchKey(msg)
	}

	// Handle editor pane input
	if p.activePane == PaneEditor && p.editorNote != nil {
		return p.handleEditorKey(msg)
	}

	// Handle g g sequence for jump to top
	if p.pendingG {
		p.pendingG = false
		if key == "g" {
			p.cursor = 0
			p.scrollOff = 0
			return p, nil
		}
		// Not a g, fall through to normal handling
	}

	// Enter search mode with /
	if key == "/" {
		p.searchMode = true
		p.searchQuery = ""
		p.updateFilteredNotes()
		return p, nil
	}

	// Tab switches between panes (only if editor has a note open)
	if key == "tab" && p.editorNote != nil {
		if p.activePane == PaneList {
			p.activePane = PaneEditor
			// Edit mode only allowed in Active filter view
			p.previewMode = p.viewFilter != FilterActive
			if !p.previewMode {
				cmd := p.editorTextarea.Focus()
				return p, cmd
			}
			p.editorTextarea.Blur()
		} else {
			p.activePane = PaneList
			p.editorTextarea.Blur()
		}
		return p, nil
	}

	// Esc returns to Active view from Archived/Deleted views
	if key == "esc" && p.viewFilter != FilterActive {
		p.viewFilter = FilterActive
		p.cursor = 0
		p.scrollOff = 0
		p.editorNote = nil
		p.previewLines = nil
		p.editorDirty = false
		return p, p.loadNotes()
	}

	// Get the notes list to navigate (filtered or all)
	notesList := p.getDisplayNotes()

	// Skip navigation operations when notes list is empty
	if len(notesList) == 0 {
		switch key {
		case "n":
			// Create new note - allowed even with empty list
			return p, p.createNote()
		case "r":
			// Refresh - allowed even with empty list
			return p, p.loadNotes()
		}
		return p, nil
	}

	switch key {
	case "j", "down":
		if p.cursor < len(notesList)-1 {
			p.cursor++
		}
		// Auto-load note content in editor
		p.loadNoteIntoEditor()
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
		// Auto-load note content in editor
		p.loadNoteIntoEditor()
	case "g":
		// Start g g sequence
		p.pendingG = true
	case "G":
		// Jump to bottom
		p.cursor = len(notesList) - 1
		p.loadNoteIntoEditor()
	case "n":
		// Create new note (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.createNote()
		}
		return p, nil
	case "X":
		// Delete note (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.deleteNote()
		}
		return p, nil
	case "x":
		// Show deleted notes view
		p.viewFilter = FilterDeleted
		p.cursor = 0
		p.scrollOff = 0
		p.editorNote = nil
		p.previewLines = nil
		p.editorDirty = false
		return p, p.loadNotes()
	case "p":
		// Toggle pin (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.togglePin()
		}
		return p, nil
	case "A":
		// Archive note (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.toggleArchive()
		}
		return p, nil
	case "a":
		// Show archived notes view
		p.viewFilter = FilterArchived
		p.cursor = 0
		p.scrollOff = 0
		p.editorNote = nil
		p.previewLines = nil
		p.editorDirty = false
		return p, p.loadNotes()
	case "r":
		// Refresh
		return p, p.loadNotes()
	case "enter":
		// Open note in editor pane (or inline vim if configured)
		note := p.getSelectedNote()
		if note != nil {
			// Check if default editor is vim/nvim - use tty.Model inline editor
			if p.isDefaultEditorVim() && p.viewFilter == FilterActive {
				if p.isInlineEditSupported() {
					return p, p.enterInlineEditMode(note.ID)
				}
				// Fall back to external editor if inline not supported
				return p, p.openInExternalEditor()
			}
			// Otherwise use built-in editor
			p.loadNoteIntoEditor()
			p.activePane = PaneEditor
			// Edit mode only allowed in Active filter view
			p.previewMode = p.viewFilter != FilterActive
			if !p.previewMode {
				p.editorTextarea.Focus()
			}
		}
	case "e":
		// Open in inline tty editor (vim in preview pane) - only in Active view
		if p.viewFilter == FilterActive {
			note := p.getSelectedNote()
			if note != nil && p.isInlineEditSupported() {
				return p, p.enterInlineEditMode(note.ID)
			}
			// Fall back to external editor if inline not supported
			return p, p.openInExternalEditor()
		}
		return p, nil
	case "E":
		// Open in external $EDITOR - same as 'e' for now (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.openInExternalEditor()
		}
		return p, nil
	case "T":
		// Convert note to task (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.openTaskModal()
		}
		return p, nil
	case "I":
		// Show info modal for selected note
		return p, p.openInfoModal()
	case "y":
		// Yank note content to clipboard
		return p, p.yankNoteContent()
	case "Y":
		// Yank note title to clipboard
		return p, p.yankNoteTitle()
	case "u":
		// Undo last delete/archive (only in Active view)
		if p.viewFilter == FilterActive {
			return p, p.undoLastAction()
		}
		return p, nil
	}
	return p, nil
}

// handleEditorKey processes keyboard input when editor pane is focused.
func (p *Plugin) handleEditorKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	// In preview mode, only allow navigation and mode switches
	if p.previewMode {
		return p.handleEditorPreviewKey(msg)
	}

	// Clear any mouse selection when typing (returns to textarea rendering)
	p.selection.Clear()

	switch key {
	case "tab":
		p.activePane = PaneList
		p.syncPreviewFromTextarea()
		p.editorTextarea.Blur()
		return p, nil

	case "esc":
		p.activePane = PaneList
		p.syncPreviewFromTextarea()
		p.editorTextarea.Blur()
		return p, nil

	case "ctrl+s":
		p.autoSaveID++
		return p, p.saveEditorContent()

	case "E":
		return p, p.openInExternalEditor()

	case "alt+c":
		return p, p.copyEditorContent()
	}

	// Detect content change for auto-save
	oldValue := p.editorTextarea.Value()

	// Delegate to textarea
	var cmd tea.Cmd
	p.editorTextarea, cmd = p.editorTextarea.Update(msg)

	// Track scroll position for mouse region registration
	p.trackTextareaScroll()

	// Check if content changed
	newValue := p.editorTextarea.Value()
	if newValue != oldValue {
		p.editorDirty = true
		// Update preview lines for when we switch to preview
		p.previewLines = strings.Split(newValue, "\n")
		if len(p.previewLines) == 0 {
			p.previewLines = []string{""}
		}
		return p, tea.Batch(cmd, p.startAutoSaveTimer())
	}

	return p, cmd
}

// handleEditorPreviewKey handles keys in preview mode (read-only).
func (p *Plugin) handleEditorPreviewKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	switch key {
	case "tab":
		p.activePane = PaneList
		return p, nil

	case "esc":
		p.activePane = PaneList
		return p, nil

	case "enter", "i":
		// Enter edit mode (only in Active filter view)
		if p.viewFilter == FilterActive {
			p.previewMode = false
			cmd := p.editorTextarea.Focus()
			return p, cmd
		}
		return p, nil

	case "e":
		note := p.getSelectedNote()
		if note != nil && p.isInlineEditSupported() {
			return p, p.enterInlineEditMode(note.ID)
		}
		return p, p.openInExternalEditor()

	case "E":
		return p, p.openInExternalEditor()

	case "alt+c":
		return p, p.copyEditorContent()

	case "j", "down", "ctrl+n":
		if p.previewCursorLine < len(p.previewLines)-1 {
			p.previewCursorLine++
		}
		p.ensurePreviewCursorVisible()

	case "k", "up", "ctrl+p":
		if p.previewCursorLine > 0 {
			p.previewCursorLine--
		}
		p.ensurePreviewCursorVisible()

	case "g":
		p.previewCursorLine = 0
		p.previewScrollOff = 0

	case "G":
		if len(p.previewLines) > 0 {
			p.previewCursorLine = len(p.previewLines) - 1
		}
		p.ensurePreviewCursorVisible()

	case "w":
		p.previewWrapEnabled = !p.previewWrapEnabled
		_ = state.SetLineWrapEnabled(p.previewWrapEnabled)
		p.previewScrollOff = 0
	}

	return p, nil
}

// ensurePreviewCursorVisible adjusts preview scroll offset to keep cursor visible.
// Uses last known height from the view dimensions.
func (p *Plugin) ensurePreviewCursorVisible() {
	contentHeight := p.height - 2 - 1 // borders - status header
	if contentHeight < 1 {
		contentHeight = 1
	}
	p.ensurePreviewCursorVisibleWithHeight(contentHeight)
}

// trackTextareaScroll updates previewScrollOff to approximate the textarea's viewport
// scroll position. Call after textarea cursor movement so mouse regions stay accurate.
func (p *Plugin) trackTextareaScroll() {
	cursorLine := p.editorTextarea.Line()
	height := p.height - 2 - 1 // borders - status header
	if height < 1 {
		height = 1
	}
	if cursorLine < p.previewScrollOff {
		p.previewScrollOff = cursorLine
	}
	if cursorLine >= p.previewScrollOff+height {
		p.previewScrollOff = cursorLine - height + 1
	}
}

// setTextareaCursorPosition navigates the textarea cursor to the specified row and column.
// Uses CursorUp/CursorDown since textarea has no SetRow API.
func (p *Plugin) setTextareaCursorPosition(row, col int) {
	lineCount := p.editorTextarea.LineCount()
	if lineCount == 0 {
		return
	}
	if row < 0 {
		row = 0
	}
	if row >= lineCount {
		row = lineCount - 1
	}

	// Navigate to target row
	current := p.editorTextarea.Line()
	for current > row {
		p.editorTextarea.CursorUp()
		current = p.editorTextarea.Line()
	}
	for current < row {
		p.editorTextarea.CursorDown()
		current = p.editorTextarea.Line()
	}

	// Set column
	p.editorTextarea.SetCursor(col)
}

// syncPreviewFromTextarea updates previewLines from the current textarea content.
// Call this whenever switching from edit mode to preview/list mode.
func (p *Plugin) syncPreviewFromTextarea() {
	content := p.editorTextarea.Value()
	p.previewLines = strings.Split(content, "\n")
	if len(p.previewLines) == 0 {
		p.previewLines = []string{""}
	}
}

// loadNoteIntoEditor loads the currently selected note into the editor pane.
// Cursor is positioned at the end of the content.
func (p *Plugin) loadNoteIntoEditor() {
	note := p.getSelectedNote()
	if note == nil {
		p.editorNote = nil
		p.previewLines = nil
		p.editorDirty = false
		return
	}

	// Don't reload if already editing this note
	if p.editorNote != nil && p.editorNote.ID == note.ID && !p.editorDirty {
		return
	}

	// If dirty, don't auto-reload (user needs to save or discard)
	if p.editorDirty {
		return
	}

	p.editorNote = note
	p.editorTextarea.SetValue(note.Content)
	p.previewLines = strings.Split(note.Content, "\n")
	if len(p.previewLines) == 0 {
		p.previewLines = []string{""}
	}
	p.previewCursorLine = len(p.previewLines) - 1
	p.previewScrollOff = 0
	p.editorDirty = false
	p.previewMode = true // Load in preview mode, Enter/Tab to edit
	p.editorTextarea.Blur()
}

// loadNoteIntoEditorAtEnd loads the currently selected note into the editor pane
// with cursor positioned at the end of the content. Used for new notes created via search.
func (p *Plugin) loadNoteIntoEditorAtEnd() {
	note := p.getSelectedNote()
	if note == nil {
		p.editorNote = nil
		p.previewLines = nil
		p.editorDirty = false
		return
	}

	p.editorNote = note
	p.editorTextarea.SetValue(note.Content)
	p.previewLines = strings.Split(note.Content, "\n")
	if len(p.previewLines) == 0 {
		p.previewLines = []string{""}
	}
	p.previewCursorLine = 0
	p.previewScrollOff = 0
	p.editorDirty = false
	p.previewMode = false // Immediately in edit mode for new notes
	p.editorTextarea.Focus()
}

// updateTextareaDimensions updates the textarea dimensions based on current layout.
func (p *Plugin) updateTextareaDimensions() {
	if p.width == 0 || p.height == 0 {
		return
	}
	p.calculatePaneWidths()
	editorWidth := p.width - p.listWidth - dividerWidth - 4 // borders + padding
	contentHeight := p.height - 2 - 1                       // borders - status header
	if editorWidth < 1 {
		editorWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	p.editorTextarea.SetWidth(editorWidth)
	p.editorTextarea.SetHeight(contentHeight)
}

// startAutoSaveTimer starts a 1-second debounce timer for auto-save.
func (p *Plugin) startAutoSaveTimer() tea.Cmd {
	p.autoSaveID++
	id := p.autoSaveID
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return AutoSaveTickMsg{ID: id}
	})
}

// saveEditorContent saves the editor content back to the note.
func (p *Plugin) saveEditorContent() tea.Cmd {
	if p.editorNote == nil || p.store == nil || !p.editorDirty {
		return nil
	}

	content := p.editorTextarea.Value()
	noteID := p.editorNote.ID
	epoch := p.ctx.Epoch

	return func() tea.Msg {
		err := p.store.UpdateContent(noteID, content)
		if err != nil {
			return NoteSavedMsg{Note: nil, Err: err, Epoch: epoch}
		}
		return NoteContentSavedMsg{ID: noteID, Err: nil, Epoch: epoch}
	}
}

// openInExternalEditor opens the current note in $EDITOR.
func (p *Plugin) openInExternalEditor() tea.Cmd {
	note := p.getSelectedNote()
	if note == nil || p.store == nil {
		return nil
	}

	// Get path to note file (creates temp file with note content)
	notePath := p.store.NotePath(note.ID)
	if notePath == "" {
		return nil
	}

	// Track the note being edited so we can read back changes after editor exits
	p.pendingInlineEditID = note.ID
	p.pendingInlineEditPath = notePath

	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			editor = "vim"
		}
		return plugin.OpenFileMsg{
			Editor: editor,
			Path:   notePath,
			LineNo: 0,
		}
	}
}

// openInInlineEditor opens the current note in $EDITOR inline (suspends TUI).
// This writes note content to a temp file, opens the editor, then reads back on exit.
func (p *Plugin) openInInlineEditor() tea.Cmd {
	note := p.getSelectedNote()
	if note == nil || p.store == nil {
		return nil
	}

	noteID := note.ID

	// Write note content to temp file and track for reading back on return
	notePath := p.store.NotePath(noteID)
	if notePath == "" {
		return nil
	}

	// Store pending edit info for reading back after editor exits
	p.pendingInlineEditID = noteID
	p.pendingInlineEditPath = notePath

	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			editor = "vim"
		}
		return plugin.OpenFileMsg{
			Editor: editor,
			Path:   notePath,
			LineNo: 0,
		}
	}
}

// readBackInlineEdit reads the temp file content after inline editor exits and updates the note.
func (p *Plugin) readBackInlineEdit() tea.Cmd {
	noteID := p.pendingInlineEditID
	notePath := p.pendingInlineEditPath
	epoch := p.ctx.Epoch

	// Clear pending state
	p.pendingInlineEditID = ""
	p.pendingInlineEditPath = ""

	if noteID == "" || notePath == "" || p.store == nil {
		return p.loadNotes()
	}

	return func() tea.Msg {
		// Read back the edited content from temp file
		content, err := os.ReadFile(notePath)
		if err != nil {
			// Failed to read, just reload notes
			return NotesLoadedMsg{Err: err, Epoch: epoch}
		}

		// Clean up temp file
		_ = os.Remove(notePath)

		// Update note content in database
		if err := p.store.UpdateContent(noteID, string(content)); err != nil {
			return NoteSavedMsg{Note: nil, Err: err, Epoch: epoch}
		}

		return NoteContentSavedMsg{ID: noteID, Err: nil, Epoch: epoch}
	}
}

// handleSearchKey processes keyboard input in search mode.
func (p *Plugin) handleSearchKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		// Exit search mode, clear query, show all notes
		p.searchMode = false
		p.searchQuery = ""
		p.filteredNotes = nil
		p.cursor = 0
		p.scrollOff = 0
		return p, nil

	case "enter":
		// NV behavior: if exact match exists, select it; otherwise create new note
		if p.searchQuery != "" {
			exactMatch := FindExactTitleMatch(p.notes, p.searchQuery)
			if exactMatch != nil {
				// Select the exact match and open in editor
				for i, n := range p.notes {
					if n.ID == exactMatch.ID {
						p.cursor = i
						break
					}
				}
				// Check if default editor is vim/nvim - use tty.Model inline editor
				if p.isDefaultEditorVim() {
					p.searchMode = false
					p.searchQuery = ""
					p.filteredNotes = nil
					p.scrollOff = 0
					if p.isInlineEditSupported() {
						return p, p.enterInlineEditMode(exactMatch.ID)
					}
					return p, p.openInExternalEditor()
				}
				p.loadNoteIntoEditor()
				p.activePane = PaneEditor
				p.previewMode = false // Edit mode
				p.editorTextarea.Focus()
				p.ctx.Logger.Debug("notes: exact match selected", "id", exactMatch.ID)
			} else if len(p.filteredNotes) > 0 {
				// Select first filtered result and open in editor
				// Check if default editor is vim/nvim - use tty.Model inline editor
				if p.isDefaultEditorVim() {
					note := p.getSelectedNote()
					p.searchMode = false
					p.searchQuery = ""
					p.filteredNotes = nil
					p.scrollOff = 0
					if note != nil && p.isInlineEditSupported() {
						return p, p.enterInlineEditMode(note.ID)
					}
					return p, p.openInExternalEditor()
				}
				p.loadNoteIntoEditor()
				p.activePane = PaneEditor
				p.previewMode = false // Edit mode
				p.editorTextarea.Focus()
				note := p.getSelectedNote()
				if note != nil {
					p.ctx.Logger.Debug("notes: filtered match selected", "id", note.ID)
				}
			} else {
				// No matches - create new note with query as title
				title := p.searchQuery
				// Clear search state before creating
				p.searchMode = false
				p.searchQuery = ""
				p.filteredNotes = nil
				return p, p.createNoteWithTitle(title)
			}
		}
		// Exit search mode and clear query after selection
		p.searchMode = false
		p.searchQuery = ""
		p.filteredNotes = nil
		p.scrollOff = 0
		return p, nil

	case "ctrl+n", "down":
		// Navigate down in results
		notesList := p.getDisplayNotes()
		if p.cursor < len(notesList)-1 {
			p.cursor++
			p.ensureCursorVisibleForList(p.height-2, len(notesList))
		}
		return p, nil

	case "ctrl+p", "up":
		// Navigate up in results
		notesList := p.getDisplayNotes()
		if p.cursor > 0 {
			p.cursor--
			p.ensureCursorVisibleForList(p.height-2, len(notesList))
		}
		return p, nil

	case "backspace":
		// Remove last character from query
		if len(p.searchQuery) > 0 {
			p.searchQuery = p.searchQuery[:len(p.searchQuery)-1]
			p.updateFilteredNotes()
		}
		return p, nil

	default:
		// Add character to search query (only printable runes)
		if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
			p.searchQuery += string(msg.Runes)
			p.updateFilteredNotes()
		}
		return p, nil
	}
}

// updateFilteredNotes updates the filtered notes list based on current query.
func (p *Plugin) updateFilteredNotes() {
	p.filteredNotes = FilterNotes(p.notes, p.searchQuery)
	// Reset cursor to 0 (or clamp if needed)
	p.cursor = 0
	p.scrollOff = 0

	// NV behavior: if exact match exists, select it automatically
	if p.searchQuery != "" {
		for i, match := range p.filteredNotes {
			if ExactTitleMatch(p.searchQuery, match.Note) {
				p.cursor = i
				break
			}
		}
	}
}

// getDisplayNotes returns the notes to display (filtered or all).
func (p *Plugin) getDisplayNotes() []Note {
	if p.searchQuery != "" && len(p.filteredNotes) > 0 {
		notes := make([]Note, len(p.filteredNotes))
		for i, m := range p.filteredNotes {
			notes[i] = m.Note
		}
		return notes
	}
	return p.notes
}

// getSelectedNote returns the currently selected note from display list.
func (p *Plugin) getSelectedNote() *Note {
	notesList := p.getDisplayNotes()
	if len(notesList) == 0 || p.cursor < 0 || p.cursor >= len(notesList) {
		return nil
	}
	return &notesList[p.cursor]
}

// View renders the plugin.
func (p *Plugin) View(width, height int) string {
	p.width = width
	p.height = height

	// Info modal takes precedence
	if p.showInfoModal {
		p.ensureInfoModal()
		content := p.renderInfoModal()
		return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
	}

	// Delete modal takes precedence
	if p.showDeleteModal {
		p.ensureDeleteModal()
		content := p.renderDeleteModal()
		return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
	}

	// Task modal takes precedence
	if p.showTaskModal {
		p.ensureTaskModal()
		content := p.renderTaskModal()
		return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
	}

	content := p.renderView()

	// Constrain output to allocated height
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
}

// selectedNote returns the currently selected note, or nil if none.
func (p *Plugin) selectedNote() *Note {
	return p.getSelectedNote()
}

// createNote returns a command that creates a new note.
func (p *Plugin) createNote() tea.Cmd {
	return p.createNoteWithTitle("")
}

// createNoteWithTitle returns a command that creates a new note with the given title.
// The title becomes the first line of the note content.
func (p *Plugin) createNoteWithTitle(title string) tea.Cmd {
	if p.store == nil {
		return nil
	}
	epoch := p.ctx.Epoch

	// Use title as initial content (first line) so cursor can be positioned after it
	content := title

	return func() tea.Msg {
		note, err := p.store.Create(title, content)
		if err != nil {
			return NoteSavedMsg{Note: nil, Err: err}
		}
		return NoteSavedMsg{Note: note, Err: nil, Epoch: epoch}
	}
}

// deleteNote returns a command that soft-deletes the selected note.
func (p *Plugin) deleteNote() tea.Cmd {
	note := p.selectedNote()
	if note == nil || p.store == nil {
		return nil
	}

	// Push undo action before delete
	p.pushUndo(UndoAction{
		Type:   UndoDelete,
		NoteID: note.ID,
		Title:  note.Title,
	})

	noteID := note.ID
	epoch := p.ctx.Epoch

	return func() tea.Msg {
		err := p.store.Delete(noteID)
		return NoteDeletedMsg{ID: noteID, Err: err, Epoch: epoch}
	}
}

// togglePin returns a command that toggles the pinned state of the selected note.
func (p *Plugin) togglePin() tea.Cmd {
	note := p.selectedNote()
	if note == nil || p.store == nil {
		return nil
	}
	noteID := note.ID
	epoch := p.ctx.Epoch

	return func() tea.Msg {
		err := p.store.TogglePin(noteID)
		return NotePinToggledMsg{ID: noteID, Err: err, Epoch: epoch}
	}
}

// toggleArchive returns a command that toggles the archived state of the selected note.
func (p *Plugin) toggleArchive() tea.Cmd {
	note := p.selectedNote()
	if note == nil || p.store == nil {
		return nil
	}

	// Push undo action only when archiving (not when unarchiving)
	if !note.Archived {
		p.pushUndo(UndoAction{
			Type:   UndoArchive,
			NoteID: note.ID,
			Title:  note.Title,
		})
	}

	noteID := note.ID
	epoch := p.ctx.Epoch

	return func() tea.Msg {
		err := p.store.ToggleArchive(noteID)
		return NoteArchiveToggledMsg{ID: noteID, Err: err, Epoch: epoch}
	}
}

// yankNoteContent copies the note content to the system clipboard.
func (p *Plugin) yankNoteContent() tea.Cmd {
	note := p.selectedNote()
	if note == nil {
		return nil
	}

	if err := clipboard.WriteAll(note.Content); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Copied note content", 2*time.Second)
}

// yankNoteTitle copies the note title (first line) to the system clipboard.
func (p *Plugin) yankNoteTitle() tea.Cmd {
	note := p.selectedNote()
	if note == nil {
		return nil
	}

	title := note.Title
	if title == "" {
		// Use first line of content if no title
		lines := strings.SplitN(note.Content, "\n", 2)
		if len(lines) > 0 {
			title = strings.TrimSpace(lines[0])
		}
	}

	if title == "" {
		return msg.ShowToast("No title to copy", 2*time.Second)
	}

	if err := clipboard.WriteAll(title); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Copied: "+title, 2*time.Second)
}

// copyEditorContent copies the current editor content to clipboard.
func (p *Plugin) copyEditorContent() tea.Cmd {
	content := p.editorTextarea.Value()
	if content == "" {
		return msg.ShowToast("No content to copy", 2*time.Second)
	}

	if err := clipboard.WriteAll(content); err != nil {
		return msg.ShowToast("Copy failed: "+err.Error(), 2*time.Second)
	}
	return msg.ShowToast("Copied to clipboard", 2*time.Second)
}

// IsFocused returns whether the plugin is focused.
func (p *Plugin) IsFocused() bool { return p.focused }

// SetFocused sets the focus state.
func (p *Plugin) SetFocused(f bool) { p.focused = f }

// Commands returns the available commands.
func (p *Plugin) Commands() []plugin.Command {
	// Info modal commands
	if p.showInfoModal {
		return []plugin.Command{
			{ID: "close", Name: "Close", Description: "Close info modal", Category: plugin.CategoryActions, Context: "notes-info", Priority: 1},
		}
	}
	// Delete modal commands
	if p.showDeleteModal {
		return []plugin.Command{
			{ID: "delete-confirm", Name: "Delete", Description: "Confirm delete", Category: plugin.CategoryActions, Context: "notes-delete-modal", Priority: 1},
			{ID: "cancel", Name: "Cancel", Description: "Cancel delete", Category: plugin.CategoryActions, Context: "notes-delete-modal", Priority: 2},
		}
	}
	// Task modal commands
	if p.showTaskModal {
		return []plugin.Command{
			{ID: "create-task", Name: "Create", Description: "Create task from note", Category: plugin.CategoryActions, Context: "notes-task-modal", Priority: 1},
			{ID: "cancel", Name: "Cancel", Description: "Cancel task creation", Category: plugin.CategoryActions, Context: "notes-task-modal", Priority: 2},
		}
	}
	if p.searchMode {
		return []plugin.Command{
			{ID: "search-confirm", Name: "Select", Description: "Select note or create new", Category: plugin.CategoryActions, Context: "notes-search", Priority: 1},
			{ID: "search-cancel", Name: "Cancel", Description: "Exit search", Category: plugin.CategoryActions, Context: "notes-search", Priority: 2},
		}
	}
	if p.activePane == PaneEditor && p.editorNote != nil {
		if p.previewMode {
			return []plugin.Command{
				{ID: "edit-mode", Name: "Edit", Description: "Enter edit mode", Category: plugin.CategoryActions, Context: "notes-preview", Priority: 1},
				{ID: "switch-pane", Name: "List", Description: "Switch to list pane", Category: plugin.CategoryNavigation, Context: "notes-preview", Priority: 2},
				{ID: "vim-edit", Name: "Vim", Description: "Open in $EDITOR inline", Category: plugin.CategoryActions, Context: "notes-preview", Priority: 3},
				{ID: "external-editor", Name: "Editor", Description: "Open in external editor", Category: plugin.CategoryActions, Context: "notes-preview", Priority: 4},
			}
		}
		cmds := []plugin.Command{
			{ID: "switch-pane", Name: "List", Description: "Switch to list pane", Category: plugin.CategoryNavigation, Context: "notes-editor", Priority: 1},
			{ID: "save", Name: "Save", Description: "Save note", Category: plugin.CategoryActions, Context: "notes-editor", Priority: 2},
			{ID: "vim-edit", Name: "Vim", Description: "Open in $EDITOR inline", Category: plugin.CategoryActions, Context: "notes-editor", Priority: 3},
			{ID: "external-editor", Name: "Editor", Description: "Open in external editor", Category: plugin.CategoryActions, Context: "notes-editor", Priority: 4},
		}
		if p.editorDirty {
			cmds[1].Name = "Save*"
		}
		return cmds
	}
	// Build commands based on current filter view
	cmds := []plugin.Command{
		{ID: "search", Name: "Search", Description: "Search notes", Category: plugin.CategorySearch, Context: "notes-list", Priority: 1},
	}

	// Show view switching commands
	if p.viewFilter == FilterActive {
		cmds = append(cmds,
			plugin.Command{ID: "show-archived", Name: "Archived", Description: "Show archived notes", Category: plugin.CategoryNavigation, Context: "notes-list", Priority: 2},
			plugin.Command{ID: "show-deleted", Name: "Deleted", Description: "Show deleted notes", Category: plugin.CategoryNavigation, Context: "notes-list", Priority: 3},
		)
	} else {
		// Add "Back" command when in Archived or Deleted view
		cmds = append(cmds,
			plugin.Command{ID: "back-to-active", Name: "Active", Description: "Return to active notes", Category: plugin.CategoryNavigation, Context: "notes-list", Priority: 0},
		)
	}

	if p.viewFilter == FilterActive {
		// Full editing commands only in Active view
		cmds = append(cmds,
			plugin.Command{ID: "new-note", Name: "New", Description: "Create new note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 4},
			plugin.Command{ID: "edit-note", Name: "Edit", Description: "Edit selected note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 5},
			plugin.Command{ID: "vim-edit", Name: "Vim", Description: "Open in $EDITOR inline", Category: plugin.CategoryActions, Context: "notes-list", Priority: 6},
			plugin.Command{ID: "external-editor", Name: "Editor", Description: "Open in external editor", Category: plugin.CategoryActions, Context: "notes-list", Priority: 7},
			plugin.Command{ID: "delete-note", Name: "Delete", Description: "Delete selected note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 8},
			plugin.Command{ID: "toggle-pin", Name: "Pin", Description: "Toggle pin on note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 9},
			plugin.Command{ID: "archive-note", Name: "Archive", Description: "Archive selected note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 10},
			plugin.Command{ID: "to-task", Name: "Task", Description: "Convert to task", Category: plugin.CategoryActions, Context: "notes-list", Priority: 11},
			plugin.Command{ID: "show-info", Name: "Info", Description: "Show note info", Category: plugin.CategoryActions, Context: "notes-list", Priority: 12},
		)
		// Show Undo command when undo is available
		if p.hasUndo() {
			cmds = append(cmds,
				plugin.Command{ID: "undo", Name: "Undo", Description: "Undo last delete/archive", Category: plugin.CategoryActions, Context: "notes-list", Priority: 0},
			)
		}
	} else {
		// Read-only view - only preview available
		cmds = append(cmds,
			plugin.Command{ID: "preview-note", Name: "View", Description: "Preview selected note", Category: plugin.CategoryActions, Context: "notes-list", Priority: 4},
			plugin.Command{ID: "show-info", Name: "Info", Description: "Show note info", Category: plugin.CategoryActions, Context: "notes-list", Priority: 5},
		)
	}

	// Yank commands available in all views
	cmds = append(cmds,
		plugin.Command{ID: "yank-content", Name: "Yank", Description: "Copy note content", Category: plugin.CategoryActions, Context: "notes-list", Priority: 13},
		plugin.Command{ID: "yank-title", Name: "YankTitle", Description: "Copy note title", Category: plugin.CategoryActions, Context: "notes-list", Priority: 14},
		plugin.Command{ID: "refresh", Name: "Refresh", Description: "Reload notes", Category: plugin.CategoryActions, Context: "notes-list", Priority: 15},
	)

	return cmds
}

// FocusContext returns the current focus context.
func (p *Plugin) FocusContext() string {
	if p.showInfoModal {
		return "notes-info"
	}
	if p.showDeleteModal {
		return "notes-delete-modal"
	}
	if p.showTaskModal {
		return "notes-task-modal"
	}
	if p.inlineEditMode {
		return "notes-inline-edit"
	}
	if p.searchMode {
		return "notes-search"
	}
	if p.activePane == PaneEditor && p.editorNote != nil {
		if p.previewMode {
			return "notes-preview"
		}
		return "notes-editor"
	}
	return "notes-list"
}

// loadNotes returns a command that loads notes from the store.
func (p *Plugin) loadNotes() tea.Cmd {
	if p.store == nil {
		return nil
	}
	// Only show loading screen on initial load; background refreshes
	// (auto-save, pin, archive, etc.) keep the current view visible.
	if p.notes == nil {
		p.loading = true
	}
	epoch := p.ctx.Epoch
	filter := p.viewFilter

	return func() tea.Msg {
		var notes []Note
		var err error

		switch filter {
		case FilterArchived:
			notes, err = p.store.ListArchived()
		case FilterDeleted:
			notes, err = p.store.ListDeleted()
		default:
			notes, err = p.store.List(false)
		}

		return NotesLoadedMsg{
			Notes: notes,
			Err:   err,
			Epoch: epoch,
		}
	}
}

// showSavedToast shows a toast notification for note save.
func showSavedToast() tea.Cmd {
	return msg.ShowToast("Saved", 2*time.Second)
}

// showRestoredToast shows a toast notification for undo/restore.
func showRestoredToast(title string) tea.Cmd {
	displayTitle := truncateTitle(title, 30)
	text := "Restored"
	if displayTitle != "" {
		text = "Restored: " + displayTitle
	}
	return msg.ShowToast(text, 2*time.Second)
}

// truncateTitle truncates a title to maxLen chars with ellipsis.
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	if maxLen <= 3 {
		return title[:maxLen]
	}
	return title[:maxLen-3] + "..."
}

// isDefaultEditorVim returns true if the default editor config is set to vim/nvim.
func (p *Plugin) isDefaultEditorVim() bool {
	if p.ctx == nil || p.ctx.Config == nil {
		return false
	}
	editor := strings.ToLower(p.ctx.Config.Plugins.Notes.DefaultEditor)
	return editor == "vim" || editor == "nvim"
}

// pushUndo adds an action to the undo stack.
func (p *Plugin) pushUndo(action UndoAction) {
	const maxUndoStack = 20
	p.undoStack = append(p.undoStack, action)
	if len(p.undoStack) > maxUndoStack {
		p.undoStack = p.undoStack[1:]
	}
}

// popUndo removes and returns the last action from the undo stack.
func (p *Plugin) popUndo() *UndoAction {
	if len(p.undoStack) == 0 {
		return nil
	}
	action := p.undoStack[len(p.undoStack)-1]
	p.undoStack = p.undoStack[:len(p.undoStack)-1]
	return &action
}

// hasUndo returns true if there are actions in the undo stack.
func (p *Plugin) hasUndo() bool {
	return len(p.undoStack) > 0
}

// undoLastAction undoes the last delete or archive action.
func (p *Plugin) undoLastAction() tea.Cmd {
	action := p.popUndo()
	if action == nil || p.store == nil {
		return msg.ShowToast("Nothing to undo", 2*time.Second)
	}

	noteID := action.NoteID
	title := action.Title
	actionType := action.Type
	epoch := p.ctx.Epoch

	return func() tea.Msg {
		var err error
		switch actionType {
		case UndoDelete:
			err = p.store.Restore(noteID)
		case UndoArchive:
			err = p.store.Unarchive(noteID)
		}
		return NoteRestoredMsg{
			ID:    noteID,
			Title: title,
			Err:   err,
			Epoch: epoch,
		}
	}
}
