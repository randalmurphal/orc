package storage

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
)

// ============================================================================
// Task and review comments - user-facing discussion
// ============================================================================

func (d *DatabaseBackend) ListTaskComments(taskID string) ([]TaskComment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbComments, err := d.db.ListTaskComments(taskID)
	if err != nil {
		return nil, fmt.Errorf("list task comments: %w", err)
	}

	result := make([]TaskComment, len(dbComments))
	for i, c := range dbComments {
		result[i] = TaskComment{
			ID:         c.ID,
			TaskID:     c.TaskID,
			Author:     c.Author,
			AuthorType: string(c.AuthorType),
			Content:    c.Content,
			Phase:      c.Phase,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
		}
	}
	return result, nil
}

func (d *DatabaseBackend) SaveTaskComment(c *TaskComment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbComment := &db.TaskComment{
		ID:         c.ID,
		TaskID:     c.TaskID,
		Author:     c.Author,
		AuthorType: db.AuthorType(c.AuthorType),
		Content:    c.Content,
		Phase:      c.Phase,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
	if err := d.db.CreateTaskComment(dbComment); err != nil {
		return fmt.Errorf("save task comment: %w", err)
	}
	c.ID = dbComment.ID
	return nil
}

func (d *DatabaseBackend) ListReviewComments(taskID string) ([]ReviewComment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbComments, err := d.db.ListReviewComments(taskID, "")
	if err != nil {
		return nil, fmt.Errorf("list review comments: %w", err)
	}

	result := make([]ReviewComment, len(dbComments))
	for i, c := range dbComments {
		result[i] = ReviewComment{
			ID:          c.ID,
			TaskID:      c.TaskID,
			ReviewRound: c.ReviewRound,
			FilePath:    c.FilePath,
			LineNumber:  c.LineNumber,
			Content:     c.Content,
			Severity:    string(c.Severity),
			Status:      string(c.Status),
			CreatedAt:   c.CreatedAt,
			ResolvedAt:  c.ResolvedAt,
			ResolvedBy:  c.ResolvedBy,
		}
	}
	return result, nil
}

func (d *DatabaseBackend) SaveReviewComment(c *ReviewComment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbComment := &db.ReviewComment{
		ID:          c.ID,
		TaskID:      c.TaskID,
		ReviewRound: c.ReviewRound,
		FilePath:    c.FilePath,
		LineNumber:  c.LineNumber,
		Content:     c.Content,
		Severity:    db.ReviewCommentSeverity(c.Severity),
		Status:      db.ReviewCommentStatus(c.Status),
		CreatedAt:   c.CreatedAt,
		ResolvedAt:  c.ResolvedAt,
		ResolvedBy:  c.ResolvedBy,
	}
	if err := d.db.CreateReviewComment(dbComment); err != nil {
		return fmt.Errorf("save review comment: %w", err)
	}
	c.ID = dbComment.ID
	return nil
}
