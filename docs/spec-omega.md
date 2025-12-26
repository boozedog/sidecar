# Agent Sidecar: Omega Specification

A standalone Go TUI that runs alongside AI coding agents, displaying real-time status via file/artifact watching. Compiled-in plugin architecture with git status, TD monitor, and Claude Code conversation browsing.

## Principles

1. **Agent-agnostic**: Works with any agent that leaves local artifacts
2. **Zero config required**: Sensible defaults, optional JSON config
3. **Silent degradation**: Missing integrations hide gracefully; optional diagnostics panel
4. **Single active panel**: One plugin visible at a time, tab to switch
5. **Reactive where possible**: fsnotify for immediate updates, polling as fallback

---

## Architecture

```
sidecar/
├── cmd/sidecar/main.go              # Entry point, plugin registration
├── internal/
│   ├── app/                         # Main TUI orchestration
│   │   ├── model.go                 # Root Bubble Tea model
│   │   ├── update.go                # Message routing, key dispatch
│   │   ├── view.go                  # Layout: header, plugin panel, footer
│   │   └── commands.go              # Shared tea.Cmd helpers
│   ├── plugin/                      # Plugin system
│   │   ├── plugin.go                # Plugin interface
│   │   ├── registry.go              # Registration, lifecycle
│   │   └── context.go               # PluginContext (shared resources)
│   ├── keymap/                      # Centralized key handling
│   │   ├── registry.go              # Command registry, context dispatch
│   │   ├── bindings.go              # Default bindings
│   │   └── config.go                # JSON config loading
│   ├── adapter/                     # Agent adapters
│   │   ├── adapter.go               # Adapter interface
│   │   ├── claudecode/              # Claude Code adapter
│   │   │   ├── adapter.go           # Session discovery, JSONL parsing
│   │   │   ├── types.go             # Message, ToolUse, etc.
│   │   │   └── watcher.go           # fsnotify on session files
│   │   └── detect.go                # Auto-detect available agents
│   ├── plugins/                     # Built-in plugins
│   │   ├── gitstatus/               # Git status tree + diff
│   │   │   ├── plugin.go
│   │   │   ├── view.go
│   │   │   ├── tree.go              # File tree data structure
│   │   │   ├── diff.go              # Diff modal
│   │   │   └── watcher.go           # fsnotify on .git/index
│   │   ├── tdmonitor/               # TD integration
│   │   │   ├── plugin.go
│   │   │   ├── view.go
│   │   │   ├── data.go              # DB queries (read-only)
│   │   │   └── types.go
│   │   └── conversations/           # Claude Code conversations
│   │       ├── plugin.go
│   │       ├── view.go
│   │       ├── session.go           # Session list
│   │       └── messages.go          # Message rendering
│   ├── styles/                      # Shared Lipgloss styles
│   │   └── styles.go                # Colors, panels, status indicators
│   └── config/                      # Configuration
│       ├── config.go                # Config struct, defaults
│       └── loader.go                # JSON loading from ~/.config/sidecar/
├── configs/
│   └── default.json                 # Default config (embedded)
└── go.mod
```

---

## Core Interfaces

### Plugin Interface

```go
package plugin

type Plugin interface {
    // Identity
    ID() string                              // "git-status", "td-monitor", etc.
    Name() string                            // Human-readable name
    Icon() string                            // Single char/emoji for tab bar

    // Lifecycle
    Init(ctx *Context) error                 // Called once at startup
    Start() tea.Cmd                          // Begin watching/polling
    Stop()                                   // Cleanup

    // Bubble Tea integration
    Update(msg tea.Msg) (Plugin, tea.Cmd)    // Handle messages
    View(width, height int) string           // Render content

    // Focus management
    IsFocused() bool
    SetFocused(bool)

    // Keybindings
    Commands() []Command                     // Plugin-specific commands
    FocusContext() string                    // Keymap context when focused
}

// Context provides shared resources to plugins
type Context struct {
    WorkDir      string                      // Current working directory
    ConfigDir    string                      // ~/.config/sidecar/
    Adapters     map[string]adapter.Adapter  // Available agent adapters
    EventBus     chan<- Event                // Cross-plugin communication
    Logger       *slog.Logger
}

// Command describes a bindable action
type Command struct {
    ID          string                       // "git:stage-file"
    Name        string                       // "Stage file"
    Handler     func() tea.Cmd
    Context     string                       // When active (plugin ID or "global")
}
```

