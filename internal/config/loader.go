package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Loader handles loading and merging configuration from multiple sources.
// It implements the 4-level configuration hierarchy:
//
//	Level 1: Runtime (env vars, CLI flags) - highest priority
//	Level 2: Personal (~/.orc/, .orc/local/) - user preferences
//	Level 3: Shared (.orc/shared/, .orc/) - team defaults
//	Level 4: Defaults (built-in) - lowest priority
//
// Note: CLI flags are not handled by the Loader. They are applied at the CLI
// layer (internal/cli) after loading config, using TrackedConfig.SetSource()
// with SourceFlag. This separation allows the config package to remain
// decoupled from CLI framework specifics (Cobra/pflag).
type Loader struct {
	projectDir string // Project directory (containing .orc/)
	userDir    string // User config directory (~/.orc/)
}

// NewLoader creates a new configuration loader.
// If projectDir is empty, FindProjectRoot is used for worktree awareness.
func NewLoader(projectDir string) *Loader {
	if projectDir == "" {
		var err error
		projectDir, err = FindProjectRoot()
		if err != nil {
			// Fall back to cwd - may not be in a project yet
			projectDir, err = os.Getwd()
			if err != nil {
				slog.Warn("NewLoader: cannot determine project directory", "error", err)
				projectDir = "."
			}
		}
	}
	userDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		userDir = filepath.Join(home, ".orc")
	}
	return &Loader{
		projectDir: projectDir,
		userDir:    userDir,
	}
}

// SetUserDir overrides the user config directory (for testing).
func (l *Loader) SetUserDir(dir string) {
	l.userDir = dir
}

// SetProjectDir overrides the project directory (for testing).
func (l *Loader) SetProjectDir(dir string) {
	l.projectDir = dir
}

// Load loads and merges configuration from all levels with source tracking.
//
// Resolution order (later levels override earlier):
//  1. Defaults (built-in)
//  2. Shared: .orc/config.yaml, .orc/shared/config.yaml
//  3. Personal: ~/.orc/config.yaml, .orc/local/config.yaml
//  4. Runtime: environment variables (ORC_*)
//
// Personal settings always override shared settings (individual autonomy).
func (l *Loader) Load() (*TrackedConfig, error) {
	tc := NewTrackedConfig()

	// Level 4: Defaults (already set in NewTrackedConfig)
	markDefaults(tc)

	// Level 3: Shared (team/project defaults)
	// Load .orc/config.yaml first, then .orc/shared/config.yaml can override
	l.loadLevel(tc, LevelShared, SourceShared, []string{
		filepath.Join(l.projectDir, OrcDir, ConfigFileName),           // .orc/config.yaml
		filepath.Join(l.projectDir, OrcDir, "shared", ConfigFileName), // .orc/shared/config.yaml
	})

	// Level 2: Personal (user preferences)
	// Load ~/.orc/config.yaml first, then .orc/local/config.yaml can override
	l.loadLevel(tc, LevelPersonal, SourcePersonal, []string{
		filepath.Join(l.userDir, ConfigFileName),                     // ~/.orc/config.yaml
		filepath.Join(l.projectDir, OrcDir, "local", ConfigFileName), // .orc/local/config.yaml
	})

	// Level 1: Runtime (env vars)
	ApplyEnvVars(tc)

	// Validate the merged configuration
	if err := tc.Config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return tc, nil
}

// loadLevel loads configuration files for a specific level.
// Files are processed in order; later files in the list override earlier ones.
func (l *Loader) loadLevel(tc *TrackedConfig, level ConfigLevel, source ConfigSource, paths []string) {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue // Skip missing files
		}
		if err := mergeFromFileWithPath(tc, path, source); err != nil {
			slog.Warn("failed to load config",
				"level", level.String(),
				"path", path,
				"error", err)
		}
	}
}

