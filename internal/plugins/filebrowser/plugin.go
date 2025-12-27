package filebrowser

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sst/sidecar/internal/plugin"
)

const (
	pluginID   = "file-browser"
	pluginName = "files"
	pluginIcon = "F"

	// Quick open limits
	quickOpenMaxFiles   = 50000           // Max files to cache (prevents OOM on huge repos)
	quickOpenMaxResults = 50              // Max matches to show
	quickOpenTimeout    = 2 * time.Second // Max time to spend scanning
)

// Message types
type (
	RefreshMsg      struct{}
	TreeBuiltMsg    struct{ Err error }
	WatchStartedMsg struct{ Watcher *Watcher }
	WatchEventMsg   struct{}
	OpenFileMsg     struct {
		Editor string
		Path   string
	}
)

// ContentMatch represents a match position within file content.
type ContentMatch struct {
	LineNo   int // 0-indexed line number
	StartCol int // Start column (byte offset)
	EndCol   int // End column (byte offset)
}

// Plugin implements file browser functionality.
type Plugin struct {
	ctx     *plugin.Context
	tree    *FileTree
	focused bool

	// Pane state
	activePane FocusPane

	// Tree state
	treeCursor    int
	treeScrollOff int

	// Preview state
	previewFile        string
	previewLines       []string
	previewHighlighted []string
	previewScroll      int
	previewError       error
	isBinary           bool
	isTruncated        bool

	// Dimensions
	width, height int
	treeWidth     int
	previewWidth  int

	// Search state (tree filename search)
	searchMode    bool
	searchQuery   string
	searchMatches []*FileNode
	searchCursor  int

	// Content search state (preview pane)
	contentSearchMode      bool
	contentSearchCommitted bool // True after Enter confirms query (enables n/N navigation)
	contentSearchQuery     string
	contentSearchMatches   []ContentMatch
	contentSearchCursor    int // Index into contentSearchMatches

	// Quick open state
	quickOpenMode    bool
	quickOpenQuery   string
	quickOpenMatches []QuickOpenMatch
	quickOpenCursor  int
	quickOpenFiles   []string // Cached file paths (relative)
	quickOpenError   string   // Error message if scan failed/limited

	// File watcher
	watcher *Watcher
}

// New creates a new File Browser plugin.
func New() *Plugin {
	return &Plugin{}
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
	p.tree = NewFileTree(ctx.WorkDir)
	return nil
}

// Start begins plugin operation.
func (p *Plugin) Start() tea.Cmd {
	return tea.Batch(
		p.refresh(),
		p.startWatcher(),
	)
}

// Stop cleans up plugin resources.
func (p *Plugin) Stop() {
	if p.watcher != nil {
		p.watcher.Stop()
	}
}

// startWatcher initializes the file system watcher.
func (p *Plugin) startWatcher() tea.Cmd {
	return func() tea.Msg {
		watcher, err := NewWatcher(p.ctx.WorkDir)
		if err != nil {
			p.ctx.Logger.Error("file browser: watcher failed", "error", err)
			return nil
		}
		return WatchStartedMsg{Watcher: watcher}
	}
}

// listenForWatchEvents waits for the next file system event.
func (p *Plugin) listenForWatchEvents() tea.Cmd {
	if p.watcher == nil {
		return nil
	}
	return func() tea.Msg {
		<-p.watcher.Events()
		return WatchEventMsg{}
	}
}

// refresh rebuilds the file tree.
func (p *Plugin) refresh() tea.Cmd {
	return func() tea.Msg {
		err := p.tree.Build()
		return TreeBuiltMsg{Err: err}
	}
}

// openFile returns a command to open a file in the user's editor.
func (p *Plugin) openFile(path string) tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			editor = "vim"
		}
		fullPath := filepath.Join(p.ctx.WorkDir, path)
		return OpenFileMsg{Editor: editor, Path: fullPath}
	}
}

