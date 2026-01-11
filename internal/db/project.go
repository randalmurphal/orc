package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phases: %w", err)
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
