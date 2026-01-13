// Package config provides configuration management for orc.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the default config file name
	ConfigFileName = "config.yaml"
	// OrcDir is the orc configuration directory
	OrcDir = ".orc"
)

// AutomationProfile defines preset automation configurations.
type AutomationProfile string

const (
	// ProfileAuto - fully automated, no human intervention (default)
	ProfileAuto AutomationProfile = "auto"
	// ProfileFast - minimal gates, speed over safety
	ProfileFast AutomationProfile = "fast"
	// ProfileSafe - AI gates for review phases, human for merge only
	ProfileSafe AutomationProfile = "safe"
	// ProfileStrict - human gates on spec/review/merge for critical projects
	ProfileStrict AutomationProfile = "strict"
)

// GateConfig defines gate behavior configuration.
type GateConfig struct {
	// DefaultType is the default gate type when not specified (default: auto)
	DefaultType string `yaml:"default_type"`

	// PhaseOverrides allows overriding gate type per phase
	// e.g., {"merge": "human", "spec": "ai"}
	PhaseOverrides map[string]string `yaml:"phase_overrides,omitempty"`

	// WeightOverrides allows overriding gates per task weight
	// e.g., {"large": {"spec": "human"}}
	WeightOverrides map[string]map[string]string `yaml:"weight_overrides,omitempty"`

	// AutoApproveOnSuccess - if true, auto gates approve when phase completes
	// without checking criteria (default: true)
	AutoApproveOnSuccess bool `yaml:"auto_approve_on_success"`

	// RetryOnFailure - if true, failed phases retry from previous phase
	// instead of stopping (default: true for test phases)
	RetryOnFailure bool `yaml:"retry_on_failure"`

	// MaxRetries - max times to retry a phase from previous phase (default: 2)
	MaxRetries int `yaml:"max_retries"`
}

// RetryConfig defines cross-phase retry behavior.
type RetryConfig struct {
	// Enabled allows phases to retry from earlier phases on failure
	Enabled bool `yaml:"enabled"`

	// RetryMap defines which phase to retry from when a phase fails
	// e.g., {"test": "implement"} means if test fails, go back to implement
	RetryMap map[string]string `yaml:"retry_map,omitempty"`

	// MaxRetries per phase before giving up
	MaxRetries int `yaml:"max_retries"`
}

// WorktreeConfig defines worktree isolation settings.
type WorktreeConfig struct {
	// Enabled enables worktree isolation for tasks (default: true)
	Enabled bool `yaml:"enabled"`

	// Dir is the directory where worktrees are created (default: .orc/worktrees)
	Dir string `yaml:"dir"`

	// CleanupOnComplete removes worktree after successful completion (default: true)
	CleanupOnComplete bool `yaml:"cleanup_on_complete"`

	// CleanupOnFail removes worktree after failure (default: false for debugging)
	CleanupOnFail bool `yaml:"cleanup_on_fail"`
}

// PRConfig defines pull request settings.
type PRConfig struct {
	// Title template for PR title (default: "[orc] {{TASK_TITLE}}")
	Title string `yaml:"title"`

	// BodyTemplate is the path to PR body template (default: templates/pr-body.md)
	BodyTemplate string `yaml:"body_template"`

	// Labels to add to the PR
	Labels []string `yaml:"labels,omitempty"`

	// Reviewers to request review from
	Reviewers []string `yaml:"reviewers,omitempty"`

	// Draft creates PR as draft (default: false)
	Draft bool `yaml:"draft"`

	// AutoMerge enables auto-merge when approved (default: true)
	AutoMerge bool `yaml:"auto_merge"`
}

// SyncStrategy defines when to sync task branch with target.
type SyncStrategy string

const (
	// SyncStrategyNone disables automatic sync
	SyncStrategyNone SyncStrategy = "none"
	// SyncStrategyPhase syncs at the start of each phase
	SyncStrategyPhase SyncStrategy = "phase"
	// SyncStrategyCompletion syncs only at task completion (before PR)
	SyncStrategyCompletion SyncStrategy = "completion"
	// SyncStrategyDetect only detects conflicts without resolving (fail-fast)
	SyncStrategyDetect SyncStrategy = "detect"
)

