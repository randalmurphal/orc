package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveWorkflowRun creates or updates a workflow run.
func (p *ProjectDB) SaveWorkflowRun(wr *WorkflowRun) error {
	var startedAt, completedAt *string
	if wr.StartedAt != nil {
		s := wr.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if wr.CompletedAt != nil {
		s := wr.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := p.Exec(`
		INSERT INTO workflow_runs (id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at, started_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			workflow_id = excluded.workflow_id,
			context_type = excluded.context_type,
			context_data = excluded.context_data,
			task_id = excluded.task_id,
			prompt = excluded.prompt,
			instructions = excluded.instructions,
			status = excluded.status,
			current_phase = excluded.current_phase,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			variables_snapshot = excluded.variables_snapshot,
			total_cost_usd = excluded.total_cost_usd,
			total_input_tokens = excluded.total_input_tokens,
			total_output_tokens = excluded.total_output_tokens,
			error = excluded.error,
			updated_at = excluded.updated_at,
			started_by = excluded.started_by
	`, wr.ID, wr.WorkflowID, wr.ContextType, wr.ContextData, wr.TaskID,
		wr.Prompt, wr.Instructions, wr.Status, wr.CurrentPhase, startedAt, completedAt,
		wr.VariablesSnapshot, wr.TotalCostUSD, wr.TotalInputTokens, wr.TotalOutputTokens,
		wr.Error, wr.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339), wr.StartedBy)
	if err != nil {
		return fmt.Errorf("save workflow run: %w", err)
	}
	return nil
}

// GetWorkflowRun retrieves a workflow run by ID.
func (p *ProjectDB) GetWorkflowRun(id string) (*WorkflowRun, error) {
	row := p.QueryRow(`
		SELECT id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at, started_by
		FROM workflow_runs WHERE id = ?
	`, id)

	wr, err := scanWorkflowRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow run %s: %w", id, err)
	}
	return wr, nil
}

// ListWorkflowRuns returns workflow runs with optional filtering.
func (p *ProjectDB) ListWorkflowRuns(opts WorkflowRunListOpts) ([]*WorkflowRun, error) {
	query := `
		SELECT id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at, started_by
		FROM workflow_runs
		WHERE 1=1
	`
	var args []any

	if opts.WorkflowID != "" {
		query += " AND workflow_id = ?"
		args = append(args, opts.WorkflowID)
	}
	if opts.TaskID != "" {
		query += " AND task_id = ?"
		args = append(args, opts.TaskID)
	}
	if opts.Status != "" {
		query += " AND status = ?"
		args = append(args, opts.Status)
	}

	query += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*WorkflowRun
	for rows.Next() {
		wr, err := scanWorkflowRunRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		runs = append(runs, wr)
	}
	return runs, rows.Err()
}

// DeleteWorkflowRun removes a workflow run and its phases.
func (p *ProjectDB) DeleteWorkflowRun(id string) error {
	_, err := p.Exec("DELETE FROM workflow_runs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete workflow run: %w", err)
	}
	return nil
}

// GetNextWorkflowRunID generates the next run ID.
func (p *ProjectDB) GetNextWorkflowRunID() (string, error) {
	num, err := p.NextSequence(context.Background(), SeqWorkflowRun)
	if err != nil {
		return "", fmt.Errorf("get next workflow run sequence: %w", err)
	}
	return fmt.Sprintf("RUN-%03d", num), nil
}

// SaveWorkflowRunPhase creates or updates a run phase.
func (p *ProjectDB) SaveWorkflowRunPhase(wrp *WorkflowRunPhase) error {
	var startedAt, completedAt *string
	if wrp.StartedAt != nil {
		s := wrp.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if wrp.CompletedAt != nil {
		s := wrp.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	res, err := p.Exec(`
		INSERT INTO workflow_run_phases (workflow_run_id, phase_template_id, status, iterations,
			started_at, completed_at, commit_sha, input_tokens, output_tokens, cost_usd,
			content, error, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_run_id, phase_template_id) DO UPDATE SET
			status = excluded.status,
			iterations = excluded.iterations,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			commit_sha = excluded.commit_sha,
			input_tokens = excluded.input_tokens,
			output_tokens = excluded.output_tokens,
			cost_usd = excluded.cost_usd,
			content = excluded.content,
			error = excluded.error,
			session_id = excluded.session_id
	`, wrp.WorkflowRunID, wrp.PhaseTemplateID, wrp.Status, wrp.Iterations,
		startedAt, completedAt, wrp.CommitSHA, wrp.InputTokens, wrp.OutputTokens, wrp.CostUSD,
		wrp.Content, wrp.Error, wrp.SessionID)
	if err != nil {
		return fmt.Errorf("save workflow run phase: %w", err)
	}

	if wrp.ID == 0 {
		id, _ := res.LastInsertId()
		wrp.ID = int(id)
	}
	return nil
}

// UpdatePhaseIterations updates only the iterations count for a running phase.
func (p *ProjectDB) UpdatePhaseIterations(runID, phaseID string, iterations int) error {
	_, err := p.Exec(`
		UPDATE workflow_run_phases
		SET iterations = ?
		WHERE workflow_run_id = ? AND phase_template_id = ?
	`, iterations, runID, phaseID)
	if err != nil {
		return fmt.Errorf("update phase iterations: %w", err)
	}
	return nil
}

// GetWorkflowRunPhases returns all phases for a workflow run.
func (p *ProjectDB) GetWorkflowRunPhases(runID string) ([]*WorkflowRunPhase, error) {
	rows, err := p.Query(`
		SELECT id, workflow_run_id, phase_template_id, status, iterations,
			started_at, completed_at, commit_sha, input_tokens, output_tokens, cost_usd,
			content, error, session_id
		FROM workflow_run_phases
		WHERE workflow_run_id = ?
		ORDER BY id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("get workflow run phases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phases []*WorkflowRunPhase
	for rows.Next() {
		wrp, err := scanWorkflowRunPhaseRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run phase: %w", err)
		}
		phases = append(phases, wrp)
	}
	return phases, rows.Err()
}

// GetRunningWorkflowsByTask returns a map of task_id -> current workflow run info.
func (p *ProjectDB) GetRunningWorkflowsByTask() (map[string]*WorkflowRun, error) {
	rows, err := p.Query(`
		SELECT id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at, started_by
		FROM workflow_runs
		WHERE status = 'running' AND task_id IS NOT NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("get running workflows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]*WorkflowRun)
	for rows.Next() {
		wr, err := scanWorkflowRunRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		if wr.TaskID != nil {
			result[*wr.TaskID] = wr
		}
	}
	return result, rows.Err()
}
