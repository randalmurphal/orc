package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// ReviewCommentSeverity represents the severity level of a review comment.
type ReviewCommentSeverity string

const (
	SeveritySuggestion ReviewCommentSeverity = "suggestion"
	SeverityIssue      ReviewCommentSeverity = "issue"
	SeverityBlocker    ReviewCommentSeverity = "blocker"
)

// ReviewCommentStatus represents the status of a review comment.
type ReviewCommentStatus string

const (
	CommentStatusOpen    ReviewCommentStatus = "open"
	CommentStatusResolved ReviewCommentStatus = "resolved"
	CommentStatusWontFix ReviewCommentStatus = "wont_fix"
)

// ReviewComment represents a code review comment.
type ReviewComment struct {
	ID          string              `json:"id"`
	TaskID      string              `json:"task_id"`
	ReviewRound int                 `json:"review_round"`
	FilePath    string              `json:"file_path,omitempty"`
	LineNumber  int                 `json:"line_number,omitempty"`
	Content     string              `json:"content"`
	Severity    ReviewCommentSeverity `json:"severity"`
	Status      ReviewCommentStatus `json:"status"`
	CreatedAt   time.Time           `json:"created_at"`
	ResolvedAt  *time.Time          `json:"resolved_at,omitempty"`
	ResolvedBy  string              `json:"resolved_by,omitempty"`
}

// CreateReviewComment adds a new review comment.
func (p *ProjectDB) CreateReviewComment(c *ReviewComment) error {
	if c.ID == "" {
		c.ID = generateCommentID()
	}
	if c.Status == "" {
		c.Status = CommentStatusOpen
	}
	if c.Severity == "" {
		c.Severity = SeveritySuggestion
	}
	if c.ReviewRound == 0 {
		c.ReviewRound = 1
	}

	_, err := p.Exec(`
		INSERT INTO review_comments (id, task_id, review_round, file_path, line_number, content, severity, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.TaskID, c.ReviewRound, c.FilePath, c.LineNumber, c.Content, c.Severity, c.Status)
	if err != nil {
		return fmt.Errorf("create review comment: %w", err)
	}

	// Reload to get created_at timestamp
	created, err := p.GetReviewComment(c.ID)
	if err == nil && created != nil {
		c.CreatedAt = created.CreatedAt
	}

	return nil
}

// GetReviewComment retrieves a single comment by ID.
func (p *ProjectDB) GetReviewComment(id string) (*ReviewComment, error) {
	row := p.QueryRow(`
		SELECT id, task_id, review_round, file_path, line_number, content, severity, status, created_at, resolved_at, resolved_by
		FROM review_comments WHERE id = ?
	`, id)
	return scanReviewComment(row)
}

// ListReviewComments returns all comments for a task.
func (p *ProjectDB) ListReviewComments(taskID string, status string) ([]ReviewComment, error) {
	query := `
		SELECT id, task_id, review_round, file_path, line_number, content, severity, status, created_at, resolved_at, resolved_by
		FROM review_comments WHERE task_id = ?
	`
	args := []any{taskID}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY file_path, line_number"

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list review comments: %w", err)
	}
	defer rows.Close()

	var comments []ReviewComment
	for rows.Next() {
		c, err := scanReviewCommentRows(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review comments: %w", err)
	}

	return comments, nil
}

// ListReviewCommentsByRound returns all comments for a specific review round.
func (p *ProjectDB) ListReviewCommentsByRound(taskID string, round int) ([]ReviewComment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, review_round, file_path, line_number, content, severity, status, created_at, resolved_at, resolved_by
		FROM review_comments WHERE task_id = ? AND review_round = ?
		ORDER BY file_path, line_number
	`, taskID, round)
	if err != nil {
		return nil, fmt.Errorf("list review comments by round: %w", err)
	}
	defer rows.Close()

	var comments []ReviewComment
	for rows.Next() {
		c, err := scanReviewCommentRows(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, *c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review comments: %w", err)
	}

	return comments, nil
}

// UpdateReviewComment updates a comment.
func (p *ProjectDB) UpdateReviewComment(c *ReviewComment) error {
	_, err := p.Exec(`
		UPDATE review_comments
		SET content = ?, severity = ?, status = ?, resolved_at = ?, resolved_by = ?
		WHERE id = ?
	`, c.Content, c.Severity, c.Status, formatNullableTime(c.ResolvedAt), c.ResolvedBy, c.ID)
	if err != nil {
		return fmt.Errorf("update review comment: %w", err)
	}
	return nil
}

// DeleteReviewComment removes a comment.
func (p *ProjectDB) DeleteReviewComment(id string) error {
	_, err := p.Exec("DELETE FROM review_comments WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete review comment: %w", err)
	}
	return nil
}

