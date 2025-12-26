package keymap

// DefaultBindings returns the default key bindings.
func DefaultBindings() []Binding {
	return []Binding{
		// Global bindings
		{Key: "q", Command: "quit", Context: "global"},
		{Key: "ctrl+c", Command: "quit", Context: "global"},
		{Key: "tab", Command: "next-plugin", Context: "global"},
		{Key: "shift+tab", Command: "prev-plugin", Context: "global"},
		{Key: "1", Command: "focus-plugin-1", Context: "global"},
		{Key: "2", Command: "focus-plugin-2", Context: "global"},
		{Key: "3", Command: "focus-plugin-3", Context: "global"},
		{Key: "4", Command: "focus-plugin-4", Context: "global"},
		{Key: "5", Command: "focus-plugin-5", Context: "global"},
		{Key: "6", Command: "focus-plugin-6", Context: "global"},
		{Key: "7", Command: "focus-plugin-7", Context: "global"},
		{Key: "8", Command: "focus-plugin-8", Context: "global"},
		{Key: "9", Command: "focus-plugin-9", Context: "global"},
		{Key: "?", Command: "toggle-help", Context: "global"},
		{Key: "!", Command: "toggle-diagnostics", Context: "global"},
		{Key: "ctrl+h", Command: "toggle-footer", Context: "global"},
		{Key: "r", Command: "refresh", Context: "global"},
		{Key: "j", Command: "cursor-down", Context: "global"},
		{Key: "down", Command: "cursor-down", Context: "global"},
		{Key: "k", Command: "cursor-up", Context: "global"},
		{Key: "up", Command: "cursor-up", Context: "global"},
		{Key: "g g", Command: "cursor-top", Context: "global"},
		{Key: "G", Command: "cursor-bottom", Context: "global"},
		{Key: "enter", Command: "select", Context: "global"},
		{Key: "esc", Command: "back", Context: "global"},

		// Git Status context
		{Key: "s", Command: "stage-file", Context: "git-status"},
		{Key: "u", Command: "unstage-file", Context: "git-status"},
		{Key: "d", Command: "show-diff", Context: "git-status"},
		{Key: "D", Command: "show-diff-staged", Context: "git-status"},
		{Key: "v", Command: "toggle-diff-mode", Context: "git-status"},
		{Key: "enter", Command: "open-file", Context: "git-status"},

		// Git Diff context
		{Key: "esc", Command: "close-diff", Context: "git-diff"},
		{Key: "j", Command: "scroll", Context: "git-diff"},
		{Key: "k", Command: "scroll", Context: "git-diff"},

		// TD Monitor context
		{Key: "a", Command: "approve-issue", Context: "td-monitor"},
		{Key: "x", Command: "delete-issue", Context: "td-monitor"},
		{Key: "enter", Command: "view-details", Context: "td-monitor"},

		// TD Detail context
		{Key: "esc", Command: "back", Context: "td-detail"},
		{Key: "a", Command: "approve-issue", Context: "td-detail"},
		{Key: "x", Command: "delete-issue", Context: "td-detail"},

		// Conversations context
		{Key: "enter", Command: "view-session", Context: "conversations"},

		// Conversation detail context
		{Key: "esc", Command: "back", Context: "conversation-detail"},
		{Key: "j", Command: "scroll", Context: "conversation-detail"},
		{Key: "k", Command: "scroll", Context: "conversation-detail"},
	}
}

// RegisterDefaults registers all default bindings with the registry.
func RegisterDefaults(r *Registry) {
	for _, b := range DefaultBindings() {
		r.RegisterBinding(b)
	}
}
