package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	// SeqThread is the sequence name for thread ID generation.
	SeqThread = "thread"
	// SeqThreadRecommendationDraft is the sequence for recommendation draft IDs.
	SeqThreadRecommendationDraft = "thread_recommendation_draft"
	// SeqThreadDecisionDraft is the sequence for decision draft IDs.
	SeqThreadDecisionDraft = "thread_decision_draft"

	ThreadStatusActive   = "active"
	ThreadStatusArchived = "archived"

	ThreadLinkTypeTask           = "task"
	ThreadLinkTypeInitiative     = "initiative"
	ThreadLinkTypeRecommendation = "recommendation"
	ThreadLinkTypeFile           = "file"
	ThreadLinkTypeDiff           = "diff"

	ThreadDraftStatusDraft    = "draft"
	ThreadDraftStatusPromoted = "promoted"
)

// Thread represents a conversation thread stored in the database.
type Thread struct {
	ID                   string
	Title                string
	Status               string
	TaskID               string
	InitiativeID         string
	SessionID            string
	FileContext          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Messages             []ThreadMessage
	Links                []ThreadLink
	RecommendationDrafts []ThreadRecommendationDraft
	DecisionDrafts       []ThreadDecisionDraft
}

// ThreadMessage represents a single message within a thread.
type ThreadMessage struct {
	ID        int64
	ThreadID  string
	Role      string
	Content   string
	CreatedAt time.Time
}

// ThreadLink represents typed context linked to a discussion thread.
type ThreadLink struct {
	ID        int64
	ThreadID  string
	LinkType  string
	TargetID  string
	Title     string
	CreatedAt time.Time
}

// ThreadRecommendationDraft captures a recommendation draft shaped in a thread.
type ThreadRecommendationDraft struct {
	ID                       string
	ThreadID                 string
	Kind                     string
	Title                    string
	Summary                  string
	ProposedAction           string
	Evidence                 string
	DedupeKey                string
	SourceTaskID             string
	SourceRunID              string
	Status                   string
	PromotedRecommendationID string
	PromotedBy               string
	PromotedAt               *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// ThreadDecisionDraft captures an initiative decision draft shaped in a thread.
type ThreadDecisionDraft struct {
	ID                 string
	ThreadID           string
	InitiativeID       string
	Decision           string
	Rationale          string
	Status             string
	PromotedDecisionID string
	PromotedBy         string
	PromotedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ThreadListOpts controls filtering for ListThreads.
type ThreadListOpts struct {
	Status       string
	TaskID       string
	InitiativeID string
	Limit        int
}

// GetNextThreadID generates the next sequential thread ID (THR-001, THR-002, ...).
func (p *ProjectDB) GetNextThreadID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqThread)
	if err != nil {
		return "", fmt.Errorf("get next thread sequence: %w", err)
	}
	return fmt.Sprintf("THR-%03d", num), nil
}

func (p *ProjectDB) getNextThreadRecommendationDraftID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqThreadRecommendationDraft)
	if err != nil {
		return "", fmt.Errorf("get next thread recommendation draft sequence: %w", err)
	}
	return fmt.Sprintf("TRD-%03d", num), nil
}

func (p *ProjectDB) getNextThreadDecisionDraftID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqThreadDecisionDraft)
	if err != nil {
		return "", fmt.Errorf("get next thread decision draft sequence: %w", err)
	}
	return fmt.Sprintf("TDD-%03d", num), nil
}

