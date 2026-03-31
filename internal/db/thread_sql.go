package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

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
		query += " AND id = " + placeholderForDialect(dialect, 1)
	}
	return query
}

func threadMessagesQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, role, content, created_at
		FROM thread_messages
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
		ORDER BY created_at ASC, id ASC
	`
}

func threadAssociationLinkTargetQuery(dialect driver.Dialect) string {
	return `
		SELECT target_id
		FROM thread_links
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
		  AND link_type = ` + placeholderForDialect(dialect, 2) + `
		ORDER BY created_at ASC, id ASC
		LIMIT 1
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
		SET updated_at = ` + placeholderForDialect(dialect, 1) + `
		WHERE id = ` + placeholderForDialect(dialect, 2)
}

func threadArchiveQuery(dialect driver.Dialect) string {
	return `
		UPDATE threads
		SET status = 'archived', updated_at = ` + placeholderForDialect(dialect, 1) + `
		WHERE id = ` + placeholderForDialect(dialect, 2)
}

func threadDeleteQuery(dialect driver.Dialect) string {
	return `DELETE FROM threads WHERE id = ` + placeholderForDialect(dialect, 1)
}

func threadSessionUpdateQuery(dialect driver.Dialect) string {
	return `
		UPDATE threads
		SET session_id = ` + placeholderForDialect(dialect, 1) + `,
		    updated_at = ` + placeholderForDialect(dialect, 2) + `
		WHERE id = ` + placeholderForDialect(dialect, 3)
}

func threadLinksQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, link_type, target_id, title, created_at
		FROM thread_links
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
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
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
		  AND link_type = ` + placeholderForDialect(dialect, 2) + `
		  AND target_id = ` + placeholderForDialect(dialect, 3)
}

func threadRecommendationDraftsQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, kind, title, summary, proposed_action, evidence, dedupe_key,
		       source_task_id, source_run_id, status, promoted_recommendation_id, promoted_by,
		       promoted_at, created_at, updated_at
		FROM thread_recommendation_drafts
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
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
		WHERE id = ` + placeholderForDialect(dialect, 1)
}

func threadRecommendationDraftPromoteQuery(dialect driver.Dialect) string {
	return `
		UPDATE thread_recommendation_drafts
		SET status = 'promoted',
		    promoted_recommendation_id = ` + placeholderForDialect(dialect, 1) + `,
		    promoted_by = ` + placeholderForDialect(dialect, 2) + `,
		    promoted_at = ` + placeholderForDialect(dialect, 3) + `,
		    updated_at = ` + placeholderForDialect(dialect, 4) + `
		WHERE id = ` + placeholderForDialect(dialect, 5)
}

func threadDecisionDraftsQuery(dialect driver.Dialect) string {
	return `
		SELECT id, thread_id, initiative_id, decision, rationale, status,
		       promoted_decision_id, promoted_by, promoted_at, created_at, updated_at
		FROM thread_decision_drafts
		WHERE thread_id = ` + placeholderForDialect(dialect, 1) + `
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
		WHERE task_id = ` + placeholderForDialect(dialect, 1) + `
		ORDER BY created_at DESC
		LIMIT 1
	`
}

func placeholders(dialect driver.Dialect, start int, count int) string {
	values := make([]string, 0, count)
	for i := 0; i < count; i++ {
		values = append(values, placeholderForDialect(dialect, start+i))
	}
	return strings.Join(values, ", ")
}

func threadAssociationMirrorUpdateQuery(dialect driver.Dialect, linkType string) (string, error) {
	switch linkType {
	case ThreadLinkTypeTask:
		return `
			UPDATE threads
			SET task_id = ` + placeholderForDialect(dialect, 1) + `
			WHERE id = ` + placeholderForDialect(dialect, 2), nil
	case ThreadLinkTypeInitiative:
		return `
			UPDATE threads
			SET initiative_id = ` + placeholderForDialect(dialect, 1) + `
			WHERE id = ` + placeholderForDialect(dialect, 2), nil
	default:
		return "", fmt.Errorf("thread link type %q does not mirror to legacy columns", linkType)
	}
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}
