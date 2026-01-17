# Cost Tracking

**Status**: Database Layer Implemented (TASK-406)
**Priority**: P1
**Last Updated**: 2026-01-17

---

## Implementation Status

### Completed (TASK-406)

| Component | Status | Location |
|-----------|--------|----------|
| Schema Migration | ✅ Done | `internal/db/schema/global_002.sql` |
| CostEntry struct | ✅ Done | `internal/db/global.go` |
| CostAggregate struct | ✅ Done | `internal/db/global.go` |
| CostBudget struct | ✅ Done | `internal/db/global.go` |
| BudgetStatus struct | ✅ Done | `internal/db/global.go` |
| RecordCostExtended() | ✅ Done | `internal/db/global.go` |
| GetCostByModel() | ✅ Done | `internal/db/global.go` |
| GetCostTimeseries() | ✅ Done | `internal/db/global.go` |
| UpdateCostAggregate() | ✅ Done | `internal/db/global.go` |
| GetCostAggregates() | ✅ Done | `internal/db/global.go` |
| GetBudget() / SetBudget() | ✅ Done | `internal/db/global.go` |
| GetBudgetStatus() | ✅ Done | `internal/db/global.go` |
| DetectModel() utility | ✅ Done | `internal/db/global.go` |
| Test coverage | ✅ Done | `internal/db/global_test.go` |

### Pending (Future Tasks)

| Component | Depends On |
|-----------|------------|
| Executor integration (call RecordCostExtended) | TASK-407 |
| API endpoints (`/api/cost/*`) | TASK-406 complete |
| CLI command (`orc cost`) | TASK-406 complete |
| Web dashboard cost widget | API endpoints |
| Budget alerting logic | Budget infrastructure |

---

## Problem Statement

Users have no visibility into:
- How many tokens tasks consume
- Estimated cost of task execution
- Historical usage trends
- Cost comparison between approaches

---

## Solution: Comprehensive Token & Cost Tracking

Track tokens at every level:
- Per-iteration
- Per-phase
- Per-task
- Per-project (aggregate)

Display estimated costs based on model pricing.

---

## Token Tracking

### Data Captured Per Iteration

```go
type IterationTokens struct {
    Iteration    int       `yaml:"iteration"`
    Phase        string    `yaml:"phase"`
    Timestamp    time.Time `yaml:"timestamp"`
    InputTokens  int       `yaml:"input_tokens"`
    OutputTokens int       `yaml:"output_tokens"`
    CacheRead    int       `yaml:"cache_read,omitempty"`
    CacheWrite   int       `yaml:"cache_write,omitempty"`
    Model        string    `yaml:"model"`
}
```

### Aggregation Levels

```yaml
# .orc/tasks/TASK-001/state.yaml
tokens:
  total:
    input: 45234
    output: 12456
    cache_read: 8000
    cache_write: 2000
    total: 67690

  by_phase:
    spec:
      input: 12000
      output: 4500
      iterations: 2
    implement:
      input: 28000
      output: 7000
      iterations: 5
    test:
      input: 5234
      output: 956
      iterations: 1

  iterations:
    - iteration: 1
      phase: spec
      input_tokens: 6000
      output_tokens: 2500
      timestamp: 2026-01-10T14:30:00Z
    # ...
```

---

## Cost Estimation

### Model Pricing Configuration

```yaml
# ~/.orc/pricing.yaml (user can override)
models:
  claude-opus-4-5-20251101:
    input_per_million: 15.00
    output_per_million: 75.00
    cache_read_per_million: 1.50
    cache_write_per_million: 18.75

  claude-sonnet-4-20250514:
    input_per_million: 3.00
    output_per_million: 15.00
    cache_read_per_million: 0.30
    cache_write_per_million: 3.75
```

### Cost Calculation

