package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// ErrBudgetNotFound is returned when no budget exists for a project.
var ErrBudgetNotFound = errors.New("budget not found")

// GlobalDB provides operations on the global database (~/.orc/orc.db).
type GlobalDB struct {
	*DB
}

// OpenGlobal opens the global database at ~/.orc/orc.db using SQLite.
func OpenGlobal() (*GlobalDB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	path := filepath.Join(home, ".orc", "orc.db")
	db, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("global"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate global db: %w", err)
	}

	return &GlobalDB{DB: db}, nil
}

// OpenGlobalWithDialect opens the global database with a specific dialect.
// For SQLite, dsn is the file path. For PostgreSQL, dsn is the connection string.
func OpenGlobalWithDialect(dsn string, dialect driver.Dialect) (*GlobalDB, error) {
	db, err := OpenWithDialect(dsn, dialect)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("global"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate global db: %w", err)
	}

	return &GlobalDB{DB: db}, nil
}

// Project represents a registered project.
type Project struct {
	ID        string
	Name      string
	Path      string
	Language  string
	CreatedAt time.Time
}

// SyncProject registers or updates a project in the global registry.
func (g *GlobalDB) SyncProject(p Project) error {
	_, err := g.Exec(`
		INSERT INTO projects (id, name, path, language, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			path = excluded.path,
			language = excluded.language
	`, p.ID, p.Name, p.Path, p.Language, p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("sync project: %w", err)
	}
	return nil
}

// GetProject retrieves a project by ID.
func (g *GlobalDB) GetProject(id string) (*Project, error) {
	row := g.QueryRow("SELECT id, name, path, language, created_at FROM projects WHERE id = ?", id)

	var p Project
	var createdAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		p.CreatedAt = t
	}

	return &p, nil
}

// GetProjectByPath retrieves a project by its filesystem path.
func (g *GlobalDB) GetProjectByPath(path string) (*Project, error) {
	row := g.QueryRow("SELECT id, name, path, language, created_at FROM projects WHERE path = ?", path)

	var p Project
	var createdAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
		return nil, fmt.Errorf("get project by path %s: %w", path, err)
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		p.CreatedAt = t
	}

	return &p, nil
}