// SyncConfig defines branch synchronization settings.
type SyncConfig struct {
	// Strategy defines when to sync: none, phase, completion, detect (default: completion)
	Strategy SyncStrategy `yaml:"strategy"`

	// FailOnConflict aborts execution on merge conflicts instead of attempting resolution (default: true)
	FailOnConflict bool `yaml:"fail_on_conflict"`

	// MaxConflictFiles is the max files with conflicts before aborting (0 = unlimited)
	MaxConflictFiles int `yaml:"max_conflict_files"`

	// SkipForWeights skips sync for these task weights
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`
}

// CompletionConfig defines task completion behavior.
type CompletionConfig struct {
	// Action defines what happens on completion: "pr", "merge", "none" (default: "pr")
	Action string `yaml:"action"`

	// TargetBranch is the branch to merge into (default: "main")
	TargetBranch string `yaml:"target_branch"`

	// DeleteBranch deletes task branch after merge (default: true)
	DeleteBranch bool `yaml:"delete_branch"`

	// PR settings (used when Action is "pr")
	PR PRConfig `yaml:"pr"`

	// Sync settings for branch synchronization
	Sync SyncConfig `yaml:"sync"`

	// WeightActions allows per-weight action overrides
	// e.g., {"trivial": "none", "small": "merge"}
	WeightActions map[string]string `yaml:"weight_actions,omitempty"`
}

// BudgetConfig defines cost budget settings.
type BudgetConfig struct {
	// ThresholdUSD is the budget threshold for cost alerts (0 = disabled)
	ThresholdUSD float64 `yaml:"threshold_usd"`

	// AlertOnExceed triggers an alert when threshold is exceeded
	AlertOnExceed bool `yaml:"alert_on_exceed"`

	// PauseOnExceed pauses task execution when budget is exceeded
	PauseOnExceed bool `yaml:"pause_on_exceed"`
}

// ExecutionConfig defines execution strategy settings.
type ExecutionConfig struct {
	// UseSessionExecution enables session-based execution with Claude's native
	// context continuity instead of flowgraph-based iteration. This provides
	// better context retention across iterations within a phase.
	// Default: false (uses flowgraph-based execution for compatibility)
	UseSessionExecution bool `yaml:"use_session_execution"`

	// SessionPersistence enables persisting sessions to disk for resume capability.
	// Only applicable when UseSessionExecution is true.
	// Default: true
	SessionPersistence bool `yaml:"session_persistence"`

	// CheckpointInterval controls how often to save iteration checkpoints.
	// 0 = only on phase completion, 1 = every iteration, N = every N iterations.
	// Only applicable when UseSessionExecution is true with FullExecutor.
	// Default: 1 for large/greenfield tasks, 0 for others
	CheckpointInterval int `yaml:"checkpoint_interval"`

	// MaxRetries is the maximum number of retry attempts when a phase fails.
	// When a phase fails (e.g., tests fail), orc will retry from an earlier phase
	// up to this many times before giving up.
	// Default: 5
	MaxRetries int `yaml:"max_retries"`
}

// PoolConfig defines token pool settings for automatic account switching.
type PoolConfig struct {
	// Enabled enables the token pool for automatic account switching on rate limits
	Enabled bool `yaml:"enabled"`

	// ConfigPath is the path to pool.yaml (default: ~/.orc/token-pool/pool.yaml)
	ConfigPath string `yaml:"config_path"`
}

// AuthConfig defines authentication settings for the server.
type AuthConfig struct {
	// Enabled enables authentication
	Enabled bool `yaml:"enabled"`

	// Type is the authentication type: "token" or "oidc"
	Type string `yaml:"type"`
}

// ServerConfig defines server configuration for team mode.
type ServerConfig struct {
	// Host is the server bind address (default: "127.0.0.1")
	Host string `yaml:"host"`

	// Port is the server port (default: 8080)
	Port int `yaml:"port"`

	// Auth configuration
	Auth AuthConfig `yaml:"auth"`
}

// TeamConfig defines organization/team settings.
// Every user is part of an organization (even solo users are an "org of 1").
// Features are opt-in with sensible defaults for solo developers.
type TeamConfig struct {
	// Name is the organization name (defaults to username or "Personal")
	Name string `yaml:"name,omitempty"`

	// ActivityLogging enables activity log for all actions (default: true)
	// Useful even for solo users as a history/audit trail
	ActivityLogging bool `yaml:"activity_logging"`

	// TaskClaiming enables task claiming/assignment features (default: false)
	// Only useful for multi-user setups - solo users don't need this
	TaskClaiming bool `yaml:"task_claiming"`

	// Visibility controls task visibility: all | assigned | owned
	// "all" = All members see all tasks (default)
	// "assigned" = Members only see tasks assigned to them or unassigned
	// "owned" = Members only see tasks they created or are assigned to
	Visibility string `yaml:"visibility"`

	// Mode is the coordination mode: local | shared_db | sync_server (future)
	// "local" = Single user, local database (default)
	// "shared_db" = Multiple users, shared PostgreSQL database
	// "sync_server" = Future: distributed sync server mode
	Mode string `yaml:"mode"`

	// ServerURL is the URL of the team server (for sync_server mode)
	ServerURL string `yaml:"server_url,omitempty"`
}

// IdentityConfig holds user identity settings for multi-user coordination.
type IdentityConfig struct {
	// Initials for executor prefix (e.g., "AM" for Alice Martinez)
	Initials string `yaml:"initials"`
	// DisplayName for team visibility (e.g., "Alice Martinez")
	DisplayName string `yaml:"display_name"`
	// Email for identification (optional)
	Email string `yaml:"email,omitempty"`
}

// TaskIDConfig holds task ID generation configuration.
type TaskIDConfig struct {
	// Mode is the coordination mode (solo, p2p, team)
	Mode string `yaml:"mode"`
	// PrefixSource determines how task ID prefix is derived (initials, username, etc)
	PrefixSource string `yaml:"prefix_source"`
}

// TestCommands defines test commands for different test types.
type TestCommands struct {
	// Unit is the command to run unit tests (default: "go test ./...")
	Unit string `yaml:"unit"`
	// Integration is the command to run integration tests
	Integration string `yaml:"integration"`
	// E2E is the command to run E2E tests
	E2E string `yaml:"e2e"`
	// Coverage is the command to generate coverage report
	Coverage string `yaml:"coverage"`
}

// TestingConfig defines test execution configuration.
type TestingConfig struct {
	// Required enforces that tests must pass (default: true)
	Required bool `yaml:"required"`
	// CoverageThreshold is the minimum coverage percentage required (0 = no threshold)
	CoverageThreshold int `yaml:"coverage_threshold"`
	// Types specifies which test types to run (unit, integration, e2e)
	Types []string `yaml:"types,omitempty"`
	// SkipForWeights skips testing for these task weights
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`
	// Commands defines test commands for different test types
	Commands TestCommands `yaml:"commands"`
	// ParseOutput enables structured parsing of test output for retry context
	ParseOutput bool `yaml:"parse_output"`
}

