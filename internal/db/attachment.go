package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Attachment represents a task attachment stored in the database.
type Attachment struct {
	ID          int64
	TaskID      string
	Filename    string
	ContentType string
	SizeBytes   int64
	Data        []byte
	IsImage     bool
	CreatedAt   time.Time
}

// SaveAttachment stores an attachment in the database.
func (p *ProjectDB) SaveAttachment(a *Attachment) error {
	isImage := 0
	if a.IsImage {
		isImage = 1
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}

	result, err := p.Exec(`
		INSERT INTO task_attachments (task_id, filename, content_type, size_bytes, data, is_image, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, filename) DO UPDATE SET
			content_type = excluded.content_type,
			size_bytes = excluded.size_bytes,
			data = excluded.data,
			is_image = excluded.is_image
	`, a.TaskID, a.Filename, a.ContentType, a.SizeBytes, a.Data, isImage, a.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	if a.ID == 0 {
		id, _ := result.LastInsertId()
		a.ID = id
	}
	return nil
}

// GetAttachment retrieves an attachment by task ID and filename.
func (p *ProjectDB) GetAttachment(taskID, filename string) (*Attachment, error) {
	row := p.QueryRow(`
		SELECT id, task_id, filename, content_type, size_bytes, data, is_image, created_at
		FROM task_attachments WHERE task_id = ? AND filename = ?
	`, taskID, filename)

	var a Attachment
	var contentType sql.NullString
	var isImage int
	var createdAt string

	if err := row.Scan(&a.ID, &a.TaskID, &a.Filename, &contentType, &a.SizeBytes, &a.Data, &isImage, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	if contentType.Valid {
		a.ContentType = contentType.String
	}
	a.IsImage = isImage == 1
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		a.CreatedAt = ts
	}

	return &a, nil
}

// ListAttachments retrieves attachment metadata for a task (without data).
func (p *ProjectDB) ListAttachments(taskID string) ([]Attachment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, filename, content_type, size_bytes, is_image, created_at
		FROM task_attachments WHERE task_id = ? ORDER BY filename
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		var contentType sql.NullString
		var isImage int
		var createdAt string

		if err := rows.Scan(&a.ID, &a.TaskID, &a.Filename, &contentType, &a.SizeBytes, &isImage, &createdAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}

		if contentType.Valid {
			a.ContentType = contentType.String
		}
		a.IsImage = isImage == 1
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			a.CreatedAt = ts
		}

		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}

	return attachments, nil
}

// DeleteAttachment removes an attachment.
// Returns an error containing "not found" if the attachment doesn't exist.
func (p *ProjectDB) DeleteAttachment(taskID, filename string) error {
	result, err := p.Exec("DELETE FROM task_attachments WHERE task_id = ? AND filename = ?", taskID, filename)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("attachment %s not found", filename)
	}

	return nil
}

// DeleteAllAttachments removes all attachments for a task.
func (p *ProjectDB) DeleteAllAttachments(taskID string) error {
	_, err := p.Exec("DELETE FROM task_attachments WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete all attachments: %w", err)
	}
	return nil
}
