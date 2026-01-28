# Bidirectional Sync: Files ↔ UI

> **Status: SUPERSEDED** (January 2026)
>
> This spec was written for the original YAML-based storage model where files were the source of truth.
> The storage model was migrated to pure SQLite in January 2026 - see `ADR-002-storage-model.md`.
> File-based synchronization is no longer applicable; the database is now the sole source of truth.

## Summary

Enable true bidirectional synchronization between the file system and web UI, where files remain the source of truth but UI actions write back to files.

## Problem Statement

Currently:
- Tasks created via CLI don't reflect file artifacts (e.g., spec.md exists but task shows in Queued)
- Dragging tasks on the board is UI-only - doesn't persist to files
- No way to manually advance task state without running executor
- Users can create artifacts manually but state doesn't reflect this

## Design Principles

1. **Files = Source of Truth** - Always. UI is a view into files.
2. **UI Actions Write to Files** - Drag operations update task.yaml
3. **Guide, Don't Block** - Show what's needed to move forward, don't prevent exploration
4. **Confirm Destructive Actions** - Moving backward, deleting, manual completion
5. **Respect Manual Work** - If user created spec.md manually, don't overwrite or ignore it

## Architecture

### Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         File System                              │
│  .orc/tasks/TASK-001/                                           │
│  ├── task.yaml      ←─────────────────────┐                     │
│  ├── state.yaml     ←─────────────────────┤                     │
│  ├── plan.yaml                            │                     │
│  └── spec.md                              │                     │
└─────────────────────────────────────────────│─────────────────────┘
         │                                   │
         │ File Watcher (fsnotify)           │ API Write
         ▼                                   │
┌─────────────────────────────────────────────│─────────────────────┐
│                      API Server             │                     │
│  ┌─────────────┐    ┌─────────────┐    ┌───┴───────┐            │
│  │ GET /tasks  │    │ File Watcher│    │PATCH /task│            │
│  │ (read)      │    │ (publish)   │    │ (write)   │            │
│  └─────────────┘    └─────────────┘    └───────────┘            │
└─────────────────────────────────────────────────────────────────┘
         │                   │                   ▲
         │ REST              │ WebSocket         │ REST
         ▼                   ▼                   │
┌─────────────────────────────────────────────────────────────────┐
│                        Web UI                                    │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ Task Store  │◄───│ WS Handler  │    │ Drag Handler│─────────┘
│  │ (reactive)  │    │ (subscribe) │    │ (write API) │          │
│  └─────────────┘    └─────────────┘    └─────────────┘          │
│         │                                     │                  │
│         ▼                                     │                  │
│  ┌─────────────┐    ┌─────────────┐          │                  │
│  │ Board View  │◄───│ Undo Stack  │◄─────────┘                  │
│  └─────────────┘    └─────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
```

### Column Derivation Logic

Current (broken):
```
column = current_phase || 'queued'
```

Proposed:
```go
func DeriveColumn(task Task, artifacts Artifacts) Column {
    // Terminal states always Done
    if task.Status in [completed, failed] {
        return Done
    }

    // Explicit phase takes precedence
    if task.CurrentPhase != "" {
        return phaseToColumn(task.CurrentPhase)
    }

    // Derive from artifacts if no explicit phase
    if artifacts.HasSpec {
        return Spec  // Or Implement if spec is "complete"
    }

    return Queued
}
```

### Artifact Detection

| Artifact | Indicates | Column Hint |
|----------|-----------|-------------|
| `spec.md` exists | Spec content present | Spec or later |
| `spec.md` + implementation commits | Past spec | Implement or later |
| Test files in worktree | Tests written | Test or later |
| `state.yaml` has completed phases | Phase history | Use last completed + 1 |

### Move Validation & Indicators

When hovering/dragging, show what's required:

| From | To | Indicator |
|------|-----|-----------|
| Queued | Spec | "Will set phase to 'spec'" |
| Queued | Implement | "Requires: spec.md (create or skip)" |
| Spec | Implement | "Will mark spec complete" |
| Implement | Spec | ⚠️ "Reset to spec phase? Progress may be lost" |
| Any | Done | ⚠️ "Mark complete without validation?" |

### API Changes

#### New RPC: MoveTask

```protobuf
// In orc/v1/task.proto
rpc MoveTask(MoveTaskRequest) returns (MoveTaskResponse);

message MoveTaskRequest {
  string task_id = 1;
  string target_column = 2;
  bool skip_requirements = 3;
}

message MoveTaskResponse {
  Task task = 1;
  repeated string skipped_phases = 2;
  repeated string warnings = 3;
}
```

#### New RPC: UndoTaskMove

```protobuf
rpc UndoTaskMove(UndoTaskMoveRequest) returns (UndoTaskMoveResponse);

message UndoTaskMoveRequest {
  string action_id = 1;
}

