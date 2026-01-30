package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// QualityCheck defines a single quality check to run after phase completion.
// These are configured at the phase template level and can be overridden per workflow.
type QualityCheck struct {
	Type      string `json:"type"`                 // "code" (uses project_commands) or "custom"
	Name      string `json:"name"`                 // For "code": 'tests', 'lint', 'build', 'typecheck'. For "custom": user-defined name
	Enabled   bool   `json:"enabled"`              // Whether this check is active
	UseShort  bool   `json:"use_short,omitempty"`  // For "code" type: use short_command if available
	Command   string `json:"command,omitempty"`    // For "custom" type or to override project command
	OnFailure string `json:"on_failure,omitempty"` // "block" (default), "warn", "skip"
	TimeoutMs int    `json:"timeout_ms,omitempty"` // 0 = use default (2 minutes)
}

// ParseQualityChecks parses a JSON string into a slice of QualityCheck.
// Returns nil for empty/null input.
func ParseQualityChecks(jsonStr string) ([]QualityCheck, error) {
	if jsonStr == "" || jsonStr == "null" {
		return nil, nil
	}
	var checks []QualityCheck
	if err := json.Unmarshal([]byte(jsonStr), &checks); err != nil {
		return nil, fmt.Errorf("parse quality checks: %w", err)
	}
	return checks, nil
}

// MarshalQualityChecks serializes quality checks to JSON string.
// Returns empty string for nil/empty slice.
func MarshalQualityChecks(checks []QualityCheck) (string, error) {
	if len(checks) == 0 {
		return "", nil
	}
	data, err := json.Marshal(checks)
	if err != nil {
		return "", fmt.Errorf("marshal quality checks: %w", err)
	}
	return string(data), nil
}

// PhaseTemplate represents a reusable phase definition.
// Agent (WHO runs it) + Prompt (WHAT to do).
type PhaseTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Agent configuration (WHO runs this phase)
	AgentID   string `json:"agent_id,omitempty"`   // References agents.id - the executor agent
	SubAgents string `json:"sub_agents,omitempty"` // JSON array of agent IDs to include as sub-agents

	// Prompt configuration (WHAT to do)
	PromptSource  string `json:"prompt_source"`  // 'embedded', 'db', 'file'
	PromptContent string `json:"prompt_content,omitempty"`
	PromptPath    string `json:"prompt_path,omitempty"`

	// Contract
	InputVariables   string `json:"input_variables,omitempty"` // JSON array
	OutputSchema     string `json:"output_schema,omitempty"`
	ProducesArtifact bool   `json:"produces_artifact"`
	ArtifactType     string `json:"artifact_type,omitempty"`
	OutputVarName    string `json:"output_var_name,omitempty"` // Variable name for output (e.g., 'SPEC_CONTENT')

	// Quality checks
	OutputType    string `json:"output_type,omitempty"`    // 'code', 'tests', 'document', 'data', 'research', 'none'
	QualityChecks string `json:"quality_checks,omitempty"` // JSON array of QualityCheck

	// Execution config
	MaxIterations   int   `json:"max_iterations"`
	ThinkingEnabled *bool `json:"thinking_enabled,omitempty"` // Phase-level concern (not agent-level)
	GateType        string `json:"gate_type"`
	Checkpoint      bool   `json:"checkpoint"`

	// Gate configuration (extended)
	GateInputConfig  string `json:"gate_input_config,omitempty"`  // JSON GateInputConfig
	GateOutputConfig string `json:"gate_output_config,omitempty"` // JSON GateOutputConfig
	GateMode         string `json:"gate_mode,omitempty"`          // 'gate' or 'reaction'
	GateAgentID      string `json:"gate_agent_id,omitempty"`      // References agents.id

	// Retry configuration
	RetryFromPhase  string `json:"retry_from_phase,omitempty"`
	RetryPromptPath string `json:"retry_prompt_path,omitempty"`

	// Metadata
	IsBuiltin bool      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Workflow represents a composed execution plan.
type Workflow struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description,omitempty"`
	WorkflowType    string    `json:"workflow_type"` // 'task', 'branch', 'standalone'
	DefaultModel    string    `json:"default_model,omitempty"`
	DefaultThinking bool      `json:"default_thinking"`
	IsBuiltin       bool      `json:"is_builtin"`
	BasedOn         string    `json:"based_on,omitempty"`
	Triggers        string    `json:"triggers,omitempty"` // JSON array of WorkflowTrigger
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Loaded relations (not stored directly)
	Phases []*WorkflowPhase `json:"phases,omitempty"`
}

