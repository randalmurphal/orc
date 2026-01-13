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
orc new <title> [--weight <weight>] [--description <desc>]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--weight`, `-w` | Task weight: trivial/small/medium/large/greenfield | auto-classify |
| `--description`, `-d` | Task description | opens editor |
| `--branch`, `-b` | Custom branch name | `orc/TASK-XXX` |
| `--template`, `-t` | Use template (bugfix, feature, refactor, migration, spike) | none |
| `--var` | Template variable (KEY=VALUE), can be repeated | none |

**Testing Detection**: Task creation automatically detects UI-related keywords in the title/description and sets:
- `requires_ui_testing: true` for UI tasks
- `testing_requirements.e2e: true` for frontend projects with UI tasks

**Examples**:
```bash
orc new "Fix typo in README" --weight trivial
orc new "Add OAuth2 authentication" -w large -d "Support Google and GitHub"
orc new "Add dark mode toggle button"   # Auto-detects UI testing required
orc new -t bugfix "Fix memory leak"
```

**Output**:
```
Task created: TASK-001
   Title:  Add dark mode toggle button
   Weight: medium
   Phases: 3
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
orc list [--status <status>] [--weight <weight>] [--all]
```

| Option | Description | Default |
|--------|-------------|---------|
| `--status`, `-s` | Filter by status | active |
| `--weight`, `-w` | Filter by weight | all |
| `--all`, `-a` | Include completed | false |

**Output**:
```
ID        WEIGHT   STATUS      PHASE      TITLE
TASK-001  medium   running     implement  Add user auth
TASK-002  small    paused      research   Fix rate limiting
```

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
orc edit <task-id> [--title <title>] [--description <desc>] [--weight <weight>]
```

| Option | Description | Notes |
|--------|-------------|-------|
| `--title`, `-t` | New task title | |
| `--description`, `-d` | New task description | |
| `--weight`, `-w` | New weight (trivial/small/medium/large/greenfield) | Triggers plan regeneration |

Weight changes regenerate the task plan with phases appropriate for the new weight. This resets all phase progress and requires the task to not be running.

**Examples**:
```bash
orc edit TASK-001 --title "Better title"
orc edit TASK-001 --weight large
orc edit TASK-001 -d "Updated description" -t "New title"
```

---

### orc run

Execute or resume a task.

```bash
orc run <task-id> [--phase <phase>] [--continue] [--dry-run] [--profile <profile>]
```

| Option | Description |
|--------|-------------|
| `--phase`, `-p` | Start from specific phase |
| `--continue`, `-C` | Resume from last position |
| `--dry-run` | Show execution plan only |
| `--profile`, `-P` | Automation profile (auto, fast, safe, strict) |

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
```

---

### orc resume

Resume a paused, blocked, interrupted, or orphaned task.

```bash
orc resume <task-id> [--force] [--stream]
```

| Option | Description |
|--------|-------------|
| `--force`, `-f` | Force resume even if task appears to be running |
| `--stream` | Stream Claude transcript to stdout |

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
orc log <task-id> [--phase <phase>] [--tail <n>] [--follow]
```

| Option | Description |
|--------|-------------|
| `--phase`, `-p` | Specific phase |
| `--tail`, `-n` | Last N lines |
| `--follow`, `-f` | Live output |

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
2. **Attention Needed** - Blocked tasks requiring input
3. **Running** - Active tasks in progress
4. **Paused** - Tasks that can be resumed
5. **Recent** - Completed/failed in last 24h

**Output**:
```
⚠️  ORPHANED (executor died)

  TASK-002  Fix login validation  (executor process not running)
  Use 'orc resume <task-id>' to continue these tasks

⚠️  ATTENTION NEEDED

  TASK-003  Add OAuth2  (blocked - needs input)

⏳ RUNNING

  TASK-001  Add user auth  [implement]

RECENT (24h)

  ✓  TASK-004  Fix typo  5 hours ago

─── 4 tasks (1 running, 1 orphaned, 1 blocked, 1 completed) ───
```

**Examples**:
```bash
orc status           # Quick overview
orc status --all     # Include all tasks
orc status --watch   # Refresh every 5s
```

---

### orc export / import

Export or import task data.

```bash
orc export <task-id> [--format md|json|yaml]
orc import <file> --task <task-id>
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
