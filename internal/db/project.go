package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// ProjectDB provides operations on a project database (.orc/orc.db).
type ProjectDB struct {
	*DB
}

// OpenProject opens the project database at {projectPath}/.orc/orc.db.
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
	frameworks, _ := json.Marshal(d.Frameworks)
	buildTools, _ := json.Marshal(d.BuildTools)

	hasTests := 0
	if d.HasTests {
		hasTests = 1
	}

	_, err := p.Exec(`
		INSERT OR REPLACE INTO detection (id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, datetime('now'))
	`, d.Language, string(frameworks), string(buildTools), hasTests, d.TestCommand, d.LintCommand)
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
	json.Unmarshal([]byte(frameworks), &d.Frameworks)
	json.Unmarshal([]byte(buildTools), &d.BuildTools)
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
	CurrentPhase string
	Branch       string
	WorktreePath string
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	TotalCostUSD float64
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

	_, err := p.Exec(`
		INSERT INTO tasks (id, title, description, weight, status, current_phase, branch, worktree_path, created_at, started_at, completed_at, total_cost_usd)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			weight = excluded.weight,
			status = excluded.status,
			current_phase = excluded.current_phase,
			branch = excluded.branch,
			worktree_path = excluded.worktree_path,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			total_cost_usd = excluded.total_cost_usd
	`, t.ID, t.Title, t.Description, t.Weight, t.Status, t.CurrentPhase, t.Branch, t.WorktreePath,
		t.CreatedAt.Format(time.RFC3339), startedAt, completedAt, t.TotalCostUSD)
	if err != nil {
		return fmt.Errorf("save task: %w", err)
	}
	return nil
}

// GetTask retrieves a task by ID.
func (p *ProjectDB) GetTask(id string) (*Task, error) {
	row := p.QueryRow(`
		SELECT id, title, description, weight, status, current_phase, branch, worktree_path, created_at, started_at, completed_at, total_cost_usd
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
	Status string
	Limit  int
	Offset int
}

// ListTasks returns tasks matching the given options.
func (p *ProjectDB) ListTasks(opts ListOpts) ([]Task, int, error) {
	// Count total
	countQuery := "SELECT COUNT(*) FROM tasks"
	countArgs := []any{}
	if opts.Status != "" {
		countQuery += " WHERE status = ?"
		countArgs = append(countArgs, opts.Status)
	}

	var total int
	if err := p.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// Query tasks
	query := `
		SELECT id, title, description, weight, status, current_phase, branch, worktree_path, created_at, started_at, completed_at, total_cost_usd
		FROM tasks
	`
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

	return tasks, total, nil
}

// scanTask scans a single task from a Row.
func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt sql.NullString
	var description, currentPhase, branch, worktreePath sql.NullString

	if err := row.Scan(&t.ID, &t.Title, &description, &t.Weight, &t.Status, &currentPhase, &branch, &worktreePath,
		&createdAt, &startedAt, &completedAt, &t.TotalCostUSD); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
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

	return &t, nil
}

// scanTaskRows scans a task from Rows.
func scanTaskRows(rows *sql.Rows) (*Task, error) {
	var t Task
	var createdAt string
	var startedAt, completedAt sql.NullString
	var description, currentPhase, branch, worktreePath sql.NullString

	if err := rows.Scan(&t.ID, &t.Title, &description, &t.Weight, &t.Status, &currentPhase, &branch, &worktreePath,
		&createdAt, &startedAt, &completedAt, &t.TotalCostUSD); err != nil {
		return nil, err
	}

	if description.Valid {
		t.Description = description.String
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
		INSERT INTO phases (task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			error_message = excluded.error_message
	`, ph.TaskID, ph.PhaseID, ph.Status, ph.Iterations, startedAt, completedAt,
		ph.InputTokens, ph.OutputTokens, ph.CostUSD, ph.ErrorMessage)
	if err != nil {
		return fmt.Errorf("save phase: %w", err)
	}
	return nil
}

// GetPhases retrieves all phases for a task.
func (p *ProjectDB) GetPhases(taskID string) ([]Phase, error) {
	rows, err := p.Query(`
		SELECT task_id, phase_id, status, iterations, started_at, completed_at, input_tokens, output_tokens, cost_usd, error_message
		FROM phases WHERE task_id = ?
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get phases: %w", err)
	}
	defer rows.Close()

	var phases []Phase
	for rows.Next() {
		var ph Phase
		var startedAt, completedAt, errorMsg sql.NullString
		if err := rows.Scan(&ph.TaskID, &ph.PhaseID, &ph.Status, &ph.Iterations, &startedAt, &completedAt,
			&ph.InputTokens, &ph.OutputTokens, &ph.CostUSD, &errorMsg); err != nil {
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
		phases = append(phases, ph)
	}

	return phases, nil
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
func (p *ProjectDB) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	rows, err := p.Query(`
		SELECT task_id, phase, snippet(transcripts_fts, 0, '<mark>', '</mark>', '...', 32), rank
		FROM transcripts_fts
		WHERE content MATCH ?
		ORDER BY rank
		LIMIT 50
	`, query)
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

	return matches, nil
}
