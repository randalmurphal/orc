package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// QualityCheck defines a single quality check to run after phase completion.
type QualityCheck struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	UseShort  bool   `json:"use_short,omitempty"`
	Command   string `json:"command,omitempty"`
	OnFailure string `json:"on_failure,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// ParseQualityChecks parses a JSON string into a slice of QualityCheck.
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

// PhaseTemplate represents a reusable phase definition.
type PhaseTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	AgentID   string `json:"agent_id,omitempty"`
	SubAgents string `json:"sub_agents,omitempty"`

	PromptSource  string `json:"prompt_source"`
	PromptContent string `json:"prompt_content,omitempty"`
	PromptPath    string `json:"prompt_path,omitempty"`

	InputVariables   string `json:"input_variables,omitempty"`
	OutputSchema     string `json:"output_schema,omitempty"`
	ProducesArtifact bool   `json:"produces_artifact"`
	ArtifactType     string `json:"artifact_type,omitempty"`
	OutputVarName    string `json:"output_var_name,omitempty"`

	OutputType    string `json:"output_type,omitempty"`
	QualityChecks string `json:"quality_checks,omitempty"`

	ThinkingEnabled *bool  `json:"thinking_enabled,omitempty"`
	GateType        string `json:"gate_type"`
	Checkpoint      bool   `json:"checkpoint"`

	GateInputConfig  string `json:"gate_input_config,omitempty"`
	GateOutputConfig string `json:"gate_output_config,omitempty"`
	GateMode         string `json:"gate_mode,omitempty"`
	GateAgentID      string `json:"gate_agent_id,omitempty"`

	RetryFromPhase  string `json:"retry_from_phase,omitempty"`
	RetryPromptPath string `json:"retry_prompt_path,omitempty"`

	RuntimeConfig string `json:"runtime_config,omitempty"`
	Type          string `json:"type,omitempty"`
	Provider      string `json:"provider,omitempty"`

	IsBuiltin bool      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Workflow represents a composed execution plan.
type Workflow struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	DefaultModel     string    `json:"default_model,omitempty"`
	DefaultProvider  string    `json:"default_provider,omitempty"`
	DefaultThinking  bool      `json:"default_thinking"`
	CompletionAction string    `json:"completion_action,omitempty"`
	TargetBranch     string    `json:"target_branch,omitempty"`
	IsBuiltin        bool      `json:"is_builtin"`
	BasedOn          string    `json:"based_on,omitempty"`
	Triggers         string    `json:"triggers,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Phases []*WorkflowPhase `json:"phases,omitempty"`
}

// OutputTransformConfig defines how phase output is transformed between loop iterations.
type OutputTransformConfig struct {
	Type        string `json:"type"`
	SourceVar   string `json:"source_var"`
	TargetVar   string `json:"target_var"`
	ExtractPath string `json:"extract_path,omitempty"`
}

// Validate checks that the OutputTransformConfig is valid.
func (c *OutputTransformConfig) Validate() error {
	if c.Type == "" {
		return fmt.Errorf("output transform: type is required")
	}
	if c.SourceVar == "" {
		return fmt.Errorf("output transform: source_var is required")
	}
	if c.TargetVar == "" {
		return fmt.Errorf("output transform: target_var is required")
	}

	validTypes := map[string]bool{
		"format_findings": true,
		"json_extract":    true,
		"passthrough":     true,
	}
	if !validTypes[c.Type] {
		return fmt.Errorf("output transform: unknown type %q", c.Type)
	}
	if c.Type == "json_extract" && c.ExtractPath == "" {
		return fmt.Errorf("output transform: extract_path required for json_extract type")
	}
	return nil
}

// LoopConfig defines looping behavior for a workflow phase.
type LoopConfig struct {
	LoopToPhase     string                 `json:"loop_to_phase"`
	Condition       json.RawMessage        `json:"condition,omitempty"`
	MaxLoops        int                    `json:"max_loops,omitempty"`
	MaxIterations   int                    `json:"max_iterations,omitempty"`
	LoopTemplates   map[string]string      `json:"loop_templates,omitempty"`
	LoopSchemas     map[string]string      `json:"loop_schemas,omitempty"`
	OutputTransform *OutputTransformConfig `json:"output_transform,omitempty"`
}

// EffectiveMaxLoops returns the effective max loop count with precedence.
func (lc *LoopConfig) EffectiveMaxLoops() int {
	if lc.MaxLoops > 0 {
		return lc.MaxLoops
	}
	if lc.MaxIterations > 0 {
		return lc.MaxIterations
	}
	return 3
}

