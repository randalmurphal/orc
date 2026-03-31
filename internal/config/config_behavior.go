package config

import "fmt"

// ResolveGateType returns the effective gate type for a phase given task weight.
func (c *Config) ResolveGateType(phase string, weight string) string {
	if c.Gates.WeightOverrides != nil {
		if weightOverrides, ok := c.Gates.WeightOverrides[weight]; ok {
			if gateType, ok := weightOverrides[phase]; ok {
				return gateType
			}
		}
	}

	if c.Gates.PhaseOverrides != nil {
		if gateType, ok := c.Gates.PhaseOverrides[phase]; ok {
			return gateType
		}
	}

	if c.Gates.DefaultType != "" {
		return c.Gates.DefaultType
	}

	return "auto"
}

// ShouldRetryFrom returns the phase to retry from if the given phase fails.
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
func (c *Config) ResolveCompletionAction(weight string) string {
	if c.Completion.WeightActions != nil {
		if action, ok := c.Completion.WeightActions[weight]; ok {
			return action
		}
	}
	return c.Completion.Action
}

// ApplyProfile applies a preset profile to the configuration.
func (c *Config) ApplyProfile(profile AutomationProfile) {
	c.Profile = profile
	c.Gates = ProfilePresets(profile)
	c.Completion.Finalize = FinalizePresets(profile)
	c.Completion.PR.AutoApprove = PRAutoApprovePreset(profile)
	c.Validation = ValidationPresets(profile)
}

// ExecutorPrefix returns the prefix for branch/worktree naming based on mode.
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
func (c *Config) EffectiveMaxRetries() int {
	if c.Execution.MaxRetries > 0 {
		return c.Execution.MaxRetries
	}
	if c.Retry.MaxRetries > 0 {
		return c.Retry.MaxRetries
	}
	return 5
}

// ResolveWorkflow resolves the workflow ID using the priority hierarchy.
func (c *Config) ResolveWorkflow(explicitWorkflow, category string) (string, string) {
	if explicitWorkflow != "" {
		return explicitWorkflow, "explicit"
	}

	if c.Workflow != "" && c.workflowDefaultsMatchBuiltins() {
		return c.Workflow, "legacy"
	}

	if categoryDefault := c.WorkflowDefaults.GetDefaultWorkflow(category); categoryDefault != "" {
		if category != "" && categoryDefault != c.WorkflowDefaults.Default {
			return categoryDefault, "category_default"
		}
		if categoryDefault == c.WorkflowDefaults.Default {
			return categoryDefault, "general_default"
		}
	}

	if c.WorkflowDefaults.Default != "" {
		return c.WorkflowDefaults.Default, "general_default"
	}

	if c.Workflow != "" {
		return c.Workflow, "legacy"
	}

	return "", "none"
}

func (c *Config) workflowDefaultsMatchBuiltins() bool {
	if c == nil {
		return false
	}

	builtins := Default().WorkflowDefaults
	return c.WorkflowDefaults == builtins
}

// IsTeamMode returns true if orc is configured for team mode (shared database).
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
		return c.DSN()
	}
	return c.Database.SQLite.GlobalPath
}