// Update handles messages.
func (p *Plugin) Update(msg tea.Msg) (plugin.Plugin, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height

	case TreeBuiltMsg:
		if msg.Err != nil {
			p.ctx.Logger.Error("file browser: tree build failed", "error", msg.Err)
		}

	case PreviewLoadedMsg:
		if msg.Path == p.previewFile {
			p.previewLines = msg.Result.Lines
			p.previewHighlighted = msg.Result.HighlightedLines
			p.isBinary = msg.Result.IsBinary
			p.isTruncated = msg.Result.IsTruncated
			p.previewError = msg.Result.Error
			p.previewScroll = 0
			// Re-run search if still in search mode (e.g., navigating files with j/k)
			if p.contentSearchMode && p.contentSearchQuery != "" {
				p.updateContentMatches()
			}
		}

	case RefreshMsg:
		return p, p.refresh()

	case WatchStartedMsg:
		p.watcher = msg.Watcher
		return p, p.listenForWatchEvents()

	case WatchEventMsg:
		// File system changed, refresh tree and continue listening
		return p, tea.Batch(
			p.refresh(),
			p.listenForWatchEvents(),
		)

	case tea.KeyMsg:
		return p.handleKey(msg)
	}

	return p, nil
}

func (p *Plugin) handleKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	// Quick open can be triggered from any context (except when already open)
	if key == "ctrl+p" && !p.quickOpenMode {
		return p.openQuickOpen()
	}

	// Handle quick open mode
	if p.quickOpenMode {
		return p.handleQuickOpenKey(msg)
	}

	// Handle content search mode input (preview pane search)
	if p.contentSearchMode {
		return p.handleContentSearchKey(msg)
	}

	// Handle tree search mode input
	if p.searchMode {
		return p.handleSearchKey(msg)
	}

	// Handle keys based on active pane
	if p.activePane == PanePreview {
		return p.handlePreviewKey(key)
	}
	return p.handleTreeKey(key)
}

func (p *Plugin) handleTreeKey(key string) (plugin.Plugin, tea.Cmd) {
	switch key {
	case "j", "down":
		if p.treeCursor < p.tree.Len()-1 {
			p.treeCursor++
			p.ensureTreeCursorVisible()
		}

	case "k", "up":
		if p.treeCursor > 0 {
			p.treeCursor--
			p.ensureTreeCursorVisible()
		}

	case "l", "right":
		node := p.tree.GetNode(p.treeCursor)
		if node != nil {
			if node.IsDir {
				_ = p.tree.Expand(node)
			} else {
				// Load file preview and switch to preview pane
				p.previewFile = node.Path
				p.previewScroll = 0
				p.previewLines = nil
				p.previewError = nil
				p.isBinary = false
				p.isTruncated = false
				p.activePane = PanePreview // Switch to preview pane
				return p, LoadPreview(p.ctx.WorkDir, node.Path)
			}
		}

	case "enter":
		node := p.tree.GetNode(p.treeCursor)
		if node != nil {
			if node.IsDir {
				// Toggle expand/collapse
				_ = p.tree.Toggle(node)
			} else {
				// Load file preview and switch to preview pane
				p.previewFile = node.Path
				p.previewScroll = 0
				p.previewLines = nil
				p.previewError = nil
				p.isBinary = false
				p.isTruncated = false
				p.activePane = PanePreview
				return p, LoadPreview(p.ctx.WorkDir, node.Path)
			}
		}

	case "h", "left":
		node := p.tree.GetNode(p.treeCursor)
		if node != nil {
			if node.IsDir && node.IsExpanded {
				p.tree.Collapse(node)
			} else if node.Parent != nil && node.Parent != p.tree.Root {
				if idx := p.tree.IndexOf(node.Parent); idx >= 0 {
					p.treeCursor = idx
					p.ensureTreeCursorVisible()
				}
			}
		}

	case "g":
		p.treeCursor = 0
		p.treeScrollOff = 0

	case "G":
		if p.tree.Len() > 0 {
			p.treeCursor = p.tree.Len() - 1
			p.ensureTreeCursorVisible()
		}

	case "ctrl+d":
		visibleHeight := p.visibleContentHeight()
		p.treeCursor += visibleHeight / 2
		if p.treeCursor >= p.tree.Len() {
			p.treeCursor = p.tree.Len() - 1
		}
		p.ensureTreeCursorVisible()

	case "ctrl+u":
		visibleHeight := p.visibleContentHeight()
		p.treeCursor -= visibleHeight / 2
		if p.treeCursor < 0 {
			p.treeCursor = 0
		}
		p.ensureTreeCursorVisible()

	case "ctrl+f", "pgdown":
		visibleHeight := p.visibleContentHeight()
		p.treeCursor += visibleHeight
		if p.treeCursor >= p.tree.Len() {
			p.treeCursor = p.tree.Len() - 1
		}
		p.ensureTreeCursorVisible()

	case "ctrl+b", "pgup":
		visibleHeight := p.visibleContentHeight()
		p.treeCursor -= visibleHeight
		if p.treeCursor < 0 {
			p.treeCursor = 0
		}
		p.ensureTreeCursorVisible()

	case "r":
		return p, p.refresh()

	case "e", "o":
		node := p.tree.GetNode(p.treeCursor)
		if node != nil && !node.IsDir {
			return p, p.openFile(node.Path)
		}

	case "/":
		p.searchMode = true
		p.searchQuery = ""
		p.searchMatches = nil
		p.searchCursor = 0

	case "n":
		// Next match
		if len(p.searchMatches) > 0 {
			p.searchCursor = (p.searchCursor + 1) % len(p.searchMatches)
			p.jumpToSearchMatch()
		}

	case "N":
		// Previous match
		if len(p.searchMatches) > 0 {
			p.searchCursor--
			if p.searchCursor < 0 {
				p.searchCursor = len(p.searchMatches) - 1
			}
			p.jumpToSearchMatch()
		}
	}

	return p, nil
}

