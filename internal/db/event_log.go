package db

import (
	"context"
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
	Data       any // JSON marshaled to TEXT
	Source     string
	CreatedAt  time.Time
	DurationMs *int64 // nullable
}

// EventLogWithTitle extends EventLog with task title for API responses.
type EventLogWithTitle struct {
	EventLog
	TaskTitle string
}

// QueryEventsOptions specifies filters for querying events.
type QueryEventsOptions struct {
	TaskID       string
	InitiativeID string
	Since        *time.Time
	Until        *time.Time
	EventTypes   []string
	Limit        int
	Offset       int
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

	// Use UTC for timestamp storage with nanosecond precision
	// Nanoseconds enable deduplication of true duplicates (same timestamp) while
	// preserving different events created in quick succession
	createdAt := event.CreatedAt.UTC().Format("2006-01-02 15:04:05.000000000")

	// INSERT OR IGNORE silently skips duplicates based on the unique index
	// (task_id, event_type, COALESCE(phase, ''), created_at) - see project_039.sql
	result, err := p.Exec(`
		INSERT OR IGNORE INTO event_log (task_id, phase, iteration, event_type, data, source, created_at, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, event.TaskID, event.Phase, event.Iteration, event.EventType, dataJSON, event.Source, createdAt, event.DurationMs)
	if err != nil {
		return fmt.Errorf("save event: %w", err)
	}

	// RowsAffected is 0 if duplicate was ignored, 1 if inserted
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event id: %w", err)
		}
		event.ID = id
	}
	return nil
}

// SaveEvents inserts multiple events in a single transaction for efficiency.
func (p *ProjectDB) SaveEvents(events []*EventLog) error {
	if len(events) == 0 {
		return nil
	}

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		for _, event := range events {
			var dataJSON *string
			if event.Data != nil {
				bytes, err := json.Marshal(event.Data)
				if err != nil {
					return fmt.Errorf("marshal event data: %w", err)
				}
				s := string(bytes)
				dataJSON = &s
			}

			createdAt := event.CreatedAt.UTC().Format("2006-01-02 15:04:05.000000000")

			// INSERT OR IGNORE silently skips duplicates based on the unique index
			// (task_id, event_type, COALESCE(phase, ''), created_at) - see project_039.sql
			result, err := tx.Exec(`
				INSERT OR IGNORE INTO event_log (task_id, phase, iteration, event_type, data, source, created_at, duration_ms)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, event.TaskID, event.Phase, event.Iteration,
				event.EventType, dataJSON, event.Source,
				createdAt, event.DurationMs)
			if err != nil {
				return fmt.Errorf("insert event: %w", err)
			}

			// RowsAffected is 0 if duplicate was ignored, 1 if inserted
			rows, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("check rows affected: %w", err)
			}
			if rows > 0 {
				id, err := result.LastInsertId()
				if err != nil {
					return fmt.Errorf("get event id: %w", err)
				}
				event.ID = id
			}
		}
		return nil
	})
}

