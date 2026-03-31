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

// ShouldSyncOnStart returns true if sync should happen before task execution starts.
func (c *Config) ShouldSyncOnStart() bool {
	if c.Completion.Sync.Strategy == SyncStrategyNone {
		return false
	}
	return c.Completion.Sync.SyncOnStart
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

// ShouldAutoTriggerFinalizeOnApproval returns true if finalize should auto-trigger when PR is approved.
func (c *Config) ShouldAutoTriggerFinalizeOnApproval() bool {
	return c.Completion.Finalize.Enabled && c.Completion.Finalize.AutoTriggerOnApproval
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


// ResolveCompletionAction returns the effective completion action for a task weight.
func (c *Config) ResolveCompletionAction(weight string) string {
	if c.Completion.WeightActions != nil {
		if action, ok := c.Completion.WeightActions[weight]; ok {
			return action
		}
	}
	return c.Completion.Action
}
