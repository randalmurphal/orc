# CLI Package

Command-line interface using Cobra. Each command is in its own file.

## File Structure

### Core Files

| File | Purpose |
|------|---------|
| `root.go` | Root command, global flags, initialization |
| `commands.go` | Command registration, helper utilities |
| `serve.go` | API server command |
| `session.go` | Interactive session handling |
| `signals.go` | Signal handling (SIGINT, SIGTERM) |
| `template.go` | CLI output templates |
| `errors.go` | CLI error types and formatting |

### Command Files (27 total)

| File | Command | Description |
|------|---------|-------------|
| `cmd_go.go` | `orc go` | Main entry point - interactive, headless, or quick mode |
| `cmd_init.go` | `orc init` | Initialize .orc/ in project (instant, <500ms) |
| `cmd_setup.go` | `orc setup` | Claude-powered interactive setup |
| `cmd_new.go` | `orc new "title"` | Create new task |
| `cmd_list.go` | `orc list` | List all tasks |
| `cmd_show.go` | `orc show TASK-ID` | Show task details |
| `cmd_edit.go` | `orc edit TASK-ID` | Edit task properties (title, description, weight, dependencies) |
| `cmd_run.go` | `orc run TASK-ID` | Execute task phases |
| `cmd_pause.go` | `orc pause TASK-ID` | Pause running task |
| `cmd_resume.go` | `orc resume TASK-ID` | Resume paused/blocked/failed task |
| `cmd_rewind.go` | `orc rewind TASK-ID --to PHASE` | Reset to before phase |
| `cmd_reset.go` | `orc reset TASK-ID` | Reset task to initial state for retry |
| `cmd_resolve.go` | `orc resolve TASK-ID` | Mark failed task as resolved without re-running |
| `cmd_status.go` | `orc status` | Show task status (with BLOCKED/READY sections) |
| `cmd_deps.go` | `orc deps [TASK-ID]` | Show task dependencies (tree/graph views) |
| `cmd_log.go` | `orc log TASK-ID` | Show task transcripts |
| `cmd_diff.go` | `orc diff TASK-ID` | Show task changes |
| `cmd_delete.go` | `orc delete TASK-ID` | Delete task and files |
| `cmd_approve.go` | `orc approve TASK-ID` | Approve pending gate |
| `cmd_config.go` | `orc config [key] [value]` | Get/set configuration |
| `cmd_pool.go` | `orc pool [subcommand]` | Manage OAuth token pool |
| `cmd_skip.go` | `orc skip TASK-ID` | Skip current phase |
| `cmd_cleanup.go` | `orc cleanup` | Clean up stale worktrees |
| `cmd_version.go` | `orc version` | Show version info |
| `cmd_projects.go` | `orc projects` | List registered projects |
| `cmd_export.go` | `orc export TASK-ID` | Export task to YAML (with plan, state, transcripts) |
| `cmd_export.go` | `orc import <file>` | Import task from YAML file |
| `cmd_initiative.go` | `orc initiative [subcommand]` | Manage initiatives (grouped tasks) |
| `cmd_knowledge.go` | `orc knowledge [subcommand]` | Manage project knowledge |
| `cmd_comment.go` | `orc comment [subcommand]` | Manage task comments and notes |

## Command Structure

Each command file follows this pattern:

```go
// cmd_example.go
package cli

import (
    "github.com/spf13/cobra"
)

func init() {
    rootCmd.AddCommand(exampleCmd)
}

var exampleCmd = &cobra.Command{
    Use:   "example [args]",
    Short: "Brief description",
    Long:  `Detailed description with examples.`,
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}
```

## Key Commands

### `orc new`
Creates task with AI-powered weight classification:
1. Prompts for task description
2. Calls Claude to classify weight (trivial/small/medium/large/greenfield)
3. Generates plan based on weight template
4. Creates task files in `.orc/tasks/TASK-XXX/`

| Flag | Description |
|------|-------------|
| `--weight, -w` | Override AI classification (trivial/small/medium/large/greenfield) |
| `--category, -c` | Set task category (feature/bug/refactor/chore/docs/test, default: feature) |
| `--description, -d` | Task description |
| `--template, -t` | Use template (bugfix, feature, refactor, migration, spike) |

### `orc run`
Executes task phases:
1. Sets up git worktree (if enabled)
2. Loads task and plan
3. For each phase:
   - Evaluates gate conditions
   - Executes phase (via executor)
   - Creates checkpoint commit
4. Creates PR or merges (based on config)

