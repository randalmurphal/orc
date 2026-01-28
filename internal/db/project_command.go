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
//
// Scope enables multi-language support:
//   - "" (empty) = global command, runs for all languages
//   - "go" = Go-specific command
//   - "frontend" = Frontend/TypeScript-specific command
//   - "python" = Python-specific command
//
// With scope, the same command name can exist multiple times:
//   - tests (scope="") - global test runner
//   - tests (scope="go") - go test ./...
//   - tests (scope="frontend") - npm test
type ProjectCommand struct {
	Name         string    `json:"name"`          // Command name: 'tests', 'lint', 'build', 'typecheck', or custom
	Scope        string    `json:"scope"`         // Language/stack scope: '', 'go', 'frontend', 'python', etc.
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

// ScopedCommandKey returns the canonical key for a scoped command.
// Format: "name" for global, "name:scope" for scoped.
func ScopedCommandKey(name, scope string) string {
	if scope == "" {
		return name
	}
	return name + ":" + scope
}

// SaveProjectCommand creates or updates a project command.
// The unique key is (name, scope), allowing the same command name with different scopes.
func (p *ProjectDB) SaveProjectCommand(cmd *ProjectCommand) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := p.Exec(`
		INSERT INTO project_commands (name, scope, domain, command, short_command, enabled, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM project_commands WHERE name = ? AND scope = ?), ?), ?)
		ON CONFLICT(name, scope) DO UPDATE SET
			domain = excluded.domain,
			command = excluded.command,
			short_command = excluded.short_command,
			enabled = excluded.enabled,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, cmd.Name, cmd.Scope, cmd.Domain, cmd.Command, cmd.ShortCommand, cmd.Enabled, cmd.Description,
		cmd.Name, cmd.Scope, now, now)
	if err != nil {
		return fmt.Errorf("save project command %s (scope=%s): %w", cmd.Name, cmd.Scope, err)
	}

	return nil
}

// GetProjectCommand retrieves a project command by name and scope.
// For backward compatibility, scope defaults to "" (global).
// Returns ErrProjectCommandNotFound if the command doesn't exist.
func (p *ProjectDB) GetProjectCommand(name string) (*ProjectCommand, error) {
	return p.GetProjectCommandScoped(name, "")
}

// GetProjectCommandScoped retrieves a project command by name and scope.
// Returns ErrProjectCommandNotFound if the command doesn't exist.
func (p *ProjectDB) GetProjectCommandScoped(name, scope string) (*ProjectCommand, error) {
	row := p.QueryRow(`
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE name = ? AND scope = ?
	`, name, scope)

	cmd, err := scanProjectCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectCommandNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project command %s (scope=%s): %w", name, scope, err)
	}

	return cmd, nil
}

// ListProjectCommands returns all project commands ordered by name and scope.
func (p *ProjectDB) ListProjectCommands() ([]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		ORDER BY name ASC, scope ASC
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

// ListProjectCommandsByScope returns all project commands for a specific scope.
// Use "" for global commands, "go" for Go-specific, "frontend" for frontend, etc.
func (p *ProjectDB) ListProjectCommandsByScope(scope string) ([]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE scope = ?
		ORDER BY name ASC
	`, scope)
	if err != nil {
		return nil, fmt.Errorf("list project commands by scope %s: %w", scope, err)
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
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE domain = ?
		ORDER BY name ASC, scope ASC
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
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE enabled = TRUE
		ORDER BY name ASC, scope ASC
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

// GetProjectCommandsMap returns all project commands as a map keyed by "name:scope".
// For global commands (scope=""), the key is just "name".
// Useful for quick lookups when executing quality checks.
func (p *ProjectDB) GetProjectCommandsMap() (map[string]*ProjectCommand, error) {
	commands, err := p.ListProjectCommands()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ProjectCommand, len(commands))
	for _, cmd := range commands {
		key := ScopedCommandKey(cmd.Name, cmd.Scope)
		result[key] = cmd
	}

	return result, nil
}

// GetProjectCommandsForScope returns commands that apply to a specific scope.
// This includes both scope-specific commands AND global (scope="") commands.
// Scope-specific commands take precedence over global ones.
func (p *ProjectDB) GetProjectCommandsForScope(scope string) (map[string]*ProjectCommand, error) {
	rows, err := p.Query(`
		SELECT name, scope, domain, command, short_command, enabled, description, created_at, updated_at
		FROM project_commands
		WHERE scope = ? OR scope = ''
		ORDER BY name ASC, scope DESC
	`, scope)
	if err != nil {
		return nil, fmt.Errorf("get project commands for scope %s: %w", scope, err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]*ProjectCommand)
	for rows.Next() {
		cmd, err := scanProjectCommandRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project command: %w", err)
		}
		// Scope-specific commands take precedence (loaded first due to ORDER BY scope DESC)
		if _, exists := result[cmd.Name]; !exists {
			result[cmd.Name] = cmd
		}
	}

	return result, rows.Err()
}

// DeleteProjectCommand removes a global project command by name.
// For backward compatibility, this only deletes commands with scope="".
func (p *ProjectDB) DeleteProjectCommand(name string) error {
	return p.DeleteProjectCommandScoped(name, "")
}

// DeleteProjectCommandScoped removes a project command by name and scope.
func (p *ProjectDB) DeleteProjectCommandScoped(name, scope string) error {
	result, err := p.Exec("DELETE FROM project_commands WHERE name = ? AND scope = ?", name, scope)
	if err != nil {
		return fmt.Errorf("delete project command %s (scope=%s): %w", name, scope, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProjectCommandNotFound
	}

	return nil
}

// SetProjectCommandEnabled enables or disables a global project command.
// For backward compatibility, this only affects commands with scope="".
func (p *ProjectDB) SetProjectCommandEnabled(name string, enabled bool) error {
	return p.SetProjectCommandEnabledScoped(name, "", enabled)
}

// SetProjectCommandEnabledScoped enables or disables a project command by name and scope.
func (p *ProjectDB) SetProjectCommandEnabledScoped(name, scope string, enabled bool) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := p.Exec(`
		UPDATE project_commands
		SET enabled = ?, updated_at = ?
		WHERE name = ? AND scope = ?
	`, enabled, now, name, scope)
	if err != nil {
		return fmt.Errorf("set project command enabled %s (scope=%s): %w", name, scope, err)
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
	var scope, shortCommand, description sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(
		&cmd.Name,
		&scope,
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

	cmd.Scope = scope.String
	cmd.ShortCommand = shortCommand.String
	cmd.Description = description.String
	cmd.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	cmd.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return cmd, nil
}

func scanProjectCommandRow(rows *sql.Rows) (*ProjectCommand, error) {
	return scanProjectCommand(rows)
}
