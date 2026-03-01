package workspace

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sessionStatusTailBytes  = 2 * 1024 * 1024
	codexSessionCacheTTL    = 5 * time.Second
	codexCwdCacheMaxEntries = 2048

	// claudeActivityThreshold is used to detect ongoing tool execution when the last
	// JSONL entry is "assistant" (which could mean tool_use in progress or turn complete).
	// Progress entries write every 1-3s during tool execution, so 5s is sufficient.
	// This is NOT used for the "thinking" case — JSONL content (last entry = user)
	// handles that directly regardless of mtime (td-b9cb0b).
	claudeActivityThreshold = 5 * time.Second

	// sessionActivityThreshold is the mtime fast-path threshold for agents that
	// use mtime-first detection (Codex, Pi, OpenCode, etc). Needs to be long
	// enough to cover both tool execution and LLM thinking gaps.
	sessionActivityThreshold = 30 * time.Second

	// subagentMaxStaleness is the maximum time since a sub-agent file was last
	// modified before we consider it abandoned. Sub-agents write progress entries
	// during execution and JSONL entries between turns. Even the longest extended
	// thinking (55s+) combined with tool execution gaps should never exceed 2 minutes.
	// Beyond this, the sub-agent is definitely finished (td-b9cb0b).
	subagentMaxStaleness = 2 * time.Minute
)

type codexSessionCacheEntry struct {
	sessionPath string
	expiresAt   time.Time
}

type codexSessionCwdCacheEntry struct {
	cwd        string
	modTime    time.Time
	size       int64
	lastAccess time.Time
}

var codexSessionCache = struct {
	mu      sync.Mutex
	entries map[string]codexSessionCacheEntry
}{
	entries: make(map[string]codexSessionCacheEntry),
}

var codexSessionCwdCache = struct {
	mu      sync.Mutex
	entries map[string]codexSessionCwdCacheEntry
}{
	entries: make(map[string]codexSessionCwdCacheEntry),
}

// isFileRecentlyModified returns true if the file at path was modified within threshold.
func isFileRecentlyModified(path string, threshold time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < threshold
}

// anyFileRecentlyModified returns true if any file with the given suffix in dir
// was modified within threshold. Used to check sub-agent session files.
func anyFileRecentlyModified(dir, suffix string, threshold time.Duration) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) < threshold {
			return true
		}
	}
	return false
}

// subagentStatus checks the most recent sub-agent in dir and returns its status.
// Uses the same detection logic as the main session: JSONL content + mtime (td-b9cb0b).
//   - fresh mtime + last=user → just submitted → active
//   - stale mtime + last=user → sub-agent model is thinking → thinking
//   - placeholder assistant → sub-agent is thinking → thinking
//   - fresh mtime + real assistant (tool_use or text) → sub-agent executing → active
//   - stale mtime + real assistant → sub-agent finished → (0, false)
func subagentStatus(dir string, mtimeThreshold time.Duration) (WorktreeStatus, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, false
	}

	// Find the most recently modified sub-agent JSONL file.
	var mostRecentPath string
	var mostRecentMtime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if mt := info.ModTime().UnixNano(); mt > mostRecentMtime {
			mostRecentMtime = mt
			mostRecentPath = filepath.Join(dir, e.Name())
		}
	}

	if mostRecentPath == "" {
		return 0, false
	}

	// If the most recent sub-agent hasn't been written to in a long time,
	// it's definitely finished regardless of JSONL content (td-b9cb0b).
	if !isFileRecentlyModified(mostRecentPath, subagentMaxStaleness) {
		return 0, false
	}

	// Check JSONL content for the sub-agent's state.
	status, ok := getClaudeSessionStatus(mostRecentPath)
	if !ok {
		return 0, false
	}

	freshMtime := isFileRecentlyModified(mostRecentPath, mtimeThreshold)

	switch status {
	case StatusActive: // last=user
		if freshMtime {
			return StatusActive, true
		}
		return StatusThinking, true
	case StatusThinking: // placeholder assistant
		return StatusThinking, true
	case StatusWaiting, StatusDone: // real assistant content (tool_use or text-only)
		if freshMtime {
			return StatusActive, true
		}
		return 0, false // sub-agent finished
	}

	return 0, false
}

