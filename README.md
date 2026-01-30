# orc

**In Opus We Trust.**

An intelligent task orchestrator for Claude Code. One orc is dumb. Many orcs working together? That's a warband. Orc doesn't overthink—it classifies your task, picks the right phases, and lets Claude do the work while you do something else.

Trivial bug fix? One phase, done in minutes. Major feature? Full lifecycle with spec, review, tests, and docs. Orc scales rigor to complexity so you don't waste ceremony on typos or skip steps on things that matter.

> **Fair warning**: This runs Claude autonomously. It will read your code, write code, run commands, and commit to git—all without asking. That's the point. If you want hand-holding, this isn't it. If you want to describe a task and come back to a PR, keep reading.

## The Opus Requirement

Orc delegates judgment to Claude. The weight classification, the phase execution, the code review—it's all Claude making decisions in loops until the work is done. This works because Opus has the judgment to know when something is actually finished versus "close enough."

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

That's it. Orc classifies the task weight, generates a phase plan, executes each phase in a loop until completion, commits after each phase, and opens a PR when done.

## How It Works

### 1. Weight Classification

Not all tasks deserve the same process. Orc classifies tasks into weights, and weights determine phases:

| Weight | What It Means | Phases |
|--------|---------------|--------|
| **trivial** | Typo fix, one-liner | tiny_spec → implement |
| **small** | Single component change | tiny_spec → implement → review |
| **medium** | Multiple files, clear scope | spec → tdd_write → implement → review → docs |
| **large** | Complex, cross-cutting, new systems | spec → tdd_write → breakdown → implement → review → docs → validate |

AI classifies automatically. Override when you know better:

```bash
orc new "Refactor the entire auth system" --weight large
```

### 2. Phase Execution (The Ralph Loop)

Each phase runs in a persistent conversation loop. Claude doesn't execute once and bail—it iterates until the phase is actually done or it hits a wall.

```
implement phase:
  iteration 1: writes initial code
  iteration 2: notices test failure, fixes it
  iteration 3: realizes edge case, handles it
  iteration 4: outputs <phase_complete>true</phase_complete>
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

Default config: everything auto except merge (human). You review before code hits main.

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

# Review and merge when ready
orc diff TASK-001
orc approve TASK-001  # approves merge gate
```

### What You Get

For a **medium** task like "Add export to CSV":

1. **Spec phase**: Claude reads your codebase, writes a spec for the feature
2. **Implement phase**: Code written, iterating until tests pass
3. **Review phase**: Two rounds of self-review catching issues
4. **Test phase**: Comprehensive tests written
5. **Docs phase**: README/docs updated

Total time: 30-90 minutes depending on complexity. You did zero of it.

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
  pr:
    auto_merge: false  # Set true to merge after finalize

# Worktree isolation (parallel execution)
worktree:
  enabled: true
  cleanup_on_complete: true
  cleanup_on_fail: false  # Keep for debugging

