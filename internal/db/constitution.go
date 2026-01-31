package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConstitutionFileName is the name of the constitution file in the .orc directory.
const ConstitutionFileName = "CONSTITUTION.md"

// Constitution represents project-level principles that guide all task execution.
// Stored as a file at .orc/CONSTITUTION.md for git-trackability.
type Constitution struct {
	Content   string    // Markdown content with principles
	Path      string    // File path (always .orc/CONSTITUTION.md)
	UpdatedAt time.Time // File modification time
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

// constitutionPath returns the path to the constitution file.
// The constitution lives at <project>/.orc/CONSTITUTION.md (git-tracked),
// NOT in the database directory (~/.orc/projects/<id>/).
func (p *ProjectDB) constitutionPath() string {
	if p.projectDir != "" {
		return filepath.Join(p.projectDir, ".orc", ConstitutionFileName)
	}
	// Fallback for tests / in-memory DBs: derive from DB path.
	// This works when DB is at <project>/.orc/orc.db (legacy layout).
	orcDir := filepath.Dir(p.Path())
	return filepath.Join(orcDir, ConstitutionFileName)
}

// SaveConstitution saves the project's constitution to .orc/CONSTITUTION.md.
func (p *ProjectDB) SaveConstitution(content string) error {
	path := p.constitutionPath()

	// Ensure .orc directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create .orc directory: %w", err)
	}

	// Write content to file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("save constitution: %w", err)
	}

	return nil
}

// LoadConstitution loads the project's constitution from .orc/CONSTITUTION.md.
// Returns ErrNoConstitution if no constitution file exists.
func (p *ProjectDB) LoadConstitution() (*Constitution, error) {
	path := p.constitutionPath()

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNoConstitution
	}
	if err != nil {
		return nil, fmt.Errorf("load constitution: %w", err)
	}

	// Get file modification time
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat constitution: %w", err)
	}

	return &Constitution{
		Content:   string(content),
		Path:      path,
		UpdatedAt: info.ModTime(),
	}, nil
}

// ConstitutionExists checks if a constitution file exists for the project.
func (p *ProjectDB) ConstitutionExists() (bool, error) {
	path := p.constitutionPath()
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check constitution exists: %w", err)
	}
	return true, nil
}

// DeleteConstitution removes the project's constitution file.
func (p *ProjectDB) DeleteConstitution() error {
	path := p.constitutionPath()
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already doesn't exist
	}
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
	defer func() { _ = rows.Close() }()

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
