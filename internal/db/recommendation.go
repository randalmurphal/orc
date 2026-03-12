package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

const (
	RecommendationKindCleanup         = "cleanup"
	RecommendationKindRisk            = "risk"
	RecommendationKindFollowUp        = "follow_up"
	RecommendationKindDecisionRequest = "decision_request"
)

const (
	RecommendationStatusPending   = "pending"
	RecommendationStatusAccepted  = "accepted"
	RecommendationStatusRejected  = "rejected"
	RecommendationStatusDiscussed = "discussed"
)

var (
	ErrRecommendationNotFound          = errors.New("recommendation not found")
	ErrInvalidRecommendationTransition = errors.New("invalid recommendation transition")
	ErrRecommendationConflict          = errors.New("recommendation conflict")
)

type Recommendation struct {
	ID             string
	Kind           string
	Status         string
	Title          string
	Summary        string
	ProposedAction string
	Evidence       string
	SourceTaskID   string
	SourceRunID    string
	SourceThreadID string
	DedupeKey      string
	DecidedBy      string
	DecidedAt      *time.Time
	DecisionReason string
	PromotedToType string
	PromotedToID   string
	PromotedBy     string
	PromotedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RecommendationHistory struct {
	ID               int64
	RecommendationID string
	FromStatus       string
	ToStatus         string
	DecidedBy        string
	DecisionReason   string
	CreatedAt        time.Time
}

type RecommendationListOpts struct {
	Status       string
	Kind         string
	SourceTaskID string
	Limit        int
}

func (p *ProjectDB) GetNextRecommendationID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqRecommendation)
	if err != nil {
		return "", fmt.Errorf("get next recommendation sequence: %w", err)
	}
	return fmt.Sprintf("REC-%03d", num), nil
}

func (p *ProjectDB) CreateRecommendation(rec *Recommendation) error {
	if rec == nil {
		return fmt.Errorf("recommendation is required")
	}
	if err := validateRecommendationForCreate(rec); err != nil {
		return err
	}
	if rec.ID == "" {
		id, err := p.GetNextRecommendationID(context.Background())
		if err != nil {
			return err
		}
		rec.ID = id
	}

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		now := p.Driver().Now()
		query, args := recommendationInsertQuery(tx.Dialect(), now, rec)
		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("insert recommendation %s: %w", rec.ID, err)
		}

		if err := insertRecommendationHistoryTx(tx, p.Driver(), &RecommendationHistory{
			RecommendationID: rec.ID,
			ToStatus:         RecommendationStatusPending,
		}); err != nil {
			return err
		}

		created, err := getRecommendationTx(tx, rec.ID)
		if err != nil {
			return err
		}
		*rec = *created
		return nil
	})
}

func (p *ProjectDB) GetRecommendation(id string) (*Recommendation, error) {
	placeholder := p.Placeholder(1)
	rec, err := getRecommendationByQueryRow(
		p.QueryRow(fmt.Sprintf(`
			SELECT id, kind, status, title, summary, proposed_action, evidence,
			       source_task_id, source_run_id, source_thread_id, dedupe_key, decided_by, decided_at,
			       decision_reason, promoted_to_type, promoted_to_id, promoted_by, promoted_at,
			       created_at, updated_at
			FROM recommendations
			WHERE id = %s
		`, placeholder), id),
	)
	if err != nil {
		return nil, fmt.Errorf("get recommendation %s: %w", id, err)
	}
	return rec, nil
}

