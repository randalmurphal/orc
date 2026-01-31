package db

import (
	"database/sql"
	"fmt"
	"time"
)

// HookScript represents a reusable hook script stored in GlobalDB.
type HookScript struct {
	ID          string
	Name        string
	Description string
	Content     string
	EventType   string
	IsBuiltin   bool
	CreatedAt   string
	UpdatedAt   string
}

// SaveHookScript saves or updates a hook script in global DB.
func (g *GlobalDB) SaveHookScript(hs *HookScript) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if hs.CreatedAt == "" {
		hs.CreatedAt = now
	}
	hs.UpdatedAt = now

	_, err := g.Exec(`
		INSERT INTO hook_scripts (id, name, description, content, event_type, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			content = excluded.content,
			event_type = excluded.event_type,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`, hs.ID, hs.Name, hs.Description, hs.Content, hs.EventType, hs.IsBuiltin, hs.CreatedAt, hs.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save hook script %s: %w", hs.ID, err)
	}
	return nil
}

// GetHookScript retrieves a hook script by ID from global DB.
// Returns nil, nil if not found.
func (g *GlobalDB) GetHookScript(id string) (*HookScript, error) {
	var hs HookScript
	err := g.QueryRow(`
		SELECT id, name, description, content, event_type, is_builtin, created_at, updated_at
		FROM hook_scripts WHERE id = ?
	`, id).Scan(
		&hs.ID, &hs.Name, &hs.Description, &hs.Content,
		&hs.EventType, &hs.IsBuiltin, &hs.CreatedAt, &hs.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get hook script %s: %w", id, err)
	}
	return &hs, nil
}

// ListHookScripts returns all hook scripts from global DB.
func (g *GlobalDB) ListHookScripts() ([]*HookScript, error) {
	rows, err := g.Query(`
		SELECT id, name, description, content, event_type, is_builtin, created_at, updated_at
		FROM hook_scripts
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list hook scripts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var scripts []*HookScript
	for rows.Next() {
		var hs HookScript
		if err := rows.Scan(
			&hs.ID, &hs.Name, &hs.Description, &hs.Content,
			&hs.EventType, &hs.IsBuiltin, &hs.CreatedAt, &hs.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan hook script: %w", err)
		}
		scripts = append(scripts, &hs)
	}
	return scripts, rows.Err()
}

// DeleteHookScript deletes a hook script by ID from global DB.
func (g *GlobalDB) DeleteHookScript(id string) error {
	_, err := g.Exec("DELETE FROM hook_scripts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete hook script %s: %w", id, err)
	}
	return nil
}