// ListProjects returns all registered projects.
func (g *GlobalDB) ListProjects() ([]Project, error) {
	rows, err := g.Query("SELECT id, name, path, language, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			p.CreatedAt = t
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

// DeleteProject removes a project from the registry.
func (g *GlobalDB) DeleteProject(id string) error {
	_, err := g.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// CostEntry represents a cost log entry.
type CostEntry struct {
	ID                  int64
	ProjectID           string
	TaskID              string
	Phase               string
	Model               string
	Iteration           int
	CostUSD             float64
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	TotalTokens         int
	InitiativeID        string
	DurationMs          int64 // Phase execution duration in milliseconds
	Timestamp           time.Time
}

// CostAggregate represents aggregated cost data for time-series queries.
type CostAggregate struct {
	ID                int64
	ProjectID         string
	Model             string
	Phase             string
	Date              string // YYYY-MM-DD
	TotalCostUSD      float64
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheTokens  int
	TurnCount         int
	TaskCount         int
	CreatedAt         time.Time
}

// CostBudget represents budget tracking for a project.
type CostBudget struct {
	ID                    int64
	ProjectID             string
	MonthlyLimitUSD       float64
	AlertThresholdPercent int
	CurrentMonth          string // YYYY-MM
	CurrentMonthSpent     float64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// BudgetStatus represents the current spend vs limit.
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

// RecordCost logs a cost entry.
//
// Deprecated: Use RecordCostExtended for full model and cache token tracking.
// This method does not support model identification, cache tokens, or initiative tracking.
func (g *GlobalDB) RecordCost(projectID, taskID, phase string, costUSD float64, inputTokens, outputTokens int) error {
	_, err := g.Exec(`
		INSERT INTO cost_log (project_id, task_id, phase, cost_usd, input_tokens, output_tokens)
		VALUES (?, ?, ?, ?, ?, ?)
	`, projectID, taskID, phase, costUSD, inputTokens, outputTokens)
	if err != nil {
		return fmt.Errorf("record cost: %w", err)
	}
	return nil
}

// CostSummary provides aggregated cost data.
type CostSummary struct {
	TotalCostUSD float64
	TotalInput   int
	TotalOutput  int
	EntryCount   int
	ByProject    map[string]float64
	ByPhase      map[string]float64
}

// GetCostSummary retrieves aggregated cost data since the given time.
// Uses a CTE (Common Table Expression) to filter cost_log once and then
// perform all aggregations against the filtered result set, improving
// performance from O(3n) to O(n) table scans when projectID filter is applied.
func (g *GlobalDB) GetCostSummary(projectID string, since time.Time) (*CostSummary, error) {
	summary := &CostSummary{
		ByProject: make(map[string]float64),
		ByPhase:   make(map[string]float64),
	}

	// Format timestamp once for all queries
	// SQLite stores as "YYYY-MM-DD HH:MM:SS", RFC3339 is "YYYY-MM-DDTHH:MM:SSZ"
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	// Build query using CTE to filter once, aggregate multiple times
	// Args: sinceStr (and projectID if provided)
	var args []any

	query := `
		WITH filtered AS (
			SELECT project_id, phase, cost_usd, input_tokens, output_tokens
			FROM cost_log
			WHERE timestamp >= ?`

	args = append(args, sinceStr)
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	query += `
		)
		SELECT 'total' as breakdown_type, '' as breakdown_key,
			COALESCE(SUM(cost_usd), 0), COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0), COUNT(*)
		FROM filtered
		UNION ALL
		SELECT 'project' as breakdown_type, project_id as breakdown_key,
			COALESCE(SUM(cost_usd), 0), 0, 0, 0
		FROM filtered
		GROUP BY project_id
		UNION ALL
		SELECT 'phase' as breakdown_type, phase as breakdown_key,
			COALESCE(SUM(cost_usd), 0), 0, 0, 0
		FROM filtered
		GROUP BY phase`

	rows, err := g.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cost summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var breakdownType, breakdownKey string
		var cost float64
		var input, output, count int
		if err := rows.Scan(&breakdownType, &breakdownKey, &cost, &input, &output, &count); err != nil {
			return nil, fmt.Errorf("scan cost summary row: %w", err)
		}

		switch breakdownType {
		case "total":
			summary.TotalCostUSD = cost
			summary.TotalInput = input
			summary.TotalOutput = output
			summary.EntryCount = count
		case "project":
			if breakdownKey != "" {
				summary.ByProject[breakdownKey] = cost
			}
		case "phase":
			if breakdownKey != "" {
				summary.ByPhase[breakdownKey] = cost
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cost summary: %w", err)
	}

	return summary, nil
}

// DetectModel returns a simplified model name from a full model identifier.
// Maps model IDs to simplified names: opus, sonnet, haiku, or unknown.
func DetectModel(modelID string) string {
	lower := strings.ToLower(modelID)
	switch {
	case strings.Contains(lower, "opus"):
		return "opus"
	case strings.Contains(lower, "sonnet"):
		return "sonnet"
	case strings.Contains(lower, "haiku"):
		return "haiku"
	default:
		return "unknown"
	}
}

// RecordCostExtended logs a cost entry with all fields including model.
func (g *GlobalDB) RecordCostExtended(entry CostEntry) error {
	_, err := g.Exec(`
		INSERT INTO cost_log (
			project_id, task_id, phase, model, iteration,
			cost_usd, input_tokens, output_tokens,
			cache_creation_tokens, cache_read_tokens, total_tokens,
			initiative_id, duration_ms
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ProjectID, entry.TaskID, entry.Phase, entry.Model, entry.Iteration,
		entry.CostUSD, entry.InputTokens, entry.OutputTokens,
		entry.CacheCreationTokens, entry.CacheReadTokens, entry.TotalTokens,
		entry.InitiativeID, entry.DurationMs)
	if err != nil {
		return fmt.Errorf("record cost extended: %w", err)
	}
	return nil
}

// GetCostByModel returns costs grouped by model for a time range.
func (g *GlobalDB) GetCostByModel(projectID string, since time.Time) (map[string]float64, error) {
	result := make(map[string]float64)
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	// Build query using same pattern as GetCostTimeseries
	withProject := projectID != ""
	query := `
		SELECT COALESCE(model, '') as model, COALESCE(SUM(cost_usd), 0)
		FROM cost_log
		WHERE timestamp >= ?
	`
	if withProject {
		query += " AND project_id = ?"
	}
	query += " GROUP BY model"

	// Build args slice upfront based on project filter (consistent with GetCostTimeseries)
	var args []any
	if withProject {
		args = []any{sinceStr, projectID}
	} else {
		args = []any{sinceStr}
	}

	rows, err := g.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cost by model: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var model string
		var cost float64
		if err := rows.Scan(&model, &cost); err != nil {
			return nil, fmt.Errorf("scan model cost: %w", err)
		}
		result[model] = cost
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model costs: %w", err)
	}

	return result, nil
}

// strftimeFormat returns the SQLite strftime format for a given granularity.
func strftimeFormat(granularity string) string {
	switch granularity {
	case "week":
		return "%Y-W%W"
	case "month":
		return "%Y-%m"
	default: // "day" or default
		return "%Y-%m-%d"
	}
}

// buildTimeseriesQuery builds the SQL query for cost timeseries aggregation.
// The granularity parameter determines the date bucketing (day, week, month).
// If withProject is true, a project_id filter placeholder is added.
func buildTimeseriesQuery(granularity string, withProject bool) string {
	dateFormat := strftimeFormat(granularity)
	// Use the same dateFormat in both SELECT and GROUP BY for consistency
	query := fmt.Sprintf(`
		SELECT
			COALESCE(project_id, '') as project_id,
			COALESCE(model, '') as model,
			'' as phase,
			strftime('%s', timestamp) as date,
			COALESCE(SUM(cost_usd), 0) as total_cost_usd,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COUNT(*) as turn_count,
			COUNT(DISTINCT task_id) as task_count
		FROM cost_log
		WHERE timestamp >= ?`, dateFormat)
	if withProject {
		query += " AND project_id = ?"
	}
	query += fmt.Sprintf(" GROUP BY strftime('%s', timestamp), model ORDER BY date ASC", dateFormat)
	return query
}

// GetCostTimeseries returns cost data bucketed by time for charting.
// Granularity can be "day", "week", or "month".
//
// Note: CostAggregate.Phase is always empty in timeseries results since
// the data is aggregated across all phases. Use GetCostAggregates for
// phase-specific breakdowns.
func (g *GlobalDB) GetCostTimeseries(projectID string, since time.Time, granularity string) ([]CostAggregate, error) {
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	// Build query using template function based on project filter
	withProject := projectID != ""
	query := buildTimeseriesQuery(granularity, withProject)

	var args []any
	if withProject {
		args = []any{sinceStr, projectID}
	} else {
		args = []any{sinceStr}
	}

	rows, err := g.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cost timeseries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []CostAggregate
	for rows.Next() {
		var agg CostAggregate
		if err := rows.Scan(
			&agg.ProjectID, &agg.Model, &agg.Phase, &agg.Date,
			&agg.TotalCostUSD, &agg.TotalInputTokens, &agg.TotalOutputTokens,
			&agg.TotalCacheTokens, &agg.TurnCount, &agg.TaskCount,
		); err != nil {
			return nil, fmt.Errorf("scan timeseries row: %w", err)
		}
		results = append(results, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeseries: %w", err)
	}

	return results, nil
}

// UpdateCostAggregate upserts an aggregate record.
func (g *GlobalDB) UpdateCostAggregate(agg CostAggregate) error {
	_, err := g.Exec(`
		INSERT INTO cost_aggregates (
			project_id, model, phase, date,
			total_cost_usd, total_input_tokens, total_output_tokens,
			total_cache_tokens, turn_count, task_count
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, model, phase, date) DO UPDATE SET
			total_cost_usd = excluded.total_cost_usd,
			total_input_tokens = excluded.total_input_tokens,
			total_output_tokens = excluded.total_output_tokens,
			total_cache_tokens = excluded.total_cache_tokens,
			turn_count = excluded.turn_count,
			task_count = excluded.task_count
	`, agg.ProjectID, agg.Model, agg.Phase, agg.Date,
		agg.TotalCostUSD, agg.TotalInputTokens, agg.TotalOutputTokens,
		agg.TotalCacheTokens, agg.TurnCount, agg.TaskCount)
	if err != nil {
		return fmt.Errorf("update cost aggregate: %w", err)
	}
	return nil
}

// GetCostAggregates retrieves aggregated cost data for a date range.
func (g *GlobalDB) GetCostAggregates(projectID string, startDate, endDate string) ([]CostAggregate, error) {
	query := `
		SELECT id, project_id, model, phase, date,
			total_cost_usd, total_input_tokens, total_output_tokens,
			total_cache_tokens, turn_count, task_count, created_at
		FROM cost_aggregates
		WHERE date >= ? AND date <= ?
	`
	args := []any{startDate, endDate}

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY date ASC"

	rows, err := g.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cost aggregates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []CostAggregate
	for rows.Next() {
		var agg CostAggregate
		var createdAt string
		if err := rows.Scan(
			&agg.ID, &agg.ProjectID, &agg.Model, &agg.Phase, &agg.Date,
			&agg.TotalCostUSD, &agg.TotalInputTokens, &agg.TotalOutputTokens,
			&agg.TotalCacheTokens, &agg.TurnCount, &agg.TaskCount, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan aggregate row: %w", err)
		}
		agg.CreatedAt = parseTimestamp(createdAt)
		results = append(results, agg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregates: %w", err)
	}

	return results, nil
}

// GetBudget retrieves the budget for a project.
// Returns (nil, nil) when no budget exists for the project.
// Returns (nil, error) for database errors.
func (g *GlobalDB) GetBudget(projectID string) (*CostBudget, error) {
	row := g.QueryRow(`
		SELECT id, project_id, monthly_limit_usd, alert_threshold_percent,
			current_month, current_month_spent, created_at, updated_at
		FROM cost_budgets
		WHERE project_id = ?
	`, projectID)

	var b CostBudget
	var createdAt, updatedAt string
	var monthlyLimit sql.NullFloat64
	if err := row.Scan(
		&b.ID, &b.ProjectID, &monthlyLimit, &b.AlertThresholdPercent,
		&b.CurrentMonth, &b.CurrentMonthSpent, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No budget configured for this project
		}
		return nil, fmt.Errorf("get budget %s: %w", projectID, err)
	}
	if monthlyLimit.Valid {
		b.MonthlyLimitUSD = monthlyLimit.Float64
	}
	b.CreatedAt = parseTimestamp(createdAt)
	b.UpdatedAt = parseTimestamp(updatedAt)

	return &b, nil
}

// SetBudget creates or updates the budget for a project.
func (g *GlobalDB) SetBudget(budget CostBudget) error {
	_, err := g.Exec(`
		INSERT INTO cost_budgets (
			project_id, monthly_limit_usd, alert_threshold_percent,
			current_month, current_month_spent, updated_at
		)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(project_id) DO UPDATE SET
			monthly_limit_usd = excluded.monthly_limit_usd,
			alert_threshold_percent = excluded.alert_threshold_percent,
			current_month = excluded.current_month,
			current_month_spent = excluded.current_month_spent,
			updated_at = datetime('now')
	`, budget.ProjectID, budget.MonthlyLimitUSD, budget.AlertThresholdPercent,
		budget.CurrentMonth, budget.CurrentMonthSpent)
	if err != nil {
		return fmt.Errorf("set budget: %w", err)
	}
	return nil
}

// GetBudgetStatus returns the current spend vs limit for a project.
// Returns (nil, nil) when no budget is configured for the project.
// Returns (nil, error) for database errors.
//
// Note: This method may execute 2 queries when the stored budget month differs
// from the current month (to recalculate current spend). This is acceptable for
// single-budget lookups. For batch operations across multiple projects, consider
// querying cost_budgets directly and computing spend separately to avoid N+1 queries.
func (g *GlobalDB) GetBudgetStatus(projectID string) (*BudgetStatus, error) {
	budget, err := g.GetBudget(projectID)
	if err != nil {
		return nil, err
	}
	if budget == nil {
		return nil, nil // No budget configured for this project
	}

	currentMonth := time.Now().UTC().Format("2006-01")

	// If budget is for a different month, we need to calculate fresh
	var currentSpent float64
	if budget.CurrentMonth == currentMonth {
		currentSpent = budget.CurrentMonthSpent
	} else {
		// Calculate from cost_log for current month
		startOfMonth := currentMonth + "-01"
		row := g.QueryRow(`
			SELECT COALESCE(SUM(cost_usd), 0)
			FROM cost_log
			WHERE project_id = ? AND timestamp >= ?
		`, projectID, startOfMonth)
		if err := row.Scan(&currentSpent); err != nil {
			return nil, fmt.Errorf("calculate current month spend: %w", err)
		}
	}

	var percentUsed float64
	if budget.MonthlyLimitUSD > 0 {
		percentUsed = (currentSpent / budget.MonthlyLimitUSD) * 100
	}

	status := &BudgetStatus{
		ProjectID:         projectID,
		MonthlyLimitUSD:   budget.MonthlyLimitUSD,
		CurrentMonthSpent: currentSpent,
		CurrentMonth:      currentMonth,
		PercentUsed:       percentUsed,
		AlertThreshold:    budget.AlertThresholdPercent,
		OverBudget:        budget.MonthlyLimitUSD > 0 && currentSpent > budget.MonthlyLimitUSD,
		AtAlertThreshold:  percentUsed >= float64(budget.AlertThresholdPercent),
	}

	return status, nil
}