func (p *Plugin) handlePreviewKey(key string) (plugin.Plugin, tea.Cmd) {
	lines := p.previewHighlighted
	if len(lines) == 0 {
		lines = p.previewLines
	}
	visibleHeight := p.visibleContentHeight()
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch key {
	case "j", "down":
		if p.previewScroll < maxScroll {
			p.previewScroll++
		}

	case "k", "up":
		if p.previewScroll > 0 {
			p.previewScroll--
		}

	case "g":
		p.previewScroll = 0

	case "G":
		p.previewScroll = maxScroll

	case "ctrl+d":
		p.previewScroll += visibleHeight / 2
		if p.previewScroll > maxScroll {
			p.previewScroll = maxScroll
		}

	case "ctrl+u":
		p.previewScroll -= visibleHeight / 2
		if p.previewScroll < 0 {
			p.previewScroll = 0
		}

	case "ctrl+f", "pgdown":
		p.previewScroll += visibleHeight
		if p.previewScroll > maxScroll {
			p.previewScroll = maxScroll
		}

	case "ctrl+b", "pgup":
		p.previewScroll -= visibleHeight
		if p.previewScroll < 0 {
			p.previewScroll = 0
		}

	case "h", "left", "esc":
		// Return to tree pane
		p.activePane = PaneTree

	case "e":
		// Open previewed file in editor
		if p.previewFile != "" {
			return p, p.openFile(p.previewFile)
		}

	case "/":
		// Enter content search mode if we have content to search
		if len(p.previewLines) > 0 && !p.isBinary {
			p.contentSearchMode = true
			p.contentSearchCommitted = false
			p.contentSearchQuery = ""
			p.contentSearchMatches = nil
			p.contentSearchCursor = 0
		}
	}

	return p, nil
}

