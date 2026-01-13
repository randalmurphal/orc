# Bidirectional Sync: Files ↔ UI

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

#### New Endpoint: PATCH /api/tasks/{id}/move

```json
// Request
{
    "target_column": "implement",
    "skip_requirements": false
}

// Response (success)
{
    "task": { ... },
    "skipped_phases": ["spec"],
    "warnings": []
}

// Response (blocked)
{
    "error": "requirements_not_met",
    "missing": [
        {
            "type": "artifact",
            "name": "spec.md",
            "hint": "Create spec.md or set skip_requirements=true"
        }
    ]
}
```

#### New Endpoint: POST /api/tasks/{id}/undo

```json
// Request
{
    "action_id": "move_abc123"
}

// Response
{
    "task": { ... },
    "reverted_to": {
        "current_phase": "spec",
        "status": "planned"
    }
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

### File Write Operations

When UI moves a task:

```go
func (s *Server) handleMoveTask(w http.ResponseWriter, r *http.Request) {
    var req MoveRequest
    json.NewDecoder(r.Body).Decode(&req)

    task, err := task.LoadFrom(s.workDir, req.TaskID)
    if err != nil {
        s.jsonError(w, "task not found", 404)
        return
    }

    // Validate move
    validation := s.validateMove(task, req.TargetColumn)
    if !validation.Allowed && !req.SkipRequirements {
        s.jsonResponse(w, validation)
        return
    }

    // Record for undo
    before := task.Snapshot()

    // Update task state
    task.CurrentPhase = columnToPhase(req.TargetColumn)
    task.Status = deriveStatus(req.TargetColumn)
    task.UpdatedAt = time.Now()

    // Write to file
    if err := task.SaveTo(task.TaskDirIn(s.workDir, task.ID)); err != nil {
        s.jsonError(w, "failed to save", 500)
        return
    }

    // File watcher will pick up change and broadcast via WebSocket

    s.jsonResponse(w, MoveResponse{
        Task:     task,
        ActionID: generateActionID(),
        Before:   before,
    })
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
