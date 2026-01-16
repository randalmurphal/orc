# Specification: Bug: Spec phase completes but spec content not saved to database

## Problem Statement

When spec phase completes successfully, the spec content is only saved to a file (`{taskDir}/artifacts/spec.md`) but never persisted to the database `specs` table. This causes downstream code that expects spec content in the database to fail, resulting in implement phase receiving template placeholders instead of actual requirements.

## Root Cause

In `internal/executor/standard.go:296-302` and `internal/executor/full.go:323-329`, after spec phase completion:
1. `SavePhaseArtifact()` is called → writes to `{taskDir}/artifacts/spec.md` file
2. `backend.SaveSpec()` is **never called** → database `specs` table remains empty

Meanwhile, `internal/executor/task_execution.go:537` calls `backend.LoadSpec()` expecting spec content to be in the database, which returns empty string.

## Success Criteria

- [ ] Spec content is saved to database (`specs` table) when spec phase completes successfully
- [ ] All three executor types (standard, full, trivial) save spec to database on completion
- [ ] `backend.LoadSpec(taskID)` returns the spec content after spec phase completes
- [ ] Implement phase receives actual spec content instead of template placeholders
- [ ] Existing artifact file saving is preserved (dual-write: file + database)
- [ ] Unit tests verify spec database persistence after phase completion

## Testing Requirements

- [ ] Unit test: StandardExecutor saves spec to database on spec phase completion
- [ ] Unit test: FullExecutor saves spec to database on spec phase completion
- [ ] Unit test: TrivialExecutor saves spec to database on spec phase completion
- [ ] Unit test: Non-spec phases do not call SaveSpec
- [ ] Integration test: Full task execution with spec phase saves spec to database
- [ ] Integration test: Implement phase can load spec from database after spec phase

## Scope

### In Scope
- Adding `backend.SaveSpec()` call after spec phase completion in all executors
- Extracting artifact content from phase output for database storage
- Adding backend parameter to executors that don't have it
- Unit tests for new behavior

### Out of Scope
- Changing artifact detection logic (file-based detection continues to work)
- Migrating existing file-based specs to database
- Changing spec validation logic
- Adding API endpoints for spec management

## Technical Approach

### Files to Modify

1. **`internal/executor/standard.go`**:
   - Already has `backend storage.Backend` via `WithStandardBackend`
   - After `SavePhaseArtifact()` for spec phase, call `backend.SaveSpec()`
   - Extract artifact content using `ExtractArtifactContent()` function

2. **`internal/executor/full.go`**:
   - Already has `backend storage.Backend` via `WithFullBackend`
   - After `SavePhaseArtifact()` for spec phase, call `backend.SaveSpec()`

3. **`internal/executor/trivial.go`**:
   - Add `backend storage.Backend` field and option function
   - After `SavePhaseArtifact()` for spec phase, call `backend.SaveSpec()`

4. **`internal/executor/worker.go`** (or wherever executors are instantiated):
   - Ensure backend is passed to all executor constructors

5. **`internal/executor/artifact.go`** or new helper:
   - Add helper function to save spec to database: `SaveSpecToDatabase(backend, taskID, output)`

### Implementation Pattern

```go
// After SavePhaseArtifact in all executors, for spec phase:
if result.Status == plan.PhaseCompleted && p.ID == "spec" {
    if e.backend != nil {
        specContent := ExtractArtifactContent(result.Output)
        if specContent != "" {
            if err := e.backend.SaveSpec(t.ID, specContent, "executor"); err != nil {
                e.logger.Warn("failed to save spec to database", "error", err)
                // Don't fail phase - file artifact was saved successfully
            }
        }
    }
}
```

## Bug Analysis

### Reproduction Steps
1. Create a task with weight `large` (requires spec phase)
2. Run `orc run TASK-XXX` - spec phase completes with commit SHA
3. Query database: `SELECT * FROM specs WHERE task_id = 'TASK-XXX'` → empty
4. Run implement phase - receives template placeholders like `[1-2 sentences]`

### Current Behavior
- Spec phase runs and generates valid specification
- `SavePhaseArtifact()` writes spec to `{taskDir}/artifacts/spec.md`
- Git commit created with spec changes
- Phase marked as completed in `phases` table
- **BUT** `specs` table remains empty for task

### Expected Behavior
- All of the above, PLUS:
- `specs` table contains spec content with source='executor'
- `backend.LoadSpec(taskID)` returns the spec content
- Implement phase template receives actual spec via `{{SPEC_CONTENT}}`

### Verification
```sql
-- After fix, this query should return spec content:
SELECT task_id, substr(content, 1, 100) as preview
FROM specs
WHERE task_id = 'TASK-XXX';
```
