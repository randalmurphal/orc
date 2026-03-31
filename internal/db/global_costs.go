package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// ErrBudgetNotFound is returned when no budget exists for a project.
var ErrBudgetNotFound = errors.New("budget not found")

// CostEntry represents a cost log entry.
type CostEntry struct {
	ID                  int64
	ProjectID           string
	TaskID              string
	Phase               string
	Model               string
	Provider            string
	Iteration           int
	CostUSD             float64
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	TotalTokens         int
	InitiativeID        string
	DurationMs          int64
	UserID              string
	Timestamp           time.Time
}

// CostAggregate represents aggregated cost data for time-series queries.
type CostAggregate struct {
	ID                int64
	ProjectID         string
	Model             string
	Phase             string
	Date              string
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
	CurrentMonth          string
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

// CostSummary provides aggregated cost data.
type CostSummary struct {
	TotalCostUSD       float64
	TotalInput         int
	TotalOutput        int
	TotalCacheRead     int
	TotalCacheCreation int
	EntryCount         int
	ByProject          map[string]float64
	ByPhase            map[string]float64
}

// GetCostSummary retrieves aggregated cost data since the given time.
func (g *GlobalDB) GetCostSummary(projectID string, since time.Time) (*CostSummary, error) {
	summary := &CostSummary{
		ByProject: make(map[string]float64),
		ByPhase:   make(map[string]float64),
	}

	sinceStr := since.UTC().Format("2006-01-02 15:04:05")
	var args []any

	query := `
		WITH filtered AS (
			SELECT project_id, phase, cost_usd, input_tokens, output_tokens,
				COALESCE(cache_read_tokens, 0) as cache_read_tokens,
				COALESCE(cache_creation_tokens, 0) as cache_creation_tokens
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
			COALESCE(SUM(output_tokens), 0), COUNT(*),
			COALESCE(SUM(cache_read_tokens), 0), COALESCE(SUM(cache_creation_tokens), 0)
		FROM filtered
		UNION ALL
		SELECT 'project' as breakdown_type, project_id as breakdown_key,
			COALESCE(SUM(cost_usd), 0), 0, 0, 0, 0, 0
		FROM filtered
		GROUP BY project_id
		UNION ALL
		SELECT 'phase' as breakdown_type, phase as breakdown_key,
			COALESCE(SUM(cost_usd), 0), 0, 0, 0, 0, 0
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
		var input, output, count, cacheRead, cacheCreation int
		if err := rows.Scan(&breakdownType, &breakdownKey, &cost, &input, &output, &count, &cacheRead, &cacheCreation); err != nil {
			return nil, fmt.Errorf("scan cost summary row: %w", err)
		}

		switch breakdownType {
		case "total":
			summary.TotalCostUSD = cost
			summary.TotalInput = input
			summary.TotalOutput = output
			summary.TotalCacheRead = cacheRead
			summary.TotalCacheCreation = cacheCreation
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
func DetectModel(provider, modelID string) string {
	if provider != "" && provider != "claude" {
		if modelID == "" {
			return "unknown"
		}
		return modelID
	}
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
			project_id, task_id, phase, model, provider, iteration,
			cost_usd, input_tokens, output_tokens,
			cache_creation_tokens, cache_read_tokens, total_tokens,
			initiative_id, duration_ms, user_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ProjectID, entry.TaskID, entry.Phase, entry.Model, entry.Provider, entry.Iteration,
		entry.CostUSD, entry.InputTokens, entry.OutputTokens,
		entry.CacheCreationTokens, entry.CacheReadTokens, entry.TotalTokens,
		entry.InitiativeID, entry.DurationMs, entry.UserID)
	if err != nil {
		return fmt.Errorf("record cost extended: %w", err)
	}
	return nil
}

// GetCostByModel returns costs grouped by model for a time range.
func (g *GlobalDB) GetCostByModel(projectID string, since time.Time) (map[string]float64, error) {
	result := make(map[string]float64)
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

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

// buildTimeseriesQuery builds the SQL query for cost timeseries aggregation.
func buildTimeseriesQuery(drv driver.Driver, granularity string, withProject bool) string {
	dateExpr := drv.DateFormat("timestamp", granularity)
	query := fmt.Sprintf(`
		SELECT
			COALESCE(project_id, '') as project_id,
			COALESCE(model, '') as model,
			'' as phase,
			%s as date,
			COALESCE(SUM(cost_usd), 0) as total_cost_usd,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(cache_creation_tokens + cache_read_tokens), 0) as total_cache_tokens,
			COUNT(*) as turn_count,
			COUNT(DISTINCT task_id) as task_count
		FROM cost_log
		WHERE timestamp >= ?`, dateExpr)
	if withProject {
		query += " AND project_id = ?"
	}
	query += fmt.Sprintf(" GROUP BY %s, model ORDER BY date ASC", dateExpr)
	return query
}

// GetCostTimeseries returns cost data bucketed by time for charting.
func (g *GlobalDB) GetCostTimeseries(projectID string, since time.Time, granularity string) ([]CostAggregate, error) {
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")
	withProject := projectID != ""
	query := buildTimeseriesQuery(g.Driver(), granularity, withProject)

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
			return nil, nil
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
	now := g.Driver().Now()
	_, err := g.Exec(fmt.Sprintf(`
		INSERT INTO cost_budgets (
			project_id, monthly_limit_usd, alert_threshold_percent,
			current_month, current_month_spent, updated_at
		)
		VALUES (?, ?, ?, ?, ?, %s)
		ON CONFLICT(project_id) DO UPDATE SET
			monthly_limit_usd = excluded.monthly_limit_usd,
			alert_threshold_percent = excluded.alert_threshold_percent,
			current_month = excluded.current_month,
			current_month_spent = excluded.current_month_spent,
			updated_at = %s
	`, now, now), budget.ProjectID, budget.MonthlyLimitUSD, budget.AlertThresholdPercent,
		budget.CurrentMonth, budget.CurrentMonthSpent)
	if err != nil {
		return fmt.Errorf("set budget: %w", err)
	}
	return nil
}

// GetBudgetStatus returns the current spend vs limit for a project.
func (g *GlobalDB) GetBudgetStatus(projectID string) (*BudgetStatus, error) {
	budget, err := g.GetBudget(projectID)
	if err != nil {
		return nil, err
	}
	if budget == nil {
		return nil, nil
	}

	currentMonth := time.Now().UTC().Format("2006-01")

	var currentSpent float64
	if budget.CurrentMonth == currentMonth {
		currentSpent = budget.CurrentMonthSpent
	} else {
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

// CostReportFilter defines the filters for cost report queries.
type CostReportFilter struct {
	UserID    string
	ProjectID string
	Since     time.Time
	GroupBy   string
}

// CostReportResult contains the aggregated cost report data.
type CostReportResult struct {
	TotalCostUSD float64
	Breakdowns   []CostBreakdownEntry
}

// CostBreakdownEntry represents a single breakdown entry in the cost report.
type CostBreakdownEntry struct {
	Key     string
	CostUSD float64
}

// GetCostReport returns aggregated cost data with optional filtering and grouping.
func (g *GlobalDB) GetCostReport(filter CostReportFilter) (CostReportResult, error) {
	var result CostReportResult

	conditions := []string{"1=1"}
	var args []any

	if filter.UserID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.Since.UTC().Format("2006-01-02 15:04:05"))
	}

	whereClause := strings.Join(conditions, " AND ")
	totalQuery := fmt.Sprintf("SELECT COALESCE(SUM(cost_usd), 0) FROM cost_log WHERE %s", whereClause)
	if err := g.QueryRow(totalQuery, args...).Scan(&result.TotalCostUSD); err != nil {
		return result, fmt.Errorf("get cost report total: %w", err)
	}

	if filter.GroupBy != "" {
		var groupCol string
		switch filter.GroupBy {
		case "user":
			groupCol = "CASE WHEN user_id = '' OR user_id IS NULL THEN 'unattributed' ELSE user_id END"
		case "project":
			groupCol = "project_id"
		case "model":
			groupCol = "CASE WHEN model = '' OR model IS NULL THEN 'unknown' ELSE model END"
		case "provider":
			groupCol = "CASE WHEN provider = '' OR provider IS NULL THEN 'claude' ELSE provider END"
		default:
			return result, fmt.Errorf("invalid group_by value: %s", filter.GroupBy)
		}

		groupQuery := fmt.Sprintf(`
			SELECT %s as group_key, COALESCE(SUM(cost_usd), 0)
			FROM cost_log WHERE %s
			GROUP BY 1
			ORDER BY 2 DESC`,
			groupCol, whereClause,
		)

		rows, err := g.Query(groupQuery, args...)
		if err != nil {
			return result, fmt.Errorf("get cost report breakdowns: %w", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var entry CostBreakdownEntry
			if err := rows.Scan(&entry.Key, &entry.CostUSD); err != nil {
				return result, fmt.Errorf("scan cost breakdown: %w", err)
			}
			result.Breakdowns = append(result.Breakdowns, entry)
		}
		if err := rows.Err(); err != nil {
			return result, fmt.Errorf("iterate cost breakdowns: %w", err)
		}
	}

	return result, nil
}