### `orc resume`
Resumes paused, blocked, interrupted, orphaned, or failed tasks:
1. Loads task state
2. For failed tasks: allows retry after fixing external issues
3. For orphaned tasks: detects dead executor and marks as interrupted
4. Identifies last incomplete phase
5. Continues execution from that phase

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-essential output |
| `--profile, -p` | Automation profile (auto/fast/safe/strict) |
| `--plain` | Disable emoji/unicode for terminal compatibility |

## Command Aliases

| Alias | Command | Description |
|-------|---------|-------------|
| `ls` | `list` | List all tasks |
| `st` | `status` | Show orc status |
| `rm` | `delete` | Delete a task |
| `remove` | `delete` | Delete a task |

## Enhanced Commands

### `orc log`
Show task transcripts with content (not just file listing).

| Flag | Description |
|------|-------------|
| `--list, -l` | List transcript files only (no content) |
| `--phase, -p` | Show specific phase transcript |
| `--all, -a` | Show all transcripts |
| `--tail, -n` | Show last N lines (default: 100, 0 for all) |
| `--follow, -f` | Stream new lines in real-time using fsnotify |

**Real-time streaming implementation** (`--follow`):
- Uses `fsnotify` for filesystem-level notifications (instant updates)
- Falls back to polling (100ms) if fsnotify fails
- Watches directory (not file) for more reliable events
- Handles file truncation by detecting size decrease and resetting
- Buffers partial lines until newline received
- Clean shutdown on SIGINT/SIGTERM (prints partial line before exit)

### `orc status`
Priority-based status display with sections: Orphaned → Attention Needed → Running → Blocked → Ready → Paused → Recent.

| Flag | Description |
|------|-------------|
| `--all, -a` | Show all tasks including completed |
| `--watch, -w` | Refresh status every 5 seconds |

**Dependency-aware sections:**
- **BLOCKED** - Tasks waiting on incomplete dependencies (shows which tasks they're blocked by)
- **READY** - Tasks with no dependencies or all dependencies completed (sorted by priority)

### `orc deps`
Show task dependencies with multiple view options.

| Flag | Description |
|------|-------------|
| `--tree` | Show full dependency tree recursively |
| `--graph` | Show ASCII dependency graph |
| `--initiative, -i` | Filter graph by initiative ID |

**Views:**
- Default (single task): Shows blocked_by, blocks, related_to, referenced_by
- `--tree`: Recursive tree showing full dependency chain
- `--graph`: ASCII graph showing task flow, single chains collapsed inline
- No args: Overview showing blocking/blocked/independent task counts

### `orc rewind`
Reset task to before a specific phase.

| Flag | Description |
|------|-------------|
| `--to` | Phase to rewind to (required) |
| `--force, -f` | Skip confirmation (for automation) |

### `orc reset`
Reset task to initial state (planned), clearing all execution progress.

| Flag | Description |
|------|-------------|
| `--force, -f` | Skip confirmation and safety checks |

Unlike `rewind` (which goes to a specific checkpoint), `reset` clears everything for a complete fresh start.

### `orc resolve`
Mark a failed task as resolved/completed without re-running.

| Flag | Description |
|------|-------------|
| `--message, -m` | Resolution message explaining why task was resolved |
| `--force, -f` | Skip confirmation prompt |

Unlike `reset` (which clears for retry), `resolve` closes the task while preserving execution state. Only works on failed tasks.

### `orc stop`
Permanently stop task and mark as failed (unlike `pause` which allows resume).

| Flag | Description |
|------|-------------|
| `--force, -f` | Skip confirmation prompt |

### `orc initiative run`
Run tasks from an initiative.

| Flag | Description |
|------|-------------|
| `--execute` | Actually run tasks (default: preview only) |
| `--parallel` | Run ready tasks in parallel |
| `--profile` | Override automation profile |

### `orc config docs`
Show searchable configuration documentation.

| Flag | Description |
|------|-------------|
| `--category, -c` | Filter by category |
| `--search, -s` | Search config options |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Task not found |
| 4 | Gate blocked |
| 5 | Execution failed |

## Testing

```bash
# Run CLI tests
go test ./internal/cli/... -v

# Test specific command
go test ./internal/cli/... -run TestInitCmd -v
```

## Adding a New Command

1. Create `cmd_mycommand.go`
2. Define command with `&cobra.Command{}`
3. Register in `init()` with `rootCmd.AddCommand()`
4. Add tests in `cmd_mycommand_test.go`
