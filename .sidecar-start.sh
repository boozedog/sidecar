#!/bin/bash
# Setup PATH for tools installed via nvm, homebrew, etc.
export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
[ -s "$NVM_DIR/nvm.sh" ] && source "$NVM_DIR/nvm.sh" 2>/dev/null
# Fallback: source shell profile if nvm not found
if ! command -v node &>/dev/null; then
  [ -f "$HOME/.zshrc" ] && source "$HOME/.zshrc" 2>/dev/null
  [ -f "$HOME/.bashrc" ] && source "$HOME/.bashrc" 2>/dev/null
fi

claude --dangerously-skip-permissions "$(cat <<'SIDECAR_PROMPT_EOF'
Task: conversations yank: Add y/Y key bindings to keymap/bindings.go

Add key bindings for y (yank-details) and Y (yank-resume) to internal/keymap/bindings.go for all relevant conversation contexts.

## Files to modify
- internal/keymap/bindings.go

## Implementation

Add these bindings after the existing conversations bindings (around line 77-132):

```go
// Conversations context (session list single-pane)
{Key: "y", Command: "yank-details", Context: "conversations"},
{Key: "Y", Command: "yank-resume", Context: "conversations"},

// Conversations sidebar context (two-pane mode, left pane)
{Key: "y", Command: "yank-details", Context: "conversations-sidebar"},
{Key: "Y", Command: "yank-resume", Context: "conversations-sidebar"},

// Conversations main context (two-pane mode, right pane - turns)
{Key: "y", Command: "yank-details", Context: "conversations-main"},
{Key: "Y", Command: "yank-resume", Context: "conversations-main"},

// Conversation detail context (turn list single-pane)
{Key: "y", Command: "yank-details", Context: "conversation-detail"},
{Key: "Y", Command: "yank-resume", Context: "conversation-detail"},

// Message detail context (single turn detail view)
{Key: "y", Command: "yank-details", Context: "message-detail"},
{Key: "Y", Command: "yank-resume", Context: "message-detail"},
```

## Acceptance
- All 10 bindings added (5 contexts × 2 commands)
- Bindings follow existing naming conventions (yank-details, yank-resume)
- Bindings are grouped with other conversation-related bindings
SIDECAR_PROMPT_EOF
)"
rm -f "/Users/marcusvorwaller/code/sidecar-td-c92aa56d-conversations-yank-add-y/y-key-bindings/.sidecar-start.sh"
