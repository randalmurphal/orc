# orc

**In Opus We Trust.**

An intelligent task orchestrator for Claude Code. One orc is dumb. Many orcs working together? That's a warband. Orc doesn't overthink—it classifies your task, picks the right phases, and lets Claude do the work while you do something else.

Trivial bug fix? One phase, done in minutes. Major feature? Full lifecycle with spec, TDD, review, and docs. Orc scales rigor to complexity so you don't waste ceremony on typos or skip steps on things that matter.

> **Fair warning**: This runs Claude autonomously. It will read your code, write code, run commands, and commit to git—all without asking. That's the point. If you want hand-holding, this isn't it. If you want to describe a task and come back to a PR, keep reading.

## The Opus Requirement

Orc delegates judgment to Claude. The workflow selection, the phase execution, the code review—it's all Claude making decisions in loops until the work is done. This works because Opus has the judgment to know when something is actually finished versus "close enough."

Sonnet can implement. Haiku can search. But the orchestration brain? That's Opus. The system is designed around trusting a model that can assess its own work, catch its own mistakes, and know when to ask for help versus when to push through.

You can configure cheaper models for specific phases, but the default is Opus everywhere because half-assed orchestration creates more work than it saves.

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

# Watch it work (optional)
orc serve  # Web UI at localhost:8080
```

That's it. Orc uses the selected workflow to determine phases, executes each phase in a loop until completion, commits after each phase, and opens a PR when done.

## How It Works

### 1. Workflow Selection

Not all tasks deserve the same process. Orc uses workflows to determine which phases run:

| Workflow | What It Means | Phases |
|----------|---------------|--------|
| **trivial** | Typo fix, one-liner | implement |
| **small** | Single component change | tiny_spec → implement → review → docs |
| **medium** | Multiple files, clear scope | spec → tdd_write → tdd_integrate → implement → review → docs |
| **large** | Complex, cross-cutting, new systems | spec → tdd_write → tdd_integrate → breakdown → implement → review → docs |

Key phases:
- **spec/tiny_spec** — generates success criteria and testing requirements
- **tdd_write** — writes failing unit tests *before* implementation
- **tdd_integrate** — writes failing integration tests to prevent dead code
- **breakdown** — decomposes large tasks into checkboxed implementation steps
- **review** — multi-agent code review with specialized reviewers

Select the workflow that matches your task:

```bash
orc new "Refactor the entire auth system" --workflow large
```

### 2. Phase Execution (The Ralph Loop)

Each phase runs in a persistent conversation loop. Claude doesn't execute once and bail—it iterates until the phase is actually done or it hits a wall.

```
implement phase:
  iteration 1: writes initial code
  iteration 2: notices test failure, fixes it
  iteration 3: realizes edge case, handles it
  iteration 4: outputs {"status": "complete", ...}
  → commits, moves to next phase
```

This is why Opus matters. Lesser models don't know when they're done.

### 3. Git-Native Checkpoints

Every task gets a branch. Every phase gets a commit. You can:

- `orc diff TASK-001` — see what changed
- `orc rewind TASK-001 --to implement` — roll back and retry
- `git log orc/TASK-001` — full audit trail

Worktrees keep tasks isolated. Run multiple tasks in parallel without git conflicts.

### 4. Quality Gates

Between phases, gates control flow:

- **auto** — proceed if phase succeeded (default)
- **ai** — Claude evaluates readiness
- **human** — you approve before continuing
- **skip** — skip the phase entirely

Default config: everything auto except merge (human). You review before code hits main.

```bash
orc gates list              # See gate config for all phases
orc gates show implement    # Detailed gate config for a phase
orc approve TASK-001        # Approve a human gate
```

### 5. Automatic Retry

Tests fail? Orc retries from implementation with context about what broke:

```
implement → test (FAIL) → implement (retry with failure context) → test (PASS)
```

No manual intervention unless it's actually stuck.

## Solo Dev Workflow

This is the primary use case. You have work to do. You describe it. Orc does it.

```bash
# Morning: queue up your tasks
orc new "Fix the login redirect bug"           # small
orc new "Add export to CSV feature"            # medium
orc new "Refactor database connection pooling" # large

# Let them run (parallel execution with worktrees)
orc run TASK-001 &
orc run TASK-002 &
orc run TASK-003 &

# Check in later
orc status

