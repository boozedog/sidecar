package cursor

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/marcus/sidecar/internal/adapter"
)

const (
	adapterID   = "cursor-cli"
	adapterName = "Cursor CLI"
)

// Adapter implements the adapter.Adapter interface for Cursor CLI sessions.
type Adapter struct {
	chatsDir string
}

// New creates a new Cursor CLI adapter.
func New() *Adapter {
	home, _ := os.UserHomeDir()
	return &Adapter{
		chatsDir: filepath.Join(home, ".cursor", "chats"),
	}
}

// ID returns the adapter identifier.
func (a *Adapter) ID() string { return adapterID }

// Name returns the human-readable adapter name.
func (a *Adapter) Name() string { return adapterName }

// Icon returns the adapter icon for badge display.
func (a *Adapter) Icon() string { return "▌" }

// Capabilities returns the supported features.
func (a *Adapter) Capabilities() adapter.CapabilitySet {
	return adapter.CapabilitySet{
		adapter.CapSessions: true,
		adapter.CapMessages: true,
		adapter.CapUsage:    false, // Token usage not available in cursor format
		adapter.CapWatch:    true,
	}
}

// Detect checks if Cursor CLI sessions exist for the given project.
func (a *Adapter) Detect(projectRoot string) (bool, error) {
	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return false, err
	}

	workspaceDir := a.workspacePath(absPath)
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	// Check if any session directories exist with store.db
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dbPath := filepath.Join(workspaceDir, e.Name(), "store.db")
		if _, err := os.Stat(dbPath); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// Sessions returns all sessions for the given project, sorted by update time.
func (a *Adapter) Sessions(projectRoot string) ([]adapter.Session, error) {
	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}

	workspaceDir := a.workspacePath(absPath)
	entries, err := os.ReadDir(workspaceDir)
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

		dbPath := filepath.Join(workspaceDir, e.Name(), "store.db")
		meta, err := a.readSessionMeta(dbPath)
		if err != nil {
			continue
		}

		// Get file modification time as UpdatedAt
		info, err := os.Stat(dbPath)
		updatedAt := time.Now()
		if err == nil {
			updatedAt = info.ModTime()
		}

		// Count messages by traversing blobs
		msgCount := a.countMessages(dbPath)

		sessions = append(sessions, adapter.Session{
			ID:           meta.AgentID,
			Name:         meta.Name,
			Slug:         shortID(meta.AgentID),
			AdapterID:    adapterID,
			AdapterName:  adapterName,
			CreatedAt:    meta.CreatedTime(),
			UpdatedAt:    updatedAt,
			Duration:     updatedAt.Sub(meta.CreatedTime()),
			IsActive:     time.Since(updatedAt) < 5*time.Minute,
			TotalTokens:  0, // Not tracked in cursor format
			EstCost:      0,
			IsSubAgent:   false,
			MessageCount: msgCount,
		})
	}

	// Sort by UpdatedAt descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Messages returns all messages for the given session.
func (a *Adapter) Messages(sessionID string) ([]adapter.Message, error) {
	dbPath := a.findSessionDB(sessionID)
	if dbPath == "" {
		return nil, nil
	}

	return a.parseMessages(dbPath)
}

// Usage returns aggregate usage stats for the given session.
// Cursor CLI doesn't track detailed token usage, so we return estimates.
func (a *Adapter) Usage(sessionID string) (*adapter.UsageStats, error) {
	messages, err := a.Messages(sessionID)
	if err != nil {
		return nil, err
	}

	stats := &adapter.UsageStats{
		MessageCount: len(messages),
	}

	// Estimate tokens from content length
	for _, m := range messages {
		chars := len(m.Content)
		stats.TotalInputTokens += chars / 4  // rough estimate
		stats.TotalOutputTokens += chars / 4 // rough estimate
	}

	return stats, nil
}

// Watch returns a channel that emits events when session data changes.
func (a *Adapter) Watch(projectRoot string) (<-chan adapter.Event, error) {
	absPath, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	return NewWatcher(a.workspacePath(absPath))
}

// workspacePath returns the path to the workspace directory in ~/.cursor/chats.
// The workspace hash is the MD5 hash of the absolute project path.
func (a *Adapter) workspacePath(projectRoot string) string {
	hash := md5.Sum([]byte(projectRoot))
	return filepath.Join(a.chatsDir, hex.EncodeToString(hash[:]))
}