### Adapter Interface

```go
package adapter

type Adapter interface {
    // Identity
    ID() string                              // "claude-code", "cursor", etc.
    Name() string

    // Detection
    Detect(workDir string) (bool, error)     // Is this agent active here?
    
    // Capabilities (what can this adapter provide?)
    Capabilities() CapabilitySet

    // Data access (return ErrNotSupported if capability missing)
    Sessions(project string) ([]Session, error)
    Messages(sessionID string) ([]Message, error)
    Usage(sessionID string) (*UsageStats, error)

    // Watching
    Watch(project string) (<-chan Event, error)
}

type CapabilitySet struct {
    Sessions     bool
    Messages     bool
    Usage        bool
    Tools        bool
    LiveWatch    bool
}

type Session struct {
    ID        string
    Project   string
    StartedAt time.Time
    UpdatedAt time.Time
    Model     string
    Messages  int
    Tokens    TokenUsage
}

type Message struct {
    ID        string
    Role      string    // "user", "assistant"
    Content   string
    Timestamp time.Time
    ToolUses  []ToolUse
    Tokens    TokenUsage
}
```

---

## Keymap System

### Command Registry

```go
package keymap

type Registry struct {
    commands map[string]Command              // ID -> Command
    bindings map[string][]Binding            // context -> bindings
    userOverrides map[string]string          // key -> command ID
}

type Binding struct {
    Key       string                         // "tab", "ctrl+s", "g g"
    Command   string                         // Command ID
    Context   string                         // "global", "git-status", etc.
}

func (r *Registry) Handle(key tea.KeyMsg, activeContext string) tea.Cmd {
    // 1. Check user overrides
    // 2. Check active context bindings
    // 3. Fall back to global bindings
}
```

### Default Bindings

| Key | Command | Context |
|-----|---------|---------|
| `q`, `ctrl+c` | quit | global |
| `tab` | next-plugin | global |
| `shift+tab` | prev-plugin | global |
| `1-9` | focus-plugin-n | global |
| `?` | toggle-help | global |
| `!` | toggle-diagnostics | global |
| `r` | refresh | global |
| `j`, `down` | cursor-down | global |
| `k`, `up` | cursor-up | global |
| `g g` | cursor-top | global |
| `G` | cursor-bottom | global |
| `enter` | select | global |
| `esc` | back/close | global |
| `s` | stage-file | git-status |
| `u` | unstage-file | git-status |
| `d` | show-diff | git-status |
| `D` | show-diff-staged | git-status |
| `v` | toggle-diff-mode | git-status |
| `a` | approve-issue | td-monitor |
| `x` | delete-issue | td-monitor |

---

## MVP Plugins

### 1. Git Status Plugin

**Features:**
- Tree view of changed files (staged/unstaged/untracked)
- Grouped by directory with expand/collapse
- Inline diff stats (+/-) per file
- Diff modal (full unified diff)
- Stage/unstage individual files or hunks
- Auto-refresh via fsnotify on `.git/index`

**Data Sources:**
- `git status --porcelain=v2 -z` (structured output)
- `git diff` / `git diff --cached`
- `git diff --stat`

**Diff Rendering (MVP):**
- Unified diff only: standard +/- format with context
- Syntax highlighting via `alecthomas/chroma` (optional, 300+ languages)

**Side-by-side diff (Phase 2):** Deferred to a standalone `termdiff` library project. No mature Go library exists for terminal side-by-side diffs—this is worth building properly as a reusable package. See Phase 2 in Implementation Phases for the plan.