# Review and finalize when ready
orc diff TASK-001
orc finalize TASK-001  # Sync with target branch, merge or enable auto-merge
```

### What You Get

For a **medium** task like "Add export to CSV":

1. **Spec phase**: Claude reads your codebase, writes success criteria and testing requirements
2. **TDD Write phase**: Failing unit tests written before implementation
3. **TDD Integrate phase**: Failing integration tests to ensure new code is wired in
4. **Implement phase**: Code written, iterating until all tests pass
5. **Review phase**: Multi-round self-review catching issues
6. **Docs phase**: README/docs updated

You did zero of it.

## Configuration

Config loads from multiple sources (later overrides earlier):

1. Built-in defaults
2. `~/.orc/config.yaml` (user)
3. `.orc/config.yaml` (project)
4. Environment variables (`ORC_*`)

### Essential Options

```yaml
# .orc/config.yaml

# Automation level
profile: auto  # auto | fast | safe | strict

# Gate control
gates:
  default_type: auto
  phase_overrides:
    merge: human  # You approve merges

# What happens when done
completion:
  action: pr           # pr | merge | none
  target_branch: main
  sync:
    strategy: completion   # none | phase | completion | detect
    fail_on_conflict: true # Abort vs warn on conflicts
  pr:
    auto_merge: false  # Set true to merge after finalize
    draft: false
    labels: []
    reviewers: []

# Worktree isolation (parallel execution)
worktree:
  enabled: true
  cleanup_on_complete: true
  cleanup_on_fail: false  # Keep for debugging

# Retry behavior
retry:
  enabled: true
  retry_map:
    test: implement      # Test failures retry from implement
    validate: implement

executor:
  max_retries: 5         # Max retry attempts per phase (default: 5)

# Code review
review:
  enabled: true
  rounds: 2  # Exploratory + validation

# Hosting provider (for PRs)
hosting:
  provider: github  # github | gitlab
```

See [CONFIG_HIERARCHY.md](docs/specs/CONFIG_HIERARCHY.md) for all options.

### Automation Profiles

| Profile | Use Case | Human Gates |
|---------|----------|-------------|
| `auto` | Solo dev, CI/CD | merge only |
| `fast` | Prototypes, experiments | none |
| `safe` | Default for teams | merge |
| `strict` | Critical code | spec + merge |

```bash
orc run TASK-001 --profile strict
```

### Environment Overrides

```bash
ORC_PROFILE=strict
ORC_MODEL=opus  # Short names: opus, sonnet, haiku
ORC_GATES_DEFAULT=human
ORC_WORKTREE_ENABLED=false
ORC_RETRY_ENABLED=false
```

## Commands

**Every command has detailed `--help` with quality guidance, common mistakes, and data flow explanations.**

### Task Lifecycle

```bash
orc new "title"              # Create task with default workflow
orc new "title" --workflow X # Select specific workflow (trivial/small/medium/large)
orc new "title" -d "desc"   # Add description (flows into every phase prompt)
orc new "title" -i INIT-001 # Link to initiative
orc run TASK-ID              # Execute task
orc run TASK-ID --profile X  # Execute with specific profile
orc stop TASK-ID             # Stop/abort running task
orc pause TASK-ID            # Pause with checkpoint
orc resume TASK-ID           # Continue paused/failed task
orc rewind TASK-ID --to X    # Roll back to phase X
orc reset TASK-ID            # Clear progress for fresh retry
orc skip TASK-ID --phase X   # Skip a phase
orc close TASK-ID            # Close task permanently
orc finalize TASK-ID         # Sync with target branch, resolve conflicts, merge
orc approve TASK-ID          # Approve human gate
orc reject TASK-ID           # Reject at gate
orc delete TASK-ID           # Delete task
```

### Inspection

```bash
orc status                   # Dashboard: what needs attention
orc status --watch           # Live updating dashboard
orc show TASK-ID             # Task details, spec, state
orc show TASK-ID --gates     # Include gate history
orc log TASK-ID              # View Claude transcripts
orc log TASK-ID --follow     # Stream live transcript
orc diff TASK-ID             # View git diff
orc deps TASK-ID             # Show dependencies (--tree, --graph)
orc costs                    # Cost report (--by user/project/model, --since)
orc scratchpad TASK-ID       # View phase observations and decisions
orc search "query"           # Search tasks
orc list                     # List tasks (alias: ls)
```

### Initiatives

Group related tasks with shared vision and decisions:

```bash
orc initiative new "Auth overhaul" --vision "JWT-based auth with refresh tokens"
orc initiative decide INIT-001 "Use bcrypt" --rationale "Industry standard"
orc initiative link INIT-001 TASK-001 TASK-002  # Batch link tasks
orc initiative run INIT-001                     # Run all ready tasks in order
orc initiative list                             # List all initiatives
orc initiative show INIT-001                    # Show details
orc initiative edit INIT-001 --status active    # Edit properties
```

Initiative **vision** and **decisions** flow into every linked task's prompts, keeping Claude aligned across multiple tasks.

### Multi-Project

```bash
orc init                     # Register project globally
orc serve                    # API + Web UI for all projects
orc projects                 # List registered projects
orc projects add .           # Register current directory
orc projects remove ID       # Unregister
orc projects default ID      # Set default project
```

Use `--project/-P` flag or `ORC_PROJECT` env var to select a project for any command.

### Import / Export

```bash
orc export --all-tasks       # Full backup (tar.gz)
orc export --all-tasks --initiatives  # Include initiatives
orc export --all-tasks --minimal      # No transcripts
orc import                   # Restore from backup (auto-detect format)
orc import --dry-run         # Preview without changes
orc import jira              # Import from Jira Cloud
orc import jira --project X  # Specific project(s)
orc import jira --jql "..."  # Filter with JQL
```

### Configuration

```bash
orc config show --source     # Show config with resolution sources
orc config get <key>         # Get specific value
orc config set <key> <val>   # Set config value
orc config edit              # Edit in $EDITOR
orc constitution show        # View project constitution
orc constitution set --file X # Set from file
orc gates list               # Show gate config for all phases
orc gates show <phase>       # Gate config for specific phase
```

### Benchmarking

Evaluate model performance across workflows:

```bash
orc bench curate import suite.yaml   # Import benchmark suite
orc bench run --baseline --trials 3  # Run all-Opus baseline
orc bench run --variant ID --trials 3 # Run specific variant
orc bench report                     # Phase leaderboard + recommendations
orc bench judge                      # Cross-model judge panel
```

### Token Pool (Rate Limit Failover)

```bash
orc pool init                # Initialize pool
orc pool add personal        # Add OAuth account
orc pool add work            # Add another
orc pool list                # View accounts
orc pool status              # Check exhaustion
orc pool remove personal     # Remove account
orc pool switch work         # Switch active account
```

## Web UI

```bash
orc serve  # localhost:8080
```

- **Dashboard**: Running tasks, recent activity, quick stats
- **Task detail**: Timeline, live transcript, controls
- **Task board**: Drag-and-drop Kanban with initiative filtering
- **Config**: Edit settings, prompts, hooks
- **Multi-project**: Switch between registered projects
- **Workflow editor**: Visual workflow designer (React Flow)

### Keyboard Shortcuts

Uses `Shift+Alt` modifier to avoid browser conflicts.

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
| `g e` | Go to environment |
| `/` | Focus search |
| `?` | Show shortcuts help |

## Advanced Features

### Worktree Isolation

Tasks run in isolated git worktrees at `~/.orc/worktrees/<project-id>/`. Multiple tasks execute in parallel without stepping on each other.

```
~/.orc/worktrees/<project-id>/
├── orc-TASK-001/  # Independent working copy
├── orc-TASK-002/  # Can run simultaneously
└── orc-TASK-003/  # No git conflicts
```

### Branch Synchronization

Parallel tasks can diverge from main, causing merge conflicts at completion. Orc syncs task branches with the target branch to catch conflicts early.

```yaml
# .orc/config.yaml
completion:
  sync:
    strategy: completion     # When to sync (none, phase, completion, detect)
    fail_on_conflict: true   # Abort on conflicts vs warn and continue
