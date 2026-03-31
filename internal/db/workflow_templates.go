package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePhaseTemplate creates or updates a phase template.
func (p *ProjectDB) SavePhaseTemplate(pt *PhaseTemplate) error {
	thinkingEnabled := sqlNullBool(pt.ThinkingEnabled)
	agentID := sqlNullString(pt.AgentID)
	subAgents := sqlNullString(pt.SubAgents)
	gateAgentID := sqlNullString(pt.GateAgentID)
	phaseType := pt.Type
	if phaseType == "" {
		phaseType = "llm"
	}

	_, err := p.Exec(`
		INSERT INTO phase_templates (id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id,
			runtime_config, type, provider)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			gate_input_config = excluded.gate_input_config,
			gate_output_config = excluded.gate_output_config,
			gate_mode = excluded.gate_mode,
			gate_agent_id = excluded.gate_agent_id,
			runtime_config = excluded.runtime_config,
			type = excluded.type,
			provider = excluded.provider,
			updated_at = excluded.updated_at
	`, pt.ID, pt.Name, pt.Description, agentID, subAgents,
		pt.PromptSource, pt.PromptContent, pt.PromptPath,
		pt.InputVariables, pt.OutputSchema, pt.ProducesArtifact, pt.ArtifactType, pt.OutputVarName,
		pt.OutputType, pt.QualityChecks,
		thinkingEnabled, pt.GateType, pt.Checkpoint,
		pt.RetryFromPhase, pt.RetryPromptPath, pt.IsBuiltin,
		pt.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339),
		pt.GateInputConfig, pt.GateOutputConfig, pt.GateMode, gateAgentID,
		pt.RuntimeConfig, phaseType, pt.Provider)
	if err != nil {
		return fmt.Errorf("save phase template: %w", err)
	}
	return nil
}

// GetPhaseTemplate retrieves a phase template by ID.
func (p *ProjectDB) GetPhaseTemplate(id string) (*PhaseTemplate, error) {
	row := p.QueryRow(`
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

// ListPhaseTemplates returns all phase templates.
func (p *ProjectDB) ListPhaseTemplates() ([]*PhaseTemplate, error) {
	rows, err := p.Query(`
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

// DeletePhaseTemplate removes a phase template.
func (p *ProjectDB) DeletePhaseTemplate(id string) error {
	_, err := p.Exec("DELETE FROM phase_templates WHERE id = ? AND is_builtin = FALSE", id)
	if err != nil {
		return fmt.Errorf("delete phase template: %w", err)
	}
	return nil
}