// LoopConfig defines looping behavior for a workflow phase.
// When configured, the phase can trigger a loop back to an earlier phase
// based on output conditions (e.g., QA finds issues → fix → retest).
type LoopConfig struct {
	// Condition defines when to loop. Options:
	// - "has_findings": loop if phase output contains findings
	// - "not_empty": loop if phase output is not empty
	// - "status_needs_fix": loop if status field equals "needs_fix"
	Condition string `json:"condition"`

	// LoopToPhase is the phase to loop back to (must be earlier in sequence).
	LoopToPhase string `json:"loop_to_phase"`

	// MaxIterations is the maximum number of loop iterations (default: 3).
	// The executor tracks iterations and stops when limit is reached.
	MaxIterations int `json:"max_iterations,omitempty"`
}

// ParseLoopConfig parses a JSON string into LoopConfig.
// Returns nil for empty/null input.
func ParseLoopConfig(jsonStr string) (*LoopConfig, error) {
	if jsonStr == "" || jsonStr == "null" {
		return nil, nil
	}
	var cfg LoopConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("parse loop config: %w", err)
	}
	return &cfg, nil
}

// MarshalLoopConfig serializes LoopConfig to JSON string.
// Returns empty string for nil.
func MarshalLoopConfig(cfg *LoopConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal loop config: %w", err)
	}
	return string(data), nil
}

