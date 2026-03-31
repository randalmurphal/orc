package config

import "time"

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
func (c *Config) ShouldSyncOnStart() bool {
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
func (c *Config) ShouldAutoTriggerFinalizeOnApproval() bool {
	return c.Completion.Finalize.Enabled && c.Completion.Finalize.AutoTriggerOnApproval
}

// ShouldAutoApprovePR returns true if AI should review and approve PRs automatically.
func (c *Config) ShouldAutoApprovePR() bool {
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
	return c.Completion.PR.AutoApprove
}

// ShouldWaitForCI returns true if we should wait for CI checks before merging.
func (c *Config) ShouldWaitForCI() bool {
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
	return c.Completion.CI.WaitForCI
}

// ShouldMergeOnCIPass returns true if we should auto-merge after CI passes.
func (c *Config) ShouldMergeOnCIPass() bool {
	if c.Profile != ProfileAuto && c.Profile != ProfileFast {
		return false
	}
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
		if c.QualityPolicy.FinalizeRequiresHuman {
			return "human"
		}
		return "auto"
	}
	return gateType
}
