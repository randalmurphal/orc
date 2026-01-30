# orc CLI Specification

**Version**: 1.0.0

---

## Synopsis

```
orc <command> [options] [arguments]
```

## Global Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--verbose` | `-v` | Increase verbosity (stackable: -vv) | off |
| `--quiet` | `-q` | Suppress non-error output | off |
| `--json` | `-j` | Output in JSON format | off |
| `--config` | `-c` | Path to config file | auto-detect |
| `--help` | `-h` | Show help | - |
| `--version` | `-V` | Show version | - |

---

## Commands

### orc init

Initialize orc in current directory.

```bash
orc init [--force] [--profile <profile>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--force` | Reinitialize existing project | false |
| `--profile` | Automation profile (auto, fast, safe, strict) | auto |

Creates `.orc/` directory structure, `orc.yaml` config, and detects project type.

**Project Detection**: Automatically detects language, frameworks, and frontend presence. For frontend projects, recommends the Playwright plugin.

**Output**:
```
Initialized orc in 42ms
  Project ID: proj-abc123
  Detected: typescript project with react, nextjs
  Config: .orc/config.yaml

Claude Code plugins (run once in Claude Code):
  /plugin marketplace add randalmurphal/orc-claude-plugin
  /plugin install orc@orc
  /plugin install playwright@claude-plugins-official  # Frontend detected

Next steps:
  orc new "task description"  # Create a new task
  orc serve                    # Start web UI at localhost:8080
```

---

### orc new

Create a new task.

```bash
orc new <title> [--weight <weight>] [--category <category>] [--description <desc>] [--attach <file>] [--initiative <id>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--weight`, `-w` | Task weight: trivial/small/medium/large/greenfield | auto-classify |
| `--category`, `-c` | Task category: feature/bug/refactor/chore/docs/test | feature |
| `--description`, `-d` | Task description | opens editor |
| `--branch`, `-b` | Custom branch name | `orc/TASK-XXX` |
| `--template`, `-t` | Use template (bugfix, feature, refactor, migration, spike) | none |
| `--var` | Template variable (KEY=VALUE), can be repeated | none |
| `--attach`, `-a` | Attach file(s) to task, can be repeated | none |
| `--initiative`, `-i` | Link task to initiative (e.g., INIT-001) | none (standalone) |
| `--blocked-by` | Task IDs that must complete first, comma-separated | none |
| `--related-to` | Related task IDs, comma-separated | none |

**Testing Detection**: Task creation automatically detects UI-related keywords in the title/description and sets:
- `requires_ui_testing: true` for UI tasks
- `testing_requirements.e2e: true` for frontend projects with UI tasks

**Attachments**: Attach screenshots, logs, or other files to provide context for the task. Files are stored in `.orc/tasks/TASK-XXX/attachments/`.

**Dependencies**: Specify task dependencies at creation time. Use `--blocked-by` for tasks that must complete first (hard dependency), and `--related-to` for informational links.

**Examples**:
```bash
orc new "Fix typo in README" --weight trivial
orc new "Add OAuth2 authentication" -w large -d "Support Google and GitHub"
orc new "Add dark mode toggle button"   # Auto-detects UI testing required
orc new "Fix login bug" --category bug
orc new -t bugfix "Fix memory leak"
orc new "UI rendering issue" --attach screenshot.png
orc new "API error" -a error.log -a response.json  # Multiple attachments
orc new "Add login flow" --initiative INIT-001     # Link to initiative
orc new "Part 2 of feature" --blocked-by TASK-001
orc new "Final step" --blocked-by TASK-001,TASK-002 --related-to TASK-003
```

**Output**:
```
Task created: TASK-001
   Title:    Add dark mode toggle button
   Weight:   medium
   Category: feature
   Phases:   3
   UI Testing: required (detected from task description)
   Testing: unit, e2e, visual

Next steps:
  orc run TASK-001    - Execute the task
  orc show TASK-001   - View task details
```

---

### orc list

List tasks.

```bash
orc list [--status <status>] [--weight <weight>] [--category <category>] [--queue <queue>] [--limit <n>] [--all]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--status`, `-s` | Filter by status | active |
| `--weight`, `-w` | Filter by weight | all |
| `--category`, `-c` | Filter by category: `feature`, `bug`, `refactor`, `chore`, `docs`, `test` | all |
| `--queue`, `-Q` | Filter by queue: `active`, `backlog` | all |
| `--limit`, `-n` | Limit output to N most recent tasks | 0 (all) |
| `--all`, `-a` | Include completed | false |

