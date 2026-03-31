package config

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

