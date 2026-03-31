package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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

// CreateThreadLink adds a typed link to a thread and touches thread updated_at.
func (p *ProjectDB) CreateThreadLink(link *ThreadLink) error {
	if err := validateThreadLink(link); err != nil {
		return err
	}

	now := time.Now().UTC()
	link.CreatedAt = now
	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		if err := ensureThreadAssociationLinkTargetTx(tx, link); err != nil {
			return err
		}
		if err := createThreadLinkTx(tx, link); err != nil {
			return err
		}
		if err := syncThreadAssociationMirrorTx(tx, link); err != nil {
			return err
		}
		return touchThreadTx(tx, link.ThreadID, now)
	})
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

func ensureThreadAssociationLinkTargetTx(tx *TxOps, link *ThreadLink) error {
	if !threadLinkTypeHasSingleTarget(link.LinkType) {
		return nil
	}

	existingTarget, err := threadAssociationLinkTargetTx(tx, link.ThreadID, link.LinkType)
	if err != nil {
		return err
	}
	if existingTarget != "" && existingTarget != link.TargetID {
		return fmt.Errorf("thread %s already linked to %s %s", link.ThreadID, link.LinkType, existingTarget)
	}
	return nil
}

func syncThreadAssociationMirrorTx(tx *TxOps, link *ThreadLink) error {
	if !threadLinkTypeHasSingleTarget(link.LinkType) {
		return nil
	}

	query, err := threadAssociationMirrorUpdateQuery(tx.Dialect(), link.LinkType)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(query, link.TargetID, link.ThreadID); err != nil {
		return fmt.Errorf("sync legacy %s mirror for thread %s: %w", link.LinkType, link.ThreadID, err)
	}
	return nil
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

func threadAssociationLinkTargetTx(tx *TxOps, threadID string, linkType string) (string, error) {
	row := tx.QueryRow(threadAssociationLinkTargetQuery(tx.Dialect()), threadID, linkType)
	var targetID sql.NullString
	if err := row.Scan(&targetID); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("load %s link for thread %s: %w", linkType, threadID, err)
	}
	return targetID.String, nil
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
	`, placeholderForDialect(pdb.Dialect(), 1))

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

func isValidThreadLinkType(linkType string) bool {
	switch linkType {
	case ThreadLinkTypeTask, ThreadLinkTypeInitiative, ThreadLinkTypeRecommendation, ThreadLinkTypeFile, ThreadLinkTypeDiff:
		return true
	default:
		return false
	}
}

func threadLinkTypeHasSingleTarget(linkType string) bool {
	switch linkType {
	case ThreadLinkTypeTask, ThreadLinkTypeInitiative:
		return true
	default:
		return false
	}
}

func validateThreadAssociationLinks(links []ThreadLink) error {
	canonicalTargets := make(map[string]string)
	for _, link := range links {
		if !threadLinkTypeHasSingleTarget(link.LinkType) {
			continue
		}
		targetID := strings.TrimSpace(link.TargetID)
		if targetID == "" {
			continue
		}
		if existingTarget, ok := canonicalTargets[link.LinkType]; ok && existingTarget != targetID {
			return fmt.Errorf("thread cannot link %s to both %s and %s", link.LinkType, existingTarget, targetID)
		}
		canonicalTargets[link.LinkType] = targetID
	}
	return nil
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
