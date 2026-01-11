package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SubtaskStatus represents the status of a queued sub-task.
type SubtaskStatus string

const (
	SubtaskPending  SubtaskStatus = "pending"
	SubtaskApproved SubtaskStatus = "approved"
	SubtaskRejected SubtaskStatus = "rejected"
)

// Subtask represents a proposed sub-task in the queue.
type Subtask struct {
	ID             string        `json:"id"`
	ParentTaskID   string        `json:"parent_task_id"`
	Title          string        `json:"title"`
	Description    string        `json:"description,omitempty"`
	ProposedBy     string        `json:"proposed_by,omitempty"`
	ProposedAt     time.Time     `json:"proposed_at"`
	Status         SubtaskStatus `json:"status"`
	ApprovedBy     string        `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time    `json:"approved_at,omitempty"`
	RejectedReason string        `json:"rejected_reason,omitempty"`
	CreatedTaskID  string        `json:"created_task_id,omitempty"`
}

// QueueSubtask adds a new sub-task to the queue.
func (p *ProjectDB) QueueSubtask(parentID, title, description, proposedBy string) (*Subtask, error) {
	id := "ST-" + uuid.New().String()[:8]

	_, err := p.Exec(`
		INSERT INTO subtask_queue (id, parent_task_id, title, description, proposed_by, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, parentID, title, description, proposedBy, SubtaskPending)
	if err != nil {
		return nil, fmt.Errorf("queue subtask: %w", err)
	}

	return p.GetSubtask(id)
}

// GetSubtask retrieves a sub-task by ID.
func (p *ProjectDB) GetSubtask(id string) (*Subtask, error) {
	row := p.QueryRow(`
		SELECT id, parent_task_id, title, description, proposed_by, proposed_at,
		       status, approved_by, approved_at, rejected_reason, created_task_id
		FROM subtask_queue WHERE id = ?
	`, id)

	return scanSubtask(row)
}

// ListPendingSubtasks returns all pending sub-tasks for a parent task.
func (p *ProjectDB) ListPendingSubtasks(parentID string) ([]*Subtask, error) {
	rows, err := p.Query(`
		SELECT id, parent_task_id, title, description, proposed_by, proposed_at,
		       status, approved_by, approved_at, rejected_reason, created_task_id
		FROM subtask_queue
		WHERE parent_task_id = ? AND status = ?
		ORDER BY proposed_at ASC
	`, parentID, SubtaskPending)
	if err != nil {
		return nil, fmt.Errorf("list pending subtasks: %w", err)
	}
	defer rows.Close()

	return scanSubtasks(rows)
}

// ListAllSubtasks returns all sub-tasks for a parent task.
func (p *ProjectDB) ListAllSubtasks(parentID string) ([]*Subtask, error) {
	rows, err := p.Query(`
		SELECT id, parent_task_id, title, description, proposed_by, proposed_at,
		       status, approved_by, approved_at, rejected_reason, created_task_id
		FROM subtask_queue
		WHERE parent_task_id = ?
		ORDER BY proposed_at ASC
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("list subtasks: %w", err)
	}
	defer rows.Close()

	return scanSubtasks(rows)
}

// CountPendingSubtasks returns the count of pending sub-tasks for a parent task.
func (p *ProjectDB) CountPendingSubtasks(parentID string) (int, error) {
	var count int
	err := p.QueryRow(`
		SELECT COUNT(*) FROM subtask_queue
		WHERE parent_task_id = ? AND status = ?
	`, parentID, SubtaskPending).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending subtasks: %w", err)
	}
	return count, nil
}

// ApproveSubtask marks a sub-task as approved.
func (p *ProjectDB) ApproveSubtask(id, approvedBy string) (*Subtask, error) {
	result, err := p.Exec(`
		UPDATE subtask_queue
		SET status = ?, approved_by = ?, approved_at = datetime('now')
		WHERE id = ? AND status = ?
	`, SubtaskApproved, approvedBy, id, SubtaskPending)
	if err != nil {
		return nil, fmt.Errorf("approve subtask: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("subtask %s not found or already processed", id)
	}

	return p.GetSubtask(id)
}

// RejectSubtask marks a sub-task as rejected with a reason.
func (p *ProjectDB) RejectSubtask(id, reason string) error {
	result, err := p.Exec(`
		UPDATE subtask_queue
		SET status = ?, rejected_reason = ?
		WHERE id = ? AND status = ?
	`, SubtaskRejected, reason, id, SubtaskPending)
	if err != nil {
		return fmt.Errorf("reject subtask: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("subtask %s not found or already processed", id)
	}

	return nil
}

// LinkSubtaskToTask updates the sub-task with the created task ID.
func (p *ProjectDB) LinkSubtaskToTask(subtaskID, taskID string) error {
	_, err := p.Exec(`
		UPDATE subtask_queue SET created_task_id = ? WHERE id = ?
	`, taskID, subtaskID)
	if err != nil {
		return fmt.Errorf("link subtask to task: %w", err)
	}
	return nil
}

// DeleteSubtask removes a sub-task from the queue.
func (p *ProjectDB) DeleteSubtask(id string) error {
	_, err := p.Exec(`DELETE FROM subtask_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete subtask: %w", err)
	}
	return nil
}

// scanSubtask scans a single row into a Subtask.
func scanSubtask(row *sql.Row) (*Subtask, error) {
	var s Subtask
	var proposedAt string
	var approvedAt sql.NullString
	var description, proposedBy, approvedBy, rejectedReason, createdTaskID sql.NullString

	err := row.Scan(
		&s.ID, &s.ParentTaskID, &s.Title, &description, &proposedBy, &proposedAt,
		&s.Status, &approvedBy, &approvedAt, &rejectedReason, &createdTaskID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan subtask: %w", err)
	}

	s.Description = description.String
	s.ProposedBy = proposedBy.String
	s.ApprovedBy = approvedBy.String
	s.RejectedReason = rejectedReason.String
	s.CreatedTaskID = createdTaskID.String

	if t, err := time.Parse("2006-01-02 15:04:05", proposedAt); err == nil {
		s.ProposedAt = t
	}
	if approvedAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", approvedAt.String); err == nil {
			s.ApprovedAt = &t
		}
	}

	return &s, nil
}

// scanSubtasks scans multiple rows into Subtasks.
func scanSubtasks(rows *sql.Rows) ([]*Subtask, error) {
	var subtasks []*Subtask

	for rows.Next() {
		var s Subtask
		var proposedAt string
		var approvedAt sql.NullString
		var description, proposedBy, approvedBy, rejectedReason, createdTaskID sql.NullString

		err := rows.Scan(
			&s.ID, &s.ParentTaskID, &s.Title, &description, &proposedBy, &proposedAt,
			&s.Status, &approvedBy, &approvedAt, &rejectedReason, &createdTaskID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan subtask: %w", err)
		}

		s.Description = description.String
		s.ProposedBy = proposedBy.String
		s.ApprovedBy = approvedBy.String
		s.RejectedReason = rejectedReason.String
		s.CreatedTaskID = createdTaskID.String

		if t, err := time.Parse("2006-01-02 15:04:05", proposedAt); err == nil {
			s.ProposedAt = t
		}
		if approvedAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", approvedAt.String); err == nil {
				s.ApprovedAt = &t
			}
		}

		subtasks = append(subtasks, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtasks: %w", err)
	}

	return subtasks, nil
}
