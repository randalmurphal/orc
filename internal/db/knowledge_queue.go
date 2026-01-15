package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// KnowledgeStatus represents the status of a queued knowledge entry.
type KnowledgeStatus string

const (
	KnowledgePending  KnowledgeStatus = "pending"
	KnowledgeApproved KnowledgeStatus = "approved"
	KnowledgeRejected KnowledgeStatus = "rejected"
)

// KnowledgeType represents the type of knowledge entry.
type KnowledgeType string

const (
	KnowledgePattern  KnowledgeType = "pattern"
	KnowledgeGotcha   KnowledgeType = "gotcha"
	KnowledgeDecision KnowledgeType = "decision"
)

// KnowledgeScope represents the scope of knowledge (project or global).
type KnowledgeScope string

const (
	KnowledgeScopeProject KnowledgeScope = "project"
	KnowledgeScopeGlobal  KnowledgeScope = "global"
)

// KnowledgeEntry represents a knowledge item in the queue.
type KnowledgeEntry struct {
	ID             string          `json:"id"`
	Type           KnowledgeType   `json:"type"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Scope          KnowledgeScope  `json:"scope"`
	SourceTask     string          `json:"source_task,omitempty"`
	Status         KnowledgeStatus `json:"status"`
	ProposedBy     string          `json:"proposed_by,omitempty"`
	ProposedAt     time.Time       `json:"proposed_at"`
	ApprovedBy     string          `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time      `json:"approved_at,omitempty"`
	RejectedReason string          `json:"rejected_reason,omitempty"`
	ValidatedAt    *time.Time      `json:"validated_at,omitempty"`
	ValidatedBy    string          `json:"validated_by,omitempty"`
}

// IsStale returns true if the entry hasn't been validated within the threshold.
func (k *KnowledgeEntry) IsStale(stalenessDays int) bool {
	if k.Status != KnowledgeApproved {
		return false
	}

	// Use validated_at if available, otherwise approved_at
	checkTime := k.ValidatedAt
	if checkTime == nil {
		checkTime = k.ApprovedAt
	}
	if checkTime == nil {
		return true // No timestamp means stale
	}

	threshold := time.Now().AddDate(0, 0, -stalenessDays)
	return checkTime.Before(threshold)
}

// QueueKnowledge adds a new knowledge entry to the queue.
func (p *ProjectDB) QueueKnowledge(ktype KnowledgeType, name, description, sourceTask, proposedBy string) (*KnowledgeEntry, error) {
	id := "K-" + uuid.New().String()[:8]

	_, err := p.Exec(`
		INSERT INTO knowledge_queue (id, type, name, description, scope, source_task, proposed_by, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, ktype, name, description, KnowledgeScopeProject, sourceTask, proposedBy, KnowledgePending)
	if err != nil {
		return nil, fmt.Errorf("queue knowledge: %w", err)
	}

	return p.GetKnowledgeEntry(id)
}

// GetKnowledgeEntry retrieves a knowledge entry by ID.
func (p *ProjectDB) GetKnowledgeEntry(id string) (*KnowledgeEntry, error) {
	row := p.QueryRow(`
		SELECT id, type, name, description, scope, source_task, status,
		       proposed_by, proposed_at, approved_by, approved_at, rejected_reason,
		       validated_at, validated_by
		FROM knowledge_queue WHERE id = ?
	`, id)

	return scanKnowledgeEntry(row)
}

// ListPendingKnowledge returns all pending knowledge entries.
func (p *ProjectDB) ListPendingKnowledge() ([]*KnowledgeEntry, error) {
	rows, err := p.Query(`
		SELECT id, type, name, description, scope, source_task, status,
		       proposed_by, proposed_at, approved_by, approved_at, rejected_reason,
		       validated_at, validated_by
		FROM knowledge_queue
		WHERE status = ?
		ORDER BY proposed_at ASC
	`, KnowledgePending)
	if err != nil {
		return nil, fmt.Errorf("list pending knowledge: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanKnowledgeEntries(rows)
}

// ListKnowledgeByType returns all entries of a specific type.
func (p *ProjectDB) ListKnowledgeByType(ktype KnowledgeType, status KnowledgeStatus) ([]*KnowledgeEntry, error) {
	rows, err := p.Query(`
		SELECT id, type, name, description, scope, source_task, status,
		       proposed_by, proposed_at, approved_by, approved_at, rejected_reason,
		       validated_at, validated_by
		FROM knowledge_queue
		WHERE type = ? AND status = ?
		ORDER BY proposed_at ASC
	`, ktype, status)
	if err != nil {
		return nil, fmt.Errorf("list knowledge by type: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanKnowledgeEntries(rows)
}

// ListKnowledgeByTask returns all entries from a specific task.
func (p *ProjectDB) ListKnowledgeByTask(taskID string) ([]*KnowledgeEntry, error) {
	rows, err := p.Query(`
		SELECT id, type, name, description, scope, source_task, status,
		       proposed_by, proposed_at, approved_by, approved_at, rejected_reason,
		       validated_at, validated_by
		FROM knowledge_queue
		WHERE source_task = ?
		ORDER BY proposed_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list knowledge by task: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanKnowledgeEntries(rows)
}

// CountPendingKnowledge returns the count of pending knowledge entries.
func (p *ProjectDB) CountPendingKnowledge() (int, error) {
	var count int
	err := p.QueryRow(`
		SELECT COUNT(*) FROM knowledge_queue WHERE status = ?
	`, KnowledgePending).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending knowledge: %w", err)
	}
	return count, nil
}