// DocumentationConfig defines documentation phase configuration.
type DocumentationConfig struct {
	// Enabled enables the docs phase (default: true)
	Enabled bool `yaml:"enabled"`
	// AutoUpdateClaudeMD enables auto-updating CLAUDE.md sections (default: true)
	AutoUpdateClaudeMD bool `yaml:"auto_update_claudemd"`
	// UpdateOn specifies when to run docs phase (feature, api_change, schema_change)
	UpdateOn []string `yaml:"update_on,omitempty"`
	// SkipForWeights skips docs for these task weights
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`
	// Sections specifies which auto-sections to maintain
	Sections []string `yaml:"sections,omitempty"`
}

// TimeoutsConfig defines timeout settings for phases.
type TimeoutsConfig struct {
	// PhaseMax is the maximum time per phase (0 = unlimited, default: 30m)
	PhaseMax time.Duration `yaml:"phase_max"`
	// IdleWarning is the duration to warn if no tool calls (default: 5m)
	IdleWarning time.Duration `yaml:"idle_warning"`
}

// QAConfig defines QA session configuration.
type QAConfig struct {
	// Enabled enables the QA phase (default: true)
	Enabled bool `yaml:"enabled"`
	// SkipForWeights skips QA for these task weights (default: [trivial])
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`
	// RequireE2E requires e2e tests to pass (default: false)
	RequireE2E bool `yaml:"require_e2e"`
	// GenerateDocs enables auto-generating feature docs (default: true)
	GenerateDocs bool `yaml:"generate_docs"`
}

// ReviewConfig defines multi-round review configuration.
type ReviewConfig struct {
	// Enabled enables the review phase (default: true)
	Enabled bool `yaml:"enabled"`
	// Rounds is the number of review rounds (default: 2)
	Rounds int `yaml:"rounds"`
	// RequirePass requires review to pass before continuing (default: true)
	RequirePass bool `yaml:"require_pass"`
}

// PlanConfig defines spec requirements and validation configuration.
type PlanConfig struct {
	// RequireSpecForExecution blocks execution if spec is missing/invalid (default: false)
	RequireSpecForExecution bool `yaml:"require_spec_for_execution"`
	// WarnOnMissingSpec warns but doesn't block when spec is missing (default: true)
	WarnOnMissingSpec bool `yaml:"warn_on_missing_spec"`
	// SkipValidationWeights skips spec validation for these weights (default: [trivial])
	SkipValidationWeights []string `yaml:"skip_validation_weights,omitempty"`
	// MinimumSections are the required sections in a spec (default: [intent, success_criteria, testing])
	MinimumSections []string `yaml:"minimum_sections,omitempty"`
}

// SubtasksConfig defines sub-task queue configuration.
type SubtasksConfig struct {
	// AllowCreation allows agents to propose sub-tasks (default: true)
	AllowCreation bool `yaml:"allow_creation"`
	// AutoApprove automatically approves proposed sub-tasks (default: false)
	AutoApprove bool `yaml:"auto_approve"`
	// MaxPending is the max number of pending sub-tasks per task (default: 10)
	MaxPending int `yaml:"max_pending"`
}

