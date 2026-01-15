package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// TeamMemberRole represents the role of a team member.
type TeamMemberRole string

const (
	RoleAdmin  TeamMemberRole = "admin"
	RoleMember TeamMemberRole = "member"
	RoleViewer TeamMemberRole = "viewer"
)

// TeamMember represents a team member.
type TeamMember struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	DisplayName string         `json:"display_name"`
	Initials    string         `json:"initials"`
	Role        TeamMemberRole `json:"role"`
	CreatedAt   time.Time      `json:"created_at"`
}

// TaskClaim represents a claim on a task by a team member.
type TaskClaim struct {
	TaskID     string     `json:"task_id"`
	MemberID   string     `json:"member_id"`
	ClaimedAt  time.Time  `json:"claimed_at"`
	ReleasedAt *time.Time `json:"released_at,omitempty"`
}

// ActivityAction represents the type of activity action.
type ActivityAction string

const (
	ActionCreated   ActivityAction = "created"
	ActionStarted   ActivityAction = "started"
	ActionPaused    ActivityAction = "paused"
	ActionCompleted ActivityAction = "completed"
	ActionFailed    ActivityAction = "failed"
	ActionCommented ActivityAction = "commented"
	ActionClaimed   ActivityAction = "claimed"
	ActionReleased  ActivityAction = "released"
)

// ActivityLog represents an activity log entry.
type ActivityLog struct {
	ID        int64          `json:"id"`
	TaskID    string         `json:"task_id,omitempty"`
	MemberID  string         `json:"member_id,omitempty"`
	Action    ActivityAction `json:"action"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// CreateTeamMember creates a new team member.
func (p *ProjectDB) CreateTeamMember(m *TeamMember) error {
	if m.ID == "" {
		m.ID = generateMemberID()
	}
	if m.Role == "" {
		m.Role = RoleMember
	}

	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			INSERT INTO team_members (id, email, display_name, initials, role, created_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'))
		`
	} else {
		query = `
			INSERT INTO team_members (id, email, display_name, initials, role, created_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`
	}

	_, err := p.Exec(query, m.ID, m.Email, m.DisplayName, m.Initials, m.Role)
	if err != nil {
		return fmt.Errorf("create team member: %w", err)
	}

	// Reload to get created_at timestamp
	created, err := p.GetTeamMember(m.ID)
	if err == nil && created != nil {
		m.CreatedAt = created.CreatedAt
	}

	return nil
}

// GetTeamMember retrieves a team member by ID.
func (p *ProjectDB) GetTeamMember(id string) (*TeamMember, error) {
	row := p.QueryRow(`
		SELECT id, email, display_name, initials, role, created_at
		FROM team_members WHERE id = ?
	`, id)
	return scanTeamMember(row)
}

// GetTeamMemberByEmail retrieves a team member by email.
func (p *ProjectDB) GetTeamMemberByEmail(email string) (*TeamMember, error) {
	row := p.QueryRow(`
		SELECT id, email, display_name, initials, role, created_at
		FROM team_members WHERE email = ?
	`, email)
	return scanTeamMember(row)
}

