# Connect RPC Migration - Completion Plan

## Current State Assessment

The migration is ~40% complete:
- ✅ Proto schemas defined (`proto/orc/v1/`)
- ✅ Connect RPC servers implemented (12 `*_server.go` files)
- ✅ Frontend migrated to Connect client
- ❌ REST handlers still exist (60 files)
- ❌ `task.Task` struct still exists
- ❌ `db.Task` struct still exists
- ❌ Backend has dual interfaces (`SaveTask` + `SaveTaskProto`)
- ❌ ~43 conversion functions still exist

## Target Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Connect RPC Layer                       │
│                    (*_server.go files)                       │
│                    Uses: orcv1.Task                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Storage Backend                          │
│                  (backend.go, db_task.go)                    │
│           SaveTask(*orcv1.Task), LoadTask() *orcv1.Task      │
│                                                              │
│   Proto ←→ DB mapping:                                       │
│   - Enums stored as integers (proto enum values)             │
│   - Complex fields as JSON (protojson)                       │
│   - Timestamps as RFC3339 strings                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        SQLite                                │
│                      (.orc/orc.db)                           │
└─────────────────────────────────────────────────────────────┘
```

**Key Principle**: `orcv1.Task` is the ONLY Task type. No `task.Task`, no `db.Task`.

---

## Migration Chunks

Execute these in order. Each chunk should result in passing tests before moving to the next.

---

## Chunk 1: Storage Layer - Proto Only Interface

**Goal**: Backend interface uses ONLY proto types. Remove dual interface.

**Files to Modify**:
- `internal/storage/backend.go`
- `internal/storage/db_task.go`
- `internal/storage/proto_convert.go`

**Files to Delete**:
- None (conversion logic moves inline to db_task.go)

### PROMPT:

```
# Chunk 1: Storage Layer Proto Migration

## Context
You are completing the Connect RPC migration for the orc project. The goal is to make `orcv1.Task` (proto type) the SINGLE source of truth. No more `task.Task` or `db.Task` intermediate types.

## Current Problem
The Backend interface has DUAL methods:
- `SaveTask(*task.Task)` - legacy
- `SaveTaskProto(*orcv1.Task)` - new

We need ONE interface using proto types only.

## Your Mission

### 1. Update internal/storage/backend.go

Change the Backend interface to use ONLY proto types:

```go
// BEFORE (current - dual interface)
SaveTask(t *task.Task) error
LoadTask(id string) (*task.Task, error)
LoadAllTasks() ([]*task.Task, error)

SaveTaskProto(t *orcv1.Task) error
LoadTaskProto(id string) (*orcv1.Task, error)
LoadAllTasksProto() ([]*orcv1.Task, error)

// AFTER (target - proto only)
SaveTask(t *orcv1.Task) error
LoadTask(id string) (*orcv1.Task, error)
LoadAllTasks() ([]*orcv1.Task, error)
// DELETE the *Proto variants - they become the main methods
```

### 2. Update internal/storage/db_task.go

Rewrite to work with `orcv1.Task` directly:

**Database Column Mapping**:
| Proto Field | DB Column | Conversion |
|-------------|-----------|------------|
| `t.Id` | `id` | string, direct |
| `t.Title` | `title` | string, direct |
| `t.Status` | `status` | int (proto enum value) |
| `t.Weight` | `weight` | int (proto enum value) |
| `t.Queue` | `queue` | int (proto enum value) |
| `t.Priority` | `priority` | int (proto enum value) |
| `t.Category` | `category` | int (proto enum value) |
| `t.Execution` | `execution_json` | JSON via protojson |
| `t.CreatedAt` | `created_at` | timestamppb → RFC3339 string |
| `t.UpdatedAt` | `updated_at` | timestamppb → RFC3339 string |
| `t.Metadata` | `metadata` | JSON direct |

**Key Pattern for Enums**:
```go
// Store enum as integer
status := int(t.Status)  // orcv1.TaskStatus_TASK_STATUS_RUNNING = 4

// Load enum from integer
t.Status = orcv1.TaskStatus(statusInt)
```

