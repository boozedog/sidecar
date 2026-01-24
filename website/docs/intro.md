---
sidebar_position: 1
title: Getting Started
---

# Sidecar

**A terminal dashboard for monitoring AI coding agents.**

Watch your agents work in real-time: see git changes, browse session history, track tasks, and manage parallel workspaces—all from a split-screen terminal UI that complements your agent workflow.

![Sidecar Git Status](../../docs/screenshots/sidecar-git.png)

## The Problem

AI coding agents are powerful but opaque. When Claude Code or Cursor makes changes, you're often waiting for a summary or switching contexts to check git status. Sidecar gives you **continuous visibility** into agent activity without interrupting your flow.

**Key capabilities:**

- **Real-time git monitoring** - Stage files, review diffs, commit changes while your agent works
- **Multi-agent support** - Browse session history from Claude Code, Cursor, Gemini CLI, and more
- **Parallel development** - Run multiple agents across git worktrees with live output streaming
- **Task integration** - Connect workspaces to TD tasks for context tracking across sessions
- **Zero context switching** - Everything in your terminal, no editor required

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash
```

**Requirements:** macOS, Linux, or WSL. Go 1.21+ if building from source.

## Quick Start

Run from any project directory:

```bash
sidecar
```

Sidecar auto-detects your git repo and active agent sessions. No configuration needed.

### Recommended Workflow

Split your terminal horizontally: agent on the left, sidecar on the right.

```
┌─────────────────────────────┬─────────────────────┐
│                             │                     │
│   Claude Code / Cursor      │      Sidecar        │
│                             │                     │
│   $ claude                  │   [Git] [Files]     │
│   > fix the auth bug...     │   [TD]  [Workspaces] │
│                             │                     │
└─────────────────────────────┴─────────────────────┘
```

**As the agent works:**

- Watch files appear in Git Status with live diffs
- See tasks progress through workflow stages in TD Monitor
- Browse and edit code yourself in File Browser
- Launch parallel agents in workspaces for multi-branch work

This setup provides full transparency into agent actions without breaking focus.

## Core Plugins

Sidecar's modular architecture provides focused tools for each aspect of your workflow. All plugins auto-refresh and support mouse + keyboard navigation.

### Git Status

**A full-featured git interface with live diff preview and commit management.**

Watch your agent's changes in real-time with syntax-highlighted diffs, stage files with a keypress, and commit without leaving the dashboard. Supports unified and side-by-side diff views, commit history with search, and branch switching.

![Git Status with Diff](../../docs/screenshots/sidecar-git.png)

**Essential shortcuts:**

| Key | Action |
|-----|--------|
| `s` | Stage file or folder |
| `u` | Unstage file |
| `d` | View full-screen diff |
| `v` | Toggle unified/side-by-side diff |
| `c` | Commit staged changes |
| `b` | Switch branches |
| `P` | Push to remote |

[Full Git Plugin documentation →](./git-plugin)

### Workspaces

**Run parallel AI agents across git worktrees with real-time output streaming.**

Create isolated branches, launch agents with custom prompts, and watch their progress in a Kanban board. Each workspace streams agent output live, shows diffs, and links to TD tasks for context. Perfect for multi-branch development or testing multiple approaches simultaneously.

**Essential shortcuts:**

| Key | Action |
|-----|--------|
| `n` | Create new workspace |
| `s` | Start agent in workspace |
| `enter` | Attach to running agent |
| `v` | Toggle list/Kanban view |
| `m` | Start merge workflow |
| `t` | Link TD task |

**Supported agents:** Claude Code, Cursor, Gemini CLI, OpenCode, Codex, Aider

[Full Workspaces Plugin documentation →](./workspaces-plugin)

### Conversations

**Browse session history from all your AI agents with search and token tracking.**

Unified view of sessions across Claude Code, Cursor, Gemini CLI, OpenCode, Codex, and Warp. Search by message content, expand to see full conversations, and track token usage per session. Useful for reviewing what your agents accomplished or resuming previous work.

![Conversations](../../docs/screenshots/sidecar-conversations.png)

**Essential shortcuts:**

| Key | Action |
|-----|--------|
| `/` | Search sessions |
| `enter` | Expand/collapse messages |
| `j/k` | Navigate sessions |

[Full Conversations Plugin documentation →](./conversations-plugin)

### TD Monitor

**Task management for AI agents working across context windows.**

Integration with [TD](https://github.com/marcus/td), a purpose-built task system that helps agents maintain context across sessions. View the current focused task, track activity logs, and submit reviews—all synchronized with your agent's workflow.

![TD Monitor](../../docs/screenshots/sidecar-td.png)

**Essential shortcuts:**

| Key | Action |
|-----|--------|
| `r` | Submit review |
| `enter` | View task details |

[Full TD documentation →](./td)

### File Browser

**Navigate and preview project files with syntax highlighting.**

Collapsible directory tree with live code preview. Browse your codebase while your agent works, open files in your editor, or search by name. Auto-refreshes when files change.

![File Browser](../../docs/screenshots/sidecar-files.png)

**Essential shortcuts:**

| Key | Action |
|-----|--------|
| `enter` | Open/close folder |
| `/` | Search files |
| `h/l` | Switch tree/preview focus |

[Full File Browser documentation →](./files-plugin)

## Global Navigation

These shortcuts work across all plugins:

| Key | Action |
|-----|--------|
| `q`, `ctrl+c` | Quit sidecar |
| `tab` / `shift+tab` | Switch between plugins |
| `1-5` | Jump to plugin by number |
| `j/k`, `↓/↑` | Navigate items in lists |
| `ctrl+d/u` | Page down/up |
| `g` / `G` | Jump to top/bottom |
| `?` | Toggle help overlay |
| `r` | Refresh current plugin |
| `!` | Open diagnostics modal |

Each plugin adds its own context-specific shortcuts shown in the footer bar.

## Themes

Sidecar ships with built-in themes plus a community theme browser with live previews.

- Press `#` to open the theme switcher
- Press `tab` to toggle between built-in and community themes
- Press `enter` to apply the highlighted theme

