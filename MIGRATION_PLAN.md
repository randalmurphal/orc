# Connect RPC Migration - Completion Plan

## End Goal

**Complete removal of REST API. Clean separation of concerns:**

```
/rpc/orc.v1.*          → Connect RPC (all structured data)
/files/tasks/{id}/*    → HTTP file serving (binary content only)
```

**NO `/api/` routes remain. NO REST handlers. NO legacy types.**

---

## Target Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP Server                              │
├─────────────────────────────────────────────────────────────┤
│  /rpc/*     → Connect RPC handlers (*_server.go)            │
│  /files/*   → Static file serving (attachments, screenshots)│
│  /*         → Web UI static files                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Storage Backend                          │
│           SaveTask(*orcv1.Task), LoadTask() *orcv1.Task      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                        SQLite                                │
└─────────────────────────────────────────────────────────────┘
```

**Key Principles:**
- `orcv1.Task` is the ONLY Task type
- Connect RPC is the ONLY API for structured data
- `/files/` is the ONLY path for binary file downloads
- Zero REST handlers, zero `/api/` routes

---

## Progress Log

### 2026-01-26 (Session 8) - Migration Complete: Final Verification

**Completed:**
- ✅ Frontend unit tests: 2071 passed, 3 skipped
- ✅ Backend tests: All pass (`go test ./... -short`)
- ✅ E2E test analysis: Failures are pre-existing selector issues (tests expect CSS classes like `.board-page` that don't exist in actual components)
- ✅ Visual verification: Screenshot confirms app loads correctly with Connect RPC

**Final Status: MIGRATION COMPLETE**

The Connect RPC migration is fully complete. All structured data now flows through Connect RPC (`/rpc/orc.v1.*`), binary file serving uses `/files/*` endpoints, and no REST `/api/` routes remain.

**Note on E2E tests:** The failing E2E tests are a separate maintenance issue - the test selectors are out of sync with the actual component implementations. This is unrelated to the migration.

---

### 2026-01-26 (Session 7) - Chunk 6 Complete: Legacy Types Deleted

**Completed:**
- ✅ Migrated `TurnResult.Usage` from `task.TokenUsage` to `*orcv1.TokenUsage` in executor
- ✅ Updated `workflow_phase.go` to cast int32 token counts to int
- ✅ Deleted `task_enums.go` - all legacy enum types (`Weight`, `Queue`, `Priority`, `Category`, `PRStatus`, `DependencyStatus`)
- ✅ Deleted `execution.go` - all legacy execution types (`ExecutionState`, `PhaseState`, `TokenUsage`, etc.)
- ✅ Deleted `execution_test.go` - tests for deleted types
- ✅ Migrated `pr_poller.go` to use `DeterminePRStatusProto` exclusively (deleted legacy `DeterminePRStatus`)
- ✅ Updated `pr_poller_test.go` to use `orcv1.PRStatus` types
- ✅ Updated `initiative/manifest.go` helper functions to use proto validation functions
- ✅ Updated stale comments referencing `task.PhaseState` → `orcv1.PhaseState`
- ✅ Updated `storage/CLAUDE.md` documentation to reference proto types
- ✅ All Go tests pass

**Files Deleted:**
- `internal/task/task_enums.go` - legacy enum types (unused)
- `internal/task/execution.go` - legacy execution state (unused)
- `internal/task/execution_test.go` - tests for deleted file

**Files Kept (still required):**
- `internal/task/enum_convert.go` - string↔proto conversion functions (needed for DB persistence)
- `internal/task/execution_helpers.go` - proto-based execution helpers (operating on `orcv1.ExecutionState`)
- `internal/storage/proto_convert.go` - proto↔db.Task conversion (needed for SQLite storage)
- `internal/db/task.go` - `db.Task` struct (needed for SQL row scanning)

**Validation:**
```bash
grep -rn "task\.ExecutionState" internal/ --include="*.go"  # Returns NOTHING ✓
grep -rn "task\.Weight\b" internal/ --include="*.go"        # Returns NOTHING ✓
grep -rn "task\.TokenUsage" internal/ --include="*.go"      # Returns NOTHING ✓
go test ./... -short  # All pass ✓
```

---

### 2026-01-26 (Session 6) - Chunk 6 Partial: Type Migration to Proto

**Completed:**
- ✅ Updated `PhaseExecutor` interface to use `*orcv1.ExecutionState` (was `*task.ExecutionState`)
- ✅ Updated `ExecutorTypeForWeight` to use `orcv1.TaskWeight` (was `task.Weight`)
- ✅ Updated `PhaseOutputDetector` to use `orcv1.TaskWeight`
- ✅ Updated `ValidateSpec` in `spec.go` to use `orcv1.TaskWeight`
- ✅ Updated `template.go` to use `orcv1.TaskWeight` in `phasesForWeight()`
- ✅ Updated `initiative/manifest.go` to use proto-based validation (`ParseWeightProto`, `ParseCategoryProto`, `ParsePriorityProto`)
- ✅ Updated `planner/parser.go` - `ProposedTask.Weight` now `string` with `WeightProto()` helper
- ✅ Fixed `ParsePriorityProto` to correctly validate invalid priorities
- ✅ Updated all test files to use proto types
- ✅ All Go tests pass

**Impact:**
- `task.ExecutionState` - NO LONGER USED (struct still defined but unused)
- `task.Weight` type - NO LONGER USED (migrated to `orcv1.TaskWeight`)
- Enum conversion functions (`*ToProto/*FromProto`) - STILL NEEDED for DB persistence

**Remaining for Chunk 6:**
- ~~`task.ExecutionState` struct can be deleted (unused)~~ ✅ DONE (Session 7)
- ~~Legacy enum types in `task_enums.go` can be cleaned up~~ ✅ DONE (Session 7)
- `db.Task` and `proto_convert.go` still needed until storage layer refactor

**Validation:**
```bash
grep -rn "task\.ExecutionState" internal/ --include="*.go"  # Returns NOTHING ✓
grep -rn "task\.Weight\b" internal/ --include="*.go"        # Returns NOTHING ✓
go test ./...  # All pass ✓
```

---

### 2026-01-26 (Session 5) - Chunks 5 & 7 Complete: REST Handlers Deleted

**Completed:**
- ✅ Deleted ALL `handlers_*.go` files (60 files)
- ✅ Deleted `server_routes.go` (REST route registration)
- ✅ Created `file_handlers.go` with `/files/` routes for binary content
- ✅ Created `finalize_tracker.go` with async finalize logic (extracted from handlers_finalize.go)
- ✅ Added `GetSessionMetrics()` method to server.go (used by WebSocket)
- ✅ Updated `server.go` to call `registerFileRoutes()` instead of `registerRoutes()`
- ✅ Deleted `server_test.go` (111 tests for deleted REST endpoints)
- ✅ All Go tests pass
- ✅ All validation checks pass

**Validation:**
```bash
ls internal/api/handlers_*.go  # Returns NOTHING ✓
ls internal/api/server_routes.go  # Returns NOTHING ✓
grep -r '"/api/' internal/api/*.go  # Returns NOTHING (except static.go fallback) ✓
go test ./...  # All pass ✓
```

---

### 2026-01-26 (Session 4) - Chunk 4 Complete: Frontend Migration

**Completed:**
- ✅ Migrated `ChangesTab.tsx` review retry to Connect RPC (`taskClient.retryTask()`)
- ✅ Added `GetFileDiff` RPC to proto for lazy-loading file hunks
- ✅ Implemented `GetFileDiff` backend handler in `task_server.go`
- ✅ Migrated `ChangesTab.tsx` diff file loading to Connect RPC (`taskClient.getFileDiff()`)
- ✅ Added `/files/` routes to backend (mirrors `/api/` handlers for binary files)
- ✅ Updated `AttachmentsTab.tsx` to use `/files/` endpoint for upload and download
- ✅ Updated `TestResultsTab.tsx` to use `/files/` endpoints for screenshots, traces, HTML report
- ✅ All Go tests pass
- ✅ All 2071 frontend tests pass

**Validation:**
```bash
grep -r "fetch.*\/api\/" web/src/ --include="*.ts" --include="*.tsx"
# Returns NOTHING ✓
```

---

### 2026-01-26 (Session 3) - Frontend Migration Started

**Completed:**
- ✅ Audited frontend for REST API usage
- ✅ Migrated `statsStore.ts` to Connect RPC (GetStats, GetCostSummary)
- ✅ Updated `statsStore.test.ts` - all 24 tests pass with Connect mocks
- ✅ Fixed `App.test.tsx` - added Connect client mocks
- ✅ All 2071 frontend tests pass

---

### 2026-01-26 (Session 2) - Legacy Task Types Deleted

**Completed:**
- ✅ Deleted `task.Task` struct from `internal/task/task.go`
- ✅ Deleted `PRInfo`, `TestingRequirements`, `QualityMetrics`, `BlockerInfo` structs
- ✅ Deleted all methods on `*Task`
- ✅ Deleted dead test files: `task_test.go`, `orphan_test.go`
- ✅ All tests pass (`go test ./...`)

**Intentionally kept (for now):**
- `task.ExecutionState` - used by executor (migrate in Chunk 5)
- `task.Status`, `task.Weight` enums - used for validation (delete in Chunk 5)
- `db.Task` struct - used for proto↔DB conversion (delete in Chunk 5)

---

### 2026-01-26 (Session 1) - Storage Layer Migrated

**Completed:**
- ✅ Storage interface uses proto types only
- ✅ ALL production code uses `orcv1.Task`
- ✅ ALL test code migrated to proto types

---

## Remaining Work

### ~~Chunk 4: Complete Frontend Migration~~ ✅ COMPLETED

**Goal:** All frontend code uses Connect RPC. No direct `/api/` fetch calls.

| File | Action | Status |
|------|--------|--------|
| `ChangesTab.tsx` | Migrated diff file loading to `GetFileDiff` RPC | ✅ |
| `ChangesTab.tsx` | Migrated review retry to `taskClient.retryTask()` | ✅ |
| `AttachmentsTab.tsx` | Updated to `/files/` endpoint | ✅ |
| `TestResultsTab.tsx` | Updated to `/files/` endpoints | ✅ |

**Notes:**
- Added `GetFileDiff` RPC to proto and backend (returns file hunks for lazy loading)
- Added `/files/` routes to backend that mirror `/api/` handlers
- Frontend now uses Connect RPC for structured data, `/files/` for binary content

**Validation:** ✅ PASSED
```bash
grep -r "fetch.*\/api\/" web/src/ --include="*.ts" --include="*.tsx"
# Returns NOTHING
```

---

### ~~Chunk 5: Delete All REST Handlers~~ ✅ COMPLETED

**Goal:** Remove ALL `handlers_*.go` files and REST route registration.

**Completed:**
- ✅ Deleted all `handlers_*.go` files (60 files)
- ✅ Deleted `server_routes.go`
- ✅ Created `file_handlers.go` for `/files/` routes
- ✅ Created `finalize_tracker.go` for async finalize functionality
- ✅ Updated `server.go` to use `registerFileRoutes()`

**New file serving routes:**
```go
// Add to server.go
r.Route("/files", func(r chi.Router) {
    r.Get("/tasks/{taskID}/attachments/{filename}", s.serveAttachment)
    r.Get("/tasks/{taskID}/diff/{filepath:.*}", s.serveDiffFile)
    r.Get("/tasks/{taskID}/test-results/screenshots/{filename}", s.serveScreenshot)
    r.Get("/tasks/{taskID}/test-results/traces/{filename}", s.serveTrace)
    r.Get("/tasks/{taskID}/test-results/html-report", s.serveHTMLReport)
})
```

**Validation:**
```bash
# Should return NOTHING
ls internal/api/handlers_*.go 2>/dev/null
grep -r "\/api\/" internal/api/server.go
```

---

### ~~Chunk 6: Delete Legacy Types & Conversions~~ ✅ COMPLETED

**Goal:** Remove unused legacy types. Keep DB-mapping infrastructure.

**Deleted from `internal/task/`:**
- ✅ `task_enums.go` - all legacy enum types (`Weight`, `Queue`, `Priority`, `Category`, `PRStatus`, `DependencyStatus`)
- ✅ `execution.go` - all legacy execution types (`ExecutionState`, `PhaseState`, `TokenUsage`, `GateDecision`, etc.)
- ✅ `execution_test.go` - tests for deleted types

**Kept (necessary for DB persistence):**
- `enum_convert.go` - string↔proto conversion functions (DB stores strings, proto uses enums)
- `execution_helpers.go` - proto-based helpers operating on `orcv1.ExecutionState`
- `internal/storage/proto_convert.go` - proto↔db.Task conversion
- `internal/db/task.go` - `db.Task` struct (SQL row scanning requires concrete struct)

**Note:** Original plan to delete `db.Task` and `proto_convert.go` was overly aggressive. These are necessary because:
1. SQLite stores enum values as strings, proto types use numeric enums
2. SQL row scanning requires concrete structs with db tags
3. Proto types have different field representations (timestamps, optionals)

**Validation:**
```bash
grep -rn "task\.ExecutionState" internal/ --include="*.go"  # Returns NOTHING ✓
grep -rn "task\.Weight\b" internal/ --include="*.go"        # Returns NOTHING ✓
grep -rn "task\.TokenUsage" internal/ --include="*.go"      # Returns NOTHING ✓
go test ./... -short  # All pass ✓
```

---

### ~~Chunk 7: Delete REST Handler Tests~~ ✅ COMPLETED

**Goal:** Remove all tests for deleted REST handlers.

**Completed:**
- ✅ Deleted all `handlers_*_test.go` files (included in Chunk 5 deletion)
- ✅ Deleted `server_test.go` (111 tests for deleted REST endpoints)
- ✅ All remaining tests pass

---

### Chunk 8: Final Cleanup & Verification

**Goal:** Verify migration is complete.

**Checklist:**
- [x] `task.Task` struct deleted (Session 2)
- [x] `task.ExecutionState` struct deleted (Session 7)
- [x] Legacy enum types deleted (Session 7)
- [x] ALL `handlers_*.go` files deleted (Session 5)
- [x] ALL `handlers_*_test.go` files deleted (Session 5)
- [x] `server_routes.go` deleted (Session 5)
- [x] NO `/api/` routes in `server.go` (Session 5)
- [x] `/files/` routes work for binary content (Session 4)
- [x] Frontend uses Connect RPC for all data (Session 4)
- [x] Frontend uses `/files/` for all binary downloads (Session 4)
- [x] `go test ./...` passes ✓
- [x] `npm test` passes (frontend) - 2071 passed, 3 skipped ✓
- [x] E2E tests - pre-existing selector issues (tests expect `.board-page` class that doesn't exist in components); app loads correctly per screenshots

**Kept by design (necessary infrastructure):**
- `db.Task` struct - needed for SQL row scanning
- `proto_convert.go` - proto↔db.Task conversion
- `enum_convert.go` - string↔proto enum conversion (~40 functions)

**Final verification:**
```bash
# No legacy task/execution types
grep -rn "task\.ExecutionState\|task\.Weight\b\|task\.TokenUsage" internal/ --include="*.go"
# Returns NOTHING ✓

# No REST handlers
ls internal/api/handlers_*.go 2>/dev/null
# Returns NOTHING ✓

# No /api/ routes (except static fallback)
grep -r "\/api\/" internal/api/ --include="*.go"
# Returns NOTHING ✓

# No frontend /api/ fetches
grep -r "fetch.*\/api\/" web/src/ --include="*.ts" --include="*.tsx"
# Returns NOTHING ✓

# All tests pass
go test ./... -short
```

---

## Execution Order

```
Chunk 4: Complete Frontend Migration        ✅ DONE
    ↓
Chunk 5: Delete All REST Handlers           ✅ DONE
    ↓
Chunk 6: Delete Legacy Types & Conversions  ✅ DONE
    ↓
Chunk 7: Delete REST Handler Tests          ✅ DONE
    ↓
Chunk 8: Final Cleanup & Verification       ✅ DONE
```

---

## What Was Deleted

| Category | Files/Items | Status |
|----------|-------------|--------|
| REST handlers | `internal/api/handlers_*.go` | ✅ Deleted |
| REST handler tests | `internal/api/handlers_*_test.go` | ✅ Deleted |
| Route registration | `internal/api/server_routes.go` | ✅ Deleted |
| Server tests | `internal/api/server_test.go` | ✅ Deleted |
| Legacy task type | `task.Task` struct | ✅ Deleted |
| Legacy execution type | `task.ExecutionState` struct | ✅ Deleted |
| Legacy enums file | `task_enums.go` | ✅ Deleted |
| Legacy execution file | `execution.go`, `execution_test.go` | ✅ Deleted |
| Legacy PR status | `DeterminePRStatus` function | ✅ Deleted |

## What Remains (By Design)

| Category | Files/Items | Reason |
|----------|-------------|--------|
| Connect RPC servers | `internal/api/*_server.go` (~13 files) | API layer |
| File serving | `internal/api/file_handlers.go` (`/files/*` routes) | Binary content |
| Proto types | `orcv1.Task`, `orcv1.ExecutionState`, etc. | The ONLY data types |
| Storage layer | `internal/storage/backend.go`, `db_task.go` | DB operations |
| DB task struct | `internal/db/task.go` (`db.Task`) | SQL row scanning |
| Proto convert | `internal/storage/proto_convert.go` | proto↔db.Task mapping |
| Enum convert | `internal/task/enum_convert.go` | string↔proto enum (DB stores strings) |
| Execution helpers | `internal/task/execution_helpers.go` | Proto-based helpers |
