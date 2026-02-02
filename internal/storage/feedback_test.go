// Package storage provides storage backend for orc.
//
// TDD Tests for TASK-741: Feedback storage operations
//
// These tests verify the storage backend operations for feedback persistence.
// The Backend interface will need new methods:
//   - SaveFeedback(f *Feedback) error
//   - GetFeedback(taskID, feedbackID string) (*Feedback, error)
//   - ListFeedback(taskID string, excludeReceived bool) ([]*Feedback, error)
//   - UpdateFeedback(f *Feedback) error
//   - DeleteFeedback(taskID, feedbackID string) error
//   - MarkFeedbackReceived(taskID string) (int, error)
//
// Success Criteria Coverage:
// - SC-7: Feedback is persisted to database
//
// These tests verify the storage layer contract that the API server depends on.
package storage

import (
	"testing"
	"time"
)

// Feedback represents user feedback to an agent during task execution.
// This struct defines the storage model for feedback.
type Feedback struct {
	ID       string
	TaskID   string
	Type     string // "inline", "general", "approval", "direction"
	File     string // For inline comments
	Line     int    // For inline comments
	Text     string
	Timing   string // "now", "when_done", "manual"
	SentAt   *time.Time
	Received bool
}

// ============================================================================
// SaveFeedback tests
// ============================================================================

// TestSaveFeedback_PersistsAllFields verifies all feedback fields are stored.
func TestSaveFeedback_PersistsAllFields(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	feedback := &Feedback{
		ID:     "fb-001",
		TaskID: "TASK-001",
		Type:   "inline",
		File:   "auth/login.go",
		Line:   47,
		Text:   "Use validateSession() instead",
		Timing: "when_done",
	}

	err := backend.SaveFeedback(feedback)
	if err != nil {
		t.Fatalf("SaveFeedback failed: %v", err)
	}

	loaded, err := backend.GetFeedback("TASK-001", "fb-001")
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	if loaded.ID != feedback.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, feedback.ID)
	}
	if loaded.TaskID != feedback.TaskID {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, feedback.TaskID)
	}
	if loaded.Type != feedback.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, feedback.Type)
	}
	if loaded.File != feedback.File {
		t.Errorf("File = %q, want %q", loaded.File, feedback.File)
	}
	if loaded.Line != feedback.Line {
		t.Errorf("Line = %d, want %d", loaded.Line, feedback.Line)
	}
	if loaded.Text != feedback.Text {
		t.Errorf("Text = %q, want %q", loaded.Text, feedback.Text)
	}
	if loaded.Timing != feedback.Timing {
		t.Errorf("Timing = %q, want %q", loaded.Timing, feedback.Timing)
	}
	if loaded.Received {
		t.Error("Received should be false for new feedback")
	}
}

// TestSaveFeedback_UpdatesExisting verifies saving with same ID updates.
func TestSaveFeedback_UpdatesExisting(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	feedback := &Feedback{
		ID:     "fb-001",
		TaskID: "TASK-001",
		Type:   "general",
		Text:   "Original text",
		Timing: "when_done",
	}

	_ = backend.SaveFeedback(feedback)

	// Update
	feedback.Text = "Updated text"
	err := backend.SaveFeedback(feedback)
	if err != nil {
		t.Fatalf("SaveFeedback (update) failed: %v", err)
	}

	loaded, _ := backend.GetFeedback("TASK-001", "fb-001")
	if loaded.Text != "Updated text" {
		t.Errorf("Text = %q, want %q", loaded.Text, "Updated text")
	}
}

// ============================================================================
// GetFeedback tests
// ============================================================================

// TestGetFeedback_NotFound returns error for non-existent feedback.
func TestGetFeedback_NotFound(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	_, err := backend.GetFeedback("TASK-001", "fb-nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent feedback, got nil")
	}
}

// ============================================================================
// ListFeedback tests
// ============================================================================

// TestListFeedback_ReturnsAllForTask returns all feedback for a task.
func TestListFeedback_ReturnsAllForTask(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Add feedback for task 1
	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "First", Timing: "when_done"})
	_ = backend.SaveFeedback(&Feedback{ID: "fb-002", TaskID: "TASK-001", Type: "general", Text: "Second", Timing: "when_done"})
	// Add feedback for task 2 (should not be included)
	_ = backend.SaveFeedback(&Feedback{ID: "fb-003", TaskID: "TASK-002", Type: "general", Text: "Other", Timing: "when_done"})

	list, err := backend.ListFeedback("TASK-001", false)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("expected 2 feedback items for TASK-001, got %d", len(list))
	}
}

