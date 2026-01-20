# Specification: Add model tracking and enhanced schema to cost_log

## Problem Statement

The global cost_log table lacks model differentiation, making it impossible to distinguish Opus vs Sonnet vs Haiku costs for analytics. Additionally, cache token tracking, initiative rollups, and budget management are not supported.

## Success Criteria

- [ ] `cost_log` table has `model` column populated for all new cost entries
- [ ] `cost_log` table has `iteration`, `cache_creation_tokens`, `cache_read_tokens`, `total_tokens`, `initiative_id`, `duration_ms` columns
- [ ] `cost_aggregates` table exists with `UNIQUE(project_id, model, phase, date)` constraint
- [ ] `cost_budgets` table exists with per-project monthly limits and alert thresholds
- [ ] Indexes exist: `idx_cost_model`, `idx_cost_model_timestamp`, `idx_cost_initiative`, `idx_cost_project_timestamp`
- [ ] `DetectModel()` function maps model IDs to simplified names (opus, sonnet, haiku, unknown)
- [ ] `RecordCostExtended()` method records all fields including model and cache tokens
- [ ] `GetCostByModel()` returns costs grouped by model for a time range
- [ ] `GetCostTimeseries()` returns time-bucketed cost data for charting (day/week/month granularity)
- [ ] `GetBudget()`, `SetBudget()`, `GetBudgetStatus()` manage project budgets
- [ ] Migration applies cleanly to existing databases without data loss
- [ ] Executor calls `recordCostToGlobal()` after each phase completion

## Testing Requirements

- [ ] Unit test: `TestDetectModel` verifies model ID parsing (opus, sonnet, haiku, unknown)
- [ ] Unit test: `TestMigration002_AppliesCleanly` verifies new columns and tables exist
- [ ] Unit test: `TestMigration002_Idempotent` verifies migration can run twice without error
- [ ] Unit test: `TestMigration002_PreservesData` verifies existing data survives migration
- [ ] Unit test: `TestMigration003_AddsDurationMs` verifies duration_ms column exists
- [ ] Unit test: `TestRecordCostExtended_AllFields` verifies all fields stored correctly
- [ ] Unit test: `TestGetCostByModel_GroupsCorrectly` verifies model cost aggregation
- [ ] Unit test: `TestGetCostTimeseries_DailyGranularity` verifies time bucketing
- [ ] Unit test: `TestGetCostTimeseries_Granularities` verifies day/week/month work
- [ ] Unit test: `TestCostAggregate_Upsert` verifies upsert behavior
- [ ] Unit test: `TestBudget_CRUD` verifies budget create/read/update
- [ ] Unit test: `TestBudgetStatus` verifies percent used and threshold calculations
- [ ] Integration test: `TestGlobalDB_CostWorkflow` verifies end-to-end workflow

## Scope

### In Scope

- Add model field and cache token columns to `cost_log` table
- Create `cost_aggregates` table for pre-computed time-series
- Create `cost_budgets` table for per-project budget tracking
- Add indexes for efficient analytics queries
- Implement `DetectModel()` function for model ID parsing
- Implement `RecordCostExtended()` for full field recording
- Implement `GetCostByModel()` for model-grouped costs
- Implement `GetCostTimeseries()` for time-bucketed data
- Implement budget CRUD and status methods
- Wire up executor to call `recordCostToGlobal()` after phase completion

### Out of Scope

- API endpoints for cost analytics (separate TASK)
- Frontend cost dashboard (separate TASK)
- Automated daily aggregate computation (future enhancement)
- Budget alert notifications (future enhancement)
- PostgreSQL-specific optimizations (SQLite primary for now)

## Technical Approach

### Schema Migration (global_002.sql)

Add columns to `cost_log`:
- `model TEXT DEFAULT ''` - Simplified model name
- `iteration INTEGER DEFAULT 0` - Phase iteration count
- `cache_creation_tokens INTEGER DEFAULT 0` - Prompt caching tokens created
- `cache_read_tokens INTEGER DEFAULT 0` - Prompt caching tokens read
- `total_tokens INTEGER DEFAULT 0` - Input + output total
- `initiative_id TEXT DEFAULT ''` - For initiative rollups

Create indexes:
- `idx_cost_model` - For model-specific queries
- `idx_cost_model_timestamp` - For model+time queries
- `idx_cost_initiative` - For initiative rollups
- `idx_cost_project_timestamp` - For project+time queries

Create `cost_aggregates` table with unique constraint on (project_id, model, phase, date).

Create `cost_budgets` table with unique constraint on project_id.

### Schema Migration (global_003.sql)

Add `duration_ms INTEGER DEFAULT 0` to `cost_log` for phase timing analytics.

### Go Implementation

1. **Types (global.go)**:
   - `CostEntry` struct with all fields
   - `CostAggregate` struct for time-series data
   - `CostBudget` struct for budget configuration
   - `BudgetStatus` struct for spend vs limit

2. **Methods (global.go)**:
   - `DetectModel(modelID string) string` - Parse model ID to simplified name
   - `RecordCostExtended(entry CostEntry) error` - Record with all fields
   - `GetCostByModel(projectID string, since time.Time) (map[string]float64, error)`
   - `GetCostTimeseries(projectID string, since time.Time, granularity string) ([]CostAggregate, error)`
   - `UpdateCostAggregate(agg CostAggregate) error`
   - `GetCostAggregates(projectID, startDate, endDate string) ([]CostAggregate, error)`
   - `GetBudget(projectID string) (*CostBudget, error)`
   - `SetBudget(budget CostBudget) error`
   - `GetBudgetStatus(projectID string) (*BudgetStatus, error)`

3. **Executor Integration (cost_tracking.go)**:
   - `CostMetadata` struct with Model, Iteration, Duration, ProjectPath
   - `recordCostToGlobal()` method called after phase completion

### Files to Modify

- `internal/db/schema/global_002.sql`: Add model columns, create aggregates/budgets tables
- `internal/db/schema/global_003.sql`: Add duration_ms column
- `internal/db/global.go`: Add types and methods
- `internal/executor/cost_tracking.go`: Add CostMetadata and recordCostToGlobal
- `internal/executor/task_execution.go`: Call recordCostToGlobal after phase completion
- `internal/db/global_test.go`: Add comprehensive tests

## Feature-Specific Analysis

### User Story

As a developer using orc, I want to track costs per model (Opus/Sonnet/Haiku) so that I can understand my spending patterns and optimize model selection for different task types.

### Acceptance Criteria

1. After running a task, I can query costs grouped by model
2. Cache token usage is tracked separately from regular tokens
3. Costs can be aggregated by day/week/month for trend analysis
4. I can set a monthly budget with alert thresholds per project
5. Budget status shows current spend vs limit and whether at threshold
6. Existing cost data is preserved during migration
7. Migration is idempotent (safe to run multiple times)