// CreateThread persists a new thread with any initial typed links.
func (p *ProjectDB) CreateThread(t *Thread) error {
	if t == nil {
		return fmt.Errorf("thread is required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("thread title is required")
	}

	id, err := p.GetNextThreadID(context.Background())
	if err != nil {
		return fmt.Errorf("generate thread id: %w", err)
	}

	now := time.Now().UTC()
	t.ID = id
	t.Status = ThreadStatusActive
	t.CreatedAt = now
	t.UpdatedAt = now

	initialLinks := mergeThreadLinks(threadInitialLinks(t), t.Links)
	if err := validateThreadAssociationLinks(initialLinks); err != nil {
		return fmt.Errorf("validate thread links: %w", err)
	}
	if err := p.RunInTx(context.Background(), func(tx *TxOps) error {
		if err := insertThreadTx(tx, t); err != nil {
			return err
		}
		for i := range initialLinks {
			initialLinks[i].ThreadID = t.ID
			if err := createThreadLinkTx(tx, &initialLinks[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("create thread %s: %w", t.Title, err)
	}

	t.Links = initialLinks
	return nil
}

// GetThread retrieves a thread by ID, including messages and persisted context.
func (p *ProjectDB) GetThread(id string) (*Thread, error) {
	row := p.QueryRow(threadSelectQuery(p.Dialect(), false), id)

	thread, err := scanThreadRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get thread %s: %w", id, err)
	}

	messages, err := p.GetThreadMessages(id)
	if err != nil {
		return nil, fmt.Errorf("get thread messages for %s: %w", id, err)
	}
	links, err := p.GetThreadLinks(id)
	if err != nil {
		return nil, fmt.Errorf("get thread links for %s: %w", id, err)
	}
	recommendationDrafts, err := p.GetThreadRecommendationDrafts(id)
	if err != nil {
		return nil, fmt.Errorf("get recommendation drafts for %s: %w", id, err)
	}
	decisionDrafts, err := p.GetThreadDecisionDrafts(id)
	if err != nil {
		return nil, fmt.Errorf("get decision drafts for %s: %w", id, err)
	}

	thread.Messages = messages
	thread.Links = mergeThreadLinks(threadLegacyLinks(thread), links)
	thread.TaskID = ThreadAssociationTarget(thread, ThreadLinkTypeTask)
	thread.InitiativeID = ThreadAssociationTarget(thread, ThreadLinkTypeInitiative)
	thread.RecommendationDrafts = recommendationDrafts
	thread.DecisionDrafts = decisionDrafts
	return thread, nil
}

// ListThreads returns threads matching the given filters.
func (p *ProjectDB) ListThreads(opts ThreadListOpts) ([]Thread, error) {
	query := threadSelectQuery(p.Dialect(), true)
	args := make([]any, 0, 4)
	argIndex := 1

	if opts.Status != "" {
		query += fmt.Sprintf(" AND status = %s", placeholderForDialect(p.Dialect(), argIndex))
		args = append(args, opts.Status)
		argIndex++
	}
	if opts.TaskID != "" {
		query += fmt.Sprintf(
			" AND (task_id = %s OR EXISTS (SELECT 1 FROM thread_links WHERE thread_id = threads.id AND link_type = '%s' AND target_id = %s))",
			placeholderForDialect(p.Dialect(), argIndex),
			ThreadLinkTypeTask,
			placeholderForDialect(p.Dialect(), argIndex+1),
		)
		args = append(args, opts.TaskID, opts.TaskID)
		argIndex += 2
	}
	if opts.InitiativeID != "" {
		query += fmt.Sprintf(
			" AND (initiative_id = %s OR EXISTS (SELECT 1 FROM thread_links WHERE thread_id = threads.id AND link_type = '%s' AND target_id = %s))",
			placeholderForDialect(p.Dialect(), argIndex),
			ThreadLinkTypeInitiative,
			placeholderForDialect(p.Dialect(), argIndex+1),
		)
		args = append(args, opts.InitiativeID, opts.InitiativeID)
		argIndex += 2
	}
	query += " ORDER BY updated_at DESC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", placeholderForDialect(p.Dialect(), argIndex))
		args = append(args, opts.Limit)
	}

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	threads := make([]Thread, 0)
	for rows.Next() {
		thread, err := scanThreadRow(rows)
		if err != nil {
			return nil, err
		}
		threads = append(threads, *thread)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate threads: %w", err)
	}
	return threads, nil
}

// CountActiveThreads returns the number of non-archived discussion threads.
func (p *ProjectDB) CountActiveThreads() (int, error) {
	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM threads WHERE status = %s",
		p.Placeholder(1),
	)

	var count int
	if err := p.QueryRow(query, ThreadStatusActive).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active threads: %w", err)
	}

	return count, nil
}

