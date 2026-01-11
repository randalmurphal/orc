// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claudeconfig"
)

// LoadProjectToolPermissions loads tool permissions from .claude/settings.json
// and applies them to the executor config if not already set.
// This allows project-level tool restrictions to be enforced during execution.
func (e *Executor) LoadProjectToolPermissions(projectRoot string) error {
	// Only load if not already configured
	if len(e.config.AllowedTools) > 0 || len(e.config.DisallowedTools) > 0 {
		return nil // Already configured, don't override
	}

	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		// No settings file is OK - no tool restrictions
		return nil
	}

	// Check for tool permissions in settings extensions
	perms, err := claudeconfig.GetToolPermissions(settings)
	if err != nil || perms == nil || perms.IsEmpty() {
		return nil
	}

	// Apply permissions to config
	if len(perms.Allow) > 0 {
		e.config.AllowedTools = perms.Allow
		e.logger.Info("loaded allowed tools from project settings", "tools", perms.Allow)
	}
	if len(perms.Deny) > 0 {
		e.config.DisallowedTools = perms.Deny
		e.logger.Info("loaded disallowed tools from project settings", "tools", perms.Deny)
	}

	// Rebuild client with new permissions
	if len(e.config.AllowedTools) > 0 || len(e.config.DisallowedTools) > 0 {
		e.rebuildClient()
	}

	return nil
}

// rebuildClient recreates the Claude client with current config settings.
func (e *Executor) rebuildClient() {
	clientOpts := []claude.ClaudeOption{
		claude.WithModel(e.config.Model),
		claude.WithWorkdir(e.config.WorkDir),
		claude.WithTimeout(e.config.Timeout),
	}

	if e.config.ClaudePath != "" {
		clientOpts = append(clientOpts, claude.WithClaudePath(e.config.ClaudePath))
	}
	if e.config.DangerouslySkipPermissions {
		clientOpts = append(clientOpts, claude.WithDangerouslySkipPermissions())
	}
	if len(e.config.AllowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithAllowedTools(e.config.AllowedTools))
	}
	if len(e.config.DisallowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithDisallowedTools(e.config.DisallowedTools))
	}

	e.client = claude.NewClaudeCLI(clientOpts...)
}