// GetConfigPaths returns the list of config file paths that would be checked.
// Useful for debugging and displaying configuration resolution.
func (l *Loader) GetConfigPaths() map[ConfigLevel][]string {
	return map[ConfigLevel][]string{
		LevelShared: {
			filepath.Join(l.projectDir, OrcDir, ConfigFileName),
			filepath.Join(l.projectDir, OrcDir, "shared", ConfigFileName),
		},
		LevelPersonal: {
			filepath.Join(l.userDir, ConfigFileName),
			filepath.Join(l.projectDir, OrcDir, "local", ConfigFileName),
		},
	}
}

// LoadWithSources loads configuration with source tracking.
// This is the main entry point, using the current working directory.
//
// 4-Level Configuration Hierarchy:
//  1. Runtime: env vars, CLI flags (highest priority)
//  2. Personal: ~/.orc/config.yaml, .orc/local/config.yaml
//  3. Shared: .orc/shared/config.yaml, .orc/config.yaml
//  4. Defaults: Built-in values (lowest priority)
//
// Key principle: Personal settings always override shared settings.
// This ensures individual developers maintain control over their preferences.
func LoadWithSources() (*TrackedConfig, error) {
	return NewLoader("").Load()
}

// LoadWithSourcesFrom loads configuration from a specific project directory.
func LoadWithSourcesFrom(projectDir string) (*TrackedConfig, error) {
	return NewLoader(projectDir).Load()
}

// mergeFromFileWithPath merges configuration from a file, tracking the file path.
func mergeFromFileWithPath(tc *TrackedConfig, path string, source ConfigSource) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}

	// Parse YAML into a map to track which fields are set
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}

	// Parse into Config
	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}

	// Merge non-zero values and track sources with file path
	mergeConfigWithPath(tc, &fileCfg, raw, source, path)

	return nil
}

// mergeConfigWithPath merges fileCfg into tc.Config, tracking sources with path.
func mergeConfigWithPath(tc *TrackedConfig, fileCfg *Config, raw map[string]interface{}, source ConfigSource, path string) {
	cfg := tc.Config

	// Top-level fields
	if _, ok := raw["version"]; ok {
		cfg.Version = fileCfg.Version
		tc.SetSourceWithPath("version", source, path)
	}
	if _, ok := raw["profile"]; ok {
		cfg.Profile = fileCfg.Profile
		tc.SetSourceWithPath("profile", source, path)
	}
	if _, ok := raw["model"]; ok {
		cfg.Model = fileCfg.Model
		tc.SetSourceWithPath("model", source, path)
	}
	if _, ok := raw["fallback_model"]; ok {
		cfg.FallbackModel = fileCfg.FallbackModel
		tc.SetSourceWithPath("fallback_model", source, path)
	}
	if _, ok := raw["max_iterations"]; ok {
		cfg.MaxIterations = fileCfg.MaxIterations
		tc.SetSourceWithPath("max_iterations", source, path)
	}
	if _, ok := raw["timeout"]; ok {
		cfg.Timeout = fileCfg.Timeout
		tc.SetSourceWithPath("timeout", source, path)
	}
	if _, ok := raw["branch_prefix"]; ok {
		cfg.BranchPrefix = fileCfg.BranchPrefix
		tc.SetSourceWithPath("branch_prefix", source, path)
	}
	if _, ok := raw["commit_prefix"]; ok {
		cfg.CommitPrefix = fileCfg.CommitPrefix
		tc.SetSourceWithPath("commit_prefix", source, path)
	}
	if _, ok := raw["claude_path"]; ok {
		cfg.ClaudePath = ExpandPath(fileCfg.ClaudePath)
		tc.SetSourceWithPath("claude_path", source, path)
	}
	if _, ok := raw["dangerously_skip_permissions"]; ok {
		cfg.DangerouslySkipPermissions = fileCfg.DangerouslySkipPermissions
		tc.SetSourceWithPath("dangerously_skip_permissions", source, path)
	}
	if _, ok := raw["templates_dir"]; ok {
		cfg.TemplatesDir = fileCfg.TemplatesDir
		tc.SetSourceWithPath("templates_dir", source, path)
	}
	if _, ok := raw["enable_checkpoints"]; ok {
		cfg.EnableCheckpoints = fileCfg.EnableCheckpoints
		tc.SetSourceWithPath("enable_checkpoints", source, path)
	}

	// Nested configs
	if rawGates, ok := raw["gates"].(map[string]interface{}); ok {
		mergeGatesConfigWithPath(cfg, fileCfg, rawGates, tc, source, path)
	}
	if rawRetry, ok := raw["retry"].(map[string]interface{}); ok {
		mergeRetryConfigWithPath(cfg, fileCfg, rawRetry, tc, source, path)
	}
	if rawWorktree, ok := raw["worktree"].(map[string]interface{}); ok {
		mergeWorktreeConfigWithPath(cfg, fileCfg, rawWorktree, tc, source, path)
	}
	if rawCompletion, ok := raw["completion"].(map[string]interface{}); ok {
		mergeCompletionConfigWithPath(cfg, fileCfg, rawCompletion, tc, source, path)
	}
	if rawExecution, ok := raw["execution"].(map[string]interface{}); ok {
		mergeExecutionConfigWithPath(cfg, fileCfg, rawExecution, tc, source, path)
	}
	if rawBudget, ok := raw["budget"].(map[string]interface{}); ok {
		mergeBudgetConfigWithPath(cfg, fileCfg, rawBudget, tc, source, path)
	}
	if rawPool, ok := raw["pool"].(map[string]interface{}); ok {
		mergePoolConfigWithPath(cfg, fileCfg, rawPool, tc, source, path)
	}
	if rawServer, ok := raw["server"].(map[string]interface{}); ok {
		mergeServerConfigWithPath(cfg, fileCfg, rawServer, tc, source, path)
	}
	if rawTeam, ok := raw["team"].(map[string]interface{}); ok {
		mergeTeamConfigWithPath(cfg, fileCfg, rawTeam, tc, source, path)
	}
	if rawTaskID, ok := raw["task_id"].(map[string]interface{}); ok {
		mergeTaskIDConfigWithPath(cfg, fileCfg, rawTaskID, tc, source, path)
	}
	if rawIdentity, ok := raw["identity"].(map[string]interface{}); ok {
		mergeIdentityConfigWithPath(cfg, fileCfg, rawIdentity, tc, source, path)
	}
	if rawDatabase, ok := raw["database"].(map[string]interface{}); ok {
		mergeDatabaseConfigWithPath(cfg, fileCfg, rawDatabase, tc, source, path)
	}
}

func mergeGatesConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["default_type"]; ok {
		cfg.Gates.DefaultType = fileCfg.Gates.DefaultType
		tc.SetSourceWithPath("gates.default_type", source, path)
	}
	if _, ok := raw["auto_approve_on_success"]; ok {
		cfg.Gates.AutoApproveOnSuccess = fileCfg.Gates.AutoApproveOnSuccess
		tc.SetSourceWithPath("gates.auto_approve_on_success", source, path)
	}
	if _, ok := raw["retry_on_failure"]; ok {
		cfg.Gates.RetryOnFailure = fileCfg.Gates.RetryOnFailure
		tc.SetSourceWithPath("gates.retry_on_failure", source, path)
	}
	if _, ok := raw["max_retries"]; ok {
		cfg.Gates.MaxRetries = fileCfg.Gates.MaxRetries
		tc.SetSourceWithPath("gates.max_retries", source, path)
	}
	if _, ok := raw["phase_overrides"]; ok {
		cfg.Gates.PhaseOverrides = fileCfg.Gates.PhaseOverrides
		tc.SetSourceWithPath("gates.phase_overrides", source, path)
	}
	if _, ok := raw["weight_overrides"]; ok {
		cfg.Gates.WeightOverrides = fileCfg.Gates.WeightOverrides
		tc.SetSourceWithPath("gates.weight_overrides", source, path)
	}
}

func mergeRetryConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["enabled"]; ok {
		cfg.Retry.Enabled = fileCfg.Retry.Enabled
		tc.SetSourceWithPath("retry.enabled", source, path)
	}
	if _, ok := raw["max_retries"]; ok {
		cfg.Retry.MaxRetries = fileCfg.Retry.MaxRetries
		tc.SetSourceWithPath("retry.max_retries", source, path)
	}
	if _, ok := raw["retry_map"]; ok {
		cfg.Retry.RetryMap = fileCfg.Retry.RetryMap
		tc.SetSourceWithPath("retry.retry_map", source, path)
	}
}

func mergeWorktreeConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["enabled"]; ok {
		cfg.Worktree.Enabled = fileCfg.Worktree.Enabled
		tc.SetSourceWithPath("worktree.enabled", source, path)
	}
	if _, ok := raw["dir"]; ok {
		cfg.Worktree.Dir = fileCfg.Worktree.Dir
		tc.SetSourceWithPath("worktree.dir", source, path)
	}
	if _, ok := raw["cleanup_on_complete"]; ok {
		cfg.Worktree.CleanupOnComplete = fileCfg.Worktree.CleanupOnComplete
		tc.SetSourceWithPath("worktree.cleanup_on_complete", source, path)
	}
	if _, ok := raw["cleanup_on_fail"]; ok {
		cfg.Worktree.CleanupOnFail = fileCfg.Worktree.CleanupOnFail
		tc.SetSourceWithPath("worktree.cleanup_on_fail", source, path)
	}
}

func mergeCompletionConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["action"]; ok {
		cfg.Completion.Action = fileCfg.Completion.Action
		tc.SetSourceWithPath("completion.action", source, path)
	}
	if _, ok := raw["target_branch"]; ok {
		cfg.Completion.TargetBranch = fileCfg.Completion.TargetBranch
		tc.SetSourceWithPath("completion.target_branch", source, path)
	}
	if _, ok := raw["delete_branch"]; ok {
		cfg.Completion.DeleteBranch = fileCfg.Completion.DeleteBranch
		tc.SetSourceWithPath("completion.delete_branch", source, path)
	}
	// PR config is nested further
	if rawPR, ok := raw["pr"].(map[string]interface{}); ok {
		if _, ok := rawPR["title"]; ok {
			cfg.Completion.PR.Title = fileCfg.Completion.PR.Title
			tc.SetSourceWithPath("completion.pr.title", source, path)
		}
		if _, ok := rawPR["body_template"]; ok {
			cfg.Completion.PR.BodyTemplate = fileCfg.Completion.PR.BodyTemplate
			tc.SetSourceWithPath("completion.pr.body_template", source, path)
		}
		if _, ok := rawPR["labels"]; ok {
			cfg.Completion.PR.Labels = fileCfg.Completion.PR.Labels
			tc.SetSourceWithPath("completion.pr.labels", source, path)
		}
		if _, ok := rawPR["reviewers"]; ok {
			cfg.Completion.PR.Reviewers = fileCfg.Completion.PR.Reviewers
			tc.SetSourceWithPath("completion.pr.reviewers", source, path)
		}
		if _, ok := rawPR["team_reviewers"]; ok {
			cfg.Completion.PR.TeamReviewers = fileCfg.Completion.PR.TeamReviewers
			tc.SetSourceWithPath("completion.pr.team_reviewers", source, path)
		}
		if _, ok := rawPR["assignees"]; ok {
			cfg.Completion.PR.Assignees = fileCfg.Completion.PR.Assignees
			tc.SetSourceWithPath("completion.pr.assignees", source, path)
		}
		if _, ok := rawPR["draft"]; ok {
			cfg.Completion.PR.Draft = fileCfg.Completion.PR.Draft
			tc.SetSourceWithPath("completion.pr.draft", source, path)
		}
		if _, ok := rawPR["maintainer_can_modify"]; ok {
			cfg.Completion.PR.MaintainerCanModify = fileCfg.Completion.PR.MaintainerCanModify
			tc.SetSourceWithPath("completion.pr.maintainer_can_modify", source, path)
		}
		if _, ok := rawPR["auto_merge"]; ok {
			cfg.Completion.PR.AutoMerge = fileCfg.Completion.PR.AutoMerge
			tc.SetSourceWithPath("completion.pr.auto_merge", source, path)
		}
		if _, ok := rawPR["auto_approve"]; ok {
			cfg.Completion.PR.AutoApprove = fileCfg.Completion.PR.AutoApprove
			tc.SetSourceWithPath("completion.pr.auto_approve", source, path)
		}
	}
	// CI config is nested further
	if rawCI, ok := raw["ci"].(map[string]interface{}); ok {
		if _, ok := rawCI["wait_for_ci"]; ok {
			cfg.Completion.CI.WaitForCI = fileCfg.Completion.CI.WaitForCI
			tc.SetSourceWithPath("completion.ci.wait_for_ci", source, path)
		}
		if _, ok := rawCI["ci_timeout"]; ok {
			cfg.Completion.CI.CITimeout = fileCfg.Completion.CI.CITimeout
			tc.SetSourceWithPath("completion.ci.ci_timeout", source, path)
		}
		if _, ok := rawCI["poll_interval"]; ok {
			cfg.Completion.CI.PollInterval = fileCfg.Completion.CI.PollInterval
			tc.SetSourceWithPath("completion.ci.poll_interval", source, path)
		}
		if _, ok := rawCI["merge_on_ci_pass"]; ok {
			cfg.Completion.CI.MergeOnCIPass = fileCfg.Completion.CI.MergeOnCIPass
			tc.SetSourceWithPath("completion.ci.merge_on_ci_pass", source, path)
		}
		if _, ok := rawCI["merge_method"]; ok {
			cfg.Completion.CI.MergeMethod = fileCfg.Completion.CI.MergeMethod
			tc.SetSourceWithPath("completion.ci.merge_method", source, path)
		}
		if _, ok := rawCI["merge_commit_template"]; ok {
			cfg.Completion.CI.MergeCommitTemplate = fileCfg.Completion.CI.MergeCommitTemplate
			tc.SetSourceWithPath("completion.ci.merge_commit_template", source, path)
		}
		if _, ok := rawCI["squash_commit_template"]; ok {
			cfg.Completion.CI.SquashCommitTemplate = fileCfg.Completion.CI.SquashCommitTemplate
			tc.SetSourceWithPath("completion.ci.squash_commit_template", source, path)
		}
		if _, ok := rawCI["verify_sha_on_merge"]; ok {
			cfg.Completion.CI.VerifySHAOnMerge = fileCfg.Completion.CI.VerifySHAOnMerge
			tc.SetSourceWithPath("completion.ci.verify_sha_on_merge", source, path)
		}
	}
}

func mergeExecutionConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["use_session_execution"]; ok {
		cfg.Execution.UseSessionExecution = fileCfg.Execution.UseSessionExecution
		tc.SetSourceWithPath("execution.use_session_execution", source, path)
	}
	if _, ok := raw["session_persistence"]; ok {
		cfg.Execution.SessionPersistence = fileCfg.Execution.SessionPersistence
		tc.SetSourceWithPath("execution.session_persistence", source, path)
	}
	if _, ok := raw["checkpoint_interval"]; ok {
		cfg.Execution.CheckpointInterval = fileCfg.Execution.CheckpointInterval
		tc.SetSourceWithPath("execution.checkpoint_interval", source, path)
	}
	if _, ok := raw["max_retries"]; ok {
		cfg.Execution.MaxRetries = fileCfg.Execution.MaxRetries
		tc.SetSourceWithPath("execution.max_retries", source, path)
	}
}

func mergeBudgetConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["threshold_usd"]; ok {
		cfg.Budget.ThresholdUSD = fileCfg.Budget.ThresholdUSD
		tc.SetSourceWithPath("budget.threshold_usd", source, path)
	}
	if _, ok := raw["alert_on_exceed"]; ok {
		cfg.Budget.AlertOnExceed = fileCfg.Budget.AlertOnExceed
		tc.SetSourceWithPath("budget.alert_on_exceed", source, path)
	}
	if _, ok := raw["pause_on_exceed"]; ok {
		cfg.Budget.PauseOnExceed = fileCfg.Budget.PauseOnExceed
		tc.SetSourceWithPath("budget.pause_on_exceed", source, path)
	}
}

func mergePoolConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["enabled"]; ok {
		cfg.Pool.Enabled = fileCfg.Pool.Enabled
		tc.SetSourceWithPath("pool.enabled", source, path)
	}
	if _, ok := raw["config_path"]; ok {
		cfg.Pool.ConfigPath = fileCfg.Pool.ConfigPath
		tc.SetSourceWithPath("pool.config_path", source, path)
	}
}

func mergeServerConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["host"]; ok {
		cfg.Server.Host = fileCfg.Server.Host
		tc.SetSourceWithPath("server.host", source, path)
	}
	if _, ok := raw["port"]; ok {
		cfg.Server.Port = fileCfg.Server.Port
		tc.SetSourceWithPath("server.port", source, path)
	}
	// Auth is nested
	if rawAuth, ok := raw["auth"].(map[string]interface{}); ok {
		if _, ok := rawAuth["enabled"]; ok {
			cfg.Server.Auth.Enabled = fileCfg.Server.Auth.Enabled
			tc.SetSourceWithPath("server.auth.enabled", source, path)
		}
		if _, ok := rawAuth["type"]; ok {
			cfg.Server.Auth.Type = fileCfg.Server.Auth.Type
			tc.SetSourceWithPath("server.auth.type", source, path)
		}
	}
}

func mergeTeamConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["name"]; ok {
		cfg.Team.Name = fileCfg.Team.Name
		tc.SetSourceWithPath("team.name", source, path)
	}
	if _, ok := raw["activity_logging"]; ok {
		cfg.Team.ActivityLogging = fileCfg.Team.ActivityLogging
		tc.SetSourceWithPath("team.activity_logging", source, path)
	}
	if _, ok := raw["task_claiming"]; ok {
		cfg.Team.TaskClaiming = fileCfg.Team.TaskClaiming
		tc.SetSourceWithPath("team.task_claiming", source, path)
	}
	if _, ok := raw["visibility"]; ok {
		cfg.Team.Visibility = fileCfg.Team.Visibility
		tc.SetSourceWithPath("team.visibility", source, path)
	}
	if _, ok := raw["mode"]; ok {
		cfg.Team.Mode = fileCfg.Team.Mode
		tc.SetSourceWithPath("team.mode", source, path)
	}
	if _, ok := raw["server_url"]; ok {
		cfg.Team.ServerURL = fileCfg.Team.ServerURL
		tc.SetSourceWithPath("team.server_url", source, path)
	}
}

func mergeTaskIDConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["mode"]; ok {
		cfg.TaskID.Mode = fileCfg.TaskID.Mode
		tc.SetSourceWithPath("task_id.mode", source, path)
	}
	if _, ok := raw["prefix_source"]; ok {
		cfg.TaskID.PrefixSource = fileCfg.TaskID.PrefixSource
		tc.SetSourceWithPath("task_id.prefix_source", source, path)
	}
}

func mergeIdentityConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["initials"]; ok {
		cfg.Identity.Initials = fileCfg.Identity.Initials
		tc.SetSourceWithPath("identity.initials", source, path)
	}
	if _, ok := raw["display_name"]; ok {
		cfg.Identity.DisplayName = fileCfg.Identity.DisplayName
		tc.SetSourceWithPath("identity.display_name", source, path)
	}
	if _, ok := raw["email"]; ok {
		cfg.Identity.Email = fileCfg.Identity.Email
		tc.SetSourceWithPath("identity.email", source, path)
	}
}

