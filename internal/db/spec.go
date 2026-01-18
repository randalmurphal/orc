package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// Spec represents a task specification stored in the database.
type Spec struct {
	TaskID      string
	Content     string
	ContentHash string
	Source      string // 'file', 'db', 'generated', 'migrated'
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SaveSpec creates or updates a spec.
func (p *ProjectDB) SaveSpec(spec *Spec) error {
	now := time.Now().Format(time.RFC3339)
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = time.Now()
	}

	_, err := p.Exec(`
		INSERT INTO specs (task_id, content, content_hash, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, spec.TaskID, spec.Content, spec.ContentHash, spec.Source,
		spec.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save spec: %w", err)
	}
	return nil
}

// GetSpec retrieves a spec by task ID.
func (p *ProjectDB) GetSpec(taskID string) (*Spec, error) {
	row := p.QueryRow(`
		SELECT task_id, content, content_hash, source, created_at, updated_at
		FROM specs WHERE task_id = ?
	`, taskID)

	var spec Spec
	var contentHash, source sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&spec.TaskID, &spec.Content, &contentHash, &source, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get spec %s: %w", taskID, err)
	}

	if contentHash.Valid {
		spec.ContentHash = contentHash.String
	}
	if source.Valid {
		spec.Source = source.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		spec.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		spec.UpdatedAt = ts
	}

	return &spec, nil
}

// DeleteSpec removes a spec.
func (p *ProjectDB) DeleteSpec(taskID string) error {
	_, err := p.Exec("DELETE FROM specs WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete spec: %w", err)
	}
	return nil
}

// SpecExists checks if a spec exists for a task.
func (p *ProjectDB) SpecExists(taskID string) (bool, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM specs WHERE task_id = ?", taskID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check spec exists: %w", err)
	}
	return count > 0, nil
}

// SearchSpecs performs full-text search on spec content.
// Returns TranscriptMatch for compatibility with unified search results.
func (p *ProjectDB) SearchSpecs(query string) ([]TranscriptMatch, error) {
	var rows *sql.Rows
	var err error

	if p.Dialect() == driver.DialectSQLite {
		sanitized := `"` + escapeQuotes(query) + `"`
		rows, err = p.Query(`
			SELECT task_id, '' as phase, snippet(specs_fts, 0, '<mark>', '</mark>', '...', 32), rank
			FROM specs_fts
			WHERE content MATCH ?
			ORDER BY rank
			LIMIT 50
		`, sanitized)
	} else {
		likePattern := "%" + query + "%"
		rows, err = p.Query(`
			SELECT task_id, '' as phase,
				SUBSTRING(content FROM GREATEST(1, POSITION($1 IN content) - 20) FOR 64) as snippet,
				0.0 as rank
			FROM specs
			WHERE content ILIKE $2
			ORDER BY task_id
			LIMIT 50
		`, query, likePattern)
	}

	if err != nil {
		return nil, fmt.Errorf("search specs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []TranscriptMatch
	for rows.Next() {
		var m TranscriptMatch
		if err := rows.Scan(&m.TaskID, &m.Phase, &m.Snippet, &m.Rank); err != nil {
			return nil, fmt.Errorf("scan spec match: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spec matches: %w", err)
	}

	return matches, nil
}

// escapeQuotes escapes double quotes for FTS5 literal matching.
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}
