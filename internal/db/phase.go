package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Phase represents a phase execution state.
type Phase struct {
	TaskID       string
	PhaseID      string
	Status       string
	Iterations   int
	StartedAt    *time.Time
	CompletedAt  *time.Time
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	ErrorMessage string
	CommitSHA    string
	SkipReason   string
	SessionID    string // Claude CLI session UUID for --resume
}

// SavePhase creates or updates a phase.
func (p *ProjectDB) SavePhase(ph *Phase) error {
	var startedAt, completedAt *string
	if ph.StartedAt != nil {
		s := ph.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if ph.CompletedAt != nil {
		s := ph.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := p.Exec(`
		INSERT INTO phases (task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			error_message = excluded.error_message,
			commit_sha = excluded.commit_sha,
			skip_reason = excluded.skip_reason,
			session_id = COALESCE(excluded.session_id, phases.session_id)
	`, ph.TaskID, ph.PhaseID, ph.Status, ph.Iterations, startedAt, completedAt,
		ph.InputTokens, ph.OutputTokens, ph.CostUSD, ph.ErrorMessage, ph.CommitSHA, ph.SkipReason, ph.SessionID)
	if err != nil {
		return fmt.Errorf("save phase: %w", err)
	}
	return nil
}

// GetPhases retrieves all phases for a task.
func (p *ProjectDB) GetPhases(taskID string) ([]Phase, error) {
	rows, err := p.Query(`
		SELECT task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason, session_id
		FROM phases WHERE task_id = ?
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get phases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phases []Phase
	for rows.Next() {
		var ph Phase
		var startedAt, completedAt, errorMsg, commitSHA, skipReason, sessionID sql.NullString
		if err := rows.Scan(&ph.TaskID, &ph.PhaseID, &ph.Status, &ph.Iterations, &startedAt, &completedAt,
			&ph.InputTokens, &ph.OutputTokens, &ph.CostUSD, &errorMsg, &commitSHA, &skipReason, &sessionID); err != nil {
			return nil, fmt.Errorf("scan phase: %w", err)
		}
		if startedAt.Valid {
			if ts, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
				ph.StartedAt = &ts
			}
		}
		if completedAt.Valid {
			if ts, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				ph.CompletedAt = &ts
			}
		}
		if errorMsg.Valid {
			ph.ErrorMessage = errorMsg.String
		}
		if commitSHA.Valid {
			ph.CommitSHA = commitSHA.String
		}
		if skipReason.Valid {
			ph.SkipReason = skipReason.String
		}
		if sessionID.Valid {
			ph.SessionID = sessionID.String
		}
		phases = append(phases, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phases: %w", err)
	}

	return phases, nil
}

// ClearPhases removes all phases for a task.
func (p *ProjectDB) ClearPhases(taskID string) error {
	_, err := p.Exec("DELETE FROM phases WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("clear phases: %w", err)
	}
	return nil
}

// ClearPhasesTx removes all phases for a task within a transaction.
func ClearPhasesTx(tx *TxOps, taskID string) error {
	_, err := tx.Exec("DELETE FROM phases WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("clear phases: %w", err)
	}
	return nil
}

// SavePhaseTx saves a phase within a transaction.
func SavePhaseTx(tx *TxOps, ph *Phase) error {
	var startedAt, completedAt *string
	if ph.StartedAt != nil {
		s := ph.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if ph.CompletedAt != nil {
		s := ph.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := tx.Exec(`
		INSERT INTO phases (task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			error_message = excluded.error_message,
			commit_sha = excluded.commit_sha,
			skip_reason = excluded.skip_reason,
			session_id = COALESCE(excluded.session_id, phases.session_id)
	`, ph.TaskID, ph.PhaseID, ph.Status, ph.Iterations, startedAt, completedAt,
		ph.InputTokens, ph.OutputTokens, ph.CostUSD, ph.ErrorMessage, ph.CommitSHA, ph.SkipReason, ph.SessionID)
	if err != nil {
		return fmt.Errorf("save phase: %w", err)
	}
	return nil
}