// QueryEvents retrieves events matching the specified filters.
// Results are returned in descending created_at order (newest first).
func (p *ProjectDB) QueryEvents(opts QueryEventsOptions) ([]EventLog, error) {
	var query strings.Builder
	var args []any

	// Base SELECT - add task join only if filtering by initiative
	if opts.InitiativeID != "" {
		query.WriteString(`
			SELECT e.id, e.task_id, e.phase, e.iteration, e.event_type, e.data, e.source, e.created_at, e.duration_ms
			FROM event_log e
			LEFT JOIN tasks t ON e.task_id = t.id
			WHERE 1=1
		`)
	} else {
		query.WriteString(`
			SELECT id, task_id, phase, iteration, event_type, data, source, created_at, duration_ms
			FROM event_log
			WHERE 1=1
		`)
	}

	// Filter by task_id
	if opts.TaskID != "" {
		if opts.InitiativeID != "" {
			query.WriteString(" AND e.task_id = ?")
		} else {
			query.WriteString(" AND task_id = ?")
		}
		args = append(args, opts.TaskID)
	}

	// Filter by initiative_id (requires task join)
	if opts.InitiativeID != "" {
		query.WriteString(" AND t.initiative_id = ?")
		args = append(args, opts.InitiativeID)
	}

	// Column prefix for when using join
	col := func(name string) string {
		if opts.InitiativeID != "" {
			return "e." + name
		}
		return name
	}

	// Filter by time range (since)
	if opts.Since != nil {
		query.WriteString(" AND " + col("created_at") + " >= ?")
		args = append(args, opts.Since.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by time range (until)
	if opts.Until != nil {
		query.WriteString(" AND " + col("created_at") + " <= ?")
		args = append(args, opts.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by event types
	if len(opts.EventTypes) > 0 {
		placeholders := make([]string, len(opts.EventTypes))
		for i, et := range opts.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		query.WriteString(" AND " + col("event_type") + " IN (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	}

	// Order by created_at descending
	query.WriteString(" ORDER BY " + col("created_at") + " DESC, " + col("id") + " DESC")

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

		// Parse created_at timestamp - try formats in order of precision
		e.CreatedAt = parseEventTimestamp(createdAt)

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

// parseEventTimestamp parses timestamps in various formats (nanoseconds, microseconds, seconds).
// Returns zero time if parsing fails.
func parseEventTimestamp(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05.000000000", // nanoseconds
		"2006-01-02 15:04:05.000000",    // microseconds
		"2006-01-02 15:04:05",           // seconds
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// QueryEventsWithTitles retrieves events with task titles joined from tasks table.
// This is used for API responses where task titles are needed for display.
// Results are returned in descending created_at order (newest first).
func (p *ProjectDB) QueryEventsWithTitles(opts QueryEventsOptions) ([]EventLogWithTitle, error) {
	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT
			e.id, e.task_id, e.phase, e.iteration, e.event_type, e.data, e.source, e.created_at, e.duration_ms,
			COALESCE(t.title, '') as task_title
		FROM event_log e
		LEFT JOIN tasks t ON e.task_id = t.id
		WHERE 1=1
	`)

	// Filter by task_id
	if opts.TaskID != "" {
		query.WriteString(" AND e.task_id = ?")
		args = append(args, opts.TaskID)
	}

	// Filter by initiative_id (requires task join)
	if opts.InitiativeID != "" {
		query.WriteString(" AND t.initiative_id = ?")
		args = append(args, opts.InitiativeID)
	}

	// Filter by time range (since)
	if opts.Since != nil {
		query.WriteString(" AND e.created_at >= ?")
		args = append(args, opts.Since.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by time range (until)
	if opts.Until != nil {
		query.WriteString(" AND e.created_at <= ?")
		args = append(args, opts.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by event types
	if len(opts.EventTypes) > 0 {
		placeholders := make([]string, len(opts.EventTypes))
		for i, et := range opts.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		query.WriteString(" AND e.event_type IN (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	}

	// Order by created_at descending
	query.WriteString(" ORDER BY e.created_at DESC, e.id DESC")

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
		return nil, fmt.Errorf("query events with titles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var events []EventLogWithTitle
	for rows.Next() {
		var e EventLogWithTitle
		var phase, dataJSON sql.NullString
		var iteration sql.NullInt64
		var durationMs sql.NullInt64
		var createdAt string

		if err := rows.Scan(
			&e.ID, &e.TaskID, &phase, &iteration, &e.EventType,
			&dataJSON, &e.Source, &createdAt, &durationMs,
			&e.TaskTitle,
		); err != nil {
			return nil, fmt.Errorf("scan event with title: %w", err)
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

		// Parse created_at timestamp - try formats in order of precision
		e.CreatedAt = parseEventTimestamp(createdAt)

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
		return nil, fmt.Errorf("iterate events with titles: %w", err)
	}

	return events, nil
}

// CountEvents returns the total count of events matching the specified filters.
// Used for pagination metadata.
func (p *ProjectDB) CountEvents(opts QueryEventsOptions) (int, error) {
	var query strings.Builder
	var args []any

	query.WriteString(`
		SELECT COUNT(*)
		FROM event_log e
	`)

	// Add task join if needed for initiative filter
	if opts.InitiativeID != "" {
		query.WriteString(" LEFT JOIN tasks t ON e.task_id = t.id")
	}

	query.WriteString(" WHERE 1=1")

	// Filter by task_id
	if opts.TaskID != "" {
		query.WriteString(" AND e.task_id = ?")
		args = append(args, opts.TaskID)
	}

	// Filter by initiative_id (requires task join)
	if opts.InitiativeID != "" {
		query.WriteString(" AND t.initiative_id = ?")
		args = append(args, opts.InitiativeID)
	}

	// Filter by time range (since)
	if opts.Since != nil {
		query.WriteString(" AND e.created_at >= ?")
		args = append(args, opts.Since.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by time range (until)
	if opts.Until != nil {
		query.WriteString(" AND e.created_at <= ?")
		args = append(args, opts.Until.UTC().Format("2006-01-02 15:04:05"))
	}

	// Filter by event types
	if len(opts.EventTypes) > 0 {
		placeholders := make([]string, len(opts.EventTypes))
		for i, et := range opts.EventTypes {
			placeholders[i] = "?"
			args = append(args, et)
		}
		query.WriteString(" AND e.event_type IN (")
		query.WriteString(strings.Join(placeholders, ", "))
		query.WriteString(")")
	}

	var count int
	err := p.QueryRow(query.String(), args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}

	return count, nil
}
