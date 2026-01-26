# Plan: Agent Selection for Shell Creation

## Goal
Allow users to optionally start an AI agent when creating a new shell, using the same agent list and skip-permissions option as worktree creation. **Critical constraint:** No extra clicks/keystrokes for users who just want a plain shell.

## Current Flow (Shell Creation)
```
Press 'n' → Type selector (Shell/Workspace) → Select Shell → Name input → Enter → Shell created
```
- ~3 keystrokes for a named shell, ~2 for unnamed
- No agent involvement

## Proposed Flow
```
Press 'n' → Type selector → Select Shell → Name input [Enter = done]
                                         → Tab → Agent list (default: None) [Enter = done]
                                                → Tab → Skip perms checkbox [Enter = done]
```

**Key insight:** Enter on the name field creates the shell immediately (same as today). Only users who Tab forward see agent selection. Default agent is "None", so even if they Tab through and press Enter, they get a plain shell.

## Design Decisions

### 1. Zero Extra Steps for Plain Shells
- **Enter on name field** = immediate shell creation (current behavior preserved)
- Agent field only reached via Tab
- Default agent = "None"

### 2. Agent List Reuse
Use same `AgentTypeOrder` from worktree creation:
- Claude Code
- Codex CLI
- Gemini CLI
- Cursor Agent
- OpenCode
- **None (default, first in list for shells)**

### 3. Skip Permissions
- Checkbox appears when agent with skip-perm flag is selected
- Same flags as worktree: `--dangerously-skip-permissions`, `--yolo`, `-f`, etc.

