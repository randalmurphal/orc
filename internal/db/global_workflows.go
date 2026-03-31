package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePhaseTemplate creates or updates a phase template in global DB.
func (g *GlobalDB) SavePhaseTemplate(pt *PhaseTemplate) error {
	thinkingEnabled := sqlNullBool(pt.ThinkingEnabled)
	agentID := sqlNullString(pt.AgentID)
	subAgents := sqlNullString(pt.SubAgents)
	gateAgentID := sqlNullString(pt.GateAgentID)
	phaseType := pt.Type
	if phaseType == "" {
		phaseType = "llm"
	}

	_, err := g.Exec(`
		INSERT INTO phase_templates (id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, system_prompt, runtime_config,
			is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id, type, provider)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			agent_id = excluded.agent_id,
			sub_agents = excluded.sub_agents,
			prompt_source = excluded.prompt_source,
			prompt_content = excluded.prompt_content,
			prompt_path = excluded.prompt_path,
			input_variables = excluded.input_variables,
			output_schema = excluded.output_schema,
			produces_artifact = excluded.produces_artifact,
			artifact_type = excluded.artifact_type,
			output_var_name = excluded.output_var_name,
			output_type = excluded.output_type,
			quality_checks = excluded.quality_checks,
			thinking_enabled = excluded.thinking_enabled,
			gate_type = excluded.gate_type,
			checkpoint = excluded.checkpoint,
			retry_from_phase = excluded.retry_from_phase,
			retry_prompt_path = excluded.retry_prompt_path,
			system_prompt = excluded.system_prompt,
			runtime_config = excluded.runtime_config,
			gate_input_config = excluded.gate_input_config,
			gate_output_config = excluded.gate_output_config,
			gate_mode = excluded.gate_mode,
			gate_agent_id = excluded.gate_agent_id,
			type = excluded.type,
			provider = excluded.provider,
			updated_at = excluded.updated_at
	`, pt.ID, pt.Name, pt.Description, agentID, subAgents,
		pt.PromptSource, pt.PromptContent, pt.PromptPath,
		pt.InputVariables, pt.OutputSchema, pt.ProducesArtifact, pt.ArtifactType, pt.OutputVarName,
		pt.OutputType, pt.QualityChecks,
		thinkingEnabled, pt.GateType, pt.Checkpoint,
		pt.RetryFromPhase, pt.RetryPromptPath, "", pt.RuntimeConfig,
		pt.IsBuiltin, pt.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339),
		pt.GateInputConfig, pt.GateOutputConfig, pt.GateMode, gateAgentID, phaseType, pt.Provider)
	if err != nil {
		return fmt.Errorf("save phase template: %w", err)
	}
	return nil
}

// GetPhaseTemplate retrieves a phase template by ID from global DB.
func (g *GlobalDB) GetPhaseTemplate(id string) (*PhaseTemplate, error) {
	row := g.QueryRow(`
		SELECT id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id,
			COALESCE(runtime_config, '') as runtime_config,
			COALESCE(type, 'llm') as type,
			COALESCE(provider, '') as provider
		FROM phase_templates WHERE id = ?
	`, id)

	pt, err := scanPhaseTemplate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get phase template %s: %w", id, err)
	}
	return pt, nil
}

// ListPhaseTemplates returns all phase templates from global DB.
func (g *GlobalDB) ListPhaseTemplates() ([]*PhaseTemplate, error) {
	rows, err := g.Query(`
		SELECT id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id,
			COALESCE(runtime_config, '') as runtime_config,
			COALESCE(type, 'llm') as type,
			COALESCE(provider, '') as provider
		FROM phase_templates
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list phase templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var templates []*PhaseTemplate
	for rows.Next() {
		pt, err := scanPhaseTemplateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan phase template: %w", err)
		}
		templates = append(templates, pt)
	}
	return templates, rows.Err()
}

// DeletePhaseTemplate removes a non-builtin phase template from global DB.
func (g *GlobalDB) DeletePhaseTemplate(id string) error {
	_, err := g.Exec("DELETE FROM phase_templates WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete phase template: %w", err)
	}
	return nil
}

// SaveWorkflow creates or updates a workflow in global DB.
func (g *GlobalDB) SaveWorkflow(w *Workflow) error {
	basedOn := sqlNullString(w.BasedOn)

	_, err := g.Exec(`
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

// GetWorkflow retrieves a workflow by ID from global DB, including its phases.
func (g *GlobalDB) GetWorkflow(id string) (*Workflow, error) {
	row := g.QueryRow(`
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

	phases, err := g.GetWorkflowPhases(id)
	if err != nil {
		return nil, fmt.Errorf("get workflow %s phases: %w", id, err)
	}
	w.Phases = phases

	return w, nil
}

// ListWorkflows returns all workflows from global DB.
func (g *GlobalDB) ListWorkflows() ([]*Workflow, error) {
	rows, err := g.Query(`
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

// DeleteWorkflow removes a non-builtin workflow from global DB.
func (g *GlobalDB) DeleteWorkflow(id string) error {
	_, err := g.Exec("DELETE FROM workflows WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete workflow: %w", err)
	}
	return nil
}

// SaveWorkflowPhase creates or updates a workflow-phase link in global DB.
func (g *GlobalDB) SaveWorkflowPhase(wp *WorkflowPhase) error {
	thinkingOverride := sqlNullBool(wp.ThinkingOverride)
	posX := sqlNullFloat64(wp.PositionX)
	posY := sqlNullFloat64(wp.PositionY)
	agentOverride := sqlNullString(wp.AgentOverride)
	subAgentsOverride := sqlNullString(wp.SubAgentsOverride)

	res, err := g.Exec(`
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

// AddWorkflowPhase adds a new phase to a workflow in global DB.
func (g *GlobalDB) AddWorkflowPhase(wp *WorkflowPhase) error {
	return g.SaveWorkflowPhase(wp)
}

// GetWorkflowPhases returns all phases for a workflow in sequence order from global DB.
func (g *GlobalDB) GetWorkflowPhases(workflowID string) ([]*WorkflowPhase, error) {
	rows, err := g.Query(`
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

// DeleteWorkflowPhase removes a phase from a workflow in global DB.
func (g *GlobalDB) DeleteWorkflowPhase(workflowID, phaseTemplateID string) error {
	_, err := g.Exec("DELETE FROM workflow_phases WHERE workflow_id = ? AND phase_template_id = ?",
		workflowID, phaseTemplateID)
	if err != nil {
		return fmt.Errorf("delete workflow phase: %w", err)
	}
	return nil
}

// SaveWorkflowVariable creates or updates a workflow variable in global DB.
func (g *GlobalDB) SaveWorkflowVariable(wv *WorkflowVariable) error {
	res, err := g.Exec(`
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

// GetWorkflowVariables returns all variables for a workflow from global DB.
func (g *GlobalDB) GetWorkflowVariables(workflowID string) ([]*WorkflowVariable, error) {
	rows, err := g.Query(`
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

// DeleteWorkflowVariable removes a variable from a workflow in global DB.
func (g *GlobalDB) DeleteWorkflowVariable(workflowID, name string) error {
	_, err := g.Exec("DELETE FROM workflow_variables WHERE workflow_id = ? AND name = ?",
		workflowID, name)
	if err != nil {
		return fmt.Errorf("delete workflow variable: %w", err)
	}
	return nil
}
