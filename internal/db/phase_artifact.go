package db

import (
	"database/sql"
	"fmt"
	"time"
)

// PhaseArtifact represents an artifact stored in the database.
// Unlike specs (which have their own table), this handles all other
// artifact-producing phases: design, tdd_write, breakdown, research, docs.
type PhaseArtifact struct {
	ID          int64
	TaskID      string
	PhaseID     string // design, tdd_write, breakdown, research, docs
	Content     string
	ContentHash string
	Source      string // 'executor', 'import', 'manual'
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SavePhaseArtifact creates or updates a phase artifact.
func (p *ProjectDB) SavePhaseArtifact(artifact *PhaseArtifact) error {
	now := time.Now().Format(time.RFC3339)
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now()
	}

	_, err := p.Exec(`
		INSERT INTO phase_artifacts (task_id, phase_id, content, content_hash, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			source = excluded.source,
			updated_at = excluded.updated_at
	`, artifact.TaskID, artifact.PhaseID, artifact.Content, artifact.ContentHash, artifact.Source,
		artifact.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save phase artifact: %w", err)
	}
	return nil
}

// GetPhaseArtifact retrieves a phase artifact by task ID and phase ID.
func (p *ProjectDB) GetPhaseArtifact(taskID, phaseID string) (*PhaseArtifact, error) {
	row := p.QueryRow(`
		SELECT id, task_id, phase_id, content, content_hash, source, created_at, updated_at
		FROM phase_artifacts WHERE task_id = ? AND phase_id = ?
	`, taskID, phaseID)

	var artifact PhaseArtifact
	var contentHash, source sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&artifact.ID, &artifact.TaskID, &artifact.PhaseID, &artifact.Content,
		&contentHash, &source, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get phase artifact %s/%s: %w", taskID, phaseID, err)
	}

	if contentHash.Valid {
		artifact.ContentHash = contentHash.String
	}
	if source.Valid {
		artifact.Source = source.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		artifact.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		artifact.UpdatedAt = ts
	}

	return &artifact, nil
}

// GetAllPhaseArtifacts retrieves all artifacts for a task.
func (p *ProjectDB) GetAllPhaseArtifacts(taskID string) ([]*PhaseArtifact, error) {
	rows, err := p.Query(`
		SELECT id, task_id, phase_id, content, content_hash, source, created_at, updated_at
		FROM phase_artifacts WHERE task_id = ?
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get all phase artifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var artifacts []*PhaseArtifact
	for rows.Next() {
		var artifact PhaseArtifact
		var contentHash, source sql.NullString
		var createdAt, updatedAt string

		if err := rows.Scan(&artifact.ID, &artifact.TaskID, &artifact.PhaseID, &artifact.Content,
			&contentHash, &source, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan phase artifact: %w", err)
		}

		if contentHash.Valid {
			artifact.ContentHash = contentHash.String
		}
		if source.Valid {
			artifact.Source = source.String
		}
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			artifact.CreatedAt = ts
		}
		if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			artifact.UpdatedAt = ts
		}

		artifacts = append(artifacts, &artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase artifacts: %w", err)
	}

	return artifacts, nil
}

// DeletePhaseArtifact removes a phase artifact.
func (p *ProjectDB) DeletePhaseArtifact(taskID, phaseID string) error {
	_, err := p.Exec("DELETE FROM phase_artifacts WHERE task_id = ? AND phase_id = ?", taskID, phaseID)
	if err != nil {
		return fmt.Errorf("delete phase artifact: %w", err)
	}
	return nil
}

// DeleteAllPhaseArtifacts removes all artifacts for a task.
func (p *ProjectDB) DeleteAllPhaseArtifacts(taskID string) error {
	_, err := p.Exec("DELETE FROM phase_artifacts WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete all phase artifacts: %w", err)
	}
	return nil
}

// PhaseArtifactExists checks if an artifact exists for a task and phase.
func (p *ProjectDB) PhaseArtifactExists(taskID, phaseID string) (bool, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM phase_artifacts WHERE task_id = ? AND phase_id = ?",
		taskID, phaseID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check phase artifact exists: %w", err)
	}
	return count > 0, nil
}