// IsLegacyCondition returns true if the condition is a JSON string.
func (lc *LoopConfig) IsLegacyCondition() bool {
	if len(lc.Condition) == 0 {
		return false
	}
	trimmed := bytes.TrimSpace(lc.Condition)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

// GetTemplateForIteration returns the template path for a given loop iteration.
func (lc *LoopConfig) GetTemplateForIteration(iteration int, baseTemplate string) string {
	if len(lc.LoopTemplates) == 0 {
		return baseTemplate
	}
	iterKey := fmt.Sprintf("%d", iteration)
	if tmpl, ok := lc.LoopTemplates[iterKey]; ok {
		return tmpl
	}
	if tmpl, ok := lc.LoopTemplates["default"]; ok {
		return tmpl
	}
	return baseTemplate
}

// GetSchemaForIteration returns the schema identifier for a given loop iteration.
func (lc *LoopConfig) GetSchemaForIteration(iteration int) string {
	if len(lc.LoopSchemas) == 0 {
		return ""
	}
	iterKey := fmt.Sprintf("%d", iteration)
	if schema, ok := lc.LoopSchemas[iterKey]; ok {
		return schema
	}
	if schema, ok := lc.LoopSchemas["default"]; ok {
		return schema
	}
	return ""
}

// ParseLoopConfig parses a JSON string into LoopConfig.
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

// WorkflowPhase links a phase template to a workflow.
type WorkflowPhase struct {
	ID              int    `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	PhaseTemplateID string `json:"phase_template_id"`
	Sequence        int    `json:"sequence"`
	DependsOn       string `json:"depends_on,omitempty"`

	AgentOverride     string `json:"agent_override,omitempty"`
	SubAgentsOverride string `json:"sub_agents_override,omitempty"`

	ModelOverride         string `json:"model_override,omitempty"`
	ProviderOverride      string `json:"provider_override,omitempty"`
	ThinkingOverride      *bool  `json:"thinking_override,omitempty"`
	GateTypeOverride      string `json:"gate_type_override,omitempty"`
	Condition             string `json:"condition,omitempty"`
	QualityChecksOverride string `json:"quality_checks_override,omitempty"`

	LoopConfig            string   `json:"loop_config,omitempty"`
	RuntimeConfigOverride string   `json:"runtime_config_override,omitempty"`
	TypeOverride          string   `json:"type_override,omitempty"`
	BeforeTriggers        string   `json:"before_triggers,omitempty"`
	PositionX             *float64 `json:"position_x,omitempty"`
	PositionY             *float64 `json:"position_y,omitempty"`
}

// WorkflowVariable defines a custom variable for a workflow.
type WorkflowVariable struct {
	ID              int    `json:"id"`
	WorkflowID      string `json:"workflow_id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	SourceType      string `json:"source_type"`
	SourceConfig    string `json:"source_config"`
	Required        bool   `json:"required"`
	DefaultValue    string `json:"default_value,omitempty"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds"`
	ScriptContent   string `json:"script_content,omitempty"`
	Extract         string `json:"extract,omitempty"`
}

// WorkflowRun represents an execution instance of a workflow.
type WorkflowRun struct {
	ID         string `json:"id"`
	WorkflowID string `json:"workflow_id"`

	ContextType string  `json:"context_type"`
	ContextData string  `json:"context_data,omitempty"`
	TaskID      *string `json:"task_id,omitempty"`

	Prompt       string `json:"prompt"`
	Instructions string `json:"instructions,omitempty"`

	Status       string     `json:"status"`
	CurrentPhase string     `json:"current_phase,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`

	VariablesSnapshot string `json:"variables_snapshot,omitempty"`

	TotalCostUSD      float64 `json:"total_cost_usd"`
	TotalInputTokens  int     `json:"total_input_tokens"`
	TotalOutputTokens int     `json:"total_output_tokens"`

	Error     string `json:"error,omitempty"`
	StartedBy string `json:"started_by,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkflowRunPhase tracks execution of a phase within a run.
type WorkflowRunPhase struct {
	ID              int        `json:"id"`
	WorkflowRunID   string     `json:"workflow_run_id"`
	PhaseTemplateID string     `json:"phase_template_id"`
	Status          string     `json:"status"`
	Iterations      int        `json:"iterations"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CommitSHA       string     `json:"commit_sha,omitempty"`
	InputTokens     int        `json:"input_tokens"`
	OutputTokens    int        `json:"output_tokens"`
	CostUSD         float64    `json:"cost_usd"`
	Content         string     `json:"content,omitempty"`
	Error           string     `json:"error,omitempty"`
	SessionID       string     `json:"session_id,omitempty"`
}

// WorkflowRunListOpts specifies filtering options for listing workflow runs.
type WorkflowRunListOpts struct {
	WorkflowID string
	TaskID     string
	Status     string
	Limit      int
	Offset     int
}