func mergeDatabaseConfigWithPath(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource, path string) {
	if _, ok := raw["driver"]; ok {
		cfg.Database.Driver = fileCfg.Database.Driver
		tc.SetSourceWithPath("database.driver", source, path)
	}
	// SQLite config is nested
	if rawSQLite, ok := raw["sqlite"].(map[string]interface{}); ok {
		if _, ok := rawSQLite["path"]; ok {
			cfg.Database.SQLite.Path = fileCfg.Database.SQLite.Path
			tc.SetSourceWithPath("database.sqlite.path", source, path)
		}
		if _, ok := rawSQLite["global_path"]; ok {
			cfg.Database.SQLite.GlobalPath = fileCfg.Database.SQLite.GlobalPath
			tc.SetSourceWithPath("database.sqlite.global_path", source, path)
		}
	}
	// Postgres config is nested
	if rawPostgres, ok := raw["postgres"].(map[string]interface{}); ok {
		if _, ok := rawPostgres["host"]; ok {
			cfg.Database.Postgres.Host = fileCfg.Database.Postgres.Host
			tc.SetSourceWithPath("database.postgres.host", source, path)
		}
		if _, ok := rawPostgres["port"]; ok {
			cfg.Database.Postgres.Port = fileCfg.Database.Postgres.Port
			tc.SetSourceWithPath("database.postgres.port", source, path)
		}
		if _, ok := rawPostgres["database"]; ok {
			cfg.Database.Postgres.Database = fileCfg.Database.Postgres.Database
			tc.SetSourceWithPath("database.postgres.database", source, path)
		}
		if _, ok := rawPostgres["user"]; ok {
			cfg.Database.Postgres.User = fileCfg.Database.Postgres.User
			tc.SetSourceWithPath("database.postgres.user", source, path)
		}
		if _, ok := rawPostgres["password"]; ok {
			cfg.Database.Postgres.Password = fileCfg.Database.Postgres.Password
			tc.SetSourceWithPath("database.postgres.password", source, path)
		}
		if _, ok := rawPostgres["ssl_mode"]; ok {
			cfg.Database.Postgres.SSLMode = fileCfg.Database.Postgres.SSLMode
			tc.SetSourceWithPath("database.postgres.ssl_mode", source, path)
		}
		if _, ok := rawPostgres["pool_max"]; ok {
			cfg.Database.Postgres.PoolMax = fileCfg.Database.Postgres.PoolMax
			tc.SetSourceWithPath("database.postgres.pool_max", source, path)
		}
	}
}

// markDefaults marks all config paths as having SourceDefault.
func markDefaults(tc *TrackedConfig) {
	paths := []string{
		"version", "profile", "model", "fallback_model", "max_iterations", "timeout",
		"branch_prefix", "commit_prefix", "claude_path", "dangerously_skip_permissions",
		"templates_dir", "enable_checkpoints",
		"gates.default_type", "gates.auto_approve_on_success", "gates.retry_on_failure", "gates.max_retries",
		"retry.enabled", "retry.max_retries", "retry.retry_map",
		"worktree.enabled", "worktree.dir", "worktree.cleanup_on_complete", "worktree.cleanup_on_fail",
		"completion.action", "completion.target_branch", "completion.delete_branch",
		"completion.pr.title", "completion.pr.body_template", "completion.pr.labels",
		"completion.pr.team_reviewers", "completion.pr.assignees", "completion.pr.maintainer_can_modify",
		"completion.pr.auto_merge", "completion.pr.auto_approve", "completion.pr.draft",
		"completion.ci.wait_for_ci", "completion.ci.ci_timeout", "completion.ci.poll_interval",
		"completion.ci.merge_on_ci_pass", "completion.ci.merge_method",
		"completion.ci.merge_commit_template", "completion.ci.squash_commit_template",
		"completion.ci.verify_sha_on_merge",
		"execution.use_session_execution", "execution.session_persistence", "execution.checkpoint_interval", "execution.max_retries",
		"budget.threshold_usd", "budget.alert_on_exceed", "budget.pause_on_exceed",
		"pool.enabled", "pool.config_path",
		"server.host", "server.port", "server.auth.enabled", "server.auth.type",
		"team.name", "team.activity_logging", "team.task_claiming", "team.visibility", "team.mode", "team.server_url",
		"task_id.mode", "task_id.prefix_source",
		"identity.initials", "identity.display_name", "identity.email",
		"database.driver", "database.sqlite.path", "database.sqlite.global_path",
		"database.postgres.host", "database.postgres.port", "database.postgres.database",
		"database.postgres.user", "database.postgres.password", "database.postgres.ssl_mode",
		"database.postgres.pool_max",
	}

	for _, path := range paths {
		tc.SetSource(path, SourceDefault)
	}
}