```

**Strategies:**
- `none` — No automatic sync, manual only
- `phase` — Sync before each phase (maximum safety)
- `completion` — Sync before PR/merge (default, balanced)
- `detect` — Check for conflicts without resolving (fail-fast)

When conflicts are detected with `fail_on_conflict: true`, the task fails with a clear message listing conflicting files and resolution options.

### Constitution

Project-level principles injected into all phase prompts. Use this to enforce coding standards, architectural decisions, or domain rules across all tasks.

```bash
orc constitution set --file coding-standards.md
orc constitution show
orc constitution delete
```

Stored at `.orc/CONSTITUTION.md` (git-tracked).

### Token Pool

Hit rate limits? Add multiple OAuth accounts. Orc rotates automatically.

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

After implementation:

1. **Round 1 (Exploratory)**: Identifies gaps, security issues, architectural concerns
2. **Round 2 (Validation)**: Verifies Round 1 issues were addressed

Includes no-op detection (catches implementations that don't actually change behavior) and success criteria verification.

### PR Status Detection

When tasks create PRs, orc polls the hosting provider (GitHub or GitLab) for status updates:

- Review state (pending, changes requested, approved)
- CI check status (pending, success, failure)
- Mergeability status
- Review and approval counts

Status is stored in the database and visible in the web UI.

### Task Completion Flow

1. Task completes → PR created (or existing PR reused) on GitHub/GitLab
2. Review PR manually
3. `orc finalize TASK-ID` → syncs with target branch, resolves conflicts, optionally enables auto-merge

PR creation is idempotent—if an open PR already exists on the task branch, it's reused rather than duplicated.

### Sub-task Proposals

During implementation, Claude can propose follow-up tasks:

```yaml
subtasks:
  allow_creation: true
  auto_approve: false  # You review proposals
  max_pending: 10
