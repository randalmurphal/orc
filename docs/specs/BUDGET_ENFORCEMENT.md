# Budget Enforcement Specification

**Status**: Executor Integration Implemented (TASK-787)
**Last Updated**: 2026-02-05

## Overview

Budget enforcement prevents runaway costs by checking spending limits before task execution. It uses store interfaces for cost data and budget limits, scoped by user and project with daily/weekly/monthly periods.

## Implementation Status

### Completed (TASK-787)

| Component | Status | Location |
|-----------|--------|----------|
| `Enforcer` struct | Done | `internal/budgets/enforcement.go` |
| `CostStore` interface | Done | `internal/budgets/enforcement.go` |
| `BudgetStore` interface | Done | `internal/budgets/enforcement.go` |
| `EnforcementResult` type | Done | `internal/budgets/enforcement.go` |
| `BudgetPeriod` enum (daily/weekly/monthly) | Done | `internal/budgets/enforcement.go` |
| `periodRange()` helper | Done | `internal/budgets/enforcement.go` |
| Executor integration (once-per-run check) | Done | `internal/executor/executor.go` |
| `CurrentUsername` export for cost user ID | Done | `internal/executor/executor.go` |
| Unit tests (12 tests) | Done | `internal/budgets/enforcement_test.go` |
| Integration tests (2 tests) | Done | `internal/executor/executor_test.go` |

### Pending (Future Tasks)

| Component | Depends On |
|-----------|------------|
| Wire `BudgetStore`/`CostStore` to real GlobalDB queries | DB query methods |
| CLI `--ignore-budget` flag | Executor integration |
| Budget warning threshold (warn before exceeding) | Enforcer |
| Budget management CLI (`orc budget`) | API endpoints |
| Web UI budget display | API endpoints |

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌─────────────┐
│   Executor   │────▶│   Enforcer   │────▶│  CostStore  │
│ (before run) │     │              │     │  (globaldb)  │
└──────────────┘     └──────────────┘     └─────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │ BudgetStore  │
                     │ (globaldb)   │
                     └──────────────┘
```

### Key Types (`internal/budgets/enforcement.go`)

```go
type Enforcer struct { costs CostStore; budgets BudgetStore; now func() time.Time }
type BudgetPeriod string  // "daily", "weekly", "monthly"
type BudgetLimit struct { UserID, ProjectID string; Period BudgetPeriod; LimitUSD float64 }
type CostRecord struct { UserID, ProjectID string; CostUSD float64; Timestamp time.Time }
type EnforcementResult struct { Allowed bool; LimitUSD, SpentUSD float64; Period BudgetPeriod; Reason string }
```

### Store Interfaces

```go
type CostStore interface {
    GetCostsInRange(ctx context.Context, userID, projectID string, from, to time.Time) ([]CostRecord, error)
}

type BudgetStore interface {
    GetBudgetLimits(ctx context.Context, userID, projectID string) ([]BudgetLimit, error)
}
```

## Behavior

| Scenario | Result |
|----------|--------|
| Empty userID | Enforcement skipped (allowed) |
| No budget limits configured | Allowed |
| Spent < limit | Allowed |
| Spent >= limit (exact-at-limit) | **Denied** |
| Multiple periods configured | All checked; first exceeded blocks |
| Store error | **Error propagated** (fail-closed) |

### Period Ranges

| Period | Start | End |
|--------|-------|-----|
| `daily` | Midnight today | Midnight tomorrow |
| `weekly` | Monday 00:00 | Next Monday 00:00 |
| `monthly` | 1st of month 00:00 | 1st of next month 00:00 |

### Executor Integration

Budget is checked **once per run** (before the first phase), not per-phase:

```go
// In executor.executePhase(), guarded by budgetChecked flag
if e.BudgetEnforcer != nil && !e.budgetChecked {
    e.budgetChecked = true
    result, err := e.BudgetEnforcer.CheckBudget(ctx, costUserID, e.ProjectID)
    // err → propagated; !result.Allowed → fmt.Errorf("budget limit reached: %s", result.Reason)
}
```

Cost user ID falls back to OS username via `currentUsername()` if `CostUserID` is empty.

## Testing

| Test File | Tests | Coverage |
|-----------|-------|----------|
| `internal/budgets/enforcement_test.go` | 12 | No user, no limits, within/exceeds/exact limit, multiple periods, store errors, period range (daily/weekly/monthly/Sunday) |
| `internal/executor/executor_test.go` | 2 | Budget enforcement denied, cost user ID fallback |