// detectAgentSessionStatus checks agent session files to determine agent state.
// Returns StatusWaiting if agent needs user approval (tool_use pending).
// Returns StatusDone if agent finished its turn (text-only response, idle).
// Returns StatusActive if agent is processing (last entry = user, or fresh mtime).
// Returns StatusThinking if agent is thinking (stale mtime, waiting for model).
// Returns (0, false) if unable to determine status.
func detectAgentSessionStatus(agentType AgentType, worktreePath string) (WorktreeStatus, bool) {
	switch agentType {
	case AgentClaude:
		return detectClaudeSessionStatus(worktreePath)
	case AgentCodex:
		return detectCodexSessionStatus(worktreePath)
	case AgentGemini:
		return detectGeminiSessionStatus(worktreePath)
	case AgentOpenCode:
		return detectOpenCodeSessionStatus(worktreePath)
	case AgentCursor:
		return detectCursorSessionStatus(worktreePath)
	case AgentPi:
		return detectPiSessionStatus(worktreePath)
	case AgentAmp:
		return detectAmpSessionStatus(worktreePath)
	default:
		return 0, false
	}
}

// claudeProjectDirName encodes an absolute path into Claude Code's project directory name.
// Claude Code replaces slashes, underscores, and other non-alphanumeric characters with dashes.
// e.g., /Users/foo/my_project becomes -Users-foo-my-project
func claudeProjectDirName(absPath string) string {
	var b strings.Builder
	b.Grow(len(absPath))
	for _, r := range absPath {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// detectClaudeSessionStatus checks Claude session files using JSONL content + mtime.
// Claude stores sessions in ~/.claude/projects/{path-with-dashes}/*.jsonl
// Sub-agent sessions in {session-uuid}/subagents/agent-*.jsonl
//
// Detection strategy (td-b9cb0b):
//  1. Always parse JSONL tail for the active/waiting distinction
//  2. last entry = user → active (agent is thinking, mtime irrelevant)
//  3. last entry = assistant → could be tool_use (active) or final response (waiting):
//     - mtime fresh → tool execution in progress (progress entries every 1-3s) → active
//     - sub-agent mtime fresh → sub-agent running → active
//     - both stale → agent finished → waiting
func detectClaudeSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	// Claude Code encodes the project path by replacing non-alphanumeric chars with dashes (td-2fca7d).
	projectDirName := claudeProjectDirName(absPath)
	projectDir := filepath.Join(home, ".claude", "projects", projectDirName)

	// Get session files sorted by mtime (most recent first).
	// We iterate candidates because the most recent file may be abandoned
	// (e.g., only file-history-snapshot entries with no user/assistant messages).
	sessionFiles, err := findRecentJSONLFiles(projectDir, "agent-")
	if err != nil || len(sessionFiles) == 0 {
		slog.Debug("claude session: no session file found", "projectDir", projectDir, "err", err)
		return 0, false
	}

	for _, sessionFile := range sessionFiles {
		// Parse JSONL content for status (td-b9cb0b, td-124b2e).
		// Returns StatusActive (last=user), StatusThinking (placeholder assistant),
		// StatusWaiting (assistant with tool_use), or StatusDone (assistant text-only).
		status, ok := getClaudeSessionStatus(sessionFile)
		if !ok {
			// No user/assistant entry found (abandoned session) — try next candidate (td-2fca7d v8).
			slog.Debug("claude session: skipping abandoned file", "file", filepath.Base(sessionFile))
			continue
		}

		// Last entry is user: agent received a prompt, API stream hasn't opened yet.
		// Fresh mtime → just submitted → active.
		// Stale mtime (>5s) → model is thinking but no placeholder was written
		// (happens when response goes straight to tool_use without text) → thinking.
		if status == StatusActive {
			if isFileRecentlyModified(sessionFile, claudeActivityThreshold) {
				slog.Debug("claude session: active (JSONL last=user, fresh mtime)", "file", filepath.Base(sessionFile))
				return StatusActive, true
			}
			slog.Debug("claude session: thinking (JSONL last=user, stale mtime)", "file", filepath.Base(sessionFile))
			return StatusThinking, true
		}

		// Last assistant entry is a placeholder (whitespace-only content):
		// Claude Code writes this when the API stream opens, before thinking finishes.
		// Model is actively thinking (td-b9cb0b).
		if status == StatusThinking {
			slog.Debug("claude session: thinking (placeholder assistant)", "file", filepath.Base(sessionFile))
			return StatusThinking, true
		}

		// Last entry is assistant with real content (tool_use or text-only).
		// Fresh mtime → progress entries still being written → active regardless.
		if isFileRecentlyModified(sessionFile, claudeActivityThreshold) {
			slog.Debug("claude session: active (assistant + fresh mtime)", "file", filepath.Base(sessionFile))
			return StatusActive, true
		}

		// Check sub-agent files: main session stops receiving writes when a sub-agent
		// is dispatched, but sub-agent files continue being written. We apply the
		// same JSONL + mtime detection to sub-agents so thinking propagates (td-b9cb0b).
		sessionUUID := strings.TrimSuffix(filepath.Base(sessionFile), ".jsonl")
		subagentsDir := filepath.Join(projectDir, sessionUUID, "subagents")
		if subStatus, subOK := subagentStatus(subagentsDir, claudeActivityThreshold); subOK {
			slog.Debug("claude session: sub-agent override", "status", subStatus, "file", filepath.Base(sessionFile))
			return subStatus, true
		}

		// Stale mtime, no active sub-agents. Content determines final status (td-124b2e):
		// - StatusWaiting (tool_use) → agent needs user approval
		// - StatusDone (text-only) → agent finished turn, idle
		slog.Debug("claude session: idle", "status", status, "file", filepath.Base(sessionFile))
		return status, true
	}

	slog.Debug("claude session: no valid session file found", "projectDir", projectDir, "candidates", len(sessionFiles))
	return 0, false
}

// detectCodexSessionStatus checks Codex session files using mtime + JSONL fallback.
// Codex stores sessions in ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl with CWD field.
// Codex has no sub-agents — all activity is recorded in one file per session.
func detectCodexSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	sessionsDir := filepath.Join(home, ".codex", "sessions")

	// Find most recent session file that matches the worktree path
	sessionFile, err := findCodexSessionForPath(sessionsDir, absPath)
	if err != nil || sessionFile == "" {
		return 0, false
	}

	// Fast path: recently modified file means agent is active
	if isFileRecentlyModified(sessionFile, sessionActivityThreshold) {
		return StatusActive, true
	}

	// Slow path: fall back to JSONL content parsing
	return getCodexLastMessageStatus(sessionFile)
}

