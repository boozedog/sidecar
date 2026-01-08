package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marcus/sidecar/internal/adapter"
)

const (
	adapterID   = "codex"
	adapterName = "Codex"
)

// Adapter implements the adapter.Adapter interface for Codex CLI sessions.
type Adapter struct {
	sessionsDir  string
	sessionIndex map[string]string
}

// New creates a new Codex adapter.
func New() *Adapter {
	home, _ := os.UserHomeDir()
	return &Adapter{
		sessionsDir:  filepath.Join(home, ".codex", "sessions"),
		sessionIndex: make(map[string]string),
	}
}

// ID returns the adapter identifier.
func (a *Adapter) ID() string { return adapterID }

// Name returns the human-readable adapter name.
func (a *Adapter) Name() string { return adapterName }

// Icon returns the adapter icon for badge display.
func (a *Adapter) Icon() string { return "▶" }

// Detect checks if Codex sessions exist for the given project.
func (a *Adapter) Detect(projectRoot string) (bool, error) {
	files, err := a.sessionFiles()
	if err != nil {
		return false, err
	}
	for _, path := range files {
		meta, err := a.parseSessionMetadata(path)
		if err != nil {
			continue
		}
		if cwdMatchesProject(projectRoot, meta.CWD) {
			return true, nil
		}
	}
	return false, nil
}

// Capabilities returns the supported features.
func (a *Adapter) Capabilities() adapter.CapabilitySet {
	return adapter.CapabilitySet{
		adapter.CapSessions: true,
		adapter.CapMessages: true,
		adapter.CapUsage:    true,
		adapter.CapWatch:    true,
	}
}

