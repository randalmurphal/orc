# orc

Task orchestration for Claude Code. One orc is dumb. A warband gets things done.

Orc wraps Claude Code in a structured workflow: classify the task, pick the right phases, execute in a loop until done, commit, PR. Trivial bug fix? One phase. Major feature? Spec, TDD, implementation, review, docs. Rigor scales to complexity.

> **Heads up**: This runs Claude autonomously. It reads code, writes code, runs commands, and commits to git without asking. That's the point. If you want guardrails, configure [human gates](#4-quality-gates). If you want to describe a task and come back to a finished PR, keep reading.

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/randalmurphal/orc/main/install.sh | sh
# or: go install github.com/randalmurphal/orc/cmd/orc@latest

# Initialize your project
cd your-project
orc init

# Create and run a task
orc new "Add dark mode toggle"
orc run TASK-001

# Web UI (optional)
orc serve  # localhost:8080
```

Orc picks the workflow, runs each phase in a loop until completion, commits after each phase, and opens a PR when done.

## How It Works

### 1. Workflows

Not every task needs the same process. Orc uses weight-based workflows to decide which phases run:

| Workflow | When | Phases |
|----------|------|--------|
| **trivial** | Typo, one-liner | implement |
| **small** | Single component | tiny_spec, implement, review, docs |
| **medium** | Multi-file, clear scope | spec, tdd_write, tdd_integrate, implement, review, docs |
| **large** | Cross-cutting, new systems | spec, tdd_write, tdd_integrate, breakdown, implement, review, docs |

What the phases do:

- **spec / tiny_spec** — success criteria and testing requirements
- **tdd_write** — failing unit tests before implementation
- **tdd_integrate** — failing integration tests to catch dead code
- **breakdown** — decomposes large tasks into implementation steps
- **implement** — writes code, iterates until tests pass
- **review** — multi-round code review
- **docs** — updates documentation

```bash
orc new "Refactor the auth system" --workflow large
```

### 2. Phase Execution

Each phase runs in a persistent loop. Claude iterates until the phase is actually complete or it's stuck:

```
implement phase:
  iteration 1: writes initial code
  iteration 2: test failure → fixes it
  iteration 3: edge case → handles it
  iteration 4: outputs {"status": "complete", ...}
  → commits, moves to next phase
```

The prompt stays the same each iteration, but the codebase evolves. This is the [Ralph Wiggum pattern](#credits) — conceptually just `while :; do cat PROMPT.md | claude-code; done` with structure around it.

### 3. Git Integration

Every task gets a branch. Every phase gets a commit.

```bash
orc diff TASK-001                # See what changed
orc rewind TASK-001 --to implement  # Roll back to a phase
git log orc/TASK-001             # Full audit trail
```

Tasks run in isolated git worktrees (`~/.orc/worktrees/`), so you can run multiple tasks in parallel without conflicts.

### 4. Quality Gates

Gates control flow between phases:

| Gate | Behavior |
|------|----------|
| **auto** | Proceed if phase succeeded (default) |
| **ai** | Claude evaluates readiness |
| **human** | You approve before continuing |
| **skip** | Skip the phase |

Default: everything auto except merge (human). You review before code hits main.

```bash
orc gates list              # Gate config for all phases
orc approve TASK-001        # Approve a human gate
orc reject TASK-001         # Block at a gate
```

### 5. Retries

Test failures trigger automatic retry from implementation with context about what broke:

```
implement → test (FAIL) → implement (retry with failure context) → test (PASS)
```

No manual intervention unless it's actually stuck.

## Typical Workflow

```bash
# Queue up tasks
orc new "Fix the login redirect bug"
orc new "Add CSV export"
orc new "Refactor connection pooling" --workflow large

# Run in parallel (worktree isolation)
orc run TASK-001 &
orc run TASK-002 &
orc run TASK-003 &

# Check progress
orc status

# Review results
orc diff TASK-001
orc finalize TASK-001  # Sync with target branch, merge or open PR
```

## Configuration

Config loads in order (later overrides earlier):

1. Built-in defaults
2. `~/.orc/config.yaml` (user)
3. `.orc/config.yaml` (project)
4. Environment variables (`ORC_*`)

```yaml
# .orc/config.yaml

profile: auto  # auto | fast | safe | strict

gates:
  default_type: auto
  phase_overrides:
    merge: human

completion:
  action: pr           # pr | merge | none
  target_branch: main
  sync:
    strategy: completion   # none | phase | completion | detect
    fail_on_conflict: true
  pr:
    auto_merge: false
    draft: false
    labels: []
    reviewers: []

worktree:
  enabled: true
  cleanup_on_complete: true
  cleanup_on_fail: false  # Keep for debugging

retry:
  enabled: true
  retry_map:
    test: implement
    validate: implement

executor:
  max_retries: 5

review:
  enabled: true
  rounds: 2  # Exploratory + validation

hosting:
  provider: github  # github | gitlab
