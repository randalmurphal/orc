// Package variable provides a unified variable resolution system for orc workflows.
// It supports multiple variable sources: static values, environment variables,
// script output, API responses, phase outputs, and prompt fragments.
package variable

import (
	"encoding/json"
	"maps"
	"time"
)

// SourceType identifies the type of variable source.
type SourceType string

const (
	// SourceStatic is a literal fixed value.
	SourceStatic SourceType = "static"

	// SourceEnv reads from an environment variable.
	SourceEnv SourceType = "env"

	// SourceScript executes a script and uses stdout as value.
	SourceScript SourceType = "script"

	// SourceAPI makes an HTTP GET request and extracts data.
	SourceAPI SourceType = "api"

	// SourcePhaseOutput reads the artifact/output from a prior phase.
	SourcePhaseOutput SourceType = "phase_output"

	// SourcePromptFragment reads a reusable prompt snippet from a file.
	SourcePromptFragment SourceType = "prompt_fragment"
)

// Definition defines a workflow variable with its source configuration.
type Definition struct {
	// Name is the variable name (e.g., "JIRA_CONTEXT", "STYLE_GUIDE").
	// When used in templates, it becomes {{WORKFLOW_NAME}} or the appropriate prefix.
	Name string `json:"name"`

	// Description explains what this variable provides.
	Description string `json:"description,omitempty"`

	// SourceType determines how the value is resolved.
	SourceType SourceType `json:"source_type"`

	// SourceConfig is source-specific configuration as JSON.
	SourceConfig json.RawMessage `json:"source_config"`

	// Required indicates if resolution failure should stop execution.
	Required bool `json:"required"`

	// DefaultValue is used if resolution fails and Required is false.
	DefaultValue string `json:"default_value,omitempty"`

	// CacheTTL specifies how long to cache the resolved value.
	// 0 means no caching.
	CacheTTL time.Duration `json:"cache_ttl,omitempty"`
}

// StaticConfig configures a static (literal) value source.
type StaticConfig struct {
	Value string `json:"value"`
}

// EnvConfig configures an environment variable source.
type EnvConfig struct {
	// Var is the environment variable name to read.
	Var string `json:"var"`

	// Default is used if the environment variable is not set.
	Default string `json:"default,omitempty"`
}

// ScriptConfig configures a script execution source.
type ScriptConfig struct {
	// Path is the script path relative to .orc/scripts/ or absolute.
	Path string `json:"path"`

	// Args are arguments passed to the script.
	Args []string `json:"args,omitempty"`

	// TimeoutMS is the maximum execution time in milliseconds.
	// Default: 5000 (5 seconds).
	TimeoutMS int `json:"timeout_ms,omitempty"`

	// WorkDir is the working directory for the script.
	// If empty, uses the project root.
	WorkDir string `json:"work_dir,omitempty"`
}

// APIConfig configures an HTTP API source.
type APIConfig struct {
	// URL is the HTTP(S) endpoint to call.
	URL string `json:"url"`

	// Method is the HTTP method (default: GET).
	Method string `json:"method,omitempty"`

	// Headers are HTTP headers to include.
	Headers map[string]string `json:"headers,omitempty"`

	// JQFilter is a jq expression to extract data from JSON response.
	// If empty, the entire response body is used.
	JQFilter string `json:"jq_filter,omitempty"`

	// TimeoutMS is the request timeout in milliseconds.
	// Default: 10000 (10 seconds).
	TimeoutMS int `json:"timeout_ms,omitempty"`
}

// PhaseOutputConfig configures a phase output source.
type PhaseOutputConfig struct {
	// Phase is the phase ID to read from (e.g., "spec", "design").
	Phase string `json:"phase"`

	// Field specifies which output to use: "artifact" or "transcript".
	// Default: "artifact".
	Field string `json:"field,omitempty"`
}

// PromptFragmentConfig configures a prompt fragment source.
type PromptFragmentConfig struct {
	// Path is the fragment path relative to .orc/prompts/fragments/ or absolute.
	Path string `json:"path"`
}

// ResolvedVariable holds a resolved variable value with metadata.
type ResolvedVariable struct {
	// Name is the variable name.
	Name string

	// Value is the resolved string value.
	Value string

	// Source indicates where the value came from.
	Source SourceType

	// ResolvedAt is when the value was resolved.
	ResolvedAt time.Time

	// CachedUntil is when the cached value expires (zero if not cached).
	CachedUntil time.Time

	// Error is set if resolution failed.
	Error error
}

// IsExpired returns true if the cached value has expired.
func (rv *ResolvedVariable) IsExpired() bool {
	if rv.CachedUntil.IsZero() {
		return true // Not cached
	}
	return time.Now().After(rv.CachedUntil)
}

