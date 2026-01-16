#!/bin/bash
PROMPT=$(cat "/Users/marcusvorwaller/code/conversations-worktree-implement/.sidecar-prompt")
rm -f "/Users/marcusvorwaller/code/conversations-worktree-implement/.sidecar-prompt"
claude --dangerously-skip-permissions "$PROMPT"
rm -f "/Users/marcusvorwaller/code/conversations-worktree-implement/.sidecar-start.sh"
