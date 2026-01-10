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
orc init [--force]
```

Creates `.orc/` directory structure and `orc.yaml` config.

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

**Examples**:
```bash
orc new "Fix typo in README" --weight trivial
orc new "Add OAuth2 authentication" -w large -d "Support Google and GitHub"
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

### orc run

Execute or resume a task.

```bash
orc run <task-id> [--phase <phase>] [--continue] [--dry-run]
```

| Option | Description |
|--------|-------------|
| `--phase`, `-p` | Start from specific phase |
| `--continue`, `-C` | Resume from last position |
| `--dry-run` | Show execution plan only |

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

### orc diff

Show changes made by task.

```bash
orc diff <task-id> [--phase <phase>] [--stat]
```

---

### orc status

Show overall orc status.

```bash
orc status
```

**Output**:
```
orc v1.0.0
Config: ./orc.yaml
Data: ./.orc

Active Tasks: 3
  TASK-001  medium   running   implement  Add user auth

Completed Today: 2
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