// handleContentSearchKey handles key input during content search mode.
// Implements vim-style two-phase search: type query, Enter to commit, then n/N navigate.
func (p *Plugin) handleContentSearchKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	// Esc always exits search mode completely
	if key == "esc" {
		p.contentSearchMode = false
		p.contentSearchCommitted = false
		p.contentSearchQuery = ""
		p.contentSearchMatches = nil
		p.contentSearchCursor = 0
		return p, nil
	}

	// Phase 1: Typing query (not yet committed)
	if !p.contentSearchCommitted {
		switch key {
		case "enter":
			// Commit the search - now n/N will navigate matches
			if len(p.contentSearchQuery) > 0 {
				p.contentSearchCommitted = true
			}
		case "backspace":
			if len(p.contentSearchQuery) > 0 {
				p.contentSearchQuery = p.contentSearchQuery[:len(p.contentSearchQuery)-1]
				p.updateContentMatches()
			}
		default:
			// All printable characters go to query (including n, N, etc.)
			if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				p.contentSearchQuery += key
				p.updateContentMatches()
			}
		}
		return p, nil
	}

	// Phase 2: Search committed - n/N navigate matches, j/k exit and navigate tree
	switch key {
	case "n":
		if len(p.contentSearchMatches) > 0 {
			p.contentSearchCursor = (p.contentSearchCursor + 1) % len(p.contentSearchMatches)
			p.scrollToContentMatch()
		}
	case "N":
		if len(p.contentSearchMatches) > 0 {
			p.contentSearchCursor--
			if p.contentSearchCursor < 0 {
				p.contentSearchCursor = len(p.contentSearchMatches) - 1
			}
			p.scrollToContentMatch()
		}
	case "j", "down":
		// Move to next file, keep search active
		if p.treeCursor < p.tree.Len()-1 {
			p.treeCursor++
			p.ensureTreeCursorVisible()
			p.contentSearchMatches = nil
			p.contentSearchCursor = 0
			return p, p.loadPreviewForCursor()
		}
	case "k", "up":
		// Move to previous file, keep search active
		if p.treeCursor > 0 {
			p.treeCursor--
			p.ensureTreeCursorVisible()
			p.contentSearchMatches = nil
			p.contentSearchCursor = 0
			return p, p.loadPreviewForCursor()
		}
	case "enter":
		// Exit search, keep position at current match
		p.contentSearchMode = false
		p.contentSearchCommitted = false
	}

	return p, nil
}

// updateContentMatches finds all matches of the search query in preview content.
func (p *Plugin) updateContentMatches() {
	p.contentSearchMatches = nil
	p.contentSearchCursor = 0

	if p.contentSearchQuery == "" {
		return
	}

	query := strings.ToLower(p.contentSearchQuery)

	for lineNo, line := range p.previewLines {
		lineLower := strings.ToLower(line)
		startIdx := 0
		for {
			idx := strings.Index(lineLower[startIdx:], query)
			if idx == -1 {
				break
			}
			absIdx := startIdx + idx
			p.contentSearchMatches = append(p.contentSearchMatches, ContentMatch{
				LineNo:   lineNo,
				StartCol: absIdx,
				EndCol:   absIdx + len(p.contentSearchQuery),
			})
			startIdx = absIdx + 1
		}
	}

	// Scroll to first match if any
	if len(p.contentSearchMatches) > 0 {
		p.scrollToContentMatch()
	}
}