// ResolveReviewComment marks a comment as resolved.
func (p *ProjectDB) ResolveReviewComment(id, resolvedBy string, status ReviewCommentStatus) error {
	if status == "" {
		status = CommentStatusResolved
	}
	now := time.Now().Format(time.RFC3339)
	_, err := p.Exec(`
		UPDATE review_comments SET status = ?, resolved_at = ?, resolved_by = ? WHERE id = ?
	`, status, now, resolvedBy, id)
	if err != nil {
		return fmt.Errorf("resolve review comment: %w", err)
	}
	return nil
}

// CountOpenComments returns the count of open comments for a task.
func (p *ProjectDB) CountOpenComments(taskID string) (int, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM review_comments WHERE task_id = ? AND status = 'open'", taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count open comments: %w", err)
	}
	return count, nil
}

// CountBlockerComments returns the count of open blocker comments for a task.
func (p *ProjectDB) CountBlockerComments(taskID string) (int, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM review_comments WHERE task_id = ? AND status = 'open' AND severity = 'blocker'", taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count blocker comments: %w", err)
	}
	return count, nil
}

// GetLatestReviewRound returns the highest review round number for a task.
func (p *ProjectDB) GetLatestReviewRound(taskID string) (int, error) {
	var round int
	err := p.QueryRow("SELECT COALESCE(MAX(review_round), 0) FROM review_comments WHERE task_id = ?", taskID).Scan(&round)
	if err != nil {
		return 0, fmt.Errorf("get latest review round: %w", err)
	}
	return round, nil
}

// DeleteTaskReviewComments removes all review comments for a task.
func (p *ProjectDB) DeleteTaskReviewComments(taskID string) error {
	_, err := p.Exec("DELETE FROM review_comments WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete task review comments: %w", err)
	}
	return nil
}

func scanReviewComment(row *sql.Row) (*ReviewComment, error) {
	var c ReviewComment
	var filePath sql.NullString
	var lineNumber sql.NullInt64
	var resolvedAt, resolvedBy sql.NullString
	var createdAt string

	err := row.Scan(&c.ID, &c.TaskID, &c.ReviewRound, &filePath, &lineNumber,
		&c.Content, &c.Severity, &c.Status, &createdAt, &resolvedAt, &resolvedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan review comment: %w", err)
	}

	if filePath.Valid {
		c.FilePath = filePath.String
	}
	if lineNumber.Valid {
		c.LineNumber = int(lineNumber.Int64)
	}
	if resolvedBy.Valid {
		c.ResolvedBy = resolvedBy.String
	}

	c.CreatedAt = parseTimestamp(createdAt)
	if resolvedAt.Valid {
		t := parseTimestamp(resolvedAt.String)
		c.ResolvedAt = &t
	}

	return &c, nil
}

func scanReviewCommentRows(rows *sql.Rows) (*ReviewComment, error) {
	var c ReviewComment
	var filePath sql.NullString
	var lineNumber sql.NullInt64
	var resolvedAt, resolvedBy sql.NullString
	var createdAt string

	err := rows.Scan(&c.ID, &c.TaskID, &c.ReviewRound, &filePath, &lineNumber,
		&c.Content, &c.Severity, &c.Status, &createdAt, &resolvedAt, &resolvedBy)
	if err != nil {
		return nil, fmt.Errorf("scan review comment: %w", err)
	}

	if filePath.Valid {
		c.FilePath = filePath.String
	}
	if lineNumber.Valid {
		c.LineNumber = int(lineNumber.Int64)
	}
	if resolvedBy.Valid {
		c.ResolvedBy = resolvedBy.String
	}

	c.CreatedAt = parseTimestamp(createdAt)
	if resolvedAt.Valid {
		t := parseTimestamp(resolvedAt.String)
		c.ResolvedAt = &t
	}

	return &c, nil
}

// parseTimestamp tries to parse a timestamp in common formats.
func parseTimestamp(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t
	}
	return time.Time{}
}

// formatNullableTime formats a time pointer for database storage.
func formatNullableTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// generateCommentID generates a unique ID for a review comment.
func generateCommentID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "RC-" + hex.EncodeToString(b)[:8]
}
