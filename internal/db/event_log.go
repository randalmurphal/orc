package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EventLog represents a persisted executor event.
// Used for timeline reconstruction and historical event queries.
type EventLog struct {
	ID         int64
	TaskID     string
	Phase      *string // nullable for task-level events
	Iteration  *int    // nullable for task-level events
	EventType  string
	Data       any     // JSON marshaled to TEXT
	Source     string
	CreatedAt  time.Time
	DurationMs *int64 // nullable
}

// QueryEventsOptions specifies filters for querying events.
type QueryEventsOptions struct {
	TaskID     string
	Since      *time.Time
	Until      *time.Time
	EventTypes []string
	Limit      int
	Offset     int
}

// SaveEvent inserts an event into the event_log table.
func (p *ProjectDB) SaveEvent(event *EventLog) error {
	var dataJSON *string
	if event.Data != nil {
		bytes, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("marshal event data: %w", err)
		}
		s := string(bytes)
		dataJSON = &s
	}

	// Use UTC for timestamp storage
	createdAt := event.CreatedAt.UTC().Format("2006-01-02 15:04:05")

	result, err := p.Exec(`
		INSERT INTO event_log (task_id, phase, iteration, event_type, data, source, created_at, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, event.TaskID, event.Phase, event.Iteration, event.EventType, dataJSON, event.Source, createdAt, event.DurationMs)
	if err != nil {
		return fmt.Errorf("save event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get event id: %w", err)
	}
	event.ID = id
	return nil
}

// QueryEvents retrieves events matching the specified filters.
// Results are returned in descending created_at order (newest first).
func (p *ProjectDB) QueryEvents(opts QueryEventsOptions) ([]EventLog, error) {
	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT id, task_id, phase, iteration, event_type, data, source, created_at, duration_ms
		FROM event_log
		WHERE 1=1
	`)

	// Filter by task_id
	if opts.TaskID != "" {
		query.WriteString(" AND task_id = ?")
		args = append(args, opts.TaskID)
	}

	// Filter by time range (since)
	if opts.Since != nil {
		query.WriteString(" AND created_at >= ?")
		args = append(args, opts.Since.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by time range (until)
	if opts.Until != nil {
		query.WriteString(" AND created_at <= ?")
		args = append(args, opts.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by event types
	if len(opts.EventTypes) > 0 {
		placeholders := make([]string, len(opts.EventTypes))
		for i, et := range opts.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		query.WriteString(" AND event_type IN (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	}

	// Order by created_at descending
	query.WriteString(" ORDER BY created_at DESC, id DESC")

	// Apply pagination
	if opts.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opts.Limit)

		if opts.Offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, opts.Offset)
		}
	}

	rows, err := p.Query(query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanEventLogs(rows)
}

// scanEventLogs scans rows into EventLog slice.
func scanEventLogs(rows *sql.Rows) ([]EventLog, error) {
	var events []EventLog
	for rows.Next() {
		var e EventLog
		var phase, dataJSON sql.NullString
		var iteration sql.NullInt64
		var durationMs sql.NullInt64
		var createdAt string

		if err := rows.Scan(
			&e.ID, &e.TaskID, &phase, &iteration, &e.EventType,
			&dataJSON, &e.Source, &createdAt, &durationMs,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		if phase.Valid {
			e.Phase = &phase.String
		}
		if iteration.Valid {
			i := int(iteration.Int64)
			e.Iteration = &i
		}
		if durationMs.Valid {
			e.DurationMs = &durationMs.Int64
		}

		// Parse created_at timestamp
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			e.CreatedAt = t.UTC()
		}

		// Parse JSON data
		if dataJSON.Valid && dataJSON.String != "" {
			var data any
			if err := json.Unmarshal([]byte(dataJSON.String), &data); err == nil {
				e.Data = data
			} else {
				// Store as raw string if not valid JSON
				e.Data = dataJSON.String
			}
		}

		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}