// scrollToContentMatch scrolls the preview to show the current match.
func (p *Plugin) scrollToContentMatch() {
	if len(p.contentSearchMatches) == 0 || p.contentSearchCursor >= len(p.contentSearchMatches) {
		return
	}

	match := p.contentSearchMatches[p.contentSearchCursor]
	visibleHeight := p.visibleContentHeight()

	// Center the match line in viewport if possible
	targetScroll := match.LineNo - visibleHeight/2
	if targetScroll < 0 {
		targetScroll = 0
	}

	maxScroll := len(p.previewLines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if targetScroll > maxScroll {
		targetScroll = maxScroll
	}

	p.previewScroll = targetScroll
}

// openQuickOpen enters quick open mode.
func (p *Plugin) openQuickOpen() (plugin.Plugin, tea.Cmd) {
	// Build file cache if empty
	if len(p.quickOpenFiles) == 0 {
		p.buildFileCache()
	}

	p.quickOpenMode = true
	p.quickOpenQuery = ""
	p.quickOpenCursor = 0
	p.updateQuickOpenMatches()

	return p, nil
}

// handleQuickOpenKey handles key input during quick open mode.
func (p *Plugin) handleQuickOpenKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		p.quickOpenMode = false
		p.quickOpenQuery = ""
		p.quickOpenMatches = nil
		p.quickOpenCursor = 0

	case "enter":
		if len(p.quickOpenMatches) > 0 && p.quickOpenCursor < len(p.quickOpenMatches) {
			return p.selectQuickOpenMatch()
		}

	case "up", "ctrl+p":
		if p.quickOpenCursor > 0 {
			p.quickOpenCursor--
		}

	case "down", "ctrl+n":
		if p.quickOpenCursor < len(p.quickOpenMatches)-1 {
			p.quickOpenCursor++
		}

	case "backspace":
		if len(p.quickOpenQuery) > 0 {
			p.quickOpenQuery = p.quickOpenQuery[:len(p.quickOpenQuery)-1]
			p.updateQuickOpenMatches()
		}

	default:
		// Append printable characters
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			p.quickOpenQuery += key
			p.updateQuickOpenMatches()
		}
	}

	return p, nil
}

// updateQuickOpenMatches filters files using fuzzy matching.
func (p *Plugin) updateQuickOpenMatches() {
	p.quickOpenMatches = FuzzyFilter(p.quickOpenFiles, p.quickOpenQuery, quickOpenMaxResults)

	// Reset cursor if out of bounds
	if p.quickOpenCursor >= len(p.quickOpenMatches) {
		if len(p.quickOpenMatches) > 0 {
			p.quickOpenCursor = len(p.quickOpenMatches) - 1
		} else {
			p.quickOpenCursor = 0
		}
	}
}

// selectQuickOpenMatch opens the selected file in preview.
func (p *Plugin) selectQuickOpenMatch() (plugin.Plugin, tea.Cmd) {
	if len(p.quickOpenMatches) == 0 || p.quickOpenCursor >= len(p.quickOpenMatches) {
		return p, nil
	}

	match := p.quickOpenMatches[p.quickOpenCursor]

	// Close quick open
	p.quickOpenMode = false
	p.quickOpenQuery = ""
	p.quickOpenMatches = nil
	p.quickOpenCursor = 0

	// Find the file in tree and expand parents
	var targetNode *FileNode
	p.walkTree(p.tree.Root, func(node *FileNode) {
		if node.Path == match.Path {
			targetNode = node
		}
	})

	if targetNode != nil {
		// Expand parents to make visible
		p.expandParents(targetNode)
		p.tree.Flatten()

		// Move tree cursor to file
		if idx := p.tree.IndexOf(targetNode); idx >= 0 {
			p.treeCursor = idx
			p.ensureTreeCursorVisible()
		}
	}

	// Load preview
	p.previewFile = match.Path
	p.previewScroll = 0
	p.previewLines = nil
	p.previewError = nil
	p.isBinary = false
	p.isTruncated = false
	p.activePane = PanePreview

	return p, LoadPreview(p.ctx.WorkDir, match.Path)
}

