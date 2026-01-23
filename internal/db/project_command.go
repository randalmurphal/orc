package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ProjectCommand represents a language-specific command for quality checks.
// Commands are seeded on `orc init` based on project detection and can be
// customized per-project.
type ProjectCommand struct {
	Name         string    `json:"name"`          // Primary key: 'test', 'lint', 'build', 'typecheck', or custom
	Domain       string    `json:"domain"`        // 'code' or 'custom'
	Command      string    `json:"command"`       // Full command: 'go test ./...'
	ShortCommand string    `json:"short_command"` // Optional short variant: 'go test -short ./...'
	Enabled      bool      `json:"enabled"`       // Whether this command is active
	Description  string    `json:"description"`   // Human-readable description
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ErrProjectCommandNotFound is returned when a command doesn't exist.
var ErrProjectCommandNotFound = errors.New("project command not found")

// SaveProjectCommand creates or updates a project command.
func (p *ProjectDB) SaveProjectCommand(cmd *ProjectCommand) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := p.Exec(`
		INSERT INTO project_commands (name, domain, command, short_command, enabled, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM project_commands WHERE name = ?), ?), ?)
		ON CONFLICT(name) DO UPDATE SET
			domain = excluded.domain,
			command = excluded.command,
			short_command = excluded.short_command,
			enabled = excluded.enabled,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, cmd.Name, cmd.Domain, cmd.Command, cmd.ShortCommand, cmd.Enabled, cmd.Description,
		cmd.Name, now, now)
	if err != nil {
		return fmt.Errorf("save project command %s: %w", cmd.Name, err)
	}

	return nil
}

// GetProjectCommand retrieves a project command by name.
// Returns ErrProjectCommandNotFound if the command doesn't exist.
func (p *ProjectDB) GetProjectCommand(name string) (*ProjectCommand, error) {
	row := p.QueryRow(`
		SELECT name, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE name = ?
	`, name)

	cmd, err := scanProjectCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectCommandNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project command %s: %w", name, err)
	}

	return cmd, nil
}

// ListProjectCommands returns all project commands ordered by name.
func (p *ProjectDB) ListProjectCommands() ([]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list project commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var commands []*ProjectCommand
	for rows.Next() {
		cmd, err := scanProjectCommandRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project command: %w", err)
		}
		commands = append(commands, cmd)
	}

	return commands, rows.Err()
}

// ListProjectCommandsByDomain returns all project commands for a specific domain.
func (p *ProjectDB) ListProjectCommandsByDomain(domain string) ([]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE domain = ?
		ORDER BY name ASC
	`, domain)
	if err != nil {
		return nil, fmt.Errorf("list project commands by domain %s: %w", domain, err)
	}
	defer func() { _ = rows.Close() }()

	var commands []*ProjectCommand
	for rows.Next() {
		cmd, err := scanProjectCommandRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project command: %w", err)
		}
		commands = append(commands, cmd)
	}

	return commands, rows.Err()
}

// ListEnabledProjectCommands returns all enabled project commands.
func (p *ProjectDB) ListEnabledProjectCommands() ([]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE enabled = TRUE
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list enabled project commands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var commands []*ProjectCommand
	for rows.Next() {
		cmd, err := scanProjectCommandRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project command: %w", err)
		}
		commands = append(commands, cmd)
	}

	return commands, rows.Err()
}

// GetProjectCommandsMap returns all project commands as a map keyed by name.
// Useful for quick lookups when executing quality checks.
func (p *ProjectDB) GetProjectCommandsMap() (map[string]*ProjectCommand, error) {
	commands, err := p.ListProjectCommands()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ProjectCommand, len(commands))
	for _, cmd := range commands {
		result[cmd.Name] = cmd
	}

	return result, nil
}

// DeleteProjectCommand removes a project command by name.
func (p *ProjectDB) DeleteProjectCommand(name string) error {
	result, err := p.Exec("DELETE FROM project_commands WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete project command %s: %w", name, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProjectCommandNotFound
	}

	return nil
}

// SetProjectCommandEnabled enables or disables a project command.
func (p *ProjectDB) SetProjectCommandEnabled(name string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := p.Exec(`
		UPDATE project_commands 
		SET enabled = ?, updated_at = ?
		WHERE name = ?
	`, enabled, now, name)
	if err != nil {
		return fmt.Errorf("set project command enabled %s: %w", name, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProjectCommandNotFound
	}

	return nil
}

// --------- Scanners ---------

func scanProjectCommand(row rowScanner) (*ProjectCommand, error) {
	cmd := &ProjectCommand{}
	var shortCommand, description sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&cmd.Name,
		&cmd.Domain,
		&cmd.Command,
		&shortCommand,
		&cmd.Enabled,
		&description,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	cmd.ShortCommand = shortCommand.String
	cmd.Description = description.String
	cmd.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	cmd.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return cmd, nil
}

func scanProjectCommandRow(rows *sql.Rows) (*ProjectCommand, error) {
	return scanProjectCommand(rows)
}
