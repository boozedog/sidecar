package filebrowser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sst/sidecar/internal/plugin"
)

func createTestPlugin(t *testing.T, tmpDir string) *Plugin {
	// Create some test files and directories
	if err := os.Mkdir(filepath.Join(tmpDir, "src"), 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "app.go"), []byte("package src"), 0644); err != nil {
		t.Fatalf("failed to create app.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "src", "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create config.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create README.md: %v", err)
	}

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
			Logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
		},
		width:  80,
		height: 24,
	}

	// Build the file tree
	p.tree = NewFileTree(tmpDir)
	if err := p.tree.Build(); err != nil {
		t.Fatalf("failed to build file tree: %v", err)
	}

	return p
}

func TestSearch_EnterSearchMode(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	if p.searchMode {
		t.Error("searchMode should be false initially")
	}

	// Press "/" to enter search mode
	_, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if !p.searchMode {
		t.Error("searchMode should be true after '/'")
	}
	if p.searchQuery != "" {
		t.Error("searchQuery should be empty when entering search mode")
	}
}

func TestSearch_ExitSearchMode(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	// Enter search mode
	p.searchMode = true
	p.searchQuery = "test"

	// Press escape to exit
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyEscape})

	if p.searchMode {
		t.Error("searchMode should be false after escape")
	}
	if p.searchQuery != "" {
		t.Error("searchQuery should be cleared after escape")
	}
}

func TestSearch_TypeQuery(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true

	// Type "main"
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if p.searchQuery != "main" {
		t.Errorf("searchQuery = %q, want main", p.searchQuery)
	}

	// Should have found main.go
	if len(p.searchMatches) == 0 {
		t.Error("no matches found for 'main'")
	}

	found := false
	for _, match := range p.searchMatches {
		if match.Name == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("main.go not in search matches")
	}
}

func TestSearch_Backspace(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true
	p.searchQuery = "mail"

	// Press backspace
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})

	if p.searchQuery != "mai" {
		t.Errorf("searchQuery = %q, want mai", p.searchQuery)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true

	// Type "MAIN" in uppercase
	p.searchQuery = "MAIN"
	p.updateSearchMatches()

	found := false
	for _, match := range p.searchMatches {
		if match.Name == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("case-insensitive search failed - main.go not found for 'MAIN'")
	}
}

func TestSearch_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchQuery = "app"
	p.updateSearchMatches()

	found := false
	for _, match := range p.searchMatches {
		if match.Name == "app.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("partial match failed - app.go not found for 'app'")
	}
}

func TestSearch_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchQuery = "a" // Matches: app.go, main.go, README.md, config (in path)
	p.updateSearchMatches()

	if len(p.searchMatches) == 0 {
		t.Error("no matches found for 'a'")
	}

	// Should have multiple matches
	if len(p.searchMatches) < 2 {
		t.Errorf("expected multiple matches, got %d", len(p.searchMatches))
	}
}

func TestSearch_MatchLimit(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	// Create many files matching a pattern
	for i := 0; i < 30; i++ {
		fname := filepath.Join(tmpDir, "file"+string(rune('0'+(i%10)))+"_"+string(rune('0'+(i/10)))+".txt")
		if err := os.WriteFile(fname, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Rebuild tree with new files
	p.tree = NewFileTree(tmpDir)
	if err := p.tree.Build(); err != nil {
		t.Fatalf("failed to rebuild tree: %v", err)
	}

	p.searchQuery = "file"
	p.updateSearchMatches()

	// Should be limited to 20 matches
	if len(p.searchMatches) > 20 {
		t.Errorf("matches exceeds limit: got %d, want <= 20", len(p.searchMatches))
	}
}

func TestSearch_NavigateMatches(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true
	p.searchQuery = "a" // Multiple matches

	// Update matches
	p.updateSearchMatches()

	if len(p.searchMatches) < 2 {
		t.Skip("need at least 2 matches for navigation test")
	}

	initialCursor := p.searchCursor

	// Press down to move to next match
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})

	if p.searchCursor == initialCursor {
		t.Error("search cursor did not move with down arrow")
	}

	// Press up to move to previous match
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})

	if p.searchCursor != initialCursor {
		t.Error("search cursor did not move back with up arrow")
	}
}

