# FinalizeExecutor Reference

Detailed documentation for the finalize phase executor.

## Execution Flow

```go
FinalizeExecutor.Execute(ctx, task, phase, state)
├── getFinalizeConfig()           # Get config with defaults
├── fetchTarget()                 # Fetch origin/main
├── checkDivergence()             # Commits ahead/behind
├── syncWithTarget()              # Merge or rebase
│   ├── syncViaMerge()            # Merge target into branch
│   └── syncViaRebase()           # Rebase onto target
├── resolveConflicts()            # AI-assisted resolution
│   └── resolveRebaseConflicts()  # Handle rebase conflicts
├── runTests()                    # Verify tests pass
├── tryFixTests()                 # AI fixes failing tests
├── assessRisk()                  # Classify merge risk
└── createFinalizeCommit()        # Document finalization
```

## Key Types

```go
type FinalizeResult struct {
    Synced            bool          // Branch synced with target
    ConflictsResolved int           // Number of conflicts resolved
    ConflictFiles     []string      // Files that had conflicts
    TestsPassed       bool          // Tests passed after sync
    TestFailures      []TestFailure // Test failure details
    RiskLevel         string        // low|medium|high|critical
    FilesChanged      int           // Files changed vs target
    LinesChanged      int           // Total lines changed
    NeedsReview       bool          // Requires additional review
    CommitSHA         string        // Final commit SHA
}
```

## Conflict Resolution

Uses Claude session for AI-assisted resolution:

```go
resolved, err := e.resolveConflicts(ctx, task, phase, state, conflictFiles, cfg)
```

**Resolution prompt rules:**
- Never remove features (both sides preserved)
- Merge intentions, not text
- Prefer additive resolutions
- Test after each file resolution

## Risk Assessment

```go
func classifyRisk(files, lines, conflicts int) string
```

| Metric | Low | Medium | High | Critical |
|--------|-----|--------|------|----------|
| Files | 1-5 | 6-15 | 16-30 | >30 |
| Lines | <100 | 100-500 | 500-1000 | >1000 |
| Conflicts | 0 | 1-3 | 4-10 | >10 |

## Escalation

```go
func (e *FinalizeExecutor) shouldEscalate(result *FinalizeResult, cfg config.FinalizeConfig) bool
```

Escalates to implement phase when:
- >10 conflicts couldn't be resolved
- >5 tests fail after fix attempts

## Configuration

```go
type FinalizeConfig struct {
    Enabled               bool                      // Enable finalize
    AutoTrigger           bool                      // Run after validate
    AutoTriggerOnApproval bool                      // Run when PR approved (auto profile)
    Sync                  FinalizeSyncConfig        // merge|rebase
    ConflictResolution    ConflictResolutionConfig  // AI resolution
    RiskAssessment        RiskAssessmentConfig      // Risk thresholds
    Gates                 FinalizeGatesConfig       // Pre-merge gates
}
```

## Auto-Trigger on PR Approval

When `AutoTriggerOnApproval` is enabled (default for `auto` profile):

1. PR status poller detects approval via `OnStatusChange` callback
2. Server calls `TriggerFinalizeOnApproval(taskID)` in `finalize_tracker.go`
3. Conditions checked: task weight supports finalize, finalize not already done
4. Finalize runs asynchronously with WebSocket progress events