// detectGeminiSessionStatus checks Gemini CLI session files.
// Gemini stores sessions in ~/.gemini/tmp/{sha256-hash}/chats/session-*.json
func detectGeminiSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	// SHA256 hash of absolute path
	hash := sha256.Sum256([]byte(absPath))
	pathHash := hex.EncodeToString(hash[:])
	chatsDir := filepath.Join(home, ".gemini", "tmp", pathHash, "chats")

	sessionFile, err := findMostRecentJSON(chatsDir, "session-")
	if err != nil || sessionFile == "" {
		return 0, false
	}

	return getGeminiLastMessageStatus(sessionFile)
}

// detectOpenCodeSessionStatus checks OpenCode session files.
// OpenCode stores in ~/.local/share/opencode/storage/ with project/session/message dirs.
func detectOpenCodeSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	storageDir := findOpenCodeStorage(home)

	// Find project matching worktree path
	projectID, err := findOpenCodeProject(storageDir, absPath)
	if err != nil || projectID == "" {
		return 0, false
	}

	// Find most recent session for project
	sessionID, err := findOpenCodeSession(storageDir, projectID)
	if err != nil || sessionID == "" {
		return 0, false
	}

	// Find last message in session
	return getOpenCodeLastMessageStatus(storageDir, sessionID)
}

