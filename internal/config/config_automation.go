package config

// AutomationEnabled returns true if automation is enabled.
func (c *Config) AutomationEnabled() bool {
	return c.Automation.Enabled
}

// GetTriggerMode returns the effective execution mode for a trigger.
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
func (c *Config) SupportsScheduleTriggers() bool {
	return c.IsTeamMode()
}
