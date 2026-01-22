package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Constitution represents project-level principles that guide all task execution.
// It uses a singleton pattern - only one constitution per project (id=1).
type Constitution struct {
	Content     string    // Markdown content with principles
	Version     string    // Semantic version for tracking changes
	ContentHash string    // SHA256 hash of content for change detection
	CreatedAt   time.Time // When first created
	UpdatedAt   time.Time // When last modified
}

// ConstitutionCheck records validation of a spec against the constitution.
type ConstitutionCheck struct {
	ID         int64     // Auto-incremented ID
	TaskID     string    // Task that was checked
	Phase      string    // Phase that was checked (usually spec/tiny_spec)
	Passed     bool      // Whether the check passed
	Violations []string  // List of violation descriptions if failed
	CheckedAt  time.Time // When the check was performed
}

// ErrNoConstitution is returned when no constitution exists for the project.
var ErrNoConstitution = errors.New("no constitution configured for this project")

// SaveConstitution saves or updates the project's constitution.
// Uses upsert pattern - creates new or updates existing.
func (p *ProjectDB) SaveConstitution(c *Constitution) error {
	// Compute content hash
	hash := sha256.Sum256([]byte(c.Content))
	c.ContentHash = hex.EncodeToString(hash[:])

	now := time.Now().UTC().Format(time.RFC3339)

	// Use INSERT OR REPLACE for upsert (SQLite)
	// The CHECK constraint ensures id=1
	query := `
		INSERT INTO constitutions (id, content, version, content_hash, created_at, updated_at)
		VALUES (1, ?, ?, ?, COALESCE((SELECT created_at FROM constitutions WHERE id = 1), ?), ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			version = excluded.version,
			content_hash = excluded.content_hash,
			updated_at = excluded.updated_at
	`

	_, err := p.Exec(query, c.Content, c.Version, c.ContentHash, now, now)
	if err != nil {
		return fmt.Errorf("save constitution: %w", err)
	}

	return nil
}

// LoadConstitution loads the project's constitution.
// Returns ErrNoConstitution if no constitution is configured.
func (p *ProjectDB) LoadConstitution() (*Constitution, error) {
	query := `
		SELECT content, version, content_hash, created_at, updated_at
		FROM constitutions
		WHERE id = 1
	`

	var c Constitution
	var createdAt, updatedAt string

	err := p.QueryRow(query).Scan(
		&c.Content,
		&c.Version,
		&c.ContentHash,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoConstitution
	}
	if err != nil {
		return nil, fmt.Errorf("load constitution: %w", err)
	}

	// Parse timestamps
	if createdAt != "" {
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	if updatedAt != "" {
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	return &c, nil
}

// ConstitutionExists checks if a constitution is configured for the project.
func (p *ProjectDB) ConstitutionExists() (bool, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM constitutions WHERE id = 1").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check constitution exists: %w", err)
	}
	return count > 0, nil
}

// DeleteConstitution removes the project's constitution.
func (p *ProjectDB) DeleteConstitution() error {
	_, err := p.Exec("DELETE FROM constitutions WHERE id = 1")
	if err != nil {
		return fmt.Errorf("delete constitution: %w", err)
	}
	return nil
}

// SaveConstitutionCheck records a constitution validation check.
func (p *ProjectDB) SaveConstitutionCheck(check *ConstitutionCheck) error {
	// Serialize violations to JSON
	violationsJSON, err := json.Marshal(check.Violations)
	if err != nil {
		return fmt.Errorf("marshal violations: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	passed := 0
	if check.Passed {
		passed = 1
	}

	query := `
		INSERT INTO constitution_checks (task_id, phase, passed, violations, checked_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := p.Exec(query, check.TaskID, check.Phase, passed, string(violationsJSON), now)
	if err != nil {
		return fmt.Errorf("save constitution check: %w", err)
	}

	id, _ := result.LastInsertId()
	check.ID = id
	check.CheckedAt, _ = time.Parse(time.RFC3339, now)

	return nil
}

// GetConstitutionChecks retrieves all constitution checks for a task.
func (p *ProjectDB) GetConstitutionChecks(taskID string) ([]ConstitutionCheck, error) {
	query := `
		SELECT id, task_id, phase, passed, violations, checked_at
		FROM constitution_checks
		WHERE task_id = ?
		ORDER BY checked_at DESC, id DESC
	`

	rows, err := p.Query(query, taskID)
	if err != nil {
		return nil, fmt.Errorf("query constitution checks: %w", err)
	}
	defer rows.Close()

	var checks []ConstitutionCheck
	for rows.Next() {
		var check ConstitutionCheck
		var passed int
		var violationsJSON string
		var checkedAt string

		if err := rows.Scan(&check.ID, &check.TaskID, &check.Phase, &passed, &violationsJSON, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan constitution check: %w", err)
		}

		check.Passed = passed == 1

		// Parse violations JSON
		if violationsJSON != "" && violationsJSON != "null" {
			if err := json.Unmarshal([]byte(violationsJSON), &check.Violations); err != nil {
				// Non-fatal: just log and continue
				check.Violations = nil
			}
		}

		// Parse timestamp
		if checkedAt != "" {
			check.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
		}

		checks = append(checks, check)
	}

	return checks, rows.Err()
}

// GetLatestConstitutionCheck retrieves the most recent constitution check for a task/phase.
func (p *ProjectDB) GetLatestConstitutionCheck(taskID, phase string) (*ConstitutionCheck, error) {
	query := `
		SELECT id, task_id, phase, passed, violations, checked_at
		FROM constitution_checks
		WHERE task_id = ? AND phase = ?
		ORDER BY checked_at DESC, id DESC
		LIMIT 1
	`

	var check ConstitutionCheck
	var passed int
	var violationsJSON string
	var checkedAt string

	err := p.QueryRow(query, taskID, phase).Scan(
		&check.ID, &check.TaskID, &check.Phase, &passed, &violationsJSON, &checkedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // No check found
	}
	if err != nil {
		return nil, fmt.Errorf("load constitution check: %w", err)
	}

	check.Passed = passed == 1

	// Parse violations JSON
	if violationsJSON != "" && violationsJSON != "null" {
		if err := json.Unmarshal([]byte(violationsJSON), &check.Violations); err != nil {
			check.Violations = nil
		}
	}

	// Parse timestamp
	if checkedAt != "" {
		check.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
	}

	return &check, nil
}
