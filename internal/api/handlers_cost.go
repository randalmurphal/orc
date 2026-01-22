package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// ============================================================================
// Cost Analytics Endpoint Types
// ============================================================================

// CostBreakdownResponse for GET /api/cost/breakdown
type CostBreakdownResponse struct {
	Period       string                   `json:"period"`        // "24h", "7d", "30d", "all"
	TotalCostUSD float64                  `json:"total_cost_usd"`
	Breakdown    map[string]CostBreakdown `json:"breakdown"` // keyed by model/phase/task/initiative
}

// CostBreakdown represents cost data for a single dimension value
type CostBreakdown struct {
	CostUSD float64 `json:"cost"`
	Tokens  int     `json:"tokens"`
	Percent float64 `json:"percent"`
}

// CostTimeseriesResponse for GET /api/cost/timeseries
type CostTimeseriesResponse struct {
	Start       string                `json:"start"`
	End         string                `json:"end"`
	Granularity string                `json:"granularity"` // "hour", "day", "week"
	Series      []CostTimeseriesPoint `json:"series"`
}

// CostTimeseriesPoint represents a single point in time with cost data
type CostTimeseriesPoint struct {
	Date    string  `json:"date"`              // YYYY-MM-DD or YYYY-MM-DD HH:00
	CostUSD float64 `json:"cost"`
	Tokens  int     `json:"tokens"`
	Model   string  `json:"model,omitempty"` // when filtered or grouped
}

// BudgetStatusResponse for GET /api/cost/budget
type BudgetStatusResponse struct {
	MonthlyLimitUSD  float64 `json:"monthly_limit_usd"`
	CurrentSpentUSD  float64 `json:"current_spent_usd"`
	RemainingUSD     float64 `json:"remaining_usd"`
	PercentUsed      float64 `json:"percent_used"`
	ProjectedMonthly float64 `json:"projected_monthly"` // extrapolated from days elapsed
	DaysRemaining    int     `json:"days_remaining"`
	OnTrack          bool    `json:"on_track"` // projected <= limit
}

// BudgetUpdateRequest for PUT /api/cost/budget
type BudgetUpdateRequest struct {
	MonthlyLimitUSD       float64 `json:"monthly_limit_usd"`
	AlertThresholdPercent int     `json:"alert_threshold_percent,omitempty"` // default 80
}

// InitiativeCostResponse for GET /api/initiatives/:id/cost
type InitiativeCostResponse struct {
	InitiativeID string             `json:"initiative_id"`
	TotalCostUSD float64            `json:"total_cost_usd"`
	ByTask       []TaskCost         `json:"by_task"`
	ByModel      map[string]float64 `json:"by_model"`
	ByPhase      map[string]float64 `json:"by_phase"`
}

// TaskCost represents cost data for a single task
type TaskCost struct {
	TaskID  string  `json:"task_id"`
	CostUSD float64 `json:"cost"`
	Tokens  int     `json:"tokens"`
}

// ============================================================================
// Cost Breakdown Handler
// ============================================================================