// detectCursorSessionStatus checks Cursor session files.
// Cursor stores in ~/.cursor/chats/{md5-hash}/{sessionID}/store.db (SQLite).
// For simplicity, we skip SQLite parsing and return false.
func detectCursorSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	// Cursor uses SQLite which requires database/sql and a driver.
	// For now, skip Cursor session detection to avoid adding dependencies.
	// Tmux pattern detection should still work for Cursor.
	return 0, false
}

// detectPiSessionStatus checks Pi Agent session files using mtime + JSONL fallback.
// Pi stores sessions in ~/.pi/agent/sessions/--{path-encoded}--/*.jsonl
// Path encoding: /home/user/project → --home-user-project--
func detectPiSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	// Pi Agent encodes paths: strip leading slash, replace remaining slashes with dashes, wrap in --
	path := strings.TrimPrefix(absPath, "/")
	encoded := strings.ReplaceAll(path, "/", "-")
	projectDir := filepath.Join(home, ".pi", "agent", "sessions", "--"+encoded+"--")

	// Find most recent session file
	sessionFiles, err := findRecentJSONLFiles(projectDir, "")
	if err != nil || len(sessionFiles) == 0 {
		return 0, false
	}

	sessionFile := sessionFiles[0]

	// Fast path: recently modified file means agent is active
	if isFileRecentlyModified(sessionFile, sessionActivityThreshold) {
		return StatusActive, true
	}

	// Slow path: fall back to JSONL content parsing
	// Pi uses "message" type entries with nested message.role field
	return getPiLastMessageStatus(sessionFile)
}

