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
│  │ React 19  │◄─►│  Go API  │◄─►│ Planner │◄─►│ Executor │        │
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
| **API Server** | Go | REST API, WebSocket for live updates, event persistence |
| **Planner** | Go | Weight classification, phase generation from templates |
| **Executor** | Go | Claude Code invocation, output parsing, completion detection |
| **Checkpoint Manager** | Go | Git commits, branch management, rewind operations |
| **Gate Evaluator** | Go | Quality checks, approval workflow |
| **Frontend** | React 19 | Task board, timeline, transcript viewer, controls |

---

## Data Flow

### Task Lifecycle

```
1. USER CREATES TASK
   └─► Task stored in SQLite (~/.orc/projects/<id>/orc.db)
       └─► Git branch created: orc/TASK-ID

2. WEIGHT CLASSIFICATION (AI)
   └─► Claude classifies: trivial/small/medium/large/greenfield
       └─► User can override

3. PLAN GENERATION
   └─► Template selected based on weight
       └─► Phase sequence stored in database

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
├── .orc/                      # All orc state
│   ├── orc.db                # SQLite database (source of truth)
│   ├── config.yaml           # Project configuration
│   ├── tasks/
│   │   └── TASK-001/
│   │       └── transcripts/  # Claude session logs (markdown exports)
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
| Frontend | React 19 | Modern React with concurrent features |
| State | SQLite | Single-file database, no external dependencies |
| Real-time | WebSocket | Persistent events stored in `event_log` table |
| Version Control | Git | Native checkpointing, branches, worktrees |
| AI Runtime | Claude Code CLI | Subprocess invocation |

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State storage | SQLite | Single-file database, portable, no server needed |
| Event persistence | Batched writes | 10 events or 5s buffer reduces DB load |
| Branch strategy | `orc/TASK-ID` | Clear namespace, worktree compatible |
| Weight classification | AI-first | Better than heuristics, user can override |
| Merge gate | Human default | Safety for production codebases |
| Execution model | Ralph-style | Persistent iteration until completion |

---

## Multi-Project Architecture

Orc supports multiple projects from a single server instance using a two-tier database model.

### Database Tiers

| Tier | Instance | Stores |
|------|----------|--------|
| **GlobalDB** | One per server | Project registry, built-in workflows, built-in agents, hook scripts, skills |
| **ProjectDB** | One per project | Tasks, initiatives, transcripts, events, config |

### Connection Management

`ProjectCache` (`internal/api/project_cache.go`) provides LRU-cached access to project databases. All API services resolve the correct backend via `getBackend(projectID)`, which looks up or opens the project's `ProjectDB` through the cache. Evicted connections are closed automatically.

### Request Routing

All project-scoped API requests include a `project_id` field. The `project/` package (`internal/project/`) maintains the project registry in `GlobalDB`. The frontend stores the active project in `projectStore` and passes `projectId` on every request via `DataProvider`.

---

## Related Documents

| Document | Purpose |
|----------|---------|
| [TASK_MODEL.md](TASK_MODEL.md) | Task structure, weight, lifecycle |
| [PHASE_MODEL.md](PHASE_MODEL.md) | Phase definitions, templates |
| [GIT_INTEGRATION.md](GIT_INTEGRATION.md) | Branches, checkpoints, worktrees |
| [EXECUTOR.md](EXECUTOR.md) | Ralph-style execution loop |
| [GATES.md](GATES.md) | Quality gates, approval workflow |
