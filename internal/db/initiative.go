package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// ============================================================================
// Initiative Types
// ============================================================================

// Initiative represents an initiative stored in the database.
type Initiative struct {
	ID               string
	Title            string
	Status           string
	OwnerInitials    string
	OwnerDisplayName string
	OwnerEmail       string
	Vision           string
	BranchBase       string // Target branch for tasks in this initiative
	BranchPrefix     string // Branch naming prefix for tasks (e.g., "feature/auth-")
	MergeStatus      string // none, pending, merged, failed
	MergeCommit      string // SHA of merge commit when merged
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

// InitiativeTaskRef represents a task reference with its details for batch loading.
type InitiativeTaskRef struct {
	InitiativeID string
	TaskID       string
	Title        string
	Status       string
	Sequence     int
}

// ============================================================================
// Initiative CRUD Operations
// ============================================================================

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
	defer func() { _ = rows.Close() }()

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

// ============================================================================
// Initiative Decision Operations
// ============================================================================

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
	defer func() { _ = rows.Close() }()

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

// ============================================================================
// Initiative Task Operations
// ============================================================================

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
	defer func() { _ = rows.Close() }()

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

// ClearInitiativeTasks removes all task references from an initiative.
func (p *ProjectDB) ClearInitiativeTasks(initiativeID string) error {
	_, err := p.Exec(`DELETE FROM initiative_tasks WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative tasks: %w", err)
	}
	return nil
}

// ============================================================================
// Initiative Dependency Operations
// ============================================================================

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

// GetInitiativeDependents retrieves IDs of initiatives that depend on initiativeID.
func (p *ProjectDB) GetInitiativeDependents(initiativeID string) ([]string, error) {
	rows, err := p.Query(`SELECT initiative_id FROM initiative_dependencies WHERE depends_on = ?`, initiativeID)
	if err != nil {
		return nil, fmt.Errorf("get initiative dependents: %w", err)
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

// ClearInitiativeDependencies removes all dependencies for an initiative.
func (p *ProjectDB) ClearInitiativeDependencies(initiativeID string) error {
	_, err := p.Exec(`DELETE FROM initiative_dependencies WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative dependencies: %w", err)
	}
	return nil
}

// ============================================================================
// Initiative Batch Loading Operations
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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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

// ============================================================================
// Initiative Transaction-Aware Operations
// ============================================================================

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

// ClearInitiativeDecisionsTx removes all decisions from an initiative within a transaction.
func ClearInitiativeDecisionsTx(tx *TxOps, initiativeID string) error {
	_, err := tx.Exec(`DELETE FROM initiative_decisions WHERE initiative_id = ?`, initiativeID)
	if err != nil {
		return fmt.Errorf("clear initiative decisions: %w", err)
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
