package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// Transcript represents a transcript entry.
type Transcript struct {
	ID        int64
	TaskID    string
	Phase     string
	Iteration int
	Role      string
	Content   string
	Timestamp time.Time
}

// AddTranscript appends a transcript entry.
func (p *ProjectDB) AddTranscript(t *Transcript) error {
	result, err := p.Exec(`
		INSERT INTO transcripts (task_id, phase, iteration, role, content)
		VALUES (?, ?, ?, ?, ?)
	`, t.TaskID, t.Phase, t.Iteration, t.Role, t.Content)
	if err != nil {
		return fmt.Errorf("add transcript: %w", err)
	}
	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

// AddTranscriptBatch inserts multiple transcript entries in a single transaction.
// This is more efficient than calling AddTranscript repeatedly for streaming data.
// All entries are inserted atomically - either all succeed or none do.
// Uses multi-row INSERT for O(1) database overhead instead of O(N).
func (p *ProjectDB) AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error {
	if len(transcripts) == 0 {
		return nil
	}

	return p.RunInTx(ctx, func(tx *TxOps) error {
		// Build multi-row INSERT for efficiency
		// SQLite supports up to 500 values per INSERT, but we batch by transcripts count
		// which is typically 50 or less per flush
		var query strings.Builder
		query.WriteString("INSERT INTO transcripts (task_id, phase, iteration, role, content) VALUES ")

		args := make([]any, 0, len(transcripts)*5)
		for i, t := range transcripts {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString("(?, ?, ?, ?, ?)")
			args = append(args, t.TaskID, t.Phase, t.Iteration, t.Role, t.Content)
		}

		result, err := tx.Exec(query.String(), args...)
		if err != nil {
			return fmt.Errorf("batch insert transcripts: %w", err)
		}

		// Get the last insert ID and work backwards to set all IDs
		// SQLite guarantees sequential IDs for multi-row inserts
		lastID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get last insert id: %w", err)
		}
		firstID := lastID - int64(len(transcripts)) + 1
		for i := range transcripts {
			transcripts[i].ID = firstID + int64(i)
		}
		return nil
	})
}

// GetTranscripts retrieves all transcripts for a task.
func (p *ProjectDB) GetTranscripts(taskID string) ([]Transcript, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, iteration, role, content, timestamp
		FROM transcripts WHERE task_id = ? ORDER BY id
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var transcripts []Transcript
	for rows.Next() {
		var t Transcript
		var timestamp, role sql.NullString
		if err := rows.Scan(&t.ID, &t.TaskID, &t.Phase, &t.Iteration, &role, &t.Content, &timestamp); err != nil {
			return nil, fmt.Errorf("scan transcript: %w", err)
		}
		if role.Valid {
			t.Role = role.String
		}
		if timestamp.Valid {
			if ts, err := time.Parse("2006-01-02 15:04:05", timestamp.String); err == nil {
				t.Timestamp = ts
			}
		}
		transcripts = append(transcripts, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transcripts: %w", err)
	}

	return transcripts, nil
}

// TranscriptMatch represents a search result.
type TranscriptMatch struct {
	TaskID  string
	Phase   string
	Snippet string
	Rank    float64
}

// SearchTranscripts performs full-text search on transcript content.
// For SQLite, uses FTS5 with MATCH. For PostgreSQL, uses ILIKE (basic search).
// Note: PostgreSQL full-text search with tsvector requires additional schema setup.
func (p *ProjectDB) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	var rows *sql.Rows
	var err error

	if p.Dialect() == driver.DialectSQLite {
		// SQLite FTS5 search
		// Sanitize query: escape quotes and wrap for literal matching
		// This prevents FTS5 syntax errors from special characters like - * " etc.
		sanitized := `"` + escapeQuotes(query) + `"`

		rows, err = p.Query(`
			SELECT task_id, phase, snippet(transcripts_fts, 0, '<mark>', '</mark>', '...', 32), rank
			FROM transcripts_fts
			WHERE content MATCH ?
			ORDER BY rank
			LIMIT 50
		`, sanitized)
	} else {
		// PostgreSQL basic search using ILIKE
		// For better performance, consider adding tsvector columns and GIN indexes
		likePattern := "%" + query + "%"
		rows, err = p.Query(`
			SELECT task_id, phase,
				SUBSTRING(content FROM GREATEST(1, POSITION($1 IN content) - 20) FOR 64) as snippet,
				0.0 as rank
			FROM transcripts
			WHERE content ILIKE $2
			ORDER BY id DESC
			LIMIT 50
		`, query, likePattern)
	}

	if err != nil {
		return nil, fmt.Errorf("search transcripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var matches []TranscriptMatch
	for rows.Next() {
		var m TranscriptMatch
		if err := rows.Scan(&m.TaskID, &m.Phase, &m.Snippet, &m.Rank); err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matches: %w", err)
	}

	return matches, nil
}