// Sessions returns all sessions for the given project, sorted by update time.
func (a *Adapter) Sessions(projectRoot string) ([]adapter.Session, error) {
	files, err := a.sessionFiles()
	if err != nil {
		return nil, err
	}

	sessions := make([]adapter.Session, 0, len(files))
	a.sessionIndex = make(map[string]string)
	for _, path := range files {
		meta, err := a.parseSessionMetadata(path)
		if err != nil {
			continue
		}
		if !cwdMatchesProject(projectRoot, meta.CWD) {
			continue
		}

		// Use first user message as name, fallback to short ID
		name := ""
		if meta.FirstUserMessage != "" {
			name = truncateTitle(meta.FirstUserMessage, 50)
		}
		if name == "" {
			name = shortID(meta.SessionID)
		}
		sessions = append(sessions, adapter.Session{
			ID:           meta.SessionID,
			Name:         name,
			AdapterID:    adapterID,
			AdapterName:  adapterName,
			AdapterIcon:  a.Icon(),
			CreatedAt:    meta.FirstMsg,
			UpdatedAt:    meta.LastMsg,
			Duration:     meta.LastMsg.Sub(meta.FirstMsg),
			IsActive:     time.Since(meta.LastMsg) < 5*time.Minute,
			TotalTokens:  meta.TotalTokens,
			MessageCount: meta.MsgCount,
		})

		a.sessionIndex[meta.SessionID] = path
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Messages returns all messages for the given session.
func (a *Adapter) Messages(sessionID string) ([]adapter.Message, error) {
	path := a.sessionFilePath(sessionID)
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var messages []adapter.Message
	var pendingTools []adapter.ToolUse
	toolIndex := make(map[string]int)
	var pendingThinking []adapter.ThinkingBlock
	var pendingUsage *adapter.TokenUsage
	var currentModel string
	var lastTimestamp time.Time

	flushPending := func(ts time.Time) {
		if len(pendingTools) == 0 && len(pendingThinking) == 0 {
			return
		}
		msg := adapter.Message{
			ID:             "synthetic-" + shortID(sessionID) + "-" + fmt.Sprintf("%d", len(messages)),
			Role:           "assistant",
			Content:        "tool calls",
			Timestamp:      ts,
			Model:          currentModel,
			ToolUses:       append([]adapter.ToolUse(nil), pendingTools...),
			ThinkingBlocks: append([]adapter.ThinkingBlock(nil), pendingThinking...),
		}
		if pendingUsage != nil {
			msg.TokenUsage = *pendingUsage
			pendingUsage = nil
		}
		messages = append(messages, msg)
		pendingTools = nil
		pendingThinking = nil
		toolIndex = make(map[string]int)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var record RawRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if !record.Timestamp.IsZero() {
			lastTimestamp = record.Timestamp
		}

		switch record.Type {
		case "turn_context":
			var payload TurnContextPayload
			if err := json.Unmarshal(record.Payload, &payload); err == nil && payload.Model != "" {
				currentModel = payload.Model
			}

		case "response_item":
			var base ResponseItemBase
			if err := json.Unmarshal(record.Payload, &base); err != nil {
				continue
			}
			switch base.Type {
			case "message":
				var msg ResponseMessagePayload
				if err := json.Unmarshal(record.Payload, &msg); err != nil {
					continue
				}
				if msg.Role != "user" && msg.Role != "assistant" {
					continue
				}
				if msg.Role == "user" {
					flushPending(record.Timestamp)
				}

				content := contentFromBlocks(msg.Content)
				message := adapter.Message{
					ID:        fmt.Sprintf("%s-%d", sessionID, len(messages)),
					Role:      msg.Role,
					Content:   content,
					Timestamp: record.Timestamp,
					Model:     currentModel,
				}
				if msg.Role == "assistant" {
					message.ToolUses = append(message.ToolUses, pendingTools...)
					message.ThinkingBlocks = append(message.ThinkingBlocks, pendingThinking...)
					pendingTools = nil
					pendingThinking = nil
					toolIndex = make(map[string]int)
					if pendingUsage != nil {
						message.TokenUsage = *pendingUsage
						pendingUsage = nil
					}
				}
				messages = append(messages, message)

			case "function_call", "custom_tool_call":
				var call ResponseToolCallPayload
				if err := json.Unmarshal(record.Payload, &call); err != nil {
					continue
				}
				input := toolInputString(call.Arguments, call.Input)
				tool := adapter.ToolUse{
					ID:    call.CallID,
					Name:  call.Name,
					Input: input,
				}
				toolIndex[call.CallID] = len(pendingTools)
				pendingTools = append(pendingTools, tool)

			case "function_call_output", "custom_tool_call_output":
				var output ResponseToolOutputPayload
				if err := json.Unmarshal(record.Payload, &output); err != nil {
					continue
				}
				out := toolOutputString(output.Output)
				if idx, ok := toolIndex[output.CallID]; ok && idx < len(pendingTools) {
					pendingTools[idx].Output = out
				} else {
					toolIndex[output.CallID] = len(pendingTools)
					pendingTools = append(pendingTools, adapter.ToolUse{
						ID:     output.CallID,
						Output: out,
					})
				}

			case "reasoning":
				var reason ResponseReasoningPayload
				if err := json.Unmarshal(record.Payload, &reason); err != nil {
					continue
				}
				for _, summary := range reason.Summary {
					if strings.TrimSpace(summary.Text) == "" {
						continue
					}
					pendingThinking = append(pendingThinking, adapter.ThinkingBlock{
						Content:    summary.Text,
						TokenCount: len(summary.Text) / 4,
					})
				}
			}

		case "event_msg":
			var event EventMsgPayload
			if err := json.Unmarshal(record.Payload, &event); err != nil {
				continue
			}
			switch event.Type {
			case "agent_reasoning":
				if strings.TrimSpace(event.Text) != "" {
					pendingThinking = append(pendingThinking, adapter.ThinkingBlock{
						Content:    event.Text,
						TokenCount: len(event.Text) / 4,
					})
				}
			case "token_count":
				if event.Info == nil || event.Info.LastTokenUsage == nil {
					continue
				}
				pendingUsage = convertUsage(event.Info.LastTokenUsage)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	flushPending(lastTimestamp)

	return messages, nil
}

// Usage returns aggregate usage stats for the given session.
func (a *Adapter) Usage(sessionID string) (*adapter.UsageStats, error) {
	messages, err := a.Messages(sessionID)
	if err != nil {
		return nil, err
	}

	stats := &adapter.UsageStats{}
	for _, msg := range messages {
		stats.TotalInputTokens += msg.InputTokens
		stats.TotalOutputTokens += msg.OutputTokens
		stats.TotalCacheRead += msg.CacheRead
		stats.TotalCacheWrite += msg.CacheWrite
		stats.MessageCount++
	}

	if stats.TotalInputTokens == 0 && stats.TotalOutputTokens == 0 && stats.TotalCacheRead == 0 {
		path := a.sessionFilePath(sessionID)
		if path != "" {
			if usage := a.totalUsageFromFile(path); usage != nil {
				stats.TotalInputTokens = usage.InputTokens
				stats.TotalOutputTokens = usage.OutputTokens + usage.ReasoningOutputTokens
				stats.TotalCacheRead = usage.CachedInputTokens
			}
		}
	}

	return stats, nil
}

// Watch returns a channel that emits events when session data changes.
func (a *Adapter) Watch(projectRoot string) (<-chan adapter.Event, error) {
	return NewWatcher(a.sessionsDir)
}

func (a *Adapter) sessionFiles() ([]string, error) {
	if _, err := os.Stat(a.sessionsDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	err := filepath.WalkDir(a.sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (a *Adapter) parseSessionMetadata(path string) (*SessionMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	meta := &SessionMetadata{Path: path}
	var sessionTimestamp time.Time
	var lastRecord time.Time
	var totalTokens int

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		var record RawRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}

		if !record.Timestamp.IsZero() {
			lastRecord = record.Timestamp
		}

		switch record.Type {
		case "session_meta":
			var payload SessionMetaPayload
			if err := json.Unmarshal(record.Payload, &payload); err != nil {
				continue
			}
			if meta.SessionID == "" {
				meta.SessionID = payload.ID
			}
			if meta.CWD == "" {
				meta.CWD = payload.CWD
			}
			if sessionTimestamp.IsZero() && !payload.Timestamp.IsZero() {
				sessionTimestamp = payload.Timestamp
			}

		case "response_item":
			var base ResponseItemBase
			if err := json.Unmarshal(record.Payload, &base); err != nil {
				continue
			}
			if base.Type != "message" {
				continue
			}
			var msg ResponseMessagePayload
			if err := json.Unmarshal(record.Payload, &msg); err != nil {
				continue
			}
			if msg.Role != "user" && msg.Role != "assistant" {
				continue
			}
			if meta.FirstMsg.IsZero() {
				meta.FirstMsg = record.Timestamp
			}
			// Extract first user message content for title
			if meta.FirstUserMessage == "" && msg.Role == "user" {
				if content := contentFromBlocks(msg.Content); content != "" {
					meta.FirstUserMessage = content
				}
			}
			meta.LastMsg = record.Timestamp
			meta.MsgCount++

		case "event_msg":
			var event EventMsgPayload
			if err := json.Unmarshal(record.Payload, &event); err != nil {
				continue
			}
			if event.Type != "token_count" || event.Info == nil {
				continue
			}
			usage := event.Info.TotalTokenUsage
			if usage == nil {
				usage = event.Info.LastTokenUsage
			}
			if usage != nil {
				totalTokens = usage.TotalTokens
				if totalTokens == 0 {
					totalTokens = usage.InputTokens + usage.OutputTokens + usage.ReasoningOutputTokens
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if meta.SessionID == "" {
		meta.SessionID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	}

	if meta.FirstMsg.IsZero() {
		meta.FirstMsg = sessionTimestamp
	}
	if meta.LastMsg.IsZero() {
		meta.LastMsg = sessionTimestamp
	}
	if meta.FirstMsg.IsZero() && !lastRecord.IsZero() {
		meta.FirstMsg = lastRecord
		meta.LastMsg = lastRecord
	}
	if meta.FirstMsg.IsZero() {
		if info, err := os.Stat(path); err == nil {
			meta.FirstMsg = info.ModTime()
			meta.LastMsg = info.ModTime()
		} else {
			meta.FirstMsg = time.Now()
			meta.LastMsg = meta.FirstMsg
		}
	}
	if meta.LastMsg.IsZero() {
		meta.LastMsg = meta.FirstMsg
	}

	meta.TotalTokens = totalTokens

	return meta, nil
}

func (a *Adapter) sessionFilePath(sessionID string) string {
	if path, ok := a.sessionIndex[sessionID]; ok && path != "" {
		return path
	}

	files, err := a.sessionFiles()
	if err != nil {
		return ""
	}
	for _, path := range files {
		if strings.Contains(filepath.Base(path), sessionID) {
			return path
		}
	}

	for _, path := range files {
		meta, err := a.parseSessionMetadata(path)
		if err != nil {
			continue
		}
		if meta.SessionID == sessionID {
			return path
		}
	}

	return ""
}

func (a *Adapter) totalUsageFromFile(path string) *TokenUsage {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var total *TokenUsage
	for scanner.Scan() {
		var record RawRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.Type != "event_msg" {
			continue
		}
		var event EventMsgPayload
		if err := json.Unmarshal(record.Payload, &event); err != nil {
			continue
		}
		if event.Type != "token_count" || event.Info == nil || event.Info.TotalTokenUsage == nil {
			continue
		}
		total = event.Info.TotalTokenUsage
	}

	if err := scanner.Err(); err != nil {
		return total
	}

	return total
}

func contentFromBlocks(blocks []ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, block := range blocks {
		if block.Text == "" {
			continue
		}
		parts = append(parts, block.Text)
	}
	return strings.Join(parts, "\n")
}

func toolInputString(arguments, input json.RawMessage) string {
	if len(arguments) > 0 && string(arguments) != "null" {
		return rawToString(arguments)
	}
	if len(input) > 0 && string(input) != "null" {
		return rawToString(input)
	}
	return ""
}

func toolOutputString(output json.RawMessage) string {
	if len(output) == 0 || string(output) == "null" {
		return ""
	}
	return rawToString(output)
}

func rawToString(raw json.RawMessage) string {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return string(raw)
}

func convertUsage(usage *TokenUsage) *adapter.TokenUsage {
	if usage == nil {
		return nil
	}
	return &adapter.TokenUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens + usage.ReasoningOutputTokens,
		CacheRead:    usage.CachedInputTokens,
	}
}

func cwdMatchesProject(projectRoot, cwd string) bool {
	if projectRoot == "" || cwd == "" {
		return false
	}
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return false
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(projectAbs); err == nil {
		projectAbs = resolved
	}
	if resolved, err := filepath.EvalSymlinks(cwdAbs); err == nil {
		cwdAbs = resolved
	}
	projectAbs = filepath.Clean(projectAbs)
	cwdAbs = filepath.Clean(cwdAbs)

	rel, err := filepath.Rel(projectAbs, cwdAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

func shortID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// truncateTitle truncates text to maxLen, adding "..." if truncated.
// It also replaces newlines with spaces for display.
func truncateTitle(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