// buildFileCache walks the filesystem to build the quick open file list.
// Respects gitignore and has limits to prevent issues on huge repos.
func (p *Plugin) buildFileCache() {
	p.quickOpenFiles = nil
	p.quickOpenError = ""

	ctx, cancel := context.WithTimeout(context.Background(), quickOpenTimeout)
	defer cancel()

	count := 0
	limited := false

	err := filepath.WalkDir(p.ctx.WorkDir, func(path string, d fs.DirEntry, err error) error {
		// Check timeout
		select {
		case <-ctx.Done():
			limited = true
			return filepath.SkipAll
		default:
		}

		if err != nil {
			return nil // Skip unreadable entries
		}

		// Get relative path
		rel, err := filepath.Rel(p.ctx.WorkDir, path)
		if err != nil {
			return nil
		}

		// Skip root
		if rel == "." {
			return nil
		}

		// Skip common large/irrelevant directories
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == ".next" || name == "dist" || name == "build" ||
				name == "__pycache__" || name == ".venv" || name == "venv" ||
				name == ".idea" || name == ".vscode" {
				return filepath.SkipDir
			}
			// Check gitignore for directories
			if p.tree != nil && p.tree.gitIgnore != nil {
				if p.tree.gitIgnore.IsIgnored(rel, true) {
					return filepath.SkipDir
				}
			}
			return nil // Don't add directories to file list
		}

		// Skip hidden files (starting with .)
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Check gitignore for files
		if p.tree != nil && p.tree.gitIgnore != nil {
			if p.tree.gitIgnore.IsIgnored(rel, false) {
				return nil
			}
		}

		// Check file limit
		if count >= quickOpenMaxFiles {
			limited = true
			return filepath.SkipAll
		}

		p.quickOpenFiles = append(p.quickOpenFiles, rel)
		count++
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		p.quickOpenError = "scan error: " + err.Error()
	} else if limited {
		if ctx.Err() != nil {
			p.quickOpenError = "scan timed out"
		} else {
			p.quickOpenError = "limited to 50000 files"
		}
	}

	// Sort files by path for consistent ordering
	sort.Strings(p.quickOpenFiles)
}

// visibleContentHeight returns the number of lines available for content.
func (p *Plugin) visibleContentHeight() int {
	// height - footer (1) - search bar (0 or 1) - pane border (2) - header (2)
	searchBar := 0
	if p.searchMode || p.contentSearchMode {
		searchBar = 1
	}
	h := p.height - 1 - searchBar - 2 - 2
	if h < 1 {
		return 1
	}
	return h
}

// ensureTreeCursorVisible adjusts scroll offset to keep cursor visible.
func (p *Plugin) ensureTreeCursorVisible() {
	visibleHeight := p.visibleContentHeight()

	if p.treeCursor < p.treeScrollOff {
		p.treeScrollOff = p.treeCursor
	} else if p.treeCursor >= p.treeScrollOff+visibleHeight {
		p.treeScrollOff = p.treeCursor - visibleHeight + 1
	}
}

// loadPreviewForCursor loads the preview for the file at the current tree cursor.
func (p *Plugin) loadPreviewForCursor() tea.Cmd {
	node := p.tree.GetNode(p.treeCursor)
	if node == nil || node.IsDir {
		return nil
	}
	p.previewFile = node.Path
	p.previewScroll = 0
	p.previewLines = nil
	p.previewError = nil
	p.isBinary = false
	p.isTruncated = false
	return LoadPreview(p.ctx.WorkDir, node.Path)
}

// handleSearchKey handles key input during search mode.
func (p *Plugin) handleSearchKey(msg tea.KeyMsg) (plugin.Plugin, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		p.searchMode = false
		p.searchQuery = ""

	case "enter":
		// Jump to selected match and exit search
		if len(p.searchMatches) > 0 {
			p.jumpToSearchMatch()
		}
		p.searchMode = false

	case "backspace":
		if len(p.searchQuery) > 0 {
			p.searchQuery = p.searchQuery[:len(p.searchQuery)-1]
			p.updateSearchMatches()
		}

	case "up", "ctrl+p":
		if p.searchCursor > 0 {
			p.searchCursor--
		}

	case "down", "ctrl+n":
		if p.searchCursor < len(p.searchMatches)-1 {
			p.searchCursor++
		}

	default:
		// Append printable characters to query
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			p.searchQuery += key
			p.updateSearchMatches()
		}
	}

	return p, nil
}

