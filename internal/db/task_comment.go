package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthorType represents who created the comment.
type AuthorType string

const (
	AuthorTypeHuman AuthorType = "human"
	AuthorTypeAgent AuthorType = "agent"
	AuthorTypeSystem AuthorType = "system"
)

// TaskComment represents a comment or note on a task.
type TaskComment struct {
	ID         string     `json:"id"`
	TaskID     string     `json:"task_id"`
	Author     string     `json:"author"`
	AuthorType AuthorType `json:"author_type"`
	Content    string     `json:"content"`
	Phase      string     `json:"phase,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CreateTaskComment adds a new comment to a task.
func (p *ProjectDB) CreateTaskComment(c *TaskComment) error {
	if c.ID == "" {
		c.ID = generateTaskCommentID()
	}
	if c.AuthorType == "" {
		c.AuthorType = AuthorTypeHuman
	}
	if c.Author == "" {
		c.Author = "anonymous"
	}

	now := time.Now().Format(time.RFC3339)
	_, err := p.Exec(`
		INSERT INTO task_comments (id, task_id, author, author_type, content, phase, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.TaskID, c.Author, c.AuthorType, c.Content, nullableString(c.Phase), now, now)
	if err != nil {
		return fmt.Errorf("create task comment: %w", err)
	}

	// Reload to get timestamps
	created, err := p.GetTaskComment(c.ID)
	if err == nil && created != nil {
		c.CreatedAt = created.CreatedAt
		c.UpdatedAt = created.UpdatedAt
	}

	return nil
}

// GetTaskComment retrieves a single comment by ID.
func (p *ProjectDB) GetTaskComment(id string) (*TaskComment, error) {
	row := p.QueryRow(`
		SELECT id, task_id, author, author_type, content, phase, created_at, updated_at
		FROM task_comments WHERE id = ?
	`, id)
	return scanTaskComment(row)
}

// ListTaskComments returns all comments for a task.
func (p *ProjectDB) ListTaskComments(taskID string) ([]TaskComment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, author, author_type, content, phase, created_at, updated_at
		FROM task_comments WHERE task_id = ?
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task comments: %w", err)
	}
	defer rows.Close()

	var comments []TaskComment
	for rows.Next() {
		c, err := scanTaskCommentRows(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task comments: %w", err)
	}

	return comments, nil
}

// ListTaskCommentsByAuthorType returns comments filtered by author type.
func (p *ProjectDB) ListTaskCommentsByAuthorType(taskID string, authorType AuthorType) ([]TaskComment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, author, author_type, content, phase, created_at, updated_at
		FROM task_comments WHERE task_id = ? AND author_type = ?
		ORDER BY created_at ASC
	`, taskID, authorType)
	if err != nil {
		return nil, fmt.Errorf("list task comments by author type: %w", err)
	}
	defer rows.Close()

	var comments []TaskComment
	for rows.Next() {
		c, err := scanTaskCommentRows(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task comments: %w", err)
	}

	return comments, nil
}

// ListTaskCommentsByPhase returns comments for a specific phase.
func (p *ProjectDB) ListTaskCommentsByPhase(taskID, phase string) ([]TaskComment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, author, author_type, content, phase, created_at, updated_at
		FROM task_comments WHERE task_id = ? AND phase = ?
		ORDER BY created_at ASC
	`, taskID, phase)
	if err != nil {
		return nil, fmt.Errorf("list task comments by phase: %w", err)
	}
	defer rows.Close()

	var comments []TaskComment
	for rows.Next() {
		c, err := scanTaskCommentRows(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task comments: %w", err)
	}

	return comments, nil
}

// UpdateTaskComment updates a comment's content.
func (p *ProjectDB) UpdateTaskComment(c *TaskComment) error {
	now := time.Now().Format(time.RFC3339)
	_, err := p.Exec(`
		UPDATE task_comments
		SET content = ?, phase = ?, updated_at = ?
		WHERE id = ?
	`, c.Content, nullableString(c.Phase), now, c.ID)
	if err != nil {
		return fmt.Errorf("update task comment: %w", err)
	}
	return nil
}

// DeleteTaskComment removes a comment.
func (p *ProjectDB) DeleteTaskComment(id string) error {
	_, err := p.Exec("DELETE FROM task_comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task comment: %w", err)
	}
	return nil
}

// DeleteAllTaskComments removes all comments for a task.
func (p *ProjectDB) DeleteAllTaskComments(taskID string) error {
	_, err := p.Exec("DELETE FROM task_comments WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete all task comments: %w", err)
	}
	return nil
}

// CountTaskComments returns the count of comments for a task.
func (p *ProjectDB) CountTaskComments(taskID string) (int, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM task_comments WHERE task_id = ?", taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count task comments: %w", err)
	}
	return count, nil
}

func scanTaskComment(row *sql.Row) (*TaskComment, error) {
	var c TaskComment
	var phase sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&c.ID, &c.TaskID, &c.Author, &c.AuthorType, &c.Content, &phase, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan task comment: %w", err)
	}

	if phase.Valid {
		c.Phase = phase.String
	}

	c.CreatedAt = parseTimestamp(createdAt)
	c.UpdatedAt = parseTimestamp(updatedAt)

	return &c, nil
}

func scanTaskCommentRows(rows *sql.Rows) (*TaskComment, error) {
	var c TaskComment
	var phase sql.NullString
	var createdAt, updatedAt string

	err := rows.Scan(&c.ID, &c.TaskID, &c.Author, &c.AuthorType, &c.Content, &phase, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan task comment: %w", err)
	}

	if phase.Valid {
		c.Phase = phase.String
	}

	c.CreatedAt = parseTimestamp(createdAt)
	c.UpdatedAt = parseTimestamp(updatedAt)

	return &c, nil
}

// generateTaskCommentID generates a unique ID for a task comment.
func generateTaskCommentID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "TC-" + hex.EncodeToString(b)[:8]
}

// nullableString returns a pointer to s if non-empty, nil otherwise.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
