# Budget Enforcement Specification

**Status**: Implemented (TASK-787)
**Last Updated**: 2026-02-05

## Overview

Budget enforcement prevents runaway costs by checking spending limits before task execution. It uses `GlobalDB.GetBudgetStatus()` to check monthly spending against configured limits per project.

## Architecture

Budget checking is implemented directly in the executor, not as a separate package:

```
┌──────────────────────┐     ┌─────────────────────┐
│  WorkflowExecutor    │────▶│  GlobalDB            │
│  checkBudget()       │     │  GetBudgetStatus()   │
│  (before first phase)│     │  GetBudget()         │
└──────────────────────┘     └─────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `internal/executor/cost_tracking.go` | `checkBudget()` method, `RecordCostEntry()` helper |
| `internal/executor/workflow_state.go` | User ID extraction from context for cost attribution |
| `internal/executor/workflow_executor.go` | Integration point (`Run()` calls `checkBudget()`) |
| `internal/cli/cmd_run.go` | `--ignore-budget` CLI flag |
| `internal/cli/cmd_resume.go` | `--ignore-budget` CLI flag |
| `internal/db/global.go` | `BudgetStatus`, `GetBudgetStatus()`, `GetBudget()`, `SetBudget()` |

### Key Types

```go
// db.BudgetStatus (internal/db/global.go)
type BudgetStatus struct {
    ProjectID         string
    MonthlyLimitUSD   float64
    CurrentMonthSpent float64
    CurrentMonth      string
    PercentUsed       float64
    AlertThreshold    int
    OverBudget        bool        // true when CurrentMonthSpent > MonthlyLimitUSD (strict >)
    AtAlertThreshold  bool
}

// executor.WorkflowRunOptions (internal/executor/workflow_executor.go)
type WorkflowRunOptions struct {
    // ... other fields
    IgnoreBudget bool  // Bypasses budget enforcement when true
}
```

## Behavior

| Scenario | Result |
|----------|--------|
| No GlobalDB available | Allowed (skip check) |
| No budget configured for project | Allowed |
| Limit set to 0 | Allowed (enforcement disabled) |
| Spent < limit | Allowed |
| Spent == limit (exact-at-limit) | Allowed (uses strict `>`) |
| Spent > limit | **Denied** (returns error) |
| Spent > limit + `--ignore-budget` | Allowed (logs warning) |
| Approaching alert threshold | Allowed (logs warning) |
| DB error during check | Allowed (logs warning, fail-open) |

**Design choice: fail-open.** Budget enforcement is best-effort. DB errors during the check log a warning but don't block execution. This prevents infrastructure issues from stopping all work.

### Executor Integration

Budget is checked **once per run** (before the first phase), not per-phase:

```go
// In WorkflowExecutor.Run(), before phase execution begins
if err := we.checkBudget(opts.IgnoreBudget); err != nil {
    return err  // Task not claimed, no state mutation
}
```

The check runs BEFORE task claiming and phase execution. If budget is exceeded, no task state is mutated and there's no orphan risk.

### CLI Integration

Both `orc run` and `orc resume` accept `--ignore-budget` which maps to `WorkflowRunOptions.IgnoreBudget`.

## Testing

| Test File | Tests | Coverage |
|-----------|-------|----------|
| `internal/executor/budget_enforcement_test.go` | 13 | Over-budget blocks, ignore-budget bypasses, alert threshold warning, no budget configured, ignore-budget field exists, nil globalDB, DB error proceeds, spent==limit not over budget, limit=0 disabled, exact-at-threshold, ignore-budget with no budget |
| `internal/executor/cost_user_id_test.go` | 2 | User ID from context, empty user ID fallback |
