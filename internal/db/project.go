package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// TxRunner provides a transactional execution interface.
// This allows operations to run within a transaction context,
// ensuring atomicity of multi-table operations.
type TxRunner interface {
	// RunInTx executes the given function within a transaction.
	// If fn returns an error, the transaction is rolled back.
	// If fn returns nil, the transaction is committed.
	RunInTx(ctx context.Context, fn func(tx *TxOps) error) error
}

// TxOps provides database operations within a transaction.
// It wraps a driver.Tx to provide the same interface as ProjectDB
// but executes all operations within the transaction.
// The context is stored and used for all operations, enabling cancellation
// and timeout propagation through the entire transaction.
type TxOps struct {
	tx      driver.Tx
	dialect driver.Dialect
	ctx     context.Context
}

// Exec executes a query within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) Exec(query string, args ...any) (sql.Result, error) {
	return t.tx.Exec(t.ctx, query, args...)
}

// Query executes a query that returns rows within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.Query(t.ctx, query, args...)
}

// QueryRow executes a query that returns at most one row within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) QueryRow(query string, args ...any) *sql.Row {
	return t.tx.QueryRow(t.ctx, query, args...)
}

// Context returns the context associated with this transaction.
func (t *TxOps) Context() context.Context {
	return t.ctx
}

// Dialect returns the database dialect.
func (t *TxOps) Dialect() driver.Dialect {
	return t.dialect
}

// ProjectDB provides operations on a project database (.orc/orc.db).
type ProjectDB struct {
	*DB
}

// OpenProject opens the project database at {projectPath}/.orc/orc.db using SQLite.
func OpenProject(projectPath string) (*ProjectDB, error) {
	path := filepath.Join(projectPath, ".orc", "orc.db")
	db, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("project"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate project db: %w", err)
	}

	return &ProjectDB{DB: db}, nil
}

// OpenProjectWithDialect opens the project database with a specific dialect.
// For SQLite, dsn is the file path. For PostgreSQL, dsn is the connection string.
func OpenProjectWithDialect(dsn string, dialect driver.Dialect) (*ProjectDB, error) {
	db, err := OpenWithDialect(dsn, dialect)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("project"); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate project db: %w", err)
	}

	return &ProjectDB{DB: db}, nil
}