// ArchiveThread sets a thread's status to archived.
func (p *ProjectDB) ArchiveThread(id string) error {
	result, err := p.Exec(threadArchiveQuery(p.Dialect()), time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("archive thread %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check archive rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("thread %s not found", id)
	}
	return nil
}

// DeleteThread removes a thread and all its messages.
func (p *ProjectDB) DeleteThread(id string) error {
	result, err := p.Exec(threadDeleteQuery(p.Dialect()), id)
	if err != nil {
		return fmt.Errorf("delete thread %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("thread %s not found", id)
	}
	return nil
}

// UpdateThreadSessionID updates the session ID for a thread.
func (p *ProjectDB) UpdateThreadSessionID(id, sessionID string) error {
	_, err := p.Exec(threadSessionUpdateQuery(p.Dialect()), sessionID, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update thread session_id %s: %w", id, err)
	}
	return nil
}

func (p *ProjectDB) touchThread(threadID string, updatedAt time.Time) error {
	_, err := p.Exec(threadTouchQuery(p.Dialect()), updatedAt, threadID)
	if err != nil {
		return fmt.Errorf("update thread updated_at: %w", err)
	}
	return nil
}

func insertThreadTx(tx *TxOps, thread *Thread) error {
	_, err := tx.Exec(threadInsertQuery(tx.Dialect()), thread.ID, thread.Title, thread.Status, thread.TaskID, thread.InitiativeID, thread.SessionID, thread.FileContext, thread.CreatedAt, thread.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert thread: %w", err)
	}
	return nil
}

func touchThreadTx(tx *TxOps, threadID string, updatedAt time.Time) error {
	_, err := tx.Exec(threadTouchQuery(tx.Dialect()), updatedAt, threadID)
	if err != nil {
		return fmt.Errorf("update thread updated_at: %w", err)
	}
	return nil
}

func getThreadTx(tx *TxOps, id string) (*Thread, error) {
	row := tx.QueryRow(threadSelectQuery(tx.Dialect(), false), id)
	thread, err := scanThreadRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thread %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	return thread, nil
}

func threadInitialLinks(thread *Thread) []ThreadLink {
	links := make([]ThreadLink, 0)
	if thread.TaskID != "" {
		links = append(links, ThreadLink{LinkType: ThreadLinkTypeTask, TargetID: thread.TaskID, Title: thread.TaskID})
	}
	if thread.InitiativeID != "" {
		links = append(links, ThreadLink{LinkType: ThreadLinkTypeInitiative, TargetID: thread.InitiativeID, Title: thread.InitiativeID})
	}
	links = append(links, threadLegacyFileLinks(thread)...)
	return mergeThreadLinks(nil, links)
}

func threadLegacyLinks(thread *Thread) []ThreadLink {
	if thread == nil {
		return nil
	}
	return threadInitialLinks(thread)
}

func threadLegacyFileLinks(thread *Thread) []ThreadLink {
	if thread == nil || thread.FileContext == "" {
		return nil
	}
	var files []string
	if err := json.Unmarshal([]byte(thread.FileContext), &files); err == nil {
		links := make([]ThreadLink, 0, len(files))
		for _, file := range files {
			if strings.TrimSpace(file) == "" {
				continue
			}
			links = append(links, ThreadLink{LinkType: ThreadLinkTypeFile, TargetID: file, Title: file})
		}
		return links
	}
	return []ThreadLink{{LinkType: ThreadLinkTypeFile, TargetID: thread.FileContext, Title: thread.FileContext}}
}

// ThreadAssociationTarget returns the canonical target for a typed thread association.
func ThreadAssociationTarget(thread *Thread, linkType string) string {
	if thread != nil {
		for _, link := range thread.Links {
			if link.LinkType == linkType && strings.TrimSpace(link.TargetID) != "" {
				return link.TargetID
			}
		}
	}
	if thread == nil {
		return ""
	}
	switch linkType {
	case ThreadLinkTypeTask:
		return thread.TaskID
	case ThreadLinkTypeInitiative:
		return thread.InitiativeID
	default:
		return ""
	}
}
