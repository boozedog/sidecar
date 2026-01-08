package opencode

import (
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
	adapterID   = "opencode"
	adapterName = "OpenCode"
)

// Adapter implements the adapter.Adapter interface for OpenCode sessions.
type Adapter struct {
	storageDir   string
	projectIndex map[string]*Project // worktree path -> Project
	sessionIndex map[string]string   // sessionID -> project ID
}

// New creates a new OpenCode adapter.
func New() *Adapter {
	home, _ := os.UserHomeDir()
	return &Adapter{
		storageDir:   filepath.Join(home, ".local", "share", "opencode", "storage"),
		projectIndex: make(map[string]*Project),
		sessionIndex: make(map[string]string),
	}
}

// ID returns the adapter identifier.
func (a *Adapter) ID() string { return adapterID }

// Name returns the human-readable adapter name.
func (a *Adapter) Name() string { return adapterName }

// Icon returns the adapter icon for badge display.
func (a *Adapter) Icon() string { return "◇" }

// Detect checks if OpenCode sessions exist for the given project.
func (a *Adapter) Detect(projectRoot string) (bool, error) {
	projectID, err := a.findProjectID(projectRoot)
	if err != nil {
		return false, nil
	}
	if projectID == "" {
		return false, nil
	}

	// Check if there are any sessions for this project
	sessionDir := filepath.Join(a.storageDir, "session", projectID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
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
	projectID, err := a.findProjectID(projectRoot)
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, nil
	}

	sessionDir := filepath.Join(a.storageDir, "session", projectID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sessions := make([]adapter.Session, 0, len(entries))
	a.sessionIndex = make(map[string]string)

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(sessionDir, e.Name())
		meta, err := a.parseSessionFile(path, projectID)
		if err != nil {
			continue
		}

		// Store in index for Messages() lookup
		a.sessionIndex[meta.SessionID] = projectID

		// Determine name - use title, first user message, or short ID
		name := meta.Title
		if name == "" && meta.FirstUserMessage != "" {
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
			EstCost:      meta.EstCost,
			IsSubAgent:   meta.ParentID != "",
			MessageCount: meta.MsgCount,
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
	messageDir := filepath.Join(a.storageDir, "message", sessionID)
	entries, err := os.ReadDir(messageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []adapter.Message

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(messageDir, e.Name())
		msg, err := a.parseMessageFile(path)
		if err != nil {
			continue
		}

		// Skip non-user/assistant messages
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}

		// Load parts for this message
		content, toolUses, thinkingBlocks, fileRefs, patchFiles := a.loadParts(msg.ID)

		// Build content string
		var contentParts []string
		if content != "" {
			contentParts = append(contentParts, content)
		}
		if len(fileRefs) > 0 {
			contentParts = append(contentParts, fmt.Sprintf("[files: %s]", strings.Join(fileRefs, ", ")))
		}
		if len(patchFiles) > 0 {
			contentParts = append(contentParts, fmt.Sprintf("[edited: %d files]", len(patchFiles)))
		}

		// Get model from either ModelID field or Model.ModelID
		model := msg.ModelID
		if model == "" && msg.Model != nil {
			model = msg.Model.ModelID
		}

		adapterMsg := adapter.Message{
			ID:             msg.ID,
			Role:           msg.Role,
			Content:        strings.Join(contentParts, "\n"),
			Timestamp:      msg.Time.CreatedTime(),
			Model:          model,
			ToolUses:       toolUses,
			ThinkingBlocks: thinkingBlocks,
		}

		// Add token usage
		if msg.Tokens != nil {
			adapterMsg.TokenUsage = adapter.TokenUsage{
				InputTokens:  msg.Tokens.Input,
				OutputTokens: msg.Tokens.Output,
			}
			if msg.Tokens.Cache != nil {
				adapterMsg.CacheRead = msg.Tokens.Cache.Read
				adapterMsg.CacheWrite = msg.Tokens.Cache.Write
			}
		}

		messages = append(messages, adapterMsg)
	}

	// Sort by timestamp ascending
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

// Usage returns aggregate usage stats for the given session.
func (a *Adapter) Usage(sessionID string) (*adapter.UsageStats, error) {
	messages, err := a.Messages(sessionID)
	if err != nil {
		return nil, err
	}

	stats := &adapter.UsageStats{}
	for _, m := range messages {
		stats.TotalInputTokens += m.InputTokens
		stats.TotalOutputTokens += m.OutputTokens
		stats.TotalCacheRead += m.CacheRead
		stats.TotalCacheWrite += m.CacheWrite
		stats.MessageCount++
	}

	return stats, nil
}

// Watch returns a channel that emits events when session data changes.
func (a *Adapter) Watch(projectRoot string) (<-chan adapter.Event, error) {
	projectID, err := a.findProjectID(projectRoot)
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, fmt.Errorf("no OpenCode project found for %s", projectRoot)
	}

	sessionDir := filepath.Join(a.storageDir, "session", projectID)
	return NewWatcher(sessionDir)
}

// findProjectID finds the OpenCode project ID for the given project root.
func (a *Adapter) findProjectID(projectRoot string) (string, error) {
	// Normalize the project root path
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	absRoot = filepath.Clean(absRoot)

	// Check cache first
	if proj, ok := a.projectIndex[absRoot]; ok {
		return proj.ID, nil
	}

	// Load all projects
	projects, err := a.loadProjects()
	if err != nil {
		return "", err
	}

	// Find matching project
	for _, proj := range projects {
		// Skip global project
		if proj.Worktree == "/" {
			continue
		}

		// Normalize worktree path
		worktree := proj.Worktree
		if resolved, err := filepath.EvalSymlinks(worktree); err == nil {
			worktree = resolved
		}
		worktree = filepath.Clean(worktree)

		if worktree == absRoot {
			a.projectIndex[absRoot] = proj
			return proj.ID, nil
		}
	}

	return "", nil
}

// loadProjects loads all projects from storage/project/*.json.
func (a *Adapter) loadProjects() ([]*Project, error) {
	projectDir := filepath.Join(a.storageDir, "project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var projects []*Project
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(projectDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var proj Project
		if err := json.Unmarshal(data, &proj); err != nil {
			continue
		}

		projects = append(projects, &proj)
	}

	return projects, nil
}

// parseSessionFile parses a session JSON file and returns metadata.
func (a *Adapter) parseSessionFile(path, projectID string) (*SessionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}

	meta := &SessionMetadata{
		Path:      path,
		SessionID: sess.ID,
		ProjectID: projectID,
		Title:     sess.Title,
		ParentID:  sess.ParentID,
		FirstMsg:  sess.Time.CreatedTime(),
		LastMsg:   sess.Time.UpdatedTime(),
	}

	// Get summary stats from session if available
	if sess.Summary != nil {
		meta.Additions = sess.Summary.Additions
		meta.Deletions = sess.Summary.Deletions
		meta.FileCount = sess.Summary.Files
	} else {
		// Fallback: load stats from session_diff only if Summary not available
		diffPath := filepath.Join(a.storageDir, "session_diff", sess.ID+".json")
		if diffData, err := os.ReadFile(diffPath); err == nil {
			var diffs []SessionDiffEntry
			if json.Unmarshal(diffData, &diffs) == nil {
				for _, d := range diffs {
					if d.Additions != nil {
						meta.Additions += *d.Additions
					}
					if d.Deletions != nil {
						meta.Deletions += *d.Deletions
					}
				}
				meta.FileCount = len(diffs)
			}
		}
	}

	// Count messages and calculate tokens
	messageDir := filepath.Join(a.storageDir, "message", sess.ID)
	if entries, err := os.ReadDir(messageDir); err == nil {
		modelCounts := make(map[string]int)
		modelTokens := make(map[string]struct{ in, out, cache int })
		var firstUserMsgTime time.Time

		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}

			msgPath := filepath.Join(messageDir, e.Name())
			msg, err := a.parseMessageFile(msgPath)
			if err != nil {
				continue
			}

			if msg.Role != "user" && msg.Role != "assistant" {
				continue
			}

			meta.MsgCount++

			// Extract first user message content for title
			if msg.Role == "user" {
				msgTime := msg.Time.CreatedTime()
				if firstUserMsgTime.IsZero() || msgTime.Before(firstUserMsgTime) {
					firstUserMsgTime = msgTime
					// Load parts to get text content
					content, _, _, _, _ := a.loadParts(msg.ID)
					if content != "" {
						meta.FirstUserMessage = content
					}
				}
			}

			if msg.Tokens != nil {
				meta.TotalTokens += msg.Tokens.Input + msg.Tokens.Output

				model := msg.ModelID
				if model == "" && msg.Model != nil {
					model = msg.Model.ModelID
				}
				if model != "" {
					modelCounts[model]++
					mt := modelTokens[model]
					mt.in += msg.Tokens.Input
					mt.out += msg.Tokens.Output
					if msg.Tokens.Cache != nil {
						mt.cache += msg.Tokens.Cache.Read
					}
					modelTokens[model] = mt
				}
			}
		}

		// Find primary model
		var maxCount int
		for model, count := range modelCounts {
			if count > maxCount {
				maxCount = count
				meta.PrimaryModel = model
			}
		}

		// Calculate cost
		for model, mt := range modelTokens {
			meta.EstCost += calculateCost(model, mt.in, mt.out, mt.cache)
		}
	}

	return meta, nil
}

