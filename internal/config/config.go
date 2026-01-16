// Package config provides configuration management for orc.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// Note: This uses GitHub's auto-merge feature which requires branch protection.
	// For repos without branch protection, use MergeOnCIPass instead.
	AutoMerge bool `yaml:"auto_merge"`

	// AutoApprove enables AI-assisted PR approval in auto mode (default: true for "auto" profile)
	// When enabled, after PR creation the AI will:
	// 1. Review the diff for obvious issues
	// 2. Verify tests passed
	// 3. Approve the PR via 'gh pr review --approve'
	// For safe/strict profiles, this is disabled and human approval is required.
	AutoApprove bool `yaml:"auto_approve"`
}

// CIConfig defines CI/CD integration settings.
type CIConfig struct {
	// WaitForCI enables waiting for CI checks to pass before merge (default: true)
	// When enabled after finalize phase:
	// 1. Push finalize changes
	// 2. Poll CI checks until all pass (or timeout)
	// 3. Merge PR directly with `gh pr merge --squash`
	WaitForCI bool `yaml:"wait_for_ci"`

	// CITimeout is the maximum time to wait for CI checks to pass (default: 10m)
	CITimeout time.Duration `yaml:"ci_timeout"`

	// PollInterval is how often to check CI status (default: 30s)
	PollInterval time.Duration `yaml:"poll_interval"`

	// MergeOnCIPass enables direct merge after CI passes (default: true for auto/fast profiles)
	// This bypasses GitHub's auto-merge feature (which requires branch protection).
	// The merge flow becomes: finalize passes + CI passes = merge directly.
	MergeOnCIPass bool `yaml:"merge_on_ci_pass"`

	// MergeMethod is the method to use when merging (squash, merge, rebase)
	// Default: squash
	MergeMethod string `yaml:"merge_method"`
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

	// SyncOnStart syncs branch with target before execution starts (default: true)
	// This catches conflicts from parallel tasks early, while the implement phase
	// can still incorporate changes and resolve them.
	SyncOnStart bool `yaml:"sync_on_start"`

	// FailOnConflict aborts execution on merge conflicts instead of attempting resolution (default: true)
	FailOnConflict bool `yaml:"fail_on_conflict"`

	// MaxConflictFiles is the max files with conflicts before aborting (0 = unlimited)
	MaxConflictFiles int `yaml:"max_conflict_files"`

	// SkipForWeights skips sync for these task weights
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`
}

// FinalizeSyncStrategy defines how to integrate target branch changes.
type FinalizeSyncStrategy string

const (
	// FinalizeSyncRebase rebases task branch onto target (linear history)
	FinalizeSyncRebase FinalizeSyncStrategy = "rebase"
	// FinalizeSyncMerge merges target into task branch (preserves history)
	FinalizeSyncMerge FinalizeSyncStrategy = "merge"
)

// FinalizeSyncConfig defines sync behavior for the finalize phase.
type FinalizeSyncConfig struct {
	// Strategy defines how to integrate target branch: rebase or merge (default: merge)
	Strategy FinalizeSyncStrategy `yaml:"strategy"`
}

// ConflictResolutionConfig defines automatic conflict resolution behavior.
type ConflictResolutionConfig struct {
	// Enabled enables AI-assisted conflict resolution (default: true)
	Enabled bool `yaml:"enabled"`

	// Instructions are additional instructions for conflict resolution
	// These are appended to the default conflict resolution rules
	Instructions string `yaml:"instructions,omitempty"`
}

// RiskAssessmentConfig defines risk assessment behavior.
type RiskAssessmentConfig struct {
	// Enabled enables risk assessment during finalize (default: true)
	Enabled bool `yaml:"enabled"`

	// ReReviewThreshold is the risk level at which to require re-review
	// Values: low, medium, high, critical (default: high)
	// When risk level meets or exceeds this threshold, recommend re-review
	ReReviewThreshold string `yaml:"re_review_threshold"`
}

// FinalizeGatesConfig defines gate behavior specific to finalize phase.
type FinalizeGatesConfig struct {
	// PreMerge gate type before creating PR/merging: auto, ai, human, none (default: auto)
	PreMerge string `yaml:"pre_merge"`
}

// FinalizeConfig defines finalize phase configuration.
type FinalizeConfig struct {
	// Enabled enables the finalize phase (default: true)
	Enabled bool `yaml:"enabled"`

	// AutoTrigger automatically runs finalize after validate phase (default: true)
	// When false, finalize must be triggered manually
	AutoTrigger bool `yaml:"auto_trigger"`

	// AutoTriggerOnApproval automatically runs finalize when PR is approved (default: true for "auto" profile)
	// Only applies when automation profile is "auto". When enabled, the PR status poller
	// will trigger finalize automatically when a PR receives approval.
	AutoTriggerOnApproval bool `yaml:"auto_trigger_on_approval"`

	// Sync settings for branch integration during finalize
	Sync FinalizeSyncConfig `yaml:"sync"`

	// ConflictResolution settings for automatic conflict resolution
	ConflictResolution ConflictResolutionConfig `yaml:"conflict_resolution"`

	// RiskAssessment settings for merge risk evaluation
	RiskAssessment RiskAssessmentConfig `yaml:"risk_assessment"`

	// Gates settings for finalize phase gates
	Gates FinalizeGatesConfig `yaml:"gates"`
}

// CompletionConfig defines task completion behavior.
type CompletionConfig struct {
	// Action defines what happens on completion: "pr", "merge", "none" (default: "pr")
	Action string `yaml:"action"`

	// TargetBranch is the branch to merge into (default: "main")
	TargetBranch string `yaml:"target_branch"`

	// DeleteBranch deletes task branch after merge (default: true)
	DeleteBranch bool `yaml:"delete_branch"`

	// WaitForCI waits for CI checks to pass before merging after finalize (default: true)
	// When enabled, after finalize completes, orc will poll PR checks until they pass
	// (or timeout), then merge the PR directly instead of relying on GitHub's auto-merge.
	WaitForCI bool `yaml:"wait_for_ci"`

	// CITimeout is the maximum time to wait for CI checks to pass (default: 10m)
	// After this timeout, the merge attempt is abandoned but the PR remains open.
	CITimeout time.Duration `yaml:"ci_timeout"`

	// MergeOnCIPass automatically merges when CI passes after finalize (default: true)
	// Requires WaitForCI to be enabled. Uses gh pr merge --squash.
	MergeOnCIPass bool `yaml:"merge_on_ci_pass"`

	// PR settings (used when Action is "pr")
	PR PRConfig `yaml:"pr"`

	// CI settings for CI/CD integration (wait for checks, auto-merge)
	CI CIConfig `yaml:"ci"`

	// Sync settings for branch synchronization
	Sync SyncConfig `yaml:"sync"`

	// Finalize settings for the finalize phase
	Finalize FinalizeConfig `yaml:"finalize"`

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

	// MaxPortAttempts is the number of ports to try if the initial port is busy (default: 10)
	// If port 8080 is busy, tries 8081, 8082, etc. up to Port + MaxPortAttempts - 1
	MaxPortAttempts int `yaml:"max_port_attempts"`

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
	// TurnMax is the maximum time per API turn/iteration (0 = unlimited, default: 10m)
	// If a single API call takes longer than this, it will be cancelled gracefully.
	TurnMax time.Duration `yaml:"turn_max"`
	// IdleWarning is the duration to warn if no tool calls (default: 5m)
	IdleWarning time.Duration `yaml:"idle_warning"`
	// HeartbeatInterval is how often to show progress dots during API calls (default: 30s)
	// Set to 0 to disable heartbeat dots.
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	// IdleTimeout is the duration after which to warn about no streaming activity (default: 2m)
	// This helps detect stuck API calls before the turn timeout.
	IdleTimeout time.Duration `yaml:"idle_timeout"`
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
	// Parallel configures parallel reviewer agents
	Parallel ParallelReviewConfig `yaml:"parallel,omitempty"`
}

// ParallelReviewConfig defines configuration for parallel reviewer agents.
type ParallelReviewConfig struct {
	// Enabled enables parallel reviewers for medium+ weight tasks (default: false)
	Enabled bool `yaml:"enabled"`
	// Perspectives defines which reviewer perspectives to use
	// Valid values: correctness, architecture, security, performance
	// Default: [correctness, architecture]
	Perspectives []string `yaml:"perspectives,omitempty"`
}

// PlanConfig defines spec requirements and validation configuration.
type PlanConfig struct {
	// RequireSpecForExecution blocks execution if spec is missing/invalid (default: false)
	RequireSpecForExecution bool `yaml:"require_spec_for_execution"`
	// WarnOnMissingSpec warns but doesn't block when spec is missing (default: true)
	WarnOnMissingSpec bool `yaml:"warn_on_missing_spec"`
	// SkipValidationWeights skips spec validation and warnings for these weights (default: [trivial, small])
	SkipValidationWeights []string `yaml:"skip_validation_weights,omitempty"`
	// MinimumSections are the required sections in a spec (default: [intent, success_criteria, testing])
	MinimumSections []string `yaml:"minimum_sections,omitempty"`
}

// ArtifactSkipConfig defines artifact detection and auto-skip behavior.
type ArtifactSkipConfig struct {
	// Enabled enables artifact detection for phases (default: true)
	Enabled bool `yaml:"enabled"`

	// AutoSkip automatically skips phases with existing artifacts without prompting (default: false)
	// When false, prompts user: "spec.md already exists. Skip spec phase? [Y/n]"
	AutoSkip bool `yaml:"auto_skip"`

	// Phases specifies which phases to check for artifacts (default: [spec, research, docs])
	// implement, test, and validate are excluded by default as they need re-execution
	Phases []string `yaml:"phases,omitempty"`
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

// TasksConfig defines task-level configuration.
type TasksConfig struct {
	// DisableAutoCommit disables automatic git commits on task creation/modification (default: false)
	// When enabled, task files are not auto-committed and must be committed manually.
	DisableAutoCommit bool `yaml:"disable_auto_commit"`
}

// ResourceTrackingConfig defines resource tracking configuration for diagnostics.
type ResourceTrackingConfig struct {
	// Enabled enables process/memory tracking before and after task execution (default: true)
	Enabled bool `yaml:"enabled"`

	// MemoryThresholdMB is the memory growth threshold that triggers warnings (default: 500)
	MemoryThresholdMB int `yaml:"memory_threshold_mb"`

	// LogOrphanedMCPOnly limits orphan logging to MCP-related processes (playwright, browsers)
	// Deprecated: use FilterSystemProcesses instead. When true, only MCP processes are logged.
	LogOrphanedMCPOnly bool `yaml:"log_orphaned_mcp_only"`

	// FilterSystemProcesses controls whether to filter out system processes from orphan detection.
	// When true (default), only processes that match orc-related patterns (claude, node, playwright,
	// chromium, etc.) are flagged as potential orphans. System processes like systemd-timedated,
	// snapper, etc. are ignored even if they started during task execution.
	// When false, all new orphaned processes are flagged (original behavior, prone to false positives).
	FilterSystemProcesses bool `yaml:"filter_system_processes"`
}

// DiagnosticsConfig defines diagnostic feature configuration.
type DiagnosticsConfig struct {
	// ResourceTracking contains settings for process/memory tracking
	ResourceTracking ResourceTrackingConfig `yaml:"resource_tracking"`
}

// DeveloperConfig defines personal developer settings for branch targeting.
// These settings live in personal config (~/.orc/config.yaml or .orc/local/config.yaml)
// and are not committed to the repository.
type DeveloperConfig struct {
	// StagingBranch is the personal staging branch for accumulating work.
	// When set and enabled, all tasks (not in an initiative) target this branch.
	// Example: "dev/randy" or "personal/alice"
	StagingBranch string `yaml:"staging_branch,omitempty"`

	// StagingEnabled toggles whether staging branch is active.
	// Allows disabling staging without removing the configuration.
	StagingEnabled bool `yaml:"staging_enabled,omitempty"`

	// AutoSyncAfter creates a PR from staging to main after N tasks merged.
	// 0 = disabled (manual sync via `orc staging sync`).
	AutoSyncAfter int `yaml:"auto_sync_after,omitempty"`
}

// PlaywrightConfig defines Playwright MCP server settings for UI testing.
type PlaywrightConfig struct {
	// Enabled enables auto-configuration of Playwright MCP for UI tasks (default: true)
	Enabled bool `yaml:"enabled"`

	// Headless runs browser in headless mode (default: true)
	// Set to false for debugging to see the browser
	Headless bool `yaml:"headless"`

	// Browser is the browser to use: chromium, firefox, webkit (default: chromium)
	Browser string `yaml:"browser"`

	// TimeoutAction is the action timeout in milliseconds (default: 5000)
	TimeoutAction int `yaml:"timeout_action"`

	// TimeoutNavigation is the navigation timeout in milliseconds (default: 60000)
	TimeoutNavigation int `yaml:"timeout_navigation"`
}

// MCPConfig defines MCP (Model Context Protocol) server configuration.
type MCPConfig struct {
	// Playwright settings for UI testing tasks
	Playwright PlaywrightConfig `yaml:"playwright"`
}

// AutomationMode defines how automation tasks are executed.
type AutomationMode string

const (
	// AutomationModeAuto fires and executes without prompts
	AutomationModeAuto AutomationMode = "auto"
	// AutomationModeApproval creates in pending state, requires human approval
	AutomationModeApproval AutomationMode = "approval"
	// AutomationModeNotify only notifies, human creates task manually
	AutomationModeNotify AutomationMode = "notify"
)

// TriggerType defines the type of automation trigger.
type TriggerType string

const (
	// TriggerTypeCount fires after N tasks/phases complete
	TriggerTypeCount TriggerType = "count"
	// TriggerTypeInitiative fires on initiative events
	TriggerTypeInitiative TriggerType = "initiative"
	// TriggerTypeEvent fires on specific events (pr_merged, etc.)
	TriggerTypeEvent TriggerType = "event"
	// TriggerTypeThreshold fires when metric crosses value
	TriggerTypeThreshold TriggerType = "threshold"
	// TriggerTypeSchedule fires on cron schedule (team mode only)
	TriggerTypeSchedule TriggerType = "schedule"
)

// TriggerConditionConfig defines when a trigger fires.
type TriggerConditionConfig struct {
	// Count-based
	Metric     string   `yaml:"metric,omitempty"`     // tasks_completed, large_tasks_completed, phases_completed
	Threshold  int      `yaml:"threshold,omitempty"`  // Number of items before triggering
	Weights    []string `yaml:"weights,omitempty"`    // Filter by task weight
	Categories []string `yaml:"categories,omitempty"` // Filter by task category

	// Initiative-based / Event-based
	Event  string            `yaml:"event,omitempty"`  // initiative_completed, pr_merged, task_completed, etc.
	Filter map[string]string `yaml:"filter,omitempty"` // Additional filters

	// Threshold-based
	Operator string  `yaml:"operator,omitempty"` // lt, gt, eq
	Value    float64 `yaml:"value,omitempty"`    // Threshold value

	// Schedule-based (team mode only)
	Schedule string `yaml:"schedule,omitempty"` // Cron expression
}

// TriggerActionConfig defines what happens when a trigger fires.
type TriggerActionConfig struct {
	Template string `yaml:"template"`            // Template name
	Priority string `yaml:"priority,omitempty"`  // Task priority
	Queue    string `yaml:"queue,omitempty"`     // Task queue
}

// TriggerCooldownConfig defines the cooldown period for a trigger.
// Supports "N tasks" format for task-count based cooldowns
// and duration format (e.g., "2h") for time-based cooldowns (team mode).
type TriggerCooldownConfig struct {
	Tasks    int           `yaml:"tasks,omitempty"`    // Number of tasks before retriggering
	Duration time.Duration `yaml:"duration,omitempty"` // Time before retriggering (team mode)
}

// UnmarshalYAML handles parsing cooldown from various formats.
func (c *TriggerCooldownConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try parsing as string first (e.g., "5 tasks" or "2h")
	var s string
	if err := unmarshal(&s); err == nil {
		return c.parseString(s)
	}

	// Try parsing as struct
	type rawCooldown TriggerCooldownConfig
	var raw rawCooldown
	if err := unmarshal(&raw); err != nil {
		return err
	}
	*c = TriggerCooldownConfig(raw)
	return nil
}

func (c *TriggerCooldownConfig) parseString(s string) error {
	// Parse "N tasks" format
	var tasks int
	if n, _ := fmt.Sscanf(s, "%d tasks", &tasks); n == 1 {
		c.Tasks = tasks
		return nil
	}
	if n, _ := fmt.Sscanf(s, "%d task", &tasks); n == 1 {
		c.Tasks = tasks
		return nil
	}

	// Parse duration format (e.g., "2h", "30m")
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid cooldown format %q: expected 'N tasks' or duration", s)
	}
	c.Duration = d
	return nil
}

// TriggerConfig defines an automation trigger.
type TriggerConfig struct {
	ID          string                 `yaml:"id"`
	Type        TriggerType            `yaml:"type"`
	Description string                 `yaml:"description,omitempty"`
	Enabled     bool                   `yaml:"enabled"`
	Mode        AutomationMode         `yaml:"mode,omitempty"`
	Condition   TriggerConditionConfig `yaml:"condition"`
	Action      TriggerActionConfig    `yaml:"action"`
	Cooldown    TriggerCooldownConfig  `yaml:"cooldown,omitempty"`
}

// TemplateScriptsConfig defines pre/post execution scripts for templates.
type TemplateScriptsConfig struct {
	Pre  []string `yaml:"pre,omitempty"`
	Post []string `yaml:"post,omitempty"`
}

// AutomationTemplateConfig defines an automation task template.
type AutomationTemplateConfig struct {
	Title       string                `yaml:"title"`
	Description string                `yaml:"description,omitempty"`
	Weight      string                `yaml:"weight"`
	Category    string                `yaml:"category"`
	Phases      []string              `yaml:"phases"`
	Prompt      string                `yaml:"prompt"` // Path to prompt template
	Scripts     TemplateScriptsConfig `yaml:"scripts,omitempty"`
}

// AutomationConfig defines automation trigger settings.
type AutomationConfig struct {
	// Enabled enables the automation system (default: true)
	Enabled bool `yaml:"enabled"`

	// GlobalCooldown is the minimum time between any automation tasks (default: 30m)
	// Prevents trigger storms
	GlobalCooldown time.Duration `yaml:"global_cooldown"`

	// MaxConcurrent is the max parallel automation tasks (default: 1)
	MaxConcurrent int `yaml:"max_concurrent"`

	// DefaultMode is the default execution mode for triggers (default: auto)
	DefaultMode AutomationMode `yaml:"default_mode"`

	// Triggers defines the automation triggers
	Triggers []TriggerConfig `yaml:"triggers,omitempty"`

	// Templates defines automation task templates
	// Key is the template ID, value is the template definition
	Templates map[string]AutomationTemplateConfig `yaml:"templates,omitempty"`
}

// PhaseModelSetting defines model and thinking configuration for a phase.
type PhaseModelSetting struct {
	// Model is the model to use for this phase.
	// Can be an alias (opus, sonnet, haiku) or full model ID.
	// Empty string means use the default model.
	Model string `yaml:"model,omitempty"`

	// Thinking enables extended thinking mode for this phase.
	// When true, "ultrathink" is injected into the prompt to activate
	// maximum thinking budget (31,999 tokens).
	Thinking bool `yaml:"thinking,omitempty"`
}

// WeightModelConfig maps phase names to model settings for a specific weight tier.
// Phase names: research, spec, design, implement, test, review, docs, validate, finalize
type WeightModelConfig map[string]PhaseModelSetting

// ModelsConfig defines model selection and thinking mode per weight tier and phase.
// This allows optimizing model usage: opus for decisions, sonnet for execution,
// thinking mode for spec/design/review phases where deep reasoning helps.
type ModelsConfig struct {
	// Default is the fallback model setting when no specific config exists.
	// Default: {Model: "opus", Thinking: false}
	Default PhaseModelSetting `yaml:"default"`

	// Trivial overrides for trivial weight tasks.
	Trivial WeightModelConfig `yaml:"trivial,omitempty"`

	// Small overrides for small weight tasks.
	Small WeightModelConfig `yaml:"small,omitempty"`

	// Medium overrides for medium weight tasks.
	Medium WeightModelConfig `yaml:"medium,omitempty"`

	// Large overrides for large weight tasks.
	Large WeightModelConfig `yaml:"large,omitempty"`

	// Greenfield overrides for greenfield weight tasks.
	Greenfield WeightModelConfig `yaml:"greenfield,omitempty"`
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

	// MCP (Model Context Protocol) server configuration
	MCP MCPConfig `yaml:"mcp"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Models configuration for per-weight, per-phase model selection
	Models ModelsConfig `yaml:"models"`

	// Automation configuration for triggers and templates
	Automation AutomationConfig `yaml:"automation"`

	// Model settings (legacy - used as fallback if Models.Default.Model is empty)
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

// ResolveModelSetting returns the effective model setting for a phase given task weight.
// Priority: weight-specific phase setting > weight default > global default > legacy Model field
func (c *Config) ResolveModelSetting(weight, phase string) PhaseModelSetting {
	// Get the weight-specific config
	var weightConfig WeightModelConfig
	switch weight {
	case "trivial":
		weightConfig = c.Models.Trivial
	case "small":
		weightConfig = c.Models.Small
	case "medium":
		weightConfig = c.Models.Medium
	case "large":
		weightConfig = c.Models.Large
	case "greenfield":
		weightConfig = c.Models.Greenfield
	}

	// Check weight-specific phase setting
	if weightConfig != nil {
		if setting, ok := weightConfig[phase]; ok {
			// Fill in missing model from default
			if setting.Model == "" {
				setting.Model = c.effectiveDefaultModel()
			}
			return setting
		}
	}

	// Return default with effective model
	result := c.Models.Default
	if result.Model == "" {
		result.Model = c.effectiveDefaultModel()
	}
	return result
}

// effectiveDefaultModel returns the default model, falling back to legacy Model field.
func (c *Config) effectiveDefaultModel() string {
	if c.Models.Default.Model != "" {
		return c.Models.Default.Model
	}
	if c.Model != "" {
		return c.Model
	}
	return "opus" // Ultimate fallback
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
			Action:        "pr",
			TargetBranch:  "main",
			DeleteBranch:  true,
			WaitForCI:     true,             // Wait for CI before merge (replaces auto-merge)
			CITimeout:     10 * time.Minute, // 10 minute default timeout
			MergeOnCIPass: true,             // Merge when CI passes
			PR: PRConfig{
				Title:        "[orc] {{TASK_TITLE}}",
				BodyTemplate: "templates/pr-body.md",
				Labels:       []string{"automated"},
				AutoMerge:    true,
				AutoApprove:  true, // AI-assisted PR approval in auto mode
			},
			CI: CIConfig{
				WaitForCI:     true,              // Wait for CI checks before merge
				CITimeout:     10 * time.Minute,  // Max 10 minutes to wait
				PollInterval:  30 * time.Second,  // Check every 30 seconds
				MergeOnCIPass: true,              // Auto-merge when CI passes
				MergeMethod:   "squash",          // Use squash merge by default
			},
			Sync: SyncConfig{
				Strategy:         SyncStrategyCompletion, // Sync before PR creation by default
				SyncOnStart:      true,                   // Sync at task start to catch stale worktrees
				FailOnConflict:   true,                   // Fail on conflicts by default - let user decide resolution
				MaxConflictFiles: 0,                      // No limit by default
				SkipForWeights:   []string{"trivial"},    // Skip sync for trivial tasks
			},
			Finalize: FinalizeConfig{
				Enabled:               true,  // Finalize phase enabled by default
				AutoTrigger:           true,  // Auto-trigger after validate
				AutoTriggerOnApproval: true,  // Auto-trigger when PR is approved (auto profile only)
				Sync: FinalizeSyncConfig{
					Strategy: FinalizeSyncMerge, // Merge target into branch (preserves history)
				},
				ConflictResolution: ConflictResolutionConfig{
					Enabled:      true, // AI-assisted conflict resolution enabled
					Instructions: "",   // No additional instructions by default
				},
				RiskAssessment: RiskAssessmentConfig{
					Enabled:           true,   // Risk assessment enabled
					ReReviewThreshold: "high", // Recommend re-review at high+ risk
				},
				Gates: FinalizeGatesConfig{
					PreMerge: "auto", // Auto gate before merge/PR by default
				},
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
			Host:            "127.0.0.1",
			Port:            8080,
			MaxPortAttempts: 10,
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
			CoverageThreshold: 85, // Default: 85% coverage required
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
			PhaseMax:          30 * time.Minute,
			TurnMax:           10 * time.Minute,
			IdleWarning:       5 * time.Minute,
			HeartbeatInterval: 30 * time.Second,
			IdleTimeout:       2 * time.Minute,
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
			SkipValidationWeights:   []string{"trivial", "small"},
			MinimumSections:         []string{"intent", "success_criteria", "testing"},
		},
		ArtifactSkip: ArtifactSkipConfig{
			Enabled:  true,                              // Check for existing artifacts
			AutoSkip: false,                             // Prompt user by default
			Phases:   []string{"spec", "research", "docs"}, // Safe phases to skip
		},
		Subtasks: SubtasksConfig{
			AllowCreation: true,
			AutoApprove:   false,
			MaxPending:    10,
		},
		Tasks: TasksConfig{
			DisableAutoCommit: false, // Auto-commit enabled by default
		},
		Diagnostics: DiagnosticsConfig{
			ResourceTracking: ResourceTrackingConfig{
				Enabled:               true,  // Enabled by default to detect orphaned processes
				MemoryThresholdMB:     500,   // Warn if memory grows by >500MB
				LogOrphanedMCPOnly:    false, // Deprecated: use FilterSystemProcesses instead
				FilterSystemProcesses: true,  // Filter system processes to avoid false positives
			},
		},
		MCP: MCPConfig{
			Playwright: PlaywrightConfig{
				Enabled:           true,       // Auto-configure for UI tasks
				Headless:          true,       // Headless for CI, override for debugging
				Browser:           "chromium", // Default browser
				TimeoutAction:     5000,       // 5s action timeout
				TimeoutNavigation: 60000,      // 60s navigation timeout
			},
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
		Automation: AutomationConfig{
			Enabled:        true,                 // Automation enabled by default
			GlobalCooldown: 30 * time.Minute,     // 30 minute global cooldown
			MaxConcurrent:  1,                    // One automation task at a time
			DefaultMode:    AutomationModeAuto,   // Auto mode by default
			Triggers:       nil,                  // No triggers defined by default
			Templates:      nil,                  // No templates defined by default
		},
		Models: ModelsConfig{
			// Default: opus without thinking
			Default: PhaseModelSetting{
				Model:    "opus",
				Thinking: false,
			},
			// Trivial: sonnet for implement (fast, cheap)
			Trivial: WeightModelConfig{
				"implement": {Model: "sonnet", Thinking: false},
			},
			// Small: sonnet for implement/test (execution phases)
			Small: WeightModelConfig{
				"implement": {Model: "sonnet", Thinking: false},
				"test":      {Model: "sonnet", Thinking: false},
			},
			// Medium: thinking for spec/design/review (decision phases)
			Medium: WeightModelConfig{
				"spec":      {Model: "opus", Thinking: true},
				"design":    {Model: "opus", Thinking: true},
				"implement": {Model: "sonnet", Thinking: false},
				"review":    {Model: "opus", Thinking: true},
				"test":      {Model: "sonnet", Thinking: false},
			},
			// Large: thinking for decision phases, opus for all
			Large: WeightModelConfig{
				"spec":      {Model: "opus", Thinking: true},
				"design":    {Model: "opus", Thinking: true},
				"implement": {Model: "opus", Thinking: false},
				"review":    {Model: "opus", Thinking: true},
				"validate":  {Model: "opus", Thinking: true},
			},
			// Greenfield: thinking for research/spec/design/review
			Greenfield: WeightModelConfig{
				"research":  {Model: "opus", Thinking: true},
				"spec":      {Model: "opus", Thinking: true},
				"design":    {Model: "opus", Thinking: true},
				"implement": {Model: "opus", Thinking: false},
				"review":    {Model: "opus", Thinking: true},
				"validate":  {Model: "opus", Thinking: true},
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
// This affects gates, finalize phase, and PR behavior.
func (c *Config) ApplyProfile(profile AutomationProfile) {
	c.Profile = profile
	c.Gates = ProfilePresets(profile)
	c.Completion.Finalize = FinalizePresets(profile)
	c.Completion.PR.AutoApprove = PRAutoApprovePreset(profile)
}

// PRAutoApprovePreset returns the auto-approve setting for a given automation profile.
func PRAutoApprovePreset(profile AutomationProfile) bool {
	switch profile {
	case ProfileAuto, ProfileFast:
		// Auto and Fast profiles enable AI-assisted PR approval
		return true
	case ProfileSafe, ProfileStrict:
		// Safe and Strict profiles require human approval
		return false
	default:
		return true // Default to auto
	}
}

// FinalizePresets returns finalize configuration for a given automation profile.
func FinalizePresets(profile AutomationProfile) FinalizeConfig {
	switch profile {
	case ProfileFast:
		// Fast: minimal overhead, rebase for linear history, skip risk assessment
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: true, // Auto-trigger on PR approval for speed
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncRebase, // Rebase for cleaner history, faster
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true, // Still resolve conflicts automatically
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           false, // Skip risk assessment for speed
				ReReviewThreshold: "high",
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "none", // No pre-merge gate for speed
			},
		}
	case ProfileSafe:
		// Safe: AI reviews, merge strategy (preserves history), risk assessment
		// No auto-trigger on approval - wait for human decision
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: false, // Don't auto-trigger - humans should review before finalize
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge, // Merge preserves history
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "medium", // Lower threshold for safety
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "ai", // AI gate before merge
			},
		}
	case ProfileStrict:
		// Strict: human gates, merge strategy, strict risk assessment
		// No auto-trigger on approval - humans must explicitly trigger finalize
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: false, // Don't auto-trigger - humans must decide
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge, // Merge preserves history
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "low", // Even low risk triggers re-review
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "human", // Human gate before merge
			},
		}
	default: // ProfileAuto
		// Auto: fully automated, merge strategy, auto gates
		// Auto-trigger on approval for full automation
		return FinalizeConfig{
			Enabled:               true,
			AutoTrigger:           true,
			AutoTriggerOnApproval: true, // Auto-trigger when PR is approved
			Sync: FinalizeSyncConfig{
				Strategy: FinalizeSyncMerge,
			},
			ConflictResolution: ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "high",
			},
			Gates: FinalizeGatesConfig{
				PreMerge: "auto",
			},
		}
	}
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

