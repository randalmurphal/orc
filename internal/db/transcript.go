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
	ID          int64
	TaskID      string
	Phase       string
	SessionID   string  // Claude session UUID
	MessageUUID string  // Individual message UUID
	ParentUUID  *string // Links to parent message (threading)
	Type        string  // "user", "assistant", "queue-operation"
	Role        string  // from message.role
	Content     string  // Full content JSON (preserves structure)
	Model       string  // Model used (assistant messages only)

	// Per-message token tracking
	InputTokens           int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int

	// Tool information
	ToolCalls   string // JSON array of tool_use blocks
	ToolResults string // JSON of toolUseResult metadata

	Timestamp time.Time
}

// AddTranscript inserts a single transcript entry.
func (p *ProjectDB) AddTranscript(t *Transcript) error {
	result, err := p.Exec(`
		INSERT INTO transcripts (
			task_id, phase, session_id, message_uuid, parent_uuid,
			type, role, content, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			tool_calls, tool_results, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		t.TaskID, t.Phase, t.SessionID, t.MessageUUID, t.ParentUUID,
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
			task_id, phase, session_id, message_uuid, parent_uuid,
			type, role, content, model,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			tool_calls, tool_results, timestamp
		) VALUES `)

		args := make([]any, 0, len(transcripts)*16)
		for i, t := range transcripts {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args,
				t.TaskID, t.Phase, t.SessionID, t.MessageUUID, t.ParentUUID,
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

// TokenUsageSummary contains aggregated token usage for a task or phase.
type TokenUsageSummary struct {
	TaskID              string
	Phase               string
	TotalInput          int
	TotalOutput         int
	TotalCacheCreation  int
	TotalCacheRead      int
	MessageCount        int
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
	MessageUUID string    // Links to the transcript that triggered this
	Items       []TodoItem
	Timestamp   time.Time
}

// TodoItem represents a single item from Claude's TodoWrite tool.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`      // "pending", "in_progress", "completed"
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

	InputTokens           int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int
	CostUSD               float64
	DurationMs            int64
	Timestamp             time.Time
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
	TotalCost    float64
	TotalInput   int
	TotalOutput  int
	TaskCount    int
	ByModel      map[string]ModelMetrics
}

// ModelMetrics contains per-model aggregated metrics.
type ModelMetrics struct {
	Model       string
	Cost        float64
	InputTokens int
	OutputTokens int
	TaskCount   int
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
