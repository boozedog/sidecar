package conversations

import (
	"regexp"
	"strings"

	"github.com/marcus/sidecar/internal/adapter"
)

// xmlTagRegex is pre-compiled for performance in hot path (called per turn on render)
var xmlTagRegex = regexp.MustCompile(`<[^>]+>`)

// Turn represents a sequence of consecutive messages from the same role.
// In Claude Code, a single "turn" may contain multiple JSONL messages:
// - User turn: system reminders, command output, actual user text
// - Assistant turn: thinking blocks, tool calls, text responses
type Turn struct {
	Role           string            // "user" or "assistant"
	Messages       []adapter.Message // All messages in this turn
	StartIndex     int               // Index of first message in original slice
	TotalTokensIn  int               // Sum of input tokens
	TotalTokensOut int               // Sum of output tokens
	ThinkingTokens int               // Sum of thinking block tokens
	ToolCount      int               // Number of tool uses
}

// FirstTimestamp returns the timestamp of the first message in the turn.
func (t *Turn) FirstTimestamp() string {
	if len(t.Messages) == 0 {
		return ""
	}
	return t.Messages[0].Timestamp.Local().Format("15:04")
}

// Preview returns a content preview from the first message with meaningful content.
func (t *Turn) Preview(maxLen int) string {
	if maxLen < 4 {
		maxLen = 4 // minimum to show "x..."
	}
	for _, msg := range t.Messages {
		if msg.Content != "" && msg.Content != "[1 tool result(s)]" {
			content := stripXMLTags(msg.Content)
			if content == "" {
				continue
			}
			if len(content) > maxLen {
				return content[:maxLen-3] + "..."
			}
			return content
		}
	}
	// Fallback: show tool result count or first content
	if len(t.Messages) > 0 && t.Messages[0].Content != "" {
		content := stripXMLTags(t.Messages[0].Content)
		if len(content) > maxLen {
			return content[:maxLen-3] + "..."
		}
		return content
	}
	return ""
}

// stripXMLTags removes XML tags from content and extracts user query if present.
func stripXMLTags(s string) string {
	// First try to extract user query
	start := strings.Index(s, "<user_query>")
	end := strings.Index(s, "</user_query>")
	if start >= 0 && end > start {
		return strings.TrimSpace(s[start+len("<user_query>") : end])
	}
	// Remove all XML tags (using pre-compiled regex for performance)
	return strings.TrimSpace(xmlTagRegex.ReplaceAllString(s, ""))
}

// GroupMessagesIntoTurns groups consecutive messages by role into turns.
func GroupMessagesIntoTurns(messages []adapter.Message) []Turn {
	if len(messages) == 0 {
		return nil
	}

	var turns []Turn
	currentTurn := Turn{
		Role:       messages[0].Role,
		StartIndex: 0,
	}

	for i, msg := range messages {
		if msg.Role != currentTurn.Role {
			// Role changed - finalize current turn and start new one
			turns = append(turns, currentTurn)
			currentTurn = Turn{
				Role:       msg.Role,
				StartIndex: i,
			}
		}

		// Add message to current turn
		currentTurn.Messages = append(currentTurn.Messages, msg)
		currentTurn.TotalTokensIn += msg.InputTokens
		currentTurn.TotalTokensOut += msg.OutputTokens
		currentTurn.ToolCount += len(msg.ToolUses)

		// Count thinking tokens
		for _, tb := range msg.ThinkingBlocks {
			currentTurn.ThinkingTokens += tb.TokenCount
		}
	}

	// Don't forget the last turn
	if len(currentTurn.Messages) > 0 {
		turns = append(turns, currentTurn)
	}

	return turns
}