// ResolutionContext provides context for variable resolution.
// This is passed to the resolver to provide access to task, phase, and workflow data.
type ResolutionContext struct {
	// TaskID is the current task ID (if attached to a task).
	TaskID string

	// TaskTitle is the task title.
	TaskTitle string

	// TaskDescription is the task description.
	TaskDescription string

	// TaskCategory is the task category.
	TaskCategory string

	// TaskWeight is the task weight (trivial, small, medium, large).
	TaskWeight string

	// Phase is the current phase ID.
	Phase string

	// WorkflowID is the workflow being executed.
	WorkflowID string

	// WorkflowRunID is the current workflow run ID.
	WorkflowRunID string

	// Iteration is the current phase iteration.
	Iteration int

	// RetryContext contains context from a previous failed attempt.
	RetryContext string

	// WorkingDir is the current working directory (worktree or project root).
	WorkingDir string

	// ProjectRoot is the project root directory.
	ProjectRoot string

	// Prompt is the user-provided prompt for this run.
	Prompt string

	// Instructions are additional user instructions.
	Instructions string

	// TargetBranch is the branch to merge into.
	TargetBranch string

	// TaskBranch is the task's working branch.
	TaskBranch string

	// PriorOutputs contains artifacts from completed phases.
	// Key is phase ID, value is the artifact content.
	PriorOutputs map[string]string

	// Environment provides access to environment variables.
	// If nil, os.Getenv is used.
	Environment map[string]string

	// ConstitutionContent is the project constitution content.
	// Used to inject project-level principles into phase prompts.
	ConstitutionContent string

	// Initiative context (when task belongs to an initiative)
	InitiativeID        string
	InitiativeTitle     string
	InitiativeVision    string
	InitiativeDecisions string // Formatted decision list
	InitiativeTasks     string // Formatted task list for automation

	// Review context
	ReviewRound    int    // Current review round (1 or 2)
	ReviewFindings string // Previous round's findings (for round 2+)

	// Project detection context
	Language     string   // Primary language (go, typescript, python, etc.)
	HasFrontend  bool     // Whether project has a frontend
	HasTests     bool     // Whether project has existing tests
	TestCommand  string   // Command to run tests
	LintCommand  string   // Command to run linting
	BuildCommand string   // Command to build project
	Frameworks   []string // Detected frameworks

	// Testing configuration
	CoverageThreshold int // Minimum test coverage percentage (default: 85)

	// UI testing context
	RequiresUITesting bool   // Whether task requires UI testing
	ScreenshotDir     string // Directory for saving screenshots
	TestResults       string // Test results from previous test phase
	TDDTestPlan       string // Manual UI test plan for Playwright MCP

	// Automation context (for automation tasks like changelog generation)
	RecentCompletedTasks string // Formatted list of recently completed tasks
	RecentChangedFiles   string // List of files changed in recent tasks
	ChangelogContent     string // Current CHANGELOG.md content
	ClaudeMDContent      string // Current CLAUDE.md content

	// QA E2E testing context
	QAIteration      int    // Current QA iteration (1, 2, 3, ...)
	QAMaxIterations  int    // Maximum QA iterations before stopping
	QAFindings       string // Formatted QA findings from qa_e2e_test phase (survives ResolveAll)
	BeforeImages     string // Newline-separated paths to baseline images for visual comparison
	PreviousFindings string // Formatted findings from previous QA iteration (for verification)
}

// VariableSet is a map of variable name to resolved value.
type VariableSet map[string]string

// Merge combines another VariableSet into this one.
// Values from other override existing values.
func (vs VariableSet) Merge(other VariableSet) {
	maps.Copy(vs, other)
}

// ParseStaticConfig parses a StaticConfig from JSON.
func ParseStaticConfig(data json.RawMessage) (*StaticConfig, error) {
	var cfg StaticConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ParseEnvConfig parses an EnvConfig from JSON.
func ParseEnvConfig(data json.RawMessage) (*EnvConfig, error) {
	var cfg EnvConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ParseScriptConfig parses a ScriptConfig from JSON.
func ParseScriptConfig(data json.RawMessage) (*ScriptConfig, error) {
	var cfg ScriptConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ParseAPIConfig parses an APIConfig from JSON.
func ParseAPIConfig(data json.RawMessage) (*APIConfig, error) {
	var cfg APIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ParsePhaseOutputConfig parses a PhaseOutputConfig from JSON.
func ParsePhaseOutputConfig(data json.RawMessage) (*PhaseOutputConfig, error) {
	var cfg PhaseOutputConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ParsePromptFragmentConfig parses a PromptFragmentConfig from JSON.
func ParsePromptFragmentConfig(data json.RawMessage) (*PromptFragmentConfig, error) {
	var cfg PromptFragmentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