```

See [CONFIG_HIERARCHY.md](docs/specs/CONFIG_HIERARCHY.md) for all options.

### Profiles

| Profile | Use Case | Human Gates |
|---------|----------|-------------|
| `auto` | Solo dev, CI/CD | merge only |
| `fast` | Prototypes, experiments | none |
| `safe` | Team default | merge |
| `strict` | Critical code | spec + merge |

```bash
orc run TASK-001 --profile strict
```

### Environment Overrides

```bash
ORC_PROFILE=strict
ORC_MODEL=opus        # Short names: opus, sonnet, haiku
ORC_GATES_DEFAULT=human
ORC_WORKTREE_ENABLED=false
ORC_RETRY_ENABLED=false
```

## Commands

All commands support `--help` with detailed usage guidance.

### Task Lifecycle

```bash
orc new "title"              # Create task (default workflow)
orc new "title" --workflow X # Specific workflow
orc new "title" -d "desc"   # With description
orc new "title" -i INIT-001 # Link to initiative
orc run TASK-ID              # Execute
orc run TASK-ID --profile X  # Execute with profile
orc stop TASK-ID             # Abort
orc pause TASK-ID            # Pause with checkpoint
orc resume TASK-ID           # Continue paused/failed task
orc rewind TASK-ID --to X   # Roll back to phase
orc reset TASK-ID            # Clear progress
orc skip TASK-ID --phase X   # Skip a phase
orc close TASK-ID            # Close permanently
orc finalize TASK-ID         # Sync, resolve conflicts, merge
orc approve TASK-ID          # Approve human gate
orc reject TASK-ID           # Reject at gate
orc delete TASK-ID           # Delete task
```

### Inspection

```bash
orc status                   # What needs attention
orc status --watch           # Live dashboard
orc show TASK-ID             # Task details and state
orc show TASK-ID --gates     # Include gate history
orc log TASK-ID              # Claude transcripts
orc log TASK-ID --follow     # Stream live
orc diff TASK-ID             # Git diff
orc deps TASK-ID             # Dependencies (--tree, --graph)
orc costs                    # Cost report (--by model, --since 7d)
orc scratchpad TASK-ID       # Phase observations
orc recommendation list      # Pending recommendations
orc search "query"           # Search tasks
orc list                     # List tasks
```

### Initiatives

Group related tasks with shared context:

```bash
orc initiative new "Auth overhaul" --vision "JWT-based auth with refresh tokens"
orc initiative decide INIT-001 "Use bcrypt" --rationale "Industry standard"
orc initiative link INIT-001 TASK-001 TASK-002
orc initiative run INIT-001   # Run all ready tasks in dependency order
orc initiative list
orc initiative show INIT-001
```

Initiative vision and decisions get injected into every linked task's prompts, keeping Claude aligned across related work.

### Multi-Project

```bash
orc init                     # Register project
orc serve                    # API + Web UI for all projects
orc projects                 # List registered projects
orc projects add .           # Register current directory
orc projects remove ID       # Unregister
orc projects default ID      # Set default
```

Use `--project/-P` or `ORC_PROJECT` to target a specific project.

### Import / Export

```bash
orc export --all-tasks                # Full backup (tar.gz)
orc export --all-tasks --initiatives  # Include initiatives
orc export --all-tasks --minimal      # No transcripts
orc import                            # Restore from backup
orc import --dry-run                  # Preview
orc import jira                       # Import from Jira Cloud
orc import jira --project X           # Specific project
orc import jira --jql "..."           # Filter with JQL
```

### Config Management

```bash
orc config show --source     # Show config with resolution sources
orc config get <key>         # Get value
orc config set <key> <val>   # Set value
orc config edit              # Open in $EDITOR
orc constitution show        # View project constitution
orc constitution set --file X
orc gates list               # Gate config
orc gates show <phase>       # Gate config for a phase
```

### Benchmarking

Evaluate model performance across workflows:

```bash
orc bench curate import suite.yaml
orc bench run --baseline --trials 3
orc bench run --variant ID --trials 3
orc bench report
orc bench judge
```

### Token Pool

Rotate between multiple API accounts to avoid rate limits:

```bash
orc pool init
orc pool add personal
orc pool add work
orc pool list
orc pool status
orc pool remove personal
orc pool switch work
```

## Web UI

```bash
orc serve  # localhost:8080
```

Pages:

- **My Work** — running tasks, attention items, recent activity
- **Task Board** — kanban with initiative filtering
- **Task Detail** — timeline, live transcript, controls
- **Initiatives** — overview with dependency graphs
- **Recommendations** — follow-up, risk, and decision inbox
- **Workflows** — visual workflow editor
- **Settings** — config, constitution, hooks, tools
- **Stats / Timeline** — execution statistics and event history

### Keyboard Shortcuts

`Shift+Alt` modifier to avoid browser conflicts:

| Key | Action |
|-----|--------|
| `Shift+Alt+K` | Command palette |
| `Shift+Alt+N` | New task |
| `Shift+Alt+B` | Toggle sidebar |
| `Shift+Alt+P` | Project switcher |
| `j/k` | Navigate task list |
| `Enter` | Open selected |
| `r` | Run task |
| `g d` | Go to dashboard |
| `g t` | Go to tasks |
| `/` | Focus search |
| `?` | Show shortcuts help |

## Advanced

### Branch Sync

Parallel tasks can diverge from main. Orc syncs task branches with the target to catch conflicts early.

```yaml
completion:
  sync:
    strategy: completion     # none | phase | completion | detect
    fail_on_conflict: true   # Abort on conflicts vs warn
