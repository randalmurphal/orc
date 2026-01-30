# CLI Commands

Full command reference for the orc CLI.

## Command Files

| File | Command | Description |
|------|---------|-------------|
| `cmd_init.go` | `orc init` | Initialize .orc/ in project (<500ms) |
| `cmd_setup.go` | `orc setup` | Claude-powered interactive setup |
| `cmd_new.go` | `orc new "title"` | Create new task |
| `cmd_list.go` | `orc list` | List all tasks (filterable, limitable) |
| `cmd_show.go` | `orc show TASK-ID` | Show task details |
| `cmd_edit.go` | `orc edit TASK-ID` | Edit task properties |
| `cmd_run.go` | `orc run TASK-ID` | Execute task phases |
| `cmd_pause.go` | `orc pause TASK-ID` | Pause running task |
| `cmd_resume.go` | `orc resume TASK-ID` | Resume paused/blocked/failed task |
| `cmd_rewind.go` | `orc rewind TASK-ID` | Reset to before phase |
| `cmd_reset.go` | `orc reset TASK-ID` | Reset task to initial state |
| `cmd_resolve.go` | `orc resolve TASK-ID` | Mark failed task as resolved |
| `cmd_status.go` | `orc status` | Show task status |
| `cmd_deps.go` | `orc deps [TASK-ID]` | Show task dependencies |
| `cmd_log.go` | `orc log TASK-ID` | Show task transcripts |
| `cmd_diff.go` | `orc diff TASK-ID` | Show task changes |
| `cmd_delete.go` | `orc delete TASK-ID` | Delete task |
| `cmd_approve.go` | `orc approve TASK-ID` | Approve pending gate |
| `cmd_config.go` | `orc config [key] [value]` | Get/set configuration |
| `cmd_pool.go` | `orc pool [subcommand]` | Manage OAuth token pool |
| `cmd_skip.go` | `orc skip TASK-ID` | Skip current phase |
| `cmd_cleanup.go` | `orc cleanup` | Clean up stale worktrees |
| `cmd_version.go` | `orc version` | Show version info |
| `cmd_projects.go` | `orc projects` | List registered projects |
| `cmd_export.go` | `orc export/import` | Export/import tasks |
| `cmd_initiative.go` | `orc initiative` | Manage initiatives |
| `cmd_initiative_plan.go` | `orc initiative plan` | Bulk-create tasks from manifest |
| `cmd_comment.go` | `orc comment` | Manage task comments |

## Task Commands

### `orc new "title"`

| Flag | Description |
|------|-------------|
| `--weight, -w` | Override weight (trivial/small/medium/large/greenfield) |
| `--category, -c` | Category (feature/bug/refactor/chore/docs/test) |
| `--description, -d` | Task description |
| `--template, -t` | Template (bugfix, feature, refactor, migration, spike) |
| `--blocked-by` | Comma-separated task IDs this task is blocked by |
| `--initiative` | Initiative ID to assign task to |

### `orc list`

| Flag | Description |
|------|-------------|
| `--initiative, -i` | Filter by initiative ID (use 'unassigned' for tasks without initiative) |
| `--status, -s` | Filter by status (pending, running, completed, etc.) |
| `--weight, -w` | Filter by weight (trivial, small, medium, large, greenfield) |
| `--limit, -n` | Limit output to N most recent tasks (0 for all) |

### `orc run TASK-ID`

| Flag | Description |
|------|-------------|
| `--force, -f` | Run even if blocked by dependencies |
| `--profile, -p` | Automation profile (auto/fast/safe/strict) |
| `--auto-skip` | Auto-skip phases with existing artifacts |

### `orc edit TASK-ID`

| Flag | Description |
|------|-------------|
| `--title` | Set title |
| `--description` | Set description |
| `--weight` | Set weight |
| `--priority` | Set priority |
| `--category` | Set category |
| `--initiative` | Set initiative (empty string to unlink) |
| `--add-blocker` | Add to blocked_by list |
| `--remove-blocker` | Remove from blocked_by list |

### `orc log TASK-ID`

| Flag | Description |
|------|-------------|
| `--list, -l` | List files only |
| `--phase, -p` | Show specific phase |
| `--all, -a` | Show all transcripts |
| `--tail, -n` | Last N lines (default: 100) |
| `--follow, -f` | Stream in real-time (fsnotify) |

### `orc status`

| Flag | Description |
|------|-------------|
| `--all, -a` | Include completed tasks |
| `--watch, -w` | Refresh every 5s |

### `orc deps [TASK-ID]`