// parseMessageFile parses a message JSON file.
func (a *Adapter) parseMessageFile(path string) (*Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// loadParts loads all parts for a message and returns aggregated content.
func (a *Adapter) loadParts(messageID string) (content string, toolUses []adapter.ToolUse, thinkingBlocks []adapter.ThinkingBlock, fileRefs []string, patchFiles []string) {
	partDir := filepath.Join(a.storageDir, "part", messageID)
	entries, err := os.ReadDir(partDir)
	if err != nil {
		return
	}

	var textParts []string

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(partDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var part Part
		if err := json.Unmarshal(data, &part); err != nil {
			continue
		}

		switch part.Type {
		case "text":
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}

		case "tool":
			tu := adapter.ToolUse{
				ID:   part.CallID,
				Name: part.Tool,
			}
			if part.State != nil {
				tu.Input = ToolInputString(part.State.Input)
				tu.Output = ToolOutputString(part.State.Output)
			}
			toolUses = append(toolUses, tu)

		case "file":
			if part.Filename != "" {
				fileRefs = append(fileRefs, part.Filename)
			}

		case "patch":
			patchFiles = append(patchFiles, part.Files...)

		case "step-finish":
			// Could extract thinking tokens here if needed
			// For now, we skip step markers

		case "step-start", "compaction":
			// Skip these types
		}
	}

	content = strings.Join(textParts, "\n")
	return
}