**Key Pattern for Timestamps**:
```go
// Store timestamp
if t.CreatedAt != nil {
    createdAt = t.CreatedAt.AsTime().Format(time.RFC3339)
}

// Load timestamp
if createdAtStr != "" {
    parsed, _ := time.Parse(time.RFC3339, createdAtStr)
    t.CreatedAt = timestamppb.New(parsed)
}
```

**Key Pattern for Execution State**:
```go
// Store as JSON
if t.Execution != nil {
    execJSON, _ := protojson.Marshal(t.Execution)
    // store execJSON in execution_json column
}

// Load from JSON
if execJSON != "" {
    t.Execution = &orcv1.ExecutionState{}
    protojson.Unmarshal([]byte(execJSON), t.Execution)
}
```

### 3. Merge proto_convert.go into db_task.go

The conversion logic in `proto_convert.go` should be inlined into the db functions. After this chunk, `proto_convert.go` should only contain:
- `protoTaskToDBTask` → DELETE (no more db.Task)
- `dbTaskToProtoTask` → DELETE (no more db.Task)
- Keep ONLY enum conversion helpers if needed for display

### 4. Delete Legacy Methods

Remove from DatabaseBackend:
- `SaveTask(*task.Task)`
- `LoadTask() *task.Task`
- `LoadAllTasks() []*task.Task`
- `taskToDBTask()`
- `dbTaskToTask()`

## Database Schema Note

The database schema doesn't change - we're changing how Go code maps to it. Enums that were stored as strings ("running", "completed") will now be stored as integers (4, 8). This is a breaking change for existing data.

**Migration Strategy**: Add a one-time migration that converts string enum values to integers, OR keep string storage but map at the Go layer.

## Validation

```bash
go build ./internal/storage/...
go test ./internal/storage/...
```

Fix any callers that break. They should be updated to use proto types.

## DO NOT

- Do NOT keep dual interfaces
- Do NOT keep db.Task type
- Do NOT keep task.Task → db.Task conversion functions
```

---

## Chunk 2: Delete Legacy Type Definitions

**Goal**: Remove `task.Task` struct and `db.Task` struct entirely.

**Files to Modify**:
- `internal/task/task.go` - DELETE Task struct, keep helper functions as standalone
- `internal/task/execution.go` - DELETE ExecutionState struct (use orcv1.ExecutionState)
- `internal/task/enum_convert.go` - Keep only proto enum helpers
- `internal/db/task.go` - DELETE Task struct

### PROMPT:

```
# Chunk 2: Delete Legacy Type Definitions

## Context
Chunk 1 converted the storage layer to use proto types. Now we delete the legacy type definitions that are no longer used.

## Your Mission

### 1. Delete internal/task/task.go Task struct

The file currently defines:
```go
type Task struct {
    ID string
    Title string
    // ... 30+ fields
}
```

**DELETE the entire struct**. Keep any standalone helper functions but convert them to work with `*orcv1.Task`:

```go
// BEFORE
func (t *Task) IsComplete() bool { return t.Status == StatusCompleted }

// AFTER
func IsComplete(t *orcv1.Task) bool {
    return t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED
}
```

### 2. Delete internal/task/execution.go ExecutionState struct

The file currently defines:
```go
type ExecutionState struct {
    Phases map[string]*PhaseState
    // ...
}
type PhaseState struct {
    Status PhaseStatus
    // ...
}
```

**DELETE these structs**. Use `orcv1.ExecutionState` and `orcv1.PhaseState` instead.

Keep helper functions but convert to proto types:

```go
// BEFORE
func (e *ExecutionState) StartPhase(phaseID string) { ... }

// AFTER
func StartPhase(e *orcv1.ExecutionState, phaseID string) { ... }
```

### 3. Clean up internal/task/enum_convert.go

This file has conversion functions between string enums and proto enums.

**KEEP**: Functions needed for database string↔proto mapping (if you chose string storage in Chunk 1)
**DELETE**: Functions that convert to/from legacy task.Status, task.Weight, etc.

The enum types like `task.Status`, `task.Weight`, `task.PhaseStatus` should be DELETED. Only `orcv1.*` enum types should remain.

### 4. Delete internal/db/task.go Task struct

```go
type Task struct {
    ID string
    Title string
    Status string
    // ... database model
}
```

**DELETE this struct entirely**. The storage layer now works with `orcv1.Task` directly.

### 5. Update proto_helpers.go