// TestListFeedback_ExcludesReceived filters out received feedback.
func TestListFeedback_ExcludesReceived(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Add feedback, one received and one not
	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "Pending", Timing: "when_done", Received: false})
	_ = backend.SaveFeedback(&Feedback{ID: "fb-002", TaskID: "TASK-001", Type: "general", Text: "Received", Timing: "when_done", Received: true})

	list, err := backend.ListFeedback("TASK-001", true)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("expected 1 pending feedback item, got %d", len(list))
	}

	if list[0].ID != "fb-001" {
		t.Errorf("expected pending feedback fb-001, got %s", list[0].ID)
	}
}

// TestListFeedback_ReturnsEmptyForNoFeedback returns empty list.
func TestListFeedback_ReturnsEmptyForNoFeedback(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	list, err := backend.ListFeedback("TASK-001", false)
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("expected 0 feedback items, got %d", len(list))
	}
}

// ============================================================================
// MarkFeedbackReceived tests
// ============================================================================

// TestMarkFeedbackReceived_MarksAllPending marks all pending as received.
func TestMarkFeedbackReceived_MarksAllPending(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Add multiple pending feedback
	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "First", Timing: "when_done"})
	_ = backend.SaveFeedback(&Feedback{ID: "fb-002", TaskID: "TASK-001", Type: "general", Text: "Second", Timing: "when_done"})

	count, err := backend.MarkFeedbackReceived("TASK-001")
	if err != nil {
		t.Fatalf("MarkFeedbackReceived failed: %v", err)
	}

	if count != 2 {
		t.Errorf("marked count = %d, want 2", count)
	}

	// Verify all are now received
	list, _ := backend.ListFeedback("TASK-001", true)
	if len(list) != 0 {
		t.Errorf("expected 0 pending after marking, got %d", len(list))
	}
}

// TestMarkFeedbackReceived_SetsSentAt sets the SentAt timestamp.
func TestMarkFeedbackReceived_SetsSentAt(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "Test", Timing: "when_done"})

	before := time.Now()
	_, _ = backend.MarkFeedbackReceived("TASK-001")
	after := time.Now()

	feedback, _ := backend.GetFeedback("TASK-001", "fb-001")
	if feedback.SentAt == nil {
		t.Fatal("SentAt should be set after marking received")
	}

	if feedback.SentAt.Before(before) || feedback.SentAt.After(after) {
		t.Errorf("SentAt = %v, should be between %v and %v", feedback.SentAt, before, after)
	}
}

// TestMarkFeedbackReceived_ReturnsZeroForEmpty returns 0 when no pending.
func TestMarkFeedbackReceived_ReturnsZeroForEmpty(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	count, err := backend.MarkFeedbackReceived("TASK-001")
	if err != nil {
		t.Fatalf("MarkFeedbackReceived failed: %v", err)
	}

	if count != 0 {
		t.Errorf("marked count = %d, want 0 for empty", count)
	}
}

// ============================================================================
// DeleteFeedback tests
// ============================================================================

// TestDeleteFeedback_RemovesFeedback verifies deletion.
func TestDeleteFeedback_RemovesFeedback(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "Test", Timing: "when_done"})

	err := backend.DeleteFeedback("TASK-001", "fb-001")
	if err != nil {
		t.Fatalf("DeleteFeedback failed: %v", err)
	}

	_, err = backend.GetFeedback("TASK-001", "fb-001")
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}
}

// TestDeleteFeedback_NonExistent returns error for non-existent.
func TestDeleteFeedback_NonExistent(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	err := backend.DeleteFeedback("TASK-001", "fb-nonexistent")
	if err == nil {
		t.Error("expected error for non-existent feedback, got nil")
	}
}

// ============================================================================
// UpdateFeedback tests
// ============================================================================

// TestUpdateFeedback_UpdatesFields verifies field updates.
func TestUpdateFeedback_UpdatesFields(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	_ = backend.SaveFeedback(&Feedback{ID: "fb-001", TaskID: "TASK-001", Type: "general", Text: "Original", Timing: "when_done"})

	err := backend.UpdateFeedback(&Feedback{
		ID:       "fb-001",
		TaskID:   "TASK-001",
		Type:     "general",
		Text:     "Updated",
		Timing:   "manual",
		Received: true,
	})
	if err != nil {
		t.Fatalf("UpdateFeedback failed: %v", err)
	}

	loaded, _ := backend.GetFeedback("TASK-001", "fb-001")
	if loaded.Text != "Updated" {
		t.Errorf("Text = %q, want %q", loaded.Text, "Updated")
	}
	if loaded.Timing != "manual" {
		t.Errorf("Timing = %q, want %q", loaded.Timing, "manual")
	}
	if !loaded.Received {
		t.Error("Received should be true after update")
	}
}

// TestUpdateFeedback_NonExistent returns error for non-existent.
func TestUpdateFeedback_NonExistent(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	err := backend.UpdateFeedback(&Feedback{
		ID:     "fb-nonexistent",
		TaskID: "TASK-001",
		Text:   "Test",
	})
	if err == nil {
		t.Error("expected error for non-existent feedback, got nil")
	}
}