// RunInTx executes the given function within a database transaction.
// If fn returns an error, the transaction is rolled back.
// If fn returns nil, the transaction is committed.
// The context is propagated to all database operations within the transaction,
// enabling proper cancellation and timeout handling.
func (p *ProjectDB) RunInTx(ctx context.Context, fn func(tx *TxOps) error) error {
	tx, err := p.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txOps := &TxOps{
		tx:      tx,
		dialect: p.Dialect(),
		ctx:     ctx,
	}

	if err := fn(txOps); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Ensure ProjectDB implements TxRunner
var _ TxRunner = (*ProjectDB)(nil)

// Detection stores project detection results.
type Detection struct {
	ID          int64
	Language    string
	Frameworks  []string
	BuildTools  []string
	HasTests    bool
	TestCommand string
	LintCommand string
	DetectedAt  time.Time
}

// StoreDetection saves detection results.
func (p *ProjectDB) StoreDetection(d *Detection) error {
	frameworks, err := json.Marshal(d.Frameworks)
	if err != nil {
		return fmt.Errorf("marshal frameworks: %w", err)
	}
	buildTools, err2 := json.Marshal(d.BuildTools)
	if err2 != nil {
		return fmt.Errorf("marshal build_tools: %w", err2)
	}

	hasTests := 0
	if d.HasTests {
		hasTests = 1
	}

	// Use dialect-aware upsert
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			INSERT OR REPLACE INTO detection (id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, datetime('now'))
		`
	} else {
		query = `
			INSERT INTO detection (id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at)
			VALUES (1, $1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (id) DO UPDATE SET
				language = EXCLUDED.language,
				frameworks = EXCLUDED.frameworks,
				build_tools = EXCLUDED.build_tools,
				has_tests = EXCLUDED.has_tests,
				test_command = EXCLUDED.test_command,
				lint_command = EXCLUDED.lint_command,
				detected_at = NOW()
		`
	}

	_, err = p.Exec(query, d.Language, string(frameworks), string(buildTools), hasTests, d.TestCommand, d.LintCommand)
	if err != nil {
		return fmt.Errorf("store detection: %w", err)
	}
	return nil
}

// LoadDetection retrieves the stored detection results.
func (p *ProjectDB) LoadDetection() (*Detection, error) {
	row := p.QueryRow(`
		SELECT id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at
		FROM detection WHERE id = 1
	`)

	var d Detection
	var frameworks, buildTools, detectedAt string
	var hasTests int
	if err := row.Scan(&d.ID, &d.Language, &frameworks, &buildTools, &hasTests, &d.TestCommand, &d.LintCommand, &detectedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load detection: %w", err)
	}

	d.HasTests = hasTests == 1
	if err := json.Unmarshal([]byte(frameworks), &d.Frameworks); err != nil {
		return nil, fmt.Errorf("unmarshal frameworks: %w", err)
	}
	if err := json.Unmarshal([]byte(buildTools), &d.BuildTools); err != nil {
		return nil, fmt.Errorf("unmarshal build_tools: %w", err)
	}
	if t, err := time.Parse("2006-01-02 15:04:05", detectedAt); err == nil {
		d.DetectedAt = t
	}

	return &d, nil
}

// Task represents a task stored in the database.
type Task struct {
	ID           string
	Title        string
	Description  string
	Weight       string
	Status       string
	StateStatus  string // State status: pending, running, completed, failed, paused, interrupted, skipped
	CurrentPhase string
	Branch       string
	WorktreePath string
	Queue        string // "active" or "backlog"
	Priority     string // "critical", "high", "normal", "low"
	Category     string // "feature", "bug", "refactor", "chore", "docs", "test"
	InitiativeID string // Links this task to an initiative (e.g., INIT-001)
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	TotalCostUSD float64
	Metadata     string // JSON object: {"key": "value", ...}
	RetryContext string // JSON: state.RetryContext serialized

	// Execution tracking for orphan detection
	ExecutorPID       int        // Process ID of executor
	ExecutorHostname  string     // Hostname running the executor
	ExecutorStartedAt *time.Time // When execution started
	LastHeartbeat     *time.Time // Last heartbeat update
}

// SaveTask creates or updates a task.
func (p *ProjectDB) SaveTask(t *Task) error {
	var startedAt, completedAt *string
	if t.StartedAt != nil {
		s := t.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	// Default queue, priority, and category if not set
	queue := t.Queue
	if queue == "" {
		queue = "active"
	}
	priority := t.Priority
	if priority == "" {
		priority = "normal"
	}
	category := t.Category
	if category == "" {
		category = "feature"
	}

	// Default state_status if not set
	stateStatus := t.StateStatus
	if stateStatus == "" {
		stateStatus = "pending"
	}

	// Format execution tracking timestamps
	var executorStartedAt, lastHeartbeat *string
	if t.ExecutorStartedAt != nil {
		s := t.ExecutorStartedAt.Format(time.RFC3339)
		executorStartedAt = &s
	}
	if t.LastHeartbeat != nil {
		s := t.LastHeartbeat.Format(time.RFC3339)
		lastHeartbeat = &s
	}

	_, err := p.Exec(`
		INSERT INTO tasks (id, title, description, weight, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, created_at, started_at, completed_at, total_cost_usd, metadata, retry_context, executor_pid, executor_hostname, executor_started_at, last_heartbeat)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			weight = excluded.weight,
			status = excluded.status,
			state_status = excluded.state_status,
			current_phase = excluded.current_phase,
			branch = excluded.branch,
			worktree_path = excluded.worktree_path,
			queue = excluded.queue,
			priority = excluded.priority,
			category = excluded.category,
			initiative_id = excluded.initiative_id,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			total_cost_usd = excluded.total_cost_usd,
			metadata = excluded.metadata,
			retry_context = excluded.retry_context,
			executor_pid = excluded.executor_pid,
			executor_hostname = excluded.executor_hostname,
			executor_started_at = excluded.executor_started_at,
			last_heartbeat = excluded.last_heartbeat
	`, t.ID, t.Title, t.Description, t.Weight, t.Status, stateStatus, t.CurrentPhase, t.Branch, t.WorktreePath,
		queue, priority, category, t.InitiativeID, t.CreatedAt.Format(time.RFC3339), startedAt, completedAt, t.TotalCostUSD, t.Metadata, t.RetryContext,
		t.ExecutorPID, t.ExecutorHostname, executorStartedAt, lastHeartbeat)
	if err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// GetTask retrieves a task by ID.
func (p *ProjectDB) GetTask(id string) (*Task, error) {
	row := p.QueryRow(`
		SELECT id, title, description, weight, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, created_at, started_at, completed_at, total_cost_usd, metadata, retry_context, executor_pid, executor_hostname, executor_started_at, last_heartbeat
		FROM tasks WHERE id = ?
	`, id)

	t, err := scanTask(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	return t, nil
}

// DeleteTask removes a task and its phases/transcripts.
func (p *ProjectDB) DeleteTask(id string) error {
	_, err := p.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// ListOpts provides filtering and pagination options.
type ListOpts struct {
	Status   string
	Queue    string // "active", "backlog", or empty for all
	Priority string // "critical", "high", "normal", "low", or empty for all
	Limit    int
	Offset   int
}

// ListTasks returns tasks matching the given options.
func (p *ProjectDB) ListTasks(opts ListOpts) ([]Task, int, error) {
	// Build WHERE clause
	var whereClauses []string
	var countArgs []any
	if opts.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		countArgs = append(countArgs, opts.Status)
	}
	if opts.Queue != "" {
		whereClauses = append(whereClauses, "queue = ?")
		countArgs = append(countArgs, opts.Queue)
	}
	if opts.Priority != "" {
		whereClauses = append(whereClauses, "priority = ?")
		countArgs = append(countArgs, opts.Priority)
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count total
	var total int
	if err := p.QueryRow("SELECT COUNT(*) FROM tasks"+whereClause, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// Query tasks
	query := `
		SELECT id, title, description, weight, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, created_at, started_at, completed_at, total_cost_usd, metadata, retry_context, executor_pid, executor_hostname, executor_started_at, last_heartbeat
		FROM tasks
	` + whereClause + " ORDER BY created_at DESC"

	args := make([]any, len(countArgs))
	copy(args, countArgs)

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		t, err := scanTaskRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, total, nil
}

// scanTask scans a single task from a Row.
func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt sql.NullString
	var description, stateStatus, currentPhase, branch, worktreePath, queue, priority, category, initiativeID, metadata, retryContext sql.NullString
	var executorPID sql.NullInt64
	var executorHostname, executorStartedAt, lastHeartbeat sql.NullString

	if err := row.Scan(&t.ID, &t.Title, &description, &t.Weight, &t.Status, &stateStatus, &currentPhase, &branch, &worktreePath,
		&queue, &priority, &category, &initiativeID, &createdAt, &startedAt, &completedAt, &t.TotalCostUSD, &metadata, &retryContext,
		&executorPID, &executorHostname, &executorStartedAt, &lastHeartbeat); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
	}
	if stateStatus.Valid {
		t.StateStatus = stateStatus.String
	} else {
		t.StateStatus = "pending" // Default
	}
	if currentPhase.Valid {
		t.CurrentPhase = currentPhase.String
	}
	if branch.Valid {
		t.Branch = branch.String
	}
	if worktreePath.Valid {
		t.WorktreePath = worktreePath.String
	}
	if queue.Valid {
		t.Queue = queue.String
	} else {
		t.Queue = "active" // Default
	}
	if priority.Valid {
		t.Priority = priority.String
	} else {
		t.Priority = "normal" // Default
	}
	if category.Valid {
		t.Category = category.String
	} else {
		t.Category = "feature" // Default
	}
	if initiativeID.Valid {
		t.InitiativeID = initiativeID.String
	}
	if metadata.Valid {
		t.Metadata = metadata.String
	}
	if retryContext.Valid {
		t.RetryContext = retryContext.String
	}

	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		t.CreatedAt = ts
	}
	if startedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
			t.StartedAt = &ts
		}
	}
	if completedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
			t.CompletedAt = &ts
		}
	}

	// Execution tracking fields
	if executorPID.Valid {
		t.ExecutorPID = int(executorPID.Int64)
	}
	if executorHostname.Valid {
		t.ExecutorHostname = executorHostname.String
	}
	if executorStartedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, executorStartedAt.String); err == nil {
			t.ExecutorStartedAt = &ts
		}
	}
	if lastHeartbeat.Valid {
		if ts, err := time.Parse(time.RFC3339, lastHeartbeat.String); err == nil {
			t.LastHeartbeat = &ts
		}
	}

	return &t, nil
}

// scanTaskRows scans a task from Rows.
func scanTaskRows(rows *sql.Rows) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt sql.NullString
	var description, stateStatus, currentPhase, branch, worktreePath, queue, priority, category, initiativeID, metadata, retryContext sql.NullString
	var executorPID sql.NullInt64
	var executorHostname, executorStartedAt, lastHeartbeat sql.NullString

	if err := rows.Scan(&t.ID, &t.Title, &description, &t.Weight, &t.Status, &stateStatus, &currentPhase, &branch, &worktreePath,
		&queue, &priority, &category, &initiativeID, &createdAt, &startedAt, &completedAt, &t.TotalCostUSD, &metadata, &retryContext,
		&executorPID, &executorHostname, &executorStartedAt, &lastHeartbeat); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
	}
	if stateStatus.Valid {
		t.StateStatus = stateStatus.String
	} else {
		t.StateStatus = "pending" // Default
	}
	if currentPhase.Valid {
		t.CurrentPhase = currentPhase.String
	}
	if branch.Valid {
		t.Branch = branch.String
	}
	if worktreePath.Valid {
		t.WorktreePath = worktreePath.String
	}
	if queue.Valid {
		t.Queue = queue.String
	} else {
		t.Queue = "active" // Default
	}
	if priority.Valid {
		t.Priority = priority.String
	} else {
		t.Priority = "normal" // Default
	}
	if category.Valid {
		t.Category = category.String
	} else {
		t.Category = "feature" // Default
	}
	if initiativeID.Valid {
		t.InitiativeID = initiativeID.String
	}
	if metadata.Valid {
		t.Metadata = metadata.String
	}
	if retryContext.Valid {
		t.RetryContext = retryContext.String
	}

	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		t.CreatedAt = ts
	}
	if startedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
			t.StartedAt = &ts
		}
	}
	if completedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
			t.CompletedAt = &ts
		}
	}

	// Execution tracking fields
	if executorPID.Valid {
		t.ExecutorPID = int(executorPID.Int64)
	}
	if executorHostname.Valid {
		t.ExecutorHostname = executorHostname.String
	}
	if executorStartedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, executorStartedAt.String); err == nil {
			t.ExecutorStartedAt = &ts
		}
	}
	if lastHeartbeat.Valid {
		if ts, err := time.Parse(time.RFC3339, lastHeartbeat.String); err == nil {
			t.LastHeartbeat = &ts
		}
	}

	return &t, nil
}

// Phase represents a phase execution state.
type Phase struct {
	TaskID       string
	PhaseID      string
	Status       string
	Iterations   int
	StartedAt    *time.Time
	CompletedAt  *time.Time
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	ErrorMessage string
	CommitSHA    string
	SkipReason   string
}

// SavePhase creates or updates a phase.
func (p *ProjectDB) SavePhase(ph *Phase) error {
	var startedAt, completedAt *string
	if ph.StartedAt != nil {
		s := ph.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if ph.CompletedAt != nil {
		s := ph.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := p.Exec(`
		INSERT INTO phases (task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			error_message = excluded.error_message,
			commit_sha = excluded.commit_sha,
			skip_reason = excluded.skip_reason
	`, ph.TaskID, ph.PhaseID, ph.Status, ph.Iterations, startedAt, completedAt,
		ph.InputTokens, ph.OutputTokens, ph.CostUSD, ph.ErrorMessage, ph.CommitSHA, ph.SkipReason)
	if err != nil {
		return fmt.Errorf("save phase: %w", err)
	}
	return nil
}

// GetPhases retrieves all phases for a task.
func (p *ProjectDB) GetPhases(taskID string) ([]Phase, error) {
	rows, err := p.Query(`
		SELECT task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason
		FROM phases WHERE task_id = ?
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get phases: %w", err)
	}
	defer rows.Close()

	var phases []Phase
	for rows.Next() {
		var ph Phase
		var startedAt, completedAt, errorMsg, commitSHA, skipReason sql.NullString
		if err := rows.Scan(&ph.TaskID, &ph.PhaseID, &ph.Status, &ph.Iterations, &startedAt, &completedAt,
			&ph.InputTokens, &ph.OutputTokens, &ph.CostUSD, &errorMsg, &commitSHA, &skipReason); err != nil {
			return nil, fmt.Errorf("scan phase: %w", err)
		}
		if startedAt.Valid {
			if ts, err := time.Parse(time.RFC3339, startedAt.String); err == nil {
				ph.StartedAt = &ts
			}
		}
		if completedAt.Valid {
			if ts, err := time.Parse(time.RFC3339, completedAt.String); err == nil {
				ph.CompletedAt = &ts
			}
		}
		if errorMsg.Valid {
			ph.ErrorMessage = errorMsg.String
		}
		if commitSHA.Valid {
			ph.CommitSHA = commitSHA.String
		}
		if skipReason.Valid {
			ph.SkipReason = skipReason.String
		}
		phases = append(phases, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phases: %w", err)
	}

	return phases, nil
}

// ClearPhases removes all phases for a task.
func (p *ProjectDB) ClearPhases(taskID string) error {
	_, err := p.Exec("DELETE FROM phases WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("clear phases: %w", err)
	}
	return nil
}

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

// GetTranscripts retrieves all transcripts for a task.
func (p *ProjectDB) GetTranscripts(taskID string) ([]Transcript, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, iteration, role, content, timestamp
		FROM transcripts WHERE task_id = ? ORDER BY id
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}
	defer rows.Close()

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
	defer rows.Close()

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

// escapeQuotes escapes double quotes for FTS5 literal matching.
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

// Initiative represents an initiative stored in the database.
type Initiative struct {
	ID               string
	Title            string
	Status           string
	OwnerInitials    string
	OwnerDisplayName string
	OwnerEmail       string
	Vision           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// InitiativeDecision represents a decision within an initiative.
type InitiativeDecision struct {
	ID           string
	InitiativeID string
	Decision     string
	Rationale    string
	DecidedBy    string
	DecidedAt    time.Time
}

// SaveInitiative creates or updates an initiative.
func (p *ProjectDB) SaveInitiative(i *Initiative) error {
	now := time.Now().Format(time.RFC3339)
	if i.CreatedAt.IsZero() {
		i.CreatedAt = time.Now()
	}

	_, err := p.Exec(`
		INSERT INTO initiatives (id, title, status, owner_initials, owner_display_name, owner_email, vision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			owner_initials = excluded.owner_initials,
			owner_display_name = excluded.owner_display_name,
			owner_email = excluded.owner_email,
			vision = excluded.vision,
			updated_at = excluded.updated_at
	`, i.ID, i.Title, i.Status, i.OwnerInitials, i.OwnerDisplayName, i.OwnerEmail, i.Vision,
		i.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}
	return nil
}

// GetInitiative retrieves an initiative by ID.
func (p *ProjectDB) GetInitiative(id string) (*Initiative, error) {
	row := p.QueryRow(`
		SELECT id, title, status, owner_initials, owner_display_name, owner_email, vision, created_at, updated_at
		FROM initiatives WHERE id = ?
	`, id)

	var i Initiative
	var ownerInitials, ownerDisplayName, ownerEmail, vision sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&i.ID, &i.Title, &i.Status, &ownerInitials, &ownerDisplayName, &ownerEmail, &vision, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get initiative %s: %w", id, err)
	}

	if ownerInitials.Valid {
		i.OwnerInitials = ownerInitials.String
	}
	if ownerDisplayName.Valid {
		i.OwnerDisplayName = ownerDisplayName.String
	}
	if ownerEmail.Valid {
		i.OwnerEmail = ownerEmail.String
	}
	if vision.Valid {
		i.Vision = vision.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		i.CreatedAt = ts
	} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		i.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		i.UpdatedAt = ts
	} else if ts, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
		i.UpdatedAt = ts
	}

	return &i, nil
}

// ListInitiatives returns initiatives matching the given options.
func (p *ProjectDB) ListInitiatives(opts ListOpts) ([]Initiative, error) {
	query := `SELECT id, title, status, owner_initials, owner_display_name, owner_email, vision, created_at, updated_at FROM initiatives`
	args := []any{}

	if opts.Status != "" {
		query += " WHERE status = ?"
		args = append(args, opts.Status)
	}
	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list initiatives: %w", err)
	}
	defer rows.Close()

	var initiatives []Initiative
	for rows.Next() {
		var i Initiative
		var ownerInitials, ownerDisplayName, ownerEmail, vision sql.NullString
		var createdAt, updatedAt string

		if err := rows.Scan(&i.ID, &i.Title, &i.Status, &ownerInitials, &ownerDisplayName, &ownerEmail, &vision, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan initiative: %w", err)
		}

		if ownerInitials.Valid {
			i.OwnerInitials = ownerInitials.String
		}
		if ownerDisplayName.Valid {
			i.OwnerDisplayName = ownerDisplayName.String
		}
		if ownerEmail.Valid {
			i.OwnerEmail = ownerEmail.String
		}
		if vision.Valid {
			i.Vision = vision.String
		}
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			i.CreatedAt = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			i.CreatedAt = ts
		}
		if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			i.UpdatedAt = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
			i.UpdatedAt = ts
		}

		initiatives = append(initiatives, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate initiatives: %w", err)
	}

	return initiatives, nil
}