```

### Jira Integration

Import Jira Cloud issues as orc tasks. Epics map to initiatives by default.

```bash
orc import jira --url https://your-org.atlassian.net --email you@org.com --token $JIRA_TOKEN
```

Or configure in `.orc/config.yaml`:

```yaml
jira:
  url: https://your-org.atlassian.net
  email: you@org.com
  token_env_var: ORC_JIRA_TOKEN
```

### Dependencies

Tasks support `blocked_by` (must complete first) and `related_to` (informational):

```bash
orc new "Part 2" --blocked-by TASK-001
orc deps TASK-001 --tree    # Visualize dependency tree
orc deps TASK-001 --graph   # Graph format
```

Initiatives also support `blocked_by` for ordering.

## Warnings

### Autonomous Execution

Orc runs Claude with full tool access. It will:
- Read and write files in your project
- Execute shell commands
- Make git commits
- Create branches and PRs

This is by design. If you're not comfortable with autonomous AI execution, add more human gates or use `--profile strict`.

### Token Costs

Large tasks can use 100K+ tokens. Opus isn't cheap. Monitor costs:

```bash
orc costs                        # Cost summary
orc costs --by model --since 7d  # Breakdown by model, last week
```

### Phase Completion Detection

Phases complete when Claude outputs structured JSON:
```json
{"status": "complete", "summary": "..."}
```

Blocked phases output `{"status": "blocked", "reason": "..."}`. If Claude doesn't output a completion signal, the phase loops until max iterations.

### Worktree Cleanup

Failed tasks keep worktrees for debugging. Clean up:

```bash
orc cleanup                  # Clean orphaned worktrees
# or manually:
git worktree list
git worktree remove ~/.orc/worktrees/<project-id>/orc-TASK-001
```

## File Layout

```
~/.orc/                                    # Global (cross-project)
├── orc.db                                 # GlobalDB: workflows, agents, costs
├── projects.yaml                          # Project registry
├── config.yaml                            # User-level config
├── prompts/                               # User prompt overrides
├── token-pool/                            # OAuth accounts
├── projects/<project-id>/                 # Per-project runtime
│   ├── orc.db                             # ProjectDB: tasks, initiatives, transcripts
│   ├── config.yaml                        # Personal project config
│   ├── prompts/                           # Personal project prompt overrides
│   ├── sequences.yaml                     # Task ID sequences
│   └── exports/                           # Backup archives
└── worktrees/<project-id>/                # Isolated worktree execution
    ├── orc-TASK-001/
    ├── orc-TASK-002/
    └── orc-TASK-003/

<project>/.orc/                            # Config-only (git-tracked)
├── config.yaml                            # Project config
├── CONSTITUTION.md                        # Project principles
├── prompts/                               # Project prompt templates
└── system_prompts/                        # System prompt overrides
```

All task data is stored in SQLite databases (or PostgreSQL for team mode), not individual files. Use `orc export` for portable backups.

## Development

### Requirements

- Go 1.24+
- Node.js 22+ and bun (for frontend)

### Setup

```bash
# Clone with dependencies
git clone https://github.com/randalmurphal/orc
git clone https://github.com/randalmurphal/llmkit
git clone https://github.com/randalmurphal/flowgraph
git clone https://github.com/randalmurphal/devflow

cd orc
make setup    # Creates go.work, installs deps
make build    # Binary at bin/orc
make test     # Run tests
```

### Web UI

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

## Standing on Shoulders

Orc didn't emerge from nothing. Key ideas came from:

### Ralph Wiggum Technique

The core execution model—persistent loops that iterate until done—comes from the Ralph Wiggum pattern. The insight: the prompt never changes, but the codebase does. Each iteration reads the same instructions but operates on evolved state.

```bash
while :; do cat PROMPT.md | claude-code; done
```

Simple systems fail simply. You can debug a bash loop. Orc wraps this in structure (phases, checkpoints, gates) but the heart is Ralph: keep going until you're actually done.

See [docs/research/RALPH_WIGGUM.md](docs/research/RALPH_WIGGUM.md) for the full breakdown.

### Vibe Kanban

The visual task management and "describe it, run it, come back to a PR" workflow draws from [Vibe Kanban](https://github.com/BloopAI/vibe-kanban)—a project that demonstrated AI-driven task execution with a Kanban interface. The idea that you queue up work, let AI execute, and review results later shaped orc's solo dev workflow.

## License

MIT

---

*Many orc strong together.*
