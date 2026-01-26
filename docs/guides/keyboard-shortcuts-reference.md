# Keyboard Shortcuts Reference

Quick reference for all keyboard shortcuts in Sidecar.

## Global Navigation

| Key | Action |
|-----|--------|
| `1-5` | Focus plugin 1-5 |
| `` ` `` | Next plugin |
| `~` | Previous plugin |
| `?` | Command palette |
| `!` | Toggle diagnostics |
| `@` | Project switcher |
| `q` | Quit (in root views) / Back (in subviews) |
| `ctrl+c` | Force quit |

## Universal List Navigation

These work in most list views:

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `g g` | Go to top |
| `G` | Go to bottom |
| `ctrl+d` | Page down |
| `ctrl+u` | Page up |
| `enter` | Select/open |

## Unified Sidebar Controls

All plugins with two-pane layouts share these:

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch focus between sidebar and main pane |
| `\` | Toggle sidebar visibility |
| `h` / `left` | Focus sidebar |
| `l` / `right` | Focus main pane |

---

## Conversations Plugin

### Session List

| Key | Action |
|-----|--------|
| `/` | Search sessions |
| `f` | Filter by adapter |
| `F` | Clear filter |
| `y` | Copy session details |
| `Y` | Copy resume command |
| `R` | Resume in workspace (opens modal to choose Shell or Worktree) |
| `r` | Refresh sessions |
| `D` | Delete session |

### Messages View

| Key | Action |
|-----|--------|
| `y` | Copy session details |
| `Y` | Copy resume command |
| `R` | Resume in workspace |

---

## Git Plugin

### File List (git-status)

| Key | Action |
|-----|--------|
| `s` | Stage file |
| `u` | Unstage file |
| `S` | Stage all |
| `U` | Unstage all |
| `c` | Commit (requires staged files) |
| `A` | Amend last commit |
| `d` / `enter` | View diff |
| `D` | Discard changes |
| `h` | Show history |
| `P` | Push menu |
| `L` | Pull menu |
| `f` | Fetch |
| `b` | Branch operations |
| `z` | Stash changes |
| `Z` | Pop stash |
| `o` | Open in GitHub |
| `O` | Open in file browser |
| `y` | Copy file info |
| `Y` | Copy file path |
| `r` | Refresh |

### Commit List

| Key | Action |
|-----|--------|
| `enter` / `d` | View commit details |
| `h` | Open history view |
| `y` | Copy commit as markdown |
| `Y` | Copy commit hash |
| `/` | Search commits |
| `f` | Filter by author |
| `p` | Filter by path |
| `F` | Clear filters |
| `n` | Next search match |
| `N` | Previous search match |
| `o` | Open commit in GitHub |
| `v` | Toggle commit graph |

### Pull Menu

| Key | Action |
|-----|--------|
| `p` | Pull with merge |
| `r` | Pull with rebase |
| `f` | Pull fast-forward only |
| `a` | Pull rebase + autostash |
| `Esc` | Cancel |

### Pull Conflict

| Key | Action |
|-----|--------|
| `a` | Abort merge/rebase |
| `Esc` | Dismiss, resolve manually |

---

## File Browser Plugin

### Tree View

| Key | Action |
|-----|--------|
| `/` | Filter files by name |
| `ctrl+p` | Quick open (fuzzy finder) |
| `ctrl+s` | Project-wide search (ripgrep) |
| `a` | Create file |
| `A` | Create directory |
| `d` | Delete (with confirmation) |
| `R` | Rename |
| `m` | Move |
| `y` | Copy to clipboard |
| `p` | Paste from clipboard |
| `s` | Cycle sort mode |
| `r` | Refresh |
| `t` | Open in new tab |
| `[` | Previous tab |
| `]` | Next tab |
| `x` | Close tab |
| `ctrl+r` | Reveal in system file manager |

---

## Workspaces Plugin

### Workspace List

| Key | Action |
|-----|--------|
| `n` | Create new workspace |
| `v` | Toggle list/kanban view |
| `r` | Refresh |
| `D` | Delete workspace |
| `p` | Push branch |
| `m` | Start merge workflow |
| `T` | Link/unlink task |
| `s` | Start agent |
| `S` | Stop agent |
| `enter` | Enter interactive mode |
| `t` | Attach to tmux session |
| `y` | Approve agent prompt |
| `N` | Reject agent prompt |
| `[` | Previous preview tab |
| `]` | Next preview tab |

### Interactive Mode

| Key | Action |
|-----|--------|
| `ctrl+\` | Exit interactive mode |
| `ctrl+]` | Attach (full-screen tmux) |
| `alt+c` | Copy selection |
| `alt+v` | Paste |

---

## TD Monitor Plugin

TD shortcuts are dynamically loaded from TD itself. Common shortcuts:

| Key | Action |
|-----|--------|
| `n` | New issue |
| `enter` | Open details |
| `S` | Statistics |
| `/` | Search |
| `e` | Edit |
| `c` | Complete |
| `d` | Delete |

---

## Project Switcher Modal

| Key | Action |
|-----|--------|
| `@` | Open/close |
| `↓` / `ctrl+n` | Next project |
| `↑` / `ctrl+p` | Previous project |
| `enter` | Switch to selected |
| `Esc` | Close |

---

## Command Palette (?)

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate |
| `ctrl+d` / `ctrl+u` | Page down/up |
| `enter` | Execute command |
| `tab` | Toggle current/all contexts |
| `Esc` | Close |