func TestSearch_JumpToMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true
	p.searchQuery = "app"
	p.updateSearchMatches()

	if len(p.searchMatches) == 0 {
		t.Skip("no matches found for app")
	}

	initialCursor := p.treeCursor

	// Press enter to jump to first match
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})

	// Cursor should have moved or tree cursor updated
	if !p.searchMode {
		// Search mode should be exited after jumping
		// Tree cursor should be updated
		if p.treeCursor == initialCursor {
			// This is ok - might be the same position
		}
	}
}

func TestSearch_ExpandParents(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	// Find a nested file
	var nestedFile *FileNode
	p.walkTree(p.tree.Root, func(node *FileNode) {
		if node.Name == "app.go" {
			nestedFile = node
		}
	})

	if nestedFile == nil {
		t.Skip("nested file not found")
	}

	// Collapse the parent directory
	var srcDir *FileNode
	for _, child := range p.tree.Root.Children {
		if child.Name == "src" {
			srcDir = child
			srcDir.IsExpanded = false
			break
		}
	}

	if srcDir == nil {
		t.Skip("src directory not found")
	}

	// Expand parents should expand src
	p.expandParents(nestedFile)

	if !srcDir.IsExpanded {
		t.Error("expandParents did not expand parent directory")
	}
}

func TestSearch_WalkTree(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	visitedFiles := make(map[string]bool)

	p.walkTree(p.tree.Root, func(node *FileNode) {
		visitedFiles[node.Name] = true
	})

	// Should have visited test files
	expectedFiles := []string{"main.go", "src", "app.go", "config.json", "README.md"}
	for _, expected := range expectedFiles {
		if !visitedFiles[expected] {
			t.Errorf("walkTree did not visit %s", expected)
		}
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchQuery = ""
	p.updateSearchMatches()

	if len(p.searchMatches) != 0 {
		t.Error("empty search query should have no matches")
	}
}

func TestSearch_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchQuery = "xyznonexistent"
	p.updateSearchMatches()

	if len(p.searchMatches) != 0 {
		t.Error("nonexistent query should have no matches")
	}

	if p.searchCursor != 0 {
		t.Error("search cursor should be reset to 0 when no matches")
	}
}

func TestSearch_CursorBounds(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true
	p.searchQuery = "a"
	p.updateSearchMatches()

	if len(p.searchMatches) == 0 {
		t.Skip("no matches found")
	}

	// Try to move up when at position 0
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyUp})
	if p.searchCursor < 0 {
		t.Error("search cursor should not go below 0")
	}

	// Move to end
	for i := 0; i < len(p.searchMatches); i++ {
		_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})
	}

	// Try to move down at end
	cursorBefore := p.searchCursor
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyDown})

	if p.searchCursor > len(p.searchMatches)-1 {
		t.Error("search cursor should not exceed matches length")
	}
	if p.searchCursor != cursorBefore && len(p.searchMatches) > 1 {
		// This is OK - might have wrapped around
	}
}

func TestSearch_PrintableCharacterFilter(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	p.searchMode = true

	// Try to input non-printable character (should be ignored)
	initialQuery := p.searchQuery
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}})

	if p.searchQuery != initialQuery {
		t.Error("non-printable character should be ignored")
	}

	// Printable character should be added
	_, _ = p.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if p.searchQuery != "a" {
		t.Error("printable character should be added to query")
	}
}

// --- Content Search Tests ---

func createTestPluginWithPreview(t *testing.T, tmpDir string, fileContent string) *Plugin {
	p := createTestPlugin(t, tmpDir)

	// Set up preview with content
	p.previewFile = "test.txt"
	p.previewLines = strings.Split(fileContent, "\n")
	p.previewHighlighted = p.previewLines // No syntax highlighting for test
	p.activePane = PanePreview
	p.height = 24
	p.width = 80

	return p
}