// ApproveKnowledge marks a knowledge entry as approved.
func (p *ProjectDB) ApproveKnowledge(id, approvedBy string) (*KnowledgeEntry, error) {
	result, err := p.Exec(`
		UPDATE knowledge_queue
		SET status = ?, approved_by = ?, approved_at = datetime('now')
		WHERE id = ? AND status = ?
	`, KnowledgeApproved, approvedBy, id, KnowledgePending)
	if err != nil {
		return nil, fmt.Errorf("approve knowledge: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("knowledge %s not found or already processed", id)
	}

	return p.GetKnowledgeEntry(id)
}

// RejectKnowledge marks a knowledge entry as rejected with a reason.
func (p *ProjectDB) RejectKnowledge(id, reason string) error {
	result, err := p.Exec(`
		UPDATE knowledge_queue
		SET status = ?, rejected_reason = ?
		WHERE id = ? AND status = ?
	`, KnowledgeRejected, reason, id, KnowledgePending)
	if err != nil {
		return fmt.Errorf("reject knowledge: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("knowledge %s not found or already processed", id)
	}

	return nil
}

// ApproveAllPending approves all pending knowledge entries.
func (p *ProjectDB) ApproveAllPending(approvedBy string) (int, error) {
	result, err := p.Exec(`
		UPDATE knowledge_queue
		SET status = ?, approved_by = ?, approved_at = datetime('now')
		WHERE status = ?
	`, KnowledgeApproved, approvedBy, KnowledgePending)
	if err != nil {
		return 0, fmt.Errorf("approve all knowledge: %w", err)
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// DeleteKnowledge removes a knowledge entry from the queue.
func (p *ProjectDB) DeleteKnowledge(id string) error {
	_, err := p.Exec(`DELETE FROM knowledge_queue WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete knowledge: %w", err)
	}
	return nil
}

// ValidateKnowledge marks a knowledge entry as validated (still relevant).
func (p *ProjectDB) ValidateKnowledge(id, validatedBy string) (*KnowledgeEntry, error) {
	result, err := p.Exec(`
		UPDATE knowledge_queue
		SET validated_at = datetime('now'), validated_by = ?
		WHERE id = ? AND status = ?
	`, validatedBy, id, KnowledgeApproved)
	if err != nil {
		return nil, fmt.Errorf("validate knowledge: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("knowledge %s not found or not approved", id)
	}

	return p.GetKnowledgeEntry(id)
}

// ListStaleKnowledge returns approved entries that haven't been validated within the threshold.
func (p *ProjectDB) ListStaleKnowledge(stalenessDays int) ([]*KnowledgeEntry, error) {
	// Get all approved entries and filter in Go (SQLite date arithmetic is limited)
	rows, err := p.Query(`
		SELECT id, type, name, description, scope, source_task, status,
		       proposed_by, proposed_at, approved_by, approved_at, rejected_reason,
		       validated_at, validated_by
		FROM knowledge_queue
		WHERE status = ?
		ORDER BY validated_at ASC NULLS FIRST, approved_at ASC
	`, KnowledgeApproved)
	if err != nil {
		return nil, fmt.Errorf("list stale knowledge: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries, err := scanKnowledgeEntries(rows)
	if err != nil {
		return nil, err
	}

	// Filter to stale entries
	var stale []*KnowledgeEntry
	for _, e := range entries {
		if e.IsStale(stalenessDays) {
			stale = append(stale, e)
		}
	}

	return stale, nil
}

// CountStaleKnowledge returns the count of stale approved knowledge entries.
func (p *ProjectDB) CountStaleKnowledge(stalenessDays int) (int, error) {
	stale, err := p.ListStaleKnowledge(stalenessDays)
	if err != nil {
		return 0, err
	}
	return len(stale), nil
}

// scanKnowledgeEntry scans a single row into a KnowledgeEntry.
func scanKnowledgeEntry(row *sql.Row) (*KnowledgeEntry, error) {
	var k KnowledgeEntry
	var proposedAt string
	var approvedAt, validatedAt sql.NullString
	var sourceTask, proposedBy, approvedBy, rejectedReason, validatedBy sql.NullString

	err := row.Scan(
		&k.ID, &k.Type, &k.Name, &k.Description, &k.Scope, &sourceTask,
		&k.Status, &proposedBy, &proposedAt, &approvedBy, &approvedAt, &rejectedReason,
		&validatedAt, &validatedBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan knowledge: %w", err)
	}

	k.SourceTask = sourceTask.String
	k.ProposedBy = proposedBy.String
	k.ApprovedBy = approvedBy.String
	k.RejectedReason = rejectedReason.String
	k.ValidatedBy = validatedBy.String

	if t, err := time.Parse("2006-01-02 15:04:05", proposedAt); err == nil {
		k.ProposedAt = t
	}
	if approvedAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", approvedAt.String); err == nil {
			k.ApprovedAt = &t
		}
	}
	if validatedAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", validatedAt.String); err == nil {
			k.ValidatedAt = &t
		}
	}

	return &k, nil
}

// scanKnowledgeEntries scans multiple rows into KnowledgeEntries.
func scanKnowledgeEntries(rows *sql.Rows) ([]*KnowledgeEntry, error) {
	var entries []*KnowledgeEntry

	for rows.Next() {
		var k KnowledgeEntry
		var proposedAt string
		var approvedAt, validatedAt sql.NullString
		var sourceTask, proposedBy, approvedBy, rejectedReason, validatedBy sql.NullString

		err := rows.Scan(
			&k.ID, &k.Type, &k.Name, &k.Description, &k.Scope, &sourceTask,
			&k.Status, &proposedBy, &proposedAt, &approvedBy, &approvedAt, &rejectedReason,
			&validatedAt, &validatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan knowledge: %w", err)
		}

		k.SourceTask = sourceTask.String
		k.ProposedBy = proposedBy.String
		k.ApprovedBy = approvedBy.String
		k.RejectedReason = rejectedReason.String
		k.ValidatedBy = validatedBy.String

		if t, err := time.Parse("2006-01-02 15:04:05", proposedAt); err == nil {
			k.ProposedAt = t
		}
		if approvedAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", approvedAt.String); err == nil {
				k.ApprovedAt = &t
			}
		}
		if validatedAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", validatedAt.String); err == nil {
				k.ValidatedAt = &t
			}
		}

		entries = append(entries, &k)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate knowledge: %w", err)
	}

	return entries, nil
}