| Flag | Description |
|------|-------------|
| `--tree` | Recursive dependency tree |
| `--graph` | ASCII dependency graph |
| `--initiative, -i` | Filter by initiative |

### `orc resolve TASK-ID`

Mark a task as resolved without re-running it. Useful when an issue was fixed manually, the failure is no longer relevant, or you want to acknowledge and close a task.

**Status behavior:**
- Without `--force`: Only works on tasks with `status=failed`
- With `--force`: Works on any status (running, paused, blocked, created, etc.)

Use `--force` when a task is stuck in 'running' status but its PR was already merged (e.g., executor crashed after merge). The command will detect merged PRs and warn if the PR is missing or not merged.

**Note:** For blocked tasks without `--force`, the command shows guidance to use `orc approve` or `orc resume` instead.

| Flag | Description |
|------|-------------|
| `--message, -m` | Resolution message |
| `--cleanup` | Abort git ops, discard changes |
| `--force, -f` | Skip confirmation and allow resolving non-failed tasks |

## Initiative Commands

### `orc initiative new "title"`

| Flag | Description |
|------|-------------|
| `--vision, -V` | Initiative vision |
| `--owner, -o` | Owner initials |
| `--blocked-by` | Blocked by initiative IDs |

### `orc initiative edit ID`

| Flag | Description |
|------|-------------|
| `--title` | Set title |
| `--vision, -V` | Set vision |
| `--owner, -o` | Set owner |
| `--blocked-by` | Set blocked_by (replaces) |
| `--add-blocker` | Add to blocked_by |
| `--remove-blocker` | Remove from blocked_by |

### `orc initiative plan <manifest.yaml>`

Bulk-create tasks from a YAML manifest file. Supports inline specs that skip the spec phase during execution.

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview tasks without creating them |
| `--yes, -y` | Skip confirmation prompt |
| `--create-initiative` | Create initiative if it doesn't exist |

See `docs/specs/FILE_FORMATS.md` for the manifest format reference.

### `orc initiative run ID`

| Flag | Description |
|------|-------------|
| `--execute` | Actually run (default: preview) |
| `--parallel` | Run ready tasks in parallel |
| `--profile` | Override automation profile |
| `--force, -f` | Run even if blocked |

## Export/Import Commands

### `orc export`

Export tasks for backup or cross-machine portability. Default output is tar.gz to `.orc/exports/`.

| Flag | Description |
|------|-------------|
| `--all-tasks` | Export all tasks (creates archive in `.orc/exports/`) |
| `--initiatives` | Include initiatives in export |
| `--format` | Archive format: `tar.gz` (default), `zip`, `dir` |
| `--minimal` | Skip transcripts and attachments (smaller archive) |
| `--no-state` | Skip execution state (not recommended) |
| `-o, --output` | Output path (default: stdout for single task, `.orc/exports/` for --all-tasks) |

**Examples:**
```bash
orc export TASK-001                     # Single task YAML to stdout
orc export TASK-001 -o task.yaml        # Single task to file
orc export --all-tasks                  # Full backup to .orc/exports/orc-export-*.tar.gz
orc export --all-tasks --format=dir     # Directory format (old behavior)
orc export --all-tasks --minimal        # Smaller backup (no transcripts)
```

### `orc import [path]`

Import tasks from archive, directory, or YAML. Auto-detects format. Default: latest archive in `.orc/exports/`.

| Flag | Description |
|------|-------------|
| `--dry-run` | Preview what would be imported without making changes |
| `--force` | Always overwrite existing tasks (default: newer wins) |
| `--skip-existing` | Never overwrite existing tasks |

**Merge behavior:**
- Newer `updated_at` wins (local preserved on tie)
- Running tasks imported as "interrupted" (use `orc resume` to continue)
- Transcripts deduplicated by TaskID+Phase+Iteration

**Examples:**
```bash
orc import                              # Auto-detect from .orc/exports/
orc import backup.tar.gz               # From archive
orc import ./backup/                    # From directory
orc import --dry-run backup.tar.gz     # Preview only
```

**Format detection:** Extension (`.tar.gz`, `.zip`, `.yaml`) or magic bytes (gzip: `0x1f 0x8b`, zip: `0x50 0x4b`).

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-essential output |
| `--profile, -p` | Automation profile |
| `--plain` | Disable emoji/unicode |

## Aliases

| Alias | Command |
|-------|---------|
| `ls` | `list` |
| `st` | `status` |
| `rm` | `delete` |
| `remove` | `delete` |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Task not found |
| 4 | Gate blocked |
| 5 | Execution failed |
