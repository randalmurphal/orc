package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ScratchpadEntry represents a structured note from a phase execution.
type ScratchpadEntry struct {
	ID        int64
	TaskID    string
	PhaseID   string
	Category  string
	Content   string
	Attempt   int
	CreatedAt time.Time
}

// SaveScratchpadEntry persists a scratchpad entry.
func (p *ProjectDB) SaveScratchpadEntry(e *ScratchpadEntry) error {
	now := time.Now().Format(time.RFC3339Nano)
	result, err := p.Exec(`
		INSERT INTO phase_scratchpad (task_id, phase_id, category, content, attempt, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.TaskID, e.PhaseID, e.Category, e.Content, e.Attempt, now)
	if err != nil {
		return fmt.Errorf("save scratchpad entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		e.ID = id
	}

	return nil
}

// GetScratchpadEntries returns all entries for a task ordered by creation time.
func (p *ProjectDB) GetScratchpadEntries(taskID string) ([]ScratchpadEntry, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase_id, category, content, attempt, created_at
		FROM phase_scratchpad
		WHERE task_id = ?
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries: %w", err)
	}
	defer rows.Close()

	return scanScratchpadEntries(rows)
}

// GetScratchpadEntriesByPhase returns entries for a task filtered by phase.
func (p *ProjectDB) GetScratchpadEntriesByPhase(taskID, phaseID string) ([]ScratchpadEntry, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase_id, category, content, attempt, created_at
		FROM phase_scratchpad
		WHERE task_id = ? AND phase_id = ?
		ORDER BY created_at ASC
	`, taskID, phaseID)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries by phase: %w", err)
	}
	defer rows.Close()

	return scanScratchpadEntries(rows)
}

// GetScratchpadEntriesByAttempt returns entries for a task, phase, and attempt.
func (p *ProjectDB) GetScratchpadEntriesByAttempt(taskID, phaseID string, attempt int) ([]ScratchpadEntry, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase_id, category, content, attempt, created_at
		FROM phase_scratchpad
		WHERE task_id = ? AND phase_id = ? AND attempt = ?
		ORDER BY created_at ASC
	`, taskID, phaseID, attempt)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries by attempt: %w", err)
	}
	defer rows.Close()

	return scanScratchpadEntries(rows)
}

func scanScratchpadEntries(rows *sql.Rows) ([]ScratchpadEntry, error) {
	var entries []ScratchpadEntry
	for rows.Next() {
		var e ScratchpadEntry
		var createdStr string
		if err := rows.Scan(&e.ID, &e.TaskID, &e.PhaseID, &e.Category, &e.Content, &e.Attempt, &createdStr); err != nil {
			return nil, fmt.Errorf("scan scratchpad entry: %w", err)
		}
		if t, err := time.Parse(time.RFC3339Nano, createdStr); err == nil {
			e.CreatedAt = t
		} else if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			e.CreatedAt = t
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scratchpad entries: %w", err)
	}
	if entries == nil {
		entries = []ScratchpadEntry{}
	}
	return entries, nil
}
