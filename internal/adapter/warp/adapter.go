package warp

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/marcus/sidecar/internal/adapter"
	_ "github.com/mattn/go-sqlite3"
)

// ansiRegex matches ANSI escape codes
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

const (
	adapterID    = "warp"
	adapterName  = "Warp"
	queryTimeout = 5 * time.Second // timeout for database queries
)

// Adapter implements the adapter.Adapter interface for Warp terminal AI sessions.
type Adapter struct {
	dbPath       string
	sessionIndex map[string]struct{} // tracks known conversation IDs
	indexMu      sync.RWMutex        // protects sessionIndex access
	db           *sql.DB             // persistent connection
	dbMu         sync.Mutex          // protects db access
}

// New creates a new Warp adapter.
func New() *Adapter {
	home, _ := os.UserHomeDir()
	dbPath := findWarpDB(home)
	return &Adapter{
		dbPath:       dbPath,
		sessionIndex: make(map[string]struct{}),
	}
}

// findWarpDB searches candidate paths for the Warp SQLite database.
// Returns the first path that exists, or the primary platform default if none found.
func findWarpDB(home string) string {
	candidates := warpDBCandidates(home)
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return filepath.Join(home, ".local", "state", "warp-terminal", "warp.sqlite")
}

// warpDBCandidates returns platform-ordered candidate paths for the Warp database.
func warpDBCandidates(home string) []string {
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, filepath.Join(home,
			"Library", "Group Containers", "2BBY89MBSN.dev.warp",
			"Library", "Application Support", "dev.warp.Warp-Stable",
			"warp.sqlite"))
	case "linux":
		stateHome := os.Getenv("XDG_STATE_HOME")
		if stateHome == "" {
			stateHome = filepath.Join(home, ".local", "state")
		}
		candidates = append(candidates, filepath.Join(stateHome, "warp-terminal", "warp.sqlite"))
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			candidates = append(candidates, filepath.Join(localAppData, "warp", "Warp", "data", "warp.sqlite"))
		}
	}

	return candidates
}

// ID returns the adapter identifier.
func (a *Adapter) ID() string { return adapterID }

// Name returns the human-readable adapter name.
func (a *Adapter) Name() string { return adapterName }

// Icon returns the adapter icon for badge display.
func (a *Adapter) Icon() string { return "»" }

// Capabilities returns the supported features.
func (a *Adapter) Capabilities() adapter.CapabilitySet {
	return adapter.CapabilitySet{
		adapter.CapSessions: true,
		adapter.CapMessages: true,
		adapter.CapUsage:    true,
		adapter.CapWatch:    true,
	}
}

// Detect checks if Warp AI sessions exist for the given project.
func (a *Adapter) Detect(projectRoot string) (bool, error) {
	if _, err := os.Stat(a.dbPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	db, err := a.getDB()
	if err != nil {
		return false, nil // DB exists but can't open - not an error, just not detected
	}

	// Check for ai_queries matching this project
	query := `
		SELECT 1 FROM ai_queries
		WHERE working_directory LIKE ? OR working_directory = ?
		LIMIT 1
	`
	projectAbs := resolveProjectPath(projectRoot)
	pattern := projectAbs + "%"

	var exists int
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	err = db.QueryRowContext(ctx, query, pattern, projectAbs).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, nil // Query failed - not an error, just not detected
	}

	return true, nil
}

