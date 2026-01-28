package config

import (
	"fmt"
	"strings"
	"time"
)

// AutomationProfile defines preset automation configurations.
type AutomationProfile string

const (
	// ProfileAuto - fully automated, no human intervention (default)
	ProfileAuto AutomationProfile = "auto"
	// ProfileFast - minimal gates, speed over safety
	ProfileFast AutomationProfile = "fast"
	// ProfileSafe - auto gates with human approval for merge
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

	// AllPhases enables gates for ALL phases when true.
	// When false (default), only phases in EnabledPhases have gates.
	AllPhases bool `yaml:"all_phases"`

	// EnabledPhases is a whitelist of phases that have gates.
	// Only used when AllPhases is false.
	// e.g., ["spec", "implement", "test", "review"]
	EnabledPhases []string `yaml:"enabled_phases,omitempty"`

	// DisabledPhases is a blacklist of phases that should NOT have gates.
	// Takes precedence over EnabledPhases.
	// e.g., ["breakdown", "docs"]
	DisabledPhases []string `yaml:"disabled_phases,omitempty"`

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
	// e.g., {"trivial": "merge", "small": "merge"} to skip PR for lightweight tasks
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

// ValidationConfig defines Haiku validation settings.
// This enables objective quality checks that don't rely on LLM self-assessment.
//
// NOTE: Quality checks (tests, lint, build, typecheck) are now defined at the
// phase template level via quality_checks JSON. Project commands are stored in
// the database and seeded during `orc init`.
type ValidationConfig struct {
	// Enabled enables validation checks (default: true)
	Enabled bool `yaml:"enabled"`

	// Model is the model to use for validation calls (default: haiku)
	Model string `yaml:"model"`

	// SkipForWeights skips validation for these task weights (default: [trivial, small])
	SkipForWeights []string `yaml:"skip_for_weights,omitempty"`

	// ValidateSpecs enables Haiku-based spec quality validation before execution (default: true)
	ValidateSpecs bool `yaml:"validate_specs"`

	// ValidateCriteria enables Haiku-based success criteria validation on implement completion
	// (default: true). This checks that all spec success criteria are met before accepting
	// phase completion, ensuring the agent actually did what the spec required.
	ValidateCriteria bool `yaml:"validate_criteria"`

	// FailOnAPIError controls behavior when validation API calls fail (rate limits, network, etc.)
	// true (default): Fail the task properly (resumable) - quality over speed
	// false: Fail open - continue execution without validation (legacy behavior)
	FailOnAPIError bool `yaml:"fail_on_api_error"`
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
	// PhaseMax is the maximum time per phase (0 = unlimited, default: 60m)
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
}

// PlanConfig defines spec content validation configuration.
type PlanConfig struct {
	// MinimumSections are the required sections in a spec (default: [intent, success_criteria, testing])
	// Used by spec phase templates to validate output quality.
	MinimumSections []string `yaml:"minimum_sections,omitempty"`
}

// WeightsConfig defines which workflow to use for each task weight.
// This replaces the hardcoded WeightToWorkflowID() function.
type WeightsConfig struct {
	// Trivial is the workflow ID for trivial weight tasks (default: implement-trivial)
	Trivial string `yaml:"trivial,omitempty"`
	// Small is the workflow ID for small weight tasks (default: implement-small)
	Small string `yaml:"small,omitempty"`
	// Medium is the workflow ID for medium weight tasks (default: implement-medium)
	Medium string `yaml:"medium,omitempty"`
	// Large is the workflow ID for large weight tasks (default: implement-large)
	Large string `yaml:"large,omitempty"`
}

// GetWorkflowID returns the workflow ID for a given weight.
// Falls back to "implement-{weight}" if not configured.
func (w WeightsConfig) GetWorkflowID(weight string) string {
	switch weight {
	case "trivial":
		if w.Trivial != "" {
			return w.Trivial
		}
		return "implement-trivial"
	case "small":
		if w.Small != "" {
			return w.Small
		}
		return "implement-small"
	case "medium":
		if w.Medium != "" {
			return w.Medium
		}
		return "implement-medium"
	case "large":
		if w.Large != "" {
			return w.Large
		}
		return "implement-large"
	default:
		return ""
	}
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
	Template string `yaml:"template"`           // Template name
	Priority string `yaml:"priority,omitempty"` // Task priority
	Queue    string `yaml:"queue,omitempty"`    // Task queue
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
	ValidGateTypes = []string{"auto", "human", "none", ""}

	// DefaultProtectedBranches are branches that cannot be directly merged to
	DefaultProtectedBranches = []string{"main", "master", "develop", "release"}

	// ValidStorageModes are the allowed values for storage.mode
	ValidStorageModes = []string{string(StorageModeHybrid), string(StorageModeFiles), string(StorageModeDatabase)}

	// ValidExportPresets are the allowed values for storage.export.preset
	ValidExportPresets = []string{string(ExportPresetMinimal), string(ExportPresetStandard), string(ExportPresetFull), ""}
)