```go
type CostEstimate struct {
    InputCost      float64 `json:"input_cost"`
    OutputCost     float64 `json:"output_cost"`
    CacheReadCost  float64 `json:"cache_read_cost"`
    CacheWriteCost float64 `json:"cache_write_cost"`
    TotalCost      float64 `json:"total_cost"`
    Currency       string  `json:"currency"` // USD
}

func calculateCost(tokens *TokenUsage, model string) *CostEstimate {
    pricing := getPricing(model)

    return &CostEstimate{
        InputCost:      float64(tokens.Input) / 1_000_000 * pricing.InputPerMillion,
        OutputCost:     float64(tokens.Output) / 1_000_000 * pricing.OutputPerMillion,
        CacheReadCost:  float64(tokens.CacheRead) / 1_000_000 * pricing.CacheReadPerMillion,
        CacheWriteCost: float64(tokens.CacheWrite) / 1_000_000 * pricing.CacheWritePerMillion,
        TotalCost:      /* sum of above */,
        Currency:       "USD",
    }
}
```

---

## CLI Display

### Task Show

```bash
$ orc show TASK-001

TASK-001 - Fix auth timeout bug
────────────────────────────────
Status:    ✅ completed
Weight:    small
Duration:  12m 34s

Phases:
  ● implement  (5 iterations, 35,000 tokens)
  ● test       (1 iteration, 6,190 tokens)

Token Usage:
  Input:    45,234 tokens
  Output:   12,456 tokens
  Cache:    8,000 read / 2,000 write
  Total:    67,690 tokens

Estimated Cost: $1.47 USD
  Input:    $0.68
  Output:   $0.93
  Cache:    -$0.14 (savings)
```

### Task List with Cost

```bash
$ orc list --cost

ID         STATUS  WEIGHT  TOKENS     COST    TITLE
──         ──────  ──────  ──────     ────    ─────
TASK-001   ✅      small   67.7K      $1.47   Fix auth timeout bug
TASK-002   ⏳      large   124.3K     $2.89   Implement dashboard
TASK-003   📋      medium  -          -       Add caching layer

Total: 192K tokens, $4.36 estimated
```

### Cost Summary

```bash
$ orc cost

Token Usage Summary
───────────────────

Today:
  Tasks:    3
  Tokens:   192,000
  Cost:     $4.36

This Week:
  Tasks:    12
  Tokens:   1.2M
  Cost:     $28.50

This Month:
  Tasks:    45
  Tokens:   5.8M
  Cost:     $142.30

By Model:
  claude-opus-4-5:    4.2M tokens ($128.00)
  claude-sonnet:      1.6M tokens ($14.30)

Top Tasks by Cost:
  1. TASK-042 (large)   $18.50  Implement user auth
  2. TASK-038 (large)   $15.20  Database migration
  3. TASK-045 (medium)  $8.40   API refactor
```

---

## Web UI Display

### Task Card

```
┌─────────────────────────────────────────────────────────────┐
│ ✅ TASK-001                                         [small] │
│   Fix auth timeout bug                                      │
│                                                             │
│   ● implement ─── ● test                                    │
│                                                             │
│   Completed in 12m 34s                                      │
│   67.7K tokens • $1.47                                      │
└─────────────────────────────────────────────────────────────┘
```

### Task Detail - Tokens Tab

```
┌─ Token Usage ───────────────────────────────────────────────┐
│                                                             │
│  Total: 67,690 tokens ($1.47)                               │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ Phase      │ Iterations │ Tokens  │ Cost    │       │   │
│  ├────────────┼────────────┼─────────┼─────────┤       │   │
│  │ implement  │ 5          │ 55,000  │ $1.21   │ ████  │   │
│  │ test       │ 1          │ 12,690  │ $0.26   │ █     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  Token Breakdown:                                           │
│  ┌─────────────┬────────────┬────────────┐                 │
│  │ Input       │ 45,234     │ $0.68      │                 │
│  │ Output      │ 12,456     │ $0.93      │                 │
│  │ Cache Read  │ 8,000      │ -$0.12     │                 │
│  │ Cache Write │ 2,000      │ $0.04      │                 │
│  └─────────────┴────────────┴────────────┘                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Dashboard - Cost Summary Widget

```
┌─ Token Usage ───────────────────────────────────────────────┐
│                                                             │
│  Today         This Week      This Month                    │
│  192K          1.2M           5.8M                          │
│  $4.36         $28.50         $142.30                       │
│                                                             │
│  ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█▇▆▅▄▃▂                             │
│  Last 30 days                                               │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## API Endpoints

