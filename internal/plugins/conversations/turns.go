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
			if runes := []rune(content); len(runes) > maxLen {
				return string(runes[:maxLen-3]) + "..."
			}
			return content
		}
	}
	// Fallback: show tool result count or first content
	if len(t.Messages) > 0 && t.Messages[0].Content != "" {
		content := stripXMLTags(t.Messages[0].Content)
		if runes := []rune(content); len(runes) > maxLen {
			return string(runes[:maxLen-3]) + "..."
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
		currentTurn.TotalTokensIn += msg.InputTokens + msg.CacheRead + msg.CacheWrite
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

// AppendMessagesToTurns incrementally adds new messages to existing turns.
// It handles the case where the first new message may extend the last turn.
// Returns the updated turns slice (may modify the last element in place).
func AppendMessagesToTurns(turns []Turn, newMessages []adapter.Message, startIndex int) []Turn {
	if len(newMessages) == 0 {
		return turns
	}

	// If no existing turns, create from scratch
	if len(turns) == 0 {
		return groupMessagesWithOffset(newMessages, startIndex)
	}

	// Check if first new message extends the last turn
	lastTurn := &turns[len(turns)-1]
	msgIdx := startIndex

	for i, msg := range newMessages {
		if i == 0 && msg.Role == lastTurn.Role {
			// Extend the last turn
			addMessageToTurn(lastTurn, msg)
			msgIdx++
			continue
		}

		if msg.Role == lastTurn.Role {
			// Still same role, extend current turn
			addMessageToTurn(lastTurn, msg)
		} else {
			// Role changed, create new turn
			newTurn := Turn{
				Role:       msg.Role,
				StartIndex: msgIdx,
			}
			addMessageToTurn(&newTurn, msg)
			turns = append(turns, newTurn)
			lastTurn = &turns[len(turns)-1]
		}
		msgIdx++
	}

	return turns
}

// groupMessagesWithOffset groups messages into turns with a starting index offset.
func groupMessagesWithOffset(messages []adapter.Message, offset int) []Turn {
	if len(messages) == 0 {
		return nil
	}

	var turns []Turn
	currentTurn := Turn{
		Role:       messages[0].Role,
		StartIndex: offset,
	}

	for i, msg := range messages {
		if msg.Role != currentTurn.Role {
			turns = append(turns, currentTurn)
			currentTurn = Turn{
				Role:       msg.Role,
				StartIndex: offset + i,
			}
		}
		addMessageToTurn(&currentTurn, msg)
	}

	if len(currentTurn.Messages) > 0 {
		turns = append(turns, currentTurn)
	}

	return turns
}

// addMessageToTurn adds a message to a turn, updating all aggregates.
func addMessageToTurn(t *Turn, msg adapter.Message) {
	t.Messages = append(t.Messages, msg)
	t.TotalTokensIn += msg.InputTokens + msg.CacheRead + msg.CacheWrite
	t.TotalTokensOut += msg.OutputTokens
	t.ToolCount += len(msg.ToolUses)
	for _, tb := range msg.ThinkingBlocks {
		t.ThinkingTokens += tb.TokenCount
	}
}
