package storage

import (
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

const maxScratchpadEntrySize = 10 * 1024 // 10KB

// ScratchpadEntry represents a structured note from a phase execution.
type ScratchpadEntry struct {
	ID        int64
	TaskID    string
	PhaseID   string
	Category  string
	Content   string
	Attempt   int
	CreatedAt time.Time
}

// SaveScratchpadEntry persists a scratchpad entry.
// Returns error if task_id or content is empty.
// Content exceeding 10KB is truncated.
func (d *DatabaseBackend) SaveScratchpadEntry(entry *ScratchpadEntry) error {
	if entry.TaskID == "" {
		return fmt.Errorf("scratchpad entry task_id is required")
	}
	if entry.Content == "" {
		return fmt.Errorf("scratchpad entry content is required")
	}

	// Truncate content exceeding 10KB
	if len(entry.Content) > maxScratchpadEntrySize {
		entry.Content = entry.Content[:maxScratchpadEntrySize]
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	dbEntry := &db.ScratchpadEntry{
		TaskID:   entry.TaskID,
		PhaseID:  entry.PhaseID,
		Category: entry.Category,
		Content:  entry.Content,
		Attempt:  entry.Attempt,
	}
	if err := d.db.SaveScratchpadEntry(dbEntry); err != nil {
		return fmt.Errorf("save scratchpad entry: %w", err)
	}
	entry.ID = dbEntry.ID
	return nil
}

// GetScratchpadEntries returns all entries for a task ordered by creation time.
func (d *DatabaseBackend) GetScratchpadEntries(taskID string) ([]ScratchpadEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbEntries, err := d.db.GetScratchpadEntries(taskID)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries: %w", err)
	}

	return dbScratchpadToStorage(dbEntries), nil
}

// GetScratchpadEntriesByPhase returns entries for a task filtered by phase.
func (d *DatabaseBackend) GetScratchpadEntriesByPhase(taskID, phaseID string) ([]ScratchpadEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbEntries, err := d.db.GetScratchpadEntriesByPhase(taskID, phaseID)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries by phase: %w", err)
	}

	return dbScratchpadToStorage(dbEntries), nil
}

// GetScratchpadEntriesByAttempt returns entries for a task, phase, and attempt.
func (d *DatabaseBackend) GetScratchpadEntriesByAttempt(taskID, phaseID string, attempt int) ([]ScratchpadEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbEntries, err := d.db.GetScratchpadEntriesByAttempt(taskID, phaseID, attempt)
	if err != nil {
		return nil, fmt.Errorf("get scratchpad entries by attempt: %w", err)
	}

	return dbScratchpadToStorage(dbEntries), nil
}

func dbScratchpadToStorage(dbEntries []db.ScratchpadEntry) []ScratchpadEntry {
	entries := make([]ScratchpadEntry, len(dbEntries))
	for i, e := range dbEntries {
		entries[i] = ScratchpadEntry{
			ID:        e.ID,
			TaskID:    e.TaskID,
			PhaseID:   e.PhaseID,
			Category:  e.Category,
			Content:   e.Content,
			Attempt:   e.Attempt,
			CreatedAt: e.CreatedAt,
		}
	}
	return entries
}