### Task Token Info

```
GET /api/tasks/:id/tokens

Response:
{
  "task_id": "TASK-001",
  "tokens": {
    "input": 45234,
    "output": 12456,
    "cache_read": 8000,
    "cache_write": 2000,
    "total": 67690
  },
  "cost": {
    "input": 0.68,
    "output": 0.93,
    "cache_read": -0.12,
    "cache_write": 0.04,
    "total": 1.47,
    "currency": "USD"
  },
  "by_phase": {
    "implement": { "tokens": 55000, "cost": 1.21, "iterations": 5 },
    "test": { "tokens": 12690, "cost": 0.26, "iterations": 1 }
  }
}
```

### Project Cost Summary

```
GET /api/cost/summary?period=week

Response:
{
  "period": "week",
  "start": "2026-01-04",
  "end": "2026-01-10",
  "tasks_completed": 12,
  "tokens": {
    "input": 890000,
    "output": 310000,
    "total": 1200000
  },
  "cost": {
    "total": 28.50,
    "currency": "USD"
  },
  "by_model": {
    "claude-opus-4-5-20251101": { "tokens": 800000, "cost": 24.00 },
    "claude-sonnet-4-20250514": { "tokens": 400000, "cost": 4.50 }
  },
  "by_day": [
    { "date": "2026-01-04", "tokens": 150000, "cost": 3.50 },
    // ...
  ]
}
```

---

## Budget Alerts (Optional)

```yaml
# .orc/config.yaml
budget:
  enabled: true
  daily_limit: 10.00      # USD
  weekly_limit: 50.00
  monthly_limit: 200.00
  alert_threshold: 0.8    # Alert at 80% of limit
```

### Alert Display

```
⚠️  Budget Alert: 82% of daily limit used

Today's usage: $8.20 / $10.00
Remaining: $1.80

Options:
  • Wait until tomorrow (resets at midnight)
  • Increase limit: orc config budget.daily_limit 20
  • Disable alerts: orc config budget.enabled false
```

---

## Data Storage

### Per-Task (state.yaml)

Detailed token breakdown stored with task state.

### Per-Project (Aggregate)

```yaml
# .orc/usage.yaml
last_updated: 2026-01-10T15:00:00Z

totals:
  all_time:
    tokens: 5800000
    cost: 142.30
    tasks: 45

  this_month:
    tokens: 5800000
    cost: 142.30
    tasks: 45

  this_week:
    tokens: 1200000
    cost: 28.50
    tasks: 12

  today:
    tokens: 192000
    cost: 4.36
    tasks: 3

daily_history:
  - date: 2026-01-10
    tokens: 192000
    cost: 4.36
    tasks: 3
  - date: 2026-01-09
    tokens: 280000
    cost: 6.50
    tasks: 4
  # Last 30 days...
```

---

## Database Schema (Implemented)

### Migration: global_002.sql

The cost tracking schema is added via `internal/db/schema/global_002.sql`:

**Extended cost_log columns:**

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `model` | TEXT | `''` | Claude model used (opus, sonnet, haiku) |
| `iteration` | INTEGER | `0` | Iteration within phase |
| `cache_creation_tokens` | INTEGER | `0` | Tokens written to prompt cache |
| `cache_read_tokens` | INTEGER | `0` | Tokens served from cache (90% cheaper) |
| `total_tokens` | INTEGER | `0` | Effective total for analytics |
| `initiative_id` | TEXT | `''` | Links costs to initiative |

**New indexes on cost_log:**

