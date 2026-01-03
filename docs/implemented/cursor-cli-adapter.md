# Cursor-CLI Adapter Implementation Plan

## Overview

This document describes the implementation plan for a cursor-cli adapter to support viewing conversations from the Cursor CLI (cursor-agent) in Sidecar's conversations plugin.

**Note:** This adapter is specifically for cursor-cli/cursor-agent, NOT the Cursor VSCode extension.

## Cursor-CLI Data Format

### Storage Location

Cursor-CLI stores session data at:
```
~/.cursor/chats/<workspace_hash>/<session_uuid>/store.db
```

Where:
- `<workspace_hash>` is an MD5 hash of the project path
- `<session_uuid>` is a UUID identifying the session

### Workspace Hash Calculation

The workspace hash is computed as:
```
MD5("/path/to/project")
```

The project folder in `~/.cursor/projects/` uses dashes instead of slashes:
- `/Users/marcus/code/sidecar` → `Users-marcus-code-sidecar`
- MD5 of `/Users/marcus/code/sidecar` = `e4e90b7bcae743c9a1a1a045a49e1b1d`

### Database Schema

The `store.db` SQLite database has two tables:

```sql
CREATE TABLE blobs (id TEXT PRIMARY KEY, data BLOB);
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT);
```

#### Meta Table

The meta table stores session metadata as hex-encoded JSON with key `"0"`:

```json
{
  "agentId": "93d7b323-695c-4b84-8b69-d48bb88ae780",
  "latestRootBlobId": "56182061ad78adbfd67c10d89d5bcaec1971e66ee77469d2e30fa48d92d64d83",
  "name": "New Agent",
  "mode": "auto-run",
  "createdAt": 1767396459642,
  "lastUsedModel": "claude-4.5-opus-high-thinking"
}
```

Fields:
- `agentId`: Session UUID (matches directory name)
- `latestRootBlobId`: SHA-256 hash pointing to latest message state
- `name`: Session display name
- `mode`: Execution mode (e.g., "auto-run")
- `createdAt`: Unix timestamp in milliseconds
- `lastUsedModel`: Model ID string

#### Blobs Table

The blobs table uses content-addressed storage with SHA-256 hashes as IDs. Each blob contains a binary-encoded structure (likely protobuf) with embedded JSON payloads.

### Blob Format

Blobs contain a binary wrapper around JSON message payloads. The format appears to be:
- Variable-length binary header with hash references
- Embedded JSON messages with structure:

```json
{
  "role": "user|assistant|system|tool",
  "content": "<string or array>",
  "providerOptions": {
    "cursor": {
      "requestId": "...",
      "modelName": "..."
    }
  }
}
```

**User messages:**
```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "<user_query>\n...\n</user_query>"
    }
  ],
  "providerOptions": {
    "cursor": {
      "requestId": "182d4d53-458c-47a5-8ed9-677469aaa5b5"
    }
  }
}
```

**Assistant messages:**
```json
{
  "id": "1",
  "role": "assistant",
  "content": [
    {
      "type": "reasoning",
      "text": "Let me...",
      "providerOptions": {
        "cursor": {
          "modelName": "claude-4.5-opus-high-thinking"
        }
      },
      "signature": "..."
    },
    {
      "type": "tool-call",
      "toolCallId": "toolu_...",
      "toolName": "Shell",
      "args": {"command": "...", "description": "..."},
      "providerOptions": {...}
    }
  ]
}
```

**Tool result messages:**
```json
{
  "role": "tool",
  "id": "toolu_...",
  "content": [
    {
      "type": "tool-result",
      "toolName": "Shell",
      "toolCallId": "toolu_...",
      "result": "Exit code: 0\n\nCommand output:\n..."
    }
  ]
}
```

### Content Block Types

| Type | Description |
|------|-------------|
| `text` | Plain text content |
| `reasoning` | Extended thinking/reasoning content |
| `tool-call` | Tool invocation with args |
| `tool-result` | Tool execution result |

### Model IDs

Known model IDs:
- `claude-4.5-opus-high-thinking`
- `gemini-3-flash`
- `gpt-5`
- `sonnet-4`
- `sonnet-4-thinking`

### Related Files

- `~/.cursor/cli-config.json` - CLI configuration
- `~/.cursor/prompt_history.json` - Recent prompts (array of strings)
- `~/.cursor/projects/<project>/repo.json` - Project metadata with UUID

