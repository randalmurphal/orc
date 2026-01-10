# Task Model

**Purpose**: Define task structure, weight classification, and lifecycle states.

---

## Task Definition

```yaml
# .orc/tasks/TASK-001/task.yaml
id: TASK-001
title: "Add user authentication"
description: |
  Implement OAuth2 authentication with Google and GitHub providers.
  Should integrate with existing user model.

weight: medium           # trivial | small | medium | large | greenfield
status: running          # created | classifying | planned | running | paused | blocked | completed | failed
branch: orc/TASK-001

created_at: 2026-01-10T10:30:00Z
created_by: randy
updated_at: 2026-01-10T12:45:00Z

metadata:
  source: cli            # cli | api | import
  tags: [auth, feature]
```

---

## Weight Classification

| Weight | Scope | Duration | Phases |
|--------|-------|----------|--------|
| **trivial** | 1 file, <10 lines | Minutes | implement |
| **small** | 1 component, <100 lines | <1 hour | implement → test |
| **medium** | Multiple files | Hours | spec → implement → review → test |
| **large** | Cross-cutting | Days | research → spec → design → implement → review → test |
| **greenfield** | New system | Weeks | research → spec → design → scaffold → implement → test → docs |

### Classification Criteria

| Dimension | Trivial | Small | Medium | Large | Greenfield |
|-----------|---------|-------|--------|-------|------------|
| Files | 1-2 | 1-5 | 3-10 | 10+ | New structure |
| Uncertainty | None | Low | Medium | High | Very high |
| Risk | None | Low | Medium | High | High |
| Dependencies | None | Internal | Some external | Many | Unknown |

---

## Status Lifecycle

```
created ──► classifying ──► planned ──► running ◄─┐
                                          │       │
                                          ▼       │
                                       paused ────┘
                                          │
                              ┌───────────┼───────────┐
                              ▼           ▼           ▼
                          completed    failed      blocked
```

| Status | Description | Transitions To |
|--------|-------------|----------------|
| `created` | Task defined, not started | classifying |
| `classifying` | AI determining weight | planned |
| `planned` | Plan generated, ready to run | running |
| `running` | Actively executing phases | paused, completed, failed, blocked |
| `paused` | Manually paused | running |
| `blocked` | Waiting for human gate | running |
| `completed` | All phases done, merged | - |
| `failed` | Unrecoverable error | - |

---

## Task State

```yaml
# .orc/tasks/TASK-001/state.yaml
current_phase: implement
current_iteration: 3
status: running

phases:
  classify:
    status: completed
    started_at: 2026-01-10T10:31:00Z
    completed_at: 2026-01-10T10:32:00Z
    result:
      weight: medium
      confidence: 0.88
    checkpoint: abc123

  spec:
    status: completed
    started_at: 2026-01-10T10:32:00Z
    completed_at: 2026-01-10T10:45:00Z
    iterations: 2
    checkpoint: def456

  implement:
    status: running
    started_at: 2026-01-10T10:45:00Z
    iterations: 3
    last_checkpoint: ghi789

gates:
  - phase: spec
    type: ai
    decision: approved
    timestamp: 2026-01-10T10:45:00Z

errors: []
```

---

## Task Operations

| Operation | Command | Effect |
|-----------|---------|--------|
| Create | `orc new "title"` | Creates task.yaml, branch |
| Run | `orc run TASK-001` | Starts/resumes execution |
| Pause | `orc pause TASK-001` | Checkpoints, stops execution |
| Rewind | `orc rewind TASK-001 --to spec` | Resets to checkpoint |
| Approve | `orc approve TASK-001` | Passes human gate |
| Abort | `orc abort TASK-001` | Marks failed, cleans up |