| Index | Columns | Purpose |
|-------|---------|---------|
| `idx_cost_model` | `model` | Model-based queries |
| `idx_cost_model_timestamp` | `model, timestamp` | Time-range queries by model |
| `idx_cost_initiative` | `initiative_id` | Initiative cost analysis |
| `idx_cost_project_timestamp` | `project_id, timestamp` | Project timeline queries |

**cost_aggregates table:**

Pre-computed time-series data for efficient dashboard queries:

```sql
CREATE TABLE cost_aggregates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL,
    model TEXT DEFAULT '',
    phase TEXT DEFAULT '',
    date TEXT NOT NULL,                    -- YYYY-MM-DD
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_tokens INTEGER DEFAULT 0,
    turn_count INTEGER DEFAULT 0,
    task_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(project_id, model, phase, date)
);
```

**cost_budgets table:**

Monthly budget tracking:

```sql
CREATE TABLE cost_budgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT UNIQUE NOT NULL,
    monthly_limit_usd REAL,
    alert_threshold_percent INTEGER DEFAULT 80,
    current_month TEXT DEFAULT '',         -- YYYY-MM
    current_month_spent REAL DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

### Go Types

```go
// CostEntry - complete snapshot of a Claude API call
type CostEntry struct {
    ID                  int64
    ProjectID           string
    TaskID              string
    Phase               string
    Model               string    // opus, sonnet, haiku, unknown
    Iteration           int
    CostUSD             float64
    InputTokens         int
    OutputTokens        int
    CacheCreationTokens int
    CacheReadTokens     int
    TotalTokens         int
    InitiativeID        string
    Timestamp           time.Time
}

// CostAggregate - pre-computed rollups for charting
type CostAggregate struct {
    ID                int64
    ProjectID         string
    Model             string
    Phase             string
    Date              string  // YYYY-MM-DD
    TotalCostUSD      float64
    TotalInputTokens  int
    TotalOutputTokens int
    TotalCacheTokens  int
    TurnCount         int
    TaskCount         int
    CreatedAt         time.Time
}

