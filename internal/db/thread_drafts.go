package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
			sourceTaskID = ThreadAssociationTarget(thread, ThreadLinkTypeTask)
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
	if err := ensureRecommendationDedupeAvailableTx(tx, rec); err != nil {
		return err
	}

	query, args := recommendationInsertQuery(tx.Dialect(), tx.Now(), rec)
	if _, err := tx.Exec(query, args...); err != nil {
		return fmt.Errorf("insert recommendation: %w", err)
	}
	created, err := getRecommendationTx(tx, rec.ID)
	if err != nil {
		return err
	}
	*rec = *created
	return nil
}

func markThreadRecommendationDraftPromotedTx(tx *TxOps, draftID string, recommendationID string, promotedBy string, promotedAt time.Time) error {
	_, err := tx.Exec(threadRecommendationDraftPromoteQuery(tx.Dialect()), recommendationID, promotedBy, promotedAt, promotedAt, draftID)
	if err != nil {
		return fmt.Errorf("promote thread recommendation draft %s: %w", draftID, err)
	}
	return nil
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