message UndoTaskMoveResponse {
  Task task = 1;
  string previous_phase = 2;
  string previous_status = 3;
}
```

### Undo/Redo Stack

Client-side stack with server-side validation:

```typescript
interface UndoableAction {
    id: string;
    type: 'move' | 'delete' | 'create';
    taskId: string;
    before: Partial<Task>;
    after: Partial<Task>;
    timestamp: number;
}

class UndoStack {
    private undoStack: UndoableAction[] = [];
    private redoStack: UndoableAction[] = [];

    push(action: UndoableAction): void;
    undo(): Promise<void>;  // Calls API to revert
    redo(): Promise<void>;  // Calls API to reapply
    canUndo(): boolean;
    canRedo(): boolean;
}
```

Keyboard shortcuts:
- `Cmd+Z` - Undo
- `Cmd+Shift+Z` - Redo

### Executor Integration

When running a task that has been manually moved:

```go
func (e *Executor) Run(task Task, plan Plan) error {
    for _, phase := range plan.Phases {
        // Check if phase should be skipped
        if e.shouldSkipPhase(task, phase) {
            reason := e.getSkipReason(task, phase)

            if e.config.AutoSkip {
                e.logger.Info("auto-skipping phase", "phase", phase, "reason", reason)
                continue
            }

            // Prompt user
            skip, err := e.promptSkipPhase(phase, reason)
            if err != nil {
                return err
            }
            if skip {
                continue
            }
        }

        // Run phase normally
        if err := e.runPhase(task, phase); err != nil {
            return err
        }
    }
    return nil
}

func (e *Executor) shouldSkipPhase(task Task, phase Phase) bool {
    switch phase.ID {
    case "spec":
        return e.artifactExists(task, "spec.md")
    case "test":
        return e.hasTestFiles(task)
    default:
        return false
    }
}
```

### Task Move Implementation

When UI moves a task via Connect RPC:

```go
func (s *taskServer) MoveTask(
    ctx context.Context,
    req *connect.Request[orcv1.MoveTaskRequest],
) (*connect.Response[orcv1.MoveTaskResponse], error) {
    task, err := s.backend.LoadTask(req.Msg.TaskId)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, err)
    }

    // Validate move
    validation := s.validateMove(task, req.Msg.TargetColumn)
    if !validation.Allowed && !req.Msg.SkipRequirements {
        return nil, connect.NewError(connect.CodeFailedPrecondition,
            fmt.Errorf("requirements not met: %v", validation.Missing))
    }

    // Record for undo
    before := task.Clone()

    // Update task state
    task.CurrentPhase = columnToPhase(req.Msg.TargetColumn)
    task.Status = deriveStatus(req.Msg.TargetColumn)
    task.UpdatedAt = timestamppb.Now()

    // Save to database
    if err := s.backend.SaveTask(task); err != nil {
        return nil, connect.NewError(connect.CodeInternal, err)
    }

    // Publish event for real-time updates
    s.publisher.Publish(events.TaskUpdated(task))

    return connect.NewResponse(&orcv1.MoveTaskResponse{
        Task:          task,
        SkippedPhases: validation.SkippedPhases,
    }), nil
}
```

## Implementation Phases

### Phase 1: Read-side improvements (Quick Win)
- [ ] Derive column from artifacts, not just current_phase
- [ ] Show indicators for what's needed to move
- [ ] Load artifact presence when fetching tasks

### Phase 2: Write-side (Core Feature)
- [ ] PATCH /api/tasks/{id}/move endpoint
- [ ] Drag-drop calls API instead of being UI-only
- [ ] Confirmation modals for backward/destructive moves
- [ ] File writes trigger WebSocket updates (already done)

### Phase 3: Undo/Redo
- [ ] Client-side undo stack
- [ ] POST /api/tasks/{id}/undo endpoint
- [ ] Keyboard shortcuts (Cmd+Z, Cmd+Shift+Z)
- [ ] Visual feedback for undo/redo

### Phase 4: Executor Integration
- [ ] Detect existing artifacts before running phase
- [ ] Prompt to skip phases with existing artifacts
- [ ] Config option for auto-skip behavior
- [ ] Record skipped phases in state.yaml

## Open Questions

1. **Conflict resolution**: If file changes while undo is in flight, what happens?
   - Proposed: Last write wins, show toast if conflict detected

2. **Multi-user**: If team mode is enabled, how do concurrent moves work?
   - Proposed: Defer to Phase 2 of team mode spec

3. **Artifact validation**: How do we know if spec.md is "complete" vs just created?
   - Proposed: Existence is enough for v1, add validation later

## Success Criteria

- [ ] Tasks with spec.md show in Spec column (or later), not Queued
- [ ] Dragging a task updates task.yaml within 500ms
- [ ] File watcher picks up the change and other clients update
- [ ] Undo reverts both UI and file state
- [ ] Running a task with existing spec prompts to skip
