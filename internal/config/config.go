// Package config provides configuration management for orc.
package config

import (
	"time"
)

const (
	// ConfigFileName is the default config file name
	ConfigFileName = "config.yaml"
	// OrcDir is the orc configuration directory
	OrcDir = ".orc"
)

// Config represents the orc configuration.
type Config struct {
	// Version is the config file version
	Version int `yaml:"version"`

	// Automation profile (auto, fast, safe, strict)
	Profile AutomationProfile `yaml:"profile"`

	// Gate configuration
	Gates GateConfig `yaml:"gates"`

	// Retry configuration for cross-phase retry
	Retry RetryConfig `yaml:"retry"`

	// Worktree isolation settings
	Worktree WorktreeConfig `yaml:"worktree"`

	// Completion settings (merge/PR after task completes)
	Completion CompletionConfig `yaml:"completion"`

	// Execution strategy settings
	Execution ExecutionConfig `yaml:"execution"`

	// Budget settings for cost tracking
	Budget BudgetConfig `yaml:"budget"`

	// Token pool settings for automatic account switching
	Pool PoolConfig `yaml:"pool"`

	// Server settings (for team mode)
	Server ServerConfig `yaml:"server"`

	// Team mode settings
	Team TeamConfig `yaml:"team"`

	// Identity settings for multi-user coordination
	Identity IdentityConfig `yaml:"identity"`

	// User identity for attribution (required for postgres mode)
	User UserConfig `yaml:"user"`

	// Task ID generation settings
	TaskID TaskIDConfig `yaml:"task_id"`

	// Testing configuration
	Testing TestingConfig `yaml:"testing"`

	// Validation configuration for Haiku validation and backpressure
	Validation ValidationConfig `yaml:"validation"`

	// ErrorPatterns describes language-specific error handling idioms.
	// Auto-detected during init, user-editable. Injected into agent prompts as {{ERROR_PATTERNS}}.
	ErrorPatterns string `yaml:"error_patterns,omitempty"`

	// Documentation configuration
	Documentation DocumentationConfig `yaml:"documentation"`

	// Timeouts configuration
	Timeouts TimeoutsConfig `yaml:"timeouts"`

	// QA session configuration
	QA QAConfig `yaml:"qa"`

	// Review configuration
	Review ReviewConfig `yaml:"review"`

	// Plan/spec configuration
	Plan PlanConfig `yaml:"plan"`

	// Quality policy configuration
	QualityPolicy QualityPolicyConfig `yaml:"quality_policy"`

	// Weights configuration - maps task weights to workflow IDs
	Weights WeightsConfig `yaml:"weights"`

	// Workflow defaults - maps task categories to workflow IDs
	WorkflowDefaults WorkflowDefaults `yaml:"workflow_defaults"`

	// Artifact skip configuration
	ArtifactSkip ArtifactSkipConfig `yaml:"artifact_skip"`

	// Sub-task queue configuration
	Subtasks SubtasksConfig `yaml:"subtasks"`

	// Tasks configuration
	Tasks TasksConfig `yaml:"tasks"`

	// Diagnostics configuration
	Diagnostics DiagnosticsConfig `yaml:"diagnostics"`

	// Developer settings for personal branch targeting (staging branches)
	Developer DeveloperConfig `yaml:"developer,omitempty"`

	// Brief generation settings for project context
	Brief BriefConfig `yaml:"brief"`

	// MCP (Model Context Protocol) server configuration
	MCP MCPConfig `yaml:"mcp"`

	// Hosting provider configuration (GitHub, GitLab, auto-detect)
	Hosting HostingConfig `yaml:"hosting"`

	// Knowledge layer configuration
	Knowledge KnowledgeConfig `yaml:"knowledge"`

	// Jira Cloud import configuration
	Jira JiraConfig `yaml:"jira"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Automation configuration for triggers and templates
	Automation AutomationConfig `yaml:"automation"`

	// Provider is the default LLM provider for all phases (default: "claude")
	// Supported: "claude", "codex"
	Provider string `yaml:"provider,omitempty"`

	// Providers contains provider-specific defaults.
	Providers ProvidersConfig `yaml:"providers,omitempty"`

	// Model is the default model for all phases (unless overridden in phase templates)
	Model         string `yaml:"model"`
	FallbackModel string `yaml:"fallback_model,omitempty"`

	// Execution settings
	MaxTurns int           `yaml:"max_turns"` // Claude CLI turn limit (default 150)
	Timeout  time.Duration `yaml:"timeout"`

	// Git settings
	BranchPrefix string `yaml:"branch_prefix"`
	CommitPrefix string `yaml:"commit_prefix"`

	// Claude CLI settings
	ClaudePath                 string `yaml:"claude_path"`
	CodexPath                  string `yaml:"codex_path,omitempty"`
	DangerouslySkipPermissions bool   `yaml:"dangerously_skip_permissions"`

	// Template paths
	TemplatesDir string `yaml:"templates_dir"`

	// Checkpoint settings
	EnableCheckpoints bool `yaml:"enable_checkpoints"`

	// Workflow is the active workflow ID for this project
	Workflow string `yaml:"workflow"`
}