// FindProjectRoot finds the main project root directory that contains the .orc/tasks directory.
// This handles git worktrees where tasks are stored in the main repo, not the worktree.
//
// Resolution order:
// 1. If current directory has .orc/tasks, use current directory
// 2. If in a git worktree, find the main repo and check for .orc/tasks there
// 3. Walk up directories looking for .orc/tasks
// 4. If still not found, return current directory as fallback
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// Check if current directory has tasks
	if hasTasksDir(cwd) {
		return cwd, nil
	}

	// Check if we're in a git worktree
	mainRepo, err := findMainGitRepo()
	if err == nil && mainRepo != "" && mainRepo != cwd {
		if hasTasksDir(mainRepo) {
			return mainRepo, nil
		}
	}

	// Walk up directories looking for .orc/tasks
	dir := cwd
	for {
		if hasTasksDir(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback: return current directory (may have .orc but no tasks yet)
	if IsInitializedAt(cwd) {
		return cwd, nil
	}

	return "", fmt.Errorf("not in an orc project (no %s directory found)", OrcDir)
}

// hasTasksDir checks if a directory has .orc/tasks
func hasTasksDir(dir string) bool {
	tasksPath := filepath.Join(dir, OrcDir, "tasks")
	info, err := os.Stat(tasksPath)
	return err == nil && info.IsDir()
}

// findMainGitRepo uses git to find the main repository when in a worktree.
// Returns empty string if not in a git repo or not in a worktree.
func findMainGitRepo() (string, error) {
	// Get the common git directory (points to main repo's .git)
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	gitCommonDir := strings.TrimSpace(string(output))
	if gitCommonDir == "" || gitCommonDir == ".git" {
		// Not in a worktree, return empty
		return "", nil
	}

	// gitCommonDir is like /path/to/main-repo/.git
	// We want /path/to/main-repo
	if filepath.Base(gitCommonDir) == ".git" {
		return filepath.Dir(gitCommonDir), nil
	}

	// Handle bare repos or unusual setups
	return filepath.Dir(gitCommonDir), nil
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

// ShouldSyncOnStart returns true if sync should happen before task execution starts.
// This catches conflicts from parallel tasks early, while the implement phase can
// still incorporate changes and resolve them.
func (c *Config) ShouldSyncOnStart() bool {
	// If sync is completely disabled, don't sync on start either
	if c.Completion.Sync.Strategy == SyncStrategyNone {
		return false
	}
	return c.Completion.Sync.SyncOnStart
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

// ShouldRunFinalize returns true if the finalize phase should run for this task weight.
func (c *Config) ShouldRunFinalize(weight string) bool {
	if !c.Completion.Finalize.Enabled {
		return false
	}
	// Trivial tasks don't need finalize
	if weight == "trivial" {
		return false
	}
	return true
}

// ShouldAutoTriggerFinalize returns true if finalize should auto-trigger after validate.
func (c *Config) ShouldAutoTriggerFinalize() bool {
	return c.Completion.Finalize.Enabled && c.Completion.Finalize.AutoTrigger
}

// ShouldAutoTriggerFinalizeOnApproval returns true if finalize should auto-trigger when PR is approved.
// This is only enabled for automation profiles that support fully automated workflows (auto, fast).
func (c *Config) ShouldAutoTriggerFinalizeOnApproval() bool {
	return c.Completion.Finalize.Enabled && c.Completion.Finalize.AutoTriggerOnApproval
}

// ShouldAutoApprovePR returns true if AI should review and approve PRs automatically.
// This is only enabled for automation profiles that support fully automated workflows (auto, fast).
// For safe/strict profiles, human approval is required.
func (c *Config) ShouldAutoApprovePR() bool {
	// Only auto mode and fast mode support auto-approval
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
	return c.Completion.PR.AutoApprove
}

// ShouldWaitForCI returns true if we should wait for CI checks before merging.
// Only enabled for auto/fast profiles.
func (c *Config) ShouldWaitForCI() bool {
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
	return c.Completion.CI.WaitForCI
}

// ShouldMergeOnCIPass returns true if we should auto-merge after CI passes.
// Only enabled for auto/fast profiles and requires WaitForCI to be enabled.
func (c *Config) ShouldMergeOnCIPass() bool {
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
	// Can't merge on CI pass if we're not waiting for CI
	return c.Completion.CI.WaitForCI && c.Completion.CI.MergeOnCIPass
}

// CITimeout returns the configured CI timeout, defaulting to 10 minutes.
func (c *Config) CITimeout() time.Duration {
	if c.Completion.CI.CITimeout <= 0 {
		return 10 * time.Minute
	}
	return c.Completion.CI.CITimeout
}

// CIPollInterval returns the CI polling interval, defaulting to 30 seconds.
func (c *Config) CIPollInterval() time.Duration {
	if c.Completion.CI.PollInterval <= 0 {
		return 30 * time.Second
	}
	return c.Completion.CI.PollInterval
}

// MergeMethod returns the configured merge method, defaulting to "squash".
func (c *Config) MergeMethod() string {
	method := c.Completion.CI.MergeMethod
	if method == "" {
		return "squash"
	}
	return method
}

// FinalizeUsesRebase returns true if finalize should use rebase strategy.
func (c *Config) FinalizeUsesRebase() bool {
	return c.Completion.Finalize.Sync.Strategy == FinalizeSyncRebase
}

// ShouldResolveConflicts returns true if AI should attempt to resolve conflicts.
func (c *Config) ShouldResolveConflicts() bool {
	return c.Completion.Finalize.ConflictResolution.Enabled
}

// GetConflictInstructions returns any additional conflict resolution instructions.
func (c *Config) GetConflictInstructions() string {
	return c.Completion.Finalize.ConflictResolution.Instructions
}

// ShouldAssessRisk returns true if risk assessment should be performed.
func (c *Config) ShouldAssessRisk() bool {
	return c.Completion.Finalize.RiskAssessment.Enabled
}

// RiskLevel represents a risk classification level.
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

// String returns the string representation of a risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ParseRiskLevel parses a risk level string.
func ParseRiskLevel(s string) RiskLevel {
	switch strings.ToLower(s) {
	case "low":
		return RiskLow
	case "medium":
		return RiskMedium
	case "high":
		return RiskHigh
	case "critical":
		return RiskCritical
	default:
		return RiskHigh // Default to high for unknown
	}
}

// ShouldReReview returns true if the given risk level meets or exceeds the re-review threshold.
func (c *Config) ShouldReReview(riskLevel RiskLevel) bool {
	if !c.Completion.Finalize.RiskAssessment.Enabled {
		return false
	}
	threshold := ParseRiskLevel(c.Completion.Finalize.RiskAssessment.ReReviewThreshold)
	return riskLevel >= threshold
}

// GetPreMergeGateType returns the gate type for the pre-merge check.
func (c *Config) GetPreMergeGateType() string {
	gateType := c.Completion.Finalize.Gates.PreMerge
	if gateType == "" {
		return "auto"
	}
	return gateType
}

// IsTeamMode returns true if orc is configured for team mode (shared database).
// Team mode enables schedule-based triggers and time-based cooldowns.
func (c *Config) IsTeamMode() bool {
	return c.Database.Driver == "postgres" || c.Team.Mode == "shared_db"
}

// AutomationEnabled returns true if automation is enabled.
func (c *Config) AutomationEnabled() bool {
	return c.Automation.Enabled
}

// GetTriggerMode returns the effective execution mode for a trigger.
// Uses the trigger's mode if set, otherwise falls back to default_mode.
func (c *Config) GetTriggerMode(trigger TriggerConfig) AutomationMode {
	if trigger.Mode != "" {
		return trigger.Mode
	}
	if c.Automation.DefaultMode != "" {
		return c.Automation.DefaultMode
	}
	return AutomationModeAuto
}

// GetAutomationTemplate returns a template by ID, or nil if not found.
func (c *Config) GetAutomationTemplate(id string) *AutomationTemplateConfig {
	if c.Automation.Templates == nil {
		return nil
	}
	if tmpl, ok := c.Automation.Templates[id]; ok {
		return &tmpl
	}
	return nil
}

// GetEnabledTriggers returns all enabled triggers.
func (c *Config) GetEnabledTriggers() []TriggerConfig {
	var enabled []TriggerConfig
	for _, t := range c.Automation.Triggers {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// GetTriggersByType returns all enabled triggers of a specific type.
func (c *Config) GetTriggersByType(triggerType TriggerType) []TriggerConfig {
	var triggers []TriggerConfig
	for _, t := range c.Automation.Triggers {
		if t.Enabled && t.Type == triggerType {
			triggers = append(triggers, t)
		}
	}
	return triggers
}

// SupportsScheduleTriggers returns true if schedule-based triggers are supported.
// Schedule triggers require team mode with a persistent server.
func (c *Config) SupportsScheduleTriggers() bool {
	return c.IsTeamMode()
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

	// ValidFinalizeSyncStrategies are the allowed values for completion.finalize.sync.strategy
	ValidFinalizeSyncStrategies = []string{
		string(FinalizeSyncRebase),
		string(FinalizeSyncMerge),
		"", // empty defaults to merge
	}

	// ValidRiskLevels are the allowed values for risk assessment thresholds
	ValidRiskLevels = []string{"low", "medium", "high", "critical", ""}

	// ValidGateTypes are the allowed values for gate types
	ValidGateTypes = []string{"auto", "ai", "human", "none", ""}

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

	// Validate finalize configuration
	if err := c.validateFinalize(); err != nil {
		return err
	}

	return nil
}

// validateFinalize validates the finalize configuration.
func (c *Config) validateFinalize() error {
	finalize := c.Completion.Finalize

	// Validate finalize sync strategy
	if !contains(ValidFinalizeSyncStrategies, string(finalize.Sync.Strategy)) {
		return fmt.Errorf("invalid completion.finalize.sync.strategy: %s (must be one of: rebase, merge)",
			finalize.Sync.Strategy)
	}

	// Validate risk assessment threshold
	if finalize.RiskAssessment.ReReviewThreshold != "" &&
		!contains(ValidRiskLevels, strings.ToLower(finalize.RiskAssessment.ReReviewThreshold)) {
		return fmt.Errorf("invalid completion.finalize.risk_assessment.re_review_threshold: %s (must be one of: low, medium, high, critical)",
			finalize.RiskAssessment.ReReviewThreshold)
	}

	// Validate pre-merge gate type
	if finalize.Gates.PreMerge != "" && !contains(ValidGateTypes, finalize.Gates.PreMerge) {
		return fmt.Errorf("invalid completion.finalize.gates.pre_merge: %s (must be one of: auto, ai, human, none)",
			finalize.Gates.PreMerge)
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
