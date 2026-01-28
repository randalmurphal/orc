package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// PhaseGate represents per-phase gate configuration.
// Supplements config.yaml with database-driven overrides.
type PhaseGate struct {
	ID        int64     `json:"id"`
	PhaseID   string    `json:"phase_id"`  // spec, implement, test, review, etc.
	GateType  string    `json:"gate_type"` // auto, human, ai, skip
	Criteria  []string  `json:"criteria"`  // Criteria for auto gates
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TaskGateOverride represents per-task gate configuration.
// Takes precedence over PhaseGate and config.yaml.
type TaskGateOverride struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"task_id"`
	PhaseID   string    `json:"phase_id"`
	GateType  string    `json:"gate_type"` // auto, human, ai, skip
	CreatedAt time.Time `json:"created_at"`
}

// ErrPhaseGateNotFound is returned when a phase gate entry doesn't exist.
var ErrPhaseGateNotFound = errors.New("phase gate not found")

// ErrTaskGateOverrideNotFound is returned when a task gate override doesn't exist.
var ErrTaskGateOverrideNotFound = errors.New("task gate override not found")

// ---------------------- Phase Gates ----------------------

// SavePhaseGate creates or updates a phase gate configuration.
func (p *ProjectDB) SavePhaseGate(gate *PhaseGate) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var criteriaJSON []byte
	if len(gate.Criteria) > 0 {
		var err error
		criteriaJSON, err = json.Marshal(gate.Criteria)
		if err != nil {
			return fmt.Errorf("marshal criteria: %w", err)
		}
	}

	enabled := 0
	if gate.Enabled {
		enabled = 1
	}

	result, err := p.Exec(`
		INSERT INTO phase_gates (phase_id, gate_type, criteria, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM phase_gates WHERE phase_id = ?), ?), ?)
		ON CONFLICT(phase_id) DO UPDATE SET
			gate_type = excluded.gate_type,
			criteria = excluded.criteria,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, gate.PhaseID, gate.GateType, string(criteriaJSON), enabled, gate.PhaseID, now, now)
	if err != nil {
		return fmt.Errorf("save phase gate %s: %w", gate.PhaseID, err)
	}

	if gate.ID == 0 {
		id, _ := result.LastInsertId()
		gate.ID = id
	}

	return nil
}

// GetPhaseGate retrieves a phase gate by phase ID.
func (p *ProjectDB) GetPhaseGate(phaseID string) (*PhaseGate, error) {
	row := p.QueryRow(`
		SELECT id, phase_id, gate_type, criteria, enabled, created_at, updated_at
		FROM phase_gates
		WHERE phase_id = ?
	`, phaseID)

	gate, err := scanPhaseGate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPhaseGateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get phase gate %s: %w", phaseID, err)
	}

	return gate, nil
}

// ListPhaseGates returns all phase gates ordered by phase ID.
func (p *ProjectDB) ListPhaseGates() ([]*PhaseGate, error) {
	rows, err := p.Query(`
		SELECT id, phase_id, gate_type, criteria, enabled, created_at, updated_at
		FROM phase_gates
		ORDER BY phase_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list phase gates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var gates []*PhaseGate
	for rows.Next() {
		gate, err := scanPhaseGateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan phase gate: %w", err)
		}
		gates = append(gates, gate)
	}

	return gates, rows.Err()
}

// ListEnabledPhaseGates returns all enabled phase gates.
func (p *ProjectDB) ListEnabledPhaseGates() ([]*PhaseGate, error) {
	rows, err := p.Query(`
		SELECT id, phase_id, gate_type, criteria, enabled, created_at, updated_at
		FROM phase_gates
		WHERE enabled = 1
		ORDER BY phase_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled phase gates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var gates []*PhaseGate
	for rows.Next() {
		gate, err := scanPhaseGateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan phase gate: %w", err)
		}
		gates = append(gates, gate)
	}

	return gates, rows.Err()
}

// GetPhaseGatesMap returns all enabled phase gates as a map keyed by phase ID.
func (p *ProjectDB) GetPhaseGatesMap() (map[string]*PhaseGate, error) {
	gates, err := p.ListEnabledPhaseGates()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*PhaseGate, len(gates))
	for _, gate := range gates {
		result[gate.PhaseID] = gate
	}

	return result, nil
}

