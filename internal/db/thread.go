package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
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
	thread.TaskID = threadAssociationTarget(thread, ThreadLinkTypeTask)
	thread.InitiativeID = threadAssociationTarget(thread, ThreadLinkTypeInitiative)
	thread.RecommendationDrafts = recommendationDrafts
	thread.DecisionDrafts = decisionDrafts
	return thread, nil
}

// GetThreadMessages retrieves all messages for a thread ordered by creation time.
func (p *ProjectDB) GetThreadMessages(threadID string) ([]ThreadMessage, error) {
	rows, err := p.Query(threadMessagesQuery(p.Dialect()), threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	messages := make([]ThreadMessage, 0)
	for rows.Next() {
		message, err := scanThreadMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread messages: %w", err)
	}
	return messages, nil
}

// GetThreadLinks retrieves all typed links for a thread ordered by creation time.
func (p *ProjectDB) GetThreadLinks(threadID string) ([]ThreadLink, error) {
	rows, err := p.Query(threadLinksQuery(p.Dialect()), threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	links := make([]ThreadLink, 0)
	for rows.Next() {
		link, err := scanThreadLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, *link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread links: %w", err)
	}

	recommendationLinks, err := p.syntheticRecommendationLinks(threadID)
	if err != nil {
		return nil, err
	}
	return mergeThreadLinks(links, recommendationLinks), nil
}

// GetThreadRecommendationDrafts retrieves recommendation drafts for a thread.
func (p *ProjectDB) GetThreadRecommendationDrafts(threadID string) ([]ThreadRecommendationDraft, error) {
	rows, err := p.Query(threadRecommendationDraftsQuery(p.Dialect()), threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread recommendation drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	drafts := make([]ThreadRecommendationDraft, 0)
	for rows.Next() {
		draft, err := scanThreadRecommendationDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread recommendation drafts: %w", err)
	}
	return drafts, nil
}

// GetThreadDecisionDrafts retrieves decision drafts for a thread.
func (p *ProjectDB) GetThreadDecisionDrafts(threadID string) ([]ThreadDecisionDraft, error) {
	rows, err := p.Query(threadDecisionDraftsQuery(p.Dialect()), threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread decision drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	drafts := make([]ThreadDecisionDraft, 0)
	for rows.Next() {
		draft, err := scanThreadDecisionDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, *draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread decision drafts: %w", err)
	}
	return drafts, nil
}

// ListThreads returns threads matching the given filters.
func (p *ProjectDB) ListThreads(opts ThreadListOpts) ([]Thread, error) {
	query := threadSelectQuery(p.Dialect(), true)
	args := make([]any, 0, 4)
	argIndex := 1

	if opts.Status != "" {
		query += fmt.Sprintf(" AND status = %s", dialectPlaceholder(p.Dialect(), argIndex))
		args = append(args, opts.Status)
		argIndex++
	}
	if opts.TaskID != "" {
		query += fmt.Sprintf(
			" AND (task_id = %s OR EXISTS (SELECT 1 FROM thread_links WHERE thread_id = threads.id AND link_type = '%s' AND target_id = %s))",
			dialectPlaceholder(p.Dialect(), argIndex),
			ThreadLinkTypeTask,
			dialectPlaceholder(p.Dialect(), argIndex+1),
		)
		args = append(args, opts.TaskID, opts.TaskID)
		argIndex += 2
	}
	if opts.InitiativeID != "" {
		query += fmt.Sprintf(
			" AND (initiative_id = %s OR EXISTS (SELECT 1 FROM thread_links WHERE thread_id = threads.id AND link_type = '%s' AND target_id = %s))",
			dialectPlaceholder(p.Dialect(), argIndex),
			ThreadLinkTypeInitiative,
			dialectPlaceholder(p.Dialect(), argIndex+1),
		)
		args = append(args, opts.InitiativeID, opts.InitiativeID)
		argIndex += 2
	}
	query += " ORDER BY updated_at DESC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", dialectPlaceholder(p.Dialect(), argIndex))
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

// AddThreadMessage adds a message to a thread.
func (p *ProjectDB) AddThreadMessage(msg *ThreadMessage) error {
	if msg == nil {
		return fmt.Errorf("thread message is required")
	}
	now := time.Now().UTC()
	msg.CreatedAt = now

	result, err := p.Exec(threadMessageInsertQuery(p.Dialect()), msg.ThreadID, msg.Role, msg.Content, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert thread message: %w", err)
	}
	if id, err := result.LastInsertId(); err == nil {
		msg.ID = id
	}

	if err := p.touchThread(msg.ThreadID, now); err != nil {
		return err
	}
	return nil
}

// CreateThreadLink adds a typed link to a thread and touches thread updated_at.
func (p *ProjectDB) CreateThreadLink(link *ThreadLink) error {
	if err := validateThreadLink(link); err != nil {
		return err
	}

	now := time.Now().UTC()
	link.CreatedAt = now
	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		if err := createThreadLinkTx(tx, link); err != nil {
			return err
		}
		return touchThreadTx(tx, link.ThreadID, now)
	})
}

// CreateThreadRecommendationDraft persists a recommendation draft shaped in a thread.
func (p *ProjectDB) CreateThreadRecommendationDraft(draft *ThreadRecommendationDraft) error {
	if err := validateThreadRecommendationDraft(draft); err != nil {
		return err
	}
	if draft.ID == "" {
		id, err := p.getNextThreadRecommendationDraftID(context.Background())
		if err != nil {
			return err
		}
		draft.ID = id
	}

	now := time.Now().UTC()
	draft.Status = ThreadDraftStatusDraft
	draft.CreatedAt = now
	draft.UpdatedAt = now

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		if err := insertThreadRecommendationDraftTx(tx, draft); err != nil {
			return err
		}
		return touchThreadTx(tx, draft.ThreadID, now)
	})
}