// ListTeamMembers returns all team members.
func (p *ProjectDB) ListTeamMembers() ([]TeamMember, error) {
	rows, err := p.Query(`
		SELECT id, email, display_name, initials, role, created_at
		FROM team_members ORDER BY display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		m, err := scanTeamMemberRows(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, *m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}

	return members, nil
}

// UpdateTeamMember updates a team member.
func (p *ProjectDB) UpdateTeamMember(m *TeamMember) error {
	_, err := p.Exec(`
		UPDATE team_members
		SET email = ?, display_name = ?, initials = ?, role = ?
		WHERE id = ?
	`, m.Email, m.DisplayName, m.Initials, m.Role, m.ID)
	if err != nil {
		return fmt.Errorf("update team member: %w", err)
	}
	return nil
}

// DeleteTeamMember removes a team member.
func (p *ProjectDB) DeleteTeamMember(id string) error {
	_, err := p.Exec("DELETE FROM team_members WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete team member: %w", err)
	}
	return nil
}

// ClaimTask creates a claim on a task by a team member.
// Returns an error if the task is already claimed by another member.
func (p *ProjectDB) ClaimTask(taskID, memberID string) error {
	// Check if task is already claimed
	existingClaim, err := p.GetActiveTaskClaim(taskID)
	if err != nil {
		return fmt.Errorf("check existing claim: %w", err)
	}
	if existingClaim != nil && existingClaim.MemberID != memberID {
		return fmt.Errorf("task already claimed by another member")
	}
	if existingClaim != nil && existingClaim.MemberID == memberID {
		// Already claimed by this member, nothing to do
		return nil
	}

	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			INSERT INTO task_claims (task_id, member_id, claimed_at)
			VALUES (?, ?, datetime('now'))
		`
	} else {
		query = `
			INSERT INTO task_claims (task_id, member_id, claimed_at)
			VALUES ($1, $2, NOW())
		`
	}

	_, err = p.Exec(query, taskID, memberID)
	if err != nil {
		return fmt.Errorf("claim task: %w", err)
	}

	// Log the activity
	if err := p.LogActivity(taskID, memberID, ActionClaimed, nil); err != nil {
		// Log but don't fail the claim
		fmt.Printf("warning: failed to log claim activity: %v\n", err)
	}

	return nil
}

// ReleaseTask releases a claim on a task.
func (p *ProjectDB) ReleaseTask(taskID, memberID string) error {
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			UPDATE task_claims
			SET released_at = datetime('now')
			WHERE task_id = ? AND member_id = ? AND released_at IS NULL
		`
	} else {
		query = `
			UPDATE task_claims
			SET released_at = NOW()
			WHERE task_id = $1 AND member_id = $2 AND released_at IS NULL
		`
	}

	_, err := p.Exec(query, taskID, memberID)
	if err != nil {
		return fmt.Errorf("release task: %w", err)
	}

	// Log the activity
	if err := p.LogActivity(taskID, memberID, ActionReleased, nil); err != nil {
		// Log but don't fail the release
		fmt.Printf("warning: failed to log release activity: %v\n", err)
	}

	return nil
}

// GetActiveTaskClaim returns the active claim for a task (if any).
func (p *ProjectDB) GetActiveTaskClaim(taskID string) (*TaskClaim, error) {
	row := p.QueryRow(`
		SELECT task_id, member_id, claimed_at, released_at
		FROM task_claims
		WHERE task_id = ? AND released_at IS NULL
		ORDER BY claimed_at DESC
		LIMIT 1
	`, taskID)

	var claim TaskClaim
	var releasedAt sql.NullString
	var claimedAt string

	err := row.Scan(&claim.TaskID, &claim.MemberID, &claimedAt, &releasedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active task claim: %w", err)
	}

	claim.ClaimedAt = parseTimestamp(claimedAt)
	if releasedAt.Valid {
		t := parseTimestamp(releasedAt.String)
		claim.ReleasedAt = &t
	}

	return &claim, nil
}

// GetMemberClaims returns all active claims for a team member.
func (p *ProjectDB) GetMemberClaims(memberID string) ([]TaskClaim, error) {
	rows, err := p.Query(`
		SELECT task_id, member_id, claimed_at, released_at
		FROM task_claims
		WHERE member_id = ? AND released_at IS NULL
		ORDER BY claimed_at DESC
	`, memberID)
	if err != nil {
		return nil, fmt.Errorf("get member claims: %w", err)
	}
	defer rows.Close()

	var claims []TaskClaim
	for rows.Next() {
		var claim TaskClaim
		var releasedAt sql.NullString
		var claimedAt string

		if err := rows.Scan(&claim.TaskID, &claim.MemberID, &claimedAt, &releasedAt); err != nil {
			return nil, fmt.Errorf("scan task claim: %w", err)
		}

		claim.ClaimedAt = parseTimestamp(claimedAt)
		if releasedAt.Valid {
			t := parseTimestamp(releasedAt.String)
			claim.ReleasedAt = &t
		}

		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task claims: %w", err)
	}

	return claims, nil
}

// GetTaskClaimHistory returns all claims for a task (including released).
func (p *ProjectDB) GetTaskClaimHistory(taskID string) ([]TaskClaim, error) {
	rows, err := p.Query(`
		SELECT task_id, member_id, claimed_at, released_at
		FROM task_claims
		WHERE task_id = ?
		ORDER BY claimed_at DESC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task claim history: %w", err)
	}
	defer rows.Close()

	var claims []TaskClaim
	for rows.Next() {
		var claim TaskClaim
		var releasedAt sql.NullString
		var claimedAt string

		if err := rows.Scan(&claim.TaskID, &claim.MemberID, &claimedAt, &releasedAt); err != nil {
			return nil, fmt.Errorf("scan task claim: %w", err)
		}

		claim.ClaimedAt = parseTimestamp(claimedAt)
		if releasedAt.Valid {
			t := parseTimestamp(releasedAt.String)
			claim.ReleasedAt = &t
		}

		claims = append(claims, claim)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task claims: %w", err)
	}

	return claims, nil
}

