# Executor Package

The executor runs task phases sequentially, managing Claude Code invocations and phase transitions.

## Architecture

```
Executor
├── PhaseRunner (interface)
│   └── ClaudeRunner (production)
├── Storage Backend
├── Prompt Service
├── Event Publisher
├── Git Service
├── Gate Evaluator
├── Trigger Engine
└── Budget Enforcer (budgets.Enforcer)
```

## Key Types

| Type | Purpose |
|------|---------|
| `Executor` | Main orchestrator - runs phases in sequence |
| `PhaseRunner` | Interface for running a single phase |
| `ClaudeRunner` | Production runner using Claude Code CLI |
| `PhaseResult` | Output from a phase execution |
| `RunOptions` | Configuration for a task run |

## Execution Flow

1. **Pre-run**: Validate task, setup worktree, check budget (once per run)
2. **Phase loop**: For each phase in workflow:
   a. Check if phase already completed (skip if so)
   b. Render prompt template with variables
   c. Execute via PhaseRunner
   d. Parse completion status from output
   e. Evaluate quality gate (auto/human/AI/skip)
   f. Publish events
   g. Git commit phase results
3. **Post-run**: Create PR if configured, update task status

## Budget Enforcement

Pre-execution spending check via `Executor.BudgetEnforcer` (`internal/budgets/enforcement.go`). Checked **once per run** before the first phase executes (`budgetChecked` flag), not per-phase.

| Behavior | Detail |
|----------|--------|
| `BudgetEnforcer` is nil | Enforcement skipped (default) |
| Empty userID | Enforcement skipped (allowed) |
| No limits configured | Allowed |
| Spent >= limit | Denied (exact-at-limit = denied) |
| Multiple periods | All checked; first exceeded blocks |
| Store errors | Propagated (fail-closed), never swallowed |

**Periods:** `daily`, `weekly` (Monday start), `monthly`. Uses `budgets.CostStore` and `budgets.BudgetStore` interfaces.
**CostUserID:** Falls back to OS username via `currentUsername()` if empty.

## Phase Completion Detection

Phases signal completion via JSON output:
```json
{"status": "complete", "summary": "implemented feature X"}
{"status": "blocked", "reason": "need API credentials"}
```

Parsed by `CheckPhaseCompletionJSON()` - returns structured result or error.

## Gate Integration

After each phase, the gate evaluator determines if work can proceed:
- `auto`: Always passes (default for most phases)
- `human`: Pauses for manual approval
- `ai`: AI evaluates quality
- `skip`: Bypasses the gate entirely

## PR Creation Flow

When task completes with `completion.action: pr`:

1. **Check for existing PR**: Query hosting provider for open PRs from the task branch
2. **If PR exists**: Reuse it (update description if needed) - prevents duplicates from re-runs
3. **If no PR**: Create new PR with spec-derived title and description
4. **Apply options**: Draft mode, labels, reviewers from task config

This makes PR creation **idempotent** - safe to re-run tasks without creating duplicate PRs.

## Retry Logic

Failed phases can be retried with context from the failure:
- `RETRY_ATTEMPT`: Current attempt number
- `RETRY_REASON`: Why previous attempt failed
- `RETRY_FEEDBACK`: Specific feedback for improvement

## Key Dependencies

| Dependency | Purpose |
|------------|---------|
| `prompt.Service` | Renders phase templates |
| `storage.Backend` | Persists task state |
| `events.Publisher` | Real-time updates |
| `git.Service` | Worktree + commit management |
| `gate.Evaluator` | Quality gate decisions |
| `trigger.Engine` | Lifecycle event triggers |
| `budgets.Enforcer` | Pre-run spending limit checks |