func (p *ProjectDB) ListRecommendations(opts RecommendationListOpts) ([]Recommendation, error) {
	query, args := recommendationListQuery(p.Dialect(), opts)
	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list recommendations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	recommendations := make([]Recommendation, 0)
	for rows.Next() {
		rec, err := scanRecommendation(rows)
		if err != nil {
			return nil, err
		}
		recommendations = append(recommendations, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendations: %w", err)
	}
	return recommendations, nil
}

func (p *ProjectDB) DeleteRecommendation(id string) error {
	placeholder := p.Placeholder(1)
	_, err := p.Exec(fmt.Sprintf("DELETE FROM recommendations WHERE id = %s", placeholder), id)
	if err != nil {
		return fmt.Errorf("delete recommendation %s: %w", id, err)
	}
	return nil
}

func (p *ProjectDB) ListRecommendationHistory(recommendationID string) ([]RecommendationHistory, error) {
	placeholder := p.Placeholder(1)
	rows, err := p.Query(fmt.Sprintf(`
		SELECT id, recommendation_id, from_status, to_status, decided_by, decision_reason, created_at
		FROM recommendation_history
		WHERE recommendation_id = %s
		ORDER BY created_at DESC, id DESC
	`, placeholder), recommendationID)
	if err != nil {
		return nil, fmt.Errorf("list recommendation history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	history := make([]RecommendationHistory, 0)
	for rows.Next() {
		entry, err := scanRecommendationHistory(rows)
		if err != nil {
			return nil, err
		}
		history = append(history, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recommendation history: %w", err)
	}
	return history, nil
}

func (p *ProjectDB) CountRecommendationsByStatus(status string) (int, error) {
	if status == "" {
		return 0, fmt.Errorf("status is required")
	}
	if !isValidRecommendationStatus(status) {
		return 0, fmt.Errorf("invalid recommendation status %q", status)
	}

	placeholder := p.Placeholder(1)
	row := p.QueryRow(fmt.Sprintf(
		`SELECT COUNT(*) FROM recommendations WHERE status = %s`,
		placeholder,
	), status)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count recommendations by status: %w", err)
	}
	return count, nil
}

func (p *ProjectDB) UpdateRecommendationStatus(
	id string,
	status string,
	decidedBy string,
	decisionReason string,
) (*Recommendation, error) {
	switch status {
	case RecommendationStatusAccepted:
		return p.AcceptRecommendation(id, decidedBy, decisionReason)
	case RecommendationStatusRejected:
		return p.RejectRecommendation(id, decidedBy, decisionReason)
	case RecommendationStatusDiscussed:
		return p.DiscussRecommendation(id, decidedBy, decisionReason)
	default:
		return nil, fmt.Errorf("unsupported recommendation status %q", status)
	}
}

func (p *ProjectDB) AcceptRecommendation(id, decidedBy, decisionReason string) (*Recommendation, error) {
	return p.decideRecommendation(id, RecommendationStatusAccepted, []string{
		RecommendationStatusPending,
		RecommendationStatusDiscussed,
	}, decidedBy, decisionReason)
}

func (p *ProjectDB) RejectRecommendation(id, decidedBy, decisionReason string) (*Recommendation, error) {
	return p.decideRecommendation(id, RecommendationStatusRejected, []string{
		RecommendationStatusPending,
		RecommendationStatusDiscussed,
	}, decidedBy, decisionReason)
}

func (p *ProjectDB) DiscussRecommendation(id, decidedBy, decisionReason string) (*Recommendation, error) {
	return p.decideRecommendation(id, RecommendationStatusDiscussed, []string{
		RecommendationStatusPending,
	}, decidedBy, decisionReason)
}

func (p *ProjectDB) decideRecommendation(
	id string,
	targetStatus string,
	allowedFrom []string,
	decidedBy string,
	decisionReason string,
) (*Recommendation, error) {
	if id == "" {
		return nil, fmt.Errorf("recommendation id is required")
	}
	if decidedBy == "" {
		return nil, fmt.Errorf("decided_by is required")
	}

	returnRec := &Recommendation{}
	err := p.RunInTx(context.Background(), func(tx *TxOps) error {
		current, err := getRecommendationTx(tx, id)
		if err != nil {
			return err
		}
		if !containsRecommendationStatus(allowedFrom, current.Status) {
			if current.Status == targetStatus {
				return fmt.Errorf("%w: recommendation %s already %s", ErrRecommendationConflict, id, targetStatus)
			}
			return fmt.Errorf(
				"%w: %s -> %s",
				ErrInvalidRecommendationTransition,
				current.Status,
				targetStatus,
			)
		}

		query, args := recommendationDecisionUpdateQuery(
			tx.Dialect(),
			p.Driver().Now(),
			id,
			current.Status,
			targetStatus,
			decidedBy,
			decisionReason,
		)
		result, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("update recommendation %s: %w", id, err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check update result for recommendation %s: %w", id, err)
		}
		if rowsAffected == 0 {
			latest, getErr := getRecommendationTx(tx, id)
			if getErr != nil {
				return getErr
			}
			if latest.Status == targetStatus {
				return fmt.Errorf("%w: recommendation %s already %s", ErrRecommendationConflict, id, targetStatus)
			}
			return fmt.Errorf(
				"%w: %s -> %s",
				ErrInvalidRecommendationTransition,
				latest.Status,
				targetStatus,
			)
		}

		updated, err := getRecommendationTx(tx, id)
		if err != nil {
			return err
		}

		if err := insertRecommendationHistoryTx(tx, p.Driver(), &RecommendationHistory{
			RecommendationID: id,
			FromStatus:       current.Status,
			ToStatus:         updated.Status,
			DecidedBy:        updated.DecidedBy,
			DecisionReason:   updated.DecisionReason,
		}); err != nil {
			return err
		}

		*returnRec = *updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returnRec, nil
}

func validateRecommendationForCreate(rec *Recommendation) error {
	if rec.Kind == "" {
		return fmt.Errorf("recommendation kind is required")
	}
	if !isValidRecommendationKind(rec.Kind) {
		return fmt.Errorf("invalid recommendation kind %q", rec.Kind)
	}
	if rec.Status == "" {
		rec.Status = RecommendationStatusPending
	}
	if rec.Status != RecommendationStatusPending {
		return fmt.Errorf("recommendation status must start as pending")
	}
	if rec.Title == "" {
		return fmt.Errorf("recommendation title is required")
	}
	if rec.Summary == "" {
		return fmt.Errorf("recommendation summary is required")
	}
	if rec.ProposedAction == "" {
		return fmt.Errorf("recommendation proposed_action is required")
	}
	if rec.Evidence == "" {
		return fmt.Errorf("recommendation evidence is required")
	}
	hasTaskProvenance := rec.SourceTaskID != "" || rec.SourceRunID != ""
	hasThreadProvenance := rec.SourceThreadID != ""
	if !hasTaskProvenance && !hasThreadProvenance {
		return fmt.Errorf("recommendation provenance requires source_thread_id or source_task_id/source_run_id")
	}
	if rec.SourceRunID != "" && rec.SourceTaskID == "" {
		return fmt.Errorf("recommendation source_task_id is required when source_run_id is set")
	}
	if rec.DedupeKey == "" {
		return fmt.Errorf("recommendation dedupe_key is required")
	}
	return nil
}

func isValidRecommendationKind(kind string) bool {
	switch kind {
	case RecommendationKindCleanup, RecommendationKindRisk, RecommendationKindFollowUp, RecommendationKindDecisionRequest:
		return true
	default:
		return false
	}
}

func isValidRecommendationStatus(status string) bool {
	switch status {
	case RecommendationStatusPending, RecommendationStatusAccepted, RecommendationStatusRejected, RecommendationStatusDiscussed:
		return true
	default:
		return false
	}
}

type recommendationRowScanner interface {
	Scan(dest ...any) error
}

func recommendationInsertQuery(dialect driver.Dialect, now string, rec *Recommendation) (string, []any) {
	if dialect == driver.DialectSQLite {
		return fmt.Sprintf(`
			INSERT INTO recommendations (
				id, kind, status, title, summary, proposed_action, evidence,
				source_task_id, source_run_id, source_thread_id, dedupe_key,
				promoted_to_type, promoted_to_id, promoted_by, promoted_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, now, now), []any{
				rec.ID,
				rec.Kind,
				rec.Status,
				rec.Title,
				rec.Summary,
				rec.ProposedAction,
				rec.Evidence,
				nullableRecommendationValue(rec.SourceTaskID),
				nullableRecommendationValue(rec.SourceRunID),
				nullableRecommendationValue(rec.SourceThreadID),
				rec.DedupeKey,
				nullableRecommendationValue(rec.PromotedToType),
				nullableRecommendationValue(rec.PromotedToID),
				nullableRecommendationValue(rec.PromotedBy),
				nullableRecommendationTime(rec.PromotedAt),
			}
	}

	return fmt.Sprintf(`
		INSERT INTO recommendations (
			id, kind, status, title, summary, proposed_action, evidence,
			source_task_id, source_run_id, source_thread_id, dedupe_key,
			promoted_to_type, promoted_to_id, promoted_by, promoted_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, %s, %s)
	`, now, now), []any{
			rec.ID,
			rec.Kind,
			rec.Status,
			rec.Title,
			rec.Summary,
			rec.ProposedAction,
			rec.Evidence,
			nullableRecommendationValue(rec.SourceTaskID),
			nullableRecommendationValue(rec.SourceRunID),
			nullableRecommendationValue(rec.SourceThreadID),
			rec.DedupeKey,
			nullableRecommendationValue(rec.PromotedToType),
			nullableRecommendationValue(rec.PromotedToID),
			nullableRecommendationValue(rec.PromotedBy),
			nullableRecommendationTime(rec.PromotedAt),
		}
}

func recommendationListQuery(dialect driver.Dialect, opts RecommendationListOpts) (string, []any) {
	baseQuery := `
		SELECT id, kind, status, title, summary, proposed_action, evidence,
		       source_task_id, source_run_id, source_thread_id, dedupe_key, decided_by, decided_at,
		       decision_reason, promoted_to_type, promoted_to_id, promoted_by, promoted_at,
		       created_at, updated_at
		FROM recommendations
	`

	where := make([]string, 0, 3)
	args := make([]any, 0, 3)
	index := 1
	appendFilter := func(column string, value string) {
		where = append(where, fmt.Sprintf("%s = %s", column, placeholderForDialect(dialect, index)))
		args = append(args, value)
		index++
	}

	if opts.Status != "" {
		appendFilter("status", opts.Status)
	}
	if opts.Kind != "" {
		appendFilter("kind", opts.Kind)
	}
	if opts.SourceTaskID != "" {
		appendFilter("source_task_id", opts.SourceTaskID)
	}

	query := baseQuery
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC, id DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", placeholderForDialect(dialect, index))
		args = append(args, opts.Limit)
	}

	return query, args
}

func recommendationDecisionUpdateQuery(
	dialect driver.Dialect,
	now string,
	id string,
	currentStatus string,
	targetStatus string,
	decidedBy string,
	decisionReason string,
) (string, []any) {
	args := []any{targetStatus, decidedBy, decisionReason, id, currentStatus}

	if dialect == driver.DialectSQLite {
		return fmt.Sprintf(`
			UPDATE recommendations
			SET status = ?, decided_by = ?, decided_at = %s, decision_reason = ?, updated_at = %s
			WHERE id = ? AND status = ?
		`, now, now), args
	}

	return fmt.Sprintf(`
		UPDATE recommendations
		SET status = $1, decided_by = $2, decided_at = %s, decision_reason = $3, updated_at = %s
		WHERE id = $4 AND status = $5
	`, now, now), args
}

func insertRecommendationHistoryTx(tx *TxOps, drv driver.Driver, entry *RecommendationHistory) error {
	now := drv.Now()
	if tx.Dialect() == driver.DialectSQLite {
		_, err := tx.Exec(fmt.Sprintf(`
			INSERT INTO recommendation_history (
				recommendation_id, from_status, to_status, decided_by, decision_reason, created_at
			) VALUES (?, ?, ?, ?, ?, %s)
		`, now), entry.RecommendationID, nullableRecommendationValue(entry.FromStatus), entry.ToStatus, nullableRecommendationValue(entry.DecidedBy), nullableRecommendationValue(entry.DecisionReason))
		if err != nil {
			return fmt.Errorf("insert recommendation history for %s: %w", entry.RecommendationID, err)
		}
		return nil
	}

	_, err := tx.Exec(fmt.Sprintf(`
		INSERT INTO recommendation_history (
			recommendation_id, from_status, to_status, decided_by, decision_reason, created_at
		) VALUES ($1, $2, $3, $4, $5, %s)
	`, now), entry.RecommendationID, nullableRecommendationValue(entry.FromStatus), entry.ToStatus, nullableRecommendationValue(entry.DecidedBy), nullableRecommendationValue(entry.DecisionReason))
	if err != nil {
		return fmt.Errorf("insert recommendation history for %s: %w", entry.RecommendationID, err)
	}
	return nil
}

func getRecommendationByQueryRow(row *sql.Row) (*Recommendation, error) {
	rec, err := scanRecommendation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func getRecommendationTx(tx *TxOps, id string) (*Recommendation, error) {
	placeholder := placeholderForDialect(tx.Dialect(), 1)
	row := tx.QueryRow(fmt.Sprintf(`
		SELECT id, kind, status, title, summary, proposed_action, evidence,
		       source_task_id, source_run_id, source_thread_id, dedupe_key, decided_by, decided_at,
		       decision_reason, promoted_to_type, promoted_to_id, promoted_by, promoted_at,
		       created_at, updated_at
		FROM recommendations
		WHERE id = %s
	`, placeholder), id)
	rec, err := scanRecommendation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrRecommendationNotFound, id)
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

func scanRecommendation(scanner recommendationRowScanner) (*Recommendation, error) {
	rec := &Recommendation{}
	var sourceTaskID sql.NullString
	var sourceRunID sql.NullString
	var sourceThreadID sql.NullString
	var decidedBy sql.NullString
	var decisionReason sql.NullString
	var promotedToType sql.NullString
	var promotedToID sql.NullString
	var promotedBy sql.NullString
	var decidedAt any
	var promotedAt any
	var createdAt any
	var updatedAt any

	err := scanner.Scan(
		&rec.ID,
		&rec.Kind,
		&rec.Status,
		&rec.Title,
		&rec.Summary,
		&rec.ProposedAction,
		&rec.Evidence,
		&sourceTaskID,
		&sourceRunID,
		&sourceThreadID,
		&rec.DedupeKey,
		&decidedBy,
		&decidedAt,
		&decisionReason,
		&promotedToType,
		&promotedToID,
		&promotedBy,
		&promotedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	rec.SourceTaskID = sourceTaskID.String
	rec.SourceRunID = sourceRunID.String
	rec.SourceThreadID = sourceThreadID.String
	rec.DecidedBy = decidedBy.String
	if t, ok := scannedTimestamp(decidedAt); ok {
		rec.DecidedAt = &t
	}
	rec.DecisionReason = decisionReason.String
	rec.PromotedToType = promotedToType.String
	rec.PromotedToID = promotedToID.String
	rec.PromotedBy = promotedBy.String
	if t, ok := scannedTimestamp(promotedAt); ok {
		rec.PromotedAt = &t
	}
	rec.CreatedAt = timestampOrZero(createdAt)
	rec.UpdatedAt = timestampOrZero(updatedAt)
	return rec, nil
}

func scanRecommendationHistory(scanner recommendationRowScanner) (*RecommendationHistory, error) {
	entry := &RecommendationHistory{}
	var fromStatus sql.NullString
	var decidedBy sql.NullString
	var decisionReason sql.NullString
	var createdAt any
	err := scanner.Scan(
		&entry.ID,
		&entry.RecommendationID,
		&fromStatus,
		&entry.ToStatus,
		&decidedBy,
		&decisionReason,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}
	entry.FromStatus = fromStatus.String
	entry.DecidedBy = decidedBy.String
	entry.DecisionReason = decisionReason.String
	entry.CreatedAt = timestampOrZero(createdAt)
	return entry, nil
}

func placeholderForDialect(dialect driver.Dialect, index int) string {
	if dialect == driver.DialectSQLite {
		return "?"
	}
	return fmt.Sprintf("$%d", index)
}

func nullableRecommendationValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableRecommendationTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}

func containsRecommendationStatus(statuses []string, status string) bool {
	for _, candidate := range statuses {
		if candidate == status {
			return true
		}
	}
	return false
}

func timestampOrZero(value any) time.Time {
	t, _ := scannedTimestamp(value)
	return t
}

func scannedTimestamp(value any) (time.Time, bool) {
	switch v := value.(type) {
	case nil:
		return time.Time{}, false
	case time.Time:
		return v, true
	case *time.Time:
		if v == nil {
			return time.Time{}, false
		}
		return *v, true
	case string:
		t := parseTimestamp(v)
		return t, !t.IsZero()
	case []byte:
		t := parseTimestamp(string(v))
		return t, !t.IsZero()
	case sql.NullString:
		if !v.Valid {
			return time.Time{}, false
		}
		t := parseTimestamp(v.String)
		return t, !t.IsZero()
	default:
		return time.Time{}, false
	}
}