// handleCostBreakdown returns cost data grouped by the specified dimension.
// GET /api/cost/breakdown?by=model&period=30d
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
	if period == "" {
		period = "all"
	}

	start := parsePeriod(period)
	projectID := s.getProjectID()

	var breakdown map[string]CostBreakdown
	var totalCostUSD float64
	var err error

	switch by {
	case "model":
		breakdown, totalCostUSD, err = s.getCostByModel(projectID, start)
	case "phase":
		breakdown, totalCostUSD, err = s.getCostByPhase(projectID, start)
	case "task":
		breakdown, totalCostUSD, err = s.getCostByTask(projectID, start)
	case "initiative":
		breakdown, totalCostUSD, err = s.getCostByInitiative(projectID, start)
	default:
		s.jsonError(w, "invalid 'by' parameter: use model, phase, task, or initiative", http.StatusBadRequest)
		return
	}

	if err != nil {
		s.jsonError(w, "failed to load cost breakdown: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate percentages
	for key, data := range breakdown {
		if totalCostUSD > 0 {
			data.Percent = (data.CostUSD / totalCostUSD) * 100
			breakdown[key] = data
		}
	}

	response := CostBreakdownResponse{
		Period:       period,
		TotalCostUSD: totalCostUSD,
		Breakdown:    breakdown,
	}

	s.jsonResponse(w, response)
}

// getCostByModel returns costs grouped by model
func (s *Server) getCostByModel(projectID string, since time.Time) (map[string]CostBreakdown, float64, error) {
	costMap, err := s.globalDB.GetCostByModel(projectID, since)
	if err != nil {
		return nil, 0, err
	}

	breakdown := make(map[string]CostBreakdown)
	var total float64

	for model, cost := range costMap {
		if model == "" {
			model = "unknown"
		}
		breakdown[model] = CostBreakdown{
			CostUSD: cost,
			// Tokens would require separate query, omitted for now
		}
		total += cost
	}

	return breakdown, total, nil
}

// getCostByPhase returns costs grouped by phase
func (s *Server) getCostByPhase(projectID string, since time.Time) (map[string]CostBreakdown, float64, error) {
	summary, err := s.globalDB.GetCostSummary(projectID, since)
	if err != nil {
		return nil, 0, err
	}

	breakdown := make(map[string]CostBreakdown)
	for phase, cost := range summary.ByPhase {
		breakdown[phase] = CostBreakdown{
			CostUSD: cost,
		}
	}

	return breakdown, summary.TotalCostUSD, nil
}

// getCostByTask returns costs grouped by task
func (s *Server) getCostByTask(projectID string, since time.Time) (map[string]CostBreakdown, float64, error) {
	// Query cost_log directly and group by task_id
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	query := `
		SELECT task_id,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(SUM(total_tokens), 0) as total_tokens
		FROM cost_log
		WHERE timestamp >= ?
	`
	args := []any{sinceStr}
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	query += " GROUP BY task_id"

	rows, err := s.globalDB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	breakdown := make(map[string]CostBreakdown)
	var total float64

	for rows.Next() {
		var taskID string
		var cost float64
		var tokens int
		if err := rows.Scan(&taskID, &cost, &tokens); err != nil {
			return nil, 0, err
		}
		breakdown[taskID] = CostBreakdown{
			CostUSD: cost,
			Tokens:  tokens,
		}
		total += cost
	}

	return breakdown, total, rows.Err()
}

// getCostByInitiative returns costs grouped by initiative
func (s *Server) getCostByInitiative(projectID string, since time.Time) (map[string]CostBreakdown, float64, error) {
	// Query cost_log directly and group by initiative_id
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")

	query := `
		SELECT COALESCE(initiative_id, '') as initiative_id,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(SUM(total_tokens), 0) as total_tokens
		FROM cost_log
		WHERE timestamp >= ? AND initiative_id != ''
	`
	args := []any{sinceStr}
	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	query += " GROUP BY initiative_id"

	rows, err := s.globalDB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	breakdown := make(map[string]CostBreakdown)
	var total float64

	for rows.Next() {
		var initiativeID string
		var cost float64
		var tokens int
		if err := rows.Scan(&initiativeID, &cost, &tokens); err != nil {
			return nil, 0, err
		}
		if initiativeID == "" {
			continue
		}
		breakdown[initiativeID] = CostBreakdown{
			CostUSD: cost,
			Tokens:  tokens,
		}
		total += cost
	}

	return breakdown, total, rows.Err()
}

// ============================================================================
// Cost Timeseries Handler
// ============================================================================

// handleCostTimeseries returns time-bucketed cost data for charting.
// GET /api/cost/timeseries?granularity=day&start=2024-01-01&end=2024-01-31&model=sonnet
func (s *Server) handleCostTimeseries(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		s.jsonError(w, "cost analytics not available", http.StatusServiceUnavailable)
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}
	if granularity != "day" && granularity != "week" && granularity != "month" {
		s.jsonError(w, "invalid granularity: use day, week, or month", http.StatusBadRequest)
		return
	}

	// Parse time range
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	modelFilter := r.URL.Query().Get("model")

	var start, end time.Time
	var err error

	if startStr == "" {
		// Default to last 30 days
		end = time.Now().UTC()
		start = end.AddDate(0, 0, -30)
	} else {
		start, err = parseDate(startStr)
		if err != nil {
			s.jsonError(w, "invalid start date: use YYYY-MM-DD or RFC3339", http.StatusBadRequest)
			return
		}
		if endStr == "" {
			end = time.Now().UTC()
		} else {
			end, err = parseDate(endStr)
			if err != nil {
				s.jsonError(w, "invalid end date: use YYYY-MM-DD or RFC3339", http.StatusBadRequest)
				return
			}
		}
	}

	projectID := s.getProjectID()

	// Get timeseries data from GlobalDB
	aggregates, err := s.globalDB.GetCostTimeseries(projectID, start, granularity)
	if err != nil {
		s.jsonError(w, "failed to load timeseries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter by model if specified
	var series []CostTimeseriesPoint
	for _, agg := range aggregates {
		if modelFilter != "" && agg.Model != modelFilter {
			continue
		}

		point := CostTimeseriesPoint{
			Date:    agg.Date,
			CostUSD: agg.TotalCostUSD,
			Tokens:  agg.TotalInputTokens + agg.TotalOutputTokens,
		}
		if modelFilter != "" {
			point.Model = agg.Model
		}
		series = append(series, point)
	}

	// Fill gaps with zero values
	series = fillTimeseriesGaps(series, start, end, granularity)

	response := CostTimeseriesResponse{
		Start:       start.Format("2006-01-02"),
		End:         end.Format("2006-01-02"),
		Granularity: granularity,
		Series:      series,
	}

	s.jsonResponse(w, response)
}

// fillTimeseriesGaps ensures every date bucket in the range has a data point
func fillTimeseriesGaps(series []CostTimeseriesPoint, start, end time.Time, granularity string) []CostTimeseriesPoint {
	if len(series) == 0 {
		return series
	}

	// Build a map of existing dates
	dataMap := make(map[string]CostTimeseriesPoint)
	for _, point := range series {
		dataMap[point.Date] = point
	}

	// Generate all dates in range
	var filled []CostTimeseriesPoint
	current := start
	for current.Before(end) || current.Equal(end) {
		dateStr := formatDateForGranularity(current, granularity)

		if point, exists := dataMap[dateStr]; exists {
			filled = append(filled, point)
		} else {
			filled = append(filled, CostTimeseriesPoint{
				Date:    dateStr,
				CostUSD: 0,
				Tokens:  0,
			})
		}

		// Advance to next bucket
		switch granularity {
		case "day":
			current = current.AddDate(0, 0, 1)
		case "week":
			current = current.AddDate(0, 0, 7)
		case "month":
			current = current.AddDate(0, 1, 0)
		}
	}

	return filled
}

// formatDateForGranularity returns the date string in the appropriate format
func formatDateForGranularity(t time.Time, granularity string) string {
	switch granularity {
	case "week":
		// SQLite uses %Y-W%W format
		year, week := t.ISOWeek()
		return time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC).AddDate(0, 0, (week-1)*7).Format("2006-W02")
	case "month":
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// ============================================================================
// Budget Handler
// ============================================================================

// handleCostBudget returns current budget status.
// GET /api/cost/budget
func (s *Server) handleCostBudget(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		s.jsonError(w, "cost analytics not available", http.StatusServiceUnavailable)
		return
	}

	projectID := s.getProjectID()

	status, err := s.globalDB.GetBudgetStatus(projectID)
	if err != nil {
		s.jsonError(w, "failed to load budget status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If no budget configured, return zeroed response
	if status == nil {
		s.jsonResponse(w, BudgetStatusResponse{
			MonthlyLimitUSD:  0,
			CurrentSpentUSD:  0,
			RemainingUSD:     0,
			PercentUsed:      0,
			ProjectedMonthly: 0,
			DaysRemaining:    0,
			OnTrack:          true,
		})
		return
	}

	// Calculate projected monthly spend based on days elapsed
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	daysElapsed := int(now.Sub(startOfMonth).Hours() / 24)
	if daysElapsed == 0 {
		daysElapsed = 1 // avoid division by zero
	}

	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	projectedMonthly := (status.CurrentMonthSpent / float64(daysElapsed)) * float64(daysInMonth)
	daysRemaining := daysInMonth - daysElapsed

	response := BudgetStatusResponse{
		MonthlyLimitUSD:  status.MonthlyLimitUSD,
		CurrentSpentUSD:  status.CurrentMonthSpent,
		RemainingUSD:     status.MonthlyLimitUSD - status.CurrentMonthSpent,
		PercentUsed:      status.PercentUsed,
		ProjectedMonthly: projectedMonthly,
		DaysRemaining:    daysRemaining,
		OnTrack:          projectedMonthly <= status.MonthlyLimitUSD,
	}

	s.jsonResponse(w, response)
}

// handleUpdateCostBudget updates budget settings.
// PUT /api/cost/budget
func (s *Server) handleUpdateCostBudget(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		s.jsonError(w, "cost analytics not available", http.StatusServiceUnavailable)
		return
	}

	var req BudgetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate
	if req.MonthlyLimitUSD < 0 {
		s.jsonError(w, "monthly_limit_usd must be non-negative", http.StatusBadRequest)
		return
	}

	if req.AlertThresholdPercent == 0 {
		req.AlertThresholdPercent = 80 // default
	}
	if req.AlertThresholdPercent < 0 || req.AlertThresholdPercent > 100 {
		s.jsonError(w, "alert_threshold_percent must be between 0 and 100", http.StatusBadRequest)
		return
	}

	projectID := s.getProjectID()
	currentMonth := time.Now().UTC().Format("2006-01")

	// Get existing budget to preserve current_month_spent if same month
	existing, err := s.globalDB.GetBudget(projectID)
	if err != nil {
		s.jsonError(w, "failed to load existing budget: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var currentSpent float64
	if existing != nil && existing.CurrentMonth == currentMonth {
		currentSpent = existing.CurrentMonthSpent
	}

	budget := db.CostBudget{
		ProjectID:             projectID,
		MonthlyLimitUSD:       req.MonthlyLimitUSD,
		AlertThresholdPercent: req.AlertThresholdPercent,
		CurrentMonth:          currentMonth,
		CurrentMonthSpent:     currentSpent,
	}

	if err := s.globalDB.SetBudget(budget); err != nil {
		s.jsonError(w, "failed to save budget: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "ok"})
}

// ============================================================================
// Initiative Cost Handler
// ============================================================================

// handleInitiativeCost returns cost rollup for a specific initiative.
// GET /api/initiatives/:id/cost
func (s *Server) handleInitiativeCost(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		s.jsonError(w, "cost analytics not available", http.StatusServiceUnavailable)
		return
	}

	initiativeID := strings.TrimPrefix(r.URL.Path, "/api/initiatives/")
	initiativeID = strings.TrimSuffix(initiativeID, "/cost")

	if initiativeID == "" {
		s.jsonError(w, "initiative ID required", http.StatusBadRequest)
		return
	}

	// Query cost_log for this initiative
	query := `
		SELECT
			task_id,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(SUM(total_tokens), 0) as total_tokens
		FROM cost_log
		WHERE initiative_id = ?
		GROUP BY task_id
	`

	rows, err := s.globalDB.Query(query, initiativeID)
	if err != nil {
		s.jsonError(w, "failed to load initiative costs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var tasks []TaskCost
	var totalCost float64

	for rows.Next() {
		var tc TaskCost
		if err := rows.Scan(&tc.TaskID, &tc.CostUSD, &tc.Tokens); err != nil {
			s.jsonError(w, "failed to scan task cost: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, tc)
		totalCost += tc.CostUSD
	}

	if err := rows.Err(); err != nil {
		s.jsonError(w, "error iterating task costs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get breakdowns by model and phase
	byModel := make(map[string]float64)
	byPhase := make(map[string]float64)

	modelQuery := `
		SELECT COALESCE(model, '') as model, COALESCE(SUM(cost_usd), 0)
		FROM cost_log
		WHERE initiative_id = ?
		GROUP BY model
	`
	modelRows, err := s.globalDB.Query(modelQuery, initiativeID)
	if err == nil {
		defer func() { _ = modelRows.Close() }()
		for modelRows.Next() {
			var model string
			var cost float64
			if err := modelRows.Scan(&model, &cost); err == nil {
				if model == "" {
					model = "unknown"
				}
				byModel[model] = cost
			}
		}
	}

	phaseQuery := `
		SELECT phase, COALESCE(SUM(cost_usd), 0)
		FROM cost_log
		WHERE initiative_id = ?
		GROUP BY phase
	`
	phaseRows, err := s.globalDB.Query(phaseQuery, initiativeID)
	if err == nil {
		defer func() { _ = phaseRows.Close() }()
		for phaseRows.Next() {
			var phase string
			var cost float64
			if err := phaseRows.Scan(&phase, &cost); err == nil {
				byPhase[phase] = cost
			}
		}
	}

	response := InitiativeCostResponse{
		InitiativeID: initiativeID,
		TotalCostUSD: totalCost,
		ByTask:       tasks,
		ByModel:      byModel,
		ByPhase:      byPhase,
	}

	s.jsonResponse(w, response)
}

// ============================================================================
// Helper Functions
// ============================================================================

// parsePeriod converts a period string to a time.Time
func parsePeriod(period string) time.Time {
	now := time.Now().UTC()
	switch period {
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	default: // "all"
		return time.Time{} // Zero time = all data
	}
}

// parseDate parses a date string in YYYY-MM-DD or RFC3339 format
func parseDate(s string) (time.Time, error) {
	// Try YYYY-MM-DD first
	t, err := time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	// Try RFC3339
	return time.Parse(time.RFC3339, s)
}

// getProjectID derives the project ID from the working directory
func (s *Server) getProjectID() string {
	// Use the project root as project ID
	// This matches the executor pattern where projectID is derived from workDir
	return s.getProjectRoot()
}