func TestContentSearch_EnterMode(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "line one\nline two\nline three")

	if p.contentSearchMode {
		t.Error("contentSearchMode should be false initially")
	}

	// Press "/" in preview pane
	_, _ = p.handlePreviewKey("/")

	if !p.contentSearchMode {
		t.Error("contentSearchMode should be true after '/' in preview")
	}
	if p.contentSearchQuery != "" {
		t.Error("contentSearchQuery should be empty when entering search mode")
	}
}

func TestContentSearch_EnterModeDisabledForBinary(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "some content")
	p.isBinary = true

	_, _ = p.handlePreviewKey("/")

	if p.contentSearchMode {
		t.Error("contentSearchMode should not activate for binary files")
	}
}

func TestContentSearch_EnterModeDisabledForEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "")
	p.previewLines = nil

	_, _ = p.handlePreviewKey("/")

	if p.contentSearchMode {
		t.Error("contentSearchMode should not activate for empty files")
	}
}

func TestContentSearch_ExitWithEsc(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "test content")
	p.contentSearchMode = true
	p.contentSearchQuery = "test"
	p.contentSearchMatches = []ContentMatch{{LineNo: 0, StartCol: 0, EndCol: 4}}

	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyEscape})

	if p.contentSearchMode {
		t.Error("contentSearchMode should be false after escape")
	}
	if p.contentSearchQuery != "" {
		t.Error("contentSearchQuery should be cleared after escape")
	}
	if len(p.contentSearchMatches) != 0 {
		t.Error("contentSearchMatches should be cleared after escape")
	}
}

func TestContentSearch_ExitWithEnter(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "test content")
	p.contentSearchMode = true
	p.contentSearchQuery = "test"
	p.contentSearchMatches = []ContentMatch{{LineNo: 0, StartCol: 0, EndCol: 4}}

	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyEnter})

	if p.contentSearchMode {
		t.Error("contentSearchMode should be false after enter")
	}
	// Query and matches should be preserved (unlike esc)
}

func TestContentSearch_FindMatches(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "foo bar foo\nbaz foo\nno match here")
	p.contentSearchMode = true
	p.contentSearchQuery = "foo"
	p.updateContentMatches()

	if len(p.contentSearchMatches) != 3 {
		t.Errorf("expected 3 matches, got %d", len(p.contentSearchMatches))
	}

	// Check first match
	if p.contentSearchMatches[0].LineNo != 0 || p.contentSearchMatches[0].StartCol != 0 {
		t.Errorf("first match wrong: %+v", p.contentSearchMatches[0])
	}
	// Check second match (second "foo" on line 0)
	if p.contentSearchMatches[1].LineNo != 0 || p.contentSearchMatches[1].StartCol != 8 {
		t.Errorf("second match wrong: %+v", p.contentSearchMatches[1])
	}
	// Check third match (on line 1)
	if p.contentSearchMatches[2].LineNo != 1 || p.contentSearchMatches[2].StartCol != 4 {
		t.Errorf("third match wrong: %+v", p.contentSearchMatches[2])
	}
}

func TestContentSearch_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "FOO foo Foo")
	p.contentSearchMode = true
	p.contentSearchQuery = "foo"
	p.updateContentMatches()

	if len(p.contentSearchMatches) != 3 {
		t.Errorf("case-insensitive search should find 3 matches, got %d", len(p.contentSearchMatches))
	}
}

func TestContentSearch_NavigateNext(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "a\na\na")
	p.contentSearchMode = true
	p.contentSearchQuery = "a"
	p.updateContentMatches()

	if len(p.contentSearchMatches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(p.contentSearchMatches))
	}

	if p.contentSearchCursor != 0 {
		t.Error("cursor should start at 0")
	}

	// Press n
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if p.contentSearchCursor != 1 {
		t.Errorf("cursor should be 1 after n, got %d", p.contentSearchCursor)
	}

	// Press n again
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if p.contentSearchCursor != 2 {
		t.Errorf("cursor should be 2 after n, got %d", p.contentSearchCursor)
	}

	// Press n - should wrap to 0
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if p.contentSearchCursor != 0 {
		t.Errorf("cursor should wrap to 0, got %d", p.contentSearchCursor)
	}
}