**View:**
```
 Git Status                          [2 staged, 3 modified]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 Staged (2)
   M config.yaml                                    +3 -1
   A internal/new.go                               +45
 
 Modified (2)
 > M src/main.go                                  +12 -3
   M internal/foo.go                               +8 -2
 
 Untracked (1)
   ? README.md
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 s stage  u unstage  d diff  enter open  ? help
```

### 2. TD Monitor Plugin

**Features:**
- Focused issue display (current work)
- In-progress issues list
- Task list (ready/reviewable/blocked)
- Activity feed (recent logs, handoffs)
- Issue detail modal with markdown rendering
- Approve/review actions

**Data Source:**
- SQLite database at `.todos/issues.db` (read-only)
- Reuse query patterns from td monitor
- Poll interval: 2s (configurable)

**View:**
```
 TD Monitor                               session: abc-123
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 Focused: td-a1b2c3d4                              P1 task
 "Implement user authentication flow"
 
 In Progress (2)
   td-e5f6g7h8  Add OAuth provider         P2  in_progress
   td-i9j0k1l2  Fix session timeout        P1  in_progress
 
 Ready (3)
 > td-m3n4o5p6  Update API docs            P3  open
   td-q7r8s9t0  Refactor db layer          P2  open
   td-u1v2w3x4  Add rate limiting          P2  open
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 enter details  a approve  r review  x delete  ? help
```

### 3. Conversations Plugin (Claude Code Adapter)

**Features:**
- List recent sessions for current project
- View conversation messages
- Token usage per session
- Tool call summary
- Read-only (no modifications)

**Data Source:**
- `~/.claude/projects/{project-hash}/*.jsonl`
- fsnotify for live updates during active sessions

**View:**
```
 Claude Code Sessions                          3 sessions
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 > 2024-01-15 14:23  "Add auth flow"      127 msgs  45k tok
   2024-01-15 10:15  "Fix build errors"    23 msgs   8k tok
   2024-01-14 16:42  "Initial setup"       45 msgs  12k tok

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 enter view  t tokens  ? help
```

**Message View:**
```
 Session: 2024-01-15 14:23                    127 messages
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 [14:23:45] user
 Add user authentication with OAuth support
 
 [14:23:52] assistant                           1.2k tokens
 I'll help you implement OAuth authentication. Let me
 start by examining your current auth setup...
 
 [tool] Read: src/auth/handler.go
 [tool] Edit: src/auth/oauth.go (+45 lines)
 
 [14:24:15] user
 Also add refresh token support
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 esc back  j/k scroll  ? help
```

---

## Data Refresh Strategy

### Hybrid Approach

1. **fsnotify (reactive)** - Use where reliable:
   - `.git/index` - git status changes
   - Claude Code session files - new messages
   - Config files - hot reload

2. **Polling (fallback)** - Use where fsnotify unreliable or unavailable:
   - SQLite databases (WAL mode can miss fsnotify)
   - Cross-filesystem scenarios
   - Default: 2s interval, configurable per-plugin

3. **Debouncing** - Prevent UI churn:
   - Batch rapid changes (100ms window)
   - Skip refresh if previous still pending

```go
type Watcher struct {
    fsWatcher    *fsnotify.Watcher
    pollInterval time.Duration
    debounce     time.Duration
    onChange     func()
}

func (w *Watcher) Start() tea.Cmd {
    // Try fsnotify first
    if err := w.fsWatcher.Add(path); err == nil {
        return w.watchFS()
    }
    // Fall back to polling
    return w.poll()
}
```

---

## Silent Degradation

### Philosophy
- Missing integrations should not clutter the UI
- User can opt-in to see what's unavailable
- Plugins self-disable when dependencies missing

### Implementation

