package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadWithSources loads configuration with source tracking.
// Load order (later sources override earlier):
//  1. Built-in defaults
//  2. System config (/etc/orc/config.yaml) - optional
//  3. User config (~/.orc/config.yaml) - optional
//  4. Project config (.orc/config.yaml)
//  5. Environment variables (ORC_*)
func LoadWithSources() (*TrackedConfig, error) {
	tc := NewTrackedConfig()

	// Mark all defaults with SourceDefault
	markDefaults(tc)

	// 2. System config (/etc/orc/config.yaml)
	systemPath := "/etc/orc/config.yaml"
	if _, err := os.Stat(systemPath); err == nil {
		if err := mergeFromFile(tc, systemPath, SourceSystem); err != nil {
			slog.Warn("failed to load system config", "path", systemPath, "error", err)
		}
	}

	// 3. User config (~/.orc/config.yaml)
	if home, err := os.UserHomeDir(); err == nil {
		userPath := filepath.Join(home, ".orc", "config.yaml")
		if _, err := os.Stat(userPath); err == nil {
			if err := mergeFromFile(tc, userPath, SourceUser); err != nil {
				slog.Warn("failed to load user config", "path", userPath, "error", err)
			}
		}
	}

	// 4. Project config (.orc/config.yaml)
	projectPath := filepath.Join(OrcDir, ConfigFileName)
	if _, err := os.Stat(projectPath); err == nil {
		if err := mergeFromFile(tc, projectPath, SourceProject); err != nil {
			return nil, err // Project config errors are fatal
		}
	}

	// 5. Environment variables
	ApplyEnvVars(tc)

	return tc, nil
}

// mergeFromFile merges configuration from a file into tc.
func mergeFromFile(tc *TrackedConfig, path string, source ConfigSource) error {
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

	// Merge non-zero values and track sources
	mergeConfig(tc, &fileCfg, raw, source)

	return nil
}

// mergeConfig merges fileCfg into tc.Config, tracking sources.
func mergeConfig(tc *TrackedConfig, fileCfg *Config, raw map[string]interface{}, source ConfigSource) {
	cfg := tc.Config

	// Top-level fields
	if _, ok := raw["version"]; ok {
		cfg.Version = fileCfg.Version
		tc.SetSource("version", source)
	}
	if _, ok := raw["profile"]; ok {
		cfg.Profile = fileCfg.Profile
		tc.SetSource("profile", source)
	}
	if _, ok := raw["model"]; ok {
		cfg.Model = fileCfg.Model
		tc.SetSource("model", source)
	}
	if _, ok := raw["fallback_model"]; ok {
		cfg.FallbackModel = fileCfg.FallbackModel
		tc.SetSource("fallback_model", source)
	}
	if _, ok := raw["max_iterations"]; ok {
		cfg.MaxIterations = fileCfg.MaxIterations
		tc.SetSource("max_iterations", source)
	}
	if _, ok := raw["timeout"]; ok {
		cfg.Timeout = fileCfg.Timeout
		tc.SetSource("timeout", source)
	}
	if _, ok := raw["branch_prefix"]; ok {
		cfg.BranchPrefix = fileCfg.BranchPrefix
		tc.SetSource("branch_prefix", source)
	}
	if _, ok := raw["commit_prefix"]; ok {
		cfg.CommitPrefix = fileCfg.CommitPrefix
		tc.SetSource("commit_prefix", source)
	}
	if _, ok := raw["claude_path"]; ok {
		cfg.ClaudePath = fileCfg.ClaudePath
		tc.SetSource("claude_path", source)
	}
	if _, ok := raw["dangerously_skip_permissions"]; ok {
		cfg.DangerouslySkipPermissions = fileCfg.DangerouslySkipPermissions
		tc.SetSource("dangerously_skip_permissions", source)
	}
	if _, ok := raw["templates_dir"]; ok {
		cfg.TemplatesDir = fileCfg.TemplatesDir
		tc.SetSource("templates_dir", source)
	}
	if _, ok := raw["enable_checkpoints"]; ok {
		cfg.EnableCheckpoints = fileCfg.EnableCheckpoints
		tc.SetSource("enable_checkpoints", source)
	}

	// Nested configs
	if rawGates, ok := raw["gates"].(map[string]interface{}); ok {
		mergeGatesConfig(cfg, fileCfg, rawGates, tc, source)
	}
	if rawRetry, ok := raw["retry"].(map[string]interface{}); ok {
		mergeRetryConfig(cfg, fileCfg, rawRetry, tc, source)
	}
	if rawWorktree, ok := raw["worktree"].(map[string]interface{}); ok {
		mergeWorktreeConfig(cfg, fileCfg, rawWorktree, tc, source)
	}
	if rawCompletion, ok := raw["completion"].(map[string]interface{}); ok {
		mergeCompletionConfig(cfg, fileCfg, rawCompletion, tc, source)
	}
	if rawExecution, ok := raw["execution"].(map[string]interface{}); ok {
		mergeExecutionConfig(cfg, fileCfg, rawExecution, tc, source)
	}
	if rawBudget, ok := raw["budget"].(map[string]interface{}); ok {
		mergeBudgetConfig(cfg, fileCfg, rawBudget, tc, source)
	}
	if rawPool, ok := raw["pool"].(map[string]interface{}); ok {
		mergePoolConfig(cfg, fileCfg, rawPool, tc, source)
	}
}

func mergeGatesConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["default_type"]; ok {
		cfg.Gates.DefaultType = fileCfg.Gates.DefaultType
		tc.SetSource("gates.default_type", source)
	}
	if _, ok := raw["auto_approve_on_success"]; ok {
		cfg.Gates.AutoApproveOnSuccess = fileCfg.Gates.AutoApproveOnSuccess
		tc.SetSource("gates.auto_approve_on_success", source)
	}
	if _, ok := raw["retry_on_failure"]; ok {
		cfg.Gates.RetryOnFailure = fileCfg.Gates.RetryOnFailure
		tc.SetSource("gates.retry_on_failure", source)
	}
	if _, ok := raw["max_retries"]; ok {
		cfg.Gates.MaxRetries = fileCfg.Gates.MaxRetries
		tc.SetSource("gates.max_retries", source)
	}
	if _, ok := raw["phase_overrides"]; ok {
		cfg.Gates.PhaseOverrides = fileCfg.Gates.PhaseOverrides
		tc.SetSource("gates.phase_overrides", source)
	}
	if _, ok := raw["weight_overrides"]; ok {
		cfg.Gates.WeightOverrides = fileCfg.Gates.WeightOverrides
		tc.SetSource("gates.weight_overrides", source)
	}
}

func mergeRetryConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["enabled"]; ok {
		cfg.Retry.Enabled = fileCfg.Retry.Enabled
		tc.SetSource("retry.enabled", source)
	}
	if _, ok := raw["max_retries"]; ok {
		cfg.Retry.MaxRetries = fileCfg.Retry.MaxRetries
		tc.SetSource("retry.max_retries", source)
	}
	if _, ok := raw["retry_map"]; ok {
		cfg.Retry.RetryMap = fileCfg.Retry.RetryMap
		tc.SetSource("retry.retry_map", source)
	}
}

func mergeWorktreeConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["enabled"]; ok {
		cfg.Worktree.Enabled = fileCfg.Worktree.Enabled
		tc.SetSource("worktree.enabled", source)
	}
	if _, ok := raw["dir"]; ok {
		cfg.Worktree.Dir = fileCfg.Worktree.Dir
		tc.SetSource("worktree.dir", source)
	}
	if _, ok := raw["cleanup_on_complete"]; ok {
		cfg.Worktree.CleanupOnComplete = fileCfg.Worktree.CleanupOnComplete
		tc.SetSource("worktree.cleanup_on_complete", source)
	}
	if _, ok := raw["cleanup_on_fail"]; ok {
		cfg.Worktree.CleanupOnFail = fileCfg.Worktree.CleanupOnFail
		tc.SetSource("worktree.cleanup_on_fail", source)
	}
}

func mergeCompletionConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["action"]; ok {
		cfg.Completion.Action = fileCfg.Completion.Action
		tc.SetSource("completion.action", source)
	}
	if _, ok := raw["target_branch"]; ok {
		cfg.Completion.TargetBranch = fileCfg.Completion.TargetBranch
		tc.SetSource("completion.target_branch", source)
	}
	if _, ok := raw["delete_branch"]; ok {
		cfg.Completion.DeleteBranch = fileCfg.Completion.DeleteBranch
		tc.SetSource("completion.delete_branch", source)
	}
	// PR config is nested further
	if rawPR, ok := raw["pr"].(map[string]interface{}); ok {
		if _, ok := rawPR["title"]; ok {
			cfg.Completion.PR.Title = fileCfg.Completion.PR.Title
			tc.SetSource("completion.pr.title", source)
		}
		if _, ok := rawPR["body_template"]; ok {
			cfg.Completion.PR.BodyTemplate = fileCfg.Completion.PR.BodyTemplate
			tc.SetSource("completion.pr.body_template", source)
		}
		if _, ok := rawPR["labels"]; ok {
			cfg.Completion.PR.Labels = fileCfg.Completion.PR.Labels
			tc.SetSource("completion.pr.labels", source)
		}
		if _, ok := rawPR["reviewers"]; ok {
			cfg.Completion.PR.Reviewers = fileCfg.Completion.PR.Reviewers
			tc.SetSource("completion.pr.reviewers", source)
		}
		if _, ok := rawPR["draft"]; ok {
			cfg.Completion.PR.Draft = fileCfg.Completion.PR.Draft
			tc.SetSource("completion.pr.draft", source)
		}
		if _, ok := rawPR["auto_merge"]; ok {
			cfg.Completion.PR.AutoMerge = fileCfg.Completion.PR.AutoMerge
			tc.SetSource("completion.pr.auto_merge", source)
		}
	}
}

func mergeExecutionConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["use_session_execution"]; ok {
		cfg.Execution.UseSessionExecution = fileCfg.Execution.UseSessionExecution
		tc.SetSource("execution.use_session_execution", source)
	}
	if _, ok := raw["session_persistence"]; ok {
		cfg.Execution.SessionPersistence = fileCfg.Execution.SessionPersistence
		tc.SetSource("execution.session_persistence", source)
	}
	if _, ok := raw["checkpoint_interval"]; ok {
		cfg.Execution.CheckpointInterval = fileCfg.Execution.CheckpointInterval
		tc.SetSource("execution.checkpoint_interval", source)
	}
}

func mergeBudgetConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["threshold_usd"]; ok {
		cfg.Budget.ThresholdUSD = fileCfg.Budget.ThresholdUSD
		tc.SetSource("budget.threshold_usd", source)
	}
	if _, ok := raw["alert_on_exceed"]; ok {
		cfg.Budget.AlertOnExceed = fileCfg.Budget.AlertOnExceed
		tc.SetSource("budget.alert_on_exceed", source)
	}
	if _, ok := raw["pause_on_exceed"]; ok {
		cfg.Budget.PauseOnExceed = fileCfg.Budget.PauseOnExceed
		tc.SetSource("budget.pause_on_exceed", source)
	}
}

func mergePoolConfig(cfg *Config, fileCfg *Config, raw map[string]interface{}, tc *TrackedConfig, source ConfigSource) {
	if _, ok := raw["enabled"]; ok {
		cfg.Pool.Enabled = fileCfg.Pool.Enabled
		tc.SetSource("pool.enabled", source)
	}
	if _, ok := raw["config_path"]; ok {
		cfg.Pool.ConfigPath = fileCfg.Pool.ConfigPath
		tc.SetSource("pool.config_path", source)
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
		"completion.pr.auto_merge", "completion.pr.draft",
		"execution.use_session_execution", "execution.session_persistence", "execution.checkpoint_interval",
		"budget.threshold_usd", "budget.alert_on_exceed", "budget.pause_on_exceed",
		"pool.enabled", "pool.config_path",
	}

	for _, path := range paths {
		tc.SetSource(path, SourceDefault)
	}
}
