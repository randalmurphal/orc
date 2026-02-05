package db

import (
	"database/sql"
	"fmt"
	"time"
)

// UserClaimHistoryEntry represents a row in the task_claim_history table.
type UserClaimHistoryEntry struct {
	ID         int64
	TaskID     string
	UserID     string
	ClaimedAt  time.Time
	ReleasedAt *time.Time
	StolenFrom *string
}

// ClaimTaskByUser atomically claims a task for a user using a single UPDATE.
// Returns the number of rows affected: 1 if claimed successfully, 0 if already
// claimed by another user or task doesn't exist.
// The claim is idempotent - re-claiming your own task succeeds.
func (p *ProjectDB) ClaimTaskByUser(taskID, userID string) (int64, error) {
	now := time.Now().Format(time.RFC3339)

	// Atomic UPDATE: succeeds if unclaimed OR already claimed by the same user
	result, err := p.Exec(`
		UPDATE tasks
		SET claimed_by = ?, claimed_at = ?
		WHERE id = ? AND (claimed_by IS NULL OR claimed_by = ?)
	`, userID, now, taskID, userID)
	if err != nil {
		return 0, fmt.Errorf("claim task %s: %w", taskID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check claim result for task %s: %w", taskID, err)
	}

	// Insert history entry on successful claim
	if rowsAffected == 1 {
		_, err = p.Exec(`
			INSERT INTO task_claim_history (task_id, user_id, claimed_at)
			VALUES (?, ?, ?)
		`, taskID, userID, now)
		if err != nil {
			return 0, fmt.Errorf("record claim history for task %s: %w", taskID, err)
		}
	}

	return rowsAffected, nil
}

// ForceClaimTaskByUser forcefully claims a task, stealing it from any current owner.
// Returns the previous owner's user ID (empty if unclaimed or self-claim).
func (p *ProjectDB) ForceClaimTaskByUser(taskID, userID string) (string, error) {
	now := time.Now().Format(time.RFC3339)

	// Read current claimer
	var currentClaimer sql.NullString
	err := p.QueryRow(`SELECT claimed_by FROM tasks WHERE id = ?`, taskID).Scan(&currentClaimer)
	if err != nil {
		return "", fmt.Errorf("get current claimer for task %s: %w", taskID, err)
	}

	// If already claimed by the same user, it's a no-op
	if currentClaimer.Valid && currentClaimer.String == userID {
		return "", nil
	}

	// Force update claimed_by regardless of current state
	_, err = p.Exec(`
		UPDATE tasks
		SET claimed_by = ?, claimed_at = ?
		WHERE id = ?
	`, userID, now, taskID)
	if err != nil {
		return "", fmt.Errorf("force claim task %s: %w", taskID, err)
	}

	// Release previous claimer's history entry if there was one
	if currentClaimer.Valid && currentClaimer.String != "" {
		_, err = p.Exec(`
			UPDATE task_claim_history
			SET released_at = ?
			WHERE task_id = ? AND user_id = ? AND released_at IS NULL
		`, now, taskID, currentClaimer.String)
		if err != nil {
			return "", fmt.Errorf("release previous claim history for task %s: %w", taskID, err)
		}
	}

	// Determine stolen_from
	var stolenFrom *string
	if currentClaimer.Valid && currentClaimer.String != "" {
		stolenFrom = &currentClaimer.String
	}

	// Insert history entry for the new claim
	_, err = p.Exec(`
		INSERT INTO task_claim_history (task_id, user_id, claimed_at, stolen_from)
		VALUES (?, ?, ?, ?)
	`, taskID, userID, now, stolenFrom)
	if err != nil {
		return "", fmt.Errorf("record force claim history for task %s: %w", taskID, err)
	}

	if stolenFrom != nil {
		return *stolenFrom, nil
	}
	return "", nil
}

// ReleaseUserClaim releases a user's claim on a task.
// Returns the number of rows affected: 1 if released, 0 if not the owner or already released.
func (p *ProjectDB) ReleaseUserClaim(taskID, userID string) (int64, error) {
	now := time.Now().Format(time.RFC3339)

	// Atomic UPDATE: only succeeds if claimed by the specified user
	result, err := p.Exec(`
		UPDATE tasks
		SET claimed_by = NULL, claimed_at = NULL
		WHERE id = ? AND claimed_by = ?
	`, taskID, userID)
	if err != nil {
		return 0, fmt.Errorf("release claim for task %s: %w", taskID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check release result for task %s: %w", taskID, err)
	}

	// Update history entry with released_at on successful release
	if rowsAffected == 1 {
		_, err = p.Exec(`
			UPDATE task_claim_history
			SET released_at = ?
			WHERE task_id = ? AND user_id = ? AND released_at IS NULL
		`, now, taskID, userID)
		if err != nil {
			return 0, fmt.Errorf("record release history for task %s: %w", taskID, err)
		}
	}

	return rowsAffected, nil
}

// GetUserClaimHistory returns all claim history entries for a task, ordered by claimed_at.
// Returns an empty (non-nil) slice if no history exists.
func (p *ProjectDB) GetUserClaimHistory(taskID string) ([]UserClaimHistoryEntry, error) {
	rows, err := p.Query(`
		SELECT id, task_id, user_id, claimed_at, released_at, stolen_from
		FROM task_claim_history
		WHERE task_id = ?
		ORDER BY claimed_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get claim history for task %s: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

	history := make([]UserClaimHistoryEntry, 0)
	for rows.Next() {
		var entry UserClaimHistoryEntry
		var claimedAt string
		var releasedAt, stolenFrom sql.NullString

		if err := rows.Scan(&entry.ID, &entry.TaskID, &entry.UserID, &claimedAt, &releasedAt, &stolenFrom); err != nil {
			return nil, fmt.Errorf("scan claim history: %w", err)
		}

		if ts, err := time.Parse(time.RFC3339, claimedAt); err == nil {
			entry.ClaimedAt = ts
		}
		if releasedAt.Valid {
			if ts, err := time.Parse(time.RFC3339, releasedAt.String); err == nil {
				entry.ReleasedAt = &ts
			}
		}
		if stolenFrom.Valid {
			entry.StolenFrom = &stolenFrom.String
		}

		history = append(history, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claim history: %w", err)
	}

	return history, nil
}
