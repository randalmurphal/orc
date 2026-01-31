package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Skill represents a reusable skill stored in GlobalDB.
type Skill struct {
	ID              string
	Name            string
	Description     string
	Content         string
	SupportingFiles map[string]string // filename -> content
	IsBuiltin       bool
	CreatedAt       string
	UpdatedAt       string
}

// SaveSkill saves or updates a skill in global DB.
func (g *GlobalDB) SaveSkill(s *Skill) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if s.CreatedAt == "" {
		s.CreatedAt = now
	}
	s.UpdatedAt = now

	var supportingFilesJSON *string
	if s.SupportingFiles != nil {
		data, err := json.Marshal(s.SupportingFiles)
		if err != nil {
			return fmt.Errorf("marshal supporting files: %w", err)
		}
		str := string(data)
		supportingFilesJSON = &str
	}

	_, err := g.Exec(`
		INSERT INTO skills (id, name, description, content, supporting_files, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			content = excluded.content,
			supporting_files = excluded.supporting_files,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`, s.ID, s.Name, s.Description, s.Content, supportingFilesJSON, s.IsBuiltin, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save skill %s: %w", s.ID, err)
	}
	return nil
}

// GetSkill retrieves a skill by ID from global DB.
// Returns nil, nil if not found.
func (g *GlobalDB) GetSkill(id string) (*Skill, error) {
	var s Skill
	var supportingFilesJSON sql.NullString

	err := g.QueryRow(`
		SELECT id, name, description, content, supporting_files, is_builtin, created_at, updated_at
		FROM skills WHERE id = ?
	`, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.Content,
		&supportingFilesJSON, &s.IsBuiltin, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get skill %s: %w", id, err)
	}

	if supportingFilesJSON.Valid && supportingFilesJSON.String != "" {
		if err := json.Unmarshal([]byte(supportingFilesJSON.String), &s.SupportingFiles); err != nil {
			return nil, fmt.Errorf("unmarshal supporting files: %w", err)
		}
	}

	return &s, nil
}

// ListSkills returns all skills from global DB.
func (g *GlobalDB) ListSkills() ([]*Skill, error) {
	rows, err := g.Query(`
		SELECT id, name, description, content, supporting_files, is_builtin, created_at, updated_at
		FROM skills
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var skills []*Skill
	for rows.Next() {
		var s Skill
		var supportingFilesJSON sql.NullString

		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.Content,
			&supportingFilesJSON, &s.IsBuiltin, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan skill: %w", err)
		}

		if supportingFilesJSON.Valid && supportingFilesJSON.String != "" {
			if err := json.Unmarshal([]byte(supportingFilesJSON.String), &s.SupportingFiles); err != nil {
				return nil, fmt.Errorf("unmarshal supporting files: %w", err)
			}
		}

		skills = append(skills, &s)
	}
	return skills, rows.Err()
}

// DeleteSkill deletes a skill by ID from global DB.
func (g *GlobalDB) DeleteSkill(id string) error {
	_, err := g.Exec("DELETE FROM skills WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete skill %s: %w", id, err)
	}
	return nil
}