## Implementation Plan

### File Layout

```
internal/adapter/cursorcli/
  adapter.go       # Main adapter implementation
  types.go         # Type definitions for SQLite/JSON parsing
  parser.go        # Blob parsing logic
  watcher.go       # fsnotify-based watcher
  register.go      # init() registration
  adapter_test.go  # Unit tests
  parser_test.go   # Parser tests
  testdata/        # Test fixtures
```

### Step 1: Define Types (types.go)

```go
package cursorcli

import (
    "encoding/json"
    "time"
)

// SessionMeta represents the meta table JSON structure.
type SessionMeta struct {
    AgentID         string `json:"agentId"`
    LatestRootBlobID string `json:"latestRootBlobId"`
    Name            string `json:"name"`
    Mode            string `json:"mode"`
    CreatedAt       int64  `json:"createdAt"` // Unix ms
    LastUsedModel   string `json:"lastUsedModel"`
}

// Message represents a parsed conversation message.
type Message struct {
    ID              string          `json:"id,omitempty"`
    Role            string          `json:"role"`
    Content         json.RawMessage `json:"content"`
    ProviderOptions *ProviderOpts   `json:"providerOptions,omitempty"`
}

// ProviderOpts contains cursor-specific options.
type ProviderOpts struct {
    Cursor *CursorOpts `json:"cursor,omitempty"`
}

// CursorOpts contains cursor-specific message metadata.
type CursorOpts struct {
    RequestID string `json:"requestId,omitempty"`
    ModelName string `json:"modelName,omitempty"`
}

// ContentBlock represents a content array element.
type ContentBlock struct {
    Type            string          `json:"type"`
    Text            string          `json:"text,omitempty"`
    ToolCallID      string          `json:"toolCallId,omitempty"`
    ToolName        string          `json:"toolName,omitempty"`
    Args            json.RawMessage `json:"args,omitempty"`
    Result          string          `json:"result,omitempty"`
    Signature       string          `json:"signature,omitempty"`
    ProviderOptions *ProviderOpts   `json:"providerOptions,omitempty"`
}

// SessionMetadata holds aggregated session data.
type SessionMetadata struct {
    SessionID    string
    Name         string
    WorkspaceHash string
    ProjectPath  string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Model        string
    MsgCount     int
    TotalTokens  int
}
```

### Step 2: Implement Blob Parser (parser.go)

The blob parser needs to:
1. Read binary blob data from SQLite
2. Extract embedded JSON payloads
3. Parse messages into structured types

```go
package cursorcli

import (
    "bytes"
    "encoding/json"
    "regexp"
)

var jsonPattern = regexp.MustCompile(`\{"(?:role|id)":[^}]+(?:\{[^}]*\}[^}]*)*\}`)

// ParseBlob extracts messages from a binary blob.
func ParseBlob(data []byte) ([]Message, error) {
    var messages []Message

    // Strategy: scan for JSON objects starting with {"role" or {"id"
    // and parse them incrementally
    for i := 0; i < len(data); i++ {
        if data[i] == '{' {
            // Try to find balanced JSON starting here
            if msg, end := extractJSON(data[i:]); msg != nil {
                messages = append(messages, *msg)
                i += end - 1
            }
        }
    }

    return messages, nil
}

// extractJSON attempts to parse a balanced JSON object.
func extractJSON(data []byte) (*Message, int) {
    depth := 0
    for i, c := range data {
        switch c {
        case '{':
            depth++
        case '}':
            depth--
            if depth == 0 {
                var msg Message
                if err := json.Unmarshal(data[:i+1], &msg); err == nil {
                    if msg.Role != "" {
                        return &msg, i + 1
                    }
                }
                return nil, i + 1
            }
        }
    }
    return nil, len(data)
}
```

### Step 3: Implement Adapter (adapter.go)

