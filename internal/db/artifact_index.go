package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

const (
	ArtifactKindAcceptedRecommendation = "accepted_recommendation"
	ArtifactKindInitiativeDecision     = "initiative_decision"
	ArtifactKindPromotedDraft          = "promoted_draft"
	ArtifactKindTaskOutcome            = "task_outcome"
)

type ArtifactIndexEntry struct {
	ID             int64
	Kind           string
	Title          string
	Content        string
	DedupeKey      string
	InitiativeID   string
	SourceTaskID   string
	SourceRunID    string
	SourceThreadID string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

type ArtifactIndexQueryOpts struct {
	Kind           string
	InitiativeID   string
	SourceTaskID   string
	SourceRunID    string
	SourceThreadID string
	Search         string
	Limit          int
	IncludeDeleted bool
}

type RecentArtifactOpts struct {
	InitiativeID string
	SourceTaskID string
	Limit        int
}

func (p *ProjectDB) SaveArtifactIndexEntry(entry *ArtifactIndexEntry) error {
	if err := validateArtifactIndexEntry(entry); err != nil {
		return err
	}

	now := p.Driver().Now()
	if p.Dialect() == driver.DialectSQLite {
		result, err := p.Exec(fmt.Sprintf(`
			INSERT INTO artifact_index (
				kind, title, content, dedupe_key, initiative_id, source_task_id,
				source_run_id, source_thread_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, now, now),
			entry.Kind,
			entry.Title,
			entry.Content,
			nullableArtifactValue(entry.DedupeKey),
			nullableArtifactValue(entry.InitiativeID),
			nullableArtifactValue(entry.SourceTaskID),
			nullableArtifactValue(entry.SourceRunID),
			nullableArtifactValue(entry.SourceThreadID),
		)
		if err != nil {
			return fmt.Errorf("insert artifact index entry: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("artifact index last insert id: %w", err)
		}
		saved, err := p.getArtifactIndexEntry(id)
		if err != nil {
			return err
		}
		*entry = *saved
		return nil
	}

	row := p.QueryRow(fmt.Sprintf(`
		INSERT INTO artifact_index (
			kind, title, content, dedupe_key, initiative_id, source_task_id,
			source_run_id, source_thread_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, %s, %s)
		RETURNING id, kind, title, content, dedupe_key, initiative_id, source_task_id,
		          source_run_id, source_thread_id, created_at, updated_at, deleted_at
	`, now, now),
		entry.Kind,
		entry.Title,
		entry.Content,
		nullableArtifactValue(entry.DedupeKey),
		nullableArtifactValue(entry.InitiativeID),
		nullableArtifactValue(entry.SourceTaskID),
		nullableArtifactValue(entry.SourceRunID),
		nullableArtifactValue(entry.SourceThreadID),
	)
	saved, err := scanArtifactIndexEntry(row)
	if err != nil {
		return fmt.Errorf("insert artifact index entry: %w", err)
	}
	*entry = *saved
	return nil
}

func (p *ProjectDB) QueryArtifactIndex(opts ArtifactIndexQueryOpts) ([]ArtifactIndexEntry, error) {
	query, args := artifactIndexQuery(p.Dialect(), opts)
	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query artifact index: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]ArtifactIndexEntry, 0)
	for rows.Next() {
		entry, err := scanArtifactIndexEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan artifact index entry: %w", err)
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact index entries: %w", err)
	}
	return entries, nil
}

func (p *ProjectDB) QueryArtifactIndexByDedupeKey(dedupeKey string) ([]ArtifactIndexEntry, error) {
	if strings.TrimSpace(dedupeKey) == "" {
		return []ArtifactIndexEntry{}, nil
	}

	return queryArtifactIndexByDedupeKey(p, p.Dialect(), dedupeKey)
}

func (p *ProjectDB) GetRecentArtifacts(opts RecentArtifactOpts) ([]ArtifactIndexEntry, error) {
	query, args := recentArtifactQuery(p.Dialect(), opts)
	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get recent artifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]ArtifactIndexEntry, 0)
	for rows.Next() {
		entry, err := scanArtifactIndexEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recent artifact: %w", err)
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent artifacts: %w", err)
	}
	return entries, nil
}

func (p *ProjectDB) getArtifactIndexEntry(id int64) (*ArtifactIndexEntry, error) {
	row := p.QueryRow(fmt.Sprintf(`
		SELECT id, kind, title, content, dedupe_key, initiative_id, source_task_id,
		       source_run_id, source_thread_id, created_at, updated_at, deleted_at
		FROM artifact_index
		WHERE id = %s
	`, placeholderForDialect(p.Dialect(), 1)), id)
	entry, err := scanArtifactIndexEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("artifact index entry %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get artifact index entry %d: %w", id, err)
	}
	return entry, nil
}

func validateArtifactIndexEntry(entry *ArtifactIndexEntry) error {
	if entry == nil {
		return fmt.Errorf("artifact index entry is required")
	}
	switch entry.Kind {
	case ArtifactKindAcceptedRecommendation, ArtifactKindInitiativeDecision, ArtifactKindPromotedDraft, ArtifactKindTaskOutcome:
	default:
		return fmt.Errorf("invalid artifact kind %q", entry.Kind)
	}
	if strings.TrimSpace(entry.Title) == "" {
		return fmt.Errorf("artifact title is required")
	}
	if strings.TrimSpace(entry.Content) == "" {
		return fmt.Errorf("artifact content is required")
	}
	return nil
}

func artifactIndexQuery(dialect driver.Dialect, opts ArtifactIndexQueryOpts) (string, []any) {
	args := make([]any, 0)
	index := 1

	tableExpr := "artifact_index ai"
	where := make([]string, 0)
	orderBy := "ai.created_at DESC, ai.id DESC"

	if opts.Search != "" {
		if dialect == driver.DialectSQLite {
			tableExpr = "artifact_index ai JOIN artifact_index_fts ON artifact_index_fts.rowid = ai.id"
			where = append(where, fmt.Sprintf("artifact_index_fts MATCH %s", placeholderForDialect(dialect, index)))
			args = append(args, `"`+escapeQuotes(opts.Search)+`"`)
			index++
		} else {
			where = append(where, fmt.Sprintf("ai.search_vector @@ plainto_tsquery('english', %s)", placeholderForDialect(dialect, index)))
			args = append(args, opts.Search)
			orderBy = fmt.Sprintf("ts_rank(ai.search_vector, plainto_tsquery('english', %s)) DESC, %s", placeholderForDialect(dialect, index), orderBy)
			index++
		}
	}

	if !opts.IncludeDeleted {
		where = append(where, "ai.deleted_at IS NULL")
	}
	if opts.Kind != "" {
		where = append(where, fmt.Sprintf("ai.kind = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.Kind)
		index++
	}
	if opts.InitiativeID != "" {
		where = append(where, fmt.Sprintf("ai.initiative_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.InitiativeID)
		index++
	}
	if opts.SourceTaskID != "" {
		where = append(where, fmt.Sprintf("ai.source_task_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.SourceTaskID)
		index++
	}
	if opts.SourceRunID != "" {
		where = append(where, fmt.Sprintf("ai.source_run_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.SourceRunID)
		index++
	}
	if opts.SourceThreadID != "" {
		where = append(where, fmt.Sprintf("ai.source_thread_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.SourceThreadID)
		index++
	}
	if opts.Search == "" && opts.Limit == 0 {
		opts.Limit = 50
	}

	query := `
		SELECT ai.id, ai.kind, ai.title, ai.content, ai.dedupe_key, ai.initiative_id,
		       ai.source_task_id, ai.source_run_id, ai.source_thread_id,
		       ai.created_at, ai.updated_at, ai.deleted_at
		FROM ` + tableExpr
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, "\n  AND ")
	}
	query += "\nORDER BY " + orderBy
	if opts.Limit > 0 {
		query += fmt.Sprintf("\nLIMIT %s", placeholderForDialect(dialect, index))
		args = append(args, opts.Limit)
	}
	return query, args
}

func recentArtifactQuery(dialect driver.Dialect, opts RecentArtifactOpts) (string, []any) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 12
	}

	args := make([]any, 0)
	index := 1
	where := []string{"deleted_at IS NULL"}

	if opts.SourceTaskID != "" && opts.InitiativeID != "" {
		where = append(where, fmt.Sprintf("(source_task_id = %s OR initiative_id = %s)", placeholderForDialect(dialect, index), placeholderForDialect(dialect, index+1)))
		args = append(args, opts.SourceTaskID, opts.InitiativeID)
		index += 2
	} else if opts.SourceTaskID != "" {
		where = append(where, fmt.Sprintf("source_task_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.SourceTaskID)
		index++
	} else if opts.InitiativeID != "" {
		where = append(where, fmt.Sprintf("initiative_id = %s", placeholderForDialect(dialect, index)))
		args = append(args, opts.InitiativeID)
		index++
	}

	query := `
		SELECT id, kind, title, content, dedupe_key, initiative_id,
		       source_task_id, source_run_id, source_thread_id,
		       created_at, updated_at, deleted_at
		FROM artifact_index
		WHERE ` + strings.Join(where, "\n  AND ") + `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + placeholderForDialect(dialect, index)
	args = append(args, limit)
	return query, args
}

type artifactIndexRowScanner interface {
	Scan(dest ...any) error
}

type artifactIndexQueryRunner interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func queryArtifactIndexByDedupeKey(
	runner artifactIndexQueryRunner,
	dialect driver.Dialect,
	dedupeKey string,
) ([]ArtifactIndexEntry, error) {
	rows, err := runner.Query(fmt.Sprintf(`
		SELECT id, kind, title, content, dedupe_key, initiative_id, source_task_id,
		       source_run_id, source_thread_id, created_at, updated_at, deleted_at
		FROM artifact_index
		WHERE dedupe_key = %s
		  AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC
	`, placeholderForDialect(dialect, 1)), dedupeKey)
	if err != nil {
		return nil, fmt.Errorf("query artifact index by dedupe key %s: %w", dedupeKey, err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]ArtifactIndexEntry, 0)
	for rows.Next() {
		entry, err := scanArtifactIndexEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan artifact dedupe match: %w", err)
		}
		entries = append(entries, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact dedupe matches: %w", err)
	}
	return entries, nil
}

func scanArtifactIndexEntry(scanner artifactIndexRowScanner) (*ArtifactIndexEntry, error) {
	entry := &ArtifactIndexEntry{}
	var dedupeKey sql.NullString
	var initiativeID sql.NullString
	var sourceTaskID sql.NullString
	var sourceRunID sql.NullString
	var sourceThreadID sql.NullString
	var createdAt any
	var updatedAt any
	var deletedAt any

	if err := scanner.Scan(
		&entry.ID,
		&entry.Kind,
		&entry.Title,
		&entry.Content,
		&dedupeKey,
		&initiativeID,
		&sourceTaskID,
		&sourceRunID,
		&sourceThreadID,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}

	if dedupeKey.Valid {
		entry.DedupeKey = dedupeKey.String
	}
	if initiativeID.Valid {
		entry.InitiativeID = initiativeID.String
	}
	if sourceTaskID.Valid {
		entry.SourceTaskID = sourceTaskID.String
	}
	if sourceRunID.Valid {
		entry.SourceRunID = sourceRunID.String
	}
	if sourceThreadID.Valid {
		entry.SourceThreadID = sourceThreadID.String
	}
	entry.CreatedAt = timestampOrZero(createdAt)
	entry.UpdatedAt = timestampOrZero(updatedAt)
	if parsedDeletedAt, ok := scannedTimestamp(deletedAt); ok {
		entry.DeletedAt = &parsedDeletedAt
	}
	return entry, nil
}

func nullableArtifactValue(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
