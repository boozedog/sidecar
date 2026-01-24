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
Do a detailed code review of td-16f45e. Focus on correctness and tests.
SIDECAR_PROMPT_EOF
)"
rm -f "/Users/marcusvorwaller/code/sidecar-td-16f45e-prompt-picker-press-'d'-to-install-defau/.sidecar-start.sh"
