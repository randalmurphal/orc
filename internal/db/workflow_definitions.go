package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SaveWorkflow creates or updates a workflow.
func (p *ProjectDB) SaveWorkflow(w *Workflow) error {
	basedOn := sqlNullString(w.BasedOn)

	_, err := p.Exec(`
		INSERT INTO workflows (id, name, description, default_model, default_provider, default_thinking, completion_action, target_branch, is_builtin, based_on, triggers, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			default_model = excluded.default_model,
			default_provider = excluded.default_provider,
			default_thinking = excluded.default_thinking,
			completion_action = excluded.completion_action,
			target_branch = excluded.target_branch,
			based_on = excluded.based_on,
			triggers = excluded.triggers,
			updated_at = excluded.updated_at
	`, w.ID, w.Name, w.Description, w.DefaultModel, w.DefaultProvider, w.DefaultThinking, w.CompletionAction,
		w.TargetBranch, w.IsBuiltin, basedOn, w.Triggers, w.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}
	return nil
}

// GetWorkflow retrieves a workflow by ID.
func (p *ProjectDB) GetWorkflow(id string) (*Workflow, error) {
	row := p.QueryRow(`
		SELECT id, name, description, default_model, default_provider, default_thinking, completion_action, target_branch, is_builtin, based_on, triggers, created_at, updated_at
		FROM workflows WHERE id = ?
	`, id)

	w, err := scanWorkflow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get workflow %s: %w", id, err)
	}
	return w, nil
}

// ListWorkflows returns all workflows.
func (p *ProjectDB) ListWorkflows() ([]*Workflow, error) {
	rows, err := p.Query(`
		SELECT id, name, description, default_model, default_provider, default_thinking, completion_action, target_branch, is_builtin, based_on, triggers, created_at, updated_at
		FROM workflows
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var workflows []*Workflow
	for rows.Next() {
		w, err := scanWorkflowRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow: %w", err)
		}
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}

// DeleteWorkflow removes a workflow and cascades to phases/variables/runs.
func (p *ProjectDB) DeleteWorkflow(id string) error {
	_, err := p.Exec("DELETE FROM workflows WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	return nil
}

// SaveWorkflowPhase creates or updates a workflow-phase link.
func (p *ProjectDB) SaveWorkflowPhase(wp *WorkflowPhase) error {
	thinkingOverride := sqlNullBool(wp.ThinkingOverride)
	posX := sqlNullFloat64(wp.PositionX)
	posY := sqlNullFloat64(wp.PositionY)
	agentOverride := sqlNullString(wp.AgentOverride)
	subAgentsOverride := sqlNullString(wp.SubAgentsOverride)

	res, err := p.Exec(`
		INSERT INTO workflow_phases (workflow_id, phase_template_id, sequence, depends_on,
			agent_override, sub_agents_override,
			model_override, provider_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, runtime_config_override, before_triggers, position_x, position_y,
			type_override)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, phase_template_id) DO UPDATE SET
			sequence = excluded.sequence,
			depends_on = excluded.depends_on,
			agent_override = excluded.agent_override,
			sub_agents_override = excluded.sub_agents_override,
			model_override = excluded.model_override,
			provider_override = excluded.provider_override,
			thinking_override = excluded.thinking_override,
			gate_type_override = excluded.gate_type_override,
			condition = excluded.condition,
			quality_checks_override = excluded.quality_checks_override,
			loop_config = excluded.loop_config,
			runtime_config_override = excluded.runtime_config_override,
			before_triggers = excluded.before_triggers,
			position_x = excluded.position_x,
			position_y = excluded.position_y,
			type_override = excluded.type_override
	`, wp.WorkflowID, wp.PhaseTemplateID, wp.Sequence, wp.DependsOn,
		agentOverride, subAgentsOverride,
		wp.ModelOverride, wp.ProviderOverride, thinkingOverride, wp.GateTypeOverride, wp.Condition,
		wp.QualityChecksOverride, wp.LoopConfig, wp.RuntimeConfigOverride, wp.BeforeTriggers, posX, posY,
		wp.TypeOverride)
	if err != nil {
		return fmt.Errorf("save workflow phase: %w", err)
	}

	if wp.ID == 0 {
		id, _ := res.LastInsertId()
		wp.ID = int(id)
	}
	return nil
}

// GetWorkflowPhases returns all phases for a workflow in sequence order.
func (p *ProjectDB) GetWorkflowPhases(workflowID string) ([]*WorkflowPhase, error) {
	rows, err := p.Query(`
		SELECT id, workflow_id, phase_template_id, sequence, depends_on,
			agent_override, sub_agents_override,
			model_override, COALESCE(provider_override, '') as provider_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, runtime_config_override, before_triggers, position_x, position_y,
			COALESCE(type_override, '') as type_override
		FROM workflow_phases
		WHERE workflow_id = ?
		ORDER BY sequence ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow phases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var phases []*WorkflowPhase
	for rows.Next() {
		wp, err := scanWorkflowPhaseRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow phase: %w", err)
		}
		phases = append(phases, wp)
	}
	return phases, rows.Err()
}