```go
// Plugin registration with availability check
func (r *Registry) Register(p Plugin) {
    if err := p.Init(r.ctx); err != nil {
        r.unavailable[p.ID()] = err.Error()
        return  // Don't add to active plugins
    }
    r.plugins = append(r.plugins, p)
}

// Diagnostics panel (toggle with !)
func (m Model) viewDiagnostics() string {
    var b strings.Builder
    b.WriteString("Diagnostics\n")
    b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
    
    for id, reason := range m.registry.unavailable {
        b.WriteString(fmt.Sprintf(" %s: %s\n", id, reason))
    }
    
    if len(m.registry.unavailable) == 0 {
        b.WriteString(" All integrations available\n")
    }
    return b.String()
}
```

---

## UI Layout

```
┌─────────────────────────────────────────────────────────┐
│ Agent Sidecar           [Git] [TD] [Claude]        14:23│  <- Header
├─────────────────────────────────────────────────────────┤
│                                                         │
│                                                         │
│                   Active Plugin Panel                   │  <- Main content
│                                                         │
│                                                         │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ tab switch  ? help  q quit                  refreshed 2s│  <- Footer
└─────────────────────────────────────────────────────────┘
```

### Header
- App title (left)
- Plugin tabs with active indicator (center)
- Clock/status (right)

### Footer
- Context-aware key hints (left)
- Last refresh timestamp (right)
- Toggleable with `ctrl+h`

---

## Configuration

### File Location
`~/.config/sidecar/config.json`

### Schema

```json
{
  "plugins": {
    "git-status": {
      "enabled": true,
      "refreshInterval": "1s"
    },
    "td-monitor": {
      "enabled": true,
      "refreshInterval": "2s",
      "dbPath": ".todos/issues.db"
    },
    "conversations": {
      "enabled": true,
      "claudeDataDir": "~/.claude"
    }
  },
  "keymap": {
    "overrides": {
      "ctrl+s": "git:stage-file",
      "ctrl+d": "git:show-diff"
    }
  },
  "ui": {
    "showFooter": true,
    "showClock": true,
    "theme": "default"
  }
}
```

---

## Implementation Phases

### Phase 1: Core Framework (Days 1-2)
1. Project setup: go.mod, directory structure
2. `internal/plugin/plugin.go` - Plugin interface
3. `internal/plugin/registry.go` - Registration, lifecycle
4. `internal/plugin/context.go` - PluginContext
5. `internal/keymap/registry.go` - Command registry
6. `internal/keymap/bindings.go` - Default bindings
7. `internal/styles/styles.go` - Color palette, panel styles

### Phase 2: Main TUI Shell (Days 2-3)
1. `internal/app/model.go` - Root model, plugin management
2. `internal/app/view.go` - Header, footer, plugin panel
3. `internal/app/update.go` - Key dispatch, message routing
4. `cmd/sidecar/main.go` - Entry point

### Phase 3: Git Status Plugin (Days 3-5)
1. `internal/plugins/gitstatus/plugin.go` - Plugin interface
2. `internal/plugins/gitstatus/tree.go` - File tree structure
3. `internal/plugins/gitstatus/view.go` - Rendering
4. `internal/plugins/gitstatus/watcher.go` - fsnotify
5. `internal/plugins/gitstatus/diff.go` - Diff modal (unified only for MVP)

### Phase 4: Claude Code Adapter (Days 5-6)
1. `internal/adapter/adapter.go` - Interface
2. `internal/adapter/claudecode/types.go` - JSONL types
3. `internal/adapter/claudecode/adapter.go` - Implementation
4. `internal/adapter/claudecode/watcher.go` - File watching

### Phase 5: Conversations Plugin (Days 6-7)
1. `internal/plugins/conversations/plugin.go` - Plugin interface
2. `internal/plugins/conversations/session.go` - Session list
3. `internal/plugins/conversations/messages.go` - Message view
4. `internal/plugins/conversations/view.go` - Rendering

### Phase 6: TD Monitor Plugin (Days 7-9)
1. `internal/plugins/tdmonitor/plugin.go` - Plugin interface
2. `internal/plugins/tdmonitor/data.go` - SQLite queries
3. `internal/plugins/tdmonitor/view.go` - Rendering
4. `internal/plugins/tdmonitor/types.go` - Data types

