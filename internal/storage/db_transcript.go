package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// AddTranscript adds a transcript to database.
func (d *DatabaseBackend) AddTranscript(t *Transcript) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbTranscript := &db.Transcript{
		TaskID:              t.TaskID,
		Phase:               t.Phase,
		SessionID:           t.SessionID,
		WorkflowRunID:       t.WorkflowRunID,
		MessageUUID:         t.MessageUUID,
		ParentUUID:          t.ParentUUID,
		Type:                t.Type,
		Role:                t.Role,
		Content:             t.Content,
		Model:               t.Model,
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		ToolCalls:           t.ToolCalls,
		ToolResults:         t.ToolResults,
		Timestamp:           time.UnixMilli(t.Timestamp),
	}
	if err := d.db.AddTranscript(dbTranscript); err != nil {
		return err
	}
	t.ID = dbTranscript.ID
	return nil
}

// AddTranscriptBatch adds multiple transcripts in a single transaction.
func (d *DatabaseBackend) AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error {
	if len(transcripts) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	dbTranscripts := make([]db.Transcript, len(transcripts))
	for i, t := range transcripts {
		dbTranscripts[i] = db.Transcript{
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			WorkflowRunID:       t.WorkflowRunID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           time.UnixMilli(t.Timestamp),
		}
	}

	if err := d.db.AddTranscriptBatch(ctx, dbTranscripts); err != nil {
		return fmt.Errorf("add transcript batch: %w", err)
	}

	for i := range transcripts {
		transcripts[i].ID = dbTranscripts[i].ID
	}
	return nil
}

// GetTranscripts retrieves transcripts for a task.
func (d *DatabaseBackend) GetTranscripts(taskID string) ([]Transcript, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTranscripts, err := d.db.GetTranscripts(taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}

	result := make([]Transcript, len(dbTranscripts))
	for i, t := range dbTranscripts {
		result[i] = Transcript{
			ID:                  t.ID,
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           t.Timestamp.UnixMilli(),
		}
	}
	return result, nil
}

// GetTranscriptsPaginated retrieves paginated transcripts with filtering.
func (d *DatabaseBackend) GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOpts := db.TranscriptPaginationOpts{
		Phase:     opts.Phase,
		Cursor:    opts.Cursor,
		Limit:     opts.Limit,
		Direction: opts.Direction,
	}

	dbTranscripts, dbPagination, err := d.db.GetTranscriptsPaginated(taskID, dbOpts)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("get paginated transcripts: %w", err)
	}

	result := make([]Transcript, len(dbTranscripts))
	for i, t := range dbTranscripts {
		result[i] = Transcript{
			ID:                  t.ID,
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           t.Timestamp.UnixMilli(),
		}
	}

	pagination := PaginationResult{
		NextCursor: dbPagination.NextCursor,
		PrevCursor: dbPagination.PrevCursor,
		HasMore:    dbPagination.HasMore,
		TotalCount: dbPagination.TotalCount,
	}

	return result, pagination, nil
}

// GetPhaseSummary returns transcript counts grouped by phase.
func (d *DatabaseBackend) GetPhaseSummary(taskID string) ([]PhaseSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbSummaries, err := d.db.GetPhaseSummary(taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase summary: %w", err)
	}

	result := make([]PhaseSummary, len(dbSummaries))
	for i, s := range dbSummaries {
		result[i] = PhaseSummary{
			Phase:           s.Phase,
			TranscriptCount: s.TranscriptCount,
		}
	}
	return result, nil
}

// SearchTranscripts performs FTS search across transcripts.
func (d *DatabaseBackend) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbMatches, err := d.db.SearchTranscripts(query)
	if err != nil {
		return nil, fmt.Errorf("search transcripts: %w", err)
	}

	result := make([]TranscriptMatch, len(dbMatches))
	for i, m := range dbMatches {
		result[i] = TranscriptMatch{
			TaskID:    m.TaskID,
			Phase:     m.Phase,
			SessionID: m.SessionID,
			Snippet:   m.Snippet,
			Rank:      m.Rank,
		}
	}
	return result, nil
}
