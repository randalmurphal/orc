// Package workflow provides configurable workflow management for orc.
// Workflows are composed of reusable phase templates, with custom variables
// and execution tracking decoupled from tasks.
package workflow

import (
	"encoding/json"
	"time"
)

// Phase status constants for workflow run phases.
// These are string values stored in the database.
const (
	PhaseStatusPending   = "pending"
	PhaseStatusRunning   = "running"
	PhaseStatusCompleted = "completed"
	PhaseStatusFailed    = "failed"
	PhaseStatusSkipped   = "skipped"
)

// PromptSource defines where a phase's prompt content comes from.
type PromptSource string

const (
	PromptSourceEmbedded PromptSource = "embedded" // From templates/prompts/*.md
	PromptSourceDB       PromptSource = "db"       // Inline in database
	PromptSourceFile     PromptSource = "file"     // From .orc/prompts/*.md
)

// GateType defines the type of approval gate for a phase.
type GateType string

const (
	GateAuto  GateType = "auto"  // AI auto-approves
	GateHuman GateType = "human" // Requires human approval
	GateSkip  GateType = "skip"  // No gate, always continues
	GateAI    GateType = "ai"    // AI gate evaluation
)

// GateMode defines whether a gate blocks progression or fires asynchronously.
type GateMode string

const (
	GateModeGate     GateMode = "gate"     // Synchronous, can block progression
	GateModeReaction GateMode = "reaction"  // Asynchronous, fire-and-forget
)

// GateAction defines the action to take on gate approval/rejection.
type GateAction string

const (
	GateActionContinue  GateAction = "continue"   // Continue to next phase
	GateActionRetry     GateAction = "retry"       // Retry from specified phase
	GateActionFail      GateAction = "fail"        // Fail the task
	GateActionSkipPhase GateAction = "skip_phase"  // Skip the next phase
	GateActionRunScript GateAction = "run_script"  // Run a script
)

// WorkflowTriggerEvent defines lifecycle event types for workflow-level triggers.
type WorkflowTriggerEvent string

const (
	WorkflowTriggerEventOnTaskCreated        WorkflowTriggerEvent = "on_task_created"
	WorkflowTriggerEventOnTaskCompleted      WorkflowTriggerEvent = "on_task_completed"
	WorkflowTriggerEventOnTaskFailed         WorkflowTriggerEvent = "on_task_failed"
	WorkflowTriggerEventOnInitiativePlanned  WorkflowTriggerEvent = "on_initiative_planned"
)

// GateInputConfig defines what context the gate evaluator receives.
type GateInputConfig struct {
	IncludePhaseOutput []string `json:"include_phase_output,omitempty" yaml:"include_phase_output,omitempty"`
	IncludeTask        bool     `json:"include_task,omitempty" yaml:"include_task,omitempty"`
	ExtraVars          []string `json:"extra_vars,omitempty" yaml:"extra_vars,omitempty"`
}

// GateOutputConfig defines what happens with gate evaluation results.
type GateOutputConfig struct {
	VariableName string     `json:"variable_name,omitempty" yaml:"variable_name,omitempty"`
	OnApproved   GateAction `json:"on_approved,omitempty" yaml:"on_approved,omitempty"`
	OnRejected   GateAction `json:"on_rejected,omitempty" yaml:"on_rejected,omitempty"`
	RetryFrom    string     `json:"retry_from,omitempty" yaml:"retry_from,omitempty"`
	Script       string     `json:"script,omitempty" yaml:"script,omitempty"`
}

// BeforePhaseTrigger defines a trigger that runs before a phase starts.
type BeforePhaseTrigger struct {
	AgentID      string           `json:"agent_id" yaml:"agent_id"`
	InputConfig  *GateInputConfig  `json:"input_config,omitempty" yaml:"input_config,omitempty"`
	OutputConfig *GateOutputConfig `json:"output_config,omitempty" yaml:"output_config,omitempty"`
	Mode         GateMode         `json:"mode,omitempty" yaml:"mode,omitempty"`
}