// CreateThreadDecisionDraft persists a decision draft shaped in a thread.
func (p *ProjectDB) CreateThreadDecisionDraft(draft *ThreadDecisionDraft) error {
	if err := validateThreadDecisionDraft(draft); err != nil {
		return err
	}
	if draft.ID == "" {
		id, err := p.getNextThreadDecisionDraftID(context.Background())
		if err != nil {
			return err
		}
		draft.ID = id
	}

	now := time.Now().UTC()
	draft.Status = ThreadDraftStatusDraft
	draft.CreatedAt = now
	draft.UpdatedAt = now

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		if err := insertThreadDecisionDraftTx(tx, draft); err != nil {
			return err
		}
		return touchThreadTx(tx, draft.ThreadID, now)
	})
}

// PromoteThreadRecommendationDraft promotes a thread draft into a persisted recommendation.
func (p *ProjectDB) PromoteThreadRecommendationDraft(ctx context.Context, threadID string, draftID string, promotedBy string) (*ThreadRecommendationDraft, *Recommendation, error) {
	if strings.TrimSpace(threadID) == "" {
		return nil, nil, fmt.Errorf("thread id is required")
	}
	if strings.TrimSpace(draftID) == "" {
		return nil, nil, fmt.Errorf("draft id is required")
	}
	if strings.TrimSpace(promotedBy) == "" {
		return nil, nil, fmt.Errorf("promoted_by is required")
	}

	recommendationID, err := p.GetNextRecommendationID(ctx)
	if err != nil {
		return nil, nil, err
	}

	var promotedDraft *ThreadRecommendationDraft
	var promotedRecommendation *Recommendation
	err = p.RunInTx(ctx, func(tx *TxOps) error {
		draft, err := getThreadRecommendationDraftTx(tx, draftID)
		if err != nil {
			return err
		}
		if draft.ThreadID != threadID {
			return fmt.Errorf("recommendation draft %s does not belong to thread %s", draftID, threadID)
		}
		if draft.Status == ThreadDraftStatusPromoted {
			return fmt.Errorf("recommendation draft %s already promoted", draftID)
		}

		thread, err := getThreadTx(tx, draft.ThreadID)
		if err != nil {
			return err
		}

		sourceTaskID := draft.SourceTaskID
		if sourceTaskID == "" {
			sourceTaskID = thread.TaskID
		}
		sourceRunID := draft.SourceRunID
		if sourceRunID == "" && sourceTaskID != "" {
			sourceRunID, err = latestWorkflowRunIDForTaskTx(tx, sourceTaskID)
			if err != nil {
				return err
			}
		}

		recommendation := &Recommendation{
			ID:             recommendationID,
			Kind:           draft.Kind,
			Status:         RecommendationStatusPending,
			Title:          draft.Title,
			Summary:        draft.Summary,
			ProposedAction: draft.ProposedAction,
			Evidence:       draft.Evidence,
			SourceTaskID:   sourceTaskID,
			SourceRunID:    sourceRunID,
			SourceThreadID: draft.ThreadID,
			DedupeKey:      recommendationDedupeKey(draft),
		}
		if err := createRecommendationTx(tx, recommendation); err != nil {
			return err
		}
		if err := insertRecommendationHistoryTx(tx, p.Driver(), &RecommendationHistory{
			RecommendationID: recommendation.ID,
			ToStatus:         RecommendationStatusPending,
		}); err != nil {
			return err
		}

		now := time.Now().UTC()
		if err := markThreadRecommendationDraftPromotedTx(tx, draft.ID, recommendation.ID, promotedBy, now); err != nil {
			return err
		}
		if err := touchThreadTx(tx, draft.ThreadID, now); err != nil {
			return err
		}

		promotedDraft, err = getThreadRecommendationDraftTx(tx, draft.ID)
		if err != nil {
			return err
		}
		promotedRecommendation, err = getRecommendationTx(tx, recommendation.ID)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return promotedDraft, promotedRecommendation, nil
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

func createThreadLinkTx(tx *TxOps, link *ThreadLink) error {
	if err := validateThreadLink(link); err != nil {
		return err
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}
	result, err := tx.Exec(threadLinkInsertQuery(tx.Dialect()), link.ThreadID, link.LinkType, link.TargetID, link.Title, link.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert thread link: %w", err)
	}
	if rows, err := result.RowsAffected(); err == nil && rows == 0 {
		existing, getErr := getThreadLinkTx(tx, link.ThreadID, link.LinkType, link.TargetID)
		if getErr != nil {
			return getErr
		}
		*link = *existing
		return nil
	}
	if id, err := result.LastInsertId(); err == nil {
		link.ID = id
		return nil
	}
	existing, err := getThreadLinkTx(tx, link.ThreadID, link.LinkType, link.TargetID)
	if err != nil {
		return err
	}
	*link = *existing
	return nil
}

func insertThreadRecommendationDraftTx(tx *TxOps, draft *ThreadRecommendationDraft) error {
	_, err := tx.Exec(threadRecommendationDraftInsertQuery(tx.Dialect()), draft.ID, draft.ThreadID, draft.Kind, draft.Title, draft.Summary, draft.ProposedAction, draft.Evidence, draft.DedupeKey, draft.SourceTaskID, draft.SourceRunID, draft.Status, draft.PromotedRecommendationID, draft.PromotedBy, nullableTime(draft.PromotedAt), draft.CreatedAt, draft.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert thread recommendation draft: %w", err)
	}
	return nil
}

func insertThreadDecisionDraftTx(tx *TxOps, draft *ThreadDecisionDraft) error {
	_, err := tx.Exec(threadDecisionDraftInsertQuery(tx.Dialect()), draft.ID, draft.ThreadID, draft.InitiativeID, draft.Decision, draft.Rationale, draft.Status, draft.PromotedDecisionID, draft.PromotedBy, nullableTime(draft.PromotedAt), draft.CreatedAt, draft.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert thread decision draft: %w", err)
	}
	return nil
}

func createRecommendationTx(tx *TxOps, rec *Recommendation) error {
	if err := validateRecommendationForCreate(rec); err != nil {
		return err
	}
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	query, args := recommendationInsertQuery(tx.Dialect(), txNow(tx.Dialect()), rec)
	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("insert recommendation: %w", err)
	}
	return nil
}

func markThreadRecommendationDraftPromotedTx(tx *TxOps, draftID string, recommendationID string, promotedBy string, promotedAt time.Time) error {
	_, err := tx.Exec(threadRecommendationDraftPromoteQuery(tx.Dialect()), recommendationID, promotedBy, promotedAt, promotedAt, draftID)
	if err != nil {
		return fmt.Errorf("promote thread recommendation draft %s: %w", draftID, err)
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

func getThreadLinkTx(tx *TxOps, threadID string, linkType string, targetID string) (*ThreadLink, error) {
	row := tx.QueryRow(threadLinkByUniqueQuery(tx.Dialect()), threadID, linkType, targetID)
	link, err := scanThreadLink(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thread link %s/%s not found", linkType, targetID)
	}
	if err != nil {
		return nil, err
	}
	return link, nil
}

func getThreadRecommendationDraftTx(tx *TxOps, draftID string) (*ThreadRecommendationDraft, error) {
	row := tx.QueryRow(threadRecommendationDraftByIDQuery(tx.Dialect()), draftID)
	draft, err := scanThreadRecommendationDraft(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("thread recommendation draft %s not found", draftID)
	}
	if err != nil {
		return nil, err
	}
	return draft, nil
}

func latestWorkflowRunIDForTaskTx(tx *TxOps, taskID string) (string, error) {
	row := tx.QueryRow(latestWorkflowRunIDForTaskQuery(tx.Dialect()), taskID)
	var runID sql.NullString
	if err := row.Scan(&runID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("load latest workflow run for task %s: %w", taskID, err)
	}
	return runID.String, nil
}

func (p *ProjectDB) syntheticRecommendationLinks(threadID string) ([]ThreadLink, error) {
	return pSyntheticRecommendationLinks(p, threadID)
}

func pSyntheticRecommendationLinks(pdb *ProjectDB, threadID string) ([]ThreadLink, error) {
	if pdb == nil {
		return nil, nil
	}
	query := fmt.Sprintf(`
		SELECT title, id, created_at
		FROM recommendations
		WHERE source_thread_id = %s
		ORDER BY created_at ASC, id ASC
	`, dialectPlaceholder(pdb.Dialect(), 1))

	rows, err := pdb.Query(query, threadID)
	if err != nil {
		return nil, fmt.Errorf("query recommendation links for thread %s: %w", threadID, err)
	}
	defer func() { _ = rows.Close() }()

	links := make([]ThreadLink, 0)
	for rows.Next() {
		var title string
		var targetID string
		var createdAt any
		if err := rows.Scan(&title, &targetID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan recommendation link: %w", err)
		}
		links = append(links, ThreadLink{
			ThreadID:  threadID,
			LinkType:  ThreadLinkTypeRecommendation,
			TargetID:  targetID,
			Title:     title,
			CreatedAt: timestampOrZero(createdAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation links: %w", err)
	}
	return links, nil
}

func validateThreadLink(link *ThreadLink) error {
	if link == nil {
		return fmt.Errorf("thread link is required")
	}
	if strings.TrimSpace(link.ThreadID) == "" {
		return fmt.Errorf("thread_id is required")
	}
	if !isValidThreadLinkType(link.LinkType) {
		return fmt.Errorf("invalid thread link type %q", link.LinkType)
	}
	if strings.TrimSpace(link.TargetID) == "" {
		return fmt.Errorf("target_id is required")
	}
	return nil
}

func validateThreadRecommendationDraft(draft *ThreadRecommendationDraft) error {
	if draft == nil {
		return fmt.Errorf("thread recommendation draft is required")
	}
	if strings.TrimSpace(draft.ThreadID) == "" {
		return fmt.Errorf("thread_id is required")
	}
	if !isValidRecommendationKind(draft.Kind) {
		return fmt.Errorf("invalid recommendation kind %q", draft.Kind)
	}
	if strings.TrimSpace(draft.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(draft.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if strings.TrimSpace(draft.ProposedAction) == "" {
		return fmt.Errorf("proposed_action is required")
	}
	if strings.TrimSpace(draft.Evidence) == "" {
		return fmt.Errorf("evidence is required")
	}
	return nil
}

func validateThreadDecisionDraft(draft *ThreadDecisionDraft) error {
	if draft == nil {
		return fmt.Errorf("thread decision draft is required")
	}
	if strings.TrimSpace(draft.ThreadID) == "" {
		return fmt.Errorf("thread_id is required")
	}
	if strings.TrimSpace(draft.Decision) == "" {
		return fmt.Errorf("decision is required")
	}
	return nil
}

func isValidThreadLinkType(linkType string) bool {
	switch linkType {
	case ThreadLinkTypeTask, ThreadLinkTypeInitiative, ThreadLinkTypeRecommendation, ThreadLinkTypeFile, ThreadLinkTypeDiff:
		return true
	default:
		return false
	}
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

func threadAssociationTarget(thread *Thread, linkType string) string {
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

func mergeThreadLinks(base []ThreadLink, extra []ThreadLink) []ThreadLink {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	merged := make([]ThreadLink, 0, len(base)+len(extra))
	appendUnique := func(link ThreadLink) {
		key := link.LinkType + "\x00" + link.TargetID
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, link)
	}
	for _, link := range base {
		appendUnique(link)
	}
	for _, link := range extra {
		appendUnique(link)
	}
	return merged
}

func recommendationDedupeKey(draft *ThreadRecommendationDraft) string {
	if draft.DedupeKey != "" {
		return draft.DedupeKey
	}
	var builder strings.Builder
	builder.WriteString("thread:")
	builder.WriteString(strings.ToLower(strings.TrimSpace(draft.ThreadID)))
	builder.WriteString(":")
	builder.WriteString(strings.ToLower(strings.TrimSpace(draft.Kind)))
	builder.WriteString(":")
	builder.WriteString(normalizeThreadDraftToken(draft.Title))
	return builder.String()
}

func normalizeThreadDraftToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		builder.WriteByte('-')
		lastDash = true
	}
	return strings.Trim(builder.String(), "-")
}

func scanThreadRow(scanner interface{ Scan(dest ...any) error }) (*Thread, error) {
	thread := &Thread{}
	var createdAt any
	var updatedAt any
	err := scanner.Scan(&thread.ID, &thread.Title, &thread.Status, &thread.TaskID, &thread.InitiativeID, &thread.SessionID, &thread.FileContext, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	thread.CreatedAt = timestampOrZero(createdAt)
	thread.UpdatedAt = timestampOrZero(updatedAt)
	return thread, nil
}

func scanThreadMessage(scanner interface{ Scan(dest ...any) error }) (*ThreadMessage, error) {
	message := &ThreadMessage{}
	var createdAt any
	if err := scanner.Scan(&message.ID, &message.ThreadID, &message.Role, &message.Content, &createdAt); err != nil {
		return nil, fmt.Errorf("scan thread message: %w", err)
	}
	message.CreatedAt = timestampOrZero(createdAt)
	return message, nil
}

func scanThreadLink(scanner interface{ Scan(dest ...any) error }) (*ThreadLink, error) {
	link := &ThreadLink{}
	var createdAt any
	if err := scanner.Scan(&link.ID, &link.ThreadID, &link.LinkType, &link.TargetID, &link.Title, &createdAt); err != nil {
		return nil, fmt.Errorf("scan thread link: %w", err)
	}
	link.CreatedAt = timestampOrZero(createdAt)
	return link, nil
}

func scanThreadRecommendationDraft(scanner interface{ Scan(dest ...any) error }) (*ThreadRecommendationDraft, error) {
	draft := &ThreadRecommendationDraft{}
	var promotedAt any
	var createdAt any
	var updatedAt any
	if err := scanner.Scan(&draft.ID, &draft.ThreadID, &draft.Kind, &draft.Title, &draft.Summary, &draft.ProposedAction, &draft.Evidence, &draft.DedupeKey, &draft.SourceTaskID, &draft.SourceRunID, &draft.Status, &draft.PromotedRecommendationID, &draft.PromotedBy, &promotedAt, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan thread recommendation draft: %w", err)
	}
	if ts, ok := scannedTimestamp(promotedAt); ok {
		draft.PromotedAt = &ts
	}
	draft.CreatedAt = timestampOrZero(createdAt)
	draft.UpdatedAt = timestampOrZero(updatedAt)
	return draft, nil
}

func scanThreadDecisionDraft(scanner interface{ Scan(dest ...any) error }) (*ThreadDecisionDraft, error) {
	draft := &ThreadDecisionDraft{}
	var promotedAt any
	var createdAt any
	var updatedAt any
	if err := scanner.Scan(&draft.ID, &draft.ThreadID, &draft.InitiativeID, &draft.Decision, &draft.Rationale, &draft.Status, &draft.PromotedDecisionID, &draft.PromotedBy, &promotedAt, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan thread decision draft: %w", err)
	}
	if ts, ok := scannedTimestamp(promotedAt); ok {
		draft.PromotedAt = &ts
	}
	draft.CreatedAt = timestampOrZero(createdAt)
	draft.UpdatedAt = timestampOrZero(updatedAt)
	return draft, nil
}

func threadInsertQuery(dialect driver.Dialect) string {
	return `
		INSERT INTO threads (id, title, status, task_id, initiative_id, session_id, file_context, created_at, updated_at)
		VALUES (` + placeholders(dialect, 1, 9) + `)
	`
}

func threadSelectQuery(dialect driver.Dialect, many bool) string {
	query := `
		SELECT id,
		       title,
		       status,
		       COALESCE(
		           (SELECT target_id
		            FROM thread_links
		            WHERE thread_id = threads.id AND link_type = 'task'
		            ORDER BY created_at ASC, id ASC
		            LIMIT 1),
		           task_id
		       ) AS task_id,
		       COALESCE(
		           (SELECT target_id
		            FROM thread_links
		            WHERE thread_id = threads.id AND link_type = 'initiative'
		            ORDER BY created_at ASC, id ASC
		            LIMIT 1),
		           initiative_id
		       ) AS initiative_id,
		       session_id,
		       file_context,
		       created_at,
		       updated_at
		FROM threads
		WHERE 1=1
	`
	if !many {
		query += " AND id = " + dialectPlaceholder(dialect, 1)
	}
	return query
}

func threadMessagesQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, role, content, created_at
		FROM thread_messages
		WHERE thread_id = ` + dialectPlaceholder(dialect, 1) + `
		ORDER BY created_at ASC, id ASC
	`
}

func threadMessageInsertQuery(dialect driver.Dialect) string {
	return `
		INSERT INTO thread_messages (thread_id, role, content, created_at)
		VALUES (` + placeholders(dialect, 1, 4) + `)
	`
}

func threadTouchQuery(dialect driver.Dialect) string {
	return `
		UPDATE threads
		SET updated_at = ` + dialectPlaceholder(dialect, 1) + `
		WHERE id = ` + dialectPlaceholder(dialect, 2)
}

func threadArchiveQuery(dialect driver.Dialect) string {
	return `
		UPDATE threads
		SET status = 'archived', updated_at = ` + dialectPlaceholder(dialect, 1) + `
		WHERE id = ` + dialectPlaceholder(dialect, 2)
}

func threadDeleteQuery(dialect driver.Dialect) string {
	return `DELETE FROM threads WHERE id = ` + dialectPlaceholder(dialect, 1)
}

func threadSessionUpdateQuery(dialect driver.Dialect) string {
	return `
		UPDATE threads
		SET session_id = ` + dialectPlaceholder(dialect, 1) + `,
		    updated_at = ` + dialectPlaceholder(dialect, 2) + `
		WHERE id = ` + dialectPlaceholder(dialect, 3)
}

func threadLinksQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, link_type, target_id, title, created_at
		FROM thread_links
		WHERE thread_id = ` + dialectPlaceholder(dialect, 1) + `
		ORDER BY created_at ASC, id ASC
	`
}

func threadLinkInsertQuery(dialect driver.Dialect) string {
	return `
		INSERT INTO thread_links (thread_id, link_type, target_id, title, created_at)
		VALUES (` + placeholders(dialect, 1, 5) + `)
		ON CONFLICT(thread_id, link_type, target_id) DO NOTHING
	`
}

func threadLinkByUniqueQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, link_type, target_id, title, created_at
		FROM thread_links
		WHERE thread_id = ` + dialectPlaceholder(dialect, 1) + `
		  AND link_type = ` + dialectPlaceholder(dialect, 2) + `
		  AND target_id = ` + dialectPlaceholder(dialect, 3)
}

func threadRecommendationDraftsQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, kind, title, summary, proposed_action, evidence, dedupe_key,
		       source_task_id, source_run_id, status, promoted_recommendation_id, promoted_by,
		       promoted_at, created_at, updated_at
		FROM thread_recommendation_drafts
		WHERE thread_id = ` + dialectPlaceholder(dialect, 1) + `
		ORDER BY created_at ASC, id ASC
	`
}

func threadRecommendationDraftInsertQuery(dialect driver.Dialect) string {
	return `
		INSERT INTO thread_recommendation_drafts (
			id, thread_id, kind, title, summary, proposed_action, evidence, dedupe_key,
			source_task_id, source_run_id, status, promoted_recommendation_id, promoted_by,
			promoted_at, created_at, updated_at
		)
		VALUES (` + placeholders(dialect, 1, 16) + `)
	`
}

func threadRecommendationDraftByIDQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, kind, title, summary, proposed_action, evidence, dedupe_key,
		       source_task_id, source_run_id, status, promoted_recommendation_id, promoted_by,
		       promoted_at, created_at, updated_at
		FROM thread_recommendation_drafts
		WHERE id = ` + dialectPlaceholder(dialect, 1)
}

func threadRecommendationDraftPromoteQuery(dialect driver.Dialect) string {
	return `
		UPDATE thread_recommendation_drafts
		SET status = 'promoted',
		    promoted_recommendation_id = ` + dialectPlaceholder(dialect, 1) + `,
		    promoted_by = ` + dialectPlaceholder(dialect, 2) + `,
		    promoted_at = ` + dialectPlaceholder(dialect, 3) + `,
		    updated_at = ` + dialectPlaceholder(dialect, 4) + `
		WHERE id = ` + dialectPlaceholder(dialect, 5)
}

func threadDecisionDraftsQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, initiative_id, decision, rationale, status,
		       promoted_decision_id, promoted_by, promoted_at, created_at, updated_at
		FROM thread_decision_drafts
		WHERE thread_id = ` + dialectPlaceholder(dialect, 1) + `
		ORDER BY created_at ASC, id ASC
	`
}

func threadDecisionDraftInsertQuery(dialect driver.Dialect) string {
	return `
		INSERT INTO thread_decision_drafts (
			id, thread_id, initiative_id, decision, rationale, status,
			promoted_decision_id, promoted_by, promoted_at, created_at, updated_at
		)
		VALUES (` + placeholders(dialect, 1, 11) + `)
	`
}

func latestWorkflowRunIDForTaskQuery(dialect driver.Dialect) string {
	return `
		SELECT id
		FROM workflow_runs
		WHERE task_id = ` + dialectPlaceholder(dialect, 1) + `
		ORDER BY created_at DESC
		LIMIT 1
	`
}

func dialectPlaceholder(dialect driver.Dialect, index int) string {
	if dialect == driver.DialectPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func placeholders(dialect driver.Dialect, start int, count int) string {
	values := make([]string, 0, count)
	for i := 0; i < count; i++ {
		values = append(values, dialectPlaceholder(dialect, start+i))
	}
	return strings.Join(values, ", ")
}

func txNow(dialect driver.Dialect) string {
	if dialect == driver.DialectPostgres {
		return "NOW()"
	}
	return "datetime('now')"
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
