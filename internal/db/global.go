package db

import (
	"database/sql"
	"encoding/json"
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
	return OpenGlobalAt(path)
}

// OpenGlobalAt opens the global database at a specific path using SQLite.
// This is useful for testing with isolated databases.
func OpenGlobalAt(path string) (*GlobalDB, error) {
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

// =============================================================================
// Workflow Operations (Global - shared across projects)
// =============================================================================

// SavePhaseTemplate creates or updates a phase template in global DB.
func (g *GlobalDB) SavePhaseTemplate(pt *PhaseTemplate) error {
	thinkingEnabled := sqlNullBool(pt.ThinkingEnabled)
	agentID := sqlNullString(pt.AgentID)
	subAgents := sqlNullString(pt.SubAgents)
	gateAgentID := sqlNullString(pt.GateAgentID)

	_, err := g.Exec(`
		INSERT INTO phase_templates (id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, system_prompt, claude_config,
			is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			agent_id = excluded.agent_id,
			sub_agents = excluded.sub_agents,
			prompt_source = excluded.prompt_source,
			prompt_content = excluded.prompt_content,
			prompt_path = excluded.prompt_path,
			input_variables = excluded.input_variables,
			output_schema = excluded.output_schema,
			produces_artifact = excluded.produces_artifact,
			artifact_type = excluded.artifact_type,
			output_var_name = excluded.output_var_name,
			output_type = excluded.output_type,
			quality_checks = excluded.quality_checks,
			max_iterations = excluded.max_iterations,
			thinking_enabled = excluded.thinking_enabled,
			gate_type = excluded.gate_type,
			checkpoint = excluded.checkpoint,
			retry_from_phase = excluded.retry_from_phase,
			retry_prompt_path = excluded.retry_prompt_path,
			system_prompt = excluded.system_prompt,
			claude_config = excluded.claude_config,
			gate_input_config = excluded.gate_input_config,
			gate_output_config = excluded.gate_output_config,
			gate_mode = excluded.gate_mode,
			gate_agent_id = excluded.gate_agent_id,
			updated_at = excluded.updated_at
	`, pt.ID, pt.Name, pt.Description, agentID, subAgents,
		pt.PromptSource, pt.PromptContent, pt.PromptPath,
		pt.InputVariables, pt.OutputSchema, pt.ProducesArtifact, pt.ArtifactType, pt.OutputVarName,
		pt.OutputType, pt.QualityChecks,
		pt.MaxIterations, thinkingEnabled, pt.GateType, pt.Checkpoint,
		pt.RetryFromPhase, pt.RetryPromptPath, "", "", // system_prompt, claude_config empty for now
		pt.IsBuiltin, pt.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339),
		pt.GateInputConfig, pt.GateOutputConfig, pt.GateMode, gateAgentID)
	if err != nil {
		return fmt.Errorf("save phase template: %w", err)
	}
	return nil
}

// GetPhaseTemplate retrieves a phase template by ID from global DB.
func (g *GlobalDB) GetPhaseTemplate(id string) (*PhaseTemplate, error) {
	row := g.QueryRow(`
		SELECT id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id
		FROM phase_templates WHERE id = ?
	`, id)

	pt, err := scanPhaseTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get phase template %s: %w", id, err)
	}
	return pt, nil
}

