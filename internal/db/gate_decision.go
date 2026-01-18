package db

import (
	"database/sql"
	"fmt"
	"time"
)

// GateDecision represents a gate approval decision.
type GateDecision struct {
	ID        int64
	TaskID    string
	Phase     string
	GateType  string // 'auto', 'ai', 'human', 'skip'
	Approved  bool
	Reason    string
	DecidedBy string
	DecidedAt time.Time
}

// AddGateDecision records a gate decision.
func (p *ProjectDB) AddGateDecision(d *GateDecision) error {
	approved := 0
	if d.Approved {
		approved = 1
	}
	if d.DecidedAt.IsZero() {
		d.DecidedAt = time.Now()
	}

	result, err := p.Exec(`
		INSERT INTO gate_decisions (task_id, phase, gate_type, approved, reason, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.TaskID, d.Phase, d.GateType, approved, d.Reason, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add gate decision: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return nil
}

// GetGateDecisions retrieves all gate decisions for a task.
func (p *ProjectDB) GetGateDecisions(taskID string) ([]GateDecision, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, gate_type, approved, reason, decided_by, decided_at
		FROM gate_decisions WHERE task_id = ? ORDER BY decided_at
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get gate decisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var decisions []GateDecision
	for rows.Next() {
		var d GateDecision
		var approved int
		var reason, decidedBy sql.NullString
		var decidedAt string

		if err := rows.Scan(&d.ID, &d.TaskID, &d.Phase, &d.GateType, &approved, &reason, &decidedBy, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan gate decision: %w", err)
		}

		d.Approved = approved == 1
		if reason.Valid {
			d.Reason = reason.String
		}
		if decidedBy.Valid {
			d.DecidedBy = decidedBy.String
		}
		if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
			d.DecidedAt = ts
		}

		decisions = append(decisions, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gate decisions: %w", err)
	}

	return decisions, nil
}

// GetGateDecisionForPhase retrieves the gate decision for a specific phase.
func (p *ProjectDB) GetGateDecisionForPhase(taskID, phase string) (*GateDecision, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase, gate_type, approved, reason, decided_by, decided_at
		FROM gate_decisions WHERE task_id = ? AND phase = ?
		ORDER BY decided_at DESC LIMIT 1
	`, taskID, phase)

	var d GateDecision
	var approved int
	var reason, decidedBy sql.NullString
	var decidedAt string

	if err := row.Scan(&d.ID, &d.TaskID, &d.Phase, &d.GateType, &approved, &reason, &decidedBy, &decidedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get gate decision: %w", err)
	}

	d.Approved = approved == 1
	if reason.Valid {
		d.Reason = reason.String
	}
	if decidedBy.Valid {
		d.DecidedBy = decidedBy.String
	}
	if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
		d.DecidedAt = ts
	}

	return &d, nil
}

// AddGateDecisionTx adds a gate decision within a transaction.
func AddGateDecisionTx(tx *TxOps, d *GateDecision) error {
	approved := 0
	if d.Approved {
		approved = 1
	}
	if d.DecidedAt.IsZero() {
		d.DecidedAt = time.Now()
	}

	result, err := tx.Exec(`
		INSERT INTO gate_decisions (task_id, phase, gate_type, approved, reason, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.TaskID, d.Phase, d.GateType, approved, d.Reason, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add gate decision: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return nil
}
