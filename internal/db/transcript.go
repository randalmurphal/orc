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

// Transcript represents a single message from a Claude JSONL session file.
// This stores the full message data including per-message token usage.
type Transcript struct {
	ID            int64
	TaskID        string
	Phase         string
	SessionID     string  // Claude session UUID
	WorkflowRunID string  // Links to workflow_runs.id for tracking
	MessageUUID   string  // Individual message UUID
	ParentUUID    *string // Links to parent message (threading)
	Type          string  // "user", "assistant", "queue-operation", "hook"
	Role          string  // from message.role
	Content       string  // Full content JSON (preserves structure)
	Model         string  // Model used (assistant messages only)

	// Per-message token tracking
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int

	// Tool information
	ToolCalls   string // JSON array of tool_use blocks
	ToolResults string // JSON of toolUseResult metadata

	Timestamp time.Time
}

// AddTranscript inserts a single transcript entry.
func (p *ProjectDB) AddTranscript(t *Transcript) error {
	// Handle empty workflow_run_id as NULL
	var runID any = t.WorkflowRunID
	if t.WorkflowRunID == "" {
		runID = nil
	}

	result, err := p.Exec(`
		INSERT INTO transcripts (
			task_id, phase, session_id, workflow_run_id, message_uuid, parent_uuid,
			type, role, content, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			tool_calls, tool_results, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.TaskID, t.Phase, t.SessionID, runID, t.MessageUUID, t.ParentUUID,
		t.Type, t.Role, t.Content, t.Model,
		t.InputTokens, t.OutputTokens, t.CacheCreationTokens, t.CacheReadTokens,
		t.ToolCalls, t.ToolResults, t.Timestamp.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("add transcript: %w", err)
	}
	id, _ := result.LastInsertId()
	t.ID = id
	return nil
}

// AddTranscriptBatch inserts multiple transcript entries in a single transaction.
// Uses multi-row INSERT for efficiency.
func (p *ProjectDB) AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error {
	if len(transcripts) == 0 {
		return nil
	}

	return p.RunInTx(ctx, func(tx *TxOps) error {
		var query strings.Builder
		query.WriteString(`INSERT INTO transcripts (
			task_id, phase, session_id, workflow_run_id, message_uuid, parent_uuid,
			type, role, content, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			tool_calls, tool_results, timestamp
		) VALUES `)

		args := make([]any, 0, len(transcripts)*17)
		for i, t := range transcripts {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

			// Handle empty workflow_run_id as NULL
			var runID any = t.WorkflowRunID
			if t.WorkflowRunID == "" {
				runID = nil
			}

			args = append(args,
				t.TaskID, t.Phase, t.SessionID, runID, t.MessageUUID, t.ParentUUID,
				t.Type, t.Role, t.Content, t.Model,
				t.InputTokens, t.OutputTokens, t.CacheCreationTokens, t.CacheReadTokens,
				t.ToolCalls, t.ToolResults, t.Timestamp.UnixMilli(),
			)
		}

		result, err := tx.Exec(query.String(), args...)
		if err != nil {
			return fmt.Errorf("batch insert transcripts: %w", err)
		}

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

// GetTranscripts retrieves all transcripts for a task ordered by timestamp.
func (p *ProjectDB) GetTranscripts(taskID string) ([]Transcript, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE task_id = ?
		ORDER BY timestamp, id
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTranscripts(rows)
}

// GetTranscriptsPaginated retrieves paginated transcripts with filtering.
func (p *ProjectDB) GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error) {
	// Apply defaults and enforce limits
	if opts.Limit == 0 {
		opts.Limit = 50
	}
	// Cap limit to prevent DoS via unbounded queries
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.Direction == "" || (opts.Direction != "asc" && opts.Direction != "desc") {
		opts.Direction = "asc"
	}

	// Build base WHERE clause (for count query - excludes cursor)
	baseWhereClauses := []string{"task_id = ?"}
	baseArgs := []any{taskID}
	if opts.Phase != "" {
		baseWhereClauses = append(baseWhereClauses, "phase = ?")
		baseArgs = append(baseArgs, opts.Phase)
	}
	baseWhereClause := strings.Join(baseWhereClauses, " AND ")

	// Build full WHERE clause (includes cursor for pagination)
	whereClauses := append([]string{}, baseWhereClauses...)
	args := append([]any{}, baseArgs...)

	// Cursor-based pagination
	if opts.Cursor > 0 {
		if opts.Direction == "asc" {
			whereClauses = append(whereClauses, "id > ?")
		} else {
			whereClauses = append(whereClauses, "id < ?")
		}
		args = append(args, opts.Cursor)
	}

	whereClause := strings.Join(whereClauses, " AND ")

	// Build ORDER BY clause
	orderBy := "id ASC"
	if opts.Direction == "desc" {
		orderBy = "id DESC"
	}

	// Fetch one extra to detect if there are more results
	fetchLimit := opts.Limit + 1

	// Execute query
	query := fmt.Sprintf(`
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE %s
		ORDER BY %s
		LIMIT ?
	`, whereClause, orderBy)

	args = append(args, fetchLimit)
	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("get paginated transcripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	transcripts, err := scanTranscripts(rows)
	if err != nil {
		return nil, PaginationResult{}, err
	}

	// Build pagination result
	var result PaginationResult
	result.HasMore = len(transcripts) > opts.Limit
	if result.HasMore {
		transcripts = transcripts[:opts.Limit]
	}

	// Get total count (using base WHERE without cursor filter)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transcripts WHERE %s", baseWhereClause)
	var totalCount int
	err = p.QueryRow(countQuery, baseArgs...).Scan(&totalCount)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("count transcripts: %w", err)
	}
	result.TotalCount = totalCount

	// Set cursors
	if len(transcripts) > 0 {
		if opts.Direction == "asc" {
			// Next cursor is the last ID
			if result.HasMore {
				lastID := transcripts[len(transcripts)-1].ID
				result.NextCursor = &lastID
			}
			// Prev cursor is the first ID (if we're not at the start)
			if opts.Cursor > 0 {
				firstID := transcripts[0].ID
				result.PrevCursor = &firstID
			}
		} else {
			// For desc order, reverse the logic
			if result.HasMore {
				lastID := transcripts[len(transcripts)-1].ID
				result.NextCursor = &lastID
			}
			if opts.Cursor > 0 {
				firstID := transcripts[0].ID
				result.PrevCursor = &firstID
			}
		}
	}

	return transcripts, result, nil
}

// GetPhaseSummary returns transcript counts grouped by phase.
func (p *ProjectDB) GetPhaseSummary(taskID string) ([]PhaseSummary, error) {
	rows, err := p.Query(`
		SELECT phase, COUNT(*) as count
		FROM transcripts
		WHERE task_id = ?
		GROUP BY phase
		ORDER BY MIN(timestamp)
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []PhaseSummary
	for rows.Next() {
		var s PhaseSummary
		if err := rows.Scan(&s.Phase, &s.TranscriptCount); err != nil {
			return nil, fmt.Errorf("scan phase summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase summaries: %w", err)
	}

	return summaries, nil
}

// GetTranscriptsByPhase retrieves transcripts for a specific task and phase.
func (p *ProjectDB) GetTranscriptsByPhase(taskID, phase string) ([]Transcript, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE task_id = ? AND phase = ?
		ORDER BY timestamp, id
	`, taskID, phase)
	if err != nil {
		return nil, fmt.Errorf("get transcripts by phase: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTranscripts(rows)
}

// GetTranscriptsBySession retrieves all transcripts for a specific session.
func (p *ProjectDB) GetTranscriptsBySession(sessionID string) ([]Transcript, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE session_id = ?
		ORDER BY timestamp, id
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts by session: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTranscripts(rows)
}

// GetLatestTranscript returns the most recent transcript for a task.
func (p *ProjectDB) GetLatestTranscript(taskID string) (*Transcript, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase, session_id, message_uuid, parent_uuid,
			   type, role, content, model,
			   input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			   tool_calls, tool_results, timestamp
		FROM transcripts
		WHERE task_id = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT 1
	`, taskID)

	var t Transcript
	var parentUUID, role, model, toolCalls, toolResults sql.NullString
	var timestamp int64

	err := row.Scan(
		&t.ID, &t.TaskID, &t.Phase, &t.SessionID, &t.MessageUUID, &parentUUID,
		&t.Type, &role, &t.Content, &model,
		&t.InputTokens, &t.OutputTokens, &t.CacheCreationTokens, &t.CacheReadTokens,
		&toolCalls, &toolResults, &timestamp,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest transcript: %w", err)
	}

	if parentUUID.Valid {
		t.ParentUUID = &parentUUID.String
	}
	t.Role = role.String
	t.Model = model.String
	t.ToolCalls = toolCalls.String
	t.ToolResults = toolResults.String
	t.Timestamp = time.UnixMilli(timestamp)

	return &t, nil
}

// scanTranscripts scans rows into Transcript slice.
func scanTranscripts(rows *sql.Rows) ([]Transcript, error) {
	var transcripts []Transcript
	for rows.Next() {
		var t Transcript
		var parentUUID, role, model, toolCalls, toolResults sql.NullString
		var timestamp int64

		if err := rows.Scan(
			&t.ID, &t.TaskID, &t.Phase, &t.SessionID, &t.MessageUUID, &parentUUID,
			&t.Type, &role, &t.Content, &model,
			&t.InputTokens, &t.OutputTokens, &t.CacheCreationTokens, &t.CacheReadTokens,
			&toolCalls, &toolResults, &timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan transcript: %w", err)
		}

		if parentUUID.Valid {
			t.ParentUUID = &parentUUID.String
		}
		t.Role = role.String
		t.Model = model.String
		t.ToolCalls = toolCalls.String
		t.ToolResults = toolResults.String
		t.Timestamp = time.UnixMilli(timestamp)

		transcripts = append(transcripts, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transcripts: %w", err)
	}
	return transcripts, nil
}

// TranscriptMatch represents a search result.
type TranscriptMatch struct {
	TaskID    string
	Phase     string
	SessionID string
	Snippet   string
	Rank      float64
}

// SearchTranscripts performs full-text search on transcript content.
func (p *ProjectDB) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	var rows *sql.Rows
	var err error

	if p.Dialect() == driver.DialectSQLite {
		sanitized := `"` + escapeQuotes(query) + `"`
		rows, err = p.Query(`
			SELECT task_id, phase, session_id,
			       snippet(transcripts_fts, 0, '<mark>', '</mark>', '...', 32), rank
			FROM transcripts_fts
			WHERE content MATCH ?
			ORDER BY rank
			LIMIT 50
		`, sanitized)
	} else {
		likePattern := "%" + query + "%"
		rows, err = p.Query(`
			SELECT task_id, phase, session_id,
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
		if err := rows.Scan(&m.TaskID, &m.Phase, &m.SessionID, &m.Snippet, &m.Rank); err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matches: %w", err)
	}

	return matches, nil
}

// escapeQuotes escapes double quotes for FTS5 MATCH queries.
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

// TokenUsageSummary contains aggregated token usage for a task or phase.
type TokenUsageSummary struct {
	TaskID             string
	Phase              string
	TotalInput         int
	TotalOutput        int
	TotalCacheCreation int
	TotalCacheRead     int
	MessageCount       int
}

// GetTaskTokenUsage returns aggregated token usage for a task.
func (p *ProjectDB) GetTaskTokenUsage(taskID string) (*TokenUsageSummary, error) {
	row := p.QueryRow(`
		SELECT task_id, '' as phase,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cache_creation_tokens), 0),
		       COALESCE(SUM(cache_read_tokens), 0),
		       COUNT(*)
		FROM transcripts
		WHERE task_id = ? AND type = 'assistant'
		GROUP BY task_id
	`, taskID)

	var s TokenUsageSummary
	err := row.Scan(&s.TaskID, &s.Phase, &s.TotalInput, &s.TotalOutput,
		&s.TotalCacheCreation, &s.TotalCacheRead, &s.MessageCount)
	if err == sql.ErrNoRows {
		return &TokenUsageSummary{TaskID: taskID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task token usage: %w", err)
	}
	return &s, nil
}

// GetPhaseTokenUsage returns aggregated token usage for a specific phase.
func (p *ProjectDB) GetPhaseTokenUsage(taskID, phase string) (*TokenUsageSummary, error) {
	row := p.QueryRow(`
		SELECT task_id, phase,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cache_creation_tokens), 0),
		       COALESCE(SUM(cache_read_tokens), 0),
		       COUNT(*)
		FROM transcripts
		WHERE task_id = ? AND phase = ? AND type = 'assistant'
		GROUP BY task_id, phase
	`, taskID, phase)

	var s TokenUsageSummary
	err := row.Scan(&s.TaskID, &s.Phase, &s.TotalInput, &s.TotalOutput,
		&s.TotalCacheCreation, &s.TotalCacheRead, &s.MessageCount)
	if err == sql.ErrNoRows {
		return &TokenUsageSummary{TaskID: taskID, Phase: phase}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get phase token usage: %w", err)
	}
	return &s, nil
}

// =============================================================================
// Todo Snapshots
// =============================================================================

// TodoSnapshot represents a point-in-time capture of a task's todo list.
type TodoSnapshot struct {
	ID          int64
	TaskID      string
	Phase       string
	MessageUUID string // Links to the transcript that triggered this
	Items       []TodoItem
	Timestamp   time.Time
}

// TodoItem represents a single item from Claude's TodoWrite tool.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // "pending", "in_progress", "completed"
	ActiveForm string `json:"active_form"`
}

// AddTodoSnapshot inserts a todo snapshot.
func (p *ProjectDB) AddTodoSnapshot(s *TodoSnapshot) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return fmt.Errorf("marshal todo items: %w", err)
	}

	result, err := p.Exec(`
		INSERT INTO todo_snapshots (task_id, phase, message_uuid, items, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`, s.TaskID, s.Phase, s.MessageUUID, string(items), s.Timestamp.UnixMilli())
	if err != nil {
		return fmt.Errorf("add todo snapshot: %w", err)
	}
	id, _ := result.LastInsertId()
	s.ID = id
	return nil
}

// GetLatestTodos returns the most recent todo snapshot for a task.
func (p *ProjectDB) GetLatestTodos(taskID string) (*TodoSnapshot, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase, message_uuid, items, timestamp
		FROM todo_snapshots
		WHERE task_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`, taskID)

	var s TodoSnapshot
	var messageUUID sql.NullString
	var itemsJSON string
	var timestamp int64

	err := row.Scan(&s.ID, &s.TaskID, &s.Phase, &messageUUID, &itemsJSON, &timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest todos: %w", err)
	}

	s.MessageUUID = messageUUID.String
	s.Timestamp = time.UnixMilli(timestamp)

	if err := json.Unmarshal([]byte(itemsJSON), &s.Items); err != nil {
		return nil, fmt.Errorf("unmarshal todo items: %w", err)
	}

	return &s, nil
}