```go
package cursorcli

import (
    "crypto/md5"
    "database/sql"
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "sort"
    "time"

    "github.com/marcus/sidecar/internal/adapter"
    _ "github.com/mattn/go-sqlite3"
)

const (
    adapterID   = "cursor-cli"
    adapterName = "Cursor CLI"
)

type Adapter struct {
    chatsDir string
}

func New() *Adapter {
    home, _ := os.UserHomeDir()
    return &Adapter{
        chatsDir: filepath.Join(home, ".cursor", "chats"),
    }
}

func (a *Adapter) ID() string   { return adapterID }
func (a *Adapter) Name() string { return adapterName }

// Detect checks if cursor-cli sessions exist for the project.
func (a *Adapter) Detect(projectRoot string) (bool, error) {
    wsHash := workspaceHash(projectRoot)
    wsDir := filepath.Join(a.chatsDir, wsHash)

    entries, err := os.ReadDir(wsDir)
    if err != nil {
        if os.IsNotExist(err) {
            return false, nil
        }
        return false, err
    }

    for _, e := range entries {
        if e.IsDir() {
            dbPath := filepath.Join(wsDir, e.Name(), "store.db")
            if _, err := os.Stat(dbPath); err == nil {
                return true, nil
            }
        }
    }
    return false, nil
}

func (a *Adapter) Capabilities() adapter.CapabilitySet {
    return adapter.CapabilitySet{
        adapter.CapSessions: true,
        adapter.CapMessages: true,
        adapter.CapUsage:    false, // Token usage not readily available
        adapter.CapWatch:    true,
    }
}

// Sessions returns all sessions for the project.
func (a *Adapter) Sessions(projectRoot string) ([]adapter.Session, error) {
    wsHash := workspaceHash(projectRoot)
    wsDir := filepath.Join(a.chatsDir, wsHash)

    entries, err := os.ReadDir(wsDir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var sessions []adapter.Session
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }

        dbPath := filepath.Join(wsDir, e.Name(), "store.db")
        meta, err := a.readSessionMeta(dbPath)
        if err != nil {
            continue
        }

        // Get file modification time as UpdatedAt
        info, _ := os.Stat(dbPath)
        updatedAt := meta.CreatedAt
        if info != nil {
            updatedAt = info.ModTime()
        }

        sessions = append(sessions, adapter.Session{
            ID:          meta.SessionID,
            Name:        meta.Name,
            Slug:        shortID(meta.SessionID),
            AdapterID:   adapterID,
            AdapterName: adapterName,
            CreatedAt:   meta.CreatedAt,
            UpdatedAt:   updatedAt,
            Duration:    updatedAt.Sub(meta.CreatedAt),
            IsActive:    time.Since(updatedAt) < 5*time.Minute,
            TotalTokens: meta.TotalTokens,
            MessageCount: meta.MsgCount,
        })
    }

    sort.Slice(sessions, func(i, j int) bool {
        return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
    })

    return sessions, nil
}

// Messages returns all messages for a session.
func (a *Adapter) Messages(sessionID string) ([]adapter.Message, error) {
    dbPath := a.findSessionDB(sessionID)
    if dbPath == "" {
        return nil, nil
    }

    db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
    if err != nil {
        return nil, err
    }
    defer db.Close()

    rows, err := db.Query("SELECT data FROM blobs")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []adapter.Message
    for rows.Next() {
        var data []byte
        if err := rows.Scan(&data); err != nil {
            continue
        }

        parsed, err := ParseBlob(data)
        if err != nil {
            continue
        }

        for _, m := range parsed {
            msg := a.convertMessage(m)
            if msg.Role != "" {
                messages = append(messages, msg)
            }
        }
    }

    return messages, nil
}

func (a *Adapter) Usage(sessionID string) (*adapter.UsageStats, error) {
    // Token usage not readily available in cursor-cli format
    messages, err := a.Messages(sessionID)
    if err != nil {
        return nil, err
    }
    return &adapter.UsageStats{
        MessageCount: len(messages),
    }, nil
}

func (a *Adapter) Watch(projectRoot string) (<-chan adapter.Event, error) {
    wsHash := workspaceHash(projectRoot)
    wsDir := filepath.Join(a.chatsDir, wsHash)
    return NewWatcher(wsDir)
}

// workspaceHash computes MD5 hash of project path.
func workspaceHash(projectRoot string) string {
    absPath, err := filepath.Abs(projectRoot)
    if err != nil {
        absPath = projectRoot
    }
    hash := md5.Sum([]byte(absPath))
    return hex.EncodeToString(hash[:])
}

func (a *Adapter) readSessionMeta(dbPath string) (*SessionMetadata, error) {
    db, err := sql.Open("sqlite3", dbPath+"?mode=ro")
    if err != nil {
        return nil, err
    }
    defer db.Close()

    var hexValue string
    err = db.QueryRow("SELECT value FROM meta WHERE key='0'").Scan(&hexValue)
    if err != nil {
        return nil, err
    }

    jsonBytes, err := hex.DecodeString(hexValue)
    if err != nil {
        return nil, err
    }

    var sm SessionMeta
    if err := json.Unmarshal(jsonBytes, &sm); err != nil {
        return nil, err
    }

    // Count messages by scanning blobs
    msgCount := a.countMessages(db)

    return &SessionMetadata{
        SessionID:   sm.AgentID,
        Name:        sm.Name,
        CreatedAt:   time.UnixMilli(sm.CreatedAt),
        Model:       sm.LastUsedModel,
        MsgCount:    msgCount,
    }, nil
}

func (a *Adapter) countMessages(db *sql.DB) int {
    rows, err := db.Query("SELECT data FROM blobs")
    if err != nil {
        return 0
    }
    defer rows.Close()

    count := 0
    for rows.Next() {
        var data []byte
        if err := rows.Scan(&data); err != nil {
            continue
        }
        msgs, _ := ParseBlob(data)
        for _, m := range msgs {
            if m.Role == "user" || m.Role == "assistant" {
                count++
            }
        }
    }
    return count
}

func (a *Adapter) findSessionDB(sessionID string) string {
    entries, err := os.ReadDir(a.chatsDir)
    if err != nil {
        return ""
    }

    for _, wsEntry := range entries {
        if !wsEntry.IsDir() {
            continue
        }
        wsPath := filepath.Join(a.chatsDir, wsEntry.Name())
        sessions, err := os.ReadDir(wsPath)
        if err != nil {
            continue
        }
        for _, sEntry := range sessions {
            if sEntry.Name() == sessionID {
                return filepath.Join(wsPath, sEntry.Name(), "store.db")
            }
        }
    }
    return ""
}

func (a *Adapter) convertMessage(m Message) adapter.Message {
    msg := adapter.Message{
        ID:   m.ID,
        Role: m.Role,
    }

    // Extract model from provider options
    if m.ProviderOptions != nil && m.ProviderOptions.Cursor != nil {
        msg.Model = m.ProviderOptions.Cursor.ModelName
    }

    // Parse content
    content, toolUses, thinkingBlocks := a.parseContent(m.Content)
    msg.Content = content
    msg.ToolUses = toolUses
    msg.ThinkingBlocks = thinkingBlocks

    return msg
}

func (a *Adapter) parseContent(raw json.RawMessage) (string, []adapter.ToolUse, []adapter.ThinkingBlock) {
    if len(raw) == 0 {
        return "", nil, nil
    }

    // Try string first
    var strContent string
    if err := json.Unmarshal(raw, &strContent); err == nil {
        return strContent, nil, nil
    }

    // Parse as array
    var blocks []ContentBlock
    if err := json.Unmarshal(raw, &blocks); err != nil {
        return "", nil, nil
    }

    var texts []string
    var toolUses []adapter.ToolUse
    var thinkingBlocks []adapter.ThinkingBlock

    for _, block := range blocks {
        switch block.Type {
        case "text":
            texts = append(texts, block.Text)
        case "reasoning":
            thinkingBlocks = append(thinkingBlocks, adapter.ThinkingBlock{
                Content:    block.Text,
                TokenCount: len(block.Text) / 4,
            })
        case "tool-call":
            inputStr := ""
            if len(block.Args) > 0 {
                inputStr = string(block.Args)
            }
            toolUses = append(toolUses, adapter.ToolUse{
                ID:    block.ToolCallID,
                Name:  block.ToolName,
                Input: inputStr,
            })
        case "tool-result":
            // Tool results are handled separately
        }
    }

    return strings.Join(texts, "\n"), toolUses, thinkingBlocks
}

func shortID(id string) string {
    if len(id) >= 8 {
        return id[:8]
    }
    return id
}
```