// readSessionMeta reads the session metadata from store.db.
func (a *Adapter) readSessionMeta(dbPath string) (*SessionMeta, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var hexValue string
	err = db.QueryRow("SELECT value FROM meta WHERE key = '0'").Scan(&hexValue)
	if err != nil {
		return nil, err
	}

	// Decode hex-encoded JSON
	jsonBytes, err := hex.DecodeString(hexValue)
	if err != nil {
		return nil, err
	}

	var meta SessionMeta
	if err := json.Unmarshal(jsonBytes, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// countMessages counts user/assistant messages in a session by traversing blobs.
func (a *Adapter) countMessages(dbPath string) int {
	messages, _ := a.parseMessages(dbPath)
	return len(messages)
}

// findSessionDB finds the store.db path for a given session ID.
func (a *Adapter) findSessionDB(sessionID string) string {
	entries, err := os.ReadDir(a.chatsDir)
	if err != nil {
		return ""
	}

	for _, wsDir := range entries {
		if !wsDir.IsDir() {
			continue
		}
		dbPath := filepath.Join(a.chatsDir, wsDir.Name(), sessionID, "store.db")
		if _, err := os.Stat(dbPath); err == nil {
			return dbPath
		}
	}
	return ""
}

// parseMessages parses all messages from a session's store.db.
func (a *Adapter) parseMessages(dbPath string) ([]adapter.Message, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Read session metadata to get root blob ID
	meta, err := a.readSessionMeta(dbPath)
	if err != nil {
		return nil, err
	}

	// Read all blobs into a map
	blobs := make(map[string][]byte)
	rows, err := db.Query("SELECT id, data FROM blobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		blobs[id] = data
	}

	// Traverse from root blob to collect messages
	var messages []adapter.Message
	a.collectMessages(blobs, meta.LatestRootBlobID, &messages)

	return messages, nil
}

// collectMessages recursively collects messages from a blob tree.
func (a *Adapter) collectMessages(blobs map[string][]byte, blobID string, messages *[]adapter.Message) {
	data, ok := blobs[blobID]
	if !ok || len(data) == 0 {
		return
	}

	// Check if this blob is JSON (starts with '{')
	if data[0] == '{' {
		msg, err := a.parseMessageBlob(data)
		if err == nil && (msg.Role == "user" || msg.Role == "assistant") {
			*messages = append(*messages, msg)
		}
		return
	}

	// Otherwise, it's a linking blob with child references
	// Format: 0x0A 0x20 [32 bytes hash] repeated, optionally followed by JSON
	offset := 0
	for offset+34 <= len(data) {
		if data[offset] != 0x0A || data[offset+1] != 0x20 {
			break
		}
		childID := hex.EncodeToString(data[offset+2 : offset+34])
		a.collectMessages(blobs, childID, messages)
		offset += 34
	}

	// Check if there's embedded JSON after the references
	if offset < len(data) {
		// Skip any non-JSON prefix bytes (field tags)
		jsonStart := offset
		for jsonStart < len(data) && data[jsonStart] != '{' {
			jsonStart++
		}
		if jsonStart < len(data) {
			msg, err := a.parseMessageBlob(data[jsonStart:])
			if err == nil && (msg.Role == "user" || msg.Role == "assistant") {
				*messages = append(*messages, msg)
			}
		}
	}
}

// parseMessageBlob parses a single message blob into an adapter.Message.
func (a *Adapter) parseMessageBlob(data []byte) (adapter.Message, error) {
	var blob MessageBlob
	if err := json.Unmarshal(data, &blob); err != nil {
		return adapter.Message{}, err
	}

	msg := adapter.Message{
		ID:   blob.ID,
		Role: blob.Role,
	}

	// Parse content
	content, toolUses, thinkingBlocks := a.parseContent(blob.Content)
	msg.Content = content
	msg.ToolUses = toolUses
	msg.ThinkingBlocks = thinkingBlocks

	return msg, nil
}

// parseContent extracts text content, tool uses, and thinking blocks from the content field.
func (a *Adapter) parseContent(rawContent json.RawMessage) (string, []adapter.ToolUse, []adapter.ThinkingBlock) {
	if len(rawContent) == 0 {
		return "", nil, nil
	}

	// Try parsing as string first
	var strContent string
	if err := json.Unmarshal(rawContent, &strContent); err == nil {
		return strContent, nil, nil
	}

	// Parse as array of content blocks
	var blocks []ContentBlock
	if err := json.Unmarshal(rawContent, &blocks); err != nil {
		return "", nil, nil
	}

	var texts []string
	var toolUses []adapter.ToolUse
	var thinkingBlocks []adapter.ThinkingBlock
	toolResultCount := 0

	for _, block := range blocks {
		switch block.Type {
		case "text":
			texts = append(texts, block.Text)
		case "reasoning":
			thinkingBlocks = append(thinkingBlocks, adapter.ThinkingBlock{
				Content:    block.Text,
				TokenCount: len(block.Text) / 4, // rough estimate
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
			toolResultCount++
		}
	}

	// If we have tool results but no text, show a placeholder
	content := ""
	if len(texts) > 0 {
		content = texts[0]
		for _, t := range texts[1:] {
			content += "\n" + t
		}
	}
	if content == "" && toolResultCount > 0 {
		content = fmt.Sprintf("[%d tool result(s)]", toolResultCount)
	}

	return content, toolUses, thinkingBlocks
}

// shortID returns the first 8 characters of an ID, or the full ID if shorter.
func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}