**Output**:
```
ID        WEIGHT   STATUS      PHASE      TITLE
TASK-001  medium   running     implement  Add user auth
TASK-002  small    paused      research   Fix rate limiting
```

**Examples**:
```bash
orc list                        # List all tasks
orc list -n 5                   # Show 5 most recent tasks
orc list --status pending -n 10 # Show 10 most recent pending tasks
orc list -i INIT-001 -n 3       # Show 3 most recent tasks in initiative
```

**Note:** Queue, priority, and category are primarily managed via the Web UI or API. See [API Reference](../API_REFERENCE.md) for PATCH `/api/tasks/:id` to set these fields.

---

### orc show

Show task details.

```bash
orc show <task-id> [--checkpoints]
```

---

### orc edit

Edit task properties after creation.

```bash
orc edit <task-id> [--title <title>] [--description <desc>] [--weight <weight>] [--priority <priority>] [--status <status>] [--initiative <id>]
```

| Option | Description | Notes |
|--------|-------------|-------|
| `--title`, `-t` | New task title | |
| `--description`, `-d` | New task description | |
| `--weight`, `-w` | New weight (trivial/small/medium/large/greenfield) | Triggers plan regeneration |
| `--priority`, `-p` | New priority (critical/high/normal/low) | |
| `--status`, `-s` | New task status | For administrative corrections |
| `--initiative`, `-i` | Link/unlink task to initiative | Use `""` to unlink |
| `--blocked-by` | Set blocked_by list (replaces existing) | Comma-separated |
| `--add-blocker` | Add task(s) to blocked_by list | Comma-separated |
| `--remove-blocker` | Remove task(s) from blocked_by list | Comma-separated |
| `--related-to` | Set related_to list (replaces existing) | Comma-separated |
| `--add-related` | Add task(s) to related_to list | Comma-separated |
| `--remove-related` | Remove task(s) from related_to list | Comma-separated |

Weight changes regenerate the task plan with phases appropriate for the new weight. Completed/skipped phases that exist in both the old and new plans retain their status. Requires the task to not be running.

**Status changes:** Valid status values are `created`, `classifying`, `planned`, `paused`, `blocked`, `completed`, `finished`, `failed`. Note: running tasks cannot have their status changed (pause first). This is intended for administrative corrections like marking already-fixed tasks as completed without re-running.

**Initiative linking:** Setting `--initiative INIT-001` links the task to an initiative. The task is auto-added to the initiative's task list (bidirectional sync). Use `--initiative ""` to unlink a task from its initiative.

**Dependency changes:** Validate that referenced task IDs exist and detect circular dependencies (A blocks B blocks A).

**Examples**:
```bash
orc edit TASK-001 --title "Better title"
orc edit TASK-001 --weight large
orc edit TASK-001 --priority critical
orc edit TASK-001 --status completed             # Mark task as done
orc edit TASK-001 -s planned                     # Reset to planned status
orc edit TASK-001 -d "Updated description" -t "New title"
orc edit TASK-001 --initiative INIT-001          # Link to initiative
orc edit TASK-001 --initiative ""                # Unlink from initiative
orc edit TASK-001 --blocked-by TASK-002,TASK-003 # Replace blockers
orc edit TASK-001 --add-blocker TASK-004         # Add a blocker
orc edit TASK-001 --remove-blocker TASK-002      # Remove a blocker
orc edit TASK-001 --related-to TASK-005          # Set related tasks
```

---

### orc run

Execute or resume a task.

```bash
orc run <task-id> [--phase <phase>] [--continue] [--dry-run] [--profile <profile>] [--auto-skip] [--force]
```

| Option | Description |
|--------|-------------|
| `--phase`, `-p` | Start from specific phase |
| `--continue`, `-C` | Resume from last position |
| `--dry-run` | Show execution plan only |
| `--profile`, `-P` | Automation profile (auto, fast, safe, strict) |
| `--auto-skip` | Automatically skip phases with existing artifacts |
| `--force`, `-f` | Run even if task has incomplete blockers |