// Sessions returns all sessions for the given project, sorted by update time.
func (a *Adapter) Sessions(projectRoot string) ([]adapter.Session, error) {
	db, err := a.getDB()
	if err != nil {
		return nil, err
	}

	projectAbs := resolveProjectPath(projectRoot)
	pattern := projectAbs + "%"

	// Query distinct conversations with aggregated data
	query := `
		SELECT
			q.conversation_id,
			q.working_directory,
			q.model_id,
			MIN(q.start_ts) as first_msg,
			MAX(q.start_ts) as last_msg,
			COUNT(*) as exchange_count,
			(SELECT input FROM ai_queries WHERE conversation_id = q.conversation_id ORDER BY start_ts LIMIT 1) as first_input,
			COALESCE(c.conversation_data, '') as conversation_data
		FROM ai_queries q
		LEFT JOIN agent_conversations c ON c.conversation_id = q.conversation_id
		WHERE q.working_directory LIKE ? OR q.working_directory = ?
		GROUP BY q.conversation_id
		ORDER BY last_msg DESC
	`

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, query, pattern, projectAbs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []adapter.Session
	for rows.Next() {
		var (
			convID         string
			workDir        string
			modelID        sql.NullString
			firstMsgStr    string
			lastMsgStr     string
			exchangeCount  int
			firstInputJSON sql.NullString
			convDataJSON   string
		)

		if err := rows.Scan(&convID, &workDir, &modelID, &firstMsgStr, &lastMsgStr, &exchangeCount, &firstInputJSON, &convDataJSON); err != nil {
			continue
		}

		// Filter by project path (with symlink resolution)
		if !cwdMatchesProject(projectRoot, workDir) {
			continue
		}

		// Parse timestamps
		firstMsg := parseWarpTimestamp(firstMsgStr)
		lastMsg := parseWarpTimestamp(lastMsgStr)

		// Extract session name from first query
		name := shortConversationID(convID)
		if firstInputJSON.Valid && firstInputJSON.String != "" {
			if queryText := extractQueryText(firstInputJSON.String); queryText != "" {
				name = truncateText(queryText, 50)
			}
		}

		// Parse conversation_data for token totals
		totalTokens := 0
		var estCost float64
		if convDataJSON != "" {
			var convData ConversationData
			if err := json.Unmarshal([]byte(convDataJSON), &convData); err == nil && convData.UsageMetadata != nil {
				for _, usage := range convData.UsageMetadata.TokenUsage {
					totalTokens += usage.WarpTokens + usage.BYOKTokens
				}
				estCost = convData.UsageMetadata.CreditsSpent / 100 // Convert credits to dollars
			}
		}

		// Determine if session is active (updated in last 5 minutes)
		isActive := time.Since(lastMsg) < 5*time.Minute

		sessions = append(sessions, adapter.Session{
			ID:           convID,
			Name:         name,
			Slug:         shortConversationID(convID),
			AdapterID:    adapterID,
			AdapterName:  adapterName,
			AdapterIcon:  a.Icon(),
			CreatedAt:    firstMsg,
			UpdatedAt:    lastMsg,
			Duration:     lastMsg.Sub(firstMsg),
			IsActive:     isActive,
			TotalTokens:  totalTokens,
			EstCost:      estCost,
			MessageCount: exchangeCount,
			FileSize:     0, // Warp uses shared SQLite DB, size not per-session
		})

		a.indexMu.Lock()
		a.sessionIndex[convID] = struct{}{}
		a.indexMu.Unlock()
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by UpdatedAt descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Messages returns all messages for the given session.
func (a *Adapter) Messages(sessionID string) ([]adapter.Message, error) {
	db, err := a.getDB()
	if err != nil {
		return nil, err
	}

	var messages []adapter.Message

	// 1. Get user queries for this conversation
	querySQL := `
		SELECT exchange_id, input, model_id, start_ts
		FROM ai_queries
		WHERE conversation_id = ?
		ORDER BY start_ts
	`
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	rows, err := db.QueryContext(ctx, querySQL, sessionID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var (
			exchangeID string
			inputJSON  sql.NullString
			modelID    sql.NullString
			startTSStr string
		)
		if err := rows.Scan(&exchangeID, &inputJSON, &modelID, &startTSStr); err != nil {
			continue
		}

		// Extract query text from input JSON
		queryText := ""
		if inputJSON.Valid && inputJSON.String != "" && inputJSON.String != "[]" {
			queryText = extractQueryText(inputJSON.String)
		}

		// Skip exchanges with no query text (continuation exchanges)
		if queryText == "" {
			continue
		}

		ts := parseWarpTimestamp(startTSStr)
		model := ""
		if modelID.Valid {
			model = modelID.String
		}

		messages = append(messages, adapter.Message{
			ID:        exchangeID,
			Role:      "user",
			Content:   queryText,
			Timestamp: ts,
			Model:     model,
		})
	}
	_ = rows.Close()

	// 2. Get blocks (tool executions) for this conversation
	blocksSQL := `
		SELECT id, stylized_command, stylized_output, exit_code, start_ts, ai_metadata
		FROM blocks
		WHERE ai_metadata LIKE ?
		ORDER BY start_ts
	`
	pattern := "%" + sessionID + "%"
	ctx2, cancel2 := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel2()
	rows, err = db.QueryContext(ctx2, blocksSQL, pattern)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var toolUses []adapter.ToolUse
	var lastBlockTS time.Time

	for rows.Next() {
		var (
			blockID    int
			cmdBytes   []byte
			outBytes   []byte
			exitCode   int
			startTSStr string
			aiMetadata sql.NullString
		)
		if err := rows.Scan(&blockID, &cmdBytes, &outBytes, &exitCode, &startTSStr, &aiMetadata); err != nil {
			continue
		}

		// Verify this block belongs to our conversation
		if !aiMetadata.Valid {
			continue
		}
		var meta BlockAIMetadata
		if err := json.Unmarshal([]byte(aiMetadata.String), &meta); err != nil {
			continue
		}
		if meta.ConversationID != sessionID {
			continue
		}

		ts := parseWarpTimestamp(startTSStr)
		if ts.After(lastBlockTS) {
			lastBlockTS = ts
		}

		// Strip ANSI codes from command and output
		cmd := stripANSI(string(cmdBytes))
		out := stripANSI(string(outBytes))

		toolUses = append(toolUses, adapter.ToolUse{
			ID:     meta.ActionID,
			Name:   "run_command",
			Input:  cmd,
			Output: truncateOutput(out, 1000),
		})
	}

	// 3. If we have tool uses, create a synthetic assistant message
	if len(toolUses) > 0 {
		// Synthesize content from tool calls
		var parts []string
		for _, tu := range toolUses {
			parts = append(parts, "[Executed: "+tu.Input+"]")
		}
		content := strings.Join(parts, "\n")

		messages = append(messages, adapter.Message{
			ID:        "assistant-" + sessionID,
			Role:      "assistant",
			Content:   content,
			Timestamp: lastBlockTS,
			ToolUses:  toolUses,
		})
	}

	// Sort all messages chronologically
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

// Usage returns aggregate usage stats for the given session.
func (a *Adapter) Usage(sessionID string) (*adapter.UsageStats, error) {
	db, err := a.getDB()
	if err != nil {
		return nil, err
	}

	// Get conversation_data from agent_conversations
	query := `SELECT conversation_data FROM agent_conversations WHERE conversation_id = ?`
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	var convDataJSON sql.NullString
	err = db.QueryRowContext(ctx, query, sessionID).Scan(&convDataJSON)
	if err == sql.ErrNoRows {
		// No usage data available, return zeros
		return &adapter.UsageStats{}, nil
	}
	if err != nil {
		return nil, err
	}

	if !convDataJSON.Valid || convDataJSON.String == "" {
		return &adapter.UsageStats{}, nil
	}

	var convData ConversationData
	if err := json.Unmarshal([]byte(convDataJSON.String), &convData); err != nil {
		return &adapter.UsageStats{}, nil
	}

	stats := &adapter.UsageStats{}
	if convData.UsageMetadata == nil {
		return stats, nil
	}

	// Aggregate token usage across all models
	for _, usage := range convData.UsageMetadata.TokenUsage {
		totalTokens := usage.WarpTokens + usage.BYOKTokens
		// Warp doesn't separate input/output, so we estimate 80% input, 20% output
		stats.TotalInputTokens += int(float64(totalTokens) * 0.8)
		stats.TotalOutputTokens += int(float64(totalTokens) * 0.2)
	}

	// Count messages from ai_queries
	countQuery := `SELECT COUNT(*) FROM ai_queries WHERE conversation_id = ?`
	ctx2, cancel2 := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel2()
	err = db.QueryRowContext(ctx2, countQuery, sessionID).Scan(&stats.MessageCount)
	if err != nil {
		stats.MessageCount = 0
	}

	return stats, nil
}

// Watch returns a channel that emits events when session data changes.
func (a *Adapter) Watch(projectRoot string) (<-chan adapter.Event, io.Closer, error) {
	return NewWatcher(a.dbPath)
}

// WatchScope returns Global because warp watches a global database file (td-7a72b6f7).
func (a *Adapter) WatchScope() adapter.WatchScope {
	return adapter.WatchScopeGlobal
}

// getDB returns a persistent database connection, creating one if needed.
func (a *Adapter) getDB() (*sql.DB, error) {
	a.dbMu.Lock()
	defer a.dbMu.Unlock()

	if a.db != nil {
		// Verify connection is still alive with timeout to prevent deadlock
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := a.db.PingContext(ctx)
		cancel()
		if err == nil {
			return a.db, nil
		}
		// Connection dead, close and recreate
		_ = a.db.Close()
		a.db = nil
	}

	// Open new connection with read-only mode and WAL mode
	connStr := a.dbPath + "?mode=ro&_journal_mode=WAL"
	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, err
	}

	// Configure connection pool for single connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	a.db = db
	return a.db, nil
}

// Close closes the persistent database connection.
func (a *Adapter) Close() error {
	a.dbMu.Lock()
	defer a.dbMu.Unlock()

	if a.db != nil {
		err := a.db.Close()
		a.db = nil
		return err
	}
	return nil
}

// resolveProjectPath returns the absolute, symlink-resolved path.
func resolveProjectPath(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return projectRoot
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs)
}

// cwdMatchesProject checks if the working directory matches the project root.
func cwdMatchesProject(projectRoot, cwd string) bool {
	if projectRoot == "" || cwd == "" {
		return false
	}
	projectAbs := resolveProjectPath(projectRoot)
	cwdAbs := resolveProjectPath(cwd)

	if projectAbs == "" || cwdAbs == "" {
		return false
	}

	rel, err := filepath.Rel(projectAbs, cwdAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

// parseWarpTimestamp parses Warp's SQLite timestamp format.
func parseWarpTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Warp uses ISO 8601 format: "2025-12-27 10:12:11"
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339,
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// extractQueryText extracts the user's query text from the input JSON.
// Input format: [{"Query": {"text": "...", ...}}]
func extractQueryText(inputJSON string) string {
	if inputJSON == "" || inputJSON == "[]" {
		return ""
	}
	var inputs []QueryInput
	if err := json.Unmarshal([]byte(inputJSON), &inputs); err != nil {
		return ""
	}
	if len(inputs) == 0 {
		return ""
	}
	return inputs[0].Query.Text
}

// shortConversationID returns the first 8 characters of a conversation ID.
func shortConversationID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	return id
}

// truncateText truncates text to maxLen, adding "..." if truncated.
func truncateText(s string, maxLen int) string {
	// Replace newlines with spaces for display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// stripANSI removes ANSI escape codes from text.
func stripANSI(s string) string {
	return strings.TrimSpace(ansiRegex.ReplaceAllString(s, ""))
}

// truncateOutput truncates command output to maxLen characters.
func truncateOutput(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
