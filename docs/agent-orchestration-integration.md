# Native Agent Orchestration in Sidecar

Design document for integrating zeroshot-style agent orchestration natively into sidecar, backed by td as the task engine.

## Problem Statement

Zeroshot provides a powerful plan-build-review cycle for autonomous code generation, but has several design limitations:

1. **Opaque execution** - Agent work happens in subprocesses with limited visibility into what's actually happening. The user watches terminal output scroll by without structured insight into progress, decisions, or blockers.

2. **Over-prescriptive prompts** - Agent instructions are 500+ lines of rigid rules ("NEVER use git diff", "YOU CANNOT ASK QUESTIONS") that constrain model reasoning. Recent work compressed these 3x, acknowledging the problem.

3. **Model selection as core complexity** - The 4-tier complexity classifier (TRIVIAL/SIMPLE/STANDARD/CRITICAL) mapped to 3 model levels (level1/level2/level3) across 4 providers adds configuration surface area that most users don't need. Useful in principle, but over-engineered as a first-class abstraction.

4. **Separate tool** - Runs outside the developer's primary workflow. No integration with the file browser, git status, or task management that sidecar already provides.

## What Zeroshot Gets Right

These are the strengths worth preserving:

### Plan-Build-Review Cycle
The core loop of planning work, implementing it, and independently validating it produces measurably better code than single-agent runs. Context degradation in long sessions is real - isolated agents checking each other's work is the right architecture.

### Blind Validation
Validators never see the implementer's context or justifications. They evaluate work on merit alone. This prevents the "rubber stamp" problem where reviewers unconsciously accept work because the reasoning sounds plausible.

### Task-Driven Execution
Every run starts from a task with acceptance criteria. The system knows what "done" means before it starts working. This maps directly to td's issue model.

### State Persistence & Recovery
All events log to SQLite. Runs can resume after crashes. Every decision is auditable. This is the same philosophy as td's action log and session history.

### Rejection Loop
When validators reject work, findings route back to the implementer with specific, actionable feedback. The loop continues until consensus or explicit failure. This is fundamentally different from "run once and hope."

### Workspace Isolation
Git worktrees or Docker containers prevent agent work from contaminating the main branch. Sidecar's workspace plugin already manages worktrees - this is a natural integration point.

## What We'd Change

### Simpler Prompt Architecture
Instead of 500-line prescriptive instructions per agent, use minimal role descriptions and let the model's native capabilities drive behavior. The prompt should describe *what* the agent needs to accomplish, not micromanage *how*.

**Zeroshot approach** (prescriptive):
```
YOU CANNOT ASK QUESTIONS. This is a non-interactive environment.
NEVER use git diff. NEVER use git status. NEVER modify files outside the workspace.
Output MUST be valid JSON matching this exact schema...
Maximum informativeness, minimum verbosity. NO EXCEPTIONS.
```

**Proposed approach** (outcome-oriented):
```
You are implementing changes for task td-a1b2: "Add rate limiting to API endpoints"

Acceptance criteria:
- Sliding window algorithm, per-IP
- Return 429 when exceeded
- Configurable limits per route

Work in the worktree at /path/to/worktree. Commit when done.
```

The model knows how to write code. Tell it what to build, not how to think.