// getPiLastMessageStatus parses a Pi session JSONL file to determine status from last message role.
func getPiLastMessageStatus(path string) (WorktreeStatus, bool) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer func() { _ = file.Close() }()

	// Seek to end - tail bytes for efficiency
	info, err := file.Stat()
	if err != nil {
		return 0, false
	}
	if info.Size() > sessionStatusTailBytes {
		if _, err := file.Seek(-sessionStatusTailBytes, io.SeekEnd); err != nil {
			return 0, false
		}
	}

	var lastRole string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 256*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			Type    string `json:"type"`
			Message *struct {
				Role string `json:"role"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type == "message" && entry.Message != nil {
			role := entry.Message.Role
			if role == "user" || role == "assistant" {
				lastRole = role
			}
		}
	}

	switch lastRole {
	case "user":
		return StatusActive, true // Agent is processing user message
	case "assistant":
		return StatusWaiting, true // Agent finished, waiting for input
	default:
		return 0, false
	}
}

// detectAmpSessionStatus checks Amp thread files using mtime + JSON fallback.
// Amp stores threads in ~/.local/share/amp/threads/T-{uuid}.json
// Each thread has an Env.Initial.Trees[] field with file:// URIs to match worktree path.
func detectAmpSessionStatus(worktreePath string) (WorktreeStatus, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, false
	}

	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return 0, false
	}

	threadsDir := filepath.Join(home, ".local", "share", "amp", "threads")

	// Find thread files matching the worktree path and get last message status
	threadFile, status, ok := findAmpThreadForPath(threadsDir, absPath)
	if !ok || threadFile == "" {
		return 0, false
	}

	// Fast path: recently modified file means agent is active
	if isFileRecentlyModified(threadFile, sessionActivityThreshold) {
		slog.Debug("amp session: active (mtime)", "file", filepath.Base(threadFile))
		return StatusActive, true
	}

	// Slow path: return the status we already parsed from JSON
	return status, true
}

// findAmpThreadForPath finds the most recent Amp thread file matching the worktree path.
// Amp threads contain Env.Initial.Trees[].URI as file:// URIs.
// Returns the thread file path, the parsed status, and true if found.
// This combines path matching and status extraction to avoid reading files twice.
func findAmpThreadForPath(threadsDir, worktreePath string) (string, WorktreeStatus, bool) {
	entries, err := os.ReadDir(threadsDir)
	if err != nil {
		return "", 0, false
	}

	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate

	// First pass: collect T-*.json files with their mtimes (no file reads yet)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "T-") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate{
			path:    filepath.Join(threadsDir, e.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	// Sort by mtime descending (most recent first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})

	// Second pass: check path match and extract status starting from most recent
	// This minimizes file reads - usually the most recent file is the one we want
	for _, c := range candidates {
		if status, ok := getAmpThreadStatus(c.path, worktreePath); ok {
			return c.path, status, true
		}
	}

	return "", 0, false
}

// getAmpThreadStatus checks if an Amp thread file matches the worktree path and returns
// the status from the last message. This combines path matching and status extraction
// into a single file read to avoid reading the file twice.
func getAmpThreadStatus(threadPath, worktreePath string) (WorktreeStatus, bool) {
	data, err := os.ReadFile(threadPath)
	if err != nil {
		return 0, false
	}

	var thread struct {
		Env *struct {
			Initial *struct {
				Trees []struct {
					URI string `json:"uri"`
				} `json:"trees"`
			} `json:"initial"`
		} `json:"env"`
		Messages []struct {
			Role string `json:"role"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &thread); err != nil {
		return 0, false
	}

	// Check if worktree path matches
	if thread.Env == nil || thread.Env.Initial == nil {
		return 0, false
	}

	pathMatches := false
	for _, tree := range thread.Env.Initial.Trees {
		treePath := strings.TrimPrefix(tree.URI, "file://")
		if cwdMatches(treePath, worktreePath) {
			pathMatches = true
			break
		}
	}

	if !pathMatches {
		return 0, false
	}

	// Find last user/assistant message
	var lastRole string
	for _, msg := range thread.Messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			lastRole = msg.Role
		}
	}

	switch lastRole {
	case "user":
		return StatusActive, true
	case "assistant":
		return StatusWaiting, true
	default:
		// Path matches but no messages yet - still a valid thread
		return 0, true
	}
}

func codexSessionCacheKey(sessionsDir, worktreePath string) string {
	return sessionsDir + "\n" + worktreePath
}

func cachedCodexSessionPath(sessionsDir, worktreePath string) (string, bool) {
	key := codexSessionCacheKey(sessionsDir, worktreePath)
	now := time.Now()

	codexSessionCache.mu.Lock()
	entry, ok := codexSessionCache.entries[key]
	codexSessionCache.mu.Unlock()

	if !ok {
		return "", false
	}
	if now.After(entry.expiresAt) {
		codexSessionCache.mu.Lock()
		delete(codexSessionCache.entries, key)
		codexSessionCache.mu.Unlock()
		return "", false
	}
	if entry.sessionPath == "" {
		return "", true
	}
	if _, err := os.Stat(entry.sessionPath); err == nil {
		return entry.sessionPath, true
	}
	codexSessionCache.mu.Lock()
	delete(codexSessionCache.entries, key)
	codexSessionCache.mu.Unlock()
	return "", false
}

func setCachedCodexSessionPath(sessionsDir, worktreePath, sessionPath string) {
	key := codexSessionCacheKey(sessionsDir, worktreePath)
	codexSessionCache.mu.Lock()
	codexSessionCache.entries[key] = codexSessionCacheEntry{
		sessionPath: sessionPath,
		expiresAt:   time.Now().Add(codexSessionCacheTTL),
	}
	codexSessionCache.mu.Unlock()
}

