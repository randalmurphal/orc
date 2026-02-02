package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// Feedback represents user feedback to an agent during task execution.
type Feedback struct {
	ID       string
	TaskID   string
	Type     string // "general", "inline", "approval", "direction"
	Text     string
	Timing   string // "now", "when_done", "manual"
	File     string // For inline comments
	Line     int    // For inline comments
	Received bool
	SentAt   *time.Time
	CreatedAt time.Time
}

// CreateFeedback adds or updates a feedback item (upsert).
func (p *ProjectDB) CreateFeedback(f *Feedback) error {
	if f.ID == "" {
		f.ID = generateFeedbackID()
	}

	now := time.Now().Format(time.RFC3339Nano)
	_, err := p.Exec(`
		INSERT OR REPLACE INTO feedback (id, task_id, type, text, timing, file, line, received, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM feedback WHERE id = ?), ?))
	`, f.ID, f.TaskID, f.Type, f.Text, f.Timing, nullableString(f.File), nullableInt(f.Line), f.Received, f.ID, now)
	if err != nil {
		return fmt.Errorf("create feedback: %w", err)
	}

	// Reload to get timestamps
	created, err := p.GetFeedback(f.TaskID, f.ID)
	if err == nil && created != nil {
		f.CreatedAt = created.CreatedAt
	}

	return nil
}

// GetFeedback retrieves a single feedback item by task ID and feedback ID.
func (p *ProjectDB) GetFeedback(taskID, feedbackID string) (*Feedback, error) {
	row := p.QueryRow(`
		SELECT id, task_id, type, text, timing, file, line, received, sent_at, created_at
		FROM feedback WHERE task_id = ? AND id = ?
	`, taskID, feedbackID)
	return scanFeedback(row)
}

// ListFeedback returns all feedback for a task.
func (p *ProjectDB) ListFeedback(taskID string, excludeReceived bool) ([]*Feedback, error) {
	var rows *sql.Rows
	var err error

	if excludeReceived {
		rows, err = p.Query(`
			SELECT id, task_id, type, text, timing, file, line, received, sent_at, created_at
			FROM feedback WHERE task_id = ? AND received = FALSE
			ORDER BY created_at ASC
		`, taskID)
	} else {
		rows, err = p.Query(`
			SELECT id, task_id, type, text, timing, file, line, received, sent_at, created_at
			FROM feedback WHERE task_id = ?
			ORDER BY created_at ASC
		`, taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var feedback []*Feedback
	for rows.Next() {
		f, err := scanFeedbackRows(rows)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate feedback: %w", err)
	}

	return feedback, nil
}

// UpdateFeedback updates a feedback item.
func (p *ProjectDB) UpdateFeedback(f *Feedback) error {
	var sentAt *string
	if f.SentAt != nil {
		ts := f.SentAt.Format(time.RFC3339Nano)
		sentAt = &ts
	}

	result, err := p.Exec(`
		UPDATE feedback
		SET type = ?, text = ?, timing = ?, file = ?, line = ?, received = ?, sent_at = ?
		WHERE id = ? AND task_id = ?
	`, f.Type, f.Text, f.Timing, nullableString(f.File), nullableInt(f.Line), f.Received, sentAt, f.ID, f.TaskID)
	if err != nil {
		return fmt.Errorf("update feedback: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("feedback not found: %s", f.ID)
	}

	return nil
}

// DeleteFeedback removes a feedback item.
func (p *ProjectDB) DeleteFeedback(taskID, feedbackID string) error {
	result, err := p.Exec("DELETE FROM feedback WHERE task_id = ? AND id = ?", taskID, feedbackID)
	if err != nil {
		return fmt.Errorf("delete feedback: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("feedback not found: %s", feedbackID)
	}

	return nil
}

// MarkFeedbackReceived marks all pending feedback for a task as received.
// Returns the count of feedback items marked.
func (p *ProjectDB) MarkFeedbackReceived(taskID string) (int, error) {
	now := time.Now().Format(time.RFC3339Nano)
	result, err := p.Exec(`
		UPDATE feedback
		SET received = TRUE, sent_at = ?
		WHERE task_id = ? AND received = FALSE
	`, now, taskID)
	if err != nil {
		return 0, fmt.Errorf("mark feedback received: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

func scanFeedback(row *sql.Row) (*Feedback, error) {
	var f Feedback
	var file sql.NullString
	var line sql.NullInt64
	var sentAt sql.NullString
	var createdAt string

	err := row.Scan(&f.ID, &f.TaskID, &f.Type, &f.Text, &f.Timing, &file, &line, &f.Received, &sentAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("feedback not found")
		}
		return nil, fmt.Errorf("scan feedback: %w", err)
	}

	if file.Valid {
		f.File = file.String
	}
	if line.Valid {
		f.Line = int(line.Int64)
	}
	if sentAt.Valid {
		t := parseTimestamp(sentAt.String)
		f.SentAt = &t
	}
	f.CreatedAt = parseTimestamp(createdAt)

	return &f, nil
}

func scanFeedbackRows(rows *sql.Rows) (*Feedback, error) {
	var f Feedback
	var file sql.NullString
	var line sql.NullInt64
	var sentAt sql.NullString
	var createdAt string

	err := rows.Scan(&f.ID, &f.TaskID, &f.Type, &f.Text, &f.Timing, &file, &line, &f.Received, &sentAt, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan feedback: %w", err)
	}

	if file.Valid {
		f.File = file.String
	}
	if line.Valid {
		f.Line = int(line.Int64)
	}
	if sentAt.Valid {
		t := parseTimestamp(sentAt.String)
		f.SentAt = &t
	}
	f.CreatedAt = parseTimestamp(createdAt)

	return &f, nil
}

// generateFeedbackID generates a unique ID for a feedback item.
func generateFeedbackID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return "FB-" + hex.EncodeToString(b)[:8]
}

// nullableInt returns a pointer to n if non-zero, nil otherwise.
func nullableInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}
