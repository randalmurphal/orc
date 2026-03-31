package config

import (
	"fmt"
	"os"
	"strings"
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

	if !IsValidLLMProvider(c.Provider) {
		return fmt.Errorf("invalid provider: %s (must be one of: claude, codex)",
			c.Provider)
	}

	if c.Hosting.Provider != "" && !contains(ValidHostingProviders, c.Hosting.Provider) {
		return fmt.Errorf("invalid hosting.provider: %s (must be one of: auto, github, gitlab)",
			c.Hosting.Provider)
	}

	if c.Completion.Action != "" && !contains(ValidCompletionActions, c.Completion.Action) {
		return fmt.Errorf("invalid completion.action: %s (must be one of: pr, merge, commit, none)",
			c.Completion.Action)
	}

	if !contains(ValidSyncStrategies, string(c.Completion.Sync.Strategy)) {
		return fmt.Errorf("invalid completion.sync.strategy: %s (must be one of: none, phase, completion, detect)",
			c.Completion.Sync.Strategy)
	}

	if c.Completion.Action == "merge" {
		targetBranch := c.Completion.TargetBranch
		if targetBranch == "" {
			targetBranch = "main"
		}
		if isProtectedBranch(targetBranch) {
			return fmt.Errorf("completion.action 'merge' is blocked for protected branch '%s'; "+
				"use 'pr' action instead to ensure code review before merging to protected branches",
				targetBranch)
		}
	}

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

	if !c.Worktree.Enabled {
		return fmt.Errorf("worktree.enabled cannot be set to false; " +
			"worktree isolation is required for safe parallel task execution and branch protection; " +
			"if you need to run without worktrees, contact maintainers to discuss your use case")
	}

	if err := c.validateDatabase(); err != nil {
		return err
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	if err := c.validateFinalize(); err != nil {
		return err
	}

	if c.Knowledge.Indexing.EmbeddingModel != "" &&
		!contains(ValidEmbeddingModels, c.Knowledge.Indexing.EmbeddingModel) {
		return fmt.Errorf("invalid knowledge.indexing.embedding_model: %s (must be one of: %s)",
			c.Knowledge.Indexing.EmbeddingModel, strings.Join(ValidEmbeddingModels, ", "))
	}

	if err := c.validateProviderRates(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateProviderRates() error {
	for provider, models := range c.Providers.Rates {
		if strings.TrimSpace(provider) == "" {
			return fmt.Errorf("invalid providers.rates: provider key cannot be empty")
		}
		for model, rate := range models {
			if strings.TrimSpace(model) == "" {
				return fmt.Errorf("invalid providers.rates.%s: model key cannot be empty", provider)
			}
			if rate.Input < 0 || rate.Output < 0 || rate.CacheRead < 0 || rate.CacheWrite < 0 {
				return fmt.Errorf("invalid providers.rates.%s.%s: rates must be non-negative", provider, model)
			}
		}
	}
	return nil
}

func (c *Config) validateFinalize() error {
	finalize := c.Completion.Finalize

	if !contains(ValidFinalizeSyncStrategies, string(finalize.Sync.Strategy)) {
		return fmt.Errorf("invalid completion.finalize.sync.strategy: %s (must be one of: rebase, merge)",
			finalize.Sync.Strategy)
	}

	if finalize.RiskAssessment.ReReviewThreshold != "" &&
		!contains(ValidRiskLevels, strings.ToLower(finalize.RiskAssessment.ReReviewThreshold)) {
		return fmt.Errorf("invalid completion.finalize.risk_assessment.re_review_threshold: %s (must be one of: low, medium, high, critical)",
			finalize.RiskAssessment.ReReviewThreshold)
	}

	if finalize.Gates.PreMerge != "" && !contains(ValidGateTypes, finalize.Gates.PreMerge) {
		return fmt.Errorf("invalid completion.finalize.gates.pre_merge: %s (must be one of: auto, ai, human, none)",
			finalize.Gates.PreMerge)
	}

	return nil
}

func isProtectedBranch(branch string) bool {
	for _, p := range DefaultProtectedBranches {
		if branch == p {
			return true
		}
	}
	return false
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// validateDatabase validates the database dialect configuration.
func (c *Config) validateDatabase() error {
	dialect := c.Database.Dialect

	if dialect == "" || dialect == "sqlite" {
		return nil
	}

	if dialect != "postgres" {
		return fmt.Errorf("invalid database.dialect: %s (must be one of: sqlite, postgres)", dialect)
	}

	if c.User.Name == "" {
		return fmt.Errorf("database.dialect 'postgres' requires user.name to be set for user attribution")
	}

	if c.Database.DSNEnv == "" {
		return fmt.Errorf("database.dialect 'postgres' requires database.dsn_env to be set (environment variable containing DSN)")
	}

	dsnValue := os.Getenv(c.Database.DSNEnv)
	if dsnValue == "" {
		return fmt.Errorf("environment variable %s is not set or is empty (required for postgres mode)", c.Database.DSNEnv)
	}

	return nil
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

	if c.Storage.Database.RetentionDays < 0 || c.Storage.Database.RetentionDays > 3650 {
		return fmt.Errorf("storage.database.retention_days must be between 0 and 3650")
	}

	return nil
}
