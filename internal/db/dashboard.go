package db

import (
	"fmt"
	"strings"
	"time"
)

// DashboardStatusCounts holds aggregated task status counts from SQL.
type DashboardStatusCounts struct {
	Completed int
	Failed    int
	Running   int
	Blocked   int
	Total     int
}

// DashboardCostByDate holds daily cost aggregation from SQL.
type DashboardCostByDate struct {
	Date    string
	CostUSD float64
}

// DashboardInitiativeStat holds per-initiative task statistics from SQL.
type DashboardInitiativeStat struct {
	InitiativeID   string
	TaskCount      int
	CompletedCount int
	CostUSD        float64
}

// GetDashboardStatusCounts returns task status counts using SQL aggregation.
// This is much faster than loading all tasks and counting in Go.
func (p *ProjectDB) GetDashboardStatusCounts() (*DashboardStatusCounts, error) {
	rows, err := p.Query(`
		SELECT status, COUNT(*) as cnt
		FROM tasks
		GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("get dashboard status counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := &DashboardStatusCounts{}
	for rows.Next() {
		var status string
		var cnt int
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		counts.Total += cnt
		switch status {
		case "completed":
			counts.Completed = cnt
		case "failed":
			counts.Failed = cnt
		case "running":
			counts.Running = cnt
		case "blocked":
			counts.Blocked = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status counts: %w", err)
	}

	return counts, nil
}

// GetDashboardCostByDate returns daily cost aggregation using SQL GROUP BY.
// Only includes phases with costs since the given time.
func (p *ProjectDB) GetDashboardCostByDate(since time.Time) ([]DashboardCostByDate, error) {
	sinceStr := since.Format(time.RFC3339)
	rows, err := p.Query(`
		SELECT DATE(t.completed_at) as day, COALESCE(SUM(ph.cost_usd), 0) as total_cost
		FROM tasks t
		JOIN phases ph ON ph.task_id = t.id
		WHERE t.completed_at IS NOT NULL AND t.completed_at >= ?
		GROUP BY DATE(t.completed_at)
		ORDER BY day
	`, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("get dashboard cost by date: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []DashboardCostByDate
	for rows.Next() {
		var r DashboardCostByDate
		if err := rows.Scan(&r.Date, &r.CostUSD); err != nil {
			return nil, fmt.Errorf("scan cost by date: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cost by date: %w", err)
	}

	return results, nil
}

// GetDashboardInitiativeStats returns per-initiative task statistics using SQL aggregation.
// Results are sorted by task count descending, limited to the given count.
func (p *ProjectDB) GetDashboardInitiativeStats(limit int) ([]DashboardInitiativeStat, error) {
	rows, err := p.Query(`
		SELECT
			initiative_id,
			COUNT(*) as task_count,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
			COALESCE(SUM(total_cost_usd), 0) as cost_usd
		FROM tasks
		WHERE initiative_id IS NOT NULL AND initiative_id != ''
		GROUP BY initiative_id
		ORDER BY task_count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get dashboard initiative stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []DashboardInitiativeStat
	for rows.Next() {
		var s DashboardInitiativeStat
		if err := rows.Scan(&s.InitiativeID, &s.TaskCount, &s.CompletedCount, &s.CostUSD); err != nil {
			return nil, fmt.Errorf("scan initiative stat: %w", err)
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate initiative stats: %w", err)
	}

	return results, nil
}

// GetInitiativeTitlesBatch loads initiative titles for multiple IDs in a single query.
// Returns a map of initiative ID to title. Missing IDs are absent from the result.
func (p *ProjectDB) GetInitiativeTitlesBatch(ids []string) (map[string]string, error) {
	if len(ids) == 0 {
		return map[string]string{}, nil
	}

	// Build parameterized IN clause
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, title FROM initiatives
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get initiative titles batch: %w", err)
	}
	defer func() { _ = rows.Close() }()

	titles := make(map[string]string, len(ids))
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, fmt.Errorf("scan initiative title: %w", err)
		}
		titles[id] = title
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate initiative titles: %w", err)
	}

	return titles, nil
}