The file `internal/task/proto_helpers.go` has helper functions for proto types. These should remain and become the primary helpers:

```go
func InitProtoExecutionState() *orcv1.ExecutionState
func StartPhaseProto(e *orcv1.ExecutionState, phaseID string)
func CompletePhaseProto(e *orcv1.ExecutionState, phaseID string, commitSHA string)
// etc.
```

Rename these to remove the "Proto" suffix since proto is now the only type:
```go
func InitExecutionState() *orcv1.ExecutionState
func StartPhase(e *orcv1.ExecutionState, phaseID string)
func CompletePhase(e *orcv1.ExecutionState, phaseID string, commitSHA string)
```

## Validation

```bash
go build ./internal/task/...
go build ./internal/db/...
go build ./...  # This will show all callers that need updating
```

Expect compilation errors in files that were using the deleted types. Those are fixed in Chunk 3.

## DO NOT

- Do NOT keep legacy Task struct "for compatibility"
- Do NOT keep dual enum types (task.Status AND orcv1.TaskStatus)
- Do NOT keep intermediate conversion functions
```

---

## Chunk 3: Update All Callers

**Goal**: Fix all code that was using `task.Task` or `db.Task` to use `orcv1.Task`.

**Files to Modify**:
- `internal/executor/*.go` (~10 files)
- `internal/cli/*.go` (~15 files)
- `internal/orchestrator/*.go` (~5 files)
- `internal/api/handlers_graph.go`
- `internal/initiative/*.go`

### PROMPT:

```
# Chunk 3: Update All Callers to Proto Types

## Context
Chunks 1-2 deleted the legacy types. Now we fix all the code that was using them.

## Your Mission

Run `go build ./...` to find all compilation errors. For each file:

### Pattern 1: Variable Declarations

```go
// BEFORE
var t *task.Task
t, err := backend.LoadTask(id)

// AFTER
var t *orcv1.Task
t, err := backend.LoadTask(id)
```

### Pattern 2: Field Access

```go
// BEFORE
if t.Status == task.StatusRunning { ... }

// AFTER
if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING { ... }
```

### Pattern 3: Struct Literals

```go
// BEFORE
t := &task.Task{
    ID: "TASK-001",
    Status: task.StatusCreated,
}

// AFTER
t := &orcv1.Task{
    Id: "TASK-001",
    Status: orcv1.TaskStatus_TASK_STATUS_CREATED,
}
```

Note: Proto field names use different casing (Id not ID, CreatedAt is a *timestamppb.Timestamp not time.Time)

### Pattern 4: Helper Function Calls

```go
// BEFORE
task.StartPhase(t.Execution, "implement")

// AFTER (if you renamed in Chunk 2)
task.StartPhase(t.Execution, "implement")  // Same, but now takes *orcv1.ExecutionState
```

### Pattern 5: Timestamp Handling

```go
// BEFORE
t.CreatedAt = time.Now()

// AFTER
t.CreatedAt = timestamppb.Now()

// BEFORE
elapsed := time.Since(t.StartedAt)

// AFTER
elapsed := time.Since(t.StartedAt.AsTime())
```

### Files to Update (in order)

1. **internal/executor/** - Core execution engine
   - workflow_executor.go
   - workflow_context.go
   - workflow_completion.go
   - finalize.go
   - retry.go

2. **internal/cli/** - Command line interface
   - cmd_run.go
   - cmd_show.go
   - cmd_list.go
   - cmd_status.go
   - cmd_import.go
   - cmd_export.go
   - cmd_new.go
   - cmd_edit.go

3. **internal/orchestrator/** - Multi-task orchestration
   - orchestrator.go
   - worker.go
   - scheduler.go

4. **internal/api/** - Only non-handler files
   - handlers_graph.go (uses task.Task currently)
   - pr_poller.go

5. **internal/initiative/** - Initiative management
   - initiative.go
   - proto_helpers.go

## Import Updates

Every file needs import updates:

```go
// REMOVE
import "github.com/randalmurphal/orc/internal/task"

// ADD (if not already present)
import orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
import "google.golang.org/protobuf/types/known/timestamppb"
```

## Validation

```bash
go build ./...
go test ./...
```

All tests should pass before proceeding to Chunk 4.

## DO NOT

- Do NOT add new conversion functions
- Do NOT keep compatibility shims
- Do NOT leave any file importing the deleted types
```

---

## Chunk 4: Delete REST Handlers

**Goal**: Remove all REST handlers. Only Connect RPC servers remain.

**Files to DELETE** (60 files):
- `internal/api/handlers_*.go` - ALL of them

**Files to Modify**:
- `internal/api/server.go` - Remove REST route registration
- `internal/api/server_routes.go` - DELETE entirely

### PROMPT:

```
# Chunk 4: Delete REST Handlers

## Context
The Connect RPC servers are complete and working. The REST handlers are now dead code.

## Your Mission

### 1. Delete ALL handler files

Delete every file matching `internal/api/handlers_*.go`:

```bash
rm internal/api/handlers_agents.go
rm internal/api/handlers_attachments.go
rm internal/api/handlers_automation.go
rm internal/api/handlers_branches.go
rm internal/api/handlers_claudemd.go
rm internal/api/handlers_config.go
rm internal/api/handlers_constitution.go
rm internal/api/handlers_dashboard.go
rm internal/api/handlers_decisions.go
rm internal/api/handlers_diff.go
rm internal/api/handlers_events.go
rm internal/api/handlers_export.go
rm internal/api/handlers_finalize.go
rm internal/api/handlers_github.go
rm internal/api/handlers_graph.go
rm internal/api/handlers_hooks.go
rm internal/api/handlers_initiatives.go
rm internal/api/handlers_knowledge.go
rm internal/api/handlers_mcp.go
rm internal/api/handlers_metrics.go
rm internal/api/handlers_notifications.go
rm internal/api/handlers_phases.go
rm internal/api/handlers_plugins.go
rm internal/api/handlers_projects.go
rm internal/api/handlers_prompts.go
rm internal/api/handlers_retry.go
rm internal/api/handlers_review_comments.go
rm internal/api/handlers_review_findings.go
rm internal/api/handlers_runs.go
rm internal/api/handlers_scripts.go
rm internal/api/handlers_session.go
rm internal/api/handlers_settings.go
rm internal/api/handlers_skills.go
rm internal/api/handlers_stats.go
rm internal/api/handlers_subtasks.go
rm internal/api/handlers_task_comments.go
rm internal/api/handlers_tasks.go
rm internal/api/handlers_tasks_control.go
rm internal/api/handlers_tasks_state.go
rm internal/api/handlers_team.go
rm internal/api/handlers_templates.go
rm internal/api/handlers_test_results.go
rm internal/api/handlers_tools.go
rm internal/api/handlers_transcripts.go
rm internal/api/handlers_workflows.go
```

### 2. Delete server_routes.go

This file registers all the REST routes. Delete it entirely.

### 3. Update server.go

Remove:
- All `r.Route("/api/...", ...)` REST route registration
- CORS middleware for REST (keep CORS for Connect)
- Any helper methods only used by REST handlers (`jsonResponse`, `jsonError`, etc.)

Keep:
- Server struct
- Connect handler registration
- Static file serving for web UI
- WebSocket (if still used) OR delete if replaced by Connect streaming

### 4. Delete websocket.go (if Connect streaming replaces it)

If `event_server.go` provides streaming via Connect RPC, delete:
- `internal/api/websocket.go`
- `internal/api/hub.go` (if exists)

### 5. Clean up unused types

Delete any types only used by REST handlers:
- Request/Response structs in handler files
- Helper types for JSON serialization

## Validation

```bash
go build ./internal/api/...
go build ./...
```

The API package should now only contain:
- server.go (Connect registration, static files)
- *_server.go (Connect service implementations)
- interceptors.go (Connect interceptors)
- pr_poller.go (background job)

## DO NOT

- Do NOT keep REST handlers "for compatibility"
- Do NOT keep route registration code
- Do NOT keep REST-specific middleware
```

---

## Chunk 5: Clean Up Conversion Functions

**Goal**: Remove all unnecessary conversion functions.

**Files to Modify**:
- `internal/task/enum_convert.go` - Minimize to essentials
- `internal/api/*_server.go` - Remove internal conversion functions

### PROMPT:

```
# Chunk 5: Clean Up Conversion Functions

## Context
With REST handlers deleted and proto types as the single source of truth, most conversion functions are now dead code.

## Your Mission

### 1. Audit internal/task/enum_convert.go

This file currently has ~17 conversion functions like:
- `StatusToProto(s string) orcv1.TaskStatus`
- `StatusFromProto(s orcv1.TaskStatus) string`
- `WeightToProto(w string) orcv1.TaskWeight`
- etc.

**KEEP** only what's needed for database storage (if using string storage):
```go
// Keep if database stores status as string
func StatusFromProto(s orcv1.TaskStatus) string {
    return s.String()  // Or custom mapping
}

func StatusToProto(s string) orcv1.TaskStatus {
    // Map string back to proto enum
}
```

**DELETE** if database stores enums as integers (proto values):
- All string↔proto conversion functions

**DELETE** regardless:
- Legacy enum type definitions (task.Status, task.Weight, etc.)
- Conversion functions for deleted types

### 2. Audit internal/api/*_server.go files

Each Connect server file may have local conversion functions. Audit each:

**config_server.go** - 6 conversion functions
**workflow_server.go** - 15 conversion functions
**task_server.go** - 7 conversion functions
**project_server.go** - 4 conversion functions
**etc.**

For each function:
- If it converts proto↔internal type: DELETE (internal type shouldn't exist)
- If it converts proto↔db type: Keep if db types still exist, else DELETE
- If it converts proto↔config type: Keep (config types are appropriate)

### 3. Delete internal/storage/proto_convert.go

If Chunk 1 inlined all conversions into db_task.go, this file should be empty or deletable.

### 4. Verify no orphaned conversion functions

```bash
grep -rn "ToProto\|FromProto" internal/ --include="*.go" | grep -v "_test.go"
```

Each result should be:
- Database layer mapping (keep)
- Config/external type mapping (keep)
- Everything else (DELETE)

## Target State

After this chunk, conversion functions exist ONLY for:
1. Database column mapping (proto enum ↔ DB storage format)
2. External config types (config.Config, claudeconfig.Settings)
3. Third-party types (git operations, GitHub API responses)

NO conversion functions for:
- task.Task ↔ orcv1.Task (task.Task deleted)
- db.Task ↔ orcv1.Task (db.Task deleted)
- Internal domain types (all use proto now)

## Validation

```bash
go build ./...
go test ./...
```

## DO NOT

- Do NOT keep conversion functions "just in case"
- Do NOT keep helper functions for deleted types
```

---

## Chunk 6: Update All Tests

**Goal**: All test files use proto types and test against Connect RPC.

**Files to Update**:
- `internal/storage/*_test.go`
- `internal/executor/*_test.go`
- `internal/cli/*_test.go`
- `internal/api/*_test.go` - Update to test Connect servers, not REST handlers

**Files to DELETE**:
- `internal/api/handlers_*_test.go` - All REST handler tests

### PROMPT:

```
# Chunk 6: Update All Tests

## Context
All production code now uses proto types. Tests must be updated to match.

## Your Mission

### 1. Delete REST handler tests

Delete all `internal/api/handlers_*_test.go` files. These tested deleted code.

### 2. Update storage tests

`internal/storage/*_test.go` files need to:
- Create `*orcv1.Task` instead of `*task.Task`
- Use proto enum values
- Use `timestamppb` for timestamps

```go
// BEFORE
t := &task.Task{
    ID: "TASK-001",
    Status: task.StatusCreated,
}

// AFTER
t := &orcv1.Task{
    Id: "TASK-001",
    Status: orcv1.TaskStatus_TASK_STATUS_CREATED,
    CreatedAt: timestamppb.Now(),
}
```

### 3. Update executor tests

`internal/executor/*_test.go` files need similar updates.

### 4. Update CLI tests

`internal/cli/*_test.go` files need similar updates.

### 5. Update API tests to use Connect client

Instead of HTTP requests to REST endpoints, tests should use Connect client:

```go
// BEFORE
req := httptest.NewRequest("GET", "/api/tasks/TASK-001", nil)
w := httptest.NewRecorder()
server.handleGetTask(w, req)

// AFTER
client := orcv1connect.NewTaskServiceClient(
    http.DefaultClient,
    server.URL,
)
resp, err := client.GetTask(ctx, connect.NewRequest(&orcv1.GetTaskRequest{
    Id: "TASK-001",
}))
```

### 6. Update test fixtures

Any test fixtures using JSON with string enums need updating:

```json
// BEFORE
{"status": "running", "weight": "medium"}

// AFTER
{"status": 4, "weight": 2}
// Or keep strings if your proto JSON uses string enum names
```

### 7. Run full test suite

```bash
go test ./... -v
```

Fix any remaining failures.

## Validation

```bash
go test ./... -race
```

All tests must pass with race detector.

## DO NOT

- Do NOT skip updating test files
- Do NOT leave tests that import deleted types
- Do NOT keep REST handler tests
```

---

## Chunk 7: Final Cleanup & Verification

**Goal**: Remove any remaining dead code and verify the migration is complete.

### PROMPT:

```
# Chunk 7: Final Cleanup & Verification

## Your Mission

### 1. Run static analysis

```bash
staticcheck ./...
go vet ./...
```

Fix all warnings about unused code.

### 2. Verify no legacy types remain

```bash
# Should return NOTHING
grep -rn "task\.Task\b" internal/ --include="*.go" | grep -v "orcv1"
grep -rn "db\.Task\b" internal/ --include="*.go"
grep -rn "task\.Status\b" internal/ --include="*.go" | grep -v "orcv1"
grep -rn "task\.Weight\b" internal/ --include="*.go" | grep -v "orcv1"
```

### 3. Verify no REST handlers remain

```bash
# Should return NOTHING
ls internal/api/handlers_*.go 2>/dev/null
```

### 4. Verify minimal conversion functions

```bash
# Should return only database/config mapping functions
grep -rn "ToProto\|FromProto" internal/ --include="*.go" | grep -v "_test.go" | wc -l
# Target: < 10 functions total
```

### 5. Run full test suite

```bash
make test
make web-test
make e2e
```

### 6. Update documentation

- Update CLAUDE.md files to reflect proto-only architecture
- Remove references to deleted types
- Document the new storage layer patterns

### 7. Clean up imports

```bash
goimports -w internal/
```

### 8. Final build verification

```bash
go build ./...
cd web && npm run build && npm run typecheck
```

## Checklist

- [ ] `task.Task` struct deleted
- [ ] `db.Task` struct deleted
- [ ] `task.ExecutionState` struct deleted
- [ ] Backend interface uses only `*orcv1.Task`
- [ ] All REST handlers deleted
- [ ] All REST handler tests deleted
- [ ] `server_routes.go` deleted
- [ ] `< 10` conversion functions remain
- [ ] `staticcheck` passes
- [ ] All tests pass
- [ ] E2E tests pass

## Migration Complete

After this chunk, the codebase should have:
- ONE Task type: `orcv1.Task`
- ONE way to store/load: `backend.SaveTask(*orcv1.Task)`
- ONE API layer: Connect RPC
- ZERO legacy types
- ZERO unnecessary conversion functions
```

---

## Execution Order

```
Chunk 1 (Storage Layer) ──────────────────────────────┐
                                                       │
Chunk 2 (Delete Legacy Types) ────────────────────────┤
                                                       │
Chunk 3 (Update Callers) ─────────────────────────────┤
                                                       │
Chunk 4 (Delete REST Handlers) ───────────────────────┤
                                                       │
Chunk 5 (Clean Up Conversions) ───────────────────────┤
                                                       │
Chunk 6 (Update Tests) ───────────────────────────────┤
                                                       │
Chunk 7 (Final Cleanup) ──────────────────────────────┘
```

**Time Estimate**:
- Chunk 1: 2-4 hours
- Chunk 2: 1-2 hours
- Chunk 3: 3-5 hours (most callers)
- Chunk 4: 1 hour (mostly deletion)
- Chunk 5: 1-2 hours
- Chunk 6: 2-4 hours
- Chunk 7: 1 hour

**Total**: ~12-20 hours of focused work

---

## Notes for Agents

1. **Each chunk should result in passing `go build ./...`** before moving to the next
2. **Chunk 3 will have the most errors** - be systematic, file by file
3. **Don't mix chunks** - complete one fully before starting the next
4. **When in doubt, delete** - git has history if something was actually needed
5. **Proto field naming differs from Go**: `Id` not `ID`, `CreatedAt` is `*timestamppb.Timestamp`