### 4. Shell + Agent Session Management
- Shells with agents use same prefix: `sidecar-sh-{project}-{index}`
- Agent started in project root (not worktree, since shells don't have one)
- Agent type stored in `ShellSession.Agent.Type` for reconnection

## Implementation Steps

### Step 1: Extend Type Selector Modal State
**File:** `internal/plugins/workspace/plugin.go`

Add fields for shell agent selection:
```go
// Type selector modal - shell agent selection
typeSelectorAgentIdx      int
typeSelectorAgentType     AgentType
typeSelectorSkipPerms     bool
typeSelectorFocusField    int  // 0=name, 1=agent, 2=skipPerms, 3=buttons
```

### Step 2: Update Type Selector Rendering
**File:** `internal/plugins/workspace/view_modals.go` (around line 1117)

When shell is selected in type selector:
1. Show name input (existing)
2. Show agent list (new) - styled like worktree create modal
3. Show skip perms checkbox if applicable (new)
4. Show Create/Cancel buttons

Layout:
```
┌─ Create ─────────────────────────┐
│ ○ Shell   ● Workspace            │
│                                  │
│ Name: [my-shell________]         │
│                                  │
│ Agent:                           │
│   ▸ None                         │
│     Claude Code                  │
│     Codex CLI                    │
│     ...                          │
│                                  │
│ ☐ Skip permissions               │
│                                  │
│      [ Create ]  [ Cancel ]      │
└──────────────────────────────────┘
```

### Step 3: Update Type Selector Key Handling
**File:** `internal/plugins/workspace/keys.go`

Modify `handleTypeSelectorKeys()`:

**Current logic (Enter on name):**
```go
if p.typeSelectorIdx == 0 { // Shell selected
    name := p.typeSelectorNameInput.Value()
    p.clearTypeSelectorModal()
    return p.createNewShell(name)
}
```

**New logic:**
```go
if p.typeSelectorIdx == 0 { // Shell selected
    switch p.typeSelectorFocusField {
    case 0: // Name field - Enter creates shell immediately (no extra steps!)
        return p.createShellWithAgent()
    case 1: // Agent list - Enter confirms agent, stays in modal
        p.typeSelectorFocusField = 2 // move to skip perms or buttons
    case 2: // Skip perms or buttons
        if onSkipPerms {
            p.typeSelectorFocusField = 3
        } else {
            return p.createShellWithAgent()
        }
    case 3: // Buttons
        return p.createShellWithAgent()
    }
}
```

Add Tab/Shift+Tab navigation between fields:
- Tab: name → agent → skipPerms (if visible) → buttons → name
- Shift+Tab: reverse

Add j/k for agent list navigation when focused.

### Step 4: Create Shell with Agent Function
**File:** `internal/plugins/workspace/shell.go`

New function `createShellWithAgent()`:
```go
func (p *Plugin) createShellWithAgent() (tea.Model, tea.Cmd) {
    name := p.typeSelectorNameInput.Value()
    agentType := p.typeSelectorAgentType
    skipPerms := p.typeSelectorSkipPerms

    p.clearTypeSelectorModal()

    // Create shell session
    cmd := p.createNewShell(name)

    // If agent selected (not None), start it after shell creation
    if agentType != AgentNone && agentType != "" {
        // Return batch: create shell, then start agent in that shell
        return p, tea.Batch(cmd, p.startAgentInShellCmd(name, agentType, skipPerms))
    }

    return p, cmd
}
```

### Step 5: Start Agent in Shell
**File:** `internal/plugins/workspace/agent.go`

New function `startAgentInShell()`:
```go
func (p *Plugin) startAgentInShell(shell *ShellSession, agentType AgentType, skipPerms bool) tea.Cmd {
    // Build agent command (reuse existing buildAgentCommand logic)
    baseCmd := AgentCommands[agentType]
    if skipPerms {
        if flag := SkipPermissionsFlags[agentType]; flag != "" {
            baseCmd += " " + flag
        }
    }

    // Send to existing tmux session
    cmd := exec.Command("tmux", "send-keys", "-t", shell.TmuxName, baseCmd, "Enter")

    return func() tea.Msg {
        if err := cmd.Run(); err != nil {
            return ShellAgentErrorMsg{Shell: shell, Err: err}
        }
        return ShellAgentStartedMsg{Shell: shell, AgentType: agentType}
    }
}
```

### Step 6: Update ShellSession Struct
**File:** `internal/plugins/workspace/types.go`

Add agent tracking to ShellSession:
```go
type ShellSession struct {
    Name         string
    TmuxName     string
    Agent        Agent
    ChosenAgent  AgentType  // NEW: track which agent was selected
    SkipPerms    bool       // NEW: track skip permissions setting
}
```

### Step 7: Handle Shell Agent Messages
**File:** `internal/plugins/workspace/update.go`

Add handlers:
```go
case ShellAgentStartedMsg:
    for i, shell := range p.shells {
        if shell.TmuxName == msg.Shell.TmuxName {
            p.shells[i].ChosenAgent = msg.AgentType
            p.shells[i].Agent.Type = msg.AgentType
            break
        }
    }
    return p, nil

case ShellAgentErrorMsg:
    p.setError(fmt.Sprintf("Failed to start agent: %v", msg.Err))
    return p, nil
```

### Step 8: Update Shell Status Detection
**File:** `internal/plugins/workspace/shell.go`

Modify `detectShellStatus()` to handle agent-specific status patterns (waiting, done, error) similar to worktree agent detection.

### Step 9: Visual Indicators
Update shell list rendering to show:
- Agent type icon/name when agent is running
- Status indicator (Active/Waiting/Done) like worktrees
- Skip perms indicator if enabled

## Testing Checklist

### No Regression Tests
- [ ] Press `n` → Shell → Enter on empty name → unnamed shell created (2 keystrokes same as before)
- [ ] Press `n` → Shell → type name → Enter → named shell created (same as before)
- [ ] Press `A` → shell created immediately (same as before)

### New Feature Tests
- [ ] Press `n` → Shell → Tab → select Claude → Enter → shell with Claude starts
- [ ] Press `n` → Shell → Tab → select Claude → Tab → toggle skip perms → Enter → shell with Claude + skip perms
- [ ] Shell with agent shows correct status (Active/Waiting/Done)
- [ ] Reconnect to shell with agent after restart preserves agent type
- [ ] Mouse: click agent in list selects it
- [ ] Mouse: click skip perms checkbox toggles it

### Edge Cases
- [ ] Agent CLI not installed → graceful error
- [ ] Shell creation fails → no agent start attempted
- [ ] Agent start fails → shell still usable

## Files Modified

| File | Changes |
|------|---------|
| `internal/plugins/workspace/plugin.go` | Add state fields for shell agent selection |
| `internal/plugins/workspace/view_modals.go` | Extend type selector modal rendering |
| `internal/plugins/workspace/keys.go` | Add Tab/j/k navigation, modify Enter behavior |
| `internal/plugins/workspace/shell.go` | New `createShellWithAgent()`, update status detection |
| `internal/plugins/workspace/agent.go` | New `startAgentInShell()` |
| `internal/plugins/workspace/types.go` | Add fields to ShellSession |
| `internal/plugins/workspace/update.go` | Handle ShellAgentStartedMsg |

## Complexity Estimate

- **State management:** Low - reuses existing patterns
- **UI changes:** Medium - extends type selector modal
- **Agent integration:** Low - reuses existing agent start logic
- **Risk:** Low - preserves existing behavior, adds opt-in feature

## Alternative Considered: Separate Type

Could add "Shell with Agent" as third option in type selector. Rejected because:
1. Makes type selector busier
2. Less discoverable (user might not notice the option)
3. Duplicates UI (two "shell" options)

The Tab-to-reveal approach is cleaner and matches how advanced options work in the worktree modal.