## Configuration

Sidecar runs with sensible defaults. Create `~/.config/sidecar/config.json` only if you need customization:

```json
{
  "plugins": {
    "git-status": { "enabled": true, "refreshInterval": "1s" },
    "td-monitor": { "enabled": true, "refreshInterval": "2s" },
    "conversations": { "enabled": true },
    "file-browser": { "enabled": true },
    "workspaces": { "enabled": true }
  },
  "ui": {
    "showFooter": true,
    "showClock": true
  }
}
```

**Plugin-specific config:** Workspace prompts support project-level overrides via `.sidecar/config.json`. See [Workspaces documentation](./workspaces-plugin#custom-prompts) for details.

## Command-Line Options

```bash
sidecar                      # Run in current directory
sidecar --project /path      # Specify project root explicitly
sidecar --debug              # Enable debug logging to stdout
sidecar --version            # Print version and exit
```

## Updates

Sidecar checks for new versions on startup and shows a notification when updates are available. Press `!` to view the diagnostics modal with the update command.

**Manual update:**

```bash
curl -fsSL https://raw.githubusercontent.com/marcus/sidecar/main/scripts/setup.sh | bash
```

## What's Next?

- **[Git Plugin](./git-plugin)** - Full reference for staging, diffing, and commits
- **[Workspaces Plugin](./workspaces-plugin)** - Parallel agent setup and management
- **[TD Integration](./td)** - Task tracking across sessions
- **[GitHub Repository](https://github.com/marcus/sidecar)** - Source code and issues

**Build from source:**

```bash
git clone https://github.com/marcus/sidecar.git
cd sidecar
make build
make install
```

Requires Go 1.21+. See the [GitHub README](https://github.com/marcus/sidecar#development) for development setup.