### Step 4: Implement Watcher (watcher.go)

```go
package cursorcli

import (
    "path/filepath"
    "time"

    "github.com/fsnotify/fsnotify"
    "github.com/marcus/sidecar/internal/adapter"
)

func NewWatcher(workspaceDir string) (<-chan adapter.Event, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    // Add workspace directory
    if err := watcher.Add(workspaceDir); err != nil {
        watcher.Close()
        return nil, err
    }

    // Add session subdirectories (fsnotify is non-recursive)
    entries, _ := os.ReadDir(workspaceDir)
    for _, e := range entries {
        if e.IsDir() {
            sessionDir := filepath.Join(workspaceDir, e.Name())
            watcher.Add(sessionDir)
        }
    }

    events := make(chan adapter.Event, 32)

    go func() {
        defer watcher.Close()
        defer close(events)

        var debounceTimer *time.Timer
        var lastSessionID string
        debounceDelay := 200 * time.Millisecond

        for {
            select {
            case event, ok := <-watcher.Events:
                if !ok {
                    return
                }

                // Only watch store.db files
                if filepath.Base(event.Name) != "store.db" {
                    // Check if new session directory was created
                    if event.Op&fsnotify.Create != 0 {
                        info, err := os.Stat(event.Name)
                        if err == nil && info.IsDir() {
                            watcher.Add(event.Name)
                        }
                    }
                    continue
                }

                // Extract session ID from path
                sessionID := filepath.Base(filepath.Dir(event.Name))
                lastSessionID = sessionID

                if debounceTimer != nil {
                    debounceTimer.Stop()
                }
                debounceTimer = time.AfterFunc(debounceDelay, func() {
                    var eventType adapter.EventType
                    switch {
                    case event.Op&fsnotify.Create != 0:
                        eventType = adapter.EventSessionCreated
                    case event.Op&fsnotify.Write != 0:
                        eventType = adapter.EventMessageAdded
                    default:
                        eventType = adapter.EventSessionUpdated
                    }

                    select {
                    case events <- adapter.Event{
                        Type:      eventType,
                        SessionID: lastSessionID,
                    }:
                    default:
                    }
                })

            case _, ok := <-watcher.Errors:
                if !ok {
                    return
                }
            }
        }
    }()

    return events, nil
}
```