// IsTaskClaimed checks if a task is currently claimed.
func (p *ProjectDB) IsTaskClaimed(taskID string) (bool, error) {
	claim, err := p.GetActiveTaskClaim(taskID)
	if err != nil {
		return false, err
	}
	return claim != nil, nil
}

// IsTaskClaimedBy checks if a task is claimed by a specific member.
func (p *ProjectDB) IsTaskClaimedBy(taskID, memberID string) (bool, error) {
	claim, err := p.GetActiveTaskClaim(taskID)
	if err != nil {
		return false, err
	}
	return claim != nil && claim.MemberID == memberID, nil
}

// LogActivity records an activity in the activity log.
func (p *ProjectDB) LogActivity(taskID, memberID string, action ActivityAction, details map[string]any) error {
	var detailsJSON *string
	if details != nil {
		data, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal activity details: %w", err)
		}
		s := string(data)
		detailsJSON = &s
	}

	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			INSERT INTO activity_log (task_id, member_id, action, details, created_at)
			VALUES (?, ?, ?, ?, datetime('now'))
		`
	} else {
		query = `
			INSERT INTO activity_log (task_id, member_id, action, details, created_at)
			VALUES ($1, $2, $3, $4, NOW())
		`
	}

	// Handle empty strings as NULL for foreign key references
	var taskIDArg, memberIDArg interface{}
	if taskID == "" {
		taskIDArg = nil
	} else {
		taskIDArg = taskID
	}
	if memberID == "" {
		memberIDArg = nil
	} else {
		memberIDArg = memberID
	}

	_, err := p.Exec(query, taskIDArg, memberIDArg, action, detailsJSON)
	if err != nil {
		return fmt.Errorf("log activity: %w", err)
	}

	return nil
}

// ListActivityOpts provides filtering options for activity logs.
type ListActivityOpts struct {
	TaskID   string
	MemberID string
	Action   ActivityAction
	Since    *time.Time
	Limit    int
	Offset   int
}

// ListActivity returns activity logs matching the given options.
func (p *ProjectDB) ListActivity(opts ListActivityOpts) ([]ActivityLog, error) {
	query := `
		SELECT id, task_id, member_id, action, details, created_at
		FROM activity_log WHERE 1=1
	`
	args := []any{}
	argIndex := 1

	if opts.TaskID != "" {
		query += fmt.Sprintf(" AND task_id = %s", p.Placeholder(argIndex))
		args = append(args, opts.TaskID)
		argIndex++
	}
	if opts.MemberID != "" {
		query += fmt.Sprintf(" AND member_id = %s", p.Placeholder(argIndex))
		args = append(args, opts.MemberID)
		argIndex++
	}
	if opts.Action != "" {
		query += fmt.Sprintf(" AND action = %s", p.Placeholder(argIndex))
		args = append(args, opts.Action)
		argIndex++
	}
	if opts.Since != nil {
		query += fmt.Sprintf(" AND created_at >= %s", p.Placeholder(argIndex))
		args = append(args, opts.Since.Format(time.RFC3339))
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %s", p.Placeholder(argIndex))
		args = append(args, opts.Limit)
		argIndex++
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %s", p.Placeholder(argIndex))
		args = append(args, opts.Offset)
	}

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()

	var activities []ActivityLog
	for rows.Next() {
		a, err := scanActivityLogRows(rows)
		if err != nil {
			return nil, err
		}
		activities = append(activities, *a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity logs: %w", err)
	}

	return activities, nil
}

// GetRecentActivity returns the most recent activity logs.
func (p *ProjectDB) GetRecentActivity(limit int) ([]ActivityLog, error) {
	return p.ListActivity(ListActivityOpts{Limit: limit})
}

// GetTaskActivity returns all activity for a specific task.
func (p *ProjectDB) GetTaskActivity(taskID string) ([]ActivityLog, error) {
	return p.ListActivity(ListActivityOpts{TaskID: taskID})
}

// GetMemberActivity returns all activity for a specific member.
func (p *ProjectDB) GetMemberActivity(memberID string) ([]ActivityLog, error) {
	return p.ListActivity(ListActivityOpts{MemberID: memberID})
}

// Helper functions

func scanTeamMember(row *sql.Row) (*TeamMember, error) {
	var m TeamMember
	var createdAt string

	err := row.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Initials, &m.Role, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan team member: %w", err)
	}

	m.CreatedAt = parseTimestamp(createdAt)
	return &m, nil
}

func scanTeamMemberRows(rows *sql.Rows) (*TeamMember, error) {
	var m TeamMember
	var createdAt string

	err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Initials, &m.Role, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan team member: %w", err)
	}

	m.CreatedAt = parseTimestamp(createdAt)
	return &m, nil
}

func scanActivityLogRows(rows *sql.Rows) (*ActivityLog, error) {
	var a ActivityLog
	var taskID, memberID sql.NullString
	var details sql.NullString
	var createdAt string

	err := rows.Scan(&a.ID, &taskID, &memberID, &a.Action, &details, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("scan activity log: %w", err)
	}

	if taskID.Valid {
		a.TaskID = taskID.String
	}
	if memberID.Valid {
		a.MemberID = memberID.String
	}
	if details.Valid && details.String != "" {
		if err := json.Unmarshal([]byte(details.String), &a.Details); err != nil {
			// Log but don't fail - store as raw string in details
			a.Details = map[string]any{"raw": details.String}
		}
	}
	a.CreatedAt = parseTimestamp(createdAt)

	return &a, nil
}

// generateMemberID generates a unique ID for a team member.
func generateMemberID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "TM-" + hex.EncodeToString(b)[:8]
}

// TaskWithClaim extends Task with claim information for team visibility.
type TaskWithClaim struct {
	Task
	ClaimedBy *TeamMember `json:"claimed_by,omitempty"`
	ClaimedAt *time.Time  `json:"claimed_at,omitempty"`
}

// ListTasksWithClaims returns tasks with their claim information.
func (p *ProjectDB) ListTasksWithClaims(opts ListOpts) ([]TaskWithClaim, int, error) {
	tasks, total, err := p.ListTasks(opts)
	if err != nil {
		return nil, 0, err
	}

	result := make([]TaskWithClaim, len(tasks))
	for i, t := range tasks {
		result[i] = TaskWithClaim{Task: t}

		// Get active claim for this task
		claim, err := p.GetActiveTaskClaim(t.ID)
		if err != nil {
			// Log but don't fail
			continue
		}
		if claim != nil {
			result[i].ClaimedAt = &claim.ClaimedAt
			// Get member info
			member, err := p.GetTeamMember(claim.MemberID)
			if err == nil && member != nil {
				result[i].ClaimedBy = member
			}
		}
	}

	return result, total, nil
}