// ListPhaseTemplates returns all phase templates from global DB.
func (g *GlobalDB) ListPhaseTemplates() ([]*PhaseTemplate, error) {
	rows, err := g.Query(`
		SELECT id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id
		FROM phase_templates
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list phase templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var templates []*PhaseTemplate
	for rows.Next() {
		pt, err := scanPhaseTemplateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan phase template: %w", err)
		}
		templates = append(templates, pt)
	}
	return templates, rows.Err()
}

// DeletePhaseTemplate removes a non-builtin phase template from global DB.
func (g *GlobalDB) DeletePhaseTemplate(id string) error {
	_, err := g.Exec("DELETE FROM phase_templates WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete phase template: %w", err)
	}
	return nil
}

// SaveWorkflow creates or updates a workflow in global DB.
func (g *GlobalDB) SaveWorkflow(w *Workflow) error {
	basedOn := sqlNullString(w.BasedOn)

	_, err := g.Exec(`
		INSERT INTO workflows (id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			workflow_type = excluded.workflow_type,
			default_model = excluded.default_model,
			default_thinking = excluded.default_thinking,
			based_on = excluded.based_on,
			triggers = excluded.triggers,
			updated_at = excluded.updated_at
	`, w.ID, w.Name, w.Description, w.WorkflowType, w.DefaultModel, w.DefaultThinking,
		w.IsBuiltin, basedOn, w.Triggers, w.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}
	return nil
}

// GetWorkflow retrieves a workflow by ID from global DB, including its phases.
func (g *GlobalDB) GetWorkflow(id string) (*Workflow, error) {
	row := g.QueryRow(`
		SELECT id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at
		FROM workflows WHERE id = ?
	`, id)

	w, err := scanWorkflow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow %s: %w", id, err)
	}

	// Load phases
	phases, err := g.GetWorkflowPhases(id)
	if err != nil {
		return nil, fmt.Errorf("get workflow %s phases: %w", id, err)
	}
	w.Phases = phases

	return w, nil
}

// ListWorkflows returns all workflows from global DB.
func (g *GlobalDB) ListWorkflows() ([]*Workflow, error) {
	rows, err := g.Query(`
		SELECT id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at
		FROM workflows
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workflows []*Workflow
	for rows.Next() {
		w, err := scanWorkflowRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

// DeleteWorkflow removes a non-builtin workflow from global DB.
func (g *GlobalDB) DeleteWorkflow(id string) error {
	_, err := g.Exec("DELETE FROM workflows WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	return nil
}

// SaveWorkflowPhase creates or updates a workflow-phase link in global DB.
func (g *GlobalDB) SaveWorkflowPhase(wp *WorkflowPhase) error {
	thinkingOverride := sqlNullBool(wp.ThinkingOverride)
	maxIterOverride := sqlNullInt(wp.MaxIterationsOverride)
	posX := sqlNullFloat64(wp.PositionX)
	posY := sqlNullFloat64(wp.PositionY)
	agentOverride := sqlNullString(wp.AgentOverride)
	subAgentsOverride := sqlNullString(wp.SubAgentsOverride)

	res, err := g.Exec(`
		INSERT INTO workflow_phases (workflow_id, phase_template_id, sequence, depends_on,
			agent_override, sub_agents_override,
			max_iterations_override, model_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, claude_config_override, before_triggers, position_x, position_y)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, phase_template_id) DO UPDATE SET
			sequence = excluded.sequence,
			depends_on = excluded.depends_on,
			agent_override = excluded.agent_override,
			sub_agents_override = excluded.sub_agents_override,
			max_iterations_override = excluded.max_iterations_override,
			model_override = excluded.model_override,
			thinking_override = excluded.thinking_override,
			gate_type_override = excluded.gate_type_override,
			condition = excluded.condition,
			quality_checks_override = excluded.quality_checks_override,
			loop_config = excluded.loop_config,
			claude_config_override = excluded.claude_config_override,
			before_triggers = excluded.before_triggers,
			position_x = excluded.position_x,
			position_y = excluded.position_y
	`, wp.WorkflowID, wp.PhaseTemplateID, wp.Sequence, wp.DependsOn,
		agentOverride, subAgentsOverride,
		maxIterOverride, wp.ModelOverride, thinkingOverride, wp.GateTypeOverride, wp.Condition,
		wp.QualityChecksOverride, wp.LoopConfig, wp.ClaudeConfigOverride, wp.BeforeTriggers, posX, posY)
	if err != nil {
		return fmt.Errorf("save workflow phase: %w", err)
	}

	if wp.ID == 0 {
		id, _ := res.LastInsertId()
		wp.ID = int(id)
	}
	return nil
}

// AddWorkflowPhase adds a new phase to a workflow in global DB.
// This is an alias for SaveWorkflowPhase that better expresses the intent
// of adding a new phase rather than updating an existing one.
func (g *GlobalDB) AddWorkflowPhase(wp *WorkflowPhase) error {
	return g.SaveWorkflowPhase(wp)
}

// GetWorkflowPhases returns all phases for a workflow in sequence order from global DB.
func (g *GlobalDB) GetWorkflowPhases(workflowID string) ([]*WorkflowPhase, error) {
	rows, err := g.Query(`
		SELECT id, workflow_id, phase_template_id, sequence, depends_on,
			agent_override, sub_agents_override,
			max_iterations_override, model_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, claude_config_override, before_triggers, position_x, position_y
		FROM workflow_phases
		WHERE workflow_id = ?
		ORDER BY sequence ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow phases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phases []*WorkflowPhase
	for rows.Next() {
		wp, err := scanWorkflowPhaseRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow phase: %w", err)
		}
		phases = append(phases, wp)
	}
	return phases, rows.Err()
}

// DeleteWorkflowPhase removes a phase from a workflow in global DB.
func (g *GlobalDB) DeleteWorkflowPhase(workflowID, phaseTemplateID string) error {
	_, err := g.Exec("DELETE FROM workflow_phases WHERE workflow_id = ? AND phase_template_id = ?",
		workflowID, phaseTemplateID)
	if err != nil {
		return fmt.Errorf("delete workflow phase: %w", err)
	}
	return nil
}

// SaveWorkflowVariable creates or updates a workflow variable in global DB.
func (g *GlobalDB) SaveWorkflowVariable(wv *WorkflowVariable) error {
	res, err := g.Exec(`
		INSERT INTO workflow_variables (workflow_id, name, description, source_type, source_config, required, default_value, cache_ttl_seconds, script_content, extract)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, name) DO UPDATE SET
			description = excluded.description,
			source_type = excluded.source_type,
			source_config = excluded.source_config,
			required = excluded.required,
			default_value = excluded.default_value,
			cache_ttl_seconds = excluded.cache_ttl_seconds,
			script_content = excluded.script_content,
			extract = excluded.extract
	`, wv.WorkflowID, wv.Name, wv.Description, wv.SourceType, wv.SourceConfig,
		wv.Required, wv.DefaultValue, wv.CacheTTLSeconds, wv.ScriptContent, wv.Extract)
	if err != nil {
		return fmt.Errorf("save workflow variable: %w", err)
	}

	if wv.ID == 0 {
		id, _ := res.LastInsertId()
		wv.ID = int(id)
	}
	return nil
}

// GetWorkflowVariables returns all variables for a workflow from global DB.
func (g *GlobalDB) GetWorkflowVariables(workflowID string) ([]*WorkflowVariable, error) {
	rows, err := g.Query(`
		SELECT id, workflow_id, name, description, source_type, source_config, required, default_value, cache_ttl_seconds, script_content, extract
		FROM workflow_variables
		WHERE workflow_id = ?
		ORDER BY name ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow variables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var vars []*WorkflowVariable
	for rows.Next() {
		wv, err := scanWorkflowVariableRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow variable: %w", err)
		}
		vars = append(vars, wv)
	}
	return vars, rows.Err()
}

// DeleteWorkflowVariable removes a variable from a workflow in global DB.
func (g *GlobalDB) DeleteWorkflowVariable(workflowID, name string) error {
	_, err := g.Exec("DELETE FROM workflow_variables WHERE workflow_id = ? AND name = ?",
		workflowID, name)
	if err != nil {
		return fmt.Errorf("delete workflow variable: %w", err)
	}
	return nil
}

// =============================================================================
// Agent Operations (Global - shared across projects)
// =============================================================================

// SaveAgent saves or updates an agent definition in global DB.
func (g *GlobalDB) SaveAgent(a *Agent) error {
	toolsJSON, err := json.Marshal(a.Tools)
	if err != nil {
		return fmt.Errorf("marshal agent tools: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt == "" {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	_, err = g.Exec(`
		INSERT INTO agents (id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			prompt = excluded.prompt,
			tools = excluded.tools,
			model = excluded.model,
			system_prompt = excluded.system_prompt,
			claude_config = excluded.claude_config,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`, a.ID, a.Name, a.Description, a.Prompt, string(toolsJSON),
		a.Model, a.SystemPrompt, a.ClaudeConfig, a.IsBuiltin, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save agent %s: %w", a.ID, err)
	}

	return nil
}

// GetAgent retrieves an agent by ID from global DB.
func (g *GlobalDB) GetAgent(id string) (*Agent, error) {
	var a Agent
	var toolsJSON string
	var model, systemPrompt, claudeConfig sql.NullString

	err := g.QueryRow(`
		SELECT id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at
		FROM agents WHERE id = ?
	`, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
		&model, &systemPrompt, &claudeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", id, err)
	}

	if model.Valid {
		a.Model = model.String
	}
	if systemPrompt.Valid {
		a.SystemPrompt = systemPrompt.String
	}
	if claudeConfig.Valid {
		a.ClaudeConfig = claudeConfig.String
	}

	if toolsJSON != "" {
		if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal agent tools: %w", err)
		}
	}

	return &a, nil
}

// ListAgents returns all agents from global DB.
func (g *GlobalDB) ListAgents() ([]*Agent, error) {
	rows, err := g.Query(`
		SELECT id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at
		FROM agents
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var agents []*Agent
	for rows.Next() {
		var a Agent
		var toolsJSON string
		var model, systemPrompt, claudeConfig sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
			&model, &systemPrompt, &claudeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}

		if model.Valid {
			a.Model = model.String
		}
		if systemPrompt.Valid {
			a.SystemPrompt = systemPrompt.String
		}
		if claudeConfig.Valid {
			a.ClaudeConfig = claudeConfig.String
		}

		if toolsJSON != "" {
			if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal agent tools: %w", err)
			}
		}

		agents = append(agents, &a)
	}

	return agents, nil
}

// DeleteAgent deletes a non-builtin agent by ID from global DB.
func (g *GlobalDB) DeleteAgent(id string) error {
	agent, err := g.GetAgent(id)
	if err != nil {
		return err
	}
	if agent == nil {
		return nil
	}
	if agent.IsBuiltin {
		return fmt.Errorf("cannot delete builtin agent %s", id)
	}

	_, err = g.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}

	return nil
}

// CountAgents returns the number of agents in global DB.
func (g *GlobalDB) CountAgents() (int, error) {
	var count int
	err := g.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count agents: %w", err)
	}
	return count, nil
}

// SavePhaseAgent creates or updates a phase-agent association in global DB.
func (g *GlobalDB) SavePhaseAgent(pa *PhaseAgent) error {
	var weightFilterJSON string
	if len(pa.WeightFilter) > 0 {
		b, err := json.Marshal(pa.WeightFilter)
		if err != nil {
			return fmt.Errorf("marshal weight filter: %w", err)
		}
		weightFilterJSON = string(b)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if pa.CreatedAt == "" {
		pa.CreatedAt = now
	}
	pa.UpdatedAt = now

	query := `
		INSERT INTO phase_agents (phase_template_id, agent_id, sequence, role, weight_filter, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(phase_template_id, agent_id) DO UPDATE SET
			sequence = excluded.sequence,
			role = excluded.role,
			weight_filter = excluded.weight_filter,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`

	res, err := g.Exec(query,
		pa.PhaseTemplateID, pa.AgentID, pa.Sequence, pa.Role,
		weightFilterJSON, pa.IsBuiltin, pa.CreatedAt, pa.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save phase agent %s/%s: %w", pa.PhaseTemplateID, pa.AgentID, err)
	}

	if pa.ID == 0 {
		id, _ := res.LastInsertId()
		pa.ID = id
	}
	return nil
}

// GetPhaseAgents returns all agent associations for a phase template from global DB.
func (g *GlobalDB) GetPhaseAgents(phaseTemplateID string) ([]*PhaseAgent, error) {
	query := `
		SELECT id, phase_template_id, agent_id, sequence, role, weight_filter, is_builtin, created_at, updated_at
		FROM phase_agents
		WHERE phase_template_id = ?
		ORDER BY sequence ASC, agent_id ASC
	`

	rows, err := g.Query(query, phaseTemplateID)
	if err != nil {
		return nil, fmt.Errorf("get phase agents for %s: %w", phaseTemplateID, err)
	}
	defer func() { _ = rows.Close() }()

	var agents []*PhaseAgent
	for rows.Next() {
		var pa PhaseAgent
		var role sql.NullString
		var weightFilterJSON sql.NullString

		if err := rows.Scan(
			&pa.ID, &pa.PhaseTemplateID, &pa.AgentID, &pa.Sequence,
			&role, &weightFilterJSON, &pa.IsBuiltin, &pa.CreatedAt, &pa.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan phase agent: %w", err)
		}

		if role.Valid {
			pa.Role = role.String
		}

		if weightFilterJSON.Valid && weightFilterJSON.String != "" {
			if err := json.Unmarshal([]byte(weightFilterJSON.String), &pa.WeightFilter); err != nil {
				return nil, fmt.Errorf("unmarshal weight filter: %w", err)
			}
		}

		agents = append(agents, &pa)
	}

	return agents, rows.Err()
}

// GetPhaseAgentsForWeight returns agent associations for a phase template,
// filtered to only agents that apply to the given task weight from global DB.
func (g *GlobalDB) GetPhaseAgentsForWeight(phaseTemplateID, weight string) ([]*PhaseAgent, error) {
	agents, err := g.GetPhaseAgents(phaseTemplateID)
	if err != nil {
		return nil, err
	}

	// Filter by weight
	var filtered []*PhaseAgent
	for _, pa := range agents {
		// No weight filter = applies to all weights
		if len(pa.WeightFilter) == 0 {
			filtered = append(filtered, pa)
			continue
		}

		// Check if weight is in filter
		for _, w := range pa.WeightFilter {
			if w == weight {
				filtered = append(filtered, pa)
				break
			}
		}
	}

	return filtered, nil
}

// GetPhaseAgentsWithDefinitions returns agent associations with full definitions from global DB.
func (g *GlobalDB) GetPhaseAgentsWithDefinitions(phaseTemplateID, weight string) ([]*AgentWithAssignment, error) {
	phaseAgents, err := g.GetPhaseAgentsForWeight(phaseTemplateID, weight)
	if err != nil {
		return nil, err
	}

	if len(phaseAgents) == 0 {
		return nil, nil
	}

	var result []*AgentWithAssignment
	for _, pa := range phaseAgents {
		agent, err := g.GetAgent(pa.AgentID)
		if err != nil {
			return nil, fmt.Errorf("get agent %s: %w", pa.AgentID, err)
		}
		if agent == nil {
			// Agent doesn't exist - skip
			continue
		}

		result = append(result, &AgentWithAssignment{
			Agent:      agent,
			PhaseAgent: pa,
		})
	}

	return result, nil
}