### Phase 7: Polish (Days 9-10)
1. `internal/config/` - JSON config loading
2. Help overlay
3. Diagnostics panel
4. Error handling refinement
5. Testing (see Testing Strategy below)

---

## Phase 2: termdiff Library

Side-by-side terminal diff rendering is non-trivial and deserves to be a standalone, reusable library. Build this as a separate project (`termdiff`) then integrate into sidecar.

**Scope:**
- Parse unified diff output into structured hunks
- Pair old/new lines within each hunk (handle adds, deletes, modifications)
- Render two columns respecting terminal width
- Word-level diff highlighting within changed lines
- Optional syntax highlighting via chroma integration
- Handle edge cases: long lines, binary files, empty hunks

**API sketch:**
```go
package termdiff

type Renderer struct {
    Width       int
    SyntaxTheme string  // chroma theme name, empty = no highlighting
    TabWidth    int
}

// Parse git diff output into structured representation
func Parse(unified string) (*Diff, error)

// Render as side-by-side, returns string ready for terminal
func (r *Renderer) RenderSideBySide(d *Diff) string

// Render as unified (passthrough with optional highlighting)
func (r *Renderer) RenderUnified(d *Diff) string
```

**Integration:** Once termdiff is stable, add `v` toggle to git-status plugin to switch between unified and side-by-side modes.

---

## Testing Strategy

Focus testing effort on boundaries and data parsing—the areas most likely to break.

**Unit tests (high priority):**
- Adapter parsing: Claude Code JSONL format, git porcelain output
- Diff parsing: unified diff to hunks
- Config loading: JSON parsing, defaults, validation

**Integration tests (medium priority):**
- Plugin lifecycle: Init/Start/Stop sequencing
- Keymap dispatch: context switching, override precedence
- File watching: debounce behavior (mock fsnotify)

**Manual testing (required before release):**
- 80x24 minimum terminal size
- Missing .git directory (silent degradation)
- Missing .todos directory (silent degradation)
- Missing Claude Code data (silent degradation)
- Rapid file changes (debounce works)

**Not tested (intentionally):**
- View rendering (too brittle, visual inspection sufficient)
- Lipgloss styling (framework responsibility)

---

## Patterns from TD to Adopt

| Pattern | Description |
|---------|-------------|
| **Flattened row lists** | Convert nested data to indexed arrays for cursor navigation |
| **Per-panel state maps** | `ScrollOffset[panel]`, `Cursor[panel]`, `SelectedID[panel]` |
| **Modal pattern** | `ModalOpen`, `ModalLoading`, separate key handler |
| **Typed messages** | Each async op returns specific message type |
| **Cursor preservation** | Save selected ID, restore position after refresh |
| **Hierarchical key handling** | Modal > Search > Main handler precedence |
| **Write lock pattern** | File-based lock for SQLite multi-process safety |

---

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea v1.2.4
    github.com/charmbracelet/lipgloss v1.0.0
    github.com/charmbracelet/glamour v0.8.0   // Markdown rendering
    github.com/alecthomas/chroma/v2 v2.14.0   // Syntax highlighting for diffs
    github.com/fsnotify/fsnotify v1.8.0
    github.com/mattn/go-sqlite3 v1.14.24
)
```

---

## Success Criteria

### MVP Complete When:
- [ ] Git status plugin shows changed files with staging actions
- [ ] Conversations plugin displays Claude Code sessions
- [ ] TD monitor plugin shows issues (if .todos/ exists)
- [ ] Tab switching between available plugins works
- [ ] Silent degradation: missing plugins don't crash or clutter
- [ ] Help overlay shows context-aware keybindings
- [ ] Config file customization works

### Quality Bar:
- Startup < 100ms
- Refresh latency < 50ms
- Zero panics on missing data
- Works in 80x24 terminal minimum