// calculateCost estimates cost based on model and token usage.
func calculateCost(model string, inputTokens, outputTokens, cacheRead int) float64 {
	var inRate, outRate float64
	model = strings.ToLower(model)

	switch {
	case strings.Contains(model, "opus"):
		inRate, outRate = 15.0, 75.0
	case strings.Contains(model, "sonnet"):
		inRate, outRate = 3.0, 15.0
	case strings.Contains(model, "haiku"):
		inRate, outRate = 0.25, 1.25
	case strings.Contains(model, "gpt-4o"):
		inRate, outRate = 2.5, 10.0
	case strings.Contains(model, "gpt-4"):
		inRate, outRate = 10.0, 30.0
	case strings.Contains(model, "o1"):
		inRate, outRate = 15.0, 60.0
	case strings.Contains(model, "gemini"):
		inRate, outRate = 1.25, 5.0
	case strings.Contains(model, "deepseek"):
		inRate, outRate = 0.14, 0.28
	default:
		// Default to sonnet rates
		inRate, outRate = 3.0, 15.0
	}

	// Cache reads get 90% discount
	regularIn := inputTokens - cacheRead
	if regularIn < 0 {
		regularIn = 0
	}
	cacheInCost := float64(cacheRead) * inRate * 0.1 / 1_000_000
	regularInCost := float64(regularIn) * inRate / 1_000_000
	outCost := float64(outputTokens) * outRate / 1_000_000

	return cacheInCost + regularInCost + outCost
}

// shortID returns the first 12 characters of an ID, or the full ID if shorter.
func shortID(id string) string {
	if len(id) >= 12 {
		return id[:12]
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
