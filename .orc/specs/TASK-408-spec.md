# Specification: Implement cost analytics API endpoints

## Problem Statement

The frontend dashboard needs comprehensive cost analytics data (breakdowns by model/phase/task/initiative, time-series, budget tracking), but the API server lacks endpoints to expose the existing GlobalDB cost tracking methods. While cost recording infrastructure exists and data is being collected, there's no way for the UI to access it.

## Success Criteria

### Endpoint Functionality
- [ ] `GET /api/cost/breakdown` returns cost data grouped by the specified dimension (model, phase, task, initiative) with period filtering (24h, 7d, 30d, all)
- [ ] `GET /api/cost/timeseries` returns time-bucketed cost data with configurable granularity (hour, day, week) and optional model filter
- [ ] `GET /api/cost/budget` returns current budget status including monthly limit, current spend, remaining, projected monthly, and on-track status
- [ ] `PUT /api/cost/budget` updates budget settings (monthly limit, alert threshold percentage)
- [ ] `GET /api/initiatives/:id/cost` returns cost rollup for a specific initiative including breakdown by task, model, and phase

### Data Accuracy
- [ ] Model breakdown percentages sum to 100% (within floating point tolerance)
- [ ] Timeseries data fills gaps with zero values for dates with no activity
- [ ] Budget calculations use correct month boundaries (1st to end of month)
- [ ] Initiative rollup includes costs from all linked tasks (verified against direct cost_log query)

### Integration
- [ ] All endpoints follow existing API patterns (Server method, cors wrapper, jsonResponse/jsonError helpers)
- [ ] Server struct has access to GlobalDB via new field (initialized in New())
- [ ] Routes registered in server.go with consistent naming under `/api/cost/`

### Performance
- [ ] 30-day breakdown query completes in < 200ms (tested with 1000+ cost_log entries)
- [ ] Budget status query returns immediately (single row lookup + aggregation)

## Testing Requirements

- [ ] Unit test: `TestHandleCostBreakdown` - verifies all breakdown dimensions (model, phase, task, initiative) with various periods
- [ ] Unit test: `TestHandleCostTimeseries` - verifies granularity options and gap filling
- [ ] Unit test: `TestHandleCostBudget` - verifies GET returns correct calculations and PUT updates persist
- [ ] Unit test: `TestHandleInitiativeCost` - verifies rollup includes all linked task costs
- [ ] Integration test: Verify GlobalDB data flows correctly to API responses
- [ ] Unit test: `TestGetInitiativeCost` in `internal/db/global_test.go` - new database method

## Scope

### In Scope
- 5 new API endpoints as specified
- Server struct modification to hold GlobalDB reference
- New handler file `internal/api/handlers_cost.go`
- New GlobalDB method `GetInitiativeCost(initiativeID)` for per-initiative rollup
- Route registration in server.go
- Request/response types following existing patterns

### Out of Scope
- Frontend components (separate task)
- Aggregate tables/materialized views (future optimization if needed)
- Cost alerts/notifications (separate feature)
- Historical data migration (existing cost_log data is sufficient)
- CSV export (frontend can do this client-side)

## Technical Approach

### Architecture Decision
The API server needs GlobalDB access. Two options:
1. **Add globalDB field to Server struct** (recommended) - Clean, testable, consistent with executor pattern
2. Open GlobalDB per-request - Wasteful, connection overhead

Recommendation: Option 1 - Add `globalDB *db.GlobalDB` field to Server struct.

### Response Types

```go
// CostBreakdownResponse for GET /api/cost/breakdown
type CostBreakdownResponse struct {
    Period       string                    `json:"period"`       // "24h", "7d", "30d", "all"
    TotalCostUSD float64                   `json:"total_cost_usd"`
    Breakdown    map[string]CostBreakdown  `json:"breakdown"`    // keyed by model/phase/task/initiative
}

type CostBreakdown struct {
    CostUSD float64 `json:"cost"`
    Tokens  int     `json:"tokens"`
    Percent float64 `json:"percent"`
}

// CostTimeseriesResponse for GET /api/cost/timeseries
type CostTimeseriesResponse struct {
    Start       string              `json:"start"`
    End         string              `json:"end"`
    Granularity string              `json:"granularity"` // "hour", "day", "week"
    Series      []CostTimeseriesPoint `json:"series"`
}

type CostTimeseriesPoint struct {
    Date   string  `json:"date"`   // YYYY-MM-DD or YYYY-MM-DD HH:00
    CostUSD float64 `json:"cost"`
    Tokens int     `json:"tokens"`
    Model  string  `json:"model,omitempty"` // when filtered or grouped
}

// BudgetStatusResponse for GET /api/cost/budget
type BudgetStatusResponse struct {
    MonthlyLimitUSD   float64 `json:"monthly_limit_usd"`
    CurrentSpentUSD   float64 `json:"current_spent_usd"`
    RemainingUSD      float64 `json:"remaining_usd"`
    PercentUsed       float64 `json:"percent_used"`
    ProjectedMonthly  float64 `json:"projected_monthly"` // extrapolated from days elapsed
    DaysRemaining     int     `json:"days_remaining"`
    OnTrack           bool    `json:"on_track"`          // projected <= limit
}

// BudgetUpdateRequest for PUT /api/cost/budget
type BudgetUpdateRequest struct {
    MonthlyLimitUSD       float64 `json:"monthly_limit_usd"`
    AlertThresholdPercent int     `json:"alert_threshold_percent,omitempty"` // default 80
}

// InitiativeCostResponse for GET /api/initiatives/:id/cost
type InitiativeCostResponse struct {
    InitiativeID string            `json:"initiative_id"`
    TotalCostUSD float64           `json:"total_cost_usd"`
    ByTask       []TaskCost        `json:"by_task"`
    ByModel      map[string]float64 `json:"by_model"`
    ByPhase      map[string]float64 `json:"by_phase"`
}

type TaskCost struct {
    TaskID  string  `json:"task_id"`
    CostUSD float64 `json:"cost"`
    Tokens  int     `json:"tokens"`
}
```

