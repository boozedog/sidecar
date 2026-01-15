# Creating Prompts

Prompts are reusable templates that configure the initial context for agents when creating worktrees. They help standardize common workflows like code reviews, bug fixes, or feature implementation.

## Configuration Locations

Prompts are defined in YAML or JSON config files:

| Scope   | Path                                  | Override Priority |
|---------|---------------------------------------|-------------------|
| Global  | `~/.config/sidecar/config.yaml`       | Lower             |
| Project | `.sidecar/config.yaml` (in project)   | Higher            |

Project prompts override global prompts with the same name.

## Config File Format

```yaml
# ~/.config/sidecar/config.yaml or .sidecar/config.yaml
prompts:
  - name: "Code Review"
    ticketMode: optional
    body: |
      Do a detailed code review of {{ticket || 'open reviews'}}.
      Focus on correctness, edge cases, and test coverage.

  - name: "Bug Fix"
    ticketMode: required
    body: |
      Fix issue {{ticket}}. Use td to track progress.
      Run tests before marking complete.

  - name: "Setup Project"
    ticketMode: none
    body: |
      Set up the development environment and verify all tests pass.
```

## Prompt Fields

### name (required)
Display name shown in the prompt picker. Keep it concise.

### ticketMode (optional)
Controls how the ticket/task field behaves:

| Mode       | Behavior                                      |
|------------|-----------------------------------------------|
| `optional` | Task field shown, can be empty (default)      |
| `required` | Task must be selected before creating         |
| `none`     | Task field is hidden, prompt stands alone     |

### body (required)
The prompt text sent to the agent. Supports template variables.

## Template Variables

### `{{ticket}}`
Expands to the selected task ID. Returns empty string if no task selected.

```yaml
body: "Fix issue {{ticket}}."
# With task td-abc123: "Fix issue td-abc123."
# Without task: "Fix issue ."
```

### `{{ticket || 'fallback'}}`
Expands to task ID, or the fallback text if no task selected.

```yaml
body: "Review {{ticket || 'all open items'}}."
# With task td-abc123: "Review td-abc123."
# Without task: "Review all open items."
```

## Examples

### Task-Required Workflow
```yaml
- name: "Implement Task"
  ticketMode: required
  body: |
    Begin work on {{ticket}}. Use td to track progress.
    Read the task description carefully before starting.
```

### Standalone Workflow
```yaml
- name: "Run Tests"
  ticketMode: none
  body: |
    Run the full test suite. Fix any failures.
    Report a summary when complete.
```

### Flexible Workflow
```yaml
- name: "Code Review Session"
  ticketMode: optional
  body: |
    Start a review session for {{ticket || 'open reviews'}}.
    Create td tasks for any issues found.
```

## Scope Indicators

In the prompt picker, prompts show their source:
- `[G]` - Global prompt (from `~/.config/sidecar/`)
- `[P]` - Project prompt (from `.sidecar/`)

Project prompts take precedence when names match.

## Tips

1. **Keep names short** - They appear in a table with limited width
2. **Use `ticketMode: none`** for prompts that don't need a specific task
3. **Use fallbacks** (`{{ticket || 'default'}}`) for optional task association
4. **Project prompts** can customize global defaults for specific repos