// DeletePhaseGate removes a phase gate by phase ID.
func (p *ProjectDB) DeletePhaseGate(phaseID string) error {
	result, err := p.Exec("DELETE FROM phase_gates WHERE phase_id = ?", phaseID)
	if err != nil {
		return fmt.Errorf("delete phase gate %s: %w", phaseID, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrPhaseGateNotFound
	}

	return nil
}

// SetPhaseGateEnabled enables or disables a phase gate.
func (p *ProjectDB) SetPhaseGateEnabled(phaseID string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := p.Exec(`
		UPDATE phase_gates
		SET enabled = ?, updated_at = ?
		WHERE phase_id = ?
	`, enabled, now, phaseID)
	if err != nil {
		return fmt.Errorf("set phase gate enabled %s: %w", phaseID, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrPhaseGateNotFound
	}

	return nil
}

// ---------------------- Task Gate Overrides ----------------------

// SaveTaskGateOverride creates or updates a task-specific gate override.
func (p *ProjectDB) SaveTaskGateOverride(override *TaskGateOverride) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := p.Exec(`
		INSERT INTO task_gate_overrides (task_id, phase_id, gate_type, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			gate_type = excluded.gate_type
	`, override.TaskID, override.PhaseID, override.GateType, now)
	if err != nil {
		return fmt.Errorf("save task gate override %s/%s: %w", override.TaskID, override.PhaseID, err)
	}

	if override.ID == 0 {
		id, _ := result.LastInsertId()
		override.ID = id
	}

	return nil
}

// GetTaskGateOverride retrieves a task gate override by task and phase ID.
func (p *ProjectDB) GetTaskGateOverride(taskID, phaseID string) (*TaskGateOverride, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase_id, gate_type, created_at
		FROM task_gate_overrides
		WHERE task_id = ? AND phase_id = ?
	`, taskID, phaseID)

	override, err := scanTaskGateOverride(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskGateOverrideNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task gate override %s/%s: %w", taskID, phaseID, err)
	}

	return override, nil
}

// ListTaskGateOverrides returns all gate overrides for a task.
func (p *ProjectDB) ListTaskGateOverrides(taskID string) ([]*TaskGateOverride, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase_id, gate_type, created_at
		FROM task_gate_overrides
		WHERE task_id = ?
		ORDER BY phase_id ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task gate overrides for %s: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

	var overrides []*TaskGateOverride
	for rows.Next() {
		override, err := scanTaskGateOverrideRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task gate override: %w", err)
		}
		overrides = append(overrides, override)
	}

	return overrides, rows.Err()
}

// GetTaskGateOverridesMap returns all gate overrides for a task as a map keyed by phase ID.
func (p *ProjectDB) GetTaskGateOverridesMap(taskID string) (map[string]*TaskGateOverride, error) {
	overrides, err := p.ListTaskGateOverrides(taskID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*TaskGateOverride, len(overrides))
	for _, override := range overrides {
		result[override.PhaseID] = override
	}

	return result, nil
}

// DeleteTaskGateOverride removes a task gate override.
func (p *ProjectDB) DeleteTaskGateOverride(taskID, phaseID string) error {
	result, err := p.Exec("DELETE FROM task_gate_overrides WHERE task_id = ? AND phase_id = ?", taskID, phaseID)
	if err != nil {
		return fmt.Errorf("delete task gate override %s/%s: %w", taskID, phaseID, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTaskGateOverrideNotFound
	}

	return nil
}

// DeleteAllTaskGateOverrides removes all gate overrides for a task.
func (p *ProjectDB) DeleteAllTaskGateOverrides(taskID string) error {
	_, err := p.Exec("DELETE FROM task_gate_overrides WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete all task gate overrides for %s: %w", taskID, err)
	}
	return nil
}

// ---------------------- Scanners ----------------------

func scanPhaseGate(row rowScanner) (*PhaseGate, error) {
	gate := &PhaseGate{}
	var criteriaJSON sql.NullString
	var enabled int
	var createdAt, updatedAt string

	err := row.Scan(
		&gate.ID,
		&gate.PhaseID,
		&gate.GateType,
		&criteriaJSON,
		&enabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	gate.Enabled = enabled == 1
	gate.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	gate.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if criteriaJSON.Valid && criteriaJSON.String != "" {
		if err := json.Unmarshal([]byte(criteriaJSON.String), &gate.Criteria); err != nil {
			gate.Criteria = nil
		}
	}

	return gate, nil
}

func scanPhaseGateRow(rows *sql.Rows) (*PhaseGate, error) {
	return scanPhaseGate(rows)
}

func scanTaskGateOverride(row rowScanner) (*TaskGateOverride, error) {
	override := &TaskGateOverride{}
	var createdAt string

	err := row.Scan(
		&override.ID,
		&override.TaskID,
		&override.PhaseID,
		&override.GateType,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	override.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return override, nil
}

func scanTaskGateOverrideRow(rows *sql.Rows) (*TaskGateOverride, error) {
	return scanTaskGateOverride(rows)
}
