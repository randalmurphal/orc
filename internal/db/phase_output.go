package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"
)

// PhaseOutput represents unified phase output storage.
// Replaces the fragmented specs and phase_artifacts tables.
type PhaseOutput struct {
	ID              int64
	WorkflowRunID   string
	PhaseTemplateID string
	TaskID          *string // Nullable for non-task runs
	Content         string
	ContentHash     string
	OutputVarName   string // Variable name (e.g., 'SPEC_CONTENT')
	ArtifactType    string // 'spec', 'tests', 'breakdown', etc.
	Source          string // 'workflow', 'import', 'manual', 'migrated'
	Iteration       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// SavePhaseOutput creates or updates a phase output.
func (p *ProjectDB) SavePhaseOutput(output *PhaseOutput) error {
	now := time.Now().Format(time.RFC3339)
	if output.CreatedAt.IsZero() {
		output.CreatedAt = time.Now()
	}

	// Compute content hash if not provided
	if output.ContentHash == "" && output.Content != "" {
		hash := sha256.Sum256([]byte(output.Content))
		output.ContentHash = fmt.Sprintf("%x", hash[:8])
	}

	_, err := p.Exec(`
		INSERT INTO phase_outputs (workflow_run_id, phase_template_id, task_id, content, content_hash,
			output_var_name, artifact_type, source, iteration, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_run_id, phase_template_id) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			output_var_name = excluded.output_var_name,
			artifact_type = excluded.artifact_type,
			source = excluded.source,
			iteration = excluded.iteration,
			updated_at = excluded.updated_at
	`, output.WorkflowRunID, output.PhaseTemplateID, output.TaskID, output.Content, output.ContentHash,
		output.OutputVarName, output.ArtifactType, output.Source, output.Iteration,
		output.CreatedAt.Format(time.RFC3339), now)
	if err != nil {
		return fmt.Errorf("save phase output: %w", err)
	}
	return nil
}

// GetPhaseOutput retrieves a phase output by run ID and phase template ID.
func (p *ProjectDB) GetPhaseOutput(runID, phaseTemplateID string) (*PhaseOutput, error) {
	row := p.QueryRow(`
		SELECT id, workflow_run_id, phase_template_id, task_id, content, content_hash,
			output_var_name, artifact_type, source, iteration, created_at, updated_at
		FROM phase_outputs WHERE workflow_run_id = ? AND phase_template_id = ?
	`, runID, phaseTemplateID)

	return scanPhaseOutput(row)
}

// GetPhaseOutputByVarName retrieves a phase output by run ID and variable name.
func (p *ProjectDB) GetPhaseOutputByVarName(runID, varName string) (*PhaseOutput, error) {
	row := p.QueryRow(`
		SELECT id, workflow_run_id, phase_template_id, task_id, content, content_hash,
			output_var_name, artifact_type, source, iteration, created_at, updated_at
		FROM phase_outputs WHERE workflow_run_id = ? AND output_var_name = ?
	`, runID, varName)

	return scanPhaseOutput(row)
}

// GetAllPhaseOutputs retrieves all phase outputs for a run.
func (p *ProjectDB) GetAllPhaseOutputs(runID string) ([]*PhaseOutput, error) {
	rows, err := p.Query(`
		SELECT id, workflow_run_id, phase_template_id, task_id, content, content_hash,
			output_var_name, artifact_type, source, iteration, created_at, updated_at
		FROM phase_outputs WHERE workflow_run_id = ?
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("get all phase outputs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var outputs []*PhaseOutput
	for rows.Next() {
		output, err := scanPhaseOutputRow(rows)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase outputs: %w", err)
	}

	return outputs, nil
}

// LoadPhaseOutputsAsMap returns all phase outputs for a run as a map of varName -> content.
// This is used by the variable resolver to populate prior outputs.
func (p *ProjectDB) LoadPhaseOutputsAsMap(runID string) (map[string]string, error) {
	rows, err := p.Query(`
		SELECT output_var_name, content FROM phase_outputs WHERE workflow_run_id = ?
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("load phase outputs as map: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var varName, content string
		if err := rows.Scan(&varName, &content); err != nil {
			return nil, fmt.Errorf("scan phase output map: %w", err)
		}
		result[varName] = content
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase outputs map: %w", err)
	}

	return result, nil
}

// GetSpecForTask retrieves the spec content for a task.
// This queries phase_outputs for the most recent SPEC_CONTENT for the task's workflow runs.
func (p *ProjectDB) GetSpecForTask(taskID string) (string, error) {
	row := p.QueryRow(`
		SELECT po.content FROM phase_outputs po
		JOIN workflow_runs wr ON wr.id = po.workflow_run_id
		WHERE wr.task_id = ? AND po.output_var_name = 'SPEC_CONTENT'
		ORDER BY po.created_at DESC
		LIMIT 1
	`, taskID)

	var content string
	if err := row.Scan(&content); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("get spec for task: %w", err)
	}
	return content, nil
}

// GetFullSpecForTask retrieves the full spec phase output for a task.
// Returns nil if no spec exists.
func (p *ProjectDB) GetFullSpecForTask(taskID string) (*PhaseOutput, error) {
	row := p.QueryRow(`
		SELECT po.id, po.workflow_run_id, po.phase_template_id, po.task_id, po.content, po.content_hash,
			po.output_var_name, po.artifact_type, po.source, po.iteration, po.created_at, po.updated_at
		FROM phase_outputs po
		JOIN workflow_runs wr ON wr.id = po.workflow_run_id
		WHERE wr.task_id = ? AND po.output_var_name = 'SPEC_CONTENT'
		ORDER BY po.created_at DESC
		LIMIT 1
	`, taskID)

	return scanPhaseOutput(row)
}

// SpecExistsForTask checks if a spec exists for a task.
func (p *ProjectDB) SpecExistsForTask(taskID string) (bool, error) {
	var count int
	err := p.QueryRow(`
		SELECT COUNT(*) FROM phase_outputs po
		JOIN workflow_runs wr ON wr.id = po.workflow_run_id
		WHERE wr.task_id = ? AND po.output_var_name = 'SPEC_CONTENT'
	`, taskID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check spec exists for task: %w", err)
	}
	return count > 0, nil
}

// SaveSpecForTask saves a spec for a task (for import compatibility).
// If no workflow run exists for the task, creates one using an existing workflow.
func (p *ProjectDB) SaveSpecForTask(taskID, content, source string) error {
	// Find existing workflow run for task
	var runID string
	err := p.QueryRow(`
		SELECT id FROM workflow_runs WHERE task_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, taskID).Scan(&runID)
	if err == sql.ErrNoRows {
		// Need to create a workflow run - find a valid workflow to use
		var workflowID string
		err = p.QueryRow(`SELECT id FROM workflows ORDER BY id LIMIT 1`).Scan(&workflowID)
		if err == sql.ErrNoRows {
			// No workflows exist - create a minimal 'import' workflow
			_, err = p.Exec(`
				INSERT OR IGNORE INTO workflows (id, name, description, created_at, updated_at)
				VALUES ('import', 'Import', 'Workflow for imported specs', datetime('now'), datetime('now'))
			`)
			if err != nil {
				return fmt.Errorf("create import workflow: %w", err)
			}
			workflowID = "import"
		} else if err != nil {
			return fmt.Errorf("find workflow for import: %w", err)
		}

		// Create a new workflow run for import
		runID = fmt.Sprintf("IMPORT-%s-%d", taskID, time.Now().Unix())
		_, err = p.Exec(`
			INSERT INTO workflow_runs (id, workflow_id, context_type, context_data, task_id, prompt, status, created_at, updated_at)
			VALUES (?, ?, 'task', '{}', ?, '', 'completed', datetime('now'), datetime('now'))
		`, runID, workflowID, taskID)
		if err != nil {
			return fmt.Errorf("create workflow run for spec import: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("find workflow run for task: %w", err)
	}

	// Save the spec as a phase output
	output := &PhaseOutput{
		WorkflowRunID:   runID,
		PhaseTemplateID: "spec", // Use 'spec' as the phase template ID
		TaskID:          &taskID,
		Content:         content,
		OutputVarName:   "SPEC_CONTENT",
		ArtifactType:    "spec",
		Source:          source,
		Iteration:       1,
	}
	return p.SavePhaseOutput(output)
}

// GetPhaseOutputsForTask retrieves all phase outputs for a task (across all runs).
func (p *ProjectDB) GetPhaseOutputsForTask(taskID string) ([]*PhaseOutput, error) {
	rows, err := p.Query(`
		SELECT po.id, po.workflow_run_id, po.phase_template_id, po.task_id, po.content, po.content_hash,
			po.output_var_name, po.artifact_type, po.source, po.iteration, po.created_at, po.updated_at
		FROM phase_outputs po
		JOIN workflow_runs wr ON wr.id = po.workflow_run_id
		WHERE wr.task_id = ?
		ORDER BY po.created_at ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase outputs for task: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var outputs []*PhaseOutput
	for rows.Next() {
		output, err := scanPhaseOutputRow(rows)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phase outputs for task: %w", err)
	}

	return outputs, nil
}

// DeletePhaseOutput removes a phase output.
func (p *ProjectDB) DeletePhaseOutput(runID, phaseTemplateID string) error {
	_, err := p.Exec("DELETE FROM phase_outputs WHERE workflow_run_id = ? AND phase_template_id = ?",
		runID, phaseTemplateID)
	if err != nil {
		return fmt.Errorf("delete phase output: %w", err)
	}
	return nil
}

// PhaseOutputExists checks if a phase output exists.
func (p *ProjectDB) PhaseOutputExists(runID, phaseTemplateID string) (bool, error) {
	var count int
	err := p.QueryRow("SELECT COUNT(*) FROM phase_outputs WHERE workflow_run_id = ? AND phase_template_id = ?",
		runID, phaseTemplateID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check phase output exists: %w", err)
	}
	return count > 0, nil
}

// scanPhaseOutput scans a single row into a PhaseOutput.
func scanPhaseOutput(row *sql.Row) (*PhaseOutput, error) {
	var output PhaseOutput
	var taskID, contentHash, artifactType, source sql.NullString
	var createdAt, updatedAt string

	if err := row.Scan(&output.ID, &output.WorkflowRunID, &output.PhaseTemplateID, &taskID,
		&output.Content, &contentHash, &output.OutputVarName, &artifactType, &source,
		&output.Iteration, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan phase output: %w", err)
	}

	if taskID.Valid {
		output.TaskID = &taskID.String
	}
	if contentHash.Valid {
		output.ContentHash = contentHash.String
	}
	if artifactType.Valid {
		output.ArtifactType = artifactType.String
	}
	if source.Valid {
		output.Source = source.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		output.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		output.UpdatedAt = ts
	}

	return &output, nil
}

// scanPhaseOutputRow scans from sql.Rows into a PhaseOutput.
func scanPhaseOutputRow(rows *sql.Rows) (*PhaseOutput, error) {
	var output PhaseOutput
	var taskID, contentHash, artifactType, source sql.NullString
	var createdAt, updatedAt string

	if err := rows.Scan(&output.ID, &output.WorkflowRunID, &output.PhaseTemplateID, &taskID,
		&output.Content, &contentHash, &output.OutputVarName, &artifactType, &source,
		&output.Iteration, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan phase output row: %w", err)
	}

	if taskID.Valid {
		output.TaskID = &taskID.String
	}
	if contentHash.Valid {
		output.ContentHash = contentHash.String
	}
	if artifactType.Valid {
		output.ArtifactType = artifactType.String
	}
	if source.Valid {
		output.Source = source.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		output.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		output.UpdatedAt = ts
	}

	return &output, nil
}
