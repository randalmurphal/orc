package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

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
		db.Close()
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
		db.Close()
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
	defer rows.Close()

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
	ID           int64
	ProjectID    string
	TaskID       string
	Phase        string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
	Timestamp    time.Time
}

// RecordCost logs a cost entry.
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
func (g *GlobalDB) GetCostSummary(projectID string, since time.Time) (*CostSummary, error) {
	summary := &CostSummary{
		ByProject: make(map[string]float64),
		ByPhase:   make(map[string]float64),
	}

	// Build query based on filters
	// Use strftime to format 'since' to match SQLite's datetime('now') format
	// SQLite stores as "YYYY-MM-DD HH:MM:SS", RFC3339 is "YYYY-MM-DDTHH:MM:SSZ"
	query := `
		SELECT
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COUNT(*)
		FROM cost_log
		WHERE timestamp >= ?
	`
	args := []any{since.UTC().Format("2006-01-02 15:04:05")}

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}

	row := g.QueryRow(query, args...)
	if err := row.Scan(&summary.TotalCostUSD, &summary.TotalInput, &summary.TotalOutput, &summary.EntryCount); err != nil {
		return nil, fmt.Errorf("get cost summary: %w", err)
	}

	// Get breakdown by project
	projQuery := `
		SELECT project_id, SUM(cost_usd)
		FROM cost_log
		WHERE timestamp >= ?
		GROUP BY project_id
	`
	rows, err := g.Query(projQuery, since.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("get cost by project: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pid string
		var cost float64
		if err := rows.Scan(&pid, &cost); err != nil {
			return nil, fmt.Errorf("scan project cost: %w", err)
		}
		summary.ByProject[pid] = cost
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project costs: %w", err)
	}

	// Get breakdown by phase
	phaseQuery := `
		SELECT phase, SUM(cost_usd)
		FROM cost_log
		WHERE timestamp >= ?
		GROUP BY phase
	`
	if projectID != "" {
		phaseQuery = `
			SELECT phase, SUM(cost_usd)
			FROM cost_log
			WHERE timestamp >= ? AND project_id = ?
			GROUP BY phase
		`
	}

	sinceStr := since.UTC().Format("2006-01-02 15:04:05")
	if projectID != "" {
		rows, err = g.Query(phaseQuery, sinceStr, projectID)
	} else {
		rows, err = g.Query(phaseQuery, sinceStr)
	}
	if err != nil {
		return nil, fmt.Errorf("get cost by phase: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var phase string
		var cost float64
		if err := rows.Scan(&phase, &cost); err != nil {
			return nil, fmt.Errorf("scan phase cost: %w", err)
		}
		summary.ByPhase[phase] = cost
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase costs: %w", err)
	}

	return summary, nil
}
