// Package storage provides storage backend for orc.
//
// TDD Tests for TASK-020: Phase scratchpad persistent note-taking
//
// These tests verify the storage backend operations for scratchpad persistence.
// The Backend interface will need new methods:
//   - SaveScratchpadEntry(entry *ScratchpadEntry) error
//   - GetScratchpadEntries(taskID string) ([]ScratchpadEntry, error)
//   - GetScratchpadEntriesByPhase(taskID, phaseID string) ([]ScratchpadEntry, error)
//   - GetScratchpadEntriesByAttempt(taskID, phaseID string, attempt int) ([]ScratchpadEntry, error)
//
// Success Criteria Coverage:
//   - SC-1: Save and retrieve scratchpad entries, ordered by creation time
//   - SC-2: Filter entries by phase_id and attempt number
//   - SC-6: SQLite migration creates the table correctly
//
// Failure Mode Coverage:
//   - Empty task_id or content returns error
//   - Filter with non-existent phase returns empty slice (not error)
//   - Entry exceeding 10KB is truncated on save
package storage

import (
	"strings"
	"testing"
	"time"
)

// ============================================================================
// SC-1: Save and retrieve scratchpad entries by task_id, ordered by creation time
// ============================================================================

// TestScratchpadEntry_SaveAndRetrieve verifies that saving 3 entries and
// retrieving by task_id returns all 3 in creation order.
func TestScratchpadEntry_SaveAndRetrieve(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entries := []ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Project uses chi router", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "spec", Category: "decision", Content: "Chose token bucket for rate limiting", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "blocker", Content: "Test framework requires Node 18+", Attempt: 1},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("SaveScratchpadEntry[%d] failed: %v", i, err)
		}
		// Small delay to ensure distinct creation times
		time.Sleep(time.Millisecond)
	}

	// Retrieve all entries for TASK-001
	loaded, err := backend.GetScratchpadEntries("TASK-001")
	if err != nil {
		t.Fatalf("GetScratchpadEntries failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}

	// Verify order is by creation time (ascending)
	if loaded[0].Content != "Project uses chi router" {
		t.Errorf("first entry content = %q, want %q", loaded[0].Content, "Project uses chi router")
	}
	if loaded[1].Content != "Chose token bucket for rate limiting" {
		t.Errorf("second entry content = %q, want %q", loaded[1].Content, "Chose token bucket for rate limiting")
	}
	if loaded[2].Content != "Test framework requires Node 18+" {
		t.Errorf("third entry content = %q, want %q", loaded[2].Content, "Test framework requires Node 18+")
	}

	// Verify fields are preserved
	if loaded[0].TaskID != "TASK-001" {
		t.Errorf("entry TaskID = %q, want %q", loaded[0].TaskID, "TASK-001")
	}
	if loaded[0].PhaseID != "spec" {
		t.Errorf("entry PhaseID = %q, want %q", loaded[0].PhaseID, "spec")
	}
	if loaded[0].Category != "observation" {
		t.Errorf("entry Category = %q, want %q", loaded[0].Category, "observation")
	}
	if loaded[0].Attempt != 1 {
		t.Errorf("entry Attempt = %d, want %d", loaded[0].Attempt, 1)
	}
	if loaded[0].CreatedAt.IsZero() {
		t.Error("entry CreatedAt should not be zero")
	}
}

// TestScratchpadEntry_SaveEmptyTaskID verifies that saving with empty task_id returns error.
func TestScratchpadEntry_SaveEmptyTaskID(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entry := &ScratchpadEntry{
		TaskID:   "",
		PhaseID:  "spec",
		Category: "observation",
		Content:  "Some content",
		Attempt:  1,
	}

	err := backend.SaveScratchpadEntry(entry)
	if err == nil {
		t.Error("SaveScratchpadEntry with empty task_id should return error")
	}
}

// TestScratchpadEntry_SaveEmptyContent verifies that saving with empty content returns error.
func TestScratchpadEntry_SaveEmptyContent(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entry := &ScratchpadEntry{
		TaskID:   "TASK-001",
		PhaseID:  "spec",
		Category: "observation",
		Content:  "",
		Attempt:  1,
	}

	err := backend.SaveScratchpadEntry(entry)
	if err == nil {
		t.Error("SaveScratchpadEntry with empty content should return error")
	}
}

// TestScratchpadEntry_RetrieveNonExistentTask returns empty slice for task with no entries.
func TestScratchpadEntry_RetrieveNonExistentTask(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entries, err := backend.GetScratchpadEntries("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetScratchpadEntries for non-existent task should not error, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestScratchpadEntry_IsolationBetweenTasks verifies entries for different tasks are isolated.
func TestScratchpadEntry_IsolationBetweenTasks(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Save entries for two different tasks
	entry1 := &ScratchpadEntry{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Task 1 entry", Attempt: 1}
	entry2 := &ScratchpadEntry{TaskID: "TASK-002", PhaseID: "spec", Category: "observation", Content: "Task 2 entry", Attempt: 1}

	if err := backend.SaveScratchpadEntry(entry1); err != nil {
		t.Fatalf("save entry1: %v", err)
	}
	if err := backend.SaveScratchpadEntry(entry2); err != nil {
		t.Fatalf("save entry2: %v", err)
	}

	// Retrieve for TASK-001 only
	entries, err := backend.GetScratchpadEntries("TASK-001")
	if err != nil {
		t.Fatalf("GetScratchpadEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for TASK-001, got %d", len(entries))
	}
	if entries[0].Content != "Task 1 entry" {
		t.Errorf("content = %q, want %q", entries[0].Content, "Task 1 entry")
	}
}

// ============================================================================
// SC-2: Filter entries by phase_id and attempt number
// ============================================================================

// TestScratchpadEntry_FilterByPhase verifies filtering entries by phase_id.
func TestScratchpadEntry_FilterByPhase(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entries := []ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Spec observation", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Implementation decision", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "spec", Category: "decision", Content: "Spec decision", Attempt: 1},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry[%d]: %v", i, err)
		}
	}

	// Filter by spec phase
	specEntries, err := backend.GetScratchpadEntriesByPhase("TASK-001", "spec")
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByPhase: %v", err)
	}
	if len(specEntries) != 2 {
		t.Fatalf("expected 2 spec entries, got %d", len(specEntries))
	}

	// Filter by implement phase
	implEntries, err := backend.GetScratchpadEntriesByPhase("TASK-001", "implement")
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByPhase: %v", err)
	}
	if len(implEntries) != 1 {
		t.Fatalf("expected 1 implement entry, got %d", len(implEntries))
	}
	if implEntries[0].Content != "Implementation decision" {
		t.Errorf("content = %q, want %q", implEntries[0].Content, "Implementation decision")
	}
}

