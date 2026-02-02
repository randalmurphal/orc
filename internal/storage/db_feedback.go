package storage

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
)

// ============================================================================
// Feedback operations - real-time user feedback to agents
// ============================================================================

func (d *DatabaseBackend) SaveFeedback(f *Feedback) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbFeedback := &db.Feedback{
		ID:       f.ID,
		TaskID:   f.TaskID,
		Type:     f.Type,
		Text:     f.Text,
		Timing:   f.Timing,
		File:     f.File,
		Line:     f.Line,
		Received:  f.Received,
		CreatedAt: f.CreatedAt,
		SentAt:   f.SentAt,
	}
	if err := d.db.CreateFeedback(dbFeedback); err != nil {
		return fmt.Errorf("save feedback: %w", err)
	}
	// Copy back generated ID and timestamps
	f.ID = dbFeedback.ID
	return nil
}

func (d *DatabaseBackend) GetFeedback(taskID, feedbackID string) (*Feedback, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbFeedback, err := d.db.GetFeedback(taskID, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("get feedback: %w", err)
	}

	return dbFeedbackToStorage(dbFeedback), nil
}

func (d *DatabaseBackend) ListFeedback(taskID string, excludeReceived bool) ([]*Feedback, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbFeedback, err := d.db.ListFeedback(taskID, excludeReceived)
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}

	result := make([]*Feedback, len(dbFeedback))
	for i, f := range dbFeedback {
		result[i] = dbFeedbackToStorage(f)
	}
	return result, nil
}

func (d *DatabaseBackend) UpdateFeedback(f *Feedback) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbFeedback := &db.Feedback{
		ID:       f.ID,
		TaskID:   f.TaskID,
		Type:     f.Type,
		Text:     f.Text,
		Timing:   f.Timing,
		File:     f.File,
		Line:     f.Line,
		Received:  f.Received,
		CreatedAt: f.CreatedAt,
		SentAt:   f.SentAt,
	}
	if err := d.db.UpdateFeedback(dbFeedback); err != nil {
		return fmt.Errorf("update feedback: %w", err)
	}
	return nil
}

func (d *DatabaseBackend) DeleteFeedback(taskID, feedbackID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.DeleteFeedback(taskID, feedbackID); err != nil {
		return fmt.Errorf("delete feedback: %w", err)
	}
	return nil
}

func (d *DatabaseBackend) MarkFeedbackReceived(taskID string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	count, err := d.db.MarkFeedbackReceived(taskID)
	if err != nil {
		return 0, fmt.Errorf("mark feedback received: %w", err)
	}
	return count, nil
}

// dbFeedbackToStorage converts a db.Feedback to storage.Feedback.
func dbFeedbackToStorage(f *db.Feedback) *Feedback {
	return &Feedback{
		ID:       f.ID,
		TaskID:   f.TaskID,
		Type:     f.Type,
		File:     f.File,
		Line:     f.Line,
		Text:     f.Text,
		Timing:   f.Timing,
		SentAt:   f.SentAt,
		Received:  f.Received,
		CreatedAt: f.CreatedAt,
	}
}