// WorkflowTrigger defines a workflow-level lifecycle trigger.
type WorkflowTrigger struct {
	Event        WorkflowTriggerEvent `json:"event" yaml:"event"`
	AgentID      string               `json:"agent_id" yaml:"agent_id"`
	InputConfig  *GateInputConfig     `json:"input_config,omitempty" yaml:"input_config,omitempty"`
	OutputConfig *GateOutputConfig    `json:"output_config,omitempty" yaml:"output_config,omitempty"`
	Mode         GateMode             `json:"mode,omitempty" yaml:"mode,omitempty"`
	Enabled      bool                 `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// RunStatus represents the execution state of a workflow run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusPaused    RunStatus = "paused"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// ContextType defines what a workflow run operates on.
type ContextType string

const (
	ContextTypeTask       ContextType = "task"       // Attached to a task (default)
	ContextTypeBranch     ContextType = "branch"     // Works on existing branch
	ContextTypePR         ContextType = "pr"         // Works on PR branch
	ContextTypeStandalone ContextType = "standalone" // No git context
	ContextTypeTag        ContextType = "tag"        // Works on tag checkout
)

// VariableSourceType defines where a variable's value comes from.
type VariableSourceType string

const (
	SourceTypeStatic        VariableSourceType = "static"         // Fixed value
	SourceTypeEnv           VariableSourceType = "env"            // Environment variable
	SourceTypeScript        VariableSourceType = "script"         // Script output
	SourceTypeAPI           VariableSourceType = "api"            // HTTP GET response
	SourceTypePhaseOutput   VariableSourceType = "phase_output"   // Prior phase artifact
	SourceTypePromptFragment VariableSourceType = "prompt_fragment" // Reusable prompt snippet
)

// WorkflowType defines the primary use case of a workflow.
type WorkflowType string

const (
	WorkflowTypeTask       WorkflowType = "task"       // Creates/attaches to task
	WorkflowTypeBranch     WorkflowType = "branch"     // Works on branches
	WorkflowTypeStandalone WorkflowType = "standalone" // No git context
)

// PhaseTemplate is a reusable phase definition (lego block).
// Agent (WHO runs it) + Prompt (WHAT to do).
// Stored in phase_templates table.
type PhaseTemplate struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description,omitempty" db:"description"`

	// Agent configuration (WHO runs this phase)
	AgentID   string `json:"agent_id,omitempty" db:"agent_id"`     // References agents.id - the executor agent
	SubAgents string `json:"sub_agents,omitempty" db:"sub_agents"` // JSON array of agent IDs

	// Prompt configuration (WHAT to do)
	PromptSource  PromptSource `json:"prompt_source" db:"prompt_source"`
	PromptContent string       `json:"prompt_content,omitempty" db:"prompt_content"`
	PromptPath    string       `json:"prompt_path,omitempty" db:"prompt_path"`

	// Contract
	InputVariables   []string `json:"input_variables,omitempty"`   // Parsed from JSON
	OutputSchema     string   `json:"output_schema,omitempty" db:"output_schema"`
	ProducesArtifact bool     `json:"produces_artifact" db:"produces_artifact"`
	ArtifactType     string   `json:"artifact_type,omitempty" db:"artifact_type"`

	// Execution config
	MaxIterations   int      `json:"max_iterations" db:"max_iterations"`
	ThinkingEnabled *bool    `json:"thinking_enabled,omitempty" db:"thinking_enabled"` // Phase-level concern
	GateType        GateType `json:"gate_type" db:"gate_type"`
	Checkpoint      bool     `json:"checkpoint" db:"checkpoint"`

	// Gate configuration (extended)
	GateMode        GateMode         `json:"gate_mode,omitempty" db:"gate_mode"`
	GateAgentID     string           `json:"gate_agent_id,omitempty" db:"gate_agent_id"`
	GateInputConfig  *GateInputConfig  `json:"gate_input_config,omitempty"`
	GateOutputConfig *GateOutputConfig `json:"gate_output_config,omitempty"`

	// Retry configuration
	RetryFromPhase  string `json:"retry_from_phase,omitempty" db:"retry_from_phase"`
	RetryPromptPath string `json:"retry_prompt_path,omitempty" db:"retry_prompt_path"`

	// Metadata
	IsBuiltin bool      `json:"is_builtin" db:"is_builtin"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Workflow composes phases into an execution plan.
// Stored in workflows table.
type Workflow struct {
	ID              string       `json:"id" db:"id"`
	Name            string       `json:"name" db:"name"`
	Description     string       `json:"description,omitempty" db:"description"`
	WorkflowType    WorkflowType `json:"workflow_type" db:"workflow_type"`
	DefaultModel    string       `json:"default_model,omitempty" db:"default_model"`
	DefaultThinking bool         `json:"default_thinking" db:"default_thinking"`
	IsBuiltin       bool         `json:"is_builtin" db:"is_builtin"`
	BasedOn         string       `json:"based_on,omitempty" db:"based_on"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`

	// Workflow-level lifecycle triggers
	Triggers []WorkflowTrigger `json:"triggers,omitempty"`

	// Loaded relations (not stored directly)
	Phases    []WorkflowPhase    `json:"phases,omitempty"`
	Variables []WorkflowVariable `json:"variables,omitempty"`
}

// WorkflowPhase links a phase template to a workflow with order and overrides.
// Stored in workflow_phases table.
type WorkflowPhase struct {
	ID              int    `json:"id" db:"id"`
	WorkflowID      string `json:"workflow_id" db:"workflow_id"`
	PhaseTemplateID string `json:"phase_template_id" db:"phase_template_id"`
	Sequence        int    `json:"sequence" db:"sequence"`
	DependsOn       []string `json:"depends_on,omitempty"` // Parsed from JSON

	// Per-workflow overrides (nil = use phase template defaults)
	MaxIterationsOverride *int     `json:"max_iterations_override,omitempty" db:"max_iterations_override"`
	ModelOverride         string   `json:"model_override,omitempty" db:"model_override"`
	ThinkingOverride      *bool    `json:"thinking_override,omitempty" db:"thinking_override"`
	GateTypeOverride      GateType `json:"gate_type_override,omitempty" db:"gate_type_override"`
	Condition             string   `json:"condition,omitempty" db:"condition"` // JSON skip conditions

	// Claude CLI configuration override (JSON)
	// Merged with PhaseTemplate.ClaudeConfig, with this taking precedence
	ClaudeConfigOverride string `json:"claude_config_override,omitempty" db:"claude_config_override"`

	// Before-phase triggers
	BeforeTriggers []BeforePhaseTrigger `json:"before_triggers,omitempty"`

	// Visual editor position (nil = auto-layout via dagre)
	PositionX *float64 `json:"position_x,omitempty" db:"position_x"`
	PositionY *float64 `json:"position_y,omitempty" db:"position_y"`

	// Loaded relation (not stored directly)
	Template *PhaseTemplate `json:"template,omitempty"`
}

// WorkflowVariable defines a custom variable for a workflow.
// Stored in workflow_variables table.
type WorkflowVariable struct {
	ID          int                `json:"id" db:"id"`
	WorkflowID  string             `json:"workflow_id" db:"workflow_id"`
	Name        string             `json:"name" db:"name"`
	Description string             `json:"description,omitempty" db:"description"`
	SourceType  VariableSourceType `json:"source_type" db:"source_type"`
	SourceConfig string            `json:"source_config" db:"source_config"` // JSON config

	Required        bool   `json:"required" db:"required"`
	DefaultValue    string `json:"default_value,omitempty" db:"default_value"`
	CacheTTLSeconds int    `json:"cache_ttl_seconds" db:"cache_ttl_seconds"`

	// For script sources, store content for cross-machine sync
	ScriptContent string `json:"script_content,omitempty" db:"script_content"`
}

// ContextData holds context-specific fields for a workflow run.
type ContextData struct {
	// Task context
	TaskID string `json:"task_id,omitempty"`

	// Branch context
	Branch       string `json:"branch,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`

	// PR context
	PRNumber int    `json:"pr_number,omitempty"`
	PRBranch string `json:"pr_branch,omitempty"`

	// Tag context
	Tag string `json:"tag,omitempty"`

	// Worktree path (set at runtime)
	WorktreePath string `json:"worktree_path,omitempty"`
}