// WorkflowPhase links a phase template to a workflow.
type WorkflowPhase struct {
	ID              int    `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	PhaseTemplateID string `json:"phase_template_id"`
	Sequence        int    `json:"sequence"`
	DependsOn       string `json:"depends_on,omitempty"` // JSON array

	// Agent overrides (WHO runs this phase)
	AgentOverride     string `json:"agent_override,omitempty"`      // Override executor agent
	SubAgentsOverride string `json:"sub_agents_override,omitempty"` // Override sub-agents (JSON array)

	// Per-workflow overrides
	MaxIterationsOverride *int   `json:"max_iterations_override,omitempty"`
	ModelOverride         string `json:"model_override,omitempty"`  // Override agent's model for this workflow
	ThinkingOverride      *bool  `json:"thinking_override,omitempty"`
	GateTypeOverride      string `json:"gate_type_override,omitempty"`
	Condition             string `json:"condition,omitempty"`              // JSON - conditional execution
	QualityChecksOverride string `json:"quality_checks_override,omitempty"` // JSON array, NULL=use template, []=disable all

	// Loop configuration (JSON) - defines iterative loop behavior
	LoopConfig string `json:"loop_config,omitempty"`

	// Claude configuration override (JSON) - merged with agent's claude_config
	ClaudeConfigOverride string `json:"claude_config_override,omitempty"`

	// Before-phase triggers (JSON array of BeforePhaseTrigger)
	BeforeTriggers string `json:"before_triggers,omitempty"`

	// Visual editor position (nil = auto-layout via dagre)
	PositionX *float64 `json:"position_x,omitempty"`
	PositionY *float64 `json:"position_y,omitempty"`
}

// WorkflowVariable defines a custom variable for a workflow.
type WorkflowVariable struct {
	ID              int    `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	SourceType      string `json:"source_type"` // 'static', 'script', 'api', 'phase_output', 'env', 'prompt_fragment'
	SourceConfig    string `json:"source_config"` // JSON
	Required        bool   `json:"required"`
	DefaultValue    string `json:"default_value,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
	ScriptContent   string `json:"script_content,omitempty"`
	Extract         string `json:"extract,omitempty"` // gjson path for JSONPath extraction
}

// WorkflowRun represents an execution instance of a workflow.
type WorkflowRun struct {
	ID         string `json:"id"`
	WorkflowID string `json:"workflow_id"`

	// Context
	ContextType string  `json:"context_type"` // 'task', 'branch', 'pr', 'standalone', 'tag'
	ContextData string  `json:"context_data,omitempty"` // JSON
	TaskID      *string `json:"task_id,omitempty"`

	// User inputs
	Prompt       string `json:"prompt"`
	Instructions string `json:"instructions,omitempty"`

	// Status
	Status       string     `json:"status"` // 'pending', 'running', 'paused', 'completed', 'failed', 'cancelled'
	CurrentPhase string     `json:"current_phase,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`

	// Runtime
	VariablesSnapshot string `json:"variables_snapshot,omitempty"` // JSON

	// Metrics
	TotalCostUSD      float64 `json:"total_cost_usd"`
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`

	// Error
	Error string `json:"error,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkflowRunPhase tracks execution of a phase within a run.
type WorkflowRunPhase struct {
	ID              int    `json:"id"`
	WorkflowRunID   string `json:"workflow_run_id"`
	PhaseTemplateID string `json:"phase_template_id"`

	// Status
	Status     string `json:"status"` // 'pending', 'running', 'completed', 'failed', 'skipped'
	Iterations int    `json:"iterations"`

	// Timing
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Git
	CommitSHA string `json:"commit_sha,omitempty"`

	// Metrics
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`

	// Output
	Content string `json:"content,omitempty"`

	// Error
	Error string `json:"error,omitempty"`

	// Session
	SessionID string `json:"session_id,omitempty"`
}

// --------- PhaseTemplate CRUD ---------

// SavePhaseTemplate creates or updates a phase template.
func (p *ProjectDB) SavePhaseTemplate(pt *PhaseTemplate) error {
	thinkingEnabled := sqlNullBool(pt.ThinkingEnabled)
	// Use sqlNullString for FK columns to convert empty string to NULL
	agentID := sqlNullString(pt.AgentID)
	subAgents := sqlNullString(pt.SubAgents)
	gateAgentID := sqlNullString(pt.GateAgentID)

	_, err := p.Exec(`
		INSERT INTO phase_templates (id, name, description, agent_id, sub_agents,
			prompt_source, prompt_content, prompt_path,
			input_variables, output_schema, produces_artifact, artifact_type, output_var_name,
			output_type, quality_checks,
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			max_iterations = excluded.max_iterations,
			thinking_enabled = excluded.thinking_enabled,
			gate_type = excluded.gate_type,
			checkpoint = excluded.checkpoint,
			retry_from_phase = excluded.retry_from_phase,
			retry_prompt_path = excluded.retry_prompt_path,
			gate_input_config = excluded.gate_input_config,
			gate_output_config = excluded.gate_output_config,
			gate_mode = excluded.gate_mode,
			gate_agent_id = excluded.gate_agent_id,
			updated_at = excluded.updated_at
	`, pt.ID, pt.Name, pt.Description, agentID, subAgents,
		pt.PromptSource, pt.PromptContent, pt.PromptPath,
		pt.InputVariables, pt.OutputSchema, pt.ProducesArtifact, pt.ArtifactType, pt.OutputVarName,
		pt.OutputType, pt.QualityChecks,
		pt.MaxIterations, thinkingEnabled, pt.GateType, pt.Checkpoint,
		pt.RetryFromPhase, pt.RetryPromptPath, pt.IsBuiltin,
		pt.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339),
		pt.GateInputConfig, pt.GateOutputConfig, pt.GateMode, gateAgentID)
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
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id
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
			max_iterations, thinking_enabled, gate_type, checkpoint,
			retry_from_phase, retry_prompt_path, is_builtin, created_at, updated_at,
			gate_input_config, gate_output_config, gate_mode, gate_agent_id
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

// --------- Workflow CRUD ---------

// SaveWorkflow creates or updates a workflow.
func (p *ProjectDB) SaveWorkflow(w *Workflow) error {
	// Use sqlNullString for BasedOn to convert empty string to NULL
	// This is required because BasedOn has a foreign key constraint
	basedOn := sqlNullString(w.BasedOn)

	_, err := p.Exec(`
		INSERT INTO workflows (id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			workflow_type = excluded.workflow_type,
			default_model = excluded.default_model,
			default_thinking = excluded.default_thinking,
			based_on = excluded.based_on,
			triggers = excluded.triggers,
			updated_at = excluded.updated_at
	`, w.ID, w.Name, w.Description, w.WorkflowType, w.DefaultModel, w.DefaultThinking,
		w.IsBuiltin, basedOn, w.Triggers, w.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}
	return nil
}

// GetWorkflow retrieves a workflow by ID.
func (p *ProjectDB) GetWorkflow(id string) (*Workflow, error) {
	row := p.QueryRow(`
		SELECT id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at
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
		SELECT id, name, description, workflow_type, default_model, default_thinking, is_builtin, based_on, triggers, created_at, updated_at
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

// --------- WorkflowPhase CRUD ---------

// SaveWorkflowPhase creates or updates a workflow-phase link.
func (p *ProjectDB) SaveWorkflowPhase(wp *WorkflowPhase) error {
	thinkingOverride := sqlNullBool(wp.ThinkingOverride)
	maxIterOverride := sqlNullInt(wp.MaxIterationsOverride)
	posX := sqlNullFloat64(wp.PositionX)
	posY := sqlNullFloat64(wp.PositionY)
	// Use sqlNullString for FK columns to convert empty string to NULL
	agentOverride := sqlNullString(wp.AgentOverride)
	subAgentsOverride := sqlNullString(wp.SubAgentsOverride)

	res, err := p.Exec(`
		INSERT INTO workflow_phases (workflow_id, phase_template_id, sequence, depends_on,
			agent_override, sub_agents_override,
			max_iterations_override, model_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, claude_config_override, before_triggers, position_x, position_y)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workflow_id, phase_template_id) DO UPDATE SET
			sequence = excluded.sequence,
			depends_on = excluded.depends_on,
			agent_override = excluded.agent_override,
			sub_agents_override = excluded.sub_agents_override,
			max_iterations_override = excluded.max_iterations_override,
			model_override = excluded.model_override,
			thinking_override = excluded.thinking_override,
			gate_type_override = excluded.gate_type_override,
			condition = excluded.condition,
			quality_checks_override = excluded.quality_checks_override,
			loop_config = excluded.loop_config,
			claude_config_override = excluded.claude_config_override,
			before_triggers = excluded.before_triggers,
			position_x = excluded.position_x,
			position_y = excluded.position_y
	`, wp.WorkflowID, wp.PhaseTemplateID, wp.Sequence, wp.DependsOn,
		agentOverride, subAgentsOverride,
		maxIterOverride, wp.ModelOverride, thinkingOverride, wp.GateTypeOverride, wp.Condition,
		wp.QualityChecksOverride, wp.LoopConfig, wp.ClaudeConfigOverride, wp.BeforeTriggers, posX, posY)
	if err != nil {
		return fmt.Errorf("save workflow phase: %w", err)
	}

	// Get the inserted/updated ID
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
			max_iterations_override, model_override, thinking_override, gate_type_override, condition,
			quality_checks_override, loop_config, claude_config_override, before_triggers, position_x, position_y
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
// Positions are keyed by phase_template_id (the stable identifier used in the editor).
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

// --------- WorkflowVariable CRUD ---------

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

// --------- WorkflowRun CRUD ---------

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
			error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			updated_at = excluded.updated_at
	`, wr.ID, wr.WorkflowID, wr.ContextType, wr.ContextData, wr.TaskID,
		wr.Prompt, wr.Instructions, wr.Status, wr.CurrentPhase, startedAt, completedAt,
		wr.VariablesSnapshot, wr.TotalCostUSD, wr.TotalInputTokens, wr.TotalOutputTokens,
		wr.Error, wr.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339))
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
			error, created_at, updated_at
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

// WorkflowRunListOpts specifies filtering options for listing workflow runs.
type WorkflowRunListOpts struct {
	WorkflowID string
	TaskID     string
	Status     string
	Limit      int
	Offset     int
}

// ListWorkflowRuns returns workflow runs with optional filtering.
func (p *ProjectDB) ListWorkflowRuns(opts WorkflowRunListOpts) ([]*WorkflowRun, error) {
	query := `
		SELECT id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at
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

// GetNextWorkflowRunID generates the next run ID (RUN-001, RUN-002, etc.).
func (p *ProjectDB) GetNextWorkflowRunID() (string, error) {
	var maxID string
	err := p.QueryRow("SELECT COALESCE(MAX(id), 'RUN-000') FROM workflow_runs").Scan(&maxID)
	if err != nil {
		return "", fmt.Errorf("get max run id: %w", err)
	}

	// Parse the number from RUN-XXX
	var num int
	if _, err := fmt.Sscanf(maxID, "RUN-%d", &num); err != nil {
		num = 0
	}
	return fmt.Sprintf("RUN-%03d", num+1), nil
}

// --------- WorkflowRunPhase CRUD ---------

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
// This is a lightweight update for real-time progress tracking during execution.
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

// GetRunningWorkflowsByTask returns a map of task_id -> current workflow run info
// for all currently running workflow runs. Used to enrich task status display.
func (p *ProjectDB) GetRunningWorkflowsByTask() (map[string]*WorkflowRun, error) {
	rows, err := p.Query(`
		SELECT id, workflow_id, context_type, context_data, task_id,
			prompt, instructions, status, current_phase, started_at, completed_at,
			variables_snapshot, total_cost_usd, total_input_tokens, total_output_tokens,
			error, created_at, updated_at
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

// --------- Helper Functions ---------

func sqlNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func sqlNullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func sqlNullFloat64(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullFloat64ToPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
}

func nullBoolToPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

func nullIntToPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	i := int(ni.Int64)
	return &i
}

// sqlNullString converts a string to sql.NullString, treating empty strings as NULL.
// This is important for foreign key fields where empty string != NULL.
func sqlNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// --------- Scanners ---------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPhaseTemplate(row rowScanner) (*PhaseTemplate, error) {
	pt := &PhaseTemplate{}
	var createdAt, updatedAt string
	var thinkingEnabled sql.NullBool
	var description, agentID, subAgents, promptContent, promptPath, inputVars, outputSchema, artifactType, outputVarName sql.NullString
	var outputType, qualityChecks sql.NullString
	var retryFromPhase, retryPromptPath sql.NullString
	var gateInputConfig, gateOutputConfig, gateMode, gateAgentID sql.NullString

	err := row.Scan(
		&pt.ID, &pt.Name, &description, &agentID, &subAgents,
		&pt.PromptSource, &promptContent, &promptPath,
		&inputVars, &outputSchema, &pt.ProducesArtifact, &artifactType, &outputVarName,
		&outputType, &qualityChecks,
		&pt.MaxIterations, &thinkingEnabled, &pt.GateType, &pt.Checkpoint,
		&retryFromPhase, &retryPromptPath, &pt.IsBuiltin, &createdAt, &updatedAt,
		&gateInputConfig, &gateOutputConfig, &gateMode, &gateAgentID,
	)
	if err != nil {
		return nil, err
	}

	pt.Description = description.String
	pt.AgentID = agentID.String
	pt.SubAgents = subAgents.String
	pt.PromptContent = promptContent.String
	pt.PromptPath = promptPath.String
	pt.InputVariables = inputVars.String
	pt.OutputSchema = outputSchema.String
	pt.ArtifactType = artifactType.String
	pt.OutputVarName = outputVarName.String
	pt.OutputType = outputType.String
	pt.QualityChecks = qualityChecks.String
	pt.ThinkingEnabled = nullBoolToPtr(thinkingEnabled)
	pt.RetryFromPhase = retryFromPhase.String
	pt.RetryPromptPath = retryPromptPath.String
	pt.GateInputConfig = gateInputConfig.String
	pt.GateOutputConfig = gateOutputConfig.String
	pt.GateMode = gateMode.String
	pt.GateAgentID = gateAgentID.String
	pt.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	pt.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return pt, nil
}

func scanPhaseTemplateRow(rows *sql.Rows) (*PhaseTemplate, error) {
	return scanPhaseTemplate(rows)
}

func scanWorkflow(row rowScanner) (*Workflow, error) {
	w := &Workflow{}
	var createdAt, updatedAt string
	var description, defaultModel, basedOn, triggers sql.NullString

	err := row.Scan(
		&w.ID, &w.Name, &description, &w.WorkflowType, &defaultModel, &w.DefaultThinking,
		&w.IsBuiltin, &basedOn, &triggers, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	w.Description = description.String
	w.DefaultModel = defaultModel.String
	w.BasedOn = basedOn.String
	w.Triggers = triggers.String
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	w.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return w, nil
}

func scanWorkflowRow(rows *sql.Rows) (*Workflow, error) {
	return scanWorkflow(rows)
}

func scanWorkflowPhaseRow(rows *sql.Rows) (*WorkflowPhase, error) {
	wp := &WorkflowPhase{}
	var dependsOn, agentOverride, subAgentsOverride sql.NullString
	var modelOverride, gateTypeOverride, condition, qualityChecksOverride, loopConfig, claudeConfigOverride sql.NullString
	var beforeTriggers sql.NullString
	var maxIterOverride sql.NullInt64
	var thinkingOverride sql.NullBool
	var posX, posY sql.NullFloat64

	err := rows.Scan(
		&wp.ID, &wp.WorkflowID, &wp.PhaseTemplateID, &wp.Sequence, &dependsOn,
		&agentOverride, &subAgentsOverride,
		&maxIterOverride, &modelOverride, &thinkingOverride, &gateTypeOverride, &condition,
		&qualityChecksOverride, &loopConfig, &claudeConfigOverride, &beforeTriggers, &posX, &posY,
	)
	if err != nil {
		return nil, err
	}

	wp.DependsOn = dependsOn.String
	wp.AgentOverride = agentOverride.String
	wp.SubAgentsOverride = subAgentsOverride.String
	wp.MaxIterationsOverride = nullIntToPtr(maxIterOverride)
	wp.ModelOverride = modelOverride.String
	wp.ThinkingOverride = nullBoolToPtr(thinkingOverride)
	wp.GateTypeOverride = gateTypeOverride.String
	wp.Condition = condition.String
	wp.QualityChecksOverride = qualityChecksOverride.String
	wp.LoopConfig = loopConfig.String
	wp.ClaudeConfigOverride = claudeConfigOverride.String
	wp.BeforeTriggers = beforeTriggers.String
	wp.PositionX = nullFloat64ToPtr(posX)
	wp.PositionY = nullFloat64ToPtr(posY)

	return wp, nil
}

func scanWorkflowVariableRow(rows *sql.Rows) (*WorkflowVariable, error) {
	wv := &WorkflowVariable{}
	var description, defaultValue, scriptContent, extract sql.NullString

	err := rows.Scan(
		&wv.ID, &wv.WorkflowID, &wv.Name, &description, &wv.SourceType, &wv.SourceConfig,
		&wv.Required, &defaultValue, &wv.CacheTTLSeconds, &scriptContent, &extract,
	)
	if err != nil {
		return nil, err
	}

	wv.Description = description.String
	wv.DefaultValue = defaultValue.String
	wv.ScriptContent = scriptContent.String
	wv.Extract = extract.String

	return wv, nil
}

func scanWorkflowRun(row rowScanner) (*WorkflowRun, error) {
	wr := &WorkflowRun{}
	var createdAt, updatedAt string
	var startedAt, completedAt sql.NullString
	var taskID, instructions, currentPhase, variablesSnapshot, runError sql.NullString

	err := row.Scan(
		&wr.ID, &wr.WorkflowID, &wr.ContextType, &wr.ContextData, &taskID,
		&wr.Prompt, &instructions, &wr.Status, &currentPhase, &startedAt, &completedAt,
		&variablesSnapshot, &wr.TotalCostUSD, &wr.TotalInputTokens, &wr.TotalOutputTokens,
		&runError, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if taskID.Valid {
		wr.TaskID = &taskID.String
	}
	wr.Instructions = instructions.String
	wr.CurrentPhase = currentPhase.String
	wr.VariablesSnapshot = variablesSnapshot.String
	wr.Error = runError.String
	wr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	wr.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		wr.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		wr.CompletedAt = &t
	}

	return wr, nil
}

func scanWorkflowRunRow(rows *sql.Rows) (*WorkflowRun, error) {
	return scanWorkflowRun(rows)
}

func scanWorkflowRunPhaseRow(rows *sql.Rows) (*WorkflowRunPhase, error) {
	wrp := &WorkflowRunPhase{}
	var startedAt, completedAt sql.NullString
	var commitSHA, content, phaseError, sessionID sql.NullString

	err := rows.Scan(
		&wrp.ID, &wrp.WorkflowRunID, &wrp.PhaseTemplateID, &wrp.Status, &wrp.Iterations,
		&startedAt, &completedAt, &commitSHA, &wrp.InputTokens, &wrp.OutputTokens, &wrp.CostUSD,
		&content, &phaseError, &sessionID,
	)
	if err != nil {
		return nil, err
	}

	wrp.CommitSHA = commitSHA.String
	wrp.Content = content.String
	wrp.Error = phaseError.String
	wrp.SessionID = sessionID.String

	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		wrp.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		wrp.CompletedAt = &t
	}

	return wrp, nil
}
