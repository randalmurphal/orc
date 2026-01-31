package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// Task represents a task stored in the database.
type Task struct {
	ID           string
	Title        string
	Description  string
	Weight       string
	WorkflowID   string // Required workflow assignment for task execution
	Status       string
	StateStatus  string // State status: pending, running, completed, failed, paused, interrupted, skipped
	CurrentPhase string
	Branch       string
	WorktreePath string
	Queue        string // "active" or "backlog"
	Priority     string // "critical", "high", "normal", "low"
	Category     string // "feature", "bug", "refactor", "chore", "docs", "test"
	InitiativeID string // Links this task to an initiative (e.g., INIT-001)
	TargetBranch string // Override target branch for PR (takes precedence over initiative/config)
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	UpdatedAt    time.Time
	TotalCostUSD float64
	Metadata     string // JSON object: {"key": "value", ...}
	RetryContext string // JSON: state.RetryContext serialized
	Quality      string // JSON: task.QualityMetrics serialized

	// Execution tracking for orphan detection
	ExecutorPID       int        // Process ID of executor
	ExecutorHostname  string     // Hostname running the executor
	ExecutorStartedAt *time.Time // When execution started
	LastHeartbeat     *time.Time // Last heartbeat update

	// Automation task flag (for efficient querying)
	IsAutomation bool // true for AUTO-XXX tasks

	// Branch control overrides
	BranchName     *string // Custom branch name (overrides auto-generated)
	PrDraft        *bool   // PR draft mode override (nil = use default)
	PrLabels       string  // JSON array of PR labels
	PrReviewers    string  // JSON array of PR reviewers
	PrLabelsSet    bool    // True if pr_labels explicitly set
	PrReviewersSet bool    // True if pr_reviewers explicitly set

	// PR tracking (set after PR creation/reuse)
	PrURL    string // URL of the created/reused PR
	PrNumber int    // PR number
	PrStatus string // PR status (pending_review, approved, merged, closed)
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

	// Convert bool to int for SQLite
	isAutomation := 0
	if t.IsAutomation {
		isAutomation = 1
	}

	// Convert branch control bools to int for SQLite
	var prDraft *int
	if t.PrDraft != nil {
		v := 0
		if *t.PrDraft {
			v = 1
		}
		prDraft = &v
	}
	prLabelsSet := 0
	if t.PrLabelsSet {
		prLabelsSet = 1
	}
	prReviewersSet := 0
	if t.PrReviewersSet {
		prReviewersSet = 1
	}

	_, err := p.Exec(`
		INSERT INTO tasks (id, title, description, weight, workflow_id, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, target_branch, created_at, started_at, completed_at, total_cost_usd, metadata, retry_context, quality, executor_pid, executor_hostname, executor_started_at, last_heartbeat, is_automation, branch_name, pr_draft, pr_labels, pr_reviewers, pr_labels_set, pr_reviewers_set, pr_url, pr_number, pr_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			weight = excluded.weight,
			workflow_id = excluded.workflow_id,
			status = excluded.status,
			state_status = excluded.state_status,
			current_phase = excluded.current_phase,
			branch = excluded.branch,
			worktree_path = excluded.worktree_path,
			queue = excluded.queue,
			priority = excluded.priority,
			category = excluded.category,
			initiative_id = excluded.initiative_id,
			target_branch = excluded.target_branch,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			total_cost_usd = excluded.total_cost_usd,
			metadata = excluded.metadata,
			retry_context = excluded.retry_context,
			quality = excluded.quality,
			executor_pid = excluded.executor_pid,
			executor_hostname = excluded.executor_hostname,
			executor_started_at = excluded.executor_started_at,
			last_heartbeat = excluded.last_heartbeat,
			is_automation = excluded.is_automation,
			branch_name = excluded.branch_name,
			pr_draft = excluded.pr_draft,
			pr_labels = excluded.pr_labels,
			pr_reviewers = excluded.pr_reviewers,
			pr_labels_set = excluded.pr_labels_set,
			pr_reviewers_set = excluded.pr_reviewers_set,
			pr_url = excluded.pr_url,
			pr_number = excluded.pr_number,
			pr_status = excluded.pr_status
	`, t.ID, t.Title, t.Description, t.Weight, t.WorkflowID, t.Status, stateStatus, t.CurrentPhase, t.Branch, t.WorktreePath,
		queue, priority, category, t.InitiativeID, t.TargetBranch, t.CreatedAt.Format(time.RFC3339), startedAt, completedAt, t.TotalCostUSD, t.Metadata, t.RetryContext, t.Quality,
		t.ExecutorPID, t.ExecutorHostname, executorStartedAt, lastHeartbeat, isAutomation,
		t.BranchName, prDraft, t.PrLabels, t.PrReviewers, prLabelsSet, prReviewersSet,
		t.PrURL, t.PrNumber, t.PrStatus)
	if err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// GetTask retrieves a task by ID.
func (p *ProjectDB) GetTask(id string) (*Task, error) {
	row := p.QueryRow(`
		SELECT id, title, description, weight, workflow_id, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, target_branch, created_at, started_at, completed_at, updated_at, total_cost_usd, metadata, retry_context, quality, executor_pid, executor_hostname, executor_started_at, last_heartbeat, is_automation, branch_name, pr_draft, pr_labels, pr_reviewers, pr_labels_set, pr_reviewers_set, pr_url, pr_number, pr_status
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
	Status       string
	Queue        string // "active", "backlog", or empty for all
	Priority     string // "critical", "high", "normal", "low", or empty for all
	IsAutomation *bool  // true = only automation tasks, false = only non-automation, nil = all
	Limit        int
	Offset       int
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
	if opts.IsAutomation != nil {
		if *opts.IsAutomation {
			whereClauses = append(whereClauses, "is_automation = 1")
		} else {
			whereClauses = append(whereClauses, "(is_automation = 0 OR is_automation IS NULL)")
		}
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
		SELECT id, title, description, weight, workflow_id, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, target_branch, created_at, started_at, completed_at, updated_at, total_cost_usd, metadata, retry_context, quality, executor_pid, executor_hostname, executor_started_at, last_heartbeat, is_automation, branch_name, pr_draft, pr_labels, pr_reviewers, pr_labels_set, pr_reviewers_set, pr_url, pr_number, pr_status
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
	defer func() { _ = rows.Close() }()

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

// AutomationTaskStats holds counts of automation tasks by status.
type AutomationTaskStats struct {
	Pending   int // created, planned
	Running   int
	Completed int // completed
}

// GetAutomationTaskStats returns counts of automation tasks by status.
// Uses a single aggregated query for efficiency.
func (p *ProjectDB) GetAutomationTaskStats() (*AutomationTaskStats, error) {
	query := `
		SELECT
			SUM(CASE WHEN status IN ('created', 'planned') THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed
		FROM tasks
		WHERE is_automation = 1
	`
	var pending, running, completed sql.NullInt64
	if err := p.QueryRow(query).Scan(&pending, &running, &completed); err != nil {
		return nil, fmt.Errorf("get automation task stats: %w", err)
	}

	return &AutomationTaskStats{
		Pending:   int(pending.Int64),
		Running:   int(running.Int64),
		Completed: int(completed.Int64),
	}, nil
}

// ============================================================================
// Task Dependencies
// ============================================================================

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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

// ============================================================================
// Extended Task operations (for pure SQL storage mode)
// ============================================================================

// TaskFull represents a task with all fields for database-only storage.
type TaskFull struct {
	Task

	// PR info
	PRUrl            string
	PRNumber         int
	PRStatus         string
	PRChecksStatus   string
	PRMergeable      bool
	PRReviewCount    int
	PRApprovalCount  int
	PRMerged         bool
	PRMergedAt       *time.Time
	PRMergeCommitSHA string
	PRTargetBranch   string
	PRLastCheckedAt  *time.Time

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

// UpdateTaskHeartbeat updates the last_heartbeat timestamp for a task.
// Used during long-running phases to prevent false orphan detection.
func (p *ProjectDB) UpdateTaskHeartbeat(taskID string) error {
	_, err := p.Exec(`UPDATE tasks SET last_heartbeat = ? WHERE id = ?`,
		time.Now().Format(time.RFC3339), taskID)
	return err
}

// SetTaskExecutor sets the executor info (PID, hostname, heartbeat) for a task.
// Used when starting fresh execution.
func (p *ProjectDB) SetTaskExecutor(taskID string, pid int, hostname string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := p.Exec(`
		UPDATE tasks
		SET executor_pid = ?, executor_hostname = ?, executor_started_at = ?, last_heartbeat = ?
		WHERE id = ?`,
		pid, hostname, now, now, taskID)
	return err
}

// ClearTaskExecutor clears the executor info (PID, hostname) for a task.
// Called when task completes, fails, or is paused to release the claim.
func (p *ProjectDB) ClearTaskExecutor(taskID string) error {
	_, err := p.Exec(`
		UPDATE tasks
		SET executor_pid = 0, executor_hostname = ''
		WHERE id = ?`,
		taskID)
	return err
}

// ============================================================================
// Transaction-aware Task operations
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

	// Convert bool to int for SQLite
	isAutomation := 0
	if t.IsAutomation {
		isAutomation = 1
	}

	// Convert branch control bools to int for SQLite
	var prDraft *int
	if t.PrDraft != nil {
		v := 0
		if *t.PrDraft {
			v = 1
		}
		prDraft = &v
	}
	prLabelsSet := 0
	if t.PrLabelsSet {
		prLabelsSet = 1
	}
	prReviewersSet := 0
	if t.PrReviewersSet {
		prReviewersSet = 1
	}

	// Format updated_at
	var updatedAt string
	if !t.UpdatedAt.IsZero() {
		updatedAt = t.UpdatedAt.Format(time.RFC3339)
	} else {
		updatedAt = time.Now().Format(time.RFC3339)
	}

	_, err := tx.Exec(`
		INSERT INTO tasks (id, title, description, weight, workflow_id, status, state_status, current_phase, branch, worktree_path, queue, priority, category, initiative_id, target_branch, created_at, started_at, completed_at, updated_at, total_cost_usd, metadata, retry_context, quality, executor_pid, executor_hostname, executor_started_at, last_heartbeat, is_automation, branch_name, pr_draft, pr_labels, pr_reviewers, pr_labels_set, pr_reviewers_set, pr_url, pr_number, pr_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			weight = excluded.weight,
			workflow_id = excluded.workflow_id,
			status = excluded.status,
			state_status = excluded.state_status,
			current_phase = excluded.current_phase,
			branch = excluded.branch,
			worktree_path = excluded.worktree_path,
			queue = excluded.queue,
			priority = excluded.priority,
			category = excluded.category,
			initiative_id = excluded.initiative_id,
			target_branch = excluded.target_branch,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			total_cost_usd = excluded.total_cost_usd,
			metadata = excluded.metadata,
			retry_context = excluded.retry_context,
			quality = excluded.quality,
			executor_pid = excluded.executor_pid,
			executor_hostname = excluded.executor_hostname,
			executor_started_at = excluded.executor_started_at,
			last_heartbeat = excluded.last_heartbeat,
			is_automation = excluded.is_automation,
			branch_name = excluded.branch_name,
			pr_draft = excluded.pr_draft,
			pr_labels = excluded.pr_labels,
			pr_reviewers = excluded.pr_reviewers,
			pr_labels_set = excluded.pr_labels_set,
			pr_reviewers_set = excluded.pr_reviewers_set,
			pr_url = excluded.pr_url,
			pr_number = excluded.pr_number,
			pr_status = excluded.pr_status
	`, t.ID, t.Title, t.Description, t.Weight, t.WorkflowID, t.Status, stateStatus, t.CurrentPhase, t.Branch, t.WorktreePath,
		queue, priority, category, t.InitiativeID, t.TargetBranch, t.CreatedAt.Format(time.RFC3339), startedAt, completedAt, updatedAt, t.TotalCostUSD, t.Metadata, t.RetryContext, t.Quality,
		t.ExecutorPID, t.ExecutorHostname, executorStartedAt, lastHeartbeat, isAutomation,
		t.BranchName, prDraft, t.PrLabels, t.PrReviewers, prLabelsSet, prReviewersSet,
		t.PrURL, t.PrNumber, t.PrStatus)
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

// ============================================================================
// Private scanner helpers
// ============================================================================

// scanTask scans a single task from a Row.
func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt, updatedAt sql.NullString
	var description, workflowID, stateStatus, currentPhase, branch, worktreePath, queue, priority, category, initiativeID, targetBranch, metadata, retryContext, quality sql.NullString
	var executorPID sql.NullInt64
	var executorHostname, executorStartedAt, lastHeartbeat sql.NullString
	var isAutomation sql.NullInt64
	var branchName, prLabels, prReviewers sql.NullString
	var prDraft, prLabelsSet, prReviewersSet sql.NullInt64
	var prURL sql.NullString
	var prNumber sql.NullInt64
	var prStatus sql.NullString

	if err := row.Scan(&t.ID, &t.Title, &description, &t.Weight, &workflowID, &t.Status, &stateStatus, &currentPhase, &branch, &worktreePath,
		&queue, &priority, &category, &initiativeID, &targetBranch, &createdAt, &startedAt, &completedAt, &updatedAt, &t.TotalCostUSD, &metadata, &retryContext, &quality,
		&executorPID, &executorHostname, &executorStartedAt, &lastHeartbeat, &isAutomation,
		&branchName, &prDraft, &prLabels, &prReviewers, &prLabelsSet, &prReviewersSet,
		&prURL, &prNumber, &prStatus); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
	}
	if workflowID.Valid {
		t.WorkflowID = workflowID.String
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
	if targetBranch.Valid {
		t.TargetBranch = targetBranch.String
	}
	if metadata.Valid {
		t.Metadata = metadata.String
	}
	if retryContext.Valid {
		t.RetryContext = retryContext.String
	}
	if quality.Valid {
		t.Quality = quality.String
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

	// Automation flag
	if isAutomation.Valid && isAutomation.Int64 == 1 {
		t.IsAutomation = true
	}
	// Updated timestamp
	if updatedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			t.UpdatedAt = ts
		}
	}

	// Branch control fields
	if branchName.Valid {
		t.BranchName = &branchName.String
	}
	if prDraft.Valid {
		v := prDraft.Int64 == 1
		t.PrDraft = &v
	}
	if prLabels.Valid {
		t.PrLabels = prLabels.String
	}
	if prReviewers.Valid {
		t.PrReviewers = prReviewers.String
	}
	if prLabelsSet.Valid && prLabelsSet.Int64 == 1 {
		t.PrLabelsSet = true
	}
	if prReviewersSet.Valid && prReviewersSet.Int64 == 1 {
		t.PrReviewersSet = true
	}

	// PR tracking fields
	if prURL.Valid {
		t.PrURL = prURL.String
	}
	if prNumber.Valid {
		t.PrNumber = int(prNumber.Int64)
	}
	if prStatus.Valid {
		t.PrStatus = prStatus.String
	}

	return &t, nil
}

// scanTaskRows scans a task from Rows.
func scanTaskRows(rows *sql.Rows) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt, updatedAt sql.NullString
	var description, workflowID, stateStatus, currentPhase, branch, worktreePath, queue, priority, category, initiativeID, targetBranch, metadata, retryContext, quality sql.NullString
	var executorPID sql.NullInt64
	var executorHostname, executorStartedAt, lastHeartbeat sql.NullString
	var isAutomation sql.NullInt64
	var branchName, prLabels, prReviewers sql.NullString
	var prDraft, prLabelsSet, prReviewersSet sql.NullInt64
	var prURL sql.NullString
	var prNumber sql.NullInt64
	var prStatus sql.NullString

	if err := rows.Scan(&t.ID, &t.Title, &description, &t.Weight, &workflowID, &t.Status, &stateStatus, &currentPhase, &branch, &worktreePath,
		&queue, &priority, &category, &initiativeID, &targetBranch, &createdAt, &startedAt, &completedAt, &updatedAt, &t.TotalCostUSD, &metadata, &retryContext, &quality,
		&executorPID, &executorHostname, &executorStartedAt, &lastHeartbeat, &isAutomation,
		&branchName, &prDraft, &prLabels, &prReviewers, &prLabelsSet, &prReviewersSet,
		&prURL, &prNumber, &prStatus); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
	}
	if workflowID.Valid {
		t.WorkflowID = workflowID.String
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
	if targetBranch.Valid {
		t.TargetBranch = targetBranch.String
	}
	if metadata.Valid {
		t.Metadata = metadata.String
	}
	if retryContext.Valid {
		t.RetryContext = retryContext.String
	}
	if quality.Valid {
		t.Quality = quality.String
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

	// Automation flag
	if isAutomation.Valid && isAutomation.Int64 == 1 {
		t.IsAutomation = true
	}
	// Updated timestamp
	if updatedAt.Valid {
		if ts, err := time.Parse(time.RFC3339, updatedAt.String); err == nil {
			t.UpdatedAt = ts
		}
	}

	// Branch control fields
	if branchName.Valid {
		t.BranchName = &branchName.String
	}
	if prDraft.Valid {
		v := prDraft.Int64 == 1
		t.PrDraft = &v
	}
	if prLabels.Valid {
		t.PrLabels = prLabels.String
	}
	if prReviewers.Valid {
		t.PrReviewers = prReviewers.String
	}
	if prLabelsSet.Valid && prLabelsSet.Int64 == 1 {
		t.PrLabelsSet = true
	}
	if prReviewersSet.Valid && prReviewersSet.Int64 == 1 {
		t.PrReviewersSet = true
	}

	// PR tracking fields
	if prURL.Valid {
		t.PrURL = prURL.String
	}
	if prNumber.Valid {
		t.PrNumber = int(prNumber.Int64)
	}
	if prStatus.Valid {
		t.PrStatus = prStatus.String
	}

	return &t, nil
}

// ============================================================================
// Task Activity Aggregation (for heatmap)
// ============================================================================

// ActivityCount represents task completions for a single date.
type ActivityCount struct {
	Date  string // YYYY-MM-DD format
	Count int
}

// GetTaskActivityByDate returns task completion counts grouped by date.
// The date range is [startDate, endDate) - inclusive start, exclusive end.
func (p *ProjectDB) GetTaskActivityByDate(startDate, endDate string) ([]ActivityCount, error) {
	query := `
		SELECT DATE(completed_at) as date, COUNT(*) as count
		FROM tasks
		WHERE completed_at IS NOT NULL
		  AND DATE(completed_at) >= ?
		  AND DATE(completed_at) < ?
		GROUP BY DATE(completed_at)
		ORDER BY date ASC
	`

	rows, err := p.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get task activity by date: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []ActivityCount
	for rows.Next() {
		var ac ActivityCount
		if err := rows.Scan(&ac.Date, &ac.Count); err != nil {
			return nil, fmt.Errorf("scan activity count: %w", err)
		}
		results = append(results, ac)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity counts: %w", err)
	}

	return results, nil
}
