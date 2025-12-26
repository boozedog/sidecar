package adapter

import "time"

// Adapter provides access to AI session data from various sources.
type Adapter interface {
	ID() string
	Name() string
	Detect(projectRoot string) (bool, error)
	Capabilities() CapabilitySet
	Sessions(projectRoot string) ([]Session, error)
	Messages(sessionID string) ([]Message, error)
	Usage(sessionID string) (*UsageStats, error)
	Watch(projectRoot string) (<-chan Event, error)
}

// Capability represents a feature supported by an adapter.
type Capability string

const (
	CapSessions Capability = "sessions"
	CapMessages Capability = "messages"
	CapUsage    Capability = "usage"
	CapWatch    Capability = "watch"
)

// CapabilitySet tracks which features an adapter supports.
type CapabilitySet map[Capability]bool

// Session represents an AI coding session.
type Session struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	IsActive  bool
}

// Message represents a message in a session.
type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
	TokenUsage
	ToolUses []ToolUse
}

// TokenUsage tracks token counts for a message or session.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
}

// ToolUse represents a tool call made by the AI.
type ToolUse struct {
	ID     string
	Name   string
	Input  string
	Output string
}

// UsageStats provides aggregate usage statistics.
type UsageStats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheRead    int
	TotalCacheWrite   int
	MessageCount      int
}

// Event represents a change in session data.
type Event struct {
	Type      EventType
	SessionID string
	Data      any
}

// EventType identifies the kind of adapter event.
type EventType string

const (
	EventSessionCreated EventType = "session_created"
	EventSessionUpdated EventType = "session_updated"
	EventMessageAdded   EventType = "message_added"
)