func cachedCodexSessionCWD(path string, info os.FileInfo) (string, bool) {
	codexSessionCwdCache.mu.Lock()
	entry, ok := codexSessionCwdCache.entries[path]
	if ok && entry.size == info.Size() && entry.modTime.Equal(info.ModTime()) {
		entry.lastAccess = time.Now()
		codexSessionCwdCache.entries[path] = entry
		codexSessionCwdCache.mu.Unlock()
		return entry.cwd, true
	}
	if ok {
		delete(codexSessionCwdCache.entries, path)
	}
	codexSessionCwdCache.mu.Unlock()
	return "", false
}

func setCodexSessionCWDCache(path string, info os.FileInfo, cwd string) {
	codexSessionCwdCache.mu.Lock()
	codexSessionCwdCache.entries[path] = codexSessionCwdCacheEntry{
		cwd:        cwd,
		modTime:    info.ModTime(),
		size:       info.Size(),
		lastAccess: time.Now(),
	}
	pruneCodexSessionCWDCacheLocked()
	codexSessionCwdCache.mu.Unlock()
}

func pruneCodexSessionCWDCacheLocked() {
	if len(codexSessionCwdCache.entries) <= codexCwdCacheMaxEntries {
		return
	}
	type cacheEntry struct {
		path       string
		lastAccess time.Time
	}
	entries := make([]cacheEntry, 0, len(codexSessionCwdCache.entries))
	for path, entry := range codexSessionCwdCache.entries {
		entries = append(entries, cacheEntry{path: path, lastAccess: entry.lastAccess})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastAccess.Before(entries[j].lastAccess)
	})
	excess := len(entries) - codexCwdCacheMaxEntries
	for i := 0; i < excess; i++ {
		delete(codexSessionCwdCache.entries, entries[i].path)
	}
}

// findMostRecentJSONL finds most recent .jsonl file in dir.
// excludePrefix: if non-empty, files starting with this prefix are skipped.
func findMostRecentJSONL(dir string, excludePrefix string) (string, error) {
	files, err := findRecentJSONLFiles(dir, excludePrefix)
	if err != nil || len(files) == 0 {
		return "", err
	}
	return files[0], nil
}

// findRecentJSONLFiles returns .jsonl files in dir sorted by mtime descending.
// Used to iterate session candidates when the most recent file is abandoned (td-2fca7d).
func findRecentJSONLFiles(dir string, excludePrefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type fileEntry struct {
		path    string
		modTime int64
	}
	var files []fileEntry

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if excludePrefix != "" && strings.HasPrefix(e.Name(), excludePrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.path
	}
	return result, nil
}

// findMostRecentJSON finds most recent .json file with given prefix.
func findMostRecentJSON(dir string, prefix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var mostRecent string
	var mostRecentTime int64

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(e.Name(), prefix) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		modTime := info.ModTime().UnixNano()
		if modTime > mostRecentTime {
			mostRecentTime = modTime
			mostRecent = filepath.Join(dir, e.Name())
		}
	}

	return mostRecent, nil
}

// readTailLines reads up to maxBytes from the end of a file and returns lines.
// If the read starts mid-line, the first partial line is dropped.
func readTailLines(path string, maxBytes int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	start := int64(0)
	if size > int64(maxBytes) {
		start = size - int64(maxBytes)
	}
	if start > 0 {
		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return nil, err
		}
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if start > 0 && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines, nil
}