**Blocking Enforcement**:

Before execution, orc checks if the task has incomplete dependencies (tasks in `blocked_by` that aren't completed). If blockers exist:

```
$ orc run TASK-062

⚠️  This task is blocked by incomplete tasks:
    - TASK-060: Add initiative_id field... (planned)
    - TASK-061: Add Initiatives section... (running)

Run anyway? [y/N]:
```

| Mode | Behavior |
|------|----------|
| Interactive | Prompts for confirmation before running |
| Quiet (`-q`) | Refuses to run without `--force` flag |
| Force (`-f`) | Skips the check entirely |

**Artifact Detection**:

When running a task, orc detects if artifacts from previous runs exist. By default, it prompts:

```
📄 Spec content already exists. Skip spec phase? [Y/n]:
```

With `--auto-skip`, phases with existing artifacts are skipped automatically without prompting.

| Phase | Detected Artifacts | Auto-Skippable |
|-------|-------------------|----------------|
| `spec` | Spec content in database (50+ chars) | Yes |
| `research` | `artifacts/research.md` or research section in spec | Yes |
| `docs` | `artifacts/docs.md` | Yes |
| `implement` | Never detected (too complex to validate) | No |
| `test` | `test-results/report.json` (detected but requires re-run) | No |
| `validate` | `artifacts/validate.md` (detected but requires re-run) | No |

**Note**: Spec content is stored in the database (not as `spec.md` file) to avoid merge conflicts in worktrees. Legacy `spec.md` files are still detected for backward compatibility.

Skip reasons are recorded in `state.yaml` for audit purposes.

**Automation Profiles**:

| Profile | Description |
|---------|-------------|
| `auto` | Default - Fully automated, all gates auto |
| `fast` | Maximum speed, no retry on failure |
| `safe` | Automatic + human gate on merge |
| `strict` | Human gates on spec and merge phases |

**Examples**:
```bash
orc run TASK-001                     # Run with default auto profile
orc run TASK-001 --profile safe      # Human approval on merge
orc run TASK-001 --profile strict    # Human approval on spec and merge
orc run TASK-001 --auto-skip         # Skip phases with existing artifacts
```

---

### orc resume

Resume a paused, blocked, interrupted, orphaned, or failed task.

```bash
orc resume <task-id> [--force] [--stream]
```

| Option | Description |
|--------|-------------|
| `--force`, `-f` | Force resume even if task appears to be running |
| `--stream` | Stream Claude transcript to stdout |

**Resumable Statuses**:
- `paused` - Task was explicitly paused
- `blocked` - Task is waiting for input or intervention
- `interrupted` - Task was interrupted (e.g., Ctrl+C)
- `orphaned` - Task shows "running" but executor process died
- `failed` - Task failed; resume retries from the last incomplete phase

**Failed Task Resume**: When resuming a failed task, execution continues from the last incomplete phase. This allows you to fix issues externally (e.g., install missing dependencies, fix config) and retry without resetting the entire task.

**Orphan Detection**: Tasks that show as "running" but whose executor process has died are automatically detected and handled:
1. Detects orphaned state (executor PID no longer running or heartbeat stale >5 min)
2. Marks task as interrupted
3. Resumes execution from the last phase

**Session Resume**: If the task has a Claude session ID, it is displayed to allow direct Claude access:
```
Session ID: sess_abc123 (use 'claude --resume sess_abc123' for direct Claude access)
```

**Examples**:
```bash
orc resume TASK-001              # Resume paused/blocked task
orc resume TASK-001 --force      # Force resume even if task appears running
orc resume TASK-001 --stream     # Resume with live transcript output
orc resume TASK-001              # Resume failed task (retries from last phase)
```

---

### orc pause

Pause a running task.

```bash
orc pause <task-id> [--reason <reason>]
```

Creates checkpoint and sets status to `paused`.

---

### orc stop

Stop a task (forceful).

```bash
orc stop <task-id> [--force]
```

---

### orc rewind

Rewind to a previous checkpoint.

```bash
orc rewind <task-id> --to <phase> [--hard]
```

| Option | Description |
|--------|-------------|
| `--to`, `-t` | Phase to rewind to |
| `--hard` | Discard later checkpoints |

---

### orc reset

Reset a task to initial state for retry.

```bash
orc reset <task-id> [--force]
```

| Option | Description |
|--------|-------------|
| `--force`, `-f` | Skip confirmation and safety checks |

Clears all execution progress and returns the task to `planned` status. Unlike `rewind` which goes back to a specific checkpoint, `reset` clears everything and starts from scratch.

**Use cases**:
- Retry a failed task from the beginning
- Clear a blocked task and try again
- Restart a paused task from scratch

**Safeguards**:
- Running tasks require `--force` or must be stopped first
- Already-planned tasks are ignored (nothing to reset)
- Confirmation prompt unless `--force` is used

**Examples**:
```bash
orc reset TASK-001           # Reset with confirmation
orc reset TASK-001 --force   # Skip confirmation (for scripts/automation)
```

---

### orc resolve

Mark a task as resolved without re-running.

```bash
orc resolve <task-id> [--yes] [--message <msg>] [--cleanup] [--force]
```

| Option | Description |
|--------|-------------|
| `--yes`, `-y` | Skip confirmation prompt (does not imply `--force`) |
| `--message`, `-m` | Resolution message explaining why task was resolved |
| `--cleanup` | Abort in-progress git operations and discard uncommitted changes in worktree |
| `--force`, `-f` | Skip confirmation and allow resolving non-failed tasks (implies `--yes`) |

Marks a task as completed (resolved) without clearing its execution state. Unlike `reset` which clears progress for retry, `resolve` closes out a task while preserving execution context.

**Use cases**:
- Issue was fixed manually outside of orc
- Failure is no longer relevant (requirements changed)
- Acknowledge and close out a failed task without retry
- **Task stuck in 'running' status but its PR was already merged** (use `--force`)
- **Executor crashed after merge but before marking task complete** (use `--force`)

**Force resolving non-failed tasks**:

By default, `resolve` only works on failed tasks. Use `--force` to resolve tasks in any status (running, paused, blocked, created, etc.). This is useful when a task is stuck but the work is already complete.

When force-resolving:
- If the task has a merged PR, it is reported: `PR merged (PR #123)`
- If no PR exists, a warning is shown: `Warning: No PR found for this task. Work may be incomplete.`
- If the PR exists but isn't merged, a warning is shown: `Warning: PR #45 is not merged (status: pending_review). Work may be incomplete.`
- The original status and force_resolved flag are recorded in metadata

```
$ orc resolve TASK-201 --force
PR merged (PR #123)
Task TASK-201 marked as resolved (was: running)

$ orc resolve TASK-202 --force  # task with no PR
Warning: No PR found for this task. Work may be incomplete.
Task TASK-202 marked as resolved (was: running)
```

**Worktree handling**:

If the task has an associated worktree with uncommitted changes, in-progress git operations (rebase/merge), or unresolved conflicts, a warning is displayed:

```
📁 Worktree: .orc/worktrees/orc-TASK-001
   ⚠️  Rebase in progress - worktree is in an incomplete state
   ⚠️  3 uncommitted file(s)
```

| Flag | Behavior |
|------|----------|
| `--cleanup` | Abort in-progress git ops, discard uncommitted changes, then resolve |
| `--yes` | Skip confirmation prompt only (still requires failed status) |
| `--force` | Skip confirmation and status checks (resolve any status) |
| (default) | Show warnings and prompt for confirmation |

**Metadata stored**:
- `resolved: true` - Indicates task was resolved, not executed to completion
- `resolved_at` - Timestamp of resolution
- `resolution_message` - Optional explanation (if provided via `-m`)
- `force_resolved: true` - Set when `--force` was used on a non-failed task
- `original_status` - The task's status before resolution (when force-resolved)
- `pr_was_merged: true` - Set if PR was merged at resolution time (when force-resolved)
- `worktree_was_dirty` - Set if worktree had uncommitted changes
- `worktree_had_conflicts` - Set if worktree had unresolved merge conflicts
- `worktree_had_incomplete_operation` - Set if worktree had rebase/merge in progress

**Status behavior**:
- Without `--force`: Only failed tasks can be resolved
- With `--force`: Any status can be resolved (running, paused, blocked, created, etc.)
- Confirmation prompt unless `--yes` or `--force` is used

**Examples**:
```bash
orc resolve TASK-001                          # Resolve failed task with confirmation
orc resolve TASK-001 --yes                    # Skip confirmation prompt
orc resolve TASK-001 -y -m "Fixed manually"   # Skip prompt with message
orc resolve TASK-001 --cleanup                # Clean up worktree state first
orc resolve TASK-001 --force                  # Resolve any status (skip checks)
orc resolve TASK-001 --force -m "PR merged"   # Force resolve with message
```

---

### orc skip

Skip a phase (mark as skipped without execution).

```bash
orc skip <task-id> --phase <phase> [--reason <reason>]
```

| Option | Description |
|--------|-------------|
| `--phase`, `-p` | Phase to skip |
| `--reason`, `-r` | Reason for skipping (recommended) |

Creates audit entry and advances to next phase.

---

### orc fork

Create a new task from an existing checkpoint (alternative approach).

```bash
orc fork <task-id> [--from <commit>] [--name <new-id>]
```

| Option | Description |
|--------|-------------|
| `--from`, `-f` | Checkpoint commit to fork from (default: current) |
| `--name`, `-n` | New task ID (default: auto-generated) |

Creates new branch `orc/NEW-TASK-ID` from the specified checkpoint.

---

### orc cleanup

Remove completed task branches and worktrees.

```bash
orc cleanup [--all] [--older-than <duration>] [--dry-run]
```

| Option | Description |
|--------|-------------|
| `--all`, `-a` | Remove all task branches (not just completed) |
| `--older-than` | Remove branches older than duration (e.g., 7d) |
| `--dry-run` | Show what would be removed without removing |

**Examples**:
```bash
orc cleanup                    # Remove completed task branches
orc cleanup --all              # Remove all task branches
orc cleanup --older-than 7d    # Remove branches older than 7 days
```

---

### orc approve

Approve a human gate.

```bash
orc approve <task-id> [--comment <comment>]
```

---

### orc reject

Reject a human gate.

```bash
orc reject <task-id> --reason <reason>
```

Rewinds to phase start and pauses.

---

### orc log

Show task transcripts.

```bash
orc log <task-id> [--phase <phase>] [--tail <n>] [--follow] [--all] [--response-only] [--prompt-only] [--raw] [--no-color]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--phase`, `-p` | Filter to specific phase (e.g., 'implement', 'test') | all |
| `--tail`, `-n` | Number of entries to show (0 for all) | 100 |
| `--follow`, `-f` | Stream new messages in real-time (tails JSONL file) | false |
| `--all`, `-a` | Show all transcripts (not just latest) | false |
| `--response-only`, `-r` | Show only Claude's responses (assistant messages) | false |
| `--prompt-only`, `-P` | Show only the prompts (user messages) | false |
| `--raw` | Show raw JSON content (unformatted) | false |
| `--no-color` | Disable color output | false |

**Data Sources:**

Transcripts are loaded from the database first. If no database transcripts exist, orc falls back to reading markdown files from `.orc/tasks/{taskID}/transcripts/`. This enables viewing transcripts even when the database wasn't synced.

**Real-time Streaming** (`--follow`):

Streams from Claude's live JSONL session file during task execution. Requires an active task (use without `--follow` for completed tasks).

**Examples**:
```bash
orc log TASK-001              # Show latest transcript (last 100 entries)
orc log TASK-001 --all        # Show all transcripts
orc log TASK-001 --phase test # Show specific phase transcript
orc log TASK-001 --tail 50    # Show last 50 entries
orc log TASK-001 --tail 0     # Show entire transcript
orc log TASK-001 --follow     # Stream new messages in real-time
orc log TASK-001 -r           # Show only Claude's responses
orc log TASK-001 --prompt-only # Show only the prompts
orc log TASK-001 --raw        # Show raw JSON content
```

---

### orc comment

Manage task comments and notes.

```bash
orc comment <subcommand> [options]
```

#### orc comment add

Add a comment to a task.

```bash
orc comment add <task-id> <content> [--author <name>] [--type <type>] [--phase <phase>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--author`, `-a` | Author name | anonymous |
| `--type`, `-t` | Author type: human, agent, system | human |
| `--phase`, `-p` | Phase this comment relates to | none |

**Examples**:
```bash
orc comment add TASK-001 "This approach won't work with existing auth"
orc comment add TASK-001 "Note: uses deprecated API" --author claude --type agent
orc comment add TASK-001 "Review feedback addressed" --phase implement
```

#### orc comment list

List comments for a task.

```bash
orc comment list <task-id> [--type <type>] [--phase <phase>]
```

| Option | Description |
|--------|-------------|
| `--type`, `-t` | Filter by author type |
| `--phase`, `-p` | Filter by phase |

**Examples**:
```bash
orc comment list TASK-001
orc comment list TASK-001 --type agent
orc comment list TASK-001 --phase implement
```

#### orc comment delete

Delete a comment.

```bash
orc comment delete <comment-id>
```

---

### orc diff

Show changes made by task.

```bash
orc diff <task-id> [--phase <phase>] [--stat]
```

---

### orc deps

Show task dependencies.

```bash
orc deps [task-id] [--tree] [--graph] [--initiative <id>] [--json]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--tree` | Show full dependency tree | false |
| `--graph` | Show ASCII dependency graph | false |
| `--initiative`, `-i` | Filter graph by initiative ID | none |

Without arguments, shows dependency overview for all tasks (blocking, blocked, independent counts).

**Dependency Types**:

| Type | Description |
|------|-------------|
| `blocked_by` | Tasks that must complete before this task (stored) |
| `blocks` | Tasks waiting on this task (computed inverse) |
| `related_to` | Related tasks for reference (stored) |
| `referenced_by` | Tasks whose descriptions mention this task (auto-detected) |

**Views**:

**Standard view** (`orc deps TASK-062`):
```
TASK-062: Add initiative filter to task list and board filter bars
──────────────────────────────────────────────────

Blocked by (2):
  ○ TASK-060  Add initiative_id field...         planned
  ○ TASK-061  Add Initiatives section...         planned

Blocks (1):
  ○ TASK-065  Add optional swimlane view...      planned

Related (1):
  TASK-063  Show initiative badge...

Status: 🚫 BLOCKED (waiting on 2 task(s): TASK-060, TASK-061)
```

**Tree view** (`orc deps --tree TASK-065`):
```
TASK-065: Add optional swimlane view toggle
└── TASK-062: Add initiative filter...
    ├── TASK-060: Add initiative_id field...        ← start here
    └── TASK-061: Add Initiatives section...
        └── TASK-060: Add initiative_id field...    ← already shown
```

Shows the full dependency tree recursively, marking completed tasks with ✓, root tasks with "← start here", and previously shown nodes with "← already shown".

**Graph view** (`orc deps --graph INIT-001`):
```
Dependency graph for INIT-001:

TASK-060
├─> TASK-061 ─> TASK-062 ─> TASK-065
├─> TASK-063
├─> TASK-064
└─> TASK-066

TASK-067 (no deps)
```

ASCII dependency graph showing task flow. Single-chain dependencies are collapsed inline. Filter by initiative with `-i`.

**JSON output** (`orc deps TASK-062 --json`):
```json
{
  "task_id": "TASK-062",
  "title": "Add initiative filter...",
  "status": "planned",
  "blocked_by": [{"id": "TASK-060", "title": "...", "status": "planned"}],
  "blocks": [{"id": "TASK-065", "title": "...", "status": "planned"}],
  "related_to": [{"id": "TASK-063", "title": "...", "status": "planned"}],
  "referenced_by": [],
  "summary": {
    "is_blocked": true,
    "unmet_blockers": 2,
    "total_blockers": 2,
    "tasks_blocking": 1,
    "related_count": 1,
    "referenced_count": 0
  }
}
```

**Overview mode** (`orc deps` without task ID):
```
⚡ BLOCKING OTHER TASKS

  TASK-060  Add initiative_id field...  → blocks: TASK-061, TASK-062, TASK-063

🚫 BLOCKED

  TASK-062  Add initiative filter...    ← waiting on: TASK-060, TASK-061

─── 10 tasks: 3 blocking, 2 blocked, 5 independent ───
```

---

### orc status

Show overall orc status.

```bash
orc status [--all] [--watch]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--all`, `-a` | Include all tasks (not just active) | false |
| `--watch`, `-w` | Refresh status every 5 seconds | false |

**Status Categories** (in priority order):
1. **Orphaned** - Tasks marked running but executor process died
2. **Attention Needed** - Blocked tasks requiring human input
3. **Running** - Active tasks in progress
4. **Blocked** - Tasks waiting on other tasks (dependency blocked)
5. **Ready** - Tasks that can run (no dependencies or all satisfied)
6. **Paused** - Tasks that can be resumed
7. **Recent** - Completed/failed in last 24h

**Output**:
```
⚠️  ORPHANED (executor died)

  TASK-002  Fix login validation  (executor process not running)
  Use 'orc resume <task-id>' to continue these tasks

⚠️  ATTENTION NEEDED

  TASK-003  Add OAuth2  (blocked - needs input)

⏳ RUNNING

  TASK-025  Fix resume command...  [implement]

🚫 BLOCKED

  TASK-062  Add initiative filter...  (by TASK-060, TASK-061)
  TASK-065  Add swimlane view...      (by TASK-060, TASK-061, TASK-062)

📋 READY

  TASK-060  Add initiative_id field...
  TASK-067  Improve card truncation...

⏸️  PAUSED

  TASK-004  Refactor API handlers  → orc resume TASK-004

RECENT (24h)

  ✓  TASK-005  Fix typo  5 hours ago

─── 8 tasks (1 running, 1 orphaned, 2 blocked, 2 ready, 1 paused, 1 completed) ───
```

**Examples**:
```bash
orc status           # Quick overview
orc status --all     # Include all tasks
orc status --watch   # Refresh every 5s
```

---

### orc export

Export task(s) and initiative(s) for cross-machine portability.

```bash
orc export [task-id|init-id] [options]
orc export --all-tasks [options]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--output`, `-o` | Output path | `.orc/exports/` for --all-tasks, stdout for single |
| `--all-tasks` | Export all tasks | false |
| `--all` | Include all data (state + transcripts + context) | false |
| `--state` | Include execution state and gate decisions | false |
| `--transcripts` | Include full transcript content | false |
| `--initiatives` | Include initiatives (with --all-tasks) | false |

**Default Location**: `.orc/exports/` - this is where `orc import` looks by default.

**What Gets Exported** (with `--all`):
- Task definition, plan, spec, state
- Transcripts (execution history)
- Comments (task and review)
- Attachments (binary files, base64-encoded)
- Gate decisions
- Initiative vision and decisions (with `--initiatives`)

**Export Format Version**: Exports include a `version` field for compatibility detection.

**Examples**:
```bash
orc export TASK-001                         # Single task to stdout
orc export TASK-001 -o task.yaml            # Single task to file
orc export --all-tasks                      # All tasks to .orc/exports/
orc export --all-tasks --all                # Full backup with all data
orc export --all-tasks --all --initiatives  # Full backup including initiatives
orc export --all-tasks -o backup.zip        # All tasks to zip archive
orc export INIT-001                         # Single initiative to stdout
```

---

### orc import

Import task(s) and initiative(s) from YAML, directory, or zip archive.

```bash
orc import [path] [options]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--force` | Always overwrite existing items | false |
| `--skip-existing` | Never overwrite existing items | false |

**Default Location**: `.orc/exports/` - matches export default.

**Smart Merge** (default behavior):
- New items are imported
- Existing items: import only if incoming has newer `updated_at`
- Use `--force` to always overwrite
- Use `--skip-existing` to never overwrite

**Auto-Detection**: Import automatically detects:
- Task YAML files (by `task` field)
- Initiative YAML files (by `type: initiative` field)
- Directories (imports all YAML, including `initiatives/` subdirectory)
- Zip archives

**Examples**:
```bash
orc import                              # Import from .orc/exports/
orc import task.yaml                    # Import single task
orc import ./backup/                    # Import all YAML from directory
orc import tasks.zip                    # Import all from zip archive
orc import tasks.zip --force            # Always overwrite existing
orc import tasks.zip --skip-existing    # Never overwrite existing
```

---

### orc config

View or modify configuration.

```bash
orc config [key] [value]
orc config --list
orc config --edit
```

---

### orc initiative

Manage initiatives (groups of related tasks).

```bash
orc initiative <subcommand> [options]
```

#### orc initiative new

Create a new initiative.

```bash
orc initiative new <title> [--vision <vision>] [--owner <initials>] [--shared] [--blocked-by <ids>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--vision`, `-V` | Initiative vision statement | none |
| `--owner`, `-o` | Owner initials | none |
| `--shared` | Create in shared directory for team access | false |
| `--blocked-by` | Comma-separated initiative IDs that must complete first | none |

**Examples**:
```bash
orc initiative new "User Authentication System"
orc initiative new "API Refactor" --vision "Modern REST API with OpenAPI spec"
orc initiative new "Dark Mode" --shared
orc initiative new "React Migration" --blocked-by INIT-001
```

#### orc initiative list

List all initiatives.

```bash
orc initiative list [--status <status>] [--shared]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--status`, `-s` | Filter by status (draft/active/completed/archived) | all |
| `--shared` | List shared initiatives | false |

Shows blocked status indicator for initiatives with incomplete blockers.

#### orc initiative show

Show initiative details including dependencies.

```bash
orc initiative show <id> [--shared]
```

Displays:
- Initiative metadata (title, status, vision, owner)
- Blocked status indicator if blocked by other initiatives
- Dependency relationships (blocked_by, blocks)
- Linked tasks with completion status
- Recorded decisions

#### orc initiative edit

Edit an initiative's properties including dependencies.

```bash
orc initiative edit <id> [options]
```

| Option | Description |
|--------|-------------|
| `--title` | Set initiative title |
| `--vision`, `-V` | Set initiative vision |
| `--owner`, `-o` | Set owner initials |
| `--blocked-by` | Set blocked_by list (replaces existing) |
| `--add-blocker` | Add initiative(s) to blocked_by list |
| `--remove-blocker` | Remove initiative(s) from blocked_by list |
| `--shared` | Use shared initiative |

**Examples**:
```bash
orc initiative edit INIT-001 --title "New Title"
orc initiative edit INIT-001 --vision "Updated vision statement"
orc initiative edit INIT-001 --blocked-by INIT-002,INIT-003
orc initiative edit INIT-001 --add-blocker INIT-004
orc initiative edit INIT-001 --remove-blocker INIT-002
```

#### orc initiative add-task

Link a task to an initiative.

```bash
orc initiative add-task <initiative-id> <task-id>
```

#### orc initiative decide

Record a decision for an initiative.

```bash
orc initiative decide <id> <decision-text> [--rationale <text>]
```

#### orc initiative activate / complete

Change initiative status.

```bash
orc initiative activate <id>
orc initiative complete <id>
```

#### orc initiative run

Run tasks from an initiative.

```bash
orc initiative run <id> [--execute] [--parallel] [--profile <profile>] [--force]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--execute` | Actually run tasks (default: preview only) | false |
| `--parallel` | Run ready tasks in parallel | false |
| `--profile`, `-p` | Automation profile | auto |
| `--force`, `-f` | Run even if blocked by other initiatives | false |

**Blocking enforcement**: If the initiative has incomplete dependencies (initiatives in `blocked_by` that aren't completed), shows warning and suggests completing blockers first or using `--force`.

**Examples**:
```bash
orc initiative run INIT-001              # Show ready tasks (safe preview)
orc initiative run INIT-001 --execute    # Actually run the tasks
orc initiative run INIT-001 --parallel   # Run ready tasks in parallel
orc initiative run INIT-001 --force      # Run even if blocked by other initiatives
```

#### orc initiative delete

Delete an initiative.

```bash
orc initiative delete <id> [--force]
```

---

### orc serve

Start the API server for the web UI.

```bash
orc serve [--port <port>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--port`, `-p` | Port to listen on | 8080 |

The API server provides:
- REST endpoints for task management
- SSE streaming for live transcript updates
- CORS headers for frontend development

**Example**:
```bash
orc serve              # Start on :8080
orc serve --port 3000  # Start on :3000
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error / not found |
| 2 | Precondition failed |
| 3 | Execution failed |
| 4 | User interrupted |

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ORC_CONFIG` | Config file path |
| `ORC_CLAUDE_PATH` | Claude binary path |
| `ORC_DATA_DIR` | Override .orc location |
| `ORC_LOG_LEVEL` | debug/info/warn/error |
| `ORC_NO_COLOR` | Disable colored output |

---

## Shell Completion

```bash
orc completion bash > /etc/bash_completion.d/orc
orc completion zsh > "${fpath[1]}/_orc"
orc completion fish > ~/.config/fish/completions/orc.fish
```