### Files to Modify

| File | Change |
|------|--------|
| `internal/api/server.go` | Add `globalDB *db.GlobalDB` field to Server struct; initialize in `New()` with `db.OpenGlobal()` |
| `internal/api/server.go` | Register 5 new routes in `registerRoutes()` |
| `internal/api/handlers_cost.go` | **NEW** - Implement all 5 handlers |
| `internal/db/global.go` | Add `GetInitiativeCost(initiativeID string) (*InitiativeCost, error)` method |
| `internal/db/global_test.go` | Add `TestGetInitiativeCost` |
| `internal/api/handlers_cost_test.go` | **NEW** - Unit tests for handlers |

### Handler Implementation Pattern

```go
// Example: handleCostBreakdown
func (s *Server) handleCostBreakdown(w http.ResponseWriter, r *http.Request) {
    if s.globalDB == nil {
        s.jsonError(w, "cost analytics not available", http.StatusServiceUnavailable)
        return
    }

    by := r.URL.Query().Get("by")
    if by == "" {
        by = "model" // default
    }

    period := r.URL.Query().Get("period")
    start, end := parsePeriod(period) // new helper

    projectID := s.getProjectID() // derive from workDir

    switch by {
    case "model":
        breakdown, err := s.globalDB.GetCostByModel(projectID, start)
        // ... format response
    case "phase":
        // ... use GetCostSummary or new method
    case "task":
        // ... aggregate from cost_log
    case "initiative":
        // ... aggregate from cost_log grouped by initiative_id
    default:
        s.jsonError(w, "invalid 'by' parameter: use model, phase, task, or initiative", http.StatusBadRequest)
        return
    }

    s.jsonResponse(w, response)
}
```

### Query Parameter Validation

| Endpoint | Param | Valid Values | Default |
|----------|-------|--------------|---------|
| breakdown | by | model, phase, task, initiative | model |
| breakdown | period | 24h, 7d, 30d, all | all |
| timeseries | granularity | hour, day, week | day |
| timeseries | start/end | RFC3339 or YYYY-MM-DD | last 30 days |
| timeseries | model | opus, sonnet, haiku | (none = all) |

### GlobalDB Method Addition

```go
// GetInitiativeCost returns cost aggregation for a specific initiative.
// Returns costs grouped by task, model, and phase.
func (g *GlobalDB) GetInitiativeCost(initiativeID string) (*InitiativeCost, error) {
    // Query cost_log WHERE initiative_id = ?
    // Aggregate by task_id, model, phase
    // Return structured result
}

type InitiativeCost struct {
    InitiativeID string
    TotalCostUSD float64
    TotalTokens  int
    ByTask       []TaskCostEntry
    ByModel      map[string]float64
    ByPhase      map[string]float64
}
```

## Feature-Specific Analysis

### User Story
As a **developer using orc**, I want **to view cost analytics in the dashboard** so that **I can understand my API spending patterns, track costs by model/phase/task, and stay within budget**.

### Acceptance Criteria
1. **Breakdown by model**: When viewing cost breakdown by model, I see Opus/Sonnet/Haiku costs with percentages that correctly reflect my actual usage from cost_log
2. **Time trends**: When viewing timeseries, I can see cost trends over time with appropriate granularity, allowing me to identify spending spikes
3. **Budget awareness**: When viewing budget status, I see clear indicators of current spend vs limit and whether I'm on track for the month
4. **Initiative tracking**: When viewing initiative costs, I see the total investment across all tasks linked to that initiative
5. **Period filtering**: All endpoints support period filtering so I can focus on recent activity or see all-time data

### Data Dependencies
- Requires `cost_log` table to have data (populated by `recordCostToGlobal()` in executor)
- Budget endpoints require `cost_budgets` table entries (can be created via PUT endpoint)
- Initiative cost requires tasks to have `initiative_id` set and cost_log entries with matching `initiative_id`

### Error Handling
| Scenario | HTTP Status | Response |
|----------|-------------|----------|
| GlobalDB unavailable | 503 | `{"error": "cost analytics not available"}` |
| Invalid 'by' parameter | 400 | `{"error": "invalid 'by' parameter: use model, phase, task, or initiative"}` |
| Invalid period | 400 | `{"error": "invalid 'period' parameter: use 24h, 7d, 30d, or all"}` |
| Initiative not found | 404 | `{"error": "initiative not found"}` |
| No budget set | 200 | Return zeroed BudgetStatusResponse with `monthly_limit_usd: 0` |