// CostBudget - monthly budget configuration
type CostBudget struct {
    ID                    int64
    ProjectID             string
    MonthlyLimitUSD       float64
    AlertThresholdPercent int     // e.g., 80 for alert at 80%
    CurrentMonth          string  // YYYY-MM
    CurrentMonthSpent     float64
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

// BudgetStatus - computed view for UI
type BudgetStatus struct {
    ProjectID         string
    MonthlyLimitUSD   float64
    CurrentMonthSpent float64
    CurrentMonth      string
    PercentUsed       float64
    AlertThreshold    int
    OverBudget        bool
    AtAlertThreshold  bool
}
```

### GlobalDB Methods

| Method | Purpose |
|--------|---------|
| `RecordCost()` | *Deprecated* - Use RecordCostExtended |
| `RecordCostExtended(entry)` | Record cost with model and cache tokens |
| `DetectModel(modelID)` | Normalize model ID to opus/sonnet/haiku/unknown |
| `GetCostByModel(projectID, since)` | Cost breakdown by model |
| `GetCostTimeseries(projectID, since, granularity)` | Time-bucketed costs (day/week/month) |
| `UpdateCostAggregate(agg)` | Upsert aggregate record |
| `GetCostAggregates(projectID, start, end)` | Retrieve pre-computed aggregates |
| `GetBudget(projectID)` | Get budget config (nil if none) |
| `SetBudget(budget)` | Create/update budget |
| `GetBudgetStatus(projectID)` | Current spend vs limit |

---

## Implementation Notes

### Capturing Tokens

Tokens come from Claude Code output (JSON format):

```json
{
  "type": "result",
  "session_id": "...",
  "usage": {
    "input_tokens": 12000,
    "output_tokens": 4500,
    "cache_read_input_tokens": 2000,
    "cache_creation_input_tokens": 500
  }
}
```

### Effective vs Raw Token Counts

**Important:** Raw `input_tokens` alone can appear misleadingly low when prompt caching is active. Claude splits input across three fields:

| Field | Description |
|-------|-------------|
| `input_tokens` | Uncached portion of input (can be as low as 50-100 tokens) |
| `cache_creation_input_tokens` | Tokens being written to cache this turn |
| `cache_read_input_tokens` | Tokens served from cache (often 10K-50K+) |

**Effective input tokens** = `input_tokens` + `cache_creation_input_tokens` + `cache_read_input_tokens`

Example: A 28K token prompt might report `input_tokens: 56` with `cache_read_input_tokens: 27944`. Displaying just the raw value would be misleading.

All executors use `EffectiveInputTokens()` to show the actual context size. See `internal/executor/session_adapter.go` for implementation.

### Pricing Updates

- Default pricing bundled with orc
- User can override in `~/.orc/pricing.yaml`
- Consider fetching from API in future

### Cache Savings

Cache usage reduces costs significantly:
- Cache read: 90% cheaper than regular input
- Show savings explicitly to encourage patterns that use cache

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for cost/token tracking code
- 100% coverage for cost calculation functions

### Unit Tests

| Test | Description |
|------|-------------|
| `TestCalculateCost` | Verify cost calculation with various token counts |
| `TestCalculateCost_CacheReadSavings` | Cache read reduces cost correctly |
| `TestCalculateCost_ZeroTokens` | Handles zero values gracefully |
| `TestCalculateCost_MixedModels` | Different pricing for different models |
| `TestAggregateTokens_ByPhase` | Correctly sums per-phase tokens |
| `TestAggregateTokens_Total` | Correctly sums total tokens |
| `TestParsePricingConfig` | Parses `~/.orc/pricing.yaml` |
| `TestParsePricingConfig_Defaults` | Falls back to bundled defaults |
| `TestBudgetAlert_ThresholdTriggered` | Alert at 80% threshold |
| `TestBudgetAlert_DailyReset` | Resets at midnight |
| `TestFormatTokens` | Formats 192000 as "192K" |
| `TestFormatCost` | Formats 1.47 as "$1.47" |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestTaskExecutionCapturesTokens` | Tokens recorded during execution |
| `TestStateYAMLContainsTokens` | state.yaml has tokens section after run |
| `TestUsageYAMLAggregation` | `.orc/usage.yaml` aggregates correctly |
| `TestAPIGetTaskTokens` | `/api/tasks/:id/tokens` returns data |
| `TestAPICostSummary_DayPeriod` | `/api/cost/summary?period=day` |
| `TestAPICostSummary_WeekPeriod` | `/api/cost/summary?period=week` |
| `TestAPICostSummary_MonthPeriod` | `/api/cost/summary?period=month` |
| `TestCLIOrcCostCommand` | `orc cost` outputs summary |
| `TestCLIOrcShowIncludesTokens` | `orc show TASK-ID` shows tokens |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_dashboard_token_widget` | `browser_navigate`, `browser_snapshot` | Dashboard shows token usage widget |
| `test_task_card_displays_cost` | `browser_snapshot` | Task card displays token count and cost |
| `test_task_detail_tokens_tab` | `browser_click`, `browser_snapshot` | Tokens tab shows breakdown |
| `test_budget_alert_appears` | `browser_snapshot` | Alert when threshold reached |
| `test_cost_summary_page` | `browser_navigate`, `browser_snapshot` | Cost page loads with chart |
| `test_stat_card_filtering` | `browser_click`, `browser_snapshot` | Clicking stat cards filters |

### Test Fixtures
- Sample state.yaml with token data
- Mock pricing.yaml configurations
- Sample usage.yaml with historical data

---

## Success Criteria

- [ ] Every iteration captures token counts
- [ ] Costs displayed on task cards and details
- [ ] `orc cost` shows summary by period
- [ ] API returns token/cost data
- [ ] Web UI shows token widget on dashboard
- [ ] Budget alerts work when configured
- [ ] Cache savings are highlighted
- [ ] Historical data available for trends
- [ ] 80%+ test coverage on cost tracking code
- [ ] All E2E tests pass
