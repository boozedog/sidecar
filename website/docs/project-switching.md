---
sidebar_position: 6
title: Project Switching
---

# Project Switching

Switch back and forth between projects instantly and seamlessly.

## Quick Start

1. Add projects to `~/.config/sidecar/config.json`:

```json
{
  "projects": {
    "list": [
      {"name": "sidecar", "path": "~/code/sidecar"},
      {"name": "frontend", "path": "~/code/frontend"},
      {"name": "backend", "path": "~/work/backend"}
    ]
  }
}
```

2. Press `@` to open the project switcher
3. Select a project and press `Enter`

## What Gets Preserved

When you switch projects, sidecar saves and restores your context per-project:

| State | Description |
|-------|-------------|
| Active plugin | Which plugin tab was focused |
| Cursor position | Selected item in file browser, git status, etc. |
| Expanded directories | File browser tree state |
| Sidebar widths | Panel sizing preferences |
| Last worktree | If using worktrees, restores your last-active one |

This means you can jump between projects and pick up exactly where you left off.

## Keyboard Shortcuts

### Opening the Switcher

| Key | Action |
|-----|--------|
| `@` | Open/close project switcher |
| `W` | Open worktree switcher (within current repo) |

### Navigation

| Key | Action |
|-----|--------|
| `j` / `ctrl+n` | Move cursor down |
| `k` / `ctrl+p` | Move cursor up |
| `Enter` | Switch to selected project |
| `Esc` | Close without switching |

Type to filter the project list. The current project is highlighted in green.

## Mouse Support

- **Click** on a project to switch to it
- **Scroll** to navigate the list
- **Click outside** the modal to close it

## Configuration

### Config Location

`~/.config/sidecar/config.json`

### Project Entry Format

```json
{
  "projects": {
    "list": [
      {
        "name": "display-name",
        "path": "/absolute/path/to/repo"
      }
    ]
  }
}
```

### Path Expansion

Paths support `~` expansion:
- `~/code/myapp` expands to `/Users/you/code/myapp`

### Per-Project Themes

Projects can have individual themes:

```json
{
  "projects": {
    "list": [
      {"name": "work", "path": "~/work/main", "theme": "dark"},
      {"name": "personal", "path": "~/code/personal", "theme": "monokai"}
    ]
  }
}
```

## Project vs Worktree Switching

Sidecar supports two types of switching:

| Feature | Project Switching (`@`) | Worktree Switching (`W`) |
|---------|------------------------|-------------------------|
| Use case | Jump between different repos | Jump between branches in same repo |
| Setup | Manual config in `config.json` | Auto-discovered from git |
| Scope | Any directory | Git worktrees only |

Use project switching for different codebases. Use worktree switching for parallel branches within the same repo.

## What Happens on Switch

When you switch projects:

1. All plugins stop (file watchers, git commands, etc.)
2. Plugin context updates to new working directory
3. All plugins reinitialize with new path
4. Your previously active plugin for that project is restored
5. A toast notification confirms the switch

## Session Isolation

Each sidecar instance maintains its own project state:

- Switching projects in one terminal doesn't affect others
- Each session tracks its own active plugin per project
- State is persisted per working directory

## Troubleshooting

### "No projects configured" message

Add projects to your config file as shown in Quick Start.

### Project path doesn't exist

The switcher shows the project but switching will fail. Verify paths:

```bash
ls ~/code/myproject
```

### Current project not highlighted

The current project shows in green with "(current)" label. If not highlighted:
- Check that the path in config exactly matches the current working directory
- Paths are compared after `~` expansion