// WorkflowRun is an execution instance of a workflow.
// Universal anchor for tracking execution, replaces task-centric approach.
// Stored in workflow_runs table.
type WorkflowRun struct {
	ID         string `json:"id" db:"id"`
	WorkflowID string `json:"workflow_id" db:"workflow_id"`

	// Context
	ContextType ContextType `json:"context_type" db:"context_type"`
	ContextData ContextData `json:"context_data"` // Parsed from JSON
	TaskID      *string     `json:"task_id,omitempty" db:"task_id"`

	// User inputs
	Prompt       string `json:"prompt" db:"prompt"`
	Instructions string `json:"instructions,omitempty" db:"instructions"`

	// Status
	Status       RunStatus `json:"status" db:"status"`
	CurrentPhase string    `json:"current_phase,omitempty" db:"current_phase"`
	StartedAt    *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Runtime data
	VariablesSnapshot map[string]string `json:"variables_snapshot,omitempty"` // Parsed from JSON

	// Metrics
	TotalCostUSD       float64 `json:"total_cost_usd" db:"total_cost_usd"`
	TotalInputTokens   int     `json:"total_input_tokens" db:"total_input_tokens"`
	TotalOutputTokens  int     `json:"total_output_tokens" db:"total_output_tokens"`

	// Error tracking
	Error string `json:"error,omitempty" db:"error"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Loaded relations (not stored directly)
	Workflow *Workflow          `json:"workflow,omitempty"`
	Phases   []WorkflowRunPhase `json:"phases,omitempty"`
}

// WorkflowRunPhase tracks execution of a single phase within a run.
// Stored in workflow_run_phases table.
type WorkflowRunPhase struct {
	ID              int    `json:"id" db:"id"`
	WorkflowRunID   string `json:"workflow_run_id" db:"workflow_run_id"`
	PhaseTemplateID string `json:"phase_template_id" db:"phase_template_id"`

	// Status
	Status     string `json:"status" db:"status"`
	Iterations int    `json:"iterations" db:"iterations"`

	// Timing
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Git tracking
	CommitSHA string `json:"commit_sha,omitempty" db:"commit_sha"`

	// Metrics
	InputTokens  int     `json:"input_tokens" db:"input_tokens"`
	OutputTokens int     `json:"output_tokens" db:"output_tokens"`
	CostUSD      float64 `json:"cost_usd" db:"cost_usd"`

	// Output
	Content string `json:"content,omitempty" db:"content"`

	// Error tracking
	Error string `json:"error,omitempty" db:"error"`

	// Claude session link
	SessionID string `json:"session_id,omitempty" db:"session_id"`

	// Loaded relation (not stored directly)
	Template *PhaseTemplate `json:"template,omitempty"`
}

// --------- Source Config Types ---------

// StaticSourceConfig for static variable values.
type StaticSourceConfig struct {
	Value string `json:"value"`
}

// EnvSourceConfig for environment variable sources.
type EnvSourceConfig struct {
	Var     string `json:"var"`
	Default string `json:"default,omitempty"`
}

// ScriptSourceConfig for script-based variable sources.
type ScriptSourceConfig struct {
	Path      string   `json:"path"`       // Path relative to .orc/scripts/
	Args      []string `json:"args,omitempty"`
	TimeoutMs int      `json:"timeout_ms,omitempty"` // Default 5000
}

// APISourceConfig for HTTP-based variable sources.
type APISourceConfig struct {
	URL       string            `json:"url"`
	Method    string            `json:"method,omitempty"` // Default GET
	Headers   map[string]string `json:"headers,omitempty"`
	JQFilter  string            `json:"jq_filter,omitempty"` // jq expression to extract value
	TimeoutMs int               `json:"timeout_ms,omitempty"`
}

// PhaseOutputSourceConfig for variables from prior phase outputs.
type PhaseOutputSourceConfig struct {
	Phase string `json:"phase"` // Phase ID
	Field string `json:"field"` // 'artifact', 'commit_sha', etc.
}

// PromptFragmentSourceConfig for reusable prompt snippets.
type PromptFragmentSourceConfig struct {
	Path string `json:"path"` // Path relative to .orc/prompts/fragments/
}

// --------- Helper Methods ---------

// MarshalContextData converts ContextData to JSON for DB storage.
func (cd ContextData) MarshalJSON() ([]byte, error) {
	type Alias ContextData
	return json.Marshal(Alias(cd))
}

// ParseInputVariables parses the JSON array of input variable names.
func (pt *PhaseTemplate) ParseInputVariables(raw string) error {
	if raw == "" {
		pt.InputVariables = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), &pt.InputVariables)
}

// InputVariablesJSON returns input variables as JSON string.
func (pt *PhaseTemplate) InputVariablesJSON() string {
	if len(pt.InputVariables) == 0 {
		return ""
	}
	b, _ := json.Marshal(pt.InputVariables)
	return string(b)
}

// ParseDependsOn parses the JSON array of phase dependencies.
func (wp *WorkflowPhase) ParseDependsOn(raw string) error {
	if raw == "" {
		wp.DependsOn = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), &wp.DependsOn)
}

// DependsOnJSON returns depends_on as JSON string.
func (wp *WorkflowPhase) DependsOnJSON() string {
	if len(wp.DependsOn) == 0 {
		return ""
	}
	b, _ := json.Marshal(wp.DependsOn)
	return string(b)
}

// ParseContextData parses the JSON context data.
func (wr *WorkflowRun) ParseContextData(raw string) error {
	if raw == "" {
		wr.ContextData = ContextData{}
		return nil
	}
	return json.Unmarshal([]byte(raw), &wr.ContextData)
}

// ContextDataJSON returns context data as JSON string.
func (wr *WorkflowRun) ContextDataJSON() string {
	b, _ := json.Marshal(wr.ContextData)
	return string(b)
}

// ParseVariablesSnapshot parses the JSON variables snapshot.
func (wr *WorkflowRun) ParseVariablesSnapshot(raw string) error {
	if raw == "" {
		wr.VariablesSnapshot = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), &wr.VariablesSnapshot)
}

// VariablesSnapshotJSON returns variables snapshot as JSON string.
func (wr *WorkflowRun) VariablesSnapshotJSON() string {
	if len(wr.VariablesSnapshot) == 0 {
		return ""
	}
	b, _ := json.Marshal(wr.VariablesSnapshot)
	return string(b)
}

// GetPhaseByID returns a phase from the run by template ID.
func (wr *WorkflowRun) GetPhaseByID(templateID string) *WorkflowRunPhase {
	for i := range wr.Phases {
		if wr.Phases[i].PhaseTemplateID == templateID {
			return &wr.Phases[i]
		}
	}
	return nil
}

// CurrentRunPhase returns the currently running or next pending phase.
func (wr *WorkflowRun) CurrentRunPhase() *WorkflowRunPhase {
	// First look for running phase
	for i := range wr.Phases {
		if wr.Phases[i].Status == PhaseStatusRunning {
			return &wr.Phases[i]
		}
	}
	// Then look for first pending phase
	for i := range wr.Phases {
		if wr.Phases[i].Status == PhaseStatusPending {
			return &wr.Phases[i]
		}
	}
	return nil
}

// IsComplete returns true if all phases are completed or skipped.
func (wr *WorkflowRun) IsComplete() bool {
	for _, phase := range wr.Phases {
		if phase.Status != PhaseStatusCompleted && phase.Status != PhaseStatusSkipped {
			return false
		}
	}
	return true
}

// GetEffectiveModel returns the model to use for a phase, considering overrides.
// Note: Full agent-based model resolution is handled in the executor.
// This method provides a fallback for workflow-level defaults.
func (wp *WorkflowPhase) GetEffectiveModel(workflow *Workflow) string {
	if wp.ModelOverride != "" {
		return wp.ModelOverride
	}
	// Agent model is resolved in executor.resolveExecutorAgent()
	return workflow.DefaultModel
}

// GetEffectiveMaxIterations returns max iterations considering overrides.
func (wp *WorkflowPhase) GetEffectiveMaxIterations() int {
	if wp.MaxIterationsOverride != nil {
		return *wp.MaxIterationsOverride
	}
	if wp.Template != nil {
		return wp.Template.MaxIterations
	}
	return 20 // Default
}

// GetEffectiveGateType returns gate type considering overrides.
func (wp *WorkflowPhase) GetEffectiveGateType() GateType {
	if wp.GateTypeOverride != "" {
		return wp.GateTypeOverride
	}
	if wp.Template != nil {
		return wp.Template.GateType
	}
	return GateAuto
}