// DatabaseConfig defines database connection settings.
type DatabaseConfig struct {
	// Driver is the database type: "sqlite" or "postgres"
	Driver string `yaml:"driver"`

	// SQLite settings
	SQLite SQLiteConfig `yaml:"sqlite"`

	// Postgres settings (for team mode)
	Postgres PostgresConfig `yaml:"postgres"`
}

// StorageMode defines how orc stores task data.
type StorageMode string

const (
	// StorageModeHybrid uses YAML files as primary with SQLite cache for search
	StorageModeHybrid StorageMode = "hybrid"
	// StorageModeFiles uses YAML files only (minimal, git-friendly)
	StorageModeFiles StorageMode = "files"
	// StorageModeDatabase uses database as primary (team/enterprise)
	StorageModeDatabase StorageMode = "database"
)

// FileStorageConfig defines file-based storage settings.
type FileStorageConfig struct {
	// CleanupOnComplete removes task files after successful completion
	// Default: true (keeps .orc/tasks/ clean)
	CleanupOnComplete bool `yaml:"cleanup_on_complete"`
}

// DatabaseStorageConfig defines database storage settings.
type DatabaseStorageConfig struct {
	// CacheTranscripts enables FTS search for transcripts
	// Default: true
	CacheTranscripts bool `yaml:"cache_transcripts"`

	// RetentionDays is how long to keep entries before cleanup
	// Default: 90
	RetentionDays int `yaml:"retention_days"`
}

// ExportPreset defines a preset export configuration.
type ExportPreset string

const (
	// ExportPresetMinimal exports only task.yaml
	ExportPresetMinimal ExportPreset = "minimal"
	// ExportPresetStandard exports task definition + final state
	ExportPresetStandard ExportPreset = "standard"
	// ExportPresetFull exports everything including transcripts
	ExportPresetFull ExportPreset = "full"
)

// ExportConfig defines what to export to branch on PR creation.
type ExportConfig struct {
	// Enabled is the master toggle for export (default: false)
	Enabled bool `yaml:"enabled"`

	// Preset sets a predefined export configuration (overrides individual flags)
	// Values: minimal, standard, full
	Preset ExportPreset `yaml:"preset,omitempty"`

	// TaskDefinition exports task.yaml and plan.yaml
	TaskDefinition bool `yaml:"task_definition"`

	// FinalState exports state.yaml
	FinalState bool `yaml:"final_state"`

	// Transcripts exports full conversation logs (usually large)
	Transcripts bool `yaml:"transcripts"`

	// ContextSummary exports generated context.md
	ContextSummary bool `yaml:"context_summary"`
}

// StorageConfig defines how orc stores and exports task data.
// This is separate from DatabaseConfig which handles connection settings.
type StorageConfig struct {
	// Mode is the storage mode: hybrid | files | database
	// Default: hybrid (best of both worlds for solo devs)
	Mode StorageMode `yaml:"mode"`

	// Files contains file storage settings
	Files FileStorageConfig `yaml:"files"`

	// Database contains database storage settings
	Database DatabaseStorageConfig `yaml:"database"`

	// Export contains settings for exporting to branch
	Export ExportConfig `yaml:"export"`
}

// SQLiteConfig defines SQLite-specific settings.
type SQLiteConfig struct {
	// Path for project database (relative to project root)
	Path string `yaml:"path"`

	// GlobalPath for global database
	GlobalPath string `yaml:"global_path"`
}

