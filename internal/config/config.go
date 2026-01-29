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

// ExpandPath expands ~ to the user's home directory.
// Returns the original path unchanged if expansion fails or not needed.
func ExpandPath(path string) string {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
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

	// Validation configuration for Haiku validation and backpressure
	Validation ValidationConfig `yaml:"validation"`

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

	// Weights configuration - maps task weights to workflow IDs
	Weights WeightsConfig `yaml:"weights"`

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

	// Hosting provider configuration (GitHub, GitLab, auto-detect)
	Hosting HostingConfig `yaml:"hosting"`

	// Database configuration
	Database DatabaseConfig `yaml:"database"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Automation configuration for triggers and templates
	Automation AutomationConfig `yaml:"automation"`

	// Model is the default model for all phases (unless overridden in phase templates)
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

// ApplyProfile applies a preset profile to the configuration.
// This affects gates, finalize phase, PR behavior, and validation settings.
func (c *Config) ApplyProfile(profile AutomationProfile) {
	c.Profile = profile
	c.Gates = ProfilePresets(profile)
	c.Completion.Finalize = FinalizePresets(profile)
	c.Completion.PR.AutoApprove = PRAutoApprovePreset(profile)
	c.Validation = ValidationPresets(profile)
}

// Load loads the config using the full loader with proper path expansion.
// This is the recommended way to load config - it handles:
// - Multiple config sources (project, local, user, env)
// - Path expansion (~ to home directory)
// - Config hierarchy merging
func Load() (*Config, error) {
	tc, err := LoadWithSources()
	if err != nil {
		return nil, err
	}
	return tc.Config, nil
}

// LoadFrom loads the config from a specific project directory.
// Uses the full config hierarchy loader with path expansion.
func LoadFrom(projectDir string) (*Config, error) {
	tc, err := LoadWithSourcesFrom(projectDir)
	if err != nil {
		return nil, err
	}
	return tc.Config, nil
}

// LoadFile loads config from a specific file path (for config editing).
// Unlike Load/LoadFrom, this does NOT merge from multiple sources.
// Use this when you need to read and modify a single config file.
func LoadFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Expand paths
	cfg.ClaudePath = ExpandPath(cfg.ClaudePath)

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
	// Allow override via environment variable (for testing)
	if envRoot := os.Getenv("ORC_PROJECT_ROOT"); envRoot != "" {
		return RequireInitAt(envRoot)
	}
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
	// Allow override via environment variable (for testing)
	if envRoot := os.Getenv("ORC_PROJECT_ROOT"); envRoot != "" {
		return envRoot, nil
	}

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

	// Check if we're inside an orc worktree directory (e.g., /repo/.orc/worktrees/task-xxx)
	// If so, extract the main project root from the path
	if mainRoot := extractMainRepoFromWorktreePath(cwd); mainRoot != "" {
		return mainRoot, nil
	}

	// Fallback: return current directory (may have .orc but no tasks yet)
	// IMPORTANT: Only if this looks like a real orc project, not a worktree with
	// tracked .orc/ files. Check for database or tasks directory, not just .orc/.
	if isRealOrcProject(cwd) {
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

// extractMainRepoFromWorktreePath extracts the main project root from a path
// that is inside an orc worktree directory (e.g., /repo/.orc/worktrees/task-xxx).
// Returns empty string if the path doesn't look like a worktree path.
func extractMainRepoFromWorktreePath(path string) string {
	// Look for ".orc/worktrees/" in the path
	worktreeMarker := filepath.Join(OrcDir, "worktrees")
	idx := strings.Index(path, worktreeMarker)
	if idx == -1 {
		return ""
	}

	// Extract the part before ".orc/worktrees/"
	// e.g., /repo/.orc/worktrees/task-xxx -> /repo
	mainRoot := path[:idx]
	if mainRoot == "" {
		return ""
	}

	// Remove trailing slash if present
	mainRoot = strings.TrimSuffix(mainRoot, string(filepath.Separator))

	// Verify this looks like a real project (has database or tasks)
	if hasTasksDir(mainRoot) || hasDatabase(mainRoot) {
		return mainRoot
	}

	return ""
}

// hasDatabase checks if a directory has .orc/orc.db
func hasDatabase(dir string) bool {
	dbPath := filepath.Join(dir, OrcDir, "orc.db")
	info, err := os.Stat(dbPath)
	return err == nil && !info.IsDir()
}

// isRealOrcProject checks if a directory is a real orc project (not just
// a worktree with tracked .orc/ files). A real project has either a database
// or a tasks directory, not just the .orc/ directory from git checkout.
func isRealOrcProject(dir string) bool {
	if !IsInitializedAt(dir) {
		return false
	}
	// Must have either database or tasks directory
	return hasTasksDir(dir) || hasDatabase(dir)
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

// ShouldValidateForWeight returns true if validation should run for this task weight.
func (c *Config) ShouldValidateForWeight(weight string) bool {
	if !c.Validation.Enabled {
		return false
	}
	for _, w := range c.Validation.SkipForWeights {
		if w == weight {
			return false
		}
	}
	return true
}

// ShouldValidateSpec returns true if Haiku spec validation should run.
func (c *Config) ShouldValidateSpec(weight string) bool {
	if !c.Validation.Enabled || !c.Validation.ValidateSpecs {
		return false
	}
	return c.ShouldValidateForWeight(weight)
}

// ShouldValidateCriteria returns true if Haiku criteria validation should run on completion.
func (c *Config) ShouldValidateCriteria(weight string) bool {
	if !c.Validation.Enabled || !c.Validation.ValidateCriteria {
		return false
	}
	return c.ShouldValidateForWeight(weight)
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

	// Validate hosting provider
	if c.Hosting.Provider != "" && !contains(ValidHostingProviders, c.Hosting.Provider) {
		return fmt.Errorf("invalid hosting.provider: %s (must be one of: auto, github, gitlab)",
			c.Hosting.Provider)
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