// DeleteWorkflowPhase removes a phase from a workflow.
func (p *ProjectDB) DeleteWorkflowPhase(workflowID, phaseTemplateID string) error {
	_, err := p.Exec("DELETE FROM workflow_phases WHERE workflow_id = ? AND phase_template_id = ?",
		workflowID, phaseTemplateID)
	if err != nil {
		return fmt.Errorf("delete workflow phase: %w", err)
	}
	return nil
}

// UpdateWorkflowPhasePositions bulk-updates position_x/position_y for phases in a workflow.
func (p *ProjectDB) UpdateWorkflowPhasePositions(workflowID string, positions map[string][2]float64) error {
	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		for phaseTemplateID, pos := range positions {
			if _, err := tx.Exec(`
				UPDATE workflow_phases SET position_x = ?, position_y = ?
				WHERE workflow_id = ? AND phase_template_id = ?
			`, pos[0], pos[1], workflowID, phaseTemplateID); err != nil {
				return fmt.Errorf("update position for phase %s: %w", phaseTemplateID, err)
			}
		}
		return nil
	})
}

// SaveWorkflowVariable creates or updates a workflow variable.
func (p *ProjectDB) SaveWorkflowVariable(wv *WorkflowVariable) error {
	res, err := p.Exec(`
		INSERT INTO workflow_variables (workflow_id, name, description, source_type, source_config, required, default_value, cache_ttl_seconds, script_content, extract)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, name) DO UPDATE SET
			description = excluded.description,
			source_type = excluded.source_type,
			source_config = excluded.source_config,
			required = excluded.required,
			default_value = excluded.default_value,
			cache_ttl_seconds = excluded.cache_ttl_seconds,
			script_content = excluded.script_content,
			extract = excluded.extract
	`, wv.WorkflowID, wv.Name, wv.Description, wv.SourceType, wv.SourceConfig,
		wv.Required, wv.DefaultValue, wv.CacheTTLSeconds, wv.ScriptContent, wv.Extract)
	if err != nil {
		return fmt.Errorf("save workflow variable: %w", err)
	}

	if wv.ID == 0 {
		id, _ := res.LastInsertId()
		wv.ID = int(id)
	}
	return nil
}

// GetWorkflowVariables returns all variables for a workflow.
func (p *ProjectDB) GetWorkflowVariables(workflowID string) ([]*WorkflowVariable, error) {
	rows, err := p.Query(`
		SELECT id, workflow_id, name, description, source_type, source_config, required, default_value, cache_ttl_seconds, script_content, extract
		FROM workflow_variables
		WHERE workflow_id = ?
		ORDER BY name ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow variables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var vars []*WorkflowVariable
	for rows.Next() {
		wv, err := scanWorkflowVariableRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow variable: %w", err)
		}
		vars = append(vars, wv)
	}
	return vars, rows.Err()
}

// DeleteWorkflowVariable removes a variable from a workflow.
func (p *ProjectDB) DeleteWorkflowVariable(workflowID, name string) error {
	_, err := p.Exec("DELETE FROM workflow_variables WHERE workflow_id = ? AND name = ?",
		workflowID, name)
	if err != nil {
		return fmt.Errorf("delete workflow variable: %w", err)
	}
	return nil
}
