package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Plan represents an execution plan stored in the database.
type Plan struct {
	TaskID      string
	Version     int
	Weight      string
	Description string
	Phases      string // JSON array of phase definitions
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SavePlan creates or updates a plan.
func (p *ProjectDB) SavePlan(plan *Plan) error {
	now := time.Now().Format(time.RFC3339)
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	_, err := p.Exec(`
		INSERT INTO plans (task_id, version, weight, description, phases, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			version = excluded.version,
			weight = excluded.weight,
			description = excluded.description,
			phases = excluded.phases,
			updated_at = excluded.updated_at
	`, plan.TaskID, plan.Version, plan.Weight, plan.Description, plan.Phases,
		plan.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	return nil
}

// GetPlan retrieves a plan by task ID.
func (p *ProjectDB) GetPlan(taskID string) (*Plan, error) {
	row := p.QueryRow(`
		SELECT task_id, version, weight, description, phases, created_at, updated_at
		FROM plans WHERE task_id = ?
	`, taskID)

	var plan Plan
	var description sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&plan.TaskID, &plan.Version, &plan.Weight, &description, &plan.Phases, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get plan %s: %w", taskID, err)
	}

	if description.Valid {
		plan.Description = description.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		plan.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		plan.UpdatedAt = ts
	}

	return &plan, nil
}

// DeletePlan removes a plan.
func (p *ProjectDB) DeletePlan(taskID string) error {
	_, err := p.Exec("DELETE FROM plans WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	return nil
}