// DeleteInitiative removes an initiative and its decisions/tasks.
func (p *ProjectDB) DeleteInitiative(id string) error {
	_, err := p.Exec("DELETE FROM initiatives WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete initiative: %w", err)
	}
	return nil
}

// AddInitiativeDecision adds a decision to an initiative.
func (p *ProjectDB) AddInitiativeDecision(d *InitiativeDecision) error {
	_, err := p.Exec(`
		INSERT INTO initiative_decisions (id, initiative_id, decision, rationale, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, d.ID, d.InitiativeID, d.Decision, d.Rationale, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add initiative decision: %w", err)
	}
	return nil
}

// GetInitiativeDecisions retrieves all decisions for an initiative.
func (p *ProjectDB) GetInitiativeDecisions(initiativeID string) ([]InitiativeDecision, error) {
	rows, err := p.Query(`
		SELECT id, initiative_id, decision, rationale, decided_by, decided_at
		FROM initiative_decisions WHERE initiative_id = ? ORDER BY decided_at
	`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("get initiative decisions: %w", err)
	}
	defer rows.Close()

	var decisions []InitiativeDecision
	for rows.Next() {
		var d InitiativeDecision
		var rationale, decidedBy sql.NullString
		var decidedAt string

		if err := rows.Scan(&d.ID, &d.InitiativeID, &d.Decision, &rationale, &decidedBy, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}

		if rationale.Valid {
			d.Rationale = rationale.String
		}
		if decidedBy.Valid {
			d.DecidedBy = decidedBy.String
		}
		if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
			d.DecidedAt = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", decidedAt); err == nil {
			d.DecidedAt = ts
		}

		decisions = append(decisions, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decisions: %w", err)
	}

	return decisions, nil
}

// AddTaskToInitiative links a task to an initiative.
func (p *ProjectDB) AddTaskToInitiative(initiativeID, taskID string, sequence int) error {
	_, err := p.Exec(`
		INSERT INTO initiative_tasks (initiative_id, task_id, sequence)
		VALUES (?, ?, ?)
		ON CONFLICT(initiative_id, task_id) DO UPDATE SET
			sequence = excluded.sequence
	`, initiativeID, taskID, sequence)
	if err != nil {
		return fmt.Errorf("add task to initiative: %w", err)
	}
	return nil
}

// RemoveTaskFromInitiative unlinks a task from an initiative.
func (p *ProjectDB) RemoveTaskFromInitiative(initiativeID, taskID string) error {
	_, err := p.Exec(`DELETE FROM initiative_tasks WHERE initiative_id = ? AND task_id = ?`, initiativeID, taskID)
	if err != nil {
		return fmt.Errorf("remove task from initiative: %w", err)
	}
	return nil
}

// GetInitiativeTasks retrieves task IDs linked to an initiative in sequence order.
func (p *ProjectDB) GetInitiativeTasks(initiativeID string) ([]string, error) {
	rows, err := p.Query(`
		SELECT task_id FROM initiative_tasks
		WHERE initiative_id = ?
		ORDER BY sequence
	`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("get initiative tasks: %w", err)
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan task id: %w", err)
		}
		taskIDs = append(taskIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task ids: %w", err)
	}

	return taskIDs, nil
}

// AddTaskDependency records that taskID depends on dependsOn.
func (p *ProjectDB) AddTaskDependency(taskID, dependsOn string) error {
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)`
	} else {
		query = `INSERT INTO task_dependencies (task_id, depends_on) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	_, err := p.Exec(query, taskID, dependsOn)
	if err != nil {
		return fmt.Errorf("add task dependency: %w", err)
	}
	return nil
}

// RemoveTaskDependency removes a dependency relationship.
func (p *ProjectDB) RemoveTaskDependency(taskID, dependsOn string) error {
	_, err := p.Exec(`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on = ?`, taskID, dependsOn)
	if err != nil {
		return fmt.Errorf("remove task dependency: %w", err)
	}
	return nil
}

// GetTaskDependencies retrieves IDs of tasks that taskID depends on.
func (p *ProjectDB) GetTaskDependencies(taskID string) ([]string, error) {
	rows, err := p.Query(`SELECT depends_on FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task dependencies: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return deps, nil
}

// GetAllTaskDependencies retrieves all task dependencies in one query.
// Returns a map from task_id to list of depends_on IDs.
func (p *ProjectDB) GetAllTaskDependencies() (map[string][]string, error) {
	rows, err := p.Query(`SELECT task_id, depends_on FROM task_dependencies ORDER BY task_id`)
	if err != nil {
		return nil, fmt.Errorf("get all task dependencies: %w", err)
	}
	defer rows.Close()

	deps := make(map[string][]string)
	for rows.Next() {
		var taskID, dependsOn string
		if err := rows.Scan(&taskID, &dependsOn); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps[taskID] = append(deps[taskID], dependsOn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return deps, nil
}

// GetTaskDependents retrieves IDs of tasks that depend on taskID.
func (p *ProjectDB) GetTaskDependents(taskID string) ([]string, error) {
	rows, err := p.Query(`SELECT task_id FROM task_dependencies WHERE depends_on = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task dependents: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependent: %w", err)
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependents: %w", err)
	}

	return deps, nil
}

// ClearTaskDependencies removes all dependencies for a task.
func (p *ProjectDB) ClearTaskDependencies(taskID string) error {
	_, err := p.Exec(`DELETE FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("clear task dependencies: %w", err)
	}
	return nil
}

// AddInitiativeDependency records that initiativeID depends on dependsOn.
func (p *ProjectDB) AddInitiativeDependency(initiativeID, dependsOn string) error {
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `INSERT OR IGNORE INTO initiative_dependencies (initiative_id, depends_on) VALUES (?, ?)`
	} else {
		query = `INSERT INTO initiative_dependencies (initiative_id, depends_on) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	_, err := p.Exec(query, initiativeID, dependsOn)
	if err != nil {
		return fmt.Errorf("add initiative dependency: %w", err)
	}
	return nil
}

// RemoveInitiativeDependency removes a dependency relationship.
func (p *ProjectDB) RemoveInitiativeDependency(initiativeID, dependsOn string) error {
	_, err := p.Exec(`DELETE FROM initiative_dependencies WHERE initiative_id = ? AND depends_on = ?`, initiativeID, dependsOn)
	if err != nil {
		return fmt.Errorf("remove initiative dependency: %w", err)
	}
	return nil
}

// GetInitiativeDependencies retrieves IDs of initiatives that initiativeID depends on (blocked_by).
func (p *ProjectDB) GetInitiativeDependencies(initiativeID string) ([]string, error) {
	rows, err := p.Query(`SELECT depends_on FROM initiative_dependencies WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("get initiative dependencies: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return deps, nil
}

// GetInitiativeDependents retrieves IDs of initiatives that depend on initiativeID.
func (p *ProjectDB) GetInitiativeDependents(initiativeID string) ([]string, error) {
	rows, err := p.Query(`SELECT initiative_id FROM initiative_dependencies WHERE depends_on = ?`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("get initiative dependents: %w", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependent: %w", err)
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependents: %w", err)
	}

	return deps, nil
}

// ClearInitiativeDependencies removes all dependencies for an initiative.
func (p *ProjectDB) ClearInitiativeDependencies(initiativeID string) error {
	_, err := p.Exec(`DELETE FROM initiative_dependencies WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative dependencies: %w", err)
	}
	return nil
}

// ClearInitiativeTasks removes all task references from an initiative.
func (p *ProjectDB) ClearInitiativeTasks(initiativeID string) error {
	_, err := p.Exec(`DELETE FROM initiative_tasks WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative tasks: %w", err)
	}
	return nil
}

// ============================================================================
// Plan operations (for pure SQL storage mode)
// ============================================================================

// Plan represents an execution plan stored in the database.
type Plan struct {
	TaskID      string
	Version     int
	Weight      string
	Description string
	Phases      string // JSON array of phase definitions
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SavePlan creates or updates a plan.
func (p *ProjectDB) SavePlan(plan *Plan) error {
	now := time.Now().Format(time.RFC3339)
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}

	_, err := p.Exec(`
		INSERT INTO plans (task_id, version, weight, description, phases, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			version = excluded.version,
			weight = excluded.weight,
			description = excluded.description,
			phases = excluded.phases,
			updated_at = excluded.updated_at
	`, plan.TaskID, plan.Version, plan.Weight, plan.Description, plan.Phases,
		plan.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	return nil
}

// GetPlan retrieves a plan by task ID.
func (p *ProjectDB) GetPlan(taskID string) (*Plan, error) {
	row := p.QueryRow(`
		SELECT task_id, version, weight, description, phases, created_at, updated_at
		FROM plans WHERE task_id = ?
	`, taskID)

	var plan Plan
	var description sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&plan.TaskID, &plan.Version, &plan.Weight, &description, &plan.Phases, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get plan %s: %w", taskID, err)
	}

	if description.Valid {
		plan.Description = description.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		plan.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		plan.UpdatedAt = ts
	}

	return &plan, nil
}

// DeletePlan removes a plan.
func (p *ProjectDB) DeletePlan(taskID string) error {
	_, err := p.Exec("DELETE FROM plans WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete plan: %w", err)
	}
	return nil
}

// ============================================================================
// Spec operations (for pure SQL storage mode)
// ============================================================================

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
	defer rows.Close()

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

// ============================================================================
// Gate decision operations (for pure SQL storage mode)
// ============================================================================

// GateDecision represents a gate approval decision.
type GateDecision struct {
	ID        int64
	TaskID    string
	Phase     string
	GateType  string // 'auto', 'ai', 'human', 'skip'
	Approved  bool
	Reason    string
	DecidedBy string
	DecidedAt time.Time
}

// AddGateDecision records a gate decision.
func (p *ProjectDB) AddGateDecision(d *GateDecision) error {
	approved := 0
	if d.Approved {
		approved = 1
	}
	if d.DecidedAt.IsZero() {
		d.DecidedAt = time.Now()
	}

	result, err := p.Exec(`
		INSERT INTO gate_decisions (task_id, phase, gate_type, approved, reason, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.TaskID, d.Phase, d.GateType, approved, d.Reason, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add gate decision: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return nil
}

// GetGateDecisions retrieves all gate decisions for a task.
func (p *ProjectDB) GetGateDecisions(taskID string) ([]GateDecision, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase, gate_type, approved, reason, decided_by, decided_at
		FROM gate_decisions WHERE task_id = ? ORDER BY decided_at
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get gate decisions: %w", err)
	}
	defer rows.Close()

	var decisions []GateDecision
	for rows.Next() {
		var d GateDecision
		var approved int
		var reason, decidedBy sql.NullString
		var decidedAt string

		if err := rows.Scan(&d.ID, &d.TaskID, &d.Phase, &d.GateType, &approved, &reason, &decidedBy, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan gate decision: %w", err)
		}

		d.Approved = approved == 1
		if reason.Valid {
			d.Reason = reason.String
		}
		if decidedBy.Valid {
			d.DecidedBy = decidedBy.String
		}
		if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
			d.DecidedAt = ts
		}

		decisions = append(decisions, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gate decisions: %w", err)
	}

	return decisions, nil
}

// GetGateDecisionForPhase retrieves the gate decision for a specific phase.
func (p *ProjectDB) GetGateDecisionForPhase(taskID, phase string) (*GateDecision, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase, gate_type, approved, reason, decided_by, decided_at
		FROM gate_decisions WHERE task_id = ? AND phase = ?
		ORDER BY decided_at DESC LIMIT 1
	`, taskID, phase)

	var d GateDecision
	var approved int
	var reason, decidedBy sql.NullString
	var decidedAt string

	if err := row.Scan(&d.ID, &d.TaskID, &d.Phase, &d.GateType, &approved, &reason, &decidedBy, &decidedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get gate decision: %w", err)
	}

	d.Approved = approved == 1
	if reason.Valid {
		d.Reason = reason.String
	}
	if decidedBy.Valid {
		d.DecidedBy = decidedBy.String
	}
	if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
		d.DecidedAt = ts
	}

	return &d, nil
}

// ============================================================================
// Attachment operations (for pure SQL storage mode)
// ============================================================================

// Attachment represents a task attachment stored in the database.
type Attachment struct {
	ID          int64
	TaskID      string
	Filename    string
	ContentType string
	SizeBytes   int64
	Data        []byte
	IsImage     bool
	CreatedAt   time.Time
}

// SaveAttachment stores an attachment in the database.
func (p *ProjectDB) SaveAttachment(a *Attachment) error {
	isImage := 0
	if a.IsImage {
		isImage = 1
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}

	result, err := p.Exec(`
		INSERT INTO task_attachments (task_id, filename, content_type, size_bytes, data, is_image, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, filename) DO UPDATE SET
			content_type = excluded.content_type,
			size_bytes = excluded.size_bytes,
			data = excluded.data,
			is_image = excluded.is_image
	`, a.TaskID, a.Filename, a.ContentType, a.SizeBytes, a.Data, isImage, a.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save attachment: %w", err)
	}
	if a.ID == 0 {
		id, _ := result.LastInsertId()
		a.ID = id
	}
	return nil
}

// GetAttachment retrieves an attachment by task ID and filename.
func (p *ProjectDB) GetAttachment(taskID, filename string) (*Attachment, error) {
	row := p.QueryRow(`
		SELECT id, task_id, filename, content_type, size_bytes, data, is_image, created_at
		FROM task_attachments WHERE task_id = ? AND filename = ?
	`, taskID, filename)

	var a Attachment
	var contentType sql.NullString
	var isImage int
	var createdAt string

	if err := row.Scan(&a.ID, &a.TaskID, &a.Filename, &contentType, &a.SizeBytes, &a.Data, &isImage, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	if contentType.Valid {
		a.ContentType = contentType.String
	}
	a.IsImage = isImage == 1
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		a.CreatedAt = ts
	}

	return &a, nil
}

// ListAttachments retrieves attachment metadata for a task (without data).
func (p *ProjectDB) ListAttachments(taskID string) ([]Attachment, error) {
	rows, err := p.Query(`
		SELECT id, task_id, filename, content_type, size_bytes, is_image, created_at
		FROM task_attachments WHERE task_id = ? ORDER BY filename
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var attachments []Attachment
	for rows.Next() {
		var a Attachment
		var contentType sql.NullString
		var isImage int
		var createdAt string

		if err := rows.Scan(&a.ID, &a.TaskID, &a.Filename, &contentType, &a.SizeBytes, &isImage, &createdAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}

		if contentType.Valid {
			a.ContentType = contentType.String
		}
		a.IsImage = isImage == 1
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			a.CreatedAt = ts
		}

		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}

	return attachments, nil
}

// DeleteAttachment removes an attachment.
// Returns an error containing "not found" if the attachment doesn't exist.
func (p *ProjectDB) DeleteAttachment(taskID, filename string) error {
	result, err := p.Exec("DELETE FROM task_attachments WHERE task_id = ? AND filename = ?", taskID, filename)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("attachment %s not found", filename)
	}

	return nil
}

// DeleteAllAttachments removes all attachments for a task.
func (p *ProjectDB) DeleteAllAttachments(taskID string) error {
	_, err := p.Exec("DELETE FROM task_attachments WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete all attachments: %w", err)
	}
	return nil
}

// ============================================================================
// Sync state operations (for CR-SQLite P2P sync)
// ============================================================================

// SyncState represents the sync state for P2P replication.
type SyncState struct {
	SiteID          string
	LastSyncVersion int64
	LastSyncAt      *time.Time
	SyncEnabled     bool
	SyncMode        string // 'none', 'folder', 'http'
	SyncEndpoint    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GetSyncState retrieves the sync state.
func (p *ProjectDB) GetSyncState() (*SyncState, error) {
	row := p.QueryRow(`
		SELECT site_id, last_sync_version, last_sync_at, sync_enabled, sync_mode, sync_endpoint, created_at, updated_at
		FROM sync_state WHERE id = 1
	`)

	var s SyncState
	var lastSyncAt, syncEndpoint sql.NullString
	var syncEnabled int
	var createdAt, updatedAt string

	if err := row.Scan(&s.SiteID, &s.LastSyncVersion, &lastSyncAt, &syncEnabled, &s.SyncMode, &syncEndpoint, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get sync state: %w", err)
	}

	s.SyncEnabled = syncEnabled == 1
	if lastSyncAt.Valid {
		if ts, err := time.Parse(time.RFC3339, lastSyncAt.String); err == nil {
			s.LastSyncAt = &ts
		}
	}
	if syncEndpoint.Valid {
		s.SyncEndpoint = syncEndpoint.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		s.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		s.UpdatedAt = ts
	}

	return &s, nil
}

// UpdateSyncState updates the sync state.
func (p *ProjectDB) UpdateSyncState(s *SyncState) error {
	now := time.Now().Format(time.RFC3339)
	syncEnabled := 0
	if s.SyncEnabled {
		syncEnabled = 1
	}

	var lastSyncAt *string
	if s.LastSyncAt != nil {
		ts := s.LastSyncAt.Format(time.RFC3339)
		lastSyncAt = &ts
	}

	_, err := p.Exec(`
		UPDATE sync_state SET
			last_sync_version = ?,
			last_sync_at = ?,
			sync_enabled = ?,
			sync_mode = ?,
			sync_endpoint = ?,
			updated_at = ?
		WHERE id = 1
	`, s.LastSyncVersion, lastSyncAt, syncEnabled, s.SyncMode, s.SyncEndpoint, now)
	if err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	return nil
}

// ============================================================================
// Extended Task operations (for pure SQL storage mode)
// ============================================================================

// TaskFull represents a task with all fields for database-only storage.
type TaskFull struct {
	Task

	// PR info
	PRUrl           string
	PRNumber        int
	PRStatus        string
	PRChecksStatus  string
	PRMergeable     bool
	PRReviewCount   int
	PRApprovalCount int
	PRMerged        bool
	PRMergedAt      *time.Time
	PRMergeCommitSHA string
	PRTargetBranch  string
	PRLastCheckedAt *time.Time

	// Testing
	TestingRequirements string // JSON
	RequiresUITesting   bool

	// Metadata
	Tags           string // JSON array
	InitiativeID   string
	MetadataSource string
	CreatedBy      string

	// Execution tracking
	ExecutorPID       int
	ExecutorHostname  string
	ExecutorStartedAt *time.Time
	LastHeartbeat     *time.Time

	// Session tracking
	SessionID           string
	SessionModel        string
	SessionStatus       string
	SessionCreatedAt    *time.Time
	SessionLastActivity *time.Time
	SessionTurnCount    int

	// Token tracking
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	TotalTokens         int

	// Retry context
	RetryContext string // JSON
}

// GetNextTaskID generates the next task ID.
func (p *ProjectDB) GetNextTaskID() (string, error) {
	var maxID sql.NullString
	err := p.QueryRow(`
		SELECT id FROM tasks
		WHERE id LIKE 'TASK-%'
		ORDER BY CAST(SUBSTR(id, 6) AS INTEGER) DESC
		LIMIT 1
	`).Scan(&maxID)

	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("get max task id: %w", err)
	}

	if !maxID.Valid || maxID.String == "" {
		return "TASK-001", nil
	}

	// Extract number and increment
	var num int
	_, err = fmt.Sscanf(maxID.String, "TASK-%d", &num)
	if err != nil {
		return "TASK-001", nil
	}

	return fmt.Sprintf("TASK-%03d", num+1), nil
}

// ============================================================================
// Transaction-aware operations (TxOps methods)
// These methods allow operations to run within a transaction context.
// ============================================================================

// SaveTaskTx saves a task within a transaction.
func SaveTaskTx(tx *TxOps, t *Task) error {
	var startedAt, completedAt *string
	if t.StartedAt != nil {
		s := t.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	// Default queue, priority, and category if not set
	queue := t.Queue
	if queue == "" {
		queue = "active"
	}
	priority := t.Priority
	if priority == "" {
		priority = "normal"
	}
	category := t.Category
	if category == "" {
		category = "feature"
	}

	// Default state_status if not set
	stateStatus := t.StateStatus
	if stateStatus == "" {
		stateStatus = "pending"
	}

	// Format execution tracking timestamps
	var executorStartedAt, lastHeartbeat *string
	if t.ExecutorStartedAt != nil {
		s := t.ExecutorStartedAt.Format(time.RFC3339)
		executorStartedAt = &s
	}
	if t.LastHeartbeat != nil {
		s := t.LastHeartbeat.Format(time.RFC3339)
		lastHeartbeat = &s
	}

	_, err := tx.Exec(`
		INSERT INTO tasks (id, title, description, weight, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, created_at, started_at, completed_at, total_cost_usd, metadata, retry_context, executor_pid, executor_hostname, executor_started_at, last_heartbeat)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			weight = excluded.weight,
			status = excluded.status,
			state_status = excluded.state_status,
			current_phase = excluded.current_phase,
			branch = excluded.branch,
			worktree_path = excluded.worktree_path,
			queue = excluded.queue,
			priority = excluded.priority,
			category = excluded.category,
			initiative_id = excluded.initiative_id,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			total_cost_usd = excluded.total_cost_usd,
			metadata = excluded.metadata,
			retry_context = excluded.retry_context,
			executor_pid = excluded.executor_pid,
			executor_hostname = excluded.executor_hostname,
			executor_started_at = excluded.executor_started_at,
			last_heartbeat = excluded.last_heartbeat
	`, t.ID, t.Title, t.Description, t.Weight, t.Status, stateStatus, t.CurrentPhase, t.Branch, t.WorktreePath,
		queue, priority, category, t.InitiativeID, t.CreatedAt.Format(time.RFC3339), startedAt, completedAt, t.TotalCostUSD, t.Metadata, t.RetryContext,
		t.ExecutorPID, t.ExecutorHostname, executorStartedAt, lastHeartbeat)
	if err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// ClearTaskDependenciesTx removes all dependencies for a task within a transaction.
func ClearTaskDependenciesTx(tx *TxOps, taskID string) error {
	_, err := tx.Exec(`DELETE FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("clear task dependencies: %w", err)
	}
	return nil
}

// AddTaskDependencyTx adds a task dependency within a transaction.
func AddTaskDependencyTx(tx *TxOps, taskID, dependsOn string) error {
	var query string
	if tx.Dialect() == driver.DialectSQLite {
		query = `INSERT OR IGNORE INTO task_dependencies (task_id, depends_on) VALUES (?, ?)`
	} else {
		query = `INSERT INTO task_dependencies (task_id, depends_on) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	_, err := tx.Exec(query, taskID, dependsOn)
	if err != nil {
		return fmt.Errorf("add task dependency: %w", err)
	}
	return nil
}

// ClearPhasesTx removes all phases for a task within a transaction.
func ClearPhasesTx(tx *TxOps, taskID string) error {
	_, err := tx.Exec("DELETE FROM phases WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("clear phases: %w", err)
	}
	return nil
}

// SavePhaseTx saves a phase within a transaction.
func SavePhaseTx(tx *TxOps, ph *Phase) error {
	var startedAt, completedAt *string
	if ph.StartedAt != nil {
		s := ph.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if ph.CompletedAt != nil {
		s := ph.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := tx.Exec(`
		INSERT INTO phases (task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message, commit_sha, skip_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			error_message = excluded.error_message,
			commit_sha = excluded.commit_sha,
			skip_reason = excluded.skip_reason
	`, ph.TaskID, ph.PhaseID, ph.Status, ph.Iterations, startedAt, completedAt,
		ph.InputTokens, ph.OutputTokens, ph.CostUSD, ph.ErrorMessage, ph.CommitSHA, ph.SkipReason)
	if err != nil {
		return fmt.Errorf("save phase: %w", err)
	}
	return nil
}

// AddGateDecisionTx adds a gate decision within a transaction.
func AddGateDecisionTx(tx *TxOps, d *GateDecision) error {
	approved := 0
	if d.Approved {
		approved = 1
	}
	if d.DecidedAt.IsZero() {
		d.DecidedAt = time.Now()
	}

	result, err := tx.Exec(`
		INSERT INTO gate_decisions (task_id, phase, gate_type, approved, reason, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, d.TaskID, d.Phase, d.GateType, approved, d.Reason, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add gate decision: %w", err)
	}
	id, _ := result.LastInsertId()
	d.ID = id
	return nil
}

// SaveInitiativeTx saves an initiative within a transaction.
func SaveInitiativeTx(tx *TxOps, i *Initiative) error {
	now := time.Now().Format(time.RFC3339)
	if i.CreatedAt.IsZero() {
		i.CreatedAt = time.Now()
	}

	_, err := tx.Exec(`
		INSERT INTO initiatives (id, title, status, owner_initials, owner_display_name, owner_email, vision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			status = excluded.status,
			owner_initials = excluded.owner_initials,
			owner_display_name = excluded.owner_display_name,
			owner_email = excluded.owner_email,
			vision = excluded.vision,
			updated_at = excluded.updated_at
	`, i.ID, i.Title, i.Status, i.OwnerInitials, i.OwnerDisplayName, i.OwnerEmail, i.Vision,
		i.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}
	return nil
}

// AddInitiativeDecisionTx adds a decision within a transaction.
func AddInitiativeDecisionTx(tx *TxOps, d *InitiativeDecision) error {
	_, err := tx.Exec(`
		INSERT INTO initiative_decisions (id, initiative_id, decision, rationale, decided_by, decided_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, d.ID, d.InitiativeID, d.Decision, d.Rationale, d.DecidedBy, d.DecidedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("add initiative decision: %w", err)
	}
	return nil
}

// ClearInitiativeTasksTx removes all task references from an initiative within a transaction.
func ClearInitiativeTasksTx(tx *TxOps, initiativeID string) error {
	_, err := tx.Exec(`DELETE FROM initiative_tasks WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative tasks: %w", err)
	}
	return nil
}

// AddTaskToInitiativeTx links a task to an initiative within a transaction.
func AddTaskToInitiativeTx(tx *TxOps, initiativeID, taskID string, sequence int) error {
	_, err := tx.Exec(`
		INSERT INTO initiative_tasks (initiative_id, task_id, sequence)
		VALUES (?, ?, ?)
		ON CONFLICT(initiative_id, task_id) DO UPDATE SET
			sequence = excluded.sequence
	`, initiativeID, taskID, sequence)
	if err != nil {
		return fmt.Errorf("add task to initiative: %w", err)
	}
	return nil
}

// ClearInitiativeDependenciesTx removes all dependencies for an initiative within a transaction.
func ClearInitiativeDependenciesTx(tx *TxOps, initiativeID string) error {
	_, err := tx.Exec(`DELETE FROM initiative_dependencies WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative dependencies: %w", err)
	}
	return nil
}

// AddInitiativeDependencyTx adds an initiative dependency within a transaction.
func AddInitiativeDependencyTx(tx *TxOps, initiativeID, dependsOn string) error {
	var query string
	if tx.Dialect() == driver.DialectSQLite {
		query = `INSERT OR IGNORE INTO initiative_dependencies (initiative_id, depends_on) VALUES (?, ?)`
	} else {
		query = `INSERT INTO initiative_dependencies (initiative_id, depends_on) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	_, err := tx.Exec(query, initiativeID, dependsOn)
	if err != nil {
		return fmt.Errorf("add initiative dependency: %w", err)
	}
	return nil
}

// ============================================================================
// Batch loading operations for initiatives (avoid N+1 queries)
// ============================================================================

// GetAllInitiativeDecisions retrieves all initiative decisions in one query.
// Returns a map from initiative_id to list of decisions.
func (p *ProjectDB) GetAllInitiativeDecisions() (map[string][]InitiativeDecision, error) {
	rows, err := p.Query(`
		SELECT id, initiative_id, decision, rationale, decided_by, decided_at
		FROM initiative_decisions ORDER BY initiative_id, decided_at
	`)
	if err != nil {
		return nil, fmt.Errorf("get all initiative decisions: %w", err)
	}
	defer rows.Close()

	decisions := make(map[string][]InitiativeDecision)
	for rows.Next() {
		var d InitiativeDecision
		var rationale, decidedBy sql.NullString
		var decidedAt string

		if err := rows.Scan(&d.ID, &d.InitiativeID, &d.Decision, &rationale, &decidedBy, &decidedAt); err != nil {
			return nil, fmt.Errorf("scan decision: %w", err)
		}

		if rationale.Valid {
			d.Rationale = rationale.String
		}
		if decidedBy.Valid {
			d.DecidedBy = decidedBy.String
		}
		if ts, err := time.Parse(time.RFC3339, decidedAt); err == nil {
			d.DecidedAt = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", decidedAt); err == nil {
			d.DecidedAt = ts
		}

		decisions[d.InitiativeID] = append(decisions[d.InitiativeID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decisions: %w", err)
	}

	return decisions, nil
}

// InitiativeTaskRef represents a task reference with its details for batch loading.
type InitiativeTaskRef struct {
	InitiativeID string
	TaskID       string
	Title        string
	Status       string
	Sequence     int
}

// GetAllInitiativeTaskRefs retrieves all initiative task references with task details in one query.
// Returns a map from initiative_id to list of task refs (already populated with title/status).
func (p *ProjectDB) GetAllInitiativeTaskRefs() (map[string][]InitiativeTaskRef, error) {
	rows, err := p.Query(`
		SELECT it.initiative_id, it.task_id, t.title, t.status, it.sequence
		FROM initiative_tasks it
		JOIN tasks t ON it.task_id = t.id
		ORDER BY it.initiative_id, it.sequence
	`)
	if err != nil {
		return nil, fmt.Errorf("get all initiative task refs: %w", err)
	}
	defer rows.Close()

	refs := make(map[string][]InitiativeTaskRef)
	for rows.Next() {
		var r InitiativeTaskRef
		if err := rows.Scan(&r.InitiativeID, &r.TaskID, &r.Title, &r.Status, &r.Sequence); err != nil {
			return nil, fmt.Errorf("scan task ref: %w", err)
		}
		refs[r.InitiativeID] = append(refs[r.InitiativeID], r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task refs: %w", err)
	}

	return refs, nil
}

// GetAllInitiativeDependencies retrieves all initiative dependencies in one query.
// Returns a map from initiative_id to list of depends_on IDs (blocked_by).
func (p *ProjectDB) GetAllInitiativeDependencies() (map[string][]string, error) {
	rows, err := p.Query(`SELECT initiative_id, depends_on FROM initiative_dependencies ORDER BY initiative_id`)
	if err != nil {
		return nil, fmt.Errorf("get all initiative dependencies: %w", err)
	}
	defer rows.Close()

	deps := make(map[string][]string)
	for rows.Next() {
		var initiativeID, dependsOn string
		if err := rows.Scan(&initiativeID, &dependsOn); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps[initiativeID] = append(deps[initiativeID], dependsOn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return deps, nil
}

// GetAllInitiativeDependents retrieves all initiative dependents in one query.
// Returns a map from initiative_id to list of initiative IDs that depend on it (blocks).
func (p *ProjectDB) GetAllInitiativeDependents() (map[string][]string, error) {
	rows, err := p.Query(`SELECT depends_on, initiative_id FROM initiative_dependencies ORDER BY depends_on`)
	if err != nil {
		return nil, fmt.Errorf("get all initiative dependents: %w", err)
	}
	defer rows.Close()

	dependents := make(map[string][]string)
	for rows.Next() {
		var dependsOn, initiativeID string
		if err := rows.Scan(&dependsOn, &initiativeID); err != nil {
			return nil, fmt.Errorf("scan dependent: %w", err)
		}
		dependents[dependsOn] = append(dependents[dependsOn], initiativeID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependents: %w", err)
	}

	return dependents, nil
}