### Provider-Agnostic by Default
Instead of a complex provider abstraction with level mappings and per-provider settings, start with a single configured provider (the user's preferred CLI agent) and add multi-provider support later if needed. Most users use one provider.

### Transparent Execution
Sidecar's TUI can show agent work in real-time: which files are being modified, what the plan is, validation progress, rejection reasons. This is the primary advantage of native integration - the orchestration isn't a black box.

### td as the Native Task Engine
Zeroshot has its own SQLite ledger for state. With native integration, td *is* the state store. Tasks, logs, handoffs, sessions, and reviews all use td's existing infrastructure. No parallel state system.

## Architecture

### Component Overview

```
                    ┌─────────────────────────────────┐
                    │          Sidecar TUI             │
                    │                                  │
                    │  ┌───────┐ ┌──────┐ ┌────────┐  │
                    │  │Git    │ │Files │ │TD      │  │
                    │  │Status │ │      │ │Monitor │  │
                    │  └───────┘ └──────┘ └────────┘  │
                    │  ┌──────────────────────────┐   │
                    │  │   Agent Orchestrator      │   │
                    │  │   Plugin (new)            │   │
                    │  └────────────┬─────────────┘   │
                    └───────────────┼──────────────────┘
                                    │
                    ┌───────────────┼──────────────────┐
                    │               ▼                   │
                    │     Orchestration Engine          │
                    │     (internal/orchestrator/)      │
                    │                                   │
                    │  ┌─────────┐  ┌──────────────┐   │
                    │  │Planner  │  │Implementer   │   │
                    │  │Agent    │  │Agent         │   │
                    │  └────┬────┘  └──────┬───────┘   │
                    │       │              │            │
                    │  ┌────▼──────────────▼───────┐   │
                    │  │   Validator Agents         │   │
                    │  │   (1-N, blind, parallel)   │   │
                    │  └───────────────────────────┘   │
                    │                                   │
                    │  ┌───────────────────────────┐   │
                    │  │   Agent Runner             │   │
                    │  │   (shells out to CLI)      │   │
                    │  └───────────────────────────┘   │
                    └───────────────────────────────────┘
                                    │
                    ┌───────────────┼──────────────────┐
                    │               ▼                   │
                    │      td (task engine)             │
                    │  tasks, logs, handoffs, sessions  │
                    └──────────────────────────────────┘
```

### Core Packages

#### `internal/orchestrator/`

The orchestration engine, independent of the TUI. Can be tested and run standalone.

```
internal/orchestrator/
  engine.go          # Core lifecycle: plan → build → validate → iterate
  agent.go           # Agent abstraction (role, prompt builder, runner)
  runner.go          # Shells out to CLI agents (claude, codex, gemini)
  planner.go         # Planning phase logic
  validator.go       # Validation phase logic (blind, parallel)
  workspace.go       # Worktree/isolation management
  events.go          # Event types emitted during orchestration
  config.go          # Orchestration settings
```

#### `internal/plugins/orchestrator/`

The sidecar plugin that provides the TUI for orchestration.

```
internal/plugins/orchestrator/
  plugin.go          # Plugin interface implementation
  view.go            # Rendering (plan view, progress, validation results)
  handlers.go        # Key/mouse input handling
  commands.go        # Plugin commands for footer hints
```

### Orchestration Engine Design

#### Engine Lifecycle

```go
type Engine struct {
    taskID     string            // td task ID
    workspace  *Workspace        // git worktree or direct
    runner     AgentRunner       // CLI agent executor
    events     chan Event        // progress events for TUI
    config     *Config           // orchestration settings
}

type Config struct {
    Provider       string   // "claude", "codex", "gemini"
    MaxIterations  int      // rejection loop limit (default: 3)
    ValidatorCount int      // number of validators (default: 2)
    Workspace      string   // "worktree" (default), "direct", "docker"
    AutoMerge      bool     // auto-merge worktree on success
}
```

#### Phases

**Phase 1: Plan**

Read the td task (title, description, acceptance criteria, logs, handoffs from prior sessions). Build a plan prompt and send to the agent. The agent returns a structured plan: files to modify, approach summary, risks.

```go
type Plan struct {
    Summary     string
    Steps       []string
    FilesTouch  []string
    Risks       []string
    Accepted    bool      // user can review before proceeding
}
```

The plan is logged to td (`td log --decision "Plan: ..."`) and displayed in the TUI. The user can accept, modify, or reject before implementation begins.

**Phase 2: Implement**

The implementer agent works in an isolated worktree. It receives:
- The task description and acceptance criteria
- The accepted plan
- The worktree path

It does not receive: validator instructions, previous rejection details from other tasks, or prescriptive coding rules.

Progress events stream to the TUI:
- Files being modified
- Commits made
- Agent thinking/reasoning (if available from provider)

Implementation is logged to td: `td log "Implemented OAuth callback handler"`.

**Phase 3: Validate**

N validator agents run in parallel. Each receives:
- The task description and acceptance criteria
- The diff (worktree vs base branch)
- Nothing else (blind validation)

Each validator independently assesses:
- Does the implementation satisfy acceptance criteria?
- Are there bugs, security issues, or missing edge cases?
- Do tests pass? (if the validator can run them)

Validators return structured results:

```go
type ValidationResult struct {
    Approved bool
    Findings []Finding
}

type Finding struct {
    Severity string  // "error", "warning", "info"
    File     string
    Line     int
    Message  string
}
```

**Phase 4: Iterate or Complete**

If all validators approve: mark complete, optionally merge worktree, update td task status.

If any validator rejects: route findings back to implementer with specific file/line feedback. The implementer gets a fresh context with just: task, plan, current code, and validator findings. Loop back to Phase 3.

After `MaxIterations` rejections: stop, report failure, log to td with details.

#### Agent Runner

The runner shells out to CLI agents, similar to zeroshot's approach but simpler:

```go
type AgentRunner interface {
    Run(ctx context.Context, prompt string, workDir string) (*AgentResult, error)
    Stream(ctx context.Context, prompt string, workDir string) (<-chan AgentEvent, error)
}

// ClaudeRunner implements AgentRunner using claude CLI
type ClaudeRunner struct {
    binary string  // path to claude binary
}

// CodexRunner implements AgentRunner using codex CLI
type CodexRunner struct {
    binary string
}
```

Each runner:
- Spawns the CLI process with the prompt
- Captures stdout/stderr
- Optionally streams events (for real-time TUI updates)
- Returns structured output or raw text

No model level abstraction. The user configures their CLI agent with whatever model they want. The orchestrator doesn't care.

#### Event System

The engine emits events consumed by the TUI plugin:

```go
type Event struct {
    Type      EventType
    Timestamp time.Time
    Data      interface{}
}

type EventType int
const (
    EventPlanStarted EventType = iota
    EventPlanReady
    EventImplementationStarted
    EventFileModified
    EventImplementationDone
    EventValidationStarted
    EventValidatorResult
    EventIterationStarted
    EventComplete
    EventFailed
)
```

### TUI Plugin Design

The orchestrator plugin integrates with sidecar's existing plugin system.

#### View Modes

1. **Task Selection** - Pick a td task to work on (or create one)
2. **Plan Review** - See the agent's plan, accept/modify/reject
3. **Implementation Progress** - Watch files being modified, see agent output
4. **Validation Results** - See each validator's findings, approval/rejection
5. **Iteration View** - Show rejection feedback being sent back to implementer
6. **Complete/Failed** - Final status with summary

#### Cross-Plugin Integration

The orchestrator plugin leverages sidecar's existing plugins:

- **Git Status**: Shows real-time diff as agent modifies files in the worktree
- **File Browser**: Navigate to files the agent is changing
- **TD Monitor**: Task status updates automatically as orchestration progresses
- **Workspace**: Worktree creation/management for isolated agent work

Messages between plugins:

```go
// Orchestrator → Git Status
gitstatus.RefreshMsg{}

// Orchestrator → File Browser
filebrowser.NavigateToFileMsg{Path: "src/auth/oauth.go"}

// Orchestrator → TD Monitor (via td CLI)
// td log "Plan accepted: implement OAuth with JWT"
// td start td-123

// Orchestrator → Workspace
workspace.CreateWorktreeMsg{Branch: "agent/td-123-oauth"}
```

#### Keyboard Commands

```
Context: orchestrator-select
  Enter    Start orchestration for selected task
  n        Create new task and start
  /        Search tasks

Context: orchestrator-plan
  Enter    Accept plan
  e        Edit plan (opens in editor)
  r        Regenerate plan
  Esc      Cancel

Context: orchestrator-running
  v        Toggle validator detail view
  d        View diff so far
  f        View modified files
  c        Cancel run
  Tab      Switch to git status plugin (shows live diff)

Context: orchestrator-results
  m        Merge worktree to main
  d        View final diff
  Enter    Accept and close task
  r        Retry with modifications
```

### td Integration Points

The orchestrator uses td throughout the lifecycle:

| Phase | td Commands |
|-------|-------------|
| Start | `td start <id>` - begin work, capture git state |
| Plan | `td log --decision "Plan: ..."` - record plan |
| Implement | `td log "Modified src/auth/oauth.go"` - progress |
| Validate (pass) | `td review <id>` - submit for review |
| Validate (fail) | `td log --blocker "Validator: missing error handling"` |
| Iterate | `td log "Addressing: missing error handling"` |
| Complete | `td approve <id>` (or user approves in td monitor) |
| Failed | `td log --blocker "Failed after 3 iterations"` |
| Handoff | `td handoff <id> --done "..." --remaining "..."` |

Session management: The orchestrator creates a td session for each agent role (planner, implementer, validator-1, validator-2). This preserves td's session isolation - the implementer session cannot approve its own work.

### Configuration

Added to sidecar's `config.json`:

```json
{
  "plugins": {
    "orchestrator": {
      "enabled": true,
      "provider": "claude",
      "maxIterations": 3,
      "validatorCount": 2,
      "workspace": "worktree",
      "autoMerge": false,
      "providerBinary": ""
    }
  }
}
```

Minimal configuration. The provider binary is auto-detected if not specified. Model selection is left to the CLI agent's own configuration.

## Implementation Phases

### Phase 1: Engine Core

Build the orchestration engine as a standalone package (`internal/orchestrator/`). No TUI dependency. Testable with mock runners.

- Engine lifecycle (plan → build → validate → iterate)
- Agent runner interface + Claude runner implementation
- Workspace management (worktree creation, cleanup)
- Event emission
- td integration (start, log, review, approve)
- Unit tests with mock agent runner

### Phase 2: TUI Plugin

Build the sidecar plugin that wraps the engine.

- Plugin boilerplate (ID, Init, Start, Stop, Update, View, Commands)
- Task selection view (reads from td)
- Plan review view
- Implementation progress view (event stream rendering)
- Validation results view
- Cross-plugin messaging (git status refresh, file browser navigation)
- Keyboard commands and footer hints

### Phase 3: Multi-Provider

Add runners for additional CLI agents.

- Codex runner
- Gemini runner
- Provider auto-detection
- Per-task provider override

### Phase 4: Advanced Features

- Docker workspace isolation
- Parallel task orchestration (multiple tasks running simultaneously)
- Custom validator configurations (security-focused, test-focused)
- Orchestration templates (like zeroshot's cluster templates, but simpler)
- Resume interrupted orchestrations

## Design Decisions

### Why shell out to CLI agents instead of API calls?

Same reasoning as zeroshot: CLI agents handle authentication, model selection, tool use, and context management. The orchestrator doesn't need to reimplement any of that. It just needs to give the agent a prompt and a working directory.

### Why not import zeroshot as a library?

Zeroshot is JavaScript/Node.js. Sidecar is Go. More importantly, the integration points are different: zeroshot manages its own state in a separate SQLite ledger, while we want td to be the single source of truth. The orchestration logic itself (plan → build → validate → iterate) is straightforward enough to implement directly.

### Why not a separate process communicating with sidecar?

Adding IPC complexity for something that benefits from tight TUI integration is wrong. The orchestrator needs to emit events that update the view in real-time, trigger cross-plugin navigation, and read td state directly. In-process is simpler and more responsive.

### Why keep blind validation?

It's the single most important architectural decision in zeroshot. When validators see the implementer's reasoning, they unconsciously defer to it. Blind validation catches real bugs that sighted review misses. Worth the extra agent invocations.

### Why not complexity-based model selection?

It adds substantial configuration surface area for marginal benefit. Most users want to use their preferred model for everything. If cost matters, they can configure their CLI agent to use a cheaper model. The orchestrator doesn't need to second-guess the user's model choice.

If demand emerges, this can be added later as an optional feature without changing the core architecture.

## Migration from Zeroshot

Users currently using zeroshot can transition incrementally:

1. Continue using zeroshot standalone for existing workflows
2. Use sidecar's orchestrator for new tasks
3. Both tools use td for task management (zeroshot via its td adapter, sidecar natively)
4. Eventually consolidate on sidecar's orchestrator as it matures

No breaking changes to td or zeroshot required.

## Open Questions

1. **Plan editing UX** - Should the plan be editable in sidecar's inline editor, or should it open an external editor? Inline is more integrated; external is more capable for large edits.

2. **Validator prompt customization** - Should users be able to configure what validators look for (e.g., "focus on security" or "focus on test coverage")? Or is the default "assess against acceptance criteria" sufficient?

3. **Agent output streaming** - Different CLI agents have different streaming capabilities. Claude Code streams; others may not. How much of the real-time progress view depends on streaming?

4. **Worktree naming convention** - `agent/<task-id>-<slug>`? `orchestrator/<task-id>`? Should this match the workspace plugin's conventions?

5. **Failure escalation** - After max iterations, should the orchestrator offer to hand off to the user (open the worktree in their editor) or just report failure?