// updateSearchMatches finds all nodes matching the search query.
func (p *Plugin) updateSearchMatches() {
	p.searchMatches = nil
	if p.searchQuery == "" {
		return
	}

	query := strings.ToLower(p.searchQuery)

	// Walk entire tree (not just visible nodes)
	p.walkTree(p.tree.Root, func(node *FileNode) {
		name := strings.ToLower(node.Name)
		if strings.Contains(name, query) {
			p.searchMatches = append(p.searchMatches, node)
		}
	})

	// Limit matches to prevent UI clutter
	if len(p.searchMatches) > 20 {
		p.searchMatches = p.searchMatches[:20]
	}

	p.searchCursor = 0
}

// walkTree recursively visits all nodes in the tree.
func (p *Plugin) walkTree(node *FileNode, fn func(*FileNode)) {
	if node == nil {
		return
	}
	for _, child := range node.Children {
		fn(child)
		if child.IsDir {
			// Load children if not already loaded
			if len(child.Children) == 0 {
				_ = p.tree.loadChildren(child)
			}
			p.walkTree(child, fn)
		}
	}
}

// jumpToSearchMatch navigates to the currently selected search match.
func (p *Plugin) jumpToSearchMatch() {
	if len(p.searchMatches) == 0 || p.searchCursor >= len(p.searchMatches) {
		return
	}

	match := p.searchMatches[p.searchCursor]

	// Expand all parent directories to make the match visible
	p.expandParents(match)

	// Reflatten the tree after expanding
	p.tree.Flatten()

	// Find the match in the flat list
	for i, node := range p.tree.FlatList {
		if node == match {
			p.treeCursor = i
			p.ensureTreeCursorVisible()
			break
		}
	}
}

// expandParents expands all ancestor directories of a node.
func (p *Plugin) expandParents(node *FileNode) {
	if node == nil || node.Parent == nil || node.Parent == p.tree.Root {
		return
	}

	// Recursively expand parents first
	p.expandParents(node.Parent)

	// Then expand this node's parent
	if node.Parent.IsDir && !node.Parent.IsExpanded {
		node.Parent.IsExpanded = true
	}
}

// View renders the plugin.
func (p *Plugin) View(width, height int) string {
	p.width = width
	p.height = height
	content := p.renderView()
	// Constrain output to allocated height to prevent header scrolling off-screen.
	// MaxHeight truncates content that exceeds the allocated space.
	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(content)
}

// IsFocused returns whether the plugin is focused.
func (p *Plugin) IsFocused() bool { return p.focused }

// SetFocused sets the focus state.
func (p *Plugin) SetFocused(f bool) { p.focused = f }

// Commands returns the available commands.
func (p *Plugin) Commands() []plugin.Command {
	return []plugin.Command{
		// Tree pane commands
		{ID: "quick-open", Name: "Open", Context: "file-browser-tree"},
		{ID: "search", Name: "Search", Context: "file-browser-tree"},
		{ID: "refresh", Name: "Refresh", Context: "file-browser-tree"},
		// Preview pane commands
		{ID: "quick-open", Name: "Open", Context: "file-browser-preview"},
		{ID: "search-content", Name: "Search", Context: "file-browser-preview"},
		{ID: "back", Name: "Back", Context: "file-browser-preview"},
		// Tree search commands
		{ID: "confirm", Name: "Go", Context: "file-browser-search"},
		{ID: "cancel", Name: "Cancel", Context: "file-browser-search"},
		// Content search commands
		{ID: "confirm", Name: "Go", Context: "file-browser-content-search"},
		{ID: "cancel", Name: "Cancel", Context: "file-browser-content-search"},
		// Quick open commands
		{ID: "select", Name: "Open", Context: "file-browser-quick-open"},
		{ID: "cancel", Name: "Cancel", Context: "file-browser-quick-open"},
	}
}

// FocusContext returns the current focus context.
func (p *Plugin) FocusContext() string {
	if p.quickOpenMode {
		return "file-browser-quick-open"
	}
	if p.contentSearchMode {
		return "file-browser-content-search"
	}
	if p.searchMode {
		return "file-browser-search"
	}
	if p.activePane == PanePreview {
		return "file-browser-preview"
	}
	return "file-browser-tree"
}