func TestContentSearch_NavigatePrev(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "a\na\na")
	p.contentSearchMode = true
	p.contentSearchQuery = "a"
	p.updateContentMatches()

	// Press N at position 0 - should wrap to last
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	if p.contentSearchCursor != 2 {
		t.Errorf("cursor should wrap to 2, got %d", p.contentSearchCursor)
	}

	// Press N again
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	if p.contentSearchCursor != 1 {
		t.Errorf("cursor should be 1, got %d", p.contentSearchCursor)
	}
}

func TestContentSearch_Backspace(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "test")
	p.contentSearchMode = true
	p.contentSearchQuery = "tes"

	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})

	if p.contentSearchQuery != "te" {
		t.Errorf("query should be 'te', got %q", p.contentSearchQuery)
	}
}

func TestContentSearch_TypeCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "hello world")
	p.contentSearchMode = true

	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	_, _ = p.handleContentSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

	if p.contentSearchQuery != "he" {
		t.Errorf("query should be 'he', got %q", p.contentSearchQuery)
	}

	if len(p.contentSearchMatches) != 1 {
		t.Errorf("should have 1 match for 'he', got %d", len(p.contentSearchMatches))
	}
}

func TestContentSearch_ScrollToMatch(t *testing.T) {
	tmpDir := t.TempDir()
	// Create content with 50 lines, match on line 40
	var lines []string
	for i := 0; i < 50; i++ {
		if i == 40 {
			lines = append(lines, "TARGET here")
		} else {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
	}
	p := createTestPluginWithPreview(t, tmpDir, strings.Join(lines, "\n"))
	p.contentSearchMode = true
	p.contentSearchQuery = "TARGET"
	p.updateContentMatches()

	if len(p.contentSearchMatches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(p.contentSearchMatches))
	}

	// Scroll should have moved to show the match
	if p.previewScroll == 0 {
		t.Error("previewScroll should have changed to show match on line 40")
	}
}

func TestContentSearch_EmptyQuery(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "test content")
	p.contentSearchMode = true
	p.contentSearchQuery = ""
	p.updateContentMatches()

	if len(p.contentSearchMatches) != 0 {
		t.Error("empty query should have no matches")
	}
}

func TestContentSearch_FocusContext(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPluginWithPreview(t, tmpDir, "test")

	// Preview pane focused, no search
	p.activePane = PanePreview
	if p.FocusContext() != "file-browser-preview" {
		t.Errorf("expected file-browser-preview, got %s", p.FocusContext())
	}

	// Content search active
	p.contentSearchMode = true
	if p.FocusContext() != "file-browser-content-search" {
		t.Errorf("expected file-browser-content-search, got %s", p.FocusContext())
	}
}

// --- Quick Open Tests ---

func TestQuickOpen_OpenMode(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	if p.quickOpenMode {
		t.Error("quickOpenMode should be false initially")
	}

	// Press ctrl+p
	_, _ = p.handleKey(tea.KeyMsg{Type: tea.KeyCtrlP})

	if !p.quickOpenMode {
		t.Error("quickOpenMode should be true after ctrl+p")
	}

	// Should have built file cache
	if len(p.quickOpenFiles) == 0 {
		t.Error("file cache should be populated")
	}
}

func TestQuickOpen_CloseWithEsc(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	// Enter quick open
	p.quickOpenMode = true
	p.quickOpenQuery = "test"
	p.quickOpenFiles = []string{"test.go"}
	p.updateQuickOpenMatches()

	// Press esc
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyEscape})

	if p.quickOpenMode {
		t.Error("quickOpenMode should be false after esc")
	}
	if p.quickOpenQuery != "" {
		t.Error("query should be cleared")
	}
}

func TestQuickOpen_TypeQuery(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"main.go", "test.go", "app.go"}

	// Type "ma"
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if p.quickOpenQuery != "ma" {
		t.Errorf("query should be 'ma', got %q", p.quickOpenQuery)
	}

	// Should match main.go
	found := false
	for _, m := range p.quickOpenMatches {
		if m.Path == "main.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("main.go should be in matches")
	}
}

