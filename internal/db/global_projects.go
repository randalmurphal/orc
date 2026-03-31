package db

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GlobalDB provides operations on the global database (~/.orc/orc.db).
type GlobalDB struct {
	*DB
}

// OpenGlobal opens the global database at ~/.orc/orc.db using SQLite.
func OpenGlobal() (*GlobalDB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	path := filepath.Join(home, ".orc", "orc.db")
	return OpenGlobalAt(path)
}

// OpenGlobalAt opens the global database at a specific path using SQLite.
// This is useful for testing with isolated databases.
func OpenGlobalAt(path string) (*GlobalDB, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("global"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate global db: %w", err)
	}

	return &GlobalDB{DB: db}, nil
}

// Project represents a registered project.
type Project struct {
	ID        string
	Name      string
	Path      string
	Language  string
	CreatedAt time.Time
}

// SyncProject registers or updates a project in the global registry.
func (g *GlobalDB) SyncProject(p Project) error {
	_, err := g.Exec(`
		INSERT INTO projects (id, name, path, language, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			path = excluded.path,
			language = excluded.language
	`, p.ID, p.Name, p.Path, p.Language, p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("sync project: %w", err)
	}
	return nil
}

// GetProject retrieves a project by ID.
func (g *GlobalDB) GetProject(id string) (*Project, error) {
	row := g.QueryRow("SELECT id, name, path, language, created_at FROM projects WHERE id = ?", id)

	var p Project
	var createdAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		p.CreatedAt = t
	}

	return &p, nil
}

// GetProjectByPath retrieves a project by its filesystem path.
func (g *GlobalDB) GetProjectByPath(path string) (*Project, error) {
	row := g.QueryRow("SELECT id, name, path, language, created_at FROM projects WHERE path = ?", path)

	var p Project
	var createdAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
		return nil, fmt.Errorf("get project by path %s: %w", path, err)
	}

	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		p.CreatedAt = t
	}

	return &p, nil
}

// ListProjects returns all registered projects.
func (g *GlobalDB) ListProjects() ([]Project, error) {
	rows, err := g.Query("SELECT id, name, path, language, created_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &createdAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			p.CreatedAt = t
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

// DeleteProject removes a project from the registry.
func (g *GlobalDB) DeleteProject(id string) error {
	_, err := g.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