// getClaudeSessionStatus reads the tail of a Claude JSONL session file and returns
// a status based on the last user/assistant entry.
//
// Claude Code writes a placeholder assistant entry with whitespace-only text (e.g. "  ")
// when the API stream opens, before extended thinking completes → StatusThinking.
//
// For real assistant entries, the content blocks determine the status (td-124b2e):
//   - tool_use blocks present → StatusWaiting (agent dispatched tool, needs user approval)
//   - text-only blocks → StatusDone (agent finished turn, idle at prompt)
func getClaudeSessionStatus(path string) (WorktreeStatus, bool) {
	lines, err := readTailLines(path, sessionStatusTailBytes)
	if err != nil {
		return 0, false
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		msgType, ok := msg["type"].(string)
		if !ok {
			continue
		}
		switch msgType {
		case "user":
			return StatusActive, true
		case "assistant":
			if isPlaceholderAssistant(msg) {
				return StatusThinking, true
			}
			if hasToolUse(msg) {
				return StatusWaiting, true
			}
			return StatusDone, true
		}
	}
	return 0, false
}

// isPlaceholderAssistant checks if an assistant JSONL entry is a placeholder
// written when the API stream opens. Claude Code writes these with content
// containing only whitespace text blocks before thinking/generation completes.
func isPlaceholderAssistant(entry map[string]any) bool {
	message, ok := entry["message"].(map[string]any)
	if !ok {
		return false
	}
	content, ok := message["content"].([]any)
	if !ok {
		return false
	}
	if len(content) == 0 {
		return true
	}
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok {
			return false
		}
		blockType, _ := b["type"].(string)
		if blockType != "text" {
			return false
		}
		text, _ := b["text"].(string)
		if strings.TrimSpace(text) != "" {
			return false
		}
	}
	return true
}

// hasToolUse checks if an assistant JSONL entry contains any tool_use content blocks.
// tool_use blocks mean the agent dispatched a tool and is waiting for user approval
// (e.g., Bash, Edit, Write). Text-only entries mean the agent finished its turn (td-124b2e).
func hasToolUse(entry map[string]any) bool {
	message, ok := entry["message"].(map[string]any)
	if !ok {
		return false
	}
	content, ok := message["content"].([]any)
	if !ok {
		return false
	}
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if b["type"] == "tool_use" {
			return true
		}
	}
	return false
}

// findCodexSessionForPath finds the most recent Codex session matching CWD.
// Codex stores sessions in a YYYY/MM/DD/ date hierarchy under sessionsDir,
// so we walk the directory tree to find all .jsonl files (td-2fca7d).
func findCodexSessionForPath(sessionsDir, worktreePath string) (string, error) {
	if cached, ok := cachedCodexSessionPath(sessionsDir, worktreePath); ok {
		return cached, nil
	}

	var bestPath string
	var bestModTime int64

	_ = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Check if CWD matches
		cwd, err := getCodexSessionCWD(path, info)
		if err != nil || !cwdMatches(cwd, worktreePath) {
			return nil
		}

		modTime := info.ModTime().UnixNano()
		if modTime > bestModTime {
			bestModTime = modTime
			bestPath = path
		}
		return nil
	})

	if bestPath == "" {
		setCachedCodexSessionPath(sessionsDir, worktreePath, "")
		return "", nil
	}

	setCachedCodexSessionPath(sessionsDir, worktreePath, bestPath)
	return bestPath, nil
}

// getCodexSessionCWD extracts CWD from first session_meta record.
func getCodexSessionCWD(path string, info os.FileInfo) (string, error) {
	if cached, ok := cachedCodexSessionCWD(path, info); ok {
		return cached, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				CWD string `json:"cwd"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.Type == "session_meta" && record.Payload.CWD != "" {
			setCodexSessionCWDCache(path, info, record.Payload.CWD)
			return record.Payload.CWD, nil
		}
	}
	return "", nil
}

// cwdMatches checks if cwd matches or is under worktreePath.
func cwdMatches(cwd, worktreePath string) bool {
	cwd = filepath.Clean(cwd)
	worktreePath = filepath.Clean(worktreePath)
	return cwd == worktreePath || strings.HasPrefix(cwd, worktreePath+string(filepath.Separator))
}

// getCodexLastMessageStatus reads Codex JSONL and finds last message role.
func getCodexLastMessageStatus(path string) (WorktreeStatus, bool) {
	lines, err := readTailLines(path, sessionStatusTailBytes)
	if err != nil {
		return 0, false
	}

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Role string `json:"role"`
			} `json:"payload"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		// Codex uses type="response_item" with payload.type="message"
		if record.Type == "response_item" && record.Payload.Type == "message" {
			switch record.Payload.Role {
			case "assistant":
				return StatusWaiting, true
			case "user":
				return StatusActive, true
			}
		}
	}
	return 0, false
}