// PostgresConfig defines PostgreSQL-specific settings.
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"` // Use env ORC_DB_PASSWORD
	SSLMode  string `yaml:"ssl_mode"`
	PoolMax  int    `yaml:"pool_max"`
}

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

	// Task ID generation settings
	TaskID TaskIDConfig `yaml:"task_id"`

	// Testing configuration
	Testing TestingConfig `yaml:"testing"`

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

	// Sub-task queue configuration
	Subtasks SubtasksConfig `yaml:"subtasks"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Model settings
	Model         string `yaml:"model"`
	FallbackModel string `yaml:"fallback_model,omitempty"`

	// Execution settings
	MaxIterations int           `yaml:"max_iterations"`
	Timeout       time.Duration `yaml:"timeout"`

	// Git settings
	BranchPrefix string `yaml:"branch_prefix"`
	CommitPrefix string `yaml:"commit_prefix"`

	// Claude CLI settings
	ClaudePath                 string `yaml:"claude_path"`
	DangerouslySkipPermissions bool   `yaml:"dangerously_skip_permissions"`

	// Template paths
	TemplatesDir string `yaml:"templates_dir"`

	// Checkpoint settings
	EnableCheckpoints bool `yaml:"enable_checkpoints"`
}

// ResolveGateType returns the effective gate type for a phase given task weight.
// Priority: weight override > phase override > default
func (c *Config) ResolveGateType(phase string, weight string) string {
	// Check weight-specific override first
	if c.Gates.WeightOverrides != nil {
		if weightOverrides, ok := c.Gates.WeightOverrides[weight]; ok {
			if gateType, ok := weightOverrides[phase]; ok {
				return gateType
			}
		}
	}

	// Check phase override
	if c.Gates.PhaseOverrides != nil {
		if gateType, ok := c.Gates.PhaseOverrides[phase]; ok {
			return gateType
		}
	}

	// Return default
	if c.Gates.DefaultType != "" {
		return c.Gates.DefaultType
	}

	return "auto"
}

// ShouldRetryFrom returns the phase to retry from if the given phase fails.
// Returns empty string if no retry configured.
func (c *Config) ShouldRetryFrom(failedPhase string) string {
	if !c.Retry.Enabled {
		return ""
	}
	if c.Retry.RetryMap != nil {
		return c.Retry.RetryMap[failedPhase]
	}
	return ""
}

// ResolveCompletionAction returns the effective completion action for a task weight.
// Priority: weight-specific override > default action
func (c *Config) ResolveCompletionAction(weight string) string {
	if c.Completion.WeightActions != nil {
		if action, ok := c.Completion.WeightActions[weight]; ok {
			return action
		}
	}
	return c.Completion.Action
}

// Default returns the default configuration.
// Default is AUTOMATION-FIRST: all gates auto, retry enabled.
func Default() *Config {
	return &Config{
		Version: 1,
		Profile: ProfileAuto,
		Gates: GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			// No phase or weight overrides by default - everything is auto
		},
		Retry: RetryConfig{
			Enabled:    true,
			MaxRetries: 5,
			// Default retry map: if test fails, go back to implement
			RetryMap: map[string]string{
				"test":      "implement",
				"test_unit": "implement",
				"test_e2e":  "implement",
				"validate":  "implement",
			},
		},
		Worktree: WorktreeConfig{
			Enabled:           true,
			Dir:               ".orc/worktrees",
			CleanupOnComplete: true,
			CleanupOnFail:     false, // Keep for debugging
		},
		Completion: CompletionConfig{
			Action:       "pr",
			TargetBranch: "main",
			DeleteBranch: true,
			PR: PRConfig{
				Title:        "[orc] {{TASK_TITLE}}",
				BodyTemplate: "templates/pr-body.md",
				Labels:       []string{"automated"},
				AutoMerge:    true,
			},
			Sync: SyncConfig{
				Strategy:       SyncStrategyCompletion, // Sync before PR creation by default
				FailOnConflict: true,                   // Fail on conflicts by default - let user decide resolution
				MaxConflictFiles: 0,                    // No limit by default
				SkipForWeights:   []string{"trivial"},  // Skip sync for trivial tasks
			},
			// Safety defaults: use PR workflow for protected branches
			// Direct merge is blocked for protected branches (main, master, develop, release)
			WeightActions: map[string]string{
				"trivial": "none", // No PR for trivial fixes (local-only changes)
				// All other weights use default "pr" action
			},
		},
		Execution: ExecutionConfig{
			UseSessionExecution: false, // Default to flowgraph for compatibility
			SessionPersistence:  true,
			CheckpointInterval:  0,  // Default to phase-complete only
			MaxRetries:          5,  // Default retry limit for phase failures
		},
		Pool: PoolConfig{
			Enabled:    false, // Disabled by default
			ConfigPath: "~/.orc/token-pool/pool.yaml",
		},
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
			Auth: AuthConfig{
				Enabled: false,
				Type:    "token",
			},
		},
		Team: TeamConfig{
			Name:            "",    // Auto-detected from username
			ActivityLogging: true,  // On by default - useful history even for solo
			TaskClaiming:    false, // Off by default - opt-in for multi-user
			Visibility:      "all",
			Mode:            "local", // Local by default, shared_db for teams
			ServerURL:       "",
		},
		Identity: IdentityConfig{
			Initials:    "",
			DisplayName: "",
		},
		TaskID: TaskIDConfig{
			Mode:         "solo",
			PrefixSource: "initials",
		},
		Testing: TestingConfig{
			Required:          true,
			CoverageThreshold: 0, // No threshold by default
			Types:             []string{"unit"},
			SkipForWeights:    []string{"trivial"},
			Commands: TestCommands{
				Unit:        "go test ./...",
				Integration: "go test -tags=integration ./...",
				E2E:         "make e2e",
				Coverage:    "go test -coverprofile=coverage.out ./...",
			},
			ParseOutput: true,
		},
		Documentation: DocumentationConfig{
			Enabled:            true,
			AutoUpdateClaudeMD: true,
			UpdateOn:           []string{"feature", "api_change"},
			SkipForWeights:     []string{"trivial"},
			Sections:           []string{"api-endpoints", "commands", "config-options"},
		},
		Timeouts: TimeoutsConfig{
			PhaseMax:    30 * time.Minute,
			IdleWarning: 5 * time.Minute,
		},
		QA: QAConfig{
			Enabled:        true,
			SkipForWeights: []string{"trivial"},
			RequireE2E:     false,
			GenerateDocs:   true,
		},
		Review: ReviewConfig{
			Enabled:     true,
			Rounds:      2,
			RequirePass: true,
		},
		Plan: PlanConfig{
			RequireSpecForExecution: false, // Don't block by default
			WarnOnMissingSpec:       true,  // Warn but don't block
			SkipValidationWeights:   []string{"trivial"},
			MinimumSections:         []string{"intent", "success_criteria", "testing"},
		},
		Subtasks: SubtasksConfig{
			AllowCreation: true,
			AutoApprove:   false,
			MaxPending:    10,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{
				Path:       ".orc/orc.db",
				GlobalPath: "~/.orc/orc.db",
			},
			Postgres: PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "orc",
				User:     "orc",
				SSLMode:  "disable",
				PoolMax:  10,
			},
		},
		Storage: StorageConfig{
			Mode: StorageModeHybrid, // Best of both worlds for solo devs
			Files: FileStorageConfig{
				CleanupOnComplete: true, // Keep .orc/tasks/ clean
			},
			Database: DatabaseStorageConfig{
				CacheTranscripts: true, // FTS search enabled by default
				RetentionDays:    90,   // Auto-cleanup old entries
			},
			Export: ExportConfig{
				Enabled:        false, // Nothing exported by default
				TaskDefinition: true,  // When enabled, export task.yaml + plan.yaml
				FinalState:     true,  // When enabled, export state.yaml
				Transcripts:    false, // Usually too large
				ContextSummary: true,  // When enabled, export context.md
			},
		},
		Model:                      "claude-opus-4-5-20251101",
		MaxIterations:              30,
		Timeout:                    10 * time.Minute,
		BranchPrefix:               "orc/",
		CommitPrefix:               "[orc]",
		ClaudePath:                 "claude",
		DangerouslySkipPermissions: true,
		TemplatesDir:               "templates",
		EnableCheckpoints:          true,
	}
}

// ProfilePresets returns gate configuration for a given automation profile.
func ProfilePresets(profile AutomationProfile) GateConfig {
	switch profile {
	case ProfileFast:
		// Fast: everything auto, no AI review, fewer retries for speed
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           2,
		}
	case ProfileSafe:
		// Safe: AI reviews, human only for merge
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			PhaseOverrides: map[string]string{
				"review": "ai",
				"merge":  "human",
			},
		}
	case ProfileStrict:
		// Strict: human gates on key decisions
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
			PhaseOverrides: map[string]string{
				"spec":     "human",
				"design":   "human",
				"review":   "ai",
				"validate": "ai",
				"merge":    "human",
			},
		}
	default: // ProfileAuto
		// Auto: fully automated, no human intervention
		return GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
			RetryOnFailure:       true,
			MaxRetries:           5,
		}
	}
}

// ApplyProfile applies a preset profile to the configuration.
func (c *Config) ApplyProfile(profile AutomationProfile) {
	c.Profile = profile
	c.Gates = ProfilePresets(profile)
}

// Load loads the config from the default location.
func Load() (*Config, error) {
	return LoadFrom(filepath.Join(OrcDir, ConfigFileName))
}

// LoadFrom loads the config from a specific path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default() // Start with defaults
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Save saves the config to the default location.
func (c *Config) Save() error {
	return c.SaveTo(filepath.Join(OrcDir, ConfigFileName))
}

// SaveTo saves the config to a specific path.
func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Init initializes the orc directory structure in the current directory.
func Init(force bool) error {
	return InitAt(".", force)
}

// InitAt initializes the orc directory structure at the specified base path.
func InitAt(basePath string, force bool) error {
	orcDir := filepath.Join(basePath, OrcDir)
	// Check if already initialized
	if !force {
		if _, err := os.Stat(orcDir); err == nil {
			return fmt.Errorf("orc already initialized (use --force to overwrite)")
		}
	}

	// Create directory structure
	dirs := []string{
		orcDir,
		filepath.Join(orcDir, "tasks"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Write default config
	cfg := Default()
	if err := cfg.SaveTo(filepath.Join(orcDir, ConfigFileName)); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// IsInitialized returns true if orc is initialized in the current directory.
func IsInitialized() bool {
	return IsInitializedAt(".")
}

// IsInitializedAt returns true if orc is initialized at the specified base path.
func IsInitializedAt(basePath string) bool {
	_, err := os.Stat(filepath.Join(basePath, OrcDir))
	return err == nil
}

// RequireInit returns an error if orc is not initialized in the current directory.
func RequireInit() error {
	return RequireInitAt(".")
}

// RequireInitAt returns an error if orc is not initialized at the specified base path.
func RequireInitAt(basePath string) error {
	if !IsInitializedAt(basePath) {
		return fmt.Errorf("not an orc project (no %s directory). Run 'orc init' first", OrcDir)
	}
	return nil
}

// ExecutorPrefix returns the prefix for branch/worktree naming based on mode.
// Returns empty string in solo mode, identity initials in p2p/team mode.
func (c *Config) ExecutorPrefix() string {
	if c.TaskID.Mode == "solo" {
		return ""
	}
	return c.Identity.Initials
}

// ShouldSkipQA returns true if QA should be skipped for the given task weight.
func (c *Config) ShouldSkipQA(weight string) bool {
	if !c.QA.Enabled {
		return true
	}
	for _, w := range c.QA.SkipForWeights {
		if w == weight {
			return true
		}
	}
	return false
}

// ShouldSkipReview returns true if review should be skipped.
func (c *Config) ShouldSkipReview() bool {
	return !c.Review.Enabled
}

// EffectiveMaxRetries returns the configured maximum retry attempts.
// This checks executor.max_retries first (the primary config location),
// then falls back to retry.max_retries for backward compatibility.
// Returns 5 (the default) if neither is explicitly set.
func (c *Config) EffectiveMaxRetries() int {
	// executor.max_retries takes precedence
	if c.Execution.MaxRetries > 0 {
		return c.Execution.MaxRetries
	}
	// Fall back to retry.max_retries for backward compatibility
	if c.Retry.MaxRetries > 0 {
		return c.Retry.MaxRetries
	}
	// Default to 5
	return 5
}

// ShouldSyncForWeight returns true if sync should be performed for this weight.
func (c *Config) ShouldSyncForWeight(weight string) bool {
	if c.Completion.Sync.Strategy == SyncStrategyNone {
		return false
	}
	for _, w := range c.Completion.Sync.SkipForWeights {
		if w == weight {
			return false
		}
	}
	return true
}

// ShouldSyncBeforePhase returns true if sync should happen before each phase.
func (c *Config) ShouldSyncBeforePhase() bool {
	return c.Completion.Sync.Strategy == SyncStrategyPhase
}

// ShouldSyncAtCompletion returns true if sync should happen at task completion.
func (c *Config) ShouldSyncAtCompletion() bool {
	return c.Completion.Sync.Strategy == SyncStrategyCompletion ||
		c.Completion.Sync.Strategy == SyncStrategyDetect
}

// ShouldDetectConflictsOnly returns true if we should only detect conflicts, not resolve.
func (c *Config) ShouldDetectConflictsOnly() bool {
	return c.Completion.Sync.Strategy == SyncStrategyDetect
}

// DSN returns the database connection string based on current config.
func (c *Config) DSN() string {
	if c.Database.Driver == "postgres" {
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			c.Database.Postgres.User,
			c.Database.Postgres.Password,
			c.Database.Postgres.Host,
			c.Database.Postgres.Port,
			c.Database.Postgres.Database,
			c.Database.Postgres.SSLMode,
		)
	}
	return c.Database.SQLite.Path
}

// GlobalDSN returns the global database connection string.
func (c *Config) GlobalDSN() string {
	if c.Database.Driver == "postgres" {
		return c.DSN() // Same DB in postgres mode
	}
	return c.Database.SQLite.GlobalPath
}

// Valid values for validation
var (
	// ValidVisibilities are the allowed values for team.visibility
	ValidVisibilities = []string{"all", "assigned", "owned"}

	// ValidModes are the allowed values for team.mode
	ValidModes = []string{"local", "shared_db", "sync_server"}

	// ValidCompletionActions are the allowed values for completion.action
	ValidCompletionActions = []string{"pr", "merge", "none", ""}

	// ValidSyncStrategies are the allowed values for completion.sync.strategy
	ValidSyncStrategies = []string{
		string(SyncStrategyNone),
		string(SyncStrategyPhase),
		string(SyncStrategyCompletion),
		string(SyncStrategyDetect),
		"", // empty defaults to completion
	}

	// DefaultProtectedBranches are branches that cannot be directly merged to
	DefaultProtectedBranches = []string{"main", "master", "develop", "release"}
)

// Validate checks if config values are valid.
func (c *Config) Validate() error {
	if c.Team.Visibility != "" && !contains(ValidVisibilities, c.Team.Visibility) {
		return fmt.Errorf("invalid team.visibility: %s (must be one of: %v)",
			c.Team.Visibility, ValidVisibilities)
	}
	if c.Team.Mode != "" && !contains(ValidModes, c.Team.Mode) {
		return fmt.Errorf("invalid team.mode: %s (must be one of: %v)",
			c.Team.Mode, ValidModes)
	}

	// Validate completion action
	if c.Completion.Action != "" && !contains(ValidCompletionActions, c.Completion.Action) {
		return fmt.Errorf("invalid completion.action: %s (must be one of: pr, merge, none)",
			c.Completion.Action)
	}

	// Validate sync strategy
	if !contains(ValidSyncStrategies, string(c.Completion.Sync.Strategy)) {
		return fmt.Errorf("invalid completion.sync.strategy: %s (must be one of: none, phase, completion, detect)",
			c.Completion.Sync.Strategy)
	}

	// SAFETY: Block "merge" action when target is a protected branch
	// This prevents accidental direct merges to main/master/develop/release
	if c.Completion.Action == "merge" {
		targetBranch := c.Completion.TargetBranch
		if targetBranch == "" {
			targetBranch = "main" // default
		}
		if isProtectedBranch(targetBranch) {
			return fmt.Errorf("completion.action 'merge' is blocked for protected branch '%s'; "+
				"use 'pr' action instead to ensure code review before merging to protected branches",
				targetBranch)
		}
	}

	// Validate weight-specific actions don't allow merge to protected branches
	for weight, action := range c.Completion.WeightActions {
		if action == "merge" {
			targetBranch := c.Completion.TargetBranch
			if targetBranch == "" {
				targetBranch = "main"
			}
			if isProtectedBranch(targetBranch) {
				return fmt.Errorf("completion.weight_actions[%s]='merge' is blocked for protected branch '%s'; "+
					"use 'pr' action instead", weight, targetBranch)
			}
		}
	}

	// SAFETY: Worktree isolation should not be disabled
	// This is a critical safety feature that prevents parallel tasks from interfering
	if !c.Worktree.Enabled {
		return fmt.Errorf("worktree.enabled cannot be set to false; " +
			"worktree isolation is required for safe parallel task execution and branch protection; " +
			"if you need to run without worktrees, contact maintainers to discuss your use case")
	}

	// Validate storage configuration
	if err := c.validateStorage(); err != nil {
		return err
	}

	return nil
}

// isProtectedBranch checks if a branch is in the protected list.
func isProtectedBranch(branch string) bool {
	for _, p := range DefaultProtectedBranches {
		if branch == p {
			return true
		}
	}
	return false
}

// contains checks if a string is in a slice.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ValidStorageModes are the allowed values for storage.mode
var ValidStorageModes = []string{string(StorageModeHybrid), string(StorageModeFiles), string(StorageModeDatabase)}

// ValidExportPresets are the allowed values for storage.export.preset
var ValidExportPresets = []string{string(ExportPresetMinimal), string(ExportPresetStandard), string(ExportPresetFull), ""}

// ResolveExportConfig returns the effective export configuration,
// applying preset overrides if a preset is specified.
func (c *StorageConfig) ResolveExportConfig() ExportConfig {
	if c.Export.Preset == "" {
		return c.Export
	}

	result := c.Export
	switch c.Export.Preset {
	case ExportPresetMinimal:
		result.TaskDefinition = true
		result.FinalState = false
		result.Transcripts = false
		result.ContextSummary = false
	case ExportPresetStandard:
		result.TaskDefinition = true
		result.FinalState = true
		result.Transcripts = false
		result.ContextSummary = true
	case ExportPresetFull:
		result.TaskDefinition = true
		result.FinalState = true
		result.Transcripts = true
		result.ContextSummary = true
	}
	return result
}

// ShouldExport returns true if any export is enabled and the master toggle is on.
func (c *StorageConfig) ShouldExport() bool {
	if !c.Export.Enabled {
		return false
	}
	resolved := c.ResolveExportConfig()
	return resolved.TaskDefinition || resolved.FinalState ||
		resolved.Transcripts || resolved.ContextSummary
}

// validateStorage validates the storage configuration.
func (c *Config) validateStorage() error {
	if c.Storage.Mode != "" && !contains(ValidStorageModes, string(c.Storage.Mode)) {
		return fmt.Errorf("invalid storage.mode: %s (must be one of: %v)",
			c.Storage.Mode, ValidStorageModes)
	}

	if c.Storage.Export.Preset != "" && !contains(ValidExportPresets, string(c.Storage.Export.Preset)) {
		return fmt.Errorf("invalid storage.export.preset: %s (must be one of: %v)",
			c.Storage.Export.Preset, ValidExportPresets)
	}

	// Validate retention days - must be between 0 and 3650 (10 years)
	if c.Storage.Database.RetentionDays < 0 || c.Storage.Database.RetentionDays > 3650 {
		return fmt.Errorf("storage.database.retention_days must be between 0 and 3650")
	}

	return nil
}
