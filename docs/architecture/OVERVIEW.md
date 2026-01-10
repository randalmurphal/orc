# Orc Architecture Overview

**Purpose**: Intelligent Claude Code orchestrator that scales rigor to task weight with full visibility, git-native checkpointing, and quality gates.

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           ORC SYSTEM                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌───────────┐   ┌──────────┐   ┌─────────┐   ┌──────────┐        │
│  │ Svelte 5  │◄─►│  Go API  │◄─►│ Planner │◄─►│ Executor │        │
│  │ Frontend  │   │  Server  │   │         │   │ (Ralph)  │        │
│  └───────────┘   └──────────┘   └─────────┘   └──────────┘        │
│       │               │              │              │              │
│       │               ▼              ▼              ▼              │
│       │        ┌──────────┐   ┌──────────┐   ┌──────────┐        │
│       │        │  .orc/   │   │  Weight  │   │  Claude  │        │
│       │        │  Files   │   │ Classify │   │   Code   │        │
│       │        └──────────┘   └──────────┘   └──────────┘        │
│       │               │                            │              │
│       │               ▼                            ▼              │
│       │        ┌──────────┐                 ┌──────────┐        │
│       └───────►│   Git    │◄────────────────│Checkpoint│        │
│                │   Repo   │                 │ Manager  │        │
│                └──────────┘                 └──────────┘        │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Core Components

| Component | Language | Responsibility |
|-----------|----------|----------------|
| **CLI** | Go | Task creation, execution control, status display |
| **API Server** | Go | REST API, WebSocket/SSE for live updates |
| **Planner** | Go | Weight classification, phase generation from templates |
| **Executor** | Go | Claude Code invocation, output parsing, completion detection |
| **Checkpoint Manager** | Go | Git commits, branch management, rewind operations |
| **Gate Evaluator** | Go | Quality checks, approval workflow |
| **Frontend** | Svelte 5 | Task list, timeline, transcript viewer, controls |

---

## Data Flow

### Task Lifecycle

```
1. USER CREATES TASK
   └─► Task stored in .orc/tasks/TASK-ID/task.yaml
       └─► Git branch created: orc/TASK-ID

2. WEIGHT CLASSIFICATION (AI)
   └─► Claude classifies: trivial/small/medium/large/greenfield
       └─► User can override

3. PLAN GENERATION
   └─► Template selected based on weight
       └─► Phase sequence stored in .orc/tasks/TASK-ID/plan.yaml

4. PHASE EXECUTION (Ralph loop)
   └─► Phase prompt constructed
       └─► Claude Code invoked (subprocess)
           └─► Output → transcript
               └─► Completion detected → checkpoint commit
                   └─► Gate evaluated
                       ├─► PASS → next phase
                       └─► BLOCK → human review

5. TASK COMPLETION
   └─► All phases complete
       └─► Merge gate (human default)
           └─► Branch merged to main
```

---

## Directory Structure

```
project/
├── .orc/                      # All orc state (git-tracked)
│   ├── config.yaml           # Project configuration
│   ├── tasks/
│   │   └── TASK-001/
│   │       ├── task.yaml     # Task definition
│   │       ├── state.yaml    # Current execution state
│   │       ├── plan.yaml     # Generated plan
│   │       └── transcripts/  # Claude session logs
│   ├── prompts/              # Prompt template overrides
│   └── worktrees/            # Git worktrees (gitignored)
├── .git/
└── orc.yaml                  # Project orc config
```

---

## Technology Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| Backend | Go 1.22+ | Single binary, fast startup, excellent concurrency |
| Frontend | Svelte 5 | Reactive, small bundles, perfect for live streaming |
| State | YAML files | Git-tracked, human-readable, no database needed |
| Version Control | Git | Native checkpointing, branches, worktrees |
| AI Runtime | Claude Code CLI | Subprocess invocation |

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State storage | Files over DB | Git-native, no dependencies |
| Branch strategy | `orc/TASK-ID` | Clear namespace, worktree compatible |
| Weight classification | AI-first | Better than heuristics, user can override |
| Merge gate | Human default | Safety for production codebases |
| Execution model | Ralph-style | Persistent iteration until completion |

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [TASK_MODEL.md](TASK_MODEL.md) | Task structure, weight, lifecycle |
| [PHASE_MODEL.md](PHASE_MODEL.md) | Phase definitions, templates |
| [GIT_INTEGRATION.md](GIT_INTEGRATION.md) | Branches, checkpoints, worktrees |
| [EXECUTOR.md](EXECUTOR.md) | Ralph-style execution loop |
| [GATES.md](GATES.md) | Quality gates, approval workflow |