// getGeminiLastMessageStatus reads Gemini JSON session file.
func getGeminiLastMessageStatus(path string) (WorktreeStatus, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	var session struct {
		Messages []struct {
			Type string `json:"type"` // "user", "gemini", "info"
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &session); err != nil {
		return 0, false
	}

	// Find last user/gemini message
	var lastType string
	for _, msg := range session.Messages {
		if msg.Type == "user" || msg.Type == "gemini" {
			lastType = msg.Type
		}
	}

	switch lastType {
	case "gemini": // gemini = assistant
		return StatusWaiting, true
	case "user":
		return StatusActive, true
	default:
		return 0, false
	}
}

// findOpenCodeStorage searches candidate paths for the OpenCode storage directory.
func findOpenCodeStorage(home string) string {
	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "opencode", "storage"))
	case "linux":
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
		candidates = append(candidates, filepath.Join(xdgData, "opencode", "storage"))
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			candidates = append(candidates, filepath.Join(localAppData, "opencode", "Data", "storage"))
		}
	}

	defaultPath := filepath.Join(home, ".local", "share", "opencode", "storage")
	if len(candidates) == 0 || candidates[len(candidates)-1] != defaultPath {
		candidates = append(candidates, defaultPath)
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return defaultPath
}

// findOpenCodeProject finds project ID matching worktree path.
func findOpenCodeProject(storageDir, worktreePath string) (string, error) {
	projectDir := filepath.Join(storageDir, "project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		path := filepath.Join(projectDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var project struct {
			ID       string `json:"id"`
			Worktree string `json:"worktree"`
		}
		if err := json.Unmarshal(data, &project); err != nil {
			continue
		}

		if cwdMatches(project.Worktree, worktreePath) {
			return project.ID, nil
		}
	}
	return "", nil
}

// findOpenCodeSession finds most recent session for project.
func findOpenCodeSession(storageDir, projectID string) (string, error) {
	sessionDir := filepath.Join(storageDir, "session", projectID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", err
	}

	var mostRecent string
	var mostRecentTime int64

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		modTime := info.ModTime().UnixNano()
		if modTime > mostRecentTime {
			mostRecentTime = modTime
			mostRecent = strings.TrimSuffix(e.Name(), ".json")
		}
	}

	return mostRecent, nil
}

// getOpenCodeLastMessageStatus finds last message role in OpenCode session.
func getOpenCodeLastMessageStatus(storageDir, sessionID string) (WorktreeStatus, bool) {
	messageDir := filepath.Join(storageDir, "message", sessionID)
	entries, err := os.ReadDir(messageDir)
	if err != nil {
		return 0, false
	}

	// Find most recent message file
	var mostRecent string
	var mostRecentTime int64

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		modTime := info.ModTime().UnixNano()
		if modTime > mostRecentTime {
			mostRecentTime = modTime
			mostRecent = filepath.Join(messageDir, e.Name())
		}
	}

	if mostRecent == "" {
		return 0, false
	}

	data, err := os.ReadFile(mostRecent)
	if err != nil {
		return 0, false
	}

	var msg struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return 0, false
	}

	switch msg.Role {
	case "assistant":
		return StatusWaiting, true
	case "user":
		return StatusActive, true
	default:
		return 0, false
	}
}