// TestScratchpadEntry_FilterByNonExistentPhase returns empty slice, not error.
func TestScratchpadEntry_FilterByNonExistentPhase(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Save an entry
	entry := &ScratchpadEntry{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Entry", Attempt: 1}
	if err := backend.SaveScratchpadEntry(entry); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Filter by non-existent phase
	entries, err := backend.GetScratchpadEntriesByPhase("TASK-001", "nonexistent")
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByPhase for non-existent phase should not error, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for non-existent phase, got %d", len(entries))
	}
}

// TestScratchpadEntry_FilterByAttempt verifies filtering entries by attempt number.
func TestScratchpadEntry_FilterByAttempt(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entries := []ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "implement", Category: "blocker", Content: "Attempt 1 blocker", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "observation", Content: "Attempt 1 observation", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Attempt 2 decision", Attempt: 2},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry[%d]: %v", i, err)
		}
	}

	// Filter by attempt 1
	attempt1, err := backend.GetScratchpadEntriesByAttempt("TASK-001", "implement", 1)
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByAttempt: %v", err)
	}
	if len(attempt1) != 2 {
		t.Fatalf("expected 2 attempt-1 entries, got %d", len(attempt1))
	}

	// Filter by attempt 2
	attempt2, err := backend.GetScratchpadEntriesByAttempt("TASK-001", "implement", 2)
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByAttempt: %v", err)
	}
	if len(attempt2) != 1 {
		t.Fatalf("expected 1 attempt-2 entry, got %d", len(attempt2))
	}
	if attempt2[0].Content != "Attempt 2 decision" {
		t.Errorf("content = %q, want %q", attempt2[0].Content, "Attempt 2 decision")
	}
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestScratchpadEntry_UnknownCategoryAccepted verifies unknown categories are stored.
func TestScratchpadEntry_UnknownCategoryAccepted(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entry := &ScratchpadEntry{
		TaskID:   "TASK-001",
		PhaseID:  "spec",
		Category: "custom_category",
		Content:  "Entry with custom category",
		Attempt:  1,
	}

	if err := backend.SaveScratchpadEntry(entry); err != nil {
		t.Fatalf("SaveScratchpadEntry with unknown category should succeed, got: %v", err)
	}

	entries, err := backend.GetScratchpadEntries("TASK-001")
	if err != nil {
		t.Fatalf("GetScratchpadEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Category != "custom_category" {
		t.Errorf("category = %q, want %q", entries[0].Category, "custom_category")
	}
}

// TestScratchpadEntry_Truncation verifies entries exceeding 10KB are truncated.
func TestScratchpadEntry_Truncation(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Create content exceeding 10KB
	largeContent := strings.Repeat("x", 11*1024) // 11KB

	entry := &ScratchpadEntry{
		TaskID:   "TASK-001",
		PhaseID:  "spec",
		Category: "observation",
		Content:  largeContent,
		Attempt:  1,
	}

	if err := backend.SaveScratchpadEntry(entry); err != nil {
		t.Fatalf("SaveScratchpadEntry with large content should succeed (truncated), got: %v", err)
	}

	entries, err := backend.GetScratchpadEntries("TASK-001")
	if err != nil {
		t.Fatalf("GetScratchpadEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Content should be truncated to 10KB
	if len(entries[0].Content) > 10*1024 {
		t.Errorf("content length = %d, should be truncated to <= %d", len(entries[0].Content), 10*1024)
	}
}

// TestScratchpadEntry_RetryScratchpadPhaseIsolation verifies that retry scratchpad
// shows only the previous attempt's entries for THAT phase, not entries from other phases.
func TestScratchpadEntry_RetryScratchpadPhaseIsolation(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	entries := []ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Spec entry", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "blocker", Content: "Implement attempt 1 blocker", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "decision", Content: "Implement attempt 2 fix", Attempt: 2},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry[%d]: %v", i, err)
		}
	}

	// When querying for implement phase attempt 1 (to show in retry attempt 2)
	attempt1Impl, err := backend.GetScratchpadEntriesByAttempt("TASK-001", "implement", 1)
	if err != nil {
		t.Fatalf("GetScratchpadEntriesByAttempt: %v", err)
	}
	if len(attempt1Impl) != 1 {
		t.Fatalf("expected 1 implement attempt-1 entry, got %d", len(attempt1Impl))
	}
	if attempt1Impl[0].Content != "Implement attempt 1 blocker" {
		t.Errorf("content = %q, want %q", attempt1Impl[0].Content, "Implement attempt 1 blocker")
	}
}