// GetTodoHistory returns all todo snapshots for a task in chronological order.
func (p *ProjectDB) GetTodoHistory(taskID string) ([]TodoSnapshot, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, message_uuid, items, timestamp
		FROM todo_snapshots
		WHERE task_id = ?
		ORDER BY timestamp ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get todo history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var snapshots []TodoSnapshot
	for rows.Next() {
		var s TodoSnapshot
		var messageUUID sql.NullString
		var itemsJSON string
		var timestamp int64

		if err := rows.Scan(&s.ID, &s.TaskID, &s.Phase, &messageUUID, &itemsJSON, &timestamp); err != nil {
			return nil, fmt.Errorf("scan todo snapshot: %w", err)
		}

		s.MessageUUID = messageUUID.String
		s.Timestamp = time.UnixMilli(timestamp)

		if err := json.Unmarshal([]byte(itemsJSON), &s.Items); err != nil {
			return nil, fmt.Errorf("unmarshal todo items: %w", err)
		}

		snapshots = append(snapshots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate todo snapshots: %w", err)
	}

	return snapshots, nil
}

// =============================================================================
// Usage Metrics
// =============================================================================

// UsageMetric represents aggregated usage data for analytics.
type UsageMetric struct {
	ID          int64
	TaskID      string
	Phase       string
	Model       string
	ProjectPath string

	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	CostUSD             float64
	DurationMs          int64
	Timestamp           time.Time
}

// AddUsageMetric inserts a usage metric entry.
func (p *ProjectDB) AddUsageMetric(m *UsageMetric) error {
	result, err := p.Exec(`
		INSERT INTO usage_metrics (
			task_id, phase, model, project_path,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			cost_usd, duration_ms, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.TaskID, m.Phase, m.Model, m.ProjectPath,
		m.InputTokens, m.OutputTokens, m.CacheCreationTokens, m.CacheReadTokens,
		m.CostUSD, m.DurationMs, m.Timestamp.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("add usage metric: %w", err)
	}
	id, _ := result.LastInsertId()
	m.ID = id
	return nil
}

// MetricsSummary contains aggregated metrics for a time period.
type MetricsSummary struct {
	TotalCost   float64
	TotalInput  int
	TotalOutput int
	TaskCount   int
	ByModel     map[string]ModelMetrics
}

// ModelMetrics contains per-model aggregated metrics.
type ModelMetrics struct {
	Model        string
	Cost         float64
	InputTokens  int
	OutputTokens int
	TaskCount    int
}

// GetMetricsSummary returns aggregated metrics since the given time.
func (p *ProjectDB) GetMetricsSummary(since time.Time) (*MetricsSummary, error) {
	// Get totals
	row := p.QueryRow(`
		SELECT COALESCE(SUM(cost_usd), 0),
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COUNT(DISTINCT task_id)
		FROM usage_metrics
		WHERE timestamp >= ?
	`, since.UnixMilli())

	summary := &MetricsSummary{
		ByModel: make(map[string]ModelMetrics),
	}
	if err := row.Scan(&summary.TotalCost, &summary.TotalInput, &summary.TotalOutput, &summary.TaskCount); err != nil {
		return nil, fmt.Errorf("get metrics summary: %w", err)
	}

	// Get per-model breakdown
	rows, err := p.Query(`
		SELECT model,
		       COALESCE(SUM(cost_usd), 0),
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COUNT(DISTINCT task_id)
		FROM usage_metrics
		WHERE timestamp >= ?
		GROUP BY model
	`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("get model metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var m ModelMetrics
		if err := rows.Scan(&m.Model, &m.Cost, &m.InputTokens, &m.OutputTokens, &m.TaskCount); err != nil {
			return nil, fmt.Errorf("scan model metrics: %w", err)
		}
		summary.ByModel[m.Model] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model metrics: %w", err)
	}

	return summary, nil
}

// DailyMetrics contains metrics aggregated by day.
type DailyMetrics struct {
	Date        string
	TotalInput  int
	TotalOutput int
	TotalCost   float64
	TaskCount   int
	ModelsUsed  int
}

// GetDailyMetrics returns daily aggregated metrics since the given time.
func (p *ProjectDB) GetDailyMetrics(since time.Time) ([]DailyMetrics, error) {
	rows, err := p.Query(`
		SELECT DATE(timestamp/1000, 'unixepoch') as date,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0),
		       COUNT(DISTINCT task_id),
		       COUNT(DISTINCT model)
		FROM usage_metrics
		WHERE timestamp >= ?
		GROUP BY date
		ORDER BY date
	`, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("get daily metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var metrics []DailyMetrics
	for rows.Next() {
		var m DailyMetrics
		if err := rows.Scan(&m.Date, &m.TotalInput, &m.TotalOutput, &m.TotalCost, &m.TaskCount, &m.ModelsUsed); err != nil {
			return nil, fmt.Errorf("scan daily metrics: %w", err)
		}
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily metrics: %w", err)
	}

	return metrics, nil
}

// GetMetricsByModel returns metrics for a specific model since the given time.
func (p *ProjectDB) GetMetricsByModel(model string, since time.Time) ([]UsageMetric, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, model, project_path,
		       input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		       cost_usd, duration_ms, timestamp
		FROM usage_metrics
		WHERE model = ? AND timestamp >= ?
		ORDER BY timestamp DESC
	`, model, since.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("get metrics by model: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanUsageMetrics(rows)
}

// CleanupOldTranscripts deletes transcripts older than the given duration.
// Returns the number of deleted rows.
func (p *ProjectDB) CleanupOldTranscripts(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()

	result, err := p.Exec(`DELETE FROM transcripts WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup old transcripts: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// GetTaskMetrics returns all metrics for a specific task.
func (p *ProjectDB) GetTaskMetrics(taskID string) ([]UsageMetric, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, model, project_path,
		       input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		       cost_usd, duration_ms, timestamp
		FROM usage_metrics
		WHERE task_id = ?
		ORDER BY timestamp
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanUsageMetrics(rows)
}

func scanUsageMetrics(rows *sql.Rows) ([]UsageMetric, error) {
	var metrics []UsageMetric
	for rows.Next() {
		var m UsageMetric
		var timestamp int64
		if err := rows.Scan(
			&m.ID, &m.TaskID, &m.Phase, &m.Model, &m.ProjectPath,
			&m.InputTokens, &m.OutputTokens, &m.CacheCreationTokens, &m.CacheReadTokens,
			&m.CostUSD, &m.DurationMs, &timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan usage metric: %w", err)
		}
		m.Timestamp = time.UnixMilli(timestamp)
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage metrics: %w", err)
	}
	return metrics, nil
}

// TranscriptPaginationOpts configures transcript pagination and filtering.
type TranscriptPaginationOpts struct {
	Phase     string // Filter by phase (optional)
	Cursor    int64  // Cursor for pagination (transcript ID, 0 = start)
	Limit     int    // Max results (default: 50, max: 200)
	Direction string // 'asc' | 'desc' (default: asc)
}

// PaginationResult contains pagination metadata.
type PaginationResult struct {
	NextCursor *int64 `json:"next_cursor,omitempty"`
	PrevCursor *int64 `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	TotalCount int    `json:"total_count"`
}

// PhaseSummary contains transcript count for a single phase.
type PhaseSummary struct {
	Phase           string `json:"phase"`
	TranscriptCount int    `json:"transcript_count"`
}

// AgentStats contains per-agent statistics.
type AgentStats struct {
	Model              string     `json:"model"`
	TokensToday        int        `json:"tokens_today"`
	TasksDoneToday     int        `json:"tasks_done_today"`
	TasksDoneTotal     int        `json:"tasks_done_total"`
	SuccessRate        float64    `json:"success_rate"`
	AvgTaskTimeSeconds int        `json:"avg_task_time_seconds"`
	IsActive           bool       `json:"is_active"` // Has running tasks
	LastActivity       *time.Time `json:"last_activity,omitempty"`
}

// GetAgentStats returns per-model/agent statistics.
// The 'today' parameter should be midnight local time for accurate "today" stats.
func (p *ProjectDB) GetAgentStats(today time.Time) (map[string]*AgentStats, error) {
	stats := make(map[string]*AgentStats)

	// Get total tasks completed per model (all time)
	rows, err := p.Query(`
		SELECT session_model,
		       COUNT(*) as total_completed,
		       SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as successful,
		       SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
		       AVG(CASE
		           WHEN completed_at IS NOT NULL AND started_at IS NOT NULL
		           THEN (julianday(completed_at) - julianday(started_at)) * 86400
		           ELSE NULL
		       END) as avg_time_seconds,
		       MAX(completed_at) as last_activity
		FROM tasks
		WHERE session_model IS NOT NULL
		  AND session_model != ''
		  AND status IN ('completed', 'failed', 'resolved')
		GROUP BY session_model
	`)
	if err != nil {
		return nil, fmt.Errorf("get agent stats total: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var model string
		var totalCompleted, successful, failed int
		var avgTime sql.NullFloat64
		var lastActivity sql.NullString

		if err := rows.Scan(&model, &totalCompleted, &successful, &failed, &avgTime, &lastActivity); err != nil {
			return nil, fmt.Errorf("scan agent stats: %w", err)
		}

		stat := &AgentStats{
			Model:          model,
			TasksDoneTotal: successful,
		}

		// Calculate success rate
		total := successful + failed
		if total > 0 {
			stat.SuccessRate = float64(successful) / float64(total)
		}

		// Average task time
		if avgTime.Valid {
			stat.AvgTaskTimeSeconds = int(avgTime.Float64)
		}

		// Last activity
		if lastActivity.Valid {
			t, err := time.Parse(time.RFC3339, lastActivity.String)
			if err == nil {
				stat.LastActivity = &t
			}
		}

		stats[model] = stat
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent stats: %w", err)
	}

	// Get today's completed tasks per model
	rows2, err := p.Query(`
		SELECT session_model,
		       COUNT(*) as tasks_today
		FROM tasks
		WHERE session_model IS NOT NULL
		  AND session_model != ''
		  AND status = 'completed'
		  AND completed_at >= ?
		GROUP BY session_model
	`, today.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get agent stats today: %w", err)
	}
	defer func() { _ = rows2.Close() }()

	for rows2.Next() {
		var model string
		var tasksToday int
		if err := rows2.Scan(&model, &tasksToday); err != nil {
			return nil, fmt.Errorf("scan today stats: %w", err)
		}
		if stat, ok := stats[model]; ok {
			stat.TasksDoneToday = tasksToday
		} else {
			stats[model] = &AgentStats{
				Model:          model,
				TasksDoneToday: tasksToday,
			}
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("iterate today stats: %w", err)
	}

	// Get today's token usage per model from usage_metrics
	todayMs := today.UnixMilli()
	rows3, err := p.Query(`
		SELECT model,
		       SUM(input_tokens + output_tokens) as total_tokens
		FROM usage_metrics
		WHERE timestamp >= ?
		GROUP BY model
	`, todayMs)
	if err != nil {
		return nil, fmt.Errorf("get token stats: %w", err)
	}
	defer func() { _ = rows3.Close() }()

	for rows3.Next() {
		var model string
		var tokensToday int
		if err := rows3.Scan(&model, &tokensToday); err != nil {
			return nil, fmt.Errorf("scan token stats: %w", err)
		}
		if stat, ok := stats[model]; ok {
			stat.TokensToday = tokensToday
		} else {
			stats[model] = &AgentStats{
				Model:       model,
				TokensToday: tokensToday,
			}
		}
	}
	if err := rows3.Err(); err != nil {
		return nil, fmt.Errorf("iterate token stats: %w", err)
	}

	// Check which models have running tasks
	rows4, err := p.Query(`
		SELECT DISTINCT session_model
		FROM tasks
		WHERE session_model IS NOT NULL
		  AND session_model != ''
		  AND status = 'running'
	`)
	if err != nil {
		return nil, fmt.Errorf("get running tasks: %w", err)
	}
	defer func() { _ = rows4.Close() }()

	for rows4.Next() {
		var model string
		if err := rows4.Scan(&model); err != nil {
			return nil, fmt.Errorf("scan running model: %w", err)
		}
		if stat, ok := stats[model]; ok {
			stat.IsActive = true
		} else {
			stats[model] = &AgentStats{
				Model:    model,
				IsActive: true,
			}
		}
	}
	if err := rows4.Err(); err != nil {
		return nil, fmt.Errorf("iterate running tasks: %w", err)
	}

	return stats, nil
}