### Step 5: Register Adapter (register.go)

```go
package cursorcli

import "github.com/marcus/sidecar/internal/adapter"

func init() {
    adapter.RegisterFactory(func() adapter.Adapter {
        return New()
    })
}
```

### Step 6: Import in main.go

Add to `cmd/sidecar/main.go`:

```go
import (
    _ "github.com/marcus/sidecar/internal/adapter/cursorcli"
)
```

## Testing Checklist

- [ ] `Detect()` correctly matches projects by workspace hash
- [ ] `Sessions()` lists all sessions with correct metadata
- [ ] `Sessions()` sorts by UpdatedAt descending
- [ ] `Messages()` extracts user/assistant messages correctly
- [ ] `Messages()` handles reasoning/thinking blocks
- [ ] `Messages()` extracts tool calls and results
- [ ] `Watch()` emits events on store.db changes
- [ ] `Watch()` handles new session directory creation
- [ ] Blob parser handles various message formats
- [ ] Model name extraction works for different models

## UI Integration Notes

1. **Resume command**: Add cursor-cli support in `internal/plugins/conversations/view.go`:
   ```go
   case "cursor-cli":
       return fmt.Sprintf("cursor-agent --resume %s", sessionID)
   ```

2. **Model short names**: Extend `modelShortName()` for cursor models:
   ```go
   case strings.Contains(model, "gemini"):
       return "Gemini"
   case strings.Contains(model, "gpt"):
       return "GPT"
   ```

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver for reading store.db
- `github.com/fsnotify/fsnotify` - File system watching (already used)

## Open Questions

1. **Blob format**: The exact binary format needs more investigation. It may be:
   - Custom binary protocol
   - Protobuf encoding
   - MessagePack

   Current approach: Scan for JSON objects within binary data.

2. **Token usage**: cursor-cli doesn't appear to store token counts in an easily accessible format. May need to:
   - Estimate from content length
   - Mark `CapUsage` as false
   - Research additional metadata locations

3. **Message ordering**: Blobs use content-addressed storage with Merkle-tree structure. Need to determine proper message ordering (likely by blob reference chain from `latestRootBlobId`).

## References

- [Cursor CLI Blog Post](https://cursor.com/blog/cli)
- [Cursor CLI Docs](https://cursor.com/docs/cli/using)
- [Cursor History Docs](https://cursor.com/docs/agent/chat/history)
- [Cursor Forum: Chat History](https://forum.cursor.com/t/chat-history-folder/7653)