# Branch sync (catches conflicts early)
sync:
  strategy: completion   # none | phase | completion | detect
  fail_on_conflict: true # Abort vs warn on conflicts

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
```

### Automation Profiles

| Profile | Use Case | Human Gates |
|---------|----------|-------------|
| `auto` | Solo dev, CI/CD | merge only |
| `fast` | Prototypes, experiments | none |
| `safe` | Default for teams | merge |
| `strict` | Critical code | spec + merge |

```bash
orc run TASK-001 --profile strict
orc config profile safe  # Set default
```

### Environment Overrides

```bash
ORC_PROFILE=strict
ORC_MODEL=claude-opus-4-5-20251101  # Default, recommended
ORC_GATES_DEFAULT=human
ORC_WORKTREE_ENABLED=false
ORC_RETRY_ENABLED=false
```

## Commands

### Task Lifecycle

```bash
orc new "title"              # Create task (AI classifies weight)
orc new "title" --weight X   # Override weight classification
orc new "title" --category X # Set category (feature/bug/refactor/chore/docs/test)
orc run TASK-ID              # Execute task
orc run TASK-ID --profile X  # Execute with specific profile
orc pause TASK-ID            # Pause with checkpoint
orc resume TASK-ID           # Continue from checkpoint
orc rewind TASK-ID --to X    # Roll back to phase X
orc reset TASK-ID            # Clear progress for fresh retry
orc resolve TASK-ID          # Mark failed task as resolved
orc approve TASK-ID          # Approve human gate
orc delete TASK-ID           # Delete task and files
```

### Inspection

```bash
orc status                   # Show all tasks
orc log TASK-ID              # View transcripts
orc diff TASK-ID             # View git diff
orc config show --source     # Show config with sources
```

### Multi-Project

```bash
orc init                     # Register project globally
orc serve                    # API + Web UI for all projects
```

### Token Pool (Rate Limit Failover)

```bash
orc pool init                # Initialize pool
orc pool add personal        # Add OAuth account
orc pool add work            # Add another
orc pool list                # View accounts
orc pool status              # Check exhaustion
```

## Web UI

```bash
orc serve  # localhost:8080
```

- **Dashboard**: Running tasks, recent activity, quick stats
- **Task detail**: Timeline, live transcript, controls
- **Config**: Edit settings, prompts, hooks
- **Multi-project**: Switch between registered projects

### Keyboard Shortcuts

Uses `Shift+Alt` modifier (⇧⌥ on Mac) to avoid browser conflicts with Cmd+K, Cmd+N, etc.

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

Tasks run in isolated git worktrees. Multiple tasks execute in parallel without stepping on each other.

```
.orc/worktrees/
├── TASK-001/  # Independent working copy
├── TASK-002/  # Can run simultaneously
└── TASK-003/  # No git conflicts
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

After implementation, before tests:

1. **Round 1 (Exploratory)**: Identifies gaps, security issues, architectural concerns
2. **Round 2 (Validation)**: Verifies Round 1 issues were addressed

### PR Status Detection

When tasks create PRs, orc polls the hosting provider (GitHub or GitLab) for status updates:

- Review state (pending, changes requested, approved)
- CI check status (pending, success, failure)
- Mergeability status
- Review and approval counts

Status is stored in the database and visible in the web UI. Polling runs every 60 seconds for tasks with open PRs, with a 30-second rate limit per task.

```bash
# Manual refresh via Connect RPC
orc.v1.HostingService/RefreshPR
```

### Sub-task Proposals

During implementation, Claude can propose follow-up tasks:

```yaml
subtasks:
  allow_creation: true
  auto_approve: false  # You review proposals
  max_pending: 10
```

### Initiatives

Group related tasks:

```bash
orc initiative new "Authentication overhaul"
orc initiative add-task INIT-001 TASK-001
orc initiative add-task INIT-001 TASK-002
orc initiative run INIT-001  # Run all in order
```

### Claude Code Integration

Orc installs slash commands into Claude Code:

| Command | Purpose |
|---------|---------|
| `/orc:init` | Initialize or create spec |
| `/orc:continue` | Resume from checkpoint |
| `/orc:status` | Show progress |
| `/orc:review` | Start code review |
| `/orc:propose` | Queue sub-task |

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
# API endpoint for cost tracking
GET /api/cost/summary?period=week
```

### Phase Completion Detection

Phases complete when Claude outputs:
```xml
<phase_complete>true</phase_complete>
```

If Claude doesn't output this marker, the phase loops until max_iterations. Clear prompts with explicit completion criteria help.

### Worktree Cleanup

Failed tasks keep worktrees for debugging. Clean up manually:

```bash
git worktree list
git worktree remove .orc/worktrees/TASK-001
```

## File Layout

```
~/.orc/
├── config.yaml          # User config
├── projects.yaml        # Global registry
├── orc.db               # Cost tracking, metadata
└── token-pool/          # OAuth accounts

.orc/                    # Per-project
├── config.yaml
├── orc.db               # Tasks, transcripts
├── tasks/TASK-001/
│   ├── task.yaml        # Definition
│   ├── plan.yaml        # Phase sequence
│   ├── state.yaml       # Execution state
│   └── transcripts/     # Claude logs
├── prompts/             # Override defaults
└── worktrees/           # Isolated execution

.claude/
├── CLAUDE.md            # Project instructions
├── plugins/orc/         # Slash commands
└── skills/              # Custom skills
```

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
- [CLI Spec](docs/specs/CLI.md)
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