```

- `none` — no automatic sync
- `phase` — sync before each phase (safest)
- `completion` — sync before PR/merge (default)
- `detect` — check for conflicts without resolving

### Constitution

Project-level principles injected into all phase prompts. Use it to enforce standards, architecture decisions, or domain rules.

```bash
orc constitution set --file coding-standards.md
orc constitution show
```

Stored at `.orc/CONSTITUTION.md` (git-tracked).

### Token Pool Config

```yaml
# ~/.orc/token-pool/pool.yaml
strategy: round-robin
switch_on_rate_limit: true
accounts:
  - id: personal
    access_token: "sk-ant-..."
  - id: work
    access_token: "sk-ant-..."
```

### Multi-Round Review

1. **Round 1 (Exploratory)** — gaps, security issues, architecture concerns
2. **Round 2 (Validation)** — verifies Round 1 findings were addressed

Includes no-op detection (catches implementations that don't change behavior) and success criteria verification.

### Dependencies

```bash
orc new "Part 2" --blocked-by TASK-001
orc deps TASK-001 --tree
orc deps TASK-001 --graph
```

Tasks support `blocked_by` (must complete first) and `related_to` (informational). Initiatives also support dependency ordering.

### Jira Import

```bash
orc import jira --url https://your-org.atlassian.net --email you@org.com --token $JIRA_TOKEN
```

Or in config:

```yaml
jira:
  url: https://your-org.atlassian.net
  email: you@org.com
  token_env_var: ORC_JIRA_TOKEN
```

Epics map to initiatives by default.

### Sub-task Proposals

Claude can propose follow-up tasks during execution:

```yaml
subtasks:
  allow_creation: true
  auto_approve: false  # Review proposals first
  max_pending: 10
```

## Warnings

**Autonomous execution** — Orc gives Claude full tool access. It will read/write files, run commands, commit, branch, and create PRs. Add human gates or use `--profile strict` if that makes you nervous.

**Token costs** — Large tasks can burn 100K+ tokens. Opus is expensive.

```bash
orc costs
orc costs --by model --since 7d
```

**Worktree cleanup** — Failed tasks keep their worktrees for debugging. Clean up with `orc cleanup` or manually via `git worktree remove`.

## File Layout

```
~/.orc/                                    # Global
├── orc.db                                 # Workflows, agents, costs
├── projects.yaml                          # Project registry
├── config.yaml                            # User config
├── prompts/                               # User prompt overrides
├── token-pool/                            # OAuth accounts
├── projects/<project-id>/                 # Per-project runtime
│   ├── orc.db                             # Tasks, initiatives, transcripts
│   ├── config.yaml                        # Personal project config
│   ├── prompts/                           # Prompt overrides
│   ├── sequences.yaml                     # Task ID sequences
│   └── exports/                           # Backups
└── worktrees/<project-id>/                # Isolated execution
    └── orc-TASK-001/

<project>/.orc/                            # Git-tracked project config
├── config.yaml
├── CONSTITUTION.md
├── prompts/
└── system_prompts/
```

Task data lives in SQLite (or PostgreSQL for teams), not files. Use `orc export` for portable backups.

## Development

### Requirements

- Go 1.24+
- Node.js 22+ and bun (for web UI)

### Setup

```bash
git clone https://github.com/randalmurphal/orc
git clone https://github.com/randalmurphal/llmkit/v2
git clone https://github.com/randalmurphal/flowgraph
git clone https://github.com/randalmurphal/devflow

cd orc
make setup    # Creates go.work, installs deps
make build    # Binary at bin/orc
make test     # Run tests
```

### Web UI Development

```bash
make dev-full  # API (:8080) + frontend (:5173)
make e2e       # Playwright tests
```

### Container

```bash
make dev           # Dev shell
make docker-test   # Tests in container
```

## Documentation

- [Architecture](docs/architecture/OVERVIEW.md)
- [Phase Model](docs/architecture/PHASE_MODEL.md)
- [Gates](docs/architecture/GATES.md)
- [Config Reference](docs/specs/CONFIG_HIERARCHY.md)
- [API Reference](docs/API_REFERENCE.md)
- [Benchmarking](docs/specs/BENCHMARK_SYSTEM.md)
- [File Formats](docs/specs/FILE_FORMATS.md)
- [ADRs](docs/decisions/)

## Credits

The core execution model comes from the **Ralph Wiggum pattern** — persistent loops that iterate until done. The prompt never changes, but the codebase does. Each iteration reads the same instructions but operates on evolved state. Orc wraps this in structure (phases, checkpoints, gates) but the heart is: keep going until you're actually done. See [docs/research/RALPH_WIGGUM.md](docs/research/RALPH_WIGGUM.md).

The visual task management and "describe it, run it, come back to a PR" workflow draws from [Vibe Kanban](https://github.com/BloopAI/vibe-kanban).

## License

MIT
