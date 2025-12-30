package app

import "testing"

func TestIsGlobalRefreshContext(t *testing.T) {
	tests := []struct {
		context string
		want    bool
	}{
		// Contexts where 'r' should trigger global refresh
		{"global", true},
		{"", true},
		{"git-status", true},
		{"git-history", true},
		{"git-commit-detail", true},
		{"git-diff", true},
		{"conversations", true},
		{"conversation-detail", true},
		{"message-detail", true},
		{"file-browser-preview", true},

		// Contexts where 'r' should be forwarded to plugin
		// (text input or plugin-specific 'r' binding)
		{"td-monitor", false},           // 'r' for mark-review
		{"file-browser-tree", false},    // 'r' for rename
		{"file-browser-search", false},  // text input
		{"file-browser-content-search", false}, // text input
		{"file-browser-quick-open", false},     // text input
		{"file-browser-file-op", false},        // text input
		{"conversations-search", false},        // text input
		{"conversations-filter", false},        // text input
		{"git-commit", false},                  // text input (commit message)
		{"td-modal", false},                    // modal view
		{"palette", false},                     // command palette
		{"diagnostics", false},                 // diagnostics view
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			got := isGlobalRefreshContext(tt.context)
			if got != tt.want {
				t.Errorf("isGlobalRefreshContext(%q) = %v, want %v", tt.context, got, tt.want)
			}
		})
	}
}

func TestIsRootContext(t *testing.T) {
	tests := []struct {
		context string
		want    bool
	}{
		// Root contexts where 'q' should quit
		{"global", true},
		{"", true},
		{"conversations", true},
		{"conversations-sidebar", true},
		{"git-status", true},
		{"git-status-commits", true},
		{"git-status-diff", true},
		{"file-browser-tree", true},
		{"td-monitor", true},

		// Non-root contexts (sub-views)
		{"git-commit", false},
		{"file-browser-preview", false},
		{"conversation-detail", false},
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			got := isRootContext(tt.context)
			if got != tt.want {
				t.Errorf("isRootContext(%q) = %v, want %v", tt.context, got, tt.want)
			}
		})
	}
}