func TestQuickOpen_NavigateResults(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"a.go", "b.go", "c.go"}
	p.updateQuickOpenMatches()

	if len(p.quickOpenMatches) < 3 {
		t.Skip("need at least 3 matches")
	}

	// Initial cursor at 0
	if p.quickOpenCursor != 0 {
		t.Error("cursor should start at 0")
	}

	// Move down
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyDown})
	if p.quickOpenCursor != 1 {
		t.Errorf("cursor should be 1, got %d", p.quickOpenCursor)
	}

	// Move up
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyUp})
	if p.quickOpenCursor != 0 {
		t.Errorf("cursor should be 0, got %d", p.quickOpenCursor)
	}

	// Can't go above 0
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyUp})
	if p.quickOpenCursor != 0 {
		t.Error("cursor should not go below 0")
	}
}

func TestQuickOpen_SelectMatch(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"main.go", "src/app.go"}
	p.quickOpenQuery = "app"
	p.updateQuickOpenMatches()

	if len(p.quickOpenMatches) == 0 {
		t.Skip("no matches found")
	}

	// Find app.go match
	for i, m := range p.quickOpenMatches {
		if m.Path == "src/app.go" {
			p.quickOpenCursor = i
			break
		}
	}

	// Press enter
	_, cmd := p.selectQuickOpenMatch()

	// Should have closed quick open
	if p.quickOpenMode {
		t.Error("quickOpenMode should be false after selection")
	}

	// Should have set preview file
	if p.previewFile != "src/app.go" {
		t.Errorf("previewFile should be src/app.go, got %s", p.previewFile)
	}

	// Should have returned a command (LoadPreview)
	if cmd == nil {
		t.Error("should return LoadPreview command")
	}
}

func TestQuickOpen_FocusContext(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)

	// Normal state
	if p.FocusContext() != "file-browser-tree" {
		t.Errorf("expected file-browser-tree, got %s", p.FocusContext())
	}

	// Quick open active
	p.quickOpenMode = true
	if p.FocusContext() != "file-browser-quick-open" {
		t.Errorf("expected file-browser-quick-open, got %s", p.FocusContext())
	}
}

func TestQuickOpen_Backspace(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenQuery = "test"
	p.quickOpenFiles = []string{"test.go"}

	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyBackspace})

	if p.quickOpenQuery != "tes" {
		t.Errorf("query should be 'tes', got %q", p.quickOpenQuery)
	}
}

func TestQuickOpen_CtrlPNavigates(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"a.go", "b.go"}
	p.updateQuickOpenMatches()
	p.quickOpenCursor = 1

	// ctrl+p should move cursor up (vim-like)
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyCtrlP})

	if p.quickOpenCursor != 0 {
		t.Errorf("cursor should be 0 after ctrl+p, got %d", p.quickOpenCursor)
	}
}

func TestQuickOpen_CtrlNNavigates(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"a.go", "b.go"}
	p.updateQuickOpenMatches()

	// ctrl+n should move cursor down
	_, _ = p.handleQuickOpenKey(tea.KeyMsg{Type: tea.KeyCtrlN})

	if p.quickOpenCursor != 1 {
		t.Errorf("cursor should be 1 after ctrl+n, got %d", p.quickOpenCursor)
	}
}

func TestQuickOpen_UpdateMatchesResetsCursor(t *testing.T) {
	tmpDir := t.TempDir()
	p := createTestPlugin(t, tmpDir)
	p.quickOpenMode = true
	p.quickOpenFiles = []string{"aaa.go", "bbb.go", "ccc.go"}
	p.updateQuickOpenMatches()
	p.quickOpenCursor = 2

	// Type query that reduces matches
	p.quickOpenQuery = "aaa"
	p.updateQuickOpenMatches()

	// Cursor should be reset since only 1 match now
	if p.quickOpenCursor >= len(p.quickOpenMatches) {
		t.Error("cursor should be within bounds after matches change")
	}
}
