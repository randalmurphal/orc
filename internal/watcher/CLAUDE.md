# Watcher Package

File system watcher for real-time task and initiative updates. Monitors `.orc/tasks/` and `.orc/initiatives/` and publishes events when files are created, modified, or deleted outside the API.

## File Structure

| File | Purpose | Lines |
|------|---------|-------|
| `watcher.go` | Main watcher, fsnotify integration, event publishing | ~600 |
| `debouncer.go` | Event coalescing, delete verification | ~205 |
| `watcher_test.go` | Test coverage | ~400 |

## Architecture

```
File System Change
    ↓
fsnotify Event (Create/Write/Remove)
    ↓
handleFSEvent() → route to task or initiative handler
    ↓
handleTaskFSEvent() / handleInitiativeFSEvent()
    ↓
Debouncer (500ms quiet period)
    ↓
Content Hash Check (skip if unchanged)
    ↓
Sync to DB (for initiatives) → Publish Event → WebSocket Broadcast
```

## Watched Files

### Task Files

| File | Event Type | Trigger |
|------|------------|---------|
| `task.yaml` | `task_created` / `task_updated` | Create (new) or Write (existing) |
| `state.yaml` | `state` | Write |
| `plan.yaml` | `task_updated` | Write |
| `spec.md` | `task_updated` | Write |

### Initiative Files

| File | Event Type | Trigger |
|------|------------|---------|
| `initiative.yaml` | `initiative_created` / `initiative_updated` | Create (new) or Write (existing) |

Task and initiative deletions are verified before publishing (100ms delay) to handle atomic saves and renames.

### Initiative Database Sync

When external edits to `initiative.yaml` are detected (file modified outside CLI), the watcher automatically syncs the changes to the database cache via `initiative.SyncToDB()`. This keeps the DB index current for fast queries.

### Weight Change Detection

The watcher tracks task weights to detect changes:

| Scenario | Behavior |
|----------|----------|
| Weight changes (non-running task) | Automatically regenerates plan.yaml with new phase sequence |
| Weight changes (running task) | Logs warning, skips regeneration (would disrupt execution) |
| Plan already matches new weight | Skips regeneration (API/CLI already handled it) |

Phase statuses are preserved when regenerating: completed/skipped phases that exist in both old and new plans retain their status.

## Key Patterns

### Debouncing

Rapid file changes are coalesced using per-task+filetype debounce keys:

```go
// Multiple rapid writes to TASK-001/task.yaml result in one event
debouncer.Trigger("TASK-001", FileTypeTask, path)
```

Default: 500ms quiet period before firing.

### Content Hashing

SHA256 hashes prevent duplicate events when file content hasn't changed:

```go
// Returns false if content matches cached hash
changed, err := w.hasContentChanged(path)
```

### Delete Verification

Delete events are verified after 100ms to catch false positives:

```go
// Atomic save: Remove original → Create temp → Rename temp
// Without verification, we'd see a false "deleted" event
debouncer.TriggerDelete(taskID, path)  // Schedules verification
debouncer.CancelDelete(taskID)          // Called if file reappears
```

## Integration

Started by API server in `StartContext()`:

```go
fw, err := watcher.New(&watcher.Config{
    WorkDir:   s.workDir,
    Publisher: s.publisher,
    Logger:    s.logger,
})
go fw.Start(ctx)
```

Events flow to WebSocket clients via the shared publisher.

## Configuration

| Option | Default | Purpose |
|--------|---------|---------|
| `DebounceMs` | 500 | Quiet period before publishing |

## Testing

```bash
go test ./internal/watcher/... -v
```

Key test scenarios:
- Create/update/delete detection
- Content change filtering (hash-based)
- Delete false positive handling
- Debouncing behavior
- Weight change detection and plan regeneration
