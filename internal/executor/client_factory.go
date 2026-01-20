// Package executor provides the execution engine for orc.
// This file provides centralized client creation to ensure consistent settings.
package executor

import (
	"github.com/randalmurphal/llmkit/claude"
)

// ClientFactory creates Claude clients with consistent settings.
// All clients created through this factory will use:
// - OutputFormatJSON (required for --json-schema)
// - Consistent model, workdir, timeout, and permission settings
type ClientFactory struct {
	config     *Config
	claudePath string
}

// NewClientFactory creates a new ClientFactory from executor config.
func NewClientFactory(cfg *Config) *ClientFactory {
	return &ClientFactory{
		config:     cfg,
		claudePath: resolveClaudePath(cfg.ClaudePath),
	}
}

// baseOptions returns options common to ALL clients.
func (f *ClientFactory) baseOptions() []claude.ClaudeOption {
	opts := []claude.ClaudeOption{
		claude.WithModel(f.config.Model),
		claude.WithWorkdir(f.config.WorkDir),
		claude.WithTimeout(f.config.Timeout),
		claude.WithOutputFormat(claude.OutputFormatJSON), // Required for --json-schema
	}

	if f.claudePath != "" {
		opts = append(opts, claude.WithClaudePath(f.claudePath))
	}

	if f.config.DangerouslySkipPermissions {
		opts = append(opts, claude.WithDangerouslySkipPermissions())
	}

	if len(f.config.AllowedTools) > 0 {
		opts = append(opts, claude.WithAllowedTools(f.config.AllowedTools))
	}

	if len(f.config.DisallowedTools) > 0 {
		opts = append(opts, claude.WithDisallowedTools(f.config.DisallowedTools))
	}

	return opts
}

// NewPhaseClient creates a client for phase execution with JSON schema.
// Parameters:
//   - schema: JSON schema to enforce structured output
//   - sessionID: Session identifier for multi-turn persistence (empty for no session)
//   - resume: If true and sessionID is set, resume an existing session
func (f *ClientFactory) NewPhaseClient(schema string, sessionID string, resume bool) claude.Client {
	opts := f.baseOptions()

	if schema != "" {
		opts = append(opts, claude.WithJSONSchema(schema))
	}

	if resume && sessionID != "" {
		opts = append(opts, claude.WithResume(sessionID))
	} else if sessionID != "" {
		opts = append(opts, claude.WithSessionID(sessionID))
	}

	return claude.NewClaudeCLI(opts...)
}

// NewValidationClient creates a client for Haiku-based validation calls.
// Uses the haiku model for fast, cheap validation checks.
func (f *ClientFactory) NewValidationClient(schema string) claude.Client {
	opts := []claude.ClaudeOption{
		claude.WithModel("haiku"),
		claude.WithWorkdir(f.config.WorkDir),
		claude.WithTimeout(f.config.Timeout),
		claude.WithOutputFormat(claude.OutputFormatJSON),
	}

	if f.claudePath != "" {
		opts = append(opts, claude.WithClaudePath(f.claudePath))
	}

	if schema != "" {
		opts = append(opts, claude.WithJSONSchema(schema))
	}

	return claude.NewClaudeCLI(opts...)
}

// NewSimpleClient creates a basic client without JSON schema.
// Use for operations that don't require structured output.
func (f *ClientFactory) NewSimpleClient() claude.Client {
	return claude.NewClaudeCLI(f.baseOptions()...)
}

// WithModel returns a new factory that uses a different model.
// The returned factory shares the same config but overrides the model.
func (f *ClientFactory) WithModel(model string) *ClientFactory {
	// Create a shallow copy with the model override
	// We don't modify the original config
	return &ClientFactory{
		config: &Config{
			ClaudePath:                 f.config.ClaudePath,
			Model:                      model, // Override
			DangerouslySkipPermissions: f.config.DangerouslySkipPermissions,
			AllowedTools:               f.config.AllowedTools,
			DisallowedTools:            f.config.DisallowedTools,
			MaxIterations:              f.config.MaxIterations,
			Timeout:                    f.config.Timeout,
			WorkDir:                    f.config.WorkDir,
			BranchPrefix:               f.config.BranchPrefix,
			CommitPrefix:               f.config.CommitPrefix,
			TemplatesDir:               f.config.TemplatesDir,
			EnableCheckpoints:          f.config.EnableCheckpoints,
			Backend:                    f.config.Backend,
			OrcConfig:                  f.config.OrcConfig,
		},
		claudePath: f.claudePath,
	}
}
